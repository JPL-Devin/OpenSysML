# Environment variables

Read by `sysml`, `sysml-lsp` and `sysml-grpc` alike. Each budget turns a non-terminating run
into a reported error instead of a hang.

| Variable | Default | Meaning |
|----------|---------|---------|
| `SYSML_LIBRARY_PATH` | unset (use the bundled standard library) | Directory to load the SysML/KerML standard library from instead of the embedded copy |
| `SYSML_MAX_STEPS` | `10000000` | Evaluation step budget: the number of expression evaluations one run may spend before it is reported as a runaway |
| `SYSML_MAX_ACTION_STEPS` | `1000000` | Token-flow steps one action run may perform |
| `SYSML_MAX_EVENTS` | `1000000` | Events one state machine run may dispatch, and the events one `%advance` drains |
| `SYSML_MAX_DO_STEPS` | `5000000` | Do actions one state machine run may perform, and the ones one `%advance` drains |
| `SYSML_MAX_ELEMENTS` | `1000000` | Collection elements one evaluation may hold — the bound on the memory a run holds rather than on the work it does |
| `SYSML_MAX_CALC_DEPTH` | `10000` (ceiling `25000`) | Nested `calc` invocations one run may hold on the stack, which is what a recursion spends |
| `SYSTEMICA_SMT` | unset (look for `z3`, then `cvc5`, on `PATH`) | Executable `%check` and `%explain` drive as their SMT solver, speaking SMT-LIB2 on standard input (experimental) |
| `SYSTEMICA_SMT_TIMEOUT` | `10s` | How long one `%check` or `%explain` query may take, as a Go duration (`5s`, `500ms`), after which the verdict is `unknown` |
| `SYSTEMICA_SMT_CORE_BUDGET` | `30s` | How long `%explain` may spend reducing an unsat core to a minimal one, as a Go duration; past it the solver's own core is reported, said not to be necessarily minimal |
| `SYSML_GRPC_INDEX_POOL` | `4` | How many standard library indexes `sysml-grpc` keeps prewarmed for the models it has not seen yet; `0` loads the library on each request instead |

Each budget is what turns a non-terminating run into a reported error instead of
a hang. They count incommensurable things — expression evaluations, action token
steps, dispatched events, do actions, materialized collection elements —
so raising one says nothing about the others, and each has its own variable.

A budget bounds **one run** — one `%eval`, one `%instantiate`, one `%calc`, one
action, one state machine — not a whole session, so a long REPL session of small
operations never runs out. A run started inside another, an action invoked from
an expression say, shares the outer run's budget rather than getting a fresh
one, and so does a run stepped through with `%step`/`%advance`.

The step and event defaults are set by how long a runaway takes to report rather
than by memory — those steps allocate nothing that outlives them (peak RSS is ~34
MB whether a run spends ten thousand steps or fifty million), and the only thing
they make grow is a `%trace`, at 34–83 bytes an entry. At the measured ~13.6M
evaluation steps/s and ~1.9M events/s each default reports a runaway within about
a second, and a fully traced run at those four ceilings holds ~320 MB.

Collection elements are the exception, and `SYSML_MAX_ELEMENTS` is the budget
that reads as memory: a materialized element is a 104-byte value living as long as
the collection holding it, and `1..10000000` conjures one per step. Every way of
materializing a sequence is charged against it — a range, a sequence literal,
`->collect` and the other collection operations — so the default bounds the
elements held at once at ~104 MB, in the same band as the figures above:

```
error: evaluation failed: collection element limit exceeded
(1000000 elements; raise SYSML_MAX_ELEMENTS to allow more)
```

Because it bounds memory and not work, the count is what a statement's evaluation
holds: a loop building a ten-element collection a million times never approaches
it, while a single `1..2000000` exceeds it at once.

`SYSML_MAX_CALC_DEPTH` reads as stack rather than as work: a recursive calculation
evaluates to its result as long as it terminates within the depth, and one that
does not terminate is reported instead of exhausting the stack:

```
error: calc recursion limit exceeded: calc P::spin nested 10000 deep
(unbounded recursion?; raise SYSML_MAX_CALC_DEPTH to allow more)
```

A nested invocation costs ~10 KB of stack, so this is the one budget with a
ceiling: a value above 25000 is refused, because past it the goroutine stack
limit — a fatal error rather than a reported one — would be reached before the
budget was. A recursion needing more depth than that wants an iterative body.

The evaluation step budget:

```
error: execution failed: eval assignment RHS: evaluation step limit exceeded
(10000000 steps; raise SYSML_MAX_STEPS to allow more)
```

A legitimately long run — a numeric integration in an action body, say — needs a
higher ceiling, so raise it for that run:

```bash
SYSML_MAX_STEPS=200000000 sysml descent.sysml
```

Unset or empty means the default. Anything that is not a positive integer is
reported at startup (and at gRPC service construction) rather than silently
ignored:

```bash
$ SYSML_MAX_STEPS=lots sysml model.sysml
sysml: SYSML_MAX_STEPS="lots" is not an integer: set it to a positive number of evaluation steps (default 10000000)
```

The other budgets behave identically, and their errors name the variable that
raises them:

```
execution exceeded max steps (1000000 steps; raise SYSML_MAX_ACTION_STEPS to allow more), possible infinite loop
state machine exceeded max events (1000000 events; raise SYSML_MAX_EVENTS to allow more), possible infinite loop
state machine exceeded max do action steps (5000000 steps; raise SYSML_MAX_DO_STEPS to allow more), possible non-terminating do behavior
```

A long simulation therefore raises the state machine bounds rather than the
evaluation one:

```bash
SYSML_MAX_EVENTS=20000000 SYSML_MAX_DO_STEPS=100000000 sysml descent.sysml
```

## The gRPC service's library index pool

`SYSML_GRPC_INDEX_POOL` is not a budget: it is how many standard library indexes
`sysml-grpc` builds ahead of the requests that need them. The library does not
depend on the model, so a model the service has not seen adds its document to a
prewarmed index instead of loading and expanding the library again — measured on
a 163-line model, ~0.5–0.9 ms rather than ~100–128 ms.

An index is handed out once and then belongs to the model that took it, so cached
models stay independent; the pool refills in the background, and a request
arriving when it is empty builds its own index, so an answer never depends on how
far prewarming got. The refill builds one index at a time and takes about as long
as a library load, so a client sweeping distinct models faster than that drains
the pool and pays the load on the requests that find it empty. Raise it for such
a client, at roughly the memory of one library index each:

```bash
SYSML_GRPC_INDEX_POOL=8 sysml-grpc
```

`0` disables prewarming, which is the behaviour of a service before the pool
existed: every cache miss loads the library itself. Anything but a non-negative
integer is reported at service construction rather than silently ignored.
