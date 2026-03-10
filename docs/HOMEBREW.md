# Homebrew Installation Guide

This guide explains how to install sword-tui using Homebrew on macOS and Linux.

## For Users

### Installing sword-tui via Homebrew

```bash
# Add the tap
brew tap kmf/sword-tui

# Install sword-tui
brew install sword-tui
```

The installation downloads pre-built binaries for your platform, so it's fast and doesn't require any build tools.

### Supported Platforms

- macOS (Intel and Apple Silicon)
- Linux (x86_64, ARM64, and i386)

### Updating

```bash
brew update
brew upgrade sword-tui
```

### Uninstalling

```bash
brew uninstall sword-tui
brew untap kmf/sword-tui  # Optional: remove the tap
```

## For Maintainers

### How It Works

The Homebrew formula downloads pre-built binaries from GitHub releases instead of building from source. This provides:
- ⚡ Fast installation (seconds instead of minutes)
- 📦 No build dependencies required
- ✅ Consistent binaries across all installations

### Setting up the Tap Repository

1. Create a new GitHub repository named `homebrew-sword-tui` ✅
2. Copy the `Formula` directory from this repository to the tap repository
3. The tap will be available as `kmf/sword-tui`

### Updating the Formula

When releasing a new version:

1. The GitHub Action workflow will automatically create a PR with the updated formula
2. Alternatively, run the update script manually:
   ```bash
   # For binary releases (recommended)
   ./scripts/update-formula-binary.sh
   
   # For source builds (legacy)
   ./scripts/update-formula.sh
   ```
3. Commit and push the updated formula to both repositories

### Formula Structure

The formula is located at `Formula/sword-tui.rb` and:
- Detects the user's platform (macOS/Linux) and architecture
- Downloads the appropriate pre-built binary from GitHub releases
- Verifies the download with SHA256 checksum
- Installs the binary and documentation

### Binary Release Requirements

For the Homebrew formula to work, each GitHub release must include:
- `sword-tui-darwin-amd64.tar.gz` (macOS Intel)
- `sword-tui-darwin-arm64.tar.gz` (macOS Apple Silicon)
- `sword-tui-linux-amd64.tar.gz` (Linux x86_64)
- `sword-tui-linux-arm64.tar.gz` (Linux ARM64)
- `sword-tui-linux-i386.tar.gz` (Linux 32-bit)

Each tarball should contain:
- The compiled binary (named `sword-tui-{os}-{arch}`)
- `README.md`
- `LICENSE`

### Testing the Formula Locally

```bash
# Test the formula
brew install --verbose Formula/sword-tui.rb

# Audit the formula
brew audit --strict Formula/sword-tui.rb

# Test that it runs
sword-tui --version
```

## Troubleshooting

### Common Issues

1. **Formula not found**: Ensure the tap is added correctly with `brew tap kmf/sword-tui`
2. **Wrong architecture**: The formula automatically detects your platform, but you can check with `uname -m`
3. **Checksum mismatch**: The formula may need updating after a new release
4. **Binary not found**: Ensure the GitHub release includes binaries for all platforms

### Getting Help

If you encounter issues with the Homebrew installation, please open an issue on the GitHub repository.