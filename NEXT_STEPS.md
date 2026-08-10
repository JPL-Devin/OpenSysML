# Systemica — NEXT_STEPS

Baseline: `main` @ `7bb77cc` (PRs #35–#43 all merged), verified locally on 2026-08-10 with Go 1.25.0.

## Where the branch actually stands

Full gate green: `gofmt -l .` empty, `go build ./...`, `go vet ./...`, `go test ./...`.

| Gate | Count | Was at `6bd3ea4` |
|---|---|---|
| OMG training corpus | **97/100 clean** — 3 files, 5 errors | 88/100 |
| Stdlib parser conformance | 94/94 clean | 94/94 |
| Execution conformance cases | 52 | 51 |
| Golden execution traces | 22 | 22 |
| Runtime robustness subtests | 31 | 29 |
| gRPC conformance cases | 5 | 5 |
| Golden AST fixtures | 34 | 30 |
| Negative parser subtests | 39 | 32 |

Every number above was re-measured from `-v` output on `7bb77cc`, not carried over.

**The corpus gate is now reproducible from cold.** #43 made the test set its own empty
`XDG_CACHE_HOME` per run, so it runs cold by construction and reports 97/100 on the first and every
subsequent run. The "run it twice" workaround is dead — drop it from any prompt or playbook that still
carries it.

### What the last batch did

Nine concurrent sessions, all merged:

| PR | Change | Corpus |
|---|---|---|
| #35 | withdraw subsetting type-conformance and end-less-flow errors | 95 → 97 |
| #36 | recount the gate numbers in every doc from measured output | docs only |
| #37 | consult imports owned by a definition/usage body during resolution | 88 → 89 |
| #38 | an individual definition is an occurrence definition | 89 → 90 |
| #39 | parse an entry/do/exit action given by reference | — |
| #40 | implicit redefinition of connector ends by position (§7.13.2) | 90 → 93 |
| #41 | inherit metadata features, apply semantic-metadata user keywords | 93 → 95 |
| #42 | give an `expose` the import kind and visibility the spec assigns it | — |
| #43 | expand chained wildcard imports on the parse path | cold 86 → 97 |

Batch A is finished. Every corpus task from the previous NEXT_STEPS landed, and #40 and #41 went well
beyond their brief — #40 also fixed four parser bugs around `end` modifiers and multiplicity and added a
general redefinition path to `resolveRedefinition`/`ResolveReference`; #41 root-caused cached library
records dropping their specialization edges.

### The 5 remaining corpus errors

| File | n | Cause |
|---|---|---|
| `27. Occurrences/Time Slice and Snapshot Example` | 2 | OMG bug: `start`/`done` should be `startShot`/`endShot` |
| `28. Individuals/Individuals and Time Slices` | 2 | same OMG bug |
| `33. Analysis/Trade Study Analysis Example` | 1 | ours: `objective cannot be typed by requirementDef` (E2) |

Only two files are now genuinely pinned OMG bugs, so **the ceiling is 98/100** and exactly one of our own
false positives remains. The `alternative` → `alternatives` typo that used to be pinned is no longer one:
#40's implicit redefinition matches it correctly.

Do not re-baseline `training_examples_expected.txt`; adjudicate each drifted file and record the verdict
in `docs/TRAINING_EXAMPLES.md`.

---

# Wave 1 — done (C1, E2, F1, F2)

Four concurrent children, disjoint file sets, plus one maintainer PR picking up a child's finding.
All five green; verified merged together locally at **98/100** with `SYSTEMICA_REQUIRE_TRAINING_CORPUS=1`.

| PR | Task | Corpus |
|---|---|---|
| #47 | C1 — CI downloads the corpus and gates on it (no longer skips) | 97, now CI-verified |
| #46 | F2 — named n-ary `connect` does **not** reproduce; regression tests at every layer | 97 |
| #49 | maintainer — anonymous inline `connect (a, b, c);` really did fail to parse | 97 |
| #48 | F1 — `individual`/`snapshot` stored in the AST and consulted by the kind tables | 97 |
| #45 | E2 — an `objective` is typed by a `requirement def` | 97 → **98** |

Merge order **#47 → #46 → #49 → #48 → #45**; #44 is independent. #49 is based on #46's branch. One
clerical conflict, #48 vs #46/#49 in `internal/core/parser/negative_test.go` (both append to the same
table), resolution verified, no rebase needed.

