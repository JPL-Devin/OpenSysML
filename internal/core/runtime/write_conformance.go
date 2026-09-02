package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// writeTarget is the declaration a written value answers to: the type the
// feature written was declared with, and the multiplicity governing how many
// values it holds.
type writeTarget struct {
	name string
	typ  *symbols.Symbol
	mult semantics.Range
}

// writeTargetKey identifies a target by where it was written and the name it
// wrote, which resolve to one declaration however often the write runs.
type writeTargetKey struct {
	scope *symbols.Scope
	name  string
}

// writeTargetIn resolves the feature an assignment names in the scope the
// statement was written in, memoized per scope and name. A name declaring no
// feature there — a value the body merely holds — states nothing to conform to.
func (ctx *Context) writeTargetIn(scope *symbols.Scope, name string) (*writeTarget, bool) {
	if ctx.resolver == nil || scope == nil || name == "" {
		return nil, false
	}
	key := writeTargetKey{scope: scope, name: name}
	if cached, ok := ctx.writeTargets[key]; ok {
		return cached, cached != nil
	}
	var target *writeTarget
	if sym, ok := ctx.resolver.LookupName(scope, name); ok && sym != nil && isFeature(sym) {
		mult, _ := ctx.extractMultiplicity(sym)
		target = &writeTarget{name: name, typ: ctx.extractType(sym), mult: mult}
	}
	if ctx.writeTargets == nil {
		ctx.writeTargets = make(map[writeTargetKey]*writeTarget)
	}
	ctx.writeTargets[key] = target
	return target, target != nil
}

// checkWrite reports a value that does not conform to the declaration of the
// feature written. KerML's FeatureWritePerformance "assigns the values of a
// feature on an occurrence to the given replacementValues", so those values are
// values of that feature and answer to its type and its multiplicity — the rule
// binding an initial value (passes.checkBoundValue, Context.checkDefaultCount),
// applied where a write replaces them.
func (ctx *Context) checkWrite(scope *symbols.Scope, what string, target *writeTarget, value Value) error {
	if target == nil {
		return nil
	}
	if msg := ctx.writeCountRefusal(target, &value); msg != "" {
		return fmt.Errorf("%s: %w: %s", what, ErrMultiplicityViolation, msg)
	}
	return ctx.checkWriteType(scope, what, target.typ, value)
}

// writeCountRefusal says why the number of values written is outside the
// target's multiplicity, or is empty where the count is admitted.
func (ctx *Context) writeCountRefusal(target *writeTarget, value *Value) string {
	return target.mult.CountViolation(ctx.boundValueCount(target.typ, value))
}

// boundValueCount counts the values of the declared type a bound value holds: its
// elements, except that the (re, im) pair representing one Complex counts as one.
func (ctx *Context) boundValueCount(declared *symbols.Symbol, value *Value) int64 {
	count := elementCount(value)
	if count == 2 && isNumericVector(*value) && ctx.model.PrimTypeOf(declared) == semantics.PrimComplex {
		return 1
	}
	return count
}

// checkBodyWrite checks a write of a value a behavior body itself holds - a
// block-local, a parameter, an output - against the declaration of the name it
// writes, before that value is stored.
func (ctx *Context) checkBodyWrite(host stmtHost, s lower.Assign, value Value) error {
	return ctx.checkNamedWrite(s.Scope, host.describe(), s.Target, value)
}

// checkNamedWrite checks a write of a name resolved in scope, for a path that
// stores the value itself rather than reaching Instance.SetFeatureValue.
func (ctx *Context) checkNamedWrite(scope *symbols.Scope, where, name string, value Value) error {
	return ctx.checkBoundName(scope, fmt.Sprintf("%s: assignment to %s", where, name), name, value)
}

// checkBoundName checks a value bound to the feature name declares in scope,
// described by what: an assignment, or a binding that gives a value to an
// output the run time computes.
func (ctx *Context) checkBoundName(scope *symbols.Scope, what, name string, value Value) error {
	target, ok := ctx.writeTargetIn(scope, name)
	if !ok {
		return nil
	}
	return ctx.checkWrite(scope, what, target, value)
}

// storeBodyValue writes a value into the behavior's own data once it conforms
// to the declaration of the name written.
func storeBodyValue(ctx *Context, host stmtHost, env *stmtEnv, name string, value Value, s lower.Assign) error {
	if err := ctx.checkBodyWrite(host, s, value); err != nil {
		return err
	}
	env.data[name] = value
	return nil
}

