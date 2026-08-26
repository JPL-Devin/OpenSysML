# Orthogonal Regions Implementation Design

## Overview

Implement orthogonal regions (concurrent substates) for state machines. `region` is an OpenSysML
extension: SysML v2 has no notation for it, and the bundled libraries have no region performance,
so UML 2.5.1 §14.2.3.3 is the reference semantics. The nearest library anchor is that only
`States.sysml` `exclusiveStates` are sequenced (`succession stateSequencing first [0..1]
exclusiveStates then [0..1] exclusiveStates`), so substates outside that set are not ordered
against each other; concurrency across regions is not derived from it.

**Goal:** Support multiple simultaneously active substates within a composite state (AND-composition).

## Current State vs Target

### Current (Single Active State)
```
StateExecutor:
  currentState: *StateNode  // Single active state
  stateStack: []*StateNode  // Hierarchical chain (parent→child)
```

### Target (Active Configuration Set)
```
StateExecutor:
  activeConfig: *StateConfiguration  // Active state configuration
  
StateConfiguration:
  regionStates: map[*StateRegion]*StateNode  // One active state per region
  hierarchyStack: []*StateNode               // Parent chain (unchanged)
```

## Key Changes

### 1. Active Configuration Model

**Replace:**
- `currentState *ast.StateNode` (single)

**With:**
- `activeConfig *StateConfiguration` (multi-region)

```go
type StateConfiguration struct {
    // For states WITH regions: map of region → active state in that region
    regionStates map[*StateRegion]*ast.StateNode
    
    // For states WITHOUT regions: single active state (nil if multi-region)
    simpleState *ast.StateNode
    
    // Hierarchical parent chain (unchanged from current)
    hierarchyStack []*ast.StateNode
}
```

**Rationale:** 
- Simple states work as before (simpleState field)
- Composite states with regions use regionStates map
- Backward compatible: existing single-state logic preserved

### 2. Region Structure Identification

**During extractGraph():**

```go
func (e *StateExecutor) extractGraph() error {
    // ...existing state collection...
    
    // NEW: For each state, check if it has regions
    for _, state := range e.states {
        if len(state.Regions) > 0 {
            // Mark state as composite with regions
            e.compositeStates[state] = state.Regions
            
            // Each region must have initial state
            for _, region := range state.Regions {
                initialState := findInitialInRegion(region)
                if initialState == nil {
                    return fmt.Errorf("region %s has no initial state", region.Name)
                }
                e.regionInitials[region] = initialState
            }
        }
    }
}
```

**New fields needed:**
- `compositeStates map[*StateNode][]*StateRegion` - states with orthogonal regions
- `regionInitials map[*StateRegion]*StateNode` - initial state per region

### 3. Event Processing (Broadcast to All Regions)

**Current (single state):**
```go
func (e *StateExecutor) ProcessNextEvent() error {
    event := e.eventQueue.Dequeue()
    
    // Check transitions from currentState
    for _, trans := range e.transitions[e.currentState] {
        if e.matchesTransition(trans, event) {
            e.fireTransition(trans, event)
            return nil
        }
    }
}
```

**Target (multi-region):**
```go
func (e *StateExecutor) ProcessNextEvent() error {
    event := e.eventQueue.Dequeue()
    
    // If in composite state with regions, broadcast to all
    if len(e.activeConfig.regionStates) > 0 {
        for region, regionState := range e.activeConfig.regionStates {
            // Check transitions from this region's active state
            for _, trans := range e.transitions[regionState] {
                if e.matchesTransition(trans, event) {
                    e.fireTransitionInRegion(region, trans, event)
                    break  // Run-to-completion within region
                }
            }
        }
    } else {
        // Simple state (no regions) - existing logic
        currentState := e.activeConfig.simpleState
        for _, trans := range e.transitions[currentState] {
            if e.matchesTransition(trans, event) {
                e.fireTransition(trans, event)
                return nil
            }
        }
    }
}
```

**Key insight:** Events broadcast to all regions. Each region processes event independently (run-to-completion per region).

### 4. State Entry/Exit with Regions

**Entering composite state with regions:**
```go
func (e *StateExecutor) enterState(state *StateNode) error {
    // Execute entry actions (existing)
    e.executeEntryActions(state)
    
    // NEW: If state has regions, enter initial state in each region
    if regions, isComposite := e.compositeStates[state]; isComposite {
        e.activeConfig.regionStates = make(map[*StateRegion]*StateNode)
        for _, region := range regions {
            initialState := e.regionInitials[region]
            e.activeConfig.regionStates[region] = initialState
            e.enterState(initialState)  // Recursive entry
        }
    } else {
        // Simple state (no regions)
        e.activeConfig.simpleState = state
    }
    
    // Execute do behavior (existing)
    e.executeDoActivity(state)
}
```

**Exiting composite state with regions:**
```go
func (e *StateExecutor) exitState(state *StateNode) error {
    // NEW: If state has regions, exit active state in each region
    if len(e.activeConfig.regionStates) > 0 {
        for _, regionState := range e.activeConfig.regionStates {
            e.exitState(regionState)  // Recursive exit
        }
        e.activeConfig.regionStates = nil
    }
    
    // Execute exit actions (existing)
    e.executeExitActions(state)
    e.activeConfig.simpleState = nil
}
```

