# API health and authenticated skills list.

scenario_preflight() {
	echo "== preflight =="

	if curl_health "${PORT_API_URL%/v1}"; then
		pass "preflight: port-api reachable"
	else
		fail "preflight: port-api not reachable at ${PORT_API_URL}"
	fi

	if curl_health "${PORT_AI_SERVICE_URL%/v1}"; then
		pass "preflight: ai-service reachable"
	else
		fail "preflight: ai-service not reachable at ${PORT_AI_SERVICE_URL}"
	fi

	if port_skills list --json >/dev/null 2>"${E2E_CONFIG_DIR}/skills-list.err"; then
		pass "preflight: port skills list"
	else
		local err
		err="$(tail -n 1 "${E2E_CONFIG_DIR}/skills-list.err" 2>/dev/null | tr -d '\r' || true)"
		fail "preflight: port skills list — ${err:-run port auth login and ensure demo catalog is seeded}"
	fi
}
