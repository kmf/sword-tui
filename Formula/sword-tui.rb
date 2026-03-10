class SwordTui < Formula
  desc "Terminal-based Bible application built with Go"
  homepage "https://github.com/kmf/sword-tui"
  version "1.11.0"
  license "GPL-2.0-or-later"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/kmf/sword-tui/releases/download/v1.11.0/sword-tui-darwin-arm64.tar.gz"
      sha256 "1288b1e01be25bfe0a13db965eed8514d52bb6d058b62cdb28110ebfa1cbab73"
    else
      url "https://github.com/kmf/sword-tui/releases/download/v1.11.0/sword-tui-darwin-amd64.tar.gz"
      sha256 "6639ec74c117a541b453a405654df10c5dd76e1289edbdd8a0b1e415ab2c0049"
    end
  end

  on_linux do
    if Hardware::CPU.arm? && Hardware::CPU.is_64_bit?
      url "https://github.com/kmf/sword-tui/releases/download/v1.11.0/sword-tui-linux-arm64.tar.gz"
      sha256 "ba8fea941de6e1e16a9c6bccf4e29e34e30a125f887048d1ada0f1c5da6f9162"
    elsif Hardware::CPU.intel? && Hardware::CPU.is_64_bit?
      url "https://github.com/kmf/sword-tui/releases/download/v1.11.0/sword-tui-linux-amd64.tar.gz"
      sha256 "5cd8435e3a5b8278580e8cdbcf369ae34a48f32ea4bf1572781fae719480d289"
    else
      url "https://github.com/kmf/sword-tui/releases/download/v1.11.0/sword-tui-linux-i386.tar.gz"
      sha256 "cc43918431e185aaa6d4affb2b4ecde9852eda0c97394463c912a37698eada4d"
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