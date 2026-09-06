package runtime

import (
	"fmt"
	"math"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// Quantity is a scalar quantity value, the representation the semantic layer
// shares with the query engine: a magnitude in a measurement reference.
type Quantity = semantics.Quantity

// Unit is a measurement reference as a quantity carries it.
type Unit = semantics.Unit

// quantityResult is the runtime value of a quantity computation: a number when
// its unit cancelled, else the quantity.
func quantityResult(q semantics.Quantity, err error) (Value, error) {
	if err != nil {
		return Value{}, err
	}
	if q.Unit.None() {
		return Value{Kind: ValConst, Const: q.Num}, nil
	}
	return NewQuantityValue(&q), nil
}

// composedQuantity is a result in the canonical form of its composed unit (`m**2`, `m`).
func composedQuantity(num semantics.Value, product semantics.UnitProduct, term semantics.UnitTerm) (Value, error) {
	return quantityResult(semantics.ComposedQuantity(num, product, term))
}

// evalIndexExpr evaluates `magnitude [unit]`, the quantity expression: a
// number and the measurement reference it is expressed in. The sequence index
// `seq#(i)` shares the node but not the meaning; the bracket the notation was
// written with is what tells the two apart, and the index is evaluated as the
// sequence operation it is (collections.go).
func (ec *EvalContext) evalIndexExpr(n *ast.IndexExpr) (Value, error) {
	if !n.Bracket {
		return ec.evalSequenceIndex(n)
	}
	term, err := ec.ctx.model.UnitTermOfExpr(ec.scope, n.Index)
	if err != nil {
		return Value{}, ec.notAQuantityError(n, err)
	}

	magnitude, err := ec.valueOperand(n.Operand)
	if err != nil {
		return Value{}, err
	}
	if magnitude.Kind != ValVector && (magnitude.Kind != ValConst || !magnitude.Const.IsNumeric()) {
		return Value{}, fmt.Errorf("%w: magnitude of a quantity is %s, want a number or a vector", ErrNotAQuantity, magnitude.Kind)
	}

	product, err := ec.ctx.model.UnitProductOfExpr(ec.scope, n.Index)
	if err != nil {
		return Value{}, fmt.Errorf("%w: %w", ErrNotAQuantity, err)
	}
	unit := Unit{Text: semantics.UnitExprText(n.Index), Product: product, Term: term}
	if magnitude.Kind == ValVector {
		// A vector in a unit is the vector quantity with that unit on every axis.
		num := magnitude.Vector().Elements
		units := make([]Unit, len(num))
		for i := range units {
			units[i] = unit
		}
		return ec.ctx.vectorQuantityValue(num, units)
	}
	return NewQuantityValue(&Quantity{Num: magnitude.Const, Unit: unit}), nil
}

// notAQuantityError reports a bracket whose index is no unit; over an operand
// declared a collection it adds that `#(…)` indexes. The operand is not evaluated.
func (ec *EvalContext) notAQuantityError(n *ast.IndexExpr, cause error) error {
	err := fmt.Errorf("%w: %w", ErrNotAQuantity, cause)
	if what, ok := ec.declaredCollection(n.Operand); ok {
		return fmt.Errorf("%w; `[…]` is the quantity notation `num [unit]`, index %s with `#(…)`", err, what)
	}
	return err
}

// declaredCollection names the collection an operand's declaration makes it:
// an Array, a vector, or a feature of more than one value.
func (ec *EvalContext) declaredCollection(operand ast.Node) (string, bool) {
	var qn *ast.QualifiedName
	switch node := operand.(type) {
	case *ast.QualifiedName:
		qn = node
	case *ast.FeatureReference:
		qn = node.Name
	}
	if qn == nil || ec.ctx.resolver == nil {
		return "", false
	}
	sym, ok := ec.ctx.resolver.ResolveQualified(ec.scope, qn)
	if !ok || sym == nil || !semantics.IsShapeFeature(sym) {
		return "", false
	}
	if typ := ec.ctx.extractType(sym); typ != nil {
		for _, lib := range []struct{ fqn, what string }{{vectorTypeFQN, "a vector"}, {arrayTypeFQN, "an array"}} {
			if libSym := ec.ctx.librarySymbol(lib.fqn); libSym != nil && ec.ctx.model.Conforms(typ, libSym) {
				return lib.what, true
			}
		}
	}
	if !ec.ctx.occursOnce(sym) {
		return "a sequence", true
	}
	return "", false
}

// asQuantity views a value as a quantity: a quantity as itself, and a bare
// number as a magnitude of dimension one, since a number is commensurable with
// a count or a ratio of like quantities but with nothing else.
func asQuantity(val Value) (*Quantity, bool) {
	switch val.Kind {
	case ValQuantity:
		return val.Quantity(), true
	case ValConst:
		if val.Const.IsNumeric() {
			return &Quantity{Num: val.Const, Unit: semantics.UnitOne()}, true
		}
	}
	return nil, false
}

// inUnit is a magnitude in the unit of a quantity asQuantity read; the unit a bare
// number was read with is no unit at all, so the result is a bare number again.
func inUnit(num semantics.Value, unit Unit) (Value, error) {
	return quantityResult(semantics.InUnit(num, unit))
}

// quantityOperands views both operands of an operation as quantities, reporting
// false when neither is one — then the operation is ordinary arithmetic.
func quantityOperands(left, right Value) (*Quantity, *Quantity, bool) {
	if left.Kind != ValQuantity && right.Kind != ValQuantity {
		return nil, nil, false
	}
	lq, lok := asQuantity(left)
	rq, rok := asQuantity(right)
	if !lok || !rok {
		return nil, nil, false
	}
	return lq, rq, true
}

// comparedOperands is quantityOperands for a comparison: a bare zero is the null
// quantity of every dimension, so `length > 0` reads it in the other's unit.
func comparedOperands(left, right Value) (*Quantity, *Quantity, bool) {
	lq, rq, ok := quantityOperands(left, right)
	if !ok {
		return nil, nil, false
	}
	lq, rq = adoptZeroUnit(left, right, lq, rq)
	return lq, rq, true
}

// adoptZeroUnit reads a bare-zero operand in the other operand's unit.
func adoptZeroUnit(left, right Value, lq, rq *Quantity) (*Quantity, *Quantity) {
	switch {
	case bareZero(left):
		lq = &Quantity{Num: lq.Num, Unit: rq.Unit}
	case bareZero(right):
		rq = &Quantity{Num: rq.Num, Unit: lq.Unit}
	}
	return lq, rq
}

// bareZero reports a number, not a quantity, whose value is zero.
func bareZero(val Value) bool {
	return val.Kind == ValConst && val.Const.IsNumeric() && toReal(val.Const) == 0
}

// addQuantities evaluates a sum or difference of quantities, in the unit of the
// left operand (a bare number where that is one).
func addQuantities(op ast.OperatorKind, left, right *Quantity) (Value, error) {
	return quantityResult(semantics.AddQuantities(op, *left, *right))
}

// scaleQuantities evaluates a product or quotient of quantities, whose unit is
// the product or quotient of theirs — `10 [m] / 2 [s]` is `5 [m/s]`.
func scaleQuantities(op ast.OperatorKind, left, right *Quantity) (Value, error) {
	return quantityResult(semantics.ScaleQuantities(op, *left, *right))
}

// powQuantity raises a quantity to a constant exponent, its unit included.
func powQuantity(base *Quantity, exponent semantics.Value) (Value, error) {
	if !exponent.IsNumeric() {
		return Value{}, fmt.Errorf("%w: exponent of a quantity is not a number", ErrTypeMismatch)
	}
	return quantityResult(semantics.PowQuantity(*base, exponent))
}

// sqrtQuantity is the square root of a quantity, `9 [m**2]` giving `3.0 [m]`;
// a unit with a base unit at an odd power has no root, so `sqrt(9 [m])` is rejected.
// A root the named units cannot spell at whole powers (`km*m`) is taken over the
// base units instead, unless a dimension-one unit (`rad`, `°`) would be lost there.
func sqrtQuantity(q *Quantity) (Value, error) {
	for _, f := range q.Unit.Term.Factors {
		if math.Mod(f.Exponent, 2) != 0 {
			return Value{}, fmt.Errorf("%w: %s (%s) raises %s to the odd power %g",
				ErrUnitRoot, q.Unit, q.Unit.Term, f.Unit.Name, f.Exponent)
		}
	}
	magnitude, term := toReal(q.Num), q.Unit.Term
	if magnitude < 0 {
		return Value{}, fmt.Errorf("%w: sqrt of a negative quantity %s", semantics.ErrArithmeticDomain, q)
	}
	root, ok := q.Unit.Product.Root(2)
	if !ok {
		for _, f := range q.Unit.Product.Powers {
			if f.DimensionOne && math.Mod(f.Exponent, 2) != 0 {
				return Value{}, fmt.Errorf("%w: %s raises the dimension-one unit %s to the odd power %g",
					ErrUnitRoot, q.Unit, f.Name, f.Exponent)
			}
		}
		magnitude = q.BaseMagnitude()
		term = semantics.UnitTerm{Scale: semantics.UnitScale(1), Factors: term.Factors}
		root, _ = term.BaseProduct().Root(2)
	}
	num, err := semantics.RealResult(math.Sqrt(magnitude))
	if err != nil {
		return Value{}, err
	}
	return composedQuantity(num, root, term.Pow(0.5))
}

// compareQuantities orders two quantities, converting the right one into the
// left one's unit.
func compareQuantities(op ast.OperatorKind, left, right *Quantity) (Value, error) {
	result, err := semantics.CompareQuantities(op, *left, *right)
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: result}}, nil
}

// equalQuantities compares two quantities for equality, in the left one's unit.
// Incommensurable units are an error rather than an inequality: they measure
// different things, so neither `==` nor `!=` is an answer about them.
func equalQuantities(op ast.OperatorKind, left, right *Quantity) (Value, error) {
	equal, err := semantics.EqualQuantities(op, *left, *right)
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: equal}}, nil
}

// negateQuantity negates a quantity's magnitude, keeping its unit and the
// magnitude's kind.
func negateQuantity(q *Quantity) (Value, error) {
	return quantityResult(semantics.NegateQuantity(*q))
}
