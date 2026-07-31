# Behavioral AST & Parser Extensions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement SysML v2 behavioral AST nodes and parser extensions to enable action control-flow graphs and state machine modeling, unblocking runtime Tiers 4–5.

**Architecture:** Additive extension—new `internal/core/ast/behavior.go` with 19 behavioral types, new `internal/core/parser/behavior.go` with specialized parsers, minimal hooks in existing `parser/defusage.go`. Lexer gains 20 keywords. All behavioral nodes implement `Node` interface, populate `Usage.Members` as first-class nodes.

**Tech Stack:** Go 1.25.10, recursive-descent parser, hand-written lexer.

---

## File Structure

**New files:**
- `internal/core/ast/behavior.go` — 19 behavioral AST types
- `internal/core/ast/behavior_test.go` — AST construction tests
- `internal/core/parser/behavior.go` — action/state parsers
- `internal/core/parser/behavior_test.go` — parser unit tests

**Modified files:**
- `internal/core/lexer/lexer.go` — 20 new keywords
- `internal/core/lexer/token.go` — TokenArrow constant
- `internal/core/parser/defusage.go` — parseUsageBody() hooks

---

## Task Outline

### Phase 1: AST Foundation
- **Task 1:** Action node types (InitialNode, FinalNode, ForkNode, JoinNode, MergeNode, DecisionNode, ActionExecutionNode)
- **Task 2:** State machine node types (StateNode, StateRegion, PseudostateNode)
- **Task 3:** Edge types (SuccessionEdge, ControlFlowEdge, ObjectFlowEdge, TransitionEdge)
- **Task 4:** Trigger event types (TriggerEvent interface, TimeEvent, ChangeEvent, AcceptEvent, CallEvent)
- **Task 5:** AST unit tests (node construction, interface compliance)

### Phase 2: Lexer Extensions
- **Task 6:** Add 20 keywords to lexer
- **Task 7:** Add TokenArrow and scanning logic

### Phase 3: Action Parser
- **Task 8:** parseActionBody() scaffold
- **Task 9:** Action node parsers (initial/final/fork/join/merge/decision)
- **Task 10:** ActionExecutionNode parser (reference + inline modes)
- **Task 11:** Succession/ControlFlow edge parsers
- **Task 12:** Action parser tests (simple, fork/join, decision)
- **Task 13:** Hook parseUsageBody() for UsageAction

### Phase 4: State Machine Parser
- **Task 14:** parseStateMachineBody() scaffold
- **Task 15:** State node parsers (simple, composite, initial/final)
- **Task 16:** State behavior parsers (entry/do/exit)
- **Task 17:** Transition edge parser
- **Task 18:** Trigger event parsers (time/change/accept/call)
- **Task 19:** Pseudostate parser
- **Task 20:** State machine parser tests (simple, behaviors, triggers, hierarchical)
- **Task 21:** Hook parseUsageBody() for UsageState

### Phase 5: Error Recovery & Integration
- **Task 22:** Error recovery (synchronize, ErrorNode insertion)
- **Task 23:** Integration tests (complex action, hierarchical state machine)
- **Task 24:** Error recovery tests

### Phase 6: Documentation
- **Task 25:** Update AGENTS.md §2.4
- **Task 26:** Add example models to testdata/

---

## Task Details

[Tasks will be filled section by section below]

---

### Task 1: Action Node Types

**Files:**
- Create: `internal/core/ast/behavior.go`

- [ ] **Step 1: Create behavior.go with package declaration**

```go
package ast

// Behavioral AST nodes for SysML v2 actions and state machines.
// These nodes implement the Node interface and populate Usage.Members
// for action and state usages (UsageAction, UsageState).
```

Run: `touch internal/core/ast/behavior.go` (if not exists), add package + comment.

- [ ] **Step 2: Define InitialNode**

```go
// InitialNode is the entry point for action execution.
type InitialNode struct {
	NodeBase
	Name string // optional identifier for edge referencing
}
```

Add to `behavior.go`.

- [ ] **Step 3: Define FinalNode**

```go
// FinalNode is the termination point for action execution.
type FinalNode struct {
	NodeBase
	Name string
}
```

- [ ] **Step 4: Define ForkNode**

```go
// ForkNode splits execution into concurrent flows (1 incoming → N outgoing).
type ForkNode struct {
	NodeBase
	Name string
}
```

- [ ] **Step 5: Define JoinNode**

```go
// JoinNode synchronizes concurrent flows (N incoming → 1 outgoing).
type JoinNode struct {
	NodeBase
	Name string
}
```

- [ ] **Step 6: Define MergeNode**

```go
// MergeNode merges alternative flows (N incoming → 1 outgoing, first-wins).
type MergeNode struct {
	NodeBase
	Name string
}
```

- [ ] **Step 7: Define DecisionNode**

```go
// DecisionNode is a conditional branch point (1 incoming → N guarded outgoing).
type DecisionNode struct {
	NodeBase
	Name string
}
```

- [ ] **Step 8: Define ActionExecutionNode**

```go
// ActionExecutionNode performs action work: invokes nested action or evaluates inline expression.
type ActionExecutionNode struct {
	NodeBase
	Name       string
	ActionRef  *QualifiedName // reference to nested action (mutually exclusive with Expression)
	Expression Node            // inline expression (mutually exclusive with ActionRef)
}
```

- [ ] **Step 9: Verify build**

Run: `go build ./internal/core/ast/`  
Expected: SUCCESS (no errors)

- [ ] **Step 10: Commit action nodes**

```bash
git add internal/core/ast/behavior.go
git commit -m "feat(ast): add action control-flow node types

7 node types for action execution graphs: InitialNode, FinalNode,
ForkNode, JoinNode, MergeNode, DecisionNode, ActionExecutionNode.
All embed NodeBase (implement Node interface)."
```

---

### Task 2: State Machine Node Types

**Files:**
- Modify: `internal/core/ast/behavior.go`

- [ ] **Step 1: Define StateNode**

```go
// StateNode represents a state in a state machine (simple, composite, or orthogonal).
type StateNode struct {
	NodeBase
	Name         string
	IsInitial    bool              // initial state marker
	IsFinal      bool              // final state marker
	Entry        []Node            // entry behaviors (action sequence)
	Do           []Node            // do activity (ongoing action)
	Exit         []Node            // exit behaviors (action sequence)
	Substates    []Node            // nested states (hierarchical)
	Regions      []*StateRegion    // orthogonal regions (parallel)
}
```

Add after ActionExecutionNode in `behavior.go`.

- [ ] **Step 2: Define StateRegion**

```go
// StateRegion is an orthogonal region within a composite state.
type StateRegion struct {
	NodeBase
	Name   string
	States []Node // states in this region
}
```

- [ ] **Step 3: Define PseudostateKind enum**

```go
// PseudostateKind discriminates pseudostate types.
type PseudostateKind int

const (
	PseudostateChoice PseudostateKind = iota // conditional branch
	PseudostateJunction                       // merge point
	PseudostateFork                           // parallel split
	PseudostateJoin                           // parallel sync
	PseudostateEntry                          // entry point (submachine)
	PseudostateExit                           // exit point (submachine)
)
```

- [ ] **Step 4: Define PseudostateNode**

```go
// PseudostateNode is a transient state for control flow.
type PseudostateNode struct {
	NodeBase
	Kind PseudostateKind
	Name string
}
```

- [ ] **Step 5: Add String() method for PseudostateKind**

```go
func (k PseudostateKind) String() string {
	switch k {
	case PseudostateChoice:
		return "choice"
	case PseudostateJunction:
		return "junction"
	case PseudostateFork:
		return "fork"
	case PseudostateJoin:
		return "join"
	case PseudostateEntry:
		return "entry"
	case PseudostateExit:
		return "exit"
	default:
		return "unknown"
	}
}
```

- [ ] **Step 6: Verify build**

Run: `go build ./internal/core/ast/`  
Expected: SUCCESS

- [ ] **Step 7: Commit state nodes**

```bash
git add internal/core/ast/behavior.go
git commit -m "feat(ast): add state machine node types

StateNode (simple/composite/initial/final), StateRegion (orthogonal),
PseudostateNode with 6 kinds (choice/junction/fork/join/entry/exit)."
```

---

### Task 3: Edge Types

**Files:**
- Modify: `internal/core/ast/behavior.go`

- [ ] **Step 1: Define SuccessionEdge**

```go
// SuccessionEdge is sequential control flow in actions (source then target).
type SuccessionEdge struct {
	NodeBase
	Source *QualifiedName // source action node
	Target *QualifiedName // target action node
}
```

Add after PseudostateNode in `behavior.go`.

- [ ] **Step 2: Define ControlFlowEdge**

```go
// ControlFlowEdge is guarded control flow from decision nodes.
type ControlFlowEdge struct {
	NodeBase
	Source *QualifiedName // source node (typically DecisionNode)
	Target *QualifiedName // target node
	Guard  Node            // boolean guard expression
}
```

- [ ] **Step 3: Define ObjectFlowEdge**

```go
// ObjectFlowEdge is data flow between action parameters/pins (Tier 5).
type ObjectFlowEdge struct {
	NodeBase
	Source *QualifiedName // source pin/parameter
	Target *QualifiedName // target pin/parameter
}
```

