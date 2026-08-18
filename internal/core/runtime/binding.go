package runtime

import (
	"fmt"
	"strings"

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

func (ctx *Context) resolveBindingValue(inst *Instance, name string) (Value, bool, error) {
	bindings := ctx.objectBindings(inst.Type)
	if len(bindings) == 0 {
		return Value{}, false, nil
	}
	target, ok := inst.FeatureValues[name]
	if !ok {
		return Value{}, false, nil
	}
	key := featureValueRef{instance: inst.ID, feature: name}
	if ctx.resolvingBindings[key] {
		return Value{}, false, &BindingCycleError{Features: []string{bindingLocationText(bindingLocation{instance: inst, name: name})}}
	}
	ctx.resolvingBindings[key] = true
	defer delete(ctx.resolvingBindings, key)

	cycleSeen := false
	var cycleFeatures []string
	for _, binding := range bindings {
		left, err := ctx.resolveBindingLocation(inst, binding, 0)
		if err != nil {
			return Value{}, false, err
		}
		right, err := ctx.resolveBindingLocation(inst, binding, 1)
		if err != nil {
			return Value{}, false, err
		}
		if !bindingLocationCarries(left, target, inst) && !bindingLocationCarries(right, target, inst) {
			continue
		}

		leftValue, leftSet, err := ctx.bindingEndpointValue(left)
		if err != nil {
			return Value{}, false, err
		}
		if ctx.resolvingBindings[featureValueRef{instance: left.instance.ID, feature: left.name}] {
			cycleSeen = true
		}
		rightValue, rightSet, err := ctx.bindingEndpointValue(right)
		if err != nil {
			return Value{}, false, err
		}
		if ctx.resolvingBindings[featureValueRef{instance: right.instance.ID, feature: right.name}] {
			cycleSeen = true
		}
		if cycleSeen {
			cycleFeatures = []string{bindingLocationText(left), bindingLocationText(right)}
		}
		switch {
		case leftSet && rightSet:
			if valueKeyFunc(leftValue) != valueKeyFunc(rightValue) {
				return Value{}, false, &BindingConflictError{
					Left: bindingLocationText(left), Right: bindingLocationText(right),
					LeftValue: leftValue, RightValue: rightValue,
				}
			}
			if bindingLocationCarries(left, target, inst) {
				return leftValue, true, nil
			}
			return rightValue, true, nil
		case leftSet:
			ctx.assignBindingLocation(left, leftValue)
			ctx.assignBindingLocation(right, leftValue)
			return leftValue, true, nil
		case rightSet:
			ctx.assignBindingLocation(left, rightValue)
			ctx.assignBindingLocation(right, rightValue)
			return rightValue, true, nil
		}
	}
	if cycleSeen {
		return Value{}, false, &BindingCycleError{Features: cycleFeatures}
	}
	return Value{}, false, nil
}

func (ctx *Context) resolveBindingLocation(owner *Instance, binding lower.Binding, end int) (bindingLocation, error) {
	path := binding.Ends[end].Path
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

func (ctx *Context) bindingEndpointValue(loc bindingLocation) (Value, bool, error) {
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

func (ctx *Context) assignBindingLocation(loc bindingLocation, val Value) {
	fv := loc.instance.FeatureValues[loc.name]
	fv.Value = val
	fv.Values = Value{}
	fv.Materialized = true
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
	return fmt.Sprintf("%s.%s", loc.instance.Type.Name, loc.name)
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
