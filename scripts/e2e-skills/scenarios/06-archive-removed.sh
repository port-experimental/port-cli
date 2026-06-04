# Regression: archive subcommand removed from skills tree.

scenario_archive_removed() {
	echo "== archive removed =="

	if port_cli skills --tree 2>&1 | grep -qE '(^|[[:space:]])archive([[:space:]]|$)'; then
		fail "skills command tree still lists archive"
	else
		pass "archive subcommand removed"
	fi
}
