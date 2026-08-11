package semantics

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

var (
	// ErrArithmeticDomain reports operands an operation is not defined for, such
	// as a negative base raised to a fractional exponent.
	ErrArithmeticDomain = errors.New("arithmetic domain error")

	// ErrArithmeticOverflow reports a result outside the range of the kind it
	// would have: an Integer that does not fit int64, or a non-finite Real.
	ErrArithmeticOverflow = errors.New("arithmetic overflow")
)

// ValueKind discriminates a model-level constant value.
type ValueKind int

const (
	ValInvalid ValueKind = iota
	ValInt
	ValReal
	ValBool
	ValInfinity // the `*` bound / unbounded value
)

// Value is a model-level-evaluated constant. Only the field selected by Kind is
// meaningful. This is a deliberately small subset: the constraint checks that
// need evaluation (multiplicity bounds, some guards) operate over integers,
// reals, booleans, and the infinity bound.
type Value struct {
	Kind ValueKind
	Int  int64
	Real float64
	Bool bool
}

// IsNumeric reports whether the value is an integer or a real.
func (v Value) IsNumeric() bool { return v.Kind == ValInt || v.Kind == ValReal }

// asReal returns the value as a float64 (int and real only).
func (v Value) asReal() float64 {
	if v.Kind == ValInt {
		return float64(v.Int)
	}
	return v.Real
}

// Eval attempts to evaluate n as a model-level constant. It returns ok=false
// for anything outside the supported subset (feature references, strings, null,
// unsupported operators, or arithmetic on infinity) — callers then skip the
// check, matching the pilot's model-level-evaluable gating.
func (m *Model) Eval(n ast.Node) (Value, bool) {
	return evalConst(n)
}

func evalConst(n ast.Node) (Value, bool) {
	switch e := n.(type) {
	case *ast.LiteralInteger:
		i, err := strconv.ParseInt(e.Value, 10, 64)
		if err != nil {
			return Value{}, false
		}
		return Value{Kind: ValInt, Int: i}, true
	case *ast.LiteralReal:
		f, err := strconv.ParseFloat(e.Value, 64)
		if err != nil {
			return Value{}, false
		}
		return Value{Kind: ValReal, Real: f}, true
	case *ast.LiteralBool:
		return Value{Kind: ValBool, Bool: e.Value}, true
	case *ast.LiteralInfinity:
		return Value{Kind: ValInfinity}, true
	case *ast.OperatorExpr:
		return evalOperator(e)
	default:
		return Value{}, false
	}
}

func evalOperator(e *ast.OperatorExpr) (Value, bool) {
	switch e.Operator {
	case ast.OpNeg, ast.OpPos, ast.OpNot:
		if len(e.Operands) != 1 {
			return Value{}, false
		}
		return evalUnary(e.Operator, e.Operands[0])
	case ast.OpConditional:
		if len(e.Operands) != 3 {
			return Value{}, false
		}
		cond, ok := evalConst(e.Operands[0])
		if !ok || cond.Kind != ValBool {
			return Value{}, false
		}
		if cond.Bool {
			return evalConst(e.Operands[1])
		}
		return evalConst(e.Operands[2])
	default:
		if len(e.Operands) != 2 {
			return Value{}, false
		}
		return evalBinary(e.Operator, e.Operands[0], e.Operands[1])
	}
}

// EvalUnary evaluates a unary operator on a constant value.
// Returns (result, true) if successful, (zero, false) otherwise.
func EvalUnary(op ast.OperatorKind, v Value) (Value, bool) {
	switch op {
	case ast.OpNeg:
		if v.Kind == ValInt {
			return Value{Kind: ValInt, Int: -v.Int}, true
		}
		if v.Kind == ValReal {
			return Value{Kind: ValReal, Real: -v.Real}, true
		}
	case ast.OpPos:
		if v.IsNumeric() {
			return v, true
		}
	case ast.OpNot:
		if v.Kind == ValBool {
			return Value{Kind: ValBool, Bool: !v.Bool}, true
		}
	}
	return Value{}, false
}

