# Releasing Systemica

A release is cut by pushing a `v*` tag. Everything after that is CircleCI: the
`release` workflow runs the test suite, cross-compiles `sysml` and `sysml-lsp`
for five platforms, and publishes the archives to a GitHub release. Nothing is
published from a laptop.

## Before tagging

Run the full gate on the commit you intend to tag:

```bash
gofmt -l .            # must print nothing
go build ./...
go vet ./...
go test ./...
```

The OMG training-corpus gate skips while the corpus is absent, so fetch it and
run it explicitly — the expected result is the pinned baseline, currently
98/100 files clean:

```bash
./scripts/download-training-examples.sh
SYSTEMICA_REQUIRE_TRAINING_CORPUS=1 go test -count=1 ./internal/core/model -run TestTrainingExamples
```

A change in that count is a finding to adjudicate file by file, never a
baseline to regenerate.

Then check the release-facing text:

- `CHANGELOG.md` has an entry for this version, dated, with the previous
  version's entry unchanged.
- `README.md` and `docs/QUICKSTART.md` transcripts match what the binary
  prints. Build it (`make build-sysml`) and paste a few commands through it.
- Test counts match a real run and agree across the docs that repeat them
  (`docs/SPEC_COMPLIANCE.md`, `docs/ARCHITECTURE.md`, `docs/TRAINING_EXAMPLES.md`,
  `README.md`), and no compliance row claims more than the implementation does.

## Tagging

The tag is the version: CircleCI passes `CIRCLE_TAG` to the build as
`VERSION`, so `sysml --version` reports it.

```bash
git checkout main && git pull
git tag -a v0.0.4 -m "v0.0.4"
git push origin v0.0.4
```

Tags are matched by `/^v.*/` in `.circleci/config.yml`. A tag on a commit that
fails the suite fails the release workflow before anything is published.

## What CircleCI publishes

`build-release` produces, in `dist/`:

- per-binary archives — `sysml-<os>-<arch>.tar.gz`,
  `sysml-lsp-<os>-<arch>.tar.gz` (`.zip` on Windows);
- bundle archives — `systemica-<os>-<arch>.tar.gz` holding both binaries under
  their plain names, which is the layout Homebrew and a PATH install expect;
- `SHA256SUMS.txt` over every archive.

Platforms: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64,
windows/amd64. `sysml-grpc` is not released; it is built from source.

`publish-github-release` uploads them with `ghr`, using a token from
`GITHUB_TOKEN`, `GH_TOKEN` or `CIRCLE_TOKEN` in the CircleCI project settings.
It runs with `-delete`, so re-running the workflow for the same tag replaces
that release's assets rather than appending duplicates.

## After the release

1. **Verify a download** on at least one platform:

   ```bash
   curl -fLO https://github.com/Open-MBEE/Systemica/releases/download/v0.0.4/systemica-linux-amd64.tar.gz
   curl -fLO https://github.com/Open-MBEE/Systemica/releases/download/v0.0.4/SHA256SUMS.txt
   sha256sum -c SHA256SUMS.txt --ignore-missing
   tar xzf systemica-linux-amd64.tar.gz && ./sysml --version
   ```

   `--version` must report the tag, not `dev`.

2. **Render the Homebrew formula** and commit it to the tap repository
   `Open-MBEE/homebrew-tap` (not this repository — the copy here is a template
   with `__VERSION__`/`__SHA256_*__` placeholders):

   ```bash
   scripts/render-homebrew-formula.sh v0.0.4 > Formula/systemica.rb
   ```

   See [packaging/homebrew/README.md](../packaging/homebrew/README.md).

3. **Say what is not signed.** macOS binaries are not Developer ID signed or
   notarized and Windows binaries are not Authenticode signed, so a browser
   download trips Gatekeeper or SmartScreen. Point release notes at
   [MACOS_DISTRIBUTION.md](MACOS_DISTRIBUTION.md), which gives the workarounds
   and what signing would take.
