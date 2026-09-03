package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Failure modes of overload selection: each returns a typed error naming the
// declarations at play, never a silent pick.
func TestInvocationSelectionRobustness(t *testing.T) {
	t.Run("calc_call_ambiguous_between_two_imports", testCalcCallAmbiguousBetweenTwoImports)
	t.Run("calc_call_ambiguity_names_only_the_tied_best", testCalcCallAmbiguityNamesOnlyTheTiedBest)
	t.Run("calc_call_fits_no_visible_candidate", testCalcCallFitsNoVisibleCandidate)
	t.Run("calc_call_fits_no_visible_candidate_behind_a_non_callable", testCalcCallFitsNoVisibleCandidateBehindANonCallable)
	t.Run("calc_call_selects_by_argument_type", testCalcCallSelectsByArgumentType)
	t.Run("global_root_call_selects_by_argument_type", testGlobalRootCallSelectsByArgumentType)
	t.Run("bare_call_selects_among_other_documents_root_declarations", testBareCallSelectsAmongOtherDocumentsRootDeclarations)
	t.Run("bare_call_reaches_private_root_declarations_of_other_documents", testBareCallReachesPrivateRootDeclarationsOfOtherDocuments)
	t.Run("calc_call_selects_by_named_argument", testCalcCallSelectsByNamedArgument)
	t.Run("calc_call_selects_by_sibling_scalar_type", testCalcCallSelectsBySiblingScalarType)
	t.Run("calc_call_typed_parameter_beats_untyped", testCalcCallTypedParameterBeatsUntyped)
	t.Run("calc_call_explicit_anything_ties_with_untyped", testCalcCallExplicitAnythingTiesWithUntyped)
	t.Run("calc_call_crossed_specificity_is_ambiguous", testCalcCallCrossedSpecificityIsAmbiguous)
	t.Run("calc_call_repeated_named_argument_binds_last", testCalcCallRepeatedNamedArgumentBindsLast)
	t.Run("calc_call_selects_among_owned_inherited_and_recursive_import", testCalcCallSelectsAmongOwnedInheritedAndRecursiveImport)
	t.Run("calc_call_selects_among_inherited_imports", testCalcCallSelectsAmongInheritedImports)
	t.Run("calc_call_selects_by_collection_literal_element_type", testCalcCallSelectsByCollectionLiteralElementType)
	t.Run("action_call_selects_by_argument_type", testActionCallSelectsByArgumentType)
	t.Run("action_call_ambiguous_between_two_imports", testActionCallAmbiguousBetweenTwoImports)
	t.Run("action_call_selects_an_action_over_a_more_specific_calc", testActionCallSelectsAnActionOverAMoreSpecificCalc)
	t.Run("action_call_naming_only_a_calc_is_not_an_action", testActionCallNamingOnlyACalcIsNotAnAction)
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

// An action performed by name runs an action of that name: a same-named calc a
// calc call would prefer, its parameter fitting the argument more closely, is not a
// candidate for an action performance.
func testActionCallSelectsAnActionOverAMoreSpecificCalc(t *testing.T) {
	src := `
		package A {
			private import ScalarValues::*;
			calc def tag { in x : Integer; return : Integer = x + 1; }
		}
		package B {
			private import ScalarValues::*;
			action def tag { in x : Real; out code : Integer; first start; action set { assign code := 2; } done; succession first start then set; succession first set then done; }
		}
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			action def Outer {
				attribute code : Integer = 0;
				first start;
				action call = tag(3);
				done;
				succession first start then call;
				succession first call then done;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	outer := findSymbolByName(idx.DocumentRoot("<test>"), "Outer", ast.DefAction)
	if outer == nil {
		t.Fatal("Outer action not found")
	}
	outputs, err := ctx.ExecuteAction(outer)
	if err != nil {
		t.Fatalf("tag(3): %v", err)
	}
	if got := intOutput(t, outputs, "code"); got != 2 {
		t.Errorf("tag(3) code = %d, want 2 from B::tag", got)
	}
}

// An action performed by a name only calcs declare is refused as not an action,
// whether the name denotes one calc or several.
func testActionCallNamingOnlyACalcIsNotAnAction(t *testing.T) {
	const src = `
		package A {
			private import ScalarValues::*;
			calc def tag { in x : Integer; return : Integer = x + 1; }
		}
		package B {
			private import ScalarValues::*;
			calc def tag { in x : String; return : Integer = 2; }
		}
		package test {
			private import ScalarValues::*;
			private import A::*;
			%s
			action def Outer {
				attribute code : Integer = 0;
				first start;
				action call = tag(3);
				done;
				succession first start then call;
				succession first call then done;
			}
		}
	`
	for _, imports := range []string{``, `private import B::*;`} {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, fmt.Sprintf(src, imports)))
		outer := findSymbolByName(idx.DocumentRoot("<test>"), "Outer", ast.DefAction)
		if outer == nil {
			t.Fatal("Outer action not found")
		}
		_, err := ctx.ExecuteAction(outer)
		if err == nil || !strings.Contains(err.Error(), "tag is not an action (calcDef)") {
			t.Errorf("imports %q: error = %v, want 'tag is not an action (calcDef)'", imports, err)
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

// A broader overload the arguments also fit is beaten by the tied ones, so the
// ambiguity error names only those.
func testCalcCallAmbiguityNamesOnlyTheTiedBest(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = 2; } }
		package C { private import ScalarValues::*; calc def pick { in x : Real; return : Integer = 3; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			private import C::*;
			calc choose { pick(3) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "choose", ast.DefCalc)
	if sym == nil {
		t.Fatal("choose calc not found")
	}
	_, err := ctx.InvokeCalc(sym, nil, rootScope)
	if !errors.Is(err, ErrAmbiguousInvocation) {
		t.Fatalf("expected ErrAmbiguousInvocation, got: %v", err)
	}
	if !strings.HasSuffix(err.Error(), "pick denotes A::pick, B::pick") {
		t.Errorf("error %q should name exactly the tied overloads A::pick, B::pick", err)
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

// A non-callable declaration found before the callable one does not take the
// call: the mismatch is reported against the callable's parameter.
func testCalcCallFitsNoVisibleCandidateBehindANonCallable(t *testing.T) {
	src := `
		package A { attribute def pick; }
		package B { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = x; } }
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
	result, err := ctx.InvokeCalc(sym, []Value{NewStringValue("x")}, rootScope)
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

// `$::pick(v)` selects among every root-level pick, not only the first indexed;
// the name is built since the expression parser does not read `$::`.
func testGlobalRootCallSelectsByArgumentType(t *testing.T) {
	src := `
		private import ScalarValues::*;
		calc def pick { in x : Integer; return : Integer = 1; }
		calc def pick { in x : String; return : Integer = 2; }
		calc def pick { in x : Boolean; return : Integer = 3; }
		package test { calc def pick { in x : Boolean; return : Integer = 4; } }
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	pkg, ok := rootScope.LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("package test not indexed")
	}
	call := func(arg ast.Node) *ast.InvocationExpr {
		qn := &ast.QualifiedName{Global: true}
		qn.Parts = append(qn.Parts, ast.NameSegment{Text: "pick"})
		return &ast.InvocationExpr{Type: qn, Args: []ast.Node{arg}}
	}
	cases := []struct {
		name string
		arg  ast.Node
		want string
	}{
		{"3", &ast.LiteralInteger{Value: "3"}, "1"},
		{`"s"`, &ast.LiteralString{Value: "s"}, "2"},
		{"true", &ast.LiteralBool{Value: true}, "3"},
	}
	for _, tc := range cases {
		val, err := ctx.EvalWithScope(call(tc.arg), pkg.Scope)
		if err != nil || FormatValue(val) != tc.want {
			t.Errorf("$::pick(%s) = (%v, %v), want %s", tc.name, val, err, tc.want)
		}
	}
}

// A bare `pick(v)` that only other documents' root declarations answer selects
// among all of them, not only the first indexed.
func testBareCallSelectsAmongOtherDocumentsRootDeclarations(t *testing.T) {
	idx := libs.NewModelIndex()
	idx.AddDocument("ints.sysml", parseAndBuild(t, `
		private import ScalarValues::*;
		calc def pick { in x : Integer; return : Integer = 1; }
		calc def pick { in x : String; return : Integer = 2; }
	`))
	idx.AddDocument("bools.sysml", parseAndBuild(t, `
		private import ScalarValues::*;
		calc def pick { in x : Boolean; return : Integer = 3; }
	`))
	idx.AddDocument("<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			calc byInt { pick(3) }
			calc byString { pick("s") }
			calc byBool { pick(true) }
		}
	`))
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	ctx := NewContext(semantics.NewModel(resolver), resolver, 10000)
	rootScope := idx.DocumentRoot("<test>")
	for calc, want := range map[string]int64{"byInt": 1, "byString": 2, "byBool": 3} {
		sym := findSymbolByName(rootScope, calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", calc)
		}
		result, err := ctx.InvokeCalc(sym, nil, rootScope)
		if err != nil {
			t.Fatalf("%s: %v", calc, err)
		}
		if result.Kind != ValConst || result.Const.Int != want {
			t.Fatalf("%s = %+v, want %d", calc, result, want)
		}
	}
}