- [ ] **Step 4: Define TransitionEdge**

```go
// TransitionEdge is a state machine transition.
type TransitionEdge struct {
	NodeBase
	Source  *QualifiedName // source state
	Target  *QualifiedName // target state
	Trigger TriggerEvent   // event that fires transition (interface, see below)
	Guard   Node           // optional guard expression
	Effect  []Node         // optional effect actions
}
```

**Note:** TriggerEvent interface defined in Task 4.

- [ ] **Step 5: Verify build (expect error — TriggerEvent not defined yet)**

Run: `go build ./internal/core/ast/`  
Expected: FAIL with "undefined: TriggerEvent"

This is expected. Task 4 will define TriggerEvent interface. Proceed to Task 4.

- [ ] **Step 6: Commit edges (will compile after Task 4)**

```bash
git add internal/core/ast/behavior.go
git commit -m "feat(ast): add edge types

SuccessionEdge, ControlFlowEdge, ObjectFlowEdge, TransitionEdge.
Build broken until Task 4 (TriggerEvent interface)."
```

---

### Task 4: Trigger Event Types

**Files:**
- Modify: `internal/core/ast/behavior.go`

- [ ] **Step 1: Define TriggerEvent interface**

```go
// TriggerEvent is the interface for state transition triggers.
type TriggerEvent interface {
	Node
	triggerEvent() // unexported marker method (closed set)
}
```

Add after TransitionEdge in `behavior.go`.

- [ ] **Step 2: Define TimeEvent**

```go
// TimeEvent fires after a specified duration.
type TimeEvent struct {
	NodeBase
	Duration Node // time expression (literal or variable)
}

func (*TimeEvent) triggerEvent() {}
```

- [ ] **Step 3: Define ChangeEvent**

```go
// ChangeEvent fires when a condition becomes true.
type ChangeEvent struct {
	NodeBase
	Condition Node // boolean expression
}

func (*ChangeEvent) triggerEvent() {}
```

- [ ] **Step 4: Define AcceptEvent**

```go
// AcceptEvent fires when a signal is received.
type AcceptEvent struct {
	NodeBase
	SignalType *QualifiedName // signal type to accept
}

func (*AcceptEvent) triggerEvent() {}
```

- [ ] **Step 5: Define CallEvent**

```go
// CallEvent fires when an operation is invoked.
type CallEvent struct {
	NodeBase
	Operation *QualifiedName // operation to invoke
}

func (*CallEvent) triggerEvent() {}
```

- [ ] **Step 6: Verify build (should now compile)**

Run: `go build ./internal/core/ast/`  
Expected: SUCCESS (TriggerEvent interface resolves TransitionEdge.Trigger)

- [ ] **Step 7: Commit trigger events**

```bash
git add internal/core/ast/behavior.go
git commit -m "feat(ast): add trigger event types

TriggerEvent interface + 4 implementations: TimeEvent, ChangeEvent,
AcceptEvent, CallEvent. Fixes build from Task 3."
```

---

### Task 5: AST Unit Tests

**Files:**
- Create: `internal/core/ast/behavior_test.go`

- [ ] **Step 1: Create test file with package**

```go
package ast

import (
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)
```

- [ ] **Step 2: Write test for action node construction**

```go
func TestActionNodeConstruction(t *testing.T) {
	span := source.Span{Start: 0, End: 10}
	
	nodes := []Node{
		&InitialNode{NodeBase: NodeBase{SpanVal: span}, Name: "start"},
		&FinalNode{NodeBase: NodeBase{SpanVal: span}, Name: "end"},
		&ForkNode{NodeBase: NodeBase{SpanVal: span}, Name: "split"},
		&JoinNode{NodeBase: NodeBase{SpanVal: span}, Name: "sync"},
		&MergeNode{NodeBase: NodeBase{SpanVal: span}, Name: "merge"},
		&DecisionNode{NodeBase: NodeBase{SpanVal: span}, Name: "decide"},
		&ActionExecutionNode{NodeBase: NodeBase{SpanVal: span}, Name: "exec"},
	}
	
	for i, node := range nodes {
		if node.Span().Start != 0 || node.Span().End != 10 {
			t.Errorf("node %d: span mismatch", i)
		}
	}
}
```

- [ ] **Step 3: Write test for state node construction**

```go
func TestStateNodeConstruction(t *testing.T) {
	span := source.Span{Start: 0, End: 20}
	
	state := &StateNode{
		NodeBase: NodeBase{SpanVal: span},
		Name:     "Active",
		IsInitial: true,
		Entry:    []Node{},
		Do:       []Node{},
		Exit:     []Node{},
	}
	
	if state.Name != "Active" {
		t.Errorf("expected name 'Active', got %q", state.Name)
	}
	if !state.IsInitial {
		t.Error("expected IsInitial=true")
	}
	if state.Span().End != 20 {
		t.Error("span mismatch")
	}
}
```

- [ ] **Step 4: Write test for TriggerEvent interface compliance**

```go
func TestTriggerEventInterface(t *testing.T) {
	// Verify all 4 trigger types implement TriggerEvent interface
	var _ TriggerEvent = (*TimeEvent)(nil)
	var _ TriggerEvent = (*ChangeEvent)(nil)
	var _ TriggerEvent = (*AcceptEvent)(nil)
	var _ TriggerEvent = (*CallEvent)(nil)
	
	// Verify they also implement Node (via NodeBase)
	var _ Node = (*TimeEvent)(nil)
	var _ Node = (*ChangeEvent)(nil)
	var _ Node = (*AcceptEvent)(nil)
	var _ Node = (*CallEvent)(nil)
}
```

- [ ] **Step 5: Write test for edge construction**

```go
func TestEdgeConstruction(t *testing.T) {
	span := source.Span{Start: 0, End: 15}
	source := &QualifiedName{Parts: []string{"a"}}
	target := &QualifiedName{Parts: []string{"b"}}
	
	succEdge := &SuccessionEdge{
		NodeBase: NodeBase{SpanVal: span},
		Source:   source,
		Target:   target,
	}
	
	if succEdge.Source.Parts[0] != "a" {
		t.Error("source mismatch")
	}
	if succEdge.Target.Parts[0] != "b" {
		t.Error("target mismatch")
	}
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/core/ast/ -run TestActionNode -v`  
Expected: PASS

Run: `go test ./internal/core/ast/ -run TestStateNode -v`  
Expected: PASS

Run: `go test ./internal/core/ast/ -run TestTriggerEvent -v`  
Expected: PASS

Run: `go test ./internal/core/ast/ -run TestEdge -v`  
Expected: PASS

- [ ] **Step 7: Commit tests**

```bash
git add internal/core/ast/behavior_test.go
git commit -m "test(ast): add behavioral AST unit tests

Tests for action nodes, state nodes, edge construction, and
TriggerEvent interface compliance. All types verified."
```

---

### Task 6: Add Keywords to Lexer

**Files:**
- Modify: `internal/core/lexer/lexer.go` (keyword map in `init()` function)

- [ ] **Step 1: Locate keyword map**

Find the `keywords` map in `lexer.go`. It should be in an `init()` function or as a package-level var.

- [ ] **Step 2: Add action keywords**

Add these 7 entries to the keyword map:

```go
"first":    TokenKeyword,
"done":     TokenKeyword,
"fork":     TokenKeyword,
"join":     TokenKeyword,
"merge":    TokenKeyword,
"decision": TokenKeyword,
"then":     TokenKeyword,
```

- [ ] **Step 3: Add state keywords**

Add these 6 entries:

```go
"state":      TokenKeyword,
"initial":    TokenKeyword,
"final":      TokenKeyword,
"entry":      TokenKeyword,
"exit":       TokenKeyword,
"transition": TokenKeyword,
```

- [ ] **Step 4: Add trigger keywords**

Add these 4 entries:

```go
"after":  TokenKeyword,
"when":   TokenKeyword,
"accept": TokenKeyword,
"on":     TokenKeyword,
```

- [ ] **Step 5: Add pseudostate keywords**

Add these 2 entries:

```go
"choice":   TokenKeyword,
"junction": TokenKeyword,
```

- [ ] **Step 6: Add region keyword (if not already present)**

```go
"region": TokenKeyword,
```

**Note:** Keywords `if`, `do`, `action` already exist—verify they are present, do not duplicate.

- [ ] **Step 7: Verify build**

Run: `go build ./internal/core/lexer/`  
Expected: SUCCESS

- [ ] **Step 8: Test keyword recognition**

Create test: `internal/core/lexer/lexer_test.go` (if not exists, or add to existing):

```go
func TestBehavioralKeywords(t *testing.T) {
	src := source.New("test", []byte("first fork state transition after"))
	l := New(src)
	
	expectedKeywords := []string{"first", "fork", "state", "transition", "after"}
	for i, expected := range expectedKeywords {
		tok := l.NextToken()
		if tok.Type != TokenKeyword {
			t.Errorf("token %d: expected TokenKeyword, got %v", i, tok.Type)
		}
		if tok.Lexeme != expected {
			t.Errorf("token %d: expected %q, got %q", i, expected, tok.Lexeme)
		}
	}
}
```

Run: `go test ./internal/core/lexer/ -run TestBehavioral -v`  
Expected: PASS

- [ ] **Step 9: Commit keywords**

