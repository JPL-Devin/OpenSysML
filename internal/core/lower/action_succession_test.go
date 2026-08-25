package lower

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestToActionGraph_ExplicitSuccessionUsage(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			action compute;
			succession first start then compute;
			succession first compute then done;
		}
	`)

	start := namedActionNode(t, graph, "start")
	compute := nodeNamed(t, graph, "compute")
	done := namedActionNode(t, graph, "done")

	if _, ok := start.(*ast.InitialNode); !ok {
		t.Fatalf("start node = %T, want implied initial node", start)
	}
	if _, ok := done.(*ast.FinalNode); !ok {
		t.Fatalf("done node = %T, want implied final node", done)
	}
	if edges := graph.Edges[start]; len(edges) != 1 || edges[0] != compute {
		t.Fatalf("start edges = %v, want [compute]", edges)
	}
	if edges := graph.Edges[compute]; len(edges) != 1 || edges[0] != done {
		t.Fatalf("compute edges = %v, want [done]", edges)
	}
	if _, ok := graph.Successions[start][compute].(*ast.Usage); !ok {
		t.Error("explicit succession was not recorded as its Usage declaration")
	}
}

func TestToActionGraph_ExplicitSuccessionFeatureChain(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			action source {
				attribute pin : Integer = 0;
			}
			action target;
			succession first source.pin then target;
		}
	`)

	source := nodeNamed(t, graph, "source")
	target := nodeNamed(t, graph, "target")
	if edges := graph.Edges[source]; len(edges) != 1 || edges[0] != target {
		t.Fatalf("source edges = %v, want [target]", edges)
	}
}

func TestToActionGraph_ExplicitSuccessionWithoutInitial(t *testing.T) {
	graph := actionGraphFor(t, `
		action seq {
			action alpha;
			action beta;
			succession first alpha then beta;
		}
	`)

	if graph.Initial != nil {
		t.Fatalf("initial node = %v, want no initial node", graph.Initial)
	}
}

func TestToActionGraph_ExplicitSuccessionUndefinedEndpoint(t *testing.T) {
	p := parser.New(source.New("test.sysml", []byte(`
		action seq {
			action compute;
			succession first missing then compute;
		}
	`)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	action := root.Members[0].(*ast.Membership).Member.(*ast.Usage)
	_, err := ToActionGraph(action, nil)
	if err == nil || !strings.Contains(err.Error(), "action succession references undefined source node") {
		t.Fatalf("error = %v, want an explicit succession source diagnostic", err)
	}
}

func TestToActionGraph_ExplicitSuccessionUnsupportedMultiplicity(t *testing.T) {
	p := parser.New(source.New("test.sysml", []byte(`
		action seq {
			action first;
			action second;
			succession [1] first first then second;
		}
	`)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	action := root.Members[0].(*ast.Membership).Member.(*ast.Usage)
	_, err := ToActionGraph(action, nil)
	if err == nil || !strings.Contains(err.Error(), "action succession has unsupported multiplicity") {
		t.Fatalf("error = %v, want an explicit succession multiplicity diagnostic", err)
	}
}

func namedActionNode(t *testing.T, graph *ActionGraph, name string) ast.Node {
	t.Helper()
	for _, node := range graph.Nodes {
		if getNodeName(node) == name {
			return node
		}
	}
	t.Fatalf("node %s not found in graph", name)
	return nil
}
