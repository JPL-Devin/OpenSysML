package runtime

import (
	"fmt"
	"math"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// Quantity is a scalar quantity value: a magnitude and the measurement
// reference it is expressed in (Quantities::ScalarQuantityValue is exactly a
// number `num` and a reference `mRef`). The unit travels with the value, so
// `1.5 [m/s]` is never mistaken for `1.5 [km/h]`.
type Quantity struct {
	Num  semantics.Value
	Unit Unit
}

// Unit is a measurement reference as a quantity carries it: its text as written or
// canonically composed, its product of named units, and its reduction to base units.
type Unit struct {
	Text    string
	Product semantics.UnitProduct
	Term    semantics.UnitTerm
}

// String renders the unit by its text, falling back to its reduction for a
// unit that has none.
func (u Unit) String() string {
	if u.Text != "" {
		return u.Text
	}
	return u.Term.String()
}

// product is the named units the unit is a product of. A unit known by its
// text alone — one rebuilt from the wire — is that name to the first power.
func (u Unit) product() semantics.UnitProduct {
	if !u.Product.IsEmpty() || u.Text == "" {
		return u.Product
	}
	return semantics.NamedUnitProduct(nil, u.Text, u.Term.Dimensionless())
}

// composedQuantity is a result in the canonical form of its composed unit (`m**2`, `m`).
// A unit that cancels leaves a number unless it names a dimension-one unit (`rad`).
func composedQuantity(num semantics.Value, product semantics.UnitProduct, term semantics.UnitTerm) (Value, error) {
	if term.Dimensionless() && !product.NamesDimensionOne() {
		return dimensionlessValue(num, term)
	}
	unit := Unit{Text: product.String(), Product: product, Term: term}
	return NewQuantityValue(&Quantity{Num: num, Unit: unit}), nil
}

// String renders the quantity as a magnitude in its unit: `1.5 [m/s]`.
func (q *Quantity) String() string {
	return q.TextWithMagnitude(constText(q.Num))
}

// TextWithMagnitude renders the quantity from an already-rendered magnitude, so
// a caller with its own convention for numbers — a trace, which distinguishes a
// whole Real from an Integer, or a result table, which rounds a Real for
// display — keeps it and still names the unit the same way. The stored
// magnitude is untouched.
func (q *Quantity) TextWithMagnitude(magnitude string) string {
	return fmt.Sprintf("%s [%s]", magnitude, q.Unit)
}

// baseMagnitude is the quantity's magnitude expressed over the base units its
// unit reduces to, which is the form two commensurable quantities compare in.
func (q *Quantity) baseMagnitude() float64 {
	return semantics.ConvertMagnitude(toReal(q.Num), q.Unit.Term.Scale, semantics.UnitScale(1))
}

// constText renders a numeric constant without a unit.
func constText(v semantics.Value) string {
	switch v.Kind {
	case semantics.ValInt:
		return fmt.Sprintf("%d", v.Int)
	case semantics.ValReal:
		return FormatReal(v.Real)
	case semantics.ValBool:
		return fmt.Sprintf("%v", v.Bool)
	case semantics.ValInfinity:
		return "∞"
	default:
		return "<invalid>"
	}
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
		return Value{}, fmt.Errorf("%w: %w", ErrNotAQuantity, err)
	}

	magnitude, err := ec.Eval(n.Operand)
	if err != nil {
		return Value{}, err
	}
	if magnitude.Kind != ValConst || !magnitude.Const.IsNumeric() {
		return Value{}, fmt.Errorf("%w: magnitude of a quantity is %s, want a number", ErrNotAQuantity, magnitude.Kind)
	}

	product, err := ec.ctx.model.UnitProductOfExpr(ec.scope, n.Index)
	if err != nil {
		return Value{}, fmt.Errorf("%w: %w", ErrNotAQuantity, err)
	}
	unit := Unit{Text: semantics.UnitExprText(n.Index), Product: product, Term: term}
	return NewQuantityValue(&Quantity{Num: magnitude.Const, Unit: unit}), nil
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
			return &Quantity{Num: val.Const, Unit: Unit{Term: semantics.UnitTerm{Scale: semantics.UnitScale(1)}}}, true
		}
	}
	return nil, false
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

