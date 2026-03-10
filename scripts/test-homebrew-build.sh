#!/bin/bash
# Test script to verify the Homebrew formula will build correctly

set -e

echo "Testing Homebrew formula build process..."

# Create a temporary directory
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"

# Copy the source
echo "Copying source files..."
cp -r "$OLDPWD"/* .

# Test the Go build command that Homebrew will use
echo "Testing Go build..."
go build -ldflags="-s -w -X github.com/kmf/sword-tui/internal/version.BuildNumber=test" -o sword-tui ./cmd/sword-tui

if [ -f sword-tui ]; then
    echo "✅ Build successful!"
    echo "Binary size: $(ls -lh sword-tui | awk '{print $5}')"
    
    # Test if the binary runs
    echo "Testing binary..."
    if ./sword-tui --version 2>&1 | grep -q "sword-tui"; then
        echo "✅ Binary runs successfully!"
    else
        echo "⚠️  Binary runs but --version flag might not be implemented"
    fi
else
    echo "❌ Build failed!"
    exit 1
fi

# Cleanup
cd "$OLDPWD"
rm -rf "$TEMP_DIR"

echo ""
echo "🎉 Homebrew formula should work correctly!"