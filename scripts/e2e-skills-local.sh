#!/usr/bin/env bash
# Local skills E2E smoke tests (NOT run in CI).
# Prerequisites: docs/skills-e2e-local.md
#
# Uses ${REPO_ROOT}/bin/port (make build). Auth and org credentials come from
# your normal ~/.port setup; only skills.targets are isolated under $TMPDIR.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck source=scripts/e2e-skills/lib/common.sh
source "${SCRIPT_DIR}/e2e-skills/lib/common.sh"

e2e_source_libs

main() {
	build_port_cli

	if [[ ! -x "${PORT_BIN}" ]]; then
		red "Binary not found at ${PORT_BIN} — run: make build"
		exit 1
	fi

	e2e_prepare_dirs
	e2e_setup_config
	e2e_print_header

	# shellcheck source=scripts/e2e-skills/scenarios/01-preflight.sh
	source "${SCRIPT_DIR}/e2e-skills/scenarios/01-preflight.sh"
	# shellcheck source=scripts/e2e-skills/scenarios/02-list-search.sh
	source "${SCRIPT_DIR}/e2e-skills/scenarios/02-list-search.sh"
	# shellcheck source=scripts/e2e-skills/scenarios/03-sync-versions.sh
	source "${SCRIPT_DIR}/e2e-skills/scenarios/03-sync-versions.sh"
	# shellcheck source=scripts/e2e-skills/scenarios/04-selection-include-exclude.sh
	source "${SCRIPT_DIR}/e2e-skills/scenarios/04-selection-include-exclude.sh"
	# shellcheck source=scripts/e2e-skills/scenarios/05-crud.sh
	source "${SCRIPT_DIR}/e2e-skills/scenarios/05-crud.sh"
	# shellcheck source=scripts/e2e-skills/scenarios/06-archive-removed.sh
	source "${SCRIPT_DIR}/e2e-skills/scenarios/06-archive-removed.sh"

	scenario_preflight
	scenario_list_search
	scenario_sync_versions
	scenario_selection_include_exclude
	scenario_crud
	scenario_archive_removed

	e2e_print_summary
}

main "$@"
