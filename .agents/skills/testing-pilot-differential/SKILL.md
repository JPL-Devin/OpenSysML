---
name: testing-pilot-differential
description: How to verify the advisory pilot-implementation differential harness (cmd/pilot-diff + scripts/download-pilot-validator.sh) end to end on Linux — provisioning the DeciSym/pilot validator, reproducing the committed baseline, and the adversarial paths (bad pin, missing tools, wrong flags) worth checking.
---

# Testing the pilot differential harness (`cmd/pilot-diff`)

The harness compares OpenSysML diagnostics against the OMG SysML v2 Pilot Implementation
(via `DeciSym/sysmlv2-validator`) over four corpus roots and writes
`build/pilot-diff/pilot-diff.{txt,json}`. `docs/project/pilot-differential-baseline.json` is the
committed result of the last run, so **the harness is testable by reproduction**: any real
regression shows up as a non-empty `jq -S` diff against that baseline.

## Prerequisites

- Go on PATH (`export PATH=/usr/local/go/bin:$PATH`), `java` (>= 21), `mvn`, `git`, `jq`.
- The OMG training corpus must be present (`./scripts/download-training-examples.sh`, in the repo
  blueprint's maintenance step) — without it the `training` root prints
  `skipping examples/sysml-v2-training: no .sysml files (corpus not downloaded?)` and the totals
  silently drop from 122 files to 22, which looks like a harness bug but is a missing corpus.
- The validator: `./scripts/download-pilot-validator.sh`. It is **not** in the blueprint, takes
  ~2-3 minutes (Maven downloads the pilot release ZIP and shades a jar), and needs network access
  to github.com and Maven Central. Once built it no-ops.
- The KerML oracle: `./scripts/download-pilot-kerml-validator.sh` (needs `javac` too). It compiles
  `scripts/pilot-kerml-validator/ValidateKerML.java` against the pilot shaded jar into
  `build/pilot-kerml-validator/`, auto-provisioning the SysML validator first if the jar is
  missing. Seconds, not minutes — safe to `--force` rebuild during testing.

## The core check (fast, ~20 s per run)

```bash
rm -rf build/pilot-diff && go run ./cmd/pilot-diff        # ~19 s wall, ~1 min CPU
diff <(jq -S . docs/project/pilot-differential-baseline.json) \
     <(jq -S . build/pilot-diff/pilot-diff.json)          # must be empty
```

Run it twice and diff the two JSONs against each other to prove determinism (the import
topological sort in `order.go` and the basename batching in `pilot.go` are the parts that could
introduce order-dependence). Observed at `3b2d14a3`: byte-identical across runs, and identical
after a full validator rebuild from scratch — that last one is the strongest evidence available,
because it shows the numbers are a property of the pinned pilot release, not of one local build.

Observed at `711684cb` (KerML root added): `335 file(s), 196 fully agreeing; 20 agreed, 834 only
ours, 158 only the pilot's`, wall time ~70 s (the KerML batch costs ~50 s), byte-identical to the
committed baseline and across runs.

Check doc claims mechanically rather than by eye:

```bash
jq -c '.totals' build/pilot-diff/pilot-diff.json
jq -r '.roots[] | [.dir,.totals.files,.totals.filesFullyAgreeing,.totals.openSysMLDiagnostics,
        .totals.pilotDiagnostics,.totals.agreement,.totals.severityMismatch,
        .totals.openSysMLOnly,.totals.pilotOnly] | @tsv' build/pilot-diff/pilot-diff.json
jq -r '.unmapped[] | "\(.side)\t\(.count)\t\(.message)"' build/pilot-diff/pilot-diff.json
```

## The KerML root and its bridge

`examples/pilot-corpora/kerml-examples` (58 `.kerml` files) is a root validated by
`build/pilot-kerml-validator/validate-kerml`, which drives the pilot's own `KerMLValidator`
through Xtext's `IResourceValidator` in **one** resource-set batch and prints GNU-format
diagnostics **relative to `--root`** (paths, not basenames — so no basename batching).

Provisioning script checks that actually distinguish working from broken:

- Re-run with no args → `KerML validator already compiled at .../classes/ValidateKerML.class`,
  exit 0, and the `.class` mtime is unchanged. The **launcher is intentionally rewritten every
  run** (so a changed pin can never leave a stale jar path), so compare the `.class` mtime, not
  the launcher's.
- `--force` → prints `Compiling ...` and the `.class` mtime advances.
- `grep jupyter-sysml-kernel build/pilot-kerml-validator/validate-kerml` must show the pinned
  `PILOT_ARTIFACT_VERSION` from `scripts/pilot-pin.sh` (`0.60.1`) and no leftover
  `__PILOT_ARTIFACT_VERSION__` placeholder.
- `PILOT_ARTIFACT_VERSION=9.9.9-bogus ./scripts/download-pilot-kerml-validator.sh` → exit 1 with
  `error: pilot shaded jar not found at .../jupyter-sysml-kernel-9.9.9-bogus-all.jar`, and the
  existing launcher is left untouched (it does *not* silently reuse the stale jar). Note it first
  calls `download-pilot-validator.sh`, which no-ops in seconds when `build/pilot-validator` exists.
- `--bogus` → exit 1, `error: unknown option: --bogus (only --force is supported)`.

Oracle behaviour (`validate-kerml`, exit codes without a pipe):

| Input | Expected |
|---|---|
| a clean corpus file (`Address Book Example/AddressBookModel.kerml`) | exit 0, no `: error:` lines (only `log4j:WARN` noise on stderr) |
| a malformed `.kerml` | exit 1 with `<file>:<line>:<col>: error: no viable alternative at input ...` plus `Couldn't resolve reference to Type '...'` — the oracle must not be silent |
| no args | exit 2 + `usage: validate-kerml --library DIR [--root DIR] [--kernel-only] FILE...` |
| a `.sysml` file | exit 2 + `Error: File must have .kerml extension: <abs path>` |
| nonexistent file | exit 2 + `Error: File not found: <abs path>` |
| a directory | walks it for `.kerml` recursively and validates the batch |
| an empty directory | exit 0 + `Warning: No .kerml files found` |

Harness paths: `-kerml-validator /nonexistent` → exit 1,
`KerML pilot validator not found at /nonexistent: run ./scripts/download-pilot-kerml-validator.sh`,
no report written. The skip warning is language-aware (`no .kerml files` for that root). To
exercise a KerML-only run (e.g. a small `-timeout`), copy just
`examples/pilot-corpora/kerml-examples` into a temp dir and pass `-repo <tmp>` with absolute
`-validator`/`-kerml-validator`; otherwise the SysML roots time out first. Strongest
non-silence proof: drop a malformed `.kerml` into that copied corpus and confirm the JSON gains
pilot-side diagnostics for it (observed: 6 pilot-only + 1 agreed on that one file).

## Running the pilot validator directly

`build/pilot-validator/validate-sysml <file.sysml>` prints diagnostics on **stderr** in GNU
format (`<basename>:<line>:<col>: severity: message`) and library `Reading ...` noise on stdout.
Exit 1 means "the batch had errors"; anything else is the validator itself failing. **Capture the
exit code without a pipe** (`... > /tmp/out 2>&1; echo $?`, or `PIPESTATUS`), otherwise a pipeline
reports the exit of the last stage and a failure looks like a pass.

## Adversarial paths that actually distinguish working from broken

| Case | Expected at `3b2d14a3` |
|---|---|
| `-validator /nonexistent` | exit 1, `pilot validator not found at /nonexistent: run ./scripts/download-pilot-validator.sh` |
| `-out /tmp/pd-out` | exit 0, both reports written there, contents identical to the baseline |
| `-repo /tmp/notrepo` (default validator) | exit 1, validator-not-found under *that* repo |
| `-repo /tmp/notrepo -validator <abs path>` | exit **0**, one `skipping <root>: no .sysml files` warning per root, `files: 0` — warned but not fatal; worth flagging if a caller could mistake it for a clean run |
| `-timeout 1s` | exit 1, `... validate-sysml failed (signal: killed)`, no report written (the JVM needs ~10 s just to load the library) |
| `-repo <empty dir>` | exit 1, seven language-aware skip warnings then `no model files found under <dir>` |

Provisioning script (`scripts/download-pilot-validator.sh`):

- Already built → prints `Pilot validator already built at ...` and exits 0. Prove it did not
  rebuild by comparing `ls -l --time-style=full-iso build/pilot-validator/validate-sysml` before
  and after, not just by reading the message.
- Pin check → `mv build/pilot-validator /tmp/pv-backup` then
  `PILOT_TAG=9999-99 ./scripts/download-pilot-validator.sh`: it clones (fast) and then aborts with
  `error: <commit> builds against sysml.release.tag=2026-05, this repository pins 9999-99`
  *before* Maven runs (`build/pilot-validator/target` stays absent). Exit 1.
- Missing tools → `env PATH=/tmp/nomvn ./scripts/download-pilot-validator.sh` where `/tmp/nomvn`
  holds symlinks to `git`, `java`, `sed`, `bash`, `dirname`, `ls` but **not** `mvn` gives
  `error: mvn is required to build the pilot validator`. If you forget `dirname`, the script
  first emits `line 16: dirname: command not found` — an artifact of the stripped PATH, not a bug.

**Never `rm -rf build/pilot-validator` while testing** — `mv` it aside and move it back, since a
rebuild costs minutes. Watch out for the classic `mv /tmp/pv-backup build/pilot-validator` when
the target directory already exists again: that nests the backup at
`build/pilot-validator/pv-backup` instead of restoring it. `rm -rf` the *new* directory first (or
restore to a fresh path) and verify with `ls build/pilot-validator | tr '\n' ' '`.

## Recording

This is CLI work: record a maximized Konsole on `DISPLAY=:0` (see the "Recording setup" section
of `testing-sysml-repl/SKILL.md`). A single `go run ./cmd/pilot-diff` prints only four progress
lines and a summary, so pair every run with the `jq`/`diff` command that turns it into a visible
pass/fail line (`&& echo '... IDENTICAL'`), otherwise the video shows nothing checkable.

## Devin Secrets Needed

None. Network access to `github.com` and Maven Central is required only for re-provisioning.
