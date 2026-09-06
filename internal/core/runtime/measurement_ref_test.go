package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// measurementRefContext evaluates expressions in a package that imports the
// units, the quantity calculations and the measurement-reference calculations.
func measurementRefContext(t *testing.T) (*Context, *symbols.Scope) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import ISQ::*;
			public import SI::*;
			public import ScalarValues::*;
			public import QuantityCalculations::*;
			public import MeasurementRefCalculations::*;
			private import MeasurementReferences::*;
			attribute halfMetre : LengthUnit { :>> unitConversion : ConversionByConvention { :>> referenceUnit = m; :>> conversionFactor = 0.5; } }
			attribute demiMetre : LengthUnit { :>> unitConversion : ConversionByConvention { :>> referenceUnit = m; :>> conversionFactor = 1/2; } }
			attribute side : LengthValue = 3 [km];
			attribute unit : LengthUnit = m;
			attribute area : AreaUnit = m * m;
			attribute speed : SpeedUnit = km / h;
			attribute wrongUnit : LengthUnit = s;
			attribute wrongDimension : AreaUnit = m * s;
			attribute notAUnit : LengthValue = m;
			attribute vq : Quantities::VectorQuantityValue = VectorFunctions::VectorOf((1.0, 2.0, 3.0)) [m];
			attribute scaled = VectorFunctions::VectorOf((1.0, 2.0)) [m] * (2 [s]);
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return ctx, pkg.Scope
}

