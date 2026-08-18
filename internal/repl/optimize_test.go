package repl

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// requireOptimizingSolver skips when the backend on PATH implements no
// (minimize)/(maximize): cvc5 refuses them, which %optimize reports rather than
// works around.
func requireOptimizingSolver(t *testing.T) {
	t.Helper()
	requireSolver(t)
	solver, err := solve.Discover()
	if err != nil {
		t.Fatalf("discovering the solver: %v", err)
	}
	caps, err := solver.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("asking %s what it supports: %v", solver.Name, err)
	}
	if !caps.Supports(solve.CapOptimization) {
		t.Skipf("%s implements no optimization: %s", solver.Name, caps.Detail(solve.CapOptimization))
	}
}

// optimizeModel states the analysis cases %optimize is asked about: what each one
// improves comes from the trade-study definition typing its objective.
const optimizeModel = `
package Trade {
	private import ScalarValues::*;
	private import TradeStudies::*;

	analysis def Bounded {
		attribute size : Integer;
		require constraint { size >= 3 }
		require constraint { size <= 9 }
		objective smallest : MinimizeObjective { attribute :>> best = size; }
	}

	analysis def Widest {
		attribute size : Integer;
		require constraint { size <= 9 }
		objective largest : MaximizeObjective {
			attribute :>> best = size;
			require constraint { size >= 0 }
		}
	}

	analysis def Unbounded {
		attribute size : Integer;
		require constraint { size >= 3 }
		objective largest : MaximizeObjective { attribute :>> best = size; }
	}

	analysis def Impossible {
		attribute size : Integer;
		require constraint { size >= 9 }
		require constraint { size <= 3 }
		objective smallest : MinimizeObjective { attribute :>> best = size; }
	}

	analysis def Ordered {
		attribute cost : Integer;
		attribute margin : Integer;
		require constraint { cost >= 3 and cost <= 9 }
		require constraint { margin >= 0 and margin <= cost }
		objective cheapest : MinimizeObjective { attribute :>> best = cost; }
		objective widest : MaximizeObjective { attribute :>> best = margin; }
	}

	analysis def OpenBound {
		attribute margin : Real;
		require constraint { margin >= 0.0 }
		require constraint { margin < 10.5 }
		objective widest : MaximizeObjective { attribute :>> best = margin; }
	}

	analysis def Nonlinear {
		attribute a : Real;
		attribute b : Real;
		require constraint { a >= 1.0 and a <= 4.0 }
		require constraint { b >= 1.0 and b <= 4.0 }
		objective gain : MaximizeObjective { attribute :>> best = a * b; }
	}

	requirement def FitsWell :> TradeStudyObjective;

	analysis def Undirected {
		attribute size : Integer;
		require constraint { size >= 1 }
		objective goal : FitsWell { attribute :>> best = size; }
	}

	analysis def NoObjective {
		attribute size : Integer;
		require constraint { size >= 1 }
	}
}
`

// optimized runs %optimize and returns its lines, with the report behind them.
func optimized(t *testing.T, s *Session, name string) (string, SolveReport) {
	t.Helper()
	got := run(t, s, "%optimize "+name)
	reports := s.OptimizeSolve(name)
	if len(reports) != 1 {
		t.Fatalf("OptimizeSolve answered %d reports, want 1", len(reports))
	}
	return got, reports[0]
}

func TestOptimizeReportsTheLeastValue(t *testing.T) {
	requireOptimizingSolver(t)
	s := checkSession(t, optimizeModel)
	got, report := optimized(t, s, "Bounded")
	wants(t, got, "✓ Analysis Bounded is optimized", "minimize smallest = `size`: 3", "Trade::Bounded::size = 3")
	rejects(t, got, "error:", "no optimum")
	if report.Status != SolveSat {
		t.Errorf("status is %s, want sat: %s", report.Status, strings.Join(report.Lines, "\n"))
	}
	if report.Solver == "" {
		t.Error("the report does not name the solver that answered")
	}
}

// The objective's own condition bounds it from below, and the case's assumption
// from above, so the greatest value comes from both together.
func TestOptimizeReportsTheGreatestValue(t *testing.T) {
	requireOptimizingSolver(t)
	s := checkSession(t, optimizeModel)
	got, report := optimized(t, s, "Widest")
	wants(t, got, "✓ Analysis Widest is optimized", "maximize largest = `size`: 9")
	if report.Status != SolveSat {
		t.Errorf("status is %s, want sat", report.Status)
	}
}

