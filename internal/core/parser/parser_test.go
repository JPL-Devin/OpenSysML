package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func newParser(src string) *Parser {
	sf := source.New("test", []byte(src))
	return New(sf)
}

func TestPeekSkipsTrivia(t *testing.T) {
	p := newParser("  \n // note\n part")
	tok := p.peek()
	if tok.Kind != lexer.Keyword || tok.KeywordID != "part" {
		t.Fatalf("peek = %+v", tok)
	}
}

func TestAdvanceAdvances(t *testing.T) {
	p := newParser("a b")
	first := p.advance()
	second := p.peek()
	if first.Kind != lexer.Identifier {
		t.Fatalf("first = %+v", first)
	}
	if second.Kind != lexer.Identifier || p.src.Text(second.Span) != "b" {
		t.Fatalf("second = %+v", second)
	}
}

func TestPeekN(t *testing.T) {
	p := newParser("a :: b")
	if p.peek().Kind != lexer.Identifier {
		t.Fatal("peek0")
	}
	if p.peekN(1).Kind != lexer.ColonColon {
		t.Fatalf("peek1 = %+v", p.peekN(1))
	}
	if p.peekN(2).Kind != lexer.Identifier {
		t.Fatalf("peek2 = %+v", p.peekN(2))
	}
}

func TestExpectRecordsDiagnostic(t *testing.T) {
	p := newParser("a")
	p.advance() // consume 'a'
	tok, ok := p.expect(lexer.Semicolon, "expected ';'")
	if ok {
		t.Fatal("expected failure at EOF")
	}
	if len(p.Diagnostics) != 1 || p.Diagnostics[0].Message != "expected ';'" {
		t.Fatalf("diags = %+v", p.Diagnostics)
	}
	if tok.Kind != lexer.EOF {
		t.Fatalf("tok = %+v", tok)
	}
}

func TestAtEOF(t *testing.T) {
	p := newParser("")
	if !p.atEOF() {
		t.Fatal("empty should be EOF")
	}
}
