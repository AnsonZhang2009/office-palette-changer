class Ostyle < Formula
  desc "Lightweight palette/font/heading stylizer for Microsoft Word & PowerPoint"
  homepage "https://github.com/ansonzhang/office-palette-changer"
  url "https://github.com/ansonzhang/office-palette-changer/archive/refs/tags/v0.1.0.tar.gz"
  # Fill in the real sha256 when tagging the release:
  #   shasum -a 256 <tarball>
  sha256 "0000000000000000000000000000000000000000000000000000000000000000"
  version "0.1.0"

  depends_on "go" => :build

  def install
    system "go", "build", "-trimpath", "-ldflags", "-s -w", "-o", "ostyle", "./cmd/ostyle"
    bin.install "ostyle"
  end

  test do
    assert_match "ostyle", shell_output("#{bin}/ostyle version")
  end
end
