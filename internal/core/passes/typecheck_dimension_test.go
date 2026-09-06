package passes

import "testing"

// dimensionDiags reports the type-tier diagnostics of a model written against
// the bundled quantity libraries.
func dimensionDiags(t *testing.T, body string) []Diagnostic {
	t.Helper()
	return libraryTypeDiags(t, "package P {\n"+
		"private import ISQ::*;\nprivate import SI::*;\n"+body+"\n}")
}

func wantOneDimensionError(t *testing.T, body, want string) {
	t.Helper()
	diags := dimensionDiags(t, body)
	if len(diags) != 1 {
		t.Fatalf("want exactly one type diagnostic, got %v", diags)
	}
	if diags[0].Severity != SeverityError {
		t.Errorf("diagnostic is %v, want an error: %s", diags[0].Severity, diags[0].Message)
	}
	if diags[0].Message != want {
		t.Errorf("message = %q, want %q", diags[0].Message, want)
	}
}

func wantNoDimensionDiags(t *testing.T, body string) {
	t.Helper()
	if diags := dimensionDiags(t, body); len(diags) != 0 {
		t.Fatalf("want no type diagnostics, got %v", diags)
	}
}

// TestBoundQuantityOfAnotherDimension: the initial value's unit measures in a
// dimension the target's quantity value type does not, and the message names
// both.
func TestBoundQuantityOfAnotherDimension(t *testing.T) {
	wantOneDimensionError(t, `attribute bad : DurationValue = 5 [m];`,
		"cannot bind m (dimension L) to a feature typed by DurationValue (dimension T)")
}

// TestBoundNestedCollectionQuantityOfAnotherDimension: a collection binds flat,
// so a unit inside a nested literal is measured against the target too.
func TestBoundNestedCollectionQuantityOfAnotherDimension(t *testing.T) {
	wantOneDimensionError(t, `attribute ls : LengthValue[*] = (1 [m], (2 [m], 3 [s]));`,
		"cannot bind s (dimension T) to a feature typed by LengthValue (dimension L)")
	wantNoDimensionDiags(t, `attribute ls : LengthValue[*] = (1 [m], (2 [m], 3 [mm]));`)
}

// TestBoundQuantityOfTheSameDimensionAtAnotherScale: a dimension has no scale,
// so any unit measuring in it conforms.
func TestBoundQuantityOfTheSameDimensionAtAnotherScale(t *testing.T) {
	wantNoDimensionDiags(t, `attribute t : DurationValue = 5 [min];`)
}

// TestBoundQuantityComposedToItsDimension: a composed unit is judged by the
// dimension it reduces to, not by how it was written.
func TestBoundQuantityComposedToItsDimension(t *testing.T) {
	wantNoDimensionDiags(t, `attribute v : SpeedValue = 10 [m] / 2 [s];`)
	wantOneDimensionError(t, `attribute t : DurationValue = 10 [m] / 2 [s];`,
		"cannot bind a value of dimension L·T^-1 to a feature typed by DurationValue (dimension T)")
}

// TestAssignedQuantityOfAnotherDimension: an assignment is judged where a bound
// value is, so the assign walker reports the same clash.
func TestAssignedQuantityOfAnotherDimension(t *testing.T) {
	wantOneDimensionError(t, `part def Host {
		attribute wrong : DurationValue = 0 [s];
		perform action b { action step { assign wrong := 3 [m/s]; } first step; }
	}`, "cannot bind m/s (dimension L·T^-1) to a feature typed by DurationValue (dimension T)")
}

// TestAssignedQuantityOfTheSameDimension: a commensurable assignment stands.
func TestAssignedQuantityOfTheSameDimension(t *testing.T) {
	wantNoDimensionDiags(t, `part def Host {
		attribute t : DurationValue = 0 [s];
		perform action b { action step { assign t := 3 [min]; } first step; }
	}`)
}

