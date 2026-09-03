package runtime

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// calcParameter is one input parameter of a calc, in positional order.
type calcParameter struct {
	Name    string          // parameter name arguments bind to
	Default ast.Node        // value-binding expression used when no argument is passed (nil if none)
	Owner   *symbols.Symbol // the calc that declares the default (a supertype for an inherited one)
	Decl    calcMemberDecl  // the declaration, closest to the invoked calc, a bound value answers to
}

// calcMemberDecl is the type and multiplicity a calc's parameter or output
// declares, and the calc whose declaration states them.
type calcMemberDecl struct {
	Target *writeTarget
	Owner  *symbols.Symbol
	// multStated: Target.mult is declared rather than the assumed 1..1.
	multStated bool
}

// redeclaring refines the declaration d redeclares: a redeclaration stating no
// type or multiplicity keeps the redeclared feature's (KerML 1.0 §7.3.4.5).
func (d calcMemberDecl) redeclaring(redeclared calcMemberDecl) calcMemberDecl {
	if d.Target == nil {
		return redeclared
	}
	if redeclared.Target == nil {
		return d
	}
	if d.Target.typ == nil {
		d.Target.typ = redeclared.Target.typ
	}
	if !d.multStated {
		d.Target.mult, d.multStated = redeclared.Target.mult, redeclared.multStated
	}
	return d
}

// check reports a value outside the declared multiplicity or type, described by
// what (asked only on refusal, a binding being the hot path); unknown: not judged.
func (d *calcMemberDecl) check(ctx *Context, value *Value, what func() string) error {
	if d.Target == nil {
		return nil
	}
	if msg := ctx.writeCountRefusal(d.Target, value); msg != "" {
		return fmt.Errorf("%s: %w: %s", what(), ErrMultiplicityViolation, msg)
	}
	if refusal, refused := ctx.writeTypeRefusal(declScope(d.Owner), d.Target.typ, value); refused {
		return fmt.Errorf("%s: %w: %s", what(), ErrTypeMismatch, refusal)
	}
	return nil
}

// calcMemberDeclOf resolves what a member of link declares for a value bound to it.
func (ctx *Context) calcMemberDeclOf(link *symbols.Symbol, usage *ast.Usage, name string) calcMemberDecl {
	sym := memberSymbol(declScope(link), usage)
	if sym == nil {
		return calcMemberDecl{}
	}
	mult, stated := ctx.extractMultiplicity(sym)
	return calcMemberDecl{
		Target:     &writeTarget{name: name, typ: ctx.extractType(sym), mult: mult},
		Owner:      link,
		multStated: stated,
	}
}

// calcShape is a calc's invocation interface: the input parameters it binds, in
// positional order, the output features it computes, and the expression its body
// returns. All are resolved across the specialization chain, so a usage typed by
// a calc definition inherits that definition's parameters, outputs and result.
type calcShape struct {
	Sym    *symbols.Symbol
	Name   string // qualified name, for diagnostics and traces
	Params []calcParameter
	// ParamNames are the Params' names by position: the slots an invocation binds.
	ParamNames []string
	// Outputs are the calc's output features — its `out` parameters and the
	// result parameter a `return` declares — in declaration order.
	Outputs []calcOutput
	// Body is the computation the calc states, lowered in the scope of the calc
	// that declares it: local declarations, assignments, conditionals, loops and
	// returns, in declaration order.
	Body      []lower.Statement
	BodyOwner *symbols.Symbol // the calc whose body declares Body
	// Steps is Body without the bindings of its `out` features, which are
	// evaluated when those features are read rather than run as statements.
	Steps []lower.Statement
	// BodyOutputs are the output features some statement of the body assigns,
	// whatever path the execution takes. A calc that binds its outputs this way
	// computes them though it returns nothing, so it states a computation.
	BodyOutputs map[string]bool
	Bindings    []lower.Binding
	ResultExpr  ast.Node
	// compiled is the body in the compiled tier once compileState says it is
	// eligible; a shape found ineligible keeps the evaluator for good.
	compiled     *compiledCalc
	compileState compileState
	// ineligibleWhy says what kept the body out of the compiled tier.
	ineligibleWhy string
}