func evalUnary(op ast.OperatorKind, operand ast.Node) (Value, bool) {
	v, ok := evalConst(operand)
	if !ok {
		return Value{}, false
	}
	return EvalUnary(op, v)
}

func evalBinary(op ast.OperatorKind, lhs, rhs ast.Node) (Value, bool) {
	l, ok := evalConst(lhs)
	if !ok {
		return Value{}, false
	}
	r, ok := evalConst(rhs)
	if !ok {
		return Value{}, false
	}

	switch op {
	case ast.OpAnd, ast.OpConditionalAnd, ast.OpOr, ast.OpConditionalOr, ast.OpXor, ast.OpImplies:
		if l.Kind != ValBool || r.Kind != ValBool {
			return Value{}, false
		}
		return evalBoolOp(op, l.Bool, r.Bool), true
	case ast.OpEq, ast.OpNeq:
		return evalEquality(op, l, r)
	case ast.OpLt, ast.OpGt, ast.OpLe, ast.OpGe:
		return evalComparison(op, l, r)
	case ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpDiv, ast.OpMod, ast.OpPow:
		return evalArithmetic(op, l, r)
	}
	return Value{}, false
}

// EvalBinary evaluates a binary operator on two constant values.
// Returns (result, true) if successful, (zero, false) otherwise.
func EvalBinary(op ast.OperatorKind, l, r Value) (Value, bool) {
	switch op {
	case ast.OpAnd, ast.OpConditionalAnd, ast.OpOr, ast.OpConditionalOr, ast.OpXor, ast.OpImplies:
		if l.Kind != ValBool || r.Kind != ValBool {
			return Value{}, false
		}
		return evalBoolOp(op, l.Bool, r.Bool), true
	case ast.OpEq, ast.OpNeq:
		return evalEquality(op, l, r)
	case ast.OpLt, ast.OpGt, ast.OpLe, ast.OpGe:
		return evalComparison(op, l, r)
	case ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpDiv, ast.OpMod, ast.OpPow:
		return evalArithmetic(op, l, r)
	}
	return Value{}, false
}

func evalBoolOp(op ast.OperatorKind, a, b bool) Value {
	var res bool
	switch op {
	case ast.OpAnd, ast.OpConditionalAnd:
		res = a && b
	case ast.OpOr, ast.OpConditionalOr:
		res = a || b
	case ast.OpXor:
		res = a != b
	case ast.OpImplies:
		res = !a || b
	}
	return Value{Kind: ValBool, Bool: res}
}

func evalEquality(op ast.OperatorKind, l, r Value) (Value, bool) {
	var eq bool
	switch {
	case l.Kind == ValBool && r.Kind == ValBool:
		eq = l.Bool == r.Bool
	case l.IsNumeric() && r.IsNumeric():
		eq = l.asReal() == r.asReal()
	default:
		return Value{}, false
	}
	if op == ast.OpNeq {
		eq = !eq
	}
	return Value{Kind: ValBool, Bool: eq}, true
}

func evalComparison(op ast.OperatorKind, l, r Value) (Value, bool) {
	if !l.IsNumeric() || !r.IsNumeric() {
		return Value{}, false
	}
	lf, rf := l.asReal(), r.asReal()
	var res bool
	switch op {
	case ast.OpLt:
		res = lf < rf
	case ast.OpGt:
		res = lf > rf
	case ast.OpLe:
		res = lf <= rf
	case ast.OpGe:
		res = lf >= rf
	}
	return Value{Kind: ValBool, Bool: res}, true
}

func evalArithmetic(op ast.OperatorKind, l, r Value) (Value, bool) {
	if !l.IsNumeric() || !r.IsNumeric() {
		return Value{}, false
	}
	if op == ast.OpPow {
		v, err := Pow(l, r)
		if err != nil {
			return Value{}, false
		}
		return v, true
	}
	// Integer arithmetic when both operands are integers (except division,
	// which may be fractional — keep it real to avoid silent truncation).
	if l.Kind == ValInt && r.Kind == ValInt && op != ast.OpDiv {
		return evalIntArith(op, l.Int, r.Int)
	}
	return evalRealArith(op, l.asReal(), r.asReal())
}

