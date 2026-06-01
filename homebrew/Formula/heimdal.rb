class Heimdal < Formula
  desc "Heimdal CLI — AI agent test aracı"
  homepage "https://heimdal.dev"
  version "0.0.0"

  on_macos do
    if Hardware::CPU.intel?
      url "https://github.com/heimdal/cli/releases/download/v0.0.0/heimdal_0.0.0_darwin_amd64.tar.gz"
      sha256 ""
    else
      url "https://github.com/heimdal/cli/releases/download/v0.0.0/heimdal_0.0.0_darwin_arm64.tar.gz"
      sha256 ""
    end
  end

  on_linux do
    url "https://github.com/heimdal/cli/releases/download/v0.0.0/heimdal_0.0.0_linux_amd64.tar.gz"
    sha256 ""
  end

  def install
    bin.install "heimdal"
  end

  test do
    assert_match "heimdal", shell_output("#{bin}/heimdal --help")
  end
end