### 5. Fork/Join Transitions (Cross-Region)

**Fork transition (1 → N):**
- Source: single state in one region (or no region)
- Target: multiple states across different regions

**Join transition (N → 1):**
- Source: specific states in all regions (AND-join)
- Target: single state (possibly exiting composite state)

**Implementation:**

```go
type TransitionEdge struct {
    // ... existing fields ...
    
    // NEW: Multi-target for fork transitions
    Targets []*QualifiedName  // Multiple targets (fork)
    
    // NEW: Multi-source for join transitions
    Sources []*QualifiedName  // Multiple sources (join, all must be active)
}

func (e *StateExecutor) fireTransition(trans *TransitionEdge, event *Event) error {
    // FORK: Single source, multiple targets
    if len(trans.Targets) > 1 {
        sourceState := e.resolveState(trans.Source)
        e.exitState(sourceState)
        
        for _, targetName := range trans.Targets {
            targetState := e.resolveState(targetName)
            targetRegion := e.findRegionForState(targetState)
            e.activeConfig.regionStates[targetRegion] = targetState
            e.enterState(targetState)
        }
        return nil
    }
    
    // JOIN: Multiple sources, single target (check all active)
    if len(trans.Sources) > 1 {
        // Check all source states are active in their respective regions
        for _, sourceName := range trans.Sources {
            sourceState := e.resolveState(sourceName)
            sourceRegion := e.findRegionForState(sourceState)
            if e.activeConfig.regionStates[sourceRegion] != sourceState {
                return nil  // Join condition not met
            }
        }
        
        // All sources active, perform join
        for _, sourceName := range trans.Sources {
            sourceState := e.resolveState(sourceName)
            e.exitState(sourceState)
        }
        
        targetState := e.resolveState(trans.Target)
        e.enterState(targetState)
        return nil
    }
    
    // Regular transition (existing logic)
    // ...
}
```

## Parser Changes

**Need to populate `StateNode.Regions` field:**

```go
func (p *parser) parseStateMember(members []ast.Node) ast.Node {
    // ... existing entry/exit/do/transition parsing ...
    
    // NEW: Parse region keyword
    if p.acceptKeyword("region") {
        region := &ast.StateRegion{
            Name: p.expectIdent().Name,
        }
        p.expect(token.LBrace)
        
        // Parse states within region
        for !p.at(token.RBrace) {
            if state := p.parseStateMember(members); state != nil {
                region.States = append(region.States, state)
            }
        }
        
        p.expect(token.RBrace)
        return region
    }
    
    // ... rest of existing logic ...
}
```

## Test Cases

### Conformance Test: state_orthogonal_regions.sysml

```sysml
state def TrafficLight {
    region pedestrian {
        entry; then start;
        state start;
        state Walk;
        state DontWalk;
        first start then Walk;
        transition first Walk when timer > 5 then DontWalk;
    }
    
    region vehicle {
        entry; then start;
        state start;
        state Green;
        state Yellow;
        state Red;
        first start then Green;
        transition first Green when timer > 30 then Yellow;
        transition first Yellow when timer > 3 then Red;
        transition first Red when timer > 20 then Green;
    }
}
```

**Expected behavior:**
- Both regions active simultaneously
- `Walk` + `Green` initially active
- Events processed independently in each region
- No coordination between regions (unless explicit join transition)

### Conformance Test: state_fork_join.sysml

```sysml
state def Parallel {
    entry; then start;
    state start;
    state Sequential;
    
    state Composite {
        region A {
            entry; then startA;
            state startA;
            state TaskA;
            first startA then TaskA;
        }
        
        region B {
            entry; then startB;
            state startB;
            state TaskB;
            first startB then TaskB;
        }
    }
    
    state Done;
    
    // FORK: Sequential → Composite (enters both regions)
    transition first Sequential when fork then Composite;
    
    // JOIN: Both regions must be in specific states
    transition join {
        from TaskA, TaskB;
        to Done;
        when allDone;
    }
}
```

## Implementation Plan

1. **Phase 1:** Update AST + Parser (populate Regions field)
2. **Phase 2:** Add StateConfiguration struct + activeConfig field
3. **Phase 3:** Refactor enterState/exitState to handle regions
4. **Phase 4:** Update ProcessNextEvent to broadcast events
5. **Phase 5:** Implement fork/join transitions
6. **Phase 6:** Create conformance tests
7. **Phase 7:** Update documentation

## Backward Compatibility

**Guaranteed:**
- States without regions work exactly as before (use simpleState field)
- Existing tests continue to pass (no regions defined)
- Hierarchical states (parent/child) unchanged
- stateStack for history tracking preserved

## UML 2.5.1 Compliance (reference semantics for this extension)

**Implemented:**
- §14.2.3.3.1: Composite states with orthogonal regions
- §14.2.3.3.2: Active configuration (one state per region)
- §14.2.3.3.3: Event broadcast to all regions
- §14.2.3.3.4: Fork transitions (1→N)
- §14.2.3.3.5: Join transitions (N→1 with AND condition)

**Not implemented (future):**
- Transition priorities within regions
- Inter-region communication (requires ports)
- Region-specific event queues (single queue sufficient for most cases)
