package runtime

import (
	"errors"

	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

var builtins = map[string]func(*EvalContext, []Value) (Value, error){
	"SequenceFunctions::size":      builtinSequenceSize,
	"SequenceFunctions::isEmpty":   builtinSequenceIsEmpty,
	"SequenceFunctions::includes":  builtinSequenceIncludes,
	"CollectionFunctions::size":    builtinCollectionSize,
	"CollectionFunctions::isEmpty": builtinCollectionIsEmpty,
	"ControlFunctions::select":     builtinControlSelect,
	"ControlFunctions::collect":    builtinControlCollect,
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
	// Stub: check if seq1 includes seq2 (all elements of seq2 in seq1)
	return Value{}, errors.New("SequenceFunctions::includes: not yet implemented")
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
	return Value{}, errors.New("ControlFunctions::select: not yet implemented")
}

func builtinControlCollect(ec *EvalContext, args []Value) (Value, error) {
	return Value{}, errors.New("ControlFunctions::collect: not yet implemented")
}
