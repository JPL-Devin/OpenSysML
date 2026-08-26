package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// libStateExecutor builds an initialized executor over the named machine in src,
// with the standard library loaded so quantities reduce against real units.
func libStateExecutor(t *testing.T, name, src string) *StateExecutor {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), name, ast.DefState)
	if sym == nil {
		t.Fatalf("state machine %s not found", name)
	}
	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("create state executor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	return exec
}

// A duration carrying a unit schedules the delay the unit says, converted to the
// clock's second: `1 [min]` waits sixty times as long as `1 [s]`.
func TestTimeTriggerUnitIsConverted(t *testing.T) {
	for _, tc := range []struct {
		name     string
		duration string
		want     float64
	}{
		{"seconds", "5 [s]", 5},
		{"real seconds", "2.5 [s]", 2.5},
		{"minutes", "1 [min]", 60},
		{"unitless", "5", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exec := libStateExecutor(t, "Machine", `package test {
				private import SI::*;
				state Machine {
					entry; then start;
					state start;
					state waiting {
						accept after `+tc.duration+` then done;
					}
					state done;
					succession first start then waiting;
				}
			}`)
			if err := exec.RunToCompletion(); err != nil {
				t.Fatalf("run: %v", err)
			}
			assertCurrentState(t, exec, "done")
			if exec.currentTime != tc.want {
				t.Errorf("clock = %v, want %v", exec.currentTime, tc.want)
			}
		})
	}
}

// A unit smaller than the clock's second converts by its own scale, so a
// sub-second duration is neither rounded to the base unit nor rejected.
func TestTimeTriggerSubSecondUnit(t *testing.T) {
	exec := libStateExecutor(t, "Machine", `package test {
		private import SI::*;
		attribute <ms> millisecond : ISQBase::DurationUnit {
			:>> unitConversion: MeasurementReferences::ConversionByConvention {
				:>> referenceUnit = s;
				:>> conversionFactor = 1/1000;
			}
		}
		state Machine {
			entry; then start;
			state start;
			state waiting {
				accept after 500 [ms] then done;
			}
			state done;
			succession first start then waiting;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "done")
	if exec.currentTime != 0.5 {
		t.Errorf("clock = %v, want 0.5 seconds", exec.currentTime)
	}
}

// An instant carrying a unit names a time on the same clock a duration measures
// on, and one already past fires at once rather than moving the clock back.
func TestTimeTriggerAbsoluteInstantWithUnit(t *testing.T) {
	exec := libStateExecutor(t, "Machine", `package test {
		private import SI::*;
		state Machine {
			entry; then start;
			state start;
			state waiting {
				accept at 2 [min] then done;
			}
			state done;
			succession first start then waiting;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "done")
	if exec.currentTime != 120 {
		t.Errorf("clock = %v, want 120 seconds", exec.currentTime)
	}
}

// A duration that does not measure time cannot be scheduled: the diagnostic
// names the quantity and says it is no duration.
func TestTimeTriggerRejectsNonTimeDimension(t *testing.T) {
	exec := libStateExecutor(t, "Machine", `package test {
		private import SI::*;
		state Machine {
			entry; then start;
			state start;
			state waiting {
				accept after 5 [kg] then done;
			}
			state done;
			succession first start then waiting;
		}
	}`)
	err := exec.RunToCompletion()
	if err == nil {
		t.Fatal("scheduling a mass as a duration succeeded")
	}
	if !strings.Contains(err.Error(), "5 [kg] is not a time") {
		t.Errorf("err = %v, want it to name the quantity as written", err)
	}
	// The dimension mismatch keeps its identity, so a caller can branch on it.
	if !errors.Is(err, ErrIncommensurableUnits) {
		t.Errorf("err = %v, want it to wrap ErrIncommensurableUnits", err)
	}
}
