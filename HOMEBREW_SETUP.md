# Homebrew Setup Summary

This document summarizes the Homebrew integration added to sword-tui.

## What was added

1. **Homebrew Formula** (`Formula/sword-tui.rb`)
   - Formula that downloads pre-built binaries for each platform
   - Automatic platform/architecture detection
   - SHA256 verification for each binary
   - Installation test using `--version` flag

2. **Version flag support** (`cmd/sword-tui/main.go`)
   - Added `--version` flag to display version information
   - Required for Homebrew formula testing

3. **Automation Scripts**
   - `scripts/update-formula.sh` - Updates formula for source builds (legacy)
   - `scripts/update-formula-binary.sh` - Updates formula for binary releases
   - `scripts/setup-homebrew-tap.sh` - Helps set up a Homebrew tap repository
   - `scripts/test-homebrew-build.sh` - Tests the build process locally

4. **GitHub Actions**
   - `.github/workflows/update-homebrew.yml` - Automatically updates formula on new releases

5. **Documentation**
   - Updated README with Homebrew installation instructions
   - `docs/HOMEBREW.md` - Detailed guide for users and maintainers

## How It Works

The Homebrew formula uses **pre-built binaries** instead of building from source:

1. When a user runs `brew install sword-tui`, Homebrew:
   - Detects their OS (macOS/Linux) and architecture (Intel/ARM/i386)
   - Downloads the appropriate pre-built binary from GitHub releases
   - Verifies the SHA256 checksum
   - Installs the binary to the Homebrew bin directory

2. Benefits:
   - ⚡ Fast installation (seconds vs minutes)
   - 📦 No build dependencies (no need for Go)
   - ✅ Consistent binaries across all users
   - 💾 Smaller download size

## Current Status

✅ **Completed**:
- Homebrew tap created at `kmf/sword-tui`
- Formula supports all platforms (macOS Intel/ARM, Linux x64/ARM/i386)
- Binary downloads working correctly
- Documentation updated

## Usage

Users can now install sword-tui via Homebrew:
```bash
brew tap kmf/sword-tui
brew install sword-tui
```

## Maintaining the Formula

When releasing a new version:

1. **Ensure binaries are built**: The GitHub release workflow should create:
   - `sword-tui-darwin-amd64.tar.gz`
   - `sword-tui-darwin-arm64.tar.gz`
   - `sword-tui-linux-amd64.tar.gz`
   - `sword-tui-linux-arm64.tar.gz`
   - `sword-tui-linux-i386.tar.gz`

2. **Update the formula**:
   ```bash
   ./scripts/update-formula-binary.sh
   ```

3. **Push to both repositories**:
   - Main repository (`kmf/sword-tui`)
   - Tap repository (`kmf/homebrew-sword-tui`)

## Alternative: Homebrew Core

For wider distribution, you could submit the formula to homebrew-core:
1. Fork https://github.com/Homebrew/homebrew-core
2. Add the formula to `Formula/s/sword-tui.rb`
3. Submit a pull request following their contribution guidelines

Note: Homebrew Core prefers formulas that build from source, so you might need to provide both options.

## Testing

Test the formula locally:
```bash
# Install from local formula
brew install --verbose Formula/sword-tui.rb

# Test that it works
sword-tui --version

# Audit the formula
brew audit --strict Formula/sword-tui.rb
```