package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Failure modes of overload selection: each returns a typed error naming the
// declarations at play, never a silent pick.
func TestInvocationSelectionRobustness(t *testing.T) {
	t.Run("calc_call_ambiguous_between_two_imports", testCalcCallAmbiguousBetweenTwoImports)
	t.Run("calc_call_fits_no_visible_candidate", testCalcCallFitsNoVisibleCandidate)
	t.Run("calc_call_selects_by_argument_type", testCalcCallSelectsByArgumentType)
	t.Run("calc_call_selects_by_named_argument", testCalcCallSelectsByNamedArgument)
	t.Run("calc_call_selects_by_sibling_scalar_type", testCalcCallSelectsBySiblingScalarType)
	t.Run("calc_call_typed_parameter_beats_untyped", testCalcCallTypedParameterBeatsUntyped)
	t.Run("calc_call_repeated_named_argument_binds_last", testCalcCallRepeatedNamedArgumentBindsLast)
	t.Run("calc_call_selects_among_owned_inherited_and_recursive_import", testCalcCallSelectsAmongOwnedInheritedAndRecursiveImport)
	t.Run("action_call_selects_by_argument_type", testActionCallSelectsByArgumentType)
	t.Run("action_call_ambiguous_between_two_imports", testActionCallAmbiguousBetweenTwoImports)
	t.Run("action_call_receiver_binds_first_input", testActionCallReceiverBindsFirstInput)
	t.Run("action_call_receiver_with_named_arguments_refused", testActionCallReceiverWithNamedArgumentsRefused)
	t.Run("action_call_arguments_read_the_performing_object", testActionCallArgumentsReadThePerformingObject)
	t.Run("state_behavior_call_arguments_read_the_performing_object", testStateBehaviorCallArgumentsReadThePerformingObject)
	t.Run("action_call_argument_cannot_name_a_feature_out_of_scope", testActionCallArgumentCannotNameAFeatureOutOfScope)
}

// overloadedActionsSrc declares two imported same-named actions, told apart
// by the type of their one input.
const overloadedActionsSrc = `
	package A {
		private import ScalarValues::*;
		action def tag { in x : Integer; out code : Integer; first start; action set { assign code := 1; } done; succession first start then set; succession first set then done; }
	}
	package B {
		private import ScalarValues::*;
		action def tag { in x : %s; out code : Integer; first start; action set { assign code := 2; } done; succession first start then set; succession first set then done; }
	}
	package test {
		private import ScalarValues::*;
		private import A::*;
		private import B::*;
		action def Outer {
			attribute code : Integer = 0;
			first start;
			action call = tag(%s);
			done;
			succession first start then call;
			succession first call then done;
		}
	}
`

func runOverloadedAction(t *testing.T, paramType, arg string) (map[string]Value, error) {
	t.Helper()
	src := fmt.Sprintf(overloadedActionsSrc, paramType, arg)
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	outer := findSymbolByName(idx.DocumentRoot("<test>"), "Outer", ast.DefAction)
	if outer == nil {
		t.Fatal("Outer action not found")
	}
	return ctx.ExecuteAction(outer)
}

// A nested action invocation runs the same-named action its argument fits,
// as a calc call does.
func testActionCallSelectsByArgumentType(t *testing.T) {
	for arg, want := range map[string]int64{"3": 1, `"s"`: 2} {
		outputs, err := runOverloadedAction(t, "String", arg)
		if err != nil {
			t.Fatalf("tag(%s): %v", arg, err)
		}
		if got := intOutput(t, outputs, "code"); got != want {
			t.Errorf("tag(%s) code = %d, want %d", arg, got, want)
		}
	}
}

// receiverActionsSrc declares two same-named actions whose outputs depend on
// the input, so a call shows both which one ran and what it was bound to.
const receiverActionsSrc = `
	package A {
		private import ScalarValues::*;
		action def tag { in x : Integer; out code : Integer; first start; action set { assign code := x + 10; } done; succession first start then set; succession first set then done; }
	}
	package B {
		private import ScalarValues::*;
		action def tag { in x : Integer; in y : Integer; out code : Integer; first start; action set { assign code := x * 100 + y; } done; succession first start then set; succession first set then done; }
	}
	package test {
		private import ScalarValues::*;
		private import A::*;
		private import B::*;
		action def Outer {
			attribute code : Integer = 0;
			first start;
			action call = %s;
			done;
			succession first start then call;
			succession first call then done;
		}
	}
`