// checkWriteType reports an element of a written value that no feature of the
// declared type could hold. A target declaring no type holds anything, and a
// value whose type the run time cannot name is not judged here.
func (ctx *Context) checkWriteType(scope *symbols.Scope, what string, declared *symbols.Symbol, value Value) error {
	if refusal, refused := ctx.writeTypeRefusal(scope, declared, &value); refused {
		return fmt.Errorf("%s: %w: %s", what, ErrTypeMismatch, refusal)
	}
	return nil
}

// writeTypeRefusal says why the first element no feature of the declared type
// could hold is refused; the second result is false where every element conforms.
func (ctx *Context) writeTypeRefusal(scope *symbols.Scope, declared *symbols.Symbol, value *Value) (string, bool) {
	if declared == nil {
		return "", false
	}
	switch value.Kind {
	case ValSequence, ValSet:
		elements := elementsOf(*value)
		for i := range elements {
			if refusal, refused := ctx.elementRefusal(scope, declared, &elements[i]); refused {
				return refusal, true
			}
		}
		return "", false
	}
	return ctx.elementRefusal(scope, declared, value)
}

// elementRefusal says why one element is refused by a feature of the declared type.
func (ctx *Context) elementRefusal(scope *symbols.Scope, declared *symbols.Symbol, element *Value) (string, bool) {
	conforms, refusal, err := ctx.valueConforms(scope, element, declared)
	if err != nil || conforms {
		return "", false
	}
	if refusal == "" {
		refusal = fmt.Sprintf("cannot write %s (%s) to a feature typed by %s",
			FormatValue(*element), describeValue(*element), symbolText(declared))
	}
	return refusal, true
}

// valueConforms reports whether a feature of the declared type may hold the
// value, by the relation the type tier applies to an initial value: the scalar
// lattice where the target is a scalar type, specialization otherwise. The
// second result says why a value was refused where the general message would
// not say it, and is empty otherwise.
func (ctx *Context) valueConforms(scope *symbols.Scope, value *Value, declared *symbols.Symbol) (bool, string, error) {
	switch value.Kind {
	case ValNull, ValInvalid:
		// Holds no value to type; how many values a feature may hold is the
		// multiplicity's to decide.
		return true, "", nil
	case ValQuantity:
		return ctx.quantityConforms(*value, declared)
	}
	prim := ctx.model.PrimTypeOf(declared)
	if got := valuePrimType(value); prim != semantics.PrimUnknown && got != semantics.PrimUnknown {
		return semantics.PrimConforms(got, prim), "", nil
	}
	// Outside the lattice, a constant's direct type is known by name only, so a
	// target specializing it may still hold the value; a disjoint one cannot.
	direct, err := ctx.directValueType(scope, *value)
	if err != nil {
		return false, "", err
	}
	if ctx.model.Conforms(direct, declared) {
		return true, "", nil
	}
	if prim == semantics.PrimUnknown && isScalarConstant(value) {
		return ctx.model.Conforms(declared, direct), "", nil
	}
	return false, "", nil
}

// isScalarConstant reports a value written as one scalar constant.
func isScalarConstant(value *Value) bool {
	return value.Kind == ValConst || value.Kind == ValString
}

// quantityConforms judges a written quantity: a scalar target by the lattice, a
// target declaring a quantity value type by the dimension that type's mRef
// fixes. A target fixing no dimension, and a unit fixing none, are not judged.
func (ctx *Context) quantityConforms(value Value, declared *symbols.Symbol) (bool, string, error) {
	if prim := ctx.model.PrimTypeOf(declared); prim != semantics.PrimUnknown {
		return semantics.PrimConforms(semantics.PrimRational, prim), "", nil
	}
	want, ok := ctx.model.DimensionOfType(declared)
	if !ok || value.Quantity() == nil {
		return true, "", nil
	}
	got, ok := ctx.model.DimensionOfUnit(value.Quantity().Unit.Term)
	if !ok || want.Term.Commensurable(got.Term) {
		return true, "", nil
	}
	return false, fmt.Sprintf("cannot write %s (%s) to a feature typed by %s (%s)",
		FormatValue(value), dimensionText(got), symbolText(declared), dimensionText(want)), nil
}

// dimensionText names a dimension as a message about a mismatch reads it.
func dimensionText(d semantics.Dimension) string {
	if d.Term.Dimensionless() {
		return "dimensionless"
	}
	return "dimension " + d.String()
}

// valuePrimType classifies a value against the scalar lattice by the value itself
// (4 / 2 is an Integer, 7 / 2 a Rational); outside the lattice it is PrimUnknown.
func valuePrimType(value *Value) semantics.PrimType {
	switch value.Kind {
	case ValConst:
		return semantics.PrimTypeOfValue(value.Const)
	case ValString:
		return semantics.PrimString
	}
	return semantics.PrimUnknown
}