// TestBoundQuantityThroughAnAlias: a quantity kind named through an alias fixes
// the dimension the aliased type does.
func TestBoundQuantityThroughAnAlias(t *testing.T) {
	wantNoDimensionDiags(t, `attribute k : ISQ::TemperatureValue = 5 [K];`)
	wantOneDimensionError(t, `attribute k : ISQ::TemperatureValue = 5 [m];`,
		"cannot bind m (dimension L) to a feature typed by ThermodynamicTemperatureValue (dimension Θ)")
}

// TestBoundQuantityWhereTheTargetFixesNoDimension: a target that declares no
// quantity kind, and one whose measurement reference is any unit, state nothing
// a value must answer to.
func TestBoundQuantityWhereTheTargetFixesNoDimension(t *testing.T) {
	wantNoDimensionDiags(t, `attribute r : ScalarValues::Real = 5;`)
	wantNoDimensionDiags(t, `attribute q : Quantities::ScalarQuantityValue = 5 [m];`)
	wantNoDimensionDiags(t, `attribute u = 5 [m];`)
}

// TestBoundBareNumberStatesNoDimension: a plain number names no unit, so it
// determines no dimension and the dimensional rule stays silent — the type
// conformance it already answered to is unchanged.
func TestBoundBareNumberStatesNoDimension(t *testing.T) {
	wantOneDimensionError(t, `attribute t : DurationValue = 5;`,
		"cannot bind Natural value to a feature typed by DurationValue")
	wantNoDimensionDiags(t, `attribute r : ScalarValues::Real = 5;`)
}

// TestBoundDimensionlessQuantity: DimensionOneValue fixes the dimension of a
// count, so a metre is refused there as a second is, and a ratio of lengths —
// which measures in it — is not.
func TestBoundDimensionlessQuantity(t *testing.T) {
	wantOneDimensionError(t, `attribute n : MeasurementReferences::DimensionOneValue = 5 [m];`,
		"cannot bind m (dimension L) to a feature typed by DimensionOneValue (dimensionless)")
	wantNoDimensionDiags(t, `attribute n : MeasurementReferences::DimensionOneValue = 10 [m] / 5 [m];`)
	wantOneDimensionError(t, `attribute t : DurationValue = 10 [m] / 5 [m];`,
		"cannot bind a dimensionless value to a feature typed by DurationValue (dimension T)")
}

// TestRecursiveRollupThroughACall: a feature whose value calls a function on a
// chain back to itself — the mass rollup `mass + sum(subcomponents.totalMass)` —
// types in finite time: the argument is left untyped where it leads back to the
// call being typed, rather than typing the call again.
func TestRecursiveRollupThroughACall(t *testing.T) {
	wantNoDimensionDiags(t, `private import NumericalFunctions::*;
	part def MassedComponent {
		part subcomponents : MassedComponent [*] default null;
		attribute mass :> ISQ::mass;
		attribute totalMass :> ISQ::mass = mass + sum(subcomponents.totalMass);
	}`)
}

// TestBoundMeasurementUnit: a unit binds to the unit definition typing it, to any
// measurement-reference supertype, and to no quantity value type; the checker
// judges each as the runtime's write conformance does. A quantity bound to a
// unit-typed feature (`hp : PowerUnit = 745.7 [W]` in the OMG examples) is not
// judged, as the pilot implementation accepts it.
func TestBoundMeasurementUnit(t *testing.T) {
	wantNoDimensionDiags(t, `attribute u : LengthUnit = m;`)
	wantNoDimensionDiags(t, `attribute u : LengthUnit = km;`)
	wantNoDimensionDiags(t, `attribute u : MeasurementReferences::ScalarMeasurementReference = m;`)
	wantNoDimensionDiags(t, `attribute u : MeasurementReferences::MeasurementUnit = m;`)
	wantOneDimensionError(t, `attribute u : LengthUnit = s;`,
		"cannot bind a value of type DurationUnit to a feature typed by LengthUnit")
	wantOneDimensionError(t, `attribute q : LengthValue = m;`,
		"cannot bind a value of type LengthUnit to a feature typed by LengthValue")
	wantNoDimensionDiags(t, `attribute u : PowerUnit = 745.7 [W];`)
}

