package runtime

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// EvalContext is the lexical environment during evaluation (Tier 3).
type EvalContext struct {
	ctx    *Context       // runtime context
	scope  *symbols.Scope // scope context for name resolution
	self   *Instance      // instance a feature name resolves against, nil when unbound
	frames []frame        // stack of local bindings (innermost = frames[len-1])
	trace  *TraceRecorder // evaluation trace recorder, nil when not tracing

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

	// inBehaviorBody marks a statement of a behavior body, which reaches the
	// object performing it only through names that resolve to its features.
	inBehaviorBody bool

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
// a feature name resolves to that instance's feature value rather than to the
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
		activation: ec.activation, inBehaviorBody: ec.inBehaviorBody,
	}
}

// nestedEnv returns a context resolving names in scope over this one's
// environment, for a declaration nested in the body being evaluated: its
// bindings stay in force under whatever frame the nested declaration pushes.
func (ec *EvalContext) nestedEnv(scope *symbols.Scope) *EvalContext {
	frames := make([]frame, len(ec.frames))
	copy(frames, ec.frames)
	return ec.over(scope, frames)
}

// closure snapshots the environment for an expression evaluated later; bindings
// are copied since an invocation's frame storage is reused once it returns.
func (ec *EvalContext) closure() *EvalContext {
	frames := make([]frame, len(ec.frames))
	for i, f := range ec.frames {
		frames[i] = f.snapshot()
	}
	return ec.over(ec.scope, frames)
}

// over is this environment resolving names in scope over frames of its own.
func (ec *EvalContext) over(scope *symbols.Scope, frames []frame) *EvalContext {
	return &EvalContext{
		ctx: ec.ctx, scope: scope, self: ec.self, frames: frames, trace: ec.trace,
		features: ec.features, resolving: ec.resolving, calcRun: ec.calcRun,
		activation: ec.activation, inBehaviorBody: ec.inBehaviorBody,
	}
}

// Push adds a new frame to the stack (on calc invocation, lambda entry).
func (ec *EvalContext) Push(bindings map[string]Value) {
	ec.frames = append(ec.frames, mapFrame(bindings))
}

// pushFrame adds a frame to the stack.
func (ec *EvalContext) pushFrame(f frame) {
	ec.frames = append(ec.frames, f)
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
		if val, ok := ec.frames[i].lookup(name); ok {
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
	case *ast.QualifiedName:
		return ec.evalName(n)
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
		// A body is a value closed over its environment, applied where it is called.
		return NewExprValue(n, ec.closure()), nil
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

// EvalDeclaredValue evaluates the value a usage declaration binds, as a read of
// the declaration does: in its own scope, and answering to its declared type.
func (ctx *Context) EvalDeclaredValue(sym *symbols.Symbol) (Value, error) {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Value == nil {
		return Value{}, fmt.Errorf("%w: %s", ErrNoValue, ctx.qualifiedSymbolName(sym))
	}
	defer ctx.beginRun()()

	return NewEvalContext(ctx, sym.OwnerScope).declaredValue(sym, usage)
}

// EvalWithScopeOn evaluates an expression against a concrete instance, so a
// feature it names reads that object's feature value. It brackets one run, as
// EvalWithScope does, which is what bounds the evaluation by the step budget.
func (ctx *Context) EvalWithScopeOn(node ast.Node, scope *symbols.Scope, self *Instance) (Value, error) {
	defer ctx.beginRun()()

	return NewEvalContextIn(ctx, scope, self).Eval(node)
}

// evalLiteralInteger evaluates an integer literal, reporting one outside the
// Integer range rather than clamping it.
func (ec *EvalContext) evalLiteralInteger(n *ast.LiteralInteger) (Value, error) {
	val, ok := ec.ctx.integerLiterals[n]
	if !ok {
		var err error
		if val, err = strconv.ParseInt(n.Value, 10, 64); err != nil {
			return Value{}, fmt.Errorf("%w: literal %s is outside the Integer range",
				semantics.ErrArithmeticOverflow, n.Value)
		}
		ec.ctx.integerLiterals[n] = val
	}
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: val}}, nil
}

// evalLiteralReal evaluates a real literal, reporting one outside the Real
// range rather than carrying it as an infinity.
func (ec *EvalContext) evalLiteralReal(n *ast.LiteralReal) (Value, error) {
	val, ok := ec.ctx.realLiterals[n]
	if !ok {
		var err error
		if val, err = semantics.ParseReal(n.Value); err != nil {
			return Value{}, fmt.Errorf("%w: literal %s is outside the Real range", err, n.Value)
		}
		ec.ctx.realLiterals[n] = val
	}
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: val}}, nil
}

// evalLiteralBool evaluates a boolean literal.
func (ec *EvalContext) evalLiteralBool(n *ast.LiteralBool) (Value, error) {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: n.Value}}, nil
}

// evalLiteralString evaluates a string literal, which spells its text with the
// quotes and escapes of the notation.
func (ec *EvalContext) evalLiteralString(n *ast.LiteralString) (Value, error) {
	return NewStringValue(lexer.StringValue(n.Value)), nil
}

// evalNull evaluates a null expression.
func (ec *EvalContext) evalNull(n *ast.NullExpr) (Value, error) {
	return Value{Kind: ValNull}, nil
}

// evalFeatureReference evaluates a feature reference (variable lookup).
func (ec *EvalContext) evalFeatureReference(n *ast.FeatureReference) (Value, error) {
	if n == nil {
		return Value{}, fmt.Errorf("empty feature reference")
	}
	return ec.evalName(n.Name)
}

// thatName is the implicit feature every usage takes from the base usage: it
// names the instance featuring the value being evaluated ([KerML, 8.4.2]).
const thatName = "that"

// thisName is the context occurrence of what is being evaluated, which for a
// performance an object owns is that object ([KerML] Occurrences::this).
const thisName = "this"

// evalName evaluates a name as a reference to what it names, which is what an
// expression written as a bare name is: `rate`, `A::B::x`.
func (ec *EvalContext) evalName(qn *ast.QualifiedName) (Value, error) {
	// Outside an expression body no body-local declaration can shadow a bound
	// name, so a frame binding is the answer: the common case, kept small.
	if qn != nil && len(qn.Parts) == 1 && (ec.scope == nil || !ec.scope.BodyLocal()) {
		if val, ok := ec.Lookup(qn.Parts[0].Text); ok {
			return val, nil
		}
	}
	return ec.evalNameGeneral(qn)
}

