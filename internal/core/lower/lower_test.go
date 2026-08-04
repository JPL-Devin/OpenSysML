package lower

import (
	"testing"
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestToActionGraph_Simple(t *testing.T) {
	src := `
		action test {
			first start;
			done end;
			then start end;
		}
	`
	
	file := source.New("test.sysml", []byte(src))
	p := parser.New(file)
	root := p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	
	// Find action usage
	var actionUsage *ast.Usage
	for _, member := range root.Members {
		if membership, ok := member.(*ast.Membership); ok {
			if usage, ok := membership.Member.(*ast.Usage); ok && usage.Kind == ast.UsageAction {
				actionUsage = usage
				break
			}
		}
	}
	
	if actionUsage == nil {
		t.Fatal("no action usage found")
	}
	
	graph, err := ToActionGraph(actionUsage)
	if err != nil {
		t.Fatalf("ToActionGraph failed: %v", err)
	}
	
	// Validate graph structure
	if graph.Initial == nil {
		t.Error("graph has no initial node")
	}
	
	if len(graph.Finals) == 0 {
		t.Error("graph has no final nodes")
	}
	
	if len(graph.Nodes) != 2 {
		t.Errorf("expected 2 nodes (initial + final), got %d", len(graph.Nodes))
	}
	
	// Validate edges: initial → final
	edges := graph.Edges[graph.Initial]
	if len(edges) != 1 {
		t.Errorf("initial node should have 1 edge, got %d", len(edges))
	}
}

func TestToStateGraph_Simple(t *testing.T) {
	src := `
		package test {
			state Machine {
				initial start;
				state idle;
				final done;
				
				start then idle;
				idle then done;
			}
		}
	`
	
	file := source.New("test.sysml", []byte(src))
	p := parser.New(file)
	root := p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("diagnostic: %v", d)
		}
		t.Fatalf("parse errors")
	}
	
	// Find state usage in package
	var stateUsage *ast.Usage
	for _, member := range root.Members {
		if membership, ok := member.(*ast.Membership); ok {
			if pkg, ok := membership.Member.(*ast.Package); ok {
				for _, pkgMember := range pkg.Members {
					if pkgMembership, ok := pkgMember.(*ast.Membership); ok {
						if usage, ok := pkgMembership.Member.(*ast.Usage); ok && usage.Kind == ast.UsageState {
							stateUsage = usage
							break
						}
					}
				}
			}
		}
	}
	
	if stateUsage == nil {
		t.Fatal("no state usage found")
	}
	
	graph, err := ToStateGraph(stateUsage)
	if err != nil {
		t.Fatalf("ToStateGraph failed: %v", err)
	}
	
	// Validate graph structure
	if graph.Initial == nil {
		t.Error("graph has no initial state")
	}
	
	if len(graph.States) < 2 {
		t.Errorf("expected at least 2 states, got %d", len(graph.States))
	}
	
	// Find initial state
	initialFound := false
	for _, state := range graph.States {
		if state.IsInitial {
			initialFound = true
			t.Logf("Found initial state: %s", state.Name)
		}
	}
	
	if !initialFound {
		t.Error("no state marked as initial")
	}
}
