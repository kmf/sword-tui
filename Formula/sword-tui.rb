class SwordTui < Formula
  desc "Terminal-based Bible application built with Go"
  homepage "https://github.com/kmf/sword-tui"
  version "2.0.0"
  license "GPL-2.0-or-later"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/kmf/sword-tui/releases/download/v2.0.0/sword-tui-darwin-arm64.tar.gz"
      sha256 "6974340822e49a4ad5fc68b5325caeea90610be25b61b487f427dfa487d27a6c"
    else
      url "https://github.com/kmf/sword-tui/releases/download/v2.0.0/sword-tui-darwin-amd64.tar.gz"
      sha256 "41faab0c5241435f05183fc9b1d79a7e9b722650ac01b413d31716af566ae583"
    end
  end

  on_linux do
    if Hardware::CPU.arm? && Hardware::CPU.is_64_bit?
      url "https://github.com/kmf/sword-tui/releases/download/v2.0.0/sword-tui-linux-arm64.tar.gz"
      sha256 "d93296afe588372198ad5c84be6f4f7f2a9b0a08091050c7656ab8ab6ee7d04b"
    elsif Hardware::CPU.intel? && Hardware::CPU.is_64_bit?
      url "https://github.com/kmf/sword-tui/releases/download/v2.0.0/sword-tui-linux-amd64.tar.gz"
      sha256 "47dd254b335e89da08d45398f11f921a9f6fe0e203d228518094f8f9659420c0"
    else
      url "https://github.com/kmf/sword-tui/releases/download/v2.0.0/sword-tui-linux-i386.tar.gz"
      sha256 "cee1ea2f3534d7744e0f724ee8ab9f7d7334b469a2e1cec46375ddea85d6d681"
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
