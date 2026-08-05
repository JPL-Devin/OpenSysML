package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// parseActionTest is a helper that parses an action body from test input.
// Input should be just the body content (excluding 'action name').
func parseActionTest(t *testing.T, input string) []ast.Node {
	src := source.New("test.sysml", []byte(input))
	p := New(src)

	// Consume opening brace (parseActionBody expects it consumed)
	_, ok := p.accept(lexer.LBrace)
	if !ok {
		t.Fatalf("expected '{', got %v", p.peek().Kind)
	}

	return p.parseActionBody()
}

func TestParseAction_Simple(t *testing.T) {
	input := `{
		first startNode;
		done endNode;
	}`

	nodes := parseActionTest(t, input)

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	// Check InitialNode
	initial, ok := nodes[0].(*ast.InitialNode)
	if !ok {
		t.Errorf("node 0: expected *ast.InitialNode, got %T", nodes[0])
	} else {
		if initial.Name != "startNode" {
			t.Errorf("InitialNode.Name: expected 'startNode', got '%s'", initial.Name)
		}
	}

	// Check FinalNode
	final, ok := nodes[1].(*ast.FinalNode)
	if !ok {
		t.Errorf("node 1: expected *ast.FinalNode, got %T", nodes[1])
	} else {
		if final.Name != "endNode" {
			t.Errorf("FinalNode.Name: expected 'endNode', got '%s'", final.Name)
		}
	}
}

func TestParseAction_ForkJoin(t *testing.T) {
	input := `{
		fork split;
		join sync;
	}`

	nodes := parseActionTest(t, input)

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	// Check ForkNode
	forkNode, ok := nodes[0].(*ast.ForkNode)
	if !ok {
		t.Errorf("node 0: expected *ast.ForkNode, got %T", nodes[0])
	} else {
		if forkNode.Name != "split" {
			t.Errorf("ForkNode.Name: expected 'split', got '%s'", forkNode.Name)
		}
	}

	// Check JoinNode
	joinNode, ok := nodes[1].(*ast.JoinNode)
	if !ok {
		t.Errorf("node 1: expected *ast.JoinNode, got %T", nodes[1])
	} else {
		if joinNode.Name != "sync" {
			t.Errorf("JoinNode.Name: expected 'sync', got '%s'", joinNode.Name)
		}
	}
}

