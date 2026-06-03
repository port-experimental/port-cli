#!/usr/bin/env bash
# Local skills E2E smoke tests (NOT run in CI).
# Prerequisites: see docs/skills-e2e-local.md
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

run_port() {
	"${PORT_BIN}" --config-file "${CONFIG_FILE}" "$@"
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

main() {
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

	mkdir -p "${E2E_CURSOR}/skills" "${E2E_WORKDIR}"
	export CURSOR_CONFIG_DIR="${E2E_CURSOR}"

	cat >"${CONFIG_FILE}" <<EOF
skills:
  targets:
    - ${E2E_CURSOR}
  select_all_groups: true
  select_all_ungrouped: true
EOF

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

	if run_port skills list --json >/dev/null 2>&1; then
		pass "preflight: port skills list (auth)"
	else
		fail "preflight: port skills list — set PORT_CLIENT_ID/PORT_CLIENT_SECRET or login"
	fi

	# 2 — list contains demo ids
	if run_port skills list --json 2>/dev/null | grep -q 'demo-onboarding'; then
		pass "list contains demo-onboarding"
	else
		fail "list missing demo-onboarding (run yarn seed:demo-skills in Port repo?)"
	fi

	# 3 — search
	if run_port skills search demo --json 2>/dev/null | grep -q 'demo'; then
		pass "search demo returns matches"
	else
		fail "search demo returned no matches"
	fi

	# 4 — sync
	if run_port skills sync --ignore-git-dirty >/dev/null 2>&1; then
		if [[ -f "${E2E_CURSOR}/skills/port/demo-engineering-required/demo-onboarding/SKILL.md" ]] || \
			find "${E2E_CURSOR}/skills/port" -path '*/demo-onboarding/SKILL.md' -print -quit 2>/dev/null | grep -q .; then
			pass "sync wrote demo skill files"
		else
			fail "sync succeeded but demo-onboarding SKILL.md not found under ${E2E_CURSOR}/skills/port"
		fi
	else
		fail "port skills sync"
	fi

	# 5 — required vs optional (best-effort: security skill absent until selected)
	if find "${E2E_CURSOR}/skills/port" -path '*/demo-security-review/SKILL.md' -print -quit 2>/dev/null | grep -q .; then
		skip "optional skill demo-security-review already present (selection may include it)"
	else
		pass "demo-security-review not synced before select"
	fi

	if run_port skills select --skill demo-security-review --ignore-git-dirty >/dev/null 2>&1; then
		if find "${E2E_CURSOR}/skills/port" -path '*/demo-security-review/SKILL.md' -print -quit 2>/dev/null | grep -q .; then
			pass "demo-security-review present after select"
		else
			fail "demo-security-review missing after select"
		fi
	else
		skip "port skills select demo-security-review (needs prior init selection)"
	fi

	# 6 — single create
	SINGLE_ID="e2e-single-${RUN_ID}"
	if run_port skills create "${FIXTURES}/single-skill" --identifier "${SINGLE_ID}" --publish >/dev/null 2>&1; then
		pass "create single skill ${SINGLE_ID}"
	else
		fail "create single skill ${SINGLE_ID}"
	fi

	# 7 — batch create
	BATCH_ROOT="${E2E_CONFIG_DIR}/batch-${RUN_ID}"
	mkdir -p "${BATCH_ROOT}/skill-a-${RUN_ID}" "${BATCH_ROOT}/skill-b-${RUN_ID}"
	cp "${FIXTURES}/batch-two-skills/skill-a/SKILL.md" "${BATCH_ROOT}/skill-a-${RUN_ID}/"
	cp "${FIXTURES}/batch-two-skills/skill-b/SKILL.md" "${BATCH_ROOT}/skill-b-${RUN_ID}/"
	if run_port skills create "${BATCH_ROOT}" --publish >/dev/null 2>&1; then
		pass "batch create two skills"
	else
		fail "batch create two skills"
	fi

	# 8 — duplicate create → non-zero
	if run_port skills create "${FIXTURES}/single-skill" --identifier "${SINGLE_ID}" >/dev/null 2>&1; then
		fail "duplicate create should fail"
	else
		pass "duplicate create exits non-zero"
	fi

	# 9 — edit with publish
	if run_port skills edit "${SINGLE_ID}" "${FIXTURES}/single-skill" --publish >/dev/null 2>&1; then
		pass "edit skill ${SINGLE_ID}"
	else
		fail "edit skill ${SINGLE_ID}"
	fi

	# 10 — archive removed
	if run_port skills archive "${SINGLE_ID}" >/dev/null 2>&1; then
		fail "archive subcommand should be removed"
	else
		pass "archive subcommand missing or fails"
	fi

	# 11 — help has no archive
	if run_port skills --help 2>&1 | grep -q 'archive'; then
		fail "skills help still lists archive"
	else
		pass "skills help has no archive"
	fi

	echo ""
	echo "Summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped"
	if [[ "${FAIL}" -gt 0 ]]; then
		exit 1
	fi
}

main "$@"
