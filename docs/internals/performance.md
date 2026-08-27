# Performance and Memory

How to profile `sysml`, what a large model costs today, and what the measurements
say about where the remaining cost is. Figures below were taken on an
`Intel Xeon Platinum 8559C`, Go 1.25, `GOMAXPROCS=8`; treat them as
ratios rather than absolutes.

## Profiling a run

The binary profiles itself, so a profile is taken of the same code a user runs
rather than of a test harness:

```bash
sysml -validate -memstats model.sysml               # what the run cost, on stderr
sysml -validate -memprofile heap.out model.sysml    # heap profile for go tool pprof
sysml -validate -cpuprofile cpu.out model.sysml     # CPU profile for go tool pprof

go tool pprof -sample_index=alloc_space -top heap.out
go tool pprof -top cpu.out
```

`-memstats` prints the wall time, the memory allocated over the whole run, the
number of allocations and collections, and the memory taken from the operating
system:

```
sysml: 684ms wall, 239.3 MiB allocated in 1638904 allocations over 16 collections, 104.0 MiB taken from the OS
```

Read the two memory figures for what they are. *Allocated* is cumulative
allocation, the pressure the run put on the collector — it is not how much memory
the process needs. *Taken from the OS* is a floor on peak resident size, which is
what bounds the model a machine can hold. Neither is the live size of a loaded
model: by the time a run ends the model is unreachable, so a heap profile written
at exit records where the run allocated, not what a held model occupies.

Peak resident size is measured from outside:

```bash
/usr/bin/time -f "%es wall, %MkB peak RSS" sysml -validate model.sysml
```

## Benchmarks

`internal/repl/bench_test.go` loads and runs synthetic models of a stated size,
so a cost that grows faster than the model is visible as a per-element figure
that grows with size:

```bash
go test ./internal/repl -run '^$' -bench . -benchmem
go test ./internal/repl -run '^$' -bench BenchmarkLoadModel -benchmem -memprofile heap.out
```

Beyond the standard figures they report:

- `live-B/op` — memory the loaded model holds, measured with the session still
  reachable, which is what bounds how large a model can be held at once.
- `B/element` — memory allocated per model element while loading.

`elements=0` is a model with no elements: its figures are what a session costs
before it holds anything, so reading a size against it separates the model's cost
from the session's.

## What a large model costs

Loading is parse, scope building, name resolution and the validation passes —
what `sysml -validate` does. Each element is a part definition, a calculation, an
action, a state machine or a part usage.

| elements | load wall | allocated | live heap held |
| -------- | --------- | --------- | -------------- |
| 0        | 0.16 ms   | 110 KiB   | 32 KiB         |
| 250      | 16 ms     | 10.3 MiB  | 2.1 MiB        |
| 1 000    | 62 ms     | 40 MiB    | 8.1 MiB        |
| 4 000    | 260 ms    | 161 MiB   | 32 MiB         |

The `elements=0` row is what a session costs before it holds a model of its own:
**32 KiB held and 110 KiB allocated**. The standard library is no longer indexed
eagerly per session, so a process serving many sessions (the LSP, a server) no
longer pays a multi-megabyte floor for each one.

Above that baseline a model costs roughly **8 KiB held per element**, and
allocates roughly **41 KiB per element** while loading, most of it short-lived
parser and resolution garbage.

Whole-binary scaling on `-validate` over generated models in standard notation,
which report nothing:

| elements | wall   | peak RSS |
| -------- | ------ | -------- |
| 3 000    | 0.30 s | 117 MiB  |
| 6 000    | 0.49 s | 159 MiB  |
| 12 000   | 0.92 s | 272 MiB  |

Both time and memory grow linearly with the model. Loading was quadratic once —
doubling the model roughly quadrupled the time — for the reasons below.

### What made it quadratic

The load path had several costs, including three scans over a namespace's
members, each performed once per member of that namespace:

- `passes.checkRedefinition` searched the enclosing scope for the symbol owning
  it, copying the scope's member-name list for every declaration checked. It cost
  70% of all memory allocated while loading a large model. The scope already
  records its owning symbol (`Scope.Owner()`), which every other pass uses, so
  the search was redundant; the same applies to the scans that asked whether a
  redefined member was declared locally or inherited, which the member's own
  `OwnerScope` answers.
- `resolve.childScope`, `passes.childScopeOf` and `symbols.bodyScopeChild` each
  walked a scope's children to find the one a declaration owns. A scope now
  answers that itself (`Scope.ChildFor`), scanning few children and indexing them
  by declaration node above a threshold.
- `resolve.importsOf` rebuilt a namespace's import list on every unqualified name
  lookup made in it. The tree is immutable after parsing, so the resolver
  memoizes it.