// evalNameGeneral evaluates a name through every source that may answer it, in
// shadowing order.
func (ec *EvalContext) evalNameGeneral(qn *ast.QualifiedName) (Value, error) {
	if qn == nil || len(qn.Parts) == 0 {
		return Value{}, fmt.Errorf("empty feature reference")
	}

	// Simple case: single-part name lookup in frame stack or scope
	if len(qn.Parts) == 1 {
		name := qn.Parts[0].Text
		// A declaration inside an expression body is local to that body and
		// shadows features of the element carrying the expression.
		if ec.scope != nil {
			if sym, ok := symbols.LookupBodyLocal(ec.scope, name); ok {
				// Body parameters and statement locals are supplied by the frame lookup below.
				_, bodyMember := sym.OwnerScope.Node().(*ast.BodyExpr)
				_, param := sym.Decl.(*ast.BodyExpr)
				if bodyMember && !param {
					if ec.resolving[name] {
						return Value{}, fmt.Errorf("%w: %s", ErrCyclicFeatureValue, name)
					}
					if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Value != nil {
						if ec.resolving == nil {
							ec.resolving = map[string]bool{}
						}
						ec.resolving[name] = true
						val, err := ec.declaredValue(sym, usage)
						delete(ec.resolving, name)
						return val, err
					}
					return Value{}, &NoValueError{Feature: name, Ref: qn}
				}
			}
		}
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
		// Then the bound instance: a feature value holds the value this object actually
		// carries, which overrides the declared default the scope would yield.
		if ec.self != nil && ec.selfFeatureInScope(name) {
			if val, ok, err := ec.selfFeatureValue(name); err != nil {
				return Value{}, err
			} else if ok {
				return val, nil
			}
		}
		// Then `that`, which every usage takes from the base usage: it names the
		// instance featuring the value being evaluated, which is the bound one.
		if name == thatName && ec.self != nil {
			return Value{Kind: ValInstance, Instance: ec.self.ID}, nil
		}
		// Then `this`, the context occurrence of what is being evaluated: the
		// object owning the performance, which is the bound instance.
		if name == thisName && ec.namesOccurrenceThis(name) {
			return ec.thisValue()
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
				// A literal of an enumeration is a value of that enumeration.
				if semantics.EnumerationOwning(sym) != nil {
					return ec.enumLiteralValue(sym)
				}
				// A library feature's value comes from the feature seam, not its
				// declared body: a warm library cache restores symbols without AST.
				if val, ok, err := ec.ctx.libraryFeatureValue(sym); ok {
					return val, err
				}
				// A part, item or structured value names an object, so the name
				// evaluates to that object rather than to a value the declaration
				// would have to hold.
				if val, ok, err := ec.occurrenceReference(sym); ok {
					return val, err
				}
				if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Value != nil {
					if ec.resolving == nil {
						ec.resolving = map[string]bool{}
					}
					ec.resolving[name] = true
					val, err := ec.declaredValue(sym, usage)
					delete(ec.resolving, name)
					return val, err
				}
				// A variation holds nothing until it is bound, whether it is read
				// through an object or through its declaration.
				if ec.ctx.model.IsVariationFeature(sym) {
					return Value{}, fmt.Errorf("%w: %s", ErrVariationUnselected, name)
				}
				return Value{}, ec.resolvedWithoutValue(sym, qn)
			}
		}
		// Nothing outside the feature supplies its value, so its own value depends
		// on itself.
		if ec.resolving[name] {
			return Value{}, fmt.Errorf("%w: %s", ErrCyclicFeatureValue, name)
		}
		// A feature the element declares but nothing gives a value to is
		// uninitialized rather than unresolved.
		if declared {
			return Value{}, &NoValueError{Feature: name, Ref: qn}
		}
		return Value{}, fmt.Errorf("%w: %s", ErrUnresolvedReference, name)
	}

	// A multi-part name A::B::x resolves as the checker resolves it — imports,
	// visibility, aliases and inherited members included — so the two agree. It
	// is read in this evaluation's scope, since one expression may be evaluated
	// in several.
	reading := ec.ctx.resolver.ReadQualified(ec.scope, qn)

	// A calc usage's output features are computed rather than declared values,
	// so a name qualified by one reads the rest from an evaluation of the usage.
	for i := 0; i < len(qn.Parts)-1; i++ {
		part, resolved := reading.Part(i)
		if !resolved {
			break
		}
		if isCalcUsageSymbol(part) {
			return ec.evalCalcUsageMembers(part, qn.Parts[i+1:])
		}
	}
	currentSym, ok := reading.Symbol()
	if !ok {
		return Value{}, ec.unresolvedQualifiedName(qn, reading)
	}

	// A library feature reads through the feature seam, whatever the library
	// declares for it and whether or not the cache kept its declaration.
	if val, ok, err := ec.ctx.libraryFeatureValue(currentSym); ok {
		return val, err
	}

	// Evaluate the final symbol's declaration
	if decl, ok := currentSym.Decl.(*ast.Usage); ok {
		// A variant names a choice its variation can be bound to, and compares
		// equal to the variation that selected it.
		if ec.ctx.model.VariationPointOwning(currentSym) != nil {
			return variantReference(currentSym), nil
		}
		// A literal of an enumeration is a value of that enumeration, whether or
		// not it declares one of its own.
		if semantics.EnumerationOwning(currentSym) != nil {
			return ec.enumLiteralValue(currentSym)
		}
		if decl.Value != nil {
			return ec.declaredValue(currentSym, decl)
		}
		if ec.ctx.model.IsVariationFeature(currentSym) {
			return Value{}, fmt.Errorf("%w: %s", ErrVariationUnselected, qualifiedNameToString(qn))
		}
		// A part, item or structured value names an object, read as that object.
		if val, ok, err := ec.occurrenceReference(currentSym); ok {
			return val, err
		}
	}
	return Value{}, ec.resolvedWithoutValue(currentSym, qn)
}

// resolvedWithoutValue reports a name that resolves to sym but reads no value:
// a feature nothing gives a value to is uninitialized, not unresolved.
func (ec *EvalContext) resolvedWithoutValue(sym *symbols.Symbol, qn *ast.QualifiedName) error {
	spelled := qualifiedNameToString(qn)
	// Definitions are types, not values.
	if declaresType(sym) {
		return fmt.Errorf("cannot evaluate definition %s", spelled)
	}
	// A calc usage is an evaluation, not a value: it is read through the output
	// features it computes, since a name it does not designate a result for has
	// no one value.
	if isCalcUsageSymbol(sym) {
		return fmt.Errorf(
			"%w: calc usage %s computes output features (%s); read one of them",
			ErrNoValue, spelled, ec.ctx.calcUsageOutputSummary(sym),
		)
	}
	// A usage of any kind — a subject or a state included — is a feature.
	if _, usage := sym.Decl.(*ast.Usage); usage || isFeature(sym) {
		return &NoValueError{Feature: spelled, Ref: qn}
	}
	return fmt.Errorf("cannot evaluate %s %s", sym.Kind, spelled)
}

// declaresType reports a symbol that declares a type: a definition, or a KerML
// class, struct, behavior, datatype or function, which the parser records as a
// usage and the symbol builder classifies as the type it declares.
func declaresType(sym *symbols.Symbol) bool {
	if isDefinitionSymbol(sym) {
		return true
	}
	switch sym.Kind {
	case symbols.SymbolKerMLType, symbols.SymbolMetaclass, symbols.SymbolAttributeDef, symbols.SymbolCalcDef:
		return true
	default:
		return false
	}
}

