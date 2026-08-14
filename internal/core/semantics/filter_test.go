package semantics

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// filterOf parses cond as a filter condition written at the document root of
// src, and returns the model, the condition, and the scope it is written in.
func filterOf(t *testing.T, src, cond string) (*Model, symbols.ElementFilter, *symbols.Scope) {
	t.Helper()
	m, root := buildModel(t, src)
	p := parser.New(source.New("<filter>", []byte(cond)))
	expr := p.ParseExpression()
	if expr == nil || len(p.Diagnostics) != 0 {
		t.Fatalf("failed to parse filter condition %q: %v", cond, p.Diagnostics)
	}
	return m, symbols.ElementFilter{Expr: expr, Scope: root, Span: expr.Span()}, root
}

// selects evaluates cond for the element named by each of names, and returns the
// verdict for each. A condition that cannot be evaluated fails the test: the
// unevaluable case is asserted on its own.
func selects(t *testing.T, src, cond string, names ...string) []bool {
	t.Helper()
	m, f, root := filterOf(t, src, cond)
	out := make([]bool, 0, len(names))
	for _, name := range names {
		got, err := m.EvalElementFilter(f, sym(t, root, name))
		if err != nil {
			t.Fatalf("%q for %s: unexpected error %v", cond, name, err)
		}
		out = append(out, got)
	}
	return out
}

// want asserts the verdicts a condition produces, in the order the elements were
// given.
func want(t *testing.T, cond string, got []bool, expect ...bool) {
	t.Helper()
	if len(got) != len(expect) {
		t.Fatalf("%q: got %d verdicts, want %d", cond, len(got), len(expect))
	}
	for i := range got {
		if got[i] != expect[i] {
			t.Fatalf("%q: verdicts %v, want %v", cond, got, expect)
		}
	}
}

// metadataModel declares a metadata type, a type specializing it, and elements
// annotated in each of the three ways metadata can be attached.
const metadataModel = `
	metadata def Safety {
		attribute isMandatory;
		attribute level;
	}
	metadata def CrashSafety :> Safety;
	metadata def Comfort;

	#Safety part def Belt;
	part seatBelt { @Safety{isMandatory = true; level = 3;} }
	part airBag { metadata crash : CrashSafety { isMandatory = false; level = 5; } }
	part radio;
	metadata comfort : Comfort about radio;
	part keylessEntry;
`

func TestFilterPrefixMetadata(t *testing.T) {
	const cond = "@Safety"
	want(t, cond, selects(t, metadataModel, cond, "Belt", "keylessEntry"), true, false)
}

func TestFilterBodyMetadata(t *testing.T) {
	const cond = "@Safety"
	want(t, cond, selects(t, metadataModel, cond, "seatBelt", "airBag", "keylessEntry"), true, true, false)
}

func TestFilterAboutMetadata(t *testing.T) {
	const cond = "@Comfort"
	want(t, cond, selects(t, metadataModel, cond, "radio", "seatBelt"), true, false)
}

// A condition naming a metadata type selects an element annotated by a type
// specializing it (KerML conformance, via Model.AllSupertypes), and an element
// annotated only by the supertype does not satisfy the specialized condition.
func TestFilterSpecializedMetadataType(t *testing.T) {
	want(t, "@Safety", selects(t, metadataModel, "@Safety", "airBag"), true)
	want(t, "@CrashSafety", selects(t, metadataModel, "@CrashSafety", "airBag", "seatBelt"), true, false)
}

func TestFilterBooleanComposition(t *testing.T) {
	cases := map[string][]bool{
		"@Safety and @CrashSafety":                   {false, true},
		"@Safety or @Comfort":                        {true, true},
		"@Safety xor @CrashSafety":                   {true, false},
		"not @CrashSafety":                           {true, false},
		"@CrashSafety implies @Safety":               {true, true},
		"(@Safety or @Comfort) and not @CrashSafety": {true, false},
	}
	for cond, expect := range cases {
		want(t, cond, selects(t, metadataModel, cond, "seatBelt", "airBag"), expect...)
	}
}

// A feature of the candidate's annotation is read from the annotation body,
// whichever form attached it, and compared against a literal.
func TestFilterFeatureComparison(t *testing.T) {
	cases := map[string][]bool{
		"@Safety and Safety::isMandatory == true":      {true, false},
		"@Safety and (as Safety).level > 4":            {false, true},
		"@Safety and (as Safety).level == 3":           {true, false},
		"@CrashSafety and (as CrashSafety).level >= 5": {false, true},
		"@Safety and not (as Safety).isMandatory":      {false, true},
		"@Safety and (as Safety).level <= 3":           {true, false},
		"@Safety and (as Safety).isMandatory != true":  {false, true},
	}
	for cond, expect := range cases {
		want(t, cond, selects(t, metadataModel, cond, "seatBelt", "airBag"), expect...)
	}
}

