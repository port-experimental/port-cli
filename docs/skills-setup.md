# Setting up AI skill hooks with Port CLI

The `port skills` commands let you automatically load skills from your Port
organization into your local AI coding tools at the start of every session.

Supported tools: **Cursor**, **Claude Code**, **Gemini CLI**, **OpenAI Codex**,
**Windsurf**, and **GitHub Copilot**.

## Prerequisites

- `port` CLI installed (`npm install -g @port-experimental/port-cli` or download from [GitHub Releases](https://github.com/port-experimental/port-cli/releases))
- A Port account with skills published in Port (served by Port ai-service)
- At least one supported AI tool installed

---

## Step 1 — Authenticate

Choose one of the following.

### Interactive (browser login)

```sh
port auth login
# optional: port auth login --region eu|us
```

This opens a browser window for SSO and stores a token in `~/.port/creds.json`.

### Non-interactive (machine credentials)

For scripts, CI, and local dev without a browser, configure Port application
credentials instead of logging in. The skills commands **prefer** machine
credentials when `client_id` and `client_secret` are set.

See the README section [Non-interactive and CI usage](../README.md#non-interactive-and-ci-usage)
for full detail. Minimal setup with environment variables:

```sh
export PORT_CLIENT_ID="your-client-id"
export PORT_CLIENT_SECRET="your-client-secret"
export PORT_API_URL="https://api.getport.io/v1"

# Local stack example:
# export PORT_API_URL="http://localhost:3000/v1"
# export PORT_AI_SERVICE_URL="http://localhost:3016/v1"
```

Equivalent options: `~/.port/config.yaml` (`client_id` / `client_secret` /
`api_url` per org), `~/.port/.env` (same `PORT_*` variable names), or global
flags `--client-id`, `--client-secret`, `--api-url` on each command.

Use your Port **application** Client ID and Secret, not the organization ID.

---

## Step 2 — Install hooks

Run the one-time setup command:

```sh
port skills init
```

You will be asked two questions:

1. **Which AI tools to install hooks for** — an interactive multi-select lists
  all supported tools. For Cursor, Claude Code, Gemini CLI, OpenAI Codex, and
   Windsurf, hooks are installed globally in your home directory (e.g.
   `~/.cursor/hooks.json`). **GitHub Copilot is repo-scoped only:** hooks are
   written to `<repo>/.github/hooks/hooks.json` and skills under
   `<repo>/.github/skills/port/`. Run `port skills init` from the repository
   root when you select Copilot.
2. **Which skills to sync** — an interactive prompt shows all available skill
  groups and individual skills from your Port organization.
  - Skills marked `required = true` in Port are always synced regardless of
  your selection. They appear as a note before the prompt.
  - Select any combination of groups and individual skills you want.

After confirming your selection, the CLI:

- Writes (or merges) a `hooks.json` / `settings.json` into each AI tool directory
- Immediately syncs the selected skills to the correct locations (see below)
- Saves your selection to `~/.port/config.yaml` so future syncs are automatic

### Non-interactive init (CI / scripts)

Pass tools and a selection strategy explicitly. Hooks are **not** installed unless
you opt in with `--install-hooks`:

```sh
port skills init --tool Cursor --install-hooks \
  --select-all-groups --select-all-ungrouped --force
```

Use `port --yes` / `-y` to skip confirmation prompts where supported.
Root `--yes` does **not** bypass the git-clean guard (use `--ignore-git-dirty`).

---

## Step 3 — Start a new AI session

Open a new Cursor window, start a Claude Code session, launch Gemini CLI, or
start any other supported tool. The hook runs `port skills sync` automatically
in the background, refreshing your local skills from Port before the AI
assistant starts.

---

## Updating your skill selection

To change which optional skills and groups are synced (without reinstalling hooks):

```sh
port skills select
```

This re-presents the same group/skill prompts as `port skills init`. Your new selection replaces the previous one and skills are re-synced.

Non-interactive example:

```sh
port skills select --select-all-groups --select-all-ungrouped --ignore-git-dirty
```

Or pick explicit groups:

```sh
port skills select --group demo-engineering-optional --group demo-security-manual
```

You can also re-run full init (including hook install):

```sh
port skills init
```

---

## Manual sync

To sync skills without changing your selection:

```sh
port skills sync
```

---

## Command reference


| Command                     | Description                                                                            |
| --------------------------- | -------------------------------------------------------------------------------------- |
| `port skills init`          | Install hooks + configure skill selection (one-time setup) |
| `port skills select`        | Change skill/group selection and re-sync (no hook changes); same selection flags as init |
| `port skills init --install-hooks` | Non-interactive: write hook files when combined with `--tool` |
| `port skills list`          | List skills with title, location, timestamps, and latest version metadata (ai-service); `--json` for machine output |
| `port skills search <query>` | Search skills by identifier or title substring (ai-service `GET /v1/skills/search`); `--json`, `--limit`, `--published-only` |
| `port skills create <dir>`  | Create a skill from a local folder (must include `SKILL.md`); `--location global\|project` (default `global`); non-interactive: pass `--identifier`, `--published`, etc. |
| `port skills edit <id> <dir>` | Upload a new version from a local folder                                             |
| `port skills archive <id>`  | Archive all versions of a skill                                                        |
| `port skills sync`          | Sync published skills to local AI tool dirs (via ai-service)                           |
| `port skills sync --ignore-git-dirty` | Sync even when `skills/port` has uncommitted git changes |
| `port skills --org NAME`    | Use a specific organization from config (default org is not hard-coded to `production`) |
| `port skills clear`         | Delete locally synced skill files from AI tool dirs (hooks remain; with confirmation)  |
| `port skills clear --force` | Delete skill files without confirmation prompt                                         |
| `port skills status`        | Show current configuration and last sync time                                          |
| `port cache clear`          | Full cleanup: remove hooks, skill files, and config — everything Port CLI installed    |
| `port cache clear --force`  | Full cleanup without confirmation prompt                                               |


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
  - /Users/you/myproject/.github/skills/port/

Project directories (1):
  - /Users/you/myproject

Skill selection:
  Groups:           all
  Ungrouped skills: all
```

The **GitHub Copilot** line is a path inside your **repository** (`…/myproject/.github`), not under your home directory like the other tools. The same `myproject` folder also appears under **Project directories** because `port skills init` registers that repo for Port `location=project` skills (all tools) and ties Copilot hooks to the repo root. That duplication in the example is intentional: one line is the Copilot skill/hook root, the other is the registered project root used when syncing.

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

This surgically removes only the Port entries from your `hooks.json` / `settings.json` files (other hooks are left untouched), deletes all skill files, and clears the skills section from `~/.port/config.yaml`. GitHub Copilot hooks under `<repo>/.github/hooks/` are found using the saved paths in your skills config, so cleanup works even if you run the command outside the repository.

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
<repo>/.github/hooks/hooks.json       ← sessionStart → port skills sync (Copilot)

port skills sync
  └─ GET {ai-service}/v1/skills (grouped catalog: groups + ungroupedSkills, with files)
  └─ for each skill, checks location from the catalog:
       "global"  → writes to every AI tool dir configured during init
                   e.g. ~/.cursor/skills/port/{group}/{skill}/SKILL.md
                   e.g. <repo>/.github/skills/port/{group}/{skill}/SKILL.md (Copilot)
       "project" → writes to the matching tool sub-directory inside each
                   project dir registered in ~/.port/config.yaml
                   e.g. ~/projects/my-app/.cursor/skills/port/{group}/{skill}/SKILL.md
                   e.g. ~/projects/my-app/.github/skills/port/{group}/{skill}/SKILL.md
  └─ removes any local skill dirs no longer in Port
  └─ if the git work tree has uncommitted changes under skills/port/, skips writes
      for that repo (exit 1 on `port skills sync`; use --ignore-git-dirty to override)

port skills clear
  └─ removes skills/port/ from every configured AI tool dir
  └─ removes skills/port/ from every registered project dir

port cache clear
  └─ removes Port hook entries from all AI tool hook/settings files (missing or
      invalid hook files are skipped — no error if hooks were never installed)
  └─ removes skills/port/ from all dirs (same as port skills clear)
  └─ clears skills config from ~/.port/config.yaml
```

### Skill location

Each skill in Port has a `location` property on the `skill` blueprint:


| Value                | Where the skill is written                                                                                                                                             |
| -------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `global` *(default)* | Your AI tool directories (`~/.cursor/skills/port/`, etc.). If GitHub Copilot is enabled, that includes `<repo>/.github/skills/port/` for each repo where you ran init. |
| `project`            | Every directory where you have run `port skills init`                                                                                                                  |


If the `location` property is missing or set to any other value, `global` is used. You do not choose this when running `port skills init` — it is fully controlled from Port.

Running `port skills init` in a project registers that directory. You can run it in multiple projects; all of them will receive project-scoped skills on every `port skills sync`.

**GitHub Copilot:** Copilot does not load agent skills or hooks from a global home directory in this flow. Hooks and synced skills live only under `<repo>/.github/`. Older CLI versions may have used `~/.copilot`; `port cache clear` removes Port hook entries from that legacy path too.

Skills are written as `SKILL.md` files under `skills/port/{group}/{skill}/`, which is the format expected by supported AI tools. Skills with no group are placed in `_skills_without_group/`. Reference, asset, script (`scripts`), and other bundled files (`additional_files`) defined on the skill entity—each an array of `{ path, content }` like references and assets—are written alongside `SKILL.md`.

---

## Hook formats by tool


| Tool           | Default hook file                 | Event key          |
| -------------- | --------------------------------- | ------------------ |
| Cursor         | `~/.cursor/hooks.json`            | `sessionStart`     |
| Claude Code    | `~/.claude/settings.json`         | `UserPromptSubmit` |
| Gemini CLI     | `~/.gemini/settings.json`         | `SessionStart`     |
| OpenAI Codex   | `~/.codex/hooks.json`             | `sessionStart`     |
| Windsurf       | `~/.codeium/windsurf/hooks.json`  | `pre_user_prompt`  |
| GitHub Copilot | `<repo>/.github/hooks/hooks.json` | `sessionStart`     |


GitHub Copilot agent hooks and synced skills are **repository-local** under
`<repo>/.github/`, following the
[agent skills specification](https://docs.github.com/en/copilot/concepts/agents/about-agent-skills).
Hook entries use GitHub’s agent format (`type: command`, `bash`, `powershell`, etc.) as described in
[About hooks](https://docs.github.com/en/copilot/concepts/agents/cloud-agent/about-hooks), not the Cursor-style `{ "command": "..." }` object.

### XDG and custom config directories

For tools that support non-default config locations, the CLI checks environment
variables before falling back to the default `~/.<tool>` path:


| Tool   | Env override        | XDG support               |
| ------ | ------------------- | ------------------------- |
| Cursor | `CURSOR_CONFIG_DIR` | `$XDG_CONFIG_HOME/cursor` |


Resolution order for each tool:

1. **Tool-specific env var** — if `CURSOR_CONFIG_DIR` is set, that path is used directly.
2. `**XDG_CONFIG_HOME*`* — if the tool has XDG support and `XDG_CONFIG_HOME` is set, the tool's XDG directory name is used under it (e.g. `$XDG_CONFIG_HOME/cursor`).
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
    - /Users/you/myproject/.github
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

## FAQ

**How are skills managed in Port?**

Skills are standard Port entities on the `skill` and `skill_group` blueprints. Because they live in the Port catalog, you can populate and keep them up to date using any of the normal ingestion methods:

- **Port UI** — create and edit skill entities directly in the catalog.
- **Port API** — `POST /v1/blueprints/skill/entities` to create or upsert skills programmatically from any script or CI pipeline.
- **Ocean integrations** — Port's 60+ plug-and-play integrations (GitHub, GitLab, Kubernetes, Jira, etc.) can map tool data to skill entities via the standard mapping configuration. See [sync data to catalog](https://docs.port.io/build-your-software-catalog/sync-data-to-catalog) for the full list.
- **Webhooks** — push skill updates from external systems by sending a payload to a Port webhook endpoint.
- **IaC (Terraform / Pulumi)** — define skill entities as infrastructure-as-code resources and apply them as part of your normal delivery pipeline.
- **Custom Ocean integrations** — build a dedicated integration for any internal tool using the Ocean framework.

Whichever method you use, `port skills sync` will pick up the latest state of all skill entities the next time a hook fires or you run the command manually.

---

**Does the CLI support skill versioning?**

No. The CLI always reflects the current state of skill entities in Port — there is no version history or rollback for locally synced skills. If you need versioning, manage it at the source: use your VCS or Port's audit log to track changes to skill entity properties over time.

---

**Can I install skills from a public skills marketplace?**

Not at this time. Skills are private to your Port organization. There is no public marketplace to browse or install community-contributed skills from. All skills must be created and managed within your own Port account.

---

## Troubleshooting

**Skills are not appearing in my AI tool**

- Verify the hook is installed: check that the appropriate hooks file exists (see table above).
- Start a brand new session (existing sessions do not re-run the hook).
- Run `port skills sync` manually to see any error output.

**Authentication errors**

- Re-run `port auth login` to refresh your token.

**Port API / ai-service errors**

- Confirm skills are published in your Port organization and ai-service is reachable.
- Check your API URL with `port config --show` (ai-service URL is derived from it).
- Use `port skills --org <name>` if you have multiple organizations in config.

**GitHub Copilot hooks not working**

- GitHub Copilot only supports repo-scoped hooks. Make sure you ran `port skills init` from the root of the repository where you want hooks installed.

