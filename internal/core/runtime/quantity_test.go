package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// quantityContext builds a runtime over the standard library and evaluates
// expressions in the scope of a package that imports SI.
func quantityContext(t *testing.T) (*Context, *symbols.Scope) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import SI::*;
			attribute speeds = (1.0, 2.0, 3.0);
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return ctx, pkg.Scope
}

// evalIn evaluates the expression written in src in scope.
func evalIn(t *testing.T, ctx *Context, scope *symbols.Scope, src string) (Value, error) {
	t.Helper()
	p := parser.New(source.New("<expr>", []byte(src)))
	expr := p.ParseExpression()
	if expr == nil || len(p.Diagnostics) > 0 {
		t.Fatalf("parse %q: %v", src, p.Diagnostics)
	}
	return ctx.EvalWithScope(expr, scope)
}

// TestQuantityEvaluation evaluates quantity expressions over library units: a
// quantity keeps its unit, commensurable units convert, and a ratio of like
// quantities is a number.
func TestQuantityEvaluation(t *testing.T) {
	ctx, scope := quantityContext(t)

	cases := []struct {
		src  string
		want string // rendered value
	}{
		{"1.5 [m/s]", "1.5 [m/s]"},
		{"1.5 [m/s] + 1.8 [km/h]", "2.0 [m/s]"},
		{"3.0 [km] + 500.0 [m]", "3.5 [km]"},
		{"10.0 [m] / 2.0 [s]", "5.0 [m/s]"},
		{"2.0 [m] * 3.0 [m]", "6.0 [m*m]"},
		{"-2.5 [m/s]", "-2.5 [m/s]"},
		{"3.0 [m] * 2.0", "6.0 [m]"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			got, err := evalIn(t, ctx, scope, tc.src)
			if err != nil {
				t.Fatalf("%s: %v", tc.src, err)
			}
			if got.Kind != ValQuantity {
				t.Fatalf("%s = %v (%s), want a quantity", tc.src, got, got.Kind)
			}
			if got.Quantity.String() != tc.want {
				t.Errorf("%s = %s, want %s", tc.src, got.Quantity, tc.want)
			}
		})
	}
}

// TestQuantityArithmeticReportsOverflow: a magnitude no Real holds is reported
// for a quantity as it is for a bare Real, rather than carried as an infinity.
func TestQuantityArithmeticReportsOverflow(t *testing.T) {
	ctx, scope := quantityContext(t)

	for _, src := range []string{
		"1e308 [m] + 1e308 [m]",
		"1e200 [m] * 1e200 [s]",
		"1e308 [m] / 1e-308 [s]",
		"1e308 [m] / 1e-308 [m]",
	} {
		got, err := evalIn(t, ctx, scope, src)
		if !errors.Is(err, semantics.ErrArithmeticOverflow) {
			t.Errorf("%s = %+v, %v; want ErrArithmeticOverflow", src, got, err)
		}
	}
}

// TestQuantityComparison compares quantities across commensurable units,
// including at the exact boundary the lunar-lander requirement sits on.
func TestQuantityComparison(t *testing.T) {
	cases := []struct {
		src  string
		want bool
	}{
		{"1.5 [m/s] <= 5.4 [km/h]", true},
		{"5.4 [km/h] <= 1.5 [m/s]", true},
		{"1.5 [m/s] < 5.4 [km/h]", false},
		{"1.5 [m/s] == 5.4 [km/h]", true},
		{"1.5 [m/s] != 5.4 [km/h]", false},
		{"2.0 [m/s] > 5.4 [km/h]", true},
		{"1.0 [km] == 1000.0 [m]", true},
		{"4.0 [m] / 2.0 [m] == 2.0", true},
	}
	ctx, scope := quantityContext(t)
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			got, err := evalIn(t, ctx, scope, tc.src)
			if err != nil {
				t.Fatalf("%s: %v", tc.src, err)
			}
			if got.Kind != ValConst || got.Const.Kind != semantics.ValBool {
				t.Fatalf("%s = %v, want a boolean", tc.src, got)
			}
			if got.Const.Bool != tc.want {
				t.Errorf("%s = %v, want %v", tc.src, got.Const.Bool, tc.want)
			}
		})
	}
}

