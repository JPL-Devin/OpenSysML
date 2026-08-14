package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
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
			target: invocation.Type,
			args:   invocation.Args,
			named:  invocation.NamedArgs,
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
// the caller, and returns the values its output parameters ended with.
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
	inputs, err := bindArguments(ctx, scope, inv, in, data)
	if err != nil {
		return nil, err
	}

	results, err := ctx.ExecuteActionWithInputs(sym, inputs)
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

// bindArguments computes the callee's input bindings. An invocation with an
// argument list binds those arguments; a bare `perform`/typed usage instead
// passes the caller's values of the parameters' own names, which is how data
// reaches an action performed inside a flow.
func bindArguments(
	ctx *Context,
	scope *symbols.Scope,
	inv actionInvocation,
	in []string,
	data map[string]Value,
) (map[string]Value, error) {
	inputs := make(map[string]Value, len(in))

	if len(inv.args) == 0 && len(inv.named) == 0 {
		for _, name := range in {
			if value, ok := data[name]; ok {
				inputs[name] = value
			}
		}
		return inputs, nil
	}

	ec := NewEvalContext(ctx, scope)
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

// actionParameters returns the names of an action's parameters that the caller
// writes (`in`, `inout`) and that it reads back (`out`, `inout`).
func actionParameters(decl ast.Node) (in, out []string) {
	var members []ast.Node
	switch d := decl.(type) {
	case *ast.Usage:
		members = d.Members
	case *ast.Definition:
		members = d.Members
	default:
		return nil, nil
	}

	for _, member := range members {
		if membership, ok := member.(*ast.Membership); ok {
			member = membership.Member
		}
		usage, ok := member.(*ast.Usage)
		if !ok {
			continue
		}
		// A parameter may be written as a redefinition of the one it overrides
		// (`in redefines ifTest;`), naming it by that redefinition.
		name, _ := ast.EffectiveName(usage)
		if name == "" {
			continue
		}
		switch usage.Direction {
		case ast.DirIn:
			in = append(in, name)
		case ast.DirOut:
			out = append(out, name)
		case ast.DirInOut:
			in = append(in, name)
			out = append(out, name)
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
