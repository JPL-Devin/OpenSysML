package solve

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/runtime"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

// solverRequiredEnv is set in CI so that an absent solver fails these tests
// instead of skipping them, as the training corpus gate is required there.
const solverRequiredEnv = "SYSTEMICA_REQUIRE_SMT"

// requireSolver returns the discovered solver, skipping the calling test when no
// solver is installed — unless one is declared mandatory, when it fails.
func requireSolver(t *testing.T) *Solver {
	t.Helper()
	solver, err := Discover()
	if err == nil {
		return solver
	}
	if !errors.Is(err, ErrNoSolver) {
		t.Fatalf("discover a solver: %v", err)
	}
	if os.Getenv(solverRequiredEnv) != "" {
		t.Fatalf("%s=%s but %v", solverRequiredEnv, os.Getenv(solverRequiredEnv), err)
	}
	t.Skipf("no SMT solver installed: %v", err)
	return nil
}

// solved asks the discovered solver about a query, expecting the wanted verdict.
func solved(t *testing.T, solver *Solver, q *Query, want Status) *Result {
	t.Helper()
	result, err := solver.Solve(context.Background(), q)
	if err != nil {
		t.Fatalf("solve: %v\nscript:\n%s", err, Script(q))
	}
	if result.Status != want {
		t.Fatalf("%s answered %s, want %s (reason %q)\nscript:\n%s",
			result.Solver, result.Status, want, result.Reason, Script(q))
	}
	return result
}

// modelValues indexes a satisfying model by the feature name it assigns.
func modelValues(t *testing.T, result *Result) map[string]string {
	t.Helper()
	out := make(map[string]string, len(result.Model))
	for _, a := range result.Model {
		if !a.Rendered {
			t.Errorf("%s was reported as raw SMT-LIB (%s), not in the notation's terms", a.Var.Name, a.Raw)
		}
		out[a.Var.Name] = a.Value
	}
	return out
}

// Over a spread of sign combinations, the solved quotient and remainder are the
// evaluator's: truncating toward zero, remainder taking the dividend's sign.
func TestSolvedIntegerDivisionAgreesWithEvaluator(t *testing.T) {
	solver := requireSolver(t)
	pairs := [][2]int{{7, 2}, {-7, 2}, {7, -2}, {-7, -2}, {9, 3}, {-9, 3}, {1, -5}, {-1, 5}, {0, -3}}
	for _, pair := range pairs {
		a, b := pair[0], pair[1]
		t.Run(fmt.Sprintf("%d_by_%d", a, b), func(t *testing.T) {
			wantQ, wantR := evaluatedDivision(t, a, b)

			// The divisor is a variable here, which is the nonlinear case: both
			// operands are pinned by conditions rather than written as literals.
			q := constraintQuery(t, fmt.Sprintf(`
				package test {
					private import ScalarValues::Integer;
					constraint def C {
						in a : Integer; in b : Integer; in q : Integer; in r : Integer;
						assert constraint { a == %d }
						assert constraint { b == %d }
						assert constraint { q == a / b }
						assert constraint { r == a %% b }
					}
				}`, a, b), "test::C")
			if !q.Nonlinear {
				t.Error("a query dividing by a variable is not marked nonlinear")
			}
			values := modelValues(t, solved(t, solver, q, StatusSat))
			if values["test::C::q"] != wantQ || values["test::C::r"] != wantR {
				t.Errorf("solved %d / %d = %s remainder %s, evaluator says %s remainder %s",
					a, b, values["test::C::q"], values["test::C::r"], wantQ, wantR)
			}

			// The literal-divisor case, which stays linear in the divisor.
			lit := constraintQuery(t, fmt.Sprintf(`
				package test {
					private import ScalarValues::Integer;
					constraint def C {
						in a : Integer; in q : Integer; in r : Integer;
						assert constraint { a == %d }
						assert constraint { q == a / %d }
						assert constraint { r == a %% %d }
					}
				}`, a, b, b), "test::C")
			if lit.Nonlinear {
				t.Error("a query dividing by a literal is marked nonlinear")
			}
			values = modelValues(t, solved(t, solver, lit, StatusSat))
			if values["test::C::q"] != wantQ || values["test::C::r"] != wantR {
				t.Errorf("solved %d / %d = %s remainder %s with a literal divisor, evaluator says %s remainder %s",
					a, b, values["test::C::q"], values["test::C::r"], wantQ, wantR)
			}
		})
	}
}

