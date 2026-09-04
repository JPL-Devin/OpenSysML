package runtime

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
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

// UndeterminedBindingError reports a feature valued by nothing but bindings that each
// link one unspecified value of it, so none of them determines what it holds.
type UndeterminedBindingError struct {
	Target   string
	Binding  string
	Endpoint string
}

func (e *UndeterminedBindingError) Error() string {
	return fmt.Sprintf("%s: %s is bound by `%s`, which links one unspecified value of each end; the model does not determine which value %s holds",
		ErrBindingEnd, e.Target, e.Binding, e.Endpoint)
}

func (e *UndeterminedBindingError) Unwrap() error { return ErrBindingEnd }

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
	value       Value
	found       bool
	contributor string
	// settled reports that no end waits on a resolution in progress, so an
	// unfound value means the binding links nothing.
	settled       bool
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
		if bindingInvolvesFeature(binding, name) && !ctx.bindingLinksNothing(binding) {
			bindings = append(bindings, binding)
		}
	}
	byFeature[name] = bindings
	return bindings
}

// bindingLinksNothing reports a binding with an end of multiplicity [0], which links no
// value and so constrains neither feature (KerML 1.0 §7.4.9.2, connector end multiplicity).
func (ctx *Context) bindingLinksNothing(binding lower.Binding) bool {
	for _, end := range binding.Ends {
		if r, ok := ctx.model.RangeOf(end.Multiplicity); ok && r.Upper.Known && !r.Upper.Infinite && r.Upper.Value == 0 {
			return true
		}
	}
	return false
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
		ctx.noteProbeWrite(target)
		target.Value = Value{}
		target.Values = Value{}
		target.Materialized = false
		target.BindingDerived = false
	}

	// A binding linking one unspecified value determines nothing, so it is reported
	// only once no binding at any level determines the feature.
	var undetermined *UndeterminedBindingError
	var unmet []lower.Binding
	path := name
	for current := inst; current != nil; {
		bindings := ctx.bindingsOf(current, path)
		if len(bindings) != 0 {
			ctx.resolvingBindings[key] = true
			result, err := ctx.resolveBindingSet(current, inst, target, path, bindings, key)
			delete(ctx.resolvingBindings, key)
			var partial *UndeterminedBindingError
			if errors.As(err, &partial) {
				if undetermined == nil {
					undetermined = partial
				}
				err = nil
			}
			if err != nil || result.found {
				return result.value, result.found, err
			}
			unmet = append(unmet, result.unmet...)
			if len(result.cycleFeatures) != 0 {
				if err := ctx.unmetBindingCounts(unmet); err != nil {
					return Value{}, false, err
				}
				return Value{}, false, &BindingCycleError{Features: result.cycleFeatures}
			}
		}
		owner, ownerFeature := current.Owner()
		if owner == nil || ownerFeature == "" {
			break
		}
		path = ownerFeature + "." + path
		current = owner
	}
	if err := ctx.unmetBindingCounts(unmet); err != nil {
		return Value{}, false, err
	}
	if undetermined != nil {
		return Value{}, false, undetermined
	}
	return Value{}, false, nil
}

// bindingSetResult is what the bindings of one owner determine about a feature.
type bindingSetResult struct {
	value         Value
	found         bool
	cycleFeatures []string
	// unmet are the whole bindings that determined nothing, so their ends link no value.
	unmet []lower.Binding
}

// unmetBindingCounts refuses a whole binding whose ends link no value while one
// of them requires some: the feature and its other end both hold nothing.
func (ctx *Context) unmetBindingCounts(unmet []lower.Binding) error {
	for _, binding := range unmet {
		if err := ctx.wholeBindingCounts(binding, Value{}); err != nil {
			return err
		}
	}
	return nil
}

// claimBinding marks the binding as the one resolving key, so its own probe of the
// other end is not mistaken for a genuine cycle; the returned func releases the claim.
func (ctx *Context) claimBinding(key featureValueRef, decl *ast.Usage) func() {
	previousOwner, hadOwner := ctx.bindingOwners[key]
	ctx.bindingOwners[key] = decl
	return func() {
		if hadOwner {
			ctx.bindingOwners[key] = previousOwner
		} else {
			delete(ctx.bindingOwners, key)
		}
	}
}

