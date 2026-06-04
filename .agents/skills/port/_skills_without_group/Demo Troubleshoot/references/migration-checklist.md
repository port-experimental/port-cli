# Migration checklist

- [ ] `yarn seed:ai` (or org has `AI_ELIGIBLE`)
- [ ] `yarn seed:demo-skills` OR curl create-skill-blueprints migration
- [ ] CI pooled orgs: `migrate_test_orgs-*` entry for skill blueprints in `infra/migration-scripts/package.json`
- [ ] Verify: `GET /v1/skills/published-files` returns rows
