package repl

import "testing"

// measurementRefSession declares a part measured in kilometres whose unit
// feature holds a measurement reference, with the SI units in scope.
func measurementRefSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	res := s.Submit(`package Survey {
		private import ISQ::*;
		private import SI::*;
		private import MeasurementReferences::*;
		part def Field {
			attribute width : LengthValue = 3 [km];
			attribute unit : LengthUnit = m;
			attribute area : AreaUnit = m * m;
		}
		part field : Field;
	}`)
	if len(res.Diagnostics) > 0 {
		t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
	}
	return s
}

// A unit named at the prompt is the measurement reference it declares, as is
// one composed from several; the reference of a quantity reads and compares.
func TestEvalMeasurementReferences(t *testing.T) {
	s := measurementRefSession(t)
	wants(t, run(t, s, "%eval m"), "✓ m", "= m")
	wants(t, run(t, s, "%eval SI::'m/s'"), "= 'm/s'")
	wants(t, run(t, s, "%eval km * s / m"), "= km*s/m")
	wants(t, run(t, s, "%eval field.unit"), "= m")
	wants(t, run(t, s, "%eval field.area"), "= m**2")
	wants(t, run(t, s, "%eval field.width.mRef"), "= km")
	wants(t, run(t, s, "%eval field.width.num"), "= 3")
	wants(t, run(t, s, "%eval field.width.mRef == km"), "= true")
	wants(t, run(t, s, "%eval field.width.mRef == field.unit"), "= false")
	wants(t, run(t, s, "%eval SI::'m/s' == m / s"), "= true")
}

// The quantity calculations compute over a reference: a number takes a unit,
// a quantity converts to one, and a reference renders as its unit text.
func TestEvalQuantityCalculationsOverReferences(t *testing.T) {
	s := measurementRefSession(t)
	wants(t, run(t, s, "%eval QuantityCalculations::ConvertQuantity(field.width, field.unit)"), "= 3000.0 [m]")
	wants(t, run(t, s, "%eval QuantityCalculations::ConvertQuantity(2 [m/s], SI::'km/h')"), "= 7.2 ['km/h']")
	wants(t, run(t, s, "%eval QuantityCalculations::'['(2.5, km)"), "= 2.5 [km]")
	wants(t, run(t, s, `%eval MeasurementRefCalculations::ToString(m ** 2)`), `= "m**2"`)
	wants(t, run(t, s, "%eval QuantityCalculations::ConvertQuantity(field.width, s)"),
		"error:", "incommensurable units")
}

// What the runtime does not hold about a unit is a typed failure naming the
// declaration, never a made-up value: the declaration's own members, and the
// operators the library defines over no reference.
func TestEvalMeasurementReferenceLimitsAreTyped(t *testing.T) {
	s := measurementRefSession(t)
	wants(t, run(t, s, "%eval m.unitConversion"),
		"error:", "library function is not evaluable",
		"MeasurementReferences::ScalarMeasurementReference::unitConversion")
	wants(t, run(t, s, "%eval m + m"), "error:", "operator '+' is not defined for a measurement reference")
	wants(t, run(t, s, "%eval m * 3"), "error:", "operator '*' is not defined for a measurement reference and an Integer")
}

// %features lists the reference an attribute holds beside the quantities.
func TestFeaturesListMeasurementReferences(t *testing.T) {
	s := measurementRefSession(t)
	run(t, s, "%instantiate Survey::field")
	wants(t, run(t, s, "%features Survey::field"), "\n  width = 3 [km]", "\n  unit = m", "\n  area = m**2")
}
