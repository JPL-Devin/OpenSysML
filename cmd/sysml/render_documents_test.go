package main

import (
	"os"
	"os/exec"
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

// TestRenderDocumentsAcrossFiles checks documents declared in different model
// files render together as one linked set.
func TestRenderDocumentsAcrossFiles(t *testing.T) {
	binary := buildCLI(t)
	models := t.TempDir()
	appendix := filepath.Join(models, "appendix.sysml")
	if err := os.WriteFile(appendix, []byte(`package Appendices {
	private import DocumentQueries::*;
	private import ScalarValues::*;

	part def Appendix :> Document {
		attribute redefines title = "Appendix";
		part tables : Section {
			attribute redefines title = "Detail Tables";
			part body : Paragraph {
				attribute redefines text = "detail";
			}
		}
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(models, "report.sysml")
	if err := os.WriteFile(report, []byte(`package Reports {
	private import DocumentQueries::*;
	private import ScalarValues::*;

	ref appendixDoc : Appendices::Appendix;

	part def MainReport :> Document {
		attribute redefines title = "Main Report";
		part intro : Paragraph {
			part see : Ref {
				ref redefines target = appendixDoc.tables;
			}
		}
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(t.TempDir(), "rendered")
	got := runCommand(t, exec.Command(binary, appendix, report, "-render-documents", dir))
	if got.status != 0 {
		t.Fatalf("exit = %d\n%s", got.status, got.output())
	}
	main, err := os.ReadFile(filepath.Join(dir, "Reports-MainReport.md"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !strings.Contains(string(main), "[Detail Tables](Appendices-Appendix.md#tables)") {
		t.Errorf("report lacks cross-file document link:\n%s", main)
	}
	side, err := os.ReadFile(filepath.Join(dir, "Appendices-Appendix.md"))
	if err != nil {
		t.Fatalf("read appendix: %v", err)
	}
	if !strings.Contains(string(side), `<a id="tables"></a>`) {
		t.Errorf("appendix lacks referenced anchor:\n%s", side)
	}
}

// TestRenderDocumentsAllOrNothing checks a set that cannot be written in full
// leaves the directory as it was, with no staged leftovers.
func TestRenderDocumentsAllOrNothing(t *testing.T) {
	binary := buildCLI(t)
	dir := filepath.Join(t.TempDir(), "rendered")
	if err := os.MkdirAll(filepath.Join(dir, "Reports-MainReport.md"), 0o750); err != nil {
		t.Fatal(err)
	}
	previous := "previous appendix\n"
	if err := os.WriteFile(filepath.Join(dir, "Reports-Appendix.md"), []byte(previous), 0o644); err != nil {
		t.Fatal(err)
	}
	bystander := "not ours\n"
	if err := os.WriteFile(filepath.Join(dir, "Reports-Appendix.md.staged"), []byte(bystander), 0o644); err != nil {
		t.Fatal(err)
	}

	got := check(t, binary, linkedModel, "-render-documents", dir)
	if got.status != 2 || !strings.Contains(got.stderr, "it is a directory") {
		t.Fatalf("exit = %d stderr = %q", got.status, got.stderr)
	}
	appendix, err := os.ReadFile(filepath.Join(dir, "Reports-Appendix.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(appendix) != previous {
		t.Errorf("failed set replaced the existing appendix:\n%s", appendix)
	}
	kept, err := os.ReadFile(filepath.Join(dir, "Reports-Appendix.md.staged"))
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != bystander {
		t.Errorf("failed set touched an unrelated file:\n%s", kept)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".sysml-") {
			t.Errorf("staged leftover %s", entry.Name())
		}
	}
}

// TestRenderDocumentEmitsIncomingAnchors checks a document rendered on its own
// carries the anchors other documents of the workspace link into it.
func TestRenderDocumentEmitsIncomingAnchors(t *testing.T) {
	binary := buildCLI(t)
	got := check(t, binary, linkedModel, "-render-document", "Reports::Appendix")
	if got.status != 0 {
		t.Fatalf("exit = %d\n%s", got.status, got.output())
	}
	if !strings.Contains(got.stdout, `<a id="tables"></a>`) {
		t.Errorf("separately rendered appendix lacks the incoming anchor:\n%s", got.stdout)
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
