# SysML v2 Behavioral AST & Parser Extensions — Design Specification

**Date:** 2026-07-30  
**Status:** Design  
**Prerequisites:** Runtime Tiers 1–3 (evaluation runtime complete)  
**Target:** Parser + AST extensions for Tiers 4–5 (behavioral simulation)

---

## Executive Summary

This specification defines parser and AST extensions to support SysML v2 behavioral modeling (actions and state machines), enabling runtime Tiers 4–5 (behavioral simulation). The current parser (`internal/core/parser/`) produces AST nodes for `action` and `state` definitions/usages but treats their bodies as undifferentiated member lists—no representation for control-flow graphs (fork/join/decision nodes, succession edges) or state machine semantics (transitions, triggers, entry/exit behaviors).

**Approach:** Additive extension following the Tier-B pattern (e.g., `FlowEnds` added to `Usage` without breaking existing code). Behavioral AST nodes live in new `internal/core/ast/behavior.go`, parser logic in new `internal/core/parser/behavior.go`. No modifications to existing `Usage`/`Definition` structs—behavioral nodes populate `Usage.Members` as first-class `Node` implementations.

**Scope:** Comprehensive behavioral modeling—both action control flow (initial/final/fork/join/merge/decision/execution nodes + succession/control-flow edges) and state machines (states with entry/do/exit behaviors, transitions with time/change/accept/call event triggers, pseudostates, hierarchical/orthogonal states). Lexer gains 20 keywords (`first`, `fork`, `join`, `state`, `transition`, `after`, `when`, etc.). Parser hooks into `parseUsageBody()` to delegate action/state body parsing to specialized parsers.

**Deliverables:** 19 new AST types (7 action nodes, 3 state nodes, 4 edge types, 4 trigger event types, 1 region type), 2 parser files (`behavior.go` for AST, `behavior_test.go` for tests), lexer keyword additions, semantic validation guidelines (deferred to passes).

---

## 1. Goals & Scope

### Goals

1. **Enable behavioral simulation (Tiers 4–5):** Provide AST representation for action control flow and state machines, unblocking runtime interpreter implementation.
2. **Preserve existing pipeline:** Additive changes only—no modifications to `Usage`/`Definition` structs, no breaking changes to existing parser logic.
3. **Type-safe AST:** Explicit node types (not discriminated unions) for compile-time correctness.
4. **Spec-compliant syntax:** Support SysML v2 textual syntax for actions (`first`, `fork`, `then`, etc.) and state machines (`state`, `transition`, `after`, `when`, etc.).
5. **Comprehensive event modeling:** All trigger types (time, change, accept, call) represented in AST.

### In Scope

**Actions:**
- Control-flow nodes: initial, final, fork, join, merge, decision, action execution
- Edges: succession (`then`), control flow (guarded), object flow (data)
- Nested action invocation, inline expressions

**State Machines:**
- States: simple, composite (hierarchical), orthogonal (regions)
- Behaviors: entry, do, exit action sequences
- Transitions: source/target, triggers, guards, effects
- Pseudostates: choice, junction, fork, join, entry, exit
- Triggers: time events (`after`), change events (`when`), accept events (signals), call events (operations)

**Parser:**
- New `behavior.go` parser module
- Hooks in `parseUsageBody()` for action/state delegation
- Error recovery (partial AST on parse errors)

**Lexer:**
- 20 new keywords (behavioral syntax)
- `->` arrow token (transition syntax)

### Out of Scope

**Deferred to semantic passes:**
- Name resolution (node/state references → symbols)
- Graph validation (reachability, fork/join balance, initial node uniqueness)
- Type checking (guard boolean, effect action validity)

**Deferred to runtime (Tiers 4–5):**
- Token/occurrence semantics (object flow execution)
- Event queue implementation (signal dispatch, time scheduling)
- Execution semantics (Petri-net stepping, state machine event handling)

**Not addressed:**
- Textual concrete syntax design (assumes SysML v2 pilot syntax)
- IDE integration (syntax highlighting, autocomplete—LSP layer)

---

## 2. Background & Motivation

### Current State (Tiers 1–3 Complete)

The SysML v2 Go implementation has completed runtime Tiers 1–3 (commits `355e0f3` through `9058d10`, 2026-07-30):
- **Tier 1:** Feature flattening (`FeaturesOf` produces effective feature lists per type)
- **Tier 2:** Instance model (lazy slot materialization, `Instantiate` creates typed instances)
- **Tier 3:** Expression evaluator (literals, operators, feature access, calc invocation, KerML builtins)

Parser supports `action def`/`action` and `state def`/`state` **declarations** (`DefAction`/`UsageAction`, `DefState`/`UsageState` enum values exist), but their **bodies** are undifferentiated: `Usage.Members` holds arbitrary `[]Node` with no behavioral semantics.

### The Gap (from `runtime/AGENTS.md` §2.4)

> "NO action-node graph (fork/join/merge/decision/initial/final). NO succession/control edges, NO item/token flows. NO state transitions (trigger/guard/effect), NO entry/exit/do behaviors. `action`/`state` bodies are undifferentiated `Members []Node`. Behavioral simulation therefore REQUIRES new AST nodes + parser grammar FIRST, following the additive Tier-B pattern."

### Why Now

1. **Runtime foundation ready:** Tier 3 evaluator can handle guards/effects (expressions), but has no control-flow graph to step through.
2. **Blocked simulation:** Cannot implement action interpreter (Tier 4) or state machine interpreter (Tier 5) without AST representing control flow / state transitions.
3. **User demand:** SysML v2 specification emphasizes behavioral modeling as core capability—expression evaluation alone insufficient for systems engineering workflows (analysis cases, verification cases require simulating behavior).

### Design Philosophy

**Additive extension (Tier-B pattern):** Existing `Usage` struct extended with `FlowEnds` field for flow usages without breaking part/attribute usages. Similarly, behavioral nodes extend AST vocabulary without modifying core types. `Usage.Members` remains `[]Node`—behavioral nodes implement `Node` interface, coexist with expressions/nested usages.

**Separation of concerns:** Behavioral grammar complex (10+ node types, 4 edge types, 4 trigger types). Isolate in dedicated `behavior.go` file rather than polluting `defusage.go` or `expr.go`. Parser hooks delegate to specialized parsers (`parseActionBody`, `parseStateMachineBody`) when `Kind == UsageAction/UsageState`.

**Spec-first, pragmatic implementation:** Follow SysML v2 pilot textual syntax where defined; make pragmatic choices for ambiguities (e.g., keyword selection). Prioritize parse-ability over perfect spec alignment—semantic validation deferred to passes.

---

## 3. Design Principles

1. **Immutable AST:** Behavioral nodes follow existing pattern—constructed during parse, never mutated. Runtime execution state lives in side tables (per `runtime/AGENTS.md` §2.1).

2. **Explicit types over discriminants:** Prefer `InitialNode`, `ForkNode`, `JoinNode` (7 types) over `ActionNode{Kind: Initial/Fork/Join}` (1 type + enum). Rationale: Type safety catches errors at compile time; parser case statements clearer; follows existing pattern (`LiteralInteger` vs `LiteralReal`, not `Literal{Kind}`).

3. **Edges as first-class nodes:** `SuccessionEdge`, `TransitionEdge`, etc. are `Node` implementations in `Usage.Members`, not inline fields on source nodes. Rationale: Matches declarative SysML syntax (`a then b;` is a statement, not a property of `a`); consistent with `Relationship` pattern; preserves parse order.

4. **Interface for polymorphism:** `TriggerEvent` interface with 4 implementations (`TimeEvent`, `ChangeEvent`, `AcceptEvent`, `CallEvent`) enables type-safe trigger handling without reflection.

5. **Reuse expression AST:** Guards, effects, do-activities, time durations are `Node` (existing expression AST). No duplication—Tier 3 evaluator handles them.

6. **Fail gracefully:** Parser produces partial AST + `ErrorNode` on syntax errors. Semantic validation (unresolved references, graph structure) deferred to passes. Rationale: Enables IDE error recovery, progressive error reporting.

7. **Context-sensitive keywords:** `action` keyword means `DefAction` at top level, `ActionExecutionNode` inside action body. `state` keyword means `DefState` at top level, `StateNode` inside state machine body. Disambiguate via parser state.

8. **Lookahead for ambiguity:** `identifier then identifier` (succession edge) vs `identifier` (node reference) resolved by 1-token lookahead (`checkAhead(KeywordThen)`).

9. **No behavioral pollution of Usage:** Behavioral fields do NOT extend `Usage` struct (avoid bloat—already 20+ fields). Behavioral content lives in `Members []Node` as polymorphic nodes.

10. **Test-driven grammar:** Every syntactic construct has unit test in `parser/behavior_test.go`. Integration tests verify realistic models parse correctly.

---

## 4. Architecture Overview

### File Structure

**New files:**
- `internal/core/ast/behavior.go` — 19 behavioral AST types (nodes, edges, triggers)
- `internal/core/parser/behavior.go` — action/state body parsers
- `internal/core/parser/behavior_test.go` — unit tests for behavioral grammar

**Modified files:**
- `internal/core/lexer/lexer.go` — add 20 keywords to keyword map
- `internal/core/lexer/token.go` — add `TokenArrow` (->)
- `internal/core/parser/defusage.go` — hook `parseUsageBody()` to delegate action/state parsing

### Data Flow

