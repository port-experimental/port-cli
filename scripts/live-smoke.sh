#!/usr/bin/env bash
# Live smoke checks against real Port orgs (read-only / dry-run only).
# Never prints credential values. Unit tests remain the CI gate; this is manual/opt-in.
set -euo pipefail

usage() {
  cat >&2 <<'EOF'
live-smoke: read-only / dry-run checks against real Port organizations.

Required environment variables (values are never printed):
  PORT_CLIENT_ID
  PORT_CLIENT_SECRET
  PORT_TARGET_CLIENT_ID
  PORT_TARGET_CLIENT_SECRET

Optional:
  PORT_API_URL          (default https://api.getport.io/v1)
  PORT_TARGET_API_URL   (default PORT_API_URL)
  PORT_BASE_ORG         (default base)
  PORT_TARGET_ORG       (default target)
  PORT_BIN              (default ./bin/port)

Run via: make live-smoke
Docs:    docs/live-smoke.md
EOF
}

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "live-smoke: missing required environment variable: ${name}" >&2
    usage
    exit 1
  fi
}

require_env PORT_CLIENT_ID
require_env PORT_CLIENT_SECRET
require_env PORT_TARGET_CLIENT_ID
require_env PORT_TARGET_CLIENT_SECRET

BASE_CLIENT_ID="$PORT_CLIENT_ID"
BASE_CLIENT_SECRET="$PORT_CLIENT_SECRET"
TARGET_CLIENT_ID="$PORT_TARGET_CLIENT_ID"
TARGET_CLIENT_SECRET="$PORT_TARGET_CLIENT_SECRET"
API_URL="${PORT_API_URL:-https://api.getport.io/v1}"
TARGET_API_URL="${PORT_TARGET_API_URL:-$API_URL}"
BASE_ORG="${PORT_BASE_ORG:-base}"
TARGET_ORG="${PORT_TARGET_ORG:-target}"
BIN="${PORT_BIN:-./bin/port}"

if [[ ! -x "$BIN" ]]; then
  echo "live-smoke: PORT_BIN is missing or not executable: ${BIN}" >&2
  echo "live-smoke: run 'make build' first, or set PORT_BIN to a built port binary." >&2
  exit 1
fi

workdir="$(mktemp -d "${TMPDIR:-/tmp}/port-live-smoke.XXXXXX")"
cleanup() { rm -rf "$workdir"; }
trap cleanup EXIT

# Credentials live only in this chmod-600 config; child processes unset env secrets.
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
  local label="$1"
  shift
  local start end code
  start=$(date +%s)
  set +e
  # Drop machine credentials from the process environment so CLI output/logs
  # cannot echo them from env; config file is the sole credential source.
  env -u PORT_CLIENT_ID -u PORT_CLIENT_SECRET -u PORT_TARGET_CLIENT_ID -u PORT_TARGET_CLIENT_SECRET \
    -u PORT_DEBUG \
    "$@"
  code=$?
  set -e
  end=$(date +%s)
  echo "live-smoke: ${label}: exit=${code} duration=$((end - start))s" >&2
  if [[ "$code" -ne 0 ]]; then
    echo "live-smoke: step failed: ${label}" >&2
    exit "$code"
  fi
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
    path = os.path.join(root, name)
    with open(path, encoding="utf-8") as f:
        return json.load(f)

def require(cond, message):
    if not cond:
        print(f"live-smoke: assertion failed: {message}", file=sys.stderr)
        sys.exit(1)

base_bps = load("base-blueprints.json")
target_bps = load("target-blueprints.json")
require(isinstance(base_bps, list), "base blueprints JSON must be a list")
require(isinstance(target_bps, list), "target blueprints JSON must be a list")
print("base_blueprints", len(base_bps))
print("target_blueprints", len(target_bps))

compare = load("compare.json")
require(isinstance(compare, dict), "compare JSON must be an object")
require("identical" in compare or "summary" in compare, "compare JSON missing identical/summary")
identical = compare.get("identical")
print("compare_identical", identical, compare.get("summary"))
if identical is True:
    print("note: orgs compared identical (expected when base and target mirror each other)")

export = load("export.json")
archive = os.path.join(root, "base-schema.tar.gz")
require(export.get("success") is True, "export schema-only must report success=true")
require(os.path.isfile(archive) and os.path.getsize(archive) > 0, "export archive missing or empty")
print("export_success", export.get("success"), "archive_bytes", os.path.getsize(archive))

migrate = load("migrate.json")
require(migrate.get("success") is True, "migrate dry-run must report success=true")
print("migrate_success", migrate.get("success"))
print(
    "migrate_counts",
    {
        k: migrate.get(k)
        for k in (
            "blueprints_created",
            "blueprints_updated",
            "blueprint_permissions_updated",
            "pages_updated",
            "integrations_updated",
        )
    },
)
print("live-smoke: OK")
PY
