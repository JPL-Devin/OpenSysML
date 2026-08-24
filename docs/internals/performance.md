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
| 0        | 0.17 ms   | 115 KiB   | 32 KiB         |
| 250      | 25 ms     | 19 MiB    | 2.7 MiB        |
| 1 000    | 96 ms     | 80 MiB    | 11 MiB         |
| 4 000    | 402 ms    | 329 MiB   | 43 MiB         |

The `elements=0` row is what a session costs before it holds a model of its own:
**32 KiB held and 115 KiB allocated**. The standard library is no longer indexed
eagerly per session, so a process serving many sessions (the LSP, a server) no
longer pays a multi-megabyte floor for each one.

Above that baseline a model costs roughly **11 KiB held per element**, and
allocates roughly **80 KiB per element** while loading, most of it short-lived
parser and resolution garbage.

Whole-binary scaling on `-validate` over generated models in standard notation,
which report nothing:

| elements | wall   | peak RSS |
| -------- | ------ | -------- |
| 3 000    | 0.41 s | 138 MiB  |
| 6 000    | 0.71 s | 207 MiB  |
| 12 000   | 1.40 s | 335 MiB  |

Both time and memory grow linearly with the model. Loading was quadratic once —
doubling the model roughly quadrupled the time — for the reasons below.

### What made it quadratic

Three scans over a namespace's members, each performed once per member of that
namespace:

- `passes.checkRedefinition` searched the enclosing scope for the symbol owning
  it, copying the scope's member-name list for every declaration checked. It cost
  70% of all memory allocated while loading a large model. The scope already
  records its owning symbol (`Scope.Owner()`), which every other pass uses, so
  the search was redundant; the same applies to the scans that asked whether a
  redefined member was declared locally or inherited, which the member's own
  `OwnerScope` answers.
- `resolve.childScope`, `passes.childScopeOf` and `symbols.bodyScopeChild` each
  walked a scope's children to find the one a declaration owns. Scopes now index
  their children by declaration node (`Scope.ChildFor`).
- `resolve.importsOf` rebuilt a namespace's import list on every unqualified name
  lookup made in it. The tree is immutable after parsing, so the resolver
  memoizes it.

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

- The parser's token buffer (`parser.fill`) is the largest single source of
  allocation while loading, at about 35% of a large model's total. It grows the
  buffer as it reads; sizing it from the source length would cut most of that,
  and with it much of the collector pressure a load creates.
- Runs over a large model spend their time in collection, not in the executor
  (above). Reducing what a load leaves behind is the lever, since the live model
  is what each cycle scans.
- Resolving inherited library features in every definition body roughly doubled
  what loading costs in both time and allocation, against the figures the
  quadratic fixes above left. It is linear in the model, and the scope and
  member scans it walks (`symbols.Scope.AllMembers`,
  `symbols.Index.LookupDirectChildrenFrom`) are where that time goes.
