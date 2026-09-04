# Performance: the 0.5 line against release 0.4.3

A release-gate measurement of `main` (`ab982673`, 2026-09-04, the 0.5 line as
it stands) against the previous release, `v0.4.3` (`99e02003`, 2026-09-02),
following the method of the [September census](performance-census-2026-09.md),
the [profile](performance-profile-2026-09.md) and the
[execution study](execution-performance-2026-09.md). The question is the
release one: does `main` perform at least as well as the last tag, and where it
does not, why.

All figures were taken on one machine — `Intel Xeon Platinum 8375C @ 2.90 GHz`,
8 CPUs, ~31 GiB, Go 1.25.0, Linux. That is a different processor from the
`8559C` the reference figures in `docs/internals/performance.md` and the census
were taken on, so absolute numbers here are **not** comparable to those
documents; the two revisions were measured back to back on this machine, and
only the ratios between them carry.

Three columns appear throughout: **0.4.3**, **main** (as found), and
**main+fix** (with the changes this document ships). Where a regression is
marked *fixed*, the fix is in the same change as this record; *explained* means
the cause is known and the cost is the intended price of a feature or fix that
landed in the interval; *open* means the cause is known but the fix is not
low-risk enough to ship here, and a proposed one is written down.

## Method

- Both revisions built with `make build` into separate worktrees
  (`git worktree add ../opensysml-v0.4.3 v0.4.3`), so `bin/sysml` of each is its
  own binary. The `v0.4.3` tag lives on the upstream remote and is annotated;
  `v0.4.3^{commit}` is `99e02003`.
- Every package that declares a benchmark — `internal/repl`,
  `internal/perfbench`, `internal/core/libs`, `internal/core/model`,
  `internal/grpc`, `internal/lsp` — run on both revisions with
  `go test ./<pkg> -run '^$' -bench . -benchmem -count 6`, compared with
  `benchstat`. A movement is reported when `p ≤ 0.05` and the change exceeds
  about 5%; smaller significant movements are listed as noise.
- Whole binary: `sysml -validate` over generated compliant models of 3 000,
  6 000 and 12 000 elements (parts with attributes, constraints, nested parts;
  calc definitions; state definitions), three runs each, wall time and maximum
  resident set from `getrusage`. Both binaries report `no errors` on all three.
  Process start as the mean of 50 `sysml --version` runs; empty-session load as
  the `LoadModel/elements=0` benchmark. The `examples/` models were run through
  `-validate`, `-instantiate`, `-action` and `-state` on both binaries.
- Regressions were profiled with `-cpuprofile`/`-memprofile` and attributed to
  the commit that introduced the responsible code with `git log -S`; no bisect
  was needed.

### Workloads that are not comparable

- `internal/lsp` has benchmarks only on `main` (formatting was added after
  0.4.3); its 0.4.3 run produced no rows, so there is no comparison.
- `internal/perfbench`'s `vehicle` corpus benchmarks, `GRPCParseFile*`,
  `Connect*HTTP*` and `internal/repl`'s `CompiledCalc/*/interpreted` exist only
  on `main`.
- No committed baseline file was regenerated. One observation from running
  the suite: on this machine the provisioned pilot corpora do not match what
  the committed baselines record — `sysml-examples` holds 98 files against a
  recorded 99 (`Simple Tests/InterfaceTest.sysml` is absent), the kerml and
  sysml digests differ, and `AnalysisIndividualExample.sysml` gains a
  type-conformance error — identically on unmodified `main` and on this
  branch, so it is a provisioning question for the corpus pins, not an effect
  of these changes.

## Benchmarks: `internal/repl`