// calcShapeOf resolves the invocation interface of a calc symbol: its
// positional input parameters (own and inherited, with defaults) and its result
// expression. The result is memoized per symbol.
func (ctx *Context) calcShapeOf(sym *symbols.Symbol) (*calcShape, error) {
	if sym == nil || sym.Decl == nil {
		return nil, fmt.Errorf("%w: invalid symbol", ErrNotACalc)
	}
	if cached, ok := ctx.calcShapes[sym]; ok {
		return cached, nil
	}

	name := ctx.qualifiedSymbolName(sym)
	if !isCalcDecl(sym.Decl) {
		return nil, fmt.Errorf("%w: %s is %s, not a calc definition or usage", ErrNotACalc, name, describeDecl(sym.Decl))
	}

	// Most general first, so an inherited parameter keeps the position it has in
	// the calc that declares it and a redeclaration refines it in place.
	chain := ctx.calcChain(sym)
	body, bodyOwner := calcBody(chain)
	shape := &calcShape{
		Sym:       sym,
		Name:      name,
		Params:    ctx.calcParameters(chain),
		Outputs:   ctx.calcOutputs(chain),
		Body:      body,
		BodyOwner: bodyOwner,
		Steps:     calcSteps(body),
	}
	shape.ParamNames = make([]string, len(shape.Params))
	for i, param := range shape.Params {
		shape.ParamNames[i] = param.Name
	}
	shape.BodyOutputs = assignedOutputs(shape.Steps, shape.Outputs)
	shape.Bindings = calcBindings(chain)
	shape.ResultExpr = resultBindingExpr(shape.Bindings)
	// A calc computes nothing when it neither returns a value nor binds an output
	// feature — by a declaration or by an assignment in its body.
	if !lower.Returns(shape.Body) && len(shape.BodyOutputs) == 0 && shape.ResultExpr == nil {
		return nil, fmt.Errorf("%w: calc %s has no return expression", ErrNoResultExpression, name)
	}

	ctx.calcShapes[sym] = shape
	return shape, nil
}

func calcBindings(chain []*symbols.Symbol) []lower.Binding {
	var out []lower.Binding
	for _, link := range chain {
		if link != nil {
			out = append(out, lower.ToBindings(link.Decl, declScope(link))...)
		}
	}
	return out
}

func resultBindingExpr(bindings []lower.Binding) ast.Node {
	var result ast.Node
	for _, binding := range bindings {
		for i := range binding.Ends {
			if binding.Ends[i].Path != "result" {
				continue
			}
			other := binding.Ends[1-i]
			if other.Expr != nil {
				result = other.Expr
			}
		}
	}
	return result
}

// calcChain returns sym's calc specialization chain, most general first. Only
// calc links contribute: a calc that specializes a non-calc type inherits no
// parameters or result from it. A library link is the normative frame every calc
// specializes — this runtime implements its parameters rather than inheriting
// them, so it contributes none.
func (ctx *Context) calcChain(sym *symbols.Symbol) []*symbols.Symbol {
	supers := ctx.model.AllSupertypes(sym)
	chain := make([]*symbols.Symbol, 0, len(supers)+1)
	for i := len(supers) - 1; i >= 0; i-- {
		if supers[i] != nil && isCalcDecl(supers[i].Decl) && !ctx.libraryDeclared(supers[i]) {
			chain = append(chain, supers[i])
		}
	}
	return append(chain, sym)
}

// calcParameters flattens the input parameters declared along chain (most
// general first). A parameter redeclared closer to the invoked calc keeps its
// inherited position and its inherited default unless it binds a new one.
func (ctx *Context) calcParameters(chain []*symbols.Symbol) []calcParameter {
	var params []calcParameter
	index := make(map[string]int)

	for _, link := range chain {
		for _, member := range declMembers(link.Decl) {
			usage, ok := member.(*ast.Usage)
			if !ok {
				continue
			}
			// A parameter written as a redefinition names the one it overrides.
			name, _ := ast.EffectiveName(usage)
			if name == "" {
				continue
			}
			if usage.Direction != ast.DirIn && usage.Direction != ast.DirInOut {
				continue
			}
			param := calcParameter{Name: name, Default: usage.Value, Owner: link, Decl: ctx.calcMemberDeclOf(link, usage, name)}
			if at, seen := index[param.Name]; seen {
				// A redeclaration binding no value keeps the inherited default,
				// which is written in the scope of the calc that stated it.
				if param.Default == nil {
					param.Default = params[at].Default
					param.Owner = params[at].Owner
				}
				param.Decl = param.Decl.redeclaring(params[at].Decl)
				params[at] = param
				continue
			}
			index[param.Name] = len(params)
			params = append(params, param)
		}
	}
	return params
}

