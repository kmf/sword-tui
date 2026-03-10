#!/bin/bash
# Script to update the Homebrew formula with the latest release

set -e

# Get the latest tag
LATEST_TAG=$(git describe --tags --abbrev=0)

# Download the tarball and calculate SHA256
echo "Downloading tarball for ${LATEST_TAG}..."
wget -q "https://github.com/kmf/sword-tui/archive/refs/tags/${LATEST_TAG}.tar.gz"
SHA256=$(sha256sum "${LATEST_TAG}.tar.gz" | awk '{print $1}')
rm "${LATEST_TAG}.tar.gz"

echo "Latest version: ${LATEST_TAG}"
echo "SHA256: ${SHA256}"

# Update the formula
sed -i.bak "s|url \".*\"|url \"https://github.com/kmf/sword-tui/archive/refs/tags/${LATEST_TAG}.tar.gz\"|" Formula/sword-tui.rb
sed -i.bak "s|sha256 \".*\"|sha256 \"${SHA256}\"|" Formula/sword-tui.rb

# Remove backup file
rm Formula/sword-tui.rb.bak

echo "Formula updated successfully!"