// A root namespace has no owner to hide its members from (KerML 8.2.3.5; the pilot
// resolves them alike), so a private or protected root overload in another
// document is selected exactly as a public one is.
func testBareCallReachesPrivateRootDeclarationsOfOtherDocuments(t *testing.T) {
	idx := libs.NewModelIndex()
	idx.AddDocument("ints.sysml", parseAndBuild(t, `
		private import ScalarValues::*;
		calc def pick { in x : Integer; return : Integer = 1; }
	`))
	idx.AddDocument("strings.sysml", parseAndBuild(t, `
		private import ScalarValues::*;
		private calc def pick { in x : String; return : Integer = 2; }
	`))
	idx.AddDocument("bools.sysml", parseAndBuild(t, `
		private import ScalarValues::*;
		protected calc def pick { in x : Boolean; return : Integer = 3; }
	`))
	idx.AddDocument("<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			calc byInt { pick(3) }
			calc byString { pick("s") }
			calc byBool { pick(true) }
		}
	`))
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	ctx := NewContext(semantics.NewModel(resolver), resolver, 10000)
	rootScope := idx.DocumentRoot("<test>")
	for calc, want := range map[string]int64{"byInt": 1, "byString": 2, "byBool": 3} {
		sym := findSymbolByName(rootScope, calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", calc)
		}
		result, err := ctx.InvokeCalc(sym, nil, rootScope)
		if err != nil {
			t.Fatalf("%s: %v", calc, err)
		}
		if result.Kind != ValConst || result.Const.Int != want {
			t.Fatalf("%s = %+v, want %d", calc, result, want)
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

// A parameter written `: Anything` and one declaring no type are the same parameter,
// so two candidates differing only in that spelling tie and the call is refused.
func testCalcCallExplicitAnythingTiesWithUntyped(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; private import Base::Anything; calc def pick { in x : Real; in y : Anything; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in x : Real; in y; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			calc choose { in w : Real; in v : Integer; pick(w, v) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "choose", ast.DefCalc)
	if sym == nil {
		t.Fatal("choose calc not found")
	}
	real := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 1.5}}
	integer := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	result, err := ctx.InvokeCalc(sym, []Value{real, integer}, rootScope)
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

// Candidates each narrower on a different parameter are incomparable: neither is
// selected, whether the wide parameter is written `: Anything` or left untyped.
func testCalcCallCrossedSpecificityIsAmbiguous(t *testing.T) {
	src := `
		package Types { attribute def Foo; }
		package A { private import ScalarValues::*; private import Base::Anything; calc def pick { in x : Anything; in y : Real; return : Integer = 1; } }
		package B { private import ScalarValues::*; private import Types::Foo; calc def pick { in x : Foo; in y; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import Types::Foo;
			private import A::*;
			private import B::*;
			calc choose { in f : Foo; in w : Real; pick(f, w) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "choose", ast.DefCalc)
	if sym == nil {
		t.Fatal("choose calc not found")
	}
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "Types::Foo"))
	if err != nil {
		t.Fatalf("instantiate Foo: %v", err)
	}
	foo := Value{Kind: ValInstance, Instance: inst.ID}
	real := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 1.5}}
	result, err := ctx.InvokeCalc(sym, []Value{foo, real}, rootScope)
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

