#!/bin/bash
# Script to update the Homebrew formula with new binary releases

set -e

# Get the latest tag
LATEST_TAG=$(git describe --tags --abbrev=0)

echo "Updating formula for ${LATEST_TAG}..."

# Download each binary and calculate SHA256
declare -A SHA256_MAP
PLATFORMS=(
  "darwin-amd64"
  "darwin-arm64"
  "linux-amd64"
  "linux-arm64"
  "linux-i386"
)

for platform in "${PLATFORMS[@]}"; do
  FILE="sword-tui-${platform}.tar.gz"
  URL="https://github.com/kmf/sword-tui/releases/download/${LATEST_TAG}/${FILE}"
  
  echo "Calculating SHA256 for ${platform}..."
  SHA256=$(curl -sL "$URL" | sha256sum | awk '{print $1}')
  SHA256_MAP[$platform]=$SHA256
  echo "  ${platform}: ${SHA256}"
done

# Update the formula
cat > Formula/sword-tui.rb << EOF
class SwordTui < Formula
  desc "Terminal-based Bible application built with Go"
  homepage "https://github.com/kmf/sword-tui"
  version "${LATEST_TAG#v}"
  license "GPL-2.0-or-later"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/kmf/sword-tui/releases/download/${LATEST_TAG}/sword-tui-darwin-arm64.tar.gz"
      sha256 "${SHA256_MAP[darwin-arm64]}"
    else
      url "https://github.com/kmf/sword-tui/releases/download/${LATEST_TAG}/sword-tui-darwin-amd64.tar.gz"
      sha256 "${SHA256_MAP[darwin-amd64]}"
    end
  end

  on_linux do
    if Hardware::CPU.arm? && Hardware::CPU.is_64_bit?
      url "https://github.com/kmf/sword-tui/releases/download/${LATEST_TAG}/sword-tui-linux-arm64.tar.gz"
      sha256 "${SHA256_MAP[linux-arm64]}"
    elsif Hardware::CPU.intel? && Hardware::CPU.is_64_bit?
      url "https://github.com/kmf/sword-tui/releases/download/${LATEST_TAG}/sword-tui-linux-amd64.tar.gz"
      sha256 "${SHA256_MAP[linux-amd64]}"
    else
      url "https://github.com/kmf/sword-tui/releases/download/${LATEST_TAG}/sword-tui-linux-i386.tar.gz"
      sha256 "${SHA256_MAP[linux-i386]}"
    end
  end

  def install
    # The binary name in the tarball follows the pattern sword-tui-{os}-{arch}
    binary_name = "sword-tui-#{OS.kernel_name.downcase}-"
    binary_name += if Hardware::CPU.arm?
      "arm64"
    elsif Hardware::CPU.is_64_bit?
      "amd64"
    else
      "i386"
    end
    
    bin.install binary_name => "sword-tui"
    
    # Also install README and LICENSE if present
    doc.install "README.md" if File.exist?("README.md")
    doc.install "LICENSE" if File.exist?("LICENSE")
  end

  test do
    # Test that the binary was installed and can run
    assert_match version.to_s, shell_output("#{bin}/sword-tui --version")
  end
end
EOF

echo "Formula updated successfully!"
echo "Version: ${LATEST_TAG}"