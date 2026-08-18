package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// restore un-consumes the tokens an abandoned attempt read, so the next parser
// sees the same token it started at.
func TestRestoreUnconsumesTokens(t *testing.T) {
	p := newParser("a b c d")
	cp := p.checkpoint()
	for i := 0; i < 3; i++ {
		p.advance()
	}
	if got := p.src.Text(p.peek().Span); got != "d" {
		t.Fatalf("before restore, peek = %q, want d", got)
	}
	p.restore(cp)
	if got := p.src.Text(p.peek().Span); got != "a" {
		t.Fatalf("after restore, peek = %q, want a", got)
	}
	for _, want := range []string{"a", "b", "c", "d"} {
		if got := p.src.Text(p.advance().Span); got != want {
			t.Fatalf("re-read = %q, want %q", got, want)
		}
	}
	if !p.atEOF() {
		t.Fatal("want EOF after re-reading every token")
	}
}

// An attempt that peeks further than it consumes keeps that lookahead across
// the restore: rewinding un-consumes tokens without dropping tokens already
// read from the lexer.
func TestRestoreKeepsLookahead(t *testing.T) {
	p := newParser("a b c d e f")
	cp := p.checkpoint()
	p.advance()
	p.advance()
	if got := p.src.Text(p.peekN(3).Span); got != "f" {
		t.Fatalf("peekN(3) = %q, want f", got)
	}
	p.restore(cp)
	for _, want := range []string{"a", "b", "c", "d", "e", "f"} {
		if got := p.src.Text(p.advance().Span); got != want {
			t.Fatalf("re-read = %q, want %q", got, want)
		}
	}
	if !p.atEOF() {
		t.Fatal("want EOF after re-reading every token")
	}
}

// The cursor stops at EOF, so restoring after an attempt that ran off the end
// still re-reads the whole stream.
func TestAdvancePastEOFDoesNotMoveCursor(t *testing.T) {
	p := newParser("a")
	cp := p.checkpoint()
	for i := 0; i < 4; i++ {
		if tok := p.advance(); i > 0 && tok.Kind != lexer.EOF {
			t.Fatalf("advance %d = %+v, want EOF", i, tok)
		}
	}
	if p.pos != 1 {
		t.Fatalf("pos = %d, want 1", p.pos)
	}
	p.restore(cp)
	if got := p.src.Text(p.peek().Span); got != "a" {
		t.Fatalf("peek = %q, want a", got)
	}
}

// restore drops the findings the abandoned attempt reported, and does not
// reach past EOF however many tokens were consumed under the checkpoint.
func TestRestoreDropsAttemptFindings(t *testing.T) {
	p := newParser("a b")
	cp := p.checkpoint()
	p.advance()
	p.advance()
	p.expect(lexer.Semicolon, "expected ';'")
	p.warn(p.peek().Span, "reserved word as a name", codeReservedKeywordName)
	p.restore(cp)
	if len(p.Diagnostics) != 0 || len(p.Warnings) != 0 {
		t.Fatalf("restore kept findings: %v / %v", p.Diagnostics, p.Warnings)
	}
	if got := p.src.Text(p.peek().Span); got != "a" {
		t.Fatalf("peek = %q, want a", got)
	}
}

// A nested-constraint attempt that finds no body rewinds cleanly, so the
// condition is read whole rather than missing the word the attempt consumed.
func TestNestedConstraintAttemptRewinds(t *testing.T) {
	src := "package p { constraint c { assert constraint x > 0; assert 1 > 0; } }"
	p := New(source.New("test", []byte(src)))
	root := p.ParseFile()
	if root == nil {
		t.Fatal("nil root")
	}
	if len(p.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", p.Diagnostics)
	}
	// The rewound word is back in the tree, not swallowed by the attempt.
	if dump := ast.Dump(root); !strings.Contains(dump, `(FeatureReference name="constraint")`) {
		t.Fatalf("condition lost the rewound word:\n%s", dump)
	}
}

// Backtracking never panics, whatever the abandoned attempt consumed — the
// parser-never-fails invariant (AGENTS.md §4).
func TestBacktrackingNeverPanics(t *testing.T) {
	for _, src := range []string{
		"package p { requirement r { assert constraint > 0; } }",
		"package p { requirement r { assert constraint x; } }",
		"package p { requirement r { assert constraint limit > 0; } }",
		"package p { requirement r { assume constraint",
		"package p { constraint c { assert constraint",
		"constraint",
	} {
		p := New(source.New("test", []byte(src)))
		if root := p.ParseFile(); root == nil {
			t.Fatalf("nil root for %q", src)
		}
	}
}
