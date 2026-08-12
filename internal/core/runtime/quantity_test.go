package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
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
		{"1.5 [m/s] + 1.8 [km/h]", "2 [m/s]"},
		{"3.0 [km] + 500.0 [m]", "3.5 [km]"},
		{"10.0 [m] / 2.0 [s]", "5 [m/s]"},
		{"2.0 [m] * 3.0 [m]", "6 [m*m]"},
		{"-2.5 [m/s]", "-2.5 [m/s]"},
		{"3.0 [m] * 2.0", "6 [m]"},
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

// TestSequenceIndexIsNotAQuantity: `seq#(i)` shares its node with a quantity
// expression but is a different operation, and stays unimplemented rather than
// being read as a magnitude in a unit.
func TestSequenceIndexIsNotAQuantity(t *testing.T) {
	ctx, scope := quantityContext(t)
	got, err := evalIn(t, ctx, scope, "speeds#(2)")
	if err == nil {
		t.Fatalf("speeds#(2) = %v, want an error", got)
	}
	if errors.Is(err, ErrNotAQuantity) {
		t.Errorf("a sequence index is not a malformed quantity: %v", err)
	}
}
