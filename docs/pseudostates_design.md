# Pseudostates Design - Choice and Junction

**Status:** Implementation Plan  
**UML Reference:** UML 2.5.1 §14.2.3.4 (Pseudostates)

## Overview

Pseudostates are transient vertices in state machines that enable complex control flow. This document covers **choice** and **junction** pseudostates.

### Choice vs Junction

**Choice (Dynamic Conditional Branch):**
- Guards evaluated **when entered** (runtime decision)
- Used for dynamic branching based on current conditions
- Outgoing transitions evaluated in order until one guard succeeds
- Must have at least one guard that evaluates to true (or else clause)

**Junction (Static Merge/Branch):**
- Guards evaluated **before entering** (static connectors)
- Used to merge multiple incoming transitions or split paths
- All outgoing guards must be mutually exclusive and complete
- Deterministic - no runtime evaluation order

### UML Semantics (UML 2.5.1 §14.2.3.4.3-4)

**Choice:**
> "A choice vertex is a dynamic conditional branch. The guards on the outgoing transitions are evaluated only when the choice is entered, at run time."

**Junction:**
> "A junction vertex is a static conditional branch. The guards on the outgoing transitions are evaluated when the incoming transition(s) are evaluated."

## Syntax

### SysML v2 / KerML Syntax

**Choice:**
```sysml
state def TrafficController {
    state Green;
    state Yellow;
    state Red;
    
    choice priorityCheck;
    
    transition Green to priorityCheck;
    transition priorityCheck to Yellow if (emergencyVehicle);
    transition priorityCheck to Red if (not emergencyVehicle);
}
```

**Junction:**
```sysml
state def SafetyMonitor {
    state Nominal;
    state Warning;
    state Critical;
    
    junction statusEval;
    
    transition statusEval to Nominal if (temp < 50);
    transition statusEval to Warning if (temp >= 50 and temp < 100);
    transition statusEval to Critical if (temp >= 100);
}
```

## Implementation Plan

### Phase 1: Parser Support

**Add to `parseStateMember()` in behavior.go:**
- Detect `choice` keyword
- Detect `junction` keyword
- Parse pseudostate name
- Create `PseudostateNode` with appropriate `Kind`

**Changes:**
- Add `choice` and `junction` to state member keyword dispatch
- Add `parseChoicePseudostate()` function
- Add `parseJunctionPseudostate()` function

### Phase 2: State Executor Graph Extraction

**Update `extractGraph()` in state_executor.go:**
- Collect `PseudostateNode` instances
- Add to `states` map (pseudostates are vertices like states)
- Extract transitions targeting pseudostates

**Data structures:**
- Add `pseudostates map[string]*ast.PseudostateNode`
- Transitions already support guard expressions via `TransitionMember.Guard`

### Phase 3: Runtime Evaluation

**Choice Evaluation:**
```go
func (e *StateExecutor) evaluateChoice(choiceName string, event Event) (string, error) {
    // Get all outgoing transitions from choice
    outgoing := e.getOutgoingTransitions(choiceName)
    
    // Evaluate guards in order (runtime)
    for _, trans := range outgoing {
        if trans.Guard == nil {
            return trans.Target, nil // else clause
        }
        
        satisfied, err := e.evaluateGuard(trans.Guard)
        if err != nil {
            return "", err
        }
        if satisfied {
            return trans.Target, nil
        }
    }
    
    return "", fmt.Errorf("no guard satisfied at choice %s", choiceName)
}
```

**Junction Evaluation:**
- Similar to choice but guards evaluated before entering junction
- Used in `fireTransition()` when source transition leads to junction
- All guards must be mutually exclusive (validation)

### Phase 4: Transition Firing Integration

**Update `fireTransition()` in state_executor.go:**

```go
// After exiting source state
target := transition.Target

// Check if target is pseudostate
if ps, ok := e.pseudostates[target]; ok {
    switch ps.Kind {
    case ast.PseudostateChoice:
        // Evaluate guards at runtime
        nextTarget, err := e.evaluateChoice(target, event)
        if err != nil {
            return err
        }
        target = nextTarget
        
    case ast.PseudostateJunction:
        // Guards already evaluated, pick deterministic path
        nextTarget, err := e.evaluateJunction(target)
        if err != nil {
            return err
        }
        target = nextTarget
    }
}

// Enter final target state
e.enterState(target)
```

### Phase 5: Conformance Tests