| figure | 0.4.3 | main | main+fix | main+fix vs 0.4.3 |
| ------ | ----- | ---- | -------- | ----------------- |
| load wall, 250 / 1000 / 4000 elements | 19.9 / 77.8 / 335 ms | 27.0 / 105 / 436 ms | 22.6 / 90.1 / 382 ms | +14% / +16% / +14% |
| load bytes allocated, 250 / 4000 | 8.2 / 130 MiB | 11.8 / 187 MiB | 9.7 / 154 MiB | +18% / +19% |
| load live heap, 250 / 4000 | 2.09 / 32.1 MiB | 2.12 / 32.4 MiB | 2.12 / 32.5 MiB | +1% |
| empty-session load wall | 255 µs | 263 µs | 263 µs | +3% (noise) |
| state-machine start, 250 / 1000 / 4000 | 8.2 / 8.2 / 8.4 µs | 61 / 252 / 992 µs | 23.6 / 98.9 / 403 µs | +187% / +1107% / +4712% |
| state-machine start bytes / allocs, 4000 | 8.3 KiB / 120 | 401 KiB / 4.6k | 8.6 KiB / 123 | +3.5% / +2.5% |
| instantiate, 250 / 1000 / 4000 | 3.1 / 3.2 / 3.2 µs | 12.0 / 11.7 / 11.9 µs | 11.5 / 11.7 / 11.8 µs | +269% / +268% / +271% |
| calc, 250 / 1000 / 4000 | 2.07 µs | 2.19–2.23 µs | 2.19–2.20 µs | +6% |
| diagnostics, 50 / 200 / 800 attributes | 88 µs / 350 µs / 1.45 ms | — | — | unchanged (p > 0.09) |

## Benchmarks: `internal/perfbench`

Rows that moved more than ~5% on `main`; everything else is within noise
(`Lex`, `IndexAdd*`, `WorkspaceEdit/synthetic`, `LookupQualified`,
`REPLEvalExpr`, `LowerStateGraph`, `LowerChain/chain1000`) or a few percent
(`Parse/synthetic` +2.5%, `FQNOf` +1.9%).

| benchmark | 0.4.3 | main | main+fix | main+fix vs 0.4.3 |
| --------- | ----- | ---- | -------- | ----------------- |
| Analyze/synthetic | 808 ms | 1234 ms | 1167 ms | +44% |
| WorkspaceEdit/synthetic/reindex+diagnostics | 909 ms | 1424 ms | 1223 ms | +35% |
| REPLLoadFile | 1.14 s | 1.45 s | 1.36 s | +19% |
| REPLSubmitSnippet | 1.02 s | 1.31 s | 1.20 s | +18% |
| LowerActionGraph | 1.51 µs | 1.93 µs | 1.96 µs | +30% |
| LowerChain/chain10 / chain100 | 7.2 / 145 µs | 9.0 / 162 µs | 9.0 / 163 µs | +24% / +13% |
| ExecuteAction | 12.5 µs | 67.0 µs | 22.0 µs | +77% |
| ExecuteActionFreshContext | 15.0 µs | 70.0 µs | 47.1 µs | +215% |
| ActionLoop/for10 / for100 / for1000 | 12.9 / 87.5 / 825 µs | 65.7 / 149 / 973 µs | 22.6 / 107 / 935 µs | +76% / +22% / +13% |
| ActionChain/chain10 / chain100 / chain1000 | 19.1 / 251 µs / 11.4 ms | 85.8 / 470 µs / 13.3 ms | 38.3 / 383 µs / 12.8 ms | +101% / +53% / +12% |
| ExecuteState | 225 µs | 255 µs | 249 µs | +11% |
| StateLoop/count50 / 500 / 5000 | 219 µs / 2.08 / 20.8 ms | 250 µs / 2.38 / 23.6 ms | 243 µs / 2.33 / 23.2 ms | +11% / +12% / +12% |
| BatchConstraints | 4.48 ms | 16.5 ms | 6.35 ms | +42% |
| SameConstraintManyInstances | 1.50 µs | 5.94 µs | 2.25 µs | +50% |
| BatchSatisfy | 4.54 s | 7.14 s | 0.67 s | −85% |
| Instantiate | 4.07 s | 6.55 s | 0.58 s | −86% |
| FeaturesOf | 11.6 s | 12.0 s | 0.78 s | −93% |
| GRPCEvaluate | 395 µs | 617 µs | 615 µs | +56% |
| GRPCVerifyConstraint | 17.6 ms | 2398 ms | 183 ms | +943% |

