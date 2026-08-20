package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func analyzeInputs(t *testing.T, name, src string) (*ast.RootNamespace, []Diagnostic, *symbols.Index) {
	t.Helper()
	sf := source.New(name, []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	parseDiags := make([]Diagnostic, 0, len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		parseDiags = append(parseDiags, Diagnostic{
			Severity: SeverityError, Span: d.Span, Message: d.Message,
			Code: "syntax", Source: "syntax",
		})
	}
	idx := symbols.NewIndexFromDoc(name, root)
	return root, parseDiags, idx
}

func TestAnalyzeCleanDocument(t *testing.T) {
	// `namespace` would be reported here: it is KerML notation, so the clean
	// document is a .kerml one.
	root, pd, idx := analyzeInputs(t, "a.kerml", "package P { namespace N; alias A for P::N; }")
	got := Analyze("a.kerml", root, pd, idx)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

func TestAnalyzeReportsNameResolution(t *testing.T) {
	root, pd, idx := analyzeInputs(t, "a.sysml", "package P { alias A for P::Missing; }")
	got := Analyze("a.sysml", root, pd, idx)
	if len(got) != 1 || got[0].Source != "name-resolution" {
		t.Fatalf("got %+v, want one name-resolution diagnostic", got)
	}
}

func TestAnalyzeSortsByOffset(t *testing.T) {
	root, pd, idx := analyzeInputs(t, "a.sysml", "package P { alias A for P::X; alias B for P::Y; }")
	got := Analyze("a.sysml", root, pd, idx)
	if len(got) != 2 {
		t.Fatalf("got %d diagnostics, want 2", len(got))
	}
	if got[0].Span.Offset > got[1].Span.Offset {
		t.Fatalf("diagnostics not sorted by offset: %d then %d", got[0].Span.Offset, got[1].Span.Offset)
	}
}
