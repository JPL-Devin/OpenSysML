package model

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/passes"
)

// dimensionDiagnostics splits a document's diagnostics into the dimensional
// warnings and the errors, which a warning must never become.
func dimensionDiagnostics(t *testing.T, src string) (warnings, errs []string) {
	t.Helper()
	ws := NewWorkspace()
	ws.Open("test.sysml", []byte(src), 1)
	for _, d := range ws.Diagnostics("test.sysml") {
		switch {
		case d.Severity == passes.SeverityError:
			errs = append(errs, d.Message)
		case strings.Contains(d.Message, "incommensurable quantities"):
			if d.Severity != passes.SeverityWarning {
				t.Fatalf("dimensional diagnostic is %v, want warning: %s", d.Severity, d.Message)
			}
			warnings = append(warnings, d.Message)
		}
	}
	return warnings, errs
}

// TestDimensionMismatchWarns locks in the reported case: a mass compared with a
// length is diagnosed before evaluation, as a warning naming both dimensions,
// and validation still succeeds.
func TestDimensionMismatchWarns(t *testing.T) {
	warnings, errs := dimensionDiagnostics(t, `package Test {
		private import ISQ::*;
		private import SI::*;
		part def Vehicle {
			attribute mass : ISQ::MassValue = 1200.0 [kg];
			constraint badUnits { mass < 1000.0 [m] }
		}
	}`)
	if len(errs) != 0 {
		t.Fatalf("a dimensional warning must not fail validation, got errors: %v", errs)
	}
	if len(warnings) != 1 {
		t.Fatalf("want 1 dimensional warning, got %d: %v", len(warnings), warnings)
	}
	for _, want := range []string{"MassValue", "dimension M", "m (dimension L)"} {
		if !strings.Contains(warnings[0], want) {
			t.Errorf("warning %q does not name %q", warnings[0], want)
		}
	}
}

// TestDimensionMismatchWarnsInArithmetic covers a sum of incommensurable
// quantities, which evaluation rejects for the same reason a comparison does.
func TestDimensionMismatchWarnsInArithmetic(t *testing.T) {
	warnings, errs := dimensionDiagnostics(t, `package Test {
		private import ISQ::*;
		private import SI::*;
		part def Vehicle {
			attribute mass : ISQ::MassValue = 1200.0 [kg];
			attribute len : ISQ::LengthValue = 3.0 [m];
			constraint bad { mass + len < 10000.0 [kg] }
		}
	}`)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(warnings) != 1 {
		t.Fatalf("want 1 dimensional warning, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "operator '+'") {
		t.Errorf("warning %q does not name the operator", warnings[0])
	}
}

// TestDimensionMismatchWarnsThroughAlias covers an operand whose quantity type is
// named through a library alias, which fixes the same dimension the type it
// aliases does.
func TestDimensionMismatchWarnsThroughAlias(t *testing.T) {
	warnings, errs := dimensionDiagnostics(t, `package Test {
		private import ISQ::*;
		private import SI::*;
		part def Oven {
			attribute temp : ISQ::TemperatureValue = 300.0 [K];
			constraint bad { temp < 400.0 [kg] }
		}
	}`)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(warnings) != 1 {
		t.Fatalf("want 1 dimensional warning, got %d: %v", len(warnings), warnings)
	}
}

// TestDimensionCommensurableSilent covers units of one dimension at different
// scales, and a derived dimension composed by unit arithmetic: both convert at
// evaluation, so neither may warn.
func TestDimensionCommensurableSilent(t *testing.T) {
	for name, src := range map[string]string{
		"mm against m": `package Test {
			private import ISQ::*;
			private import SI::*;
			part def Vehicle {
				attribute len : ISQ::LengthValue = 1200.0 [mm];
				constraint ok { len < 1000.0 [m] }
			}
		}`,
		"speed against km per h": `package Test {
			private import ISQ::*;
			private import SI::*;
			part def Vehicle {
				attribute v : ISQ::SpeedValue = 10.0 [m/s];
				constraint ok { v < 20.0 [km/h] }
			}
		}`,
	} {
		t.Run(name, func(t *testing.T) {
			warnings, errs := dimensionDiagnostics(t, src)
			if len(errs) != 0 {
				t.Fatalf("unexpected errors: %v", errs)
			}
			if len(warnings) != 0 {
				t.Fatalf("commensurable operands must not warn, got: %v", warnings)
			}
		})
	}
}

// TestDimensionlessSilent covers ordinary arithmetic on numbers, which carries no
// dimension to disagree about.
func TestDimensionlessSilent(t *testing.T) {
	warnings, errs := dimensionDiagnostics(t, `package Test {
		part def Vehicle {
			attribute count : ScalarValues::Integer = 3;
			attribute ratio : ScalarValues::Real = 0.5;
			constraint ok { count < 5 and ratio * 2.0 > 0.5 }
		}
	}`)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(warnings) != 0 {
		t.Fatalf("dimensionless operands must not warn, got: %v", warnings)
	}
}

// TestDimensionUnknownSilent covers the documented limitation: a dimension that
// only evaluation determines — a calculation result, an untyped parameter, an
// unresolved reference, or a value an assignment may replace — is not guessed at.
// A parameter that declares a quantity type does fix a dimension, and warns.
func TestDimensionUnknownSilent(t *testing.T) {
	for name, src := range map[string]string{
		"calc result": `package Test {
			private import ISQ::*;
			private import SI::*;
			calc def Limit { return : ISQ::LengthValue; }
			part def Vehicle {
				attribute mass : ISQ::MassValue = 1200.0 [kg];
				constraint maybe { mass < Limit() }
			}
		}`,
		"untyped constraint parameter": `package Test {
			private import ISQ::*;
			private import SI::*;
			constraint def Under {
				in limit;
				in actual : ISQ::MassValue;
				actual < limit;
			}
		}`,
		"assignable attribute": `package Test {
			private import SI::*;
			action travel {
				attribute speed : ScalarValues::Real = 0.0;
				attribute quick : ScalarValues::Boolean = false;
				first start;
				action compute { assign speed := 10.0 [SI::m] / 2.0 [SI::s]; }
				action check { assign quick := speed >= 18.0 [SI::km/SI::h]; }
				done end;
				then start compute;
				then compute check;
				then check end;
			}
		}`,
	} {
		t.Run(name, func(t *testing.T) {
			warnings, errs := dimensionDiagnostics(t, src)
			if len(errs) != 0 {
				t.Fatalf("unexpected errors: %v", errs)
			}
			if len(warnings) != 0 {
				t.Fatalf("an undetermined dimension must not warn, got: %v", warnings)
			}
		})
	}
}
