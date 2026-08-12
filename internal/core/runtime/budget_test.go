package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

// TestMaxStepsFromValue covers the budget the environment resolves to: unset,
// empty, valid, padded, and the three shapes that are errors.
func TestMaxStepsFromValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  int64
		// errContains is empty when the value is accepted.
		errContains string
	}{
		{name: "unset", value: "", want: DefaultMaxSteps},
		{name: "whitespace_only", value: "   ", want: DefaultMaxSteps},
		{name: "valid", value: "5000000", want: 5000000},
		{name: "surrounding_whitespace", value: "  2500 \n", want: 2500},
		{name: "one", value: "1", want: 1},
		{name: "zero", value: "0", errContains: "must be greater than zero"},
		{name: "negative", value: "-5", errContains: "must be greater than zero"},
		{name: "non_numeric", value: "lots", errContains: "is not an integer"},
		{name: "float", value: "1e6", errContains: "is not an integer"},
		{name: "trailing_garbage", value: "1000steps", errContains: "is not an integer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := maxStepsFromValue(tt.value)
			if tt.errContains != "" {
				if err == nil {
					t.Fatalf("maxStepsFromValue(%q) = %d, want an error", tt.value, got)
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err, tt.errContains)
				}
				// The message must name the variable and the offending value, so
				// the reader knows what to fix.
				if !strings.Contains(err.Error(), MaxStepsEnvVar) {
					t.Errorf("error %q does not name %s", err, MaxStepsEnvVar)
				}
				if !strings.Contains(err.Error(), tt.value) {
					t.Errorf("error %q does not quote the offending value %q", err, tt.value)
				}
				if got != 0 {
					t.Errorf("rejected value yielded budget %d, want 0", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("maxStepsFromValue(%q) errored: %v", tt.value, err)
			}
			if got != tt.want {
				t.Errorf("maxStepsFromValue(%q) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

// TestMaxStepsFromEnv: the resolver reads the process environment, and reports
// an unusable value there the same way.
func TestMaxStepsFromEnv(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		t.Setenv(MaxStepsEnvVar, "")
		got, err := MaxStepsFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != DefaultMaxSteps {
			t.Errorf("got %d, want the default %d", got, DefaultMaxSteps)
		}
	})

	t.Run("raised", func(t *testing.T) {
		t.Setenv(MaxStepsEnvVar, "250000")
		got, err := MaxStepsFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 250000 {
			t.Errorf("got %d, want 250000", got)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Setenv(MaxStepsEnvVar, "many")
		if _, err := MaxStepsFromEnv(); err == nil {
			t.Fatal("expected an error for a non-numeric value")
		}
	})
}

// TestStepLimitErrorNamesEffectiveBudgetAndVariable: the reported limit is the
// one in force, and the message says which variable raises it.
func TestStepLimitErrorNamesEffectiveBudgetAndVariable(t *testing.T) {
	src := `part def Simple {}`
	model, resolver, _ := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 3)

	var err error
	for i := 0; i < 4 && err == nil; i++ {
		_, err = ctx.Eval(&ast.LiteralInteger{Value: "1"})
	}
	if err == nil {
		t.Fatal("expected the step budget to be exceeded")
	}
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("expected ErrStepLimitExceeded, got %v", err)
	}
	if !strings.Contains(err.Error(), "(3 steps") {
		t.Errorf("error %q does not report the effective budget of 3", err)
	}
	if !strings.Contains(err.Error(), MaxStepsEnvVar) {
		t.Errorf("error %q does not name %s", err, MaxStepsEnvVar)
	}
}

// TestRaisedBudgetRunsLongerLoop: a 10 000-iteration loop exhausts the default
// budget and completes under a raised one, which is the point of making the
// budget configurable.
func TestRaisedBudgetRunsLongerLoop(t *testing.T) {
	src := `
		package L {
			action loopn {
				attribute i = 0;
				attribute s = 0.0;
				first start;
				action go { while i < 10000 { assign s := s + 1.5; assign i := i + 1; } }
				done end;
				then start go;
				then go end;
			}
		}
	`
	file := parseAndBuild(t, src)

	runLoop := func(t *testing.T, maxSteps int64) error {
		idx, _, ctx := buildRuntime(t, "<test>", file)
		ctx.maxSteps = maxSteps
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "loopn", ast.DefAction)
		if sym == nil {
			t.Fatal("action loopn not found")
		}
		_, err := ctx.ExecuteAction(sym)
		return err
	}

	t.Run("default_budget_stops_it", func(t *testing.T) {
		err := runLoop(t, DefaultMaxSteps)
		if err == nil {
			t.Fatal("expected the default budget to stop the loop")
		}
		if !errors.Is(err, ErrStepLimitExceeded) {
			t.Fatalf("expected ErrStepLimitExceeded, got %v", err)
		}
	})

	t.Run("raised_budget_completes_it", func(t *testing.T) {
		if err := runLoop(t, 5000000); err != nil {
			t.Fatalf("loop failed under a raised budget: %v", err)
		}
	})
}
