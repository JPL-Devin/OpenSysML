---
name: testing-pilot-differential
description: How to verify the advisory pilot-implementation differential harness (cmd/pilot-diff + scripts/download-pilot-validator.sh) end to end on Linux — provisioning the DeciSym/pilot validator, reproducing the committed baseline, and the adversarial paths (bad pin, missing tools, wrong flags) worth checking.
---

# Testing the pilot differential harness (`cmd/pilot-diff`)

The harness compares OpenSysML diagnostics against the OMG SysML v2 Pilot Implementation
(via `DeciSym/sysmlv2-validator`) over four corpus roots and writes
`build/pilot-diff/pilot-diff.{txt,json}`. `docs/project/pilot-differential-baseline.json` is the
committed result of the *last refreshed* run, so **the harness is testable by reproduction** —
but only while the baseline is current. Check that first. As of `30449705` it **is** current:
a live run gives `349 file(s), 254 fully agreeing; 20 agreed, 343 only ours, 459 only the
pilot's`, byte-identical to the committed baseline, and `docs/project/pilot-differential.md`'s
"Results" table matches. When it is stale (it was at `ac4ac4fb`, and again while the F60–F69 fix
PRs were in flight), a non-empty `jq -S` baseline diff is *not* by itself evidence of a
regression — see "Isolating one change's effect" below.

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

Observed at `90da2cad` (KerML root added): `338 file(s), 196 fully agreeing; 20 agreed, 851 only
ours, 145 only the pilot's`, wall time ~70 s (the KerML batch costs ~50 s), byte-identical to the
committed baseline and across runs.

Observed at `82ff0fac` (F34, per-file language dispatch): `349 file(s), 222 fully agreeing; 20
agreed diagnostic(s), 564 only ours, 459 only the pilot's`, wall time ~82 s, byte-identical across
runs (both `.json` and `.txt`). The committed baseline is stale against this (338 / 221 / 20 / 560
/ 145), so use the entry-keyed delta below rather than `jq -S`.

When the baseline is stale, audit a change by comparing per-file *entries* keyed by
`(root.name, file.path)` — the whole entry value, not the whole file — so that added roots/files
and aggregate totals do not drown out the question you are asking (did any pre-existing verdict
move?). A run at `82ff0fac` gives 0 changed, 0 removed, 10 added
(`examples/parser_features_demo_*.kerml`). Files clean on both sides appear in no entry map at
all, so a newly-compared clean file shows up only as `filesFullyAgreeing +1`.

## Auditing a docs-only PR's numeric claims against the report