// unresolvedQualifiedName reports a multi-part name the resolver rejected: as
// ambiguous when it named several elements; otherwise against the variants or
// literals of a variation or enumeration the deepest resolved segment reached.
func (ec *EvalContext) unresolvedQualifiedName(qn *ast.QualifiedName, reading resolve.Reading) error {
	written := qualifiedNameToString(qn)
	if qn.Global {
		written = "$::" + written
	}
	if n, ok := reading.Ambiguity(); ok {
		return fmt.Errorf("%w: %s (%d candidates)", ErrAmbiguousReference, written, n)
	}
	for i := len(qn.Parts) - 2; i >= 0; i-- {
		owner, ok := reading.Part(i)
		if !ok {
			continue
		}
		memberName := qn.Parts[i+1].Text
		if ec.ctx.model.IsVariationFeature(owner) {
			return fmt.Errorf("%w: %s is not a variant of %s (%s)",
				ErrNotAVariant, memberName, owner.Name, ec.ctx.variantSummary(owner))
		}
		if owner.Kind == symbols.SymbolEnumerationDef {
			return fmt.Errorf("%w: %s is not a literal of %s (%s)",
				ErrNotALiteral, memberName, owner.Name, ec.ctx.enumerationSummary(owner))
		}
		break
	}
	return fmt.Errorf("%w: %s", ErrUnresolvedReference, written)
}

// declaredValue evaluates the value a declaration binds in the scope it was written
// in (its units and imports answer its names); the value answers to the declared type.
func (ec *EvalContext) declaredValue(sym *symbols.Symbol, usage *ast.Usage) (Value, error) {
	val, err := ec.evalIn(sym.OwnerScope).Eval(usage.Value)
	if err != nil {
		return Value{}, err
	}
	what := fmt.Sprintf("feature value %s", ec.ctx.qualifiedSymbolName(sym))
	if err := ec.ctx.checkWriteType(sym.OwnerScope, what, ec.ctx.extractType(sym), val); err != nil {
		return Value{}, err
	}
	return ec.bindVariationOf(sym, val)
}

// occurrenceReference evaluates a name denoting one object — an occurrence or a
// structured value — as that object, materialized once. Reports whether the
// symbol denotes such an object.
func (ec *EvalContext) occurrenceReference(sym *symbols.Symbol) (Value, bool, error) {
	if !ec.ctx.namesOneObject(sym) {
		return Value{}, false, nil
	}
	inst, err := ec.ctx.occurrenceOf(sym)
	if err != nil {
		return Value{}, true, fmt.Errorf("usage %s: %w", symbolText(sym), err)
	}
	return Value{Kind: ValInstance, Instance: inst.ID}, true, nil
}

// namesOccurrenceThis reports whether the name resolves to the library's
// context occurrence feature `this` where the expression was written.
func (ec *EvalContext) namesOccurrenceThis(name string) bool {
	if ec.scope == nil {
		return false
	}
	sym, ok := ec.ctx.resolver.LookupName(ec.scope, name)
	return ok && ec.ctx.resolver.IsOccurrenceThis(sym)
}

// thisValue is the object owning the performance being evaluated. A body no
// object owns has none: `this` there is the performance itself.
func (ec *EvalContext) thisValue() (Value, error) {
	object := ec.ctx.resolver.ThisContext(ec.scope)
	if object == nil {
		return Value{}, fmt.Errorf("%w: this names the performance itself, which no object owns",
			ErrThisNotAnObject)
	}
	if ec.self == nil {
		return Value{}, fmt.Errorf("%w: no object of %s performs this body",
			ErrThisNotAnObject, symbolText(object))
	}
	return Value{Kind: ValInstance, Instance: ec.self.ID}, nil
}

// selfFeatureInScope reports whether the bound instance's feature of that name
// may answer here: in a behavior body only when the name resolves to it.
func (ec *EvalContext) selfFeatureInScope(name string) bool {
	if !ec.inBehaviorBody {
		return true
	}
	return namesPerformerFeature(ec.ctx, ec.self, ec.scope, name)
}

// selfFeatureValue reads the named feature value of the bound instance. Reports whether the
// instance has such a feature value; an error means the feature value exists but could not be
// materialized.
func (ec *EvalContext) selfFeatureValue(name string) (Value, bool, error) {
	if _, ok := ec.self.FeatureValues[name]; !ok {
		return Value{}, false, nil
	}
	fv, err := ec.self.GetFeatureValue(ec.ctx, name)
	if err != nil {
		return Value{}, true, err
	}
	value := fv.HeldValue()
	if value.Kind == ValInvalid {
		return Value{}, true, fmt.Errorf("%w: %s", ErrUninitializedFeatureValue, name)
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
	// from that evaluation rather than from a feature value.
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
		var noValue *NoValueError
		if errors.As(err, &noValue) {
			if unresolved := ec.chainMembersDeclared(base, parts); unresolved != nil {
				return Value{}, unresolved
			}
		}
		return Value{}, err
	}

	if operand.Kind == ValInstance {
		if _, ok := ec.ctx.instances[operand.Instance]; !ok {
			return Value{}, fmt.Errorf("instance ID %d not found", operand.Instance)
		}
	}

	return ec.chainMemberValue(operand, n.Member.Parts, "")
}

// chainMembersDeclared reports the first chain member nothing declares, so
// `wheels.nonexistent` is unresolved rather than unset when wheels has no value.
func (ec *EvalContext) chainMembersDeclared(base ast.Node, parts []ast.NameSegment) error {
	ref, ok := base.(*ast.FeatureReference)
	if !ok || ref.Name == nil || ec.ctx.resolver == nil {
		return nil
	}
	cur, ok := ec.ctx.resolver.ResolveQualified(ec.scope, ref.Name)
	if !ok || cur == nil {
		return nil
	}
	for _, part := range parts {
		next, ok := ec.ctx.declaredMember(cur, part.Text)
		if !ok {
			return fmt.Errorf("%w: %s has no member %s", ErrUnresolvedReference, cur.Name, part.Text)
		}
		cur = next
	}
	return nil
}

