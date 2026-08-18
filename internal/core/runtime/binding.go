package runtime

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// BindingConflictError reports unequal values held by the two ends of a
// binding connector.
type BindingConflictError struct {
	Left       string
	Right      string
	LeftValue  Value
	RightValue Value
}

func (e *BindingConflictError) Error() string {
	return fmt.Sprintf("%s: %s = %s, %s = %s", ErrBindingConflict,
		e.Left, bindingValueText(e.LeftValue), e.Right, bindingValueText(e.RightValue))
}

func (e *BindingConflictError) Unwrap() error { return ErrBindingConflict }

// BindingCycleError reports a binding component that has no valued end.
type BindingCycleError struct {
	Features []string
}

func (e *BindingCycleError) Error() string {
	return fmt.Sprintf("%s: %s", ErrBindingCycle, strings.Join(e.Features, " -> "))
}

func (e *BindingCycleError) Unwrap() error { return ErrBindingCycle }

type bindingLocation struct {
	instance *Instance
	name     string
	path     string
}

type bindingEndpoint struct {
	location bindingLocation
	expr     ast.Node
	scope    *symbols.Scope
}

type bindingResult struct {
	value Value
	found bool
	err   error
}

func (ctx *Context) objectBindings(typeSym *symbols.Symbol) []lower.Binding {
	if typeSym == nil {
		return nil
	}
	if cached, ok := ctx.bindingIR[typeSym]; ok {
		return cached
	}
	var bindings []lower.Binding
	chain := ctx.model.AllSupertypes(typeSym)
	for i := len(chain) - 1; i >= 0; i-- {
		if chain[i] != nil {
			bindings = append(bindings, lower.ToBindings(chain[i].Decl, declScope(chain[i]))...)
		}
	}
	bindings = append(bindings, lower.ToBindings(typeSym.Decl, declScope(typeSym))...)
	ctx.bindingIR[typeSym] = bindings
	return bindings
}

func (ctx *Context) resolveBindingValue(inst *Instance, name string) (value Value, found bool, err error) {
	bindings := ctx.objectBindings(inst.Type)
	if len(bindings) == 0 {
		return Value{}, false, nil
	}
	target, ok := inst.FeatureValues[name]
	if !ok {
		return Value{}, false, nil
	}
	key := featureValueRef{instance: inst.ID, feature: name}
	if cached, ok := ctx.bindingResults[key]; ok {
		return cached.value, cached.found, cached.err
	}
	if ctx.resolvingBindings[key] {
		return Value{}, false, &BindingCycleError{Features: []string{bindingLocationText(bindingLocation{instance: inst, name: name})}}
	}
	ctx.resolvingBindings[key] = true
	defer func() {
		delete(ctx.resolvingBindings, key)
		ctx.bindingResults[key] = bindingResult{value: value, found: found, err: err}
	}()

	cycleSeen := false
	var cycleFeatures []string
	for _, binding := range bindings {
		if !bindingInvolvesFeature(binding, name) {
			continue
		}
		left, err := ctx.resolveBindingEndpoint(inst, binding, 0)
		if err != nil {
			return Value{}, false, err
		}
		right, err := ctx.resolveBindingEndpoint(inst, binding, 1)
		if err != nil {
			return Value{}, false, err
		}
		leftCarries := left.location.instance != nil && bindingLocationCarries(left.location, target, inst)
		rightCarries := right.location.instance != nil && bindingLocationCarries(right.location, target, inst)
		if !leftCarries && !rightCarries && left.location.path != name && right.location.path != name {
			continue
		}

		leftValue, leftSet, err := ctx.bindingEndpointValue(left, inst)
		if err != nil {
			return Value{}, false, err
		}
		if left.location.instance != nil && ctx.resolvingBindings[featureValueRef{instance: left.location.instance.ID, feature: left.location.name}] {
			cycleSeen = true
		}
		rightValue, rightSet, err := ctx.bindingEndpointValue(right, inst)
		if err != nil {
			return Value{}, false, err
		}
		if right.location.instance != nil && ctx.resolvingBindings[featureValueRef{instance: right.location.instance.ID, feature: right.location.name}] {
			cycleSeen = true
		}
		if cycleSeen {
			cycleFeatures = []string{ctx.bindingEndpointText(binding, 0), ctx.bindingEndpointText(binding, 1)}
		}
		switch {
		case leftSet && rightSet:
			if valueKeyFunc(leftValue) != valueKeyFunc(rightValue) {
				return Value{}, false, &BindingConflictError{
					Left: ctx.bindingEndpointText(binding, 0), Right: ctx.bindingEndpointText(binding, 1),
					LeftValue: leftValue, RightValue: rightValue,
				}
			}
			if leftCarries {
				return leftValue, true, nil
			}
			return rightValue, true, nil
		case leftSet:
			if err := ctx.assignBindingEndpoint(right, leftValue, binding, 1); err != nil {
				return Value{}, false, err
			}
			if leftCarries {
				return leftValue, true, nil
			}
			if rightCarries {
				return leftValue, true, nil
			}
		case rightSet:
			if err := ctx.assignBindingEndpoint(left, rightValue, binding, 0); err != nil {
				return Value{}, false, err
			}
			if leftCarries || rightCarries {
				return rightValue, true, nil
			}
		}
	}
	if cycleSeen {
		return Value{}, false, &BindingCycleError{Features: cycleFeatures}
	}
	return Value{}, false, nil
}