```bash
git add internal/core/lexer/lexer.go internal/core/lexer/lexer_test.go
git commit -m "feat(lexer): add 20 behavioral keywords

Action: first, done, fork, join, merge, decision, then
State: state, initial, final, entry, exit, transition
Trigger: after, when, accept, on
Pseudostate: choice, junction
Region: region"
```

---

### Task 7: Add TokenArrow

**Files:**
- Modify: `internal/core/lexer/token.go`
- Modify: `internal/core/lexer/lexer.go` (scanToken method)

- [ ] **Step 1: Add TokenArrow constant**

In `token.go`, find the `const` block with token types. Add:

```go
TokenArrow // "->" (transition arrow)
```

Add after existing operator tokens (e.g., after `TokenMinus` or similar).

- [ ] **Step 2: Add TokenArrow to String() method**

If `token.go` has a `func (t TokenType) String()` method, add case:

```go
case TokenArrow:
    return "->"
```

- [ ] **Step 3: Implement arrow scanning**

In `lexer.go`, find the `scanToken()` method. Locate the `case '-':` branch (for TokenMinus).

Modify to:

```go
case '-':
    if l.match('>') {
        l.addToken(TokenArrow)
    } else {
        l.addToken(TokenMinus)
    }
```

**Note:** If `match()` helper doesn't exist, implement:

```go
func (l *Lexer) match(expected rune) bool {
    if l.isAtEnd() || l.peek() != expected {
        return false
    }
    l.advance()
    return true
}

func (l *Lexer) peek() rune {
    if l.isAtEnd() {
        return 0
    }
    return l.input[l.current]
}
```

- [ ] **Step 4: Test arrow scanning**

Add to `lexer_test.go`:

```go
func TestTokenArrow(t *testing.T) {
    src := source.New("test", []byte("a -> b"))
    l := New(src)
    
    l.NextToken() // 'a'
    tok := l.NextToken()
    
    if tok.Type != TokenArrow {
        t.Errorf("expected TokenArrow, got %v", tok.Type)
    }
    if tok.Lexeme != "->" {
        t.Errorf("expected '->', got %q", tok.Lexeme)
    }
}
```

Run: `go test ./internal/core/lexer/ -run TestTokenArrow -v`  
Expected: PASS

- [ ] **Step 5: Verify minus still works**

```go
func TestMinusNotArrow(t *testing.T) {
    src := source.New("test", []byte("a - b"))
    l := New(src)
    
    l.NextToken() // 'a'
    tok := l.NextToken()
    
    if tok.Type != TokenMinus {
        t.Errorf("expected TokenMinus, got %v", tok.Type)
    }
}
```

Run: `go test ./internal/core/lexer/ -run TestMinusNotArrow -v`  
Expected: PASS

- [ ] **Step 6: Commit TokenArrow**

```bash
git add internal/core/lexer/token.go internal/core/lexer/lexer.go internal/core/lexer/lexer_test.go
git commit -m "feat(lexer): add TokenArrow for transition syntax

Scans '->' as single token (not minus + greater). Used in state
machine transitions: 'transition source -> target'."
```

---

### Task 8: parseActionBody() Scaffold

**Files:**
- Create: `internal/core/parser/behavior.go`

- [ ] **Step 1: Create behavior.go with package + imports**

```go
package parser

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
)
```

- [ ] **Step 2: Add parseActionBody method to Parser**

```go
// parseActionBody parses the body of an action usage.
// Expects '{' already consumed, returns list of action nodes + edges.
func (p *Parser) parseActionBody() []ast.Node {
	var members []ast.Node
	
	for !p.check(lexer.TokenRightBrace) && !p.isAtEnd() {
		members = append(members, p.parseActionMember())
	}
	
	p.consume(lexer.TokenRightBrace, "expected '}' after action body")
	return members
}
```

- [ ] **Step 3: Add parseActionMember dispatcher**

```go
// parseActionMember parses one action member: node or edge.
func (p *Parser) parseActionMember() ast.Node {
	// Check for keyword dispatch
	if p.match(lexer.TokenKeyword) {
		kw := p.previous().Lexeme
		switch kw {
		case "first":
			return p.parseInitialNode()
		case "done":
			return p.parseFinalNode()
		case "fork":
			return p.parseForkNode()
		case "join":
			return p.parseJoinNode()
		case "merge":
			return p.parseMergeNode()
		case "decision":
			return p.parseDecisionNode()
		case "action":
			return p.parseActionExecutionNode()
		case "then":
			return p.parseSuccessionEdge()
		default:
			// Unknown keyword, return ErrorNode
			return &ast.ErrorNode{
				NodeBase: ast.NodeBase{SpanVal: p.previous().Span},
				Err:      "unknown action keyword: " + kw,
			}
		}
	}
	
	// Not a keyword — assume expression or edge with inferred 'then'
	// For now, return ErrorNode (Task 11 will handle edges)
	return &ast.ErrorNode{
		NodeBase: ast.NodeBase{SpanVal: p.peek().Span},
		Err:      "expected action node or edge keyword",
	}
}
```

- [ ] **Step 4: Add stub node parsers**

```go
// Stubs — Tasks 9-10 will implement
func (p *Parser) parseInitialNode() ast.Node {
	return &ast.InitialNode{NodeBase: ast.NodeBase{SpanVal: p.previous().Span}}
}

func (p *Parser) parseFinalNode() ast.Node {
	return &ast.FinalNode{NodeBase: ast.NodeBase{SpanVal: p.previous().Span}}
}

func (p *Parser) parseForkNode() ast.Node {
	return &ast.ForkNode{NodeBase: ast.NodeBase{SpanVal: p.previous().Span}}
}

func (p *Parser) parseJoinNode() ast.Node {
	return &ast.JoinNode{NodeBase: ast.NodeBase{SpanVal: p.previous().Span}}
}

func (p *Parser) parseMergeNode() ast.Node {
	return &ast.MergeNode{NodeBase: ast.NodeBase{SpanVal: p.previous().Span}}
}

func (p *Parser) parseDecisionNode() ast.Node {
	return &ast.DecisionNode{NodeBase: ast.NodeBase{SpanVal: p.previous().Span}}
}

func (p *Parser) parseActionExecutionNode() ast.Node {
	return &ast.ActionExecutionNode{NodeBase: ast.NodeBase{SpanVal: p.previous().Span}}
}

func (p *Parser) parseSuccessionEdge() ast.Node {
	return &ast.SuccessionEdge{NodeBase: ast.NodeBase{SpanVal: p.previous().Span}}
}
```

- [ ] **Step 5: Verify build**

Run: `go build ./internal/core/parser/`  
Expected: SUCCESS (stubs compile, no implementation yet)

- [ ] **Step 6: Commit scaffold**

```bash
git add internal/core/parser/behavior.go
git commit -m "feat(parser): scaffold parseActionBody + dispatcher

Keyword dispatcher for action nodes (first/done/fork/join/merge/
decision/action/then). Stubs return empty nodes. Tasks 9-11 will
implement parsers."
```

---

### Task 9: Action Node Parsers

**Files:**
- Modify: `internal/core/parser/behavior.go`

- [ ] **Step 1: Implement parseInitialNode**

Replace stub with:

```go
// parseInitialNode parses: first [name] ;
func (p *Parser) parseInitialNode() ast.Node {
	start := p.previous().Span
	
	var name string
	if p.check(lexer.TokenIdentifier) {
		name = p.advance().Lexeme
	}
	
	p.consume(lexer.TokenSemicolon, "expected ';' after initial node")
	
	return &ast.InitialNode{
		NodeBase: ast.NodeBase{SpanVal: start.To(p.previous().Span.End)},
		Name:     name,
	}
}
```

- [ ] **Step 2: Implement parseFinalNode**

```go
// parseFinalNode parses: done [name] ;
func (p *Parser) parseFinalNode() ast.Node {
	start := p.previous().Span
	
	var name string
	if p.check(lexer.TokenIdentifier) {
		name = p.advance().Lexeme
	}
	
	p.consume(lexer.TokenSemicolon, "expected ';' after final node")
	
	return &ast.FinalNode{
		NodeBase: ast.NodeBase{SpanVal: start.To(p.previous().Span.End)},
		Name:     name,
	}
}
```

- [ ] **Step 3: Implement parseForkNode**

```go
// parseForkNode parses: fork [name] ;
func (p *Parser) parseForkNode() ast.Node {
	start := p.previous().Span
	
	var name string
	if p.check(lexer.TokenIdentifier) {
		name = p.advance().Lexeme
	}
	
	p.consume(lexer.TokenSemicolon, "expected ';' after fork node")
	
	return &ast.ForkNode{
		NodeBase: ast.NodeBase{SpanVal: start.To(p.previous().Span.End)},
		Name:     name,
	}
}
```

- [ ] **Step 4: Implement parseJoinNode**

```go
// parseJoinNode parses: join [name] ;
func (p *Parser) parseJoinNode() ast.Node {
	start := p.previous().Span
	
	var name string
	if p.check(lexer.TokenIdentifier) {
		name = p.advance().Lexeme
	}
	
	p.consume(lexer.TokenSemicolon, "expected ';' after join node")
	
	return &ast.JoinNode{
		NodeBase: ast.NodeBase{SpanVal: start.To(p.previous().Span.End)},
		Name:     name,
	}
}
```

