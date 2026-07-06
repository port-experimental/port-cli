#!/usr/bin/env bash
set -euo pipefail

: "${PORT_CLIENT_ID:?Set PORT_CLIENT_ID for the base org}"
: "${PORT_CLIENT_SECRET:?Set PORT_CLIENT_SECRET for the base org}"
: "${PORT_TARGET_CLIENT_ID:?Set PORT_TARGET_CLIENT_ID for the target org}"
: "${PORT_TARGET_CLIENT_SECRET:?Set PORT_TARGET_CLIENT_SECRET for the target org}"

BASE_CLIENT_ID="$PORT_CLIENT_ID"
BASE_CLIENT_SECRET="$PORT_CLIENT_SECRET"
TARGET_CLIENT_ID="$PORT_TARGET_CLIENT_ID"
TARGET_CLIENT_SECRET="$PORT_TARGET_CLIENT_SECRET"
API_URL="${PORT_API_URL:-https://api.getport.io/v1}"
TARGET_API_URL="${PORT_TARGET_API_URL:-$API_URL}"
BASE_ORG="${PORT_BASE_ORG:-base}"
TARGET_ORG="${PORT_TARGET_ORG:-target}"
BIN="${PORT_BIN:-./bin/port}"

workdir="$(mktemp -d)"
cleanup() { rm -rf "$workdir"; }
trap cleanup EXIT

cat > "$workdir/config.yaml" <<EOF
default_org: ${BASE_ORG}
organizations:
  ${BASE_ORG}:
    client_id: ${BASE_CLIENT_ID}
    client_secret: ${BASE_CLIENT_SECRET}
    api_url: ${API_URL}
  ${TARGET_ORG}:
    client_id: ${TARGET_CLIENT_ID}
    client_secret: ${TARGET_CLIENT_SECRET}
    api_url: ${TARGET_API_URL}
EOF
chmod 600 "$workdir/config.yaml"

run() {
  label="$1"; shift
  start=$(date +%s)
  env -u PORT_CLIENT_ID -u PORT_CLIENT_SECRET -u PORT_TARGET_CLIENT_ID -u PORT_TARGET_CLIENT_SECRET "$@"
  code=$?
  end=$(date +%s)
  echo "${label}: exit=${code} duration=$((end-start))s" >&2
  return "$code"
}

run "base blueprints" "$BIN" --no-env-file --config "$workdir/config.yaml" api blueprints list --org "$BASE_ORG" --format json > "$workdir/base-blueprints.json"
run "target blueprints" "$BIN" --no-env-file --config "$workdir/config.yaml" api blueprints list --org "$TARGET_ORG" --format json > "$workdir/target-blueprints.json"
run "compare" "$BIN" --no-env-file --config "$workdir/config.yaml" compare --source "$BASE_ORG" --target "$TARGET_ORG" --output json > "$workdir/compare.json"
run "export schema" "$BIN" --no-env-file --config "$workdir/config.yaml" export --base-org "$BASE_ORG" --skip-entities --output "$workdir/base-schema.tar.gz" --output-format json > "$workdir/export.json"
run "migrate dry-run" "$BIN" --no-env-file --config "$workdir/config.yaml" migrate --source-org "$BASE_ORG" --target-org "$TARGET_ORG" --skip-entities --dry-run --output-format json > "$workdir/migrate.json"

python3 - <<'PY' "$workdir"
import json, os, sys
root = sys.argv[1]
def load(name):
    with open(os.path.join(root, name), encoding='utf-8') as f:
        return json.load(f)
print('base_blueprints', len(load('base-blueprints.json')))
print('target_blueprints', len(load('target-blueprints.json')))
compare = load('compare.json')
print('compare_identical', compare.get('identical'), compare.get('summary'))
export = load('export.json')
print('export_success', export.get('success'), 'archive_bytes', os.path.getsize(os.path.join(root, 'base-schema.tar.gz')))
migrate = load('migrate.json')
print('migrate_success', migrate.get('success'))
print('migrate_counts', {k: migrate.get(k) for k in ('blueprints_created', 'blueprints_updated', 'blueprint_permissions_updated', 'pages_updated', 'integrations_updated')})
PY