// declaredMember is what an object of sym holds under name: a feature of its
// shape, or a member (calc usage, variant) the model reaches by name.
func (ctx *Context) declaredMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	for _, feat := range ctx.FeaturesOf(sym) {
		if feat.Name == name && feat.Symbol != nil {
			return feat.Symbol, true
		}
	}
	return ctx.model.LookupMember(sym, name)
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
		// KerML holds a numerical vector in its `elements` feature, which the
		// runtime represents as the collection itself.
		if parts[0].Text == vectorElementsFeature && isNumericVector(value) {
			return ec.chainMemberValue(value, parts[1:], from)
		}
		return ec.chainOverElements(value, parts, from)
	case ValInstance, ValVariant:
		// handled below
	case ValEnumLiteral:
		// A literal is an occurrence of its enumeration, so its own features are
		// read from the object that literal stands for.
		inst, err := ec.ctx.enumLiteralObject(value.Literal())
		if err != nil {
			return Value{}, err
		}
		return ec.chainMemberValue(Value{Kind: ValInstance, Instance: inst.ID}, parts, from)
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
	fvDecl, ok := inst.FeatureValues[name]
	if !ok {
		// A calc usage is an evaluation rather than a feature value, so its outputs are
		// read from a run of it against this object.
		if sym, found := ec.ctx.model.LookupMember(inst.Type, name); found && isCalcUsageSymbol(sym) {
			return ec.calcUsageMemberValue(sym, inst, parts[1:])
		}
		return Value{}, fmt.Errorf("member %s not found in instance", name)
	}
	// A variant named through the variation feature it belongs to is the choice
	// itself, not a member of the variation's value.
	if variant, rest, ok := ec.variantSegment(fvDecl.Feature, parts[1:]); ok {
		if len(rest) == 0 {
			return variantReference(variant), nil
		}
		// Members are read from the object the variant stands for.
		val, err := ec.ctx.variantValue(fvDecl.Feature.Symbol, variant, inst.ID)
		if err != nil {
			return Value{}, err
		}
		return ec.chainMemberValue(val, rest, variant.Name)
	}
	// Read through GetFeatureValue so a derived or composite member is materialized
	// on demand rather than read as an empty feature value.
	fv, err := inst.GetFeatureValue(ec.ctx, name)
	if err != nil {
		return Value{}, err
	}
	member := fv.HeldValue()
	if member.Kind == ValInvalid {
		return Value{}, fmt.Errorf("%w: %s", ErrUninitializedFeatureValue, name)
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

// enumLiteralValue is the value a literal declares — a scalar-specializing
// enumeration's literal *is* its value — else the identity of the literal.
func (ec *EvalContext) enumLiteralValue(sym *symbols.Symbol) (Value, error) {
	value := semantics.LiteralValue(sym)
	if value == nil {
		return NewEnumLiteral(sym), nil
	}
	val, err := ec.evalIn(declScope(sym)).Eval(value)
	if err != nil {
		return Value{}, fmt.Errorf("enumeration literal %s: %w", sym.Name, err)
	}
	return val, nil
}

// EnumerationLiteralValue is the value sym has when it is an enumeration
// literal, reported as such so a caller holding only a symbol — an `%eval` of a
// literal — answers with the value rather than "no value".
func (ctx *Context) EnumerationLiteralValue(sym *symbols.Symbol) (Value, bool, error) {
	if semantics.EnumerationOwning(sym) == nil {
		return Value{}, false, nil
	}
	val, err := NewEvalContext(ctx, declScope(sym)).enumLiteralValue(sym)
	return val, true, err
}

// enumLiteralObject returns the object a literal stands for, materialized once
// so the features it carries read the same object every time.
func (ctx *Context) enumLiteralObject(literal *symbols.Symbol) (*Instance, error) {
	if literal == nil {
		return nil, fmt.Errorf("%w: the literal was never resolved", ErrNotALiteral)
	}
	inst, err := ctx.occurrenceOf(literal)
	if err != nil {
		return nil, fmt.Errorf("enumeration literal %s: %w", literal.Name, err)
	}
	return inst, nil
}

// enumerationSummary names the literals an enumeration declares, for a report
// about a qualified name that is none of them.
func (ctx *Context) enumerationSummary(enum *symbols.Symbol) string {
	literals := ctx.model.LiteralsOf(enum)
	if len(literals) == 0 {
		return "it declares no literals"
	}
	names := make([]string, 0, len(literals))
	for _, lit := range literals {
		names = append(names, lit.Name)
	}
	return "literals: " + strings.Join(names, ", ")
}

// unimplementedOperators names the operators the runtime does not evaluate and
// says what each would need, so reaching one reports why rather than "unsupported".
var unimplementedOperators = map[ast.OperatorKind]string{
	ast.OpBitNot: "bitwise complement is declared by no function library the runtime applies",
	ast.OpAs:     "a cast needs the runtime type of a value, which values do not carry yet",
	ast.OpMeta:   "metadata access is evaluated from a MetadataAccessExpression, not this operator",
	ast.OpAll:    "'all' needs the extent of a type, which the runtime does not enumerate",
	ast.OpIndex:  "indexing is evaluated from an IndexExpression, not this operator",
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
	case ast.OpAt:
		if ec.classifiesValue(n) {
			return ec.evalTypeClassification(n)
		}
		return ec.evalClassification(n)
	case ast.OpMetaAt:
		return ec.evalClassification(n)
	case ast.OpHasType, ast.OpIsType:
		return ec.evalTypeClassification(n)
	default:
		if why, ok := unimplementedOperators[n.Operator]; ok {
			return Value{}, fmt.Errorf("%w: '%s': %s", ErrUnsupportedOperator, n.Operator, why)
		}
		return Value{}, fmt.Errorf("%w: '%s'", ErrUnsupportedOperator, n.Operator)
	}
}

// classifiesValue reports whether `x @ T` is `x istype T`: a subject is written
// and T is an ordinary type, not a metadata type (which only annotations have).
func (ec *EvalContext) classifiesValue(n *ast.OperatorExpr) bool {
	if len(n.Operands) != 1 || n.TypeRef == nil {
		return false
	}
	target, ok := ec.resolveClassificationType(n.TypeRef)
	return ok && !semantics.IsMetadataType(target)
}

// resolveClassificationType resolves the type a classification names, seeing
// through an alias to the type it stands for.
func (ec *EvalContext) resolveClassificationType(qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	target, ok := ec.ctx.resolver.ResolveQualified(ec.scope, qn)
	if !ok || target == nil {
		return nil, false
	}
	if canonical, ok := ec.ctx.resolver.ResolveAliasTarget(target); ok {
		target = canonical
	}
	return target, true
}

// evalTypeClassification evaluates `x hastype T`, `x istype T` and the value
// form of `x @ T`; only `hastype` demands the value's direct type be T itself.
func (ec *EvalContext) evalTypeClassification(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 1 || n.TypeRef == nil {
		return Value{}, fmt.Errorf("%w: '%s' requires one value and one type",
			ErrTypeMismatch, n.Operator)
	}
	target, ok := ec.resolveClassificationType(n.TypeRef)
	if !ok {
		return Value{}, fmt.Errorf("%w: %s", ErrUnresolvedType,
			qualifiedNameToString(n.TypeRef))
	}
	value, err := ec.evalTypeSubject(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	matches, err := ec.valueHasType(value, target, n.Operator == ast.OpHasType)
	if err != nil {
		return Value{}, err
	}
	return boolValue(matches), nil
}

func (ec *EvalContext) valueHasType(value Value, target *symbols.Symbol, exact bool) (bool, error) {
	switch value.Kind {
	case ValSequence:
		if value.Sequence() == nil || value.Sequence().Size() == 0 {
			return true, nil
		}
		for _, element := range value.Sequence().Elements() {
			matches, err := ec.valueHasType(element, target, exact)
			if err != nil {
				return false, err
			}
			if !matches {
				return false, nil
			}
		}
		return true, nil
	case ValSet:
		if value.Set() == nil || value.Set().Size() == 0 {
			return true, nil
		}
		for _, element := range value.Set().Elements() {
			matches, err := ec.valueHasType(element, target, exact)
			if err != nil {
				return false, err
			}
			if !matches {
				return false, nil
			}
		}
		return true, nil
	}
	direct, err := ec.ctx.directValueType(ec.scope, value)
	if err != nil {
		return false, err
	}
	if exact {
		return direct == target, nil
	}
	return ec.ctx.model.Conforms(direct, target), nil
}

func (ec *EvalContext) evalTypeSubject(node ast.Node) (Value, error) {
	qn := ast.AsQualifiedName(node)
	if qn == nil {
		return ec.Eval(node)
	}
	sym, ok := ec.ctx.resolver.ResolveQualified(ec.scope, qn)
	if !ok || sym == nil {
		return ec.Eval(node)
	}
	if isOccurrenceUsage(sym) {
		inst, err := ec.ctx.occurrenceOf(sym)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: ValInstance, Instance: inst.ID}, nil
	}
	if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Value == nil {
		mult, _ := ec.ctx.extractMultiplicity(sym)
		if !mult.Lower.Infinite && mult.Lower.Value == 0 {
			// Classification treats an optional valueless usage as its empty collection.
			return NewSequenceValue(NewSequence()), nil
		}
	}
	return ec.Eval(node)
}