func (ctx *Context) resolveBindingSet(owner, targetInst *Instance, target *FeatureValue,
	endpointName string, bindings []lower.Binding, key featureValueRef) (bindingSetResult, error) {
	var result bindingSetResult
	var attempts []bindingAttempt
	var undetermined *UndeterminedBindingError
	for _, binding := range bindings {
		partial, err := ctx.partialBinding(owner, targetInst, target, binding, key)
		if err != nil {
			return result, err
		}
		if partial {
			// A binding of one unspecified value per end constrains the ends without
			// determining either: the value comes from a default, a write or a whole binding.
			if undetermined == nil && target.Feature.DefaultValue == nil && !target.Written {
				undetermined = &UndeterminedBindingError{
					Target:   bindingLocationText(bindingLocation{instance: targetInst, name: key.feature}),
					Binding:  ctx.bindingText(binding),
					Endpoint: endpointName,
				}
			}
			continue
		}
		attempt := ctx.attemptBinding(owner, targetInst, target, binding, key)
		if attempt.err != nil {
			return result, attempt.err
		}
		if attempt.cycle {
			result.cycleFeatures = attempt.cycleFeatures
		}
		if !attempt.found {
			if attempt.settled {
				result.unmet = append(result.unmet, binding)
			}
			continue
		}
		if err := ctx.wholeBindingCounts(binding, attempt.value); err != nil {
			return result, err
		}
		if end := bindingEndForPath(binding, endpointName); end >= 0 {
			attempt.contributor = ctx.bindingEndpointText(binding, 1-end)
		}
		attempts = append(attempts, attempt)
	}
	// Every binding of the whole feature identifies it with its other end, so
	// two of them must agree on its values, a sequence like a scalar.
	if len(attempts) > 1 {
		for _, attempt := range attempts[1:] {
			if !valueEqual(attempts[0].value, attempt.value) {
				return result, &BindingConflictError{
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
			return result, err
		}
	}
	if len(attempts) != 0 {
		result.value, result.found = attempts[0].value, true
		return result, nil
	}
	if len(result.cycleFeatures) == 0 && undetermined != nil {
		return result, undetermined
	}
	return result, nil
}

// partialBinding reports an end admitting fewer values than its feature holds, which links one of them
// per link (KerML 1.0 §7.4.9.2); a feature holding fewer than an end's lower bound violates multiplicity.
func (ctx *Context) partialBinding(owner, targetInst *Instance, target *FeatureValue,
	binding lower.Binding, key featureValueRef) (bool, error) {
	defer ctx.claimBinding(key, binding.Decl)()
	partial := false
	for end := range binding.Ends {
		stated, ok := ctx.model.RangeOf(binding.Ends[end].Multiplicity)
		if !ok {
			continue
		}
		bounded := stated.Upper.Known && !stated.Upper.Infinite
		required := stated.Lower.Known && stated.Lower.Value > 0
		if !bounded && !required {
			continue
		}
		endpoint, err := ctx.resolveBindingEndpoint(owner, binding, end)
		if err != nil {
			return false, err
		}
		loc := endpoint.location
		if loc.instance == nil {
			continue
		}
		fv := loc.instance.FeatureValues[loc.name]
		declared := fv.Feature.Multiplicity.Upper
		narrows := bounded && (declared.Infinite || (declared.Known && declared.Value > stated.Upper.Value))
		if !narrows && !required {
			continue
		}
		// The end being resolved counts what it holds on its own: reading it through
		// its bindings is this very resolution, and cannot make the binding whole.
		carries := bindingLocationCarries(loc, target, targetInst)
		var val Value
		var found bool
		if carries {
			val, found, err = ctx.ownEndpointValue(loc)
		} else {
			val, found, err = ctx.bindingEndpointValue(endpoint, owner, false)
		}
		if err != nil {
			return false, err
		}
		count := int64(len(elementsOf(val)))
		if found && required && count < stated.Lower.Value {
			return false, ctx.bindingEndCountError(binding, end, stated, count)
		}
		if narrows && (carries || !found || count == 0 || count > stated.Upper.Value) {
			partial = true
		}
	}
	return partial, nil
}

// ownEndpointValue is what a feature holds without resolving its bindings: its
// written, defaulted or already-assigned value.
func (ctx *Context) ownEndpointValue(loc bindingLocation) (Value, bool, error) {
	fv := loc.instance.FeatureValues[loc.name]
	if !fv.Materialized && !fv.BindingDerived && ctx.CompositeTypeOf(fv.Feature) == nil {
		if _, err := loc.instance.materializeFeatureValueIntrinsic(ctx, loc.name); err != nil {
			return Value{}, false, err
		}
	}
	val := fv.HeldValue()
	return val, val.Kind != ValInvalid, nil
}

// wholeBindingCounts checks the value a whole binding identifies its ends with
// against the number of values each end states it links.
func (ctx *Context) wholeBindingCounts(binding lower.Binding, val Value) error {
	count := int64(len(elementsOf(val)))
	for end := range binding.Ends {
		stated, ok := ctx.model.RangeOf(binding.Ends[end].Multiplicity)
		if !ok {
			continue
		}
		if (stated.Lower.Known && count < stated.Lower.Value) ||
			(stated.Upper.Known && !stated.Upper.Infinite && count > stated.Upper.Value) {
			return ctx.bindingEndCountError(binding, end, stated, count)
		}
	}
	return nil
}

func (ctx *Context) bindingEndCountError(binding lower.Binding, end int, stated semantics.Range, count int64) error {
	return fmt.Errorf("%w: `%s` links %s of %s, which holds %d value(s)",
		ErrMultiplicityViolation, ctx.bindingText(binding), stated.Text(),
		ctx.bindingEndpointText(binding, end), count)
}

// bindingText spells a binding as written, end multiplicities included.
func (ctx *Context) bindingText(binding lower.Binding) string {
	ends := make([]string, len(binding.Ends))
	for i, end := range binding.Ends {
		ends[i] = ctx.bindingEndpointText(binding, i)
		if r, ok := ctx.model.RangeOf(end.Multiplicity); ok {
			ends[i] = r.Text() + " " + ends[i]
		}
	}
	return "bind " + strings.Join(ends, " = ")
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
	defer ctx.claimBinding(key, binding.Decl)()

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
	attempt.settled = !ctx.resolvingBindingEnd(left, leftCarries) && !ctx.resolvingBindingEnd(right, rightCarries)

	// Read the other end first: an object it already holds is what the
	// materializing end becomes, not a fresh object of its own.
	var leftValue, rightValue Value
	var leftSet, rightSet bool
	if leftCarries {
		rightValue, rightSet, err = ctx.bindingEndpointValue(right, owner, rightCarries)
		if err == nil {
			leftValue, leftSet, err = ctx.bindingEndpointValue(left, owner, !(rightSet && ctx.unmaterializedObjectEnd(left)))
		}
	} else {
		leftValue, leftSet, err = ctx.bindingEndpointValue(left, owner, leftCarries)
		if err == nil {
			rightValue, rightSet, err = ctx.bindingEndpointValue(right, owner, !(leftSet && ctx.unmaterializedObjectEnd(right)))
		}
	}
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

// resolvingBindingEnd reports an end other than the one being resolved whose own
// resolution is in progress, so what it holds is not yet known.
func (ctx *Context) resolvingBindingEnd(endpoint bindingEndpoint, carries bool) bool {
	loc := endpoint.location
	return !carries && loc.instance != nil &&
		ctx.resolvingBindings[featureValueRef{instance: loc.instance.ID, feature: loc.name}]
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
			return bindingLocation{}, fmt.Errorf("%w %q: %w", ErrBindingEnd, path, err)
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
	// A destroyed object's feature is neither read into nor written from a
	// binding, whichever end of it the object is.
	if err := ctx.checkNotDestroyed(current); err != nil {
		return bindingLocation{}, fmt.Errorf("%w %q: %w", ErrBindingEnd, path, err)
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

// unmaterializedObjectEnd reports whether an endpoint is an object-valued
// feature the run has not yet materialized, so materializing it would build a
// fresh object rather than read one.
func (ctx *Context) unmaterializedObjectEnd(endpoint bindingEndpoint) bool {
	loc := endpoint.location
	if loc.instance == nil {
		return false
	}
	fv := loc.instance.FeatureValues[loc.name]
	return !fv.Materialized && ctx.CompositeTypeOf(fv.Feature) != nil
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
	if err := ctx.checkDefault(inst, fv, name, val, admitDeclared); err != nil {
		return err
	}
	val, err := ctx.admitted(fv.Feature, val, admitDeclared)
	if err != nil {
		return err
	}
	ctx.noteProbeWrite(fv)
	if fv.Feature.Scalar() {
		fv.Value = val
		fv.Values = Value{}
	} else {
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
