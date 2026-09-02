package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// operandDomain checks that an argument conforms to the parameter type one
// function library package declares for its operators.
type operandDomain func(name, param string, val Value) error

// operatorForms lists the operator functions each package declares over `x`
// and `y`, with the domain its parameter types impose. `..` is a builtin.
var operatorForms = []struct {
	pkg    string
	ops    []string
	domain operandDomain
}{
	{"DataFunctions", []string{"+", "-", "*", "/", "%", "**", "^", "<", "<=", ">", ">=", "not", "xor", "|", "&"}, anyOperand},
	{"ScalarFunctions", []string{"+", "-", "*", "/", "%", "**", "^", "<", "<=", ">", ">=", "not", "xor", "|", "&"}, anyOperand},
	{"NumericalFunctions", []string{"+", "-", "*", "/", "%", "**", "^", "<", "<=", ">", ">="}, numericalOperand},
	{"RealFunctions", []string{"+", "-", "*", "/", "**", "^", "<", "<=", ">", ">="}, realOperand},
	{"RationalFunctions", []string{"+", "-", "*", "/", "**", "^", "<", "<=", ">", ">="}, realOperand},
	{"IntegerFunctions", []string{"+", "-", "*", "/", "%", "<", "<=", ">", ">="}, integerOperand},
	{"IntegerFunctions", []string{"**", "^"}, integerBaseNaturalExponent},
	{"NaturalFunctions", []string{"+", "*", "/", "%", "<", "<=", ">", ">="}, naturalOperand},
	{"BooleanFunctions", []string{"not", "xor", "|", "&"}, booleanOperand},
}

// operatorKinds maps each operator function's name to the operator it applies.
var operatorKinds = map[string]ast.OperatorKind{
	"+": ast.OpAdd, "-": ast.OpSub, "*": ast.OpMul, "/": ast.OpDiv, "%": ast.OpMod,
	"**": ast.OpPow, "^": ast.OpPow,
	"<": ast.OpLt, "<=": ast.OpLe, ">": ast.OpGt, ">=": ast.OpGe,
	"not": ast.OpNot, "xor": ast.OpXor, "|": ast.OpOr, "&": ast.OpAnd,
}

// equalityForms are the `'=='` declarations, each over two [0..1] operands.
var equalityForms = []string{
	"BaseFunctions::==", "DataFunctions::==", "BooleanFunctions::==",
	"IntegerFunctions::==", "NaturalFunctions::==", "RationalFunctions::==", "RealFunctions::==",
}

// registerOperatorFunctions registers the explicit function form of every
// operator the library declares, each delegating to the operator's evaluator.
func registerOperatorFunctions() {
	for _, form := range operatorForms {
		for _, op := range form.ops {
			registerOperatorForm(form.pkg+"::"+op, op, form.domain)
		}
	}
	for _, fqn := range equalityForms {
		registerValueFunction(fqn, []string{"x", "y"}, 0, equalityForm(ast.OpEq))
	}
	registerValueFunction("BaseFunctions::!=", []string{"x", "y"}, 0, equalityForm(ast.OpNeq))
	registerValueFunction("BaseFunctions::===", []string{"x", "y"}, 0, identityForm(false))
	registerValueFunction("DataFunctions::===", []string{"x", "y"}, 0, identityForm(false))
	registerValueFunction("BaseFunctions::!==", []string{"x", "y"}, 0, identityForm(true))

	// Every declaration of these names applies the same Boolean operation.
	registerLocalNames(map[string]string{
		"not": "DataFunctions::not",
		"xor": "DataFunctions::xor",
	})
}

