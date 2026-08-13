package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

// A range is not a value kind of its own: the Kernel Function Library declares
// `..` as a function whose result is a collection — abstractly over DataValue
// (DataFunctions::'..', `return : DataValue[0..*] ordered`) and concretely over
// the integers (IntegerFunctions::'..', `return : Integer[0..*]`). So `1..5` is
// the ordered sequence (1, 2, 3, 4, 5), which is what the library's own use of
// it needs: SequenceFunctions::subsequence is
// `(startIndex..endIndex)->collect {in i; seq#(i)}`, and its `tail(seq)` case —
// `subsequence(seq, 2)` of a one-element sequence — reaches `2..1`, so a range
// whose lower bound exceeds its upper one is the empty sequence the library's
// `[0..*]` result allows rather than an error.

// evalRange evaluates `lower..upper` (IntegerFunctions::'..').
func (ec *EvalContext) evalRange(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 2 {
		return Value{}, fmt.Errorf("%w: '..' requires 2 operands, got %d", ErrCalcArity, len(n.Operands))
	}
	lower, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	upper, err := ec.Eval(n.Operands[1])
	if err != nil {
		return Value{}, err
	}
	return ec.rangeSequence("'..'", lower, upper)
}

// rangeSequence builds the ordered sequence of integers from lower to upper.
// Every element counts one step of the budget, so a range wider than the budget
// reports ErrStepLimitExceeded rather than exhausting memory.
func (ec *EvalContext) rangeSequence(op string, lowerVal, upperVal Value) (Value, error) {
	lower, err := rangeBound(op, "lower", lowerVal)
	if err != nil {
		return Value{}, err
	}
	upper, err := rangeBound(op, "upper", upperVal)
	if err != nil {
		return Value{}, err
	}
	// A descending range names no integer: the library's own subsequence reaches
	// one and expects nothing from it.
	if lower > upper {
		return sequenceOf(nil), nil
	}
	// A model-supplied width can overflow or exceed what is allocatable, so it only
	// hints at the capacity; the step budget below bounds the sequence.
	const maxHint = 4096
	hint := upper - lower + 1
	if hint <= 0 || hint > maxHint {
		hint = maxHint
	}
	elements := make([]Value, 0, hint)
	for i := lower; ; i++ {
		if err := ec.ctx.incrementStep(); err != nil {
			return Value{}, err
		}
		elements = append(elements, integerValue(i))
		if i == upper {
			break
		}
	}
	return sequenceOf(elements), nil
}

// rangeBound reads one bound of a range: IntegerFunctions::'..' declares both
// `Integer[1]`, so a Real bound does not conform and is reported rather than
// truncated to the integer it is nearest.
func rangeBound(op, which string, val Value) (int64, error) {
	if val.Kind != ValConst || val.Const.Kind != semantics.ValInt {
		return 0, fmt.Errorf(
			"%w: %s requires Integer bounds (IntegerFunctions::'..' declares in %s: Integer[1]), got %s",
			ErrTypeMismatch, op, which, describeValue(val),
		)
	}
	return val.Const.Int, nil
}

// builtinIntegerRange is IntegerFunctions::'..' called as a function.
func builtinIntegerRange(ec *EvalContext, args []Value) (Value, error) {
	const op = "IntegerFunctions::'..'"
	if err := checkArity(op, args, 2); err != nil {
		return Value{}, err
	}
	return ec.rangeSequence(op, args[0], args[1])
}
