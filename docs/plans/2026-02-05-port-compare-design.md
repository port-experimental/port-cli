# Port Compare Command Design

## Overview

Add a `port compare` command that compares two Port organizations and shows differences across all resource types. Supports multiple output formats (text, JSON, HTML) for different use cases: quick terminal checks, CI/CD pipelines, and detailed reports.

## Command Interface

```bash
# Compare two live organizations (from config)
port compare --source staging --target production

# Compare with inline credentials (override config)
port compare --source staging --target-client-id X --target-client-secret Y

# Compare export files (offline comparison)
port compare --source ./staging-export.tar.gz --target ./prod-export.tar.gz

# Output options
port compare --source A --target B --output json
port compare --source A --target B --output html --html-file report.html
port compare --source A --target B --output html --html-simple

# Verbosity
port compare --source A --target B                 # Summary only
port compare --source A --target B --verbose       # Summary + identifiers
port compare --source A --target B --full          # Full field-level diff

# Filtering
port compare --source A --target B --include blueprints,actions,pages

# CI/CD mode
port compare --source A --target B --fail-on-diff  # Exit 1 if differences
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--source` | Source org name (from config) or export file path | Required |
| `--target` | Target org name (from config) or export file path | Required |
| `--source-client-id` | Override source org client ID | From config |
| `--source-client-secret` | Override source org client secret | From config |
| `--target-client-id` | Override target org client ID | From config |
| `--target-client-secret` | Override target org client secret | From config |
| `--output` | Output format: `text`, `json`, `html` | `text` |
| `--html-file` | Path for HTML report output | `comparison-report.html` |
| `--html-simple` | Generate lightweight static HTML (no interactive features) | `false` |
| `--verbose` | Show changed resource identifiers | `false` |
| `--full` | Show full field-level differences | `false` |
| `--include` | Comma-separated list of resource types to compare | All types |
| `--fail-on-diff` | Exit with code 1 if differences found | `false` |

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Identical, or differences found but `--fail-on-diff` not set |
| `1` | Differences found (when `--fail-on-diff` is set) |
| `2` | Error (auth failure, network issue, invalid input) |

## Resource Types

Full catalog compared by default:

| Resource | API Endpoint | Notes |
|----------|--------------|-------|
| Blueprints | `/blueprints` | Include permissions |
| Actions | `/actions` | Include permissions, filter self-service only |
| Scorecards | `/scorecards` | Per-blueprint |
| Pages | `/pages` | Include permissions |
| Integrations | `/integrations` | Include installation config |
| Webhooks | `/webhooks` | Subset of integrations |
| Teams | `/teams` | Team definitions |
| Users | `/users` | User list (email, role) |
| Automations | `/automations` | Automation rules |

Use `--include` to filter: `--include blueprints,actions,pages`

## Excluded Fields

Fields excluded from comparison by default (always differ between orgs or are cosmetic):

**Metadata fields:**
- `_id`
- `id`
- `orgId`
- `createdAt`
- `createdBy`
- `updatedAt`
- `updatedBy`

**Cosmetic fields:**
- `icon`
- `color`

## Output Formats

### Text Output (default)

**Summary only (default):**
```
Comparing staging -> production

Blueprints:    3 added, 2 modified, 1 removed
Actions:       0 added, 5 modified, 0 removed
Scorecards:    1 added, 0 modified, 0 removed
Pages:         0 added, 12 modified, 2 removed
Integrations:  0 added, 1 modified, 0 removed
Teams:         identical
Users:         2 added, 0 modified, 0 removed
Automations:   identical

Total: 6 added, 20 modified, 3 removed
```

**With `--verbose`:**
```
Comparing staging -> production

Blueprints: 3 added, 2 modified, 1 removed
  Added:    microservice, deployment, cluster
  Modified: service, environment
  Removed:  legacy_app

Actions: 0 added, 5 modified, 0 removed
  Modified: deploy_service, restart_pod, scale_deployment,
            create_ticket, notify_oncall
...
```

**With `--full`:**
```
Comparing staging -> production

Blueprints: 3 added, 2 modified, 1 removed

  [+] microservice (added)
      identifier: microservice
      title: Microservice
      schema:
        properties:
          language: { type: string }
          framework: { type: string }

  [~] service (modified)
      schema.properties.tier:
        - { type: string, enum: [critical, standard] }
        + { type: string, enum: [critical, standard, low] }

      relations.environment:
        - { target: env, required: false }
        + { target: environment, required: true }

  [-] legacy_app (removed)
...
```

### JSON Output

