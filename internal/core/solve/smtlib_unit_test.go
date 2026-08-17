package solve

import (
	"math/big"
	"strings"
	"testing"
)

// TestWriteRat: a real literal is written exactly — a decimal when the magnitude
// has a terminating one, else the quotient itself, so no rounding enters a script.
func TestWriteRat(t *testing.T) {
	cases := []struct {
		num, den int64
		want     string
	}{
		{0, 1, "0.0"},
		{3, 1, "3.0"},
		{3, 2, "1.5"},
		{5, 4, "1.25"},
		{1, 20, "0.05"},
		{-3, 2, "(- 1.5)"},
		{1, 3, "(/ 1.0 3.0)"},
		{-1, 3, "(- (/ 1.0 3.0))"},
		{25, 1000, "0.025"},
	}
	for _, c := range cases {
		got := writeRat(big.NewRat(c.num, c.den))
		if got != c.want {
			t.Errorf("%d/%d writes as %s, want %s", c.num, c.den, got, c.want)
		}
	}
}

// TestWriteInt: SMT-LIB numerals are non-negative, so a negative one is written
// as a negation.
func TestWriteInt(t *testing.T) {
	if got := writeInt(-7); got != "(- 7)" {
		t.Errorf("-7 writes as %s", got)
	}
	if got := writeInt(7); got != "7" {
		t.Errorf("7 writes as %s", got)
	}
}

// TestSMTSymbol quotes what is not a simple symbol and escapes reversibly, so two
// names never collapse into one symbol.
func TestSMTSymbol(t *testing.T) {
	cases := map[string]string{
		"pressure":     "pressure",
		"test::C::x":   "|test::C::x|",
		"a b":          "|a b|",
		"":             "||",
		"pipe|name":    "|pipe!pname|",
		`back\slash`:   "|back!bslash|",
		"bang!name":    "|bang!!name|",
		"bang!name x":  "|bang!!name x|",
		"pipe!pname":   "|pipe!!pname|",
		"pipe!pname x": "|pipe!!pname x|",
	}
	for name, want := range cases {
		if got := smtSymbol(name); got != want {
			t.Errorf("%q writes as %s, want %s", name, got, want)
		}
	}
	// Quoting is only lexical, so an escaped name must not be left unquoted:
	// |pipe!pname| and pipe!pname would be one symbol.
	for _, pair := range [][2]string{
		{"pipe|name x", "pipe!pname x"},
		{"pipe|name", "pipe!pname"},
		{`back\slash`, "back!bslash"},
	} {
		if smtSymbol(pair[0]) == smtSymbol(pair[1]) {
			t.Errorf("%q and %q write as the same symbol", pair[0], pair[1])
		}
	}
}

// TestWriteTermOperators writes every compound operator, so the script speaks
// SMT-LIB rather than the source notation.
func TestWriteTermOperators(t *testing.T) {
	x := VarTerm(&Var{Name: "x", Sort: Int})
	b := VarTerm(&Var{Name: "b", Sort: Bool})
	cases := []struct {
		term *Term
		want string
	}{
		{Not(b), "(not b)"},
		{And(b, BoolTerm(true)), "(and b true)"},
		{Or(b, BoolTerm(false)), "(or b false)"},
		{Binary(OpXor, Bool, b, b), "(xor b b)"},
		{Binary(OpImplies, Bool, b, b), "(=> b b)"},
		{Binary(OpEq, Bool, x, IntTerm(1)), "(= x 1)"},
		{Binary(OpNe, Bool, x, IntTerm(1)), "(distinct x 1)"},
		{Binary(OpLt, Bool, x, IntTerm(1)), "(< x 1)"},
		{Binary(OpLe, Bool, x, IntTerm(1)), "(<= x 1)"},
		{Binary(OpGt, Bool, x, IntTerm(1)), "(> x 1)"},
		{Binary(OpGe, Bool, x, IntTerm(1)), "(>= x 1)"},
		{Binary(OpAdd, Int, x, IntTerm(2)), "(+ x 2)"},
		{Binary(OpSub, Int, x, IntTerm(2)), "(- x 2)"},
		{Binary(OpMul, Int, x, IntTerm(2)), "(* x 2)"},
		{Binary(OpDiv, Real, ToReal(x), RealTerm(big.NewRat(1, 2))), "(/ (to_real x) 0.5)"},
		{Unary(OpNeg, Int, x), "(- x)"},
		{Ite(b, x, IntTerm(0)), "(ite b x 0)"},
		{StringTerm(`say "hi"`), `"say ""hi"""`},
		{ValueTerm(Sort{Kind: SortDatatype, Name: "test::Finish"}, "test::Finish::matte"), "|test::Finish::matte|"},
	}
	for _, c := range cases {
		if got := writeTerm(c.term); got != c.want {
			t.Errorf("term writes as %s, want %s", got, c.want)
		}
	}
}

// TestTermConstructorsFold: a double negation and a singleton junction fold, and
// widening a real or an integer literal needs no conversion term.
func TestTermConstructorsFold(t *testing.T) {
	b := VarTerm(&Var{Name: "b", Sort: Bool})
	if got := Not(Not(b)); got != b {
		t.Error("a double negation does not fold")
	}
	if got := And(b); got != b {
		t.Error("a one-term conjunction does not fold")
	}
	if got := And(); got.Op != OpBool || !got.Bool {
		t.Error("an empty conjunction is not true")
	}
	if got := Or(); got.Op != OpBool || got.Bool {
		t.Error("an empty disjunction is not false")
	}
	real := RealTerm(big.NewRat(1, 2))
	if got := ToReal(real); got != real {
		t.Error("widening a real does not leave it alone")
	}
	if got := ToReal(IntTerm(3)); got.Op != OpReal || got.Real.Cmp(big.NewRat(3, 1)) != 0 {
		t.Error("widening an integer literal does not fold to a real literal")
	}
}

