# Homebrew packaging

`systemica.rb.template` is a Homebrew formula template for the `systemica-<os>-<arch>`
bundle archives published with each tagged release (they contain `sysml` and `sysml-lsp`
under their plain names, which is the layout Homebrew's `bin.install` expects).

Render it for a published tag:

```bash
scripts/render-homebrew-formula.sh v0.3.0 > systemica.rb
```

The script reads `SHA256SUMS.txt` from the release (downloading it if a local copy is not
passed as the second argument) and substitutes the version, URLs, and checksums.

## Why this matters on macOS

`brew install` fetches archives with `curl`, which does not set the
`com.apple.quarantine` extended attribute, so a Homebrew-installed `sysml` does not trigger
the Gatekeeper "developer cannot be verified" prompt. Homebrew applies quarantine only to
*casks* (hence the cask-only `--no-quarantine` flag); formulae are not quarantined. See
[docs/MACOS_DISTRIBUTION.md](../../docs/MACOS_DISTRIBUTION.md) for the full analysis.

## Not yet published — maintainer action required

**There is no tap yet.** A formula must live in a tap repository named
`homebrew-<something>` (e.g. `Open-MBEE/homebrew-tap`), so that users can run:

```bash
brew tap Open-MBEE/tap
brew install systemica
```

Creating that repository is a maintainer decision and was deliberately not done from this
change. Once it exists, the release process is:

1. Tag and let CircleCI publish the release (archives + `SHA256SUMS.txt`).
2. Run `scripts/render-homebrew-formula.sh <tag> > Formula/systemica.rb` in the tap.
3. Commit and push the tap.

Automating step 2–3 from CI needs a token with write access to the tap repository; that is
out of scope here.
