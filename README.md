# Port CLI

A modular command-line interface for Port that enables data import/export, organization migration, and API operations using a pluggable module architecture.

## Features

- 📤 **Export**: Backup Port data (blueprints, entities, scorecards, actions, teams, automations, pages, integrations)
- 📥 **Import**: Restore data from backups
- 🔄 **Migrate**: Transfer data between Port organizations
- 🔍 **Compare**: Diff two Port organizations and generate reports (text, JSON, HTML)
- 🔌 **API Operations**: Direct CRUD operations on Port resources
- 🤖 **Plugin**: Sync AI skills from Port into your local AI coding tools (Cursor, Claude Code, Gemini CLI, OpenAI Codex, Windsurf, Agents, GitHub Copilot)

## Installation

### Through npm

**Global installation:**
```bash
npm install -g @port-experimental/port-cli
```

**Use with npx (no installation needed):**
```bash
npx @port-experimental/port-cli --version
```

**Local installation in your project:**
```bash
npm install @port-experimental/port-cli
```

### Quick Install Script

**Linux/macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/port-experimental/port-cli/main/scripts/install.sh | bash
```

This will download and install the latest release binary to `/usr/local/bin` (or `~/.local/bin` if you don't have write permissions).

**Verify installation:**
```bash
port --version
```

### Binary Releases

Download pre-built binaries for your platform from [GitHub Releases](https://github.com/port-experimental/port-cli/releases).

### Docker

**Build the image:**
```bash
docker build -t port-cli .
```

**Run a command:**
```bash
docker run --rm \
  -e PORT_CLIENT_ID="your-client-id" \
  -e PORT_CLIENT_SECRET="your-client-secret" \
  port-cli --help
```

**Export with output written to the host:**
```bash
docker run --rm \
  -e PORT_CLIENT_ID="your-client-id" \
  -e PORT_CLIENT_SECRET="your-client-secret" \
  -v $(pwd)/output:/data \
  port-cli export --output /data/backup.tar.gz
```

### Build from Source

For development or if you need the latest unreleased code:

```bash
git clone https://github.com/port-experimental/port-cli.git
cd port-cli
make build
./bin/port --help
```

**Note:** When building from source, use `./bin/port` instead of `port` in commands. For installed binaries, use `port` directly.

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
# Export data
port export --output backup.tar.gz

# Import data
port import --input backup.tar.gz

# Compare organizations
port compare --source staging --target production

# Migrate between organizations
port migrate --source-org prod --target-org staging

# API operations
port api blueprints list

# Install AI skill hooks (one-time setup)
port plugin init
```

**Note:** If you built from source instead of installing, use `./bin/port` instead of `port` in the commands above.

## Commands

- `port export` - Export data from Port
- `port import` - Import data to Port
- `port compare` - Compare two Port organizations
- `port migrate` - Migrate data between organizations
- `port api` - Direct API operations (blueprints, entities)
- `port plugin` - Manage Port AI skill hooks and local skill sync
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

### Compare Organizations

By default, `port compare` compares **all** resource types (blueprints, actions, scorecards, pages, integrations, teams, users). Use `--include` to narrow the comparison to specific types.

```bash
# Compare two configured organizations (all resource types)
port compare --source staging --target production

# Compare with verbose output (show identifiers)
port compare --source staging --target production --verbose

# Compare with full field-level diff
port compare --source staging --target production --full

# Compare only pages
port compare --source staging --target production --include pages

# Compare pages and blueprints together
port compare --source staging --target production --include pages,blueprints

# Compare export files
port compare --source ./staging-backup.tar.gz --target ./prod-backup.tar.gz

# Compare only pages between export files
port compare --source ./staging-backup.tar.gz --target ./prod-backup.tar.gz --include pages

# Output as JSON (for scripting)
port compare --source staging --target production --output json

# Generate interactive HTML report
port compare --source staging --target production --output html --html-file report.html

# CI/CD mode: exit code 1 if differences found
port compare --source staging --target production --fail-on-diff

# CI/CD mode scoped to pages only
port compare --source staging --target production --include pages --fail-on-diff
```

Valid `--include` values: `blueprints`, `actions`, `scorecards`, `pages`, `integrations`, `teams`, `users`.

### Pre-Production Testing

```bash
# Export from production
./bin/port export --output prod.tar.gz --org production

# Import to staging
./bin/port import --input prod.tar.gz --org staging

# Compare to verify changes
./bin/port compare --source prod.tar.gz --target staging --verbose

# Test changes in staging...

# When ready, migrate back
./bin/port migrate --source-org staging --target-org production
```

### Docker

```bash
# Export to a local directory
docker run --rm \
  -e PORT_CLIENT_ID="your-client-id" \
  -e PORT_CLIENT_SECRET="your-client-secret" \
  -v $(pwd)/output:/data \
  port-cli export --output /data/backup.tar.gz

# Import from a local file
docker run --rm \
  -e PORT_CLIENT_ID="your-client-id" \
  -e PORT_CLIENT_SECRET="your-client-secret" \
  -v $(pwd)/output:/data \
  port-cli import --input /data/backup.tar.gz

# Compare two organizations
docker run --rm \
  -e PORT_CLIENT_ID="source-client-id" \
  -e PORT_CLIENT_SECRET="source-client-secret" \
  -e PORT_TARGET_CLIENT_ID="target-client-id" \
  -e PORT_TARGET_CLIENT_SECRET="target-client-secret" \
  port-cli compare --fail-on-diff
```

### AI Skill Hooks (Plugin)

Automatically sync skills from your Port organization into local AI coding tools (Cursor, Claude Code, Gemini CLI, OpenAI Codex, Windsurf, Agents, GitHub Copilot).

```bash
# One-time setup: install session-start hooks and select skills
port plugin init

# Manually sync skills (also runs automatically on every new AI session)
port plugin sync

# Check what's configured
port plugin status

# Remove all locally synced skill files (hooks remain; skills re-sync on next session)
port plugin clear

# Fully uninstall the plugin (hooks, skills, and config)
port plugin remove
```

See [docs/plugin-setup.md](docs/plugin-setup.md) for full setup instructions.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines.

## Release Process

See [RELEASE.md](RELEASE.md) for release procedures.

## License

MIT License - see [LICENSE](LICENSE)

## References

- [Port Documentation](https://docs.getport.io)
- [Port API Reference](https://docs.getport.io/api-reference/port-api)
