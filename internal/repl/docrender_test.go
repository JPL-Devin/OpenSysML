package repl

import (
	"strings"
	"testing"
)

// docRenderModel declares a document over a small part tree: a titled report
// with a query-backed table and a numbered list.
const docRenderModel = docQueryModel + `package Reports {
	private import DocumentQueries::*;
	private import Observatory::*;

	part def MassReport :> Document {
		attribute redefines title = "Telescope Mass Report";

		part intro : Paragraph {
			attribute redefines text = "Mass rollup for the telescope assembly.";
		}

		part breakdown : Section {
			attribute redefines title = "Heavy Subsystems";

			part masses : Table {
				attribute redefines caption = "Heavy subsystems by mass";
				calc rows : HeavySubsystems {
					in root = telescope;
				}
			}

			part items : List {
				attribute redefines style = "number";
				calc entries : HeavySubsystems {
					in root = telescope;
				}
			}
		}
	}
}
`

func docRenderSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	if res := s.Submit(docRenderModel); len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("model did not analyse cleanly: %v", res.Diagnostics)
	}
	return s
}

func TestRenderDocumentPrintsMarkdown(t *testing.T) {
	s := docRenderSession(t)
	got := run(t, s, "%render-document Reports::MassReport")
	wants(t, got,
		"# Telescope Mass Report",
		"Mass rollup for the telescope assembly.",
		"## Heavy Subsystems",
		"<!-- caption -->\n*Heavy subsystems by mass*",
		"| name | mass |",
		"| --- | --- |",
		"| mount | 15 |",
		"| segmentControl | 20 |",
	)
	if strings.Index(got, "mount") > strings.Index(got, "segmentControl") {
		t.Errorf("rows are not in the query's order:\n%s", got)
	}
}

func TestRenderDocumentUsageAndErrors(t *testing.T) {
	s := docRenderSession(t)
	wants(t, run(t, s, "%render-document"), renderDocumentUsage)
	wants(t, run(t, s, "%render-document NoSuchDocument"), "error:", "NoSuchDocument")
	wants(t, run(t, s, "%render-document Observatory::HeavySubsystems"),
		"error:", "not a document", "DocumentQueries::Document")
	wants(t, run(t, s, "%render-document Reports::MassReport root=telescope"),
		"error:", "binds its queries' parameters in the model")
}

func TestRenderDocumentMarkdownAPI(t *testing.T) {
	s := docRenderSession(t)
	markdown, err := s.RenderDocumentMarkdown("Reports::MassReport")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.HasPrefix(markdown, "# Telescope Mass Report\n") {
		t.Errorf("markdown does not open with the title heading:\n%s", markdown)
	}
	if !strings.HasSuffix(markdown, "\n") {
		t.Errorf("markdown does not end with a newline")
	}
}

func TestRenderDocumentListedInHelpAndCompletion(t *testing.T) {
	s := docRenderSession(t)
	wants(t, run(t, s, "%help"), "%render-document <name>")
	comp := s.Complete("%render-doc", len("%render-doc"))
	found := false
	for _, cand := range comp.Candidates {
		if cand == "%render-document" {
			found = true
		}
	}
	if !found {
		t.Errorf("%%render-document is not completed: %v", comp.Candidates)
	}
}
