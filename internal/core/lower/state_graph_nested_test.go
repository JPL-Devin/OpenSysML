package lower

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// stateUsageIn returns the first state usage declared in the package in src.
func stateUsageIn(t *testing.T, src string) *ast.Usage {
	t.Helper()
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}

	var found *ast.Usage
	var walk func(members []ast.Node)
	walk = func(members []ast.Node) {
		for _, member := range members {
			if membership, ok := member.(*ast.Membership); ok {
				member = membership.Member
			}
			switch n := member.(type) {
			case *ast.Package:
				walk(n.Members)
			case *ast.Usage:
				if n.Kind == ast.UsageState && found == nil {
					found = n
				}
			}
		}
	}
	walk(root.Members)
	if found == nil {
		t.Fatal("no state usage found")
	}
	return found
}

// A pseudostate declared inside a composite state is part of the graph and knows
// which state owns it, which is what a history pseudostate restores from.
func TestToStateGraph_NestedPseudostateOwner(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				initial start;
				state outer {
					state a;
					state b;
					choice pick;
				}
				start then outer;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}

	pick := graph.Pseudostates["pick"]
	if pick == nil {
		t.Fatal("pseudostate declared inside outer was not collected")
	}
	owner := graph.PseudostateOwner[pick]
	if owner == nil || owner.Name != "outer" {
		t.Fatalf("PseudostateOwner[pick] = %v, want outer", owner)
	}
}

// A pseudostate declared directly in the machine has no owning composite state.
func TestToStateGraph_TopLevelPseudostateHasNoOwner(t *testing.T) {
	graph, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				initial start;
				choice pick;
				state a;
				start then pick;
			}
		}
	`), nil)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}

	pick := graph.Pseudostates["pick"]
	if pick == nil {
		t.Fatal("choice pseudostate not collected")
	}
	if owner, ok := graph.PseudostateOwner[pick]; ok {
		t.Errorf("PseudostateOwner[pick] = %v, want no owner", owner)
	}
}

// Lowered without the name-resolution tier's resolver, nothing else reports an
// endpoint that names no vertex, so lowering reports it here.
func TestToStateGraph_EndpointNamingNoVertexIsReportedWithoutAResolver(t *testing.T) {
	_, err := ToStateGraph(stateUsageIn(t, `
		package test {
			state Machine {
				initial start;
				state busy;
				start then busy;
				transition busy to nowhere;
			}
		}
	`), nil)
	if err == nil {
		t.Fatal("expected an error for an endpoint naming no vertex")
	}
	if got := err.Error(); !strings.Contains(got, "nowhere") {
		t.Errorf("expected the error to name the endpoint, got %q", got)
	}
}
