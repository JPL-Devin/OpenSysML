package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// TestScaleStaysExact: a conversion factor is kept as a ratio, so converting at
// the boundary of a comparison answers exactly — 5.4 [km/h] is 1.5 [m/s], which
// evaluating 1000/3600 first would answer 1.4999999999999998 for.
func TestScaleStaysExact(t *testing.T) {
	kmPerHour := UnitScale(1000).DividedBy(UnitScale(3600))
	if got := ConvertMagnitude(5.4, kmPerHour, UnitScale(1)); got != 1.5 {
		t.Errorf("5.4 km/h = %v m/s, want exactly 1.5", got)
	}
	if got := ConvertMagnitude(1.5, UnitScale(1), kmPerHour); got != 5.4 {
		t.Errorf("1.5 m/s = %v km/h, want exactly 5.4", got)
	}
}

// TestScalePowInverts: a negative exponent inverts the ratio rather than
// evaluating it, which is what keeps a composed unit such as `km/h` exact.
func TestScalePowInverts(t *testing.T) {
	inv := UnitScale(3600).Pow(-1)
	if inv.Num != 1 || inv.Den != 3600 {
		t.Fatalf("3600^-1 = %v, want 1/3600", inv)
	}
	if got := UnitScale(1000).Times(inv); ConvertMagnitude(5.4, got, UnitScale(1)) != 1.5 {
		t.Errorf("1000·(1/3600) = %v, which does not convert 5.4 to 1.5", got)
	}
}

// TestScaleReduces: a whole ratio is normalized and a common divisor cancelled,
// so equal scale factors compare equal.
func TestScaleReduces(t *testing.T) {
	if got := UnitScale(1000).DividedBy(UnitScale(3600)); got != (Scale{Num: 5, Den: 18}) {
		t.Errorf("1000/3600 = %v, want 5/18", got)
	}
	if got := UnitScale(6).DividedBy(UnitScale(3)); got != UnitScale(2) {
		t.Errorf("6/3 = %v, want 2", got)
	}
	if got := UnitScale(1).DividedBy(UnitScale(-2)); got != (Scale{Num: -1, Den: 2}) {
		t.Errorf("1/-2 = %v, want -1/2", got)
	}
}

// TestUnitTermAlgebra composes and compares terms over base units, which is what
// makes two quantities comparable.
func TestUnitTermAlgebra(t *testing.T) {
	metre := &symbols.Symbol{Name: "metre"}
	second := &symbols.Symbol{Name: "second"}
	m := UnitTerm{Scale: UnitScale(1), Factors: []UnitFactor{{Unit: metre, Exponent: 1}}}
	s := UnitTerm{Scale: UnitScale(1), Factors: []UnitFactor{{Unit: second, Exponent: 1}}}

	speed := m.DividedBy(s)
	if speed.Dimensionless() {
		t.Error("m/s has a dimension")
	}
	if speed.Commensurable(m) {
		t.Error("m/s and m do not measure the same thing")
	}
	if ratio := m.DividedBy(m); !ratio.Dimensionless() {
		t.Errorf("m/m = %v, want dimension one", ratio)
	}
	if area := m.Times(m); area.Factors[0].Exponent != 2 {
		t.Errorf("m·m = %v, want m^2", area)
	}
	if inv := speed.Pow(-1); !inv.Commensurable(s.DividedBy(m)) {
		t.Errorf("(m/s)^-1 = %v, want s/m", inv)
	}
}
