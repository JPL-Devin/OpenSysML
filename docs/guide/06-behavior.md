# 6. Behavior: actions and state machines

Actions and state machines are executed, not just parsed. A debugger steps through
them, and the non-interactive `-action` and `-state` flags run them to completion and report the
values they produce. A behavior can be performed by an object, in which case the messages it sends
are routed over that object's connections.

**Action execution (step-by-step):**
```sysml
sysml> action SimpleWorkflow {
  ...>     attribute result = 0;
  ...>     first start;
  ...>     then action compute { assign result := 42; }
  ...>     then done;
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
  Values:
    result = 0

sysml> %continue
✓ Action completed
  Final state: Completed
  Results:
    result = 42
```

**State machine execution:**

A machine completes when a transition reaches `done`, the terminal state the standard
library provides for every state machine. Entering it runs the exit actions, and then the
machine reports itself completed. With orthogonal regions, each region has its own `done`,
and the machine completes only once every region has reached it.

```sysml
sysml> state TrafficLight {
  ...>     entry; then start;
  ...>     state start;
  ...>     state green { accept after 25 then yellow; }
  ...>     state yellow { accept after 5 then red; }
  ...>     state red { accept after 30 then done; }
  ...>     succession first start then green;
  ...> }
✓ state TrafficLight

sysml> %state TrafficLight
✓ Started state machine executor for "TrafficLight"
  Current state: start
  Time: 0.0
  Events: 1

Use %events to see queue, %current for state, %advance <time> to step

sysml> %advance 25
✓ Advanced to 25.0 (2 event(s) processed)
  Current state: yellow
  Last event at: 25.0
  Remaining events: 1

sysml> %current
Current state: yellow
Time: 25.0
Last event at: 25.0
Execution state: Running

sysml> %advance 5
✓ Advanced to 30.0 (1 event(s) processed)
  Current state: red
  Last event at: 30.0
  Remaining events: 1

sysml> %advance 30
✓ Advanced to 60.0 (1 event(s) processed)
  Current state: done
  Last event at: 60.0
  Remaining events: 0

✓ State machine completed (a transition reached `done`)
```

**Action debugging commands:**
- `%action <name> [<object>]` — Start an action debugging session, optionally performed by an instantiated object
- `%step` — Advance all tokens one step
- `%continue` — Run to completion, or to the first breakpoint hit
- `%tokens` — Show active tokens with data
- `%break <node>` — Set breakpoint on a named node; `%continue` stops when a token reaches it
- `%stop` — Stop debugging

**State machine debugging commands:**
- `%state <name> [<object>]` — Start a state machine debugging session; naming an instantiated object runs the machine on behalf of that object, so what it sends routes over that object's connections. Naming the machine the object exhibits attaches to its running machine instead (see [below](#an-object-runs-the-behaviors-its-type-exhibits))
- `%events` — Show event queue
- `%current` — Show current state, stack, data
- `%advance <time>` — Advance simulation time by `<time>` units, processing every event due
- `%stop` — Stop debugging

For complete workflows, see
[examples/action-executor-demo.sysml](../../examples/action-executor-demo.sysml),
[examples/orthogonal-regions-demo.sysml](../../examples/orthogonal-regions-demo.sysml) and
[examples/pseudostates-demo.sysml](../../examples/pseudostates-demo.sysml).

## An object runs the behaviors its type exhibits

A type that exhibits a state machine or performs an action binds that behavior to every object of
the type: instantiating an object gives it an execution of its own, tied to its identity. Two
objects of the same type run two independent machines, each with its own current state, event
queue and feature values, and an assignment in a behavior body writes the feature value of the
object performing it.

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
  ...>     action bumpBy { in n; action apply { assign count := count + n; } first apply; then done; }
  ...> }
✓ part def Monitor

sysml> %instantiate Monitor
✓ Created instance of Monitor
  ID: 1
  Use %features Monitor to inspect

