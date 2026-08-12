package runtime

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// EvalContext is the lexical environment during evaluation (Tier 3).
type EvalContext struct {
	ctx    *Context           // runtime context
	scope  *symbols.Scope     // scope context for name resolution
	self   *Instance          // instance a feature name resolves against, nil when unbound
	frames []map[string]Value // stack of local bindings (innermost = frames[len-1])
	trace  *TraceRecorder     // evaluation trace recorder, nil when not tracing

	// features are the features of the element being evaluated — a requirement's
	// or constraint's own, inherited and rebound features — which its conditions
	// may name wherever those conditions were written.
	features map[string]scopedExpr
}

// NewEvalContext creates an evaluation context with an empty frame stack. It
// inherits the runtime context's trace recorder, so every evaluation reached
// from a traced context is recorded, including nested calc invocations.
func NewEvalContext(ctx *Context, scope *symbols.Scope) *EvalContext {
	return &EvalContext{
		ctx:    ctx,
		scope:  scope,
		frames: nil,
		trace:  ctx.trace,
	}
}

// NewEvalContextIn creates an evaluation context bound to an instance, so that
// a feature name resolves to that instance's slot value rather than to the
// declared default of the same name.
func NewEvalContextIn(ctx *Context, scope *symbols.Scope, self *Instance) *EvalContext {
	ec := NewEvalContext(ctx, scope)
	ec.self = self
	return ec
}

// evalIn returns a context that resolves names in scope while sharing this
// one's bindings and trace, for a body member written in another declaration's
// scope (an inherited calc result or parameter default).
func (ec *EvalContext) evalIn(scope *symbols.Scope) *EvalContext {
	if scope == nil || scope == ec.scope {
		return ec
	}
	return &EvalContext{ctx: ec.ctx, scope: scope, self: ec.self, frames: ec.frames, trace: ec.trace, features: ec.features}
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
// When the context is traced, the evaluation is recorded after its
// sub-expressions, which makes sub-expression order part of the trace.
func (ec *EvalContext) Eval(node ast.Node) (Value, error) {
	if ec.trace == nil {
		return ec.eval(node)
	}
	ec.trace.BeginEval()
	value, err := ec.eval(node)
	ec.trace.EndEval(TraceLabel(node), value, err)
	return value, err
}

// eval dispatches one expression node, without trace bookkeeping.
func (ec *EvalContext) eval(node ast.Node) (Value, error) {
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
	case *ast.FeatureReference:
		return ec.evalFeatureReference(n)
	case *ast.FeatureChainExpr:
		return ec.evalFeatureChain(n)
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
	case *ast.BodyExpr:
		// BodyExpr is not directly evaluated - wrapped as ValExpr for delayed evaluation
		return Value{Kind: ValExpr, Expr: n}, nil
	default:
		return Value{}, fmt.Errorf("unsupported node type: %T", node)
	}
}

// Eval is the top-level entry point for evaluating an expression in an empty environment.
// Resolves names from the root scope.
func (ctx *Context) Eval(node ast.Node) (Value, error) {
	// Use resolver's root scope for name resolution
	// (In a full implementation, this would track evaluation context scope)
	ec := NewEvalContext(ctx, nil)
	return ec.Eval(node)
}

