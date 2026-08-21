package runtime

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// A completion self-transition on a simple state re-runs its exit and entry
// actions every round, so the run must stay bounded and report rather than hang.
func TestSimpleSelfTransitionThatNeverSettlesIsBounded(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			attribute log : Integer = 0;

			initial start;
			state s {
				entry { log = log + 1; }
				exit { log = log + 1; }
			}

			start then s;
			transition s to s do assign log := log + 1;
		}
	}`)

	done := make(chan error, 1)
	go func() { done <- exec.RunToCompletion() }()

	var err error
	select {
	case err = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("run to completion hangs on a simple state transitioning to itself")
	}
	if err == nil {
		t.Fatal("expected a budget error for a self-transition that never settles")
	}
	if !strings.Contains(err.Error(), "exceeded max") && !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("err = %v; want a budget error", err)
	}
}

// A `first` marker named as a transition's source is refused when the machine is
// built: whether it names one edge or a second one is unadjudicated (row ~485).
func TestFirstMarkerNamedAsATransitionSourceIsRefused(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		attribute def StartSignal;
		state Machine {
			first start then off;
			state off;
			transition t1 first start accept StartSignal then off;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state machine Machine not found")
	}

	_, err := newStateExecutor(ctx, sym, nil)
	if err == nil {
		t.Fatal("expected building the machine to report the marker source")
	}
	if !strings.Contains(err.Error(), "start") {
		t.Errorf("err = %v; want it to name the endpoint", err)
	}
}

// The effect of a self-transition is executed between the exit and the entry, so
// an effect reading a feature the machine does not declare reports there.
func TestSimpleSelfTransitionEffectReadingAnUnknownFeatureIsReported(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			attribute log : Integer = 0;

			initial start;
			state s {
				entry { log = log + 1; }
				exit { log = log + 1; }
			}

			start then s;
			transition s to s accept again do assign log := missingName + 1;
		}
	}`)

	exec.SendSignal("again", nil)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("err = %v; want ErrUnresolvedReference", err)
	}
	if !strings.Contains(err.Error(), "missingName") {
		t.Errorf("err = %v; want it to name the unresolved feature", err)
	}
	// The exit ran before the effect failed, so the entry did not: log is 1+1.
	if got := exec.StateData()["log"]; got.Const.Int != 2 {
		t.Errorf("log = %v; want 2 (entry, exit), the entry not re-run after a failed effect", got.Const.Int)
	}
}