- [ ] **Step 5: Implement parseMergeNode**

```go
// parseMergeNode parses: merge [name] ;
func (p *Parser) parseMergeNode() ast.Node {
	start := p.previous().Span
	
	var name string
	if p.check(lexer.TokenIdentifier) {
		name = p.advance().Lexeme
	}
	
	p.consume(lexer.TokenSemicolon, "expected ';' after merge node")
	
	return &ast.MergeNode{
		NodeBase: ast.NodeBase{SpanVal: start.To(p.previous().Span.End)},
		Name:     name,
	}
}
```

- [ ] **Step 6: Implement parseDecisionNode**

```go
// parseDecisionNode parses: decision [name] ;
func (p *Parser) parseDecisionNode() ast.Node {
	start := p.previous().Span
	
	var name string
	if p.check(lexer.TokenIdentifier) {
		name = p.advance().Lexeme
	}
	
	p.consume(lexer.TokenSemicolon, "expected ';' after decision node")
	
	return &ast.DecisionNode{
		NodeBase: ast.NodeBase{SpanVal: start.To(p.previous().Span.End)},
		Name:     name,
	}
}
```

- [ ] **Step 7: Verify build**

Run: `go build ./internal/core/parser/`  
Expected: SUCCESS

- [ ] **Step 8: Commit node parsers**

```bash
git add internal/core/parser/behavior.go
git commit -m "feat(parser): implement action node parsers

6 control-flow node parsers: initial, final, fork, join, merge,
decision. All parse optional name + semicolon."
```

---

### Task 10: ActionExecutionNode Parser

**Files:**
- Modify: `internal/core/parser/behavior.go`

- [ ] **Step 1: Implement parseActionExecutionNode**

Replace stub with:

```go
// parseActionExecutionNode parses:
//   action [name] actionRef ;
//   action [name] { expression } ;
func (p *Parser) parseActionExecutionNode() ast.Node {
	start := p.previous().Span
	
	var name string
	if p.check(lexer.TokenIdentifier) && !p.checkNext(lexer.TokenLeftBrace) {
		name = p.advance().Lexeme
	}
	
	var actionRef *ast.QualifiedName
	var expression ast.Node
	
	if p.check(lexer.TokenLeftBrace) {
		// Inline expression mode
		p.advance() // consume '{'
		expression = p.parseExpression()
		p.consume(lexer.TokenRightBrace, "expected '}' after action expression")
	} else if p.check(lexer.TokenIdentifier) {
		// Reference mode
		actionRef = p.parseQualifiedName()
	} else {
		return &ast.ErrorNode{
			NodeBase: ast.NodeBase{SpanVal: start},
			Err:      "expected action reference or '{' after 'action'",
		}
	}
	
	p.consume(lexer.TokenSemicolon, "expected ';' after action execution node")
	
	return &ast.ActionExecutionNode{
		NodeBase:   ast.NodeBase{SpanVal: start.To(p.previous().Span.End)},
		Name:       name,
		ActionRef:  actionRef,
		Expression: expression,
	}
}
```

**Note:** `checkNext()` helper checks lookahead without advancing:

```go
// checkNext checks if second token matches (lookahead 2).
func (p *Parser) checkNext(t lexer.TokenType) bool {
	if p.current+1 >= len(p.tokens) {
		return false
	}
	return p.tokens[p.current+1].Type == t
}
```

Add this helper if not present.

- [ ] **Step 2: Verify build**

Run: `go build ./internal/core/parser/`  
Expected: SUCCESS

- [ ] **Step 3: Commit ActionExecutionNode parser**

```bash
git add internal/core/parser/behavior.go
git commit -m "feat(parser): implement ActionExecutionNode parser

Dual-mode: reference (action [name] ref;) or inline expression
(action [name] { expr };). Mutually exclusive ActionRef/Expression."
```

---

### Task 11: Edge Parsers (Action)

**Files:**
- Modify: `internal/core/parser/behavior.go`

- [ ] **Step 1: Implement parseSuccessionEdge**

Replace stub with:

```go
// parseSuccessionEdge parses: then source target ;
func (p *Parser) parseSuccessionEdge() ast.Node {
	start := p.previous().Span
	
	source := p.parseQualifiedName()
	target := p.parseQualifiedName()
	
	p.consume(lexer.TokenSemicolon, "expected ';' after succession edge")
	
	return &ast.SuccessionEdge{
		NodeBase: ast.NodeBase{SpanVal: start.To(p.previous().Span.End)},
		Source:   source,
		Target:   target,
	}
}
```

- [ ] **Step 2: Add parseControlFlowEdge**

```go
// parseControlFlowEdge parses: then source target if guard ;
func (p *Parser) parseControlFlowEdge(source, target *ast.QualifiedName, start source.Span) ast.Node {
	// 'if' keyword already consumed
	guard := p.parseExpression()
	
	p.consume(lexer.TokenSemicolon, "expected ';' after control flow edge")
	
	return &ast.ControlFlowEdge{
		NodeBase: ast.NodeBase{SpanVal: start.To(p.previous().Span.End)},
		Source:   source,
		Target:   target,
		Guard:    guard,
	}
}
```

- [ ] **Step 3: Update parseSuccessionEdge to detect guards**

Replace parseSuccessionEdge with:

```go
// parseSuccessionEdge parses: then source target [if guard] ;
func (p *Parser) parseSuccessionEdge() ast.Node {
	start := p.previous().Span
	
	source := p.parseQualifiedName()
	target := p.parseQualifiedName()
	
	// Check for optional guard
	if p.match(lexer.TokenKeyword) && p.previous().Lexeme == "if" {
		return p.parseControlFlowEdge(source, target, start)
	}
	
	p.consume(lexer.TokenSemicolon, "expected ';' after succession edge")
	
	return &ast.SuccessionEdge{
		NodeBase: ast.NodeBase{SpanVal: start.To(p.previous().Span.End)},
		Source:   source,
		Target:   target,
	}
}
```

- [ ] **Step 4: Add ObjectFlowEdge parser (stub for Tier 5)**

```go
// parseObjectFlowEdge parses: flow source target ; (Tier 5, deferred)
func (p *Parser) parseObjectFlowEdge() ast.Node {
	start := p.previous().Span
	
	// Stub: minimal parse, return ErrorNode (not implemented)
	return &ast.ErrorNode{
		NodeBase: ast.NodeBase{SpanVal: start},
		Err:      "ObjectFlowEdge not implemented (Tier 5)",
	}
}
```

- [ ] **Step 5: Verify build**

Run: `go build ./internal/core/parser/`  
Expected: SUCCESS

- [ ] **Step 6: Commit edge parsers**

```bash
git add internal/core/parser/behavior.go
git commit -m "feat(parser): implement action edge parsers

SuccessionEdge (then source target;) and ControlFlowEdge (then source
target if guard;). ObjectFlowEdge stubbed (Tier 5 deferred)."
```

---

### Task 12: Action Parser Tests

**Files:**
- Create: `internal/core/parser/behavior_test.go`

- [ ] **Step 1: Create test file with helpers**

```go
package parser

import (
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func parseActionTest(t *testing.T, input string) []ast.Node {
	src := source.New("test.sysml", []byte(input))
	lex := lexer.New(src)
	p := New(lex)
	return p.parseActionBody()
}
```

- [ ] **Step 2: Test simple action with initial + final**

```go
func TestParseAction_Simple(t *testing.T) {
	input := `{
		first start;
		done end;
	}`
	
	members := parseActionTest(t, input)
	
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
	
	initial, ok := members[0].(*ast.InitialNode)
	if !ok {
		t.Errorf("expected InitialNode, got %T", members[0])
	}
	if initial.Name != "start" {
		t.Errorf("expected name 'start', got %q", initial.Name)
	}
	
	final, ok := members[1].(*ast.FinalNode)
	if !ok {
		t.Errorf("expected FinalNode, got %T", members[1])
	}
	if final.Name != "end" {
		t.Errorf("expected name 'end', got %q", final.Name)
	}
}
```

- [ ] **Step 3: Test fork/join**

```go
func TestParseAction_ForkJoin(t *testing.T) {
	input := `{
		first start;
		fork split;
		action taskA NestedAction;
		action taskB NestedAction;
		join sync;
		done end;
	}`
	
	members := parseActionTest(t, input)
	
	if len(members) != 6 {
		t.Fatalf("expected 6 members, got %d", len(members))
	}
	
	fork, ok := members[1].(*ast.ForkNode)
	if !ok {
		t.Errorf("expected ForkNode at index 1, got %T", members[1])
	}
	if fork.Name != "split" {
		t.Errorf("expected fork name 'split', got %q", fork.Name)
	}
	
	join, ok := members[4].(*ast.JoinNode)
	if !ok {
		t.Errorf("expected JoinNode at index 4, got %T", members[4])
	}
	if join.Name != "sync" {
		t.Errorf("expected join name 'sync', got %q", join.Name)
	}
}
```

- [ ] **Step 4: Test decision with edges**