// calcBody returns the computation the invoked calc runs — its own body if that
// states one, otherwise the closest inherited one — with the calc that declares
// it, whose scope the body's statements are written in.
func calcBody(chain []*symbols.Symbol) ([]lower.Statement, *symbols.Symbol) {
	var stated []lower.Statement
	var owner *symbols.Symbol
	for i := len(chain) - 1; i >= 0; i-- {
		link := chain[i]
		stmts := lower.CalcBody(declMembers(link.Decl), link.Scope)
		if lower.Returns(stmts) {
			return stmts, link
		}
		// A body that computes but returns nothing leaves an inherited result in
		// force, so keep looking up the chain before settling for it.
		if stated == nil && len(stmts) > 0 {
			stated, owner = stmts, link
		}
	}
	return stated, owner
}

// calcArgs are the arguments of one calc invocation. The notation keeps the two
// forms mutually exclusive: positional arguments bind in parameter order, named
// arguments by parameter name.
type calcArgs struct {
	positional []Value
	named      map[string]Value
}

// InvokeCalc invokes a calculation with the given positional arguments and
// returns its result. Arguments bind to the calc's input parameters in
// declaration order; a parameter with no argument falls back to its declared
// default. The body is evaluated in the calc's own scope, so scope is used only
// as a fallback for a symbol that owns no scope. A library function's `expr`
// parameter takes a body or expression value, applied only when selected, or
// the operand's value itself.
func (ctx *Context) InvokeCalc(sym *symbols.Symbol, args []Value, scope *symbols.Scope) (Value, error) {
	defer ctx.beginRun()()

	return ctx.invokeCalc(sym, calcArgs{positional: args}, scope)
}

// InvokeCalcNamed invokes a calculation with arguments bound by parameter name.
// A parameter with no argument falls back to its declared default.
func (ctx *Context) InvokeCalcNamed(sym *symbols.Symbol, args map[string]Value, scope *symbols.Scope) (Value, error) {
	defer ctx.beginRun()()

	return ctx.invokeCalc(sym, calcArgs{named: args}, scope)
}

func (ctx *Context) invokeCalcNamedShapeOn(shape *calcShape, args map[string]Value, scope *symbols.Scope, self *Instance) (Value, error) {
	return ctx.invokeCalcShape(shape, calcArgs{named: args}, scope, self)
}

// invokeCalc resolves the calc's shape and invokes it, the single path every
// calc invocation takes. A function library declaration the library gives no
// body is applied by its built-in implementation instead.
func (ctx *Context) invokeCalc(sym *symbols.Symbol, args calcArgs, scope *symbols.Scope) (Value, error) {
	return ctx.invokeCalcWithSelf(sym, args, scope, nil)
}

func (ctx *Context) invokeCalcWithSelf(sym *symbols.Symbol, args calcArgs, scope *symbols.Scope, self *Instance) (Value, error) {
	if fn, ok := ctx.builtinFor(sym); ok {
		return ctx.invokeBuiltinValues(sym, fn, args, scope, self)
	}
	if fn, ok := ctx.libraryFunctionFor(sym); ok {
		return fn.invoke(ctx, args)
	}

	shape, err := ctx.calcShapeOf(sym)
	if err != nil {
		return Value{}, err
	}
	return ctx.invokeCalcShape(shape, args, scope, self)
}

// invokeBuiltinValues applies a built-in to arguments a direct invocation has
// already evaluated, bound to its declared parameters as a call would bind them.
func (ctx *Context) invokeBuiltinValues(sym *symbols.Symbol, fn builtinFunc, args calcArgs, callerScope *symbols.Scope, self *Instance) (Value, error) {
	bound, err := bindBuiltinValues(ctx.qualifiedSymbolName(sym), args)
	if err != nil {
		return Value{}, err
	}
	return fn(NewEvalContextIn(ctx, ctx.calcScope(sym, nil, callerScope), self), bound)
}

// invocationFrame is the storage one calc invocation runs in, held off the
// context's free list until the invocation returns, so no two active ones share it.
type invocationFrame struct {
	ec     EvalContext
	frames [1]frame
	// slots hold the parameters; bindings the locals the body declares beside them.
	slots    slotFrame
	bindings map[string]Value
	host     calcStmtHost
	env      stmtEnv
	engine   stmtEngine
}

// locals is the frame the invocation's parameters and body locals are bound in.
func (f *invocationFrame) locals() frame {
	return frame{slots: &f.slots, vars: f.bindings}
}

// maxFreeInvocationFrames bounds the frames kept, so one deep recursion does not
// pin that many for the context's whole life.
const maxFreeInvocationFrames = 1024