// A guarded condition is decided by its guard where that settles it, so reading
// a feature of an annotation an element does not carry is never reached.
func TestFilterGuardShortCircuits(t *testing.T) {
	const cond = "@CrashSafety and (as CrashSafety).level >= 5"
	want(t, cond, selects(t, metadataModel, cond, "airBag", "keylessEntry"), true, false)
}

// An unevaluable condition has no truth value: it reports why, and the caller
// that enumerates candidates keeps the element (SatisfiesElementFilter).
func TestFilterUnevaluableExpression(t *testing.T) {
	cases := []struct {
		cond      string
		candidate string
		reason    string
	}{
		{"@Nonexistent", "keylessEntry", "does not resolve"},
		{"someFunction(1)", "keylessEntry", "invocation"},
		{"@Safety and (as Comfort).level > 1", "seatBelt", "not annotated by"},
	}
	for _, tc := range cases {
		cond, reason := tc.cond, tc.reason
		m, f, root := filterOf(t, metadataModel, cond)
		cand := sym(t, root, tc.candidate)
		if _, err := m.EvalElementFilter(f, cand); err == nil {
			t.Fatalf("%q should not be evaluable", cond)
		} else if !errors.Is(err, ErrFilterUnevaluable) || !strings.Contains(err.Error(), reason) {
			t.Fatalf("%q: error %v, want unevaluable mentioning %q", cond, err, reason)
		}
		if !m.SatisfiesElementFilter(f, cand) {
			t.Fatalf("%q: an unevaluable condition must keep the element", cond)
		}
	}
}

// A condition that evaluates to something other than a boolean cannot select
// elements, and says so.
func TestFilterNotBoolean(t *testing.T) {
	m, f, root := filterOf(t, metadataModel, "Safety::level")
	_, err := m.EvalElementFilter(f, sym(t, root, "seatBelt"))
	if err == nil || !errors.Is(err, ErrFilterNotBoolean) {
		t.Fatalf("a non-boolean condition should report ErrFilterNotBoolean, got %v", err)
	}
}

// A condition is compiled once and each candidate's verdict remembered, which is
// what makes filtering affordable on every import enumeration.
func TestFilterMemoization(t *testing.T) {
	m, f, root := filterOf(t, metadataModel, "@Safety")
	first := m.CompileElementFilter(f)
	if first != m.CompileElementFilter(f) {
		t.Fatal("compiling the same condition twice should reuse the predicate")
	}
	cand := sym(t, root, "seatBelt")
	if _, err := m.EvalElementFilter(f, cand); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := m.filterVerdicts[filterKey{pred: first, cand: cand}]; !ok {
		t.Fatal("a decided verdict should be remembered")
	}
}

// The compiled form of a condition is what a library restored from an index
// cache carries, and it decides candidates the same way as the parsed condition.
func TestFilterCompiledPredicateDecidesAlone(t *testing.T) {
	m, f, root := filterOf(t, metadataModel, "@Safety and Safety::isMandatory == true")
	compiled := symbols.ElementFilter{Pred: m.CompileElementFilter(f), Span: f.Span}
	for _, tc := range []struct {
		name  string
		match bool
	}{{"seatBelt", true}, {"airBag", false}, {"keylessEntry", false}} {
		cand := sym(t, root, tc.name)
		fromExpr, errExpr := m.EvalElementFilter(f, cand)
		fromPred, errPred := m.EvalElementFilter(compiled, cand)
		if (errExpr == nil) != (errPred == nil) || fromExpr != fromPred {
			t.Fatalf("%s: parsed condition = %v/%v, compiled = %v/%v", tc.name, fromExpr, errExpr, fromPred, errPred)
		}
		if errExpr == nil && fromExpr != tc.match {
			t.Fatalf("%s: verdict %v, want %v", tc.name, fromExpr, tc.match)
		}
	}
}

// FilterPredicateOf compiles the condition of a declared `filter` member, which
// is the route the validation pass and the record writer take.
func TestFilterPredicateOfDeclaredFilterMember(t *testing.T) {
	m, root := buildModel(t, metadataModel+"\npackage P { filter @Safety; }")
	pkg := sym(t, root, "P")
	filters := symbols.NamespaceFiltersIn(pkg.Scope)
	if len(filters) != 1 {
		t.Fatalf("NamespaceFiltersIn(P) = %d filters, want 1", len(filters))
	}
	pred := m.FilterPredicateOf(filters[0].Scope, filters[0].Expr)
	if pred == nil || pred.Op != symbols.FilterClassify {
		t.Fatalf("compiled `filter @Safety;` = %+v, want a classification", pred)
	}
	if got, err := m.EvalElementFilter(filters[0], sym(t, root, "seatBelt")); err != nil || !got {
		t.Fatalf("the declared filter should select seatBelt, got %v err=%v", got, err)
	}
}
