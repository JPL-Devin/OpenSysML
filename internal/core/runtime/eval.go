package runtime

import (
	"fmt"
	"math"
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

	// resolving holds the features whose own value is being evaluated, so a value
	// written in terms of a same-named outer one does not resolve to itself.
	resolving map[string]bool

	// calcRun is the calc evaluation whose output feature is being computed, so an
	// output binding written in terms of the calc's other outputs reads them from
	// the same evaluation. It is nil everywhere else.
	calcRun *calcRun

	// activation identifies the execution of the body this evaluation belongs to,
	// so every output read of one calc usage within it comes from one evaluation
	// of that usage. It is zero outside a body, where nothing can change between
	// two reads.
	activation int64
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

// beginStep gives an evaluation outside a body a scope of its own, so what it reads
// - a calc usage's outputs, a collection's elements - is not held past the step. The
// returned function ends it.
func (ec *EvalContext) beginStep() func() {
	activation, end := ec.ctx.beginStep()
	ec.activation = activation
	return end
}

// evalIn returns a context that resolves names in scope while sharing this
// one's bindings and trace, for a body member written in another declaration's
// scope (an inherited calc result or parameter default).
func (ec *EvalContext) evalIn(scope *symbols.Scope) *EvalContext {
	if scope == nil || scope == ec.scope {
		return ec
	}
	return &EvalContext{
		ctx: ec.ctx, scope: scope, self: ec.self, frames: ec.frames, trace: ec.trace,
		features: ec.features, resolving: ec.resolving, calcRun: ec.calcRun,
		activation: ec.activation,
	}
}

// nestedEnv returns a context resolving names in scope over this one's
// environment, for a declaration nested in the body being evaluated: its
// bindings stay in force under whatever frame the nested declaration pushes.
func (ec *EvalContext) nestedEnv(scope *symbols.Scope) *EvalContext {
	frames := make([]map[string]Value, len(ec.frames))
	copy(frames, ec.frames)
	return &EvalContext{
		ctx: ec.ctx, scope: scope, self: ec.self, frames: frames, trace: ec.trace,
		features: ec.features, resolving: ec.resolving, calcRun: ec.calcRun,
		activation: ec.activation,
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
	case *ast.IndexExpr:
		return ec.evalIndexExpr(n)
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
	defer ctx.beginRun()()

	// Use resolver's root scope for name resolution
	// (In a full implementation, this would track evaluation context scope)
	ec := NewEvalContext(ctx, nil)
	return ec.Eval(node)
}

// EvalWithScope evaluates an expression with a given scope context for name resolution.
func (ctx *Context) EvalWithScope(node ast.Node, scope *symbols.Scope) (Value, error) {
	defer ctx.beginRun()()

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
		// Then another output feature of the calc whose output is being computed:
		// an `out` binding may be written in terms of the calc's other outputs,
		// which are evaluated from the same run of its body.
		if value, ok, err := ec.calcRun.lookupOutput(ec.ctx, name); ok {
			return value, err
		}
		// Then a valued feature of the element being evaluated: it is declared
		// inside that element, so it masks a same-named member of the object
		// carrying it, and a value a typed usage binds masks the default of the
		// declaration it redefines.
		// A feature whose own value is already being evaluated is skipped, so
		// `in mass = mass` reads the outer mass rather than itself.
		bound, declared := ec.features[name]
		if declared && bound.expr != nil && !ec.resolving[name] {
			if ec.resolving == nil {
				ec.resolving = map[string]bool{}
			}
			ec.resolving[name] = true
			val, err := ec.evalIn(bound.scope).Eval(bound.expr)
			delete(ec.resolving, name)
			return val, err
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
		// Then the scope the expression was written in: a sibling attribute, a
		// member of an enclosing namespace, or a name an import brought in, found
		// the way a written reference finds it. The declaration's own value is
		// evaluated in the scope it was declared in, so the imports in force there
		// — rather than the ones in force here — answer the names it uses.
		if ec.scope != nil && !ec.resolving[name] {
			if sym, ok := ec.ctx.resolver.LookupName(ec.scope, name); ok && sym != nil {
				// A variant names a choice, not the value it declares.
				if ec.ctx.model.VariationPointOwning(sym) != nil {
					return variantReference(sym), nil
				}
				if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Value != nil {
					if ec.resolving == nil {
						ec.resolving = map[string]bool{}
					}
					ec.resolving[name] = true
					val, err := ec.evalIn(sym.OwnerScope).Eval(usage.Value)
					delete(ec.resolving, name)
					if err != nil {
						return Value{}, err
					}
					return ec.bindVariationOf(sym, val)
				}
				// A variation holds nothing until it is bound, whether it is read
				// through an object or through its declaration.
				if ec.ctx.model.IsVariationFeature(sym) {
					return Value{}, fmt.Errorf("%w: %s", ErrVariationUnselected, name)
				}
			}
		}
		// Nothing outside the feature supplies its value, so its own value depends
		// on itself.
		if ec.resolving[name] {
			return Value{}, fmt.Errorf("%w: %s", ErrCyclicSlot, name)
		}
		// A feature the element declares but nothing gives a value to is
		// uninitialized rather than unresolved.
		if declared {
			return Value{}, fmt.Errorf("%w for feature %s", ErrNoValue, name)
		}
		return Value{}, fmt.Errorf("%w: %s", ErrUnresolvedFeature, name)
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
		// A calc usage's output features are computed rather than declared
		// values, so the rest of the name is read from an evaluation of the usage.
		if isCalcUsageSymbol(currentSym) {
			return ec.evalCalcUsageMembers(currentSym, n.Name.Parts[i:])
		}
		memberName := n.Name.Parts[i].Text
		nextSym, found := ec.ctx.model.LookupMember(currentSym, memberName)
		if !found {
			// A name qualified by a variation designates one of its variants, so
			// one that is not a variant is reported as such.
			if ec.ctx.model.IsVariationFeature(currentSym) {
				return Value{}, fmt.Errorf("%w: %s is not a variant of %s (%s)",
					ErrNotAVariant, memberName, currentSym.Name,
					ec.ctx.variantSummary(currentSym))
			}
			return Value{}, fmt.Errorf("member %s not found in %s", memberName, currentSym.Name)
		}
		currentSym = nextSym
	}

	// Evaluate the final symbol's declaration
	switch decl := currentSym.Decl.(type) {
	case *ast.Usage:
		// A variant names a choice its variation can be bound to, and compares
		// equal to the variation that selected it.
		if ec.ctx.model.VariationPointOwning(currentSym) != nil {
			return variantReference(currentSym), nil
		}
		if decl.Value != nil {
			val, err := ec.Eval(decl.Value)
			if err != nil {
				return Value{}, err
			}
			return ec.bindVariationOf(currentSym, val)
		}
		if ec.ctx.model.IsVariationFeature(currentSym) {
			return Value{}, fmt.Errorf("%w: %s", ErrVariationUnselected, qualifiedNameToString(n.Name))
		}
		// A calc usage is an evaluation, not a value: it is read through the output
		// features it computes, since a name it does not designate a result for has
		// no one value.
		if isCalcUsageSymbol(currentSym) {
			return Value{}, fmt.Errorf(
				"%w: calc usage %s computes output features (%s); read one of them",
				ErrNoValue, qualifiedNameToString(n.Name), ec.ctx.calcUsageOutputSummary(currentSym),
			)
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
	value := slot.HeldValue()
	if value.Kind == ValInvalid {
		return Value{}, true, fmt.Errorf("%w: %s", ErrUninitializedSlot, name)
	}
	return value, true, nil
}

// evalFeatureChain evaluates a feature chain expression (e.g., obj.member.submember).
func (ec *EvalContext) evalFeatureChain(n *ast.FeatureChainExpr) (Value, error) {
	if n.Member == nil || len(n.Member.Parts) == 0 {
		return Value{}, fmt.Errorf("empty member chain")
	}
	base, parts := chainBase(n)

	// A calc usage carries no value of its own: its output features are computed
	// by evaluating it, so `c.a` runs the usage — once — and reads the output
	// from that evaluation rather than from a slot.
	if sym, ok := ec.calcUsageOperand(base); ok {
		return ec.evalCalcUsageMembers(sym, parts)
	}

	// A part carries no value of its own: it denotes an occurrence, whose features
	// `lander.mass.mDry` reads, so the chain is read from that object.
	if sym, ok := ec.occurrenceOperand(base); ok {
		inst, err := ec.ctx.occurrenceOf(sym)
		if err != nil {
			return Value{}, fmt.Errorf("usage %s: %w", sym.Name, err)
		}
		return ec.chainMemberValue(Value{Kind: ValInstance, Instance: inst.ID}, parts, sym.Name)
	}

	// Evaluate the operand (left side of the chain)
	operand, err := ec.Eval(n.Operand)
	if err != nil {
		return Value{}, err
	}

	if operand.Kind == ValInstance {
		if _, ok := ec.ctx.instances[operand.Instance]; !ok {
			return Value{}, fmt.Errorf("instance ID %d not found", operand.Instance)
		}
	}

	return ec.chainMemberValue(operand, n.Member.Parts, "")
}

// chainBase flattens a nested feature chain: `lander.mass.mDry` is one chain of
// members from `lander`, not a chain through the value of `lander.mass`.
func chainBase(n *ast.FeatureChainExpr) (ast.Node, []ast.NameSegment) {
	operand, parts := n.Operand, n.Member.Parts
	for {
		inner, ok := operand.(*ast.FeatureChainExpr)
		if !ok || inner.Member == nil || len(inner.Member.Parts) == 0 {
			return operand, parts
		}
		parts = append(append([]ast.NameSegment{}, inner.Member.Parts...), parts...)
		operand = inner.Operand
	}
}

// chainMemberValue reads the members named by parts from the object value names,
// navigating through the objects the intermediate members name. from names the
// member value came from, for a diagnostic about chaining through it.
//
// A chain's values are its last feature's values over every object the features
// before it name (KerML 1.0 §7.3.4.6), so a multi-valued member is navigated
// through each of its objects, concatenated in order and flattened one level.
func (ec *EvalContext) chainMemberValue(value Value, parts []ast.NameSegment, from string) (Value, error) {
	if len(parts) == 0 {
		return value, nil
	}

	switch value.Kind {
	case ValSequence, ValSet:
		return ec.chainOverElements(value, parts, from)
	case ValInstance, ValVariant:
		// handled below
	default:
		return Value{}, fmt.Errorf("cannot chain through non-instance member %s (%v)", from, value.Kind)
	}

	// A selected variant is chained through the object it materialized.
	id, isObject := value.Object()
	if !isObject {
		return Value{}, fmt.Errorf("cannot chain through non-instance member %s (%v)", from, value.Kind)
	}
	inst, ok := ec.ctx.instances[id]
	if !ok {
		return Value{}, fmt.Errorf("instance ID %d not found for member %s", id, from)
	}
	name := parts[0].Text
	slotDecl, ok := inst.Slots[name]
	if !ok {
		// A calc usage is an evaluation rather than a slot, so its outputs are
		// read from a run of it against this object.
		if sym, found := ec.ctx.model.LookupMember(inst.Type, name); found && isCalcUsageSymbol(sym) {
			return ec.calcUsageMemberValue(sym, inst, parts[1:])
		}
		return Value{}, fmt.Errorf("member %s not found in instance", name)
	}
	// A variant named through the variation feature it belongs to is the choice
	// itself, not a member of the variation's value.
	if variant, rest, ok := ec.variantSegment(slotDecl.Feature, parts[1:]); ok {
		if len(rest) == 0 {
			return variantReference(variant), nil
		}
		// Members are read from the object the variant stands for.
		val, err := ec.ctx.variantValue(slotDecl.Feature.Symbol, variant, inst.ID)
		if err != nil {
			return Value{}, err
		}
		return ec.chainMemberValue(val, rest, variant.Name)
	}
	// Read through GetSlot so a derived or composite member is materialized
	// on demand rather than read as an empty slot.
	slot, err := inst.GetSlot(ec.ctx, name)
	if err != nil {
		return Value{}, err
	}
	member := slot.HeldValue()
	if member.Kind == ValInvalid {
		return Value{}, fmt.Errorf("%w: %s", ErrUninitializedSlot, name)
	}
	return ec.chainMemberValue(member, parts[1:], name)
}

// chainOverElements reads the rest of a chain from every element of a
// multi-valued member, concatenating the values each contributes.
func (ec *EvalContext) chainOverElements(value Value, parts []ast.NameSegment, from string) (Value, error) {
	var collected []Value
	for _, elem := range elementsOf(value) {
		val, err := ec.chainMemberValue(elem, parts, from)
		if err != nil {
			return Value{}, err
		}
		contributed := elementsOf(val)
		if err := ec.ctx.chargeElements(int64(len(contributed))); err != nil {
			return Value{}, err
		}
		collected = append(collected, contributed...)
	}
	return sequenceOf(collected), nil
}

// unimplementedOperators names the operators the runtime does not evaluate and
// says what each would need, so reaching one reports why rather than "unsupported".
var unimplementedOperators = map[ast.OperatorKind]string{
	ast.OpBitNot:  "bitwise complement is declared by no function library the runtime applies",
	ast.OpHasType: "classification needs the runtime type of a value, which values do not carry yet",
	ast.OpIsType:  "classification needs the runtime type of a value, which values do not carry yet",
	ast.OpAt:      "classification needs the runtime type of a value, which values do not carry yet",
	ast.OpMetaAt:  "metadata classification needs the metadata of a value at runtime",
	ast.OpAs:      "a cast needs the runtime type of a value, which values do not carry yet",
	ast.OpMeta:    "metadata access is evaluated from a MetadataAccessExpression, not this operator",
	ast.OpAll:     "'all' needs the extent of a type, which the runtime does not enumerate",
	ast.OpIndex:   "indexing is evaluated from an IndexExpression, not this operator",
}

// evalOperator evaluates an operator expression. A constant one is answered by
// the folder; every other operator the folder recognizes is evaluated here, so
// an operand that depends on a parameter does not make the operator fail.
func (ec *EvalContext) evalOperator(n *ast.OperatorExpr) (Value, error) {
	// Try constant folding first
	if semVal, ok := ec.ctx.model.Eval(n); ok {
		return Value{Kind: ValConst, Const: semVal}, nil
	}

	// Otherwise, recursively eval operands
	switch n.Operator {
	case ast.OpConditional:
		return ec.evalConditional(n)
	case ast.OpNullCoalesce:
		return ec.evalNullCoalesce(n)
	case ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpDiv, ast.OpMod, ast.OpPow:
		return ec.evalArithmetic(n)
	case ast.OpEq, ast.OpNeq:
		return ec.evalEquality(n)
	case ast.OpEqEqEq, ast.OpNeqEqEq:
		return ec.evalIdentity(n)
	case ast.OpLt, ast.OpLe, ast.OpGt, ast.OpGe:
		return ec.evalComparison(n)
	case ast.OpAnd, ast.OpConditionalAnd, ast.OpOr, ast.OpConditionalOr, ast.OpXor, ast.OpImplies:
		return ec.evalLogical(n)
	case ast.OpNeg, ast.OpPos, ast.OpNot:
		return ec.evalUnary(n)
	case ast.OpRange:
		return ec.evalRange(n)
	default:
		if why, ok := unimplementedOperators[n.Operator]; ok {
			return Value{}, fmt.Errorf("%w: '%s': %s", ErrUnsupportedOperator, n.Operator, why)
		}
		return Value{}, fmt.Errorf("%w: '%s'", ErrUnsupportedOperator, n.Operator)
	}
}

// evalConditional evaluates `if c ? a else b`, evaluating only the branch the
// condition selects — the other one is never evaluated, so a guarded recursion
// terminates at its base case.
func (ec *EvalContext) evalConditional(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 3 {
		return Value{}, fmt.Errorf("conditional requires 3 operands, got %d", len(n.Operands))
	}
	cond, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	held, err := boolOperand("condition of 'if'", cond)
	if err != nil {
		return Value{}, err
	}
	if held {
		return ec.Eval(n.Operands[1])
	}
	return ec.Eval(n.Operands[2])
}

// evalNullCoalesce evaluates `a ?? b`, evaluating b only when a is null.
func (ec *EvalContext) evalNullCoalesce(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 2 {
		return Value{}, fmt.Errorf("'??' requires 2 operands, got %d", len(n.Operands))
	}
	left, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	if left.Kind != ValNull {
		return left, nil
	}
	return ec.Eval(n.Operands[1])
}

// evalIdentity evaluates the identity operators (===, !==). Two values are the
// same one when they have the same kind and the same content, so an Integer is
// never identical to a Real of equal magnitude.
func (ec *EvalContext) evalIdentity(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 2 {
		return Value{}, fmt.Errorf("identity requires 2 operands, got %d", len(n.Operands))
	}
	left, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	right, err := ec.Eval(n.Operands[1])
	if err != nil {
		return Value{}, err
	}

	same := valueIdentical(left, right)
	if n.Operator == ast.OpNeqEqEq {
		same = !same
	}
	return boolValue(same), nil
}

// valueIdentical reports whether two values are the same value, which is what
// the identity operator `===` and SequenceFunctions::same ask. Identity is
// stricter than equality: a value of another kind, or a constant of another
// kind, is never the same value, so an Integer is not identical to a Real of
// equal magnitude.
func valueIdentical(left, right Value) bool {
	if left.Kind != right.Kind {
		return false
	}
	if left.Kind == ValConst && left.Const.Kind != right.Const.Kind {
		return false
	}
	return valueEqual(left, right)
}

// evalArithmetic evaluates arithmetic operators (+, -, *, /, %, **).
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

	// A quantity carries its unit through arithmetic: a sum converts, a product
	// composes units.
	if lq, rq, ok := quantityOperands(left, right); ok {
		switch n.Operator {
		case ast.OpAdd, ast.OpSub:
			return addQuantities(n.Operator, lq, rq)
		case ast.OpMul, ast.OpDiv:
			return scaleQuantities(n.Operator, lq, rq)
		case ast.OpPow:
			if right.Kind != ValConst {
				return Value{}, fmt.Errorf("%w: exponent of a quantity is a quantity", ErrTypeMismatch)
			}
			return powQuantity(lq, right.Const)
		case ast.OpMod:
			return Value{}, fmt.Errorf("%w: '%%' is not defined for a quantity", ErrTypeMismatch)
		}
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
		case ast.OpMod:
			if right.Const.Int == 0 {
				return Value{}, fmt.Errorf("division by zero")
			}
			result = left.Const.Int % right.Const.Int
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
	case ast.OpMod:
		if rightReal == 0 {
			return Value{}, fmt.Errorf("division by zero")
		}
		result = math.Mod(leftReal, rightReal)
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

	// Comparing a value with a variant compares it with the value that variant
	// declares; comparing two variants compares the choice itself.
	if (left.Kind == ValVariant) != (right.Kind == ValVariant) {
		if left, err = ec.ctx.variantAsValue(left); err != nil {
			return Value{}, err
		}
		if right, err = ec.ctx.variantAsValue(right); err != nil {
			return Value{}, err
		}
	}

	// Quantities compare in a common unit; incommensurable ones are an error,
	// not an inequality.
	if lq, rq, ok := quantityOperands(left, right); ok {
		return equalQuantities(n.Operator, lq, rq)
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

	// Quantities are ordered in a common unit, so a magnitude is never compared
	// across units without conversion.
	if lq, rq, ok := quantityOperands(left, right); ok {
		return compareQuantities(n.Operator, lq, rq)
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

// evalLogical evaluates the Boolean binary operators. `and`, `or` and `implies`
// decide on their left operand alone where they can, so the operand a guard
// rules out is never evaluated.
func (ec *EvalContext) evalLogical(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 2 {
		return Value{}, fmt.Errorf("logical operator requires 2 operands, got %d", len(n.Operands))
	}

	left, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	l, err := boolOperand(fmt.Sprintf("left operand of '%s'", n.Operator), left)
	if err != nil {
		return Value{}, err
	}

	switch n.Operator {
	case ast.OpAnd, ast.OpConditionalAnd:
		if !l {
			return boolValue(false), nil
		}
	case ast.OpOr, ast.OpConditionalOr:
		if l {
			return boolValue(true), nil
		}
	case ast.OpImplies:
		if !l {
			return boolValue(true), nil
		}
	}

	right, err := ec.Eval(n.Operands[1])
	if err != nil {
		return Value{}, err
	}
	r, err := boolOperand(fmt.Sprintf("right operand of '%s'", n.Operator), right)
	if err != nil {
		return Value{}, err
	}

	var result bool
	switch n.Operator {
	case ast.OpAnd, ast.OpConditionalAnd:
		result = l && r
	case ast.OpOr, ast.OpConditionalOr:
		result = l || r
	case ast.OpXor:
		result = l != r
	case ast.OpImplies:
		result = !l || r
	default:
		return Value{}, fmt.Errorf("%w: '%s' is not a Boolean operator", ErrUnsupportedOperator, n.Operator)
	}
	return boolValue(result), nil
}

// boolOperand reads a Boolean out of a value, naming what was expected when the
// value is not one.
func boolOperand(what string, v Value) (bool, error) {
	if v.Kind != ValConst || v.Const.Kind != semantics.ValBool {
		return false, fmt.Errorf("%w: %s must be Boolean, got %s", ErrTypeMismatch, what, v.Kind)
	}
	return v.Const.Bool, nil
}

// evalUnary evaluates the unary operators (-, +, not).
func (ec *EvalContext) evalUnary(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 1 {
		return Value{}, fmt.Errorf("unary operator requires 1 operand, got %d", len(n.Operands))
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
		return boolValue(!operand.Const.Bool), nil
	case ast.OpNeg, ast.OpPos:
		if operand.Kind == ValQuantity {
			if n.Operator == ast.OpPos {
				return operand, nil
			}
			return negateQuantity(operand.Quantity), nil
		}
		// Arithmetic sign: -number, +number
		if operand.Kind != ValConst {
			return Value{}, fmt.Errorf("unary '%s' requires numeric operand, got %v", n.Operator, operand.Kind)
		}
		result, ok := semantics.EvalUnary(n.Operator, operand.Const)
		if !ok {
			return Value{}, fmt.Errorf("unary '%s' is not defined for %v", n.Operator, operand.Const)
		}
		return Value{Kind: ValConst, Const: result}, nil
	default:
		return Value{}, fmt.Errorf("%w: '%s' is not a unary operator", ErrUnsupportedOperator, n.Operator)
	}
}

// evalSequenceExpr evaluates a sequence expression, `(1, 2, 3)`. A KerML
// sequence is flat: an element that is itself a collection contributes its
// elements, which is what makes SequenceFunctions::union the sequence
// expression `(seq1, seq2)` rather than a two-element sequence of sequences.
func (ec *EvalContext) evalSequenceExpr(n *ast.SequenceExpr) (Value, error) {
	elements := make([]Value, 0, len(n.Elements))
	for _, elem := range n.Elements {
		val, err := ec.Eval(elem)
		if err != nil {
			return Value{}, err
		}
		elements = append(elements, elementsOf(val)...)
	}
	return ec.newSequence(elements)
}

// evalCollectExpr evaluates `operand.{in x; ...}`, the collect notation, which
// KerML defines as ControlFunctions::collect of the operand and the body. It
// evaluates through that one implementation, so the notation and the call
// `collect(seq, {in x; ...})` compute the same result.
func (ec *EvalContext) evalCollectExpr(n *ast.CollectExpr) (Value, error) {
	return ec.evalCollectionNotation("collect", n.Operand, n.Body, builtinControlCollect)
}

// evalSelectExpr evaluates `operand.?{in x; ...}`, the select notation, which
// KerML defines as ControlFunctions::select of the operand and the body.
func (ec *EvalContext) evalSelectExpr(n *ast.SelectExpr) (Value, error) {
	return ec.evalCollectionNotation("select", n.Operand, n.Body, builtinControlSelect)
}

// evalCollectionNotation evaluates a notation whose meaning is a library
// function of an operand and a body: the body is evaluated to the function it
// denotes rather than to a value, and the operation decides what to call it
// with.
func (ec *EvalContext) evalCollectionNotation(
	notation string,
	operandExpr, bodyExpr ast.Node,
	fn func(*EvalContext, []Value) (Value, error),
) (Value, error) {
	operand, err := ec.Eval(operandExpr)
	if err != nil {
		return Value{}, err
	}
	if bodyExpr == nil {
		return Value{}, fmt.Errorf("%w: %s states no body", ErrNoResultExpression, notation)
	}
	body, err := ec.Eval(bodyExpr)
	if err != nil {
		return Value{}, err
	}
	return fn(ec, []Value{operand, body})
}

// evalInvocation evaluates a function/calc invocation.
func (ec *EvalContext) evalInvocation(n *ast.InvocationExpr) (Value, error) {
	// Build qualified name string for builtin lookup
	qualName := qualifiedNameToString(n.Type)

	// A receiver binds by position, so it has no meaning beside arguments that
	// bind by name: reported rather than evaluated and dropped.
	if n.Operand != nil && len(n.NamedArgs) > 0 {
		return Value{}, fmt.Errorf(
			"%w: %s is called with a receiver and named arguments",
			ErrReceiverWithNamedArgs, qualName,
		)
	}

	// Eval args in source order. An operand is the first argument of the
	// invocation it is written before: `seq->size()` invokes size with seq, which
	// is how the semantics layer reads the same expression (passes/
	// typecheck_expr.go), so the two agree on which parameter an argument binds.
	exprs := n.Args
	if n.Operand != nil {
		exprs = append([]ast.Node{n.Operand}, n.Args...)
	}
	args := make([]Value, len(exprs))
	for i, arg := range exprs {
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
		// The same holds of the collection functions: `seq->size()` and `size(seq)`
		// denote SequenceFunctions::size, whose implementation the qualified name
		// reaches above. Only an unqualified name the model declares nothing for
		// gets here, so a model's own `size` calc still denotes itself.
		if fn, isBuiltin := builtinsByLocalName[qualName]; isBuiltin {
			if len(named) > 0 {
				return Value{}, fmt.Errorf("%w: builtin %s takes positional arguments only", ErrUnknownParameter, qualName)
			}
			return fn(ec, args)
		}
		return Value{}, fmt.Errorf("%w: calc %s", ErrUnresolvedReference, qualName)
	}

	// A name that resolves to one of the collection function declarations is
	// computed by the implementation of that operation, whatever notation the
	// call was written in and whether or not the library declaration carries a
	// body to evaluate instead.
	if fn, isBuiltin := ec.ctx.builtinFor(calcSym); isBuiltin {
		if len(named) > 0 {
			return Value{}, fmt.Errorf("%w: builtin %s takes positional arguments only", ErrUnknownParameter, qualName)
		}
		return fn(ec, args)
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
	case ValVariant:
		// A variation compares equal to the variant it selected.
		return a.Variant == b.Variant
	case ValQuantity:
		// Incommensurable units are not equal here: an equality that has to hold
		// or fail (a set member, a sequence element) has no error to report.
		converted, err := b.Quantity.convertTo(a.Quantity.Unit)
		return err == nil && toReal(a.Quantity.Num) == converted
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
