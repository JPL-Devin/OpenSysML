package runtime

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

// EvalContext is the lexical environment during evaluation (Tier 3).
type EvalContext struct {
	ctx    *Context            // runtime context
	frames []map[string]Value  // stack of local bindings (innermost = frames[len-1])
}

// NewEvalContext creates an evaluation context with an empty frame stack.
func NewEvalContext(ctx *Context) *EvalContext {
	return &EvalContext{
		ctx:    ctx,
		frames: nil,
	}
}

// Push adds a new frame to the stack (on calc invocation, lambda entry).
func (ec *EvalContext) Push(bindings map[string]Value) {
	ec.frames = append(ec.frames, bindings)
}

// Pop removes the top frame from the stack (on return, lambda exit).
func (ec *EvalContext) Pop() {
	if len(ec.frames) > 0 {
		ec.frames = ec.frames[:len(ec.frames)-1]
	}
}

// Lookup searches for a name in the frame stack (innermost first).
func (ec *EvalContext) Lookup(name string) (Value, bool) {
	for i := len(ec.frames) - 1; i >= 0; i-- {
		if val, ok := ec.frames[i][name]; ok {
			return val, true
		}
	}
	return Value{}, false
}

// Eval evaluates an expression node. Returns a Value or an error.
// Increments ctx.steps on each eval call; errors when ctx.steps >= ctx.maxSteps.
func (ec *EvalContext) Eval(node ast.Node) (Value, error) {
	// Step counter
	if err := ec.ctx.incrementStep(); err != nil {
		return Value{}, err
	}
	
	// Dispatch by node type (scaffolding; full implementation in later tasks)
	switch n := node.(type) {
	case *ast.LiteralInteger:
		return ec.evalLiteralInteger(n)
	case *ast.LiteralReal:
		return ec.evalLiteralReal(n)
	case *ast.LiteralBool:
		return ec.evalLiteralBool(n)
	case *ast.LiteralString:
		return ec.evalLiteralString(n)
	case *ast.NullExpr:
		return ec.evalNull(n)
	case *ast.OperatorExpr:
		return ec.evalOperator(n)
	case *ast.SequenceExpr:
		return ec.evalSequenceExpr(n)
	case *ast.CollectExpr:
		return ec.evalCollectExpr(n)
	case *ast.SelectExpr:
		return ec.evalSelectExpr(n)
	case *ast.InvocationExpr:
		return ec.evalInvocation(n)
	default:
		return Value{}, fmt.Errorf("unsupported node type: %T", node)
	}
}

// Eval is the top-level entry point for evaluating an expression in an empty environment.
func (ctx *Context) Eval(node ast.Node) (Value, error) {
	ec := NewEvalContext(ctx)
	return ec.Eval(node)
}

// evalLiteralInteger evaluates an integer literal.
func (ec *EvalContext) evalLiteralInteger(n *ast.LiteralInteger) (Value, error) {
	val, _ := strconv.ParseInt(n.Value, 10, 64)
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: val}}, nil
}

// evalLiteralReal evaluates a real literal.
func (ec *EvalContext) evalLiteralReal(n *ast.LiteralReal) (Value, error) {
	val, _ := strconv.ParseFloat(n.Value, 64)
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: val}}, nil
}

// evalLiteralBool evaluates a boolean literal.
func (ec *EvalContext) evalLiteralBool(n *ast.LiteralBool) (Value, error) {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: n.Value}}, nil
}

// evalLiteralString evaluates a string literal.
func (ec *EvalContext) evalLiteralString(n *ast.LiteralString) (Value, error) {
	// Strip quotes
	str := n.Value
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}
	return Value{Kind: ValString, Str: str}, nil
}

// evalNull evaluates a null expression.
func (ec *EvalContext) evalNull(n *ast.NullExpr) (Value, error) {
	return Value{Kind: ValNull}, nil
}

// evalOperator evaluates an operator expression.
func (ec *EvalContext) evalOperator(n *ast.OperatorExpr) (Value, error) {
	// Try constant folding first
	if semVal, ok := ec.ctx.model.Eval(n); ok {
		return Value{Kind: ValConst, Const: semVal}, nil
	}

	// Otherwise, recursively eval operands
	switch n.Operator {
	case ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpDiv:
		return ec.evalArithmetic(n)
	case ast.OpEq, ast.OpNeq:
		return ec.evalEquality(n)
	case ast.OpLt, ast.OpLe, ast.OpGt, ast.OpGe:
		return ec.evalComparison(n)
	case ast.OpAnd, ast.OpOr:
		return ec.evalLogical(n)
	case ast.OpNeg:
		return ec.evalNeg(n)
	case ast.OpNot:
		return ec.evalNot(n)
	default:
		return Value{}, fmt.Errorf("unsupported operator: %v", n.Operator)
	}
}

// evalArithmetic evaluates arithmetic operators (+, -, *, /).
func (ec *EvalContext) evalArithmetic(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) < 2 {
		return Value{}, fmt.Errorf("arithmetic operator requires 2 operands")
	}
	left, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	right, err := ec.Eval(n.Operands[1])
	if err != nil {
		return Value{}, err
	}

	// Simplified: assume both are ValConst int/real
	if left.Kind != ValConst || right.Kind != ValConst {
		return Value{}, ErrTypeMismatch
	}

	// Integer arithmetic
	if left.Const.Kind == semantics.ValInt && right.Const.Kind == semantics.ValInt {
		var result int64
		switch n.Operator {
		case ast.OpAdd:
			result = left.Const.Int + right.Const.Int
		case ast.OpSub:
			result = left.Const.Int - right.Const.Int
		case ast.OpMul:
			result = left.Const.Int * right.Const.Int
		case ast.OpDiv:
			if right.Const.Int == 0 {
				return Value{}, fmt.Errorf("division by zero")
			}
			result = left.Const.Int / right.Const.Int
		}
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: result}}, nil
	}

	// Real arithmetic (coerce int to real if needed)
	leftReal := toReal(left.Const)
	rightReal := toReal(right.Const)
	var result float64
	switch n.Operator {
	case ast.OpAdd:
		result = leftReal + rightReal
	case ast.OpSub:
		result = leftReal - rightReal
	case ast.OpMul:
		result = leftReal * rightReal
	case ast.OpDiv:
		result = leftReal / rightReal
	}
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: result}}, nil
}

