# Sync full demo catalog and assert only active semver content is written locally.

scenario_sync_versions() {
	echo "== sync active versions =="

	e2e_begin_scenario
	e2e_apply_skills_selection "${DEMO_ALL_GROUPS}" "" "" true

	if ! e2e_sync_skills >/dev/null 2>&1; then
		fail "sync all demo groups for version check"
		return
	fi
	pass "sync all demo groups for version check"

	assert_skills_present "synced" \
		"${DEMO_SKILL_ONBOARDING}" \
		"${DEMO_SKILL_API_GUIDE}" \
		"${DEMO_SKILL_STANDALONE}" \
		"${DEMO_SKILL_TROUBLESHOOT}" \
		"${DEMO_SKILL_WORKFLOWS}" \
		"${DEMO_SKILL_SECURITY}"

	assert_only_demo_skills "catalog" \
		"${DEMO_SKILL_ONBOARDING}" \
		"${DEMO_SKILL_API_GUIDE}" \
		"${DEMO_SKILL_STANDALONE}" \
		"${DEMO_SKILL_TROUBLESHOOT}" \
		"${DEMO_SKILL_WORKFLOWS}" \
		"${DEMO_SKILL_SECURITY}"

	assert_active_version_markers
}
