#!/bin/bash
# Script to help set up a Homebrew tap repository

set -e

echo "This script will help you set up a Homebrew tap for sword-tui"
echo "Prerequisites:"
echo "  - You need a GitHub account"
echo "  - You need to create a repository named 'homebrew-sword-tui'"
echo ""
read -p "Have you created the 'homebrew-sword-tui' repository? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Please create the repository first at: https://github.com/new"
    echo "Name it: homebrew-sword-tui"
    exit 1
fi

# Clone the tap repository
read -p "Enter your GitHub username: " GITHUB_USERNAME
echo "Cloning tap repository..."
git clone "git@github.com:${GITHUB_USERNAME}/homebrew-sword-tui.git" /tmp/homebrew-sword-tui

# Copy formula
echo "Copying formula..."
cp -r Formula /tmp/homebrew-sword-tui/

# Update formula with current SHA256
echo "Updating formula with latest release info..."
cd /tmp/homebrew-sword-tui
../scripts/update-formula.sh

# Create README for the tap
cat > README.md << 'EOF'
# Homebrew Tap for sword-tui

This is a Homebrew tap for [sword-tui](https://github.com/kmf/sword-tui), a terminal-based Bible application.

## Installation

```bash
brew tap kmf/sword-tui
brew install sword-tui
```

## Formula

The formula is maintained in the main sword-tui repository and synchronized here for Homebrew distribution.
EOF

# Commit and push
echo "Setting up git..."
git add .
git commit -m "Initial tap setup with sword-tui formula"
git push origin main

echo ""
echo "✅ Homebrew tap setup complete!"
echo ""
echo "Users can now install sword-tui with:"
echo "  brew tap ${GITHUB_USERNAME}/sword-tui"
echo "  brew install sword-tui"
echo ""
echo "Don't forget to update the README.md in the main repository with the correct tap name!"

# Cleanup
rm -rf /tmp/homebrew-sword-tui