sysml> %state Monitor
✓ Debugging state machine "modes" exhibited by object #1 of "Monitor"
  Current state: idle
  Time: 0.0
  Events: 1

sysml> %step
✓ Event dispatched
  Current state: awake
  Time: 10.0
  Events: 0

sysml> %features Monitor
Instance: Monitor (ID: 1)
Features:
  count = 11
  modes = Instance(ID: 2)
    idle = <unknown>
    awake = <unknown>
  bumpBy = Instance(ID: 3)
    n = <unknown>
    apply = <unknown>
```

`%instantiate` started the machine, and `%state Monitor` attached the debugger to *that object's*
machine rather than to a detached run of the usage. `%step`, `%advance`, `%current` and `%events`
therefore drive that machine, and `%features` shows the values its entry actions wrote: `1` from
`idle`, then `10` more from `awake` once the timer fired.

The two-argument form does the same when the machine it names is the one the object exhibits:
`%state Monitor::modes Monitor` attaches to the running machine and says so in a `note:` line,
rather than performing `modes` a second time against the same feature values (which would run
its entry actions again, leaving `count` at `2` instead of `1`). Only a machine the object does
not exhibit — one it merely performs — is started as a detached performance by that form. When
the object exhibits one definition as several usages (`exhibit state front : Blink; exhibit
state rear : Blink;`), naming the definition names no one machine, so `%state Blink lamp` refuses
and names `Lamp::front` and `Lamp::rear` to name instead.

The object can also be a part reached through composition, or an id. With `part def Driver {
part r : Monitor; }`, `part driver : Driver;` and `%instantiate driver`, `%state driver.r` debugs the nested part's own
machine, and `%state #2` the same by the identity `%features driver` prints for it (`r =
Instance(ID: 2)`). A path that stops short of an object says which segment failed:

```sysml
sysml> %state driver.x
error: driver.x reaches no object at "x": object #1 of "driver" has no feature "x"
```

Naming a usage whose *definition* alone was instantiated is reported as such, with what to
instantiate instead. With `part monitor : Monitor;` declared:

```sysml
sysml> %instantiate Monitor
sysml> %state Monitor::modes monitor
error: no instance of the usage "monitor": object #1 of "Monitor" is of its definition "Monitor", not of the usage — use %instantiate monitor to create the usage's object, or name Monitor to address it
```

**When a machine starts, and how far it runs.** The object's feature values are built and its
constant defaults evaluated first, so an entry action sees the declared initial values. The
machine is then initialized and run until it is *quiescent*: no event is due at the current
time, no do action can run, and no message is in flight. A machine waiting on a timer or an
`accept` is quiescent, and advancing time is what lets it proceed. Objects that signal one
another are run together until they all settle, within the event and do-step budgets described in
[reference/environment.md](../reference/environment.md); an exchange that never settles reports a
budget error rather than hanging. Instantiating the same name twice creates a second object with
its own identity and its own machines: `%instantiate` reports the new object, and the name then
refers to it. An exhibited machine with no initial state is reported as such. A performed action
that declares no flow has nothing to step, but the object is still created. A performed action
waiting at an `accept` is also quiescent, and a message sent later by a sibling object
wakes it up.

**Editing the model while an object runs.** Submitting an unrelated declaration keeps the object
(its identity survives the rebuilt analysis) but not the execution it was running. An execution
belongs to the graph, names and message bus of the analysis it started in, so the object's
behaviors are **restarted from their initial states** in the rebuilt analysis, and any values the
discarded run wrote are dropped with it. The restart is reported (`note: the exhibited state machine
modes of object #1 was restarted from its initial state because the model was rebuilt`), and a
`%state` session follows the object onto its restarted machine, so a restarted behavior exchanges
messages with objects instantiated after the edit in the usual way. Redeclaring what the object
runs (its type's features, or the body of a machine or action it runs) produces a different
object, so the original is dropped with a stated reason and `%instantiate` creates a new one.

**Invoking an operation.** `%invoke <object> <op> [<p>=<expr>]` runs an action owned by the
object's type, performed by that object — named as `%state` names one, so `%invoke driver.r bumpBy
n=4` and `%invoke #2 bumpBy n=4` reach a nested part:

```sysml
sysml> %invoke Monitor bumpBy n=4
✓ Invoked bumpBy on object #1 of "Monitor"

sysml> %features Monitor
Instance: Monitor (ID: 1)
Features:
  count = 15
  modes = Instance(ID: 2)
    idle = <unknown>
    awake = <unknown>
  bumpBy = Instance(ID: 3)
    n = <unknown>
    apply = <unknown>
```

Each argument is written as `<parameter>=<expression>`. An unbound parameter, an argument that
names no parameter, and an operation the type does not own are each reported as errors. A `calc`
or `constraint` cannot yet be invoked this way.

## Token-flow patterns

Each model below is in
[examples/action-executor-demo.sysml](../../examples/action-executor-demo.sysml), and the output
shown is from
`sysml -action ActionExecutorDemo::<name> examples/action-executor-demo.sysml`.

### Sequential: start → action → done

The sequential flow below uses standard implicit succession notation:

```sysml
action sequential {
    attribute result : Integer = 0;

    first start;
    then action compute { assign result := 42 * 2; }
    then done;
}
```

A single token is created at `start`, moves to `compute`, which runs its body, and is consumed at
`done`:

```
$ sysml -action ActionExecutorDemo::sequential examples/action-executor-demo.sysml
✓ Action completed
  Final state: Completed
  Results:
    result = 84
```

---

### Fork and join: parallel paths

`fork` and `join` are action node literals, so this action is standard notation.

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

    succession first start then split;
    succession first split then left;
    succession first split then middle;
    succession first split then right;
    succession first left then sync;
    succession first middle then sync;
    succession first right then sync;
    succession first sync then done;
}
```

`fork` places a token on each outgoing succession. `join` is an AND-join: it waits for a token
on *every* incoming succession before a single token continues. A fork duplicates control, not
values: all three branches are steps of the same performance, so every assignment is visible
when it completes.

```bash
$ sysml -action ActionExecutorDemo::forkJoin examples/action-executor-demo.sysml
✓ Action completed
  Final state: Completed
  Results:
    task1 = 10
    task2 = 20
    task3 = 30