func TestParseAction_Decision(t *testing.T) {
	input := `{
		first start;
		decision check;
		done success;
		then start check;
		then check success if true;
	}`

	nodes := parseActionTest(t, input)

	if len(nodes) != 5 {
		t.Fatalf("expected 5 nodes, got %d", len(nodes))
	}

	// Check InitialNode
	initial, ok := nodes[0].(*ast.InitialNode)
	if !ok {
		t.Errorf("node 0: expected *ast.InitialNode, got %T", nodes[0])
	} else {
		if initial.Name != "start" {
			t.Errorf("InitialNode.Name: expected 'start', got '%s'", initial.Name)
		}
	}

	// Check DecisionNode
	decision, ok := nodes[1].(*ast.DecisionNode)
	if !ok {
		t.Errorf("node 1: expected *ast.DecisionNode, got %T", nodes[1])
	} else {
		if decision.Name != "check" {
			t.Errorf("DecisionNode.Name: expected 'check', got '%s'", decision.Name)
		}
	}

	// Check FinalNode
	final, ok := nodes[2].(*ast.FinalNode)
	if !ok {
		t.Errorf("node 2: expected *ast.FinalNode, got %T", nodes[2])
	} else {
		if final.Name != "success" {
			t.Errorf("FinalNode.Name: expected 'success', got '%s'", final.Name)
		}
	}

	// Check SuccessionEdge: then start check
	succEdge, ok := nodes[3].(*ast.SuccessionEdge)
	if !ok {
		t.Errorf("node 3: expected *ast.SuccessionEdge, got %T", nodes[3])
	} else {
		if succEdge.Source == nil {
			t.Errorf("SuccessionEdge.Source is nil")
		} else if len(succEdge.Source.Parts) != 1 || succEdge.Source.Parts[0].Text != "start" {
			t.Errorf("SuccessionEdge.Source: expected 'start', got '%v'", succEdge.Source.Parts)
		}
		if succEdge.Target == nil {
			t.Errorf("SuccessionEdge.Target is nil")
		} else if len(succEdge.Target.Parts) != 1 || succEdge.Target.Parts[0].Text != "check" {
			t.Errorf("SuccessionEdge.Target: expected 'check', got '%v'", succEdge.Target.Parts)
		}
	}

	// Check ControlFlowEdge: then check success if true
	cfEdge, ok := nodes[4].(*ast.ControlFlowEdge)
	if !ok {
		t.Errorf("node 4: expected *ast.ControlFlowEdge, got %T", nodes[4])
	} else {
		if cfEdge.Source == nil {
			t.Errorf("ControlFlowEdge.Source is nil")
		} else if len(cfEdge.Source.Parts) != 1 || cfEdge.Source.Parts[0].Text != "check" {
			t.Errorf("ControlFlowEdge.Source: expected 'check', got '%v'", cfEdge.Source.Parts)
		}
		if cfEdge.Target == nil {
			t.Errorf("ControlFlowEdge.Target is nil")
		} else if len(cfEdge.Target.Parts) != 1 || cfEdge.Target.Parts[0].Text != "success" {
			t.Errorf("ControlFlowEdge.Target: expected 'success', got '%v'", cfEdge.Target.Parts)
		}
		if cfEdge.Guard == nil {
			t.Errorf("ControlFlowEdge.Guard is nil")
		} else if litBool, ok := cfEdge.Guard.(*ast.LiteralBool); !ok {
			t.Errorf("ControlFlowEdge.Guard: expected *ast.LiteralBool, got %T", cfEdge.Guard)
		} else if !litBool.Value {
			t.Errorf("ControlFlowEdge.Guard value: expected true, got %v", litBool.Value)
		}
	}
}

// Phase C1 Tests

func parseResultBodyTest(t *testing.T, input string) []ast.Node {
	src := source.New("test.sysml", []byte(input))
	p := New(src)

	// Consume opening brace
	_, ok := p.accept(lexer.LBrace)
	if !ok {
		t.Fatalf("expected '{', got %v", p.peek().Kind)
	}

	return p.parseResultBody()
}

func parseConstraintBodyTest(t *testing.T, input string) []ast.Node {
	src := source.New("test.sysml", []byte(input))
	p := New(src)

	// Consume opening brace
	_, ok := p.accept(lexer.LBrace)
	if !ok {
		t.Fatalf("expected '{', got %v", p.peek().Kind)
	}

	return p.parseConstraintBody()
}

func TestParseResultBody_Simple(t *testing.T) {
	input := `{
		return x + 5;
	}`

	nodes := parseResultBodyTest(t, input)

	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	result, ok := nodes[0].(*ast.ResultMember)
	if !ok {
		t.Errorf("node 0: expected *ast.ResultMember, got %T", nodes[0])
	} else {
		if result.Expression == nil {
			t.Errorf("ResultMember.Expression is nil")
		}
		// Check it's an operator expression (x + 5)
		if _, ok := result.Expression.(*ast.OperatorExpr); !ok {
			t.Errorf("ResultMember.Expression: expected *ast.OperatorExpr, got %T", result.Expression)
		}
	}
}

func TestParseResultBody_Multiple(t *testing.T) {
	input := `{
		return a;
		return b * 2;
	}`

	nodes := parseResultBodyTest(t, input)

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	for i, node := range nodes {
		if _, ok := node.(*ast.ResultMember); !ok {
			t.Errorf("node %d: expected *ast.ResultMember, got %T", i, node)
		}
	}
}

