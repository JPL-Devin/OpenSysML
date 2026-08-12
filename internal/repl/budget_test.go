package repl

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/runtime"
)

// TestSessionMaxStepsDefault: a fresh session runs on the default budget.
func TestSessionMaxStepsDefault(t *testing.T) {
	if got := NewSession().MaxSteps(); got != runtime.DefaultMaxSteps {
		t.Errorf("MaxSteps() = %d, want %d", got, runtime.DefaultMaxSteps)
	}
}

// TestSetMaxSteps: a raised budget is adopted and applies to the runtime context
// created afterwards; a non-positive one is refused.
func TestSetMaxSteps(t *testing.T) {
	s := NewSession()
	if res := s.Submit("package P { part def Q; }"); len(res.Diagnostics) > 0 {
		t.Fatalf("submit reported diagnostics: %v", res.Diagnostics)
	}
	if _, err := s.getOrCreateRuntime(); err != nil {
		t.Fatalf("getOrCreateRuntime: %v", err)
	}

	s.instances["P::Q"] = &runtime.Instance{ID: 1}

	if err := s.SetMaxSteps(4200); err != nil {
		t.Fatalf("SetMaxSteps: %v", err)
	}
	if got := s.MaxSteps(); got != 4200 {
		t.Errorf("MaxSteps() = %d, want 4200", got)
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		t.Fatalf("getOrCreateRuntime: %v", err)
	}
	if got := ctx.MaxSteps(); got != 4200 {
		t.Errorf("runtime context budget = %d, want 4200", got)
	}
	// Instances belonged to the discarded context, whose IDs the new one reuses.
	if len(s.instances) != 0 {
		t.Errorf("instances survived the new context: %v", s.instances)
	}

	for _, bad := range []int64{0, -1} {
		if err := s.SetMaxSteps(bad); err == nil {
			t.Errorf("SetMaxSteps(%d) was accepted", bad)
		}
	}
	if got := s.MaxSteps(); got != 4200 {
		t.Errorf("a refused budget changed MaxSteps() to %d", got)
	}
}
