package repl

import (
	"github.com/Open-MBEE/Systemica/internal/core/source"
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
	joined, _ := s.accept("package P { } // redefined")
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

// A comment typed on its own line documents the declaration that follows, so
// redeclaring that declaration replaces the comment with it instead of leaving
// stale documentation above whatever is current.
func TestLeadingCommentIsReplacedWithItsDeclaration(t *testing.T) {
	s := NewSession()
	s.accept("// doc for A")
	s.accept("part def A;")
	joined, _ := s.accept("part def A { part y; }")

	if strings.Contains(joined, "doc for A") {
		t.Errorf("stale comment survived the redeclaration: %q", joined)
	}
	if got := len(s.List()); got != 1 {
		t.Errorf("want 1 snippet, got %d: %v", got, s.List())
	}
}

// A comment with nothing after it is still part of the session and still saved.
func TestTrailingCommentIsKept(t *testing.T) {
	s := NewSession()
	s.accept("part def A;")
	joined, _ := s.accept("// thinking out loud")
	if !strings.Contains(joined, "thinking out loud") {
		t.Errorf("comment dropped: %q", joined)
	}
}

// A comment folded into the declaration below it must not shift the line
// numbers of that declaration's diagnostics.
func TestSubmitReportsTheSubmittedLine(t *testing.T) {
	s := NewSession()
	s.Submit("// doc for A")
	res := s.Submit("part def A { part x : Missing; }")
	sf := source.New(docName, []byte(res.Source))
	for _, d := range res.Diagnostics {
		line := sf.Lines().PosAt(d.Span.Offset).Line - res.baseLine() + 1
		if line != 1 {
			t.Errorf("diagnostic reported on line %d of a one-line submission: %s", line, d.Message)
		}
	}
}
