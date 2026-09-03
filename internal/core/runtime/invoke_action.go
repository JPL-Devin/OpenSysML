package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
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
	args   []ast.Node
	named  []ast.NamedArg
	// expr is the invocation expression form, whose arguments select among
	// same-named actions as they do for calcs.
	expr *ast.InvocationExpr
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
		return expressionInvocation(invocation), true
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

// expressionInvocation reads an invocation expression as the action call it
// writes. A receiver is the first argument, as `seq->size()` is for a calc.
func expressionInvocation(e *ast.InvocationExpr) actionInvocation {
	args := e.Args
	if e.Operand != nil {
		args = append([]ast.Node{e.Operand}, e.Args...)
	}
	return actionInvocation{target: e.Type, args: args, named: e.NamedArgs, expr: e}
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

	in, out := actionParameters(sym.Decl)
	inputs, err := bindArguments(ctx, scope, inv, in, data, self)
	if err != nil {
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
	switch {
	case inv.referrer != nil:
		sym, ok = ctx.resolver.ResolveReferenceTarget(scope, inv.referrer, target)
	case inv.expr != nil:
		sel := passes.SelectInvocation(ctx.resolver, ctx.model, scope, inv.expr)
		if sel.Ambiguous {
			return nil, ambiguousInvocationError(name, sel.Tied)
		}
		sym = sel.Called()
		ok = sym != nil
	default:
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

// bindArguments computes the callee's input bindings. An invocation with an
// argument list binds those arguments; a bare `perform`/typed usage instead
// passes the caller's values of the parameters' own names, which is how data
// reaches an action performed inside a flow. Arguments are evaluated as the
// caller's body is: over its values, performed by self.
func bindArguments(
	ctx *Context,
	scope *symbols.Scope,
	inv actionInvocation,
	in []string,
	data map[string]Value,
	self *Instance,
) (map[string]Value, error) {
	inputs := make(map[string]Value, len(in))

	if inv.expr != nil && inv.expr.Operand != nil && len(inv.named) > 0 {
		return nil, fmt.Errorf(
			"%w: %s is called with a receiver and named arguments",
			ErrReceiverWithNamedArgs, qualifiedNameText(inv.target),
		)
	}
	if len(inv.args) == 0 && len(inv.named) == 0 {
		for _, name := range in {
			if value, ok := data[name]; ok {
				inputs[name] = value
			}
		}
		return inputs, nil
	}

	ec := NewEvalContextIn(ctx, scope, self)
	ec.inBehaviorBody = true
	ec.Push(data)
	defer ec.beginStep()()

	if len(inv.args) > len(in) {
		return nil, fmt.Errorf(
			"action %s takes %d input parameter(s), got %d argument(s)",
			qualifiedNameText(inv.target), len(in), len(inv.args),
		)
	}
	for i, arg := range inv.args {
		value, err := ec.Eval(arg)
		if err != nil {
			return nil, fmt.Errorf("eval argument %d of %s: %w", i+1, qualifiedNameText(inv.target), err)
		}
		inputs[in[i]] = value
	}

	for _, named := range inv.named {
		if named.Name == nil || len(named.Name.Parts) == 0 {
			return nil, fmt.Errorf("unnamed argument in invocation of %s", qualifiedNameText(inv.target))
		}
		name := named.Name.Parts[len(named.Name.Parts)-1].Text
		if !contains(in, name) {
			return nil, fmt.Errorf(
				"action %s has no input parameter %q",
				qualifiedNameText(inv.target), name,
			)
		}
		value, err := ec.Eval(named.Value)
		if err != nil {
			return nil, fmt.Errorf("eval argument %q of %s: %w", name, qualifiedNameText(inv.target), err)
		}
		inputs[name] = value
	}
	return inputs, nil
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
}

// actionParameterDecls returns the parameters an action declares, in declaration
// order.
func actionParameterDecls(decl ast.Node) []actionParameter {
	var members []ast.Node
	switch d := decl.(type) {
	case *ast.Usage:
		members = d.Members
	case *ast.Definition:
		members = d.Members
	default:
		return nil
	}

	var params []actionParameter
	for _, member := range members {
		if membership, ok := member.(*ast.Membership); ok {
			member = membership.Member
		}
		usage, ok := member.(*ast.Usage)
		if !ok {
			continue
		}
		if usage.Direction == ast.DirNone {
			continue
		}
		// A parameter may be written as a redefinition of the one it overrides
		// (`in redefines ifTest;`), naming it by that redefinition.
		name, _ := ast.EffectiveName(usage)
		if name == "" {
			continue
		}
		params = append(params, actionParameter{
			Name:       name,
			Direction:  usage.Direction,
			HasDefault: usage.Value != nil,
		})
	}
	return params
}

// actionParameters returns the names of an action's parameters that the caller
// writes (`in`, `inout`) and that it reads back (`out`, `inout`).
func actionParameters(decl ast.Node) (in, out []string) {
	for _, param := range actionParameterDecls(decl) {
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