func bindingInvolvesFeature(binding lower.Binding, name string) bool {
	for _, end := range binding.Ends {
		if root := strings.SplitN(end.Path, ".", 2)[0]; root == name {
			return true
		}
	}
	return false
}

func (ctx *Context) resolveBindingEndpoint(owner *Instance, binding lower.Binding, end int) (bindingEndpoint, error) {
	path := binding.Ends[end].Path
	if path == "" {
		if binding.Ends[end].Expr != nil {
			return bindingEndpoint{expr: binding.Ends[end].Expr, scope: binding.Scope}, nil
		}
		return bindingEndpoint{}, fmt.Errorf("%w: empty endpoint", ErrBindingEnd)
	}
	loc, err := ctx.resolveBindingLocation(owner, path)
	if err != nil {
		return bindingEndpoint{}, err
	}
	loc.path = path
	return bindingEndpoint{location: loc}, nil
}

func (ctx *Context) resolveBindingLocation(owner *Instance, path string) (bindingLocation, error) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 || parts[0] == "" {
		return bindingLocation{}, fmt.Errorf("%w: empty endpoint", ErrBindingEnd)
	}
	current := owner
	for _, part := range parts[:len(parts)-1] {
		fv, err := current.GetFeatureValue(ctx, part)
		if err != nil {
			return bindingLocation{}, fmt.Errorf("%w %q: %v", ErrBindingEnd, path, err)
		}
		next, isObject := fv.HeldValue().Object()
		if !isObject {
			return bindingLocation{}, fmt.Errorf("%w %q: %s is not an object", ErrBindingEnd, path, part)
		}
		var ok bool
		current, ok = ctx.instances[next]
		if !ok {
			return bindingLocation{}, fmt.Errorf("%w %q: instance %d is not materialized", ErrBindingEnd, path, next)
		}
	}
	name := parts[len(parts)-1]
	if _, ok := current.FeatureValues[name]; !ok {
		return bindingLocation{}, fmt.Errorf("%w %q: feature %s not found", ErrBindingEnd, path, name)
	}
	return bindingLocation{instance: current, name: name}, nil
}

func (ctx *Context) bindingEndpointValue(endpoint bindingEndpoint, owner *Instance) (Value, bool, error) {
	if endpoint.expr != nil {
		value, err := ctx.EvalWithScopeOn(endpoint.expr, endpoint.scope, owner)
		if err != nil {
			if errors.Is(err, ErrUninitializedFeatureValue) {
				return Value{}, false, nil
			}
			return Value{}, false, fmt.Errorf("%w: expression %s: %v",
				ErrBindingEnd, ctx.bindingExprText(endpoint.expr, endpoint.scope), err)
		}
		if value.Kind == ValInvalid {
			return Value{}, false, nil
		}
		return value, true, nil
	}
	loc := endpoint.location
	fv := loc.instance.FeatureValues[loc.name]
	if !fv.Materialized {
		if _, err := loc.instance.materializeFeatureValueIntrinsic(ctx, loc.name); err != nil {
			return Value{}, false, err
		}
	}
	if val := fv.HeldValue(); val.Kind != ValInvalid {
		return val, true, nil
	}
	if ctx.resolvingBindings[featureValueRef{instance: loc.instance.ID, feature: loc.name}] {
		return Value{}, false, nil
	}
	val, found, err := ctx.resolveBindingValue(loc.instance, loc.name)
	if err != nil {
		return Value{}, false, err
	}
	return val, found, nil
}

