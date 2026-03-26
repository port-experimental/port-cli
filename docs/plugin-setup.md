# Setting up AI skill hooks with Port CLI

The `port plugin` commands let you automatically load skills from your Port
organization into your local AI coding tools (Cursor, Claude Code, Agents)
at the start of every session.

## Prerequisites

- `port` CLI installed (`npm install -g @port-experimental/port-cli` or download from [GitHub Releases](https://github.com/port-experimental/port-cli/releases))
- A Port account with skills configured in the `skill` and `skill_group` blueprints
- At least one supported AI tool installed (Cursor, Claude Code, or Agents)

---

## Step 1 — Authenticate

If you haven't already, log in to Port:

```sh
port auth login
```

This opens a browser window for SSO and stores a token in `~/.port/creds.json`.

---

## Step 2 — Install hooks

Run the one-time setup command:

```sh
port plugin init
```

You will be asked two questions:

1. **Where to install the hooks** — choose between:
   - **Global** (`~/.cursor`, `~/.claude`, `~/.agents`) — applies to every project on your machine
   - **Local** (`.cursor`, `.claude`, `.agents` in your current directory) — applies only to this project

2. **Which skills to sync** — an interactive multi-select shows all available skill groups and individual skills from your Port organization.
   - Skills marked `required = true` in Port are always synced regardless of your selection. They appear as a note before the prompt.
   - Select any combination of groups and individual skills you want.

After confirming your selection, the CLI:
- Writes (or merges) a `hooks.json` / `settings.json` into each AI tool directory
- Immediately syncs the selected skills to the correct locations (see below)
- Saves your selection to `~/.port/config.yaml` so future syncs are automatic

---

## Step 3 — Start a new AI session

Open a new Cursor window, start a Claude Code session, or launch your Agents runtime. The hook runs `port plugin sync` automatically in the background, refreshing your local skills from Port before the AI assistant starts.

---

## Updating your skill selection

To change which skills are synced, re-run init:

```sh
port plugin init
```

This re-presents the full setup prompt. Your new selection is saved and the skills are immediately re-synced.

---

## Manual sync

To sync skills without changing your selection:

```sh
port plugin sync
```

---

## Command reference

| Command | Description |
|---------|-------------|
| `port plugin init` | Install hooks + configure skill selection (one-time setup, re-run to change selection) |
| `port plugin sync` | Sync skills using saved selection, removing any stale local skills |
| `port plugin clear` | Delete all locally synced Port skills (with confirmation) |
| `port plugin clear --force` | Delete without confirmation prompt |
| `port plugin remove` | Fully uninstall the plugin: removes hooks, skills, and config (other hooks preserved) |
| `port plugin remove --force` | Uninstall without confirmation prompt |
| `port plugin status` | Show current configuration and last sync time |

---

## Checking your configuration

```sh
port plugin status
```

Output example:

```
Port Plugin Status
────────────────────────────────────────
Scope:           global
Last synced:     2026-03-25T09:00:00Z

Targets (3):
  - /Users/you/.cursor/skills/port/
  - /Users/you/.claude/skills/port/
  - /Users/you/.agents/skills/port/

Skill selection: custom
  Groups (2):
    - engineering-skills
    - devops-skills
  Individual skills (1):
    - pr-review
```

---

## Deleting locally synced skills

To remove all Port skills from your local AI tool directories:

```sh
port plugin clear
```

This deletes the `skills/port/` directory from every configured target and prompts for confirmation first. To skip the prompt:

```sh
port plugin clear --force
```

> **Note:** This only removes the skill files — it does **not** remove the session-start hooks. Skills will be re-synced automatically the next time you start a new AI session, or you can run `port plugin sync` to sync immediately.

---

## Uninstalling

To fully remove the Port plugin — hooks, skill files, and saved config:

```sh
port plugin remove
```

This surgically removes only the Port entries from your `hooks.json` / `settings.json` files. Any other hooks you have configured are left untouched.

To skip the confirmation:

```sh
port plugin remove --force
```

---

## How it works

```
~/.cursor/hooks.json           ← sessionStart → port plugin sync
~/.claude/settings.json        ← UserPromptSubmit → port plugin sync
~/.agents/hooks.json           ← sessionStart → port plugin sync

port plugin sync
  └─ GET /v1/blueprints/skill_group/entities
  └─ GET /v1/blueprints/skill/entities
  └─ for each skill, checks skill.properties.location:
       "global"  → writes to ~/.cursor/skills/port/{group}/{skill}/SKILL.md
                        ~/.claude/skills/port/{group}/{skill}/SKILL.md
                        ~/.agents/skills/port/{group}/{skill}/SKILL.md
       "project" → writes to ./{cwd}/skills/port/{group}/{skill}/SKILL.md
  └─ removes any local skill dirs no longer in Port

port plugin clear
  └─ removes ~/.cursor/skills/port/
  └─ removes ~/.claude/skills/port/
  └─ removes ~/.agents/skills/port/
```

### Skill location

Each skill in Port has a `location` property on the `skill` blueprint:

| Value | Where the skill is written |
|-------|---------------------------|
| `global` *(default)* | Your AI tool directories (`~/.cursor/skills/port/`, etc.) |
| `project` | The current working directory where you ran `port plugin sync` |

If the `location` property is missing or set to any other value, `global` is used. You do not choose this when running `port plugin init` — it is fully controlled from Port.

Skills are written as `SKILL.md` files under `skills/port/{group}/{skill}/`, which is the format expected by Cursor, Claude Code, and the Agents runtime. Skills with no group are placed in `_skills_without_group/`. Reference and asset files defined on the skill entity are written alongside `SKILL.md`.

---

## Configuration file

The plugin stores its state in `~/.port/config.yaml` under a `plugin` section:

```yaml
plugin:
  scope: global
  targets:
    - /Users/you/.cursor
    - /Users/you/.claude
    - /Users/you/.agents
  selected_groups:
    - engineering-skills
  selected_skills:
    - pr-review
  last_synced_at: "2026-03-25T09:00:00Z"
```

You can edit this file directly if you prefer.

---

## Troubleshooting

**Skills are not appearing in Cursor**
- Verify the hook is installed: check that `~/.cursor/hooks.json` contains a `sessionStart` entry.
- Start a brand new Cursor window (existing sessions do not re-run the hook).
- Run `port plugin sync` manually to see any error output.

**Authentication errors**
- Re-run `port auth login` to refresh your token.

**Port API errors**
- Confirm your Port account has the `skill` and `skill_group` blueprints set up.
- Check your API URL with `port config --show`.