// maxPooledBindings is the widest frame whose storage a free frame keeps: clearing
// keeps the backing storage, so a wider one is dropped rather than pinned.
const maxPooledBindings = 32

// acquireInvocationFrame takes a frame off the free list, or makes one when it is empty.
func (ctx *Context) acquireInvocationFrame() *invocationFrame {
	if n := len(ctx.freeInvocationFrames); n > 0 {
		frame := ctx.freeInvocationFrames[n-1]
		ctx.freeInvocationFrames[n-1] = nil
		ctx.freeInvocationFrames = ctx.freeInvocationFrames[:n-1]
		if frame.bindings == nil {
			frame.bindings = make(map[string]Value)
		}
		return frame
	}
	return &invocationFrame{bindings: make(map[string]Value)}
}

// releaseInvocationFrame empties frame and keeps it for the next invocation; the
// caller has ended the activation, so nothing memoized still reads the bindings.
func (ctx *Context) releaseInvocationFrame(frame *invocationFrame) {
	bindings, slots := frame.bindings, frame.slots
	if frame.locals().width() > maxPooledBindings {
		bindings, slots = nil, slotFrame{}
	} else {
		clear(bindings)
		slots.release()
	}
	frameBuf := frame.engine.frameBuf
	clear(frameBuf)
	*frame = invocationFrame{bindings: bindings, slots: slots}
	frame.engine.frameBuf = frameBuf[:0]
	if len(ctx.freeInvocationFrames) < maxFreeInvocationFrames {
		ctx.freeInvocationFrames = append(ctx.freeInvocationFrames, frame)
	}
}

// invokeCalcShape binds arguments and evaluates the calc body. Binding happens
// in the calc's own environment: defaults and the result expression see the
// parameters and the calc's lexical scope, never the caller's frames.
func (ctx *Context) invokeCalcShape(shape *calcShape, args calcArgs, callerScope *symbols.Scope, self *Instance) (Value, error) {
	if err := shape.checkArgs(args); err != nil {
		return Value{}, err
	}

	// A pure body runs compiled unless the run is traced, which records every
	// sub-expression, an argument is not a scalar, or a bound object may answer
	// a library constant the body reads before the library does.
	if ctx.compileCalcs && ctx.trace == nil {
		if compiled := ctx.compiledCalcOf(shape); compiled != nil && (self == nil || !compiled.readsLibrary) {
			if result, ran, err := compiled.invokeBoxed(ctx, args); ran {
				return result, err
			}
		}
	}

	if err := ctx.enterCalc(shape.Name); err != nil {
		return Value{}, err
	}
	defer ctx.leaveCalc()

	frame := ctx.acquireInvocationFrame()
	defer ctx.releaseInvocationFrame(frame)

	// The invocation is one activation, defaults included, so a calc usage read while
	// binding answers this invocation alone and ends with it, before the frame is reused.
	activation := ctx.newActivation()
	defer ctx.endActivation(activation)

	frame.slots.reset(shape.ParamNames)
	locals := frame.locals()
	ec := &frame.ec
	*ec = EvalContext{
		ctx:        ctx,
		scope:      ctx.calcScope(shape.BodyOwner, shape.Sym, callerScope),
		self:       self,
		frames:     append(frame.frames[:0], locals),
		trace:      ctx.trace,
		activation: activation,
	}

	if ec.trace != nil {
		ec.trace.RecordCalcEnter(shape.Name)
	}

	if err := ctx.bindCalcParameters(shape, ec, args, callerScope, locals, nil); err != nil {
		if ec.trace != nil {
			ec.trace.RecordCalcExitError(shape.Name, err)
		}
		return Value{}, err
	}

	result, err := ctx.runCalcBody(shape, frame, callerScope, self, activation)
	if ec.trace != nil {
		if err != nil {
			ec.trace.RecordCalcExitError(shape.Name, err)
		} else {
			ec.trace.RecordCalcExit(shape.Name, result)
		}
	}
	if err != nil {
		return Value{}, calcFrame(shape.Name, err)
	}
	return result, nil
}

// enterCalc spends one of the run's calc depth budget, so a recursion evaluates
// while it terminates within the budget. leaveCalc takes it back off.
func (ctx *Context) enterCalc(name string) error {
	if int64(ctx.calcDepth) >= ctx.maxCalcDepth {
		return ctx.calcDepthExceeded(name)
	}
	ctx.calcDepth++
	return nil
}

