package repl

import (
	"strings"
	"testing"
)

func TestNewSessionEmpty(t *testing.T) {
	s := NewSession()
	if s == nil {
		t.Fatal("NewSession returned nil")
	}
	if got := s.List(); len(got) != 0 {
		t.Fatalf("new session should have no declarations, got %v", got)
	}
}

func TestAcceptReplacesByName(t *testing.T) {
	s := NewSession()
	s.accept("package P { }")
	s.accept("namespace N;")
	joined := s.accept("package P { } // redefined")
	// P should appear once (the new one); N preserved; order = N then new P.
	if got := s.List(); len(got) != 2 {
		t.Fatalf("want 2 snippets, got %d: %v", len(got), got)
	}
	if !strings.Contains(joined, "redefined") {
		t.Fatalf("joined missing new P: %q", joined)
	}
	if strings.Count(joined, "package P") != 1 {
		t.Fatalf("P not deduplicated: %q", joined)
	}
}

func TestSubmitResolvesAcrossSubmissions(t *testing.T) {
	s := NewSession()
	r1 := s.Submit("package P { }")
	if len(r1.Diagnostics) != 0 {
		t.Fatalf("clean package should have no diags, got %v", r1.Diagnostics)
	}
	if len(r1.Declared) != 1 || r1.Declared[0] != "P" {
		t.Fatalf("want declared [P], got %v", r1.Declared)
	}
	// A later submission referencing an undefined name yields a diagnostic.
	r2 := s.Submit("namespace N { import Missing::X; }")
	if len(r2.Diagnostics) == 0 {
		t.Fatalf("expected unresolved-reference diagnostic")
	}
}