// registerOperatorForm registers one operator function. `+` and `-` declare
// `y` [0..1], so given one argument they are the unary operators.
func registerOperatorForm(fqn, op string, domain operandDomain) {
	kind := operatorKinds[op]
	switch op {
	case "not":
		registerValueFunction(fqn, []string{"x"}, 1, unaryForm(kind, domain))
	case "+", "-":
		registerValueFunction(fqn, []string{"x", "y"}, 1, arithmeticForm(kind, domain))
	case "*", "/", "%", "**", "^":
		registerValueFunction(fqn, []string{"x", "y"}, 2, arithmeticForm(kind, domain))
	case "<", "<=", ">", ">=":
		registerValueFunction(fqn, []string{"x", "y"}, 2, comparisonForm(kind, domain))
	case "xor", "|", "&":
		registerValueFunction(fqn, []string{"x", "y"}, 2, booleanForm(kind, domain))
	}
}

// checkOperands applies the package's domain to each argument given.
func checkOperands(name string, domain operandDomain, args []Value) error {
	for i, param := range []string{"x", "y"}[:len(args)] {
		if err := domain(name, param, args[i]); err != nil {
			return err
		}
	}
	return nil
}

// arithmeticForm is `'+'`, `'-'`, `'*'`, `'/'`, `'%'`, `'**'` and `'^'` as
// functions; `'+'(x)` and `'-'(x)` are the unary sign operators.
func arithmeticForm(op ast.OperatorKind, domain operandDomain) libraryApply {
	return func(name string, _ *Context, args []Value) (Value, error) {
		if len(args) == 2 && argumentOmitted(args[1]) {
			args = args[:1]
		}
		if err := checkOperands(name, domain, args); err != nil {
			return Value{}, err
		}
		if len(args) == 1 {
			unary := ast.OpPos
			if op == ast.OpSub {
				unary = ast.OpNeg
			}
			val, err := unaryValue(unary, args[0])
			return operatorResult(name, val, err)
		}
		val, err := arithmeticValues(op, args[0], args[1], source.Span{})
		return operatorResult(name, val, err)
	}
}

// comparisonForm is `'<'`, `'<='`, `'>'` and `'>='` as functions.
func comparisonForm(op ast.OperatorKind, domain operandDomain) libraryApply {
	return func(name string, _ *Context, args []Value) (Value, error) {
		if err := checkOperands(name, domain, args); err != nil {
			return Value{}, err
		}
		val, err := comparisonValues(op, args[0], args[1], source.Span{})
		return operatorResult(name, val, err)
	}
}

// booleanForm is `'xor'`, `'|'` and `'&'` as functions; both operands are
// given, so nothing is short-circuited.
func booleanForm(op ast.OperatorKind, domain operandDomain) libraryApply {
	return func(name string, _ *Context, args []Value) (Value, error) {
		if err := checkOperands(name, domain, args); err != nil {
			return Value{}, err
		}
		l, err := boolOperand(fmt.Sprintf("function %s parameter %q", name, "x"), args[0])
		if err != nil {
			return Value{}, err
		}
		r, err := boolOperand(fmt.Sprintf("function %s parameter %q", name, "y"), args[1])
		if err != nil {
			return Value{}, err
		}
		return combineBooleans(op, l, r)
	}
}

// unaryForm is `'not'` as a function.
func unaryForm(op ast.OperatorKind, domain operandDomain) libraryApply {
	return func(name string, _ *Context, args []Value) (Value, error) {
		if err := checkOperands(name, domain, args); err != nil {
			return Value{}, err
		}
		val, err := unaryValue(op, args[0])
		return operatorResult(name, val, err)
	}
}

// equalityForm is `'=='` or `'!='` as a function; an omitted operand is null,
// so two omitted operands are equal and an omitted one equals no value.
func equalityForm(op ast.OperatorKind) libraryApply {
	return func(name string, ctx *Context, args []Value) (Value, error) {
		val, err := ctx.equalityValues(op, args[0], args[1])
		return operatorResult(name, val, err)
	}
}

// identityForm is `'==='` (negated=false) or `'!=='` as a function.
func identityForm(negated bool) libraryApply {
	return func(_ string, _ *Context, args []Value) (Value, error) {
		return boolValue(valueIdentical(args[0], args[1]) != negated), nil
	}
}

