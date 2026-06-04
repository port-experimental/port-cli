# Demo catalog list and search against live ai-service.

scenario_list_search() {
	echo "== list & search =="

	if port_skills list --json 2>/dev/null | grep -q "${DEMO_SKILL_ONBOARDING}"; then
		pass "list contains ${DEMO_SKILL_ONBOARDING}"
	else
		fail "list missing ${DEMO_SKILL_ONBOARDING} (run yarn seed:demo-skills in Port repo?)"
	fi

	if port_skills list --json 2>/dev/null | grep -q '2.0.0'; then
		pass "list JSON includes active demo-onboarding version 2.0.0"
	else
		fail "list JSON missing version 2.0.0 (seed active version?)"
	fi

	if port_skills search demo --json 2>/dev/null | grep -q 'demo'; then
		pass "search demo returns matches"
	else
		fail "search demo returned no matches"
	fi
}
