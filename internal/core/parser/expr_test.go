package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

func dumpExpr(t *testing.T, src string) string {
	t.Helper()
	p := newParser(src)
	e := p.ParseExpression()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("diags for %q = %+v", src, p.Diagnostics)
	}
	return strings.TrimSpace(ast.Dump(e))
}

func TestPrecedenceAddMul(t *testing.T) {
	got := dumpExpr(t, "1 + 2 * 3")
	want := strings.Join([]string{
		`(OperatorExpr operator="+"`,
		`  (LiteralInteger value="1")`,
		`  (OperatorExpr operator="*"`,
		`    (LiteralInteger value="2")`,
		`    (LiteralInteger value="3")))`,
	}, "\n")
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestLeftAssoc(t *testing.T) {
	got := dumpExpr(t, "1 - 2 - 3")
	want := strings.Join([]string{
		`(OperatorExpr operator="-"`,
		`  (OperatorExpr operator="-"`,
		`    (LiteralInteger value="1")`,
		`    (LiteralInteger value="2"))`,
		`  (LiteralInteger value="3"))`,
	}, "\n")
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestPowRightAssoc(t *testing.T) {
	got := dumpExpr(t, "2 ** 3 ** 4")
	want := strings.Join([]string{
		`(OperatorExpr operator="**"`,
		`  (LiteralInteger value="2")`,
		`  (OperatorExpr operator="**"`,
		`    (LiteralInteger value="3")`,
		`    (LiteralInteger value="4")))`,
	}, "\n")
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestUnaryNeg(t *testing.T) {
	e := exprOf(t, "-5").(*ast.OperatorExpr)
	if e.Operator != ast.OpNeg || len(e.Operands) != 1 {
		t.Fatalf("e = %+v", e)
	}
}

func TestNotOperator(t *testing.T) {
	e := exprOf(t, "not x").(*ast.OperatorExpr)
	if e.Operator != ast.OpNot {
		t.Fatalf("op = %v", e.Operator)
	}
}

func TestAllExtent(t *testing.T) {
	e := exprOf(t, "all X").(*ast.OperatorExpr)
	if e.Operator != ast.OpAll {
		t.Fatalf("op = %v", e.Operator)
	}
}

func TestConditional(t *testing.T) {
	e := exprOf(t, "if c ? a else b").(*ast.OperatorExpr)
	if e.Operator != ast.OpConditional || len(e.Operands) != 3 {
		t.Fatalf("e = %+v", e)
	}
}

func TestClassificationAs(t *testing.T) {
	e := exprOf(t, "x as Integer").(*ast.OperatorExpr)
	if e.Operator != ast.OpAs || e.TypeRef == nil {
		t.Fatalf("e = %+v", e)
	}
	if e.TypeRef.Parts[0].Text != "Integer" {
		t.Fatalf("typeref = %+v", e.TypeRef)
	}
}

func TestRange(t *testing.T) {
	e := exprOf(t, "1 .. 10").(*ast.OperatorExpr)
	if e.Operator != ast.OpRange || len(e.Operands) != 2 {
		t.Fatalf("e = %+v", e)
	}
}

func TestImplies(t *testing.T) {
	e := exprOf(t, "a implies b").(*ast.OperatorExpr)
	if e.Operator != ast.OpImplies {
		t.Fatalf("op = %v", e.Operator)
	}
}

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

func TestPostfixFeatureChain(t *testing.T) {
	e := exprOf(t, "a.b").(*ast.FeatureChainExpr)
	if e.Member == nil || e.Member.Parts[0].Text != "b" {
		t.Fatalf("member = %+v", e.Member)
	}
	if _, ok := e.Operand.(*ast.FeatureReference); !ok {
		t.Fatalf("operand = %#v", e.Operand)
	}
}

func TestPostfixChainDeep(t *testing.T) {
	// a.b.c  => (chain (chain a b) c)
	e := exprOf(t, "a.b.c").(*ast.FeatureChainExpr)
	if e.Member.Parts[0].Text != "c" {
		t.Fatalf("outer member = %+v", e.Member)
	}
	if _, ok := e.Operand.(*ast.FeatureChainExpr); !ok {
		t.Fatalf("inner = %#v", e.Operand)
	}
}

func TestPostfixIndexHash(t *testing.T) {
	e := exprOf(t, "a#(0)").(*ast.IndexExpr)
	if _, ok := e.Index.(*ast.LiteralInteger); !ok {
		t.Fatalf("index = %#v", e.Index)
	}
}

func TestPostfixIndexBracket(t *testing.T) {
	e := exprOf(t, "a[1]").(*ast.IndexExpr)
	if _, ok := e.Index.(*ast.LiteralInteger); !ok {
		t.Fatalf("index = %#v", e.Index)
	}
}

func TestPostfixInvocationArrow(t *testing.T) {
	e := exprOf(t, "coll->select(x)").(*ast.InvocationExpr)
	if e.Type == nil || e.Type.Parts[0].Text != "select" {
		t.Fatalf("type = %+v", e.Type)
	}
	if e.Operand == nil {
		t.Fatalf("expected receiver operand")
	}
	if len(e.Args) != 1 {
		t.Fatalf("args = %+v", e.Args)
	}
}

func TestPostfixCollect(t *testing.T) {
	e := exprOf(t, "a.{ x }").(*ast.CollectExpr)
	if _, ok := e.Body.(*ast.BodyExpr); !ok {
		t.Fatalf("body = %#v", e.Body)
	}
}

func TestPostfixSelect(t *testing.T) {
	e := exprOf(t, "a.?{ x }").(*ast.SelectExpr)
	if _, ok := e.Body.(*ast.BodyExpr); !ok {
		t.Fatalf("body = %#v", e.Body)
	}
}

func TestPostfixArrowThenChain(t *testing.T) {
	// coll->size().x  => chain( invocation(coll,size), x )
	e := exprOf(t, "coll->size().x").(*ast.FeatureChainExpr)
	if e.Member.Parts[0].Text != "x" {
		t.Fatalf("member = %+v", e.Member)
	}
	if _, ok := e.Operand.(*ast.InvocationExpr); !ok {
		t.Fatalf("operand = %#v", e.Operand)
	}
}