func (ctx *Context) assignBindingEndpoint(endpoint bindingEndpoint, val Value, binding lower.Binding, end int) error {
	if endpoint.expr != nil {
		return fmt.Errorf("%w: cannot assign %s from %s",
			ErrBindingEnd, ctx.bindingEndpointText(binding, end), ctx.bindingEndpointText(binding, 1-end))
	}
	loc := endpoint.location
	return ctx.assignBindingValue(loc.instance, loc.instance.FeatureValues[loc.name], loc.name, val)
}

func (ctx *Context) assignBindingValue(inst *Instance, fv *FeatureValue, name string, val Value) error {
	if err := ctx.checkDefaultCount(inst, fv, name, val); err != nil {
		return err
	}
	if isScalarFeature(fv.Feature) {
		fv.Value = val
		fv.Values = Value{}
	} else {
		if val.Kind != ValSequence && val.Kind != ValSet {
			val = sequenceOf(elementsOf(val))
		}
		fv.Value = Value{}
		fv.Values = val
	}
	fv.Materialized = true
	return nil
}

func bindingLocationCarries(loc bindingLocation, fv *FeatureValue, inst *Instance) bool {
	return loc.instance == inst && loc.instance.FeatureValues[loc.name] == fv
}

func bindingLocationText(loc bindingLocation) string {
	if loc.instance == nil {
		return loc.name
	}
	if loc.path != "" {
		return loc.path
	}
	if loc.instance.Type == nil {
		return loc.name
	}
	return fmt.Sprintf("%s.%s", loc.instance.Type.Name, loc.name)
}

func (ctx *Context) bindingEndpointText(binding lower.Binding, end int) string {
	endpoint := binding.Ends[end]
	if endpoint.Path != "" {
		return endpoint.Path
	}
	return ctx.bindingExprText(endpoint.Expr, binding.Scope)
}

func (ctx *Context) bindingExprText(expr ast.Node, scope *symbols.Scope) string {
	if expr == nil {
		return "<empty>"
	}
	if scope != nil && scope.Owner() != nil {
		if sf := ctx.sources[scope.Owner().DocName]; sf != nil {
			if text := strings.TrimSpace(sf.Text(expr.Span())); text != "" {
				return text
			}
		}
	}
	switch node := expr.(type) {
	case *ast.OperatorExpr:
		parts := make([]string, len(node.Operands))
		for i, operand := range node.Operands {
			parts[i] = ctx.bindingExprText(operand, scope)
		}
		return strings.Join(parts, " "+node.Operator.String()+" ")
	case *ast.FeatureReference:
		return ctx.bindingExprText(node.Name, scope)
	case *ast.QualifiedName:
		parts := make([]string, len(node.Parts))
		for i, part := range node.Parts {
			parts[i] = part.Text
		}
		return strings.Join(parts, "::")
	case *ast.LiteralInteger:
		return node.Value
	case *ast.LiteralReal:
		return node.Value
	case *ast.LiteralBool:
		return strconv.FormatBool(node.Value)
	case *ast.LiteralString:
		return node.Value
	}
	return ast.Dump(expr)
}

func bindingValueText(val Value) string {
	switch val.Kind {
	case ValConst:
		return fmt.Sprintf("%v", val.Const)
	case ValString:
		return fmt.Sprintf("%q", val.Str)
	case ValEnumLiteral:
		return val.LiteralText()
	case ValInstance:
		return fmt.Sprintf("instance(%d)", val.Instance)
	default:
		return val.Kind.String()
	}
}
