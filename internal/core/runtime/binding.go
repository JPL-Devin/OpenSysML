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
	Target     string
	Left       string
	Right      string
	LeftValue  Value
	RightValue Value
}

func (e *BindingConflictError) Error() string {
	if e.Target != "" {
		return fmt.Sprintf("%s at %s: %s = %s, %s = %s", ErrBindingConflict,
			e.Target, e.Left, bindingValueText(e.LeftValue), e.Right, bindingValueText(e.RightValue))
	}
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

type bindingAttempt struct {
	value         Value
	found         bool
	contributor   string
	cycle         bool
	cycleFeatures []string
	assignment    *bindingAssignment
	err           error
}

type bindingAssignment struct {
	endpoint bindingEndpoint
	value    Value
	binding  lower.Binding
	end      int
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

func (ctx *Context) bindingsForFeature(typeSym *symbols.Symbol, name string) []lower.Binding {
	if typeSym == nil {
		return nil
	}
	byFeature, ok := ctx.bindingFeatures[typeSym]
	if !ok {
		byFeature = make(map[string][]lower.Binding)
		ctx.bindingFeatures[typeSym] = byFeature
	}
	if bindings, ok := byFeature[name]; ok {
		return bindings
	}
	var bindings []lower.Binding
	for _, binding := range ctx.objectBindings(typeSym) {
		if bindingInvolvesFeature(binding, name) {
			bindings = append(bindings, binding)
		}
	}
	byFeature[name] = bindings
	return bindings
}

func (ctx *Context) resolveBindingValue(inst *Instance, name string) (Value, bool, error) {
	target, ok := inst.FeatureValues[name]
	if !ok {
		return Value{}, false, nil
	}
	key := featureValueRef{instance: inst.ID, feature: name}
	if ctx.resolvingBindings[key] {
		return Value{}, false, &BindingCycleError{Features: []string{bindingLocationText(bindingLocation{instance: inst, name: name})}}
	}
	if target.BindingDerived && ctx.CompositeTypeOf(target.Feature) == nil {
		target.Value = Value{}
		target.Values = Value{}
		target.Materialized = false
		target.BindingDerived = false
	}

	path := name
	for current := inst; current != nil; {
		bindings := ctx.bindingsForFeature(current.Type, path)
		if len(bindings) != 0 {
			ctx.resolvingBindings[key] = true
			value, found, cycle, cycleFeatures, err := ctx.resolveBindingSet(
				current, inst, target, path, bindings, key,
			)
			delete(ctx.resolvingBindings, key)
			if err != nil || found {
				return value, found, err
			}
			if cycle {
				return Value{}, false, &BindingCycleError{Features: cycleFeatures}
			}
		}
		owner, ownerFeature := current.Owner()
		if owner == nil || ownerFeature == "" {
			break
		}
		path = ownerFeature + "." + path
		current = owner
	}
	return Value{}, false, nil
}

func (ctx *Context) resolveBindingSet(owner, targetInst *Instance, target *FeatureValue,
	endpointName string, bindings []lower.Binding, key featureValueRef) (Value, bool, bool, []string, error) {
	var cycleFeatures []string
	var attempts []bindingAttempt
	for _, binding := range bindings {
		attempt := ctx.attemptBinding(owner, targetInst, target, binding, key)
		if attempt.err != nil {
			return Value{}, false, false, nil, attempt.err
		}
		if attempt.cycle {
			cycleFeatures = attempt.cycleFeatures
		}
		if attempt.found {
			if end := bindingEndForPath(binding, endpointName); end >= 0 {
				attempt.contributor = ctx.bindingEndpointText(binding, 1-end)
			}
			attempts = append(attempts, attempt)
		}
	}
	if len(attempts) > 1 {
		if !isScalarFeature(target.Feature) {
			return Value{}, false, false, nil, fmt.Errorf(
				"%w: multiple bindings contribute to multi-valued endpoint %q (element-wise contribution is not implemented)",
				ErrBindingEnd, endpointName,
			)
		}
		for _, attempt := range attempts[1:] {
			if !valueEqual(attempts[0].value, attempt.value) {
				return Value{}, false, false, nil, &BindingConflictError{
					Target:     bindingLocationText(bindingLocation{instance: targetInst, name: key.feature}),
					Left:       attempts[0].contributor,
					Right:      attempt.contributor,
					LeftValue:  attempts[0].value,
					RightValue: attempt.value,
				}
			}
		}
	}
	selected := attempts
	if len(selected) > 1 {
		selected = selected[:1]
	}
	if len(selected) == 1 && selected[0].assignment != nil {
		assignment := selected[0].assignment
		if err := ctx.assignBindingEndpoint(
			assignment.endpoint, assignment.value, assignment.binding, assignment.end,
		); err != nil {
			return Value{}, false, false, nil, err
		}
	}
	if len(attempts) != 0 {
		return attempts[0].value, true, false, nil, nil
	}
	return Value{}, false, len(cycleFeatures) != 0, cycleFeatures, nil
}

func bindingEndForPath(binding lower.Binding, path string) int {
	for end, endpoint := range binding.Ends {
		if endpoint.Path == path {
			return end
		}
	}
	return -1
}

func (ctx *Context) attemptBinding(owner, targetInst *Instance, target *FeatureValue,
	binding lower.Binding, key featureValueRef) (attempt bindingAttempt) {
	previousOwner, hadOwner := ctx.bindingOwners[key]
	ctx.bindingOwners[key] = binding.Decl
	defer func() {
		if hadOwner {
			ctx.bindingOwners[key] = previousOwner
		} else {
			delete(ctx.bindingOwners, key)
		}
	}()

	left, err := ctx.resolveBindingEndpoint(owner, binding, 0)
	if err != nil {
		attempt.err = err
		return attempt
	}
	right, err := ctx.resolveBindingEndpoint(owner, binding, 1)
	if err != nil {
		attempt.err = err
		return attempt
	}
	leftCarries := left.location.instance != nil &&
		bindingLocationCarries(left.location, target, targetInst)
	rightCarries := right.location.instance != nil &&
		bindingLocationCarries(right.location, target, targetInst)
	if !leftCarries && !rightCarries {
		return attempt
	}

	leftValue, leftSet, err := ctx.bindingEndpointValue(left, owner, leftCarries)
	if err != nil {
		attempt.err = err
		return attempt
	}
	if left.location.instance != nil &&
		ctx.genuineBindingCycle(featureValueRef{
			instance: left.location.instance.ID,
			feature:  left.location.name,
		}, binding.Decl) {
		attempt.cycle = true
	}
	rightValue, rightSet, err := ctx.bindingEndpointValue(right, owner, rightCarries)
	if err != nil {
		attempt.err = err
		return attempt
	}
	if right.location.instance != nil &&
		ctx.genuineBindingCycle(featureValueRef{
			instance: right.location.instance.ID,
			feature:  right.location.name,
		}, binding.Decl) {
		attempt.cycle = true
	}
	if attempt.cycle {
		attempt.cycleFeatures = []string{
			ctx.bindingEndpointText(binding, 0),
			ctx.bindingEndpointText(binding, 1),
		}
	}

	switch {
	case leftSet && rightSet:
		leftDerived := bindingEndpointDerived(left)
		rightDerived := bindingEndpointDerived(right)
		if leftDerived && !rightDerived {
			attempt.value = rightValue
			attempt.found = true
			attempt.assignment = &bindingAssignment{
				endpoint: left, value: rightValue, binding: binding, end: 0,
			}
			return attempt
		}
		if rightDerived && !leftDerived {
			attempt.value = leftValue
			attempt.found = true
			attempt.assignment = &bindingAssignment{
				endpoint: right, value: leftValue, binding: binding, end: 1,
			}
			return attempt
		}
		if !valueEqual(leftValue, rightValue) {
			attempt.err = &BindingConflictError{
				Left: ctx.bindingEndpointText(binding, 0), Right: ctx.bindingEndpointText(binding, 1),
				LeftValue: leftValue, RightValue: rightValue,
			}
			return attempt
		}
		attempt.value = leftValue
		attempt.found = true
	case leftSet:
		attempt.assignment = &bindingAssignment{
			endpoint: right, value: leftValue, binding: binding, end: 1,
		}
		if leftCarries || rightCarries {
			attempt.value = leftValue
			attempt.found = true
		}
	case rightSet:
		attempt.assignment = &bindingAssignment{
			endpoint: left, value: rightValue, binding: binding, end: 0,
		}
		if leftCarries || rightCarries {
			attempt.value = rightValue
			attempt.found = true
		}
	}
	return attempt
}

func (ctx *Context) genuineBindingCycle(ref featureValueRef, binding *ast.Usage) bool {
	return ctx.resolvingBindings[ref] && ctx.bindingOwners[ref] != binding
}

func bindingInvolvesFeature(binding lower.Binding, name string) bool {
	for _, end := range binding.Ends {
		if end.Path == name {
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

func (ctx *Context) bindingEndpointValue(endpoint bindingEndpoint, owner *Instance, materialize bool) (Value, bool, error) {
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
	if fv.BindingDerived && materialize {
		return Value{}, false, nil
	}
	if fv.BindingDerived {
		if ctx.CompositeTypeOf(fv.Feature) != nil {
			if val := fv.HeldValue(); val.Kind != ValInvalid {
				return val, true, nil
			}
		}
		val, found, err := ctx.resolveBindingValue(loc.instance, loc.name)
		if err != nil {
			return Value{}, false, err
		}
		if found {
			return val, true, nil
		}
		return Value{}, false, nil
	}
	if !fv.Materialized {
		if !materialize && ctx.CompositeTypeOf(fv.Feature) != nil {
			return Value{}, false, nil
		}
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

func bindingEndpointDerived(endpoint bindingEndpoint) bool {
	if endpoint.location.instance == nil {
		return false
	}
	return endpoint.location.instance.FeatureValues[endpoint.location.name].BindingDerived
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
			elements := elementsOf(val)
			if err := ctx.chargeElements(int64(len(elements))); err != nil {
				return err
			}
			val = sequenceOf(elements)
		}
		fv.Value = Value{}
		fv.Values = val
	}
	fv.Materialized = true
	fv.BindingDerived = true
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
	return FormatValue(val)
}
