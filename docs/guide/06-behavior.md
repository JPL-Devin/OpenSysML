# 6. Behavior: actions and state machines

An action or a state machine is executed, not just parsed: a debugger steps it, and the
non-interactive `-action`/`-state` flags run it to completion and report what it produced.
An object may perform the behavior, in which case what it sends routes over that object's
connections.

**Action execution (step-by-step):**
```sysml
sysml> action SimpleWorkflow {
  ...>     attribute result = 0;
  ...>     first start;
  ...>     action compute { assign result := 42; }
  ...>     done end;
  ...>     then start compute;
  ...>     then compute end;
  ...> }
✓ action SimpleWorkflow

sysml> %action SimpleWorkflow
✓ Started action executor for "SimpleWorkflow"
  State: Running
  Tokens: 1

Use %step to advance, %tokens to inspect, %continue to run to completion

sysml> %step
✓ Step complete
  State: Running
  Tokens: 1

sysml> %tokens
Active tokens (1):
  Token 1 @ compute
    result = 0

sysml> %continue
✓ Action completed
  Final state: Completed
  Results:
    result = 42
```

**State machine execution:**
```sysml
sysml> state TrafficLight {
  ...>     initial start;
  ...>     state green { accept after 25 then yellow; }
  ...>     state yellow { accept after 5 then red; }
  ...>     state red { accept after 30 then off; }
  ...>     final off;
  ...>     start then green;
  ...> }
✓ state TrafficLight

sysml> %state TrafficLight
✓ Started state machine executor for "TrafficLight"
  Current state: start
  Time: 0.00
  Events: 1

Use %events to see queue, %current for state, %advance <time> to step

sysml> %advance 25
✓ Advanced to 25.00 (2 event(s) processed)
  Current state: yellow
  Last event at: 25.00
  Remaining events: 1

sysml> %current
Current state: yellow
Time: 25.00
Last event at: 25.00
Execution state: Running

sysml> %advance 5
✓ Advanced to 30.00 (1 event(s) processed)
  Current state: red
  Last event at: 30.00
  Remaining events: 1

sysml> %advance 30
✓ Advanced to 60.00 (1 event(s) processed)
  Current state: off
  Last event at: 60.00
  Remaining events: 0

✓ State machine completed (final state reached)
```

**Action debugging commands:**
- `%action <name> [<object>]` — Start action debugging session, optionally performed by an instantiated object
- `%step` — Advance all tokens one step
- `%continue` — Run to completion, or to the first breakpoint hit
- `%tokens` — Show active tokens with data
- `%break <node>` — Set breakpoint on a named node; `%continue` stops when a token reaches it
- `%stop` — Stop debugging

**State machine debugging commands:**
- `%state <name> [<object>]` — Start state machine debugging; naming an instantiated object performs the machine for it, so what it sends routes over that object's connections
- `%events` — Show event queue
- `%current` — Show current state, stack, data
- `%advance <time>` — Advance simulation time by `<time>` units, processing every event due
- `%stop` — Stop debugging

**See [examples/action-executor-demo.sysml](../../examples/action-executor-demo.sysml) and [examples/state-machine-demo.sysml](../../examples/state-machine-demo.sysml) for complete workflows.**

## An object runs the behaviors its type exhibits

A type that exhibits a state machine or performs an action binds that behavior to every object of
the type: materializing the object gives it an execution of its own, bound to its identity. Two
objects of one type run two machines, with their own current state, event queue and feature values,
and what a body assigns is the feature value of the object performing it.

```sysml
sysml> part def Monitor {
  ...>     attribute count = 0;
  ...>     exhibit state modes {
  ...>         entry; then idle;
  ...>         state idle {
  ...>             entry action bump { assign count := count + 1; }
  ...>             accept after 10 then awake;
  ...>         }
  ...>         state awake { entry action mark { assign count := count + 10; } }
  ...>     }
  ...>     action bumpBy { in n; first apply; action apply { assign count := count + n; } }
  ...> }
✓ part def Monitor

sysml> %instantiate Monitor
✓ Created instance of Monitor
  ID: 1
  Use %features Monitor to inspect

sysml> %state Monitor
✓ Debugging state machine "modes" exhibited by object #1 of "Monitor"
  Current state: idle
  Time: 0.00
  Events: 1

sysml> %step
✓ Event dispatched
  Current state: awake
  Time: 10.00
  Events: 0

sysml> %features Monitor
Instance: Monitor (ID: 1)
Features:
  count = 11
```

`%instantiate` started the machine and `%state Monitor` bound the debugger to *that object's*
machine rather than to a detached run of the usage, so `%step`, `%advance`, `%current` and
`%events` drive it and `%features` shows what its entry actions wrote — `1` from `idle`, then
`10` more from `awake` once the timer was dispatched.

