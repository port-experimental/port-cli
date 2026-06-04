# Create, batch create, duplicate guard, and edit (uses ephemeral skill ids).

scenario_crud() {
	echo "== create / edit =="

	local single_id="e2e-single-${E2E_RUN_ID}"
	local batch_root="${E2E_CONFIG_DIR}/batch-${E2E_RUN_ID}"

	if port_skills create "${FIXTURES}/single-skill" --identifier "${single_id}" --publish >/dev/null 2>&1; then
		pass "create single skill ${single_id}"
	else
		fail "create single skill ${single_id}"
	fi

	local batch_a="e2e-skill-a-${E2E_RUN_ID}"
	local batch_b="e2e-skill-b-${E2E_RUN_ID}"
	mkdir -p "${batch_root}/skill-a-${E2E_RUN_ID}" "${batch_root}/skill-b-${E2E_RUN_ID}"
	sed "s/name: e2e-skill-a/name: ${batch_a}/" "${FIXTURES}/batch-two-skills/skill-a/SKILL.md" >"${batch_root}/skill-a-${E2E_RUN_ID}/SKILL.md"
	sed "s/name: e2e-skill-b/name: ${batch_b}/" "${FIXTURES}/batch-two-skills/skill-b/SKILL.md" >"${batch_root}/skill-b-${E2E_RUN_ID}/SKILL.md"

	if port_skills create "${batch_root}" --publish >/dev/null 2>"${E2E_CONFIG_DIR}/batch-create.err"; then
		pass "batch create two skills"
	else
		local err
		err="$(tail -n 1 "${E2E_CONFIG_DIR}/batch-create.err" 2>/dev/null | tr -d '\r' || true)"
		fail "batch create two skills${err:+ — ${err}}"
	fi

	if port_skills create "${FIXTURES}/single-skill" --identifier "${single_id}" >/dev/null 2>&1; then
		fail "duplicate create should fail"
	else
		pass "duplicate create exits non-zero"
	fi

	if port_skills edit "${single_id}" "${FIXTURES}/single-skill" --publish >/dev/null 2>&1; then
		pass "edit skill ${single_id}"
	else
		fail "edit skill ${single_id}"
	fi
}