```
Source text
    ↓
Lexer (extended keywords)
    ↓
Parser (defusage.go parseUsage)
    ↓
parseUsageBody() checks Kind
    ├─ UsageAction → parseActionBody() [behavior.go]
    ├─ UsageState → parseStateMachineBody() [behavior.go]
    └─ Other → existing logic
    ↓
Usage.Members populated with behavioral nodes
    ↓
Semantic passes validate structure
    ↓
Runtime Tier 4/5 interprets behavioral nodes
```

### AST Taxonomy

**19 new types (all implement `Node` via `NodeBase`):**

**Action nodes (7):**
1. `InitialNode` — entry point
2. `FinalNode` — termination
3. `ForkNode` — concurrent split
4. `JoinNode` — concurrent sync
5. `MergeNode` — alternative merge
6. `DecisionNode` — conditional branch
7. `ActionExecutionNode` — action invocation / inline expression

**State nodes (3):**
8. `StateNode` — state in state machine (entry/do/exit, substates, regions)
9. `StateRegion` — orthogonal region
10. `PseudostateNode` — choice/junction/fork/join/entry/exit (Kind discriminant)

**Edges (4):**
11. `SuccessionEdge` — sequential flow (`then`)
12. `ControlFlowEdge` — guarded flow (decision branches)
13. `ObjectFlowEdge` — data flow (parameters)
14. `TransitionEdge` — state transition (trigger/guard/effect)

**Triggers (4, implement `TriggerEvent` interface):**
15. `TimeEvent` — `after <duration>`
16. `ChangeEvent` — `when <condition>`
17. `AcceptEvent` — `accept <signal>`
18. `CallEvent` — `on <operation>`

**Container:**
19. `StateRegion` — orthogonal region container

### Parser Strategy

**Hook injection:** Minimal modification to `parser/defusage.go`:

```go
// In parseUsageBody() after consuming opening brace:
switch u.Kind {
case UsageAction:
    u.Members = p.parseActionBody()
    return
case UsageState:
    u.Members = p.parseStateMachineBody()
    return
default:
    // existing logic
}
```

**Specialized parsers:** New `parser/behavior.go` contains:
- `parseActionBody() []Node` — loops over action body keywords, returns behavioral nodes
- `parseStateMachineBody() []Node` — loops over state keywords, returns state/transition nodes
- Node-specific parsers: `parseForkNode()`, `parseStateNode()`, `parseTransitionEdge()`, etc.
- Trigger parsers: `parseTimeEvent()`, `parseChangeEvent()`, etc.

**Reuse existing:** Expression parsing (`parseExpression()`) reused for guards, effects, durations, conditions.

---

## 5. AST Structure — Action Nodes

**File:** `internal/core/ast/behavior.go`

All action nodes implement `Node` interface via embedded `NodeBase` (provides `Span()`, `LeadingTrivia()`, `TrailingTrivia()`).

### 5.1 InitialNode

Entry point for action execution. Every action body must have exactly one initial node (validated in semantic pass).

```go
type InitialNode struct {
    NodeBase
    Name string  // optional identifier for referencing in edges
}
```

**Syntax:** `first <name>;`  
**Example:** `first start;`

### 5.2 FinalNode

Termination point. Action completes successfully when execution reaches a final node.

```go
type FinalNode struct {
    NodeBase
    Name string
}
```

**Syntax:** `done <name>;`  
**Example:** `done end;`

### 5.3 ForkNode

Concurrent split (1 incoming → N outgoing parallel flows). Execution spawns concurrent threads for all outgoing edges.

```go
type ForkNode struct {
    NodeBase
    Name string
}
```

**Syntax:** `fork <name>;`  
**Example:** `fork parallel_split;`

### 5.4 JoinNode

Concurrent synchronization (N incoming → 1 outgoing). Execution blocks until all incoming flows complete.

```go
type JoinNode struct {
    NodeBase
    Name string
}
```

**Syntax:** `join <name>;`  
**Example:** `join sync_point;`

### 5.5 MergeNode

Alternative merge (N incoming → 1 outgoing). First incoming flow wins; others ignored.

```go
type MergeNode struct {
    NodeBase
    Name string
}
```

**Syntax:** `merge <name>;`  
**Example:** `merge continue;`

### 5.6 DecisionNode

Conditional branch (1 incoming → N outgoing guarded flows). Evaluation selects outgoing edge based on guard expressions.

```go
type DecisionNode struct {
    NodeBase
    Name string
}
```

**Syntax:** `decision <name>;`  
**Example:** `decision check_condition;`

**Note:** Outgoing edges from DecisionNode are `ControlFlowEdge` (not `SuccessionEdge`) — carry guard expressions.

### 5.7 ActionExecutionNode

Performs action work: invokes nested action, evaluates inline expression, or sends signal.

```go
type ActionExecutionNode struct {
    NodeBase
    Name       string
    ActionRef  *QualifiedName  // reference to nested action (usage or def)
    Expression Node             // inline expression (alternative to ActionRef)
}
```

**Dual modes:**
- **Action reference:** `ActionRef` points to another action, `Expression` is `nil`
- **Inline expression:** `Expression` holds expression AST, `ActionRef` is `nil`

**Syntax:**
- Reference: `action <name> : <ActionRef>;`
- Inline: `action <name> { <Expression> };`

**Examples:**
- `action step1 : processData;` (reference)
- `action step2 { x = x + 1; };` (inline)

### Design Notes

- **Name optionality:** All nodes have optional `Name` field. If omitted in source, parser generates synthetic name (`fork_1`, `join_2`, etc.) for edge referencing.
- **Spannable:** `NodeBase` embedding ensures all nodes carry source location for diagnostics.
- **No successor fields:** Nodes do NOT store outgoing edges inline. Edges are separate `SuccessionEdge`/`ControlFlowEdge` nodes in `Usage.Members`. Rationale: matches declarative syntax, preserves parse order, avoids graph mutation.

---

## 6. AST Structure — State Machine Nodes

### 6.1 StateNode

Represents a state in a state machine. Supports simple, composite (hierarchical), and orthogonal (parallel regions) states.

```go
type StateNode struct {
    NodeBase
    Name         string
    IsInitial    bool              // initial state marker (one per region)
    IsFinal      bool              // final state marker (terminal)
    Entry        []Node            // entry behaviors (action sequence)
    Do           []Node            // do activity (ongoing action)
    Exit         []Node            // exit behaviors (action sequence)
    Substates    []Node            // nested states (hierarchical composition)
    Regions      []*StateRegion    // orthogonal regions (parallel composition)
}
```

**Fields:**
- **Name:** State identifier (required).
- **IsInitial/IsFinal:** Boolean flags set by `initial state` or `final state` keywords. Alternative to separate `InitialStateNode`/`FinalStateNode` types (simpler taxonomy).
- **Entry/Do/Exit:** Action sequences executed on state entry, during state occupancy, and on state exit. Each is `[]Node` (can be single expression or action block).
- **Substates:** Nested `StateNode` instances for hierarchical state machines.
- **Regions:** Orthogonal regions (`*StateRegion`) for concurrent substates.

**Syntax:**
```
state <name> {
    entry { <actions> }
    do { <actions> }
    exit { <actions> }
    state <substate> { ... }
}

initial state <name>;
final state <name>;
```

**Example:**
```
state Active {
    entry { log("Entering active"); }
    do { monitor(); }
    exit { cleanup(); }
    
    state Busy { ... }
    state Idle { ... }
}

initial state Init;
final state Done;
```

### 6.2 StateRegion

Orthogonal region within a composite state. Enables parallel state machines.

```go
type StateRegion struct {
    NodeBase
    Name   string
    States []Node  // states in this region (StateNode instances)
}
```

**Syntax:** Implicit in composite state body (regions inferred from multiple independent state hierarchies).  
**Note:** Region detection is parser heuristic—if composite state has multiple top-level substates with no transitions between them, treat as regions.

**Example:**
```
state Composite {
    region R1 {
        state A;
        state B;
        transition A -> B;
    }
    region R2 {
        state X;
        state Y;
        transition X -> Y;
    }
}
```

### 6.3 PseudostateNode

Transient state used for control flow (choice, junction) and structural composition (fork, join, entry, exit points).

```go
type PseudostateKind int

const (
    PseudostateChoice PseudostateKind = iota  // conditional branch
    PseudostateJunction                       // merge point (no guards)
    PseudostateFork                           // parallel split
    PseudostateJoin                           // parallel sync
    PseudostateEntry                          // entry point (submachine)
    PseudostateExit                           // exit point (submachine)
)

type PseudostateNode struct {
    NodeBase
    Kind PseudostateKind
    Name string
}
```

**Discriminated by Kind:** Single type for 6 pseudostate semantics. Rationale: All share similar runtime behavior (transient, no entry/exit), unified type simplifies traversal.

**Syntax:**
```
choice <name>;
junction <name>;
fork <name>;
join <name>;
entry <name>;
exit <name>;
```

**Semantics:**
- **Choice:** Multiple outgoing transitions with guards (evaluated at runtime).
- **Junction:** Multiple outgoing transitions, first valid wins (static).
- **Fork:** Concurrent split (enters multiple substates).
- **Join:** Concurrent sync (waits for multiple substates).
- **Entry/Exit:** Submachine interface points (deferred to Tier 5 advanced features).

### Design Notes

- **IsInitial/IsFinal flags vs. separate types:** Flags chosen for simplicity—state semantics identical except for marker. Alternative: `InitialStateNode`, `FinalStateNode` types (more verbose, same runtime behavior).
- **Entry/Do/Exit as []Node:** Reuses expression AST. Single action = 1-element slice; block = N-element slice. Parser handles both `entry { x; }` and `entry x;`.
- **Regions:** Orthogonal states are advanced feature—syntax TBD (spec ambiguous). Initial implementation may parse regions explicitly or infer from independent substate hierarchies.
- **Pseudostates unified:** 6 kinds share type. If semantics diverge in Tier 5, refactor to explicit types.

