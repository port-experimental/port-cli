# Setting up AI skill hooks with Port CLI

The `port skills` commands let you automatically load skills from your Port
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
port skills init
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
start any other supported tool. The hook runs `port skills sync` automatically
in the background, refreshing your local skills from Port before the AI
assistant starts.

---

## Updating your skill selection

To change which skills are synced, re-run init:

```sh
port skills init
```

This re-presents the full setup prompt. Your new selection is saved and the skills are immediately re-synced.

---

## Manual sync

To sync skills without changing your selection:

```sh
port skills sync
```

---

## Command reference

| Command | Description |
|---------|-------------|
| `port skills init` | Install hooks + configure skill selection (one-time setup, re-run to change selection) |
| `port skills sync` | Sync skills using saved selection, removing any stale local skills |
| `port skills clear` | Delete locally synced skill files from AI tool dirs (hooks remain; with confirmation) |
| `port skills clear --force` | Delete skill files without confirmation prompt |
| `port skills status` | Show current configuration and last sync time |
| `port cache clear` | Full cleanup: remove hooks, skill files, and config — everything Port CLI installed |
| `port cache clear --force` | Full cleanup without confirmation prompt |

---

## Checking your configuration

```sh
port skills status
```

Output example:

```
Port Skills Status
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

## Clearing locally synced skills

To remove all Port skill files from your local AI tool directories without touching hooks or config:

```sh
port skills clear
```

This deletes the `skills/port/` directory from every configured target and project dir, and prompts for confirmation first. To skip the prompt:

```sh
port skills clear --force
```

> **Note:** This only removes the skill files — hooks and config remain intact. Skills will be re-synced automatically the next time you start a new AI session, or run `port skills sync` to sync immediately.

---

## Full cleanup

To remove everything Port CLI installed — hooks, skill files, and saved config:

```sh
port cache clear
```

This surgically removes only the Port entries from your `hooks.json` / `settings.json` files (other hooks are left untouched), deletes all skill files, and clears the skills section from `~/.port/config.yaml`.

To skip the confirmation:

```sh
port cache clear --force
```

---

## How it works

```
~/.cursor/hooks.json                  ← sessionStart → port skills sync
~/.claude/settings.json               ← UserPromptSubmit → port skills sync
~/.gemini/settings.json               ← SessionStart → port skills sync
~/.codex/hooks.json                   ← sessionStart → port skills sync
~/.codeium/windsurf/hooks.json        ← pre_user_prompt → port skills sync
~/.copilot/hooks.json                 ← sessionStart → port skills sync

port skills sync
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

port skills clear
  └─ removes skills/port/ from every configured AI tool dir
  └─ removes skills/port/ from every registered project dir

port cache clear
  └─ removes Port hook entries from all AI tool hook/settings files
  └─ removes skills/port/ from all dirs (same as port skills clear)
  └─ clears skills config from ~/.port/config.yaml
```

### Skill location

Each skill in Port has a `location` property on the `skill` blueprint:

| Value | Where the skill is written |
|-------|---------------------------|
| `global` *(default)* | Your AI tool directories (`~/.cursor/skills/port/`, etc.) |
| `project` | Every directory where you have run `port skills init` |

If the `location` property is missing or set to any other value, `global` is used. You do not choose this when running `port skills init` — it is fully controlled from Port.

Running `port skills init` in a project registers that directory. You can run it in multiple projects; all of them will receive project-scoped skills on every `port skills sync`.

Skills are written as `SKILL.md` files under `skills/port/{group}/{skill}/`, which is the format expected by supported AI tools. Skills with no group are placed in `_skills_without_group/`. Reference and asset files defined on the skill entity are written alongside `SKILL.md`.

---

## Hook formats by tool

| Tool | Default hook file | Event key |
|------|-------------------|-----------|
| Cursor | `~/.cursor/hooks.json` | `sessionStart` |
| Claude Code | `~/.claude/settings.json` | `UserPromptSubmit` |
| Gemini CLI | `~/.gemini/settings.json` | `SessionStart` |
| OpenAI Codex | `~/.codex/hooks.json` | `sessionStart` |
| Windsurf | `~/.codeium/windsurf/hooks.json` | `pre_user_prompt` |
| GitHub Copilot | `~/.copilot/hooks.json` | `sessionStart` |

GitHub Copilot uses `~/.copilot` for personal (global) skills and `<repo>/.github`
for project-scoped skills, following the
[agent skills specification](https://docs.github.com/en/copilot/concepts/agents/about-agent-skills).

### XDG and custom config directories

For tools that support non-default config locations, the CLI checks environment
variables before falling back to the default `~/.<tool>` path:

| Tool | Env override | XDG support |
|------|-------------|-------------|
| Cursor | `CURSOR_CONFIG_DIR` | `$XDG_CONFIG_HOME/cursor` |

Resolution order for each tool:

1. **Tool-specific env var** — if `CURSOR_CONFIG_DIR` is set, that path is used directly.
2. **`XDG_CONFIG_HOME`** — if the tool has XDG support and `XDG_CONFIG_HOME` is set, the tool's XDG directory name is used under it (e.g. `$XDG_CONFIG_HOME/cursor`).
3. **Default** — `~/.<tool>` (e.g. `~/.cursor`).

Other tools (Claude Code, Gemini CLI, etc.) do not currently support custom
config directories and always use their default paths.

---

## Configuration file

The CLI stores its state in `~/.port/config.yaml` under a `skills` section:

```yaml
skills:
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

Older configs may still use a top-level `plugin:` key; the CLI reads that for backward compatibility and writes `skills:` on the next save.

You can edit this file directly if you prefer.

---

## Troubleshooting

**Skills are not appearing in my AI tool**
- Verify the hook is installed: check that the appropriate hooks file exists (see table above).
- Start a brand new session (existing sessions do not re-run the hook).
- Run `port skills sync` manually to see any error output.

**Authentication errors**
- Re-run `port auth login` to refresh your token.

**Port API errors**
- Confirm your Port account has the `skill` and `skill_group` blueprints set up.
- Check your API URL with `port config --show`.

**GitHub Copilot hooks not working**
- GitHub Copilot only supports repo-scoped hooks. Make sure you ran `port skills init` from the root of the repository where you want hooks installed.
