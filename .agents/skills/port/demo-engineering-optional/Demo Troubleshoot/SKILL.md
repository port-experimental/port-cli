---
name: demo-troubleshoot
description: Use when debugging local MCP, skill blueprints, migrations, or published skill file resolution.
---

# Demo Troubleshoot

Runbook for skill system + MCP on localhost.

## Triage order

1. Blueprints exist: `_skill`, `_skill_version`, `_skill_file`, `_skill_group`.
2. `_skill.relations.skill_active_version` points at the version to serve (see v1.0.0 for legacy `release_state` model).
3. `SKILL.md` exists on that version with non-empty body after frontmatter.
4. MCP transport headers (see `references/mcp-streaming.md`).
5. Migration status (`references/migration-checklist.md`).

## Skill blueprint model

See `references/skill-blueprints.md` for entity graph and relation names.

## Escalation

If legacy `skill` blueprint still has entities, catalog merges legacy + new model (new wins on name collision).
