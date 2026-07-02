# Changelog

All notable changes to this project will be documented in this file.

## 0.3.5 (02-07-2026)

### Added
- `port export`, `port import`, and `port migrate` now support `--max-errors` to control how many errors are shown in text output; use `--max-errors 0` to show all errors or `--max-errors -1` to hide detailed errors. Default is `5`

### Fixed
- `port migrate` now updates an existing page if a page planned for creation already exists in the target org.

## 0.3.4 (01-07-2026)

### Added
- `port migrate` now handles large entity migrations more reliably by processing entities in smaller batches instead of loading the full migration set at once.

### Fixed
- `port migrate` now shows the specific migration errors that occurred and preserves partial migration counts when a migration stops.

## 0.3.3 (30-06-2026)

### Added
- New `--skip-system-blueprint-properties` flag for `port export`, `port import`, and `port migrate`. By default, `--skip-system-blueprints` now still carries custom schema additions on known system blueprints; the new flag opts out and drops those patches entirely. Default ownership on `_rule_result` and `_user` is treated as Port-managed and is not copied.

## 0.3.2 (30-06-2026)

### Added
- Streaming export writers for JSON and tar archives, plus streaming import readers for JSON and tar archives, so large entity exports/imports no longer require materializing the full export file in memory.
- Entity count-based retrieval for export, import, and migrate: blueprints with more than 10,000 entities use paginated entity search, while smaller blueprints continue to use the canonical entity GET endpoint.

### Fixed
- Avoid unbounded entity GET requests for blueprints with more than 10,000 entities.

## 0.3.0 (30-06-2026)

### Added
- `port api` subcommands for pages, teams, users, scorecards, actions, permissions, agents, AI invoke, action-runs, webhooks, and audit log.
- Per-resource ID filters on `port export`: `--scorecards`, `--actions`, `--pages`, `--integrations`, `--teams`, `--users`, and `--entities` (each accepts comma-separated IDs for client-side filtering after bulk fetch).
- `port migrate` now supports the same per-resource flags as `port export`, with optional ID filters. Use `--integrations` to migrate integration mapping for specific installation IDs only.
- `--jq` flag on `port clear` for conditional entity deletion (e.g. `port clear --entities --blueprint aiSpec --jq '.properties.organization == "example-org"'`).
- New `--users-as-disabled` flag for `port import` and `port migrate`. When set, non-admin users are created with `DISABLED` status instead of the default `STAGED`. Admin users are always created as `STAGED` regardless of this flag.
- Streaming export writers for JSON and tar archives, plus streaming import readers for JSON and tar archives, so large entity exports/imports no longer require materializing the full export file in memory.
- Entity count-based retrieval for export, import, and migrate: blueprints with more than 10,000 entities use paginated entity search, while smaller blueprints continue to use the canonical entity GET endpoint.
- Skills: catalog now loads from the Port AI service (`GET /v1/skills`) instead of blueprint entities. Added `port skills search`, `port skills select`, and multipart skill upload via `port skills create`; removed `archive`, `load`, and `unload` commands. Supports M2M credentials, `--location` on create, and active-version publishing.

### Changed
- Per-resource flags on `port export` and `port migrate` (`--blueprints`, `--actions`, `--scorecards`, `--pages`, `--integrations`, `--teams`, `--users`, `--entities`) now implicitly restrict the operation to that resource type when explicitly set — no `--include` required. Flags can be combined freely (e.g. `--blueprints bp1 --actions act1` exports or migrates only blueprints and actions).
- User import now uses the `_user` blueprint entity bulk API (`POST /blueprints/_user/entities/bulk`) instead of the invite/update user APIs. New users are created with `STAGED` status (pending activation) rather than receiving an invitation email. Existing users are updated with source data as-is.
- Export: `--pages` and `--actions` auto-include their permission types; page exports filter folders to only ancestors of the selected pages.
- Import: `port import --include` for org-level resources (pages, actions, integrations, teams, users) no longer requires blueprints in the input file.