- Every scope allocated a member map even when it had no named members, and a
  populated key also paid map-bucket overhead plus a separate one-element slice.
  The eager per-scope member map is gone: scopes now store named members in
  declaration order, scan small scopes, and build a lookup index lazily only
  above the 12-entry threshold.

### What made reporting findings quadratic

Reporting a finding needs the line and column its offset falls on, which
`SourceFile.Lines()` answers from a line index over the whole file. The index was
rebuilt on every call, and the calls are made once per finding, so validating a
model that reports something cost the file's size times the number of findings: a
16 000-element model that warns on every usage took 123 s and allocated 258 GiB,
against 1.0 s over the same model that warns on nothing. A source file's content
is immutable, so the index is now built once per file and reused, and the REPL's
diagnostic paths hold one index per submission rather than one per finding. That
model now takes 2.1 s and allocates 1.4 GiB — around 570 B allocated per finding
reported, flat in the size of the file.

Materialization rollback had the same shape. A failed creation drops every object
it reached, which it used to identify by copying the whole live-instance key set
before each (also nested) materialization — 81% of all memory allocated while
instantiating. The context now records instance registrations in an append-only
log and marks its length, so a rollback walks only what the failed creation
added.

### Where a load's allocation goes

A load allocates substantially more than the model it leaves behind holds, and a
CPU profile of it is spent in the collector, so allocation volume is what a
load's wall time follows. The load's major allocation sources include:

- The validation passes each walked the document's symbol tree themselves, copying
  every scope's member list on the way — 17 traversals of the same tree per
  document. The passes of one run share a `Context`, which now holds the
  traversal, and the walk is made once.
- A scope used to allocate a member map and a separate member-order slice even
  when it held no named member. Flat declaration-ordered storage removes both
  costs from empty scopes and avoids per-key buckets and one-element slices from
  populated scopes.
- `parser.fill` grows the token buffer as it reads. The parser sizes that buffer
  from source length; the estimate suits dense models and over-allocates on
  sparse ones.
- A wildcard import surfaces a name by enumerating the target namespace's direct
  children (`symbols.Index.LookupDirectChildren`), which rebuilt and re-sorted
  that list on every unqualified lookup made through the import. The index now
  keeps the list per namespace and visibility, and drops what it kept whenever a
  write lands in any of its tables, so a lookup after an edit is recomputed.

The scope representation is shaped by the actual models. In the standard
library, 10,666 scopes contain no named members 70.5% of the time, 99.3% contain
at most eight named members, and the largest contains 516. A generated model
reaches 16,000 named members in one scope. The former representation therefore
paid for maps on roughly 72% of scopes that never used one, while a populated
member cost roughly 80 bytes of map and slice overhead for 24 bytes of data. A
linear scan is appropriate for the common small scopes; the lazy index prevents
the large scopes from becoming quadratic.

### What a load repeated for nothing

Four costs were paid per element where the model needs them once. Removing them
together cut allocation count at 12 000 elements by 40% (7.70M to 4.62M), bytes
allocated by 32% (744.9 to 503.6 MiB), peak resident size by 23% (353 to
272 MiB) and wall time by 20% (1.14 s to 0.92 s):

- The parser recorded every run of whitespace as a trivia entry on the node that
  followed it. Whitespace is most of a file's trivia and no consumer reads it, so
  only notes and comments are recorded now.
- `source.SourceFile.Text` copied the bytes a span covers into a new string, once
  per name, keyword and literal the parser reads. Spans are now taken from one
  cached copy of the content, so a span costs no allocation of its own.
- Comparing a symbol's fully-qualified name against a known one built that name
  first — walking the owner chain and concatenating it — for every candidate
  supertype of every declaration. `symbols.HasFQN` compares the segments against
  the chain from the end instead, so a mismatch allocates nothing.
- `passes.W9CInheritedNameConflictPass` merged every member of every library base
  a declaration conforms to into one map keyed by name, per declaration, to
  answer two questions: whether a name the declaration owns is also contributed
  by a base, and whether two bases contribute one name. The first asks only about
  the few names the declaration owns, so it now asks each base for those; the
  second is only possible with two bases, so the map is built only then.
- A scope indexed its children by declaration node as it built them: a map per
  scope with children. Member lookup already scans small scopes and indexes
  lazily above a threshold, and `Scope.ChildFor` now does the same, so the common
  small scope holds no map.

Diagnostics, resolution results and exit status are byte-identical, over the
bundled `.sysml` and `.kerml` fixtures as well as the test suite.

## What running a model costs

