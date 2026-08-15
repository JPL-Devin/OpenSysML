package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

// Quantity is a scalar quantity value: a magnitude and the measurement
// reference it is expressed in (Quantities::ScalarQuantityValue is exactly a
// number `num` and a reference `mRef`). The unit travels with the value, so
// `1.5 [m/s]` is never mistaken for `1.5 [km/h]`.
type Quantity struct {
	Num  semantics.Value
	Unit Unit
}

// Unit is a measurement reference as a quantity carries it: the expression it
// was written as, for diagnostics and printing, and its reduction to base
// units, which is what decides whether two quantities can be combined.
type Unit struct {
	Text string
	Term semantics.UnitTerm
}

// String renders the unit as written, falling back to its reduction for a unit
// composed by an operation rather than written down.
func (u Unit) String() string {
	if u.Text != "" {
		return u.Text
	}
	return u.Term.String()
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
		return fmt.Sprintf("%g", v.Real)
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

	unit := Unit{Text: semantics.UnitExprText(n.Index), Term: term}
	return Value{Kind: ValQuantity, Quantity: &Quantity{Num: magnitude.Const, Unit: unit}}, nil
}

// asQuantity views a value as a quantity: a quantity as itself, and a bare
// number as a magnitude of dimension one, since a number is commensurable with
// a count or a ratio of like quantities but with nothing else.
func asQuantity(val Value) (*Quantity, bool) {
	switch val.Kind {
	case ValQuantity:
		return val.Quantity, true
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
// left operand: the unit the model wrote the quantity being added to in.
func addQuantities(op ast.OperatorKind, left, right *Quantity) (Value, error) {
	converted, err := right.convertTo(left.Unit)
	if err != nil {
		return Value{}, err
	}
	magnitude := toReal(left.Num)
	if op == ast.OpAdd {
		magnitude += converted
	} else {
		magnitude -= converted
	}
	return quantityValue(magnitude, left.Unit), nil
}

// scaleQuantities evaluates a product or quotient of quantities, whose unit is
// the product or quotient of theirs — `10 [m] / 2 [s]` is `5 [m/s]`.
func scaleQuantities(op ast.OperatorKind, left, right *Quantity) (Value, error) {
	if op == ast.OpDiv && toReal(right.Num) == 0 {
		return Value{}, ErrDivisionByZero
	}
	// The magnitudes are combined in the units they were written in, and the
	// resulting unit is composed from the same two units, so no conversion is
	// involved and nothing is normalized away.
	var (
		magnitude float64
		unit      Unit
	)
	if op == ast.OpMul {
		magnitude = toReal(left.Num) * toReal(right.Num)
		unit = Unit{Text: composedUnitText(left.Unit, right.Unit, op), Term: left.Unit.Term.Times(right.Unit.Term)}
	} else {
		magnitude = toReal(left.Num) / toReal(right.Num)
		unit = Unit{Text: composedUnitText(left.Unit, right.Unit, op), Term: left.Unit.Term.DividedBy(right.Unit.Term)}
	}
	if unit.Term.Dimensionless() {
		// A ratio of like quantities is a number, not a quantity of no unit.
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal,
			Real: semantics.ConvertMagnitude(magnitude, unit.Term.Scale, semantics.UnitScale(1))}}, nil
	}
	return quantityValue(magnitude, unit), nil
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
	term := base.Unit.Term.Pow(toReal(exponent))
	if term.Dimensionless() {
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal,
			Real: semantics.ConvertMagnitude(toReal(num), term.Scale, semantics.UnitScale(1))}}, nil
	}
	unit := Unit{Text: fmt.Sprintf("(%s)%s%s", base.Unit, ast.OpPow, constText(exponent)), Term: term}
	return Value{Kind: ValQuantity, Quantity: &Quantity{Num: num, Unit: unit}}, nil
}

// composedUnitText renders the unit an operation on two quantities produces. A
// bare number scaling a quantity contributes no unit, so it names none.
func composedUnitText(left, right Unit, op ast.OperatorKind) string {
	if right.Text == "" && right.Term.Dimensionless() {
		return left.String()
	}
	if left.Text == "" && left.Term.Dimensionless() && op == ast.OpMul {
		return right.String()
	}
	return groupUnitText(left) + op.String() + groupUnitText(right)
}

// groupUnitText renders a unit as an operand of a composition, parenthesizing
// one that is itself composed: `(m/s)*(kg/s)` says what it means, while
// `m/s*kg/s` reads as a different unit than the one composed.
func groupUnitText(u Unit) string {
	text := u.String()
	if strings.ContainsAny(text, "*/^·") {
		return "(" + text + ")"
	}
	return text
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

// negateQuantity negates a quantity's magnitude, keeping its unit.
func negateQuantity(q *Quantity) Value {
	return quantityValue(-toReal(q.Num), q.Unit)
}

// quantityValue builds a quantity value from a computed magnitude, which is
// real: a conversion factor makes it one even where both operands were whole.
func quantityValue(magnitude float64, unit Unit) Value {
	num := semantics.Value{Kind: semantics.ValReal, Real: magnitude}
	return Value{Kind: ValQuantity, Quantity: &Quantity{Num: num, Unit: unit}}
}