Adjudication PRs (e.g. #356, the S1–S10 / F60–F69 classes) assert per-root, per-category and
per-message counts. Derive every one of them from `build/pilot-diff/pilot-diff.txt`, never from
the doc. The JSON carries no per-diagnostic message, so the txt is the only source for message
and category counts. Parsing rules that matter:

- A root section starts with `<name> (<dir>)`; inside it a file block is `  <path>` and the
  diagnostic buckets are `    only OpenSysML (candidate false positives):`,
  `    only the pilot's ...`, `    agree...`, `    severity...`. Reset the current bucket on every
  new file line, or later files' pilot-only diagnostics leak into your only-ours totals.
- An entry line is `      line N  severity  category  xK` followed by **K** message lines. Count
  the K message lines, not the entry — otherwise multi-message lines (`x2`) undercount.
- Shortcut when a root has `pilotDiagnostics: 0` and `agreement: 0` (true for `pilot-examples`
  and `pilot-validation`): every `        opensysml:` line in that root's section is an only-ours
  diagnostic, so `grep -c` over the sliced section is an independent cross-check of the parser.
- **Agreement and severity-only must be counted by summing the `xK` multiplicities, not the
  message lines.** An agreement entry lists K `opensysml:` *and* K `pilot:` lines, so counting
  messages doubles it; the severity bucket's header is `same line and category, different
  severity:`, which a parser keyed on `only ...` silently mis-buckets.
- **Root file counts must come from `.roots[].totals`, not `len(.roots[].files)`** — the per-file
  array omits files both tools are silent on.
- Cross-check the doc's class table by summing its Files and Diags columns; they must equal the
  measured number of files carrying only-ours diagnostics (root `files` − `filesFullyAgreeing`)
  and the root only-ours totals.

Observed at `75672e91` (PR #356) — useful reference values: totals `338 / 221 / 20 agreed /
560 only ours / 145 only pilot`, byte-identical to the committed baseline; `pilot-examples` 314
and `pilot-validation` 59 = **373** over **73** of 154 files; categories syntax 274,
unresolved-reference 82, kind-mismatch 14, unmapped 2, units 1; and the two generic recovery
messages measured **102** `expected a body member` + **73** `expected a namespace member` = 175
(the doc claimed 74 + 64 = 138, so recovery-vs-finding splits in these docs are the claim most
likely to be stale — always recount).

## Single-file `-validate` is not the harness's loading model

The harness opens each corpus root as **one workspace batch** (import-topologically sorted, see
`order.go` / `openSysMLDiagnostics` in `opensysml.go`), so a corpus file that imports a sibling
file — e.g. `Vehicle Example/VehicleIndividuals.sysml`, which does `private import
VehicleUsages::*` from `VehicleUsages.sysml` in the same directory — reports unresolved-reference
errors under a bare single-file `bin/sysml -validate <f>` even when the differential shows it
fully agreeing. Before calling such a file a failure, either pass the sibling files on the same
`-validate` command line or check the file's entry in `build/pilot-diff/pilot-diff.json`
(`.roots[].files[]` omits only files both tools are silent on; a fully-agreeing file with shared
diagnostics is still listed with empty `openSysMLOnly`/`pilotOnly`/`severityMismatch` buckets, so
treat absence — or all-empty disagreement buckets — as full agreement, confirming by comparing
against the same file's entry in `docs/project/pilot-differential-baseline.json`).

## Running a doc's inline reproducers through both tools

Adjudication rows quote a reproducer instead of committing a fixture, so re-derive them:
write the snippet to a temp `.sysml`, then `go build -o /tmp/sysml ./cmd/sysml &&
/tmp/sysml -validate <f>` for our side and `build/pilot-validator/validate-sysml <f>` for the
reference (capture `$?` without a pipe). A row that claims "ours" needs our message present *and*
the pilot silent; if the pilot also errors, the row is wrong.

Pitfall: the pilot's grammar requires a visibility keyword on imports, so a reproducer with
`import ISQ::*;` fails on the *pilot* side with `mismatched input 'import' expecting '}'` plus
cascading `Couldn't resolve reference to Type` errors — which looks like the reference rejecting
the construct under test. Always write `private import ISQ::*;` (what the corpora do) and put
library-dependent snippets inside a named `package`.

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

## The KerML root and its bridge

`examples/pilot-corpora/kerml-examples` (58 `.kerml` files) is a root validated by
`build/pilot-kerml-validator/validate-kerml`, which drives the pilot's own `KerMLValidator`
through Xtext's `IResourceValidator` in **one** resource-set batch and prints GNU-format
diagnostics **relative to `--root`** (paths, not basenames — so no basename batching).
For the K6 `disjoint from` reproducer, the wrapper injects the library itself, so no
`--library` flag is needed: `validate-kerml --root /tmp/k6 /tmp/k6/Decl.kerml`.

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

Since F34, language is a per-file property (`source.KindOf`), so a root collects both extensions
and runs one reference invocation per language over all of that language's files. stderr prints one
line per language per root (`testdata: 10 SysML file(s)` then `testdata: 1 KerML file(s)`), and our
own `.kerml` fixtures under `testdata/` and `examples/` are compared.

The control for a dispatch change: a synthetic repo with byte-identical `testdata/adv.sysml` and
`testdata/adv.kerml`, run at HEAD and in a parent worktree (`-repo` plus absolute validator flags).
The two files getting *different* pilot messages is the proof that two oracles ran; the parent
run reporting only `adv.sysml` is the proof the delta belongs to the change.

## Testing language-scoped (`.sysml` vs `.kerml`) diagnostic behaviour

Some checks are gated on the document's language via `source.KindOf(name)` (e.g. the KerML
type tier in `internal/core/passes/typecheck.go`). **Which surface you observe from decides
whether you see it at all**, because only some surfaces analyse under the real file name:

| Surface | Document name passed to `passes.Analyze` | Language honoured? |
|---|---|---|
| `cmd/pilot-diff` (`opensysml.go`, `ws.Open(rel, ...)`) | corpus-relative path with extension | **yes** |
| `sysml-lsp` / `sysml-grpc` (`internal/core/model/workspace.go`) | the opened file's URI/path | **yes** |
| `cmd/sysml` / REPL (`internal/repl/session.go`) | the constant `"<repl>"` | **no** — `KindUnknown`, so SysML rules |

So `go run ./cmd/sysml foo.kerml` is **not** a valid way to observe KerML-only leniency: files
loaded on the command line are appended to one accumulated session buffer named `<repl>`, and
`typecheck.go` documents that an unknown-kind document deliberately reads as SysML. If a task
says "run the CLI over a .kerml file and confirm it is clean", expect it *not* to be clean and
flag the surface mismatch instead of reporting a bug in the type checker.

Two cheap surfaces that *do* prove the split:

1. **A synthetic two-language mini-repo through pilot-diff.** Put byte-identical content in
   `<tmp>/testdata/adv.sysml` and `<tmp>/examples/pilot-corpora/kerml-examples/adv.kerml`, then
   `go run ./cmd/pilot-diff -repo <tmp> -validator <abs>/build/pilot-validator/validate-sysml \
   -kerml-validator <abs>/build/pilot-kerml-validator/validate-kerml -out <tmp-out>`.
   The other five roots warn `skipping ...: no .sysml files` and are skipped, which is fine.
   Read `pilot-diff.txt`: the message must appear under `adv.sysml` and be absent under
   `adv.kerml`. Run the *same* command from the parent-revision worktree as the control —
   without it, an absent message proves nothing.
2. **A ~30-line stdio LSP driver.** `make build-lsp`, then spawn `bin/sysml-lsp`, send
   `initialize` (sleep ~2 s), `initialized`, and `textDocument/didOpen` with
   `uri: file:///tmp/x.kerml` (sleep ~4 s), and print the `textDocument/publishDiagnostics`
   payloads. `languageId` is irrelevant — the URI extension is what `KindOf` reads. This is the
   closest thing to the real editor-facing surface and takes seconds.

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
