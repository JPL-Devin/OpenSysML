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
