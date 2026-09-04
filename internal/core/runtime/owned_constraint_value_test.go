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