---

## 7. AST Structure — Edges & Triggers

All edges are first-class `Node` implementations added to `Usage.Members`. Resolver maps edge source/target names to node symbols during semantic pass.

### 7.1 SuccessionEdge

Sequential control flow in actions. Source node completes → target node starts.

```go
type SuccessionEdge struct {
    NodeBase
    Source *QualifiedName  // source action node
    Target *QualifiedName  // target action node
}
```

**Syntax:** `<source> then <target>;`  
**Example:** `start then step1;`

**Semantics:** Unconditional flow. When source node finishes, target node becomes active.

### 7.2 ControlFlowEdge

Guarded control flow from decision nodes. Guard expression selects branch.

```go
type ControlFlowEdge struct {
    NodeBase
    Source *QualifiedName  // source node (typically DecisionNode)
    Target *QualifiedName  // target node
    Guard  Node            // boolean guard expression
}
```

**Syntax:** `<source> then <target> if <guard>;`  
**Example:** `check then yes_branch if (x > 0);`

**Semantics:** Conditional flow. Guard evaluated when source node completes; if true, target node activates.

### 7.3 ObjectFlowEdge

Data flow between action parameters / pins.

```go
type ObjectFlowEdge struct {
    NodeBase
    Source *QualifiedName  // source pin/parameter
    Target *QualifiedName  // target pin/parameter
}
```

**Syntax:** TBD (deferred to Tier 5 — requires parameter/pin grammar).  
**Placeholder:** `<source> flows to <target>;`

**Semantics:** Token flow. When source produces value, value transferred to target.

### 7.4 TransitionEdge

State machine transition. Fires when trigger occurs, guard satisfied.

```go
type TransitionEdge struct {
    NodeBase
    Source  *QualifiedName  // source state
    Target  *QualifiedName  // target state
    Trigger TriggerEvent    // event that fires transition (interface)
    Guard   Node            // optional guard expression
    Effect  []Node          // optional effect actions
}
```

**Syntax:** `transition <source> -> <target> [<trigger>] [if <guard>] [do <effect>];`

**Examples:**
```
transition Idle -> Active after 5s;
transition Active -> Idle when (done == true) do { cleanup(); };
transition Off -> On accept PowerSignal if (enabled);
```

**Semantics:**
- **Source state:** Current state (must be active for transition to fire).
- **Target state:** Next state after transition.
- **Trigger:** Event that enables transition (time/change/accept/call).
- **Guard:** Boolean condition (optional; transition fires only if true).
- **Effect:** Actions executed during transition (optional).

---

## 7.5 Trigger Events

All triggers implement `TriggerEvent` interface for polymorphic handling.

```go
// TriggerEvent is marker interface for state transition triggers
type TriggerEvent interface {
    Node
    triggerEvent()  // unexported marker method
}
```

### 7.5.1 TimeEvent

Fires after specified duration.

```go
type TimeEvent struct {
    NodeBase
    Duration Node  // time expression (e.g., literal "5s" or variable)
}

func (*TimeEvent) triggerEvent() {}
```

**Syntax:** `after <duration>`  
**Examples:**
- `after 5s` (literal)
- `after timeout` (variable reference)

**Semantics:** Transition fires when `Duration` time elapses since entering source state.

### 7.5.2 ChangeEvent

Fires when condition becomes true.

```go
type ChangeEvent struct {
    NodeBase
    Condition Node  // boolean expression
}

func (*ChangeEvent) triggerEvent() {}
```

**Syntax:** `when <condition>`  
**Example:** `when (temperature > 100)`

**Semantics:** Runtime continuously evaluates `Condition`; transition fires on false→true edge.

### 7.5.3 AcceptEvent

Fires when signal received.

```go
type AcceptEvent struct {
    NodeBase
    SignalType *QualifiedName  // signal type to accept
}

func (*AcceptEvent) triggerEvent() {}
```

**Syntax:** `accept <SignalType>`  
**Example:** `accept StartSignal`

**Semantics:** Transition fires when signal instance of `SignalType` dispatched to state machine. Requires event queue in runtime (Tier 5).

### 7.5.4 CallEvent

Fires when operation invoked.

```go
type CallEvent struct {
    NodeBase
    Operation *QualifiedName  // operation to invoke
}

func (*CallEvent) triggerEvent() {}
```

**Syntax:** `on <operation>`  
**Example:** `on start()`

**Semantics:** Transition fires when `Operation` called on state machine. Requires operation dispatch infrastructure (Tier 5).

---

## 7.6 Design Notes

### Edge Representation

**Why first-class nodes?**
- Matches declarative syntax (`then`, `transition` are statements, not node properties).
- Preserves parse order in `Usage.Members`.
- Consistent with `Relationship` pattern (edges as separate entities).
- Avoids graph mutation (nodes immutable, edges added to list during parse).

**Name resolution deferred:** Edges store `*QualifiedName` for source/target. Semantic pass resolves names → node symbols, builds executable graph.

### Trigger Interface

**Why interface + 4 types?**
- Type-safe discrimination (no `switch Kind` on enum).
- Extensible (future trigger types add new implementations).
- Follows Go idiom (small interfaces, explicit implementations).

**Marker method `triggerEvent()`:** Unexported, prevents external types implementing interface (closed set).

### Expression Reuse

Guards (`ControlFlowEdge.Guard`, `TransitionEdge.Guard`), effects (`TransitionEdge.Effect`), conditions (`ChangeEvent.Condition`), durations (`TimeEvent.Duration`) are `Node` — reuse existing expression AST. Tier 3 evaluator handles them without new code.

---

## 8. Parser Integration — Action Bodies

### 8.1 Hook Point

**File:** `internal/core/parser/defusage.go`  
**Function:** `parseUsageBody()`

After consuming opening brace, check `u.Kind`:

```go
func (p *Parser) parseUsageBody(u *Usage) {
    if !p.match(TokenLBrace) {
        return // no body
    }
    
    // Hook for behavioral bodies
    switch u.Kind {
    case UsageAction:
        u.Members = p.parseActionBody()
        return
    case UsageState:
        u.Members = p.parseStateMachineBody()
        return
    }
    
    // Existing logic for other usages
    for !p.match(TokenRBrace) && !p.atEnd() {
        u.Members = append(u.Members, p.parseMember())
    }
}
```

**Minimal modification:** 5-line switch statement. No changes to existing `parseMember()` logic.

---

### 8.2 Action Body Parser

**File:** `internal/core/parser/behavior.go` (new)

```go
// parseActionBody parses action body: nodes + edges
func (p *Parser) parseActionBody() []Node {
    members := []Node{}
    
    for !p.match(TokenRBrace) && !p.atEnd() {
        switch {
        case p.match(KeywordFirst):
            members = append(members, p.parseInitialNode())
        case p.match(KeywordAction):
            members = append(members, p.parseActionExecutionNode())
        case p.match(KeywordFork):
            members = append(members, p.parseForkNode())
        case p.match(KeywordJoin):
            members = append(members, p.parseJoinNode())
        case p.match(KeywordMerge):
            members = append(members, p.parseMergeNode())
        case p.match(KeywordDecision):
            members = append(members, p.parseDecisionNode())
        case p.match(KeywordDone):
            members = append(members, p.parseFinalNode())
        case p.check(TokenIdentifier) && p.checkAhead(KeywordThen):
            members = append(members, p.parseSuccessionOrControlFlow())
        default:
            // Fallback: nested usage, expression, or error
            members = append(members, p.parseExpression())
        }
        p.matchSemicolon()  // optional semicolons
    }
    
    return members
}
```

**Keyword dispatch:** Each keyword routes to specialized parser. Lookahead (`checkAhead`) disambiguates edges from expressions.

---

### 8.3 Node Parsers

#### InitialNode

```go
func (p *Parser) parseInitialNode() *InitialNode {
    start := p.current.Span
    p.advance()  // consume 'first'
    
    name := ""
    if p.check(TokenIdentifier) {
        name = p.advance().Lexeme
    }
    
    return &InitialNode{
        NodeBase: NodeBase{SpanVal: start.To(p.previous.Span)},
        Name:     name,
    }
}
```

**Pattern:** Consume keyword, optionally consume name, return node with span.

#### ForkNode / JoinNode / MergeNode / DecisionNode

Identical pattern to `InitialNode` — consume keyword, optional name.

#### FinalNode

```go
func (p *Parser) parseFinalNode() *FinalNode {
    start := p.current.Span
    p.advance()  // consume 'done'
    
    name := ""
    if p.check(TokenIdentifier) {
        name = p.advance().Lexeme
    }
    
    return &FinalNode{
        NodeBase: NodeBase{SpanVal: start.To(p.previous.Span)},
        Name:     name,
    }
}
```

#### ActionExecutionNode

```go
func (p *Parser) parseActionExecutionNode() *ActionExecutionNode {
    start := p.current.Span
    p.advance()  // consume 'action'
    
    name := ""
    if p.check(TokenIdentifier) {
        name = p.advance().Lexeme
    }
    
    node := &ActionExecutionNode{
        NodeBase: NodeBase{SpanVal: start},
        Name:     name,
    }
    
    // Check for action reference (: <name>) or inline expression ({ ... })
    if p.match(TokenColon) {
        node.ActionRef = p.parseQualifiedName()
    } else if p.match(TokenLBrace) {
        node.Expression = p.parseExpression()
        p.consume(TokenRBrace, "expected '}' after action expression")
    }
    
    node.NodeBase.SpanVal = start.To(p.previous.Span)
    return node
}
```