// EvalWithScope evaluates an expression with a given scope context for name resolution.
func (ctx *Context) EvalWithScope(node ast.Node, scope *symbols.Scope) (Value, error) {
	ec := NewEvalContext(ctx, scope)
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

// evalFeatureReference evaluates a feature reference (variable lookup).
func (ec *EvalContext) evalFeatureReference(n *ast.FeatureReference) (Value, error) {
	if n.Name == nil || len(n.Name.Parts) == 0 {
		return Value{}, fmt.Errorf("empty feature reference")
	}

	// Simple case: single-part name lookup in frame stack or scope
	if len(n.Name.Parts) == 1 {
		name := n.Name.Parts[0].Text
		// Try frame stack first (local bindings from calc/lambda params)
		if val, ok := ec.Lookup(name); ok {
			return val, nil
		}
		// Then the bound instance: a slot holds the value this object actually
		// carries, which overrides the declared default the scope would yield.
		if ec.self != nil {
			if val, ok, err := ec.selfSlotValue(name); err != nil {
				return Value{}, err
			} else if ok {
				return val, nil
			}
		}
		// Then a feature of the element being evaluated: a value a typed usage
		// binds masks the default carried by the declaration it redefines.
		if bound, ok := ec.features[name]; ok {
			if bound.expr == nil {
				return Value{}, fmt.Errorf("%w for feature %s", ErrNoValue, name)
			}
			return ec.evalIn(bound.scope).Eval(bound.expr)
		}
		// Try scope lookup (sibling attributes, inherited members)
		if ec.scope != nil {
			if sym, ok := ec.scope.LookupLocal(name); ok && sym != nil {
				if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Value != nil {
					return ec.Eval(usage.Value)
				}
			}
		}
		return Value{}, fmt.Errorf("unresolved feature: %s", name)
	}

	// Multi-part qualified names: A::B::x
	// Spec-compliant: Use model.LookupMember for member traversal.
	// Use resolver logic for first part (handles scope, imports, global index),
	// then walk remaining parts with model.LookupMember for inherited members.

	// Build single-segment qualified name for first part resolution via resolver
	firstName := n.Name.Parts[0]
	firstQN := &ast.QualifiedName{
		Global: n.Name.Global,
		Parts:  []ast.NameSegment{firstName},
	}
	firstQN.NodeBase = n.Name.NodeBase

	// Resolve first part using resolver's qualified-name logic (handles global index)
	currentSym, ok := ec.ctx.resolver.ResolveQualified(ec.scope, firstQN)
	if !ok {
		return Value{}, fmt.Errorf("unresolved first part of qualified name: %s", firstName.Text)
	}

	// Walk remaining parts using model.LookupMember (spec requirement)
	for i := 1; i < len(n.Name.Parts); i++ {
		memberName := n.Name.Parts[i].Text
		nextSym, found := ec.ctx.model.LookupMember(currentSym, memberName)
		if !found {
			return Value{}, fmt.Errorf("member %s not found in %s", memberName, currentSym.Name)
		}
		currentSym = nextSym
	}

	// Evaluate the final symbol's declaration
	switch decl := currentSym.Decl.(type) {
	case *ast.Usage:
		if decl.Value != nil {
			return ec.Eval(decl.Value)
		}
		return Value{}, fmt.Errorf("usage %s has no value", qualifiedNameToString(n.Name))
	case *ast.Definition:
		// Definitions are types, not values
		return Value{}, fmt.Errorf("cannot evaluate definition %s", qualifiedNameToString(n.Name))
	default:
		return Value{}, fmt.Errorf("cannot evaluate element type %T", decl)
	}
}

// selfSlotValue reads the named slot of the bound instance. Reports whether the
// instance has such a slot; an error means the slot exists but could not be
// materialized.
func (ec *EvalContext) selfSlotValue(name string) (Value, bool, error) {
	if _, ok := ec.self.Slots[name]; !ok {
		return Value{}, false, nil
	}
	slot, err := ec.self.GetSlot(ec.ctx, name)
	if err != nil {
		return Value{}, true, err
	}
	if slot.Values.Kind != ValInvalid {
		return slot.Values, true, nil
	}
	if slot.Value.Kind == ValInvalid {
		return Value{}, true, fmt.Errorf("%w: %s", ErrUninitializedSlot, name)
	}
	return slot.Value, true, nil
}

// evalFeatureChain evaluates a feature chain expression (e.g., obj.member.submember).
func (ec *EvalContext) evalFeatureChain(n *ast.FeatureChainExpr) (Value, error) {
	// Evaluate the operand (left side of the chain)
	operand, err := ec.Eval(n.Operand)
	if err != nil {
		return Value{}, err
	}

	// Feature chains only work on instances
	if operand.Kind != ValInstance {
		return Value{}, fmt.Errorf("feature chain requires instance, got %v", operand.Kind)
	}

	// Get the instance
	inst, ok := ec.ctx.instances[operand.Instance]
	if !ok {
		return Value{}, fmt.Errorf("instance ID %d not found", operand.Instance)
	}

	// Walk the member chain
	if n.Member == nil || len(n.Member.Parts) == 0 {
		return Value{}, fmt.Errorf("empty member chain")
	}

	// Navigate through the chain
	currentInst := inst
	for i, part := range n.Member.Parts {
		memberName := part.Text
		if _, ok := currentInst.Slots[memberName]; !ok {
			return Value{}, fmt.Errorf("member %s not found in instance", memberName)
		}
		// Read through GetSlot so a derived or composite member is materialized
		// on demand rather than read as an empty slot.
		slot, err := currentInst.GetSlot(ec.ctx, memberName)
		if err != nil {
			return Value{}, err
		}

		// Get the slot's value
		slotVal := slot.Value

		// If this is the last part, return the value
		if i == len(n.Member.Parts)-1 {
			return slotVal, nil
		}

		// Otherwise, navigate to the next instance
		if slotVal.Kind != ValInstance {
			return Value{}, fmt.Errorf("cannot chain through non-instance member %s", memberName)
		}

		nextInst, ok := ec.ctx.instances[slotVal.Instance]
		if !ok {
			return Value{}, fmt.Errorf("instance ID %d not found for member %s", slotVal.Instance, memberName)
		}
		currentInst = nextInst
	}

	return Value{}, fmt.Errorf("unexpected: fell through feature chain evaluation")
}

// evalOperator evaluates an operator expression.
func (ec *EvalContext) evalOperator(n *ast.OperatorExpr) (Value, error) {
	// Try constant folding first
	if semVal, ok := ec.ctx.model.Eval(n); ok {
		return Value{Kind: ValConst, Const: semVal}, nil
	}

	// Otherwise, recursively eval operands
	switch n.Operator {
	case ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpDiv, ast.OpPow:
		return ec.evalArithmetic(n)
	case ast.OpEq, ast.OpNeq:
		return ec.evalEquality(n)
	case ast.OpLt, ast.OpLe, ast.OpGt, ast.OpGe:
		return ec.evalComparison(n)
	case ast.OpAnd, ast.OpOr:
		return ec.evalLogical(n)
	case ast.OpNeg, ast.OpNot:
		return ec.evalNeg(n)
	default:
		return Value{}, fmt.Errorf("unsupported operator: %v", n.Operator)
	}
}

// evalArithmetic evaluates arithmetic operators (+, -, *, /, **).
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

	// Exponentiation shares the folder's implementation, so a folded and an
	// evaluated `**` agree; the folder declines where this reports the error.
	if n.Operator == ast.OpPow {
		res, err := semantics.Pow(left.Const, right.Const)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: ValConst, Const: res}, nil
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
	if len(n.Operands) != 2 {
		return Value{}, fmt.Errorf("equality requires 2 operands, got %d", len(n.Operands))
	}

	left, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	right, err := ec.Eval(n.Operands[1])
	if err != nil {
		return Value{}, err
	}

	equal := valueEqual(left, right)

	// Handle != operator
	if n.Operator == ast.OpNeq {
		equal = !equal
	}

	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: equal}}, nil
}

