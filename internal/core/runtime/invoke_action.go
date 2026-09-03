package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// maxActionNestingDepth bounds how deep action-in-action invocation may go. An
// action that reaches itself, directly or through a cycle, would otherwise
// recurse until the process ran out of stack rather than reporting the model
// error, since each nested invocation runs on a fresh executor.
const maxActionNestingDepth = 32

// actionInvocation is a nested action usage that performs another action, in any
// of the three forms the parser produces:
//
//	perform Callee;            // anonymous usage, 'references' relationship
//	action call : Callee;      // named usage, 'typing' relationship
//	action call = Callee(1);   // named usage, invocation expression value
type actionInvocation struct {
	target *ast.QualifiedName
	// invoked reports the `Callee(...)` form: the argument list, even an empty
	// one, states every input the caller passes.
	invoked bool
	args    []ast.Node
	named   []ast.NamedArg
	// referrer is the usage owning a reference subsetting, whose own effective
	// name is the one the target names (see resolve.ResolveReferenceTarget).
	referrer ast.Node
}

// nestedInvocation reports the action a nested usage performs, if any. A usage
// that only carries its own body (assignments, sends, accepts) performs nothing.
// Only typing and reference-subsetting edges name a performed action: the port
// of `accept msg : T via p` is a via edge, not a reference subsetting.
func nestedInvocation(usage *ast.Usage) (actionInvocation, bool) {
	if invocation, ok := usage.Value.(*ast.InvocationExpr); ok && invocation.Type != nil {
		return actionInvocation{
			target:  invocation.Type,
			invoked: true,
			args:    invocation.Args,
			named:   invocation.NamedArgs,
		}, true
	}
	for _, rel := range usage.Relationships {
		if rel.Kind != ast.RelTyping && rel.Kind != ast.RelReferences {
			continue
		}
		if qn, ok := rel.Target.(*ast.QualifiedName); ok {
			inv := actionInvocation{target: qn}
			if rel.Kind == ast.RelReferences {
				inv.referrer = usage
			}
			return inv, true
		}
	}
	return actionInvocation{}, false
}

// invokeAction runs the action named by inv to completion as a sub-execution of
// the caller, and returns the values its output parameters ended with. The
// performed action runs as self, the object performing the caller, so what it
// accepts and sends carries that object's identity.
//
// The callee gets a fresh executor with its own tokens, so values cross the
// boundary only through parameters: arguments (or, for an argument-less
// invocation, caller values of the same name) seed the callee's `in` and `inout`
// parameters, and its `out` and `inout` parameters come back to the caller. An
// action with no parameters therefore reads and writes nothing in its caller.
func invokeAction(
	ctx *Context,
	scope *symbols.Scope,
	inv actionInvocation,
	data map[string]Value,
	self *Instance,
) (map[string]Value, error) {
	sym, err := resolveActionSymbol(ctx, scope, inv)
	if err != nil {
		return nil, err
	}

	if ctx.actionDepth >= maxActionNestingDepth {
		return nil, fmt.Errorf(
			"action invocation nested more than %d deep at %s (recursive action?)",
			maxActionNestingDepth, qualifiedNameText(inv.target),
		)
	}
	ctx.actionDepth++
	defer func() { ctx.actionDepth-- }()

	params := ctx.actionParametersOf(sym)
	in, out := parameterNames(params)
	inputs := make(map[string]Value, len(in))
	if inv.invoked {
		ec := NewEvalContext(ctx, scope)
		ec.Push(data)
		defer ec.beginStep()()
		if err := bindArgumentList(ec, inv, in, inputs); err != nil {
			return nil, err
		}
	} else {
		// A bare `perform`/typed usage reads the caller's values of the parameters'
		// own names, which is how data reaches an action performed inside a flow.
		for _, name := range in {
			if value, ok := data[name]; ok {
				inputs[name] = value
			}
		}
	}
	if err := checkInputsBound(inv, params, inputs); err != nil {
		return nil, err
	}

	results, err := ctx.ExecuteActionPerformedBy(sym, self, inputs)
	if err != nil {
		return nil, fmt.Errorf("invoke action %s: %w", qualifiedNameText(inv.target), err)
	}

	outputs := make(map[string]Value, len(out))
	for _, name := range out {
		if value, ok := results[name]; ok {
			outputs[name] = value
		}
	}
	return outputs, nil
}

