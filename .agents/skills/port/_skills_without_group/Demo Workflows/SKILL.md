---
name: demo-workflows
description: Use when the user asks about Port workflows, automation steps, conditions, or self-service actions triggered from catalog entities.
---

# Demo Workflows

Guidance for workflow design in Port (demo content).

## Workflow

1. Identify trigger: entity created/updated, timer, or webhook.
2. Map inputs from entity properties and relations.
3. Define steps and failure handling — see `references/workflow-patterns.md`.
4. Use `references/conditions.md` for branch logic.
5. Start from `assets/workflow-starter.json` when scaffolding.

## Location

This skill is seeded with `location: project` (project-scoped install path in Port CLI semantics).

## Scripts

See `scripts/README.md` for how bundled scripts relate to workflow runners (documentation only in demo).
