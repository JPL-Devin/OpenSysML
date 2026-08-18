# Homebrew formula for Systemica, maintained here and copied to the tap repo
# Open-MBEE/homebrew-tap as Formula/systemica.rb.
#
# Per release, exactly four fields change: the `sha256` values. Get them from
# the release's SHA256SUMS.txt, e.g. for v0.0.4:
#
#   curl -fL https://github.com/Open-MBEE/OpenSysML/releases/download/v0.0.4/SHA256SUMS.txt
#
# and take the line for each systemica-<os>-<arch>.tar.gz. The four URLs change
# only in the tag path component; Homebrew scans `version` from that tag, and
# `brew audit --strict` rejects a redundant explicit `version` line.
#
# Or render the whole file (recommended, no manual copying):
#
#   scripts/render-homebrew-formula.sh v0.0.4 > Formula/systemica.rb
#
# The PLACEHOLDER checksums below are intentionally invalid: this file as
# committed here is the source template. Do not commit it to the tap unrendered.
class Systemica < Formula
  desc "SysML v2 toolchain: interactive REPL and language server"
  homepage "https://github.com/Open-MBEE/OpenSysML"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/Open-MBEE/OpenSysML/releases/download/__TAG__/systemica-darwin-arm64.tar.gz"
      sha256 "__SHA256_DARWIN_ARM64__"
    end
    on_intel do
      url "https://github.com/Open-MBEE/OpenSysML/releases/download/__TAG__/systemica-darwin-amd64.tar.gz"
      sha256 "__SHA256_DARWIN_AMD64__"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/Open-MBEE/OpenSysML/releases/download/__TAG__/systemica-linux-arm64.tar.gz"
      sha256 "__SHA256_LINUX_ARM64__"
    end
    on_intel do
      url "https://github.com/Open-MBEE/OpenSysML/releases/download/__TAG__/systemica-linux-amd64.tar.gz"
      sha256 "__SHA256_LINUX_AMD64__"
    end
  end

  def install
    bin.install "sysml", "sysml-lsp"
  end

  test do
    # Release binaries embed the tag (e.g. "sysml v0.0.4") via ldflags; `version`
    # is that tag without the leading "v", scanned from the URL.
    assert_match version.to_s, shell_output("#{bin}/sysml --version")
    assert_match version.to_s, shell_output("#{bin}/sysml-lsp --version")

    # Evaluate an expression non-interactively: exercises lexer, parser, and runtime.
    assert_match "= 8", shell_output("#{bin}/sysml -e '5 + 3'")
  end
end
