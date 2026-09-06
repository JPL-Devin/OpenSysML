package runtime

import (
	"fmt"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// MeasurementRef is a ScalarMeasurementReference held as a value: the Unit a
// quantity carries (named product = declarations, term = reduction), by itself.
type MeasurementRef struct {
	Unit Unit
}

// NewMeasurementRefValue wraps a unit as a measurement reference value.
func NewMeasurementRefValue(unit Unit) Value {
	return Value{Kind: ValMeasurementRef, ref: &MeasurementRef{Unit: unit}}
}

// measurementRefOf is the reference of a unit, spelled by its product when the
// notation gave it no text of its own.
func measurementRefOf(unit Unit) Value {
	if unit.Text == "" && !unit.Product.IsEmpty() {
		unit.Text = unit.Product.String()
	}
	return NewMeasurementRefValue(unit)
}

// Declaration is the one unit declaration the reference names at the first
// power (`SI::m`); nil for a unit composed of several (`m/s`, `m**2`).
func (r *MeasurementRef) Declaration() *symbols.Symbol {
	if r == nil || len(r.Unit.Product.Powers) != 1 {
		return nil
	}
	power := r.Unit.Product.Powers[0]
	if power.Exponent != 1 {
		return nil
	}
	return power.Unit
}

// String renders the reference as a quantity's unit is written: `m`, `km/h`, `m**2`.
func (r *MeasurementRef) String() string {
	if r == nil {
		return "<unknown>"
	}
	if r.Unit.Product.IsEmpty() {
		return r.Unit.Term.String()
	}
	return r.Unit.String()
}

// equal holds for one reduction at one scale (`SI::'m/s' == m/s`, `km/m == m/mm`);
// a named dimension-one unit reduces to nothing, so it is only itself (`rad != sr`).
func (r *MeasurementRef) equal(other *MeasurementRef) bool {
	if r == nil || other == nil {
		return r == other
	}
	if !r.Unit.Term.Same(other.Unit.Term) {
		return false
	}
	if r.namesDimensionOne() || other.namesDimensionOne() {
		return r.Unit.Product.Equal(other.Unit.Product)
	}
	return true
}

// key identifies the reference the way equal compares it, for a set or map: the
// scale as one number, so `0.5` and `1/2` key alike as they compare alike.
func (r *MeasurementRef) key() string {
	if r == nil {
		return ""
	}
	scale := r.Unit.Term.Scale
	key := r.Unit.Term.DimensionKey() + "@" + strconv.FormatFloat(scale.Num/scale.Den, 'g', -1, 64)
	if r.namesDimensionOne() {
		key += "|" + r.Unit.Product.String()
	}
	return key
}

// namesDimensionOne reports a unit of dimension one by name (`rad`, `one`), not a
// ratio that cancels to a number (`km/m`).
func (r *MeasurementRef) namesDimensionOne() bool {
	return r.Unit.Term.Dimensionless() && r.Unit.Product.NamesDimensionOne()
}

// MeasurementUnitValue is the reference a measurement-unit declaration names
// (true for one). A library unit is the declaration whatever defines it (`'m/s'
// = m/s`); a model feature typed by a unit and given a value holds that value
// instead (false).
func (ctx *Context) MeasurementUnitValue(sym *symbols.Symbol) (Value, bool, error) {
	if sym == nil || ctx.model == nil || !ctx.model.IsMeasurementUnit(sym) {
		return Value{}, false, nil
	}
	if !ctx.libraryDeclared(sym) && ctx.extractDefaultValue(sym) != nil {
		return Value{}, false, nil
	}
	term, err := ctx.model.UnitTermOf(sym)
	if err != nil {
		return Value{}, true, fmt.Errorf("%w: %s: %w", ErrNotAQuantity, sym.Name, err)
	}
	name := unitSymbolName(sym)
	product := semantics.NamedUnitProduct(sym, name, term.Dimensionless())
	return NewMeasurementRefValue(Unit{Text: name, Product: product, Term: term}), true, nil
}

// unitSymbolName is the symbol a unit is written by (`km`), its name otherwise.
func unitSymbolName(sym *symbols.Symbol) string {
	if sym.ShortName != "" {
		return lexer.NameText(sym.ShortName)
	}
	return lexer.NameText(sym.Name)
}

// composeMeasurementRefs is the unit a `*`, `/` or `**`/`^` (Real exponent) of
// references names, as MeasurementRefCalculations declares; false otherwise.
func composeMeasurementRefs(op ast.OperatorKind, left, right Value) (Value, bool) {
	if left.Kind != ValMeasurementRef {
		return Value{}, false
	}
	x := left.MeasurementRef().Unit
	switch op {
	case ast.OpMul, ast.OpDiv:
		if right.Kind != ValMeasurementRef {
			return Value{}, false
		}
		y := right.MeasurementRef().Unit
		if op == ast.OpMul {
			return measurementRefOf(Unit{Product: x.Product.Times(y.Product), Term: x.Term.Times(y.Term)}), true
		}
		return measurementRefOf(Unit{Product: x.Product.DividedBy(y.Product), Term: x.Term.DividedBy(y.Term)}), true
	case ast.OpPow:
		if right.Kind != ValConst || !right.Const.IsNumeric() {
			return Value{}, false
		}
		exponent := right.Const.AsReal()
		return measurementRefOf(Unit{Product: x.Product.Pow(exponent), Term: x.Term.Pow(exponent)}), true
	}
	return Value{}, false
}

// Library types and features a measurement reference or scalar quantity answers.
const (
	scalarMRefTypeFQN        = "MeasurementReferences::ScalarMeasurementReference"
	derivedUnitFQN           = "MeasurementReferences::DerivedUnit"
	scalarQuantityTypeFQN    = "Quantities::ScalarQuantityValue"
	mRefIsBoundFeature       = "isBound"
	mRefMRefsFeature         = "mRefs"
	mRefOrthogonalFeature    = "isOrthogonal"
	tensorContravariantOrder = "contravariantOrder"
	tensorCovariantOrder     = "covariantOrder"
)

// measurementRefFeature answers the Array/TensorMeasurementReference features a
// scalar reference fixes; the declaration's other members are a typed error.
func (ctx *Context) measurementRefFeature(val Value, name string) (Value, bool, error) {
	ref := val.MeasurementRef()
	switch name {
	case arrayDimensionsFeature:
		return ctx.answerSequence(nil)
	case arrayRankFeature:
		return integerValue(0), true, nil
	case arrayFlattenedSizeFeature:
		return integerValue(1), true, nil
	case arrayElementsFeature, mRefMRefsFeature:
		return ctx.answerSequence([]Value{val})
	case mRefIsBoundFeature:
		return boolValue(false), true, nil
	case mRefOrthogonalFeature:
		return boolValue(true), true, nil
	}
	if !ctx.measurementRefDeclares(ref, name) {
		return Value{}, false, nil
	}
	return Value{}, true, fmt.Errorf(
		"%w: %s::%s: a measurement reference value holds the unit %s and its reduction %s, not the declaration's member %s",
		ErrUnevaluableLibraryFunction, scalarMRefTypeFQN, name, ref, ref.Unit.Term, name,
	)
}

// measurementRefDeclares reports whether name is a member of the reference's
// declaration or of every scalar measurement reference.
func (ctx *Context) measurementRefDeclares(ref *MeasurementRef, name string) bool {
	if decl := ref.Declaration(); decl != nil {
		if _, ok := ctx.model.LookupMember(decl, name); ok {
			return true
		}
	}
	typ, err := ctx.loadedLibraryType(scalarMRefTypeFQN)
	if err != nil {
		return false
	}
	_, ok := ctx.model.LookupMember(typ, name)
	return ok
}

// quantityFeature answers a scalar quantity's `num`, `mRef` and the tensor and
// Array features a scalar fixes (order 0, one element); isBound is not held.
func (ctx *Context) quantityFeature(val Value, name string) (Value, bool, error) {
	q := val.Quantity()
	switch name {
	case vectorQuantityNumFeature:
		return Value{Kind: ValConst, Const: q.Num}, true, nil
	case vectorQuantityMRefFeature:
		return measurementRefOf(q.Unit), true, nil
	case arrayDimensionsFeature:
		return ctx.answerSequence(nil)
	case arrayRankFeature, tensorOrderFeature, tensorContravariantOrder, tensorCovariantOrder:
		return integerValue(0), true, nil
	case arrayFlattenedSizeFeature:
		return integerValue(1), true, nil
	case arrayElementsFeature:
		return ctx.answerSequence([]Value{{Kind: ValConst, Const: q.Num}})
	case mRefIsBoundFeature:
		return Value{}, true, fmt.Errorf(
			"%w: %s::isBound: a scalar quantity value holds its num and mRef, not whether the quantity is bound",
			ErrUnevaluableLibraryFunction, scalarQuantityTypeFQN,
		)
	}
	return Value{}, false, nil
}

// vectorQuantityMRef is the one reference a vector quantity's axes share, however
// spelt; axes in different units have no scalar reference faithful to all, a typed error.
func vectorQuantityMRef(vq *VectorQuantity) (Value, error) {
	unit, ok := vq.sharedUnit()
	if !ok {
		return Value{}, fmt.Errorf(
			"%w: Quantities::VectorQuantityValue::mRef: the axes of %s carry different units, and no one measurement reference names them all",
			ErrUnevaluableLibraryFunction, vq.format(semantics.FormatConst),
		)
	}
	return measurementRefOf(unit), nil
}

// measurementRefValueType is the declaration's type (`LengthUnit`), or
// DerivedUnit for a unit composed of powers of several declarations.
func (ctx *Context) measurementRefValueType(ref *MeasurementRef) (*symbols.Symbol, error) {
	if decl := ref.Declaration(); decl != nil {
		if typ := ctx.extractType(decl); typ != nil {
			return typ, nil
		}
	}
	return ctx.loadedLibraryType(derivedUnitFQN)
}

// measurementRefConforms judges a reference written to a feature of the declared
// type as the checker does: by its type, or by dimension against a unit definition.
func (ctx *Context) measurementRefConforms(ref *MeasurementRef, declared *symbols.Symbol) (bool, string, error) {
	typ, err := ctx.measurementRefValueType(ref)
	if err != nil {
		return false, "", err
	}
	c := ctx.model.MeasurementRefConforms(typ, ref.Unit.Term, declared)
	if !c.Known || c.Holds {
		return true, "", nil
	}
	return false, fmt.Sprintf("cannot write the measurement reference %s, %s, to a feature typed by %s",
		ref, c.Found, symbolText(declared)), nil
}