```

If a branch never arrives, that is a deadlock rather than a failure, and the run is reported
as undecided.

---

### Decision and else: conditional branching

`decide` with a guarded branch and an `else` branch is standard notation. The
OpenSysML spelling `decision` is the one that produces a warning.

```sysml
action conditional {
    attribute x : Integer = 15;
    attribute taken : Integer = 0;

    first start;
    action pathA { assign taken := 1; }
    action pathB { assign taken := 2; }

    succession first start then check;
    succession first pathA then done;
    succession first pathB then done;

    decide check;
    if x > 10 then pathA;
    else pathB;
}
```

`decide` evaluates its guards in the order written, with the action's features in scope, and takes
the first guard that holds. The `else` branch is taken when no guard holds. With `x = 15`:

```bash
$ sysml -action ActionExecutorDemo::conditional examples/action-executor-demo.sysml
✓ Action completed
  Final state: Completed
  Results:
    taken = 1
    x = 15
```

Setting `x` to `5` gives `taken = 2`. The state-machine counterparts (orthogonal
regions, choice and junction) appear in
[examples/orthogonal-regions-demo.sysml](../../examples/orthogonal-regions-demo.sysml) and
[examples/pseudostates-demo.sysml](../../examples/pseudostates-demo.sysml), and every case the
executors are tested against lives under `internal/core/runtime/testdata/conformance/`.

A run that stops early, whether through deadlock or by hitting a budget, is reported as an
undecided check rather than a failure. The budgets are documented in
[reference/environment.md](../reference/environment.md).

---

Next: [7. Saving, and converting to RDF](07-saving-and-rdf.md).
