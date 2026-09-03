package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// TestQuantityCalculationsAreAllDispatchable: every Quantities and Units calculation is
// registered with its parameters in declared order (anonymous ones by position) or is a builtin.
func TestQuantityCalculationsAreAllDispatchable(t *testing.T) {
	packages := map[string]string{
		"QuantityCalculations":       "Domain Libraries/Quantities and Units/QuantityCalculations.sysml",
		"MeasurementRefCalculations": "Domain Libraries/Quantities and Units/MeasurementRefCalculations.sysml",
		"VectorCalculations":         "Domain Libraries/Quantities and Units/VectorCalculations.sysml",
		"TensorCalculations":         "Domain Libraries/Quantities and Units/TensorCalculations.sysml",
	}

	for pkg, path := range packages {
		t.Run(pkg, func(t *testing.T) {
			data, err := libs.DefaultSource().Read(path)
			if err != nil {
				t.Fatalf("Read(%q): %v", path, err)
			}
			p := parser.New(source.New(path, data))
			file := p.ParseFile()
			if len(p.Diagnostics) > 0 {
				t.Fatalf("%s has %d parse diagnostics, want 0: %v", path, len(p.Diagnostics), p.Diagnostics)
			}
			idx := symbols.NewIndex()
			idx.AddDocument(path, file)
			resolver := resolve.New(idx)
			ctx := NewContext(semantics.NewModel(resolver), resolver, 10000)

			declared := 0
			for _, sym := range idx.LookupDirectChildren(pkg) {
				if !isCalcSymbol(sym) {
					continue
				}
				declared++
				fqn := ctx.qualifiedSymbolName(sym)
				fn, ok := libraryFunctionByName(fqn)
				if !ok {
					if _, isBuiltin := builtins[fqn]; !isBuiltin {
						t.Errorf("%s is declared in %s and is not dispatchable", fqn, path)
					}
					continue
				}
				params := declaredInputs(sym)
				if len(params) != len(fn.params) {
					t.Errorf("%s declares %v, implementation takes %v", fqn, params, fn.params)
					continue
				}
				for i, name := range params {
					if name != "" && fn.params[i] != name {
						t.Errorf("%s parameter %d is %q, implementation names it %q", fqn, i, name, fn.params[i])
					}
				}
			}
			if declared == 0 {
				t.Fatalf("%s declares no calculations; the gate found nothing to check", pkg)
			}
		})
	}
}

// declaredInputs lists a calc's own input parameters in declared order, an
// anonymous one as "".
func declaredInputs(sym *symbols.Symbol) []string {
	var params []string
	for _, member := range declMembers(sym.Decl) {
		usage, ok := member.(*ast.Usage)
		if !ok || (usage.Direction != ast.DirIn && usage.Direction != ast.DirInOut) {
			continue
		}
		name, _ := ast.EffectiveName(usage)
		params = append(params, name)
	}
	return params
}

// quantityCalculationsContext evaluates expressions in a package that imports
// ISQ, SI and QuantityCalculations, as the ISQ examples do.
func quantityCalculationsContext(t *testing.T) (*Context, *symbols.Scope) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import ISQ::*;
			public import SI::*;
			public import QuantityCalculations::*;
			public import TrigFunctions::*;
			attribute side : LengthValue = 3 [m];
			attribute area : AreaValue = side * side;
			attribute none : LengthValue[0..*] = ();
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return ctx, pkg.Scope
}