```go
func TestParseAction_Decision(t *testing.T) {
	input := `{
		first start;
		decision check;
		then check taskA if condition;
		then check taskB;
		done end;
	}`
	
	members := parseActionTest(t, input)
	
	if len(members) != 5 {
		t.Fatalf("expected 5 members, got %d", len(members))
	}
	
	decision, ok := members[1].(*ast.DecisionNode)
	if !ok {
		t.Errorf("expected DecisionNode, got %T", members[1])
	}
	if decision.Name != "check" {
		t.Errorf("expected decision name 'check', got %q", decision.Name)
	}
	
	// First edge is control flow (guarded)
	cfEdge, ok := members[2].(*ast.ControlFlowEdge)
	if !ok {
		t.Errorf("expected ControlFlowEdge, got %T", members[2])
	}
	if cfEdge.Source.Parts[0] != "check" {
		t.Errorf("expected source 'check', got %q", cfEdge.Source.Parts[0])
	}
	if cfEdge.Guard == nil {
		t.Error("expected guard expression, got nil")
	}
	
	// Second edge is succession (no guard)
	succEdge, ok := members[3].(*ast.SuccessionEdge)
	if !ok {
		t.Errorf("expected SuccessionEdge, got %T", members[3])
	}
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/core/parser/ -run TestParseAction_Simple -v`  
Expected: PASS

Run: `go test ./internal/core/parser/ -run TestParseAction_ForkJoin -v`  
Expected: PASS

Run: `go test ./internal/core/parser/ -run TestParseAction_Decision -v`  
Expected: PASS

- [ ] **Step 6: Commit tests**

```bash
git add internal/core/parser/behavior_test.go
git commit -m "test(parser): add action parser tests

3 tests: simple (initial+final), fork/join, decision with guarded
edges. All verify node types, names, and relationships."
```

---

### Task 13: Hook parseUsageBody() for Actions

**Files:**
- Modify: `internal/core/parser/defusage.go` (parseUsageBody method)

- [ ] **Step 1: Locate parseUsageBody method**

Find the method in `defusage.go`. It should switch on usage kind (UsagePart, UsageAttribute, etc.).

- [ ] **Step 2: Add UsageAction case**

Add case to switch:

```go
case ast.UsageAction:
	if p.check(lexer.TokenLeftBrace) {
		p.advance() // consume '{'
		usage.Members = p.parseActionBody()
	}
```

**Note:** `parseActionBody()` already consumes closing `}` (see Task 8).

- [ ] **Step 3: Verify build**

Run: `go build ./internal/core/parser/`  
Expected: SUCCESS

- [ ] **Step 4: Add integration test**

In `behavior_test.go`, add:

```go
func TestParseUsageAction_Integration(t *testing.T) {
	input := `action TestAction {
		first start;
		action exec NestedAction;
		done end;
	}`
	
	src := source.New("test.sysml", []byte(input))
	lex := lexer.New(src)
	p := New(lex)
	root := p.ParseFile()
	
	// Find action usage in root
	if len(root.Members) == 0 {
		t.Fatal("expected at least 1 member in root")
	}
	
	usage, ok := root.Members[0].(*ast.Usage)
	if !ok || usage.Kind != ast.UsageAction {
		t.Fatalf("expected UsageAction, got %T / kind %v", root.Members[0], usage.Kind)
	}
	
	if len(usage.Members) != 3 {
		t.Errorf("expected 3 action members, got %d", len(usage.Members))
	}
}
```

Run: `go test ./internal/core/parser/ -run TestParseUsageAction_Integration -v`  
Expected: PASS

- [ ] **Step 5: Commit integration**

```bash
git add internal/core/parser/defusage.go internal/core/parser/behavior_test.go
git commit -m "feat(parser): hook parseUsageBody for UsageAction

Dispatches to parseActionBody() on UsageAction kind. Integration test
verifies end-to-end: parse action {...} → Members populated."
```

---

### Task 14: parseStateMachineBody() Scaffold

**Files:**
- Modify: `internal/core/parser/behavior.go`

- [ ] **Step 1: Add parseStateMachineBody method**

```go
// parseStateMachineBody parses the body of a state machine usage.
// Expects '{' already consumed, returns list of state nodes + transitions.
func (p *Parser) parseStateMachineBody() []ast.Node {
	var members []ast.Node
	
	for !p.check(lexer.TokenRightBrace) && !p.isAtEnd() {
		members = append(members, p.parseStateMember())
	}
	
	p.consume(lexer.TokenRightBrace, "expected '}' after state machine body")
	return members
}
```

- [ ] **Step 2: Add parseStateMember dispatcher**

```go
// parseStateMember parses one state member: state node or transition.
func (p *Parser) parseStateMember() ast.Node {
	if p.match(lexer.TokenKeyword) {
		kw := p.previous().Lexeme
		switch kw {
		case "state":
			return p.parseStateNode()
		case "initial":
			return p.parseInitialState()
		case "final":
			return p.parseFinalState()
		case "transition":
			return p.parseTransitionEdge()
		case "choice", "junction", "fork", "join", "entry", "exit":
			return p.parsePseudostateNode(kw)
		default:
			return &ast.ErrorNode{
				NodeBase: ast.NodeBase{SpanVal: p.previous().Span},
				Err:      "unknown state keyword: " + kw,
			}
		}
	}
	
	return &ast.ErrorNode{
		NodeBase: ast.NodeBase{SpanVal: p.peek().Span},
		Err:      "expected state node or transition keyword",
	}
}
```

- [ ] **Step 3: Add stub state parsers**

```go
// Stubs — Tasks 15-19 will implement
func (p *Parser) parseStateNode() ast.Node {
	return &ast.StateNode{NodeBase: ast.NodeBase{SpanVal: p.previous().Span}}
}

func (p *Parser) parseInitialState() ast.Node {
	return &ast.StateNode{
		NodeBase:  ast.NodeBase{SpanVal: p.previous().Span},
		IsInitial: true,
	}
}

func (p *Parser) parseFinalState() ast.Node {
	return &ast.StateNode{
		NodeBase: ast.NodeBase{SpanVal: p.previous().Span},
		IsFinal:  true,
	}
}

func (p *Parser) parseTransitionEdge() ast.Node {
	return &ast.TransitionEdge{NodeBase: ast.NodeBase{SpanVal: p.previous().Span}}
}

func (p *Parser) parsePseudostateNode(kind string) ast.Node {
	return &ast.PseudostateNode{NodeBase: ast.NodeBase{SpanVal: p.previous().Span}}
}
```

- [ ] **Step 4: Verify build**

Run: `go build ./internal/core/parser/`  
Expected: SUCCESS

- [ ] **Step 5: Commit scaffold**

```bash
git add internal/core/parser/behavior.go
git commit -m "feat(parser): scaffold parseStateMachineBody + dispatcher

Keyword dispatcher for state nodes (state/initial/final/transition/
choice/junction/fork/join/entry/exit). Stubs return empty nodes.
Tasks 15-19 will implement parsers."
```

---

### Task 15: State Node Parsers

**Files:**
- Modify: `internal/core/parser/behavior.go`

- [ ] **Step 1: Implement parseStateNode**

Replace stub with:

```go
// parseStateNode parses: state name [{ entry/do/exit/substates }] ;
func (p *Parser) parseStateNode() ast.Node {
	start := p.previous().Span
	
	name := p.consume(lexer.TokenIdentifier, "expected state name").Lexeme
	
	state := &ast.StateNode{
		NodeBase:  ast.NodeBase{SpanVal: start},
		Name:      name,
		Entry:     []ast.Node{},
		Do:        []ast.Node{},
		Exit:      []ast.Node{},
		Substates: []ast.Node{},
		Regions:   []*ast.StateRegion{},
	}
	
	if p.match(lexer.TokenLeftBrace) {
		state.Entry, state.Do, state.Exit, state.Substates, state.Regions = p.parseStateBody()
		p.consume(lexer.TokenRightBrace, "expected '}' after state body")
	}
	
	p.consume(lexer.TokenSemicolon, "expected ';' after state")
	state.NodeBase.SpanVal = start.To(p.previous().Span.End)
	return state
}
```

- [ ] **Step 2: Implement parseInitialState**

Replace stub:

```go
// parseInitialState parses: initial state name ;
func (p *Parser) parseInitialState() ast.Node {
	start := p.previous().Span
	
	p.consume(lexer.TokenKeyword, "expected 'state' after 'initial'") // state keyword
	name := p.consume(lexer.TokenIdentifier, "expected state name").Lexeme
	
	p.consume(lexer.TokenSemicolon, "expected ';' after initial state")
	
	return &ast.StateNode{
		NodeBase:  ast.NodeBase{SpanVal: start.To(p.previous().Span.End)},
		Name:      name,
		IsInitial: true,
		Entry:     []ast.Node{},
		Do:        []ast.Node{},
		Exit:      []ast.Node{},
	}
}
```

- [ ] **Step 3: Implement parseFinalState**

Replace stub:

```go
// parseFinalState parses: final state name ;
func (p *Parser) parseFinalState() ast.Node {
	start := p.previous().Span
	
	p.consume(lexer.TokenKeyword, "expected 'state' after 'final'")
	name := p.consume(lexer.TokenIdentifier, "expected state name").Lexeme
	
	p.consume(lexer.TokenSemicolon, "expected ';' after final state")
	
	return &ast.StateNode{
		NodeBase: ast.NodeBase{SpanVal: start.To(p.previous().Span.End)},
		Name:     name,
		IsFinal:  true,
		Entry:    []ast.Node{},
		Do:       []ast.Node{},
		Exit:     []ast.Node{},
	}
}
```