// An objective improving without limit has no optimum: that is its own status,
// and no number is offered as one.
func TestOptimizeReportsAnUnboundedObjective(t *testing.T) {
	requireOptimizingSolver(t)
	s := checkSession(t, optimizeModel)
	got, report := optimized(t, s, "Unbounded")
	wants(t, got, "! Analysis Unbounded has no optimum", "no greatest value")
	rejects(t, got, "is optimized", "error:")
	if report.Status != SolveUnbounded {
		t.Errorf("status is %s, want unbounded: %s", report.Status, strings.Join(report.Lines, "\n"))
	}
}

func TestOptimizeReportsAnInfeasibleAnalysis(t *testing.T) {
	requireOptimizingSolver(t)
	s := checkSession(t, optimizeModel)
	got, report := optimized(t, s, "Impossible")
	wants(t, got, "✗ Analysis Impossible has no values satisfying its conditions")
	rejects(t, got, "is optimized", "=")
	if report.Status != SolveUnsat {
		t.Errorf("status is %s, want unsat", report.Status)
	}
}

// Objectives are improved lexicographically in declaration order: the least cost
// first, then the greatest margin among the assignments achieving it.
func TestOptimizeReportsObjectivesInDeclarationOrder(t *testing.T) {
	requireOptimizingSolver(t)
	s := checkSession(t, optimizeModel)
	got, report := optimized(t, s, "Ordered")
	wants(t, got, "minimize cheapest = `cost`: 3", "maximize widest = `margin`: 3")
	if report.Status != SolveSat {
		t.Fatalf("status is %s, want sat: %s", report.Status, strings.Join(report.Lines, "\n"))
	}
	cheapest, widest := strings.Index(got, "cheapest"), strings.Index(got, "widest")
	if cheapest < 0 || widest < cheapest {
		t.Errorf("objectives are not reported in declaration order:\n%s", got)
	}
}

// A bound no assignment attains is never reported as a value: the report says
// there is no optimum and offers only the feasible value found.
func TestOptimizeReportsABoundThatIsNotAttained(t *testing.T) {
	requireOptimizingSolver(t)
	s := checkSession(t, optimizeModel)
	got, report := optimized(t, s, "OpenBound")
	if report.Status != SolveNoOptimum {
		t.Fatalf("status is %s, want no-optimum: %s", report.Status, strings.Join(report.Lines, "\n"))
	}
	wants(t, got, "! Analysis OpenBound is satisfiable, but its optimum was not established",
		"maximize widest = `margin`: no optimum reported")
	rejects(t, got, "is optimized")
}

// Refusals need no solver: they are settled while translating.
func TestOptimizeRefusesANonlinearObjective(t *testing.T) {
	s := checkSession(t, optimizeModel)
	got, report := optimized(t, s, "Nonlinear")
	wants(t, got, "error:", "not optimizable", "nonlinear")
	if report.Status != SolveUnavailable {
		t.Errorf("status is %s, want unavailable", report.Status)
	}
}

func TestOptimizeRefusesAnObjectiveWithoutADirection(t *testing.T) {
	s := checkSession(t, optimizeModel)
	got, report := optimized(t, s, "Undirected")
	wants(t, got, "error:", "no direction", "MinimizeObjective")
	if report.Status != SolveUnavailable {
		t.Errorf("status is %s, want unavailable", report.Status)
	}
}

func TestOptimizeReportsAnAnalysisStatingNoObjective(t *testing.T) {
	s := checkSession(t, optimizeModel)
	got, _ := optimized(t, s, "NoObjective")
	wants(t, got, "error:", "states no objective")
}

func TestOptimizeRejectsAnElementThatIsNoAnalysis(t *testing.T) {
	s := checkSession(t, optimizeModel+"\npackage Other { part def P; }")
	wants(t, run(t, s, "%optimize Other::P"), "error:", "analysis")
}

func TestOptimizeReportsAnUnknownName(t *testing.T) {
	s := checkSession(t, optimizeModel)
	wants(t, run(t, s, "%optimize Nope"), "error:")
}