**Test: state_choice_pseudostate.sysml**
```sysml
package ChoiceTest {
    state def Controller {
        attribute priority : Integer = 0;
        
        state Idle;
        state HighPriority;
        state LowPriority;
        
        choice checkPriority;
        
        initial init;
        transition init then Idle;
        transition Idle accept Start to checkPriority;
        transition checkPriority to HighPriority if (priority > 5);
        transition checkPriority to LowPriority if (priority <= 5);
    }
    
    state sm : Controller;
}
```

**Expected output:**
```json
{
  "finalState": "LowPriority",
  "stateVisits": ["Idle", "LowPriority"],
  "outputs": {}
}
```

**Test: state_junction_pseudostate.sysml**
```sysml
package JunctionTest {
    state def Monitor {
        attribute temp : Integer = 75;
        
        state Nominal;
        state Warning;
        state Critical;
        
        junction statusEval;
        
        initial init;
        transition init then statusEval;
        transition statusEval to Nominal if (temp < 50);
        transition statusEval to Warning if (temp >= 50 and temp < 100);
        transition statusEval to Critical if (temp >= 100);
    }
    
    state sm : Monitor;
}
```

**Expected output:**
```json
{
  "finalState": "Warning",
  "stateVisits": ["Warning"],
  "outputs": {}
}
```

### Phase 6: Documentation

**Update SPEC_COMPLIANCE.md:**
- Move choice/junction from "Advanced" to "Implemented"
- State Machines: 14/14 → 16/16 features
- Add test coverage: 21 → 23 conformance cases

## Validation Rules

1. **Choice:**
   - Must have at least one outgoing transition
   - At least one guard must be satisfiable (or else clause)
   - Guards evaluated in definition order

2. **Junction:**
   - Outgoing guards must be mutually exclusive
   - Guards must be complete (cover all cases)
   - No runtime evaluation order (deterministic)

3. **Both:**
   - Cannot have entry/exit/do behaviors
   - Must be transient (no dwelling)
   - Incoming transitions can have guards
   - Outgoing transitions must have guards (except else clause)

## Backward Compatibility

✅ No breaking changes:
- Existing state machines continue to work
- Pseudostates are additive features
- No changes to state transition semantics

## References

- UML 2.5.1 §14.2.3.4.3 (Choice Pseudostates)
- UML 2.5.1 §14.2.3.4.4 (Junction Pseudostates)
- SysML v2 Pilot Implementation (state machine examples)

## Fork and Join

Fork and join are declared like choice and junction:

```sysml
state Machine {
    initial init;
    state idle;
    state running {
        region left  { initial ls; state working;  then ls working; }
        region right { initial rs; state watching; then rs watching; }
    }
    fork split;
    join sync;
    final done;

    init then idle;
    transition idle to split;
    transition split to working;    // one branch per region
    transition split to watching;
    transition working to sync;
    transition watching to sync;
    transition sync to done;
}
```

**Fork** (`fireForkTransition`): every outgoing branch is taken at once. Branches
must be unguarded and must target states in distinct orthogonal regions of one
composite state; that composite state is entered and each region's active state
becomes its branch target instead of the region's initial state.

**Join** (`fireJoinTransition`): the transition only proceeds once the source
state of every incoming branch is active. A branch that arrives early leaves its
source state active and waits, so the join synchronizes the regions before the
composite state is exited.

**Entry/exit points** (`ast.PseudostateEntry`, `ast.PseudostateExit`) are routed
like a junction — the transition continues along the point's own outgoing
transition — but there is no textual notation for them, so they can only be built
programmatically.

**History** (`ast.PseudostateShallowHistory`, `ast.PseudostateDeepHistory`) is
owned by the composite state it restores — `lower.StateGraph.PseudostateOwner`
records that ownership — and re-enters the configuration that state was last left
in. `exitState` records the configuration on the way out, per composite state: the
substate that was active, and the active state of each orthogonal region.

`fireHistoryTransition` reads that record before leaving the source
configuration, since a transition out of the owner's own substates would
otherwise overwrite it, then:

- a **shallow** history enters the recorded substate, or re-enters the owner with
  one branch per region holding that region's recorded state;
- a **deep** history keeps following the record downwards (`deepestRecorded`),
  collecting a branch for every nested region it passes, so the innermost recorded
  state is the one entered;
- when the owner has never been exited there is nothing to restore, so the
  history's own outgoing transition supplies the target — UML's default history
  transition.

Entering a branch nested below a region runs the entry behaviors of the states
above it inside that region, so a restored deep configuration is not entered
sideways.

Like entry/exit points, history has no textual notation, so it can only be built
programmatically.