func TestParseConstraintBody_Assert(t *testing.T) {
	input := `{
		assert x > 0;
	}`

	nodes := parseConstraintBodyTest(t, input)

	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	constraint, ok := nodes[0].(*ast.ConstraintMember)
	if !ok {
		t.Errorf("node 0: expected *ast.ConstraintMember, got %T", nodes[0])
	} else {
		if !constraint.IsAssert {
			t.Errorf("ConstraintMember.IsAssert: expected true, got false")
		}
		if constraint.IsNegated {
			t.Errorf("ConstraintMember.IsNegated: expected false, got true")
		}
		if constraint.Expression == nil {
			t.Errorf("ConstraintMember.Expression is nil")
		}
	}
}

func TestParseConstraintBody_AssertNot(t *testing.T) {
	input := `{
		assert not z < 0;
	}`

	nodes := parseConstraintBodyTest(t, input)

	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	constraint, ok := nodes[0].(*ast.ConstraintMember)
	if !ok {
		t.Errorf("node 0: expected *ast.ConstraintMember, got %T", nodes[0])
	} else {
		if !constraint.IsAssert {
			t.Errorf("ConstraintMember.IsAssert: expected true, got false")
		}
		if !constraint.IsNegated {
			t.Errorf("ConstraintMember.IsNegated: expected true, got false")
		}
		if constraint.Expression == nil {
			t.Errorf("ConstraintMember.Expression is nil")
		}
	}
}

func TestParseConstraintBody_Assume(t *testing.T) {
	input := `{
		assume y != null;
	}`

	nodes := parseConstraintBodyTest(t, input)

	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	constraint, ok := nodes[0].(*ast.ConstraintMember)
	if !ok {
		t.Errorf("node 0: expected *ast.ConstraintMember, got %T", nodes[0])
	} else {
		if constraint.IsAssert {
			t.Errorf("ConstraintMember.IsAssert: expected false, got true")
		}
		if constraint.IsNegated {
			t.Errorf("ConstraintMember.IsNegated: expected false, got true")
		}
		if constraint.Expression == nil {
			t.Errorf("ConstraintMember.Expression is nil")
		}
	}
}

func TestParseConstraintBody_Multiple(t *testing.T) {
	input := `{
		assert x > 0;
		assume y != null;
		assert not z < 0;
	}`

	nodes := parseConstraintBodyTest(t, input)

	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}

	// Check first: assert x > 0
	c1, ok := nodes[0].(*ast.ConstraintMember)
	if !ok {
		t.Errorf("node 0: expected *ast.ConstraintMember, got %T", nodes[0])
	} else {
		if !c1.IsAssert || c1.IsNegated {
			t.Errorf("node 0: expected assert (not negated), got IsAssert=%v IsNegated=%v", c1.IsAssert, c1.IsNegated)
		}
	}

	// Check second: assume y != null
	c2, ok := nodes[1].(*ast.ConstraintMember)
	if !ok {
		t.Errorf("node 1: expected *ast.ConstraintMember, got %T", nodes[1])
	} else {
		if c2.IsAssert || c2.IsNegated {
			t.Errorf("node 1: expected assume (not negated), got IsAssert=%v IsNegated=%v", c2.IsAssert, c2.IsNegated)
		}
	}

	// Check third: assert not z < 0
	c3, ok := nodes[2].(*ast.ConstraintMember)
	if !ok {
		t.Errorf("node 2: expected *ast.ConstraintMember, got %T", nodes[2])
	} else {
		if !c3.IsAssert || !c3.IsNegated {
			t.Errorf("node 2: expected assert not, got IsAssert=%v IsNegated=%v", c3.IsAssert, c3.IsNegated)
		}
	}
}

// Phase C2: Requirement Body Tests

// parseRequirementBodyTest is a helper that parses a requirement body from test input.
func parseRequirementBodyTest(t *testing.T, input string) []ast.Node {
	src := source.New("test.sysml", []byte(input))
	p := New(src)

	// Consume opening brace
	_, ok := p.accept(lexer.LBrace)
	if !ok {
		t.Fatalf("expected '{', got %v", p.peek().Kind)
	}

	return p.parseRequirementBody()
}

