package runtime

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

var builtins map[string]func(*EvalContext, []Value) (Value, error)

func init() {
	builtins = map[string]func(*EvalContext, []Value) (Value, error){
		"SequenceFunctions::size":      builtinSequenceSize,
		"SequenceFunctions::isEmpty":   builtinSequenceIsEmpty,
		"SequenceFunctions::includes":  builtinSequenceIncludes,
		"CollectionFunctions::size":    builtinCollectionSize,
		"CollectionFunctions::isEmpty": builtinCollectionIsEmpty,
		"ControlFunctions::select":     builtinControlSelect,
		"ControlFunctions::collect":    builtinControlCollect,
	}
}

func builtinSequenceSize(ec *EvalContext, args []Value) (Value, error) {
	if len(args) != 1 {
		return Value{}, errors.New("SequenceFunctions::size: expected 1 argument")
	}

	col := args[0]
	var sz int64
	switch col.Kind {
	case ValSequence:
		sz = int64(col.Sequence.Size())
	case ValSet:
		sz = int64(col.Set.Size())
	default:
		return Value{}, errors.New("SequenceFunctions::size: expected collection")
	}

	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: sz}}, nil
}

func builtinSequenceIsEmpty(ec *EvalContext, args []Value) (Value, error) {
	if len(args) != 1 {
		return Value{}, errors.New("SequenceFunctions::isEmpty: expected 1 argument")
	}

	sizeVal, err := builtinSequenceSize(ec, args)
	if err != nil {
		return Value{}, err
	}

	isEmpty := sizeVal.Const.Int == 0
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: isEmpty}}, nil
}

func builtinSequenceIncludes(ec *EvalContext, args []Value) (Value, error) {
	if len(args) != 2 {
		return Value{}, errors.New("SequenceFunctions::includes: expected 2 arguments")
	}

	seq := args[0]
	target := args[1]

	if seq.Kind != ValSequence {
		return Value{}, errors.New("SequenceFunctions::includes: first argument must be a sequence")
	}

	// Empty sequence contains nothing
	if seq.Sequence == nil {
		return boolValue(false), nil
	}

	// Check each element
	for i := 0; i < seq.Sequence.Size(); i++ {
		elem, err := seq.Sequence.At(i)
		if err != nil {
			return Value{}, err
		}
		if valueEqual(elem, target) {
			return boolValue(true), nil
		}
	}

	return boolValue(false), nil
}

func boolValue(b bool) Value {
	return Value{
		Kind: ValConst,
		Const: semantics.Value{
			Kind: semantics.ValBool,
			Bool: b,
		},
	}
}

func builtinCollectionSize(ec *EvalContext, args []Value) (Value, error) {
	return builtinSequenceSize(ec, args)
}

func builtinCollectionIsEmpty(ec *EvalContext, args []Value) (Value, error) {
	return builtinSequenceIsEmpty(ec, args)
}

func builtinControlSelect(ec *EvalContext, args []Value) (Value, error) {
	if len(args) != 2 {
		return Value{}, errors.New("ControlFunctions::select: expected 2 arguments")
	}

	// First arg must be collection
	col := args[0]
	var elements []Value
	switch col.Kind {
	case ValSequence:
		if col.Sequence == nil {
			elements = []Value{}
		} else {
			elements = col.Sequence.Elements()
		}
	case ValSet:
		elements = col.Set.Elements()
	default:
		return Value{}, errors.New("ControlFunctions::select: first argument must be collection")
	}

	// Second arg must be ValExpr wrapping BodyExpr
	if args[1].Kind != ValExpr {
		return Value{}, errors.New("ControlFunctions::select: second argument must be body expression")
	}

	bodyExpr, ok := args[1].Expr.(*ast.BodyExpr)
	if !ok {
		return Value{}, errors.New("ControlFunctions::select: second argument must be BodyExpr")
	}

	// Expect exactly one parameter
	if len(bodyExpr.Params) != 1 {
		return Value{}, errors.New("ControlFunctions::select: body expression must have exactly one parameter")
	}

	paramName := bodyExpr.Params[0].Name

	// Filter elements
	result := NewSequence()
	for _, elem := range elements {
		// Bind parameter to element
		ec.Push(map[string]Value{paramName: elem})

		// Evaluate predicate
		predVal, err := ec.Eval(bodyExpr.Result)
		ec.Pop()

		if err != nil {
			return Value{}, err
		}

		// Check if predicate returns boolean
		if predVal.Kind != ValConst || predVal.Const.Kind != semantics.ValBool {
			return Value{}, fmt.Errorf("ControlFunctions::select: predicate must return boolean, got %v", predVal.Kind)
		}

		// Check if predicate is true
		if predVal.Const.Bool {
			result.Append(elem)
		}
	}

	return Value{Kind: ValSequence, Sequence: result}, nil
}

func builtinControlCollect(ec *EvalContext, args []Value) (Value, error) {
	if len(args) != 2 {
		return Value{}, errors.New("ControlFunctions::collect: expected 2 arguments")
	}

	// First arg must be collection
	col := args[0]
	var elements []Value
	switch col.Kind {
	case ValSequence:
		if col.Sequence == nil {
			elements = []Value{}
		} else {
			elements = col.Sequence.Elements()
		}
	case ValSet:
		elements = col.Set.Elements()
	default:
		return Value{}, errors.New("ControlFunctions::collect: first argument must be collection")
	}

	// Second arg must be ValExpr wrapping BodyExpr
	if args[1].Kind != ValExpr {
		return Value{}, errors.New("ControlFunctions::collect: second argument must be body expression")
	}

	bodyExpr, ok := args[1].Expr.(*ast.BodyExpr)
	if !ok {
		return Value{}, errors.New("ControlFunctions::collect: second argument must be BodyExpr")
	}

	// Expect exactly one parameter
	if len(bodyExpr.Params) != 1 {
		return Value{}, errors.New("ControlFunctions::collect: body expression must have exactly one parameter")
	}

	paramName := bodyExpr.Params[0].Name

	// Map elements
	result := NewSequence()
	for _, elem := range elements {
		// Bind parameter to element
		ec.Push(map[string]Value{paramName: elem})

		// Evaluate mapping expression
		mappedVal, err := ec.Eval(bodyExpr.Result)
		ec.Pop()

		if err != nil {
			return Value{}, err
		}

		result.Append(mappedVal)
	}

	return Value{Kind: ValSequence, Sequence: result}, nil
}
