package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	corequery "github.com/Open-MBEE/OpenSysML/internal/core/query"
)

func TestMetaHelpAndList(t *testing.T) {
	s := NewSession()
	s.Submit("package P { }")
	out, quit, err := s.runMeta("%help")
	if err != nil || quit {
		t.Fatalf("%%help: err=%v quit=%v", err, quit)
	}
	if !strings.Contains(strings.Join(out, "\n"), "%load") {
		t.Errorf("%%help should list commands: %v", out)
	}
	out, _, _ = s.runMeta("%list")
	if !strings.Contains(strings.Join(out, "\n"), "package P") {
		t.Errorf("%%list should show declarations: %v", out)
	}
}

func TestMetaClear(t *testing.T) {
	s := NewSession()
	s.Submit("package P { }")
	if _, _, err := s.runMeta("%clear"); err != nil {
		t.Fatal(err)
	}
	if len(s.List()) != 0 {
		t.Errorf("%%clear should empty session, got %v", s.List())
	}
}

func TestMetaLoad(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "m.sysml")
	if err := os.WriteFile(f, []byte("package Loaded { }"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewSession()
	if _, _, err := s.runMeta("%load " + f); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(s.List(), "\n"), "Loaded") {
		t.Errorf("%%load should submit file contents: %v", s.List())
	}
}

func TestIsMeta(t *testing.T) {
	if !isMeta("%help") || isMeta("package P") || isMeta("") {
		t.Fatal("isMeta classification wrong")
	}
}

func TestQueryAndMetaQuery(t *testing.T) {
	fresh := NewSession()
	if _, err := fresh.Query(`oslc.where=rdf:type="PartUsage"`); err == nil {
		t.Fatal("fresh Session.Query unexpectedly succeeded")
	} else if queryErr, ok := err.(*corequery.Error); !ok || queryErr.Kind != corequery.ErrNoModel {
		t.Fatalf("fresh Session.Query error = %#v, want ErrNoModel", err)
	}
	freshLines, _, err := fresh.runMeta(`%query oslc.where=rdf:type="PartUsage"`)
	if err != nil || len(freshLines) != 1 || !strings.HasPrefix(freshLines[0], "error: no model loaded") {
		t.Fatalf("fresh %%query = lines %v, err %v", freshLines, err)
	}
	broken := NewSession()
	if result := broken.Submit("package Broken {"); len(result.Diagnostics) == 0 {
		t.Fatal("broken model unexpectedly had no diagnostics")
	}
	if _, err := broken.Query(`oslc.where=rdf:type="PartUsage"`); err == nil {
		t.Fatal("query against broken model unexpectedly succeeded")
	} else if queryErr, ok := err.(*corequery.Error); !ok || queryErr.Kind != corequery.ErrNoModel {
		t.Fatalf("broken Session.Query error = %#v, want ErrNoModel", err)
	}
	s := NewSession()
	if result := s.Submit("package Demo { part def Wheel; part wheel : Wheel; }"); len(result.Diagnostics) != 0 {
		t.Fatalf("Submit diagnostics = %v", result.Diagnostics)
	}
	lines, err := s.Query(`oslc.where=rdf:type="PartUsage"&oslc.select=sysml:name`)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != "Demo::wheel  PartUsage  name=wheel" {
		t.Fatalf("Session.Query = %v", lines)
	}
	lines, err = s.Query(`oslc.where=sysml:name="spare"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 0 {
		t.Fatalf("no-match Session.Query = %v", lines)
	}
	lines, _, err = s.runMeta(`%query oslc.where=rdf:type="PartUsage"&oslc.select=sysml:name`)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != "Demo::wheel  PartUsage  name=wheel" {
		t.Fatalf("%%query = %v", lines)
	}
	lines, _, err = s.runMeta(`  %query oslc.where=rdf:type="PartUsage"&oslc.select=sysml:name`)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != "Demo::wheel  PartUsage  name=wheel" {
		t.Fatalf("indented %%query = %v", lines)
	}
	lines, _, err = s.runMeta(`%query oslc.where=rdf:type=`)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "error: ") {
		t.Fatalf("malformed %%query = %v", lines)
	}
}

// A body-expression parameter and a loop- or branch-body declaration exist only
// inside their body, so the scope-tree search backing %eval must not surface them.
func TestLookupInScopeTreeSkipsBodyLocalNames(t *testing.T) {
	s := NewSession()
	s.Submit(`package P {
		action def Sample {
			in attribute samples;
			assert constraint { samples->forAll { in bodyParam; bodyParam > 0 } }
			loop action charging { } until true;
			if true { action thenLocal; } else { action elseLocal; }
		}
	}`)
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		t.Fatal("no document scope")
	}
	if sym, _ := doc.Scope.LookupLocal("P"); sym == nil {
		t.Fatal("package P not in the document scope")
	}
	for _, name := range []string{"bodyParam", "charging", "thenLocal", "elseLocal"} {
		if syms := collectInScopeTree(doc.Scope, name); len(syms) > 0 {
			t.Errorf("%s is body-local and must not be found in the scope tree", name)
		}
	}
	if syms := collectInScopeTree(doc.Scope, "samples"); len(syms) == 0 {
		t.Error("samples is a member of Sample and must still be found")
	}
}