// Where SMT-LIB's `div`/`mod` differ from the evaluator, the Euclidean answer is
// refused rather than merely being self-consistent.
func TestSolvedDivisionRejectsEuclideanAnswer(t *testing.T) {
	solver := requireSolver(t)
	// -7 / 2 is -3 remainder -1 truncating, -4 remainder 1 Euclidean.
	q := constraintQuery(t, `
		package test {
			private import ScalarValues::Integer;
			constraint def C {
				in a : Integer; in b : Integer;
				assert constraint { a == -7 }
				assert constraint { b == 2 }
				assert constraint { a / b == -4 }
			}
		}`, "test::C")
	solved(t, solver, q, StatusUnsat)

	r := constraintQuery(t, `
		package test {
			private import ScalarValues::Integer;
			constraint def C {
				in a : Integer; in b : Integer;
				assert constraint { a == -7 }
				assert constraint { b == 2 }
				assert constraint { a % b == 1 }
			}
		}`, "test::C")
	solved(t, solver, r, StatusUnsat)
}

// SMT-LIB's division is total, so without a guard a solver could satisfy a
// condition by dividing by zero; the guard makes that unsatisfiable.
func TestDivisorGuardRulesOutDivisionByZero(t *testing.T) {
	solver := requireSolver(t)
	cases := map[string]string{
		"integer division": `in a : Integer; in b : Integer;
			assert constraint { b == 0 }
			assert constraint { a / b == 0 }`,
		"integer remainder": `in a : Integer; in b : Integer;
			assert constraint { b == 0 }
			assert constraint { a % b == 0 }`,
		"real division": `in x : Real; in y : Real;
			assert constraint { y == 0.0 }
			assert constraint { x / y == 0.0 }`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			q := constraintQuery(t, fmt.Sprintf(`
				package test {
					private import ScalarValues::Integer;
					private import ScalarValues::Real;
					constraint def C { %s }
				}`, body), "test::C")
			solved(t, solver, q, StatusUnsat)
		})
	}
}

// A condition that guards its own division leaves the division unevaluated when
// the divisor is zero, so the query cannot assert the divisor non-zero and the
// translation refuses rather than denying assignments the evaluator accepts.
func TestComputedDivisorRefusedWhereItMayGoUnevaluated(t *testing.T) {
	cases := map[string]string{
		"under or":        `assert constraint { b == 0 or a / b > 1 }`,
		"under implies":   `assert constraint { b != 0 implies a / b > 1 }`,
		"under not":       `assert constraint { not (a / b > 1) }`,
		"under a branch":  `assert constraint { (if b != 0 ? a / b else 0) > 1 }`,
		"negated written": `assert not constraint { a / b > 1 }`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			refused := refusal(t, fmt.Sprintf(`
				package test {
					private import ScalarValues::Integer;
					constraint def C {
						in a : Integer; in b : Integer;
						%s
					}
				}`, body), "test::C")
			if !strings.Contains(refused.Construct, "computed divisor") {
				t.Errorf("refusal names %q, want the computed-divisor case", refused.Construct)
			}
		})
	}
}

// The same conditions hold for the evaluator with a zero divisor, which is why the
// hoisted guard would be wrong: it is only asserted where a division always runs.
func TestGuardedDivisionSatisfiableWithALiteralDivisor(t *testing.T) {
	solver := requireSolver(t)
	q := constraintQuery(t, `
		package test {
			private import ScalarValues::Integer;
			constraint def C {
				in a : Integer;
				assert constraint { a / 2 > 1 }
			}
		}`, "test::C")
	solved(t, solver, q, StatusSat)
}

// evaluatedDivision is what internal/core/runtime computes for `a / b` and `a %
// b`, the normative answer this encoding has to reproduce.
func evaluatedDivision(t *testing.T, a, b int) (quotient, remainder string) {
	t.Helper()
	ctx, idx := fixture(t, "<eval>", fmt.Sprintf(`
		package eval {
			private import ScalarValues::Integer;
			part def P {
				attribute a : Integer = %d;
				attribute b : Integer = %d;
				attribute q : Integer = a / b;
				attribute r : Integer = a %% b;
			}
		}`, a, b))
	inst, err := ctx.Instantiate(symbolNamed(t, idx, "eval::P"))
	if err != nil {
		t.Fatalf("instantiate eval::P: %v", err)
	}
	return slotInteger(t, ctx, inst, "q"), slotInteger(t, ctx, inst, "r")
}

// slotInteger is the integer a slot holds, materializing it as any reader does.
func slotInteger(t *testing.T, ctx *runtime.Context, inst *runtime.Instance, name string) string {
	t.Helper()
	slot, err := inst.GetSlot(ctx, name)
	if err != nil {
		t.Fatalf("read slot %s: %v", name, err)
	}
	val := slot.HeldValue()
	if val.Kind != runtime.ValConst || val.Const.Kind != semantics.ValInt {
		t.Fatalf("slot %s holds %v, want an integer", name, val)
	}
	return fmt.Sprintf("%d", val.Const.Int)
}
