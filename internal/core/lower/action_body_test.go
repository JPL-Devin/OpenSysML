package lower

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// An action node's statements are lowered in declaration order, and its accept
// parameter is lowered with the type it was declared with, so the executor never
// has to walk the node's members again.
func TestActionBodyLowering(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute total : Integer = 0;
			first start;
			action sender {
				send 5 to reader;
				assign total := 1;
				send "note" to logger;
			}
			action reader accept payload : Warning;
			done end;
			then start sender;
			then sender reader;
			then reader end;
		}
	`)

	sender := nodeNamed(t, graph, "sender")
	body := graph.Bodies[sender]
	if len(body) != 3 {
		t.Fatalf("sender lowered to %d statements, want 3: %#v", len(body), body)
	}

	first, ok := body[0].(Send)
	if !ok {
		t.Fatalf("statement 0 = %T, want Send", body[0])
	}
	if first.Target != "reader" {
		t.Errorf("statement 0 target = %q, want %q", first.Target, "reader")
	}
	if assign, ok := body[1].(Assign); !ok {
		t.Errorf("statement 1 = %T, want Assign", body[1])
	} else if assign.Target != "total" {
		t.Errorf("statement 1 target = %q, want %q", assign.Target, "total")
	}
	if third, ok := body[2].(Send); !ok {
		t.Errorf("statement 2 = %T, want Send", body[2])
	} else if third.Target != "logger" {
		t.Errorf("statement 2 target = %q, want %q", third.Target, "logger")
	}

	reader := nodeNamed(t, graph, "reader")
	accept, ok := graph.Accepts[reader]
	if !ok {
		t.Fatal("reader lowered without an accept")
	}
	if accept.ParamName != "payload" || accept.SignalType != "Warning" {
		t.Errorf("reader accept = %+v, want {payload Warning}", accept)
	}

	if _, ok := graph.Accepts[nodeNamed(t, graph, "sender")]; ok {
		t.Error("sender lowered with an accept it does not declare")
	}
}

func actionGraphFor(t *testing.T, src string) *ActionGraph {
	t.Helper()
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}

	for _, member := range root.Members {
		membership, ok := member.(*ast.Membership)
		if !ok {
			continue
		}
		usage, ok := membership.Member.(*ast.Usage)
		if !ok || usage.Kind != ast.UsageAction {
			continue
		}
		graph, err := ToActionGraph(usage)
		if err != nil {
			t.Fatalf("ToActionGraph: %v", err)
		}
		return graph
	}
	t.Fatal("no action usage found")
	return nil
}

func nodeNamed(t *testing.T, graph *ActionGraph, name string) ast.Node {
	t.Helper()
	for _, node := range graph.Nodes {
		if usage, ok := node.(*ast.Usage); ok && usage.Ident.Name == name {
			return node
		}
	}
	t.Fatalf("node %s not found in graph", name)
	return nil
}
