# Setting up AI skill hooks with Port CLI

The `port plugin` commands let you automatically load skills from your Port
organization into your local AI coding tools at the start of every session.

Supported tools: **Cursor**, **Claude Code**, **Gemini CLI**, **OpenAI Codex**,
**Windsurf**, and **GitHub Copilot**.

## Prerequisites

- `port` CLI installed (`npm install -g @port-experimental/port-cli` or download from [GitHub Releases](https://github.com/port-experimental/port-cli/releases))
- A Port account with skills configured in the `skill` and `skill_group` blueprints
- At least one supported AI tool installed

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

1. **Which AI tools to install hooks for** — an interactive multi-select lists
   all supported tools. Hooks are installed globally in your home directory
   (e.g. `~/.cursor/hooks.json`, `~/.copilot/hooks.json`).
   GitHub Copilot uses `~/.copilot` for personal skills and `<repo>/.github`
   for project-scoped skills.

2. **Which skills to sync** — an interactive prompt shows all available skill
   groups and individual skills from your Port organization.
   - Skills marked `required = true` in Port are always synced regardless of
     your selection. They appear as a note before the prompt.
   - Select any combination of groups and individual skills you want.

After confirming your selection, the CLI:
- Writes (or merges) a `hooks.json` / `settings.json` into each AI tool directory
- Immediately syncs the selected skills to the correct locations (see below)
- Saves your selection to `~/.port/config.yaml` so future syncs are automatic

---

## Step 3 — Start a new AI session

Open a new Cursor window, start a Claude Code session, launch Gemini CLI, or
start any other supported tool. The hook runs `port plugin sync` automatically
in the background, refreshing your local skills from Port before the AI
assistant starts.

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
| `port plugin clear` | Delete all locally synced Port skills from AI tool dirs and project dirs (with confirmation) |
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
Last synced:     2026-03-25T09:00:00Z

Hook targets (6):
  - /Users/you/.cursor/skills/port/
  - /Users/you/.claude/skills/port/
  - /Users/you/.gemini/skills/port/
  - /Users/you/.codex/skills/port/
  - /Users/you/.codeium/windsurf/skills/port/
  - /Users/you/.copilot/skills/port/

Project directories (1):
  - /Users/you/myproject

Skill selection:
  Groups:           all
  Ungrouped skills: all
```

---

## Deleting locally synced skills

To remove all Port skills from your local AI tool directories and project directories:

```sh
port plugin clear
```

This deletes the `skills/port/` directory from every configured target and project dir, and prompts for confirmation first. To skip the prompt:

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
~/.cursor/hooks.json                  ← sessionStart → port plugin sync
~/.claude/settings.json               ← UserPromptSubmit → port plugin sync
~/.gemini/settings.json               ← SessionStart → port plugin sync
~/.codex/hooks.json                   ← sessionStart → port plugin sync
~/.codeium/windsurf/hooks.json        ← pre_user_prompt → port plugin sync
~/.copilot/hooks.json                 ← sessionStart → port plugin sync

port plugin sync
  └─ GET /v1/blueprints/skill_group/entities
  └─ GET /v1/blueprints/skill/entities
  └─ for each skill, checks skill.properties.location:
       "global"  → writes to every AI tool dir configured during init
                   e.g. ~/.cursor/skills/port/{group}/{skill}/SKILL.md
                   e.g. ~/.copilot/skills/port/{group}/{skill}/SKILL.md
       "project" → writes to the matching tool sub-directory inside each
                   project dir registered in ~/.port/config.yaml
                   e.g. ~/projects/my-app/.cursor/skills/port/{group}/{skill}/SKILL.md
                   e.g. ~/projects/my-app/.github/skills/port/{group}/{skill}/SKILL.md
  └─ removes any local skill dirs no longer in Port

port plugin clear
  └─ removes skills/port/ from every configured AI tool dir
  └─ removes skills/port/ from every registered project dir

port plugin remove
  └─ removes Port hook entries from all AI tool hook/settings files
  └─ removes skills/port/ from all dirs (same as clear)
  └─ clears plugin config from ~/.port/config.yaml
```

### Skill location

Each skill in Port has a `location` property on the `skill` blueprint:

| Value | Where the skill is written |
|-------|---------------------------|
| `global` *(default)* | Your AI tool directories (`~/.cursor/skills/port/`, etc.) |
| `project` | Every directory where you have run `port plugin init` |

If the `location` property is missing or set to any other value, `global` is used. You do not choose this when running `port plugin init` — it is fully controlled from Port.

Running `port plugin init` in a project registers that directory. You can run it in multiple projects; all of them will receive project-scoped skills on every `port plugin sync`.

Skills are written as `SKILL.md` files under `skills/port/{group}/{skill}/`, which is the format expected by supported AI tools. Skills with no group are placed in `_skills_without_group/`. Reference and asset files defined on the skill entity are written alongside `SKILL.md`.

---

## Hook formats by tool

| Tool | Hook file | Event key |
|------|-----------|-----------|
| Cursor | `~/.cursor/hooks.json` | `sessionStart` |
| Claude Code | `~/.claude/settings.json` | `UserPromptSubmit` |
| Gemini CLI | `~/.gemini/settings.json` | `SessionStart` |
| OpenAI Codex | `~/.codex/hooks.json` | `sessionStart` |
| Windsurf | `~/.codeium/windsurf/hooks.json` | `pre_user_prompt` |
| GitHub Copilot | `~/.copilot/hooks.json` | `sessionStart` |

GitHub Copilot uses `~/.copilot` for personal (global) skills and `<repo>/.github`
for project-scoped skills, following the
[agent skills specification](https://docs.github.com/en/copilot/concepts/agents/about-agent-skills).

---

## Configuration file

The plugin stores its state in `~/.port/config.yaml` under a `plugin` section:

```yaml
plugin:
  targets:
    - /Users/you/.cursor
    - /Users/you/.claude
    - /Users/you/.gemini
    - /Users/you/.codex
    - /Users/you/.codeium/windsurf
    - /Users/you/.copilot
  project_dirs:
    - /Users/you/myproject
  select_all_groups: true
  select_all_ungrouped: true
  selected_groups: []
  selected_skills: []
  last_synced_at: "2026-03-25T09:00:00Z"
```

You can edit this file directly if you prefer.

---

## Troubleshooting

**Skills are not appearing in my AI tool**
- Verify the hook is installed: check that the appropriate hooks file exists (see table above).
- Start a brand new session (existing sessions do not re-run the hook).
- Run `port plugin sync` manually to see any error output.

**Authentication errors**
- Re-run `port auth login` to refresh your token.

**Port API errors**
- Confirm your Port account has the `skill` and `skill_group` blueprints set up.
- Check your API URL with `port config --show`.

**GitHub Copilot hooks not working**
- GitHub Copilot only supports repo-scoped hooks. Make sure you ran `port plugin init` from the root of the repository where you want hooks installed.