## Benchmarks: the other packages

| package / benchmark | 0.4.3 | main | movement |
| ------------------- | ----- | ---- | -------- |
| `core/libs` DecodeSnapshot | — | — | +4.9% (p = 0.002; below threshold) |
| `core/model` AnalyseUnresolved / AnalyseResolved | — | — | +1.8% / +2.8% |
| `grpc` ParseFileColdInline | — | — | +6.2% wall; allocations unchanged |
| `grpc` ParseFileColdShared | — | — | unchanged |
| `lsp` formatting benchmarks | absent | present | no comparison |

`ParseFileColdInline` is the only movement above threshold here; it is the same
diagnostics cost as the `Analyze` row above seen through the gRPC service (a
cold parse runs the full pass registry) and is covered by the same finding.

## Whole-binary scaling

`sysml -validate` over generated compliant models that report `no errors`,
three runs each; wall seconds and maximum resident set.

| elements | 0.4.3 | main | main+fix | main+fix vs 0.4.3 |
| -------- | ----- | ---- | -------- | ----------------- |
| 3 000 | 0.26 s / 105 MiB | 0.36 s / 145 MiB | 0.31 s / 129 MiB | +19% / +23% |
| 6 000 | 0.53 s / 155 MiB | 0.72 s / 209 MiB | 0.62 s / 190 MiB | +17% / +23% |
| 12 000 | 1.10–1.44 s / 266 MiB | 1.48–1.61 s / 336 MiB | 1.30 s / 321 MiB | +18% (against the 1.10 s best) / +21% |

Scaling is still linear on both sides; the slope moved. The CPU profile of the
12 000-element run on `main` attributes the whole difference to
`Workspace.Diagnostics` (0.58 s of 1.21 s on 0.4.3; 0.93 s of 1.61 s on
`main`): parsing, indexing and name resolution take the same time on both
revisions. The passes that grew are listed under *Findings*. Memory follows the
same path: the extra resident set is the garbage the new passes make, not live
model.

### Process start and session floor

| figure | 0.4.3 | main |
| ------ | ----- | ---- |
| `sysml --version`, mean of 50 | 2.0 ms | 3.4 ms |
| binary size | 21.7 MiB | 37.9 MiB |
| Go package initialisation (`GODEBUG=inittrace=1`, sum) | 0.77 ms | 1.95 ms |
| empty-session load (`LoadModel/elements=0`) | 255 µs | 263 µs |

Start-up is 1.4 ms slower per process, well inside the < 20 ms the Unreleased
changelog claims. The growth is the binary: `sysml` now links `internal/grpc`
(and with it the protobuf and gRPC stacks), the Flexo sync client
(`internal/flexo`), `internal/codegen` and `internal/edit`, none of which 0.4.3
linked, so it maps 16 MiB more text and runs 1.2 ms more package initialisers
(`protobuf/reflect/protodesc` and `descriptorpb` are the two largest new ones).
This is the expected price of the feature and is recorded as *explained*.

### Example models

Wall time and maximum resident set, three runs each, best shown; every run of
each pair returned the same exit code except where noted.