// Every protected import of a general type contributes its overload to the
// specializing body, not only the first import's.
func testCalcCallSelectsAmongInheritedImports(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in x : String; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			part def Base { protected import A::*; protected import B::*; }
			part def Derived :> Base {
				attribute i : Integer = pick(3);
				attribute s : Integer = pick("s");
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	derived := findSymbolByName(idx.DocumentRoot("<test>"), "Derived", ast.DefPart)
	if derived == nil {
		t.Fatal("Derived part def not found")
	}
	inst, err := ctx.Instantiate(derived)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	for attr, want := range map[string]string{"i": "1", "s": "2"} {
		fv, err := inst.GetFeatureValue(ctx, attr)
		if err != nil {
			t.Fatalf("%s: %v", attr, err)
		}
		if got := FormatTraceValue(fv.HeldValue()); got != want {
			t.Fatalf("%s = %s, want %s", attr, got, want)
		}
	}
}

// A collection literal is typed by its elements, so overloads differing only
// in element type are told apart.
func testCalcCallSelectsByCollectionLiteralElementType(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def count { in xs : String[*]; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def count { in xs : Integer[*]; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			calc ofText { count(("a", "b")) }
			calc ofNumbers { count((1, 2, 3)) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	for calc, want := range map[string]int64{"ofText": 1, "ofNumbers": 2} {
		sym := findSymbolByName(rootScope, calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", calc)
		}
		result, err := ctx.InvokeCalc(sym, nil, rootScope)
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
		package Qualified {
			private import ScalarValues::*;
			package A { calc def pick { in x : Integer; return : Integer = 1; } }
			package B { calc def pick { in x : String; return : Integer = 2; } }
			package Both {
				public import A::*;
				public import B::*;
			}
			calc def Derived :> Inherited::ByNumber, Inherited::ByText;
			calc def Importing :> Inherited::ByText { public import A::*; }
			calc reexportedInt { in v : Integer; Both::pick(v) }
			calc reexportedStr { in v : String; Both::pick(v) }
			calc inheritedInt { in v : Integer; Derived::pick(v) }
			calc inheritedStr { in v : String; Derived::pick(v) }
			calc inheritedHidesImportStr { in v : String; Importing::pick(v) }
			calc inheritedHidesImportInt { in v : Integer; Importing::pick(v) }
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
		{"Qualified::reexportedInt", intArg, 1},
		{"Qualified::reexportedStr", NewStringValue("s"), 2},
		{"Qualified::inheritedInt", intArg, 1},
		{"Qualified::inheritedStr", NewStringValue("s"), 2},
		{"Qualified::inheritedHidesImportStr", NewStringValue("s"), 2},
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
	hidden := idx.LookupQualified("Qualified::inheritedHidesImportInt")
	if len(hidden) != 1 {
		t.Fatalf("inheritedHidesImportInt: %d symbols, want 1", len(hidden))
	}
	if _, err := ctx.InvokeCalc(hidden[0], []Value{intArg}, rootScope); err == nil ||
		!strings.Contains(err.Error(), "typed by String") {
		t.Fatalf("Importing::pick(3) ran the import the inherited pick hides: err = %v", err)
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