// TestQuantityIncommensurable: an operation between units that measure different
// things is an error, never a comparison of the bare magnitudes.
func TestQuantityIncommensurable(t *testing.T) {
	ctx, scope := quantityContext(t)
	for _, src := range []string{
		"1.5 [m/s] <= 2.0 [s]",
		"1.5 [m] + 2.0 [s]",
		"1.5 [m] <= 2.0",
		"1.5 [m] == 1.5 [s]",
	} {
		t.Run(src, func(t *testing.T) {
			got, err := evalIn(t, ctx, scope, src)
			if !errors.Is(err, ErrIncommensurableUnits) {
				t.Fatalf("%s = %v, %v; want ErrIncommensurableUnits", src, got, err)
			}
		})
	}
}

// TestQuantityExponentiation raises quantities to constant exponents. The
// magnitude comes from semantics.Pow, the implementation `**` shares with the
// folder and the scalar path, so Integer operands with a non-negative exponent
// keep an Integer magnitude while the unit is raised as a real exponent.
func TestQuantityExponentiation(t *testing.T) {
	ctx, scope := quantityContext(t)

	cases := []struct {
		src      string
		want     string // rendered value
		wantKind semantics.ValueKind
	}{
		{"(2 [m]) ** 3", "8 [(m)**3]", semantics.ValInt},
		{"(2.0 [m]) ** 3", "8.0 [(m)**3]", semantics.ValReal},
		{"(3.0 [m/s]) ** 2.0", "9.0 [(m/s)**2]", semantics.ValReal},
		{"(2.0 [m]) ** -1", "0.5 [(m)**-1]", semantics.ValReal},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			got, err := evalIn(t, ctx, scope, tc.src)
			if err != nil {
				t.Fatalf("%s: %v", tc.src, err)
			}
			if got.Kind != ValQuantity {
				t.Fatalf("%s = %v (%s), want a quantity", tc.src, got, got.Kind)
			}
			if got.Quantity.String() != tc.want {
				t.Errorf("%s = %s, want %s", tc.src, got.Quantity, tc.want)
			}
			if got.Quantity.Num.Kind != tc.wantKind {
				t.Errorf("%s magnitude is %v, want %v", tc.src, got.Quantity.Num.Kind, tc.wantKind)
			}
		})
	}
}

// TestQuantityExponentiationReports: the magnitude of an exponentiated quantity
// obeys the same domain and range as a bare number's, so an undefined or
// non-finite result is reported rather than carried as an Inf or a NaN in a
// unit.
func TestQuantityExponentiationReports(t *testing.T) {
	ctx, scope := quantityContext(t)

	cases := []struct {
		src     string
		wantErr error
	}{
		{"(0.0 [m]) ** -1.0", semantics.ErrArithmeticDomain},
		{"(-2.0 [m]) ** 0.5", semantics.ErrArithmeticDomain},
		{"(1.0e300 [m]) ** 3.0", semantics.ErrArithmeticOverflow},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			got, err := evalIn(t, ctx, scope, tc.src)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("%s = %v, %v; want %v", tc.src, got, err, tc.wantErr)
			}
		})
	}
}

// TestComposedUnitText renders the unit an operation composes. An operand that
// is itself composed is parenthesized, since `m/s*kg/s` names a different unit
// than the product of `m/s` and `kg/s`; an atomic one stays bare.
func TestComposedUnitText(t *testing.T) {
	atomic := func(text string) Unit {
		return Unit{Text: text, Term: semantics.UnitTerm{Scale: semantics.UnitScale(1)}}
	}
	// A bare number contributes no unit: no text, and a dimensionless reduction.
	number := Unit{Term: semantics.UnitTerm{Scale: semantics.UnitScale(1)}}

	cases := []struct {
		name        string
		left, right Unit
		op          ast.OperatorKind
		want        string
	}{
		{"atomic times atomic", atomic("m"), atomic("m"), ast.OpMul, "m*m"},
		{"atomic over atomic", atomic("m"), atomic("s"), ast.OpDiv, "m/s"},
		{"atomic times composed", atomic("kg"), atomic("m/s"), ast.OpMul, "kg*(m/s)"},
		{"composed over atomic", atomic("m/s"), atomic("s"), ast.OpDiv, "(m/s)/s"},
		{"composed times composed", atomic("m/s"), atomic("kg/s"), ast.OpMul, "(m/s)*(kg/s)"},
		{"composed over composed", atomic("m/s"), atomic("kg/s"), ast.OpDiv, "(m/s)/(kg/s)"},
		{"exponentiated operand", atomic("(m)**2"), atomic("s"), ast.OpDiv, "((m)**2)/s"},
		{"quantity scaled by a number", atomic("m/s"), number, ast.OpMul, "m/s"},
		{"quantity divided by a number", atomic("m/s"), number, ast.OpDiv, "m/s"},
		{"number scaling a quantity", number, atomic("m/s"), ast.OpMul, "m/s"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := composedUnitText(tc.left, tc.right, tc.op); got != tc.want {
				t.Errorf("composedUnitText(%s, %s, %s) = %q, want %q", tc.left, tc.right, tc.op, got, tc.want)
			}
		})
	}
}

