package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// Failure modes of overload selection: each returns a typed error naming the
// declarations at play, never a silent pick.
func TestInvocationSelectionRobustness(t *testing.T) {
	t.Run("calc_call_ambiguous_between_two_imports", testCalcCallAmbiguousBetweenTwoImports)
	t.Run("calc_call_fits_no_visible_candidate", testCalcCallFitsNoVisibleCandidate)
	t.Run("calc_call_selects_by_argument_type", testCalcCallSelectsByArgumentType)
	t.Run("calc_call_selects_by_named_argument", testCalcCallSelectsByNamedArgument)
}

// Two imported calcs with the same name take the same argument type, and the
// call is refused as ambiguous rather than answered by whichever came first.
func testCalcCallAmbiguousBetweenTwoImports(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			calc choose { in v : Integer; pick(v) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "choose", ast.DefCalc)
	if sym == nil {
		t.Fatal("choose calc not found")
	}
	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	result, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if err == nil {
		t.Fatalf("expected an ambiguity error, calc returned %+v", result)
	}
	if !errors.Is(err, ErrAmbiguousInvocation) {
		t.Fatalf("expected ErrAmbiguousInvocation, got: %v", err)
	}
	for _, want := range []string{"A::pick", "B::pick"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name candidate %s", err, want)
		}
	}
}

// No candidate takes a String, so the call fails with a type mismatch that
// still names the first candidate's expectation.
func testCalcCallFitsNoVisibleCandidate(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in x : Boolean; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			calc choose { in v : String; pick(v) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "choose", ast.DefCalc)
	if sym == nil {
		t.Fatal("choose calc not found")
	}
	arg := NewStringValue("x")
	result, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if err == nil {
		t.Fatalf("expected a type mismatch, calc returned %+v", result)
	}
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("expected ErrTypeMismatch, got: %v", err)
	}
}

// The candidate whose parameter type the argument conforms to is the one run.
func testCalcCallSelectsByArgumentType(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in x : String; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			calc chooseInt { in v : Integer; pick(v) }
			calc chooseStr { in v : String; pick(v) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	cases := []struct {
		calc string
		arg  Value
		want int64
	}{
		{"chooseInt", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}, 1},
		{"chooseStr", NewStringValue("s"), 2},
	}
	for _, tc := range cases {
		sym := findSymbolByName(rootScope, tc.calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", tc.calc)
		}
		result, err := ctx.InvokeCalc(sym, []Value{tc.arg}, rootScope)
		if err != nil {
			t.Fatalf("%s: %v", tc.calc, err)
		}
		if result.Kind != ValConst || result.Const.Int != tc.want {
			t.Fatalf("%s = %+v, want %d", tc.calc, result, tc.want)
		}
	}
}

// A named argument binds by the parameter's name, which is what tells two
// same-arity candidates apart.
func testCalcCallSelectsByNamedArgument(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def pick { in width : Integer; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in height : Integer; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			calc byWidth { in v : Integer; pick(width = v) }
			calc byHeight { in v : Integer; pick(height = v) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	for calc, want := range map[string]int64{"byWidth": 1, "byHeight": 2} {
		sym := findSymbolByName(rootScope, calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", calc)
		}
		arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
		result, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
		if err != nil {
			t.Fatalf("%s: %v", calc, err)
		}
		if result.Kind != ValConst || result.Const.Int != want {
			t.Fatalf("%s = %+v, want %d", calc, result, want)
		}
	}
}