**When a machine starts, and how far it runs.** The object's feature values are built and its
constant defaults evaluated first, so an entry action sees declared initial values; the machine is
then initialized and run to *quiescence* — no event due at the current time, no runnable do action,
no message in flight. A machine waiting on a timer or an `accept` is quiescent, and advancing time
is what moves it on. Objects that signal each other are drained together, bounded by the event and
do-step budgets in [reference/environment.md](../reference/environment.md): an exchange that never
settles reports a budget error rather than hanging. Materializing the same name twice makes a second
object with its own identity and its own machines; `%instantiate` reports the new object, and the
name then denotes it. An exhibited machine with no initial state is reported; a performed action that
states no flow simply has no step to perform, so the object is still created. A performed action
parked at an `accept` is quiescent too, and a message a sibling object sends later wakes it.

**Editing the model while an object runs.** An unrelated declaration keeps the object — its identity
survives the rebuilt analysis — but not the execution it was running: an execution belongs to the
graph, names and message bus of the analysis it started in, so the object's behaviors are **started
again from their initial states** in the rebuilt analysis and what the discarded run wrote is
dropped with them. The restart is reported (`note: the exhibited state machine modes of object #1 was
restarted from its initial state because the model was rebuilt`), and a `%state` session follows the
object onto its restarted machine, so a restarted behavior exchanges messages with objects
materialized after the edit like any other. Re-declaring what the object runs — its type's features
or the body of a machine or action it runs — makes it a different object, so it is dropped with a
reported reason and `%instantiate` starts a fresh one.

**Invoking an operation.** `%invoke <object> <op> [<p>=<expr>]` runs an action the object's type
owns, performed by that object:

```sysml
sysml> %invoke Monitor bumpBy n=4
✓ Invoked bumpBy on object #1 of "Monitor"

sysml> %features Monitor
Instance: Monitor (ID: 1)
Features:
  count = 15
```

Each argument is written `<parameter>=<expression>`; an unbound parameter, an argument naming no
parameter, and an operation the type does not own are each reported as errors. A `calc` or
`constraint` named as an operation is not invocable this way yet.

## Token-flow patterns

Every model below is in
[examples/action-executor-demo.sysml](../../examples/action-executor-demo.sysml), and the
output shown is what `sysml -action ActionExecutorDemo::<name> examples/action-executor-demo.sysml`
prints.

### Sequential: start → action → done

```sysml
action sequential {
    attribute result : Integer = 0;

    first start;
    action compute { assign result := 42 * 2; }
    done finish;

    then start compute;
    then compute finish;
}
```

One token spawns at `start`, moves to `compute`, which runs its body, and is consumed at
`finish`:

```
✓ Action completed
  Final state: Completed
  Results:
    result = 84
```

---

### Fork and join: parallel paths

```sysml
action forkJoin {
    attribute task1 : Integer = 0;
    attribute task2 : Integer = 0;
    attribute task3 : Integer = 0;

    first start;
    fork split;
    action left { assign task1 := 10; }
    action middle { assign task2 := 20; }
    action right { assign task3 := 30; }
    join sync;
    done finish;

    then start split;
    then split left;
    then split middle;
    then split right;
    then left sync;
    then middle sync;
    then right sync;
    then sync finish;
}
```

`fork` puts a token on each outgoing succession; `join` is an AND-join, so it waits for a
token on *every* incoming succession before one continues. A fork duplicates control, not
values: all three branches are steps of the one performance, so every assignment is visible
when it completes.

```
✓ Action completed
  Final state: Completed
  Results:
    task1 = 10
    task2 = 20
    task3 = 30
```

A branch that never arrives is a deadlock, not a failure: the run is reported as undecided.

---

### Decision and else: conditional branching

```sysml
action conditional {
    attribute x : Integer = 15;
    attribute taken : Integer = 0;

    first start;
    action pathA { assign taken := 1; }
    action pathB { assign taken := 2; }
    done finish;

    then start check;
    then pathA finish;
    then pathB finish;

    decide check;
    if x > 10 then pathA;
    else pathB;
}
```

`decide` evaluates its guards in the order written, with the action's features in scope, and
takes the first that holds; `else` is taken when none does. With `x = 15`:

```
✓ Action completed
  Final state: Completed
  Results:
    taken = 1
    x = 15
```

Set `x` to `5` and `taken` is `2`. The state-machine counterparts — fork/join pseudostates,
choice and junction, history — are in
[examples/state-machine-demo.sysml](../../examples/state-machine-demo.sysml) and
[examples/combined-behavioral-demo.sysml](../../examples/combined-behavioral-demo.sysml), and
every case the executors are held to is under
`internal/core/runtime/testdata/conformance/`.

A run that stops short — a deadlock, or a budget reached — is reported as a check that was never
decided rather than as a failure; the bounds are in
[reference/environment.md](../reference/environment.md).

---

Next: [7. Saving, and converting to RDF](07-saving-and-rdf.md).
