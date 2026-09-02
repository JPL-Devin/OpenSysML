# Execution performance, September 2026

A follow-up to the September 2026 performance census, measuring model
*execution* rather than validation: expression and calculation evaluation
throughput, absolute state-machine and instantiation throughput from the Go
benchmarks, and CPU and allocation profiles of the execution paths.

All figures were taken on one machine — `Intel Xeon Platinum 8559C`, 8 CPUs,
Go 1.25, Linux — at revision `af33e195`. Treat absolute numbers as ratios
elsewhere.

## Method: marginal cost per evaluation

A batch of identical cases is evaluated against the same model
(`calc def Fib` recursive over `if`, `calc def SumTo` linear-recursive, and
the literal `2 + 3`) in one process via repeated `-e` flags on `bin/sysml`.
The per-evaluation figure is the marginal cost between a 1-case and an
N-case invocation of the same process, which cancels process start-up,
parsing, and library loading. Minimum of the repetitions taken.

## Results

| workload | start-up + 1 eval | marginal cost/eval |
| -------- | ----------------- | ------------------ |
| `2 + 3` | 0.11 s | below noise (< 0.1 ms) |
| `Fib(18)` — ~8 400 recursive calc calls | 0.13 s | 12.2 ms |
| `SumTo(150)` — 150 recursive calc calls | 0.11 s | 0.06 ms |

Per calc invocation that is roughly 1.5 µs. Recursion depth is not a
practical limit at these scales: `SumTo(400)` evaluates in the same
sub-millisecond band.

## Absolute throughput: state machines and instantiation

`go test ./internal/repl -run '^$' -bench 'RunCalc|RunStateMachine|Instantiate' -benchmem -count 6`
over an already-loaded model (start-up excluded, medians):

| figure | 250 elements | 1 000 elements | 4 000 elements |
| ------ | -------------- | -------------- | --------------- |
| state-machine start | 13.6 µs / 9.2 KiB / 120 allocs | 40 µs | 205 µs |
| calc invocation (`Calc0(2.0, 3.0)`) | 12.6 µs / 4.8 KiB / 70 allocs | 36 µs | 220 µs |
| instantiate (`Comp0`) | 11.3 µs / 2.9 KiB / 39 allocs | 36 µs | 225 µs |

## The two optimization gaps the profiles show

**Per-run target lookup is O(model), and dominates.** Allocations per run are
constant across model sizes, yet wall time grows ~16× from 250 to 4 000
elements. The CPU profile of `BenchmarkRunCalc` at 4 000 elements puts 67%
of samples under `repl.collectInScopeTree` (with `symbols.Scope.LookupLocalAll`
at 25%): every `RunCalc`/`RunStateMachine`/`InstantiateNamed` re-walks the
whole document scope tree to find its target by simple name, even though the
session already holds a qualified-name index (`browseIndex().LookupQualified`
is consulted only for qualified names). Resolving simple names through the
index — or memoizing the tree walk per (document-version, name) — would make
a run's cost independent of model size. The runtime itself is cheap: the
size-independent floor is ~1–3 µs.

*Closed.* The session now tabulates the simple names of its documents once per
scope tree (`repl.nameTable`) and answers lookups from the table, rebuilt only
when a submission or reset replaces a document. Re-running the benchmark above
(same machine, `-count 6`, benchstat): state-machine start 6.4 µs, calc
invocation 4.5 µs, instantiate 3.3 µs — the same at 250, 1 000 and 4 000
elements, a 97–98% drop at 4 000. `collectInScopeTree` no longer appears in
the profile; the table's own cost is under 1% of samples.

**The evaluator allocates ~1.7 KiB per calc invocation, and GC eats half the
CPU.** Profiling 20 × `Fib(20)` (~440 000 calc invocations), 45% of CPU is
garbage collection, and the allocation profile puts 59% of all bytes in
`runtime.Context.bindCalcParameters`, with `NewEvalContext` (8%) and
`newStmtEngine` (3%) adding a fresh context and statement engine per
invocation. Reusing evaluation frames across calls — a free-list on the
`Context`, or binding parameters into a caller-provided frame instead of a
fresh map — is the standard fix and would directly lift recursive-calc
throughput. At ~1.5 µs per invocation this is not currently a user-visible
problem; it is the first thing to reach for if execution-bound models appear.

## Assessment

