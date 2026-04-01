# `port skills` — AI Skills Sync for Developer Workstations

## What it does

Developers using AI coding tools (Cursor, Claude Code, Windsurf, etc.) can load **skills from the Port registry** into their local environment automatically. Skills are fetched once at the start of every AI session via **hooks** — a native mechanism in these tools that runs a command on session start.

---

## Commands

### `port skills init`

**One-time setup.** Installs the session-start hook into the AI tool config directories.

**Flow:**
1. Prompts: **"Where should the hooks be installed?"**
   - **Global** (`~/.cursor/`, `~/.claude/`, etc.) — applies to all projects
   - **Local** (`.cursor/`, `.claude/`, etc. in the current repo) — project-scoped
2. Writes/merges a `hooks.json` (or tool-specific hook config) into each target directory with a hook that calls `port skills sync`
3. If no skill selection has been configured yet, immediately triggers the skill selection prompt (same as `sync` first run)

**Result:** Every new AI session in Cursor / Claude Code / supported tools will auto-fetch the user's selected skills from Port.

---

### `port skills sync`

**Fetches skills from Port and writes them to disk.** Can be run manually or automatically via the hook.

**Flow:**
1. Authenticates with Port (reuses existing `port auth login` credentials from `~/.port/creds.json`)
2. Calls Port API for `skill_group` and `skill` entities
3. **First run (or `--select`):** Presents an interactive multi-select of available skill groups and individual skills. Saves selection to `~/.port/config.yaml` under a `skills` section (legacy `plugin` key is still read for backward compatibility)
4. **Subsequent runs:** Uses the saved selection from config
5. Writes each selected skill under the appropriate AI tool directories
6. For Cursor specifically, may output `additional_context` JSON so skills appear in the session system prompt

**Config (persisted in `~/.port/config.yaml`):**
```yaml
skills:
  targets:
    - ~/.cursor
    - ~/.claude
  selected_groups:
    - "engineering-skills"
    - "devops-skills"
  selected_skills:
    - "create-adr"
    - "pr-review"
```

---

### `port skills status`

*(Lightweight addition)* Shows the current skills config: which targets are hooked, which skills/groups are selected, when skills were last synced.

---

## Hook mechanism (per tool)

| Tool | Hook file | Trigger | What runs |
|------|-----------|---------|-----------|
| **Cursor** | `hooks.json` | `sessionStart` | `port skills sync` |
| **Claude Code** | `settings.json` → `hooks` | `UserPromptSubmit` | `port skills sync` |
| **Other supported tools** | varies | varies | `port skills sync` |

---

## Key design decisions

- **Extensible target list.** Adding a new AI tool is just adding a path + hook format to a registry — no code restructure needed.
- **Leverages existing port-cli auth.** No separate auth flow — uses the same `port auth login` SSO/OAuth flow the CLI already has.
- **Config lives in `~/.port/config.yaml`.** Follows the existing port-cli pattern. Skill selection lives in a `skills` section in the same file.
- **Interactive selection with Charm Huh.** Uses the same TUI library the port-cli already uses for prompts.
- **Idempotent.** Running `init` or `sync` multiple times is safe — hooks are merged, skills are overwritten.

---

## User journey (end to end)

```
$ port auth login              # Already exists — authenticates with Port
$ port skills init             # Installs hooks, prompts for targets
  → Select targets: Cursor, Claude Code, …
  → Select skill groups: [x] engineering  [x] devops  [ ] design
  ✓ Hooks installed
  ✓ Skills synced

# From now on, every new session auto-runs:
#   port skills sync

$ port skills sync --select   # Re-pick skills anytime
$ port skills status          # Check what's configured
```

---

## What ships

| Artifact | Description |
|----------|-------------|
| `internal/commands/skills.go` | `port skills` command group (`init`, `sync`, `list`, `clear`, `remove`, `status`) |
| `internal/modules/skills/` | Business logic: hook installation, skill fetching, skill writing, config management |
| `skills` section in `~/.port/config.yaml` | Persisted skill selection and target config |
| `docs/skills-setup.md` | Tutorial: how to set up AI skills hooks on your machine |