- [ ] **Step 4: Verify build**

Run: `go build ./internal/core/parser/`  
Expected: SUCCESS

- [ ] **Step 5: Commit**

```bash
git add internal/core/parser/behavior.go
git commit -m "feat(parser): implement state node parsers

StateNode (simple/composite), initial state, final state parsers.
parseStateBody() scaffold for Task 16."
```

---

### Task 16: State Behavior Parsers

**Files:**
- Modify: `internal/core/parser/behavior.go`

- [ ] **Step 1: Implement parseStateBody**

```go
// parseStateBody parses entry/do/exit behaviors and substates.
func (p *Parser) parseStateBody() (entry, do, exit, substates []ast.Node, regions []*ast.StateRegion) {
	for !p.check(lexer.TokenRightBrace) && !p.isAtEnd() {
		if p.match(lexer.TokenKeyword) {
			kw := p.previous().Lexeme
			switch kw {
			case "entry":
				entry = append(entry, p.parseStateBehavior())
			case "exit":
				exit = append(exit, p.parseStateBehavior())
			case "do":
				do = append(do, p.parseStateBehavior())
			case "state":
				p.backup() // put 'state' back for parseStateNode
				substates = append(substates, p.parseStateMember())
			case "region":
				regions = append(regions, p.parseStateRegion())
			default:
				p.backup()
				return
			}
		} else {
			break
		}
	}
	return
}
```

**Note:** Add `backup()` helper if not present:

```go
func (p *Parser) backup() {
	if p.current > 0 {
		p.current--
	}
}
```

- [ ] **Step 2: Implement parseStateBehavior**

```go
// parseStateBehavior parses: entry/do/exit { statements } ;
func (p *Parser) parseStateBehavior() ast.Node {
	start := p.previous().Span
	
	p.consume(lexer.TokenLeftBrace, "expected '{' after entry/do/exit")
	
	// Parse action statements (simplified: parse as expressions for now)
	var statements []ast.Node
	for !p.check(lexer.TokenRightBrace) && !p.isAtEnd() {
		statements = append(statements, p.parseExpression())
		p.consume(lexer.TokenSemicolon, "expected ';' after statement")
	}
	
	p.consume(lexer.TokenRightBrace, "expected '}' after behavior")
	
	// Return as SequenceExpr (placeholder for behavior list)
	return &ast.SequenceExpr{
		NodeBase: ast.NodeBase{SpanVal: start.To(p.previous().Span.End)},
		Elements: statements,
	}
}
```

- [ ] **Step 3: Implement parseStateRegion (stub)**

```go
// parseStateRegion parses: region name { states } ; (deferred syntax)
func (p *Parser) parseStateRegion() *ast.StateRegion {
	start := p.previous().Span
	
	name := p.consume(lexer.TokenIdentifier, "expected region name").Lexeme
	
	// Stub: return empty region
	return &ast.StateRegion{
		NodeBase: ast.NodeBase{SpanVal: start},
		Name:     name,
		States:   []ast.Node{},
	}
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/core/parser/behavior.go
git commit -m "feat(parser): implement state behavior parsers

entry/do/exit behavior parsing via parseStateBehavior(). Substates
via recursive parseStateMember(). Region syntax stubbed."
```

---

### Task 17: Transition Edge Parser

**Files:**
- Modify: `internal/core/parser/behavior.go`

- [ ] **Step 1: Implement parseTransitionEdge**

Replace stub:

```go
// parseTransitionEdge parses: transition source -> target [trigger] [if guard] [do effect] ;
func (p *Parser) parseTransitionEdge() ast.Node {
	start := p.previous().Span
	
	source := p.parseQualifiedName()
	p.consume(lexer.TokenArrow, "expected '->' in transition")
	target := p.parseQualifiedName()
	
	var trigger ast.TriggerEvent
	var guard ast.Node
	var effect []ast.Node
	
	// Optional trigger (after/when/accept/on keywords)
	if p.check(lexer.TokenKeyword) {
		kw := p.peek().Lexeme
		if kw == "after" || kw == "when" || kw == "accept" || kw == "on" {
			trigger = p.parseTriggerEvent()
		}
	}
	
	// Optional guard
	if p.match(lexer.TokenKeyword) && p.previous().Lexeme == "if" {
		guard = p.parseExpression()
	}
	
	// Optional effect
	if p.match(lexer.TokenKeyword) && p.previous().Lexeme == "do" {
		p.consume(lexer.TokenLeftBrace, "expected '{' after 'do'")
		for !p.check(lexer.TokenRightBrace) && !p.isAtEnd() {
			effect = append(effect, p.parseExpression())
			p.consume(lexer.TokenSemicolon, "expected ';' after effect statement")
		}
		p.consume(lexer.TokenRightBrace, "expected '}' after effect")
	}
	
	p.consume(lexer.TokenSemicolon, "expected ';' after transition")
	
	return &ast.TransitionEdge{
		NodeBase: ast.NodeBase{SpanVal: start.To(p.previous().Span.End)},
		Source:   source,
		Target:   target,
		Trigger:  trigger,
		Guard:    guard,
		Effect:   effect,
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/core/parser/behavior.go
git commit -m "feat(parser): implement transition edge parser

Full syntax: transition source -> target [trigger] [if guard] [do {...}];
Trigger parsing delegated to Task 18."
```

---

### Task 18: Trigger Event Parsers

**Files:**
- Modify: `internal/core/parser/behavior.go`

- [ ] **Step 1: Implement parseTriggerEvent dispatcher**

```go
// parseTriggerEvent dispatches to specific trigger parsers.
func (p *Parser) parseTriggerEvent() ast.TriggerEvent {
	if !p.match(lexer.TokenKeyword) {
		return nil
	}
	
	kw := p.previous().Lexeme
	switch kw {
	case "after":
		return p.parseTimeEvent()
	case "when":
		return p.parseChangeEvent()
	case "accept":
		return p.parseAcceptEvent()
	case "on":
		return p.parseCallEvent()
	default:
		return nil
	}
}
```

- [ ] **Step 2: Implement parseTimeEvent**

```go
// parseTimeEvent parses: after duration
func (p *Parser) parseTimeEvent() ast.TriggerEvent {
	start := p.previous().Span
	
	duration := p.parseExpression()
	
	return &ast.TimeEvent{
		NodeBase: ast.NodeBase{SpanVal: start.To(p.previous().Span.End)},
		Duration: duration,
	}
}
```

- [ ] **Step 3: Implement parseChangeEvent**

```go
// parseChangeEvent parses: when condition
func (p *Parser) parseChangeEvent() ast.TriggerEvent {
	start := p.previous().Span
	
	condition := p.parseExpression()
	
	return &ast.ChangeEvent{
		NodeBase:  ast.NodeBase{SpanVal: start.To(p.previous().Span.End)},
		Condition: condition,
	}
}
```

- [ ] **Step 4: Implement parseAcceptEvent**

```go
// parseAcceptEvent parses: accept SignalType
func (p *Parser) parseAcceptEvent() ast.TriggerEvent {
	start := p.previous().Span
	
	signalType := p.parseQualifiedName()
	
	return &ast.AcceptEvent{
		NodeBase:   ast.NodeBase{SpanVal: start.To(p.previous().Span.End)},
		SignalType: signalType,
	}
}
```

- [ ] **Step 5: Implement parseCallEvent**

```go
// parseCallEvent parses: on operation
func (p *Parser) parseCallEvent() ast.TriggerEvent {
	start := p.previous().Span
	
	operation := p.parseQualifiedName()
	
	return &ast.CallEvent{
		NodeBase:  ast.NodeBase{SpanVal: start.To(p.previous().Span.End)},
		Operation: operation,
	}
}
```

- [ ] **Step 6: Commit**

```bash
git add internal/core/parser/behavior.go
git commit -m "feat(parser): implement trigger event parsers

TimeEvent (after duration), ChangeEvent (when condition), AcceptEvent
(accept signal), CallEvent (on operation)."
```

---

### Task 19: Pseudostate Parser

**Files:**
- Modify: `internal/core/parser/behavior.go`

- [ ] **Step 1: Implement parsePseudostateNode**

Replace stub:

```go
// parsePseudostateNode parses: choice/junction/fork/join/entry/exit [name] ;
func (p *Parser) parsePseudostateNode(kindStr string) ast.Node {
	start := p.previous().Span
	
	var kind ast.PseudostateKind
	switch kindStr {
	case "choice":
		kind = ast.PseudostateChoice
	case "junction":
		kind = ast.PseudostateJunction
	case "fork":
		kind = ast.PseudostateFork
	case "join":
		kind = ast.PseudostateJoin
	case "entry":
		kind = ast.PseudostateEntry
	case "exit":
		kind = ast.PseudostateExit
	default:
		return &ast.ErrorNode{
			NodeBase: ast.NodeBase{SpanVal: start},
			Err:      "unknown pseudostate kind: " + kindStr,
		}
	}
	
	var name string
	if p.check(lexer.TokenIdentifier) {
		name = p.advance().Lexeme
	}
	
	p.consume(lexer.TokenSemicolon, "expected ';' after pseudostate")
	
	return &ast.PseudostateNode{
		NodeBase: ast.NodeBase{SpanVal: start.To(p.previous().Span.End)},
		Kind:     kind,
		Name:     name,
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/core/parser/behavior.go
git commit -m "feat(parser): implement pseudostate parser

Parses 6 pseudostate kinds (choice/junction/fork/join/entry/exit)
with optional name."
```