Execution is fast in absolute terms — microseconds per calc invocation or
state-machine start, sub-millisecond for deep recursive calculations — and
the two gaps the profiles show are internal and localized: an O(model)
per-run name lookup in the REPL session layer, and per-invocation allocation
in the calc evaluator. Both are fixable without architectural change.

## Follow-up: calc invocation frames are reused

Measured on the same machine, same method, after the calc evaluator began
keeping the frame an invocation runs in — parameter bindings, evaluation
context, statement engine — on a free list of the `Context` and reusing it for
the next invocation once the first has returned:

| figure | before | after |
| ------ | ------ | ----- |
| marginal cost per `Fib(18)` eval (8 361 calc invocations) | 13.1 ms | 8.9 ms |
| marginal cost per `Fib(20)` eval (21 891 calc invocations) | 33.1 ms | 24.6 ms |
| bytes allocated per calc invocation (`Fib(20)` marginal) | ~1.7 KiB | ~130 B |
| GC share of CPU, 20 × `Fib(20)` | 50% | 17% |
| GC cycles, 20 × `Fib(20)` | 34 | 11 |

Per calc invocation that is roughly 1.1 µs. `bindCalcParameters` no longer
appears in the allocation profile; what remains per invocation is the
positional argument slice of `evalInvocation` and the boxed values themselves.

## Follow-up (2026-09-02): the evaluator's per-invocation overhead

Measured on the same machine, same method, at revision `d14896bc` before and
after four changes to the calc evaluator, made one at a time and measured
after each: `runtime.Value` shrank from 120 to 64 bytes (the scalar stays
inline; string, sequence, set, expression, quantity and symbol payloads share
one slot); parsed literals and resolved invocation targets are memoized per
`Context` keyed by the AST node, which a REPL or LSP edit replaces along with
the context; a calc's parameters bind into slot-indexed frames resolved once
per calc shape, and a bare name outside a body-local scope is answered from
the frames before the general resolution chain runs; and the positional
argument slice and the statement engine's frame stack are borrowed from
per-context storage instead of allocated per call. The benchmark is `Fib(25)`
(242 785 calc invocations), marginal between a 1- and a 6-case run.

| figure | before | after |
| ------ | ------ | ----- |
| marginal cost per `Fib(25)` eval | 246 ms | 158 ms |
| per calc invocation | 1.01 µs | 0.65 µs |
| allocations per `Fib(25)` eval (`-memstats` marginal) | 971 000 (4.0 per invocation) | ~160 (< 0.001 per invocation) |
| bytes allocated per `Fib(25)` eval | 38.9 MiB (168 B per invocation) | ~0.1 MiB |
| `BenchmarkRunCalc` (`-count 6`, geomean) | 2.15 µs / 888 B / 26 allocs | 1.88 µs / 768 B / 25 allocs |

Stage by stage, per calc invocation: 1.01 µs → 0.83 µs (smaller `Value`) →
0.65 µs (memoized literals and targets) → 0.65 µs (slot frames; the map
lookups it removed were already cheap once the target was memoized) →
0.65 µs (allocation trimming; the young garbage it removed was cheap to
collect). Every result, error message, trace and step count is unchanged; the
execution-trace goldens and the pilot differential's bucket counts are as
committed.

The profile of 3 × `Fib(27)` is now flat. Before, 26% of CPU was `duffcopy`
and 7% `duffzero` (the 120-byte `Value` returned through ten frames per
operation), 12% garbage collection and 7% string-keyed map access. After, no
function exceeds 12% flat: `EvalContext.Eval` 12%, `eval` 9%,
`evalFeatureReference` 7%, `evalOperator` 6%, `evalArithmetic` 5%,
`evalName` 5%, `duffcopy` 4%, `runCalcBody` 4%, `incrementStep` 3%,
garbage collection 2%. What remains is the tree walk itself: about twelve
`Eval` dispatches per `Fib` invocation, each returning a 64-byte `Value` and
an error through three or four frames, plus the invocation's own bookkeeping
(frame acquisition, activation, parameter binding and type checking) at
roughly 15% of the total.

That is a 1.55× improvement per invocation against the 2× this pass aimed for.
Closing the rest within the tree walk would mean flattening the
`(Value, error)` return chain into an out-parameter across every `eval*`
function and shrinking `semantics.Value` — a wide, mechanical change — and
the calc *usage* environment (`bindCalcUsage`) still binds into a map, as its
statements read and write outputs by name. The larger step, a compiled fast
path for pure calc bodies, is the planned follow-up and is out of scope here.
