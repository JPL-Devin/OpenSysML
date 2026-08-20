---
name: testing-pilot-differential
description: How to verify the advisory pilot-implementation differential harness (cmd/pilot-diff + scripts/download-pilot-validator.sh) end to end on Linux — provisioning the DeciSym/pilot validator, reproducing the committed baseline, and the adversarial paths (bad pin, missing tools, wrong flags) worth checking.
---

# Testing the pilot differential harness (`cmd/pilot-diff`)

The harness compares OpenSysML diagnostics against the OMG SysML v2 Pilot Implementation
(via `DeciSym/sysmlv2-validator`) over four corpus roots and writes
`build/pilot-diff/pilot-diff.{txt,json}`. `docs/project/pilot-differential-baseline.json` is the
committed result of the *last refreshed* run, so **the harness is testable by reproduction** —
but only while the baseline is current. Check that first: as of `ac4ac4fb` the committed baseline
is **stale** (it records 276 files / 175 fully agreeing / 18 agreement / 457 ours / 188 pilot's,
while a live run gives 277 / 186 / 20 / 395 / 152), and `docs/project/pilot-differential.md`'s
"Results" table and "Unmapped messages" table match the stale baseline, not the harness. So a
non-empty `jq -S` baseline diff is *not* by itself evidence of a regression — see "Isolating one
change's effect" below.

## Prerequisites

- Go on PATH (`export PATH=/usr/local/go/bin:$PATH`), `java` (>= 21), `mvn`, `git`, `jq`.
- The OMG training corpus must be present (`./scripts/download-training-examples.sh`, in the repo
  blueprint's maintenance step) — without it the `training` root prints
  `skipping examples/sysml-v2-training: no .sysml files (corpus not downloaded?)` and the totals
  silently drop from 122 files to 22, which looks like a harness bug but is a missing corpus.
- The validator: `./scripts/download-pilot-validator.sh`. It is **not** in the blueprint, takes
  ~2-3 minutes (Maven downloads the pilot release ZIP and shades a jar), and needs network access
  to github.com and Maven Central. Once built it no-ops.

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

## Isolating one change's effect (works even with a stale baseline)

The reliable control is to run the *parent commit's* harness code over the *same* corpora and
diff the two JSONs:

```bash
git worktree add /tmp/wt-base <parent-sha>
(cd /tmp/wt-base && go run ./cmd/pilot-diff -repo /path/to/real/checkout -out /tmp/pd-base)
go run ./cmd/pilot-diff -out /tmp/pd-head
diff <(jq -S . /tmp/pd-base/pilot-diff.json) <(jq -S . /tmp/pd-head/pilot-diff.json)
```

`-repo <real checkout>` is what makes this work: it points the corpus roots (and the default
validator path) at the provisioned checkout. **Do not** try to symlink `examples/sysml-v2-training`
or `examples/pilot-corpora` into the worktree — the walker does not follow symlinks, so the roots
silently report `skipping ...: no .sysml files` and the file count drops (277 → 177), which looks
like a regression but is the missing corpus.

Check doc claims mechanically rather than by eye:

```bash
jq -c '.totals' build/pilot-diff/pilot-diff.json
jq -r '.roots[] | [.dir,.totals.files,.totals.filesFullyAgreeing,.totals.openSysMLDiagnostics,
        .totals.pilotDiagnostics,.totals.agreement,.totals.severityMismatch,
        .totals.openSysMLOnly,.totals.pilotOnly] | @tsv' build/pilot-diff/pilot-diff.json
jq -r '.unmapped[] | "\(.side)\t\(.count)\t\(.message)"' build/pilot-diff/pilot-diff.json
```

The JSON carries no message text per entry, so to check a *category* claim for one message
(e.g. "`Must invoke ...` is now `kind-mismatch`, not `unmapped`") read `pilot-diff.txt`: it lists
`line N  severity  category  xK` followed by the messages, grouped under the file path. Combine
both: the message must be absent from the JSON `unmapped[]` list *and* present under the new
category in the txt, with `totals.pilotOnly` unchanged (a category move must not create an
agreement).

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
