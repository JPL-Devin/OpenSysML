# Lowering Layer Integration Plan

## Status

**Phase 1 COMPLETE**: Explicit execution IR created
- `internal/core/lower/` package exists
- `ActionGraph` and `StateGraph` IR types defined
- `ToActionGraph()` and `ToStateGraph()` conversion functions working
- Tests passing: lowering from AST to graph works

**Phase 2 IN PROGRESS**: Refactor executors to consume IR

## Integration Strategy

### ActionExecutor Refactoring

**Current state:**
- Has embedded graph extraction (lines 169-277 in action_executor.go)
- Uses `nodes`, `edges`, `guards`, `dataFlows` fields
- `extractGraph()` builds these from AST directly

**Target state:**
- Store `graph *lower.ActionGraph` field
- Call `lower.ToActionGraph(action.Decl)` in constructor
- Replace all `e.nodes` → `e.graph.Nodes`
- Replace all `e.edges` → `e.graph.Edges`
- Replace all `e.guards` → `e.graph.Guards`
- Replace all `e.dataFlows` → `e.graph.DataFlows`
- Delete `extractGraph()`, `findNodeByName()`, `parsePinReference()` (now in lower package)

**References to update** (28 locations):
```
191,195,286,383: e.nodes
216,235,246,451,487,556,581,610,631: e.edges
218,219,221,250,251,253,646: e.guards
268,505,507,509,511,516: e.dataFlows
```

**New code:**
```go
// In newActionExecutor:
graph, err := lower.ToActionGraph(action.Decl)
if err != nil {
    return nil, fmt.Errorf("lower action to graph: %w", err)
}
exec.graph = graph
```

### StateExecutor Refactoring

**Current state:**
- Has embedded graph extraction (lines 83-158 in state_executor.go)
- Uses `states`, `pseudostates`, `transitions`, `compositeStates`, etc.
- `extractGraph()` builds these from AST

**Target state:**
- Store `graph *lower.StateGraph` field
- Call `lower.ToStateGraph(stateMachine.Decl)` in constructor
- Replace all field references with `e.graph.*`
- Delete `extractGraph()`, `collectStates()`, `collectTransition()`, etc.
- Update `findStateByName()` to use `e.graph.States`

**Key difference from ActionExecutor:**
- StateGraph.Transition is a struct with Source/Target/Trigger/Guard/Effect
- Current code uses `map[*ast.StateNode][]*ast.TransitionEdge`
- Need to iterate `e.graph.Transitions[state]` and access `.Target`, `.Trigger`, etc.

## Testing Strategy

1. **Unit test refactor**: Update executor unit tests to parse-and-execute
2. **Conformance validation**: Enable finalState checking
3. **Regression check**: Ensure all 23 conformance tests still pass

## Why This Matters

The lowering layer solves the architectural problem: parser emits declarative members (TransitionMember, EntryMember), executors need operational graphs (nodes + edges). Currently executors try to bridge this gap with ad-hoc `extractGraph()` functions that are incomplete. By making the IR explicit:

1. **Testable**: Can validate lowering independent of execution
2. **Debuggable**: Can inspect IR to see what executor will run
3. **Correct**: Parser outputs and executor expectations are decoupled
4. **Evolvable**: Can add new AST constructs without breaking executors

This is the #1 blocker identified in the runtime roadblocks analysis. Once complete, we can properly test state machine execution and implement the remaining missing features.
