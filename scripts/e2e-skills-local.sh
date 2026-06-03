#!/usr/bin/env bash
# Local skills E2E smoke tests (NOT run in CI).
# Prerequisites: see docs/skills-e2e-local.md
#
# Uses the repo-local binary by default: ${REPO_ROOT}/bin/port (from `make build`).
# Override with PORT_BIN. Set SKIP_BUILD=1 to skip `make build` (e.g. when invoked via `make e2e-skills-local`).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
FIXTURES="${SCRIPT_DIR}/e2e-skills/fixtures"

PORT_BIN="${PORT_BIN:-${REPO_ROOT}/bin/port}"
PORT_API_URL="${PORT_API_URL:-http://localhost:3000/v1}"
PORT_AI_SERVICE_URL="${PORT_AI_SERVICE_URL:-http://localhost:3016/v1}"
ORG="${ORG:-org_BneDtWovPqXaA2VZ}"

RUN_ID="$(date +%s)"
E2E_CONFIG_DIR="${TMPDIR:-/tmp}/port-cli-e2e-${RUN_ID}"
E2E_WORKDIR="${E2E_CONFIG_DIR}/workdir"
E2E_CURSOR="${E2E_WORKDIR}/.cursor"
CONFIG_FILE="${E2E_CONFIG_DIR}/config.yaml"

PASS=0
FAIL=0
SKIP=0

green() { printf '\033[32m%s\033[0m\n' "$*"; }
red() { printf '\033[31m%s\033[0m\n' "$*"; }
yellow() { printf '\033[33m%s\033[0m\n' "$*"; }

pass() { PASS=$((PASS + 1)); green "PASS: $1"; }
fail() { FAIL=$((FAIL + 1)); red "FAIL: $1"; }
skip() { SKIP=$((SKIP + 1)); yellow "SKIP: $1"; }

# Load ~/.port/.env into this shell (the Port CLI also reads it, but we need vars for --client-id flags).
load_port_auth_env() {
	local env_file="${PORT_ENV_FILE:-${HOME}/.port/.env}"
	if [[ -f "${env_file}" ]]; then
		set -a
		# shellcheck disable=SC1090
		source "${env_file}"
		set +a
	fi
	export PORT_API_URL="${PORT_API_URL:-http://localhost:3000/v1}"
	export PORT_AI_SERVICE_URL="${PORT_AI_SERVICE_URL:-http://localhost:3016/v1}"
}

# Root-level flags only (--org belongs on `port skills`, not the root command).
port_auth_args() {
	local args=()
	if [[ -n "${PORT_CLIENT_ID:-}" ]]; then
		args+=(--client-id "${PORT_CLIENT_ID}")
	fi
	if [[ -n "${PORT_CLIENT_SECRET:-}" ]]; then
		args+=(--client-secret "${PORT_CLIENT_SECRET}")
	fi
	if [[ -n "${PORT_API_URL:-}" ]]; then
		args+=(--api-url "${PORT_API_URL}")
	fi
	printf '%s\n' "${args[@]}"
}

skills_org_name() {
	if [[ -n "${ORG:-}" ]]; then
		printf '%s' "${ORG}"
	elif [[ -n "${PORT_DEFAULT_ORG:-}" ]]; then
		printf '%s' "${PORT_DEFAULT_ORG}"
	fi
}

run_port_skills() {
	local org
	org="$(skills_org_name)"
	if [[ -n "${org}" ]]; then
		run_port skills --org "${org}" "$@"
	else
		run_port skills "$@"
	fi
}

