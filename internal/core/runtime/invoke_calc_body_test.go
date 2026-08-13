package runtime

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// inheritedBodyModel declares the same attribute name in two packages: the
// calculation stating the body reads the one its own package declares, and the
// calculation inheriting the body must read that same one.
const inheritedBodyModel = `
package lib {
	attribute factor = 10;
	calc def Scale {
		in n;
		attribute acc = 0;
		attribute i = 0;
		while i < n {
			acc = acc + factor;
			i = i + 1;
		}
		return : Integer = acc;
	}
}
package app {
	attribute factor = 100;
	calc def Scale :> lib::Scale {
		in n;
	}
}
`

// calcByName returns the calc named name declared by the package named pkg.
func calcByName(t *testing.T, root *symbols.Scope, pkg, name string) (*symbols.Symbol, *symbols.Scope) {
	t.Helper()
	for _, child := range root.Children() {
		if child.Owner() == nil || child.Owner().Name != pkg {
			continue
		}
		sym, ok := child.LookupLocal(name)
		if !ok || sym == nil {
			t.Fatalf("calc %s::%s not found", pkg, name)
		}
		return sym, child
	}
	t.Fatalf("package %s not found", pkg)
	return nil, nil
}

// TestInheritedCalcBodyRunsInDeclaringScope requires a body inherited through
// specialization to evaluate in the scope of the calculation declaring it, so a
// name the specializing package redeclares does not change the computation.
func TestInheritedCalcBodyRunsInDeclaringScope(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, inheritedBodyModel))
	root := idx.DocumentRoot("<test>")

	stated, statedScope := calcByName(t, root, "lib", "Scale")
	inherited, inheritedScope := calcByName(t, root, "app", "Scale")

	statedValue, err := ctx.InvokeCalc(stated, []Value{constInt(3)}, statedScope)
	if err != nil {
		t.Fatalf("lib::Scale: %v", err)
	}
	if statedValue.Const.Int != 30 {
		t.Fatalf("lib::Scale(3) = %s, want 30", FormatTraceValue(statedValue))
	}

	inheritedValue, err := ctx.InvokeCalc(inherited, []Value{constInt(3)}, inheritedScope)
	if err != nil {
		t.Fatalf("app::Scale: %v", err)
	}
	if inheritedValue.Const.Int != 30 {
		t.Fatalf("app::Scale(3) = %s, want 30 (the factor lib declares)", FormatTraceValue(inheritedValue))
	}
}

// statementBodyModel is a calc with a statement body, invoked by every entry
// point the runtime offers.
const statementBodyModel = `
package test {
	calc def factorial {
		in n;
		attribute acc = 1;
		attribute i = 1;
		while i <= n {
			acc = acc * i;
			i = i + 1;
		}
		return : Integer = acc;
	}
	calc def twice {
		in n;
		return factorial(n) + factorial(n);
	}
}
`

// TestCalcStatementBodyFromEveryEntryPoint requires the statement body to run
// the same way whether it is invoked positionally, by name, or from another
// calculation's expression.
func TestCalcStatementBodyFromEveryEntryPoint(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, statementBodyModel))
	root := idx.DocumentRoot("<test>")
	factorial, scope := calcByName(t, root, "test", "factorial")
	twice, _ := calcByName(t, root, "test", "twice")

	positional, err := ctx.InvokeCalc(factorial, []Value{constInt(6)}, scope)
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if positional.Const.Int != 720 {
		t.Fatalf("InvokeCalc(6) = %s, want 720", FormatTraceValue(positional))
	}

	named, err := ctx.InvokeCalcNamed(factorial, map[string]Value{"n": constInt(6)}, scope)
	if err != nil {
		t.Fatalf("InvokeCalcNamed: %v", err)
	}
	if named.Const.Int != 720 {
		t.Fatalf("InvokeCalcNamed(n = 6) = %s, want 720", FormatTraceValue(named))
	}

	nested, err := ctx.InvokeCalc(twice, []Value{constInt(6)}, scope)
	if err != nil {
		t.Fatalf("nested invocation: %v", err)
	}
	if nested.Const.Int != 1440 {
		t.Fatalf("twice(6) = %s, want 1440", FormatTraceValue(nested))
	}
}

// TestCalcStatementBodySpendsTheStepBudget requires a calc invocation to begin a
// run of its own, so a loop spending steps is bounded by the budget rather than
// by whatever an earlier run left.
func TestCalcStatementBodySpendsTheStepBudget(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, statementBodyModel))
	root := idx.DocumentRoot("<test>")
	factorial, scope := calcByName(t, root, "test", "factorial")

	if _, err := ctx.InvokeCalc(factorial, []Value{constInt(8)}, scope); err != nil {
		t.Fatalf("first run: %v", err)
	}
	first := ctx.steps
	if first == 0 {
		t.Fatalf("a looping calc spent no step of the budget")
	}
	for run := 1; run < 3; run++ {
		if _, err := ctx.InvokeCalc(factorial, []Value{constInt(8)}, scope); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		if ctx.steps != first {
			t.Fatalf("run %d spent %d steps, want %d: each run begins its own budget", run, ctx.steps, first)
		}
	}
}

