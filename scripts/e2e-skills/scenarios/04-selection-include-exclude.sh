# Selection via selected_groups (explicit, deterministic — no team defaults).

scenario_selection_include_exclude() {
	echo "== selection (selected_groups) =="

	scenario_selection_required_and_ungrouped
	scenario_selection_widen_then_narrow
}

scenario_selection_required_and_ungrouped() {
	echo "-- required group + ungrouped --"

	e2e_begin_scenario
	e2e_apply_skills_selection "${DEMO_GROUP_REQUIRED}" "" "" true

	if ! e2e_sync_skills >/dev/null 2>&1; then
		fail "sync required group + ungrouped"
		return
	fi
	pass "sync required group + ungrouped"

	assert_skills_present "expected" \
		"${DEMO_SKILL_ONBOARDING}" \
		"${DEMO_SKILL_API_GUIDE}" \
		"${DEMO_SKILL_STANDALONE}"
	assert_skills_absent "excluded" \
		"${DEMO_SKILL_TROUBLESHOOT}" \
		"${DEMO_SKILL_WORKFLOWS}" \
		"${DEMO_SKILL_SECURITY}"
	assert_only_demo_skills "catalog" \
		"${DEMO_SKILL_ONBOARDING}" \
		"${DEMO_SKILL_API_GUIDE}" \
		"${DEMO_SKILL_STANDALONE}"

	assert_skill_md_has "standalone body" "${DEMO_SKILL_STANDALONE}" "${MARKER_STANDALONE_ACTIVE}"
	assert_active_version_markers
}

scenario_selection_widen_then_narrow() {
	echo "-- widen to optional group, then narrow back --"

	e2e_begin_scenario
	e2e_apply_skills_selection "${DEMO_GROUP_REQUIRED},${DEMO_GROUP_OPTIONAL}" "" "" true

	if ! e2e_sync_skills >/dev/null 2>&1; then
		fail "sync required + optional groups"
		return
	fi
	pass "sync required + optional groups"

	assert_skills_present "expected with optional" \
		"${DEMO_SKILL_ONBOARDING}" \
		"${DEMO_SKILL_TROUBLESHOOT}" \
		"${DEMO_SKILL_WORKFLOWS}"
	assert_skills_absent "still excluded" "${DEMO_SKILL_SECURITY}"

	assert_skill_md_has "troubleshoot active version" "${DEMO_SKILL_TROUBLESHOOT}" "${MARKER_TROUBLESHOOT_ACTIVE}"

	# Narrow selection (simulates dropping a group / exclude effect) and re-sync.
	e2e_apply_skills_selection "${DEMO_GROUP_REQUIRED}" "" "" true

	if ! e2e_sync_skills >/dev/null 2>&1; then
		fail "sync after narrowing to required only"
		return
	fi
	pass "sync after narrowing to required only"

	assert_skills_present "kept" \
		"${DEMO_SKILL_ONBOARDING}" \
		"${DEMO_SKILL_API_GUIDE}"
	assert_skills_absent "pruned" \
		"${DEMO_SKILL_TROUBLESHOOT}" \
		"${DEMO_SKILL_WORKFLOWS}"
	assert_only_demo_skills "catalog after prune" \
		"${DEMO_SKILL_ONBOARDING}" \
		"${DEMO_SKILL_API_GUIDE}" \
		"${DEMO_SKILL_STANDALONE}"
}
