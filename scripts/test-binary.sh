#!/bin/bash
# Test script for standalone binary

set -e

BINARY="${1:-bin/port}"

if [ ! -f "$BINARY" ]; then
    echo "Error: Binary not found at $BINARY"
    echo "Usage: $0 [path-to-binary]"
    exit 1
fi

echo "Testing Port CLI binary: $BINARY"
echo ""

# Test 1: Version command
echo "Test 1: Version command"
if "$BINARY" version > /dev/null 2>&1; then
    echo "✓ Version command works"
    "$BINARY" version
else
    echo "✗ Version command failed"
    exit 1
fi
echo ""

# Test 2: Help command
echo "Test 2: Help command"
if "$BINARY" --help > /dev/null 2>&1; then
    echo "✓ Help command works"
else
    echo "✗ Help command failed"
    exit 1
fi
echo ""

# Test 3: Config command (should work without credentials)
echo "Test 3: Config command"
if "$BINARY" config --show > /dev/null 2>&1; then
    echo "✓ Config command works"
else
    echo "✗ Config command failed"
    exit 1
fi
echo ""

# Test 4: Export command (should show error about missing credentials)
echo "Test 4: Export command (should fail with auth error)"
if "$BINARY" export --output test.tar.gz 2>&1 | grep -q "missing\|credentials\|authentication"; then
    echo "✓ Export command correctly shows authentication error"
else
    echo "✗ Export command didn't show expected error"
    exit 1
fi
echo ""

# Test 5: Binary size check
echo "Test 5: Binary size"
SIZE=$(stat -f%z "$BINARY" 2>/dev/null || stat -c%s "$BINARY" 2>/dev/null)
SIZE_MB=$((SIZE / 1024 / 1024))
echo "Binary size: ${SIZE_MB}MB"
if [ "$SIZE_MB" -lt 50 ]; then
    echo "✓ Binary size is reasonable"
else
    echo "⚠ Binary size is larger than expected (${SIZE_MB}MB)"
fi
echo ""

# Test 6: File type check
echo "Test 6: File type"
if file "$BINARY" | grep -q "executable\|Mach-O\|ELF"; then
    echo "✓ Binary is executable"
else
    echo "✗ Binary is not recognized as executable"
    exit 1
fi
echo ""

echo "All tests passed! ✓"

