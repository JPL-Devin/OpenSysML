# 1. Install

Install `sysml`, `sysml-lsp` and `sysml-grpc`, and verify what you installed. Nothing later in
this guide needs anything else on the machine.

## From a release build (recommended)

Download the latest release for your platform from [GitHub Releases](https://github.com/Open-MBEE/Systemica/releases):

**Linux (x64; use `systemica-linux-arm64.tar.gz` on arm64):**
```bash
wget https://github.com/Open-MBEE/Systemica/releases/latest/download/systemica-linux-amd64.tar.gz
tar xzf systemica-linux-amd64.tar.gz
sudo mv sysml sysml-lsp /usr/local/bin/
chmod +x /usr/local/bin/sysml /usr/local/bin/sysml-lsp
```

**macOS (Intel or Apple Silicon) — Homebrew is the recommended path:**
```bash
brew install Open-MBEE/tap/systemica
```
This avoids the Gatekeeper prompt described in [macOS: Gatekeeper](#macos-gatekeeper).

Use that fully-qualified name rather than tapping first. Homebrew 6 requires third-party taps
to be trusted before their code is loaded; installing by fully-qualified name trusts only this
formula, whereas the two-step form needs a trust step in between:
```bash
brew tap Open-MBEE/tap
brew trust --formula Open-MBEE/tap/systemica   # or: brew trust Open-MBEE/tap, for the whole tap
brew install systemica
```

**macOS, direct download (fallback):** use `curl`, not a browser.
```bash
# Apple Silicon; use systemica-darwin-amd64.tar.gz on Intel
curl -fL -o systemica.tar.gz https://github.com/Open-MBEE/Systemica/releases/latest/download/systemica-darwin-arm64.tar.gz
tar xzf systemica.tar.gz
sudo mv sysml sysml-lsp /usr/local/bin/
```

**Windows:**
Download `systemica-windows-amd64.zip` from [releases](https://github.com/Open-MBEE/Systemica/releases/latest), extract, and add to PATH. Windows SmartScreen may warn that the publisher is unrecognized; the binaries are not Authenticode-signed.

**Available binaries:**
- `sysml` — Interactive REPL
- `sysml-lsp` — Language Server Protocol server

`sysml-grpc` — the service the Python bindings talk to — is published as a raw
`sysml-grpc-<os>-<arch>` file with a `.sha256` sidecar rather than in an archive, because
`pysysml` downloads and verifies it itself (see [python/README.md](../../python/README.md)).
`make build-grpc` builds it from source.

**Archive layout:** `systemica-<os>-<arch>.tar.gz` bundles contain both binaries under their
plain names (`sysml`, `sysml-lsp`); the older single-binary `sysml-<os>-<arch>.tar.gz` and
`sysml-lsp-<os>-<arch>.tar.gz` archives are still published. The bundles and
`SHA256SUMS.txt` are published from v0.0.4 onward; for earlier releases use the
single-binary archives. The `sysml-grpc` binaries and their sidecars are published from the
next release onward, and `SHA256SUMS.txt` covers every archive and every published
`sysml-grpc` binary:

```bash
curl -fLO https://github.com/Open-MBEE/Systemica/releases/latest/download/SHA256SUMS.txt
shasum -a 256 -c SHA256SUMS.txt --ignore-missing   # macOS; use sha256sum -c on Linux
```

## macOS: Gatekeeper

When macOS refuses to run a downloaded binary with **"cannot be opened because the developer
cannot be verified"**, the cause is the `com.apple.quarantine` extended attribute that
browsers attach to downloads, combined with the fact that these binaries are not signed with
an Apple Developer ID or notarized. It is not a broken binary.

Ways to avoid it, best first:

1. **Install with Homebrew** (`brew install Open-MBEE/tap/systemica`). Homebrew
   downloads with `curl` and does not quarantine formula binaries. This is the recommended
   path, and the accepted stopgap until the releases are signed and notarized.
2. **Download with `curl` or `wget`** (as shown above). They do not set the quarantine
   attribute, so no prompt appears.
3. **Install with a Go toolchain** — built locally, never quarantined:
   ```bash
   go install github.com/Open-MBEE/Systemica/cmd/sysml@latest
   go install github.com/Open-MBEE/Systemica/cmd/sysml-lsp@latest
   ```
4. **Clear the attribute** if you already downloaded the archive in a browser. Verify the
   checksum first — you are turning off a security check, so make sure you have the file we
   published:
   ```bash
   shasum -a 256 systemica-darwin-arm64.tar.gz   # compare against SHA256SUMS.txt
   xattr -d com.apple.quarantine /usr/local/bin/sysml /usr/local/bin/sysml-lsp
   ```
   `xattr -d: No such xattr` simply means the file was not quarantined. Use
   `xattr -c <file>` to clear all attributes, or `xattr -dr com.apple.quarantine <dir>` for a
   directory.

See [MACOS_DISTRIBUTION.md](../project/macos-distribution.md) for the root-cause analysis and for what
signing + notarizing the releases would require.

## From source

**Prerequisites:**
- Go 1.25 or later
- Git
- Make (optional but recommended)

**Build:**
```bash
git clone https://github.com/Open-MBEE/Systemica.git
cd Systemica
make build       # builds bin/sysml, bin/sysml-lsp, and bin/sysml-grpc
# OR
go build -o sysml ./cmd/sysml
go build -o sysml-lsp ./cmd/sysml-lsp
```

**Install (optional):**
```bash
make install     # installs to $GOPATH/bin
# OR
sudo mv bin/sysml bin/sysml-lsp bin/sysml-grpc /usr/local/bin/
```

---

Next: [2. Your first model](02-first-model.md).
