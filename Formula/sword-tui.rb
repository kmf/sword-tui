class SwordTui < Formula
  desc "Terminal-based Bible application built with Go"
  homepage "https://github.com/kmf/sword-tui"
  version "2.0.1"
  license "GPL-2.0-or-later"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/kmf/sword-tui/releases/download/v2.0.1/sword-tui-darwin-arm64.tar.gz"
      sha256 "db62cca076d7c6dc2cdf10c22061489abdbdd7280767caef7b0fff6ae8c155ef"
    else
      url "https://github.com/kmf/sword-tui/releases/download/v2.0.1/sword-tui-darwin-amd64.tar.gz"
      sha256 "827762bf019fe000cfe75e99e31d1ca7a3541fd33e65905a5adb019e2bd97fe2"
    end
  end

  on_linux do
    if Hardware::CPU.arm? && Hardware::CPU.is_64_bit?
      url "https://github.com/kmf/sword-tui/releases/download/v2.0.1/sword-tui-linux-arm64.tar.gz"
      sha256 "2ea37496c57078c9e4a3a7a98326626b1a011fd5ebbdca6835a44519ff705786"
    elsif Hardware::CPU.intel? && Hardware::CPU.is_64_bit?
      url "https://github.com/kmf/sword-tui/releases/download/v2.0.1/sword-tui-linux-amd64.tar.gz"
      sha256 "b08514c37987358e464bfb199332916b5a730b59fb407be98457e647c792a156"
    else
      url "https://github.com/kmf/sword-tui/releases/download/v2.0.1/sword-tui-linux-i386.tar.gz"
      sha256 "9c190548c3a7a1e3fdfea7e453f93ff3a37a3b36f2941168490ba84d5a74177c"
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
