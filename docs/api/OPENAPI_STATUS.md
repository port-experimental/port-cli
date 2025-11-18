# OpenAPI Integration Status

## Current Status

✅ **CLI Commands Working**: All API commands are fully functional using the current implementation.

⚠️ **OpenAPI Generation**: Currently blocked due to OpenAPI 3.1.0 compatibility issues with `oapi-codegen`.

## Implementation Progress

### ✅ Completed
- [x] Created `scripts/generate-api.sh` - Downloads spec and generates code
- [x] Added `make generate-api` target to Makefile
- [x] Updated `.gitignore` for generated files
- [x] Created integration documentation

### ⏳ Pending
- [ ] Generate OpenAPI client code (blocked by 3.1.0 compatibility)
- [ ] Integrate generated client into adapter layer
- [ ] Update methods to use generated types

## What's Working

The CLI API commands work perfectly with the current implementation:

```bash
# All these commands work:
port api blueprints list
port api blueprints get <id>
port api blueprints create --data <file>
port api entities list
port api entities get <blueprint> <entity>
# ... and more
```

See [CLI_API_COMMANDS.md](CLI_API_COMMANDS.md) for complete command reference.

## OpenAPI Generation Issue

### Problem
The Port OpenAPI spec is version 3.1.0, but `oapi-codegen` has limited support for 3.1.0 features:
- Null types in `oneOf` schemas
- Some advanced schema features

### Error
```
error generating Go schema for property 'invocationMethod': 
error generating type for oneOf: 
error generating Go schema for property 'mapping': 
error generating type for additional properties: 
error resolving primitive type: unhandled Schema type: &[null]
```

### Solutions

#### Option 1: Wait for oapi-codegen 3.1.0 Support
- Track: https://github.com/oapi-codegen/oapi-codegen/issues/373
- When available, run `make generate-api`

#### Option 2: Convert Spec to 3.0.0
- Create a script to downgrade the spec
- Remove/nullify problematic 3.1.0 features
- Generate from converted spec

#### Option 3: Use Alternative Generator
- Try `openapi-generator` (supports 3.1.0)
- Or `go-swagger` (different approach)

#### Option 4: Manual Type Generation
- Generate types for specific endpoints we need
- Use `-include-tags` or `-include-paths` flags
- Gradually expand coverage

## Current Implementation

The current implementation:
- ✅ Works perfectly for all existing commands
- ✅ Has proper authentication and token management
- ✅ Includes retry logic and error handling
- ✅ Supports all current API endpoints
- ✅ Backward compatible interface

## Integration Path (When Ready)

Once OpenAPI generation works:

1. **Generate Code**:
   ```bash
   make generate-api
   ```

2. **Follow Integration Guide**:
   - See `internal/api/INTEGRATION_GUIDE.md`
   - Update `client.go` to wrap generated client
   - Update `requests.go` methods

3. **Benefits**:
   - Type safety
   - All endpoints available
   - Auto-validation
   - Easy expansion

## Setup Guide (When Ready)

### Prerequisites
1. Go 1.21 or later
2. `oapi-codegen` tool (will be installed automatically)

### Steps to Generate Code

1. **Install oapi-codegen**:
   ```bash
   go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
   ```

2. **Generate API Client Code**:
   ```bash
   make generate-api
   ```
   This downloads the OpenAPI spec and generates:
   - `internal/api/generated/types.go` - All API types
   - `internal/api/generated/client.go` - HTTP client

3. **Update Dependencies**:
   ```bash
   go mod tidy
   ```

4. **Follow Integration Guide**:
   See `internal/api/INTEGRATION_GUIDE.md` for detailed integration steps.

## Recommendation

**For Now**: Continue using the current implementation. It's stable and works well.

**Future**: When oapi-codegen supports 3.1.0 or we convert the spec, integrate the generated client for:
- Better type safety
- Easier addition of new endpoints
- Automatic validation

## Files Ready for Integration

All infrastructure is in place:
- ✅ Generation script (`scripts/generate-api.sh`)
- ✅ Makefile target (`make generate-api`)
- ✅ Integration guide (`internal/api/INTEGRATION_GUIDE.md`)
- ✅ Documentation structure

Just waiting for compatible code generation.

