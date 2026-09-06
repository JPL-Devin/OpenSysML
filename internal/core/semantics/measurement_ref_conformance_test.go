package semantics_test

import (
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

// measurementRefModel indexes the libraries with a package whose features are
// bound to unit expressions, so both a unit and an expression over units resolve.
func measurementRefModel(t *testing.T) (*semantics.Model, *symbols.Index) {
	t.Helper()
	idx := libs.NewModelIndex()
	idx.AddDocument("<t>", parser.New(source.New("<t>", []byte(`package T {
		private import ISQ::*;
		private import SI::*;
		attribute area = m * m;
		attribute speed = m / s;
		attribute cubed = m ** 3;
		attribute sum = m + m;
	}`))).ParseFile())
	idx.ExpandWildcardImports()
	return semantics.NewModel(resolve.New(idx)), idx
}

func boundOperatorExpr(t *testing.T, idx *symbols.Index, fqn string) (*symbols.Scope, *ast.OperatorExpr) {
	t.Helper()
	sym := dimensionSymbol(t, idx, fqn)
	u, ok := sym.Decl.(*ast.Usage)
	if !ok || u.Value == nil {
		t.Fatalf("%s is not a usage with a value", fqn)
	}
	e, ok := u.Value.(*ast.OperatorExpr)
	if !ok {
		t.Fatalf("%s is bound to a %T, want an operator expression", fqn, u.Value)
	}
	return sym.OwnerScope, e
}

// TestMeasurementRefConforms: a unit conforms to the unit definition typing it and
// to every measurement-reference supertype; a composed unit, typed DerivedUnit,
// conforms to a unit definition of its dimension and to no other; a quantity value
// type is never the type of a reference, whatever the dimension.
func TestMeasurementRefConforms(t *testing.T) {
	m, idx := measurementRefModel(t)
	metre, err := m.UnitTermOf(dimensionSymbol(t, idx, "SI::m"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.UnitTermOf(dimensionSymbol(t, idx, "SI::s"))
	if err != nil {
		t.Fatal(err)
	}
	lengthUnit := dimensionSymbol(t, idx, "ISQBase::LengthUnit")
	derivedUnit := dimensionSymbol(t, idx, "MeasurementReferences::DerivedUnit")
	cases := []struct {
		name  string
		typ   *symbols.Symbol
		unit  semantics.UnitTerm
		want  string
		holds bool
		found string
	}{
		{"m : LengthUnit", lengthUnit, metre, "ISQBase::LengthUnit", true, ""},
		{"m : ScalarMeasurementReference", lengthUnit, metre, "MeasurementReferences::ScalarMeasurementReference", true, ""},
		{"m : TensorMeasurementReference", lengthUnit, metre, "MeasurementReferences::TensorMeasurementReference", true, ""},
		{"s : LengthUnit", dimensionSymbol(t, idx, "ISQBase::DurationUnit"), second, "ISQBase::LengthUnit", false, "a measurement reference of dimension T"},
		{"m*m : AreaUnit", derivedUnit, metre.Times(metre), "ISQSpaceTime::AreaUnit", true, ""},
		{"m*m : LengthUnit", derivedUnit, metre.Times(metre), "ISQBase::LengthUnit", false, "a measurement reference of dimension L^2"},
		{"m*m : MeasurementUnit", derivedUnit, metre.Times(metre), "MeasurementReferences::MeasurementUnit", true, ""},
		{"m*m : DerivedUnit", derivedUnit, metre.Times(metre), "MeasurementReferences::DerivedUnit", true, ""},
		{"m : DerivedUnit", lengthUnit, metre, "MeasurementReferences::DerivedUnit", false, "a measurement reference typed LengthUnit"},
		{"m : LengthValue", lengthUnit, metre, "ISQBase::LengthValue", false, "a measurement reference typed LengthUnit"},
		{"m*m : AreaValue", derivedUnit, metre.Times(metre), "ISQSpaceTime::AreaValue", false, "a measurement reference typed DerivedUnit"},
		{"m : Real", lengthUnit, metre, "ScalarValues::Real", false, "a measurement reference typed LengthUnit"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := m.MeasurementRefConforms(tc.typ, tc.unit, dimensionSymbol(t, idx, tc.want))
			if !c.Known {
				t.Fatalf("conformance of %s is unknown", tc.name)
			}
			if c.Holds != tc.holds {
				t.Fatalf("conformance of %s holds = %v, want %v (found %q)", tc.name, c.Holds, tc.holds, c.Found)
			}
			if !strings.Contains(c.Found, tc.found) {
				t.Fatalf("conformance of %s found %q, want it to mention %q", tc.name, c.Found, tc.found)
			}
		})
	}
	if c := m.MeasurementRefConforms(nil, metre, lengthUnit); c.Known {
		t.Error("a reference of no type judged known")
	}
}

// TestMeasurementRefExpr: `*`, `/` and `**` over units are a DerivedUnit of the
// composed dimension; `+` over units and expressions over no unit are not references.
func TestMeasurementRefExpr(t *testing.T) {
	m, idx := measurementRefModel(t)
	derivedUnit := dimensionSymbol(t, idx, "MeasurementReferences::DerivedUnit")
	for _, tc := range []struct {
		feature string
		want    string
		holds   bool
	}{
		{"T::area", "ISQSpaceTime::AreaUnit", true},
		{"T::area", "ISQBase::LengthUnit", false},
		{"T::speed", "ISQSpaceTime::SpeedUnit", true},
		{"T::speed", "ISQBase::LengthUnit", false},
		{"T::cubed", "ISQSpaceTime::VolumeUnit", true},
	} {
		t.Run(tc.feature+" : "+tc.want, func(t *testing.T) {
			scope, e := boundOperatorExpr(t, idx, tc.feature)
			if got := m.MeasurementRefExprType(scope, e); got != derivedUnit {
				t.Fatalf("type of %s = %v, want DerivedUnit", tc.feature, got)
			}
			c, ok := m.MeasurementRefExprConformance(scope, e, dimensionSymbol(t, idx, tc.want))
			if !ok || !c.Known {
				t.Fatalf("%s against %s is not judged (%v, %v)", tc.feature, tc.want, ok, c.Known)
			}
			if c.Holds != tc.holds {
				t.Fatalf("%s conforms to %s = %v, want %v (found %q)", tc.feature, tc.want, c.Holds, tc.holds, c.Found)
			}
		})
	}
	scope, sum := boundOperatorExpr(t, idx, "T::sum")
	if got := m.MeasurementRefExprType(scope, sum); got != nil {
		t.Errorf("m + m typed %v, want no measurement reference", got)
	}
	if _, ok := m.MeasurementRefExprConformance(scope, sum, derivedUnit); ok {
		t.Error("m + m judged as a measurement reference")
	}
}
