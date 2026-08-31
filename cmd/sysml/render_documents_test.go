package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// linkedModel declares two documents referencing each other's content, so the
// multi-document mode writes a set whose cross-document links resolve.
const linkedModel = `package Reports {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	ref appendixDoc : Appendix;
	ref mainDoc : MainReport;

	part def Appendix :> Document {
		attribute redefines title = "Appendix";
		part overview : Paragraph {
			part back : Ref {
				ref redefines target = mainDoc;
			}
		}
		part tables : Section {
			attribute redefines title = "Detail Tables";
			part body : Paragraph {
				attribute redefines text = "detail";
			}
		}
	}

	part def MainReport :> Document {
		attribute redefines title = "Main Report";
		part intro : Paragraph {
			part see : Ref {
				ref redefines target = appendixDoc.tables;
			}
		}
	}
}
`

// TestRenderDocumentsFlag checks -render-documents writes every document of
// the model as a linked Markdown set into the named directory.
func TestRenderDocumentsFlag(t *testing.T) {
	binary := buildCLI(t)
	dir := filepath.Join(t.TempDir(), "rendered")

	got := check(t, binary, linkedModel, "-render-documents", dir)
	if got.status != 0 {
		t.Fatalf("exit = %d\n%s", got.status, got.output())
	}
	report, err := os.ReadFile(filepath.Join(dir, "Reports-MainReport.md"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !strings.Contains(string(report), "[Detail Tables](Reports-Appendix.md#tables)") {
		t.Errorf("report lacks cross-document link:\n%s", report)
	}
	appendix, err := os.ReadFile(filepath.Join(dir, "Reports-Appendix.md"))
	if err != nil {
		t.Fatalf("read appendix: %v", err)
	}
	if !strings.Contains(string(appendix), `<a id="tables"></a>`) {
		t.Errorf("appendix lacks referenced anchor:\n%s", appendix)
	}
	if !strings.Contains(string(appendix), "[Main Report](Reports-MainReport.md)") {
		t.Errorf("appendix lacks root link:\n%s", appendix)
	}

	// A repeated run reproduces the same bytes: the set is deterministic.
	again := filepath.Join(t.TempDir(), "again")
	if got := check(t, binary, linkedModel, "-render-documents", again); got.status != 0 {
		t.Fatalf("repeat exit = %d\n%s", got.status, got.output())
	}
	for _, name := range []string{"Reports-MainReport.md", "Reports-Appendix.md"} {
		first, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		second, err := os.ReadFile(filepath.Join(again, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(first) != string(second) {
			t.Errorf("%s differs between runs", name)
		}
	}
}

// TestRenderDocumentsNoDocuments checks a model without documents is a
// documented failure rather than an empty directory.
func TestRenderDocumentsNoDocuments(t *testing.T) {
	binary := buildCLI(t)
	got := check(t, binary, "package Empty {}\n", "-render-documents", filepath.Join(t.TempDir(), "rendered"))
	if got.status != 2 || !strings.Contains(got.stderr, "declares no documents") {
		t.Errorf("exit = %d stderr = %q", got.status, got.stderr)
	}
}

// TestRenderDocumentsFlagConflicts checks -render-documents refuses runs
// asking for something else too.
func TestRenderDocumentsFlagConflicts(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, linkedModel, "-render-documents", "rendered", "-render-document", "Reports::MainReport"),
		2, "ask for one per run")
	wantReport(t, check(t, binary, linkedModel, "-render-documents", "rendered", "-render", "SomeView"),
		2, "ask for one per run")
	wantReport(t, check(t, binary, linkedModel, "-render-documents", "rendered", "-o", "out.md"),
		2, "cannot be combined with -output")
	wantReport(t, check(t, binary, linkedModel, "-render-documents", "rendered", "-query", "parts"),
		2, "-query")
	wantReport(t, check(t, binary, linkedModel, "-render-documents", "rendered", "-constraint", "C"),
		2, "check it in its own run")
}
