package runtime

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"strings"
	"testing"
)

// partChainModel declares a calc usage nested in a part, whose outputs a feature
// chain through the part reads: `lander.mass.mDry`.
const partChainModel = `
package test {
	private import ScalarValues::*;
	calc def MassEstimate {
		in mPropUsable : Real;
		out mProp = mPropUsable * 1.02;
		out mDry = 100.0 + mPropUsable * 0.4;
		out mWet = mDry + mProp;
	}
	part lander {
		calc mass : MassEstimate { in mPropUsable = 250.0; }
		attribute dryMass : Real = mass.mDry;
		part tank {
			attribute volume : Real = 3.0;
		}
	}
	attribute throughPart : Real = lander.mass.mDry;
	attribute throughAttribute : Real = lander.dryMass;
	attribute throughNestedPart : Real = lander.tank.volume;
}
`

// evalNamedAttribute evaluates the value binding of the named attribute of
// package test in src.
func evalNamedAttribute(t *testing.T, src, name string) (Value, error) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg == nil || pkg.Scope == nil {
		t.Fatal("package test not found")
	}
	sym, ok := pkg.Scope.LookupLocal(name)
	if !ok || sym == nil {
		t.Fatalf("attribute %s not found", name)
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Value == nil {
		t.Fatalf("attribute %s binds no value", name)
	}
	return ctx.EvalWithScope(usage.Value, sym.OwnerScope)
}

// TestCalcUsageReadThroughPartChain requires a feature chain through a part to
// read the outputs of a calc usage the part declares, in that part's context.
func TestCalcUsageReadThroughPartChain(t *testing.T) {
	for _, tt := range []struct {
		name string
		want float64
	}{
		{"throughPart", 200.0},
		{"throughAttribute", 200.0},
		{"throughNestedPart", 3.0},
	} {
		got, err := evalNamedAttribute(t, partChainModel, tt.name)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if got.Const.Real != tt.want {
			t.Errorf("%s = %s, want %v", tt.name, FormatTraceValue(got), tt.want)
		}
	}
}

// TestCalcUsageChainDiagnostics requires the chain to name what went wrong: a
// usage read without naming an output, and a name it declares no output for.
func TestCalcUsageChainDiagnostics(t *testing.T) {
	tests := []struct {
		expr string
		want []string
	}{
		{"lander.mass", []string{"mDry", "mWet", "read one of them"}},
		{"lander.mass.nope", []string{"nope", "mDry"}},
		{"lander.nope", []string{"nope"}},
	}
	for _, tt := range tests {
		src := strings.Replace(
			partChainModel,
			"attribute throughPart : Real = lander.mass.mDry;",
			"attribute probe : Real = "+tt.expr+";",
			1,
		)
		_, err := evalNamedAttribute(t, src, "probe")
		if err == nil {
			t.Errorf("%s: want a diagnostic, got a value", tt.expr)
			continue
		}
		for _, want := range tt.want {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: error = %v, want it to mention %q", tt.expr, err, want)
			}
		}
	}
}