### Fixed
- API client now honors `Retry-After` on 429 responses, raises entity import concurrency to 30, and bounds export blueprint concurrency to 10.
- Import and migrate: blueprint updates use fetch-and-merge so Phase 1 PUTs never strip existing relation definitions from the target.
- Import and migrate: scorecard updates fetch the existing set, merge changes, and bulk-PUT the result — preventing sibling scorecards from being deleted.
- Import and migrate: system audit fields (`createdAt`, `updatedAt`, `createdBy`, `updatedBy`, `id`) are stripped from fetched blueprints before PUT.
- `port clear` now paginates through all entities before deletion and validates `--blueprint` names strictly (typos return a clear `BLUEPRINT_NOT_FOUND` error instead of a misleading org error).
- Skills: `auto_sync` groups no longer bypass group selection during init — all skills follow the user's chosen groups.

## 0.2.25 (24-06-2026)

### Documentation
- Documented `port clear` in README with resource types, limitations, and org-reset workflows.

## 0.2.17 (28-05-2026)

### Fixed
- Fixed 8 bugs preventing blueprint/action permissions from being imported or migrated: JSON loader now reads `blueprint_permissions` and `action_permissions` keys, export emits consistent snake_case keys, permission errors no longer report false success, `--include` flag is now enforced for permission resource types, permission comparison uses normalized JSON instead of `reflect.DeepEqual`, dry-run now reports permission counts, silent fetch failures now surface as warnings, and `migrate` now collects and applies blueprint/action permissions.

## 0.2.16 (27-05-2026)

### Added
- Skills now support versioned skill files. The CLI fetches the latest version of each skill, paginates large skill catalogs, and preserves legacy directory names for backward compatibility.

### Fixed
- Fixed `compareVersionStrings` pre-release segment ordering.
- Removed duplicate API calls during `port skills init` by reusing the fetched catalog.
- Narrowed `isMissingSkillBlueprintError` to match Port-specific error codes instead of broad substrings.

## 0.2.15 (25-05-2026)

### Added
- Added `port skills remove` command to prune skill groups, individual skills, and AI tool targets from the saved selection without re-running `init`. Supports interactive selection and `--group`, `--skill`, `--tool` flags.
- Added `port tree` command to display a tree view of Port resources.

## 0.2.14 (19-05-2026)

### Fixed
- Fixed skills belonging to multiple groups being silently dropped to a single group. Skills now appear under every group they belong to in `port skills list` and are written to all corresponding group directories on sync.

## 0.2.13 (18-05-2026)

### Added
- Added `port skills add` command to extend your saved skill selection or add AI tool hooks without re-running the full `port skills init` prompt. Supports interactive selection of only new groups, skills, and tools, or non-interactive `--group`, `--skill`, and `--tool` flags. Syncs skills after updating the configuration.

## 0.2.12 (18-05-2026)

### Added
- Skills sync now includes `scripts` and `additional_files` from Port skill entities.

## 0.2.11 (18-05-2026)

### Fixed
- Improved error messages for missing credentials and authentication issues with dynamic command suggestions pointing to the correct `port config` and `port auth` commands.

## 0.2.10 (14-05-2026)

### Fixed
- Fixed `_rule_result` blueprint updates using PUT instead of PATCH, which caused relation fields with `rule_result_target` type to be rejected. Relations of that type are now omitted before sending `_rule_result` payloads.

## 0.2.9 (08-05-2026)

### Fixed
- Applied `--include-rule-results` flag consistently across `export` and `migrate` commands (default: `true`). Previously the flag only affected `import`.

## 0.2.8 (08-05-2026)

### Added
- Added `--include-rule-results` flag to `import` (default `true`) to control whether `_rule_result` entities are included.

### Fixed
- Fixed aggregation property imports that depend on `_rule_result` relations by retrying failed Phase 2d (aggregation properties) after system blueprints are applied in Phase 3.
- Fixed mirror properties that reference aggregation properties on related blueprints by collecting Phase 2c failures and retrying them after Phase 2d (aggregation properties) completes.
- Fixed a race condition in the two-pass concurrent aggregation property import by switching to topological level-by-level application.
- Fixed `after_item_not_in_parent` (403) being misclassified as `AUTH` instead of `DEPENDENCY`.
- Fixed `removeSingleFailingPageField` to explicitly null the `after` field on PATCH instead of omitting it, so stale values are cleared in the target.

## 0.2.7 (07-05-2026)

### Fixed
- Fixed non-deterministic `DEPENDENCY` errors on repeated `migrate` runs caused by mirror properties running before aggregation properties in the import phase order. Added `TopologicalSortAggProps` to apply aggregation properties level-by-level across blueprint dependencies.
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
