package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func TestToActionGraphInheritedActionNode(t *testing.T) {
	src := `
		action def Base { action a; }
		action def Derived :> Base { first start then a; }
	`
	derived, scope, _ := inheritedActionDecl(t, src, "Derived")
	graph, err := ToActionGraph(derived, scope)
	if err != nil {
		t.Fatalf("ToActionGraph: %v", err)
	}
	inherited := namedNode(graph, "a")
	if inherited == nil {
		t.Fatal("inherited action node was not collected")
	}
	if edges := graph.Edges[graph.Initial]; len(edges) != 1 || edges[0].Target != inherited {
		t.Fatalf("initial edges = %v, want one edge to inherited a", edges)
	}
}

func TestToActionGraphInheritedActionNodeThroughTwoSpecializations(t *testing.T) {
	src := `
		action def Grand { action a; }
		action def Base :> Grand;
		action def Derived :> Base { first start then a; }
	`
	derived, scope, _ := inheritedActionDecl(t, src, "Derived")
	graph, err := ToActionGraph(derived, scope)
	if err != nil {
		t.Fatalf("ToActionGraph: %v", err)
	}
	if namedNode(graph, "a") == nil {
		t.Fatal("two-level inherited action node was not collected")
	}
}

func TestToActionGraphQualifiedInheritedActionNode(t *testing.T) {
	src := `
		action def Base { action a; }
		action def Derived :> Base { first start then Base::a; }
	`
	derived, scope, _ := inheritedActionDecl(t, src, "Derived")
	graph, err := ToActionGraph(derived, scope)
	if err != nil {
		t.Fatalf("ToActionGraph: %v", err)
	}
	inherited := namedNode(graph, "a")
	if inherited == nil {
		t.Fatal("qualified inherited action node was not collected")
	}
	if edges := graph.Edges[graph.Initial]; len(edges) != 1 || edges[0].Target != inherited {
		t.Fatalf("initial edges = %v, want one edge to qualified inherited a", edges)
	}
}

func TestToActionGraphLocalActionShadowsInheritedNode(t *testing.T) {
	src := `
		action def Base { action a; }
		action def Derived :> Base { action a; first start then a; }
	`
	derived, scope, root := inheritedActionDecl(t, src, "Derived")
	graph, err := ToActionGraph(derived, scope)
	if err != nil {
		t.Fatalf("ToActionGraph: %v", err)
	}
	local := actionMember(t, root, "Derived", "a")
	if namedNode(graph, "a") != local {
		t.Fatalf("a node = %p, want local declaration %p", namedNode(graph, "a"), local)
	}
}

func TestToActionGraphDoesNotAdoptInheritedInitialOrFinal(t *testing.T) {
	src := `
		action def Base {
			first start;
			action a;
			done;
			succession first start then a;
			succession first a then done;
		}
		action def Derived :> Base { first start then a; }
	`
	derived, scope, root := inheritedActionDecl(t, src, "Derived")
	graph, err := ToActionGraph(derived, scope)
	if err != nil {
		t.Fatalf("ToActionGraph: %v", err)
	}
	base := actionDefinition(t, root, "Base")
	baseScope := scope.Parent().ChildFor(base)
	for _, node := range base.Members {
		switch n := unwrapMembership(node).(type) {
		case *ast.InitialNode, *ast.FinalNode:
			for _, graphNode := range graph.Nodes {
				if graphNode == n {
					t.Fatalf("inherited %T became a node of the derived graph", n)
				}
			}
			if baseScope == nil {
				t.Fatal("base body scope missing")
			}
		}
	}
	if _, ok := graph.Initial.(*ast.InitialNode); !ok {
		t.Fatalf("derived initial = %T, want synthesized initial", graph.Initial)
	}
}

func TestToActionGraphInheritedActionNodeCycleTerminates(t *testing.T) {
	src := `
		action def A :> B;
		action def B :> A;
		action def Derived :> A { first start then missing; }
	`
	derived, scope, _ := inheritedActionDecl(t, src, "Derived")
	if _, err := ToActionGraph(derived, scope); err == nil {
		t.Fatal("expected an undefined endpoint error")
	}
}

func inheritedActionDecl(t *testing.T, src, name string) (ast.Node, *symbols.Scope, *ast.RootNamespace) {
	t.Helper()
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc("test.sysml", root)
	doc := idx.DocumentRoot("test.sysml")
	decl := actionDefinition(t, root, name)
	scope := doc.ChildFor(decl)
	if scope == nil {
		t.Fatalf("scope for %s missing", name)
	}
	return decl, scope, root
}

func actionDefinition(t *testing.T, root *ast.RootNamespace, name string) *ast.Definition {
	t.Helper()
	var find func([]ast.Node) *ast.Definition
	find = func(members []ast.Node) *ast.Definition {
		for _, member := range members {
			switch n := unwrapMembership(member).(type) {
			case *ast.Definition:
				if n.Kind == ast.DefAction && n.Ident.Name == name {
					return n
				}
				if found := find(n.Members); found != nil {
					return found
				}
			case *ast.Package:
				if found := find(n.Members); found != nil {
					return found
				}
			}
		}
		return nil
	}
	if decl := find(root.Members); decl != nil {
		return decl
	}
	t.Fatalf("action definition %s not found", name)
	return nil
}

func actionMember(t *testing.T, root *ast.RootNamespace, action, member string) ast.Node {
	t.Helper()
	decl := actionDefinition(t, root, action)
	for _, candidate := range decl.Members {
		if n := unwrapMembership(candidate); getNodeName(n) == member {
			return n
		}
	}
	t.Fatalf("action member %s not found", member)
	return nil
}

func namedNode(graph *ActionGraph, name string) ast.Node {
	for _, node := range graph.Nodes {
		if nodeAnswersTo(node, name) {
			return node
		}
	}
	return nil
}