// registerGenericExtrema registers the `max` and `min` DataFunctions and
// ScalarFunctions declare over every ordered value.
func registerGenericExtrema() {
	for _, pkg := range []string{"DataFunctions", "ScalarFunctions"} {
		registerValueFunction(pkg+"::max", []string{"x", "y"}, 2, genericExtremum(true))
		registerValueFunction(pkg+"::min", []string{"x", "y"}, 2, genericExtremum(false))
	}
}

// genericExtremum picks the larger (or smaller) of two values the way the
// ordering operators compare them: numbers keep their kind as the numeric
// `max`/`min` do, and strings and quantities answer with the operand chosen.
// A kind the library declares no ordering for is refused.
func genericExtremum(larger bool) libraryApply {
	extremum := numericScalars([]string{"x", "y"}, numericExtremum(larger))
	return func(name string, ctx *Context, args []Value) (Value, error) {
		x, y := args[0], args[1]
		switch {
		case x.Kind == ValConst && x.Const.IsNumeric() && y.Kind == ValConst && y.Const.IsNumeric():
			return extremum(name, ctx, args)
		case x.Kind == ValString && y.Kind == ValString, x.Kind == ValQuantity && y.Kind == ValQuantity:
			less, err := comparisonValues(ast.OpLt, x, y, source.Span{})
			if err != nil {
				return Value{}, fmt.Errorf("function %s: %w", name, err)
			}
			if less.Const.Bool == larger {
				return y, nil
			}
			return x, nil
		}
		return Value{}, fmt.Errorf(
			"%w: function %s orders numbers, strings and quantities, not %s and %s",
			ErrTypeMismatch, name, describeOperand(x), describeOperand(y),
		)
	}
}

// operatorResult names the function an operator error was raised in.
func operatorResult(name string, val Value, err error) (Value, error) {
	if err != nil {
		return Value{}, fmt.Errorf("function %s: %w", name, err)
	}
	return val, nil
}

// anyOperand is the domain of DataFunctions and ScalarFunctions, whose
// operators the evaluator defines for whichever values it can.
func anyOperand(string, string, Value) error { return nil }

// numericalOperand admits the NumericalValues: Integers, Reals and Complexes.
func numericalOperand(name, param string, val Value) error {
	if val.Kind == ValComplex || (val.Kind == ValConst && val.Const.IsNumeric()) {
		return nil
	}
	return operandMismatch(name, param, "a numeric", val)
}

// realOperand admits an Integer or a Real, which both conform to Real.
func realOperand(name, param string, val Value) error {
	if val.Kind == ValConst && val.Const.IsNumeric() {
		return nil
	}
	return operandMismatch(name, param, "a Real", val)
}

// integerOperand admits an Integer only: a Real does not conform to Integer.
func integerOperand(name, param string, val Value) error {
	if val.Kind == ValConst && val.Const.Kind == semantics.ValInt {
		return nil
	}
	return operandMismatch(name, param, "an Integer", val)
}

// naturalOperand admits a non-negative Integer.
func naturalOperand(name, param string, val Value) error {
	if err := integerOperand(name, param, val); err != nil {
		return err
	}
	if val.Const.Int < 0 {
		return fmt.Errorf("%w: function %s parameter %q requires a Natural value, got %d", ErrTypeMismatch, name, param, val.Const.Int)
	}
	return nil
}

// integerBaseNaturalExponent is IntegerFunctions::'**' and '^': `x` an Integer
// and `y` a Natural, so the result is an Integer.
func integerBaseNaturalExponent(name, param string, val Value) error {
	if param == "y" {
		return naturalOperand(name, param, val)
	}
	return integerOperand(name, param, val)
}

// booleanOperand admits a Boolean.
func booleanOperand(name, param string, val Value) error {
	if val.Kind == ValConst && val.Const.Kind == semantics.ValBool {
		return nil
	}
	return operandMismatch(name, param, "a Boolean", val)
}

func operandMismatch(name, param, want string, val Value) error {
	return fmt.Errorf("%w: function %s parameter %q requires %s value, got %s", ErrTypeMismatch, name, param, want, describeOperand(val))
}
