package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

// A body-expression parameter and a loop-body declaration exist only inside
// their body, so the scope-tree search backing %eval must not surface them.
func TestLookupInScopeTreeSkipsBodyLocalNames(t *testing.T) {
	s := NewSession()
	s.Submit(`package P {
		action def Sample {
			in attribute samples;
			assert constraint { samples->forAll { in bodyParam; bodyParam > 0 } }
			loop action charging { } until true;
		}
	}`)
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		t.Fatal("no document scope")
	}
	if sym, _ := doc.Scope.LookupLocal("P"); sym == nil {
		t.Fatal("package P not in the document scope")
	}
	for _, name := range []string{"bodyParam", "charging"} {
		if sym := lookupInScopeTree(doc.Scope, name); sym != nil {
			t.Errorf("%s is body-local and must not be found in the scope tree", name)
		}
	}
	if sym := lookupInScopeTree(doc.Scope, "samples"); sym == nil {
		t.Error("samples is a member of Sample and must still be found")
	}
}