func runReceiverAction(t *testing.T, call string) (map[string]Value, error) {
	t.Helper()
	src := fmt.Sprintf(receiverActionsSrc, call)
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	outer := findSymbolByName(idx.DocumentRoot("<test>"), "Outer", ast.DefAction)
	if outer == nil {
		t.Fatal("Outer action not found")
	}
	return ctx.ExecuteAction(outer)
}

// A receiver written before a nested action call is its first argument, both
// for choosing among the same-named actions and for binding the chosen one.
func testActionCallReceiverBindsFirstInput(t *testing.T) {
	for call, want := range map[string]int64{"3->tag()": 13, "3->tag(4)": 304, "tag(3, 4)": 304} {
		outputs, err := runReceiverAction(t, call)
		if err != nil {
			t.Fatalf("%s: %v", call, err)
		}
		if got := intOutput(t, outputs, "code"); got != want {
			t.Errorf("%s code = %d, want %d", call, got, want)
		}
	}
}

// A receiver binds by position and so has no place beside named arguments.
func testActionCallReceiverWithNamedArgumentsRefused(t *testing.T) {
	outputs, err := runReceiverAction(t, "3->tag(y = 4)")
	if err == nil {
		t.Fatalf("expected a refusal, action returned %v", outputs)
	}
	if !errors.Is(err, ErrReceiverWithNamedArgs) {
		t.Fatalf("expected ErrReceiverWithNamedArgs, got: %v", err)
	}
}

// performedCallSrc has a part perform a behavior calling an action with the part's own
// feature, whose instance value differs from the default; receiver and argument forms.
const performedCallSrc = `
	package P {
		private import ScalarValues::*;
		action def report { in x : Integer; out code : Integer; first start; action set { assign code := x + 10; } done; succession first start then set; succession first set then done; }
		part def Rover {
			attribute speed : Integer = 7;
			action def drive {
				attribute code : Integer = 0;
				first start;
				%s
				done;
				succession first start then call;
				succession first call then done;
			}
			state def Drive {
				attribute code : Integer = 0;
				entry; then run;
				state run { %s }
				transition first run then done;
			}
		}
		part rover1 : Rover { attribute redefines speed = 9; }
	}
`

// performedCallRuntime builds the model with the given action and state calls
// and instantiates the performing part.
func performedCallRuntime(t *testing.T, actionCall, stateCall string) (*symbols.Index, *Context, *Instance) {
	t.Helper()
	src := fmt.Sprintf(performedCallSrc, actionCall, stateCall)
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	self, err := ctx.Instantiate(oneSymbol(t, idx, "P::rover1"))
	if err != nil {
		t.Fatalf("instantiate rover1: %v", err)
	}
	return idx, ctx, self
}

// A nested action call's arguments, receiver included, read the performing object:
// `speed->report()` passes the instance's speed, not the declared default.
func testActionCallArgumentsReadThePerformingObject(t *testing.T) {
	for _, call := range []string{"action call = speed->report();", "perform action call = report(speed);"} {
		idx, ctx, self := performedCallRuntime(t, call, "")
		outputs, err := ctx.ExecuteActionPerformedBy(oneSymbol(t, idx, "P::Rover::drive"), self, nil)
		if err != nil {
			t.Fatalf("%s: %v", call, err)
		}
		if got := intOutput(t, outputs, "code"); got != 19 {
			t.Errorf("%s code = %d, want 19 (rover1's speed 9 + 10)", call, got)
		}
	}
}

// A state behavior's action call reads the performing object the same way.
func testStateBehaviorCallArgumentsReadThePerformingObject(t *testing.T) {
	for _, call := range []string{"entry perform action w = speed->report();", "exit perform action w = report(speed);"} {
		idx, ctx, self := performedCallRuntime(t, "", call)
		data, _, err := ctx.ExecuteStatePerformedBy(oneSymbol(t, idx, "P::Rover::Drive"), self, nil)
		if err != nil {
			t.Fatalf("%s: %v", call, err)
		}
		if got := intOutput(t, data, "code"); got != 19 {
			t.Errorf("%s code = %d, want 19 (rover1's speed 9 + 10)", call, got)
		}
	}
}

