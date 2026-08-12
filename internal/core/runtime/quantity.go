package runtime

import (
	"fmt"
	"math"

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
	return fmt.Sprintf("%s [%s]", constText(q.Num), q.Unit)
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
// `seq#(i)` shares the node but not the meaning, and is not evaluated here.
func (ec *EvalContext) evalIndexExpr(n *ast.IndexExpr) (Value, error) {
	if !n.Bracket {
		// The sequence index is a separate operation and stays unsupported.
		return Value{}, fmt.Errorf("unsupported node type: %T", n)
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

	unit := Unit{Text: unitText(n.Index), Term: term}
	return Value{Kind: ValQuantity, Quantity: &Quantity{Num: magnitude.Const, Unit: unit}}, nil
}

// unitText renders a unit expression as written, so a diagnostic and a printed
// value name the unit the model used rather than its reduction.
func unitText(node ast.Node) string {
	switch n := node.(type) {
	case *ast.FeatureReference:
		return qualifiedNameText(n.Name)
	case *ast.OperatorExpr:
		switch {
		case len(n.Operands) == 2:
			return unitText(n.Operands[0]) + n.Operator.String() + unitText(n.Operands[1])
		case len(n.Operands) == 1:
			return n.Operator.String() + unitText(n.Operands[0])
		}
	case *ast.LiteralInteger:
		return n.Value
	case *ast.LiteralReal:
		return n.Value
	}
	return ""
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
		return Value{}, fmt.Errorf("division by zero")
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

// powQuantity raises a quantity to a constant exponent, its unit included.
func powQuantity(base *Quantity, exponent semantics.Value) (Value, error) {
	if !exponent.IsNumeric() {
		return Value{}, fmt.Errorf("%w: exponent of a quantity is not a number", ErrTypeMismatch)
	}
	exp := toReal(exponent)
	term := base.Unit.Term.Pow(exp)
	magnitude := math.Pow(toReal(base.Num), exp)
	if term.Dimensionless() {
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal,
			Real: semantics.ConvertMagnitude(magnitude, term.Scale, semantics.UnitScale(1))}}, nil
	}
	return quantityValue(magnitude, Unit{Text: fmt.Sprintf("(%s)%s%s", base.Unit, ast.OpPow, constText(exponent)), Term: term}), nil
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
	return left.String() + op.String() + right.String()
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
