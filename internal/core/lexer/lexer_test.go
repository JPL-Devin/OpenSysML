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
