package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// InvokeOperation runs a behavior the object's type owns with the object as the
// performer: what the body reads and writes is that object's feature values, and
// what it sends and accepts carries that object's identity. Arguments bind to the
// operation's `in` and `inout` parameters by name.
//
// Known limitation: only an action member is executable as an operation. A calc
// member is invoked as an expression against a scope rather than performed by an
// object, and a state member runs as the object's exhibited machine instead
// (see Instance.ExhibitedState).
func (ctx *Context) InvokeOperation(inst *Instance, name string, args map[string]Value) (map[string]Value, error) {
	if inst == nil {
		return nil, fmt.Errorf("%w: no object to perform %s", ErrNoSuchBehavior, name)
	}
	sym, err := ctx.operationOf(inst, name)
	if err != nil {
		return nil, err
	}
	inputs, err := operationInputs(sym, name, args)
	if err != nil {
		return nil, err
	}
	results, err := ctx.ExecuteActionPerformedBy(sym, inst, inputs)
	if err != nil {
		return nil, fmt.Errorf("invoke %s on object #%d: %w", name, inst.ID, err)
	}

	_, out := actionParameters(sym.Decl)
	outputs := make(map[string]Value, len(out))
	for _, param := range out {
		if value, ok := results[param]; ok {
			outputs[param] = value
		}
	}
	return outputs, nil
}

// operationOf resolves the member of the object's type that name invokes, and
// reports a member that states no executable behavior.
func (ctx *Context) operationOf(inst *Instance, name string) (*symbols.Symbol, error) {
	var member *symbols.Symbol
	for _, candidate := range ctx.model.MembersOf(inst.Type) {
		if candidate.Name == name {
			member = candidate
			break
		}
	}
	if member == nil {
		return nil, fmt.Errorf("%w: %s of object #%d (type %s)",
			ErrNoSuchBehavior, name, inst.ID, symbolText(inst.Type))
	}
	switch member.Kind {
	case symbols.SymbolActionDef, symbols.SymbolActionUsage:
		return member, nil
	case symbols.SymbolStateDef, symbols.SymbolStateUsage:
		return nil, fmt.Errorf("%w: %s of %s is a state machine, which runs as the object's exhibited machine",
			ErrUnsupportedClassifierBehavior, name, symbolText(inst.Type))
	case symbols.SymbolCalcDef, symbols.SymbolCalcUsage,
		symbols.SymbolConstraintDef, symbols.SymbolConstraintUsage:
		return nil, fmt.Errorf("%w: %s of %s is a %s, which is evaluated as an expression rather than performed by an object",
			ErrUnsupportedClassifierBehavior, name, symbolText(inst.Type), member.Kind)
	default:
		return nil, fmt.Errorf("%w: %s of %s is a %s",
			ErrNotABehavior, name, symbolText(inst.Type), member.Kind)
	}
}

// operationInputs binds arguments to the operation's input parameters, reporting
// an argument naming no parameter and a parameter left with no value: either
// would otherwise run the body against values the invocation never stated.
func operationInputs(sym *symbols.Symbol, name string, args map[string]Value) (map[string]Value, error) {
	params := actionParameterDecls(sym.Decl)
	inputs := make(map[string]Value, len(args))
	for _, param := range params {
		if param.Direction == ast.DirOut {
			continue
		}
		value, bound := args[param.Name]
		switch {
		case bound:
			inputs[param.Name] = value
		case param.HasDefault:
		default:
			return nil, fmt.Errorf("%w: parameter %s of operation %s has no argument and no default",
				ErrUnboundParameter, param.Name, name)
		}
	}
	for arg := range args {
		if !bindsParameter(params, arg) {
			return nil, fmt.Errorf("%w: %s is no input parameter of operation %s",
				ErrUnboundParameter, arg, name)
		}
	}
	return inputs, nil
}

// bindsParameter reports whether name is an input parameter an invocation binds.
func bindsParameter(params []actionParameter, name string) bool {
	for _, param := range params {
		if param.Name == name && param.Direction != ast.DirOut {
			return true
		}
	}
	return false
}
