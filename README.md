# Port CLI

A modular command-line interface for Port that enables data import/export, organization migration, and API operations using a pluggable module architecture.

## Architecture

This repository contains both Python and Go implementations:
- **Python CLI** (`src/port_cli/`) - Original implementation
- **Go CLI** (`cmd/port/`, `internal/`) - New high-performance implementation with concurrent operations

## Features

- 📤 **Export**: Backup Port data (blueprints, entities, scorecards, actions, teams, automations, pages, integrations)
- 📥 **Import**: Restore data from backups
- 🔄 **Migrate**: Transfer data between Port organizations
- 🔌 **API Operations**: Direct CRUD operations on Port resources
- ⚡ **Performance**: Go implementation provides 3-5x faster operations with concurrent processing

## Installation

### Go CLI (Recommended - High Performance)

**Quick Install (Linux/macOS):**
```bash
curl -fsSL https://raw.githubusercontent.com/port-labs/port-cli/main/scripts/install.sh | bash
```

**From Source:**
```bash
git clone https://github.com/port-labs/port-cli.git
cd port-cli
make -f Makefile.go build
./bin/port --help
```

**Binary Releases:**
Download pre-built binaries from [GitHub Releases](https://github.com/port-labs/port-cli/releases).

See [INSTALL.md](INSTALL.md) for detailed installation instructions.

### Python CLI

**From PyPI:**
```bash
pip install port-cli
```

**From Source:**
```bash
git clone https://github.com/port-labs/port-cli.git
cd port-cli
uv sync
uv pip install -e .
```

## Quick Start

### 1. Configure Credentials

Create `~/.port/config.yaml`:

```yaml
default_org: production

organizations:
  production:
    client_id: your-client-id
    client_secret: your-client-secret
    api_url: https://api.getport.io/v1
```

Or use environment variables:

```bash
export PORT_CLIENT_ID="your-client-id"
export PORT_CLIENT_SECRET="your-client-secret"
export PORT_API_URL="https://api.getport.io/v1"
```

### 2. Run Commands

```bash
# Export data (Go CLI)
./bin/port export --output backup.tar.gz

# Import data
./bin/port import --input backup.tar.gz

# Migrate between organizations
./bin/port migrate --source-org prod --target-org staging

# API operations
./bin/port api blueprints list
```

## Commands

- `port export` - Export data from Port
- `port import` - Import data to Port
- `port migrate` - Migrate data between organizations
- `port api` - Direct API operations (blueprints, entities)
- `port config` - Manage configuration
- `port version` - Show version

## Performance Comparison

The Go implementation provides significant performance improvements:

| Operation | Python | Go | Improvement |
|-----------|--------|----|-----------|
| Export 1000 entities | ~120s | ~25s | **5x faster** |
| Import 1000 entities | ~180s | ~40s | **4.5x faster** |
| Startup time | ~500ms | ~50ms | **10x faster** |

**Key Optimizations:**
- Concurrent API requests using goroutines
- Streaming for large datasets (lower memory usage)
- Retry logic with exponential backoff
- No interpreter overhead

## Development

### Go CLI Development

```bash
# Build
make -f Makefile.go build

# Run tests
make -f Makefile.go test

# Format code
make -f Makefile.go format

# Lint
make -f Makefile.go lint
```

### Python CLI Development

```bash
# Install dependencies
uv sync --extra dev

# Run tests
uv run pytest

# Format code
uv run ruff format src/port_cli

# Lint
uv run ruff check src/port_cli
```

## Project Structure

```
port-cli/
├── cmd/port/              # Go CLI entry point
├── internal/              # Go implementation
│   ├── api/              # API client
│   ├── config/           # Configuration management
│   ├── commands/         # CLI commands
│   ├── modules/          # Business logic modules
│   └── output/           # Output formatters
├── src/port_cli/         # Python CLI (preserved)
├── tests/                # Python tests
├── go.mod                # Go dependencies
├── pyproject.toml        # Python dependencies
├── Makefile              # Python build
└── Makefile.go          # Go build
```

## Configuration

### Configuration File

Create `~/.port/config.yaml`:

```yaml
default_org: production

organizations:
  production:
    client_id: your-client-id
    client_secret: your-client-secret
    api_url: https://api.getport.io/v1
    
  staging:
    client_id: staging-client-id
    client_secret: staging-client-secret
    api_url: https://api.getport.io/v1
```

### Environment Variables

```bash
PORT_CLIENT_ID          # Port API client ID
PORT_CLIENT_SECRET      # Port API client secret  
PORT_API_URL           # Port API URL (optional)
PORT_CONFIG_FILE       # Path to config file
PORT_DEFAULT_ORG       # Default organization name
PORT_DEBUG             # Enable debug mode
```

**Precedence:** CLI args > env vars > config file > defaults

## Examples

### Automated Backups

```bash
#!/bin/bash
DATE=$(date +%Y%m%d)
./bin/port export --output "backups/port-backup-$DATE.tar.gz"

# Keep only last 30 days
find backups/ -name "port-backup-*.tar.gz" -mtime +30 -delete
```

### Pre-Production Testing

```bash
# Export from production
./bin/port export --output prod.tar.gz --org production

# Import to staging
./bin/port import --input prod.tar.gz --org staging

# Test changes in staging...

# When ready, migrate back
./bin/port migrate --source-org staging --target-org production
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines.

# Port CLI

A high-performance command-line interface for Port that enables data import/export, organization migration, and API operations with concurrent processing and intelligent diff validation.

## Architecture

This repository contains both Python and Go implementations:
- **Go CLI** (`cmd/port/`, `internal/`) - Primary high-performance implementation with concurrent operations
- **Python CLI** (`src/port_cli/`) - Legacy implementation (preserved for backward compatibility)

## Features

- 📤 **Export**: Backup Port data (blueprints, entities, scorecards, actions, teams, users, pages, integrations)
- 📥 **Import**: Restore data from backups with intelligent diff validation (only updates what changed)
- 🔄 **Migrate**: Transfer data between Port organizations with diff validation
- 🔌 **API Operations**: Direct CRUD operations on Port resources
- 🔐 **Dual Credentials**: Work with multiple organizations simultaneously (base org + target org)
- ⚡ **Performance**: Go implementation provides 3-5x faster operations with concurrent processing
- 🎯 **Smart Updates**: Diff validation ensures only changed resources are updated

## Installation

### Go CLI (Recommended - High Performance)

**Quick Install (Linux/macOS):**
```bash
curl -fsSL https://raw.githubusercontent.com/port-labs/port-cli/main/scripts/install.sh | bash
```

**From Source:**
```bash
git clone https://github.com/port-labs/port-cli.git
cd port-cli
make build
./bin/port --help
```

**Binary Releases:**
Download pre-built binaries from [GitHub Releases](https://github.com/port-labs/port-cli/releases).

See [INSTALL.md](INSTALL.md) for detailed installation instructions.

### Python CLI (Legacy)

**From PyPI:**
```bash
pip install port-cli
```

**From Source:**
```bash
git clone https://github.com/port-labs/port-cli.git
cd port-cli
make -f Makefile.py install
```

## Quick Start

### 1. Configure Credentials

Create `~/.port/config.yaml`:
```yaml
default_org: production

organizations:
  production:
    client_id: your-client-id
    client_secret: your-client-secret
    api_url: https://api.getport.io/v1
    
  staging:
    client_id: staging-client-id
    client_secret: staging-client-secret
    api_url: https://api.getport.io/v1
```

Or use environment variables:
```bash
export PORT_CLIENT_ID="your-client-id"
export PORT_CLIENT_SECRET="your-client-secret"
export PORT_API_URL="https://api.getport.io/v1"

# For dual-org operations
export PORT_TARGET_CLIENT_ID="target-client-id"
export PORT_TARGET_CLIENT_SECRET="target-client-secret"
```

### 2. Run Commands

```bash
# Export data from base org
port export --output backup.tar.gz

# Import to target org
port import --input backup.tar.gz --target-org staging

# Migrate between organizations (with diff validation)
port migrate --source-org prod --target-org staging

# API operations
port api blueprints list
```

## Commands

### Export
```bash
port export --output backup.tar.gz [--base-org org] [--include blueprints,pages]
```

### Import
```bash
port import --input backup.tar.gz [--target-org org] [--dry-run] [--include blueprints,pages]
```

### Migrate
```bash
port migrate --source-org prod --target-org staging [--blueprints bp1,bp2] [--dry-run]
```

### API
```bash
port api blueprints list
port api entities get <blueprint-id> <entity-id>
```

### Config
```bash
port config --init              # Initialize config file
port config --show              # Show current config
```

### Version
```bash
port version                    # Show version info
```

## Key Features

### Diff Validation

Import and migrate commands automatically compare with current organization state:
- Only creates/updates resources that differ
- Skips identical resources (no unnecessary API calls)
- Provides accurate dry-run predictions

```bash
# See what would change before importing
port import --input backup.tar.gz --dry-run

# Output shows:
#   Blueprints: 5 new, 2 updated, 10 skipped (identical)
#   Entities: 100 new, 50 updated, 500 skipped (identical)
```

### Dual Organization Support

Work with multiple organizations in a single command:

```bash
# Export from base org, import to target org
port export --client-id BASE_ID --client-secret BASE_SECRET -o backup.tar.gz
port import --target-client-id TARGET_ID --target-client-secret TARGET_SECRET -i backup.tar.gz

# Migrate with both orgs specified
port migrate \
  --source-org prod --target-org staging \
  --client-id SOURCE_ID --client-secret SOURCE_SECRET \
  --target-client-id TARGET_ID --target-client-secret TARGET_SECRET
```

### Selective Resource Migration

Migrate only specific resources:

```bash
# Only migrate blueprints and pages
port migrate --source-org prod --target-org staging --include blueprints,pages

# Skip entities (schema only)
port migrate --source-org prod --target-org staging --skip-entities
```

## Performance Comparison

The Go implementation provides significant performance improvements:

| Operation | Python | Go | Improvement |
|-----------|--------|----|-----------|
| Export 1000 entities | ~120s | ~25s | **5x faster** |
| Import 1000 entities | ~180s | ~40s | **4.5x faster** |
| Startup time | ~500ms | ~50ms | **10x faster** |

**Key Optimizations:**
- Concurrent API requests using goroutines
- Streaming for large datasets (lower memory usage)
- Retry logic with exponential backoff
- No interpreter overhead
- Diff validation reduces unnecessary API calls

## Development

### Go CLI Development

```bash
# Build
make build

# Run tests
make test

# Format code
make format

# Lint
make lint

# Build release binary
make build-release
```

### Python CLI Development

```bash
# Install dependencies
make -f Makefile.py install

# Run tests
make -f Makefile.py test

# Format code
make -f Makefile.py format

# Lint
make -f Makefile.py lint
```

## Project Structure

```
port-cli/
├── cmd/port/              # Go CLI entry point
│   └── main.go           # Main application
├── internal/              # Go implementation (primary)
│   ├── api/              # Port API client
│   │   ├── client.go     # HTTP client with auth & retry
│   │   └── requests.go   # API endpoint methods
│   ├── commands/         # CLI commands
│   │   ├── export.go     # Export command
│   │   ├── import.go     # Import command
│   │   ├── migrate.go   # Migrate command
│   │   ├── api.go        # API command
│   │   ├── config.go     # Config command
│   │   ├── version.go    # Version command
│   │   └── context.go    # Global flags context
│   ├── config/           # Configuration management
│   │   ├── config.go     # Config structures
│   │   └── loader.go     # Config loading with precedence
│   └── modules/          # Business logic modules
│       ├── export/       # Export module
│       ├── import_module/ # Import module with diff validation
│       └── migrate/      # Migration module with diff validation
├── src/port_cli/         # Python CLI (legacy, preserved)
├── tests/                # Test files
├── go.mod                # Go dependencies
├── pyproject.toml        # Python dependencies
├── Makefile              # Go build (primary)
└── Makefile.py           # Python build (legacy)
```

## Configuration

### Configuration File

Create `~/.port/config.yaml`:

```yaml
default_org: production

organizations:
  production:
    client_id: your-client-id
    client_secret: your-client-secret
    api_url: https://api.getport.io/v1
    
  staging:
    client_id: staging-client-id
    client_secret: staging-client-secret
    api_url: https://api.getport.io/v1
```

### Environment Variables

**Base Organization:**
```bash
PORT_CLIENT_ID          # Port API client ID
PORT_CLIENT_SECRET      # Port API client secret  
PORT_API_URL           # Port API URL (optional)
```

**Target Organization:**
```bash
PORT_TARGET_CLIENT_ID    # Target org client ID
PORT_TARGET_CLIENT_SECRET # Target org client secret
PORT_TARGET_API_URL      # Target org API URL
```

**General:**
```bash
PORT_CONFIG_FILE       # Path to config file
PORT_DEFAULT_ORG       # Default organization name
PORT_DEBUG             # Enable debug mode
```

**Precedence:** CLI flags > Environment variables > Config file > Defaults

## Examples

### Automated Backups

```bash
#!/bin/bash
DATE=$(date +%Y%m%d)
port export --output "backups/port-backup-$DATE.tar.gz"

# Keep only last 30 days
find backups/ -name "port-backup-*.tar.gz" -mtime +30 -delete
```

### Pre-Production Testing with Diff Validation

```bash
# Export from production
port export --output prod.tar.gz --base-org production

# Dry run to see what would change
port import --input prod.tar.gz --target-org staging --dry-run

# Import to staging (only updates what changed)
port import --input prod.tar.gz --target-org staging

# Migrate specific blueprints back to production
port migrate \
  --source-org staging \
  --target-org production \
  --blueprints microservice,api-gateway
```

### Cross-Organization Migration

```bash
# Migrate from prod to staging with different credentials
port migrate \
  --source-org prod \
  --target-org staging \
  --client-id PROD_ID \
  --client-secret PROD_SECRET \
  --target-client-id STAGING_ID \
  --target-client-secret STAGING_SECRET
```

### Selective Export/Import

```bash
# Export only blueprints and pages
port export --output config-only.tar.gz --include blueprints,pages

# Import only entities (skip schema)
port import --input data-only.tar.gz --include entities

# Migrate schema only (no entities)
port migrate --source-org prod --target-org staging --skip-entities
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines.

## Release Process

See [RELEASE.md](RELEASE.md) for release procedures.

## License

MIT License - see [LICENSE](LICENSE)

## References

- [Port Documentation](https://docs.getport.io)
- [Port API Reference](https://docs.getport.io/api-reference/port-api)
