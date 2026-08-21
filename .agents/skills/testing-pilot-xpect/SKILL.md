---
name: testing-pilot-xpect
description: How to verify the advisory pilot Xpect oracle harness (cmd/pilot-xpect + scripts/download-pilot-xpect.sh) end to end on Linux — provisioning the pinned .xt suites, reproducing the committed baseline, proving determinism under -jobs concurrency, spot-checking the oracle's truthfulness against an independent surface, and the adversarial mutations worth trying.
---

# Testing the pilot Xpect oracle harness (`cmd/pilot-xpect`)

Third sibling of `testing-pilot-differential` and `testing-grammar-coverage` (same pin
`scripts/pilot-pin.sh`, same "committed artifact, testable by reproduction" shape, same
`build/`-only unvendored corpus). This one reads the OMG pilot's own Xpect `.xt` test files as
*data* — no Java, no Xtext — and adjudicates each **declared** expectation
(`errors`/`warnings`/`noErrors`/`linkedName`; `scope` is counted but not adjudicated) against our
front end.

## Prerequisites

- `export PATH=/usr/local/go/bin:$PATH`, `git`, `jq`, network to github.com. No Java, no Maven —
  unlike `pilot-diff`, this harness runs nothing from the pilot.
- `./scripts/download-pilot-xpect.sh` — ~10 s sparse blobless clone into
  `build/pilot-xpect-corpus/{kerml,sysml}`, gitignored. Expect **303 KerML + 125 SysML = 428**
  `.xt` files. Not in the blueprint; re-provisioning from scratch is cheap, so always test it.
- `make build` (for `bin/sysml-lsp`) if you want the independent spot-check surface below.

## The core check (~5 s per run)

```bash
rm -rf build/pilot-xpect && go run ./cmd/pilot-xpect
cmp build/pilot-xpect/pilot-xpect.json docs/project/pilot-xpect-baseline.json   # must be silent
```

Observed at `19a3ce03`: `428 .xt file(s), 0 unparsed; 1261 assertion(s), 1326 expectation(s):
382 agree, 713 disagree, 0 unlocated, 231 not adjudicated`, byte-identical to the committed
baseline, and the numeric tables in `docs/project/pilot-xpect.md` match it (per-suite kerml
303/968/319/418/231, sysml 125/358/63/295/0; 10 ignored XPECT-shaped fragments; 0 missing
resources).

**Determinism under concurrency is the main risk** (`compareAll` in `main.go` fans N goroutines
into one shared result slice). Test it explicitly, not just by repeating the default run:

```bash
for j in 1 3 16 0; do go run ./cmd/pilot-xpect -jobs $j -out /tmp/xj$j; done
for j in 1 3 16 0; do cmp /tmp/xj$j/pilot-xpect.json docs/project/pilot-xpect-baseline.json; done
```

`-jobs 0` is clamped to 1. Compare the `.txt` too, not only the `.json`.

## Provisioning checks that distinguish working from broken

- Move, do not delete: `mv build/pilot-xpect-corpus /tmp/xpect-corpus-backup`. After a fresh
  download, `diff -r /tmp/xpect-corpus-backup build/pilot-xpect-corpus` must be empty — that is the
  strongest available evidence that the tag is pinned rather than a moving branch.
- Idempotence: second run prints `Suite already present at ...` twice, exit 0. Prove it by
  `stat -c '%n %Y'` on the two suite dirs before/after, not by the message.
- Bogus pin: `PILOT_TAG=9999-99 ./scripts/download-pilot-xpect.sh` → git `fatal: Remote branch
  9999-99 not found`, **exit 128** (not 1 — it is git's status under `set -e`), and
  `build/pilot-xpect-corpus` is left absent (everything is staged in a `mktemp -d` and only `mv`d
  in at the end). Check for a half-populated corpus with `find … -name '*.xt' | wc -l` = 0.
- Nothing vendored: `git status --porcelain` clean after every step.

## No-corpus degradation

`mv` the corpus aside, then `go run ./cmd/pilot-xpect -out /tmp/nocorpus` → **exit 1**, stderr
`skipping kerml: build/pilot-xpect-corpus/kerml is absent (run scripts/download-pilot-xpect.sh)`
(same for sysml) then `pilot-xpect: no suite found under build/pilot-xpect-corpus; …`, and
`/tmp/nocorpus` is never created. It must never exit 0 or report 100 % agreement.

## Spot-checking the oracle's truthfulness (the failure mode that matters)

Agreeing rows are pruned from the baseline (`report.go: pruned`), so "absent from the baseline"
must be shown to mean *agree*, not *silently dropped*. Use `bin/sysml-lsp` over stdio as the
**independent** surface — it is the only easy one that honours the file's language and resolves
sibling files:

- `textDocument/definition` on the `linkedName` target text tells you which symbol we bound.
  Example at `19a3ce03`: in `testsuite/MemberNameTests_LocalNamedMember.kerml.xt`, `A_alias`
  (line 37, declared `test.A`) jumps to the `alias A_alias for A;` line → ours `test.A_alias`,
  confirming the report's headline finding (an alias resolves to itself, 41 of 43 linkedName
  disagreements); the sibling note at line 39 (`at A_Id --> test.A`) jumps to `classifier <A_Id> A;`
  → a genuine agreement.