**C1 changes the working rule on the expectations file.** While the gate was invisible to CI, children
were told never to commit `internal/core/model/testdata/training_examples_expected.txt`. Now that the
gate is a required check, a PR that leaves it uncommitted goes red the moment it lands. The rule is:

> A PR that moves the corpus **regenerates and commits the expectations file in that same PR**, and
> corpus-moving PRs are **serialised** — only one in flight at a time.

The original no-commit rule was right for its situation (seven children editing one file against a
moving baseline) and is wrong for this one. Review the regenerated diff: any entry other than the one
the PR intends to move is a finding to report, not to commit.

---

# Batch E — reproducibility and CI

## E1 — cold-cache corpus reproducibility ✅ done (#43)

The cold run reported 86/100 while the warm run reported 97/100. The cache was not at fault: on the
parse path `symbols.Index.ExpandWildcardImports` propagated wildcard re-exports only **one level per
call** (`KerML` imports `Kernel::*` imports `Core::*` imports `Root::*`), and `resolveWildcardTarget`
resolved a relative target only against the importing package and the global namespace, so
`KerML::Core`'s `import Root::*` never found its sibling `KerML::Root` and `KerML::Element` was never
registered. Each cached run re-ran the pass and filled in one more link, which is exactly why the second
run looked better. Separately, parsed symbols did not record their short name, producing the cold-run
`unresolved reference: kg`.

Fixed by iterating the expansion to a fixpoint and searching enclosing namespaces outward (KerML
§8.2.3.5). The gate now sets its own `t.TempDir()` cache, so it runs cold by construction.

This was a real correctness bug on any fresh machine, not a test artefact — the LSP and REPL take the
same path — and it went undetected for the whole of Batch A. It left one new ⚠️ row, tracked as B5.

## C1 — CI, and the corpus download (blocking, maintainer) ✅ done (#47)

CI now downloads the corpus, caches it, verifies 100 files are present, and runs a dedicated corpus-gate
step that fails on `--- SKIP`. `SYSTEMICA_REQUIRE_TRAINING_CORPUS=1` makes an absent corpus an error
rather than a silent pass; the local-developer skip path is kept. The CI log now prints
`97/100 training files clean` instead of skipping.

Original write-up:

CI does not run `./scripts/download-training-examples.sh`, so the corpus gate **silently skips there**.
Ten PRs have now merged on local evidence alone. E1 was the blocker; the gate is cold-safe, so add the
download step. Do this before Batch B so the rest of the work is actually verified by CI.

## E2 — the last corpus false positive: an `objective` typed by a `requirement def` ✅ done (#45)

Landed with the strict reading, approved by the maintainer: an `objective` accepts only
requirement-definition kinds, so `objective o : Vehicle` on a `part def` now reports where it previously
did not. `subject` was left on the structural kinds. Corpus 97 → 98, the ceiling.

Original write-up:

1 error, `33. Analysis/Trade Study Analysis Example.sysml`. The file writes `objective : MaximizeObjective;`
and the standard library declares `requirement def MaximizeObjective :> TradeStudyObjective`
(`Domain Libraries/Analysis/TradeStudies.sysml:98`), so the type is the one the library intends. The
`UsageObjective` row of the kind table in `passes/typecheck.go` accepts only the structural definition
kinds, not `requirementDef` — an objective *is* a requirement.

Narrow the rule, do not delete the check; a negative test must still reject a genuinely incompatible
pair. Takes the corpus to its 98/100 ceiling. Small and well isolated.

---

# Batch B — semantics and runtime depth

