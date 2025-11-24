#!/bin/bash

# Build script for sword-tui that increments build number

BUILD_FILE=".build_number"

# Read current build number or start at 1
if [ -f "$BUILD_FILE" ]; then
    BUILD_NUM=$(cat "$BUILD_FILE")
else
    BUILD_NUM=0
fi

# Increment build number
BUILD_NUM=$((BUILD_NUM + 1))

# Save new build number
echo "$BUILD_NUM" > "$BUILD_FILE"

# Build with the build number injected
echo "Building sword-tui (build $BUILD_NUM)..."
go build -ldflags "-X sword-tui/internal/version.BuildNumber=$BUILD_NUM" -o sword-tui ./cmd/sword-tui

if [ $? -eq 0 ]; then
    echo "Build successful: sword-tui (build $BUILD_NUM)"
else
    echo "Build failed"
    exit 1
fi
