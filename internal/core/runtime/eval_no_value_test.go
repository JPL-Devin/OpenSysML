package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// valuelessFeaturesModel declares features nothing gives a value to, single-
// and multi-valued, in a scope an expression may read them from.
const valuelessFeaturesModel = `
package test {
	private import ScalarValues::*;
	part def Wheel { attribute radius : Real = 0.3; }
	part def Car {
		attribute mass : Real = 1500.0;
		attribute unsetMass : Real;
		attribute tags : String[3];
		part wheels : Wheel[4];
	}
}
`

// TestDeclaredFeatureWithoutValueIsNotUnresolved: a name that resolves to a
// feature nothing gives a value to reports the missing value, single- or
// multi-valued, and is never reported as a name that fails to resolve — which
// is reserved for names no declaration answers.
func TestDeclaredFeatureWithoutValueIsNotUnresolved(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, valuelessFeaturesModel)
	ctx := NewContext(model, resolver, 10000)
	pkg, _ := root.LookupLocal("test")
	car, _ := pkg.Scope.LookupLocal("Car")
	scope := car.Scope

	if got, err := ctx.EvalWithScope(parseExpr(t, "mass"), scope); err != nil {
		t.Fatalf("mass: %v", err)
	} else if got.Kind != ValConst {
		t.Fatalf("mass = %s, want a value", FormatTraceValue(got))
	}

	for _, name := range []string{"unsetMass", "tags", "wheels", "wheels.radius", "test::Car::wheels"} {
		expr := parseExpr(t, name)
		_, err := ctx.EvalWithScope(expr, scope)
		var noValue *NoValueError
		if !errors.As(err, &noValue) {
			t.Errorf("%s: err = %v; want NoValueError", name, err)
			continue
		}
		if errors.Is(err, ErrUnresolvedReference) {
			t.Errorf("%s: a declared feature was reported as unresolved: %v", name, err)
		}
		if noValue.Ref == nil || !namesIn(expr, noValue.Ref) {
			t.Errorf("%s: NoValueError.Ref = %v; want the name read in the expression", name, noValue.Ref)
		}
	}

	_, err := ctx.EvalWithScope(parseExpr(t, "nonexistent"), scope)
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Errorf("nonexistent: err = %v; want ErrUnresolvedReference", err)
	}
}

// namesIn reports whether qn is one of the names expr is written as.
func namesIn(expr ast.Node, qn *ast.QualifiedName) bool {
	switch n := expr.(type) {
	case *ast.FeatureReference:
		return n.Name == qn
	case *ast.FeatureChainExpr:
		return n.Member == qn || namesIn(n.Operand, qn)
	default:
		return false
	}
}