Batch A is done, so B is unblocked. These are the ⚠️ and ❌ rows left in `docs/SPEC_COMPLIANCE.md`.
Each is its own session with the §5.2 four-layer contract.

## B1 — runtime approximations

Roughly in descending order of value:

- **Accept does not suspend.** `action_executor.go` reports `ErrNoMatchingMessage` where SysML suspends
  the action until a matching message arrives. Still the largest behavioral deviation; needs a real
  waiting state in the executor plus a deadlock/step-budget story.
- **Port routing ignores direction and conjugation.** A message reaches every port connected by a
  connector in the same behavior body, and a port of the enclosing part is invisible to the behavior.
- **Accept-parameter visibility.** The payload is bound in shared token data, which scoping does not
  model, so a sibling node reading it by simple name reports unresolved.
- **Transition endpoint names** are deferred to `lower/state_graph.go`, so a misspelled endpoint surfaces
  at lowering instead of at the name-resolution tier. Error timing is part of the contract (AGENTS.md §4)
  — moving it is the point of the task, and the affected tests must be updated deliberately.
- **Dangling transition detection is lenient.** UML 2.5.1 §14.2.3.9 requires exactly one source and one
  target vertex per transition; today an unresolvable target is tolerated at lowering and becomes a
  runtime no-op. Hard cases worth naming in the prompt: a target in a sibling orthogonal region (legal),
  a target in an unrelated machine (illegal), an entry/exit point on a composite state (legal), a target
  resolving to a non-vertex such as an attribute (illegal), the sourceless `accept … then` nested form
  (legal), and a junction chain terminating nowhere (illegal, and not a cycle).
- **Calc recursion** is depth-bounded and rejected rather than evaluated.

## B2 — naming rules still open

- Effective name of an unnamed redefining feature (`in item;` in `action shoot : Shoot` is named
  `image`), KerML §7.3.4.5 / SysML §7.6.5 — still ❌. The anonymous parameter redefines the right
  feature, but its name is not bound in the owning scope.
- A redefinition as a naming feature — today only a reference subsetting's target names an unnamed
  feature. The ⚠️ left by PR #33.
- A nested non-parameter usage sharing a name with an inherited feature is a name conflict per SysML
  §7.6.1 / KerML §7.3.2.1 and should be *diagnosed*. PR #34 correctly stopped treating it as an implicit
  redefinition but did not add the conflict diagnostic.

## B5 — a privately wildcard-imported name is still reachable by qualified reference (new, from #43)

`⚠️ A private import X::* is not re-exported by its namespace (KerML 8.2.3.3)`. The privately imported
name is correctly not carried on to a package importing the namespace, but it is still registered under
the namespace's own FQN, so a direct qualified reference resolves to it. `resolve/alias.go`
`resolveCachedAliasTarget` currently *relies* on that to follow an alias of a privately imported type,
so closing the hole means giving aliases another way to reach their target — which is why #43 stopped
here rather than expanding scope. Read that PR's thread before starting.

## B4 — visibility rows opened by #42

The expose work landed two honest ❌s that should not be left to rot:

- **Protected imports are treated as private.** SysML v2 §7.5.3 makes a protected import visible in
  specializations of the importing definition or usage; today its members resolve in the owning body
  only. This is the general visibility rule, not an expose quirk.
- **`validateExposeOwningNamespace`.** An `expose` in a non-view definition or usage body is parsed and
  resolved rather than diagnosed; the spec requires the importOwningNamespace of an Expose to be a
  ViewUsage.

Do the protected-import one first — the expose validation is a small diagnostic on top of it.

## B3 — implicit library import (do this LAST)

`❌ Unqualified library names in files that do not import their library`. Deferred again for the same
reason: it can mask corpus regressions by making unresolved references resolve for the wrong reason.
Land it only after E2, when the corpus is at its 98/100 ceiling, and gate it with a **file-by-file**
diff of the corpus before and after, not the total. E1 having landed, that diff is now trustworthy.

