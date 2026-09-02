package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// A node's parameters and attributes are lowered as features of the node, so
// each performance of it holds them apart from the enclosing action's features.
func TestActionNodeDeclaresItsOwnFeatures(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute total : Integer = 0;
			first start;
			then action p {
				in a : Integer = 3;
				out v : Integer;
				attribute k : Integer = 1;
				action inner;
				assign v := a;
			}
			then done;
		}
	`)

	p := nodeNamed(t, graph, "p")
	features := graph.Features[p]
	if len(features) != 3 {
		t.Fatalf("p declares %d features, want 3: %+v", len(features), features)
	}
	want := []struct {
		name      string
		direction ast.FeatureDirection
		valued    bool
	}{
		{"a", ast.DirIn, true},
		{"v", ast.DirOut, false},
		{"k", ast.DirNone, true},
	}
	for i, w := range want {
		got := features[i]
		if got.Name != w.name || got.Direction != w.direction || (got.Value != nil) != w.valued {
			t.Errorf("feature %d = %s/%v valued=%v, want %s/%v valued=%v",
				i, got.Name, got.Direction, got.Value != nil, w.name, w.direction, w.valued)
		}
		if got.Node == nil {
			t.Errorf("feature %s carries no declaration", got.Name)
		}
	}
	if len(graph.Bodies[p]) != 1 {
		t.Errorf("p lowered %d statements, want 1", len(graph.Bodies[p]))
	}
}

// A binding connector at a node's pin is lowered to the node and pin it
// addresses, with the other end kept as written; one with a node pin at each end
// is lowered once per end.
func TestActionBindingAtANodePin(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute x : Integer = 5;
			out attribute total : Integer;
			bind add.a = x;
			bind total = add.sum;
			bind add.sum = twice.n;
			bind total = x;
			first start;
			then action add { in a : Integer; out sum : Integer; }
			then action twice { in n : Integer; }
			then done;
		}
	`)

	add := nodeNamed(t, graph, "add")
	twice := nodeNamed(t, graph, "twice")
	if len(graph.Bindings) != 4 {
		t.Fatalf("lowered %d pin bindings, want 4: %+v", len(graph.Bindings), graph.Bindings)
	}
	want := []struct {
		node      ast.Node
		pin       string
		other     string
		otherNode ast.Node
		otherPin  string
	}{
		{add, "a", "x", nil, ""},
		{add, "sum", "total", nil, ""},
		{add, "sum", "twice.n", twice, "n"},
		{twice, "n", "add.sum", add, "sum"},
	}
	for i, w := range want {
		got := graph.Bindings[i]
		if got.Node != w.node || got.Pin != w.pin || FeaturePath(got.Other) != w.other ||
			got.OtherNode != w.otherNode || got.OtherPin != w.otherPin {
			t.Errorf("binding %d = %s.%s = %s (other node %v.%s), want %s = %s (other node %v.%s)",
				i, getNodeName(got.Node), got.Pin, FeaturePath(got.Other), got.OtherNode, got.OtherPin,
				w.pin, w.other, w.otherNode, w.otherPin)
		}
		if got.Decl == nil {
			t.Errorf("binding %d carries no declaration", i)
		}
	}
}

// A binding end naming a node without a pin identifies no feature to bind.
func TestActionBindingAtANodeWithoutAPinIsReported(t *testing.T) {
	_, err := actionGraphErr(t, `
		action test {
			attribute x : Integer = 5;
			bind add = x;
			first start;
			then action add { in a : Integer; }
			then done;
		}
	`)
	if err == nil {
		t.Fatal("binding a node itself lowered without error")
	}
}
