# Session Context

## User Prompts

### Prompt 1

update the changelog with the changes from the last few releases (catching up with the changelog)

### Prompt 2

Why does it say unreleased?, why do we not have 1.18?

### Prompt 3

I just released 0.2.3

### Prompt 4

no it's fine

### Prompt 5

can you create a PR for this changelog

### Prompt 6

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

### Prompt 7

why are we missing 0.2.1 and 0.2.2, please add them

