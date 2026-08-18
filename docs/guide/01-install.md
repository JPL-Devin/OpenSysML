# 1. Install

Install `sysml`, `sysml-lsp` and `sysml-grpc`, and verify what you installed. Nothing later in
this guide needs anything else on the machine.

## From a release build (recommended)

Download the latest release for your platform from [GitHub Releases](https://github.com/Open-MBEE/OpenSysML/releases):

**Linux (x64; use `opensysml-linux-arm64.tar.gz` on arm64):**
```bash
wget https://github.com/Open-MBEE/OpenSysML/releases/latest/download/opensysml-linux-amd64.tar.gz
tar xzf opensysml-linux-amd64.tar.gz
sudo mv sysml sysml-lsp /usr/local/bin/
chmod +x /usr/local/bin/sysml /usr/local/bin/sysml-lsp
```

**macOS (Intel or Apple Silicon) — Homebrew is the recommended path:**
```bash
brew install Open-MBEE/tap/opensysml
```
This avoids the Gatekeeper prompt described in [macOS: Gatekeeper](#macos-gatekeeper).

Use that fully-qualified name rather than tapping first. Homebrew 6 requires third-party taps
to be trusted before their code is loaded; installing by fully-qualified name trusts only this
formula, whereas the two-step form needs a trust step in between:
```bash
brew tap Open-MBEE/tap
brew trust --formula Open-MBEE/tap/opensysml   # or: brew trust Open-MBEE/tap, for the whole tap
brew install opensysml
```

**macOS, direct download (fallback):** use `curl`, not a browser.
```bash
# Apple Silicon; use opensysml-darwin-amd64.tar.gz on Intel
curl -fL -o opensysml.tar.gz https://github.com/Open-MBEE/OpenSysML/releases/latest/download/opensysml-darwin-arm64.tar.gz
tar xzf opensysml.tar.gz
sudo mv sysml sysml-lsp /usr/local/bin/
```

**Windows:**
Download `opensysml-windows-amd64.zip` from [releases](https://github.com/Open-MBEE/OpenSysML/releases/latest), extract, and add to PATH. Windows SmartScreen may warn that the publisher is unrecognized; the binaries are not Authenticode-signed.

**Available binaries:**
- `sysml` — Interactive REPL
- `sysml-lsp` — Language Server Protocol server

`sysml-grpc` — the service the Python bindings talk to — is published as a raw
`sysml-grpc-<os>-<arch>` file with a `.sha256` sidecar rather than in an archive, because
`opensysml` downloads and verifies it itself (see [python/README.md](../../python/README.md)).
`make build-grpc` builds it from source.

**Archive layout:** `opensysml-<os>-<arch>.tar.gz` bundles contain both binaries under their
plain names (`sysml`, `sysml-lsp`); the older single-binary `sysml-<os>-<arch>.tar.gz` and
`sysml-lsp-<os>-<arch>.tar.gz` archives are still published. The bundles and
`SHA256SUMS.txt` are published from v0.0.4 onward; for earlier releases use the
single-binary archives. The `sysml-grpc` binaries and their sidecars are published from the
next release onward, and `SHA256SUMS.txt` covers every archive and every published
`sysml-grpc` binary:

```bash
curl -fLO https://github.com/Open-MBEE/OpenSysML/releases/latest/download/SHA256SUMS.txt
shasum -a 256 -c SHA256SUMS.txt --ignore-missing   # macOS; use sha256sum -c on Linux
```

## macOS: Gatekeeper

When macOS refuses to run a downloaded binary with **"cannot be opened because the developer
cannot be verified"**, the cause is the `com.apple.quarantine` extended attribute that
browsers attach to downloads, combined with the fact that these binaries are not signed with
an Apple Developer ID or notarized. It is not a broken binary.

Ways to avoid it, best first:

1. **Install with Homebrew** (`brew install Open-MBEE/tap/opensysml`). Homebrew
   downloads with `curl` and does not quarantine formula binaries. This is the recommended
   path, and the accepted stopgap until the releases are signed and notarized.
2. **Download with `curl` or `wget`** (as shown above). They do not set the quarantine
   attribute, so no prompt appears.
3. **Install with a Go toolchain** — built locally, never quarantined:
   ```bash
   go install github.com/Open-MBEE/OpenSysML/cmd/sysml@latest
   go install github.com/Open-MBEE/OpenSysML/cmd/sysml-lsp@latest
   ```
4. **Clear the attribute** if you already downloaded the archive in a browser. Verify the
   checksum first — you are turning off a security check, so make sure you have the file we
   published:
   ```bash
   shasum -a 256 opensysml-darwin-arm64.tar.gz   # compare against SHA256SUMS.txt
   xattr -d com.apple.quarantine /usr/local/bin/sysml /usr/local/bin/sysml-lsp
   ```
   `xattr -d: No such xattr` simply means the file was not quarantined. Use
   `xattr -c <file>` to clear all attributes, or `xattr -dr com.apple.quarantine <dir>` for a
   directory.

See [MACOS_DISTRIBUTION.md](../project/macos-distribution.md) for the root-cause analysis and for what
signing + notarizing the releases would require.

## Installing a solver (optional)

Nothing above needs an SMT solver: the whole guide, and every normative check —
`%constraint`, `%requirement`, `%satisfy`, `%eval` — runs on the concrete evaluator, which is
the normative implementation. A solver is needed only by the **experimental** extension
`%check`/`%explain`, which asks whether a constraint *can* be satisfied rather than whether it
holds of an object (see [reference/repl-commands.md](../reference/repl-commands.md)).

The solver is a separate program, run as a process and spoken to in SMT-LIB2 — nothing is
linked in and nothing is bundled in the release archives, which stay single static binaries.
Either [z3](https://github.com/Z3Prover/z3) (MIT) or [cvc5](https://github.com/cvc5/cvc5)
works; z3 is the one to install unless you have a reason to prefer cvc5.

**macOS and Linux, Homebrew — automatic:** z3 is a dependency of the formula, so the
recommended install already brings a working `%check`:
```bash
brew install Open-MBEE/tap/opensysml   # installs z3 too
brew install z3                        # or just the solver, next to a non-brew sysml
```

**Debian and Ubuntu:**
```bash
sudo apt install z3          # provides /usr/bin/z3
```

**Other Linux distributions** — each of these packages provides a `z3` executable:
```bash
sudo dnf install z3          # Fedora
sudo pacman -S z3            # Arch (extra/z3)
sudo apk add z3              # Alpine (community repository)
nix-shell -p z3              # nixpkgs, for one shell; or: nix profile install nixpkgs#z3
```
Of these, apt is the one run while writing this; the rest were read off their package indexes
(Fedora's `z3`, `extra/z3`, Alpine `community/z3`, and nixpkgs' `z3`, all shipping a `z3`
program), so a distribution that has renamed or dropped the package is the case to expect
trouble from.

**Windows:** take the official prebuilt archive from
[z3's releases](https://github.com/Z3Prover/z3/releases) — `z3-<version>-x64-win.zip` (for
example `z3-5.1.0-x64-win.zip`; `arm64` and `x86` builds are published too). Unzip it and
either add the archive's `bin` directory to `PATH`, or point `OPENSYSML_SMT` at the executable:
```powershell
$env:OPENSYSML_SMT = "C:\tools\z3-5.1.0-x64-win\bin\z3.exe"
```
[Scoop](https://scoop.sh) packages the same archive, so `scoop install z3` puts `z3.exe` on
`PATH` for you.

**Any platform with Python — the `pip` fallback:** the `z3-solver` wheels (MIT) are published
for Linux, macOS and Windows and carry the executable, not just the Python module:
```bash
python3 -m venv .venv
.venv/bin/pip install z3-solver     # z3 lands in .venv/bin/z3
```
An activated virtual environment therefore puts `z3` on `PATH` and needs nothing else. Without
activating it, name the executable instead:
```bash
OPENSYSML_SMT=$PWD/.venv/bin/z3 sysml model.sysml
```

**cvc5, the alternative backend:** there is no Homebrew formula and no Debian/Ubuntu package;
take a prebuilt archive from [cvc5's releases](https://github.com/cvc5/cvc5/releases)
(`cvc5-Linux-x86_64-static.zip`, `cvc5-macOS-arm64-static.zip`, `cvc5-Win64-x86_64-static.zip`
and so on), whose `bin/cvc5` is what goes on `PATH`. cvc5 is under a modified BSD licence, but
its default build links GMP under LGPL-3, and it can be configured against GPL libraries (the
`*-gpl` archives are those builds). That matters if you **redistribute** cvc5; it does not
change how you may use OpenSysML, which links neither solver.

### Verifying the solver is found

`%check` names the solver it used, so the verdict line is the verification:

```
sysml> %check P::C
✗ Constraint C is unsatisfiable (z3, 8ms)
```

Discovery order: `OPENSYSML_SMT` first — an executable name or a path, and a value naming no
executable is an error rather than a silent fallback — then `z3` on `PATH`, then `cvc5`. z3 wins
when both are installed, wherever they sit in `PATH`. `OPENSYSML_SMT_TIMEOUT` (default `10s`)
bounds one query, after which the verdict is `unknown` rather than an error; see
[reference/environment.md](../reference/environment.md).

With no solver anywhere, `%check` and `%explain` report that instead of a verdict, and every
other command is unaffected:

```
sysml> %check P::C
error: no SMT solver found: install z3 (`apt install z3`, `brew install z3`) or cvc5, or set OPENSYSML_SMT to a solver executable; looked for [z3 cvc5] on PATH
```

## From source

**Prerequisites:**
- Go 1.25 or later
- Git
- Make (optional but recommended)

**Build:**
```bash
git clone https://github.com/Open-MBEE/OpenSysML.git
cd OpenSysML
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