**Dual modes:** Colon (`:`) signals reference, brace (`{`) signals inline expression.

---

### 8.4 Edge Parsers

#### SuccessionEdge / ControlFlowEdge

```go
func (p *Parser) parseSuccessionOrControlFlow() Node {
    start := p.current.Span
    source := p.parseQualifiedName()
    p.consume(KeywordThen, "expected 'then' in succession")
    target := p.parseQualifiedName()
    
    // Check for guard (if present, ControlFlowEdge)
    if p.match(KeywordIf) {
        guard := p.parseExpression()
        return &ControlFlowEdge{
            NodeBase: NodeBase{SpanVal: start.To(p.previous.Span)},
            Source:   source,
            Target:   target,
            Guard:    guard,
        }
    }
    
    // No guard → SuccessionEdge
    return &SuccessionEdge{
        NodeBase: NodeBase{SpanVal: start.To(p.previous.Span)},
        Source:   source,
        Target:   target,
    }
}
```

**Unified parser:** `then` with guard → `ControlFlowEdge`, without guard → `SuccessionEdge`.

#### ObjectFlowEdge

Deferred to Tier 5 (requires parameter/pin grammar). Placeholder parser:

```go
func (p *Parser) parseObjectFlowEdge() *ObjectFlowEdge {
    // TBD: syntax for data flow
    return nil
}
```

---

### 8.5 Error Recovery

**Strategy:** On parse error, insert `ErrorNode`, skip to next statement (semicolon or keyword).

```go
// In parseActionBody() default case:
default:
    p.error("unexpected token in action body")
    members = append(members, &ErrorNode{NodeBase: NodeBase{SpanVal: p.current.Span}})
    p.synchronize()  // skip to semicolon or keyword
}

func (p *Parser) synchronize() {
    for !p.atEnd() {
        if p.previous.Type == TokenSemicolon {
            return
        }
        switch p.current.Type {
        case KeywordFirst, KeywordFork, KeywordJoin, KeywordAction, KeywordDone:
            return
        }
        p.advance()
    }
}
```

**Goal:** Produce partial AST + diagnostic, continue parsing next statement.

---

## 9. Parser Integration — State Machine Bodies

### 9.1 State Machine Body Parser

**File:** `internal/core/parser/behavior.go`

```go
// parseStateMachineBody parses state machine: states + transitions
func (p *Parser) parseStateMachineBody() []Node {
    members := []Node{}
    
    for !p.match(TokenRBrace) && !p.atEnd() {
        switch {
        case p.match(KeywordInitial) && p.check(KeywordState):
            members = append(members, p.parseInitialState())
        case p.match(KeywordFinal) && p.check(KeywordState):
            members = append(members, p.parseFinalState())
        case p.match(KeywordState):
            members = append(members, p.parseStateNode())
        case p.match(KeywordChoice):
            members = append(members, p.parsePseudostate(PseudostateChoice))
        case p.match(KeywordJunction):
            members = append(members, p.parsePseudostate(PseudostateJunction))
        case p.match(KeywordTransition):
            members = append(members, p.parseTransitionEdge())
        default:
            members = append(members, p.parseExpression())
        }
        p.matchSemicolon()
    }
    
    return members
}
```

**Two-keyword lookahead:** `initial state` and `final state` require consuming two keywords before dispatching.

---

### 9.2 State Node Parsers

#### Simple StateNode

```go
func (p *Parser) parseStateNode() *StateNode {
    start := p.current.Span
    p.advance()  // consume 'state'
    
    name := p.consume(TokenIdentifier, "expected state name").Lexeme
    
    state := &StateNode{
        NodeBase: NodeBase{SpanVal: start},
        Name:     name,
    }
    
    if !p.match(TokenLBrace) {
        // Simple state (no body)
        state.NodeBase.SpanVal = start.To(p.previous.Span)
        return state
    }
    
    // Parse state body
    state = p.parseStateBody(state)
    p.consume(TokenRBrace, "expected '}' after state body")
    state.NodeBase.SpanVal = start.To(p.previous.Span)
    return state
}
```

**Two forms:** `state X;` (simple) vs `state X { ... }` (composite).

#### State Body Parser

```go
func (p *Parser) parseStateBody(state *StateNode) *StateNode {
    for !p.check(TokenRBrace) && !p.atEnd() {
        switch {
        case p.match(KeywordEntry):
            state.Entry = p.parseStateBehavior()
        case p.match(KeywordDo):
            state.Do = p.parseStateBehavior()
        case p.match(KeywordExit):
            state.Exit = p.parseStateBehavior()
        case p.match(KeywordState):
            state.Substates = append(state.Substates, p.parseStateNode())
        case p.match(KeywordRegion):
            state.Regions = append(state.Regions, p.parseStateRegion())
        default:
            p.advance()  // skip unknown token
        }
    }
    return state
}
```

**Keywords:** `entry`, `do`, `exit` for behaviors; `state` for substates; `region` for orthogonal regions.

#### State Behavior Parser

```go
func (p *Parser) parseStateBehavior() []Node {
    actions := []Node{}
    
    if !p.match(TokenLBrace) {
        // Single action/expression
        actions = append(actions, p.parseExpression())
        return actions
    }
    
    // Action block
    for !p.match(TokenRBrace) && !p.atEnd() {
        actions = append(actions, p.parseExpression())
        p.matchSemicolon()
    }
    
    return actions
}
```

**Two forms:** `entry <expr>;` (single) vs `entry { <expr1>; <expr2>; }` (block).

#### Initial / Final State

```go
func (p *Parser) parseInitialState() *StateNode {
    start := p.current.Span
    p.advance()  // 'initial' already consumed
    p.consume(KeywordState, "expected 'state' after 'initial'")
    
    name := p.consume(TokenIdentifier, "expected state name").Lexeme
    
    return &StateNode{
        NodeBase:  NodeBase{SpanVal: start.To(p.previous.Span)},
        Name:      name,
        IsInitial: true,
    }
}

func (p *Parser) parseFinalState() *StateNode {
    start := p.current.Span
    p.advance()  // 'final' already consumed
    p.consume(KeywordState, "expected 'state' after 'final'")
    
    name := p.consume(TokenIdentifier, "expected state name").Lexeme
    
    return &StateNode{
        NodeBase: NodeBase{SpanVal: start.To(p.previous.Span)},
        Name:     name,
        IsFinal:  true,
    }
}
```

**Flags set:** `IsInitial` / `IsFinal` distinguish from regular states.

#### StateRegion Parser

```go
func (p *Parser) parseStateRegion() *StateRegion {
    start := p.current.Span
    p.advance()  // consume 'region'
    
    name := ""
    if p.check(TokenIdentifier) {
        name = p.advance().Lexeme
    }
    
    region := &StateRegion{
        NodeBase: NodeBase{SpanVal: start},
        Name:     name,
    }
    
    if !p.match(TokenLBrace) {
        return region  // empty region
    }
    
    // Parse region states
    for !p.match(TokenRBrace) && !p.atEnd() {
        if p.match(KeywordState) {
            region.States = append(region.States, p.parseStateNode())
        } else {
            p.advance()  // skip
        }
    }
    
    region.NodeBase.SpanVal = start.To(p.previous.Span)
    return region
}
```

---

### 9.3 Pseudostate Parser

```go
func (p *Parser) parsePseudostate(kind PseudostateKind) *PseudostateNode {
    start := p.current.Span
    p.advance()  // keyword already consumed
    
    name := ""
    if p.check(TokenIdentifier) {
        name = p.advance().Lexeme
    }
    
    return &PseudostateNode{
        NodeBase: NodeBase{SpanVal: start.To(p.previous.Span)},
        Kind:     kind,
        Name:     name,
    }
}
```

**Unified parser:** Kind passed as parameter (called from switch in `parseStateMachineBody()`).

---

### 9.4 Transition Edge Parser

```go
func (p *Parser) parseTransitionEdge() *TransitionEdge {
    start := p.current.Span
    p.advance()  // consume 'transition'
    
    source := p.parseQualifiedName()
    p.consume(TokenArrow, "expected '->' in transition")
    target := p.parseQualifiedName()
    
    trans := &TransitionEdge{
        NodeBase: NodeBase{SpanVal: start},
        Source:   source,
        Target:   target,
    }
    
    // Optional trigger
    if p.match(KeywordAfter) {
        trans.Trigger = p.parseTimeEvent()
    } else if p.match(KeywordWhen) {
        trans.Trigger = p.parseChangeEvent()
    } else if p.match(KeywordAccept) {
        trans.Trigger = p.parseAcceptEvent()
    } else if p.match(KeywordOn) {
        trans.Trigger = p.parseCallEvent()
    }
    
    // Optional guard
    if p.match(KeywordIf) {
        trans.Guard = p.parseExpression()
    }
    
    // Optional effect
    if p.match(KeywordDo) {
        trans.Effect = p.parseStateBehavior()
    }
    
    trans.NodeBase.SpanVal = start.To(p.previous.Span)
    return trans
}
```

**Syntax order:** `transition source -> target [trigger] [if guard] [do effect];`

---

### 9.5 Trigger Event Parsers

#### TimeEvent