func TestParseRequirementBody_Subject(t *testing.T) {
	input := `{
		subject vehicle : Vehicle;
	}`

	nodes := parseRequirementBodyTest(t, input)

	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	subject, ok := nodes[0].(*ast.SubjectMember)
	if !ok {
		t.Errorf("node 0: expected *ast.SubjectMember, got %T", nodes[0])
	} else {
		if subject.Name != "vehicle" {
			t.Errorf("SubjectMember.Name: expected 'vehicle', got '%s'", subject.Name)
		}
		if subject.TypeRef == nil {
			t.Errorf("SubjectMember.TypeRef is nil")
		} else if len(subject.TypeRef.Parts) != 1 || subject.TypeRef.Parts[0].Text != "Vehicle" {
			t.Errorf("SubjectMember.TypeRef: expected 'Vehicle', got %+v", subject.TypeRef.Parts)
		}
	}
}

func TestParseRequirementBody_Assume(t *testing.T) {
	input := `{
		assume x > 0;
	}`

	nodes := parseRequirementBodyTest(t, input)

	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	assume, ok := nodes[0].(*ast.AssumeMember)
	if !ok {
		t.Errorf("node 0: expected *ast.AssumeMember, got %T", nodes[0])
	} else {
		if assume.Expression == nil {
			t.Errorf("AssumeMember.Expression is nil")
		}
	}
}

func TestParseRequirementBody_Require(t *testing.T) {
	input := `{
		require brakes.functional;
	}`

	nodes := parseRequirementBodyTest(t, input)

	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	require, ok := nodes[0].(*ast.RequireMember)
	if !ok {
		t.Errorf("node 0: expected *ast.RequireMember, got %T", nodes[0])
	} else {
		if require.Expression == nil {
			t.Errorf("RequireMember.Expression is nil")
		}
	}
}

func TestParseRequirementBody_Actor(t *testing.T) {
	input := `{
		actor driver : Driver;
	}`

	nodes := parseRequirementBodyTest(t, input)

	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	actor, ok := nodes[0].(*ast.ActorMember)
	if !ok {
		t.Errorf("node 0: expected *ast.ActorMember, got %T", nodes[0])
	} else {
		if actor.Name != "driver" {
			t.Errorf("ActorMember.Name: expected 'driver', got '%s'", actor.Name)
		}
		if actor.TypeRef == nil {
			t.Errorf("ActorMember.TypeRef is nil")
		} else if len(actor.TypeRef.Parts) != 1 || actor.TypeRef.Parts[0].Text != "Driver" {
			t.Errorf("ActorMember.TypeRef: expected 'Driver', got %+v", actor.TypeRef.Parts)
		}
	}
}

func TestParseRequirementBody_Complete(t *testing.T) {
	input := `{
		subject vehicle : Vehicle;
		assume vehicle.speed > 0;
		require vehicle.brakes.functional;
		actor driver : Driver;
	}`

	nodes := parseRequirementBodyTest(t, input)

	if len(nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(nodes))
	}

	// Check subject
	if _, ok := nodes[0].(*ast.SubjectMember); !ok {
		t.Errorf("node 0: expected *ast.SubjectMember, got %T", nodes[0])
	}

	// Check assume
	if _, ok := nodes[1].(*ast.AssumeMember); !ok {
		t.Errorf("node 1: expected *ast.AssumeMember, got %T", nodes[1])
	}

	// Check require
	if _, ok := nodes[2].(*ast.RequireMember); !ok {
		t.Errorf("node 2: expected *ast.RequireMember, got %T", nodes[2])
	}

	// Check actor
	if _, ok := nodes[3].(*ast.ActorMember); !ok {
		t.Errorf("node 3: expected *ast.ActorMember, got %T", nodes[3])
	}
}

// Phase C4: State Body Tests

