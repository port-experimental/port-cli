# Generated OpenAPI Client Code

This directory contains code generated from the Port API OpenAPI specification.

## Generation

To regenerate this code, run:

```bash
make generate-api
```

Or directly:

```bash
./scripts/generate-api.sh
```

## Files

- `types.go` - All OpenAPI schema types (Blueprint, Entity, etc.)
- `client.go` - Generated HTTP client with methods for each endpoint

## Usage

The generated client is wrapped by `internal/api/client.go` to provide:
- Token management and authentication
- Retry logic
- Backward-compatible interface

You should not use the generated client directly. Use `api.NewClient()` instead.

## Updating

When the OpenAPI spec changes:
1. Run `make generate-api` to regenerate
2. Review changes in generated code
3. Update adapter in `client.go` if needed
4. Run tests to ensure compatibility

