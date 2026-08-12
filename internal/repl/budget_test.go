package repl

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/runtime"
)

// TestSessionBudgetsDefault: a fresh session runs on the default bounds.
func TestSessionBudgetsDefault(t *testing.T) {
	if got := NewSession().Budgets(); got != runtime.DefaultBudgets() {
		t.Errorf("Budgets() = %+v, want %+v", got, runtime.DefaultBudgets())
	}
}

// TestSetBudgets: raised bounds are adopted and apply to the runtime context
// created afterwards; a non-positive bound is refused.
func TestSetBudgets(t *testing.T) {
	s := NewSession()
	if res := s.Submit("package P { part def Q; }"); len(res.Diagnostics) > 0 {
		t.Fatalf("submit reported diagnostics: %v", res.Diagnostics)
	}
	if _, err := s.getOrCreateRuntime(); err != nil {
		t.Fatalf("getOrCreateRuntime: %v", err)
	}

	s.instances["P::Q"] = &runtime.Instance{ID: 1}

	want := runtime.Budgets{MaxSteps: 4200, MaxActionSteps: 42, MaxStateEvents: 43, MaxDoSteps: 44}
	if err := s.SetBudgets(want); err != nil {
		t.Fatalf("SetBudgets: %v", err)
	}
	if got := s.Budgets(); got != want {
		t.Errorf("Budgets() = %+v, want %+v", got, want)
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		t.Fatalf("getOrCreateRuntime: %v", err)
	}
	if got := ctx.Budgets(); got != want {
		t.Errorf("runtime context bounds = %+v, want %+v", got, want)
	}
	// Instances belonged to the discarded context, whose IDs the new one reuses.
	if len(s.instances) != 0 {
		t.Errorf("instances survived the new context: %v", s.instances)
	}

	for _, bad := range []runtime.Budgets{
		{MaxSteps: 0, MaxActionSteps: 1, MaxStateEvents: 1, MaxDoSteps: 1},
		{MaxSteps: 1, MaxActionSteps: 1, MaxStateEvents: -1, MaxDoSteps: 1},
		{},
	} {
		if err := s.SetBudgets(bad); err == nil {
			t.Errorf("SetBudgets(%+v) was accepted", bad)
		}
	}
	if got := s.Budgets(); got != want {
		t.Errorf("a refused set changed the bounds to %+v", got)
	}
}
