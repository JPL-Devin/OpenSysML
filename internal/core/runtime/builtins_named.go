package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// builtinDeferredParams lists, per built-in, the positions of the parameters
// the library declares `expr`: their arguments bind unevaluated, so the function
// decides whether and when each is evaluated, as `if` and `and` require.
var builtinDeferredParams = map[string]map[int]bool{
	"ControlFunctions::if":      {1: true, 2: true},
	"ControlFunctions::??":      {1: true},
	"ControlFunctions::and":     {1: true},
	"ControlFunctions::or":      {1: true},
	"ControlFunctions::implies": {1: true},
}

// registerNamedOperatorBuiltins adds the function-call forms of the operators
// whose results are collections or whose arguments are expressions, and the
// aggregations that take an explicit identity element.
func registerNamedOperatorBuiltins() {
	// BaseFunctions declares '#' and ',' over any sequence; SequenceFunctions'
	// index and union are the same operations.
	builtins["BaseFunctions::#"] = builtinSequenceIndex
	builtins["BaseFunctions::,"] = builtinSequenceConcat
	// CollectionFunctions::'==' is `col1.elements->equals(col2.elements)`.
	builtins["CollectionFunctions::=="] = builtinSequenceEquals

	// The range, declared abstractly over DataValue and ScalarValue and
	// concretely over Integer; every level yields the integer sequence.
	builtins["DataFunctions::.."] = rangeBuiltin("DataFunctions::'..'")
	builtins["ScalarFunctions::.."] = rangeBuiltin("ScalarFunctions::'..'")

	builtins["ControlFunctions::if"] = builtinControlIf
	builtins["ControlFunctions::??"] = builtinControlNullCoalesce
	builtins["ControlFunctions::and"] = builtinControlLogical(ast.OpConditionalAnd)
	builtins["ControlFunctions::or"] = builtinControlLogical(ast.OpConditionalOr)
	builtins["ControlFunctions::implies"] = builtinControlLogical(ast.OpImplies)

	builtins["NumericalFunctions::sum0"] = builtinNumericalSum0
	builtins["NumericalFunctions::product1"] = builtinNumericalProduct1
}

// evalDeferred evaluates an argument bound to an `expr` parameter: the
// expression as it was written, or the result of a parameterless body.
func (ec *EvalContext) evalDeferred(op string, val Value) (Value, error) {
	if val.Kind != ValExpr {
		return val, nil
	}
	if body, ok := val.Expr().(*ast.BodyExpr); ok {
		if len(body.Params) != 0 {
			return Value{}, fmt.Errorf("%w: %s evaluates its body with no argument, but it declares %d parameter(s)",
				ErrBodyArity, op, len(body.Params))
		}
		return ec.applyBody(body)
	}
	return ec.Eval(val.Expr())
}

// builtinControlIf is ControlFunctions::'if'(test, thenValue, elseValue): the
// selected branch alone is evaluated, and an omitted else branch is null.
func builtinControlIf(ec *EvalContext, args []Value) (Value, error) {
	const op = "ControlFunctions::'if'"
	if len(args) != 2 && len(args) != 3 {
		return Value{}, fmt.Errorf("%w: %s takes 2..3 argument(s), got %d", ErrCalcArity, op, len(args))
	}
	held, err := boolOperand("test of "+op, args[0])
	if err != nil {
		return Value{}, err
	}
	if held {
		return ec.evalDeferred(op, args[1])
	}
	if len(args) == 2 {
		return nullValue(), nil
	}
	return ec.evalDeferred(op, args[2])
}

// builtinControlNullCoalesce is ControlFunctions::'??'(firstValue, secondValue),
// which evaluates secondValue only when firstValue is null.
func builtinControlNullCoalesce(ec *EvalContext, args []Value) (Value, error) {
	const op = "ControlFunctions::'??'"
	if err := checkArity(op, args, 2); err != nil {
		return Value{}, err
	}
	return coalesceNull(args[0], func() (Value, error) { return ec.evalDeferred(op, args[1]) })
}

// builtinControlLogical is ControlFunctions::'and', 'or' or 'implies': the
// first value decides alone where it can, and secondValue is evaluated only
// where it cannot.
func builtinControlLogical(op ast.OperatorKind) func(*EvalContext, []Value) (Value, error) {
	name := fmt.Sprintf("ControlFunctions::'%s'", op)
	return func(ec *EvalContext, args []Value) (Value, error) {
		if err := checkArity(name, args, 2); err != nil {
			return Value{}, err
		}
		l, err := boolOperand("firstValue of "+name, args[0])
		if err != nil {
			return Value{}, err
		}
		if decided, result := shortCircuit(op, l); decided {
			return boolValue(result), nil
		}
		second, err := ec.evalDeferred(name, args[1])
		if err != nil {
			return Value{}, err
		}
		r, err := boolOperand("secondValue of "+name, second)
		if err != nil {
			return Value{}, err
		}
		return combineBooleans(op, l, r)
	}
}

// builtinSequenceConcat is BaseFunctions::',', the sequence of seq1's elements
// followed by seq2's.
func builtinSequenceConcat(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("BaseFunctions::','", args, 2); err != nil {
		return Value{}, err
	}
	return ec.concatSequences(args[0], args[1])
}

// builtinNumericalSum0 is NumericalFunctions::sum0(collection, zero):
// `collection->reduce '+' ?? zero`, so an empty collection is the zero given.
func builtinNumericalSum0(ec *EvalContext, args []Value) (Value, error) {
	return aggregateWithIdentity("NumericalFunctions::sum0", "zero", args, ast.OpAdd)
}

// builtinNumericalProduct1 is NumericalFunctions::product1(collection, one):
// `collection->reduce '*' ?? one`.
func builtinNumericalProduct1(ec *EvalContext, args []Value) (Value, error) {
	return aggregateWithIdentity("NumericalFunctions::product1", "one", args, ast.OpMul)
}

// aggregateWithIdentity folds a collection under op from its first element, and
// answers the identity given for an empty one. The library asserts the identity
// is one (`inv { isZero(zero) }`), so another value is reported.
func aggregateWithIdentity(op, param string, args []Value, operator ast.OperatorKind) (Value, error) {
	if err := checkArity(op, args, 2); err != nil {
		return Value{}, err
	}
	identity := args[1]
	if !isIdentityElement(identity, operator) {
		predicate := "isZero"
		if operator == ast.OpMul {
			predicate = "isUnit"
		}
		return Value{}, fmt.Errorf("%w: %s requires %s(%s), got %s",
			ErrTypeMismatch, op, predicate, param, FormatValue(identity))
	}
	if len(elementsOf(args[0])) == 0 {
		return identity, nil
	}
	return aggregate(op, args[:1], operator)
}

// isIdentityElement reports whether val is the additive (0) or multiplicative
// (1) identity of the numbers this runtime aggregates.
func isIdentityElement(val Value, operator ast.OperatorKind) bool {
	want := 0.0
	if operator == ast.OpMul {
		want = 1
	}
	switch val.Kind {
	case ValConst:
		return val.Const.IsNumeric() && toReal(val.Const) == want
	case ValComplex:
		return val.Complex() == complex(want, 0)
	case ValQuantity:
		return toReal(val.Quantity().Num) == want
	}
	return false
}
