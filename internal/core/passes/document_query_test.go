package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func documentQueryDiagnostics(t *testing.T, body string) []Diagnostic {
	t.Helper()
	index := symbols.NewIndex()
	if err := libs.NewLoader(libs.DefaultSource(), nil).LoadAll(index); err != nil {
		t.Fatalf("load standard library: %v", err)
	}
	name := "queries.sysml"
	p := parser.New(source.New(name, []byte(`
package Fixture {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	`+body+`
}`)))
	root := p.ParseFile()
	index.AddDocument(name, root)
	index.ExpandWildcardImports()
	return Analyze(name, root, parserDiagnostics(p), index)
}

func parserDiagnostics(p *parser.Parser) []Diagnostic {
	out := make([]Diagnostic, 0, len(p.Diagnostics))
	for _, diagnostic := range p.Diagnostics {
		out = append(out, Diagnostic{
			Severity: SeverityError,
			Span:     diagnostic.Span,
			Message:  diagnostic.Message,
			Source:   "parser",
		})
	}
	return out
}

func TestDocumentQueryPassAcceptsComposableQuery(t *testing.T) {
	diagnostics := documentQueryDiagnostics(t, `
calc def InterfacesFor :> Query {
	in subsystem : Element;
	Descendants(source = subsystem, maxDepth = 2)
}
calc def APSInterfaces :> Query {
	in subsystem : Element;
	InterfacesFor(subsystem = subsystem)
}
`)
	if len(diagnostics) != 0 {
		t.Fatalf("valid document queries reported diagnostics: %v", diagnostics)
	}
}

func TestDocumentQueryPassReportsCompositionCycle(t *testing.T) {
	diagnostics := documentQueryDiagnostics(t, `
calc def CycleA :> Query { in subsystem : Element; CycleB(subsystem = subsystem) }
calc def CycleB :> Query { in subsystem : Element; CycleA(subsystem = subsystem) }
`)
	var found bool
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "document-query-composition-cycle" {
			found = true
			if diagnostic.Source != "document-query" || diagnostic.Span.Len == 0 {
				t.Fatalf("cycle diagnostic lacks typed source location: %+v", diagnostic)
			}
		}
	}
	if !found {
		t.Fatalf("missing composition-cycle diagnostic: %v", diagnostics)
	}
}