// directValueType names the type a value is of, resolved in the scope reading
// it: the scalar type of a constant, the type of the object an instance value
// denotes, the enumeration a literal belongs to.
func (ctx *Context) directValueType(scope *symbols.Scope, value Value) (*symbols.Symbol, error) {
	var name string
	switch value.Kind {
	case ValConst:
		switch value.Const.Kind {
		case semantics.ValInt:
			name = "Integer"
		case semantics.ValReal:
			name = "Real"
		case semantics.ValBool:
			name = "Boolean"
		default:
			return nil, fmt.Errorf("%w: %s", ErrUndeterminedValueType, value.Kind)
		}
	case ValString:
		name = "String"
	case ValInstance:
		inst, ok := ctx.instances[value.Instance]
		if !ok || inst == nil || inst.Type == nil {
			return nil, fmt.Errorf("%w: instance %d", ErrUndeterminedValueType, value.Instance)
		}
		if typ := ctx.extractType(inst.Type); typ != nil {
			return typ, nil
		}
		return inst.Type, nil
	case ValVariant:
		if value.Variant() == nil {
			return nil, fmt.Errorf("%w: variant", ErrUndeterminedValueType)
		}
		return value.Variant(), nil
	case ValEnumLiteral:
		if value.Literal() == nil {
			return nil, fmt.Errorf("%w: enumeration literal", ErrUndeterminedValueType)
		}
		enum := semantics.EnumerationOwning(value.Literal())
		if enum == nil {
			return nil, fmt.Errorf("%w: enumeration literal %s",
				ErrUndeterminedValueType, value.Literal().Name)
		}
		return enum, nil
	case ValQuantity:
		if value.Quantity() == nil {
			return nil, fmt.Errorf("%w: quantity", ErrUndeterminedValueType)
		}
		return ctx.directValueType(scope, Value{Kind: ValConst, Const: value.Quantity().Num})
	case ValComplex:
		name = "Complex"
		if re, ok := value.realPart(); ok {
			return ctx.directValueType(scope, realConst(re))
		}
	default:
		return nil, fmt.Errorf("%w: %s", ErrUndeterminedValueType, value.Kind)
	}
	typeSym := ctx.resolveType(scope, name)
	if typeSym == nil {
		return nil, fmt.Errorf("%w: direct type %q", ErrUndeterminedValueType, name)
	}
	return typeSym, nil
}

// evalClassification evaluates `@T` (metadata T annotates the subject) and `@@T`
// (the subject's own metaclass conforms to T) through the semantic model, so the
// verdict is the one an element filter writing the same test reaches.
func (ec *EvalContext) evalClassification(n *ast.OperatorExpr) (Value, error) {
	elem, err := ec.classifiedElement(n)
	if err != nil {
		return Value{}, err
	}
	classified, err := ec.ctx.model.EvalClassification(ec.scope, n, elem)
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: classified}}, nil
}

// classifiedElement is the element a classification's subject denotes, since
// metadata annotates elements and not values: the object being evaluated for an
// implicit subject or `self`, the element a name names, else what its value
// denotes.
func (ec *EvalContext) classifiedElement(n *ast.OperatorExpr) (*symbols.Symbol, error) {
	if len(n.Operands) > 1 {
		return nil, semantics.UnevaluableClassification(
			fmt.Sprintf("`%s` classifies one subject, and %d were given", n.Operator, len(n.Operands)), n.Span())
	}
	if len(n.Operands) == 0 || isSelfName(n.Operands[0]) {
		if ec.self == nil {
			return nil, semantics.UnevaluableClassification(
				fmt.Sprintf("`%s` leaves its subject implicit and no object is being evaluated", n.Operator), n.Span())
		}
		return ec.self.Type, nil
	}
	subject := n.Operands[0]
	// A name is the element it names: what `p @ Safety` classifies is the
	// declaration p, the same element a filter condition would be judged for.
	if qn := subjectName(subject); qn != nil {
		if sym, ok := ec.ctx.resolver.ResolveQualified(ec.scope, qn); ok && sym != nil {
			return sym, nil
		}
	}
	val, err := ec.Eval(subject)
	if err != nil {
		return nil, err
	}
	elem, ok := ec.elementDenotedBy(val)
	if !ok {
		return nil, semantics.UnevaluableClassification(
			fmt.Sprintf("a %s denotes no element to classify", val.Kind), subject.Span())
	}
	return elem, nil
}

// elementDenotedBy is the element a value stands for: the classifier an object
// was materialized from, the variant a variation was bound to, or the literal an
// enumeration value is. Every other value is a datum, which nothing annotates.
func (ec *EvalContext) elementDenotedBy(val Value) (*symbols.Symbol, bool) {
	switch val.Kind {
	case ValInstance:
		inst, ok := ec.ctx.instances[val.Instance]
		if !ok || inst.Type == nil {
			return nil, false
		}
		return inst.Type, true
	case ValVariant:
		return val.Variant(), val.Variant() != nil
	case ValEnumLiteral:
		return val.Literal(), val.Literal() != nil
	default:
		return nil, false
	}
}

// subjectName is the qualified name a classification's subject is written as, or
// nil for a subject that is no name.
func subjectName(n ast.Node) *ast.QualifiedName {
	switch subject := n.(type) {
	case *ast.FeatureReference:
		return subject.Name
	case *ast.QualifiedName:
		return subject
	default:
		return nil
	}
}

