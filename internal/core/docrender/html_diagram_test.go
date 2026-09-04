package docrender

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

func renderedFigure(t *testing.T, caption string, rendering *view.Rendering, direction view.Direction) string {
	t.Helper()
	w := &htmlWriter{}
	if err := w.writeFigure("", "d", caption, rendering, direction); err != nil {
		t.Fatalf("writeFigure: %v", err)
	}
	return w.b.String()
}

// TestHTMLDiagramMermaidKinds checks every graph-shaped kind is Mermaid source
// in a figure, drawn in the diagram's direction.
func TestHTMLDiagramMermaidKinds(t *testing.T) {
	for kind, want := range map[view.Kind]string{
		view.KindTree:            "flowchart TD",
		view.KindInterconnection: "flowchart LR",
		view.KindAction:          "flowchart TD",
		view.KindState:           "stateDiagram-v2",
		view.KindSequence:        "sequenceDiagram",
	} {
		got := renderedFigure(t, "", graphRendering(kind), "")
		if !strings.Contains(got, `<pre class="mermaid">`) || !strings.Contains(got, want) {
			t.Errorf("%s: %s", kind, got)
		}
		if !strings.Contains(got, `data-diagram-kind="`+string(kind)+`"`) {
			t.Errorf("%s: kind not carried: %s", kind, got)
		}
	}
	got := renderedFigure(t, "flow of a|b", graphRendering(view.KindTree), view.DirectionRightLeft)
	if !strings.Contains(got, `data-direction="RL"`) || !strings.Contains(got, "flowchart RL") {
		t.Errorf("direction not carried: %s", got)
	}
	if !strings.Contains(got, `<figcaption class="sysml-caption">flow of a|b</figcaption>`) {
		t.Errorf("caption: %s", got)
	}
}

// TestHTMLDiagramTableKind checks a table-kind view renders as a real table,
// keeps its notices as comments, and explains an empty rendering.
func TestHTMLDiagramTableKind(t *testing.T) {
	got := renderedFigure(t, "Masses", &view.Rendering{
		Kind:    view.KindTable,
		Columns: []string{"name", "mass"},
		Rows:    [][]string{{"optics", "8.5"}, {"mount|base", "15"}},
		Notices: []string{"attribute stage --> not projected"},
	}, "")
	for _, want := range []string{
		"<!-- not represented: attribute stage - -> not projected -->",
		`<table class="sysml-table" data-content="table">`,
		`<th scope="col" data-column="mass">mass</th>`,
		`<td class="sysml-cell" data-column="name">mount|base</td>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("table figure lacks %q:\n%s", want, got)
		}
	}
	empty := renderedFigure(t, "", &view.Rendering{Kind: view.KindTable}, "")
	if !strings.Contains(empty, "the view exposes nothing; the rendering is empty") {
		t.Errorf("empty table unexplained: %s", empty)
	}
}

// TestHTMLDiagramErrors checks the typed errors for a diagram with no
// rendering and for a kind no renderer can draw.
func TestHTMLDiagramErrors(t *testing.T) {
	w := &htmlWriter{}
	var typed *Error
	if err := w.writeFigure("", "d", "", nil, ""); !errors.As(err, &typed) || typed.Kind != ErrorMissingRendering {
		t.Fatalf("error = %v, want %s", err, ErrorMissingRendering)
	}
	for _, kind := range []view.Kind{view.KindTextual, view.KindGeometry} {
		err := w.writeFigure("", "d", "", &view.Rendering{Kind: kind}, "")
		if !errors.As(err, &typed) || typed.Kind != ErrorUnrenderableDiagram {
			t.Fatalf("%s: error = %v, want %s", kind, err, ErrorUnrenderableDiagram)
		}
		if !strings.Contains(typed.Error(), "which HTML cannot draw") {
			t.Errorf("%s: message names the wrong backend: %s", kind, typed.Error())
		}
	}
}