// toReal converts a semantics.Value to float64.
func toReal(v semantics.Value) float64 {
	if v.Kind == semantics.ValInt {
		return float64(v.Int)
	}
	return v.Real
}

// evalEquality evaluates equality operators (==, !=).
func (ec *EvalContext) evalEquality(n *ast.OperatorExpr) (Value, error) {
	return Value{}, fmt.Errorf("equality not yet implemented")
}

// evalComparison evaluates comparison operators (<, <=, >, >=).
func (ec *EvalContext) evalComparison(n *ast.OperatorExpr) (Value, error) {
	return Value{}, fmt.Errorf("comparison not yet implemented")
}

// evalLogical evaluates logical operators (&&, ||).
func (ec *EvalContext) evalLogical(n *ast.OperatorExpr) (Value, error) {
	return Value{}, fmt.Errorf("logical not yet implemented")
}

// evalNeg evaluates unary negation (-).
func (ec *EvalContext) evalNeg(n *ast.OperatorExpr) (Value, error) {
	return Value{}, fmt.Errorf("negation not yet implemented")
}

// evalNot evaluates logical not (!).
func (ec *EvalContext) evalNot(n *ast.OperatorExpr) (Value, error) {
	return Value{}, fmt.Errorf("not not yet implemented")
}

// evalSequenceExpr evaluates a sequence expression (1, 2, 3).
func (ec *EvalContext) evalSequenceExpr(n *ast.SequenceExpr) (Value, error) {
	seq := NewSequence()
	for _, elem := range n.Elements {
		val, err := ec.Eval(elem)
		if err != nil {
			return Value{}, err
		}
		seq.Append(val)
	}
	return Value{Kind: ValSequence, Sequence: seq}, nil
}

// evalCollectExpr evaluates `operand . body` — map over collection.
func (ec *EvalContext) evalCollectExpr(n *ast.CollectExpr) (Value, error) {
	operand, err := ec.Eval(n.Operand)
	if err != nil {
		return Value{}, err
	}
	
	var elements []Value
	switch operand.Kind {
	case ValSequence:
		elements = operand.Sequence.Elements()
	case ValSet:
		elements = operand.Set.Elements()
	default:
		return Value{}, fmt.Errorf("%w: collect operand must be collection", ErrTypeMismatch)
	}
	
	result := NewSequence()
	for _, elem := range elements {
		// Push 'it' binding for body
		ec.Push(map[string]Value{"it": elem})
		val, err := ec.Eval(n.Body)
		ec.Pop()
		if err != nil {
			return Value{}, err
		}
		result.Append(val)
	}
	
	return Value{Kind: ValSequence, Sequence: result}, nil
}

// evalSelectExpr evaluates `operand .? body` — filter collection.
func (ec *EvalContext) evalSelectExpr(n *ast.SelectExpr) (Value, error) {
	operand, err := ec.Eval(n.Operand)
	if err != nil {
		return Value{}, err
	}
	
	var elements []Value
	switch operand.Kind {
	case ValSequence:
		elements = operand.Sequence.Elements()
	case ValSet:
		elements = operand.Set.Elements()
	default:
		return Value{}, fmt.Errorf("%w: select operand must be collection", ErrTypeMismatch)
	}
	
	result := NewSequence()
	for _, elem := range elements {
		ec.Push(map[string]Value{"it": elem})
		predVal, err := ec.Eval(n.Body)
		ec.Pop()
		if err != nil {
			return Value{}, err
		}
		
		// Check if predicate is true (ValConst boolean)
		if predVal.Kind == ValConst && predVal.Const.Kind == semantics.ValBool && predVal.Const.Bool {
			result.Append(elem)
		}
	}
	
	return Value{Kind: ValSequence, Sequence: result}, nil
}

// evalInvocation evaluates a function/calc invocation.
func (ec *EvalContext) evalInvocation(n *ast.InvocationExpr) (Value, error) {
	// Build qualified name string for builtin lookup
	qualName := qualifiedNameToString(n.Type)
	
	// Eval args
	args := make([]Value, len(n.Args))
	for i, arg := range n.Args {
		val, err := ec.Eval(arg)
		if err != nil {
			return Value{}, err
		}
		args[i] = val
	}
	
	// Check builtin registry
	if fn, ok := builtins[qualName]; ok {
		return fn(ec, args)
	}
	
	// User-defined calc: resolve target
	// (Simplified: requires test to provide proper scope context; defer full resolution to integration)
	return Value{}, fmt.Errorf("user-defined calc invocation not yet fully implemented: %s", qualName)
}

// qualifiedNameToString converts a QualifiedName AST node to "Package::Name" format.
func qualifiedNameToString(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	parts := make([]string, 0, len(qn.Parts))
	for _, seg := range qn.Parts {
		if seg.Text != "" {
			parts = append(parts, seg.Text)
		}
	}
	return strings.Join(parts, "::")
}

