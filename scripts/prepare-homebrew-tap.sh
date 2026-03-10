#!/bin/bash
# Script to prepare the Homebrew tap repository locally

set -e

echo "Preparing Homebrew tap for sword-tui..."

# Create tap directory
TAP_DIR="homebrew-sword-tui"
if [ -d "$TAP_DIR" ]; then
    echo "Error: $TAP_DIR directory already exists!"
    echo "Please remove it or choose a different location."
    exit 1
fi

echo "Creating tap directory: $TAP_DIR"
mkdir -p "$TAP_DIR"

# Copy formula
echo "Copying formula..."
cp -r Formula "$TAP_DIR/"

# Update formula with current SHA256
echo "Calculating SHA256 for latest release..."
cd "$TAP_DIR"

# For now, we'll prepare it with empty SHA256
# You'll need to update this after creating the release
cat > Formula/sword-tui.rb << 'EOF'
class SwordTui < Formula
  desc "Terminal-based Bible application built with Go"
  homepage "https://github.com/kmf/sword-tui"
  url "https://github.com/kmf/sword-tui/archive/refs/tags/v1.11.0.tar.gz"
  sha256 "" # This will need to be updated with the actual SHA256 of the release tarball
  license "GPL-2.0-or-later"
  head "https://github.com/kmf/sword-tui.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X github.com/kmf/sword-tui/internal/version.BuildNumber=#{version}
    ]
    system "go", "build", *std_go_args(ldflags:), "./cmd/sword-tui"
  end

  test do
    # Test that the binary was installed and can run
    assert_match version.to_s, shell_output("#{bin}/sword-tui --version")
  end
end
EOF

# Create README for the tap
cat > README.md << 'EOF'
# Homebrew Tap for sword-tui

This is a Homebrew tap for [sword-tui](https://github.com/kmf/sword-tui), a terminal-based Bible application.

## Installation

```bash
brew tap kmf/sword-tui
brew install sword-tui
```

## Updating

```bash
brew update
brew upgrade sword-tui
```

## Formula

The formula is maintained in the main sword-tui repository and synchronized here for Homebrew distribution.

## License

This tap is licensed under the same license as sword-tui (GPL-2.0-or-later).
EOF

# Create .gitignore
cat > .gitignore << 'EOF'
.DS_Store
*.swp
*~
EOF

# Initialize git repository
echo "Initializing git repository..."
git init
git add .
git commit -m "Initial tap setup with sword-tui formula"

cd ..

echo ""
echo "✅ Homebrew tap prepared successfully!"
echo ""
echo "📋 Next steps:"
echo ""
echo "1. Create a new GitHub repository named 'homebrew-sword-tui'"
echo "   Go to: https://github.com/new"
echo "   - Repository name: homebrew-sword-tui"
echo "   - Make it public"
echo "   - Don't initialize with README, .gitignore, or license"
echo ""
echo "2. Push the tap to GitHub:"
echo "   cd homebrew-sword-tui"
echo "   git remote add origin git@github.com:kmf/homebrew-sword-tui.git"
echo "   git push -u origin main"
echo ""
echo "3. Update the formula with the correct SHA256:"
echo "   - Download the release tarball from GitHub"
echo "   - Run: sha256sum v1.11.0.tar.gz"
echo "   - Update the sha256 line in Formula/sword-tui.rb"
echo "   - Commit and push the change"
echo ""
echo "4. Test the tap:"
echo "   brew tap kmf/sword-tui"
echo "   brew install sword-tui"
echo ""
echo "The tap directory is ready at: ./homebrew-sword-tui"