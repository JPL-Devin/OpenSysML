package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// evalUnderElementBudget evaluates expr with room for maxElements materialized
// elements, and enough steps that the element ceiling is what stops it.
func evalUnderElementBudget(t *testing.T, expr string, maxElements int64) (Value, error) {
	t.Helper()
	ec, value := collectionExprContext(t, expr, DefaultMaxSteps)
	ec.ctx.maxElements = maxElements
	return ec.Eval(value)
}

// TestElementBudgetBoundsEveryMaterialization requires each way of materializing
// a sequence to be charged: what costs memory is the element, not the operation
// that produced it.
func TestElementBudgetBoundsEveryMaterialization(t *testing.T) {
	for _, expr := range []string{
		"1..100",
		"(1, 2, 3, 4, 5)",
		"xs->collect{in i; i * i}",
		"xs->including(4)",
		"xs->union(ys)",
		"xs->select{in i; i > 0}",
		"xs->subsequence(1, 3)",
		"xs->tail()",
	} {
		if _, err := evalUnderElementBudget(t, expr, 2); err == nil {
			t.Errorf("%s: want the element budget's error, got a value", expr)
		} else if !errors.Is(err, ErrElementLimitExceeded) {
			t.Errorf("%s: error = %v, want ErrElementLimitExceeded", expr, err)
		}
		if _, err := evalUnderElementBudget(t, expr, DefaultMaxElements); err != nil {
			t.Errorf("%s: rejected under the default budget: %v", expr, err)
		}
	}
}

// TestElementBudgetIsNotTheStepBudget requires a range too large to hold to
// report the element ceiling rather than the step one: they bound different
// things, so the remedy the message names has to be the right variable.
func TestElementBudgetIsNotTheStepBudget(t *testing.T) {
	_, err := evalUnderElementBudget(t, "1..1000000", 1000)
	if err == nil {
		t.Fatal("want the element budget's error, got a value")
	}
	if !errors.Is(err, ErrElementLimitExceeded) {
		t.Fatalf("error = %v, want ErrElementLimitExceeded", err)
	}
	if errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("error %q also reads as the step budget's", err)
	}
	if !strings.Contains(err.Error(), "(1000 elements") ||
		!strings.Contains(err.Error(), MaxElementsEnvVar) {
		t.Errorf("error %q does not report the budget in force and the variable raising it", err)
	}
}

// TestElementBudgetCountsElementsHeldNotProduced requires a loop building a small
// collection each iteration to run: what the budget bounds is the memory a run
// holds, and an iteration's collection is gone by the next one.
func TestElementBudgetCountsElementsHeldNotProduced(t *testing.T) {
	src := `
		package test {
			calc def Repeat {
				attribute i : Integer = 0;
				attribute acc : Integer = 0;
				while i < 50 {
					assign acc := acc + (1..10)->NumericalFunctions::sum();
					assign i := i + 1;
				}
				acc
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	// Room for two iterations' worth of elements, spent 50 times over.
	ctx.maxElements = 20
	sym, scope := calcByName(t, idx.DocumentRoot("<test>"), "test", "Repeat")

	result, err := ctx.InvokeCalc(sym, nil, scope)
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if result.Const.Int != 2750 {
		t.Errorf("acc = %d, want 2750", result.Const.Int)
	}
}

// TestElementBudgetIsReleasedByEveryStep requires a machine whose guard
// materializes a collection on every event to run: a step's collection is gone by
// the next step, so a long run is bounded by what a step holds, not by its length.
func TestElementBudgetIsReleasedByEveryStep(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;

			state machine {
				attribute i : Integer = 0;

				entry; then start;
				state start;
				state step {
					entry { assign i := i + 1; }
				}
				state again {
					entry { assign i := i + 1; }
				}
				final done;

				succession first start then step;

				transition first step if (1, 2, 3)->NumericalFunctions::sum() > i then again;
				transition first step if (1, 2, 3)->NumericalFunctions::sum() <= i then done;
				transition first again then step;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	// Room for two guard evaluations' worth of elements, spent on every step.
	ctx.maxElements = 6
	sym := findBehavioralSymbol(t, idx.DocumentRoot("<test>"), ast.DefState, ast.UsageState)

	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("create state executor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := exec.stateData["i"].Const.Int; got != 7 {
		t.Errorf("i = %d, want 7", got)
	}
}

// TestElementBudgetIsPerRun requires the count to start over with each run, so a
// session evaluating many collections is not stopped by the ones before.
func TestElementBudgetIsPerRun(t *testing.T) {
	model, resolver, _ := parseAndBuildModel(t, `part def Simple {}`)
	ctx := NewContext(model, resolver, DefaultMaxSteps)
	ctx.maxElements = 4

	for i := 0; i < 3; i++ {
		end := ctx.beginRun()
		if err := ctx.chargeElements(4); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		end()
	}
}