func evalIntArith(op ast.OperatorKind, a, b int64) (Value, bool) {
	switch op {
	case ast.OpAdd:
		return Value{Kind: ValInt, Int: a + b}, true
	case ast.OpSub:
		return Value{Kind: ValInt, Int: a - b}, true
	case ast.OpMul:
		return Value{Kind: ValInt, Int: a * b}, true
	case ast.OpMod:
		if b == 0 {
			return Value{}, false
		}
		return Value{Kind: ValInt, Int: a % b}, true
	}
	return Value{}, false
}

func evalRealArith(op ast.OperatorKind, a, b float64) (Value, bool) {
	switch op {
	case ast.OpAdd:
		return Value{Kind: ValReal, Real: a + b}, true
	case ast.OpSub:
		return Value{Kind: ValReal, Real: a - b}, true
	case ast.OpMul:
		return Value{Kind: ValReal, Real: a * b}, true
	case ast.OpDiv:
		if b == 0 {
			return Value{}, false
		}
		return Value{Kind: ValReal, Real: a / b}, true
	}
	return Value{}, false
}

// Pow evaluates l ** r (equivalently l ^ r) — the single implementation the
// constant folder and the runtime share, so a folded and an evaluated
// exponentiation agree. Integer operands with a non-negative exponent give an
// Integer, as IntegerFunctions::'**' declares; every other numeric combination
// gives a Real, as RealFunctions::'**' does. A result that is not a finite value
// of that kind is an error rather than a NaN, an infinity, or a wrapped integer:
// the folder declines on it, the runtime reports it.
func Pow(l, r Value) (Value, error) {
	if !l.IsNumeric() || !r.IsNumeric() {
		return Value{}, fmt.Errorf("%w: ** requires numeric operands", ErrArithmeticDomain)
	}

	if l.Kind == ValInt && r.Kind == ValInt && r.Int >= 0 {
		res, ok := intPow(l.Int, r.Int)
		if !ok {
			return Value{}, fmt.Errorf("%w: %d ** %d exceeds the Integer range", ErrArithmeticOverflow, l.Int, r.Int)
		}
		return Value{Kind: ValInt, Int: res}, nil
	}

	base, exp := l.asReal(), r.asReal()
	switch {
	case base == 0 && exp < 0:
		return Value{}, fmt.Errorf("%w: 0 ** %v is undefined (negative exponent)", ErrArithmeticDomain, exp)
	case base < 0 && exp != math.Trunc(exp):
		return Value{}, fmt.Errorf("%w: %v ** %v is not a Real (negative base, fractional exponent)", ErrArithmeticDomain, base, exp)
	}
	res := math.Pow(base, exp)
	if math.IsNaN(res) || math.IsInf(res, 0) {
		return Value{}, fmt.Errorf("%w: %v ** %v is not a finite Real", ErrArithmeticOverflow, base, exp)
	}
	return Value{Kind: ValReal, Real: res}, nil
}

// intPow computes a**n for n >= 0 by repeated squaring, reporting ok=false when
// the result leaves the int64 range rather than wrapping.
func intPow(a, n int64) (int64, bool) {
	res := int64(1)
	for n > 0 {
		if n&1 == 1 {
			var ok bool
			if res, ok = mulInt(res, a); !ok {
				return 0, false
			}
		}
		if n >>= 1; n == 0 {
			break
		}
		var ok bool
		if a, ok = mulInt(a, a); !ok {
			return 0, false
		}
	}
	return res, true
}

// mulInt multiplies two int64 values, reporting ok=false on overflow.
func mulInt(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	// The least int64 negated is not an int64, and the division check below
	// cannot see that case because the quotient equals the dividend.
	if (a == math.MinInt64 && b == -1) || (b == math.MinInt64 && a == -1) {
		return 0, false
	}
	res := a * b
	if res/b != a {
		return 0, false
	}
	return res, true
}
