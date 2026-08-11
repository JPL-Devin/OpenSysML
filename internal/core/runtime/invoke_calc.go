package runtime

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// maxCalcNestingDepth bounds how deep calc-in-calc invocation may go. A calc
// that reaches itself, directly or through a cycle, would otherwise recurse
// until the process ran out of stack rather than reporting the model error.
const maxCalcNestingDepth = 32

// calcParameter is one input parameter of a calc, in positional order.
type calcParameter struct {
	Name    string          // parameter name arguments bind to
	Default ast.Node        // value-binding expression used when no argument is passed (nil if none)
	Owner   *symbols.Symbol // the calc that declares the default (a supertype for an inherited one)
}

// calcShape is a calc's invocation interface: the input parameters it binds, in
// positional order, and the expression its body returns. Both are resolved
// across the specialization chain, so a usage typed by a calc definition
// inherits that definition's parameters and result.
type calcShape struct {
	Sym         *symbols.Symbol
	Name        string // qualified name, for diagnostics and traces
	Params      []calcParameter
	Result      ast.Node
	ResultOwner *symbols.Symbol // the calc whose body declares Result
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
		return nil, fmt.Errorf("%w: %s is not a calc definition or usage (%T)", ErrNotACalc, name, sym.Decl)
	}

	// Most general first, so an inherited parameter keeps the position it has in
	// the calc that declares it and a redeclaration refines it in place.
	chain := ctx.calcChain(sym)
	result, resultOwner := calcResult(chain)
	shape := &calcShape{
		Sym:         sym,
		Name:        name,
		Params:      calcParameters(chain),
		Result:      result,
		ResultOwner: resultOwner,
	}
	if shape.Result == nil {
		return nil, fmt.Errorf("%w: calc %s has no return expression", ErrNoResultExpression, name)
	}

	ctx.calcShapes[sym] = shape
	return shape, nil
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
			if !ok || usage.Ident.Name == "" {
				continue
			}
			if usage.Direction != ast.DirIn && usage.Direction != ast.DirInOut {
				continue
			}
			param := calcParameter{Name: usage.Ident.Name, Default: usage.Value, Owner: link}
			if at, seen := index[param.Name]; seen {
				if param.Default == nil {
					param.Default = params[at].Default
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

// calcResult returns the result expression the invoked calc evaluates — its own
// if it declares one, otherwise the closest inherited one — with the calc that
// declares it, whose scope the expression is written in.
func calcResult(chain []*symbols.Symbol) (ast.Node, *symbols.Symbol) {
	for i := len(chain) - 1; i >= 0; i-- {
		for _, member := range declMembers(chain[i].Decl) {
			if expr := resultExpression(member); expr != nil {
				return expr, chain[i]
			}
		}
	}
	return nil, nil
}

// resultExpression returns the value a calc body member returns, for either
// notation: `return <expr>;` and a bound return parameter `return : T = <expr>;`.
// A return parameter that binds no value only names the result, so it carries no
// expression and leaves the inherited one in force.
func resultExpression(member ast.Node) ast.Node {
	switch m := member.(type) {
	case *ast.ResultMember:
		return m.Expression
	case *ast.Usage:
		if m.Direction == ast.DirOut {
			return m.Value
		}
	}
	return nil
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
	return ctx.invokeCalc(sym, calcArgs{positional: args}, scope)
}

// InvokeCalcNamed invokes a calculation with arguments bound by parameter name.
// A parameter with no argument falls back to its declared default.
func (ctx *Context) InvokeCalcNamed(sym *symbols.Symbol, args map[string]Value, scope *symbols.Scope) (Value, error) {
	return ctx.invokeCalc(sym, calcArgs{named: args}, scope)
}

// invokeCalc resolves the calc's shape and invokes it, the single path every
// calc invocation takes. A function library declaration the library gives no
// body is applied by its built-in implementation instead.
func (ctx *Context) invokeCalc(sym *symbols.Symbol, args calcArgs, scope *symbols.Scope) (Value, error) {
	if fn, ok := ctx.libraryFunctionFor(sym); ok {
		return fn.invoke(ctx, args)
	}

	shape, err := ctx.calcShapeOf(sym)
	if err != nil {
		return Value{}, err
	}
	return ctx.invokeCalcShape(shape, args, scope)
}

// invokeCalcShape binds arguments and evaluates the calc body. Binding happens
// in the calc's own environment: defaults and the result expression see the
// parameters and the calc's lexical scope, never the caller's frames.
func (ctx *Context) invokeCalcShape(shape *calcShape, args calcArgs, callerScope *symbols.Scope) (Value, error) {
	if err := shape.checkArgs(args); err != nil {
		return Value{}, err
	}

	if ctx.calcDepth >= maxCalcNestingDepth {
		return Value{}, fmt.Errorf(
			"%w: calc %s nested more than %d deep (recursive calc?)",
			ErrCalcRecursionLimit, shape.Name, maxCalcNestingDepth,
		)
	}
	ctx.calcDepth++
	defer func() { ctx.calcDepth-- }()

	ec := NewEvalContext(ctx, ctx.calcScope(shape.ResultOwner, shape.Sym, callerScope))

	if ec.trace != nil {
		ec.trace.RecordCalcEnter(shape.Name)
	}

	bindings := make(map[string]Value, len(shape.Params))
	ec.Push(bindings)

	for i, param := range shape.Params {
		defaultScope := ctx.calcScope(param.Owner, shape.Sym, callerScope)
		value, source, err := ec.evalIn(defaultScope).bindCalcParameter(shape, param, args, i)
		if err != nil {
			if ec.trace != nil {
				ec.trace.RecordCalcExitError(shape.Name, err)
			}
			return Value{}, err
		}
		bindings[param.Name] = value
		if ec.trace != nil {
			ec.trace.RecordCalcBind(param.Name, value, source)
		}
	}

	result, err := ec.Eval(shape.Result)
	if ec.trace != nil {
		if err != nil {
			ec.trace.RecordCalcExitError(shape.Name, err)
		} else {
			ec.trace.RecordCalcExit(shape.Name, result)
		}
	}
	if err != nil {
		return Value{}, fmt.Errorf("calc %s: %w", shape.Name, err)
	}
	return result, nil
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

// hasCalcBody reports whether sym's calc chain declares a result expression to
// evaluate. A library function declared without one — or a symbol loaded from the
// library index, which carries no declaration at all — has no body.
func (ctx *Context) hasCalcBody(sym *symbols.Symbol) bool {
	if sym == nil || sym.Decl == nil || !isCalcDecl(sym.Decl) {
		return false
	}
	result, _ := calcResult(ctx.calcChain(sym))
	return result != nil
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
