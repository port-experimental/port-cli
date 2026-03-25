# `port plugin` — AI Skills Sync for Developer Workstations

## What it does

Developers using AI coding tools (Cursor, Claude Code, Windsurf, etc.) can load **skills from the Port registry** into their local environment automatically. Skills are fetched once at the start of every AI session via **hooks** — a native mechanism in these tools that runs a command on session start.

---

## Commands

### `port plugin init`

**One-time setup.** Installs the session-start hook into the AI tool config directories.

**Flow:**
1. Prompts: **"Where should the hooks be installed?"**
   - **Global** (`~/.cursor/`, `~/.claude/`, `~/.agents/`) — applies to all projects
   - **Local** (`.cursor/`, `.claude/`, `.agents/` in the current repo) — project-scoped
2. Writes/merges a `hooks.json` into each target directory with a `sessionStart` hook that calls `port plugin load-skills`
3. If no skill selection has been configured yet, immediately triggers the skill selection prompt (same as `load-skills` first run)

**Result:** Every new AI session in Cursor / Claude Code / Agents will auto-fetch the user's selected skills from Port.

---

### `port plugin load-skills`

**Fetches skills from Port and writes them to disk.** Can be run manually or automatically via the hook.

**Flow:**
1. Authenticates with Port (reuses existing `port auth login` credentials from `~/.port/creds.json`)
2. Calls Port API: `GET /v1/blueprints/skill_group/entities` and `GET /v1/blueprints/skill/entities`
3. **First run (or `--select`):** Presents an interactive multi-select of available skill groups and individual skills. Saves selection to `~/.port/config.yaml` under a new `plugin` section
4. **Subsequent runs:** Uses the saved selection from config
5. Writes each selected skill as `{target}/skills/{skill-group-name}/{skill-identifier}/SKILL.md` (plus any reference/asset files) into each configured target directory
6. For Cursor specifically, also outputs `additional_context` JSON to stdout so skills appear in the session system prompt

**Config (persisted in `~/.port/config.yaml`):**
```yaml
plugin:
  scope: global            # or "local"
  targets:                 # extensible list of AI tool directories
    - ~/.cursor
    - ~/.claude
    - ~/.agents
  selected_groups:
    - "engineering-skills"
    - "devops-skills"
  selected_skills:
    - "create-adr"
    - "pr-review"
```

---

### `port plugin status`

*(Lightweight addition)* Shows the current plugin config: which targets are hooked, which skills/groups are selected, when skills were last synced.

---

## Hook mechanism (per tool)

| Tool | Hook file | Trigger | What runs |
|------|-----------|---------|-----------|
| **Cursor** | `hooks.json` | `sessionStart` | `port plugin load-skills` |
| **Claude Code** | `settings.json` → `hooks` | `UserPromptSubmit` | `port plugin load-skills` |
| **Agents (generic)** | `hooks.json` | `sessionStart` | `port plugin load-skills` |

---

## Key design decisions

- **Extensible target list.** Adding a new AI tool is just adding a path + hook format to a registry — no code restructure needed.
- **Leverages existing port-cli auth.** No separate auth flow — uses the same `port auth login` SSO/OAuth flow the CLI already has.
- **Config lives in `~/.port/config.yaml`.** Follows the existing port-cli pattern. Skill selection is just a new `plugin` section in the same file.
- **Interactive selection with Charm Huh.** Uses the same TUI library the port-cli already uses for prompts.
- **Idempotent.** Running `init` or `load-skills` multiple times is safe — hooks are merged, skills are overwritten.

---

## User journey (end to end)

```
$ port auth login              # Already exists — authenticates with Port
$ port plugin init             # Installs hooks, prompts for global vs local
  → Where to install? (global / local): global
  → Select skill groups: [x] engineering  [x] devops  [ ] design
  → Select individual skills: [x] pr-review  [ ] create-adr
  ✓ Hooks installed in ~/.cursor, ~/.claude, ~/.agents
  ✓ Skills synced (12 skills from 2 groups)

# From now on, every new Cursor/Claude session auto-runs:
#   port plugin load-skills
# which refreshes skills silently in the background.

$ port plugin load-skills --select   # Re-pick skills anytime
$ port plugin status                  # Check what's configured
```

---

## What ships

| Artifact | Description |
|----------|-------------|
| `internal/commands/plugin.go` | `port plugin` command group with `init`, `load-skills`, `status` subcommands |
| `internal/modules/plugin/` | Business logic: hook installation, skill fetching, skill writing, config management |
| `plugin` section in `~/.port/config.yaml` | Persisted skill selection and target config |
| `docs/plugin-setup.md` | Tutorial: how to set up AI skills hooks on your machine |
