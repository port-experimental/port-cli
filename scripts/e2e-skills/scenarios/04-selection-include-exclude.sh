# Group selection, include_groups / exclude_groups, and on-disk reconciliation.

scenario_selection_include_exclude() {
	echo "== selection, include/exclude =="

	scenario_selection_explicit_groups_only
	scenario_selection_team_include_exclude
}

scenario_selection_explicit_groups_only() {
	echo "-- explicit --group selection --"

	e2e_reset_synced_skills
	write_e2e_skills_config

	if ! (
		cd "${E2E_WORKDIR}" && port_skills init --tool Cursor \
			--group "${DEMO_GROUP_REQUIRED}" \
			--select-all-ungrouped \
			--ignore-git-dirty
	) >/dev/null 2>&1; then
		fail "init with required group + ungrouped"
		return
	fi
	pass "init with required group + ungrouped"

	assert_skill_on_disk "required: onboarding" "${DEMO_SKILL_ONBOARDING}"
	assert_skill_on_disk "required: api-guide" "${DEMO_SKILL_API_GUIDE}"
	assert_skill_on_disk "ungrouped: standalone" "${DEMO_SKILL_STANDALONE}"
	assert_skill_md_has "standalone body" "${DEMO_SKILL_STANDALONE}" "${MARKER_STANDALONE_ACTIVE}"

	assert_skill_not_on_disk "optional group excluded: troubleshoot" "${DEMO_SKILL_TROUBLESHOOT}"
	assert_skill_not_on_disk "optional group excluded: workflows" "${DEMO_SKILL_WORKFLOWS}"
	assert_skill_not_on_disk "security group excluded: security-review" "${DEMO_SKILL_SECURITY}"

	assert_active_version_markers
}

scenario_selection_team_include_exclude() {
	echo "-- team_group_defaults + include/exclude --"

	e2e_reset_synced_skills
	write_e2e_skills_selection "${DEMO_GROUP_REQUIRED}" "" false true true

	if ! port_skills sync --ignore-git-dirty >/dev/null 2>&1; then
		fail "sync with include_groups=${DEMO_GROUP_REQUIRED}"
		return
	fi
	pass "sync with include_groups=${DEMO_GROUP_REQUIRED}"

	assert_skill_on_disk "include: onboarding" "${DEMO_SKILL_ONBOARDING}"
	assert_skill_on_disk "include: api-guide" "${DEMO_SKILL_API_GUIDE}"
	assert_skill_not_on_disk "include excludes optional troubleshoot" "${DEMO_SKILL_TROUBLESHOOT}"

	# Widen selection via include_groups
	write_e2e_skills_selection "${DEMO_GROUP_REQUIRED},${DEMO_GROUP_OPTIONAL}" "" false true true

	if ! port_skills sync --ignore-git-dirty >/dev/null 2>&1; then
		fail "sync after adding optional group to include_groups"
		return
	fi
	pass "sync after adding optional group to include_groups"

	assert_skill_on_disk "include optional: troubleshoot" "${DEMO_SKILL_TROUBLESHOOT}"
	assert_skill_md_lacks "troubleshoot still on active version" "${DEMO_SKILL_TROUBLESHOOT}" "${MARKER_TROUBLESHOOT_V100}"

	# Exclude optional group again
	write_e2e_skills_selection "${DEMO_GROUP_REQUIRED},${DEMO_GROUP_OPTIONAL}" "${DEMO_GROUP_OPTIONAL}" false true true

	if ! port_skills sync --ignore-git-dirty >/dev/null 2>&1; then
		fail "sync with exclude_groups=${DEMO_GROUP_OPTIONAL}"
		return
	fi
	pass "sync with exclude_groups=${DEMO_GROUP_OPTIONAL}"

	assert_skill_not_on_disk "exclude prunes troubleshoot from disk" "${DEMO_SKILL_TROUBLESHOOT}"
	assert_skill_on_disk "exclude keeps required onboarding" "${DEMO_SKILL_ONBOARDING}"
}