// calcDepthExceeded reports the calc depth budget spent; kept out of line so
// enterCalc inlines into every invocation.
//
//go:noinline
func (ctx *Context) calcDepthExceeded(name string) error {
	return fmt.Errorf(
		"%w: calc %s nested %d deep (unbounded recursion?; raise %s to allow more)",
		ErrCalcRecursionLimit, name, ctx.maxCalcDepth, MaxCalcDepthEnvVar,
	)
}

// leaveCalc returns the calc depth enterCalc spent.
func (ctx *Context) leaveCalc() {
	ctx.calcDepth--
}

// bindCalcParameters binds the calc's input parameters into bindings: each
// parameter's argument, or, for a parameter no argument supplies, the default
// declared closest to the invoked calc, evaluated in the scope of the calc
// declaring it. ec supplies the environment defaults are evaluated in; nested,
// when non-null, is the environment reading a nested usage, which the bindings
// the usage itself declares are written in and evaluate against.
func (ctx *Context) bindCalcParameters(
	shape *calcShape,
	ec *EvalContext,
	args calcArgs,
	callerScope *symbols.Scope,
	bindings frame,
	nested *EvalContext,
) error {
	for i := range shape.Params {
		param := &shape.Params[i]
		defaultScope := ctx.calcScope(param.Owner, shape.Sym, callerScope)
		binder := ec
		if nested != nil && param.Owner == shape.Sym {
			binder = nested
		}
		value, source, err := binder.evalIn(defaultScope).bindCalcParameter(shape, param, args, i)
		if err != nil {
			return err
		}
		// The parameter holds the value bound to it, so that value answers to the
		// parameter's declaration as a written one does.
		if err := param.Decl.check(ctx, &value, func() string {
			return fmt.Sprintf("calc %s: %s for parameter %q", shape.Name, source, param.Name)
		}); err != nil {
			return err
		}
		bindings.bindParam(i, param.Name, value)
		if ec.trace != nil {
			ec.trace.RecordCalcBind(param.Name, value, source)
		}
	}
	return nil
}

// runCalcBody runs the calc's computation in the invocation's frame — its
// bindings hold the parameters and the locals — and answers with the one value
// the invocation yields: what the body returned, or, for a body that returns
// nothing, the calc's designated output feature, evaluated in the invocation's
// activation, which the caller ends after it.
func (ctx *Context) runCalcBody(shape *calcShape, frame *invocationFrame, callerScope *symbols.Scope, self *Instance, activation int64) (Value, error) {
	frame.host = calcStmtHost{ctx: ctx, shape: shape, self: self}
	frame.env = stmtEnv{data: frame.locals()}
	frame.engine = stmtEngine{ctx: ctx, host: &frame.host, env: &frame.env, activation: activation, frameBuf: frame.engine.frameBuf}
	result, returned, err := runCalcSteps(&frame.engine, &frame.host, shape)
	if err != nil {
		return Value{}, err
	}
	if returned {
		return result, nil
	}
	out, err := shape.designatedOutput()
	if err != nil {
		return Value{}, err
	}
	// The designated output's binding may name the calc's other outputs, and one
	// naming itself is a cycle rather than an evaluation, so it is evaluated
	// through the same run bookkeeping a calc usage's outputs use.
	run := newCalcRun(shape, callerScope, self, frame.locals())
	run.activation = activation
	// The invocation already holds this evaluation's nesting feature value.
	run.onStack = true
	return run.value(ctx, out)
}

// runCalcSteps runs the calc's lowered computation on engine, whose data holds
// the calc's parameters on the way in and its locals on the way out, reporting
// the value host took from a `return` and whether the body returned one.
func runCalcSteps(engine *stmtEngine, host *calcStmtHost, shape *calcShape) (Value, bool, error) {
	flow, err := engine.run(shape.Steps)
	if err != nil {
		return Value{}, false, err
	}
	return host.result, flow == flowReturn, nil
}

// checkArgs rejects an argument list that cannot bind to the parameters at all:
// more positional arguments than parameters, or a name that is not one.
func (shape *calcShape) checkArgs(args calcArgs) error {
	if len(args.positional) > len(shape.Params) {
		return fmt.Errorf(
			"%w: calc %s takes %d argument(s), got %d",
			ErrCalcArity, shape.Name, len(shape.Params), len(args.positional),
		)
	}

	// Sorted, so the reported name does not depend on map iteration order.
	names := make([]string, 0, len(args.named))
	for name := range args.named {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if !shape.hasParameter(name) {
			return fmt.Errorf(
				"%w: calc %s has no input parameter %q",
				ErrUnknownParameter, shape.Name, name,
			)
		}
	}
	return nil
}

