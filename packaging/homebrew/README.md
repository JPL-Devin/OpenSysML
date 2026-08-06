# Homebrew packaging

`brew install` is the recommended macOS install path: Homebrew fetches archives with `curl`,
which does not set the `com.apple.quarantine` extended attribute, so a Homebrew-installed
`sysml` never triggers the Gatekeeper "developer cannot be verified" prompt. (Homebrew
applies quarantine only to *casks* — hence the cask-only `--no-quarantine` flag — not to
formulae.) It is the accepted stopgap until the releases are Developer ID signed and
notarized; see [docs/MACOS_DISTRIBUTION.md](../../docs/MACOS_DISTRIBUTION.md).

`Formula/systemica.rb` here is the maintained source of the formula. It carries
`__VERSION__` / `__TAG__` / `__SHA256_*__` placeholders and is **not installable as-is**;
`scripts/render-homebrew-formula.sh` substitutes them from a release's `SHA256SUMS.txt` and
strips the maintainer-facing header comment.

## The tap does not exist yet

**`brew tap Open-MBEE/tap && brew install systemica` will not work until the maintainer
creates `Open-MBEE/homebrew-tap` and pushes a rendered formula.** Nothing in this repository
creates or publishes it. One-time setup:

1. Create a public GitHub repository named exactly **`Open-MBEE/homebrew-tap`**. The
   `homebrew-` prefix is what makes `brew tap Open-MBEE/tap` resolve to it.
2. Render the formula for the newest release and commit it as **`Formula/systemica.rb`**:

   ```bash
   # in a clone of Open-MBEE/Systemica
   ./scripts/render-homebrew-formula.sh v0.3.0 > /tmp/systemica.rb

   # in a clone of Open-MBEE/homebrew-tap
   mkdir -p Formula && cp /tmp/systemica.rb Formula/systemica.rb
   git add Formula/systemica.rb
   git commit -m "systemica 0.3.0"
   git push
   ```

   The tag must be a release that already has `systemica-<os>-<arch>.tar.gz` archives and
   `SHA256SUMS.txt` attached — i.e. one built after this change lands. The script fails
   loudly if a checksum is missing.
3. Verify before announcing it:

   ```bash
   brew tap Open-MBEE/tap
   brew install --verbose systemica
   brew test systemica
   brew audit --strict --online Open-MBEE/tap/systemica
   ```

## Per release

The release job publishes stable artifact names, so only two things change per release:

1. Tag `vX.Y.Z` and let CircleCI publish the release (per-binary archives,
   `systemica-<os>-<arch>.tar.gz`/`.zip` bundles, and `SHA256SUMS.txt`).
2. In the tap: `./scripts/render-homebrew-formula.sh vX.Y.Z > Formula/systemica.rb`, commit,
   push. Five values change — `version` and the four `sha256` lines.

This can be automated later: a tag-triggered job could run the render script and push the
tap commit, but that needs a token with write access to `Open-MBEE/homebrew-tap` stored as a
CI secret, which is a maintainer decision and is deliberately not set up here.
