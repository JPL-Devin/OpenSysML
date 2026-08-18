---
name: testing-homebrew-packaging
description: How to verify the OpenSysML Homebrew formula end to end on Linux — rendering packaging/homebrew/Formula/opensysml.rb with scripts/render-homebrew-formula.sh, installing/testing/auditing it through a throwaway local tap, and the adversarial paths worth checking.
---

# Verifying the OpenSysML Homebrew formula (Linux)

`packaging/homebrew/Formula/opensysml.rb` is a **template** with `__TAG__` / `__SHA256_*__`
placeholders. `scripts/render-homebrew-formula.sh <tag>` substitutes them from the release's
`SHA256SUMS.txt` and strips the maintainer header. The tap repo `Open-MBEE/homebrew-tap` does
not exist yet, so everything below uses a throwaway **local** tap.

## Prerequisites

- Homebrew is not preinstalled by the repo blueprint. On this box it lives at
  `/home/linuxbrew/.linuxbrew/bin/brew` — `export PATH=/home/linuxbrew/.linuxbrew/bin:$PATH`.
  If it is absent, install it (`/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`).
- Network access to `github.com` is required (release archives + `brew audit --online`).
- No Go build is needed: the formula installs **prebuilt release binaries**, so this path is
  independent of `make build`.

## Always start from a clean slate

A previous run leaves state that makes a rerun a no-op and silently proves nothing:

```bash
brew uninstall opensysml 2>/dev/null
brew untap local/<previous-tap-name> 2>/dev/null
which sysml   # must print nothing before you begin
brew tap      # confirm no leftover local/* taps
```

Use a **fresh** tap name each run (`local/pr73check`, not a reused one) so `tap-new` itself is
exercised. First `brew install` pulls ~12 dependencies (gcc, glibc, binutils…) and takes
2–4 minutes; budget for it and answer the `[y/n]` prompt (or `--yes`).

## The verification recipe (mirrors packaging/homebrew/README.md)

```bash
./scripts/render-homebrew-formula.sh v0.0.4 > /tmp/opensysml.rb
brew tap-new local/<fresh> --no-git
cp /tmp/opensysml.rb "$(brew --repository local/<fresh>)/Formula/opensysml.rb"
brew install local/<fresh>/opensysml && brew test local/<fresh>/opensysml
brew audit --strict --online local/<fresh>/opensysml
```

All four must exit 0. Benign noise to ignore: `fatal: ambiguous argument
'refs/remotes/origin/main'` during auto-update, `Sandbox unavailable: building without
sandboxing!`, and ``AllCops/UseProjectIndex` is enabled but the `rubydex` gem is not installed``.

## Assertions that actually discriminate

- **No `version` line.** Homebrew scans the version from the tag in the release URL;
  `brew audit --strict` fails with `` `version 0.0.4` is redundant with version scanned from
  URL``. Prove the removal matters with a **counterfactual**: re-insert the line into the tap
  copy and rerun the audit — it must fail with exactly that wording, then restore the file.
  Just seeing a green audit does not prove the fix.
- **Checksums match the release**, not merely "look like hashes". Download
  `https://github.com/Open-MBEE/OpenSysML/releases/download/<tag>/SHA256SUMS.txt` and compare
  the four `opensysml-<os>-<arch>.tar.gz` entries against the `url`/`sha256` pairs parsed out
  of the rendered file. Note the manifest also contains per-binary `sysml-*` / `sysml-lsp-*`
  entries — the formula only uses the `opensysml-*` bundles.
- **Zero placeholders**: `grep -c '__[A-Z0-9_]*__' /tmp/opensysml.rb` → `0`. The script has its
  own guard for this (exit 1), so a rendering regression usually surfaces as a script failure.
- **The installed binaries are the brew ones**: check `which sysml` resolves under
  `/home/linuxbrew/...`, not `./bin/sysml`, before smoke-testing. `sysml --version` must print
  the release tag (`sysml v0.0.4`), not a dev/commit string.

## Adversarial paths (all cheap, all worth running)

| Command | Expected |
|---|---|
| `render-homebrew-formula.sh v9.9.9` | exit 22, `curl: (22) ... error: 404`, no `class OpenSysML` in output |
| `render-homebrew-formula.sh <tag> <sums-with-one-line>` | exit 1, `error: no checksum for opensysml-darwin-amd64.tar.gz` |
| `render-homebrew-formula.sh` (no args) | exit 2, `usage: ... <tag> [SHA256SUMS.txt]` |
| `brew install` of the **unrendered** template | must FAIL — download URL contains literal `__TAG__` and 404s |

Rehearse the 404 case sparingly: repeated unauthenticated GitHub hits can turn it into a
misleading `HTTP Error 403: rate limit exceeded`. Report the 404 wording, not the 403.

## Homebrew 6.x tap trust

Homebrew 6 requires tap trust. Taps you create locally with `brew tap-new` are trusted
automatically (`==> Trusted formula local/<tap>/opensysml`), but installing from tap B will warn
that tap A "is not trusted" if A is still tapped. That warning is cosmetic for this workflow;
untap old taps to silence it, or `brew trust local/<tap>`.

## Recording

This is CLI work, so record a real terminal: see the "Recording setup" section of
`.agents/skills/testing-sysml-repl/SKILL.md` (Konsole on `DISPLAY=:0`, `ctrl+plus` to enlarge
the font, `wmctrl` to maximize). `clear` between steps keeps each assertion legible on camera.

## Devin Secrets Needed

None. All release assets are public; `brew audit --online` works unauthenticated.