```go
func (p *Parser) parseTimeEvent() *TimeEvent {
    start := p.current.Span
    duration := p.parseExpression()  // e.g., "5s" literal or variable
    return &TimeEvent{
        NodeBase: NodeBase{SpanVal: start.To(p.previous.Span)},
        Duration: duration,
    }
}
```

**Example:** `after 5s` → `Duration` is `LiteralInteger{Text: "5s"}` (or `LiteralReal` if `5.0s`).

#### ChangeEvent

```go
func (p *Parser) parseChangeEvent() *ChangeEvent {
    start := p.current.Span
    condition := p.parseExpression()
    return &ChangeEvent{
        NodeBase:  NodeBase{SpanVal: start.To(p.previous.Span)},
        Condition: condition,
    }
}
```

**Example:** `when (x > 10)` → `Condition` is binary operator expression.

#### AcceptEvent

```go
func (p *Parser) parseAcceptEvent() *AcceptEvent {
    start := p.current.Span
    signalType := p.parseQualifiedName()
    return &AcceptEvent{
        NodeBase:   NodeBase{SpanVal: start.To(p.previous.Span)},
        SignalType: signalType,
    }
}
```

**Example:** `accept StartSignal` → `SignalType` resolves to signal definition.

#### CallEvent

```go
func (p *Parser) parseCallEvent() *CallEvent {
    start := p.current.Span
    operation := p.parseQualifiedName()
    return &CallEvent{
        NodeBase:  NodeBase{SpanVal: start.To(p.previous.Span)},
        Operation: operation,
    }
}
```

**Example:** `on start()` → `Operation` resolves to operation definition.

---

## 10. Lexer Extensions

### 10.1 New Keywords

**File:** `internal/core/lexer/lexer.go`  
**Modification:** Add to `keywords` map in `func init()`.

**Action keywords (7):**
```go
"first":    TokenKeyword,  // InitialNode
"done":     TokenKeyword,  // FinalNode
"fork":     TokenKeyword,  // ForkNode
"join":     TokenKeyword,  // JoinNode
"merge":    TokenKeyword,  // MergeNode
"decision": TokenKeyword,  // DecisionNode
"then":     TokenKeyword,  // SuccessionEdge
```

**State keywords (6):**
```go
"state":      TokenKeyword,  // StateNode
"initial":    TokenKeyword,  // IsInitial flag
"final":      TokenKeyword,  // IsFinal flag
"entry":      TokenKeyword,  // entry behavior
"exit":       TokenKeyword,  // exit behavior
"transition": TokenKeyword,  // TransitionEdge
```

**Trigger keywords (4):**
```go
"after":  TokenKeyword,  // TimeEvent
"when":   TokenKeyword,  // ChangeEvent
"accept": TokenKeyword,  // AcceptEvent
"on":     TokenKeyword,  // CallEvent
```

**Pseudostate keywords (2):**
```go
"choice":   TokenKeyword,  // PseudostateChoice
"junction": TokenKeyword,  // PseudostateJunction
```

**Reused keywords (already in lexer):**
- `if` — guards (ControlFlowEdge.Guard, TransitionEdge.Guard)
- `do` — effects / do-activity (TransitionEdge.Effect, StateNode.Do)
- `action` — action definition/usage (already `DefAction`/`UsageAction`)
- `region` — orthogonal regions (StateRegion)

**Total new keywords:** 20 (19 above + 1 for region if not already present).

---

### 10.2 New Tokens

**File:** `internal/core/lexer/token.go`

```go
const (
    // ... existing tokens
    TokenArrow  // "->" (transition arrow)
)
```

**Token addition:** `->` operator for `transition source -> target` syntax.

---

### 10.3 Lexer Scanning Logic

**File:** `internal/core/lexer/lexer.go`  
**Function:** `scanToken()`

Add arrow (`->`) scanning:

```go
func (l *Lexer) scanToken() {
    // ... existing switch
    switch l.ch {
    // ... existing cases
    case '-':
        if l.match('>') {
            l.addToken(TokenArrow)
        } else {
            l.addToken(TokenMinus)
        }
    // ... rest
    }
}
```

**Logic:** Consume `-`, check next char: if `>`, emit `TokenArrow`; else emit `TokenMinus`.

---

### 10.4 Keyword Contextuality

**All keywords emit `TokenKeyword`**. Parser interprets meaning based on context:

- `action` at top-level → `DefAction` or `UsageAction` (existing logic in `parseDefinition` / `parseUsage`)
- `action` inside action body → `ActionExecutionNode` (new logic in `parseActionBody`)
- `state` at top-level → `DefState` or `UsageState`
- `state` inside state machine body → `StateNode`

**No lexer changes needed for context sensitivity**—lexer emits generic `TokenKeyword`, parser disambiguates.

---

### 10.5 Reserved Keyword Conflicts

**Check:** Do new keywords conflict with existing identifiers in pilot models?

**Analysis:**
- `first`, `done`, `fork`, `join`, `merge`, `decision`, `then` — unlikely conflicts (not common variable names in SysML).
- `state`, `transition`, `entry`, `exit` — potential conflicts if used as attribute/part names.
- `after`, `when`, `accept`, `on` — common English words, higher conflict risk.

**Mitigation:**
1. **Pilot corpus scan:** Grep SysML-v2-Pilot-Implementation for identifiers matching new keywords (e.g., `grep -r "\bstate\b" sysml.library/`).
2. **Escape hatch:** Allow backtick-quoted identifiers (`\`state\``) to bypass keyword reservation (deferred to implementation).
3. **Scoped keywords:** Reserve keywords only in behavioral contexts (inside `action {` or `state {` bodies). Requires lexer mode switching—complex, defer to future if conflicts arise.

**Decision:** Accept keyword reservation, document conflicts in migration guide. Pilot models rarely use behavioral keywords as identifiers (SysML v2 spec reserves them).

---

## 11. Error Handling & Validation

### 11.1 Parser Error Recovery

**Goal:** Produce partial AST + diagnostics on syntax errors. Do not abort parsing—continue to next statement.

#### Strategy 1: Missing Names

**Input:** `fork;` (no name)  
**Recovery:** Generate synthetic name (`fork_1`, `fork_2`, ...), continue.

```go
name := ""
if p.check(TokenIdentifier) {
    name = p.advance().Lexeme
} else {
    name = p.generateSyntheticName("fork")  // "fork_1"
    p.error("expected identifier after 'fork'")
}
```

#### Strategy 2: Dangling Edges

**Input:** `a then b;` but `a` never declared  
**Recovery:** Accept edge, defer validation to semantic pass.

**Rationale:** Name resolution is semantic concern, not syntactic. Parser produces AST with `*QualifiedName` references; semantic pass reports "unresolved reference" if name lookup fails.

#### Strategy 3: Malformed Transitions

**Input:** `transition state1 state2;` (no arrow)  
**Recovery:** Insert `ErrorNode`, report diagnostic, skip to semicolon.

```go
if !p.match(TokenArrow) {
    p.error("expected '->' in transition")
    return &ErrorNode{NodeBase: NodeBase{SpanVal: p.current.Span}}
}
```

#### Strategy 4: Invalid Trigger Syntax

**Input:** `transition a -> b after;` (no duration)  
**Recovery:** Create `TimeEvent` with `nil` Duration, report error.

```go
if !p.check(TokenIdentifier) && !p.check(TokenInteger) {
    p.error("expected duration after 'after'")
    return &TimeEvent{NodeBase: NodeBase{SpanVal: start}, Duration: nil}
}
```

#### Strategy 5: Unmatched Braces

**Input:** `state x { entry { doSomething();` (missing 2 closing braces)  
**Recovery:** Synchronize at next `state` keyword or EOF.

```go
func (p *Parser) synchronize() {
    for !p.atEnd() {
        if p.previous.Type == TokenSemicolon {
            return
        }
        if p.check(KeywordState) || p.check(KeywordTransition) {
            return
        }
        p.advance()
    }
}
```

**Call synchronize()** in error branches to skip malformed content.

---

### 11.2 Semantic Validation (Post-Parse)

**Location:** `internal/core/passes/` (new pass: `behavioral.go`)

#### Validation 1: Name Resolution

**Check:** `SuccessionEdge.Source/Target`, `TransitionEdge.Source/Target` resolve to declared nodes/states.

**Implementation:**
```go
func (p *BehavioralPass) validateEdges(scope *symbols.Scope, members []ast.Node) {
    for _, member := range members {
        if edge, ok := member.(*ast.SuccessionEdge); ok {
            if _, ok := p.resolveNode(scope, edge.Source); !ok {
                p.reportError(edge.Source.Span(), "unresolved node reference")
            }
            if _, ok := p.resolveNode(scope, edge.Target); !ok {
                p.reportError(edge.Target.Span(), "unresolved node reference")
            }
        }
        // Similar for TransitionEdge, ControlFlowEdge
    }
}
```

#### Validation 2: Graph Structure

**Check:** Action control-flow graph well-formed.

**Rules:**
- **Initial node uniqueness:** Exactly 1 `InitialNode` per action (error if 0 or >1).
- **Reachability:** All nodes reachable from initial node (warn if unreachable).
- **Fork/join balance:** Every fork has matching join downstream (warn if imbalanced).
- **Final node termination:** At least one path from initial → final (warn if no termination).

**Implementation:** Deferred to Tier 4 runtime (requires graph traversal algorithms). Semantic pass validates only syntactic constraints.

#### Validation 3: State Machine Structure

**Check:** State machine well-formed.

