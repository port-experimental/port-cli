# Changelog

All notable changes to this project will be documented in this file.

## 0.1.14 (17-03-2026)

Changes since `v0.1.13`:

### Added

- Added `api pages` subcommands for page management:
  - `port api pages get [page-id]`
  - `port api pages delete [page-id]`
- Added support for preserving blueprint `ownership` during import, including deferred re-application after blueprint relations exist.
- Added ownership-specific topological sorting for blueprint imports so direct ownership is applied first, followed by inherited ownership in dependency order.

### Changed

- Blueprint import now applies `ownership` updates level-by-level based on ownership path dependencies to avoid orphaned inherited ownership updates.
- Added regression coverage for page API commands and blueprint ownership import ordering.
