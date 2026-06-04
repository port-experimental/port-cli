# Sync all demo groups and assert only active semver content is written locally.

scenario_sync_versions() {
	echo "== sync active versions =="

	e2e_reset_synced_skills

	if ! (
		cd "${E2E_WORKDIR}" && port_skills init --tool Cursor --select-all-groups --select-all-ungrouped --ignore-git-dirty
	) >/dev/null 2>&1; then
		fail "init sync all groups for version check"
		return
	fi
	pass "init sync all groups for version check"

	assert_skill_on_disk "demo-onboarding on disk" "${DEMO_SKILL_ONBOARDING}"
	assert_skill_on_disk "demo-api-guide on disk" "${DEMO_SKILL_API_GUIDE}"
	assert_active_version_markers
}