// evalComparison evaluates comparison operators (<, <=, >, >=).
func (ec *EvalContext) evalComparison(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 2 {
		return Value{}, fmt.Errorf("comparison requires 2 operands, got %d", len(n.Operands))
	}

	left, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}

	right, err := ec.Eval(n.Operands[1])
	if err != nil {
		return Value{}, err
	}

	// Both must be ValConst
	if left.Kind != ValConst || right.Kind != ValConst {
		return Value{}, fmt.Errorf("comparison operands must be constants, got %s and %s", left.Kind, right.Kind)
	}

	// Compare integers
	if left.Const.Kind == semantics.ValInt && right.Const.Kind == semantics.ValInt {
		var result bool
		switch n.Operator {
		case ast.OpLt:
			result = left.Const.Int < right.Const.Int
		case ast.OpLe:
			result = left.Const.Int <= right.Const.Int
		case ast.OpGt:
			result = left.Const.Int > right.Const.Int
		case ast.OpGe:
			result = left.Const.Int >= right.Const.Int
		default:
			return Value{}, fmt.Errorf("unknown comparison operator: %v", n.Operator)
		}
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: result}}, nil
	}

	// Compare reals (coerce int to real)
	leftReal := toReal(left.Const)
	rightReal := toReal(right.Const)
	var result bool
	switch n.Operator {
	case ast.OpLt:
		result = leftReal < rightReal
	case ast.OpLe:
		result = leftReal <= rightReal
	case ast.OpGt:
		result = leftReal > rightReal
	case ast.OpGe:
		result = leftReal >= rightReal
	default:
		return Value{}, fmt.Errorf("unknown comparison operator: %v", n.Operator)
	}
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: result}}, nil
}

// evalLogical evaluates logical operators (&&, ||) with short-circuit.
func (ec *EvalContext) evalLogical(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 2 {
		return Value{}, fmt.Errorf("logical operator requires 2 operands, got %d", len(n.Operands))
	}

	// Evaluate left operand
	left, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	if left.Kind != ValConst || left.Const.Kind != semantics.ValBool {
		return Value{}, fmt.Errorf("logical operator requires bool operands, got %v", left.Kind)
	}

	// Short-circuit for &&: if left is false, return false
	if n.Operator == ast.OpAnd && !left.Const.Bool {
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: false}}, nil
	}

	// Short-circuit for ||: if left is true, return true
	if n.Operator == ast.OpOr && left.Const.Bool {
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: true}}, nil
	}

	// Evaluate right operand
	right, err := ec.Eval(n.Operands[1])
	if err != nil {
		return Value{}, err
	}
	if right.Kind != ValConst || right.Const.Kind != semantics.ValBool {
		return Value{}, fmt.Errorf("logical operator requires bool operands, got %v", right.Kind)
	}

	// Compute result
	var result bool
	switch n.Operator {
	case ast.OpAnd:
		result = left.Const.Bool && right.Const.Bool
	case ast.OpOr:
		result = left.Const.Bool || right.Const.Bool
	default:
		return Value{}, fmt.Errorf("unsupported logical operator: %v", n.Operator)
	}

	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: result}}, nil
}

