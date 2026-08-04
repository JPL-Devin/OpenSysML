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
