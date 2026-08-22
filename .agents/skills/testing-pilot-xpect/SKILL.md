---
name: testing-pilot-xpect
description: How to verify the advisory pilot Xpect oracle harness (cmd/pilot-xpect + scripts/download-pilot-xpect.sh) end to end on Linux — provisioning the pinned .xt suites, reproducing the committed baseline, proving determinism under -jobs concurrency, spot-checking the oracle's truthfulness against an independent surface, and the adversarial mutations worth trying.
---

# Testing the pilot Xpect oracle harness (`cmd/pilot-xpect`)

Third sibling of `testing-pilot-differential` and `testing-grammar-coverage` (same pin
`scripts/pilot-pin.sh`, same "committed artifact, testable by reproduction" shape, same
`build/`-only unvendored corpus). This one reads the OMG pilot's own Xpect `.xt` test files as
*data* — no Java, no Xtext — and adjudicates each **declared** expectation
(`errors`/`warnings`/`noErrors`/`linkedName`, and since wave 7F also `scope`) against our
front end. Only `exportedObjects` (1 assertion) is still `not adjudicated`.

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

Observed on `main` after the wave-6 round: `428 .xt file(s), 0 unparsed; 1261 assertion(s),
1326 expectation(s): 449 agree, 646 disagree, 0 unlocated, 231 not adjudicated`. After wave 7F
(scope adjudicated) plus the central #421 rebaseline: `522 agree, 803 disagree, 0 unlocated,
1 not adjudicated`; per-suite kerml 303/968/454/513/1, sysml 125/358/68/290/0. After wave 8F
(per-fixture declared resource set, foreign-diagnostic filter, `exportedObjects` adjudicated):
`564 agree, 762 disagree, 0 unlocated, 0 not adjudicated`, plus a new line
`19 diagnostic(s) another declared resource raised, not adjudicated against the file`; per-suite
kerml 457/511/0, sysml 107/251/0. The scope row and its classes are unchanged by 8F
(`73 agree / 157 disagree`, `other-paths 12 | extra-names 32 | missing-names 10 |
missing-and-extra 7 | library-names 96`). Read the live totals from the baseline rather than this
paragraph; it is an anchor, not the check.

**A wave that moves the verdicts must also rebaseline.** `cmp` against
`docs/project/pilot-xpect-baseline.json` is the only thing that catches a missed rebaseline:
`cmd/pilot-diff`'s `TestW6FXpectDocumentCountsMatchBaseline` only guards `pilot-xpect.md`
*against the baseline JSON*, so a branch that leaves **both** stale still passes `go test ./...`.
Always run the `cmp` explicitly and diff `jq .totals` of the two — this was a real finding on the
wave-8F branch (harness emitted 564/762/0 while the committed baseline and doc still held
522/803/1).

Cross-check every numeric table in `docs/project/pilot-xpect.md` (kind table, tolerance tables,
per-suite table, scope class table) against the fresh `.txt` — those tables are hand-maintained and
are the usual thing to go stale after a merge or rebaseline.
See `testing-pilot-differential/SKILL.md` for the complete mechanical-guard scope and review-only surfaces.

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

## Isolating a merge's effect on the adjudication (fast, covers all rows)

When a branch merges `main` and you must prove no verdict silently moved, do not re-spot-check by
hand: diff the *rows* of the old and new baselines. `git show <pre-merge-sha>:docs/project/pilot-xpect-baseline.json`
and compare keyed on `suites[].files[].path` + `rows[].line` + `rows[].kind`, comparing
`(verdict, tolerance, names, ours)`. If the two baselines are byte-identical (`cmp`), the merge
provably changed nothing in the oracle's output. Beware: `suites[].dir` is the **full corpus path**,
and agreeing rows are *pruned* from the JSON — never infer "agree" by matching on `dir`, and treat
"absent from JSON" as agree only after confirming the totals.

## Spot-checking the oracle's truthfulness (the failure mode that matters)

Agreeing rows are pruned from the baseline (`report.go: pruned`), so "absent from the baseline"
must be shown to mean *agree*, not *silently dropped*. Use `bin/sysml-lsp` over stdio as the
**independent** surface — it is the only easy one that honours the file's language and resolves
sibling files:

- `textDocument/definition` on the `linkedName` target text tells you which symbol we bound.
  Historical example, and the one that proved the surface works: at `19a3ce03`, in
  `testsuite/MemberNameTests_LocalNamedMember.kerml.xt`, `A_alias` (line 37, declared `test.A`)
  jumped to the `alias A_alias for A;` line → ours `test.A_alias`, which was the report's headline
  finding (an alias resolving to itself, 41 of the 43 `linkedName` disagreements). Wave 6 fixed it
  and that column is now 194 of 194, so the same probe today lands on `classifier <A_Id> A;`.
- `textDocument/publishDiagnostics` after `didOpen` reproduces `noErrors`/`errors` rows. Copy the
  `.xt` verbatim to a `.kerml` file (the XPECT notes are ordinary comments, so it is valid source)
  and, when the `XPECT_SETUP` `ResourceSet` names `/src/...` files, drop those beside it and pass
  the directory as `rootUri`. Confirmed exactly the reported single error
  `ambiguous reference: SamePackage::container (2 candidates)` at line 29 of
  `imports/global/DependencySamePackageName.kerml.xt`.
- `bin/sysml`'s REPL is **not** useful here: `%print` echoes the declaration and there is no
  meta-command that prints a resolved FQN, and `cmd/sysml` analyses under the name `<repl>` so the
  KerML language tier is not honoured (see `testing-pilot-differential`).
- **Probing a `scope` row:** copy the `.xt` to `Main.kerml` in a temp dir together with the `/src/`
  files its `XPECT_SETUP` `ResourceSet` names, insert `alias __pN for <name with '.' → '::'>;` into
  the *same namespace* as the anchor, and read `publishDiagnostics`: an in-scope name is clean, an
  out-of-scope one gives `unresolved reference: …`. `model.VisibleNames`/`ElementOnPath`/`FQNOf`/
  `ScopeAt` have no caller outside `cmd/pilot-xpect/scope.go` (grep to re-confirm), so the LSP
  really is an independent surface.
- **Caveat that will confuse you:** a scope "missing" name is missing from the *enumeration*, not
  from the resolver. `VisibleNames` truncates a path at the first repeated element, so e.g.
  `testsuite/ShadowingTests_CircleProblem2.kerml.xt:22` reports 3 missing names that all resolve
  fine through the LSP. The direction is conservative (it under-reports our agreement) and is
  documented in `pilot-xpect.md`; do not file it as a scoring bug.
- Each fixture loads exactly the resources its `XPECT_SETUP` `ResourceSet` names — its own body, the
  `/src/` files and the `/library*` copies — never our embedded stdlib in their place, so an
  independent LSP check must open the same set to be comparable.

### Auditing the foreign-diagnostic filter (`compare.go: foreignDiagnostic`)

A fixture's declared library set is usually incomplete, so unresolved references raised *for a
library file* arrive attached to the file under test. The harness drops them **by byte offset**:
`source.Span` carries no file identity, so "outside the fixture's model text" (past EOF, or inside a
note per `xtFile.Noted`) is the only signal. That filter can only make the comparison weaker, so
audit it rather than trusting the count. The cheap, decisive probe is a throwaway
`cmd/pilot-xpect/zz_tmp_*_test.go` (package `main`, delete it afterwards) that walks the corpus,
calls `loadResourceSet` + `ws.Diagnostics(main)` exactly as `compareFile` does, and for each
diagnostic where `foreignDiagnostic(f, off)` is true prints the offset, the fixture's own text at
that span, and the text at the *same span* in every declared resource. Provenance is proven when the
text at that offset in some declared `/library*` file is literally the identifier the message names.
At wave 8F the 19 drops split 14 past-EOF / 5 inside a note, and all 19 matched a library file
exactly (`Objects.kerml` → `Link`/`BinaryLink`/`Occurrence`, `Connections.sysml` →
`BinaryLinkObject::source`, `States.sysml` → `StatePerformance`, `Metadata.sysml` → `Metaobject`).
Pair it with a negative control: inject a genuine `part zzzProbe : ZZZ_no_such_type;` into a
fixture's *model body* — the new diagnostic must flip that fixture's `noErrors` row to `disagree`
and must **not** raise `foreignDiagnostics`.

`exportedObjects` (`exported.go`) is adjudicated as of 8F, so `notAdjudicated` is 0. Prove the
adjudicator is live in both directions on the sole fixture,
`indexing/NameEscape.kerml.xt`: adding a bogus `sysml::Package: ZZZ_bogus` line inside the note's
`---` fence must give `disagree … missing 1`, and adding a real `class zzz_extra {}` to the body
must give `disagree … extra 1 (sysml::Class: NameEscape::zzz_extra)`.

## Adversarial mutations (mutate a copy or restore from the `mv`d backup, then `diff -r`)

| Mutation | Expected |
|---|---|
| delete the `END_SETUP` line | file listed under `unparsed files` as `XPECT_SETUP block is not terminated by END_SETUP`, `filesUnparsed` +1, `files` still 428 |
| `XPECT_SETUP` → `XPECT_SETUPX` | `no XPECT_SETUP block` |
| turn `// XPECT errors …` into `//* XPECT errors …` with no `*/` | `line N: `//* XPECT errors` note is not terminated` |
| drop one declared name from an *agreeing* `scope` note | that row flips to `disagree(extra-names)` with `extra 1` — proves the set comparison is live |
| add `ZZZ_not_a_name` to an agreeing `scope` note | `disagree(missing-names)` naming it, and *not* `other-paths`/`library-names` |
| add a real member (`class zzz_extra {}`) to a `library-names` fixture's namespace | class moves to `extra-names` — proves the `self`/`that` filter covers *all* differences, not just the 5 names the row text samples |
| delete a `/src/` file named by an `XPECT_SETUP` `ResourceSet` | totals report `N missing declared resource(s)` and list the declaring fixtures; assertion/expectation counts stay 1261/1326 — it must not silently agree |
| `END_SETUP` → `END_SETUPX` | `XPECT_SETUP block is not terminated by END_SETUP` — the terminator is the token `\bEND_SETUP\b` (`xt.go`), so a suffixed keyword does not terminate. Locked by `w5c_xt_test.go`; a plain substring search here was the one real defect testing found. |

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

`go run ./cmd/pilot-diff` (~1m12s) must still print the headline the *committed* baseline holds —
after the wave-8G rebaseline that is `353 file(s), 308 fully agreeing; 23 agreed diagnostic(s), 119
only ours, 137 only the pilot's`. Read the number out of
`docs/project/pilot-differential-baseline.json` rather than trusting this line, since a landing fix
round moves it. When the baseline is itself stale (it was at `19a3ce03`, holding 273 / 281 / 317), a
failing `cmp` against it is *not* evidence of an Xpect regression — compare the summary line, and see
`testing-pilot-differential`'s "Isolating one change's effect" for the entry-keyed delta.

## Recording

Shell-only; no GUI is involved, so no recording is needed unless you deliberately drive a Konsole.

## Devin Secrets Needed

None. Network access to github.com is required only to provision the suites.