func parseStateBodyTest(t *testing.T, input string) []ast.Node {
	t.Helper()
	src := source.New("test.sysml", []byte(input))
	p := New(src)

	// Consume opening brace
	_, ok := p.accept(lexer.LBrace)
	if !ok {
		t.Fatalf("expected '{', got %v", p.peek().Kind)
	}

	return p.parseStateBody()
}

func TestParseStateBody_Entry(t *testing.T) {
	input := `{
		entry { action initialize; }
	}`

	nodes := parseStateBodyTest(t, input)

	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	entry, ok := nodes[0].(*ast.EntryMember)
	if !ok {
		t.Errorf("node 0: expected *ast.EntryMember, got %T", nodes[0])
	} else {
		if len(entry.Actions) != 1 {
			t.Errorf("EntryMember.Actions: expected 1 action, got %d", len(entry.Actions))
		}
	}
}

func TestParseStateBody_Do(t *testing.T) {
	input := `{
		do { action process; }
	}`

	nodes := parseStateBodyTest(t, input)

	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	do, ok := nodes[0].(*ast.DoMember)
	if !ok {
		t.Errorf("node 0: expected *ast.DoMember, got %T", nodes[0])
	} else {
		if len(do.Actions) != 1 {
			t.Errorf("DoMember.Actions: expected 1 action, got %d", len(do.Actions))
		}
	}
}

func TestParseStateBody_Exit(t *testing.T) {
	input := `{
		exit { action cleanup; }
	}`

	nodes := parseStateBodyTest(t, input)

	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	exit, ok := nodes[0].(*ast.ExitMember)
	if !ok {
		t.Errorf("node 0: expected *ast.ExitMember, got %T", nodes[0])
	} else {
		if len(exit.Actions) != 1 {
			t.Errorf("ExitMember.Actions: expected 1 action, got %d", len(exit.Actions))
		}
	}
}

func TestParseStateBody_Substate(t *testing.T) {
	input := `{
		state Active;
		state Idle;
	}`

	nodes := parseStateBodyTest(t, input)

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	substate1, ok := nodes[0].(*ast.SubstateMember)
	if !ok {
		t.Errorf("node 0: expected *ast.SubstateMember, got %T", nodes[0])
	} else {
		if substate1.Name != "Active" {
			t.Errorf("SubstateMember.Name: expected 'Active', got '%s'", substate1.Name)
		}
	}

	substate2, ok := nodes[1].(*ast.SubstateMember)
	if !ok {
		t.Errorf("node 1: expected *ast.SubstateMember, got %T", nodes[1])
	} else {
		if substate2.Name != "Idle" {
			t.Errorf("SubstateMember.Name: expected 'Idle', got '%s'", substate2.Name)
		}
	}
}

func TestParseStateBody_Transition(t *testing.T) {
	input := `{
		transition Active to Idle when timeout;
	}`

	nodes := parseStateBodyTest(t, input)

	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	transition, ok := nodes[0].(*ast.TransitionMember)
	if !ok {
		t.Errorf("node 0: expected *ast.TransitionMember, got %T", nodes[0])
	} else {
		if transition.Source == nil {
			t.Errorf("TransitionMember.Source is nil")
		} else if len(transition.Source.Parts) != 1 || transition.Source.Parts[0].Text != "Active" {
			t.Errorf("TransitionMember.Source: expected 'Active', got %+v", transition.Source.Parts)
		}

		if transition.Target == nil {
			t.Errorf("TransitionMember.Target is nil")
		} else if len(transition.Target.Parts) != 1 || transition.Target.Parts[0].Text != "Idle" {
			t.Errorf("TransitionMember.Target: expected 'Idle', got %+v", transition.Target.Parts)
		}

		if transition.Trigger == nil {
			t.Errorf("TransitionMember.Trigger is nil (expected timeout)")
		}
	}
}

