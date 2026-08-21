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
}

// calcShape is a calc's invocation interface: the input parameters it binds, in
// positional order, the output features it computes, and the expression its body
// returns. All are resolved across the specialization chain, so a usage typed by
// a calc definition inherits that definition's parameters, outputs and result.
type calcShape struct {
	Sym    *symbols.Symbol
	Name   string // qualified name, for diagnostics and traces
	Params []calcParameter
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
		Params:    calcParameters(chain),
		Outputs:   calcOutputs(chain),
		Body:      body,
		BodyOwner: bodyOwner,
		Steps:     calcSteps(body),
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
// parameters or result from it.
func (ctx *Context) calcChain(sym *symbols.Symbol) []*symbols.Symbol {
	supers := ctx.model.AllSupertypes(sym)
	chain := make([]*symbols.Symbol, 0, len(supers)+1)
	for i := len(supers) - 1; i >= 0; i-- {
		if supers[i] != nil && isCalcDecl(supers[i].Decl) {
			chain = append(chain, supers[i])
		}
	}
	return append(chain, sym)
}

// calcParameters flattens the input parameters declared along chain (most
// general first). A parameter redeclared closer to the invoked calc keeps its
// inherited position and its inherited default unless it binds a new one.
func calcParameters(chain []*symbols.Symbol) []calcParameter {
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
			param := calcParameter{Name: name, Default: usage.Value, Owner: link}
			if at, seen := index[param.Name]; seen {
				// A redeclaration binding no value keeps the inherited default,
				// which is written in the scope of the calc that stated it.
				if param.Default == nil {
					param.Default = params[at].Default
					param.Owner = params[at].Owner
				}
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
// as a fallback for a symbol that owns no scope.
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

func (ctx *Context) invokeCalcNamedOn(sym *symbols.Symbol, args map[string]Value, scope *symbols.Scope, self *Instance) (Value, error) {
	defer ctx.beginRun()()

	return ctx.invokeCalcWithSelf(sym, calcArgs{named: args}, scope, self)
}

// invokeCalc resolves the calc's shape and invokes it, the single path every
// calc invocation takes. A function library declaration the library gives no
// body is applied by its built-in implementation instead.
func (ctx *Context) invokeCalc(sym *symbols.Symbol, args calcArgs, scope *symbols.Scope) (Value, error) {
	return ctx.invokeCalcWithSelf(sym, args, scope, nil)
}

func (ctx *Context) invokeCalcWithSelf(sym *symbols.Symbol, args calcArgs, scope *symbols.Scope, self *Instance) (Value, error) {
	if fn, ok := ctx.libraryFunctionFor(sym); ok {
		return fn.invoke(ctx, args)
	}

	shape, err := ctx.calcShapeOf(sym)
	if err != nil {
		return Value{}, err
	}
	return ctx.invokeCalcShape(shape, args, scope, self)
}

// invokeCalcShape binds arguments and evaluates the calc body. Binding happens
// in the calc's own environment: defaults and the result expression see the
// parameters and the calc's lexical scope, never the caller's frames.
func (ctx *Context) invokeCalcShape(shape *calcShape, args calcArgs, callerScope *symbols.Scope, self *Instance) (Value, error) {
	if err := shape.checkArgs(args); err != nil {
		return Value{}, err
	}

	leave, err := ctx.enterCalc(shape.Name)
	if err != nil {
		return Value{}, err
	}
	defer leave()

	ec := NewEvalContextIn(ctx, ctx.calcScope(shape.BodyOwner, shape.Sym, callerScope), self)

	if ec.trace != nil {
		ec.trace.RecordCalcEnter(shape.Name)
	}

	bindings := make(map[string]Value, len(shape.Params))
	ec.Push(bindings)

	if err := ctx.bindCalcParameters(shape, ec, args, callerScope, bindings, nil); err != nil {
		if ec.trace != nil {
			ec.trace.RecordCalcExitError(shape.Name, err)
		}
		return Value{}, err
	}

	result, err := ctx.runCalcBody(shape, bindings, callerScope, self)
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
// while it terminates within the budget. The returned function takes it back off.
func (ctx *Context) enterCalc(name string) (func(), error) {
	if int64(ctx.calcDepth) >= ctx.maxCalcDepth {
		return nil, fmt.Errorf(
			"%w: calc %s nested %d deep (unbounded recursion?; raise %s to allow more)",
			ErrCalcRecursionLimit, name, ctx.maxCalcDepth, MaxCalcDepthEnvVar,
		)
	}
	ctx.calcDepth++
	return func() { ctx.calcDepth-- }, nil
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
	bindings map[string]Value,
	nested *EvalContext,
) error {
	for i, param := range shape.Params {
		defaultScope := ctx.calcScope(param.Owner, shape.Sym, callerScope)
		binder := ec
		if nested != nil && param.Owner == shape.Sym {
			binder = nested
		}
		value, source, err := binder.evalIn(defaultScope).bindCalcParameter(shape, param, args, i)
		if err != nil {
			return err
		}
		bindings[param.Name] = value
		if ec.trace != nil {
			ec.trace.RecordCalcBind(param.Name, value, source)
		}
	}
	return nil
}

// runCalcBody runs the calc's computation in an environment of its own —
// bindings holds its parameters and its locals — and answers with the one value
// the invocation yields: what the body returned, or, for a body that returns
// nothing, the calc's designated output feature.
func (ctx *Context) runCalcBody(shape *calcShape, bindings map[string]Value, callerScope *symbols.Scope, self *Instance) (Value, error) {
	result, returned, activation, err := ctx.runCalcSteps(shape, bindings, self)
	// The invocation's activation ends with the value it yields, which is the last
	// thing evaluated in it.
	defer ctx.endActivation(activation)
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
	run := newCalcRun(shape, callerScope, self, bindings)
	run.activation = activation
	// The invocation already holds this evaluation's nesting feature value.
	run.onStack = true
	return run.value(ctx, out)
}

// runCalcSteps runs the calc's lowered computation, reporting whether it
// returned a value and the activation it ran in. bindings holds the calc's
// parameters on the way in and its locals on the way out.
func (ctx *Context) runCalcSteps(shape *calcShape, bindings map[string]Value, self *Instance) (Value, bool, int64, error) {
	host := &calcStmtHost{shape: shape, self: self}
	engine := newStmtEngine(ctx, host, bindings)

	flow, err := engine.run(shape.Steps)
	if err != nil {
		return Value{}, false, engine.activation, err
	}
	return host.result, flow == flowReturn, engine.activation, nil
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
// position or by name), else its declared default. A parameter with neither is
// unbound, which is a modeling error rather than a null value.
func (ec *EvalContext) bindCalcParameter(
	shape *calcShape,
	param calcParameter,
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

// hasCalcBody reports whether sym's calc chain states a computation of its own:
// a result expression, or a body assigning an output it declares. A library
// function declared without one — or a symbol loaded from the library index,
// which carries no declaration at all — has no body.
func (ctx *Context) hasCalcBody(sym *symbols.Symbol) bool {
	if sym == nil || sym.Decl == nil || !isCalcDecl(sym.Decl) {
		return false
	}
	chain := ctx.calcChain(sym)
	body, _ := calcBody(chain)
	if lower.Returns(body) {
		return true
	}
	return len(assignedOutputs(calcSteps(body), calcOutputs(chain))) > 0
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
// it has one and its symbol kind otherwise, as a cached library symbol does.
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
