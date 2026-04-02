# Changelog

All notable changes to this project will be documented in this file.

## 0.1.18 (02-04-2026)

### Added
- Added `clear` command with `--pages` support to delete root catalog pages and folders, including optional protected-item deletion via `--delete-protected`.

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