// accept at/after/when carry an expression whose role only the introducing
// keyword reveals, so the parser records it as a typed trigger event.
func TestParseStateBody_AcceptTransitionTriggerKinds(t *testing.T) {
	nodes := parseStateBodyTest(t, `{
		accept after 10 then Idle;
		accept at deadline then Idle;
		accept when temp > 5 then Idle;
		accept go then Idle;
	}`)

	if len(nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(nodes))
	}
	triggers := make([]ast.Node, len(nodes))
	for i, n := range nodes {
		trans, ok := n.(*ast.TransitionMember)
		if !ok {
			t.Fatalf("node %d: expected *ast.TransitionMember, got %T", i, n)
		}
		triggers[i] = trans.Trigger
	}

	after, ok := triggers[0].(*ast.TimeEvent)
	if !ok {
		t.Fatalf("`accept after`: expected *ast.TimeEvent, got %T", triggers[0])
	}
	if after.Absolute {
		t.Error("`accept after` should be a relative TimeEvent")
	}
	if after.Duration == nil {
		t.Error("`accept after`: TimeEvent.Duration is nil")
	}

	at, ok := triggers[1].(*ast.TimeEvent)
	if !ok {
		t.Fatalf("`accept at`: expected *ast.TimeEvent, got %T", triggers[1])
	}
	if !at.Absolute {
		t.Error("`accept at` should be an absolute TimeEvent")
	}

	change, ok := triggers[2].(*ast.ChangeEvent)
	if !ok {
		t.Fatalf("`accept when`: expected *ast.ChangeEvent, got %T", triggers[2])
	}
	if change.Condition == nil {
		t.Error("`accept when`: ChangeEvent.Condition is nil")
	}

	if _, isTime := triggers[3].(*ast.TimeEvent); isTime {
		t.Error("`accept <signal>` should not be a TimeEvent")
	}
	if _, isChange := triggers[3].(*ast.ChangeEvent); isChange {
		t.Error("`accept <signal>` should not be a ChangeEvent")
	}
}

func TestParseStateBody_TransitionWithGuardAndEffect(t *testing.T) {
	input := `{
		transition Running to Stopped if ready do { action finalize; };
	}`

	nodes := parseStateBodyTest(t, input)

	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	transition, ok := nodes[0].(*ast.TransitionMember)
	if !ok {
		t.Errorf("node 0: expected *ast.TransitionMember, got %T", nodes[0])
	} else {
		if transition.Guard == nil {
			t.Errorf("TransitionMember.Guard is nil")
		}

		if len(transition.Effect) != 1 {
			t.Errorf("TransitionMember.Effect: expected 1 action, got %d", len(transition.Effect))
		}
	}
}

func TestParseStateBody_Complete(t *testing.T) {
	input := `{
		entry { action initialize; }
		do { action process; }
		exit { action cleanup; }
		state Active;
		state Idle;
		transition Active to Idle when timeout;
	}`

	nodes := parseStateBodyTest(t, input)

	if len(nodes) != 6 {
		t.Fatalf("expected 6 nodes, got %d", len(nodes))
	}

	// Check entry
	if _, ok := nodes[0].(*ast.EntryMember); !ok {
		t.Errorf("node 0: expected *ast.EntryMember, got %T", nodes[0])
	}

	// Check do
	if _, ok := nodes[1].(*ast.DoMember); !ok {
		t.Errorf("node 1: expected *ast.DoMember, got %T", nodes[1])
	}

	// Check exit
	if _, ok := nodes[2].(*ast.ExitMember); !ok {
		t.Errorf("node 2: expected *ast.ExitMember, got %T", nodes[2])
	}

	// Check substates
	if _, ok := nodes[3].(*ast.SubstateMember); !ok {
		t.Errorf("node 3: expected *ast.SubstateMember, got %T", nodes[3])
	}
	if _, ok := nodes[4].(*ast.SubstateMember); !ok {
		t.Errorf("node 4: expected *ast.SubstateMember, got %T", nodes[4])
	}

	// Check transition
	if _, ok := nodes[5].(*ast.TransitionMember); !ok {
		t.Errorf("node 5: expected *ast.TransitionMember, got %T", nodes[5])
	}
}