// TestBoundComposedMeasurementUnit: a product, quotient or power of units is a
// DerivedUnit of the composed dimension, so it binds to a unit definition of
// that dimension and is refused by one of another with the dimension named.
func TestBoundComposedMeasurementUnit(t *testing.T) {
	wantNoDimensionDiags(t, `attribute a : AreaUnit = m * m;`)
	wantNoDimensionDiags(t, `attribute kpl : MeasurementReferences::DerivedUnit = km / L;`)
	wantNoDimensionDiags(t, `attribute v : SpeedUnit = m / s;`)
	wantNoDimensionDiags(t, `attribute v : SpeedUnit = km / h;`)
	wantNoDimensionDiags(t, `attribute c : VolumeUnit = m ** 3;`)
	wantNoDimensionDiags(t, `attribute u : MeasurementReferences::MeasurementUnit = m / s;`)
	wantOneDimensionError(t, `attribute a : AreaUnit = m * s;`,
		"cannot bind a measurement reference of dimension L·T to a feature typed by AreaUnit")
	wantOneDimensionError(t, `attribute u : LengthUnit = m / s;`,
		"cannot bind a measurement reference of dimension L·T^-1 to a feature typed by LengthUnit")
	// A quantity value type of the same dimension is no type of a unit: the
	// runtime refuses the write, so the checker refuses the binding.
	wantOneDimensionError(t, `attribute a : AreaValue = m * m;`,
		"cannot bind a measurement reference typed DerivedUnit to a feature typed by AreaValue")
	wantOneDimensionError(t, `attribute v : SpeedValue = km / h;`,
		"cannot bind a measurement reference typed DerivedUnit to a feature typed by SpeedValue")
	wantNoDimensionDiags(t, `attribute a : AreaValue = 2 [m] * 3 [m];`)
	wantNoDimensionDiags(t, `attribute n : ScalarValues::Natural = 1 * 1;`)
}

// TestComposedMeasurementUnitAsAnArgument: an operator expression over units is
// a measurement reference where an overload is chosen by argument type, so
// ToString(m / s) selects MeasurementRefCalculations::ToString and a quantity
// parameter refuses it by name; one over numbers (`1 * 1`) is the number it
// evaluates to, not the DerivedUnit unit notation would read it as.
func TestComposedMeasurementUnitAsAnArgument(t *testing.T) {
	wantNoDimensionDiags(t, `private import MeasurementRefCalculations::*;
		attribute text : ScalarValues::String = ToString(m / s);`)
	wantNoDimensionDiags(t, `private import QuantityCalculations::*;
		attribute q : LengthValue = ConvertQuantity(3 [km], m);`)
	wantNoDimensionDiags(t, `private import QuantityCalculations::*;
		attribute q : LengthValue = ConvertQuantity(3 [km], km / m * m);`)
	wantOneDimensionError(t, `private import QuantityCalculations::*;
		attribute q : LengthValue = ConvertQuantity(m, m);`,
		"argument 1 of ConvertQuantity expects ScalarQuantityValue, found LengthUnit")
	wantOneDimensionError(t, `private import QuantityCalculations::*;
		attribute q : LengthValue = ConvertQuantity(m / m, m);`,
		"argument 1 of ConvertQuantity expects ScalarQuantityValue, found DerivedUnit")
	wantNoDimensionDiags(t, `private import QuantityCalculations::*;
		attribute q : LengthValue = ConvertQuantity(1 * 1, m);`)
	wantNoDimensionDiags(t, `private import QuantityCalculations::*;
		attribute q : LengthValue = ConvertQuantity(1 / 1, m);`)
	wantNoDimensionDiags(t, `private import QuantityCalculations::*;
		attribute q : LengthValue = ConvertQuantity(1 ** 2, m);`)
}
