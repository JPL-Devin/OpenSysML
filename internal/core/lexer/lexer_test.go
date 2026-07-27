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

func TestSLNoteSpanIncludesTerminator(t *testing.T) {
	// "// hi\n" is 6 bytes; SL_NOTE per grammar includes the trailing \n.
	toks := lex(t, "// hi\n")
	if toks[0].Kind != SLNote {
		t.Fatalf("kind = %v, want SLNote", toks[0].Kind)
	}
	if toks[0].Span.Len != 6 {
		t.Fatalf("SLNote span len = %d, want 6", toks[0].Span.Len)
	}
}

func TestSLNoteNoTerminatorAtEOF(t *testing.T) {
	// No trailing newline: span is just the "// hi" (5 bytes).
	toks := lex(t, "// hi")
	if toks[0].Kind != SLNote {
		t.Fatalf("kind = %v, want SLNote", toks[0].Kind)
	}
	if toks[0].Span.Len != 5 {
		t.Fatalf("SLNote span len = %d, want 5", toks[0].Span.Len)
	}
}

func TestRegularCommentSpanLen(t *testing.T) {
	toks := lex(t, "/* c */")
	if toks[0].Span.Len != 7 {
		t.Fatalf("RegularComment span len = %d, want 7", toks[0].Span.Len)
	}
}

func TestMLNoteSpanLen(t *testing.T) {
	toks := lex(t, "//* note */")
	if toks[0].Kind != MLNote {
		t.Fatalf("kind = %v, want MLNote", toks[0].Kind)
	}
	if toks[0].Span.Len != 11 {
		t.Fatalf("MLNote span len = %d, want 11", toks[0].Span.Len)
	}
}

func TestUnterminatedMLNote(t *testing.T) {
	toks := lex(t, "//* open")
	if toks[0].Kind != MLNote {
		t.Fatalf("kind = %v, want MLNote", toks[0].Kind)
	}
	if toks[0].Span.Len != 8 {
		t.Fatalf("MLNote span len = %d, want 8 (to EOF)", toks[0].Span.Len)
	}
}

func TestIdentifier(t *testing.T) {
	toks := lex(t, "Engine _x9 abc")
	want := []Kind{Identifier, Whitespace, Identifier, Whitespace, Identifier, EOF}
	if !eq(kinds(toks), want) {
		t.Fatalf("kinds = %v, want %v", kinds(toks), want)
	}
}

func TestKeyword(t *testing.T) {
	toks := lex(t, "part def package")
	for i, ki := range []int{0, 2, 4} {
		if toks[ki].Kind != Keyword {
			t.Fatalf("token %d kind = %v, want Keyword", i, toks[ki].Kind)
		}
	}
	if toks[0].KeywordID != "part" {
		t.Fatalf("KeywordID = %q, want part", toks[0].KeywordID)
	}
}

func TestKeywordPrefixIsIdentifier(t *testing.T) {
	toks := lex(t, "partial")
	if toks[0].Kind != Identifier {
		t.Fatalf("kind = %v, want Identifier", toks[0].Kind)
	}
}

func TestCRLFWhitespaceIsOneToken(t *testing.T) {
	toks := lex(t, "\r\n\r\n")
	if !eq(kinds(toks), []Kind{Whitespace, EOF}) {
		t.Fatalf("kinds = %v, want Whitespace EOF", kinds(toks))
	}
	if toks[0].Span.Len != 4 {
		t.Fatalf("ws span len = %d, want 4", toks[0].Span.Len)
	}
}

func TestUnrestrictedName(t *testing.T) {
	toks := lex(t, "'my name'")
	if !eq(kinds(toks), []Kind{UnrestrictedName, EOF}) {
		t.Fatalf("kinds = %v", kinds(toks))
	}
	if toks[0].Span.Len != 9 {
		t.Fatalf("span len = %d, want 9", toks[0].Span.Len)
	}
}

func TestUnrestrictedNameWithEscape(t *testing.T) {
	toks := lex(t, `'a\'b'`)
	if !eq(kinds(toks), []Kind{UnrestrictedName, EOF}) {
		t.Fatalf("kinds = %v, want UnrestrictedName EOF", kinds(toks))
	}
	if toks[0].Span.Len != 6 {
		t.Fatalf("span len = %d, want 6", toks[0].Span.Len)
	}
}

func TestUnterminatedUnrestrictedName(t *testing.T) {
	toks := lex(t, "'open\n")
	if toks[0].Kind != Error {
		t.Fatalf("kind = %v, want Error", toks[0].Kind)
	}
}