---

# Batch F — parser bugs surfaced by Batch A (new)

Found by children that correctly stopped rather than expanding scope.

## F1 — the `individual` modifier is dropped at parse time ✅ done (#48)

`ast.Usage` gained `IsIndividual`/`IsSnapshot`, both parser paths store them, and the kind tables consult
them. A review round caught a second hole: `individual part p;` still dropped the modifier because
`parseFeatureModifiers` returned early on `individual <usageKind>`; kind and modifier are now both kept
in bodies, at top level and in parameters.

**Known limitation, not a task.** The modifier is not reflected in the usage's *symbol* kind, so
`individual testSystem` still indexes as an attribute usage. This is honest and is compensated by the
typecheck widening; it is recorded as a ⚠️ row in `docs/SPEC_COMPLIANCE.md`. Do not task it.

Original write-up:

Still present on `7bb77cc`. `internal/core/parser/behavior.go:390` reads, verbatim:

```go
// Note: isIndividual, isSnapshot consumed but not stored in AST (no fields yet)
_, _ = isIndividual, isSnapshot
```

So `individual testSystem : TestSystem` is checked as an ordinary usage rather than an occurrence kind.
#38 fixed the kind tables for `individual def`, which is why the corpus is clean, but the usage-level
modifier still evaporates. Needs an AST field, the parser storing it, the kind tables consulting it,
a golden fixture and a negative case. `isSnapshot` is the same shape and should go with it.

## F2 — n-ary `connect (a, b, c)` — verify before tasking ✅ done (#46, plus #49)

**It does not reproduce for a named connector declaration.** End counts for binary/3-end/4-end are 2/3/4
at parse, at the constraint tier and through lowering, with runtime `PeerPorts` 1/2/3. #46 is regression
tests at every layer plus one conformance case; no behaviour change was invented, which is what the task
asked for.

The **anonymous inline** form `connect (a, b, c);` was real, and was the child's "Found, not fixed" item:
`parseBodyMember` had a hand-rolled binary-only reader instead of calling `parseConnectorEnds`. Fixed by
the maintainer in #49.

Original write-up:

A3 reported that the n-ary form silently loses all but the first end at parse time. On `7bb77cc`
`parseConnectorEnds` appends every end in the comma loop, so the parse-time claim does not reproduce as
described — it may have been fixed during #40's review rounds, or the loss may be downstream in
`passes/constraint.go` `checkConnectorEnds` / `lower/connection.go`. **Reproduce it first**; if it is
real, the task is wherever the ends are actually dropped, and if it is not, close it out.

---

# Batch G — found by wave 1 (new)

## G1 — a `subject` escapes the usage-kind check depending on what precedes it in the body

Found by E2, reproduced on the merged wave-1 stack:

```
requirement def R { subject s : A; }                 -> no diagnostic
requirement def R { attribute x; subject s : A; }    -> subject cannot be typed by actionDef
requirement def R { doc /* d */ subject s : A; }     -> no diagnostic
requirement R { attribute x; subject s : A; }        -> no diagnostic   (usage: never checked)
concern def R { subject s : A; }                     -> reports
```

It is **not** a first-member rule, which is how it first read. Whenever the requirement-specific body
path is taken (`parser/defusage.go:798`, and always for a requirement *usage* at `defusage.go:1426`),
`subject` is parsed as `*ast.SubjectMember` rather than a `Usage`, and `passes/typecheck.go` only walks
`*ast.Usage`. So whether a subject is type-checked depends on which keyword happens to open the body.

**The fix belongs in the pass, not the parser:** typecheck should handle `*ast.SubjectMember` too instead
of depending on the parse path. It touches `passes/typecheck.go`, which both #45 and #48 edit, so it
cannot start until wave 1 has merged.

## G2 — candidates from wave 1's "Found, not fixed", not yet tasked