// convertTo expresses the quantity in unit, which requires the two units to be
// commensurable: their reductions must be over the same base units, or the
// magnitudes measure different things and no factor relates them.
func (q *Quantity) convertTo(unit Unit) (float64, error) {
	if !q.Unit.Term.Commensurable(unit.Term) {
		return 0, fmt.Errorf("%w: cannot express %s (%s) in %s (%s)", ErrIncommensurableUnits,
			q.Unit, q.Unit.Term, unit, unit.Term)
	}
	if unit.Term.Scale.IsZero() {
		return 0, fmt.Errorf("%w: %s reduces to a zero scale factor", ErrIncommensurableUnits, unit)
	}
	return semantics.ConvertMagnitude(toReal(q.Num), q.Unit.Term.Scale, unit.Term.Scale), nil
}

// addQuantities evaluates a sum or difference of quantities, in the unit of the
// left operand; Integer magnitudes in one unit stay Integer, a conversion makes a Real.
func addQuantities(op ast.OperatorKind, left, right *Quantity) (Value, error) {
	converted, err := right.convertTo(left.Unit)
	if err != nil {
		return Value{}, err
	}
	rhs := semantics.Value{Kind: semantics.ValReal, Real: converted}
	if right.Num.Kind == semantics.ValInt && left.Unit.Term.Scale == right.Unit.Term.Scale {
		rhs = right.Num
	}
	num, err := magnitudeArith(op, left.Num, rhs)
	if err != nil {
		return Value{}, err
	}
	return NewQuantityValue(&Quantity{Num: num, Unit: left.Unit}), nil
}

// scaleQuantities evaluates a product or quotient of quantities, whose unit is
// the product or quotient of theirs — `10 [m] / 2 [s]` is `5 [m/s]`.
func scaleQuantities(op ast.OperatorKind, left, right *Quantity) (Value, error) {
	// The magnitudes are combined in the units they were written in, and the
	// resulting unit is composed from the same two units, so no conversion is
	// involved and nothing is normalized away.
	num, err := magnitudeArith(op, left.Num, right.Num)
	if err != nil {
		return Value{}, err
	}
	var (
		product semantics.UnitProduct
		term    semantics.UnitTerm
	)
	if op == ast.OpMul {
		product = left.Unit.product().Times(right.Unit.product())
		term = left.Unit.Term.Times(right.Unit.Term)
	} else {
		product = left.Unit.product().DividedBy(right.Unit.product())
		term = left.Unit.Term.DividedBy(right.Unit.Term)
	}
	return composedQuantity(num, product, term)
}

// powQuantity raises a quantity to a constant exponent, its unit included. The
// magnitude goes through semantics.Pow, the one implementation of `**` the
// folder and the runtime share, so a quantity reports the domain and overflow
// cases identically to a bare number instead of carrying an Inf or a NaN.
func powQuantity(base *Quantity, exponent semantics.Value) (Value, error) {
	if !exponent.IsNumeric() {
		return Value{}, fmt.Errorf("%w: exponent of a quantity is not a number", ErrTypeMismatch)
	}
	num, err := semantics.Pow(base.Num, exponent)
	if err != nil {
		return Value{}, err
	}
	return composedQuantity(num, base.Unit.product().Pow(toReal(exponent)), base.Unit.Term.Pow(toReal(exponent)))
}

