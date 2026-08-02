package semantics

import (
	"strconv"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
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
	// Integer arithmetic when both operands are integers (except division,
	// which may be fractional — keep it real to avoid silent truncation).
	if l.Kind == ValInt && r.Kind == ValInt && op != ast.OpDiv && op != ast.OpPow {
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
	case ast.OpPow:
		return pow(a, b)
	}
	return Value{}, false
}

// pow computes a**b for non-negative integer exponents (the case bounds use),
// via repeated multiplication to avoid a math import. Non-integer or negative
// exponents are outside the supported subset and return ok=false.
func pow(a, b float64) (Value, bool) {
	n := int(b)
	if float64(n) != b || n < 0 {
		return Value{}, false
	}
	res := 1.0
	for i := 0; i < n; i++ {
		res *= a
	}
	return Value{Kind: ValReal, Real: res}, true
}
