#!/bin/bash
set -e

# Script to download OpenAPI spec and generate Go client code
# Usage: ./scripts/generate-api.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
OPENAPI_URL="https://raw.githubusercontent.com/port-experimental/port-js/main/openapi.json"
OPENAPI_SPEC_DIR="$PROJECT_ROOT/api-specs"
OPENAPI_SPEC_FILE="$OPENAPI_SPEC_DIR/openapi.json"
GENERATED_DIR="$PROJECT_ROOT/internal/api/generated"

echo "🔧 Generating OpenAPI client code..."
echo ""

# Create directories
mkdir -p "$OPENAPI_SPEC_DIR"
mkdir -p "$GENERATED_DIR"

# Download OpenAPI spec
echo "📥 Downloading OpenAPI spec from $OPENAPI_URL..."
if command -v curl >/dev/null 2>&1; then
    curl -sSL -o "$OPENAPI_SPEC_FILE" "$OPENAPI_URL"
elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$OPENAPI_SPEC_FILE" "$OPENAPI_URL"
else
    echo "❌ Error: Neither curl nor wget found. Please install one of them."
    exit 1
fi

if [ ! -f "$OPENAPI_SPEC_FILE" ] || [ ! -s "$OPENAPI_SPEC_FILE" ]; then
    echo "❌ Error: Failed to download OpenAPI spec"
    exit 1
fi

echo "✅ OpenAPI spec downloaded to $OPENAPI_SPEC_FILE"
echo ""

# Check if oapi-codegen is installed
if ! command -v oapi-codegen >/dev/null 2>&1; then
    echo "📦 Installing oapi-codegen..."
    go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
    # Add GOPATH/bin to PATH if not already there
    export PATH="$PATH:$(go env GOPATH)/bin"
fi

# Ensure oapi-codegen is in PATH
if ! command -v oapi-codegen >/dev/null 2>&1; then
    export PATH="$PATH:$(go env GOPATH)/bin"
fi

# Generate types
echo "🔨 Generating Go types..."
oapi-codegen \
    -generate types \
    -package generated \
    -o "$GENERATED_DIR/types.go" \
    "$OPENAPI_SPEC_FILE"

if [ $? -ne 0 ]; then
    echo "❌ Error: Failed to generate types"
    exit 1
fi

echo "✅ Types generated: $GENERATED_DIR/types.go"
echo ""

# Generate client
echo "🔨 Generating Go client..."
oapi-codegen \
    -generate client \
    -package generated \
    -o "$GENERATED_DIR/client.go" \
    "$OPENAPI_SPEC_FILE"

if [ $? -ne 0 ]; then
    echo "❌ Error: Failed to generate client"
    exit 1
fi

echo "✅ Client generated: $GENERATED_DIR/client.go"
echo ""

echo "✨ OpenAPI code generation complete!"
echo ""
echo "Generated files:"
echo "  - $GENERATED_DIR/types.go"
echo "  - $GENERATED_DIR/client.go"
echo ""
echo "Next steps:"
echo "  1. Review the generated code"
echo "  2. The adapter layer in internal/api/client.go will use these generated types"
echo "  3. Run 'go mod tidy' to update dependencies"