// isSelfName reports whether a subject is written as `self`, which names the
// object being evaluated — the same subject the notation leaves out.
func isSelfName(n ast.Node) bool {
	qn := subjectName(n)
	return qn != nil && len(qn.Parts) == 1 && qn.Parts[0].Text == "self"
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

// evalNullCoalesce evaluates `a ?? b`, evaluating b only when a is empty.
func (ec *EvalContext) evalNullCoalesce(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 2 {
		return Value{}, fmt.Errorf("'??' requires 2 operands, got %d", len(n.Operands))
	}
	left, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	return coalesceNull(left, func() (Value, error) { return ec.Eval(n.Operands[1]) })
}

// coalesceNull is `??` over an evaluated first operand: the operand unless it
// is empty, else the second operand, evaluated only then.
func coalesceNull(first Value, second func() (Value, error)) (Value, error) {
	if !isEmptyValue(first) {
		return first, nil
	}
	return second()
}

// isEmptyValue reports whether a value is the empty sequence, which `null`,
// `()` and an empty set all denote.
func isEmptyValue(val Value) bool {
	switch val.Kind {
	case ValNull:
		return true
	case ValSequence, ValSet:
		return len(elementsOf(val)) == 0
	}
	return false
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
	if isEmptyValue(left) || isEmptyValue(right) {
		return isEmptyValue(left) && isEmptyValue(right)
	}
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
	return arithmeticValues(n.Operator, left, right, n.Span())
}

// arithmeticValues applies a binary arithmetic operator to two evaluated
// operands; the operator notation and the library's `'+'` forms both use it.
func arithmeticValues(op ast.OperatorKind, left, right Value, span source.Span) (Value, error) {
	// '+' over two strings concatenates, the one arithmetic operator
	// StringFunctions declares; a non-string operand is not coerced.
	if op == ast.OpAdd && left.Kind == ValString && right.Kind == ValString {
		return concatStrings(left.Str(), right.Str()), nil
	}

	// A quantity carries its unit through arithmetic: a sum converts, a product
	// composes units.
	if lq, rq, ok := quantityOperands(left, right); ok {
		switch op {
		case ast.OpAdd, ast.OpSub:
			return addQuantities(op, lq, rq)
		case ast.OpMul, ast.OpDiv:
			return scaleQuantities(op, lq, rq)
		case ast.OpPow:
			if right.Kind != ValConst {
				return Value{}, fmt.Errorf("%w: exponent of a quantity is a quantity", ErrTypeMismatch)
			}
			return powQuantity(lq, right.Const)
		case ast.OpMod:
			return Value{}, fmt.Errorf("%w: '%%' is not defined for a quantity", ErrTypeMismatch)
		}
	}

	// A complex operand makes the operation ComplexFunctions', the numeric
	// operand beside it being a Complex too.
	if lz, rz, ok := complexOperands(left, right); ok {
		return complexArithmetic(op, lz, rz, left, right, span)
	}

	// Arithmetic is defined on constants; anything else names the operator and
	// both operand types rather than reporting a bare mismatch.
	if left.Kind != ValConst || right.Kind != ValConst {
		return Value{}, &OperandTypeError{
			Op:    op.String(),
			Left:  describeOperand(left),
			Right: describeOperand(right),
			Span:  span,
		}
	}

	res, err := constArithmetic(op, left.Const, right.Const)
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: ValConst, Const: res}, nil
}

// constArithmetic is arithmetic over two scalar constants, the core the
// evaluator and the compiled calc tier share so both report the same results
// and the same errors.
func constArithmetic(op ast.OperatorKind, left, right semantics.Value) (semantics.Value, error) {
	// Exponentiation shares the folder's implementation, so a folded and an
	// evaluated `**` agree; the folder declines where this reports the error.
	if op == ast.OpPow {
		return semantics.Pow(left, right)
	}

	// Integer arithmetic: an out-of-range result is reported, not wrapped.
	if left.Kind == semantics.ValInt && right.Kind == semantics.ValInt {
		// A quotient is a Rational: the exact ratio, rounded once to float64 so
		// operands beyond 2^53 are not rounded before dividing.
		if op == ast.OpDiv {
			q, ok := semantics.IntQuotient(left.Int, right.Int)
			if !ok {
				return semantics.Value{}, ErrDivisionByZero
			}
			return semantics.Value{Kind: semantics.ValReal, Real: q}, nil
		}
		var result int64
		switch op {
		case ast.OpAdd, ast.OpSub, ast.OpMul:
			var ok bool
			if result, ok = semantics.IntArith(op, left.Int, right.Int); !ok {
				return semantics.Value{}, integerOverflow(op, left.Int, right.Int)
			}
		case ast.OpMod:
			if right.Int == 0 {
				return semantics.Value{}, ErrDivisionByZero
			}
			result = left.Int % right.Int
		}
		return semantics.Value{Kind: semantics.ValInt, Int: result}, nil
	}

	// Real arithmetic (coerce int to real if needed)
	leftReal := toReal(left)
	rightReal := toReal(right)
	var result float64
	switch op {
	case ast.OpAdd:
		result = leftReal + rightReal
	case ast.OpSub:
		result = leftReal - rightReal
	case ast.OpMul:
		result = leftReal * rightReal
	case ast.OpDiv:
		// A real quotient by zero is reported, as an integer one, a quantity one
		// and the constant folder all report it, rather than carried as an infinity.
		if rightReal == 0 {
			return semantics.Value{}, ErrDivisionByZero
		}
		result = leftReal / rightReal
	case ast.OpMod:
		if rightReal == 0 {
			return semantics.Value{}, ErrDivisionByZero
		}
		result = math.Mod(leftReal, rightReal)
	}
	// A result that is not a finite Real is reported, not carried as an infinity.
	return realResult(result)
}

// integerOverflow reports an Integer operation whose result leaves the range.
func integerOverflow(op ast.OperatorKind, left, right int64) error {
	return fmt.Errorf("%w: %d %s %d exceeds the Integer range",
		semantics.ErrArithmeticOverflow, left, op.String(), right)
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
	return ec.ctx.equalityValues(n.Operator, left, right)
}

// equalityValues applies `==` or `!=` to two evaluated operands; the operator
// notation and the library's `'=='` forms both use it.
func (ctx *Context) equalityValues(op ast.OperatorKind, left, right Value) (Value, error) {
	// Comparing a value with a variant compares it with the value that variant
	// declares; comparing two variants compares the choice itself.
	if (left.Kind == ValVariant) != (right.Kind == ValVariant) {
		var err error
		if left, err = ctx.variantAsValue(left); err != nil {
			return Value{}, err
		}
		if right, err = ctx.variantAsValue(right); err != nil {
			return Value{}, err
		}
	}

	// Quantities compare in a common unit; incommensurable ones are an error,
	// not an inequality.
	if lq, rq, ok := quantityOperands(left, right); ok {
		return equalQuantities(op, lq, rq)
	}

	equal := valueEqual(left, right)
	if op == ast.OpNeq {
		equal = !equal
	}
	return boolValue(equal), nil
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
	return comparisonValues(n.Operator, left, right, n.Span())
}