---

### Task 20: State Machine Parser Tests

**Files:**
- Modify: `internal/core/parser/behavior_test.go`

- [ ] **Step 1: Add test helper**

```go
func parseStateMachineTest(t *testing.T, input string) []ast.Node {
	src := source.New("test.sysml", []byte(input))
	lex := lexer.New(src)
	p := New(lex)
	return p.parseStateMachineBody()
}
```

- [ ] **Step 2: Test simple state machine**

```go
func TestParseStateMachine_Simple(t *testing.T) {
	input := `{
		initial state Idle;
		state Active;
		final state Done;
	}`
	
	members := parseStateMachineTest(t, input)
	
	if len(members) != 3 {
		t.Fatalf("expected 3 members, got %d", len(members))
	}
	
	idle, ok := members[0].(*ast.StateNode)
	if !ok || !idle.IsInitial {
		t.Errorf("expected initial state, got %T", members[0])
	}
	
	done, ok := members[2].(*ast.StateNode)
	if !ok || !done.IsFinal {
		t.Errorf("expected final state, got %T", members[2])
	}
}
```

- [ ] **Step 3: Test state with behaviors**

```go
func TestParseStateMachine_Behaviors(t *testing.T) {
	input := `{
		state Active {
			entry { initialize(); };
			do { process(); };
			exit { cleanup(); };
		};
	}`
	
	members := parseStateMachineTest(t, input)
	
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	
	state, ok := members[0].(*ast.StateNode)
	if !ok {
		t.Fatalf("expected StateNode, got %T", members[0])
	}
	
	if len(state.Entry) == 0 {
		t.Error("expected entry behavior")
	}
	if len(state.Do) == 0 {
		t.Error("expected do behavior")
	}
	if len(state.Exit) == 0 {
		t.Error("expected exit behavior")
	}
}
```

- [ ] **Step 4: Test transition with trigger**

```go
func TestParseStateMachine_Transition(t *testing.T) {
	input := `{
		transition Idle -> Active after 10;
		transition Active -> Done when ready;
	}`
	
	members := parseStateMachineTest(t, input)
	
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
	
	trans1, ok := members[0].(*ast.TransitionEdge)
	if !ok {
		t.Fatalf("expected TransitionEdge, got %T", members[0])
	}
	if trans1.Trigger == nil {
		t.Error("expected trigger on first transition")
	}
	if _, ok := trans1.Trigger.(*ast.TimeEvent); !ok {
		t.Errorf("expected TimeEvent, got %T", trans1.Trigger)
	}
}
```

- [ ] **Step 5: Commit**

```bash
git add internal/core/parser/behavior_test.go
git commit -m "test(parser): add state machine parser tests

3 tests: simple states, behaviors (entry/do/exit), transitions with
triggers (TimeEvent, ChangeEvent)."
```

---

### Task 21: Hook parseUsageBody() for States

**Files:**
- Modify: `internal/core/parser/defusage.go`

- [ ] **Step 1: Add UsageState case**

Add to `parseUsageBody()` switch:

```go
case ast.UsageState:
	if p.check(lexer.TokenLeftBrace) {
		p.advance() // consume '{'
		usage.Members = p.parseStateMachineBody()
	}
```

- [ ] **Step 2: Add integration test**

In `behavior_test.go`:

```go
func TestParseUsageState_Integration(t *testing.T) {
	input := `state TestStateMachine {
		initial state Idle;
		transition Idle -> Active when ready;
		final state Done;
	}`
	
	src := source.New("test.sysml", []byte(input))
	lex := lexer.New(src)
	p := New(lex)
	root := p.ParseFile()
	
	if len(root.Members) == 0 {
		t.Fatal("expected at least 1 member in root")
	}
	
	usage, ok := root.Members[0].(*ast.Usage)
	if !ok || usage.Kind != ast.UsageState {
		t.Fatalf("expected UsageState, got %T / kind %v", root.Members[0], usage.Kind)
	}
	
	if len(usage.Members) != 3 {
		t.Errorf("expected 3 state machine members, got %d", len(usage.Members))
	}
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/core/parser/defusage.go internal/core/parser/behavior_test.go
git commit -m "feat(parser): hook parseUsageBody for UsageState

Dispatches to parseStateMachineBody() on UsageState kind. Integration
test verifies end-to-end: parse state {...} → Members populated."
```

---

### Task 22: Error Recovery

**Files:**
- Modify: `internal/core/parser/behavior.go`

- [ ] **Step 1: Add synchronize helper**

```go
// synchronize advances past tokens until a safe synchronization point.
func (p *Parser) synchronize() {
	p.advance()
	
	for !p.isAtEnd() {
		if p.previous().Type == lexer.TokenSemicolon {
			return
		}
		
		// Synchronize on behavioral keywords
		if p.peek().Type == lexer.TokenKeyword {
			kw := p.peek().Lexeme
			switch kw {
			case "first", "done", "fork", "join", "merge", "decision", "action", "then",
				"state", "initial", "final", "transition", "entry", "exit", "region":
				return
			}
		}
		
		p.advance()
	}
}
```

- [ ] **Step 2: Wrap parseActionMember with recovery**

Update `parseActionMember()`:

```go
func (p *Parser) parseActionMember() ast.Node {
	// Try parse, recover on error
	defer func() {
		if r := recover(); r != nil {
			p.synchronize()
		}
	}()
	
	// ... existing dispatch logic
}
```

**Note:** Go doesn't use exceptions — this pattern is for illustration. Better approach:

```go
func (p *Parser) parseActionMember() ast.Node {
	if p.match(lexer.TokenKeyword) {
		kw := p.previous().Lexeme
		switch kw {
		case "first":
			node := p.parseInitialNode()
			if _, ok := node.(*ast.ErrorNode); ok {
				p.synchronize()
			}
			return node
		// ... other cases
		default:
			err := &ast.ErrorNode{
				NodeBase: ast.NodeBase{SpanVal: p.previous().Span},
				Err:      "unknown action keyword: " + kw,
			}
			p.synchronize()
			return err
		}
	}
	
	err := &ast.ErrorNode{
		NodeBase: ast.NodeBase{SpanVal: p.peek().Span},
		Err:      "expected action node or edge keyword",
	}
	p.synchronize()
	return err
}
```

- [ ] **Step 3: Apply same pattern to parseStateMember**

Update `parseStateMember()` with error recovery after ErrorNode creation.

- [ ] **Step 4: Add missing name recovery**

In `parseInitialNode()`, if name missing:

```go
var name string
if p.check(lexer.TokenIdentifier) {
	name = p.advance().Lexeme
} else {
	name = "<missing>" // synthetic name for recovery
}
```

Apply to all node parsers (fork, join, state, etc.).

- [ ] **Step 5: Commit**

```bash
git add internal/core/parser/behavior.go
git commit -m "feat(parser): add error recovery for behavioral parsers

synchronize() skips to safe keywords or semicolons. ErrorNode
insertion on malformed nodes. Synthetic names for missing identifiers."
```

---

### Task 23: Integration Tests

**Files:**
- Modify: `internal/core/parser/behavior_test.go`

- [ ] **Step 1: Test complex action**

```go
func TestParseAction_Complex(t *testing.T) {
	input := `action ComplexAction {
		first start;
		fork split;
		action branchA SubActionA;
		action branchB SubActionB;
		join sync;
		decision check;
		then sync check;
		then check successPath if result == true;
		then check failPath if result == false;
		action successPath HandleSuccess;
		action failPath HandleFailure;
		merge converge;
		then successPath converge;
		then failPath converge;
		done end;
	}`
	
	src := source.New("test.sysml", []byte(input))
	lex := lexer.New(src)
	p := New(lex)
	root := p.ParseFile()
	
	if len(root.Members) == 0 {
		t.Fatal("expected at least 1 member")
	}
	
	usage, ok := root.Members[0].(*ast.Usage)
	if !ok || usage.Kind != ast.UsageAction {
		t.Fatalf("expected UsageAction, got %T", root.Members[0])
	}
	
	// Count node types
	var initials, forks, joins, decisions, merges, finals, execs, edges int
	for _, m := range usage.Members {
		switch m.(type) {
		case *ast.InitialNode:
			initials++
		case *ast.ForkNode:
			forks++
		case *ast.JoinNode:
			joins++
		case *ast.DecisionNode:
			decisions++
		case *ast.MergeNode:
			merges++
		case *ast.FinalNode:
			finals++
		case *ast.ActionExecutionNode:
			execs++
		case *ast.SuccessionEdge, *ast.ControlFlowEdge:
			edges++
		}
	}
	
	if initials != 1 || finals != 1 {
		t.Errorf("expected 1 initial and 1 final, got %d/%d", initials, finals)
	}
	if forks != 1 || joins != 1 {
		t.Errorf("expected 1 fork and 1 join, got %d/%d", forks, joins)
	}
	if edges < 5 {
		t.Errorf("expected at least 5 edges, got %d", edges)
	}
}
```