// TestMeasurementRefValues: a unit declaration evaluates to the measurement
// reference it names, references compose as MeasurementRefCalculations declares,
// and a quantity is built from or converted to one.
func TestMeasurementRefValues(t *testing.T) {
	ctx, scope := measurementRefContext(t)

	cases := []struct {
		src  string
		want string
	}{
		{"m", "m"},
		{"SI::m", "m"},
		{"SI::km", "km"},
		{"SI::'m/s'", "'m/s'"},
		{"MeasurementReferences::one", "one"},
		{"unit", "m"},
		{"m * s", "m*s"},
		{"m / s", "m/s"},
		{"m ** 2", "m**2"},
		{"m ** 0.5", "m**0.5"},
		{"km / m", "km/m"},
		{"m / m", "1"},
		{"area", "m**2"},
		{"speed", "km/h"},
		{"MeasurementRefCalculations::'*'(m, s)", "m*s"},
		{"MeasurementRefCalculations::'/'(m, s)", "m/s"},
		{"MeasurementRefCalculations::'**'(m, 2)", "m**2"},
		{"MeasurementRefCalculations::'^'(m, 3)", "m**3"},
		{"MeasurementRefCalculations::ToString(km)", `"km"`},
		{"ToString(m / s)", `"m/s"`},
		{"ToString(SI::'m/s')", `"'m/s'"`},
		{"QuantityCalculations::'['(3, m)", "3 [m]"},
		{"'['(2.5, km)", "2.5 [km]"},
		{"'['(2, m / s)", "2 [m/s]"},
		{"ConvertQuantity(3 [km], m)", "3000.0 [m]"},
		{"ConvertQuantity(300 [cm], m)", "3.0 [m]"},
		{"ConvertQuantity(3 [m], cm)", "300.0 [cm]"},
		{"ConvertQuantity(3 [km], km)", "3 [km]"},
		{"ConvertQuantity(side, m)", "3000.0 [m]"},
		{"ConvertQuantity(2 [m/s], SI::'km/h')", "7.2 ['km/h']"},
		{"ConvertQuantity(1 [h], s)", "3600.0 [s]"},
		{"ConvertQuantity(1 [m*m], m ** 2)", "1 [m**2]"},
		{"side.num", "3"},
		{"side.mRef", "km"},
		{"(2.5 [m/s]).mRef", "m/s"},
		{"(2.5 [m/s]).num", "2.5"},
		{"side.rank", "0"},
		{"side.dimensions", "[]"},
		{"side.elements", "[3]"},
		{"side.order", "0"},
		{"vq.mRef", "m"},
		{"vq.num", "[1.0, 2.0, 3.0]"},
		{"scaled.mRef", "m*s"},
		{"scaled.mRef == m * s", "true"},
		{"m.rank", "0"},
		{"m.dimensions", "[]"},
		{"m.flattenedSize", "1"},
		{"m.elements", "[m]"},
		{"m.mRefs", "[m]"},
		{"m.isBound", "false"},
		{"m.isOrthogonal", "true"},
		{"side.mRef.isBound", "false"},
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

// TestMeasurementRefEquality: references are equal at one reduction and one
// scale, dimension-one units by spelling, and a set keys them the same way.
func TestMeasurementRefEquality(t *testing.T) {
	ctx, scope := measurementRefContext(t)

	cases := []struct {
		left, right string
		want        bool
	}{
		{"m", "SI::m", true},
		{"m", "unit", true},
		{"side.mRef", "km", true},
		{"side.mRef", "m", false},
		{"km", "m", false},
		{"SI::'m/s'", "m / s", true},
		{"SI::'km/h'", "km / h", true},
		{"SI::'km/h'", "m / s", false},
		{"m ** 2", "m * m", true},
		{"area", "m ** 2", true},
		{"m / m", "s / s", true},
		{"m / m", "MeasurementReferences::one", false},
		{"rad", "rad", true},
		{"rad", "sr", false},
		{"m", "3", false},
		{"m", `"m"`, false},
		{"m", "1 [m]", false},
		{"vq.mRef", "m", true},
		{"(1 [m]).mRef", "(2 [m]).mRef", true},
		{"halfMetre", "demiMetre", true},
		{"halfMetre", "m", false},
		{"halfMetre / s", "demiMetre / s", true},
	}
	for _, tc := range cases {
		t.Run(tc.left+" == "+tc.right, func(t *testing.T) {
			left, err := evalIn(t, ctx, scope, tc.left)
			if err != nil {
				t.Fatalf("%s: %v", tc.left, err)
			}
			right, err := evalIn(t, ctx, scope, tc.right)
			if err != nil {
				t.Fatalf("%s: %v", tc.right, err)
			}
			if got := valueEqual(left, right); got != tc.want {
				t.Errorf("valueEqual(%s, %s) = %v, want %v", tc.left, tc.right, got, tc.want)
			}
			if got := valueEqual(right, left); got != tc.want {
				t.Errorf("valueEqual(%s, %s) = %v, want %v", tc.right, tc.left, got, tc.want)
			}
			if sameKey := valueKeyFunc(left) == valueKeyFunc(right); sameKey != tc.want {
				t.Errorf("valueKey(%s) == valueKey(%s) is %v, want %v", tc.left, tc.right, sameKey, tc.want)
			}
			got, err := evalIn(t, ctx, scope, tc.left+" == "+tc.right)
			if err != nil {
				t.Fatalf("%s == %s: %v", tc.left, tc.right, err)
			}
			if FormatValue(got) != FormatValue(boolValue(tc.want)) {
				t.Errorf("%s == %s = %s, want %v", tc.left, tc.right, FormatValue(got), tc.want)
			}
		})
	}
}

// TestMeasurementRefClassification: a reference is of its declaration's type; a
// composed unit is a DerivedUnit, a unit of powers of other units.
func TestMeasurementRefClassification(t *testing.T) {
	ctx, scope := measurementRefContext(t)

	cases := []struct {
		src  string
		want string
	}{
		{"m istype LengthUnit", "true"},
		{"m istype DurationUnit", "false"},
		{"m istype MeasurementReferences::ScalarMeasurementReference", "true"},
		{"m istype MeasurementReferences::MeasurementUnit", "true"},
		{"m istype MeasurementReferences::TensorMeasurementReference", "true"},
		{"m hastype LengthUnit", "true"},
		{"m hastype MeasurementReferences::TensorMeasurementReference", "false"},
		{"side.mRef istype LengthUnit", "true"},
		{"(m * s) istype MeasurementReferences::MeasurementUnit", "true"},
		{"(m * s) istype MeasurementReferences::DerivedUnit", "true"},
		{"m istype MeasurementReferences::DerivedUnit", "false"},
		{"(m * s) istype LengthUnit", "false"},
		{"(m * m) istype AreaUnit", "false"},
		{"(m * m) @ MeasurementReferences::MeasurementUnit", "true"},
		{"m istype ScalarValues::Real", "false"},
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

// TestMeasurementRefReport: what a reference cannot answer is a typed error
// naming the declaration or the operator, never a number or a made-up member.
func TestMeasurementRefReport(t *testing.T) {
	ctx, scope := measurementRefContext(t)

	cases := []struct {
		src  string
		want error
		text string
	}{
		{"wrongUnit", ErrTypeMismatch, "cannot write the measurement reference s, a measurement reference of dimension T, to a feature typed by LengthUnit"},
		{"wrongDimension", ErrTypeMismatch, "cannot write the measurement reference m*s, a measurement reference of dimension L·T, to a feature typed by AreaUnit"},
		{"notAUnit", ErrTypeMismatch, "cannot write the measurement reference m, a measurement reference typed LengthUnit, to a feature typed by LengthValue"},
		{"m + m", ErrTypeMismatch, "operator '+' is not defined for a measurement reference and a measurement reference"},
		{"m - s", ErrTypeMismatch, "operator '-' is not defined for a measurement reference and a measurement reference"},
		{"m * 3", ErrTypeMismatch, "operator '*' is not defined for a measurement reference and an Integer"},
		{"3 * m", ErrTypeMismatch, "operator '*' is not defined for an Integer and a measurement reference"},
		{"m / 2.0", ErrTypeMismatch, "operator '/' is not defined for a measurement reference and a Real"},
		{"m ** s", ErrTypeMismatch, "operator '**' is not defined for a measurement reference and a measurement reference"},
		{"-m", ErrTypeMismatch, "unary '-' requires numeric operand, got measurement reference"},
		{"m < s", nil, "comparison operands must be constants, got measurement reference and measurement reference"},
		{"m.unitConversion", ErrUnevaluableLibraryFunction, "MeasurementReferences::ScalarMeasurementReference::unitConversion: a measurement reference value holds the unit m and its reduction metre, not the declaration's member unitConversion"},
		{"m.quantityDimension", ErrUnevaluableLibraryFunction, "MeasurementReferences::ScalarMeasurementReference::quantityDimension: a measurement reference value holds the unit m"},
		{"m.definitionalQuantityValues", ErrUnevaluableLibraryFunction, "not the declaration's member definitionalQuantityValues"},
		{"(m / s).quantityDimension", ErrUnevaluableLibraryFunction, "a measurement reference value holds the unit m/s and its reduction metre·second^-1"},
		{"m.foo", ErrTypeMismatch, "measurement reference has no feature foo"},
		{"side.isBound", ErrUnevaluableLibraryFunction, "Quantities::ScalarQuantityValue::isBound: a scalar quantity value holds its num and mRef, not whether the quantity is bound"},
		{"side.quantityDimension", ErrTypeMismatch, "a quantity in km has no feature quantityDimension"},
		{"ConvertQuantity(side, s)", ErrIncommensurableUnits, "function QuantityCalculations::ConvertQuantity: incommensurable units: cannot express km (1000·metre) in s (second)"},
		{"ConvertQuantity(1 [m], SI::'m/s')", ErrIncommensurableUnits, "cannot express m (metre) in 'm/s' (metre·second^-1)"},
		{"ConvertQuantity(3, m)", ErrIncommensurableUnits, "cannot express 1 (1) in m (metre)"},
		{"ConvertQuantity(m, m)", ErrTypeMismatch, `function QuantityCalculations::ConvertQuantity parameter "x" requires a quantity`},
		{"'['(3, 1 [m])", ErrTypeMismatch, `function QuantityCalculations::'[' parameter "mRef" requires a measurement reference such as SI::m, got a quantity in m`},
		{"'['(\"3\", m)", ErrTypeMismatch, `function QuantityCalculations::'[' parameter "num" requires a numeric value`},
		{"MeasurementRefCalculations::'/'(m, 2)", ErrTypeMismatch, `function MeasurementRefCalculations::'/' parameter "y" requires a measurement reference such as SI::m, got an Integer`},
		{"MeasurementRefCalculations::'^'(m, s)", ErrTypeMismatch, `function MeasurementRefCalculations::'^' parameter "y" requires a numeric value`},
		{"MeasurementRefCalculations::ToString(1 [m])", ErrTypeMismatch, `function MeasurementRefCalculations::ToString parameter "x" requires a measurement reference such as SI::m, got a quantity in m`},
		{"MeasurementRefCalculations::'CoordinateFrame*'(m, s)", ErrUnevaluableLibraryFunction, "MeasurementRefCalculations::'CoordinateFrame*'"},
		{"MeasurementRefCalculations::'CoordinateFrame/'(m, s)", ErrUnevaluableLibraryFunction, "MeasurementRefCalculations::'CoordinateFrame/'"},
		{"VectorCalculations::'['((1.0, 2.0), m)", ErrUnevaluableLibraryFunction, "VectorCalculations::'['"},
		{"VectorCalculations::transform(m, vq)", ErrUnevaluableLibraryFunction, "VectorCalculations::transform: a CoordinateTransformation has no representation"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			got, err := evalIn(t, ctx, scope, tc.src)
			if err == nil {
				t.Fatalf("%s = %s, want error %v", tc.src, FormatValue(got), tc.want)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("%s: error %v, want %v", tc.src, err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.text) {
				t.Errorf("%s: error %q does not mention %q", tc.src, err, tc.text)
			}
		})
	}
}

// TestVectorQuantityMRefNeedsOneUnit: a vector quantity whose axes carry
// different units has no one scalar reference, so mRef is a typed error.
func TestVectorQuantityMRefNeedsOneUnit(t *testing.T) {
	ctx, scope := measurementRefContext(t)
	metre, err := evalIn(t, ctx, scope, "m")
	if err != nil {
		t.Fatalf("m: %v", err)
	}
	second, err := evalIn(t, ctx, scope, "s")
	if err != nil {
		t.Fatalf("s: %v", err)
	}
	num := []semantics.Value{{Kind: semantics.ValReal, Real: 1}, {Kind: semantics.ValReal, Real: 2}}

	uniform := NewVectorQuantityValue(num, []Unit{metre.MeasurementRef().Unit, metre.MeasurementRef().Unit})
	got, ok, err := ctx.structuredFeature(uniform, "mRef")
	if !ok || err != nil || !valueEqual(got, metre) {
		t.Fatalf("mRef of %s = %s, %v, %v; want m", FormatValue(uniform), FormatValue(got), ok, err)
	}

	mixed := NewVectorQuantityValue(num, []Unit{metre.MeasurementRef().Unit, second.MeasurementRef().Unit})
	_, ok, err = ctx.structuredFeature(mixed, "mRef")
	if !ok || !errors.Is(err, ErrUnevaluableLibraryFunction) {
		t.Fatalf("mRef of %s = %v, %v; want %v", FormatValue(mixed), ok, err, ErrUnevaluableLibraryFunction)
	}
	if want := "Quantities::VectorQuantityValue::mRef: the axes of ⟨1.0 [m], 2.0 [s]⟩ carry different units, and no one measurement reference names them all"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not mention %q", err, want)
	}
}

// TestMeasurementRefDescribed: the kind describes, renders and traces itself.
func TestMeasurementRefDescribed(t *testing.T) {
	ctx, scope := measurementRefContext(t)
	ref, err := evalIn(t, ctx, scope, "km / h")
	if err != nil {
		t.Fatalf("km / h: %v", err)
	}
	if got := describeOperand(ref); got != "a measurement reference" {
		t.Errorf("describeOperand(km/h) = %q", got)
	}
	if got := describeValue(ref); got != "measurement reference" {
		t.Errorf("describeValue(km/h) = %q", got)
	}
	if got := FormatValue(ref); got != "km/h" {
		t.Errorf("FormatValue(km/h) = %q", got)
	}
	if got := ref.Kind.String(); got != "measurement reference" {
		t.Errorf("Kind.String() = %q", got)
	}
	if ref.MeasurementRef().Declaration() != nil {
		t.Error("km/h names a single declaration")
	}
	single, err := evalIn(t, ctx, scope, "SI::km")
	if err != nil {
		t.Fatalf("SI::km: %v", err)
	}
	if decl := single.MeasurementRef().Declaration(); decl == nil || unitSymbolName(decl) != "km" {
		t.Errorf("SI::km names the declaration %v, want km", decl)
	}
	var nilRef *MeasurementRef
	if got := nilRef.String(); got != "<unknown>" {
		t.Errorf("nil reference renders %q", got)
	}
	if nilRef.equal(nilRef) != true || nilRef.equal(ref.MeasurementRef()) {
		t.Error("nil reference equality is not identity")
	}
}