// comparisonValues applies an ordering operator to two evaluated operands; the
// operator notation and the library's `'<'` forms both use it.
func comparisonValues(op ast.OperatorKind, left, right Value, span source.Span) (Value, error) {
	// Quantities are ordered in a common unit, so a magnitude is never compared
	// across units without conversion.
	if lq, rq, ok := quantityOperands(left, right); ok {
		return compareQuantities(op, lq, rq)
	}

	// StringFunctions declares the comparisons over two String operands, so a
	// string orders against a string and against nothing else.
	if left.Kind == ValString || right.Kind == ValString {
		if left.Kind != ValString || right.Kind != ValString {
			return Value{}, &OperandTypeError{
				Op:    op.String(),
				Left:  describeOperand(left),
				Right: describeOperand(right),
				Span:  span,
			}
		}
		ordered, err := compareStrings(op, left.Str(), right.Str())
		if err != nil {
			return Value{}, err
		}
		return boolValue(ordered), nil
	}

	// Both must be ValConst
	if left.Kind != ValConst || right.Kind != ValConst {
		return Value{}, fmt.Errorf("comparison operands must be constants, got %s and %s", left.Kind, right.Kind)
	}

	result, err := constComparison(op, left.Const, right.Const)
	if err != nil {
		return Value{}, err
	}
	return boolValue(result), nil
}

// constComparison orders two scalar constants, the core the evaluator and the
// compiled calc tier share.
func constComparison(op ast.OperatorKind, left, right semantics.Value) (bool, error) {
	// Compare integers
	if left.Kind == semantics.ValInt && right.Kind == semantics.ValInt {
		switch op {
		case ast.OpLt:
			return left.Int < right.Int, nil
		case ast.OpLe:
			return left.Int <= right.Int, nil
		case ast.OpGt:
			return left.Int > right.Int, nil
		case ast.OpGe:
			return left.Int >= right.Int, nil
		default:
			return false, fmt.Errorf("unknown comparison operator: %v", op)
		}
	}

	// Compare reals (coerce int to real)
	leftReal := toReal(left)
	rightReal := toReal(right)
	switch op {
	case ast.OpLt:
		return leftReal < rightReal, nil
	case ast.OpLe:
		return leftReal <= rightReal, nil
	case ast.OpGt:
		return leftReal > rightReal, nil
	case ast.OpGe:
		return leftReal >= rightReal, nil
	default:
		return false, fmt.Errorf("unknown comparison operator: %v", op)
	}
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

	if decided, result := shortCircuit(n.Operator, l); decided {
		return boolValue(result), nil
	}

	right, err := ec.Eval(n.Operands[1])
	if err != nil {
		return Value{}, err
	}
	r, err := boolOperand(fmt.Sprintf("right operand of '%s'", n.Operator), right)
	if err != nil {
		return Value{}, err
	}
	return combineBooleans(n.Operator, l, r)
}

// shortCircuit reports whether a Boolean operator is decided by its left
// operand alone, and the result when it is: `and` by false, `or` by true and
// `implies` by false. `xor`, `|` and `&` always read both operands.
func shortCircuit(op ast.OperatorKind, l bool) (decided, result bool) {
	switch op {
	case ast.OpAnd, ast.OpConditionalAnd:
		return !l, false
	case ast.OpOr, ast.OpConditionalOr:
		return l, true
	case ast.OpImplies:
		return !l, true
	}
	return false, false
}

// combineBooleans applies a binary Boolean operator to two Booleans; the
// operator notation and the library's `'xor'` forms both use it.
func combineBooleans(op ast.OperatorKind, l, r bool) (Value, error) {
	switch op {
	case ast.OpAnd, ast.OpConditionalAnd:
		return boolValue(l && r), nil
	case ast.OpOr, ast.OpConditionalOr:
		return boolValue(l || r), nil
	case ast.OpXor:
		return boolValue(l != r), nil
	case ast.OpImplies:
		return boolValue(!l || r), nil
	}
	return Value{}, fmt.Errorf("%w: '%s' is not a Boolean operator", ErrUnsupportedOperator, op)
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

	// The least Integer is the one literal whose magnitude alone is outside the
	// range, so its sign is read together with it; every other operand is
	// evaluated as usual.
	if n.Operator == ast.OpNeg {
		if lit, ok := n.Operands[0].(*ast.LiteralInteger); ok {
			if _, err := strconv.ParseInt(lit.Value, 10, 64); err != nil {
				if val, err := strconv.ParseInt("-"+lit.Value, 10, 64); err == nil {
					return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: val}}, nil
				}
			}
		}
	}

	operand, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	return unaryValue(n.Operator, operand)
}

// unaryValue applies `not`, `-` or `+` to an evaluated operand; the operator
// notation and the library's `'not'` forms both use it.
func unaryValue(op ast.OperatorKind, operand Value) (Value, error) {
	switch op {
	case ast.OpNot:
		if operand.Kind != ValConst {
			return Value{}, fmt.Errorf("%w: logical not requires bool operand, got %v", ErrTypeMismatch, operand.Kind)
		}
	case ast.OpNeg, ast.OpPos:
		if operand.Kind == ValQuantity {
			if op == ast.OpPos {
				return operand, nil
			}
			return negateQuantity(operand.Quantity())
		}
		if operand.Kind == ValComplex {
			if op == ast.OpPos {
				return operand, nil
			}
			return NewComplex(-operand.Complex()), nil
		}
		// Arithmetic sign: -number, +number
		if operand.Kind != ValConst {
			return Value{}, fmt.Errorf("%w: unary '%s' requires numeric operand, got %v", ErrTypeMismatch, op, operand.Kind)
		}
	default:
		return Value{}, fmt.Errorf("%w: '%s' is not a unary operator", ErrUnsupportedOperator, op)
	}
	result, err := constUnary(op, operand.Const)
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: ValConst, Const: result}, nil
}

