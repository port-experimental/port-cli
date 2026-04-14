# Session Context

## User Prompts

### Prompt 1

review this PR https://github.com/port-experimental/port-cli/pull/51/changes

### Prompt 2

Base directory for this skill: /Users/eric/.claude/skills/review

<!-- AUTO-GENERATED from SKILL.md.tmpl — do not edit directly -->
<!-- Regenerate: bun run gen:skill-docs -->

## Preamble (run first)

```bash
_UPD=$(~/.claude/skills/gstack/bin/gstack-update-check 2>/dev/null || .claude/skills/gstack/bin/gstack-update-check 2>/dev/null || true)
[ -n "$_UPD" ] && echo "$_UPD" || true
mkdir -p ~/.gstack/sessions
touch ~/.gstack/sessions/"$PPID"
_SESSIONS=$(find ~/.gstack/sessions -mmin -120 -typ...

### Prompt 3

git pull and create a changelog for 0.2.24

### Prompt 4

git pull and create a changelog for 0.2.4

### Prompt 5

Push to main

