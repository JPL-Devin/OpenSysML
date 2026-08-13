package runtime

import (
	"errors"
	"strings"
	"testing"
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
