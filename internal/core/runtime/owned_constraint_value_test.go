package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ownedConstraintValueFixture declares requirements whose constraints bind a
// value, beside the ordinary constraint usage each mirrors.
func ownedConstraintValueFixture(t *testing.T) (*Context, *symbols.Scope) {
	t.Helper()
	return conditionFixture(t, `
		package test {
			constraint plain = false;
			requirement def Base {
				require constraint failed = false;
				assume constraint granted = true;
			}
			requirement def Sub :> Base {
				require constraint :>> failed = true;
			}
			requirement base : Base;
			requirement sub : Sub;
		}
	`)
}

func memberNamed(t *testing.T, owner *symbols.Symbol, name string) *symbols.Symbol {
	t.Helper()
	sym, ok := owner.Scope.LookupLocal(name)
	if !ok {
		t.Fatalf("%s::%s not found", owner.Name, name)
	}
	return sym
}

// A value bound to a require/assume constraint is its feature value, read as a
// constraint usage's value is: `failed = false` reads false, `granted = true` true.
func TestOwnedConstraintValueReadsAsFeatureValue(t *testing.T) {
	ctx, pkg := ownedConstraintValueFixture(t)
	base := requirementNamed(t, pkg, "Base")
	for _, tc := range []struct {
		sym  *symbols.Symbol
		want string
	}{
		{requirementNamed(t, pkg, "plain"), "false"},
		{memberNamed(t, base, "failed"), "false"},
		{memberNamed(t, base, "granted"), "true"},
	} {
		val, err := ctx.EvalDeclaredValue(tc.sym)
		if err != nil {
			t.Fatalf("%s: %v", tc.sym.Name, err)
		}
		if got := FormatTraceValue(val); got != tc.want {
			t.Errorf("%s = %s, want %s", tc.sym.Name, got, tc.want)
		}
	}
}

// An instance of the requirement carries the constraint's value as a feature
// value; a redefinition in a specialization replaces it.
func TestOwnedConstraintValueMaterializesAndRedefines(t *testing.T) {
	ctx, pkg := ownedConstraintValueFixture(t)
	for _, tc := range []struct{ req, want string }{{"base", "false"}, {"sub", "true"}} {
		inst, err := ctx.Instantiate(requirementNamed(t, pkg, tc.req))
		if err != nil {
			t.Fatalf("Instantiate %s: %v", tc.req, err)
		}
		fv, err := inst.GetFeatureValue(ctx, "failed")
		if err != nil {
			t.Fatalf("%s.failed: %v", tc.req, err)
		}
		if got := FormatTraceValue(fv.HeldValue()); got != tc.want {
			t.Errorf("%s.failed = %s, want %s", tc.req, got, tc.want)
		}
	}
}

// The value is not the condition: a value-only constraint states nothing to
// check, exactly as `constraint plain = false;` does, so the check is no verdict
// rather than a vacuous pass.
func TestOwnedConstraintValueIsNotACondition(t *testing.T) {
	ctx, pkg := ownedConstraintValueFixture(t)
	base := requirementNamed(t, pkg, "Base")
	for _, sym := range []*symbols.Symbol{
		requirementNamed(t, pkg, "plain"),
		memberNamed(t, base, "failed"),
		memberNamed(t, base, "granted"),
	} {
		if conds := ctx.ConditionsOf(sym, sym.OwnerScope); len(conds) != 0 {
			t.Errorf("%s states [%s], want no condition", sym.Name, labelsOf(conds))
		}
		if err := RequireConstraint(sym); err != nil {
			t.Fatalf("%s: %v", sym.Name, err)
		}
		if _, err := ctx.EvaluateConstraintOn(sym, sym.OwnerScope, nil); !errors.Is(err, ErrNoConditions) {
			t.Errorf("%s: err = %v, want ErrNoConditions", sym.Name, err)
		}
	}
	for _, name := range []string{"Base", "Sub", "base", "sub"} {
		sym := requirementNamed(t, pkg, name)
		if _, err := ctx.EvaluateRequirementOn(sym, sym.OwnerScope, nil); !errors.Is(err, ErrNoConditions) {
			t.Errorf("%s: err = %v, want ErrNoConditions", name, err)
		}
	}
}

// Invoking a require constraint as a calc names its kind the way the notation does.
func TestOwnedConstraintIsDescribedByItsNotation(t *testing.T) {
	ctx, pkg := ownedConstraintValueFixture(t)
	failed := memberNamed(t, requirementNamed(t, pkg, "Base"), "failed")
	_, err := ctx.InvokeCalc(failed, nil, pkg)
	if !errors.Is(err, ErrNotACalc) {
		t.Fatalf("err = %v, want ErrNotACalc", err)
	}
	if !strings.Contains(err.Error(), "a require constraint usage") {
		t.Errorf("err = %v, want it described as a require constraint usage", err)
	}
}

