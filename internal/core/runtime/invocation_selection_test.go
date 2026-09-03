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
	t.Run("calc_call_selects_by_sibling_scalar_type", testCalcCallSelectsBySiblingScalarType)
	t.Run("calc_call_selects_among_owned_inherited_and_recursive_import", testCalcCallSelectsAmongOwnedInheritedAndRecursiveImport)
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

// Two candidates whose parameters specialize the same scalar type are told
// apart by the argument's declared type, not by the scalar both reduce to.
func testCalcCallSelectsBySiblingScalarType(t *testing.T) {
	src := `
		package Q { private import ScalarValues::*; attribute def Mass :> Real; attribute def Volume :> Real; }
		package A { private import ScalarValues::*; private import Q::*; calc def pick { in m : Mass; return : Integer = 1; } }
		package B { private import ScalarValues::*; private import Q::*; calc def pick { in v : Volume; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import Q::*;
			private import A::*;
			private import B::*;
			calc byMass { in kg : Mass; pick(kg) }
			calc byVolume { in litre : Volume; pick(litre) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	for calc, want := range map[string]int64{"byMass": 1, "byVolume": 2} {
		sym := findSymbolByName(rootScope, calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", calc)
		}
		arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 3}}
		result, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
		if err != nil {
			t.Fatalf("%s: %v", calc, err)
		}
		if result.Kind != ValConst || result.Const.Int != want {
			t.Fatalf("%s = %+v, want %d", calc, result, want)
		}
	}
}

// Overloads reached as owned members, through two generals, and through two
// descendants of one recursive import are all candidates, and each call runs
// the one its argument fits.
func testCalcCallSelectsAmongOwnedInheritedAndRecursiveImport(t *testing.T) {
	src := `
		package Owned {
			private import ScalarValues::*;
			calc def pick { in x : Integer; return : Integer = 1; }
			calc def pick { in x : String; return : Integer = 2; }
			calc byInt { in v : Integer; pick(v) }
			calc byStr { in v : String; pick(v) }
		}
		package Inherited {
			private import ScalarValues::*;
			calc def ByNumber { calc def pick { in x : Integer; return : Integer = 1; } }
			calc def ByText { calc def pick { in x : String; return : Integer = 2; } }
			calc def byInt :> ByNumber, ByText { in v : Integer; return : Integer = pick(v); }
			calc def byStr :> ByNumber, ByText { in v : String; return : Integer = pick(v); }
		}
		package Recursive {
			private import ScalarValues::*;
			package Lib {
				package Numbers { calc def pick { in x : Integer; return : Integer = 1; } }
				package Text { calc def pick { in x : String; return : Integer = 2; } }
			}
			package C {
				private import ScalarValues::*;
				private import Lib::**;
				calc byInt { in v : Integer; pick(v) }
				calc byStr { in v : String; pick(v) }
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	intArg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	cases := []struct {
		qual string
		arg  Value
		want int64
	}{
		{"Owned::byInt", intArg, 1},
		{"Owned::byStr", NewStringValue("s"), 2},
		{"Inherited::byInt", intArg, 1},
		{"Inherited::byStr", NewStringValue("s"), 2},
		{"Recursive::C::byInt", intArg, 1},
		{"Recursive::C::byStr", NewStringValue("s"), 2},
	}
	for _, tc := range cases {
		matches := idx.LookupQualified(tc.qual)
		if len(matches) != 1 {
			t.Fatalf("%s: %d symbols, want 1", tc.qual, len(matches))
		}
		result, err := ctx.InvokeCalc(matches[0], []Value{tc.arg}, rootScope)
		if err != nil {
			t.Fatalf("%s: %v", tc.qual, err)
		}
		if result.Kind != ValConst || result.Const.Int != tc.want {
			t.Fatalf("%s = %+v, want %d", tc.qual, result, tc.want)
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
