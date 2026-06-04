#!/usr/bin/env bash
# Deprecated wrapper — local skills E2E now runs via Go (see docs/skills-e2e-local.md).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec make -C "${SCRIPT_DIR}/.." e2e-skills-local
