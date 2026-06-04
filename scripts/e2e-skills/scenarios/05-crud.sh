# Upload, batch upload, upsert, and folder/name validation.

scenario_crud() {
	echo "== upload / upsert =="

	local single_id="e2e-single-${E2E_RUN_ID}"
	local single_dir="${E2E_CONFIG_DIR}/${single_id}"
	local batch_root="${E2E_CONFIG_DIR}/batch-${E2E_RUN_ID}"

	mkdir -p "${single_dir}"
	sed "s/name: e2e-single-skill/name: ${single_id}/" "${FIXTURES}/single-skill/SKILL.md" >"${single_dir}/SKILL.md"

	if port_skills upload "${single_dir}" --publish >/dev/null 2>&1; then
		pass "upload single skill ${single_id}"
	else
		fail "upload single skill ${single_id}"
	fi

	local batch_a="e2e-skill-a-${E2E_RUN_ID}"
	local batch_b="e2e-skill-b-${E2E_RUN_ID}"
	mkdir -p "${batch_root}/${batch_a}" "${batch_root}/${batch_b}"
	sed "s/name: e2e-skill-a/name: ${batch_a}/" "${FIXTURES}/batch-two-skills/skill-a/SKILL.md" >"${batch_root}/${batch_a}/SKILL.md"
	sed "s/name: e2e-skill-b/name: ${batch_b}/" "${FIXTURES}/batch-two-skills/skill-b/SKILL.md" >"${batch_root}/${batch_b}/SKILL.md"

	if port_skills upload "${batch_root}" --publish >/dev/null 2>"${E2E_CONFIG_DIR}/batch-upload.err"; then
		pass "batch upload two skills"
	else
		local err
		err="$(tail -n 1 "${E2E_CONFIG_DIR}/batch-upload.err" 2>/dev/null | tr -d '\r' || true)"
		fail "batch upload two skills${err:+ — ${err}}"
	fi

	if port_skills upload "${single_dir}" --publish >/dev/null 2>&1; then
		pass "re-upload upserts new version for ${single_id}"
	else
		fail "re-upload upsert ${single_id}"
	fi

	if port_skills upload "${FIXTURES}/name-mismatch" >/dev/null 2>&1; then
		fail "name/folder mismatch upload should fail"
	else
		pass "name/folder mismatch rejected"
	fi

	if port_skills unpublish "${single_id}" >/dev/null 2>&1; then
		pass "unpublish ${single_id}"
	else
		fail "unpublish ${single_id}"
	fi
}