```json
{
  "source": "staging",
  "target": "production",
  "timestamp": "2026-02-05T19:30:00Z",
  "summary": {
    "total_added": 6,
    "total_modified": 20,
    "total_removed": 3,
    "identical": false
  },
  "differences": {
    "blueprints": {
      "added": [
        { "identifier": "microservice", "data": { ... } }
      ],
      "modified": [
        {
          "identifier": "service",
          "source": { ... },
          "target": { ... },
          "changes": [
            {
              "path": "schema.properties.tier.enum",
              "source_value": ["critical", "standard"],
              "target_value": ["critical", "standard", "low"]
            }
          ]
        }
      ],
      "removed": [
        { "identifier": "legacy_app", "data": { ... } }
      ]
    },
    "actions": { ... },
    "scorecards": { ... },
    ...
  }
}
```

### HTML Output

**Interactive (default):**
- Self-contained single HTML file with embedded CSS/JS
- Collapsible sections per resource type
- Side-by-side diff view for modified resources
- Search and filter functionality
- Color-coded additions (green), removals (red), modifications (yellow)
- Summary dashboard at top

**Simple (`--html-simple`):**
- Lightweight static HTML
- No JavaScript required
- Linear document format
- Suitable for email or printing

## Architecture

### File Structure

```
internal/
├── commands/
│   └── compare.go              # CLI command registration
└── modules/
    └── compare/
        ├── compare.go          # Main orchestration
        ├── fetcher.go          # Fetch data from orgs or files
        ├── differ.go           # Compute differences
        ├── output_text.go      # Text formatter
        ├── output_json.go      # JSON formatter
        ├── output_html.go      # HTML report generator
        └── templates/
            ├── report.html     # Interactive HTML template
            └── simple.html     # Simple HTML template
```

### Data Flow

```
┌─────────────┐     ┌─────────────┐
│   Source    │     │   Target    │
│  (org/file) │     │  (org/file) │
└──────┬──────┘     └──────┬──────┘
       │                   │
       ▼                   ▼
┌─────────────────────────────────┐
│           Fetcher               │
│  - Load from API or tar.gz     │
│  - Normalize data structure    │
└──────────────┬──────────────────┘
               │
               ▼
┌─────────────────────────────────┐
│           Differ                │
│  - Compare each resource type  │
│  - Identify add/modify/remove  │
│  - Compute field-level changes │
│  - Exclude metadata fields     │
└──────────────┬──────────────────┘
               │
               ▼
┌─────────────────────────────────┐
│         Formatter               │
│  - Text / JSON / HTML output   │
│  - Apply verbosity level       │
└──────────────┬──────────────────┘
               │
               ▼
┌─────────────────────────────────┐
│          Output                 │
│  - stdout (text/json)          │
│  - file (html)                 │
└─────────────────────────────────┘
```

### Key Components

**Fetcher (`fetcher.go`):**
- Detect input type (org name vs file path)
- For orgs: authenticate and fetch all resource types via API
- For files: extract and parse tar.gz export
- Return normalized `OrgData` struct

**Differ (`differ.go`):**
- Generic diff algorithm for any resource type
- Match resources by identifier
- Compute field-level changes using deep comparison
- Filter excluded fields before comparison
- Return `DiffResult` with added/modified/removed lists

**Formatters:**
- Implement common `Formatter` interface
- Text: colored terminal output with tiered verbosity
- JSON: structured output for programmatic use
- HTML: render Go templates with diff data

## Implementation Plan

### Phase 1: Core Infrastructure
1. Create `compare` module structure
2. Implement `Fetcher` for live org data
3. Implement `Fetcher` for export file parsing
4. Implement `Differ` with field exclusion

### Phase 2: Output Formats
5. Implement text output (all verbosity levels)
6. Implement JSON output
7. Create HTML templates (interactive + simple)
8. Implement HTML output generator

### Phase 3: CLI Integration
9. Register `compare` command with all flags
10. Wire up credential resolution (config + overrides)
11. Implement exit code logic
12. Add progress reporting for large orgs

### Phase 4: Testing & Polish
13. Unit tests for differ logic
14. Integration tests with real org data
15. Documentation and examples

## Testing Strategy

**Unit tests:**
- Differ correctly identifies additions, modifications, removals
- Field exclusion works for nested paths
- Fetcher handles both org and file inputs

**Integration tests:**
- Export org A, export org B, compare exports matches compare live
- HTML report renders correctly
- JSON output is valid and parseable

**Manual testing:**
- Compare production org to itself (should be identical)
- Compare staging to production (should show real differences)
- Verify HTML report in multiple browsers

## Success Criteria

- Compare two orgs with 100+ blueprints completes in < 30 seconds
- All output formats produce accurate, readable results
- Exit codes work correctly for CI/CD integration
- HTML report is self-contained and works offline
