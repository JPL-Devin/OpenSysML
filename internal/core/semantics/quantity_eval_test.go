package semantics_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// quantityModel is the units repro: stages whose masses are quantities, plus
// attributes covering every fold the const-folder is expected to make.
const quantityModel = `private import ScalarValues::*;
private import SI::*;
private import ISQ::*;

package Q {
	part def Stage {
		attribute mass :> ISQ::mass;
	}
	part def FirstStage :> Stage {
		attribute :>> mass = 2290000 [kg];
	}
	part def Folds {
		attribute scaled = 2 [kg] * 3;
		attribute summed = 1 [km] + 500 [m];
		attribute negated = -(3 [kg]);
		attribute quotient = 6 [m] / 2 [s];
		attribute ratio = 4 [m] / 2 [m];
		attribute heavier = 5 [kg] > 4000 [g];
		attribute mixed = 1 [kg] + 1 [m];
		attribute dryMass :> ISQ::mass;
		attribute propellantMass :> ISQ::mass;
		attribute total = dryMass + propellantMass;
	}
}`

func quantityFixture(t *testing.T) (*semantics.Model, *symbols.Index) {
	t.Helper()
	idx := libs.NewModelIndex()
	p := parser.New(source.New("q.sysml", []byte(quantityModel)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx.AddDocument("q.sysml", root)
	idx.ExpandWildcardImports()
	return semantics.NewModel(resolve.New(idx)), idx
}

// fold parses expr at the fixture's document root and folds it as a quantity.
func fold(t *testing.T, m *semantics.Model, idx *symbols.Index, expr string) semantics.Quantity {
	t.Helper()
	p := parser.New(source.New("<expr>", []byte(expr)))
	node := p.ParseExpression()
	if node == nil || len(p.Diagnostics) != 0 {
		t.Fatalf("parse %q: %v", expr, p.Diagnostics)
	}
	q, ok := m.EvalQuantity(idx.DocumentRoot("q.sysml"), node)
	if !ok {
		t.Fatalf("%q does not fold to a quantity", expr)
	}
	return q
}

func declaredValue(t *testing.T, m *semantics.Model, idx *symbols.Index, fqn, property string) symbols.FilterValue {
	t.Helper()
	matches := symbols.PreferDeclared(idx.LookupQualified(fqn))
	if len(matches) != 1 {
		t.Fatalf("%s matched %d symbols, want 1", fqn, len(matches))
	}
	values, present := m.DeclaredFeatureValues(matches[0], property)
	if !present {
		t.Fatalf("%s has no declared value for %s", fqn, property)
	}
	if len(values) != 1 {
		t.Fatalf("%s.%s has %d values, want 1", fqn, property, len(values))
	}
	return values[0]
}

// TestDeclaredQuantityValueKeepsItsUnit: `2290000 [kg]` folds to a quantity
// carrying the magnitude as written, the unit's spelling, and its reduction.
func TestDeclaredQuantityValueKeepsItsUnit(t *testing.T) {
	m, idx := quantityFixture(t)
	value := declaredValue(t, m, idx, "Q::FirstStage", "mass")
	if value.Kind != symbols.FilterValueQuantity {
		t.Fatalf("kind = %v, want quantity", value.Kind)
	}
	q, ok := semantics.QuantityOf(value)
	if !ok {
		t.Fatal("quantity value carries no quantity")
	}
	if q.Num.Kind != semantics.ValInt || q.Num.Int != 2290000 {
		t.Errorf("magnitude = %+v, want Integer 2290000", q.Num)
	}
	if q.Unit.Text != "kg" {
		t.Errorf("unit text = %q, want kg", q.Unit.Text)
	}
	if q.String() != "2290000 [kg]" {
		t.Errorf("String() = %q, want %q", q.String(), "2290000 [kg]")
	}
	if q.Unit.Term.String() != "1000·gram" {
		t.Errorf("unit term = %s, want 1000·gram", q.Unit.Term)
	}
}

// TestQuantityExpressionsFold: constant expressions over quantities fold with
// the runtime's unit rules, and a cancelled or compared unit leaves a scalar.
func TestQuantityExpressionsFold(t *testing.T) {
	m, idx := quantityFixture(t)
	cases := []struct {
		property string
		kind     symbols.FilterValueKind
		text     string
	}{
		{"scaled", symbols.FilterValueQuantity, "6 [kg]"},
		{"summed", symbols.FilterValueQuantity, "1.5 [km]"},
		{"negated", symbols.FilterValueQuantity, "-3 [kg]"},
		{"quotient", symbols.FilterValueQuantity, "3.0 [m/s]"},
		{"ratio", symbols.FilterValueReal, "2.0"},
		{"heavier", symbols.FilterValueBool, "true"},
	}
	for _, tc := range cases {
		t.Run(tc.property, func(t *testing.T) {
			value := declaredValue(t, m, idx, "Q::Folds", tc.property)
			if value.Kind != tc.kind {
				t.Fatalf("kind = %v, want %v", value.Kind, tc.kind)
			}
			var got string
			switch value.Kind {
			case symbols.FilterValueQuantity:
				q, _ := semantics.QuantityOf(value)
				got = q.String()
			case symbols.FilterValueReal:
				got = semantics.FormatReal(value.Real)
			case symbols.FilterValueBool:
				if value.Bool {
					got = "true"
				} else {
					got = "false"
				}
			}
			if got != tc.text {
				t.Errorf("%s = %s, want %s", tc.property, got, tc.text)
			}
		})
	}
}

// TestUnfoldableQuantityValuesStayUnknown: a sum of incommensurable units and
// an expression over unbound features are not constants, and say so.
func TestUnfoldableQuantityValuesStayUnknown(t *testing.T) {
	m, idx := quantityFixture(t)
	for _, property := range []string{"mixed", "total"} {
		value := declaredValue(t, m, idx, "Q::Folds", property)
		if value.Kind != symbols.FilterValueUnknown {
			t.Errorf("%s folded to %v, want unknown", property, value.Kind)
		}
	}
}

// TestCompareQuantitiesConvertsCommensurableUnits: ordering converts the right
// operand into the left unit, so 1 kg and 1000 g are equal and 999 g is less.
func TestCompareQuantitiesConvertsCommensurableUnits(t *testing.T) {
	m, idx := quantityFixture(t)
	kilograms := func(n int) semantics.Quantity { return fold(t, m, idx, fmt.Sprintf("%d [kg]", n)) }
	grams := func(n int) semantics.Quantity { return fold(t, m, idx, fmt.Sprintf("%d [g]", n)) }
	equal, err := semantics.EqualQuantities(ast.OpEq, kilograms(1), grams(1000))
	if err != nil || !equal {
		t.Fatalf("1 kg == 1000 g: %v, %v", equal, err)
	}
	less, err := semantics.CompareQuantities(ast.OpLt, grams(999), kilograms(1))
	if err != nil || !less {
		t.Fatalf("999 g < 1 kg: %v, %v", less, err)
	}
	greater, err := semantics.CompareQuantities(ast.OpGt, grams(999), kilograms(1))
	if err != nil || greater {
		t.Fatalf("999 g > 1 kg: %v, %v", greater, err)
	}
}

// TestIncommensurableQuantitiesAreTypedErrors: comparison and addition across
// dimensions report ErrIncommensurableUnits instead of comparing magnitudes.
func TestIncommensurableQuantitiesAreTypedErrors(t *testing.T) {
	m, idx := quantityFixture(t)
	kilograms := func(n int) semantics.Quantity { return fold(t, m, idx, fmt.Sprintf("%d [kg]", n)) }
	metres := func(n int) semantics.Quantity { return fold(t, m, idx, fmt.Sprintf("%d [m]", n)) }
	if _, err := semantics.CompareQuantities(ast.OpLt, kilograms(1), metres(2)); !errors.Is(err, semantics.ErrIncommensurableUnits) {
		t.Errorf("1 kg < 2 m: err = %v, want ErrIncommensurableUnits", err)
	}
	if _, err := semantics.EqualQuantities(ast.OpEq, kilograms(1), metres(1)); !errors.Is(err, semantics.ErrIncommensurableUnits) {
		t.Errorf("1 kg == 1 m: err = %v, want ErrIncommensurableUnits", err)
	}
	if _, err := semantics.QuantityBinary(ast.OpAdd, kilograms(1), metres(1)); !errors.Is(err, semantics.ErrIncommensurableUnits) {
		t.Errorf("1 kg + 1 m: err = %v, want ErrIncommensurableUnits", err)
	}
}

// TestQuantityArithmeticFailures: a zero divisor and an Integer overflow are
// typed errors, never a silent infinity or wrap-around.
func TestQuantityArithmeticFailures(t *testing.T) {
	m, idx := quantityFixture(t)
	kilograms := func(n int64) semantics.Quantity { return fold(t, m, idx, fmt.Sprintf("%d [kg]", n)) }
	metres := func(n int) semantics.Quantity { return fold(t, m, idx, fmt.Sprintf("%d [m]", n)) }
	zero := semantics.Quantity{Num: semantics.Value{Kind: semantics.ValInt}, Unit: semantics.UnitOne()}
	if _, err := semantics.QuantityBinary(ast.OpDiv, kilograms(1), zero); !errors.Is(err, semantics.ErrDivisionByZero) {
		t.Errorf("1 kg / 0: err = %v, want ErrDivisionByZero", err)
	}
	if _, err := semantics.QuantityBinary(ast.OpMul, kilograms(1<<62), kilograms(4)); !errors.Is(err, semantics.ErrArithmeticOverflow) {
		t.Errorf("2^62 kg * 4 kg: err = %v, want ErrArithmeticOverflow", err)
	}
	exponent := semantics.Quantity{Num: semantics.Value{Kind: semantics.ValInt, Int: 2}, Unit: semantics.UnitOne()}
	squared, err := semantics.QuantityBinary(ast.OpPow, metres(3), exponent)
	if err != nil || squared.String() != "9 [m**2]" {
		t.Errorf("3 m ^ 2 = %v, %v; want 9 [m**2]", squared.String(), err)
	}
	if _, err := semantics.QuantityBinary(ast.OpPow, metres(3), metres(2)); !errors.Is(err, semantics.ErrQuantityOperand) {
		t.Errorf("3 m ^ 2 m: err = %v, want ErrQuantityOperand", err)
	}
}
