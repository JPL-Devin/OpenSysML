# Performance and Memory

How to profile `sysml`, what a large model costs today, and what the measurements
say about where the remaining cost is. Figures below were taken on an
`Intel Xeon Platinum 8175M @ 2.50GHz`, Go 1.23, `GOMAXPROCS=8`; treat them as
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
| 0        | 80 ms     | 32 MiB    | 14 MiB         |
| 250      | 106 ms    | 44 MiB    | 16 MiB         |
| 1 000    | 186 ms    | 83 MiB    | 24 MiB         |
| 4 000    | 538 ms    | 242 MiB   | 55 MiB         |

Two things follow from the `elements=0` row. A session costs about **14 MiB held
and 32 MiB allocated before it holds any model of its own**: that is the standard
library, which every session indexes so that names such as a quantity's unit
resolve. And each session holds its own copy — a process serving many sessions
(the LSP, a server) pays that per session, not once.

Above that baseline a model costs roughly **10 KiB held per element**, and
allocates roughly **50 KiB per element** while loading, most of it short-lived
parser and resolution garbage.

Whole-binary scaling on `-validate` over generated models, before and after the
fixes described below:

| elements | wall (before) | peak RSS (before) | wall (now) | peak RSS (now) |
| -------- | ------------- | ----------------- | ---------- | -------------- |
| 4 000    | 1.64 s        | 140 MiB           | 0.59 s     | 105 MiB        |
| 8 000    | 6.35 s        | 249 MiB           | 1.13 s     | 196 MiB        |
| 16 000   | 25.64 s       | 492 MiB           | 2.19 s     | 353 MiB        |

Before, doubling the model roughly quadrupled the time: loading was quadratic in
the size of the model. A 4 000-element model allocated 840 MiB to end up holding
55 MiB; it now allocates 242 MiB. Now both time and memory grow linearly with the
model — 4 000, 8 000 and 16 000 elements allocate 242, 451 and 873 MiB.

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

## What running a model costs

Runs are measured against an already-loaded model, with the session's runtime
built before measurement starts — a session builds its runtime and indexes the
standard library on its first run, and that one-time cost otherwise swamps the
run being measured.

| elements | start state machine | evaluate calculation | instantiate part def |
| -------- | ------------------- | -------------------- | -------------------- |
| 250      | 26 µs, 4.0 KiB      | 23 µs, 3.7 KiB       | 20 µs, 1.6 KiB       |
| 1 000    | 82 µs, 4.0 KiB      | 77 µs, 3.7 KiB       | 73 µs, 1.6 KiB       |
| 4 000    | 334 µs, 4.0 KiB     | 435 µs, 3.7 KiB      | 329 µs, 1.6 KiB      |

Execution memory is **flat in the size of the model** — a run allocates a few
kilobytes whatever the surrounding model — so memory pressure from executing
models is not where a large model hurts; loading it is.

Execution *time*, however, grows with the size of the surrounding model: starting
the same three-state machine takes 13 times longer in a 4 000-element model than
in a 250-element one, while allocating exactly the same memory. A CPU profile of
that benchmark is spent in the garbage collector — `runtime.scanobject` and its
neighbors account for over half of it — so the cost is collection scanning the
live model, paid by whatever allocates next, rather than work the run itself does.
What this says about a real workload is that the collector, not the run, is what
grows: a long-lived session over a large model tunes better with `GOGC` than with
a faster executor.

## Notes for further work

- The parser's token buffer (`parser.fill`) is now the largest single source of
  allocation while loading, at about 54% of a large model's total. It grows the
  buffer as it reads; sizing it from the source length would cut most of that,
  and with it much of the collector pressure a load creates.
- Runs over a large model spend their time in collection, not in the executor
  (above). Reducing what a load leaves behind is the lever, since the live model
  is what each cycle scans.
- A session holds its own standard-library index (14 MiB). Sharing one index
  across sessions in one process would remove that per-session floor.