**Rules:**
- **Initial state uniqueness:** Each region has exactly 1 `IsInitial=true` state (error if 0 or >1).
- **Transition validity:** Source/target states exist in same region (error if cross-region without explicit connection).
- **Trigger consistency:** `AcceptEvent.SignalType` resolves to signal definition, `CallEvent.Operation` resolves to operation.

#### Validation 4: Type Checking

**Check:** Expression types correct.

**Rules:**
- **Guard boolean:** `ControlFlowEdge.Guard`, `TransitionEdge.Guard` must evaluate to boolean (error if int/string).
- **Effect action validity:** `TransitionEdge.Effect` contains valid action expressions (no type mismatches).
- **Duration numeric:** `TimeEvent.Duration` must be numeric (int/real).

**Implementation:** Reuse existing `TypeCheckPass` infrastructure (extend to behavioral nodes).

---

### 11.3 Ambiguity Resolution

#### Ambiguity 1: `action` Keyword Overloading

**Context:** Top-level vs inside action body.

**Resolution:**
```go
// In parseDefinition() or parseUsage():
if keyword == KeywordAction && !inActionBody {
    return parseActionDefinition()
}

// In parseActionBody():
if keyword == KeywordAction {
    return parseActionExecutionNode()
}
```

**Parser state:** Track `inActionBody` flag or check call stack.

#### Ambiguity 2: `state` Keyword Overloading

**Context:** Top-level vs inside state machine body.

**Resolution:** Same as `action` — context-sensitive parsing.

#### Ambiguity 3: Edge vs Node Detection

**Input:** `identifier then identifier` (edge) vs `identifier` (node reference)

**Resolution:** Lookahead.

```go
if p.check(TokenIdentifier) && p.checkAhead(KeywordThen) {
    return parseSuccessionEdge()
} else if p.check(TokenIdentifier) {
    return parseExpression()  // node reference or other
}
```

**Lookahead:** `checkAhead(kind)` peeks 1 token ahead without consuming.

---

### 11.4 Diagnostic Quality

**Goal:** Error messages guide user to fix.

**Examples:**
- `"expected '->' in transition at line 42"` (specific token expected)
- `"unresolved node reference 'step1' at line 45"` (semantic error with name)
- `"guard expression must be boolean, got Integer"` (type error with expected/actual)

**Implementation:** Include span (`source.Span`) in all diagnostics for IDE integration (underline error location).

---

## 12. Testing Strategy

### 12.1 Unit Tests — AST Construction

**File:** `internal/core/ast/behavior_test.go`

**Tests:**
1. **Node construction** — verify all 19 types construct correctly, embed `NodeBase`, implement `Node` interface.
2. **Span propagation** — verify `Span()` returns correct source range.
3. **TriggerEvent interface** — verify 4 trigger types implement interface, marker method prevents external implementations.

**Example:**
```go
func TestActionNodeConstruction(t *testing.T) {
    node := &InitialNode{
        NodeBase: NodeBase{SpanVal: source.Span{Start: 0, End: 10}},
        Name:     "start",
    }
    if node.Span().Start != 0 || node.Span().End != 10 {
        t.Errorf("span mismatch")
    }
}

func TestTriggerEventInterface(t *testing.T) {
    var _ TriggerEvent = (*TimeEvent)(nil)
    var _ TriggerEvent = (*ChangeEvent)(nil)
    var _ TriggerEvent = (*AcceptEvent)(nil)
    var _ TriggerEvent = (*CallEvent)(nil)
}
```

---

### 12.2 Parser Tests — Action Bodies

**File:** `internal/core/parser/behavior_test.go`

#### Test 1: Simple Action

```go
func TestParseActionBody_Simple(t *testing.T) {
    src := `
        action process {
            first start;
            action step1;
            start then step1;
            step1 then done;
            done end;
        }
    `
    
    root := parseSource(t, src)
    usage := findUsage(root, "process")
    
    // Verify members: 1 InitialNode, 1 ActionExecutionNode, 1 FinalNode, 2 SuccessionEdges
    if len(usage.Members) != 5 {
        t.Errorf("expected 5 members, got %d", len(usage.Members))
    }
    
    // Type assertions
    assertType(t, usage.Members[0], (*InitialNode)(nil))
    assertType(t, usage.Members[1], (*ActionExecutionNode)(nil))
    // ...
}
```

#### Test 2: Fork/Join

```go
func TestParseActionBody_ForkJoin(t *testing.T) {
    src := `
        action parallel {
            first start;
            fork split;
            action task1;
            action task2;
            join sync;
            done end;
            
            start then split;
            split then task1;
            split then task2;
            task1 then sync;
            task2 then sync;
            sync then end;
        }
    `
    
    root := parseSource(t, src)
    usage := findUsage(root, "parallel")
    
    // Verify ForkNode, JoinNode, 6 edges
    assertNodeCount(t, usage.Members, (*ForkNode)(nil), 1)
    assertNodeCount(t, usage.Members, (*JoinNode)(nil), 1)
    assertNodeCount(t, usage.Members, (*SuccessionEdge)(nil), 6)
}
```

#### Test 3: Decision + Guards

```go
func TestParseActionBody_Decision(t *testing.T) {
    src := `
        action conditional {
            first start;
            decision check;
            action yes_branch;
            action no_branch;
            merge continue;
            done end;
            
            start then check;
            check then yes_branch if (x > 0);
            check then no_branch if (x <= 0);
            yes_branch then continue;
            no_branch then continue;
            continue then end;
        }
    `
    
    root := parseSource(t, src)
    usage := findUsage(root, "conditional")
    
    // Verify DecisionNode, 2 ControlFlowEdges with guards, 4 SuccessionEdges, MergeNode
    assertNodeCount(t, usage.Members, (*DecisionNode)(nil), 1)
    assertNodeCount(t, usage.Members, (*ControlFlowEdge)(nil), 2)
    assertNodeCount(t, usage.Members, (*MergeNode)(nil), 1)
    
    // Check guards non-nil
    for _, member := range usage.Members {
        if edge, ok := member.(*ControlFlowEdge); ok {
            if edge.Guard == nil {
                t.Error("ControlFlowEdge missing guard")
            }
        }
    }
}
```

---

### 12.3 Parser Tests — State Machines

#### Test 1: Simple State Machine

```go
func TestParseStateMachineBody_Simple(t *testing.T) {
    src := `
        state machine traffic {
            initial state red;
            state yellow;
            state green;
            
            transition red -> green after 30s;
            transition green -> yellow after 25s;
            transition yellow -> red after 5s;
        }
    `
    
    root := parseSource(t, src)
    usage := findUsage(root, "traffic")
    
    // Verify 3 StateNodes (red.IsInitial=true), 3 TransitionEdges with TimeEvents
    states := filterNodes(usage.Members, (*StateNode)(nil))
    if len(states) != 3 {
        t.Errorf("expected 3 states, got %d", len(states))
    }
    
    redState := findState(states, "red")
    if !redState.IsInitial {
        t.Error("red state should be initial")
    }
    
    transitions := filterNodes(usage.Members, (*TransitionEdge)(nil))
    if len(transitions) != 3 {
        t.Errorf("expected 3 transitions, got %d", len(transitions))
    }
    
    // Check all triggers are TimeEvents
    for _, trans := range transitions {
        if _, ok := trans.Trigger.(*TimeEvent); !ok {
            t.Error("expected TimeEvent trigger")
        }
    }
}
```

#### Test 2: State with Behaviors

```go
func TestParseStateMachineBody_Behaviors(t *testing.T) {
    src := `
        state machine door {
            state open {
                entry { log("Opening"); }
                do { monitor(); }
                exit { log("Closing"); }
            }
            state closed;
            
            transition closed -> open when (sensor == true);
            transition open -> closed after 10s if (auto);
        }
    `
    
    root := parseSource(t, src)
    usage := findUsage(root, "door")
    
    openState := findState(usage.Members, "open")
    if len(openState.Entry) == 0 {
        t.Error("open state missing entry behavior")
    }
    if len(openState.Do) == 0 {
        t.Error("open state missing do behavior")
    }
    if len(openState.Exit) == 0 {
        t.Error("open state missing exit behavior")
    }
    
    transitions := filterNodes(usage.Members, (*TransitionEdge)(nil))
    changeTransition := findTransition(transitions, "closed", "open")
    if _, ok := changeTransition.Trigger.(*ChangeEvent); !ok {
        t.Error("expected ChangeEvent trigger")
    }
    
    timeTransition := findTransition(transitions, "open", "closed")
    if _, ok := timeTransition.Trigger.(*TimeEvent); !ok {
        t.Error("expected TimeEvent trigger")
    }
    if timeTransition.Guard == nil {
        t.Error("transition missing guard")
    }
}
```

#### Test 3: Accept & Call Events

```go
func TestParseStateMachineBody_Events(t *testing.T) {
    src := `
        state machine receiver {
            initial state idle;
            state active;
            
            transition idle -> active accept StartSignal;
            transition active -> idle on stop();
        }
    `
    
    root := parseSource(t, src)
    usage := findUsage(root, "receiver")
    
    transitions := filterNodes(usage.Members, (*TransitionEdge)(nil))
    
    acceptTrans := findTransition(transitions, "idle", "active")
    if _, ok := acceptTrans.Trigger.(*AcceptEvent); !ok {
        t.Error("expected AcceptEvent trigger")
    }
    
    callTrans := findTransition(transitions, "active", "idle")
    if _, ok := callTrans.Trigger.(*CallEvent); !ok {
        t.Error("expected CallEvent trigger")
    }
}
```

---

### 12.4 Integration Tests

