package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

func exprOf(t *testing.T, src string) ast.Node {
	t.Helper()
	p := newParser(src)
	e := p.ParseExpression()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("diags for %q = %+v", src, p.Diagnostics)
	}
	return e
}

func TestParseLiteralInteger(t *testing.T) {
	e := exprOf(t, "42")
	lit, ok := e.(*ast.LiteralInteger)
	if !ok || lit.Value != "42" {
		t.Fatalf("e = %#v", e)
	}
}

func TestParseLiteralReal(t *testing.T) {
	if _, ok := exprOf(t, "3.14").(*ast.LiteralReal); !ok {
		t.Fatalf("not a real")
	}
}

func TestParseLiteralBool(t *testing.T) {
	if b := exprOf(t, "true").(*ast.LiteralBool); !b.Value {
		t.Fatalf("expected true")
	}
	if b := exprOf(t, "false").(*ast.LiteralBool); b.Value {
		t.Fatalf("expected false")
	}
}

func TestParseLiteralString(t *testing.T) {
	if s := exprOf(t, `"hi"`).(*ast.LiteralString); s.Value != `"hi"` {
		t.Fatalf("s = %q", s.Value)
	}
}

func TestParseNull(t *testing.T) {
	if _, ok := exprOf(t, "null").(*ast.NullExpr); !ok {
		t.Fatalf("not null")
	}
}

func TestParseFeatureReference(t *testing.T) {
	fr := exprOf(t, "A::B").(*ast.FeatureReference)
	if fr.Name == nil || len(fr.Name.Parts) != 2 {
		t.Fatalf("fr = %+v", fr)
	}
}

func TestParseParenSingle(t *testing.T) {
	// A single parenthesized expression collapses to that expression.
	if _, ok := exprOf(t, "(42)").(*ast.LiteralInteger); !ok {
		t.Fatalf("expected literal inside parens")
	}
}

func TestParseSequence(t *testing.T) {
	seq := exprOf(t, "(1, 2, 3)").(*ast.SequenceExpr)
	if len(seq.Elements) != 3 {
		t.Fatalf("seq = %+v", seq)
	}
}

func TestParseEmptySequence(t *testing.T) {
	seq := exprOf(t, "()").(*ast.SequenceExpr)
	if len(seq.Elements) != 0 {
		t.Fatalf("seq = %+v", seq)
	}
}

func TestParseConstructor(t *testing.T) {
	c := exprOf(t, "new Vehicle(1, 2)").(*ast.ConstructorExpr)
	if c.Type == nil || len(c.Args) != 2 {
		t.Fatalf("c = %+v", c)
	}
}

func TestParseBodyExpr(t *testing.T) {
	b := exprOf(t, "{ in x; x }").(*ast.BodyExpr)
	if len(b.Params) != 1 || b.Params[0].Name != "x" {
		t.Fatalf("params = %+v", b.Params)
	}
	if _, ok := b.Result.(*ast.FeatureReference); !ok {
		t.Fatalf("result = %#v", b.Result)
	}
}

func TestParseInfinity(t *testing.T) {
	if _, ok := exprOf(t, "*").(*ast.LiteralInfinity); !ok {
		t.Fatalf("expected infinity literal")
	}
}
