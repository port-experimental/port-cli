---
name: security-pr-review
description: Use when reviewing pull requests that touch integrations, secrets, or customer skill content before merge.
---

# Security PR review

Project-scoped checklist for secure changes in Port-backed repositories.

## Scope

- New integrations and webhook endpoints
- Changes under `skills/` or `.cursor/skills/port/`
- Credentials in CI or local env files

## Steps

1. Walk `references/pr-review.md` for PR-level checks.
2. Map findings to `references/owasp-checklist.md`.
3. Validate secrets handling per `references/secrets-handling.md`.
4. Record issues in `assets/finding-template.md`.
5. Score severity using `assets/severity-rubric.json`.

## Output

Table of findings: severity, file, recommendation, owner.
