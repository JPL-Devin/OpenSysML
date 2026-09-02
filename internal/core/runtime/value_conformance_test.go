package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// valueConformanceContext builds a runtime over a model whose feature values and calc
// arguments are typed wider than their targets, so the type checker defers them.
func valueConformanceContext(t *testing.T) (*Context, *symbols.Index, *symbols.Scope) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			attribute seven : Real = 7.0;
			attribute two : Real = 2.0;
			attribute xs = (1, 2, 3);

			part def P {
				attribute exact : Integer = 3;
				attribute whole : Integer = 4 / 2;
				attribute half : Integer = 7 / 2;
				attribute nat : Natural = 4 / 2;
				attribute neg : Natural = -4 / 2;
				attribute fromReal : Integer = two;
				attribute computed : Integer = seven - two;
				attribute computedHalf : Integer = seven / two;
			}
			part p : P;
			part q : P { attribute :>> half = 4 / 2; }

			calc def Half { in x : Integer; return : Rational = x / 2; }
			calc def Twice { in x : Integer = 7 / 2; return : Integer = x * 2; }
			calc def Base { in x : Integer; return : Integer = x; }
			calc def Derived :> Base;
			calc def Redef :> Base { in :>> x = 7 / 2; }
			calc def IntDiv { return : Integer = 7 / 2; }
			calc def WholeDiv { return : Integer = 4 / 2; }
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return ctx, idx, pkg.Scope
}

func instantiateNamed(t *testing.T, ctx *Context, idx *symbols.Index, qualified string) *Instance {
	t.Helper()
	matches := idx.LookupQualified(qualified)
	if len(matches) != 1 {
		t.Fatalf("%s: %d matching symbols, want 1", qualified, len(matches))
	}
	inst, err := ctx.Instantiate(matches[0])
	if err != nil {
		t.Fatalf("Instantiate %s: %v", qualified, err)
	}
	return inst
}

// A feature value is a binding, so the value materialized for a default must be
// an instance of the declared type: 4 / 2 is the Integer 2, 7 / 2 is not one.
func TestDefaultValueMustConformAtMaterialization(t *testing.T) {
	ctx, idx, _ := valueConformanceContext(t)
	inst := instantiateNamed(t, ctx, idx, "test::p")

	for _, tc := range []struct {
		feature string
		want    string
	}{
		{"exact", "3"},
		{"whole", "2.0"},
		{"nat", "2.0"},
		{"fromReal", "2.0"},
		{"computed", "5.0"},
	} {
		fv, err := inst.GetFeatureValue(ctx, tc.feature)
		if err != nil {
			t.Errorf("%s: %v", tc.feature, err)
			continue
		}
		if got := FormatTraceValue(fv.HeldValue()); got != tc.want {
			t.Errorf("%s = %s, want %s", tc.feature, got, tc.want)
		}
	}
	for _, feature := range []string{"half", "neg", "computedHalf"} {
		if _, err := inst.GetFeatureValue(ctx, feature); !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: error = %v, want ErrTypeMismatch", feature, err)
		}
	}
}

// A usage restating a default binds its own value, which is checked the same way.
func TestRestatedDefaultConformsByItsOwnValue(t *testing.T) {
	ctx, idx, _ := valueConformanceContext(t)
	inst := instantiateNamed(t, ctx, idx, "test::q")
	fv, err := inst.GetFeatureValue(ctx, "half")
	if err != nil {
		t.Fatalf("half: %v", err)
	}
	if got := FormatTraceValue(fv.HeldValue()); got != "2.0" {
		t.Errorf("q.half = %s, want 2.0", got)
	}
}

// Reading a package-level declaration checks its value against its type too.
func TestDeclaredValueReadChecksItsType(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			attribute whole : Integer = 4 / 2;
			attribute half : Integer = 7 / 2;
		}
	`))
	symbol := func(name string) *symbols.Symbol {
		matches := idx.LookupQualified("test::" + name)
		if len(matches) != 1 {
			t.Fatalf("test::%s: %d matching symbols, want 1", name, len(matches))
		}
		return matches[0]
	}
	if got, err := ctx.EvalDeclaredValue(symbol("whole")); err != nil || FormatTraceValue(got) != "2.0" {
		t.Errorf("whole = %s, %v; want 2.0", FormatTraceValue(got), err)
	}
	if _, err := ctx.EvalDeclaredValue(symbol("half")); !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("half: error = %v, want ErrTypeMismatch", err)
	}
}

// A positional, named or default argument — through an inherited or redefined
// parameter too — must be a value of the parameter's type; so must the result.
func TestCalcParameterAndResultMustConform(t *testing.T) {
	ctx, _, scope := valueConformanceContext(t)

	for _, tc := range []struct {
		expr string
		want string
	}{
		{"Half(4 / 2)", "1.0"},
		{"Half(x = 4 / 2)", "1.0"},
		{"Half(two)", "1.0"},
		{"Twice(2)", "4"},
		{"Twice(x = 4 / 2)", "4.0"},
		{"Derived(4 / 2)", "2.0"},
		{"Redef(4 / 2)", "2.0"},
		{"Redef(x = 4 / 2)", "2.0"},
		{"WholeDiv()", "2.0"},
	} {
		got, err := evalIn(t, ctx, scope, tc.expr)
		if err != nil {
			t.Errorf("%s: %v", tc.expr, err)
			continue
		}
		if s := FormatTraceValue(got); s != tc.want {
			t.Errorf("%s = %s, want %s", tc.expr, s, tc.want)
		}
	}
	for _, expr := range []string{
		"Half(7 / 2)",
		"Half(1.5)",
		"Half(x = 1.5)",
		"Half(seven / two)",
		"Twice()",
		"Derived(1.5)",
		"Redef(1.5)",
		"Redef()",
		"IntDiv()",
	} {
		if _, err := evalIn(t, ctx, scope, expr); !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: error = %v, want ErrTypeMismatch", expr, err)
		}
	}
}

// A whole-valued Real names a position; a fractional one names none, and is
// never truncated to one.
func TestSequenceIndexAcceptsAWholeValuedReal(t *testing.T) {
	ctx, _, scope := valueConformanceContext(t)

	for _, tc := range []struct {
		expr string
		want string
	}{
		{"xs#(2)", "2"},
		{"xs#(4 / 2)", "2"},
		{"xs#(two)", "2"},
		{"xs#(6 / 2)", "3"},
	} {
		got, err := evalIn(t, ctx, scope, tc.expr)
		if err != nil {
			t.Errorf("%s: %v", tc.expr, err)
			continue
		}
		if s := FormatTraceValue(got); s != tc.want {
			t.Errorf("%s = %s, want %s", tc.expr, s, tc.want)
		}
	}
	for _, expr := range []string{"xs#(1.5)", "xs#(7 / 2)", "xs#(seven / two)", "xs#(true)"} {
		if _, err := evalIn(t, ctx, scope, expr); !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: error = %v, want ErrTypeMismatch", expr, err)
		}
	}
	for _, expr := range []string{"xs#(0)", "xs#(-1)", "xs#(8 / 2)", "xs#(0 / 2)"} {
		if _, err := evalIn(t, ctx, scope, expr); !errors.Is(err, ErrIndexOutOfRange) {
			t.Errorf("%s: error = %v, want ErrIndexOutOfRange", expr, err)
		}
	}
}