// TestQuantityCalculations evaluates each QuantityCalculations function the
// runtime computes, over quantities written in SI units.
func TestQuantityCalculations(t *testing.T) {
	ctx, scope := quantityCalculationsContext(t)

	cases := []struct {
		src  string
		want string
	}{
		{"sqrt(area)", "3.0 [m]"},
		{"sqrt(9 [m**2])", "3.0 [m]"},
		{"sqrt(16 [m*m])", "4.0 [m]"},
		{"sqrt(4 [m**2/s**2])", "2.0 [m/s]"},
		{"sqrt(2.25 [km**2])", "1.5 [km]"},
		{"sqrt(4 [N**2])", "2.0 [N]"},
		{"sqrt(9 [rad**2])", "3.0 [rad]"},
		{"sqrt(9 [m/m])", "3.0"},
		{"sqrt(9 [m*m/m**2])", "3.0"},
		{"sqrt(4 [km*m])", "63.245553203367585 [m]"},
		{"sqrt(16.0)", "4.0"},
		{"abs(-3 [m])", "3 [m]"},
		{"abs(-2.5 [m/s])", "2.5 [m/s]"},
		{"abs(2 [kg])", "2 [kg]"},
		{"floor(2.7 [m])", "2 [m]"},
		{"floor(-2.2 [m])", "-3 [m]"},
		{"round(2.5 [m])", "3 [m]"},
		{"round(2.4 [m])", "2 [m]"},
		{"max(1 [m], 200 [cm])", "200 [cm]"},
		{"max(300 [cm], 2 [m])", "300 [cm]"},
		{"max(1 [m], 100 [cm])", "1 [m]"},
		{"QuantityCalculations::min(1 [m], 200 [cm])", "1 [m]"},
		{"QuantityCalculations::min(300 [cm], 2 [m])", "2 [m]"},
		{"QuantityCalculations::min(1 [m], 100 [cm])", "1 [m]"},
		{"(1 [m], 2 [m])->sum()", "3 [m]"},
		{"(1 [km], 500 [m])->sum()", "1.5 [km]"},
		{"(side, side, side)->sum()", "9 [m]"},
		{"sum((2 [m], 3 [m]))", "5 [m]"},
		{"(2 [m], 3 [m])->product()", "6 [m**2]"},
		{"(2 [m], 3 [s])->product()", "6 [m*s]"},
		{"(2 [m])->sum()", "2 [m]"},
		{"none->sum()", "0"},
		{"none->product()", "1"},
		{"QuantityCalculations::'+'(1 [m], 2 [m])", "3 [m]"},
		{"QuantityCalculations::'+'(2 [m])", "2 [m]"},
		{"QuantityCalculations::'-'(5 [m], 2 [m])", "3 [m]"},
		{"QuantityCalculations::'-'(2 [m])", "-2 [m]"},
		{"QuantityCalculations::'*'(2 [m], 3 [m])", "6 [m**2]"},
		{"QuantityCalculations::'/'(6 [m], 3 [s])", "2.0 [m/s]"},
		{"QuantityCalculations::'/'(6 [m], 3 [m])", "2.0"},
		{"QuantityCalculations::'**'(3 [m], 2)", "9 [m**2]"},
		{"QuantityCalculations::'^'(2 [m], 3)", "8 [m**3]"},
		{"QuantityCalculations::'<'(1 [m], 200 [cm])", "true"},
		{"QuantityCalculations::'>'(1 [m], 200 [cm])", "false"},
		{"QuantityCalculations::'<='(1 [m], 100 [cm])", "true"},
		{"QuantityCalculations::'>='(1 [m], 100 [cm])", "true"},
		{"QuantityCalculations::'=='(1 [m], 100 [cm])", "true"},
		{"QuantityCalculations::'=='(1 [m], 101 [cm])", "false"},
		{"isZero(0 [m])", "true"},
		{"isZero(0.0 [m/s])", "true"},
		{"isZero(1 [m])", "false"},
		{"isUnit(1 [m])", "true"},
		{"isUnit(2 [m])", "false"},
		{"ToString(1.5 [m/s])", `"1.5 [m/s]"`},
		{"ToString(2 [m] * 3 [m])", `"6 [m**2]"`},
		{"ToInteger(3 [m])", "3"},
		{"ToInteger(3.0 [m])", "3"},
		{"ToReal(3 [m])", "3.0"},
		{"ToRational(1.5 [m])", "1.5"},
		{"ToDimensionOneValue(2.5)", "2.5"},
		{"sin(90 ['°']) > 0.99999", "true"},
		{"cos(0 [rad])", "1.0"},
		{"sin(0.0 [rad])", "0.0"},
		{"tan(45 ['°']) < 1.0001 and tan(45 ['°']) > 0.9999", "true"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			got, err := evalIn(t, ctx, scope, tc.src)
			if err != nil {
				t.Fatalf("%s: %v", tc.src, err)
			}
			if rendered := FormatValue(got); rendered != tc.want {
				t.Errorf("%s = %s, want %s", tc.src, rendered, tc.want)
			}
		})
	}
}