// sqrtQuantity is the square root of a quantity, `9 [m**2]` giving `3.0 [m]`;
// a unit with a base unit at an odd power has no root, so `sqrt(9 [m])` is rejected.
func sqrtQuantity(q *Quantity) (Value, error) {
	for _, f := range q.Unit.Term.Factors {
		if math.Mod(f.Exponent, 2) != 0 {
			return Value{}, fmt.Errorf("%w: %s (%s) raises %s to the odd power %g",
				ErrUnitRoot, q.Unit, q.Unit.Term, f.Unit.Name, f.Exponent)
		}
	}
	magnitude := toReal(q.Num)
	if magnitude < 0 {
		return Value{}, fmt.Errorf("%w: sqrt of a negative quantity %s", semantics.ErrArithmeticDomain, q)
	}
	num, err := realResult(math.Sqrt(magnitude))
	if err != nil {
		return Value{}, err
	}
	return composedQuantity(num, q.Unit.product().Pow(0.5), q.Unit.Term.Pow(0.5))
}

// dimensionlessValue is the number a ratio of like quantities computes,
// keeping its kind unless a scale factor is left to apply.
func dimensionlessValue(num semantics.Value, term semantics.UnitTerm) (Value, error) {
	if term.Scale != semantics.UnitScale(1) {
		var err error
		if num, err = realResult(semantics.ConvertMagnitude(toReal(num), term.Scale, semantics.UnitScale(1))); err != nil {
			return Value{}, err
		}
	}
	return Value{Kind: ValConst, Const: num}, nil
}

// magnitudeArith combines two magnitudes as the bare operator does:
// Integer operands keep an Integer result except under `/`.
func magnitudeArith(op ast.OperatorKind, left, right semantics.Value) (semantics.Value, error) {
	if left.Kind == semantics.ValInt && right.Kind == semantics.ValInt {
		if op == ast.OpDiv {
			q, ok := semantics.IntQuotient(left.Int, right.Int)
			if !ok {
				return semantics.Value{}, ErrDivisionByZero
			}
			return semantics.Value{Kind: semantics.ValReal, Real: q}, nil
		}
		res, ok := semantics.IntArith(op, left.Int, right.Int)
		if !ok {
			return semantics.Value{}, integerOverflow(op, left.Int, right.Int)
		}
		return semantics.Value{Kind: semantics.ValInt, Int: res}, nil
	}
	res, ok := semantics.RealArith(op, toReal(left), toReal(right))
	if !ok {
		return semantics.Value{}, ErrDivisionByZero
	}
	return realResult(res)
}

// compareQuantities orders two quantities, converting the right one into the
// left one's unit.
func compareQuantities(op ast.OperatorKind, left, right *Quantity) (Value, error) {
	converted, err := right.convertTo(left.Unit)
	if err != nil {
		return Value{}, err
	}
	lhs := toReal(left.Num)
	var result bool
	switch op {
	case ast.OpLt:
		result = lhs < converted
	case ast.OpLe:
		result = lhs <= converted
	case ast.OpGt:
		result = lhs > converted
	case ast.OpGe:
		result = lhs >= converted
	default:
		return Value{}, fmt.Errorf("unknown comparison operator: %v", op)
	}
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: result}}, nil
}

// equalQuantities compares two quantities for equality, in the left one's unit.
// Incommensurable units are an error rather than an inequality: they measure
// different things, so neither `==` nor `!=` is an answer about them.
func equalQuantities(op ast.OperatorKind, left, right *Quantity) (Value, error) {
	converted, err := right.convertTo(left.Unit)
	if err != nil {
		return Value{}, err
	}
	equal := toReal(left.Num) == converted
	if op == ast.OpNeq {
		equal = !equal
	}
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: equal}}, nil
}

// negateQuantity negates a quantity's magnitude, keeping its unit and the
// magnitude's kind.
func negateQuantity(q *Quantity) (Value, error) {
	num, err := constUnary(ast.OpNeg, q.Num)
	if err != nil {
		return Value{}, err
	}
	return NewQuantityValue(&Quantity{Num: num, Unit: q.Unit}), nil
}