- `internal/core/parser/defusage.go:365-373` — `individual part ip : Vehicle;` parses as
  `ast.Usage{Kind: individual}` with the `part` keyword absorbed (same for `individual occurrence`).
  Fixing it reclassifies the usage kind and reaches `internal/core/symbols/builder.go`.
- `internal/core/parser/behavior.go:267-280` — `action a { in snapshot ; }` parses with zero
  diagnostics; an anonymous untyped parameter is silently accepted. Pre-existing for `in event ;` too.

---

# Batch C — release and tooling (unchanged, maintainer/account-gated)

## C2 — macOS distribution follow-through (PR #30 landed)

Root cause is confirmed `com.apple.quarantine`, not a missing signature: Go's linker already ad-hoc signs
darwin/arm64 even when cross-compiling from Linux (`flags=0x20002` in the Mach-O CodeDirectory), so
ad-hoc `codesign` in CI would change nothing.

1. **Create `Open-MBEE/homebrew-tap`** (maintainer) and push `Formula/systemica.rb` from PR #30. Until
   that repo exists the documented `brew tap Open-MBEE/tap && brew install systemica` does not work.
   Then verify a real install on a Mac — nothing in this work has been executed on macOS.
2. **Notarization** removes the prompt entirely for direct downloads and needs an Apple Developer Program
   account ($99/yr), a Developer ID Application certificate, an App Store Connect API key in CI secrets,
   and a macOS runner. Steps and cost are in `docs/MACOS_DISTRIBUTION.md`.
3. **Windows has the same shape of problem** — releases ship unsigned `.exe`s and SmartScreen warns
   "unrecognized publisher". Documented, not fixed; an EV/OV code-signing certificate is the fix.

## C3 — automate the formula bump

The formula pins per-release URLs and SHA256s. The release job already publishes `SHA256SUMS.txt`; a
release step that opens a bump PR against the tap would keep it from going stale.

---

# Batch D — small, low-risk

## D2 — `entry <actionName>;` does not parse ✅ done (#39)

## D3 — pseudostate keywords are a Systemica extension

PR #32 established that the OMG textual grammar has no production for any pseudostate or for event
deferral (checked `SysML.xtext` `/* STATES */`, tag 2026-05). The keywords are documented as an extension
in `docs/grammar/README.md`. If OMG later standardizes a notation this is the row to revisit — worth
raising with the OMG SST if you want the extension aligned early.

---

# How to run the next batch

The last batch cost ~273 ACU across 9 sessions and the avoidable part of that was structural, not
technical. Three rules:

1. **Partition by disjoint file sets, not by task independence.** All seven Batch A children edited
   `internal/core/model/testdata/training_examples_expected.txt`, so every PR conflicted with every other
   one and the corpus figure churned 88 → 89 → 90 → 91 → 93 → 95 → 97 while sessions re-measured against a
   moving baseline. Children should adjudicate in `docs/TRAINING_EXAMPLES.md` and report counts, but not
   commit the expectations file; one follow-up session regenerates it after the batch merges.
2. **Give every child an explicit file list and a stop rule.** "If you find a bug outside this list,
   write it up under 'Found, not fixed' and carry on." A3 ran to 69 ACU and about eleven review rounds,
   expanding from one spec row into four parser bugs and two resolver rewrites — all real, none in scope.
   Cap review iteration at four rounds, then report the remainder.
3. **Children escalate spec disagreements; they do not settle them.** Two merged PRs relaxed a checker or
   re-pointed a test on a child's own reading of the spec, and #42 declined a requested fix on spec
   grounds. Each was defensible and each should have been a decision, not a commit.

## Suggested sequencing

1. ~~**C1** first~~ and ~~**E2**, **F1**, **F2** concurrently~~ — wave 1, done; see the wave-1 section
   above for the merge order.
2. **B1** as several separate sessions, with **B2**, **B4** and **B5** alongside. These share
   `docs/SPEC_COMPLIANCE.md` and little else, so they parallelize well; the two `state_executor.go`
   items in B1 must run one at a time.
3. **B3 last**, gated on a per-file corpus diff.
