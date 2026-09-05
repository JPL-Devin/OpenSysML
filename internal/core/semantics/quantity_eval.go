package semantics

import (
	"errors"
	"fmt"
	"math"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

var (
	// ErrDivisionByZero reports a division or remainder with a zero divisor.
	ErrDivisionByZero = errors.New("division by zero")

	// ErrQuantityOperand reports an operator applied to operands it is not
	// defined on, such as a quantity exponent or `not` of a quantity.
	ErrQuantityOperand = errors.New("operator is not defined on these operands")
)

// EvalQuantity folds a constant expression over quantities `magnitude [unit]`.
// A result whose unit cancels, or a comparison, is a bare value in UnitOne.
func (m *Model) EvalQuantity(scope *symbols.Scope, n ast.Node) (Quantity, bool) {
	switch e := n.(type) {
	case *ast.IndexExpr:
		if !e.Bracket {
			return Quantity{}, false
		}
		return m.quantityTerm(scope, e)
	case *ast.OperatorExpr:
		return m.foldQuantityOperator(scope, e)
	}
	v, ok := evalConst(n)
	if !ok {
		return Quantity{}, false
	}
	return Quantity{Num: v, Unit: UnitOne()}, true
}

// quantityTerm folds `magnitude [unit]` when the unit resolves in the library.
func (m *Model) quantityTerm(scope *symbols.Scope, e *ast.IndexExpr) (Quantity, bool) {
	magnitude, ok := evalConst(e.Operand)
	if !ok || !magnitude.IsNumeric() {
		return Quantity{}, false
	}
	term, err := m.UnitTermOfExpr(scope, e.Index)
	if err != nil {
		return Quantity{}, false
	}
	product, err := m.UnitProductOfExpr(scope, e.Index)
	if err != nil {
		return Quantity{}, false
	}
	unit := Unit{Text: UnitExprText(e.Index), Product: product, Term: term}
	return Quantity{Num: magnitude, Unit: unit}, true
}

func (m *Model) foldQuantityOperator(scope *symbols.Scope, e *ast.OperatorExpr) (Quantity, bool) {
	switch e.Operator {
	case ast.OpNeg, ast.OpPos, ast.OpNot:
		if len(e.Operands) != 1 {
			return Quantity{}, false
		}
		operand, ok := m.EvalQuantity(scope, e.Operands[0])
		if !ok {
			return Quantity{}, false
		}
		result, err := QuantityUnary(e.Operator, operand)
		return result, err == nil
	case ast.OpConditional:
		if len(e.Operands) != 3 {
			return Quantity{}, false
		}
		cond, ok := evalConst(e.Operands[0])
		if !ok || cond.Kind != ValBool {
			return Quantity{}, false
		}
		if cond.Bool {
			return m.EvalQuantity(scope, e.Operands[1])
		}
		return m.EvalQuantity(scope, e.Operands[2])
	}
	if len(e.Operands) != 2 {
		return Quantity{}, false
	}
	left, ok := m.EvalQuantity(scope, e.Operands[0])
	if !ok {
		return Quantity{}, false
	}
	right, ok := m.EvalQuantity(scope, e.Operands[1])
	if !ok {
		return Quantity{}, false
	}
	result, err := QuantityBinary(e.Operator, left, right)
	return result, err == nil
}

// QuantityUnary applies a unary operator to a quantity, keeping its unit.
func QuantityUnary(op ast.OperatorKind, q Quantity) (Quantity, error) {
	if op == ast.OpNeg && q.Num.IsNumeric() {
		return NegateQuantity(q)
	}
	if !q.Unit.None() {
		return Quantity{}, ErrQuantityOperand
	}
	num, ok := EvalUnary(op, q.Num)
	if !ok {
		return Quantity{}, ErrQuantityOperand
	}
	return Quantity{Num: num, Unit: UnitOne()}, nil
}

// QuantityBinary applies a binary operator with the runtime's unit semantics:
// sums convert into the left unit, products compose units, comparisons convert.
func QuantityBinary(op ast.OperatorKind, left, right Quantity) (Quantity, error) {
	if left.Unit.None() && right.Unit.None() {
		if (op == ast.OpDiv || op == ast.OpMod) && right.Num.IsNumeric() && right.Num.AsReal() == 0 {
			return Quantity{}, ErrDivisionByZero
		}
		num, ok := EvalBinary(op, left.Num, right.Num)
		if !ok {
			return Quantity{}, ErrQuantityOperand
		}
		return Quantity{Num: num, Unit: UnitOne()}, nil
	}
	if !left.Num.IsNumeric() || !right.Num.IsNumeric() {
		return Quantity{}, ErrQuantityOperand
	}
	switch op {
	case ast.OpAdd, ast.OpSub:
		return AddQuantities(op, left, right)
	case ast.OpMul, ast.OpDiv:
		return ScaleQuantities(op, left, right)
	case ast.OpPow:
		if !right.Unit.None() {
			return Quantity{}, ErrQuantityOperand
		}
		return PowQuantity(left, right.Num)
	case ast.OpEq, ast.OpNeq:
		equal, err := EqualQuantities(op, left, right)
		if err != nil {
			return Quantity{}, err
		}
		return Quantity{Num: Value{Kind: ValBool, Bool: equal}, Unit: UnitOne()}, nil
	case ast.OpLt, ast.OpGt, ast.OpLe, ast.OpGe:
		holds, err := CompareQuantities(op, left, right)
		if err != nil {
			return Quantity{}, err
		}
		return Quantity{Num: Value{Kind: ValBool, Bool: holds}, Unit: UnitOne()}, nil
	}
	return Quantity{}, ErrQuantityOperand
}

// AddQuantities is a sum or difference in the left operand's unit; Integer
// magnitudes in one unit stay Integer, a conversion makes a Real.
func AddQuantities(op ast.OperatorKind, left, right Quantity) (Quantity, error) {
	converted, err := right.ConvertTo(left.Unit)
	if err != nil {
		return Quantity{}, err
	}
	rhs := Value{Kind: ValReal, Real: converted}
	if right.Num.Kind == ValInt && left.Unit.Term.Scale == right.Unit.Term.Scale {
		rhs = right.Num
	}
	num, err := MagnitudeArith(op, left.Num, rhs)
	if err != nil {
		return Quantity{}, err
	}
	return InUnit(num, left.Unit)
}

// ScaleQuantities is a product or quotient whose unit is the product or
// quotient of the operands' units: `10 [m] / 2 [s]` is `5.0 [m/s]`.
func ScaleQuantities(op ast.OperatorKind, left, right Quantity) (Quantity, error) {
	num, err := MagnitudeArith(op, left.Num, right.Num)
	if err != nil {
		return Quantity{}, err
	}
	if op == ast.OpMul {
		return ComposedQuantity(num, left.Unit.Product.Times(right.Unit.Product), left.Unit.Term.Times(right.Unit.Term))
	}
	return ComposedQuantity(num, left.Unit.Product.DividedBy(right.Unit.Product), left.Unit.Term.DividedBy(right.Unit.Term))
}

// PowQuantity raises a quantity to a constant exponent, its unit included.
func PowQuantity(base Quantity, exponent Value) (Quantity, error) {
	if !exponent.IsNumeric() {
		return Quantity{}, fmt.Errorf("%w: exponent of a quantity is not a number", ErrQuantityOperand)
	}
	num, err := Pow(base.Num, exponent)
	if err != nil {
		return Quantity{}, err
	}
	return ComposedQuantity(num, base.Unit.Product.Pow(exponent.AsReal()), base.Unit.Term.Pow(exponent.AsReal()))
}

// NegateQuantity negates a magnitude, keeping its unit and kind.
func NegateQuantity(q Quantity) (Quantity, error) {
	if q.Num.Kind == ValInt && q.Num.Int == math.MinInt64 {
		return Quantity{}, fmt.Errorf("%w: -(%d) exceeds the Integer range", ErrArithmeticOverflow, q.Num.Int)
	}
	num, ok := EvalUnary(ast.OpNeg, q.Num)
	if !ok {
		return Quantity{}, ErrQuantityOperand
	}
	return InUnit(num, q.Unit)
}

// CompareQuantities orders two quantities, converting the right one into the
// left one's unit.
func CompareQuantities(op ast.OperatorKind, left, right Quantity) (bool, error) {
	converted, err := right.ConvertTo(left.Unit)
	if err != nil {
		return false, err
	}
	lhs := left.Num.AsReal()
	switch op {
	case ast.OpLt:
		return lhs < converted, nil
	case ast.OpLe:
		return lhs <= converted, nil
	case ast.OpGt:
		return lhs > converted, nil
	case ast.OpGe:
		return lhs >= converted, nil
	default:
		return false, fmt.Errorf("unknown comparison operator: %v", op)
	}
}

// EqualQuantities compares two quantities in the left one's unit; incommensurable
// units are an error, since neither `==` nor `!=` is an answer about them.
func EqualQuantities(op ast.OperatorKind, left, right Quantity) (bool, error) {
	converted, err := right.ConvertTo(left.Unit)
	if err != nil {
		return false, err
	}
	equal := left.Num.AsReal() == converted
	if op == ast.OpNeq {
		equal = !equal
	}
	return equal, nil
}

// ComposedQuantity is a result in the canonical form of its composed unit; a
// unit that cancels leaves a number unless it names a dimension-one unit (`rad`).
func ComposedQuantity(num Value, product UnitProduct, term UnitTerm) (Quantity, error) {
	if term.Dimensionless() && !product.NamesDimensionOne() {
		return dimensionlessQuantity(num, term)
	}
	return Quantity{Num: num, Unit: Unit{Text: product.String(), Product: product, Term: term}}, nil
}

// InUnit is a magnitude in an operand's unit; no unit leaves a bare number.
func InUnit(num Value, unit Unit) (Quantity, error) {
	if unit.None() {
		return dimensionlessQuantity(num, unit.Term)
	}
	return Quantity{Num: num, Unit: unit}, nil
}

// dimensionlessQuantity is the bare number a ratio of like quantities computes,
// keeping its kind unless a scale factor is left to apply.
func dimensionlessQuantity(num Value, term UnitTerm) (Quantity, error) {
	if term.Scale != UnitScale(1) {
		var err error
		if num, err = RealResult(ConvertMagnitude(num.AsReal(), term.Scale, UnitScale(1))); err != nil {
			return Quantity{}, err
		}
	}
	return Quantity{Num: num, Unit: UnitOne()}, nil
}

// MagnitudeArith combines two magnitudes as the bare operator does: Integer
// operands keep an Integer result except under `/`.
func MagnitudeArith(op ast.OperatorKind, left, right Value) (Value, error) {
	if left.Kind == ValInt && right.Kind == ValInt {
		if op == ast.OpDiv {
			q, ok := IntQuotient(left.Int, right.Int)
			if !ok {
				return Value{}, ErrDivisionByZero
			}
			return Value{Kind: ValReal, Real: q}, nil
		}
		res, ok := IntArith(op, left.Int, right.Int)
		if !ok {
			return Value{}, IntegerOverflow(op, left.Int, right.Int)
		}
		return Value{Kind: ValInt, Int: res}, nil
	}
	res, ok := RealArith(op, left.AsReal(), right.AsReal())
	if !ok {
		return Value{}, ErrDivisionByZero
	}
	return RealResult(res)
}

// IntegerOverflow reports an Integer operation whose result leaves the range.
func IntegerOverflow(op ast.OperatorKind, left, right int64) error {
	return fmt.Errorf("%w: %d %s %d exceeds the Integer range", ErrArithmeticOverflow, left, op.String(), right)
}

// RealResult wraps a computed Real, reporting a NaN or an infinity instead of
// carrying it.
func RealResult(x float64) (Value, error) {
	switch {
	case math.IsNaN(x):
		return Value{}, fmt.Errorf("%w: argument outside the function's domain", ErrArithmeticDomain)
	case math.IsInf(x, 0):
		return Value{}, fmt.Errorf("%w: result is not a finite Real", ErrArithmeticOverflow)
	}
	return Value{Kind: ValReal, Real: x}, nil
}