// TestFormatTraceValueQuantity: a trace of a unit-carrying value names the
// unit, since a magnitude alone answers nothing about what was computed.
func TestFormatTraceValueQuantity(t *testing.T) {
	metre := Unit{Text: "m/s", Term: semantics.UnitTerm{Scale: semantics.UnitScale(1)}}
	cases := []struct {
		name string
		val  Value
		want string
	}{
		{"real magnitude", Value{Kind: ValQuantity, Quantity: &Quantity{
			Num: semantics.Value{Kind: semantics.ValReal, Real: 1.5}, Unit: metre}}, "1.5 [m/s]"},
		{"whole real magnitude", Value{Kind: ValQuantity, Quantity: &Quantity{
			Num: semantics.Value{Kind: semantics.ValReal, Real: 5}, Unit: metre}}, "5.0 [m/s]"},
		{"integer magnitude", Value{Kind: ValQuantity, Quantity: &Quantity{
			Num: semantics.Value{Kind: semantics.ValInt, Int: 5}, Unit: metre}}, "5 [m/s]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatTraceValue(tc.val); got != tc.want {
				t.Errorf("FormatTraceValue(%s) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

// TestSequenceIndexIsNotAQuantity: `seq#(i)` shares its node with a quantity
// expression but is a different operation — it indexes the sequence rather than
// reading the index as a measurement unit, and an index it cannot answer is
// reported as an index error and not as a malformed quantity.
func TestSequenceIndexIsNotAQuantity(t *testing.T) {
	ctx, scope := quantityContext(t)

	got, err := evalIn(t, ctx, scope, "speeds#(2)")
	if err != nil {
		t.Fatalf("speeds#(2): %v", err)
	}
	if got.Kind != ValConst || got.Const.Kind != semantics.ValReal || got.Const.Real != 2.0 {
		t.Errorf("speeds#(2) = %v, want the real 2.0", got)
	}

	if _, err := evalIn(t, ctx, scope, "speeds#(4)"); !errors.Is(err, ErrIndexOutOfRange) {
		t.Errorf("speeds#(4) error = %v, want ErrIndexOutOfRange", err)
	}
	if _, err := evalIn(t, ctx, scope, "speeds#(4)"); errors.Is(err, ErrNotAQuantity) {
		t.Errorf("a sequence index is not a malformed quantity: %v", err)
	}
}

// TestQuantityAndIndexNotationsCoexist pins both meanings of the shared node in
// one place: the bracket form is a quantity, the parenthesized form an index,
// and a model can write the two of them in one expression.
func TestQuantityAndIndexNotationsCoexist(t *testing.T) {
	ctx, scope := quantityContext(t)

	quantity, err := evalIn(t, ctx, scope, "5 [m]")
	if err != nil {
		t.Fatalf("5 [m]: %v", err)
	}
	if quantity.Kind != ValQuantity || quantity.Quantity.String() != "5 [m]" {
		t.Errorf("5 [m] = %v (%s), want the quantity 5 [m]", quantity, quantity.Kind)
	}

	// The index of a sequence of quantities is a quantity, so the two notations
	// compose: `(1 [m], 2 [m])#(2)` is `2 [m]`.
	indexed, err := evalIn(t, ctx, scope, "(1 [m], 2 [m])#(2)")
	if err != nil {
		t.Fatalf("(1 [m], 2 [m])#(2): %v", err)
	}
	if indexed.Kind != ValQuantity || indexed.Quantity.String() != "2 [m]" {
		t.Errorf("(1 [m], 2 [m])#(2) = %v (%s), want the quantity 2 [m]", indexed, indexed.Kind)
	}

	// An index that is not a whole number is an index error, not a unit: the
	// notation `speeds#(1.5)` names no position of the sequence.
	if _, err := evalIn(t, ctx, scope, "speeds#(1.5)"); !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("speeds#(1.5) error = %v, want ErrTypeMismatch", err)
	}
	// A unit named where an index belongs stays a quantity error, since the
	// bracket says the expression is a quantity.
	if _, err := evalIn(t, ctx, scope, "speeds [m]"); !errors.Is(err, ErrNotAQuantity) {
		t.Errorf("speeds [m] error = %v, want ErrNotAQuantity", err)
	}
}