- `textDocument/publishDiagnostics` after `didOpen` reproduces `noErrors`/`errors` rows. Copy the
  `.xt` verbatim to a `.kerml` file (the XPECT notes are ordinary comments, so it is valid source)
  and, when the `XPECT_SETUP` `ResourceSet` names `/src/...` files, drop those beside it and pass
  the directory as `rootUri`. Confirmed exactly the reported single error
  `ambiguous reference: SamePackage::container (2 candidates)` at line 29 of
  `imports/global/DependencySamePackageName.kerml.xt`.
- `bin/sysml`'s REPL is **not** useful here: `%print` echoes the declaration and there is no
  meta-command that prints a resolved FQN, and `cmd/sysml` analyses under the name `<repl>` so the
  KerML language tier is not honoured (see `testing-pilot-differential`).
- `/library*` resources are deliberately replaced by our embedded stdlib, so a single-file LSP check
  is faithful for suites whose setup names only library files plus `ThisFile`.

## Adversarial mutations (mutate a copy or restore from the `mv`d backup, then `diff -r`)

| Mutation | Expected |
|---|---|
| delete the `END_SETUP` line | file listed under `unparsed files` as `XPECT_SETUP block is not terminated by END_SETUP`, `filesUnparsed` +1, `files` still 428 |
| `XPECT_SETUP` → `XPECT_SETUPX` | `no XPECT_SETUP block` |
| turn `// XPECT errors …` into `//* XPECT errors …` with no `*/` | `line N: `//* XPECT errors` note is not terminated` |
| **`END_SETUP` → `END_SETUPX`** | **not detected** — `xt.go` finds the terminator with a plain `strings.Index(text, "END_SETUP")`, so a corrupted/suffixed keyword still counts as terminated. Use a deleted line, not a renamed one, when you want the unparsed path. |

An unparsed file lowers the assertion/row/disagree counts (it is counted, not compared), so always
restore and re-`cmp` against the baseline afterwards.

## Test-suite liveness

```bash
go test -count=1 ./cmd/pilot-xpect
OPENSYSML_REQUIRE_PILOT_XPECT=1 go test -count=1 ./cmd/pilot-xpect
```

With the corpus moved aside the plain run **skips** (`ok`, 0.001 s) and the `REQUIRE` run **fails**
with `build/pilot-xpect-corpus/kerml is absent; run scripts/download-pilot-xpect.sh`. Prove the
census in `w5c_census_test.go` is live two ways: perturb one pinned triple (e.g. kerml
`kindErrors: {295, 17, 328}` → `329`) → fails naming that kind; and perturb the *reader*
(narrow `xpectLineRe`'s `[ \t]` classes to `[ ]`) → fails with collapsed counts
(`errors = [14 0 14]`). Revert both and `git update-index --refresh` before checking
`git status --porcelain` — plain `git status` can report a stat-dirty file after `cp`-restores.

## Regression neighbour

`go run ./cmd/pilot-diff` (~1m12s) must still print `349 file(s), 283 fully agreeing; 20 agreed
diagnostic(s), 232 only ours, 142 only the pilot's`. Note that at `19a3ce03`
`docs/project/pilot-differential-baseline.json` is **stale** against that (it holds 273 / 281 /
317), so `cmp` against it fails for reasons unrelated to any Xpect work — compare the summary line,
and see `testing-pilot-differential`'s "Isolating one change's effect" for the entry-keyed delta.

## Recording

Shell-only; no GUI is involved, so no recording is needed unless you deliberately drive a Konsole.

## Devin Secrets Needed

None. Network access to github.com is required only to provision the suites.
