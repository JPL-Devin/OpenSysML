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

## Token-flow patterns

### Sequential: initial → action → final

```sysml
action def SequentialAction {
    action start : InitialNode;
    action compute : ActionExecutionNode {
        return 42 * 2;  // Evaluates inline expression
    }
    action end : FinalNode;
    
    succession start then compute then end;
}
```

**Execution Flow:**
1. Token spawns at `start` (InitialNode)
2. Token moves to `compute` (ActionExecutionNode)
3. Expression `42 * 2` evaluates, result stored in token data
4. Token moves to `end` (FinalNode)
5. Token consumed, execution completes

**Internal mechanism:** `stepToken(0)` called 3 times to advance token through graph.

---

### Fork and join: parallel paths

```sysml
action def ParallelAction {
    action start : InitialNode;
    action fork : ForkNode;
    
    action task1 : ActionExecutionNode { return 10; }
    action task2 : ActionExecutionNode { return 20; }
    action task3 : ActionExecutionNode { return 30; }
    
    action join : JoinNode;
    action end : FinalNode;
    
    succession start then fork;
    succession fork then task1;
    succession fork then task2;
    succession fork then task3;
    succession task1 then join;
    succession task2 then join;
    succession task3 then join;
    succession join then end;
}
```

**Execution Flow:**
1. 1 token at `start`
2. Fork: **1 token → 3 concurrent tokens** (task1, task2, task3)
3. Each token evaluates its action independently
4. Join: **Barrier synchronization** - waits for ALL 3 tokens to arrive
5. Join: **3 tokens → 1 merged token** (data merged via last-write-wins)
6. Token continues to `end`, consumed, completes

**Key semantics:**
- ForkNode creates N tokens (one per outgoing edge), data copied to each
- JoinNode waits until tokens on ALL incoming edges arrive (Petri-net AND-join)
- Data merge uses last-write-wins strategy for conflicting keys

---

### Decision and merge: conditional branching

```sysml
action def ConditionalAction {
    action start : InitialNode;
    action decision : DecisionNode;
    
    action pathA : ActionExecutionNode { return "took path A"; }
    action pathB : ActionExecutionNode { return "took path B"; }
    
    action merge : MergeNode;
    action end : FinalNode;
    
    succession start then decision;
    flow decision to pathA if (x > 10);   // Guard: x > 10
    flow decision to pathB if (x <= 10);  // Guard: x <= 10
    succession pathA then merge;
    succession pathB then merge;
    succession merge then end;
}
```

**Execution Flow with x=15:**
1. Token at `decision` with data `{x: 15}`
2. DecisionNode evaluates guards in order:
   - Guard `x > 10`: `15 > 10` → **true** → take pathA
3. Token executes `pathA`, stores result
4. Token reaches `merge`
5. MergeNode: **first token wins**, marks merge as visited
6. Token continues to `end`

**Execution Flow with x=5:**
1. Token at `decision` with data `{x: 5}`
2. DecisionNode evaluates guards:
   - Guard `x > 10`: `5 > 10` → false
   - Guard `x <= 10`: `5 <= 10` → **true** → take pathB
3. Token executes `pathB`, stores result
4. Token reaches `merge` (**first to arrive wins**)
5. Token continues to `end`

**Key semantics:**
- DecisionNode evaluates guards with token data in scope
- First true guard wins (deterministic, order-dependent)
- Unguarded edges act as "else" branch (evaluated last)
- MergeNode: first token passes, subsequent tokens discarded (OR-join)

---

### All of them at once: fork → decision → merge → join

```sysml
action def ComplexWorkflow {
    action start : InitialNode;
    action fork : ForkNode;
    
    // Branch 1: Conditional path
    action decision1 : DecisionNode;
    action branch1A : ActionExecutionNode { return "1A"; }
    action branch1B : ActionExecutionNode { return "1B"; }
    action merge1 : MergeNode;
    
    // Branch 2: Conditional path
    action decision2 : DecisionNode;
    action branch2A : ActionExecutionNode { return "2A"; }
    action branch2B : ActionExecutionNode { return "2B"; }
    action merge2 : MergeNode;
    
    action join : JoinNode;
    action final : ActionExecutionNode { return "complete"; }
    action end : FinalNode;
    
    succession start then fork;
    
    // Branch 1
    succession fork then decision1;
    flow decision1 to branch1A if (x > 50);
    flow decision1 to branch1B;  // else
    succession branch1A then merge1;
    succession branch1B then merge1;
    succession merge1 then join;
    
    // Branch 2
    succession fork then decision2;
    flow decision2 to branch2A if (x > 25);
    flow decision2 to branch2B;  // else
    succession branch2A then merge2;
    succession branch2B then merge2;
    succession merge2 then join;
    
    succession join then final;
    succession final then end;
}
```

**Execution Flow with x=60:**
1. Fork: 1 token → 2 concurrent tokens (branch1, branch2)
2. **Branch 1:** decision1 evaluates `60 > 50` → true → branch1A → merge1
3. **Branch 2:** decision2 evaluates `60 > 25` → true → branch2A → merge2
4. Join: waits for both branches to reach join
5. Join: 2 tokens → 1 merged token
6. Final action executes, token to end, consumed

**Execution Flow with x=30:**
1. Fork: 1 token → 2 concurrent tokens
2. **Branch 1:** decision1: `30 > 50` → false → else branch → branch1B → merge1
3. **Branch 2:** decision2: `30 > 25` → true → branch2A → merge2
4. Join: both branches synchronized
5. Complete

**Execution Flow with x=10:**
1. Fork: 1 token → 2 concurrent tokens
2. **Branch 1:** decision1: `10 > 50` → false → branch1B → merge1
3. **Branch 2:** decision2: `10 > 25` → false → branch2B → merge2
4. Join: both branches synchronized
5. Complete

**Key semantics:**
- Combines all control flow patterns
- Fork creates concurrent execution paths
- Each path independently evaluates decisions
- Merge (OR-join) within each branch
- Join (AND-join) synchronizes parallel branches
- Demonstrates deterministic, compositional semantics

Each pattern above has a runnable model in
[examples/action-executor-demo.sysml](../../examples/action-executor-demo.sysml); the state
machine equivalents are in
[examples/state-machine-demo.sysml](../../examples/state-machine-demo.sysml) and
[examples/combined-behavioral-demo.sysml](../../examples/combined-behavioral-demo.sysml).

A run that stops short — a deadlock, or a budget reached — is reported as a check that was never
decided rather than as a failure; the bounds are in
[reference/environment.md](../reference/environment.md).

---

Next: [7. Saving, and converting to RDF](07-saving-and-rdf.md).