func resolveActionSymbol(
	ctx *Context,
	scope *symbols.Scope,
	inv actionInvocation,
) (*symbols.Symbol, error) {
	target := inv.target
	if target == nil || len(target.Parts) == 0 {
		return nil, fmt.Errorf("empty action reference")
	}
	name := qualifiedNameText(target)
	if scope == nil || ctx.resolver == nil {
		return nil, fmt.Errorf("cannot resolve action %s: no scope", name)
	}
	var sym *symbols.Symbol
	var ok bool
	if inv.referrer != nil {
		sym, ok = ctx.resolver.ResolveReferenceTarget(scope, inv.referrer, target)
	} else {
		sym, ok = ctx.resolver.ResolveQualified(scope, target)
	}
	if !ok || sym == nil {
		return nil, fmt.Errorf("unresolved action reference: %s", name)
	}
	if inv.referrer != nil && sym.Decl == inv.referrer {
		return nil, fmt.Errorf("unresolved action reference: %s (a perform statement cannot perform itself)", name)
	}
	if sym.Kind != symbols.SymbolActionDef && sym.Kind != symbols.SymbolActionUsage {
		return nil, fmt.Errorf("%s is not an action (%v)", name, sym.Kind)
	}
	return sym, nil
}

// bindArgumentList binds an invocation's arguments into inputs by the callee's parameter
// order (positional) or names (named); arguments are evaluated in ec, the caller's context.
func bindArgumentList(ec *EvalContext, inv actionInvocation, in []string, inputs map[string]Value) error {
	if len(inv.args) > len(in) {
		return fmt.Errorf(
			"%w: action %s takes %d input parameter(s), got %d argument(s)",
			ErrActionArity, qualifiedNameText(inv.target), len(in), len(inv.args),
		)
	}
	for i, arg := range inv.args {
		value, err := ec.Eval(arg)
		if err != nil {
			return fmt.Errorf("eval argument %d of %s: %w", i+1, qualifiedNameText(inv.target), err)
		}
		inputs[in[i]] = value
	}

	for _, named := range inv.named {
		if named.Name == nil || len(named.Name.Parts) == 0 {
			return fmt.Errorf("unnamed argument in invocation of %s", qualifiedNameText(inv.target))
		}
		name := named.Name.Parts[len(named.Name.Parts)-1].Text
		if !contains(in, name) {
			return fmt.Errorf(
				"%w: action %s has no input parameter %q",
				ErrUnknownParameter, qualifiedNameText(inv.target), name,
			)
		}
		value, err := ec.Eval(named.Value)
		if err != nil {
			return fmt.Errorf("eval argument %q of %s: %w", name, qualifiedNameText(inv.target), err)
		}
		inputs[name] = value
	}
	return nil
}

// actionParameter is one parameter an action declares.
type actionParameter struct {
	Name string
	// Direction is the parameter's declared direction, which decides whether the
	// caller writes it, reads it back, or both.
	Direction ast.FeatureDirection
	// HasDefault reports whether the declaration gives the parameter a value, so
	// an invocation binding no argument to it still binds a value.
	HasDefault bool
	// IsResult marks the `return` parameter, what the action's value read yields.
	IsResult bool
}

// actionParametersOf returns an action's parameters in invocation order: its own, then
// the inherited ones none redefines (KerML 7.4.7.2) — the signature the type checker uses.
func (ctx *Context) actionParametersOf(sym *symbols.Symbol) []actionParameter {
	var params []actionParameter
	for _, param := range ctx.model.BehaviorParametersOf(sym) {
		if param.Symbol == nil || param.Symbol.Name == "" {
			continue
		}
		usage, _ := param.Symbol.Decl.(*ast.Usage)
		params = append(params, actionParameter{
			Name:       param.Symbol.Name,
			Direction:  param.Direction,
			HasDefault: usage != nil && usage.Value != nil,
			IsResult:   param.IsResult,
		})
	}
	return params
}

// parameterNames splits parameters into those the caller writes and reads back.
func parameterNames(params []actionParameter) (in, out []string) {
	for _, param := range params {
		switch param.Direction {
		case ast.DirIn:
			in = append(in, param.Name)
		case ast.DirOut:
			out = append(out, param.Name)
		case ast.DirInOut:
			in = append(in, param.Name)
			out = append(out, param.Name)
		}
	}
	return in, out
}

func contains(names []string, name string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}

// qualifiedNameText renders a qualified name as written, for diagnostics.
func qualifiedNameText(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	parts := make([]string, 0, len(qn.Parts))
	for _, part := range qn.Parts {
		parts = append(parts, part.Text)
	}
	return strings.Join(parts, "::")
}