// TestQuantityCalculationsReport: each failure a QuantityCalculations function
// has is a typed error naming the function.
func TestQuantityCalculationsReport(t *testing.T) {
	ctx, scope := quantityCalculationsContext(t)

	cases := []struct {
		src  string
		want error
		text string
	}{
		{"sqrt(9 [m])", ErrUnitRoot, "function QuantityCalculations::sqrt: unit has no root: m (metre) raises metre to the odd power 1"},
		{"sqrt(8 [m**3])", ErrUnitRoot, "raises metre to the odd power 3"},
		{"sqrt(4 [m**2/s])", ErrUnitRoot, "raises second to the odd power -1"},
		{"sqrt(9 [rad])", ErrUnitRoot, "function QuantityCalculations::sqrt: unit has no root: rad raises the dimension-one unit rad to the odd power 1"},
		{"sqrt(9 ['°'])", ErrUnitRoot, "° raises the dimension-one unit ° to the odd power 1"},
		{"sqrt(9 [rad**3])", ErrUnitRoot, "raises the dimension-one unit rad to the odd power 3"},
		{"sqrt(9 [rad*m**2])", ErrUnitRoot, "raises the dimension-one unit rad to the odd power 1"},
		{"sqrt(-4 [m**2])", semantics.ErrArithmeticDomain, "sqrt of a negative quantity -4 [m**2]"},
		{"sqrt(\"9\")", ErrTypeMismatch, `function QuantityCalculations::sqrt parameter "x" requires a quantity, got string`},
		{"abs(true)", ErrTypeMismatch, `function QuantityCalculations::abs parameter "x" requires a quantity`},
		{"max(1 [m], 1 [s])", ErrIncommensurableUnits, "function QuantityCalculations::max"},
		{"QuantityCalculations::min(1 [kg], 1 [m])", ErrIncommensurableUnits, "function QuantityCalculations::min"},
		{"(1 [m], 1 [s])->sum()", ErrIncommensurableUnits, ""},
		{"QuantityCalculations::'+'(1 [m], 1 [s])", ErrIncommensurableUnits, "function QuantityCalculations::+"},
		{"QuantityCalculations::'<'(1 [m], 1 [s])", ErrIncommensurableUnits, "function QuantityCalculations::<"},
		{"QuantityCalculations::'/'(1 [m], 0 [s])", ErrDivisionByZero, "function QuantityCalculations::/"},
		{"ToInteger(2.5 [m])", ErrTypeMismatch, "function QuantityCalculations::ToInteger requires a whole magnitude, 2.5 [m] has none"},
		{"sin(90 [m])", ErrTypeMismatch, `function TrigFunctions::sin parameter "theta" requires a number of radians or an angle quantity, got a quantity in m`},
		{"arcsin(1 [rad])", ErrTypeMismatch, `function TrigFunctions::arcsin parameter "x" requires a numeric value, got a quantity in rad`},
		{"IntegerFunctions::abs(1 [rad])", ErrTypeMismatch, `function IntegerFunctions::abs parameter "x" requires a numeric value, got a quantity in rad`},
		{"RealFunctions::sqrt(4 ['°'])", ErrTypeMismatch, `function RealFunctions::sqrt parameter "x" requires a numeric value, got a quantity in °`},
		{"NaturalFunctions::max(1 [rad], 2)", ErrTypeMismatch, `function NaturalFunctions::max parameter "x" requires a numeric value`},
		{"OpenSysMLMathFunctions::exp(1 [rad])", ErrTypeMismatch, `function OpenSysMLMathFunctions::exp parameter "x" requires a numeric value`},
		{"ConvertQuantity(1 [m], 2 [cm])", ErrUnevaluableLibraryFunction, "QuantityCalculations::ConvertQuantity: a measurement reference is a library declaration"},
		{"QuantityCalculations::'['(1, 2)", ErrUnevaluableLibraryFunction, "QuantityCalculations::[: a measurement reference"},
		{"MeasurementRefCalculations::'*'(1, 2)", ErrUnevaluableLibraryFunction, "MeasurementRefCalculations::*: a measurement reference"},
		{"MeasurementRefCalculations::ToString(1)", ErrUnevaluableLibraryFunction, "MeasurementRefCalculations::ToString"},
		{"VectorCalculations::outer((1.0, 2.0), (3.0, 4.0))", ErrUnevaluableLibraryFunction, "VectorCalculations::outer: a tensor quantity has no representation"},
		{"VectorCalculations::scalarQuantityVectorMult(2 [m], (1.0, 2.0))", ErrUnevaluableLibraryFunction, "VectorCalculations::scalarQuantityVectorMult: a vector quantity has no representation"},
		{"VectorCalculations::transform(1, (1.0, 2.0))", ErrUnevaluableLibraryFunction, "VectorCalculations::transform: a coordinate transformation has no representation"},
		{"TensorCalculations::'+'(1, 2)", ErrUnevaluableLibraryFunction, "TensorCalculations::+: a tensor quantity has no representation"},
		{"TensorCalculations::tensorTensorMult(1, 2)", ErrUnevaluableLibraryFunction, "TensorCalculations::tensorTensorMult"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			got, err := evalIn(t, ctx, scope, tc.src)
			if err == nil {
				t.Fatalf("%s = %s, want error %v", tc.src, FormatValue(got), tc.want)
			}
			if !errors.Is(err, tc.want) {
				t.Errorf("%s: error %v, want %v", tc.src, err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.text) {
				t.Errorf("%s: error %q does not mention %q", tc.src, err, tc.text)
			}
		})
	}
}