// evalNeg evaluates unary negation (-, not).
func (ec *EvalContext) evalNeg(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 1 {
		return Value{}, fmt.Errorf("negation requires 1 operand, got %d", len(n.Operands))
	}

	operand, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}

	switch n.Operator {
	case ast.OpNot:
		// Logical not: not bool
		if operand.Kind != ValConst || operand.Const.Kind != semantics.ValBool {
			return Value{}, fmt.Errorf("logical not requires bool operand, got %v", operand.Kind)
		}
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: !operand.Const.Bool}}, nil
	case ast.OpNeg:
		// Arithmetic negation: -number
		if operand.Kind != ValConst {
			return Value{}, fmt.Errorf("arithmetic negation requires numeric operand, got %v", operand.Kind)
		}
		result, ok := semantics.EvalUnary(ast.OpNeg, operand.Const)
		if !ok {
			return Value{}, fmt.Errorf("arithmetic negation failed for %v", operand.Const)
		}
		return Value{Kind: ValConst, Const: result}, nil
	default:
		return Value{}, fmt.Errorf("unsupported negation operator: %v", n.Operator)
	}
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

	// Eval args in source order
	args := make([]Value, len(n.Args))
	for i, arg := range n.Args {
		val, err := ec.Eval(arg)
		if err != nil {
			return Value{}, err
		}
		args[i] = val
	}

	named := make(map[string]Value, len(n.NamedArgs))
	for _, arg := range n.NamedArgs {
		if arg.Name == nil || len(arg.Name.Parts) == 0 {
			return Value{}, fmt.Errorf("unnamed argument in invocation of %s", qualName)
		}
		val, err := ec.Eval(arg.Value)
		if err != nil {
			return Value{}, err
		}
		named[arg.Name.Parts[len(arg.Name.Parts)-1].Text] = val
	}

	// Check builtin registry
	if fn, ok := builtins[qualName]; ok {
		if len(named) > 0 {
			return Value{}, fmt.Errorf("%w: builtin %s takes positional arguments only", ErrUnknownParameter, qualName)
		}
		return fn(ec, args)
	}

	// User-defined calc: resolve target symbol from the evaluation context scope.
	calcSym, ok := ec.ctx.resolver.ResolveQualified(ec.scope, n.Type)
	if !ok || calcSym == nil {
		// A KerML function library function is evaluable even where the model
		// imports no part of the library, so a name that denotes no declaration
		// still denotes the library function of that name.
		if fn, isLib := unresolvedLibraryFunction(n.Type, qualName); isLib {
			return fn.invoke(ec.ctx, calcArgs{positional: args, named: named})
		}
		return Value{}, fmt.Errorf("%w: calc %s", ErrUnresolvedReference, qualName)
	}

	// Every invocation goes through the one calc path, so an expression and a
	// direct InvokeCalc bind parameters and trace identically. The notation keeps
	// the argument forms mutually exclusive.
	if len(named) > 0 {
		return ec.ctx.InvokeCalcNamed(calcSym, named, ec.scope)
	}
	return ec.ctx.InvokeCalc(calcSym, args, ec.scope)
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

// valueEqual checks deep equality of two runtime values.
func valueEqual(a, b Value) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case ValConst:
		// Delegate to semantics layer for const equality
		result, ok := semantics.EvalBinary(ast.OpEq, a.Const, b.Const)
		return ok && result.Kind == semantics.ValBool && result.Bool
	case ValString:
		return a.Str == b.Str
	case ValNull:
		return true
	case ValInstance:
		return a.Instance == b.Instance
	case ValSequence:
		return sequenceEqual(a.Sequence, b.Sequence)
	case ValSet:
		return setEqual(a.Set, b.Set)
	default:
		return false
	}
}

// sequenceEqual checks structural equality of sequences (element-wise).
func sequenceEqual(a, b *Sequence) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Size() != b.Size() {
		return false
	}
	for i := 0; i < a.Size(); i++ {
		aElem, _ := a.At(i)
		bElem, _ := b.At(i)
		if !valueEqual(aElem, bElem) {
			return false
		}
	}
	return true
}

// setEqual checks set equality (same keys via valueKey).
func setEqual(a, b *Set) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Size() != b.Size() {
		return false
	}
	for key := range a.elements {
		if _, exists := b.elements[key]; !exists {
			return false
		}
	}
	return true
}
