# Changelog

All notable changes to this project will be documented in this file.

## 0.2.19

### Added
- **Page permissions** support: `export`, `import`, `migrate`, and `compare` now handle page permissions as a resource type (`--include page-permissions`). Permissions are collected per-page during export, round-tripped through tar/JSON archives, diffed against the current state on import, and applied after pages exist.

### Fixed
- `port migrate`: `aggregationProperties` with cross-blueprint `pathFilter` references are now applied in topological dependency order, then retried after all system blueprint updates. Silently nullified `pathFilter` values caused by out-of-order API calls no longer occur.
- `port migrate`: Inherited `ownership` chains are now applied in topological order so that a blueprint's owner blueprint is updated before its dependents. Previously, concurrent application could silently drop inherited ownership.
- `port migrate`: `mirrorProperties` that fail Phase 2c (because they reference aggregation properties that do not yet exist) are automatically retried after Phase 2d creates those agg props.
- Blueprint and action permissions that reference orphaned relations or properties (API 422 `invalid_permissions`) are automatically sanitized and retried: the invalid fields are stripped, a warning is emitted, and the cleaned permissions are applied. Previously, any orphaned field caused the entire permissions update to fail.
- Banner wrapping: the version banner separator and centering now use the widest content element rather than the logo width, preventing ASCII art from shattering on wide taglines.

## 0.2.6

### Added
- `port skills add` to extend your saved skill selection or add AI tool hooks without re-running the full `port skills init` prompt. Supports interactive selection of only new groups, skills, and tools, or non-interactive `--group`, `--skill`, and `--tool` flags. Syncs skills after updating the configuration.
- Expanded `clear` command with new resource type flags: `--blueprints`, `--entities`, `--actions`, `--automations`, and `--scorecards`. Deletion order ensures dependents are removed before parents (e.g. entities before blueprints).
- System blueprints (identifiers starting with `_`, such as `_user` and `_team`) are always skipped for `--blueprints` and skipped by default for `--entities`, `--actions`, and `--scorecards`. Use `--include-system-blueprints` to opt in.

### Fixed
- Removed `-d` shorthand from `--data` flags in the `api` command to resolve conflict with the root `--debug` (`-d`) flag.

## 0.2.5

### Breaking

- **GitHub Copilot (`port skills`):** Hooks and synced skills are no longer installed under `~/.copilot`. They are repo-local only: `<repo>/.github/hooks/hooks.json` and `<repo>/.github/skills/port/`. Users who relied on the old layout should run `port skills init` again from each repository root with GitHub Copilot selected. `port cache clear` still removes Port entries from legacy `~/.copilot` hook files.

### Changed

- GitHub Copilot `hooks.json` entries now follow GitHub’s agent hook shape (`type`, `bash`, `powershell`, etc.) instead of the Cursor-style `{ "command": "..." }` object.
- `port skills sync` output merges the GitHub Copilot repo path into one summary line (no duplicate “global” / “project” rows for the same `.github` tree) and states that catalog skills are not synced to a separate global directory.

## 0.2.4 (14-04-2026)

### Added
- Added `--no-bearer` flag to `port auth token` to print the raw token without the `Bearer ` prefix.

### Fixed
- Fixed OAuth login flow not responding to Ctrl+C interrupt.

## 0.2.3 (10-04-2026)

### Fixed
- Added `SERVER_ERROR` error category and fixed `not_found` underscore matching in the import error categorizer.
- Sanitized null team fields before import to prevent `invalid_request` errors.

## 0.2.2 (10-04-2026)

### Added
- Excluded `_rule_result` blueprint from exports by default (can be included with `--include-system-blueprints`).

### Fixed
- Fixed `clear --pages` not deleting pages whose identifiers contain underscores: protected-item detection now checks for a leading underscore (`_foo`) instead of any underscore (`foo_bar`).
- Resolved security vulnerabilities in dependencies.
- Fixed skills sync hooks and improved test coverage.

## 0.2.1 (07-04-2026)

### Fixed
- Fixed CI release workflow warnings (updated goreleaser action and version).
- Fixed `--version` flag incorrectly showing "unknown (built from source)" instead of the injected build version.

## 0.2.0 (07-04-2026)

### Added
- Added `port auth status` command to display the current authentication status.
- Added `port api <method> <path>` generic command for making arbitrary Port API calls directly from the CLI.
- Added `port plugin` commands (`init`, `sync`, `remove`, `list`) for managing Port skills across AI coding assistants (Claude Code, GitHub Copilot, Gemini, Codex, Windsurf).
- Added branded banner/logo to `port version` output.
- Added `clear` command with `--pages` support to delete root catalog pages and folders, including optional protected-item deletion via `--delete-protected-pages`.
- Added refresh token support to OAuth authentication for automatic token renewal without re-login.

### Changed
- Improved CLI error output with styled formatting.

## 0.1.17 (26-03-2026)

### Added
- Added `auth` command with functionalities for login, logout and retrieving auth token. Login command will add organization to port config if it is missing and set a default organization if none is specified
- Add option to skip system blueprints when importing/exporting/migrating blueprints/entities with flag `--skip-system-blueprints`.
- All commands now use the auth token generated (if in cache, i.e the user ran `port auth login`)
- Added `port config get/set` subcommands

### Fixed
- Fixed bug where `--skip-entities` did not work correctly for users/teams

## 0.1.16 (19-03-2026)

### Added

- Added `--compact` flag in `api pages get` sub commmand to remove unnecessary `"widgets"` key
- Added `--verbose` flag to `import` command to print verbose debug info

### Fixed

- Fixed pages import bug where non-existent "after" references cause undefined behaviour in pages/folder placement

## 0.1.15 (18-03-2026)

### Added

- Added export and import support for catalog sidebar folders.
- Added `--show-pages-pipeline` to display the planned sidebar pages/folders execution order during import.

### Fixed

- Fixed sidebar import ordering so pages and folders are created in dependency order.
- Fixed nested folder creation to preserve `parent` relationships.

## 0.1.14 (17-03-2026)

### Added

- Added `api pages` subcommands for page management:
  - `port api pages get [page-id]`
  - `port api pages delete [page-id]`
- Added support for preserving blueprint `ownership` during import, including deferred re-application after blueprint relations exist.
- Added ownership-specific topological sorting for blueprint imports so direct ownership is applied first, followed by inherited ownership in dependency order.

### Changed

- Blueprint import now applies `ownership` updates level-by-level based on ownership path dependencies to avoid orphaned inherited ownership updates.
- Added regression coverage for page API commands and blueprint ownership import ordering.