func TestOptimizeWithoutAName(t *testing.T) {
	s := checkSession(t, optimizeModel)
	wants(t, run(t, s, "%optimize"), "usage: %optimize <name>")
}

func TestOptimizeReportsAnAbsentSolver(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv(solve.SolverEnv, "")
	s := checkSession(t, optimizeModel)
	got, report := optimized(t, s, "Bounded")
	wants(t, got, "error:", "z3")
	rejects(t, got, "is optimized", "no optimum")
	if report.Status != SolveUnavailable {
		t.Errorf("status is %s, want unavailable", report.Status)
	}
}

// Optimization is a z3 extension: a backend without it is reported as such,
// never degraded to a plain satisfiability check presented as an optimum.
func TestOptimizeReportsABackendWithoutOptimization(t *testing.T) {
	script := t.TempDir() + "/cvc5"
	// Answers `sat` but rejects the optimization commands, as cvc5 does: without
	// the capability check its `sat` would read as an optimum.
	fake := "#!/bin/sh\nwhile read -r line; do case \"$line\" in " +
		"*maximize*|*minimize*|*get-objectives*|*opt.priority*) echo unsupported;; " +
		"*check-sat*) echo sat;; esac; done\n"
	if err := os.WriteFile(script, []byte(fake), 0o700); err != nil { // #nosec G306 -- a test's own executable
		t.Fatalf("write fake solver: %v", err)
	}
	t.Setenv(solve.SolverEnv, script)
	s := checkSession(t, optimizeModel)
	got, report := optimized(t, s, "Bounded")
	wants(t, got, "error:", "cvc5", "optimization")
	rejects(t, got, "is optimized", "= 3")
	if report.Status != SolveUnavailable {
		t.Errorf("status is %s, want unavailable", report.Status)
	}
}

func TestOptimizeReportsASolverProcessFailure(t *testing.T) {
	script := t.TempDir() + "/z3-mute"
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 3\n"), 0o700); err != nil { // #nosec G306 -- a test's own executable
		t.Fatalf("write fake solver: %v", err)
	}
	t.Setenv(solve.SolverEnv, script)
	s := checkSession(t, optimizeModel)
	got := run(t, s, "%optimize Bounded")
	wants(t, got, "error:", "z3-mute")
	rejects(t, got, "is optimized", "unknown")
}

// An undecided answer stays undecided: no optimum is invented for it.
func TestOptimizeReportsAnUndecidedAnswer(t *testing.T) {
	script := t.TempDir() + "/z3-undecided"
	fake := "#!/bin/sh\nprintf 'unknown\\n(:reason-unknown \"incomplete\")\\n'\nwhile read -r line; do :; done\n"
	if err := os.WriteFile(script, []byte(fake), 0o700); err != nil { // #nosec G306 -- a test's own executable
		t.Fatalf("write fake solver: %v", err)
	}
	t.Setenv(solve.SolverEnv, script)
	s := checkSession(t, optimizeModel)
	got, report := optimized(t, s, "Bounded")
	wants(t, got, "? Analysis Bounded is undecided", "Reason: incomplete")
	rejects(t, got, "is optimized", "error:")
	if report.Status != SolveUnknown {
		t.Errorf("status is %s, want unknown", report.Status)
	}
}

func TestOptimizeIsListedInHelpAndCompletion(t *testing.T) {
	wants(t, strings.Join(helpText(), "\n"), "%optimize <name>")
	if got := NewSession().Complete("%opti", len("%opti")); !contains(got.Candidates, "%optimize") {
		t.Errorf("completing %%opti offers %v", got.Candidates)
	}
	for _, name := range metaCommands() {
		if name == "%optimize" {
			return
		}
	}
	t.Error("the optimize command is not dispatched")
}

// %optimize is read-only exactly as %check is: it materializes nothing and leaves
// a debugging session running, since it declares nothing.
func TestOptimizeKeepsADebuggingSessionAndMaterializesNothing(t *testing.T) {
	requireOptimizingSolver(t)
	s := checkSession(t, optimizeModel+`
package Debug {
	action def Walk {
		first start;
		then action step1;
		then done;
	}
}`)
	wants(t, run(t, s, "%action Debug::Walk"), "Walk")
	run(t, s, "%optimize Bounded")
	rejects(t, run(t, s, "%tokens"), "no active")
	wants(t, run(t, s, "%instances"), "no instances")
}