| model / command | 0.4.3 | main |
| --------------- | ----- | ---- |
| `action-executor-demo.sysml -action ActionExecutorDemo::sequential` | 0.03 s / 50 MiB | 0.03 s / 60 MiB |
| `orthogonal-regions-demo.sysml -state …::TrafficLight -advance 20` | 0.04 s / 56 MiB | 0.05 s / 67 MiB |
| `phase-c-behavioral-bodies.sysml -instantiate PhaseC::Vehicle` | 0.03 s / 50 MiB | 0.03 s / 61 MiB |
| `phase-c-behavioral-bodies.sysml -state PhaseC::AutopilotMode -advance 20` | 0.03 s / 50 MiB | 0.03 s / 61 MiB |
| `self-model/surfaces.sysml -validate` | 0.11 s / 78 MiB, exit 2 | 0.11 s / 90 MiB, exit 2 |
| `self-model/pipeline.sysml -validate` | 0.04 s / 51 MiB | 0.04 s / 61 MiB |
| `disposal-robot-demo/robot.sysml -validate` | 0.05 s / 56 MiB, exit 0 | 0.06 s / 67 MiB, **exit 2** |

At example scale the two binaries are indistinguishable in wall time; the
~10 MiB of extra resident set is the larger binary plus the library index the
new passes touch. One difference is not performance: `robot.sysml` validates
clean on 0.4.3 and fails on `main` with
`cannot override the binding value of TradeStudies::MaximizeObjective::best`
(and the same for `MinimizeObjective::best`), so its `-action`, `-state` and
`-satisfy` runs no longer start (the README's commands exit 2). This is the new
feature-value-overriding rule (`changes/unreleased/feature-value-overriding.added.md`)
firing on a committed example. It is out of scope for a performance record and
is reported as a finding for the rule's author: either the example or the rule
is wrong, and the pilot validator should adjudicate.

## Findings

Ordered by size. Each names the cause, the responsible change, the measured
cost, and its status.

### 1. State-machine start scans the whole model — *partly fixed, open*

`RunStateMachine` went from a constant 8 µs to a cost linear in model size
(992 µs at 4 000 elements). Commit `25fe8b9d` (*refuse `%state <machine>`
alone for a definition exhibited through typed usages*) added
`Session.exhibitingTypes`, which — when a state definition has no exhibiting
object — walks every scope of every document and asks
`Context.ExhibitsState` of every member to name the types that would exhibit
it. The benchmark's machine has no exhibitor, so it pays the walk on every
start. Two costs compounded:

- `Scope.Members()` was called per scope, which deduplicates through a map and
  allocates the member slice; 70% of the allocation profile. **Fixed**: the
  walk now uses `Scope.ForEachMember`, skipping anonymous members exactly as
  `Members()` did. Bytes per start are back to 8.6 KiB (from 401 KiB) and
  wall drops 2.5×.
- The walk itself is O(model) per start and is not cached. **Open.**
  Proposed fix: index the exhibit usages once per session generation (a
  `map[stateDef][]owner` built from the same walk and invalidated where
  `Session.acceptFrom` rebuilds the index), so the lookup is O(1); or restrict
  the walk to the scopes the resolver has already recorded as referring to the
  definition. Either restores the constant-time start. Remaining cost:
  +15 µs at 250 elements, +394 µs at 4 000.

### 2. Instantiation runs behaviors of the whole part tree — *explained / open*

`repl` `Instantiate` 3.1 → 11.7 µs (+270%); `perfbench`
`ExecuteActionFreshContext` +215%; `GRPCEvaluate` +56%; and most of
`GRPCVerifyConstraint` (below). Commit `37de3db2` (*read the instantiated
object, run whole, after `-instantiate`*) added `materializeBehavingParts` and
`runsBehaviors`: creating an object now materializes every required composite
part whose type (transitively) runs a classifier behavior, so the object
reaches quiescence as a whole. `runsBehaviors` memoizes its verdict per type
within a `Context`, but reaching it walks `FeaturesOf` for every part type
along the way, and `startBehaviorsOfAll` / `aliasRedefinedFeatureValues` run
per created object. This is the semantics the fix was made for and the cost is
proportional to the part tree, so it is **explained** for the REPL. For
short-lived contexts it is **open** (finding 3).

### 3. gRPC verification builds a fresh runtime per request — *partly fixed, open*

`GRPCVerifyConstraint` 17.6 ms → 2 398 ms as found; 183 ms after the fix
below. `Service.newVerifyContext` creates a new `resolve.Resolver`,
`semantics.Model` and `runtime.Context` per request (as it did on 0.4.3), so
nothing runtime-side is memoized across requests. On 0.4.3 that was cheap
because verification instantiated only the subject. With finding 2 every
request instantiates the subject's whole behaving part tree, which for the
synthetic model means computing `FeaturesOf` for most of the type graph from
scratch — 87 MB and 526 k allocations per request, and a GC that spends a
quarter of the profile scanning the cached model.

- Inside `FeaturesOf`, `findOwnerType` located the type owning a feature by
  scanning the parent scope's members (`MemberNames` + `LookupLocalAll`) until
  one declared the owner node — O(members) per feature, O(n²) per type. This
  was also the case on 0.4.3, which is why `perfbench` `FeaturesOf` took 11.6 s
  there; finding 2 simply made it hot. **Fixed**: `Scope.Owner()` already
  records that symbol, so the scan is now a fast path with the old search as
  fallback for scopes re-owned by another declaration (metadata bodies).
  `FeaturesOf` −93%, `perfbench` `Instantiate` −86% and `BatchSatisfy` −85%
  against 0.4.3; `GRPCVerifyConstraint` 13×.
- The remaining 10× is the per-request `Context`. **Open.** Proposed fix: hold
  one `semantics.Model` + resolver per cached model hash (both are immutable
  side tables over an immutable index, so sharing is sound) and derive
  `FeaturesOf`/`runsBehaviors` from the model rather than the context, or
  keep a per-hash `runtime.Context` template whose type-level caches are
  copied into the request context. Either amortizes the type-graph work across
  requests; neither belongs in a release-gate change because the service's
  cache eviction and the `Context` lifetime contract need a design pass.

### 4. `MembersOf` recomputed per action frame — *partly fixed, explained*

`ExecuteAction` 12.5 → 67.0 µs; `ActionLoop/for10` and `ActionChain/chain10`
+410% / +350%; `BatchConstraints` and `SameConstraintManyInstances` +270% /
+295%. Commits `62def3d4` (*give each nested action node its own performance
frame*) and `fa5fc3da` (*seed inherited action defaults when the performance
starts*) build a frame per node performance and, in `newRootFrame` →
`addFeatureDirections` / `performanceFeatures`, walk
`semantics.Model.MembersOf(action)` each time — 33% of the CPU and 50% of the
allocation profile. `effectiveMembers` (constraint evaluation) and
`operationOf` did the same per evaluation.

- **Fixed**: `Context.membersOf` memoizes `Model.MembersOf` per type for the
  life of the context (the model is fixed for that life; callers only read the
  list). `ExecuteAction` 67 → 22 µs, `BatchConstraints` 16.5 → 6.4 ms,
  `SameConstraintManyInstances` 5.9 → 2.2 µs.
- Also in `lookupSubaction`, every name read during evaluation resolved
  through the resolver before checking whether any action performance was on
  the stack; for constraint and calc evaluation none ever is. **Fixed**: the
  resolver lookup is skipped when no frame carries a performance, which is
  exactly the case in which the function returned "not declared" regardless.
- The rest (+77% on `ExecuteAction`, +13% at `for1000`) is the per-node frame
  itself: a `performances` entry, its pin table and direction map per node
  performance instead of per action. That is the data the nested-frame feature
  exists to keep and is **explained**; the amortized cost at 1 000 iterations
  is ~110 ns per node.

### 5. New validation passes on the load path — *partly fixed, explained*

`LoadModel` +30–36%, `Analyze/synthetic` +53%,
`WorkspaceEdit/reindex+diagnostics` +57%, `REPLLoadFile` +28%, and the whole
of the whole-binary slope. `internal/core/passes` gained ten files and 61
commits in the interval; the CPU profile of the 12 000-element validation
puts 0.30 s of the 0.35 s difference in one of them, `ControlNodeSuccessionPass`
(commit `279408b8`, *validate control-node successions and owning type*),
whose `semantics.Model.ActionSuccessions` built the set of visible members of
every action (`MembersOf`, which reaches through the library `Action`
supertypes) before looking at any inherited succession.

- **Fixed**: the visible-member set is built only when an inherited succession
  is actually named — the only case that consults it. The pass drops from
  0.30 s to 0.16 s on the 12 000-element model; `LoadModel` from +30–36% to
  +14–16%, whole-binary validation from +33–41% to +17–19%.
- The remainder is the new rules doing their work (control-node succession
  counting still reads inherited successions; type-check and constraint tiers
  grew by a few percent each) and is **explained**. A cheaper
  `ActionSuccessions` would memoize the per-action result in the semantics
  model's side tables, as `MembersOf` is; the pass and the runtime lowering
  both call it for the same actions.

### 6. Smaller movements — *explained*

- `LowerActionGraph` +30%, `LowerChain/chain10` +24%: lowering now records
  the per-node performance data of finding 4; absolute cost 0.4 µs per graph.
- `ExecuteState` / `StateLoop` +11–12%: a constant per step that the fixes
  here do not move; the profile spreads it over the state executor's
  transition handling rather than one function.
- `RunCalc` +6%: 0.12 µs per call, unchanged by the fixes here; below the
  size at which a profile separates it from the runtime's frame set-up.
- `sysml --version` +1.4 ms: binary size and protobuf initialisers, above.
- `core/libs` `DecodeSnapshot` +4.9%, `core/model` `Analyse*` +2–3%,
  `Parse/synthetic` +2.5%: significant but below threshold; the snapshot
  grew with the library index fields the new passes read.

## Verdict

`main` is not on par with 0.4.3. Loading and validating a model is 14–19%
slower and allocates ~20% more; starting a state machine costs time linear in
model size where it was constant; instantiation is 3.7× slower per object; and
the gRPC verification request, which rebuilds its runtime per call, is 10×
slower even after the fixes here. Parsing, indexing, name resolution, the
diagnostics benchmarks, empty-session load and the live heap are unchanged.
None of the movements is a leak or a wrong algorithm in the sense of the
census: each is a feature or rule landed in the interval whose price was not
measured. The four fixes in this change recover roughly half of the load-path
cost and most of the action/constraint-evaluation cost and remove a pre-existing
quadratic step in `FeaturesOf`; the two open items (finding 1's per-start
scan and finding 3's per-request runtime) have proposed fixes and should be
taken before 0.5 is tagged if the REPL's state-machine start or the gRPC
verification latency are release criteria.

## Reproducing

```bash
git remote add upstream https://github.com/Open-MBEE/OpenSysML.git   # read-only
git fetch upstream --tags
git worktree add ../opensysml-v0.4.3 v0.4.3
(cd ../opensysml-v0.4.3 && make build)
make build
for pkg in internal/repl internal/perfbench internal/core/libs internal/core/model internal/grpc internal/lsp; do
  (cd ../opensysml-v0.4.3 && go test ./$pkg -run '^$' -bench . -benchmem -count 6) > old.$pkg.txt
  go test ./$pkg -run '^$' -bench . -benchmem -count 6 > new.$pkg.txt
  benchstat old.$pkg.txt new.$pkg.txt
done
go test ./internal/repl -run '^$' -bench RunStateMachine/elements=4000 -cpuprofile sm.cpu -memprofile sm.mem
go test ./internal/perfbench -run '^$' -bench GRPCVerifyConstraint -benchtime 20x -cpuprofile gv.cpu
../opensysml-v0.4.3/bin/sysml -validate gen12000.sysml -cpuprofile old.cpu
bin/sysml -validate gen12000.sysml -cpuprofile new.cpu
```
