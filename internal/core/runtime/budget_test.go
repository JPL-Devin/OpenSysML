package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

// TestBudgetFromValue covers what each variable resolves to: unset, empty,
// valid, padded, and the shapes that are errors.
func TestBudgetFromValue(t *testing.T) {
	values := []struct {
		name  string
		value string
		// want is relative to the variable's default when useDefault is set.
		want       int64
		useDefault bool
		// errContains is empty when the value is accepted.
		errContains string
	}{
		{name: "unset", value: "", useDefault: true},
		{name: "whitespace_only", value: "   ", useDefault: true},
		{name: "valid", value: "5000000", want: 5000000},
		{name: "surrounding_whitespace", value: "  2500 \n", want: 2500},
		{name: "one", value: "1", want: 1},
		{name: "zero", value: "0", errContains: "must be greater than zero"},
		{name: "negative", value: "-5", errContains: "must be greater than zero"},
		{name: "non_numeric", value: "lots", errContains: "is not an integer"},
		{name: "float", value: "1e6", errContains: "is not an integer"},
		{name: "trailing_garbage", value: "1000steps", errContains: "is not an integer"},
	}

	for _, v := range budgetVars {
		for _, tt := range values {
			t.Run(v.env+"/"+tt.name, func(t *testing.T) {
				got, err := budgetFromValue(v, tt.value)
				if tt.errContains != "" {
					if err == nil {
						t.Fatalf("budgetFromValue(%q) = %d, want an error", tt.value, got)
					}
					if !strings.Contains(err.Error(), tt.errContains) {
						t.Errorf("error %q does not contain %q", err, tt.errContains)
					}
					// The message must name the variable and the offending value,
					// so the reader knows what to fix.
					if !strings.Contains(err.Error(), v.env) {
						t.Errorf("error %q does not name %s", err, v.env)
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
					t.Fatalf("budgetFromValue(%q) errored: %v", tt.value, err)
				}
				want := tt.want
				if tt.useDefault {
					want = v.def
				}
				if got != want {
					t.Errorf("budgetFromValue(%q) = %d, want %d", tt.value, got, want)
				}
			})
		}
	}
}

// TestBudgetsFromLookup: each variable sets its own bound and leaves the others
// at their defaults, and every unusable value is reported at once.
func TestBudgetsFromLookup(t *testing.T) {
	t.Run("all_unset", func(t *testing.T) {
		got, err := budgetsFromLookup(func(string) string { return "" })
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != DefaultBudgets() {
			t.Errorf("got %+v, want the defaults %+v", got, DefaultBudgets())
		}
	})

	t.Run("each_variable_sets_only_its_own_bound", func(t *testing.T) {
		for _, v := range budgetVars {
			got, err := budgetsFromLookup(func(name string) string {
				if name == v.env {
					return " 4242 "
				}
				return ""
			})
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", v.env, err)
			}
			want := DefaultBudgets()
			*v.field(&want) = 4242
			if got != want {
				t.Errorf("%s: got %+v, want %+v", v.env, got, want)
			}
		}
	})

	t.Run("reports_every_unusable_value", func(t *testing.T) {
		got, err := budgetsFromLookup(func(name string) string {
			if name == MaxStepsEnvVar || name == MaxDoStepsEnvVar {
				return "lots"
			}
			return ""
		})
		if err == nil {
			t.Fatalf("got %+v, want an error", got)
		}
		for _, name := range []string{MaxStepsEnvVar, MaxDoStepsEnvVar} {
			if !strings.Contains(err.Error(), name) {
				t.Errorf("error %q does not name %s", err, name)
			}
		}
		if got != (Budgets{}) {
			t.Errorf("rejected environment yielded %+v, want the zero value", got)
		}
	})
}

// TestBudgetsFromEnv: the resolver reads the process environment, and reports an
// unusable value there the same way.
func TestBudgetsFromEnv(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		for _, v := range budgetVars {
			t.Setenv(v.env, "")
		}
		got, err := BudgetsFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != DefaultBudgets() {
			t.Errorf("got %+v, want the defaults %+v", got, DefaultBudgets())
		}
	})

	t.Run("raised", func(t *testing.T) {
		for _, v := range budgetVars {
			t.Setenv(v.env, "")
		}
		t.Setenv(MaxStepsEnvVar, "250000")
		t.Setenv(MaxStateEventsEnvVar, "20000")
		got, err := BudgetsFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.MaxSteps != 250000 || got.MaxStateEvents != 20000 {
			t.Errorf("got %+v, want MaxSteps 250000 and MaxStateEvents 20000", got)
		}
		if got.MaxDoSteps != DefaultMaxDoSteps {
			t.Errorf("MaxDoSteps = %d, want the untouched default %d", got.MaxDoSteps, DefaultMaxDoSteps)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Setenv(MaxActionStepsEnvVar, "many")
		if _, err := BudgetsFromEnv(); err == nil {
			t.Fatal("expected an error for a non-numeric value")
		}
	})
}