// constUnary applies `not`, `-` or `+` to a scalar constant, the core the
// evaluator and the compiled calc tier share.
func constUnary(op ast.OperatorKind, operand semantics.Value) (semantics.Value, error) {
	if op == ast.OpNot {
		// Logical not: not bool
		if operand.Kind != semantics.ValBool {
			return semantics.Value{}, fmt.Errorf("%w: logical not requires bool operand, got %s", ErrTypeMismatch, FormatConst(operand))
		}
		return semantics.Value{Kind: semantics.ValBool, Bool: !operand.Bool}, nil
	}
	if op == ast.OpNeg && operand.Kind == semantics.ValInt && operand.Int == math.MinInt64 {
		return semantics.Value{}, fmt.Errorf("%w: -(%d) exceeds the Integer range",
			semantics.ErrArithmeticOverflow, operand.Int)
	}
	result, ok := semantics.EvalUnary(op, operand)
	if !ok {
		return semantics.Value{}, fmt.Errorf("%w: unary '%s' is not defined for %s", ErrTypeMismatch, op, FormatConst(operand))
	}
	return result, nil
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
	fn builtinFunc,
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

// invocationKey identifies one invocation expression in the scope it is
// evaluated in, which is what its written name resolves against.
type invocationKey struct {
	node  *ast.InvocationExpr
	scope *symbols.Scope
}

// invocationTarget is what an invocation expression denotes, resolved once per
// context; at most one implementation is set, in the order they are tried.
type invocationTarget struct {
	qualName    string
	calc        *symbols.Symbol  // the declaration the written name resolves to, nil for none
	builtin     builtinFunc      // the built-in the name denotes: the library declaration calc is
	builtinName string           // the built-in's registered name, keying its declared signature
	library     *libraryFunction // the library function the name denotes: the library declaration calc is
	shape       *calcShape       // calc's invocation interface, nil when it has none
}

// invocationTarget resolves what n denotes in this context's scope, memoized
// per context: resolution reads only the model, which is fixed for its life.
// The name resolves as the validator resolves it, so a library function is
// callable only where the model imports it or writes it qualified, and a
// declaration of the model's own is invoked as written even under a name a
// library built-in is registered by.
func (ec *EvalContext) invocationTarget(n *ast.InvocationExpr) *invocationTarget {
	key := invocationKey{node: n, scope: ec.scope}
	if target, ok := ec.ctx.invocationTargets[key]; ok {
		return target
	}
	target := &invocationTarget{qualName: qualifiedNameToString(n.Type)}
	if sym, ok := ec.ctx.resolver.ResolveQualified(ec.scope, n.Type); ok && sym != nil {
		target.calc = sym
		if fn, ok := ec.ctx.builtinFor(sym); ok {
			target.builtin, target.builtinName = fn, ec.ctx.qualifiedSymbolName(sym)
		} else if fn, ok := ec.ctx.libraryFunctionFor(sym); ok {
			target.library = fn
		} else if shape, err := ec.ctx.calcShapeOf(sym); err == nil {
			target.shape = shape
		}
	}
	ec.ctx.invocationTargets[key] = target
	return target
}

// unresolvedInvocation reports a call to a name that denotes nothing, with the
// same "did you mean" hint the validator gives an unqualified reference.
func (ec *EvalContext) unresolvedInvocation(qn *ast.QualifiedName, written string) error {
	if qn != nil && len(qn.Parts) == 1 && !qn.Global && ec.ctx.resolver != nil {
		return fmt.Errorf("%w: %s", ErrUnresolvedReference, ec.ctx.resolver.UnresolvedName(ec.scope, written))
	}
	return fmt.Errorf("%w: %s", ErrUnresolvedReference, written)
}

// evalInvocation evaluates a function/calc invocation.
func (ec *EvalContext) evalInvocation(n *ast.InvocationExpr) (Value, error) {
	target := ec.invocationTarget(n)
	qualName := target.qualName

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
	// A calc bound by position alone consumes its arguments within the call, so
	// they live on the context's argument stack rather than in a slice of their own.
	if target.shape != nil && len(n.NamedArgs) == 0 {
		return ec.invokeCalcShapeStacked(target.shape, exprs)
	}
	// A built-in binds its arguments by its declared signature.
	if target.builtin != nil {
		return ec.invokeBuiltin(target.builtinName, target.builtin, exprs, n.NamedArgs)
	}

	args := make([]Value, len(exprs))
	for i, arg := range exprs {
		val, err := ec.Eval(arg)
		if err != nil {
			return Value{}, err
		}
		args[i] = val
	}

	var named map[string]Value
	if len(n.NamedArgs) > 0 {
		named = make(map[string]Value, len(n.NamedArgs))
	}
	for _, arg := range n.NamedArgs {
		if arg.Name == nil || len(arg.Name.Parts) == 0 {
			return Value{}, fmt.Errorf("unnamed argument in invocation of %s", qualName)
		}
		name := arg.Name.Parts[len(arg.Name.Parts)-1].Text
		if _, dup := named[name]; dup {
			return Value{}, fmt.Errorf("%w: %s binds parameter %q twice", ErrCalcArity, qualName, name)
		}
		val, err := ec.Eval(arg.Value)
		if err != nil {
			return Value{}, err
		}
		named[name] = val
	}

	// An argument that fails is reported before the target is judged. A name
	// that resolves to nothing denotes nothing, not the library function of
	// that name: the validator reports the same expression unresolved.
	if target.calc == nil && target.library == nil {
		return Value{}, ec.unresolvedInvocation(n.Type, qualName)
	}
	// Every invocation goes through the one calc path, so an expression and a
	// direct InvokeCalc bind parameters and trace identically. The notation keeps
	// the argument forms mutually exclusive.
	callArgs := calcArgs{positional: args}
	if len(named) > 0 {
		callArgs = calcArgs{named: named}
	}
	if target.library != nil {
		return target.library.invoke(ec.ctx, callArgs)
	}
	if target.shape == nil {
		return ec.ctx.invokeCalcWithSelf(target.calc, callArgs, ec.scope, ec.self)
	}
	return ec.ctx.invokeCalcShape(target.shape, callArgs, ec.scope, ec.self)
}

// invokeCalcShapeStacked evaluates exprs onto the context's argument stack and
// invokes shape with them, popping them however the invocation ends.
func (ec *EvalContext) invokeCalcShapeStacked(shape *calcShape, exprs []ast.Node) (Value, error) {
	ctx := ec.ctx
	base := len(ctx.argStack)
	for _, arg := range exprs {
		val, err := ec.Eval(arg)
		if err != nil {
			ctx.popArgs(base)
			return Value{}, err
		}
		ctx.argStack = append(ctx.argStack, val)
	}
	top := len(ctx.argStack)
	args := ctx.argStack[base:top:top]
	result, err := ctx.invokeCalcShape(shape, calcArgs{positional: args}, ec.scope, ec.self)
	ctx.popArgs(base)
	return result, err
}

// popArgs releases the arguments pushed since the stack was base deep.
func (ctx *Context) popArgs(base int) {
	clear(ctx.argStack[base:])
	ctx.argStack = ctx.argStack[:base]
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
	if isEmptyValue(a) || isEmptyValue(b) {
		return isEmptyValue(a) && isEmptyValue(b)
	}
	// A complex number equals the number it is, whichever kind carries it.
	if a.Kind == ValComplex || b.Kind == ValComplex {
		return complexEqual(a, b)
	}
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case ValConst:
		// Delegate to semantics layer for const equality
		result, ok := semantics.EvalBinary(ast.OpEq, a.Const, b.Const)
		return ok && result.Kind == semantics.ValBool && result.Bool
	case ValString:
		return a.Str() == b.Str()
	case ValNull:
		return true
	case ValInstance:
		return a.Instance == b.Instance
	case ValSequence:
		return sequenceEqual(a.Sequence(), b.Sequence())
	case ValSet:
		return setEqual(a.Set(), b.Set())
	case ValVariant:
		// A variation compares equal to the variant it selected.
		return a.Variant() == b.Variant()
	case ValEnumLiteral:
		// A literal is its own identity: two literals are equal exactly when they
		// are the same declaration, across enumerations included.
		return a.Literal() == b.Literal()
	case ValQuantity:
		// Incommensurable units are not equal here: an equality that has to hold
		// or fail (a set member, a sequence element) has no error to report.
		converted, err := b.Quantity().convertTo(a.Quantity().Unit)
		return err == nil && toReal(a.Quantity().Num) == converted
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

// setEqual checks set equality as an unordered multiset of exact values.
func setEqual(a, b *Set) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Size() != b.Size() {
		return false
	}
	used := make([]bool, b.Size())
	rights := b.Elements()
	for _, left := range a.Elements() {
		found := false
		for i, right := range rights {
			if !used[i] && valueEqual(left, right) {
				used[i] = true
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
