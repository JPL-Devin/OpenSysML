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