// constSequence returns a sequence value of the given integers, the argument a
// `for` loop over a parameter iterates.
func constSequence(values ...int64) Value {
	sequence := NewSequence()
	for _, v := range values {
		sequence.Append(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: v}})
	}
	return Value{Kind: ValSequence, Sequence: sequence}
}

// TestCalcForLoopOverParameterSequence requires a `for` loop to iterate the
// sequence a parameter carries, in the order the sequence states.
func TestCalcForLoopOverParameterSequence(t *testing.T) {
	const src = `
package test {
	calc def total {
		in xs;
		attribute sum = 0;
		for x in xs {
			sum = sum + x;
		}
		return : Integer = sum;
	}
}
`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	root := idx.DocumentRoot("<test>")
	total, scope := calcByName(t, root, "test", "total")

	value, err := ctx.InvokeCalc(total, []Value{constSequence(1, 2, 3, 4)}, scope)
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if value.Const.Int != 10 {
		t.Fatalf("total((1, 2, 3, 4)) = %s, want 10", FormatTraceValue(value))
	}
}

// TestCalcStatementBodyIsLoweredOnce requires the shape cache to hold the
// lowered body, so an invocation does not lower the calculation again.
func TestCalcStatementBodyIsLoweredOnce(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, statementBodyModel))
	root := idx.DocumentRoot("<test>")
	factorial, _ := calcByName(t, root, "test", "factorial")

	first, err := ctx.calcShapeOf(factorial)
	if err != nil {
		t.Fatalf("calcShapeOf: %v", err)
	}
	second, err := ctx.calcShapeOf(factorial)
	if err != nil {
		t.Fatalf("calcShapeOf again: %v", err)
	}
	if first != second {
		t.Fatalf("calcShapeOf returned a new shape, want the cached one")
	}
	if len(first.Body) == 0 {
		t.Fatalf("shape of %s carries no body", first.Name)
	}
	if first.BodyOwner == nil || first.BodyOwner.Decl == nil {
		t.Fatalf("shape of %s names no body owner", first.Name)
	}
	if _, ok := first.BodyOwner.Decl.(*ast.Definition); !ok {
		t.Fatalf("body owner of %s is %T, want the calc definition", first.Name, first.BodyOwner.Decl)
	}
}

// resultPlacementModel declares the result of each calculation before the steps
// computing it, and states one result as a bare expression followed by a note.
const resultPlacementModel = `
package test {
	calc def factorial {
		in n;
		return : Integer = acc;
		attribute acc = 1;
		attribute i = 1;
		while i <= n {
			acc = acc * i;
			i = i + 1;
		}
	}
	calc def twice {
		in n;
		n * 2
		doc /* the result is the expression above */
	}
}
`

// TestCalcResultIsAnsweredAfterTheSteps requires the result a body declares to
// be evaluated once the steps have run, wherever among the members it is
// written: a result parameter names the answer, it does not stop the body.
func TestCalcResultIsAnsweredAfterTheSteps(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, resultPlacementModel))
	root := idx.DocumentRoot("<test>")

	factorial, scope := calcByName(t, root, "test", "factorial")
	value, err := ctx.InvokeCalc(factorial, []Value{constInt(6)}, scope)
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if value.Const.Int != 720 {
		t.Fatalf("factorial(6) = %s, want 720: the loop must run before the result is read", FormatTraceValue(value))
	}

	twice, twiceScope := calcByName(t, root, "test", "twice")
	doubled, err := ctx.InvokeCalc(twice, []Value{constInt(4)}, twiceScope)
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if doubled.Const.Int != 8 {
		t.Fatalf("twice(4) = %s, want 8: an expression result is not dropped by what follows it", FormatTraceValue(doubled))
	}
}

// successionModel writes a succession among the members of a calculation body,
// the form `then a b;` a member-attached `then` is written back as.
const successionModel = `
package test {
	calc def sequenced {
		in n;
		action first;
		action second;
		then first second;
		return : Integer = n + 1;
	}
}
`

// TestCalcBodySuccessionIsNotAStep requires a succession among a calculation's
// members to leave the computation alone: the body states its order already.
func TestCalcBodySuccessionIsNotAStep(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, successionModel))
	root := idx.DocumentRoot("<test>")

	sequenced, scope := calcByName(t, root, "test", "sequenced")
	value, err := ctx.InvokeCalc(sequenced, []Value{constInt(3)}, scope)
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if value.Const.Int != 4 {
		t.Fatalf("sequenced(3) = %s, want 4", FormatTraceValue(value))
	}
}
