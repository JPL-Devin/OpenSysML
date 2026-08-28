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