# Isolated skills config under $TMPDIR, but reuse org credentials + OAuth creds from ~/.port/.
setup_e2e_config() {
	local user_config="${HOME}/.port/config.yaml"
	local user_creds="${HOME}/.port/creds.json"

	if [[ -f "${user_creds}" ]]; then
		ln -sf "${user_creds}" "${E2E_CONFIG_DIR}/creds.json"
	fi

	if [[ -f "${user_config}" ]] && python3 -c 'import yaml' 2>/dev/null; then
		python3 - "${user_config}" "${CONFIG_FILE}" "${E2E_CURSOR}" <<'PY'
import os
import sys
import yaml

src, dst, cursor = sys.argv[1], sys.argv[2], sys.argv[3]
with open(src, encoding="utf-8") as f:
    data = yaml.safe_load(f) or {}
org_override = os.environ.get("ORG") or os.environ.get("PORT_DEFAULT_ORG")
if org_override:
    data["default_org"] = org_override
data["skills"] = {
    "targets": [cursor],
    "select_all_groups": True,
    "select_all_ungrouped": True,
}
with open(dst, "w", encoding="utf-8") as f:
    yaml.safe_dump(data, f, sort_keys=False)
PY
		return
	fi

	if [[ -f "${user_config}" ]]; then
		yellow "NOTE: install PyYAML (pip install pyyaml) to merge ~/.port/config.yaml; using env/flags only for auth"
	fi

	cat >"${CONFIG_FILE}" <<EOF
skills:
  targets:
    - ${E2E_CURSOR}
  select_all_groups: true
  select_all_ungrouped: true
EOF
}

run_port() {
	local auth_args=()
	local arg
	if [[ -n "${PORT_CLIENT_ID:-}${PORT_CLIENT_SECRET:-}${PORT_API_URL:-}${PORT_DEFAULT_ORG:-}" ]]; then
		while IFS= read -r arg; do
			[[ -n "${arg}" ]] && auth_args+=("${arg}")
		done < <(port_auth_args)
	fi
	"${PORT_BIN}" --config "${CONFIG_FILE}" "${auth_args[@]}" "$@"
}

assert_cmd() {
	local name="$1"
	shift
	if "$@"; then
		pass "${name}"
	else
		fail "${name}"
	fi
}

build_port_cli() {
	if [[ "${SKIP_BUILD:-0}" == "1" ]]; then
		return
	fi
	echo "Building port CLI → ${REPO_ROOT}/bin/port"
	make -C "${REPO_ROOT}" build
}