Runs are measured against an already-loaded model, with the session's runtime
built before measurement starts — a session builds its runtime and indexes the
standard library on its first run, and that one-time cost otherwise swamps the
run being measured.

| elements | start state machine | evaluate calculation | instantiate part def |
| -------- | ------------------- | -------------------- | -------------------- |
| 250      | 12 µs, 5.3 KiB      | 12 µs, 4.5 KiB       | 11 µs, 2.8 KiB       |
| 1 000    | 36 µs, 5.3 KiB      | 36 µs, 4.5 KiB       | 34 µs, 2.8 KiB       |
| 4 000    | 202 µs, 5.3 KiB     | 199 µs, 4.5 KiB      | 198 µs, 2.8 KiB      |

Execution memory is **flat in the size of the model** — a run allocates a few
kilobytes whatever the surrounding model — so memory pressure from executing
models is not where a large model hurts; loading it is.

Execution *time*, however, grows with the size of the surrounding model: starting
the same three-state machine takes about 17 times longer in a 4 000-element model
than in a 250-element one, while allocating exactly the same memory. A CPU profile of
that benchmark is spent in the garbage collector — `runtime.scanobject` and its
neighbors account for over half of it — so the cost is collection scanning the
live model, paid by whatever allocates next, rather than work the run itself does.
What this says about a real workload is that the collector, not the run, is what
grows: a long-lived session over a large model tunes better with `GOGC` than with
a faster executor.

## Notes for further work

- Runs over a large model spend their time in collection, not in the executor
  (above). Reducing what a load leaves behind is the lever, since the live model
  is what each cycle scans.
- Resolving inherited library features in every definition body costs about 1.3x
  a load at 12 000 elements — 0.757s to 1.007s — and the factor grows with the
  model. Earlier notes here put it at 2x; that figure came from a much older
  baseline carrying unrelated changes, so do not repeat it. Allocated *bytes*
  went down (647.5 to 529.7 MiB) while allocation *count* went up (4.54M to
  6.03M), and it is the count the regression follows: parsing is unchanged at
  450ms, resolution goes 90ms to 200ms, the validation passes 130ms to 420ms and
  the collector's mark workers 440ms to 640ms. The work is linear in the model
  and semantically required, so what is left to win is in the scans it walks.
  Hot callers now use non-copying member iteration, while callers that need an
  owned slice continue to use `symbols.Scope.AllMembers`.
- Flattening each type's inherited members into an in-memory table was measured
  and rejected. The table was built by merging a type's direct-supertype tables
  with its own scope, name-indexed so a lookup became one probe, cached per index
  generation for library types only, with a per-symbol plan so a lookup on a user
  type cost one map hit and one probe. Against the same clean models it came out
  a wash: 0.981s to 0.983s at 12 000 elements, allocation count 6,077,232 to
  6,070,375, peak resident mixed (−5.7 MiB at 3 000, −10.2 MiB at 6 000, +5.8 MiB
  at 12 000). The reason is that `semantics.Model.MemberSources` already memoizes
  its breadth-first closure and hits that cache about 92% of the time, so a
  lookup was already close to a single map probe, and the whole inherited-member
  cluster is only ~3.5% of a clean load's allocated objects and ~7% of its CPU —
  parsing and symbol building dominate both. A table therefore relocates that
  work rather than removing it: on a clean 12 000-element model only eight
  library types are reached at all, and building their tables plus the per-type
  plans costs about what the closures cost. Two intermediate shapes were
  distinctly worse — recomputing the contributor set per lookup instead of
  hitting the memoized closure (+1.3M allocations), and rebuilding a table on
  every attempt when a build failed and cached nothing (+286k allocations).
  Do not revisit this without moving the tables off the load: built when the
  library index is built, persisted alongside the other library facts and shared
  by pointer, no table build and no per-type closure happens during an analysis
  at all, which is where the remaining win is.
- Two facts about the bundled library came out of that measurement.
  `Performances::Evaluation` appears among its own contributors, a self-edge the
  breadth-first walk hides by seeding its visited set with the starting symbol,
  and `Calculations::Calculation` and `Constraints::ConstraintCheck` are reached
  through a member that is not itself a library symbol.
- The “small scopes could use a slice instead of a map” lever is closed:
  flat declaration-ordered storage replaces the eager map, and scopes above the
  threshold build a lookup index lazily. The measured scope shape is why the
  threshold matters.
- Recycling parser token buffers across parses was measured and rejected. It
  reduced allocated bytes by only 1.6% while increasing wall time by 3.8% and
  peak RSS by 0.8%. The source-bytes-per-token ratio is 13.8 on the standard
  library but 5.6 on generated models, so no fixed presize divisor serves both;
  do not revisit this lever without new evidence.