**File:** `internal/core/parser/behavior_integration_test.go`

#### Test 1: Complex Action

```go
func TestParseComplexAction(t *testing.T) {
    src := `
        action workflow {
            first start;
            fork parallel;
            action pathA1;
            action pathA2;
            action pathB1;
            fork nestedFork;
            action pathB2a;
            action pathB2b;
            join nestedJoin;
            join sync;
            decision check;
            action success;
            action failure;
            merge final;
            done end;
            
            start then parallel;
            parallel then pathA1;
            parallel then pathB1;
            pathA1 then pathA2;
            pathB1 then nestedFork;
            nestedFork then pathB2a;
            nestedFork then pathB2b;
            pathB2a then nestedJoin;
            pathB2b then nestedJoin;
            nestedJoin then sync;
            pathA2 then sync;
            sync then check;
            check then success if (result == true);
            check then failure if (result == false);
            success then final;
            failure then final;
            final then end;
        }
    `
    
    root := parseSource(t, src)
    usage := findUsage(root, "workflow")
    
    // Verify all node types present, edge count correct
    assertNodeCount(t, usage.Members, (*InitialNode)(nil), 1)
    assertNodeCount(t, usage.Members, (*FinalNode)(nil), 1)
    assertNodeCount(t, usage.Members, (*ForkNode)(nil), 2)
    assertNodeCount(t, usage.Members, (*JoinNode)(nil), 2)
    assertNodeCount(t, usage.Members, (*DecisionNode)(nil), 1)
    assertNodeCount(t, usage.Members, (*MergeNode)(nil), 1)
    assertNodeCount(t, usage.Members, (*SuccessionEdge)(nil), 14)
    assertNodeCount(t, usage.Members, (*ControlFlowEdge)(nil), 2)
}
```

#### Test 2: Hierarchical State Machine

```go
func TestParseHierarchicalStateMachine(t *testing.T) {
    src := `
        state machine system {
            initial state Init;
            state Active {
                initial state Idle;
                state Busy {
                    initial state Processing;
                    state Waiting;
                    transition Processing -> Waiting when (blocked);
                    transition Waiting -> Processing when (unblocked);
                }
                transition Idle -> Busy when (taskArrived);
                transition Busy -> Idle when (taskComplete);
            }
            final state Done;
            
            transition Init -> Active;
            transition Active -> Done when (shutdownRequested);
        }
    `
    
    root := parseSource(t, src)
    usage := findUsage(root, "system")
    
    // Verify state hierarchy
    activeState := findState(usage.Members, "Active")
    if len(activeState.Substates) != 2 {
        t.Errorf("Active state should have 2 substates, got %d", len(activeState.Substates))
    }
    
    busyState := findState(activeState.Substates, "Busy")
    if len(busyState.Substates) != 2 {
        t.Errorf("Busy state should have 2 substates, got %d", len(busyState.Substates))
    }
}
```

---

### 12.5 Error Recovery Tests

```go
func TestParseErrorRecovery_MissingThen(t *testing.T) {
    src := `
        action bad {
            first a;
            a b;  // missing 'then'
            b then done;
            done end;
        }
    `
    
    root := parseSource(t, src)
    usage := findUsage(root, "bad")
    
    // Verify ErrorNode inserted, parsing continued
    errorNodes := filterNodes(usage.Members, (*ErrorNode)(nil))
    if len(errorNodes) == 0 {
        t.Error("expected ErrorNode for malformed succession")
    }
    
    // Check remaining nodes parsed correctly
    assertNodeCount(t, usage.Members, (*InitialNode)(nil), 1)
    assertNodeCount(t, usage.Members, (*FinalNode)(nil), 1)
}

func TestParseErrorRecovery_InvalidTransition(t *testing.T) {
    src := `
        state machine bad {
            initial state a;
            transition -> nowhere;  // missing source
            state b;
            transition a -> b;
        }
    `
    
    root := parseSource(t, src)
    usage := findUsage(root, "bad")
    
    // Verify error reported, second transition parsed
    transitions := filterNodes(usage.Members, (*TransitionEdge)(nil))
    if len(transitions) < 1 {
        t.Error("valid transition should still parse after error")
    }
}
```

---

### 12.6 Test Helpers

**File:** `internal/core/parser/behavior_test.go`

```go
func parseSource(t *testing.T, src string) *ast.RootNamespace {
    source := source.New("test.sysml", []byte(src))
    p := parser.New(source)
    root := p.ParseFile()
    if len(p.Errors()) > 0 {
        for _, err := range p.Errors() {
            t.Log(err)
        }
    }
    return root
}

func findUsage(root *ast.RootNamespace, name string) *ast.Usage {
    for _, member := range root.Members {
        if u, ok := member.(*ast.Usage); ok && u.Ident.ShortName == name {
            return u
        }
    }
    return nil
}

func assertNodeCount(t *testing.T, members []ast.Node, typ ast.Node, expected int) {
    count := 0
    for _, m := range members {
        if reflect.TypeOf(m) == reflect.TypeOf(typ) {
            count++
        }
    }
    if count != expected {
        t.Errorf("expected %d nodes of type %T, got %d", expected, typ, count)
    }
}
```

---

## 13. Implementation Phases

Behavioral AST/parser extensions are large scope (19 types, 2 parser files, 20 keywords). Phased implementation reduces risk.

### Phase 1: AST Foundation (1–2 days)

**Deliverables:**
- `internal/core/ast/behavior.go` — all 19 types (7 action nodes, 3 state nodes, 4 edges, 4 triggers, 1 region)
- `internal/core/ast/behavior_test.go` — unit tests for type construction, interface compliance

**Acceptance criteria:**
- All types compile, embed `NodeBase`, implement `Node`
- `TriggerEvent` interface prevents external implementations
- Unit tests green

**No parser changes yet** — types exist, unused.

---

### Phase 2: Lexer Extensions (0.5 days)

**Deliverables:**
- `internal/core/lexer/lexer.go` — 20 new keywords in map
- `internal/core/lexer/token.go` — `TokenArrow` constant
- Lexer scanning for `->` arrow

**Acceptance criteria:**
- Lexer tokenizes `first`, `fork`, `state`, `transition`, `->`, etc.
- Keyword tests verify correct token emission

---

### Phase 3: Action Parser (2–3 days)

**Deliverables:**
- `internal/core/parser/behavior.go` — `parseActionBody()`, node parsers (`parseInitialNode`, `parseForkNode`, etc.), edge parsers (`parseSuccessionEdge`, `parseControlFlowEdge`)
- `internal/core/parser/defusage.go` — hook in `parseUsageBody()` for `UsageAction`
- `internal/core/parser/behavior_test.go` — action body tests (simple, fork/join, decision)

**Acceptance criteria:**
- All action syntax parses correctly
- Fork/join, decision/merge, succession edges produce correct AST
- Error recovery tests pass (malformed input → partial AST + ErrorNode)
- Integration test: complex action with nested forks

---

### Phase 4: State Machine Parser (3–4 days)

**Deliverables:**
- `internal/core/parser/behavior.go` — extend with `parseStateMachineBody()`, state parsers (`parseStateNode`, `parseStateBody`, `parseTransitionEdge`), trigger parsers (`parseTimeEvent`, `parseChangeEvent`, `parseAcceptEvent`, `parseCallEvent`)
- `internal/core/parser/defusage.go` — hook for `UsageState`
- `internal/core/parser/behavior_test.go` — state machine tests (simple, behaviors, triggers, hierarchical)

**Acceptance criteria:**
- All state machine syntax parses correctly
- Entry/do/exit behaviors, substates, regions parse
- All 4 trigger types (time, change, accept, call) parse
- Integration test: hierarchical state machine with 3 levels

---

### Phase 5: Semantic Validation (2 days, can overlap Phase 4)

**Deliverables:**
- `internal/core/passes/behavioral.go` — new pass for name resolution, graph validation
- Tests for unresolved references, initial node uniqueness, guard type checking

**Acceptance criteria:**
- Pass reports diagnostic for unresolved node/state names
- Pass detects missing initial nodes, duplicate initial states
- Pass type-checks guards (boolean), durations (numeric)

---

### Phase 6: Documentation & Pilot Integration (1 day)

**Deliverables:**
- Update `runtime/AGENTS.md` §2.4 (behavioral gap now closed)
- Migration guide for pilot models (keyword conflicts)
- Example models in `testdata/`

**Acceptance criteria:**
- AGENTS.md reflects completed behavioral AST
- Pilot models parse without errors (or conflicts documented)

---

**Total estimate:** 9–12 days (single developer, full-time). Parallelizable: Phases 1–2 by one developer, Phases 3–4 by another (AST types stable after Phase 1).

---

## 14. Open Questions & Future Work

### Open Questions (to resolve during implementation)

1. **Region syntax:** SysML v2 spec ambiguous on orthogonal region syntax. Pilot uses `region R { ... }` keyword, but metamodel implies implicit regions. **Decision needed:** Explicit `region` keyword (requires parsing) vs. heuristic detection (infer from independent state graphs).

2. **Synthetic name strategy:** When node missing name (`fork;`), generate `fork_1`, `fork_2`, etc. **Counter scope:** Global (per file) or local (per action body)? Recommend local (avoids cross-action conflicts).

3. **ObjectFlowEdge syntax:** Deferred to Tier 5 (requires parameter/pin grammar). **Placeholder:** Reserve edge type, leave parser stub. Tier 5 spec defines syntax.

4. **Pseudostate entry/exit points:** Metamodel defines entry/exit pseudostates for submachine interfaces. **Scope:** Defer to advanced Tier 5 features (hierarchical action invocation).

