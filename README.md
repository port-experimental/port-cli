# Port CLI

A modular command-line interface for Port that enables data import/export, organization migration, and API operations using a pluggable module architecture.

## Features

- 📤 **Export**: Backup Port data (blueprints, entities, scorecards, actions, teams, automations, pages, integrations)
- 📥 **Import**: Restore data from backups
- 🔄 **Migrate**: Transfer data between Port organizations
- 🔌 **API Operations**: Direct CRUD operations on Port resources

## Installation

### Go CLI 

**Quick Install (Linux/macOS):**
```bash
curl -fsSL https://raw.githubusercontent.com/port-experimental/port-cli/main/scripts/install.sh | bash
```

**From Source:**
```bash
git clone https://github.com/port-experimental/port-cli.git
cd port-cli
make build
./bin/port --help
```

**Binary Releases:**
Download pre-built binaries from [GitHub Releases](https://github.com/port-experimental/port-cli/releases).

See [INSTALL.md](INSTALL.md) for detailed installation instructions.


## Quick Start

### 1. Configure Credentials

Run `port config --init` to create a configuration file at `~/.port/config.yaml`:

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
├── go.mod                # Go dependencies
└── Makefile              # Go build
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

## Release Process

See [RELEASE.md](RELEASE.md) for release procedures.

## License

MIT License - see [LICENSE](LICENSE)

## References

- [Port Documentation](https://docs.getport.io)
- [Port API Reference](https://docs.getport.io/api-reference/port-api)