main() {
	build_port_cli

	echo "Port skills local E2E (run_id=${RUN_ID})"
	echo "  PORT_BIN=${PORT_BIN}"
	echo "  PORT_API_URL=${PORT_API_URL}"
	echo "  PORT_AI_SERVICE_URL=${PORT_AI_SERVICE_URL}"
	echo "  CONFIG_FILE=${CONFIG_FILE}"
	echo ""

	if [[ ! -x "${PORT_BIN}" ]]; then
		red "Binary not found at ${PORT_BIN} — run: make build"
		exit 1
	fi

	mkdir -p "${E2E_CURSOR}/skills" "${E2E_WORKDIR}" "${E2E_CONFIG_DIR}"
	export CURSOR_CONFIG_DIR="${E2E_CURSOR}"

	load_port_auth_env
	setup_e2e_config

	if [[ -n "${PORT_CLIENT_ID:-}" ]]; then
		echo "  Auth: machine credentials (PORT_CLIENT_ID from env / ~/.port/.env)"
	elif [[ -f "${E2E_CONFIG_DIR}/creds.json" ]]; then
		echo "  Auth: OAuth creds.json (symlinked from ~/.port/)"
	else
		echo "  Auth: none detected — set PORT_CLIENT_ID/SECRET in ~/.port/.env or run port auth login"
	fi
	if org="$(skills_org_name)"; [[ -n "${org}" ]]; then
		echo "  Skills org: ${org}"
	fi
	echo ""

	# 1 — Preflight
	if curl -sf "${PORT_API_URL%/v1}/v1/health" >/dev/null 2>&1 || curl -sf "${PORT_API_URL%/v1}/health" >/dev/null 2>&1; then
		pass "preflight: port-api reachable"
	else
		fail "preflight: port-api not reachable at ${PORT_API_URL}"
	fi

	if curl -sf "${PORT_AI_SERVICE_URL%/v1}/v1/health" >/dev/null 2>&1 || curl -sf "${PORT_AI_SERVICE_URL%/v1}/health" >/dev/null 2>&1; then
		pass "preflight: ai-service reachable"
	else
		fail "preflight: ai-service not reachable at ${PORT_AI_SERVICE_URL}"
	fi

	if run_port_skills list --json >/dev/null 2>"${E2E_CONFIG_DIR}/skills-list.err"; then
		pass "preflight: port skills list (auth)"
	else
		preflight_err="$(tail -n 1 "${E2E_CONFIG_DIR}/skills-list.err" 2>/dev/null | tr -d '\r' || true)"
		fail "preflight: port skills list — ${preflight_err:-check ~/.port/.env, ~/.port/config.yaml, or port auth login}"
	fi

	# 2 — list contains demo ids
	if run_port_skills list --json 2>/dev/null | grep -q 'demo-onboarding'; then
		pass "list contains demo-onboarding"
	else
		fail "list missing demo-onboarding (run yarn seed:demo-skills in Port repo?)"
	fi

	# 3 — search
	if run_port_skills search demo --json 2>/dev/null | grep -q 'demo'; then
		pass "search demo returns matches"
	else
		fail "search demo returned no matches"
	fi

	# 4 — sync
	if run_port_skills sync --ignore-git-dirty >/dev/null 2>&1; then
		if [[ -f "${E2E_CURSOR}/skills/port/demo-engineering-required/demo-onboarding/SKILL.md" ]] || \
			find "${E2E_CURSOR}/skills/port" -path '*/demo-onboarding/SKILL.md' -print -quit 2>/dev/null | grep -q .; then
			pass "sync wrote demo skill files"
		else
			fail "sync succeeded but demo-onboarding SKILL.md not found under ${E2E_CURSOR}/skills/port"
		fi
	else
		fail "port skills sync"
	fi

	# 5 — grouped demo skill synced (e2e config selects all groups)
	if find "${E2E_CURSOR}/skills/port" -path '*/demo-security-review/SKILL.md' -print -quit 2>/dev/null | grep -q .; then
		pass "demo-security-review synced with select_all_groups"
	else
		fail "demo-security-review missing after sync (check demo seed and selection)"
	fi

	# 6 — single create
	SINGLE_ID="e2e-single-${RUN_ID}"
	if run_port_skills create "${FIXTURES}/single-skill" --identifier "${SINGLE_ID}" --publish >/dev/null 2>&1; then
		pass "create single skill ${SINGLE_ID}"
	else
		fail "create single skill ${SINGLE_ID}"
	fi

	# 7 — batch create
	BATCH_ROOT="${E2E_CONFIG_DIR}/batch-${RUN_ID}"
	mkdir -p "${BATCH_ROOT}/skill-a-${RUN_ID}" "${BATCH_ROOT}/skill-b-${RUN_ID}"
	cp "${FIXTURES}/batch-two-skills/skill-a/SKILL.md" "${BATCH_ROOT}/skill-a-${RUN_ID}/"
	cp "${FIXTURES}/batch-two-skills/skill-b/SKILL.md" "${BATCH_ROOT}/skill-b-${RUN_ID}/"
	if run_port_skills create "${BATCH_ROOT}" --publish >/dev/null 2>&1; then
		pass "batch create two skills"
	else
		fail "batch create two skills"
	fi

	# 8 — duplicate create → non-zero
	if run_port_skills create "${FIXTURES}/single-skill" --identifier "${SINGLE_ID}" >/dev/null 2>&1; then
		fail "duplicate create should fail"
	else
		pass "duplicate create exits non-zero"
	fi

	# 9 — edit with publish
	if run_port_skills edit "${SINGLE_ID}" "${FIXTURES}/single-skill" --publish >/dev/null 2>&1; then
		pass "edit skill ${SINGLE_ID}"
	else
		fail "edit skill ${SINGLE_ID}"
	fi

	# 10 — archive removed (fang prints parent help for unknown subcommands; use --tree)
	if run_port skills --tree 2>&1 | grep -qE '(^|[[:space:]])archive([[:space:]]|$)'; then
		fail "skills command tree still lists archive"
	else
		pass "archive subcommand removed"
	fi

	echo ""
	echo "Summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped"
	if [[ "${FAIL}" -gt 0 ]]; then
		exit 1
	fi
}

main "$@"