// A require constraint typed by a definition reads the parameter values its own
// body binds (`in x = m;`), where the requirement evaluates it; a redefinition
// rebinding one parameter keeps the others.
func TestOwnedConstraintParametersBindInRequirement(t *testing.T) {
	ctx, pkg := conditionFixture(t, `
		package test {
			private import ScalarValues::Real;
			constraint def Below { in x : Real; in limit : Real; x < limit }
			requirement def Base {
				attribute m : Real = 300.0;
				require constraint n : Below { in x = m; in limit = 400.0; }
			}
			requirement def Sub :> Base {
				require constraint :>> n { in limit = 200.0; }
			}
			requirement base : Base;
			requirement sub : Sub;
		}
	`)
	for _, tc := range []struct {
		req  string
		want bool
	}{{"base", true}, {"sub", false}} {
		sym := requirementNamed(t, pkg, tc.req)
		holds, err := ctx.EvaluateRequirementOn(sym, sym.OwnerScope, nil)
		if tc.want {
			if err != nil || !holds {
				t.Errorf("%s: holds = %v, err = %v; want satisfied", tc.req, holds, err)
			}
			continue
		}
		var violation *ViolationError
		if !errors.As(err, &violation) {
			t.Errorf("%s: err = %v; want a violation of x < limit", tc.req, err)
		}
	}
}

// A named constraint nested in another reads the enclosing one's bound
// parameters (`in y = x` where the outer usage binds `x`), while its own
// parameters mask same-named ones of the outer usage (pilot 2026-07 accepts the shape).
func TestNestedOwnedConstraintReadsEnclosingParameters(t *testing.T) {
	ctx, pkg := conditionFixture(t, `
		package test {
			private import ScalarValues::Real;
			constraint def Below { in y : Real; in limit : Real; y < limit }
			requirement def Outer {
				subject s;
				in x : Real;
				in limit : Real;
				require constraint inner : Below { in y = x; in limit = 1000.0; }
			}
			requirement def Base {
				attribute m : Real default = 500.0;
				require constraint outer : Outer { in x = m; in limit = 400.0; }
			}
			requirement def High :> Base {
				attribute :>> m = 5000.0;
			}
			requirement base : Base;
			requirement high : High;
		}
	`)
	for _, tc := range []struct {
		req  string
		want bool
	}{{"base", true}, {"high", false}} {
		sym := requirementNamed(t, pkg, tc.req)
		conds := ctx.ConditionsOf(sym, sym.OwnerScope)
		if len(conds) != 1 || len(conds[0].Constraints) != 2 ||
			conds[0].Constraints[0].Name != "outer" || conds[0].Constraints[1].Name != "inner" {
			t.Fatalf("%s states [%s] through %v, want y < limit through [outer inner]", tc.req, labelsOf(conds), conds)
		}
		holds, err := ctx.EvaluateRequirementOn(sym, sym.OwnerScope, nil)
		if tc.want {
			if err != nil || !holds {
				t.Errorf("%s: holds = %v, err = %v; want satisfied", tc.req, holds, err)
			}
			continue
		}
		var violation *ViolationError
		if !errors.As(err, &violation) {
			t.Errorf("%s: err = %v; want a violation of y < limit", tc.req, err)
		}
	}
}

// A named constraint's parameters mask the subject and actors the requirement
// binds by name: `v` and `bound` in Below's body are its parameters, not the
// requirement's subject `v` or actor `bound` (pilot 2026-07 accepts the shape).
func TestOwnedConstraintParametersMaskSubjectAndActorBindings(t *testing.T) {
	ctx, pkg := conditionFixture(t, `
		package test {
			private import ScalarValues::Real;
			part def Vehicle { attribute mass : Real default = 300.0; }
			constraint def Below { in v : Real; in bound : Real; v < bound }
			part car : Vehicle;
			part station : Vehicle;
			requirement def Base {
				subject v : Vehicle;
				actor bound : Vehicle = station;
				attribute m : Real default = 5.0;
				require constraint below : Below { in v = m; in bound = 10.0; }
				require constraint { v.mass > 0.0 }
			}
			requirement def High :> Base {
				attribute :>> m = 50.0;
			}
			requirement base : Base;
			requirement high : High;
			satisfy base by car;
			satisfy high by car;
		}
	`)
	car, err := ctx.Instantiate(memberPath(t, pkg, "car"))
	if err != nil {
		t.Fatalf("instantiate car: %v", err)
	}
	assertions := ctx.SatisfyAssertionsIn(pkg)
	if len(assertions) != 2 {
		t.Fatalf("assertions = %d, want the two the package states", len(assertions))
	}
	for i, want := range []bool{true, false} {
		a := assertions[i]
		result, err := ctx.CheckSatisfactionOn(a, car)
		if want {
			if err != nil || !result.Holds {
				t.Errorf("%s: holds = %v, err = %v; want satisfied", a.Text(), result.Holds, err)
			}
			continue
		}
		var violation *ViolationError
		if !errors.As(err, &violation) || violation.Condition != "v < bound" {
			t.Errorf("%s: err = %v; want a violation of v < bound", a.Text(), err)
		}
	}
}