5. **Keyword conflicts:** If pilot models use `state`, `entry`, `transition` as identifiers, **mitigation:** Backtick-quoted identifiers (`\`state\``) or scoped keyword reservation (only inside behavioral bodies). Evaluate pilot corpus before finalizing.

---

### Future Work (out of current scope)

#### Tier 4 Runtime (Action Interpreter)

**Prerequisites:** Behavioral AST (this spec) complete.

**Scope:**
- Token-flow execution (Petri-net semantics)
- Fork/join concurrency model (threads, synchronization)
- Decision evaluation (guard-based branching)
- Object flow (parameter binding, data transfer)
- Step-by-step debugger integration

**Deliverables:** `internal/core/runtime/action_interpreter.go`, execution state model, scheduler.

---

#### Tier 5 Runtime (State Machine Interpreter)

**Prerequisites:** Tier 4 complete, behavioral AST complete.

**Scope:**
- Event queue (signal dispatch, time events)
- Run-to-completion semantics (process event → fire transitions → settle)
- Hierarchical state machines (entry/exit of nested states)
- Orthogonal regions (concurrent state occupancy)
- History pseudostates (shallow/deep)

**Deliverables:** `internal/core/runtime/statemachine_interpreter.go`, event scheduler, time model.

---

#### Advanced AST Features

**Deferred behavioral nodes:**
- **ParameterNode / PinNode:** Action input/output parameters (for object flow)
- **SendSignalNode / ReceiveSignalNode:** Explicit signal actions
- **HistoryPseudostate:** Shallow/deep history markers
- **TerminateNode:** Abrupt termination (different from FinalNode)

**Trigger extensions:**
- **RelativeTimeEvent:** `after 5s relative to <event>`
- **CompoundTrigger:** Boolean combinations (`after 5s and when x > 10`)

---

#### LSP Integration

**Features:**
- Syntax highlighting for behavioral keywords
- Autocomplete for node/state names in edges
- Hover tooltips for node types
- Go-to-definition for edge source/target
- Refactoring: rename node (updates all edge references)

**Implementation:** Extend `internal/lsp/` with behavioral AST traversal.

---

#### Analysis Pass

**Graph algorithms:**
- Reachability analysis (detect unreachable nodes/states)
- Deadlock detection (join without matching fork)
- Liveness analysis (guarantee termination)
- Cycle detection (infinite loops in action graphs)

**Implementation:** New pass in `internal/core/passes/graph_analysis.go`.

---

## Appendix A: Grammar Summary

**Action body syntax (EBNF):**

```ebnf
ActionBody ::= "{" ActionMember* "}"

ActionMember ::=
    | InitialNode
    | FinalNode
    | ForkNode
    | JoinNode
    | MergeNode
    | DecisionNode
    | ActionExecutionNode
    | SuccessionEdge
    | ControlFlowEdge
    | Expression  // fallback

InitialNode ::= "first" Identifier? ";"
FinalNode ::= "done" Identifier? ";"
ForkNode ::= "fork" Identifier? ";"
JoinNode ::= "join" Identifier? ";"
MergeNode ::= "merge" Identifier? ";"
DecisionNode ::= "decision" Identifier? ";"

ActionExecutionNode ::=
    | "action" Identifier ":" QualifiedName ";"
    | "action" Identifier "{" Expression "}" ";"

SuccessionEdge ::= QualifiedName "then" QualifiedName ";"
ControlFlowEdge ::= QualifiedName "then" QualifiedName "if" Expression ";"
```

---

**State machine body syntax (EBNF):**

```ebnf
StateMachineBody ::= "{" StateMachineMember* "}"

StateMachineMember ::=
    | StateNode
    | InitialState
    | FinalState
    | PseudostateNode
    | TransitionEdge
    | Expression  // fallback

InitialState ::= "initial" "state" Identifier ";"
FinalState ::= "final" "state" Identifier ";"

StateNode ::=
    | "state" Identifier ";"  // simple state
    | "state" Identifier StateBody  // composite state

StateBody ::= "{" StateMember* "}"

StateMember ::=
    | "entry" StateBehavior
    | "do" StateBehavior
    | "exit" StateBehavior
    | StateNode  // nested state
    | StateRegion

StateBehavior ::=
    | Expression ";"  // single action
    | "{" (Expression ";")* "}"  // action block

StateRegion ::= "region" Identifier? "{" StateNode* "}"

PseudostateNode ::=
    | "choice" Identifier? ";"
    | "junction" Identifier? ";"
    | "fork" Identifier? ";"
    | "join" Identifier? ";"

TransitionEdge ::= "transition" QualifiedName "->" QualifiedName TriggerClause? GuardClause? EffectClause? ";"

TriggerClause ::=
    | "after" Expression  // TimeEvent
    | "when" Expression   // ChangeEvent
    | "accept" QualifiedName  // AcceptEvent
    | "on" QualifiedName  // CallEvent

GuardClause ::= "if" Expression
EffectClause ::= "do" StateBehavior
```

---

**QualifiedName (existing):**

```ebnf
QualifiedName ::= Identifier ("::" Identifier)*
```

---

**Expression (existing):**

Reuses existing expression grammar (`internal/core/ast/expr.go`). Includes literals, operators, feature references, invocations, etc.

---

## Appendix B: Example Models

### Example 1: Simple Action with Fork/Join

```sysml
action processData {
    first start;
    fork parallel;
    action processChunkA;
    action processChunkB;
    join sync;
    action combineResults;
    done end;
    
    start then parallel;
    parallel then processChunkA;
    parallel then processChunkB;
    processChunkA then sync;
    processChunkB then sync;
    sync then combineResults;
    combineResults then end;
}
```

**Expected AST:**
- 1 InitialNode ("start")
- 1 ForkNode ("parallel")
- 2 ActionExecutionNodes ("processChunkA", "processChunkB")
- 1 JoinNode ("sync")
- 1 ActionExecutionNode ("combineResults")
- 1 FinalNode ("end")
- 6 SuccessionEdges

---

### Example 2: Decision with Guards

```sysml
action validateInput {
    first start;
    decision check;
    action acceptInput;
    action rejectInput;
    merge continue;
    done end;
    
    start then check;
    check then acceptInput if (input.valid == true);
    check then rejectInput if (input.valid == false);
    acceptInput then continue;
    rejectInput then continue;
    continue then end;
}
```

**Expected AST:**
- 1 DecisionNode ("check")
- 2 ControlFlowEdges (with guards: `input.valid == true`, `input.valid == false`)
- 1 MergeNode ("continue")

---

### Example 3: Simple State Machine

```sysml
state machine trafficLight {
    initial state red {
        entry { turnOn(redLight); }
        exit { turnOff(redLight); }
    }
    state yellow {
        entry { turnOn(yellowLight); }
        exit { turnOff(yellowLight); }
    }
    state green {
        entry { turnOn(greenLight); }
        exit { turnOff(greenLight); }
    }
    
    transition red -> green after 30s;
    transition green -> yellow after 25s;
    transition yellow -> red after 5s;
}
```

**Expected AST:**
- 3 StateNodes (red.IsInitial=true)
- Each state has Entry/Exit behaviors (1 expression each)
- 3 TransitionEdges with TimeEvent triggers (30s, 25s, 5s)

---

### Example 4: Hierarchical State Machine

```sysml
state machine system {
    initial state Init;
    state Active {
        initial state Idle;
        state Busy {
            entry { startProcessing(); }
            do { process(); }
            exit { cleanup(); }
        }
        
        transition Idle -> Busy when (taskArrived == true);
        transition Busy -> Idle when (taskComplete == true);
    }
    final state Shutdown;
    
    transition Init -> Active;
    transition Active -> Shutdown accept ShutdownSignal;
}
```

**Expected AST:**
- Root-level: 3 StateNodes (Init, Active, Shutdown)
- Active.Substates: 2 StateNodes (Idle, Busy)
- Busy state: Entry/Do/Exit populated
- 4 TransitionEdges:
  - Idle → Busy: ChangeEvent trigger
  - Busy → Idle: ChangeEvent trigger
  - Init → Active: no trigger (immediate)
  - Active → Shutdown: AcceptEvent trigger

---

### Example 5: State Machine with Multiple Trigger Types

```sysml
state machine device {
    initial state Off;
    state Standby;
    state Active;
    
    transition Off -> Standby on powerOn();
    transition Standby -> Active accept StartSignal;
    transition Active -> Standby when (idle > 60s);
    transition Standby -> Off after 300s if (batteryLow);
}
```

**Expected AST:**
- 3 StateNodes (Off.IsInitial=true)
- 4 TransitionEdges:
  - Off → Standby: CallEvent (powerOn operation)
  - Standby → Active: AcceptEvent (StartSignal)
  - Active → Standby: ChangeEvent (idle > 60s)
  - Standby → Off: TimeEvent (300s) + guard (batteryLow)

---

### Example 6: Orthogonal Regions

```sysml
state machine concurrentSystem {
    state Active {
        region ProcessingRegion {
            initial state Idle;
            state Busy;
            transition Idle -> Busy;
            transition Busy -> Idle;
        }
        
        region MonitoringRegion {
            initial state Monitoring;
            state Alerting;
            transition Monitoring -> Alerting when (errorDetected);
            transition Alerting -> Monitoring when (errorCleared);
        }
    }
}
```

**Expected AST:**
- 1 StateNode (Active) with 2 StateRegions
- Each region has independent state graph
- Transitions scoped to their region