- [ ] **Step 2: Test hierarchical state machine**

```go
func TestParseStateMachine_Hierarchical(t *testing.T) {
	input := `state System {
		initial state Idle;
		state Active {
			entry { start(); };
			initial state Processing;
			state Paused;
			transition Processing -> Paused when pauseRequested;
			transition Paused -> Processing when resumeRequested;
			exit { stop(); };
		};
		transition Idle -> Active when activated;
		transition Active -> Idle when deactivated;
		final state Terminated;
	}`
	
	src := source.New("test.sysml", []byte(input))
	lex := lexer.New(src)
	p := New(lex)
	root := p.ParseFile()
	
	usage, ok := root.Members[0].(*ast.Usage)
	if !ok || usage.Kind != ast.UsageState {
		t.Fatalf("expected UsageState, got %T", root.Members[0])
	}
	
	// Find Active state with substates
	var activeState *ast.StateNode
	for _, m := range usage.Members {
		if s, ok := m.(*ast.StateNode); ok && s.Name == "Active" {
			activeState = s
			break
		}
	}
	
	if activeState == nil {
		t.Fatal("expected Active state not found")
	}
	
	if len(activeState.Entry) == 0 || len(activeState.Exit) == 0 {
		t.Error("expected entry and exit behaviors in Active state")
	}
	
	if len(activeState.Substates) != 2 { // Processing + Paused
		t.Errorf("expected 2 substates in Active, got %d", len(activeState.Substates))
	}
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/core/parser/behavior_test.go
git commit -m "test(parser): add complex integration tests

ComplexAction: fork/join, decision/merge, 5+ edges.
HierarchicalStateMachine: composite state with substates, entry/exit,
nested transitions."
```

---

### Task 24: Error Recovery Tests

**Files:**
- Modify: `internal/core/parser/behavior_test.go`

- [ ] **Step 1: Test missing semicolon**

```go
func TestErrorRecovery_MissingSemicolon(t *testing.T) {
	input := `{
		first start
		done end;
	}`
	
	members := parseActionTest(t, input)
	
	// Should parse despite error (synchronize at 'done')
	if len(members) < 1 {
		t.Fatal("expected at least 1 member after recovery")
	}
}
```

- [ ] **Step 2: Test missing name**

```go
func TestErrorRecovery_MissingName(t *testing.T) {
	input := `{
		fork;
		join sync;
	}`
	
	members := parseActionTest(t, input)
	
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
	
	fork, ok := members[0].(*ast.ForkNode)
	if !ok {
		t.Errorf("expected ForkNode, got %T", members[0])
	}
	if fork.Name == "" {
		// Missing name recovered as empty (or synthetic "<missing>")
		// Either is acceptable
	}
}
```

- [ ] **Step 3: Test unknown keyword**

```go
func TestErrorRecovery_UnknownKeyword(t *testing.T) {
	input := `{
		first start;
		unknown badnode;
		done end;
	}`
	
	members := parseActionTest(t, input)
	
	// Should have 3 members: initial, ErrorNode, final
	if len(members) != 3 {
		t.Errorf("expected 3 members (with ErrorNode), got %d", len(members))
	}
	
	if _, ok := members[1].(*ast.ErrorNode); !ok {
		t.Errorf("expected ErrorNode at index 1, got %T", members[1])
	}
}
```

- [ ] **Step 4: Commit**

```go
git add internal/core/parser/behavior_test.go
git commit -m "test(parser): add error recovery tests

Missing semicolon, missing name (synthetic recovery), unknown keyword
(ErrorNode insertion). All tests verify continued parsing post-error."
```

---

### Task 25: Update AGENTS.md

**Files:**
- Modify: `runtime/AGENTS.md` (§2.4 "The Behavioral Gap")

- [ ] **Step 1: Replace §2.4 content**

Find §2.4 ("The Behavioral Gap — NOT reusable") and replace with:

```markdown
### 2.4 Behavioral AST — READY FOR RUNTIME (Tiers 4–5)

**Status:** Implemented as of 2026-07-31.

`internal/core/ast/behavior.go` models action control-flow graphs and state machines:

**Action nodes (7 types):**
- `InitialNode`, `FinalNode` — entry/termination
- `ForkNode`, `JoinNode` — parallel split/sync
- `MergeNode`, `DecisionNode` — alternative flow merge, conditional branch
- `ActionExecutionNode` — action invocation (reference or inline expression)

**State nodes (3 types):**
- `StateNode` — simple/composite/initial/final, with Entry/Do/Exit behaviors + Substates + Regions
- `StateRegion` — orthogonal regions (parallel states)
- `PseudostateNode` — transient control (choice/junction/fork/join/entry/exit)

**Edges (4 types):**
- `SuccessionEdge` — sequential flow (`then source target;`)
- `ControlFlowEdge` — guarded flow (`then source target if guard;`)
- `ObjectFlowEdge` — data flow (Tier 5 stub)
- `TransitionEdge` — state transition (`transition source -> target [trigger] [if guard] [do {...}];`)

**Triggers (4 types):**
- `TimeEvent` (`after duration`), `ChangeEvent` (`when condition`), `AcceptEvent` (`accept signal`), `CallEvent` (`on operation`)

**Parser integration:**
- `internal/core/parser/behavior.go` — specialized parsers for actions + states
- `parseUsageBody()` hooks in `defusage.go` dispatch UsageAction → `parseActionBody()`, UsageState → `parseStateMachineBody()`
- Lexer gains 20 keywords (first/done/fork/join/merge/decision/then/state/initial/final/entry/exit/transition/after/when/accept/on/choice/junction/region) + TokenArrow (`->`)
- Error recovery: synchronize on keywords/semicolons, ErrorNode insertion, synthetic names

**Runtime implications:**
- **Tier 4** (behavioral interpreter) can now traverse action graphs (initial → fork → join → final) and state machines (states + transitions)
- **Tier 5** (scheduler) can implement token-flow semantics for actions, event-driven stepping for states
- All behavioral nodes populate `Usage.Members` as first-class AST nodes — no special treatment needed

**Grammar:** See `docs/superpowers/specs/2026-07-30-behavioral-ast-parser-extensions.md` Appendix A for full EBNF.

**Examples:** See `testdata/behavioral/` for example models.
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`  
Expected: SUCCESS (no broken links)

- [ ] **Step 3: Commit**

```bash
git add runtime/AGENTS.md
git commit -m "docs(agents): update §2.4 — behavioral AST now implemented

Replaces 'The Behavioral Gap' with status update: 19 AST types
implemented, parser integrated, ready for Tiers 4–5."
```

---

### Task 26: Example Models

**Files:**
- Create: `testdata/behavioral/simple_action.sysml`
- Create: `testdata/behavioral/fork_join_action.sysml`
- Create: `testdata/behavioral/simple_statemachine.sysml`
- Create: `testdata/behavioral/hierarchical_statemachine.sysml`

- [ ] **Step 1: Create simple_action.sysml**

```sysml
// Simple sequential action
action SimpleAction {
	first start;
	action step1 DoWorkA;
	action step2 DoWorkB;
	then start step1;
	then step1 step2;
	then step2 end;
	done end;
}
```

- [ ] **Step 2: Create fork_join_action.sysml**

```sysml
// Parallel action with fork/join
action ParallelAction {
	first start;
	fork split;
	action branchA TaskA;
	action branchB TaskB;
	action branchC TaskC;
	join sync;
	done end;
	
	then start split;
	then split branchA;
	then split branchB;
	then split branchC;
	then branchA sync;
	then branchB sync;
	then branchC sync;
	then sync end;
}
```

- [ ] **Step 3: Create simple_statemachine.sysml**

```sysml
// Simple state machine with transitions
state TrafficLight {
	initial state Red;
	state Yellow;
	state Green;
	
	transition Red -> Green after 30;
	transition Green -> Yellow after 25;
	transition Yellow -> Red after 5;
}
```

- [ ] **Step 4: Create hierarchical_statemachine.sysml**

```sysml
// Hierarchical state machine with composite states
state MediaPlayer {
	initial state Stopped;
	
	state Playing {
		entry { startPlayback(); };
		do { updateProgress(); };
		exit { pausePlayback(); };
		
		initial state NormalSpeed;
		state FastForward;
		state Rewind;
		
		transition NormalSpeed -> FastForward when ffButtonPressed;
		transition NormalSpeed -> Rewind when rwButtonPressed;
		transition FastForward -> NormalSpeed when playButtonPressed;
		transition Rewind -> NormalSpeed when playButtonPressed;
	};
	
	transition Stopped -> Playing when playButtonPressed;
	transition Playing -> Stopped when stopButtonPressed;
	
	final state Terminated;
}
```

- [ ] **Step 5: Verify files parse**

Run: `go run cmd/sysml-lsp/main.go --check testdata/behavioral/*.sysml` (or equivalent parser check)  
Expected: No parse errors (all 4 files valid)

- [ ] **Step 6: Commit examples**

```bash
git add testdata/behavioral/
git commit -m "docs(examples): add behavioral model examples

4 examples: simple action, fork/join, simple state machine, hierarchical
state machine. All parse with behavior.go parser."
```
