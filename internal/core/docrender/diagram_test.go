package docrender

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

func graphRendering(kind view.Kind) *view.Rendering {
	return &view.Rendering{
		Kind: kind,
		Roots: []*view.Node{
			{ID: "n0", Kind: "part", Name: "a"},
			{ID: "n1", Kind: "part", Name: "b"},
		},
		Edges: []view.Edge{{From: "n0", To: "n1"}},
	}
}

func renderedDiagram(t *testing.T, caption string, rendering *view.Rendering, direction view.Direction) string {
	t.Helper()
	blocks, err := diagramBlocks("d", caption, rendering, direction)
	if err != nil {
		t.Fatalf("diagramBlocks: %v", err)
	}
	return strings.Join(blocks, "\n\n")
}

func TestDiagramMermaidKinds(t *testing.T) {
	cases := []struct {
		kind view.Kind
		want string
	}{
		{view.KindTree, "flowchart TD"},
		{view.KindInterconnection, "flowchart LR"},
		{view.KindAction, "flowchart TD"},
		{view.KindState, "stateDiagram-v2"},
		{view.KindSequence, "sequenceDiagram"},
	}
	for _, c := range cases {
		got := renderedDiagram(t, "", graphRendering(c.kind), "")
		if !strings.HasPrefix(got, "```mermaid\n") || !strings.HasSuffix(got, "\n```") {
			t.Errorf("%s: not fenced:\n%s", c.kind, got)
		}
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: missing %q:\n%s", c.kind, c.want, got)
		}
	}
}

func TestDiagramDirection(t *testing.T) {
	got := renderedDiagram(t, "", graphRendering(view.KindTree), view.DirectionRightLeft)
	if !strings.Contains(got, "flowchart RL") {
		t.Errorf("tree RL: %s", got)
	}
	got = renderedDiagram(t, "", graphRendering(view.KindInterconnection), view.DirectionTopBottom)
	if !strings.Contains(got, "flowchart TB") {
		t.Errorf("interconnection TB: %s", got)
	}
	got = renderedDiagram(t, "", graphRendering(view.KindState), view.DirectionLeftRight)
	if !strings.Contains(got, "stateDiagram-v2\n  direction LR") {
		t.Errorf("state LR: %s", got)
	}
	got = renderedDiagram(t, "", graphRendering(view.KindState), "")
	if strings.Contains(got, "direction") {
		t.Errorf("undirected state carries a direction: %s", got)
	}
}

func TestDiagramCaption(t *testing.T) {
	got := renderedDiagram(t, "flow of a|b", graphRendering(view.KindTree), "")
	if !strings.HasPrefix(got, "*flow of a\\|b*\n\n```mermaid") {
		t.Errorf("caption: %s", got)
	}
}

func TestDiagramTableKind(t *testing.T) {
	rendering := &view.Rendering{
		Kind:    view.KindTable,
		Columns: []string{"name", "mass"},
		Rows:    [][]string{{"optics", "8.5"}, {"mount|base", "15"}},
	}
	got := renderedDiagram(t, "Masses", rendering, "")
	want := "*Masses*\n\n| name | mass |\n| --- | --- |\n| optics | 8.5 |\n| mount\\|base | 15 |"
	if got != want {
		t.Errorf("table = %q, want %q", got, want)
	}
}

func TestDiagramMissingRendering(t *testing.T) {
	_, err := diagramBlocks("d", "", nil, "")
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != ErrorMissingRendering {
		t.Fatalf("error = %v, want %s", err, ErrorMissingRendering)
	}
}

func TestDiagramUnrenderableKind(t *testing.T) {
	for _, kind := range []view.Kind{view.KindTextual, view.KindGeometry} {
		_, err := diagramBlocks("d", "", &view.Rendering{Kind: kind}, "")
		var typed *Error
		if !errors.As(err, &typed) || typed.Kind != ErrorUnrenderableDiagram {
			t.Fatalf("%s: error = %v, want %s", kind, err, ErrorUnrenderableDiagram)
		}
		if typed.Actual != string(kind) {
			t.Fatalf("%s: actual = %q", kind, typed.Actual)
		}
	}
}

func TestDiagramDeterminism(t *testing.T) {
	first := renderedDiagram(t, "c", graphRendering(view.KindState), view.DirectionLeftRight)
	second := renderedDiagram(t, "c", graphRendering(view.KindState), view.DirectionLeftRight)
	if first != second {
		t.Fatalf("renderings differ:\n%s\n---\n%s", first, second)
	}
}
