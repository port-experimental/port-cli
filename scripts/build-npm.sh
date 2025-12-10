#!/bin/bash
# Build script for npm package
# Prepares npm package with Go binaries for all platforms

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
NPM_DIR="$PROJECT_ROOT/npm"
DIST_DIR="$PROJECT_ROOT/dist"

echo "Building npm package for Port CLI..."

# Get version from git tag or environment variable
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "0.0.0")}"
# Remove 'v' prefix if present
VERSION="${VERSION#v}"

echo "Version: $VERSION"

# Create binaries directory
BINARIES_DIR="$NPM_DIR/binaries"
mkdir -p "$BINARIES_DIR"

# Check if dist directory exists (from GoReleaser)
if [ -d "$DIST_DIR" ] && [ "$(ls -A $DIST_DIR/*.tar.gz $DIST_DIR/*.zip 2>/dev/null)" ]; then
    echo "Extracting binaries from GoReleaser dist output..."
    
    # Extract binaries from archives
    for archive in "$DIST_DIR"/*.tar.gz "$DIST_DIR"/*.zip; do
        if [ -f "$archive" ]; then
            archive_name=$(basename "$archive")
            echo "Processing $archive_name..."
            
            # Extract binary name from archive
            if [[ "$archive_name" == *"windows"* ]]; then
                # Windows zip file
                temp_dir=$(mktemp -d)
                unzip -q "$archive" -d "$temp_dir"
                # Find the .exe file
                exe_file=$(find "$temp_dir" -name "*.exe" -type f | head -1)
                if [ -n "$exe_file" ]; then
                    # Extract platform info from archive name: port-cli_VERSION_OS_ARCH.zip
                    # Format: port-cli_0.1.1_windows_amd64.zip
                    if [[ "$archive_name" =~ port-cli_[0-9.]+_(.+)_(.+)\.zip ]]; then
                        os="${BASH_REMATCH[1]}"
                        arch="${BASH_REMATCH[2]}"
                        target_name="port-${os}-${arch}.exe"
                        cp "$exe_file" "$BINARIES_DIR/$target_name"
                        chmod +x "$BINARIES_DIR/$target_name"
                        echo "  Extracted: $target_name"
                    fi
                fi
                rm -rf "$temp_dir"
            else
                # Unix tar.gz file
                temp_dir=$(mktemp -d)
                tar -xzf "$archive" -C "$temp_dir"
                # Find the binary (should be named 'port')
                binary_file=$(find "$temp_dir" -name "port" -type f | head -1)
                if [ -n "$binary_file" ]; then
                    # Extract platform info from archive name: port-cli_VERSION_OS_ARCH.tar.gz
                    # Format: port-cli_0.1.1_linux_amd64.tar.gz
                    if [[ "$archive_name" =~ port-cli_[0-9.]+_(.+)_(.+)\.tar\.gz ]]; then
                        os="${BASH_REMATCH[1]}"
                        arch="${BASH_REMATCH[2]}"
                        target_name="port-${os}-${arch}"
                        cp "$binary_file" "$BINARIES_DIR/$target_name"
                        chmod +x "$BINARIES_DIR/$target_name"
                        echo "  Extracted: $target_name"
                    fi
                fi
                rm -rf "$temp_dir"
            fi
        fi
    done
else
    echo "Dist directory not found or empty. Building binaries from source..."
    
    # Build binaries for all platforms
    cd "$PROJECT_ROOT"
    
    # Get build flags from Makefile or use defaults
    VERSION_TAG="v${VERSION}"
    BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    
    LDFLAGS="-s -w -X 'main.version=${VERSION_TAG}' -X 'main.buildDate=${BUILD_DATE}' -X 'main.commit=${COMMIT}'"
    
    echo "Building for linux/amd64..."
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "${LDFLAGS}" -buildmode=pie -o "$BINARIES_DIR/port-linux-amd64" ./cmd/port
    
    echo "Building for linux/arm64..."
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "${LDFLAGS}" -buildmode=pie -o "$BINARIES_DIR/port-linux-arm64" ./cmd/port
    
    echo "Building for darwin/amd64..."
    CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "${LDFLAGS}" -buildmode=pie -o "$BINARIES_DIR/port-darwin-amd64" ./cmd/port
    
    echo "Building for darwin/arm64..."
    CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "${LDFLAGS}" -buildmode=pie -o "$BINARIES_DIR/port-darwin-arm64" ./cmd/port
    
    echo "Building for windows/amd64..."
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "${LDFLAGS}" -buildmode=pie -o "$BINARIES_DIR/port-windows-amd64.exe" ./cmd/port
    
    echo "Building for windows/arm64..."
    CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -trimpath -ldflags "${LDFLAGS}" -buildmode=pie -o "$BINARIES_DIR/port-windows-arm64.exe" ./cmd/port
    
    chmod +x "$BINARIES_DIR"/port-*
fi

# Verify binaries were created
BINARY_COUNT=$(find "$BINARIES_DIR" -type f \( -name "port-*" -o -name "port-*.exe" \) | wc -l | tr -d ' ')
if [ "$BINARY_COUNT" -eq 0 ]; then
    echo "Error: No binaries found in $BINARIES_DIR"
    exit 1
fi

echo "Found $BINARY_COUNT binaries:"
ls -lh "$BINARIES_DIR"

# Update package.json version
echo "Updating package.json version to $VERSION..."
if command -v node >/dev/null 2>&1; then
    node -e "
    const fs = require('fs');
    const path = require('path');
    const pkgPath = path.join('$NPM_DIR', 'package.json');
    const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf8'));
    pkg.version = '$VERSION';
    fs.writeFileSync(pkgPath, JSON.stringify(pkg, null, 2) + '\n');
    "
else
    # Fallback to sed if node is not available
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "s/\"version\": \".*\"/\"version\": \"$VERSION\"/" "$NPM_DIR/package.json"
    else
        sed -i "s/\"version\": \".*\"/\"version\": \"$VERSION\"/" "$NPM_DIR/package.json"
    fi
fi

# Copy LICENSE and README to npm directory
if [ -f "$PROJECT_ROOT/LICENSE" ]; then
    cp "$PROJECT_ROOT/LICENSE" "$NPM_DIR/LICENSE"
fi

if [ -f "$PROJECT_ROOT/README.md" ]; then
    # We already have npm/README.md, but we could update it if needed
    echo "README.md already exists in npm directory"
fi

echo ""
echo "✓ npm package prepared successfully!"
echo "  Version: $VERSION"
echo "  Binaries: $BINARY_COUNT"
echo "  Location: $NPM_DIR"
echo ""
echo "To test locally:"
echo "  cd $NPM_DIR"
echo "  npm pack"
echo "  npm install -g port-cli-*.tgz"