// testQuantityCalculationThatHasNoValue: a calculation with no answer (a root no unit
// has, a value no representation holds) is a typed error, never a made-up unit or a NaN.
func testQuantityCalculationThatHasNoValue(t *testing.T) {
	ctx, scope := quantityCalculationsContext(t)
	for _, tt := range []struct {
		expr string
		want error
	}{
		{"sqrt(side)", ErrUnitRoot},
		{"sqrt(-1 * area)", semantics.ErrArithmeticDomain},
		{"sqrt(side, side)", ErrCalcArity},
		{"max(side, 1 [s])", ErrIncommensurableUnits},
		{"QuantityCalculations::'/'(side, 0 [s])", ErrDivisionByZero},
		{"ConvertQuantity(side, side)", ErrUnevaluableLibraryFunction},
		{"VectorCalculations::outer((1.0, 2.0), (3.0, 4.0))", ErrUnevaluableLibraryFunction},
		{"TensorCalculations::isZeroTensorQuantity(side)", ErrUnevaluableLibraryFunction},
	} {
		got, err := evalIn(t, ctx, scope, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
}

// TestVectorCalculations: the VectorCalculations functions the vector
// representation carries compute as their VectorFunctions counterparts.
func TestVectorCalculations(t *testing.T) {
	ctx, scope := quantityCalculationsContext(t)

	cases := []struct {
		src  string
		want string
	}{
		{"VectorCalculations::isZeroVectorQuantity((0.0, 0.0))", "true"},
		{"VectorCalculations::isZeroVectorQuantity((0.0, 1.0))", "false"},
		{"VectorCalculations::isUnitVectorQuantity((0.0, 1.0))", "true"},
		{"VectorCalculations::isUnitVectorQuantity((3.0, 4.0))", "false"},
		{"VectorCalculations::'+'((1.0, 2.0), (3.0, 4.0))", "[4.0, 6.0]"},
		{"VectorCalculations::'-'((3.0, 4.0), (1.0, 2.0))", "[2.0, 2.0]"},
		{"VectorCalculations::scalarVectorMult(2.0, (1.0, 2.0))", "[2.0, 4.0]"},
		{"VectorCalculations::vectorScalarMult((1.0, 2.0), 2.0)", "[2.0, 4.0]"},
		{"VectorCalculations::vectorScalarDiv((2.0, 4.0), 2.0)", "[1.0, 2.0]"},
		{"VectorCalculations::inner((1.0, 2.0), (3.0, 4.0))", "11.0"},
		{"VectorCalculations::norm((3.0, 4.0))", "5.0"},
		{"VectorCalculations::angle((1.0, 0.0), (0.0, 1.0)) > 1.5707", "true"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			got, err := evalIn(t, ctx, scope, tc.src)
			if err != nil {
				t.Fatalf("%s: %v", tc.src, err)
			}
			if rendered := FormatValue(got); rendered != tc.want {
				t.Errorf("%s = %s, want %s", tc.src, rendered, tc.want)
			}
		})
	}
}

// TestQuantitySumWithAndWithoutImport: `sum` over quantities computes the same whether
// it resolves to NumericalFunctions::sum or, with the import, QuantityCalculations::sum.
func TestQuantitySumWithAndWithoutImport(t *testing.T) {
	for _, imported := range []bool{false, true} {
		name := "without import"
		importLine := "public import NumericalFunctions::*;"
		if imported {
			name = "with import"
			importLine = "public import QuantityCalculations::*;"
		}
		t.Run(name, func(t *testing.T) {
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
				package test {
					public import ISQ::*;
					public import SI::*;
					`+importLine+`
				}
			`))
			pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
			if !ok || pkg.Scope == nil {
				t.Fatal("test package not indexed")
			}
			got, err := evalIn(t, ctx, pkg.Scope, "(1 [m], 2 [m])->sum()")
			if err != nil {
				t.Fatalf("(1 [m], 2 [m])->sum(): %v", err)
			}
			if rendered := FormatValue(got); rendered != "3 [m]" {
				t.Errorf("(1 [m], 2 [m])->sum() = %s, want 3 [m]", rendered)
			}
		})
	}
}

// TestQuantityPowerAndProductAgree: l*l and l**2 are the same quantity in the
// same rendering, magnitude kind included.
func TestQuantityPowerAndProductAgree(t *testing.T) {
	ctx, scope := quantityCalculationsContext(t)

	cases := []struct{ product, power, want string }{
		{"side * side", "side ** 2", "9 [m**2]"},
		{"side * side / side", "side ** 2 / side", "3.0 [m]"},
		{"2.5 [m] * 2.5 [m]", "2.5 [m] ** 2", "6.25 [m**2]"},
		{"2 [km/h] * 2 [km/h]", "2 [km/h] ** 2", "4 [km**2/h**2]"},
	}
	for _, tc := range cases {
		t.Run(tc.power, func(t *testing.T) {
			product, err := evalIn(t, ctx, scope, tc.product)
			if err != nil {
				t.Fatalf("%s: %v", tc.product, err)
			}
			power, err := evalIn(t, ctx, scope, tc.power)
			if err != nil {
				t.Fatalf("%s: %v", tc.power, err)
			}
			if got := FormatValue(product); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.product, got, tc.want)
			}
			if got := FormatValue(power); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.power, got, tc.want)
			}
			if product.Quantity().Num.Kind != power.Quantity().Num.Kind {
				t.Errorf("%s has magnitude kind %v, %s has %v", tc.product, product.Quantity().Num.Kind, tc.power, power.Quantity().Num.Kind)
			}
		})
	}
}