// optional reports whether the parameter may go without an argument: its
// declared multiplicity admits no value, as `[0..1]` does.
func (param *calcParameter) optional() bool {
	return param.Decl.Target != nil && param.Decl.multStated && param.Decl.Target.mult.AllowsNone()
}

// hasParameter reports whether the calc declares an input parameter of that name.
func (shape *calcShape) hasParameter(name string) bool {
	for _, param := range shape.Params {
		if param.Name == name {
			return true
		}
	}
	return false
}

// bindCalcParameter resolves the value of one parameter: its argument (by
// position or by name), else its declared default, else null for one whose
// multiplicity admits no value. A parameter with none of these is unbound,
// which is a modeling error rather than a null value.
func (ec *EvalContext) bindCalcParameter(
	shape *calcShape,
	param *calcParameter,
	args calcArgs,
	position int,
) (Value, string, error) {
	if position < len(args.positional) {
		return args.positional[position], "argument", nil
	}
	if value, ok := args.named[param.Name]; ok {
		return value, "argument", nil
	}
	if param.Default == nil {
		if param.optional() {
			return nullValue(), "omitted", nil
		}
		return Value{}, "", fmt.Errorf(
			"%w: calc %s parameter %q has no argument and no default",
			ErrUnboundParameter, shape.Name, param.Name,
		)
	}
	value, err := ec.Eval(param.Default)
	if err != nil {
		return Value{}, "", fmt.Errorf(
			"calc %s: default for parameter %q: %w", shape.Name, param.Name, err,
		)
	}
	return value, "default", nil
}

// calcScope returns the scope an inherited or own calc body member is written
// in: the declaring calc's own scope, falling back to the invoked calc's and
// then to the caller's for a declaration that owns no scope.
func (ctx *Context) calcScope(declarer, invoked *symbols.Symbol, callerScope *symbols.Scope) *symbols.Scope {
	for _, sym := range []*symbols.Symbol{declarer, invoked} {
		if sym != nil && sym.Scope != nil {
			return sym.Scope
		}
	}
	return callerScope
}

// isCalcDecl reports whether a declaration is a calc definition or calc usage.
func isCalcDecl(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefCalc
	case *ast.Usage:
		return d.Kind == ast.UsageCalc
	default:
		return false
	}
}

// isCalcSymbol reports whether sym declares a calc, reading its declaration when
// it has one and its symbol kind otherwise.
func isCalcSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.Decl != nil {
		return isCalcDecl(sym.Decl)
	}
	return sym.Kind == symbols.SymbolCalcDef || sym.Kind == symbols.SymbolCalcUsage
}

// isActionSymbol reports whether sym declares an action, reading its declaration
// when it has one and its symbol kind otherwise.
func isActionSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefAction
	case *ast.Usage:
		return d.Kind == ast.UsageAction
	}
	if sym.Decl != nil {
		return false
	}
	return sym.Kind == symbols.SymbolActionDef || sym.Kind == symbols.SymbolActionUsage
}

// isStateSymbol reports whether sym declares a state, reading its declaration when
// it has one and its symbol kind otherwise.
func isStateSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefState
	case *ast.Usage:
		return d.Kind == ast.UsageState
	}
	if sym.Decl != nil {
		return false
	}
	return sym.Kind == symbols.SymbolStateDef || sym.Kind == symbols.SymbolStateUsage
}

// declMembers returns the body members of a definition or usage, unwrapping the
// Membership wrappers the parser produces.
func declMembers(decl ast.Node) []ast.Node {
	var members []ast.Node
	switch d := decl.(type) {
	case *ast.Definition:
		members = d.Members
	case *ast.Usage:
		members = d.Members
	default:
		return nil
	}

	out := make([]ast.Node, 0, len(members))
	for _, member := range members {
		if membership, ok := member.(*ast.Membership); ok {
			member = membership.Member
		}
		out = append(out, member)
	}
	return out
}

// qualifiedSymbolName renders a symbol's qualified name, falling back to its
// local name when no index is reachable.
func (ctx *Context) qualifiedSymbolName(sym *symbols.Symbol) string {
	if sym == nil {
		return ""
	}
	if ctx.resolver != nil {
		if idx := ctx.resolver.Index(); idx != nil {
			if fqn := idx.GetFQN(sym); fqn != "" {
				return fqn
			}
		}
	}
	return sym.Name
}