// TestSetBudgets: a context runs under the bounds it is given, and rejects a set
// holding a non-positive bound rather than running under it.
func TestSetBudgets(t *testing.T) {
	model, resolver, _ := parseAndBuildModel(t, `part def Simple {}`)
	ctx := NewContext(model, resolver, DefaultMaxSteps)
	if got := ctx.Budgets(); got != DefaultBudgets() {
		t.Errorf("a new context runs under %+v, want the defaults %+v", got, DefaultBudgets())
	}

	want := Budgets{MaxSteps: 11, MaxActionSteps: 22, MaxStateEvents: 33, MaxDoSteps: 44}
	if err := ctx.SetBudgets(want); err != nil {
		t.Fatalf("SetBudgets: %v", err)
	}
	if got := ctx.Budgets(); got != want {
		t.Errorf("Budgets() = %+v, want %+v", got, want)
	}

	for _, bad := range []Budgets{
		{MaxSteps: 0, MaxActionSteps: 1, MaxStateEvents: 1, MaxDoSteps: 1},
		{MaxSteps: 1, MaxActionSteps: -1, MaxStateEvents: 1, MaxDoSteps: 1},
		{},
	} {
		if err := ctx.SetBudgets(bad); err == nil {
			t.Errorf("SetBudgets(%+v) was accepted", bad)
		}
	}
	if got := ctx.Budgets(); got != want {
		t.Errorf("a rejected set changed the bounds to %+v, want %+v", got, want)
	}
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

// TestRaisedBudgetRunsLongerLoop: a 10 000-iteration loop exhausts the 100 000
// steps that used to be the default and completes under today's, which is the
// point of both raising the default and making it configurable.
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

	t.Run("old_default_stops_it", func(t *testing.T) {
		err := runLoop(t, 100000)
		if err == nil {
			t.Fatal("expected a budget of 100000 steps to stop the loop")
		}
		if !errors.Is(err, ErrStepLimitExceeded) {
			t.Fatalf("expected ErrStepLimitExceeded, got %v", err)
		}
	})

	t.Run("current_default_completes_it", func(t *testing.T) {
		if err := runLoop(t, DefaultMaxSteps); err != nil {
			t.Fatalf("loop failed under the default budget: %v", err)
		}
	})
}

// TestActionStepBudgetIsConfigurable: the action executor's token-flow bound
// comes from the context, and its error names the variable that raises it.
func TestActionStepBudgetIsConfigurable(t *testing.T) {
	src := `
		package L {
			action seq {
				attribute i = 0;
				first start;
				action a { assign i := i + 1; }
				action b { assign i := i + 1; }
				done end;
				then start a;
				then a b;
				then b end;
			}
		}
	`
	file := parseAndBuild(t, src)

	runSeq := func(t *testing.T, maxActionSteps int64) error {
		idx, _, ctx := buildRuntime(t, "<test>", file)
		ctx.maxActionSteps = maxActionSteps
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "seq", ast.DefAction)
		if sym == nil {
			t.Fatal("action seq not found")
		}
		_, err := ctx.ExecuteAction(sym)
		return err
	}

	if err := runSeq(t, DefaultMaxActionSteps); err != nil {
		t.Fatalf("action failed under the default token-flow budget: %v", err)
	}

	err := runSeq(t, 1)
	if err == nil {
		t.Fatal("expected a token-flow budget of 1 to stop the action")
	}
	if !strings.Contains(err.Error(), MaxActionStepsEnvVar) {
		t.Errorf("error %q does not name %s", err, MaxActionStepsEnvVar)
	}
}

// TestStateBudgetsAreConfigurable: the event and do activity bounds come from
// the context too, each naming its own variable.
func TestStateBudgetsAreConfigurable(t *testing.T) {
	// A machine whose state is re-entered every round never settles, so whichever
	// bound is reached first stops it.
	machine := func() *ast.Usage {
		return &ast.Usage{
			Kind:  ast.UsageState,
			Ident: ast.Identification{Name: "Machine"},
			Members: []ast.Node{
				&ast.StateNode{Name: "init", IsInitial: true},
				&ast.StateNode{Name: "spin", Do: []ast.Node{&ast.AssignmentActionNode{
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "ticks"}}},
					Value:  &ast.LiteralInteger{Value: "1"},
				}}},
				transitionMember("init", "spin"),
				transitionMember("spin", "spin"),
			},
		}
	}

	tests := []struct {
		name    string
		budgets func(*Context)
		wantVar string
	}{
		{
			name:    "events",
			budgets: func(ctx *Context) { ctx.maxStateEvents = 3 },
			wantVar: MaxStateEventsEnvVar,
		},
		{
			name:    "do_steps",
			budgets: func(ctx *Context) { ctx.maxDoSteps = 1 },
			wantVar: MaxDoStepsEnvVar,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := stateExecutorFor(t, machine())
			tt.budgets(exec.ctx)
			if err := exec.initialize(); err != nil {
				t.Fatalf("initialize: %v", err)
			}
			err := exec.RunToCompletion()
			if err == nil {
				t.Fatal("expected a budget error for a machine that never settles")
			}
			if !strings.Contains(err.Error(), tt.wantVar) {
				t.Errorf("error %q does not name %s", err, tt.wantVar)
			}
		})
	}
}
