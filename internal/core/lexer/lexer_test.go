package lexer

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// lex is a test helper: collects all tokens including trivia until EOF.
func lex(t *testing.T, input string) []Token {
	t.Helper()
	sf := source.New("t.sysml", []byte(input))
	lx := New(sf)
	var toks []Token
	for {
		tok := lx.Next()
		toks = append(toks, tok)
		if tok.Kind == EOF {
			return toks
		}
		if len(toks) > 10000 {
			t.Fatal("lexer did not terminate")
		}
	}
}

func TestEmptyInputYieldsEOF(t *testing.T) {
	toks := lex(t, "")
	if len(toks) != 1 || toks[0].Kind != EOF {
		t.Fatalf("empty input toks = %v, want single EOF", toks)
	}
	if toks[0].Span.Offset != 0 {
		t.Fatalf("EOF offset = %d, want 0", toks[0].Span.Offset)
	}
}

func TestEOFIsIdempotent(t *testing.T) {
	sf := source.New("t.sysml", []byte(""))
	lx := New(sf)
	_ = lx.Next()
	if k := lx.Next().Kind; k != EOF {
		t.Fatalf("second Next() after EOF = %v, want EOF", k)
	}
}

// kinds extracts just the Kind sequence for compact assertions.
func kinds(toks []Token) []Kind {
	ks := make([]Kind, len(toks))
	for i, t := range toks {
		ks[i] = t.Kind
	}
	return ks
}

func eq(a, b []Kind) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestWhitespace(t *testing.T) {
	toks := lex(t, "   \t\n ")
	want := []Kind{Whitespace, EOF}
	if !eq(kinds(toks), want) {
		t.Fatalf("kinds = %v, want %v", kinds(toks), want)
	}
	if toks[0].Span.Len != 6 {
		t.Fatalf("ws span len = %d, want 6", toks[0].Span.Len)
	}
}

func TestSLNote(t *testing.T) {
	toks := lex(t, "// hello\n")
	if !eq(kinds(toks), []Kind{SLNote, EOF}) {
		t.Fatalf("kinds = %v", kinds(toks))
	}
}

func TestMLNoteBeatsSLNote(t *testing.T) {
	toks := lex(t, "//* note */")
	if !eq(kinds(toks), []Kind{MLNote, EOF}) {
		t.Fatalf("kinds = %v, want MLNote EOF", kinds(toks))
	}
}

func TestRegularComment(t *testing.T) {
	toks := lex(t, "/* c */")
	if !eq(kinds(toks), []Kind{RegularComment, EOF}) {
		t.Fatalf("kinds = %v, want RegularComment EOF", kinds(toks))
	}
}

func TestUnterminatedBlockComment(t *testing.T) {
	toks := lex(t, "/* open")
	if toks[0].Kind != RegularComment {
		t.Fatalf("first kind = %v, want RegularComment", toks[0].Kind)
	}
	if toks[0].Span.Len != 7 {
		t.Fatalf("span len = %d, want 7 (to EOF)", toks[0].Span.Len)
	}
}