// TestLogicSelection names the logic from the sorts and operators used, so a
// solver is not asked for more theory than the script needs.
func TestLogicSelection(t *testing.T) {
	logic := func(vars []*Var, nonlinear bool, terms ...*Term) string {
		q := &Query{Vars: vars, Nonlinear: nonlinear}
		for _, term := range terms {
			q.Assertions = append(q.Assertions, Assertion{Term: term})
		}
		return q.Logic()
	}
	boolVar := &Var{Name: "b", Sort: Bool}
	intVar := &Var{Name: "i", Sort: Int}
	realVar := &Var{Name: "r", Sort: Real}
	strVar := &Var{Name: "s", Sort: String}
	dtVar := &Var{Name: "d", Sort: Sort{Kind: SortDatatype, Name: "test::Finish"}}

	cases := []struct {
		got, want string
	}{
		{logic([]*Var{boolVar}, false, VarTerm(boolVar)), "QF_UF"},
		{logic([]*Var{intVar}, false, Binary(OpGe, Bool, VarTerm(intVar), IntTerm(0))), "QF_LIA"},
		{logic([]*Var{realVar}, false, Binary(OpGe, Bool, VarTerm(realVar), RealTerm(big.NewRat(1, 2)))), "QF_LRA"},
		{logic([]*Var{intVar, realVar}, false, VarTerm(intVar), VarTerm(realVar)), "QF_LIRA"},
		{logic([]*Var{intVar}, true, VarTerm(intVar)), "QF_NIA"},
		{logic([]*Var{realVar}, true, VarTerm(realVar)), "QF_NRA"},
		{logic([]*Var{intVar, realVar}, true, VarTerm(intVar), VarTerm(realVar)), "QF_NIRA"},
		{logic([]*Var{strVar}, false, VarTerm(strVar)), "ALL"},
		{logic([]*Var{dtVar}, false, VarTerm(dtVar)), "ALL"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("logic is %s, want %s", c.got, c.want)
		}
	}
}

// TestScriptShape: a script sets its logic, declares each datatype and variable
// once, comments every assertion, and ends by asking for satisfiability.
func TestScriptShape(t *testing.T) {
	finish := Sort{
		Kind: SortDatatype, Name: "test::Finish", Origin: "test::Finish",
		Values: []string{"test::Finish::matte", "test::Finish::polished"},
	}
	choice := &Var{Name: "test::ring::finish", Sort: finish, Location: "ring.sysml:4:3"}
	level := &Var{Name: "test::ring::level", Sort: Real, Dimension: "m", Location: "ring.sysml:5:3"}
	q := &Query{
		Kind:    "constraint",
		Element: "Polished",
		Sorts:   []Sort{finish},
		Vars:    []*Var{choice, level},
		Assertions: []Assertion{{
			Term: Binary(OpEq, Bool, VarTerm(choice), ValueTerm(finish, "test::Finish::matte")),
			From: Provenance{Kind: "constraint", Element: "Polished", Condition: "finish == Finish::matte", Role: RoleRequired, Location: "ring.sysml:7:3"},
		}},
	}
	want := strings.Join([]string{
		"; Systemica SMT-LIB2 translation of constraint Polished",
		"; the runtime evaluator remains normative; solving is an optional extension",
		"(set-logic ALL)",
		"; |test::Finish| of test::Finish",
		"(declare-datatypes ((|test::Finish| 0)) (((|test::Finish::matte|) (|test::Finish::polished|))))",
		"; test::ring::finish, declared at ring.sysml:4:3",
		"(declare-const |test::ring::finish| |test::Finish|)",
		"; test::ring::level in base units of m, declared at ring.sysml:5:3",
		"(declare-const |test::ring::level| Real)",
		"; required condition: finish == Finish::matte — constraint Polished, at ring.sysml:7:3",
		"(assert (= |test::ring::finish| |test::Finish::matte|))",
		"(check-sat)",
		"",
	}, "\n")
	if got := Script(q); got != want {
		t.Errorf("script is:\n%s\nwant:\n%s", got, want)
	}
}

// TestScriptCommentsStayOnOneLine: a condition written across lines still
// comments one line, so the script stays parsable.
func TestScriptCommentsStayOnOneLine(t *testing.T) {
	q := &Query{
		Kind:    "constraint",
		Element: "Multi",
		Assertions: []Assertion{{
			Term: BoolTerm(true),
			From: Provenance{Kind: "constraint", Element: "Multi", Condition: "a\n&& b", Role: RoleRequired},
		}},
	}
	for _, line := range strings.Split(strings.TrimSuffix(Script(q), "\n"), "\n") {
		if strings.HasPrefix(line, ";") && strings.Contains(line, "\n") {
			t.Errorf("comment spans lines: %q", line)
		}
		if !strings.HasPrefix(line, ";") && strings.Contains(line, "&& b") {
			t.Errorf("condition text escaped its comment: %q", line)
		}
	}
	if !strings.Contains(Script(q), "; required condition: a && b") {
		t.Errorf("condition is not commented on one line:\n%s", Script(q))
	}
}