// An argument reaches the performing object only through names the caller's
// scope resolves to its features, not through every feature the instance has.
func testActionCallArgumentCannotNameAFeatureOutOfScope(t *testing.T) {
	src := `package P {
		private import ScalarValues::*;
		action def report { in x : Integer; out code : Integer; first start; action set { assign code := x + 10; } done; succession first start then set; succession first set then done; }
		part def Rover { attribute speed : Integer = 7; }
		action def drive {
			attribute code : Integer = 0;
			first start;
			action call = report(speed);
			done;
			succession first start then call;
			succession first call then done;
		}
		part rover1 : Rover;
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	self, err := ctx.Instantiate(oneSymbol(t, idx, "P::rover1"))
	if err != nil {
		t.Fatalf("instantiate rover1: %v", err)
	}
	outputs, err := ctx.ExecuteActionPerformedBy(oneSymbol(t, idx, "P::drive"), self, nil)
	if err == nil {
		t.Fatalf("expected an unresolved reference, action returned %v", outputs)
	}
	if !errors.Is(err, ErrUnresolvedReference) || !strings.Contains(err.Error(), "speed") {
		t.Fatalf("expected ErrUnresolvedReference naming speed, got: %v", err)
	}
}

// Two imported actions taking the same argument type are refused as ambiguous.
func testActionCallAmbiguousBetweenTwoImports(t *testing.T) {
	outputs, err := runOverloadedAction(t, "Integer", "3")
	if err == nil {
		t.Fatalf("expected an ambiguity error, action returned %v", outputs)
	}
	if !errors.Is(err, ErrAmbiguousInvocation) {
		t.Fatalf("expected ErrAmbiguousInvocation, got: %v", err)
	}
	for _, want := range []string{"A::tag", "B::tag"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name candidate %s", err, want)
		}
	}
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

// An untyped parameter takes anything, so the candidate typing it runs for the
// arguments it fits, whichever import lists first.
func testCalcCallTypedParameterBeatsUntyped(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def pick { in x : Real; in y; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in x : Real; in y : Integer; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			calc typed { in v : Integer; pick(1, v) }
			calc loose { in v : Integer; pick(1, "b") }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	for calc, want := range map[string]int64{"typed": 2, "loose": 1} {
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

// A named argument written twice binds its last value, so the overload that
// value fits is the one run.
func testCalcCallRepeatedNamedArgumentBindsLast(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in x : String; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			calc lastStr { in i : Integer; in s : String; pick(x = i, x = s) }
			calc lastInt { in i : Integer; in s : String; pick(x = s, x = i) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	intArg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	for calc, want := range map[string]int64{"lastStr": 2, "lastInt": 1} {
		sym := findSymbolByName(rootScope, calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", calc)
		}
		result, err := ctx.InvokeCalc(sym, []Value{intArg, NewStringValue("s")}, rootScope)
		if err != nil {
			t.Fatalf("%s: %v", calc, err)
		}
		if result.Kind != ValConst || result.Const.Int != want {
			t.Fatalf("%s = %+v, want %d", calc, result, want)
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

// Overloads reached as owned members, through two generals, and through two descendants
// of one recursive import are all candidates; each call runs the one its argument fits.
func testCalcCallSelectsAmongOwnedInheritedAndRecursiveImport(t *testing.T) {
	src := `
		package Owned {
			private import ScalarValues::*;
			calc def pick { in x : Integer; return : Integer = 1; }
			calc def pick { in x : String; return : Integer = 2; }
			calc byInt { in v : Integer; pick(v) }
			calc byStr { in v : String; pick(v) }
			calc qualifiedInt { in v : Integer; Owned::pick(v) }
			calc qualifiedStr { in v : String; Owned::pick(v) }
		}
		package Inherited {
			private import ScalarValues::*;
			calc def ByNumber { calc def pick { in x : Integer; return : Integer = 1; } }
			calc def ByText { calc def pick { in x : String; return : Integer = 2; } }
			calc def byInt :> ByNumber, ByText { in v : Integer; return : Integer = pick(v); }
			calc def byStr :> ByNumber, ByText { in v : String; return : Integer = pick(v); }
			calc def Base {
				calc def pick { in x : Integer; return : Integer = 1; }
				calc def pick { in x : String; return : Integer = 2; }
			}
			calc def oneBaseInt :> Base { in v : Integer; return : Integer = pick(v); }
			calc def oneBaseStr :> Base { in v : String; return : Integer = pick(v); }
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
		{"Owned::qualifiedInt", intArg, 1},
		{"Owned::qualifiedStr", NewStringValue("s"), 2},
		{"Inherited::byInt", intArg, 1},
		{"Inherited::byStr", NewStringValue("s"), 2},
		{"Inherited::oneBaseInt", intArg, 1},
		{"Inherited::oneBaseStr", NewStringValue("s"), 2},
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
