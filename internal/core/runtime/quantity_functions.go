package runtime

import (
	"fmt"
	"math"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// Reasons the domain library's calculations this runtime does not evaluate are
// reported with, named per representation the runtime lacks.
const (
	noMeasurementRefValue = "a measurement reference is a library declaration, not a value the evaluator " +
		"passes as an argument; write the quantity as `num [unit]`"
	noVectorQuantity = "a vector quantity has no representation: a vector is a sequence of numbers " +
		"and carries no measurement reference"
	noTensorQuantity           = "a tensor quantity has no representation: the runtime has no tensor value kind"
	noCoordinateTransformation = "a coordinate transformation has no representation: " +
		"a coordinate frame is a library declaration, not a value"
)

// init registers the Quantities and Units domain library's calculation packages:
// computed over the quantity or vector representation, or unevaluable by name.
func init() {
	registerQuantityCalculations()
	registerMeasurementRefCalculations()
	registerVectorCalculations()
	registerTensorCalculations()
}

// registerQuantityCalculations registers QuantityCalculations over the quantity
// representation; `sum` folds in the first element's unit, not the body's unvalued `zero`.
func registerQuantityCalculations() {
	registerUnevaluable("QuantityCalculations::[", []string{"num", "mRef"}, 2, noMeasurementRefValue)
	registerValueFunction("QuantityCalculations::isZero", []string{"x"}, 1, quantityPredicate(0))
	registerValueFunction("QuantityCalculations::isUnit", []string{"x"}, 1, quantityPredicate(1))
	registerValueFunction("QuantityCalculations::abs", []string{"x"}, 1, quantityMagnitudeUnary(numericAbs))
	registerValueFunction("QuantityCalculations::+", []string{"x", "y"}, 1, quantityAdditive(ast.OpAdd))
	registerValueFunction("QuantityCalculations::-", []string{"x", "y"}, 1, quantityAdditive(ast.OpSub))
	registerValueFunction("QuantityCalculations::*", []string{"x", "y"}, 2, quantityMultiplicative(ast.OpMul))
	registerValueFunction("QuantityCalculations::/", []string{"x", "y"}, 2, quantityMultiplicative(ast.OpDiv))
	registerValueFunction("QuantityCalculations::**", []string{"x", "y"}, 2, quantityPower)
	registerValueFunction("QuantityCalculations::^", []string{"x", "y"}, 2, quantityPower)
	registerValueFunction("QuantityCalculations::<", []string{"x", "y"}, 2, quantityComparison(ast.OpLt))
	registerValueFunction("QuantityCalculations::>", []string{"x", "y"}, 2, quantityComparison(ast.OpGt))
	registerValueFunction("QuantityCalculations::<=", []string{"x", "y"}, 2, quantityComparison(ast.OpLe))
	registerValueFunction("QuantityCalculations::>=", []string{"x", "y"}, 2, quantityComparison(ast.OpGe))
	registerValueFunction("QuantityCalculations::==", []string{"x", "y"}, 2, quantityEquality)
	registerValueFunction("QuantityCalculations::max", []string{"x", "y"}, 2, quantityExtremum(ast.OpGt))
	registerValueFunction("QuantityCalculations::min", []string{"x", "y"}, 2, quantityExtremum(ast.OpLt))
	registerValueFunction("QuantityCalculations::sqrt", []string{"x"}, 1, quantitySqrt)
	registerValueFunction("QuantityCalculations::floor", []string{"x"}, 1, quantityMagnitudeUnary(floorToInteger))
	registerValueFunction("QuantityCalculations::round", []string{"x"}, 1, quantityMagnitudeUnary(roundToInteger))
	registerValueFunction("QuantityCalculations::ToString", []string{"x"}, 1, quantityToString)
	registerValueFunction("QuantityCalculations::ToInteger", []string{"x"}, 1, quantityToInteger)
	registerValueFunction("QuantityCalculations::ToRational", []string{"x"}, 1, quantityToReal)
	registerValueFunction("QuantityCalculations::ToReal", []string{"x"}, 1, quantityToReal)
	registerValueFunction("QuantityCalculations::ToDimensionOneValue", []string{"x"}, 1, toDimensionOneValue)
	registerValueFunction("QuantityCalculations::sum", []string{"collection"}, 1, quantityAggregate(ast.OpAdd))
	registerValueFunction("QuantityCalculations::product", []string{"collection"}, 1, quantityAggregate(ast.OpMul))
	registerUnevaluable("QuantityCalculations::ConvertQuantity", []string{"x", "targetMRef"}, 2, noMeasurementRefValue)
}

// registerMeasurementRefCalculations registers MeasurementRefCalculations, all
// of which take a measurement reference as an argument.
func registerMeasurementRefCalculations() {
	registerUnevaluable("MeasurementRefCalculations::*", []string{"x", "y"}, 2, noMeasurementRefValue)
	registerUnevaluable("MeasurementRefCalculations::/", []string{"x", "y"}, 2, noMeasurementRefValue)
	registerUnevaluable("MeasurementRefCalculations::**", []string{"x", "y"}, 2, noMeasurementRefValue)
	registerUnevaluable("MeasurementRefCalculations::^", []string{"x", "y"}, 2, noMeasurementRefValue)
	registerUnevaluable("MeasurementRefCalculations::CoordinateFrame*", []string{"x", "y"}, 2, noMeasurementRefValue)
	registerUnevaluable("MeasurementRefCalculations::CoordinateFrame/", []string{"x", "y"}, 2, noMeasurementRefValue)
	registerUnevaluable("MeasurementRefCalculations::ToString", []string{"x"}, 1, noMeasurementRefValue)
}

// registerVectorCalculations registers VectorCalculations over a vector of numbers;
// those needing a measurement reference, a quantity-scaled vector or a tensor are named.
func registerVectorCalculations() {
	registerUnevaluable("VectorCalculations::[", []string{"elements", "mRef"}, 2, noMeasurementRefValue)
	registerValueFunction("VectorCalculations::isZeroVectorQuantity", []string{"v"}, 1, vectorIsZero)
	registerValueFunction("VectorCalculations::isUnitVectorQuantity", []string{"v"}, 1, vectorIsUnit)
	registerValueFunction("VectorCalculations::+", []string{"v", "w"}, 2, vectorAdd)
	registerValueFunction("VectorCalculations::-", []string{"v", "w"}, 2, vectorSubtract)
	registerValueFunction("VectorCalculations::scalarVectorMult", []string{"x", "v"}, 2, scalarVectorMult)
	registerValueFunction("VectorCalculations::vectorScalarMult", []string{"v", "x"}, 2, vectorScalarMult)
	registerUnevaluable("VectorCalculations::scalarQuantityVectorMult", []string{"x", "v"}, 2, noVectorQuantity)
	registerUnevaluable("VectorCalculations::vectorScalarQuantityMult", []string{"v", "x"}, 2, noVectorQuantity)
	registerValueFunction("VectorCalculations::vectorScalarDiv", []string{"v", "x"}, 2, vectorScalarDiv)
	registerUnevaluable("VectorCalculations::vectorScalarQuantityDiv", []string{"v", "x"}, 2, noVectorQuantity)
	registerValueFunction("VectorCalculations::inner", []string{"v", "w"}, 2, vectorInner)
	registerUnevaluable("VectorCalculations::outer", []string{"v", "w"}, 2, noTensorQuantity)
	registerValueFunction("VectorCalculations::norm", []string{"v"}, 1, vectorNorm)
	registerValueFunction("VectorCalculations::angle", []string{"v", "w"}, 2, vectorAngle)
	registerUnevaluable("VectorCalculations::transform",
		[]string{"transformation", "sourceVector"}, 2, noCoordinateTransformation)
}

// registerTensorCalculations registers TensorCalculations as unevaluable: the
// runtime has no tensor value.
func registerTensorCalculations() {
	registerUnevaluable("TensorCalculations::[", []string{"elements", "mRef"}, 2, noTensorQuantity)
	registerUnevaluable("TensorCalculations::isZeroTensorQuantity", []string{"x"}, 1, noTensorQuantity)
	registerUnevaluable("TensorCalculations::isUnitTensorQuantity", []string{"x"}, 1, noTensorQuantity)
	registerUnevaluable("TensorCalculations::+", []string{"x", "y"}, 2, noTensorQuantity)
	registerUnevaluable("TensorCalculations::-", []string{"x", "y"}, 2, noTensorQuantity)
	registerUnevaluable("TensorCalculations::scalarTensorMult", []string{"x", "t"}, 2, noTensorQuantity)
	registerUnevaluable("TensorCalculations::TensorScalarMult", []string{"t", "x"}, 2, noTensorQuantity)
	registerUnevaluable("TensorCalculations::scalarQuantityTensorMult", []string{"x", "t"}, 2, noTensorQuantity)
	registerUnevaluable("TensorCalculations::TensorScalarQuantityMult", []string{"t", "x"}, 2, noTensorQuantity)
	registerUnevaluable("TensorCalculations::tensorVectorMult", []string{"t", "v"}, 2, noTensorQuantity)
	registerUnevaluable("TensorCalculations::vectorTensorMult", []string{"v", "t"}, 2, noTensorQuantity)
	registerUnevaluable("TensorCalculations::tensorTensorMult", []string{"s", "t"}, 2, noTensorQuantity)
	registerUnevaluable("TensorCalculations::transform",
		[]string{"transformation", "sourceTensor"}, 2, noCoordinateTransformation)
}

// numericArgument reads an argument to a numeric parameter: a number, or a
// quantity of dimension one as the number it reduces to (`30 ['°']` is π/6).
func numericArgument(val Value) (semantics.Value, bool) {
	switch val.Kind {
	case ValConst:
		return val.Const, val.Const.IsNumeric()
	case ValQuantity:
		q := val.Quantity()
		if !q.Unit.Term.Dimensionless() {
			return semantics.Value{}, false
		}
		if q.Unit.Term.Scale == semantics.UnitScale(1) {
			return q.Num, true
		}
		return semantics.Value{Kind: semantics.ValReal, Real: q.baseMagnitude()}, true
	}
	return semantics.Value{}, false
}

// quantityArg reads a ScalarQuantityValue argument: a quantity, or a number as
// a quantity of dimension one.
func quantityArg(name, param string, val Value) (*Quantity, error) {
	q, ok := asQuantity(val)
	if !ok {
		return nil, fmt.Errorf("%w: function %s parameter %q requires a quantity, got %s",
			ErrTypeMismatch, name, param, describeValue(val))
	}
	return q, nil
}

// quantityPredicate is isZero or isUnit: whether the magnitude is the given
// number, as the library bodies test `x.num`.
func quantityPredicate(magnitude float64) libraryApply {
	return func(name string, _ *Context, args []Value) (Value, error) {
		q, err := quantityArg(name, "x", args[0])
		if err != nil {
			return Value{}, err
		}
		return boolValue(toReal(q.Num) == magnitude), nil
	}
}

// quantityMagnitudeUnary applies a numeric function to the magnitude, keeping
// the unit: abs, floor and round.
func quantityMagnitudeUnary(apply func([]semantics.Value) (semantics.Value, error)) libraryApply {
	return func(name string, _ *Context, args []Value) (Value, error) {
		q, err := quantityArg(name, "x", args[0])
		if err != nil {
			return Value{}, err
		}
		num, err := apply([]semantics.Value{q.Num})
		if err != nil {
			return Value{}, fmt.Errorf("function %s: %w", name, err)
		}
		return NewQuantityValue(&Quantity{Num: num, Unit: q.Unit}), nil
	}
}

// quantityAdditive is '+' or '-': the binary operator, or with y omitted the
// unary one.
func quantityAdditive(op ast.OperatorKind) libraryApply {
	return func(name string, _ *Context, args []Value) (Value, error) {
		x, err := quantityArg(name, "x", args[0])
		if err != nil {
			return Value{}, err
		}
		if argumentOmitted(args[1]) {
			if op == ast.OpSub {
				return negateQuantity(x)
			}
			return NewQuantityValue(x), nil
		}
		y, err := quantityArg(name, "y", args[1])
		if err != nil {
			return Value{}, err
		}
		val, err := addQuantities(op, x, y)
		return val, functionError(name, err)
	}
}

// quantityMultiplicative is '*' or '/', composing the operands' units.
func quantityMultiplicative(op ast.OperatorKind) libraryApply {
	return func(name string, _ *Context, args []Value) (Value, error) {
		x, y, err := quantityArgs(name, args)
		if err != nil {
			return Value{}, err
		}
		val, err := scaleQuantities(op, x, y)
		return val, functionError(name, err)
	}
}

// quantityPower is '**' and '^': the quantity raised to a Real exponent.
func quantityPower(name string, _ *Context, args []Value) (Value, error) {
	x, err := quantityArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	y, err := scalarArg(name, "y", args[1])
	if err != nil {
		return Value{}, err
	}
	val, err := powQuantity(x, y)
	return val, functionError(name, err)
}

// quantityComparison is one of the four orderings, in the left operand's unit.
func quantityComparison(op ast.OperatorKind) libraryApply {
	return func(name string, _ *Context, args []Value) (Value, error) {
		x, y, err := quantityArgs(name, args)
		if err != nil {
			return Value{}, err
		}
		val, err := compareQuantities(op, x, y)
		return val, functionError(name, err)
	}
}

// quantityEquality is '==', in the left operand's unit.
func quantityEquality(name string, _ *Context, args []Value) (Value, error) {
	x, y, err := quantityArgs(name, args)
	if err != nil {
		return Value{}, err
	}
	val, err := equalQuantities(ast.OpEq, x, y)
	return val, functionError(name, err)
}

// quantityExtremum is max (op '>') or min (op '<'): the winning operand as written,
// `max(1 [m], 200 [cm])` being `200 [cm]`, and the first where the two are equal.
func quantityExtremum(op ast.OperatorKind) libraryApply {
	return func(name string, _ *Context, args []Value) (Value, error) {
		x, y, err := quantityArgs(name, args)
		if err != nil {
			return Value{}, err
		}
		yWins, err := compareQuantities(op, y, x)
		if err != nil {
			return Value{}, fmt.Errorf("function %s: %w", name, err)
		}
		if yWins.Const.Bool {
			return args[1], nil
		}
		return args[0], nil
	}
}

// quantitySqrt is sqrt: the root of the magnitude in the root of the unit.
func quantitySqrt(name string, _ *Context, args []Value) (Value, error) {
	x, err := quantityArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	val, err := sqrtQuantity(x)
	return val, functionError(name, err)
}

// quantityToString renders the quantity as the REPL prints it: `1.5 [m/s]`.
func quantityToString(name string, _ *Context, args []Value) (Value, error) {
	x, err := quantityArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return NewStringValue(x.String()), nil
}

// quantityToInteger is ToInteger: the magnitude, which must be a whole number.
func quantityToInteger(name string, _ *Context, args []Value) (Value, error) {
	x, err := quantityArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	if x.Num.Kind == semantics.ValInt {
		return Value{Kind: ValConst, Const: x.Num}, nil
	}
	if x.Num.Real != math.Trunc(x.Num.Real) {
		return Value{}, fmt.Errorf("%w: function %s requires a whole magnitude, %s has none",
			ErrTypeMismatch, name, x)
	}
	num, err := integerResult(x.Num.Real)
	if err != nil {
		return Value{}, fmt.Errorf("function %s: %w", name, err)
	}
	return Value{Kind: ValConst, Const: num}, nil
}

// quantityToReal is ToReal and ToRational: the magnitude as a Real.
func quantityToReal(name string, _ *Context, args []Value) (Value, error) {
	x, err := quantityArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: toReal(x.Num)}}, nil
}

// toDimensionOneValue is ToDimensionOneValue: a Real as a quantity of dimension
// one, which a bare number already is to every quantity operation.
func toDimensionOneValue(name string, _ *Context, args []Value) (Value, error) {
	x, err := scalarArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: ValConst, Const: x}, nil
}

// quantityAggregate is sum or product over quantities, folded in the first element's
// unit; an empty collection has no unit, so it is the dimensionless 0 or 1.
func quantityAggregate(op ast.OperatorKind) libraryApply {
	return func(name string, _ *Context, args []Value) (Value, error) {
		return aggregate(name, args, op)
	}
}

// vectorIsUnit is VectorCalculations::isUnitVectorQuantity: whether the norm is one.
func vectorIsUnit(name string, _ *Context, args []Value) (Value, error) {
	elements, err := realElements(name, "v", args[0])
	if err != nil {
		return Value{}, err
	}
	return boolValue(euclideanNorm(elements) == 1), nil
}

// quantityArgs reads the two ScalarQuantityValue parameters x and y.
func quantityArgs(name string, args []Value) (*Quantity, *Quantity, error) {
	x, err := quantityArg(name, "x", args[0])
	if err != nil {
		return nil, nil, err
	}
	y, err := quantityArg(name, "y", args[1])
	if err != nil {
		return nil, nil, err
	}
	return x, y, nil
}

// functionError names the function in the error of a quantity operation.
func functionError(name string, err error) error {
	if err != nil {
		return fmt.Errorf("function %s: %w", name, err)
	}
	return nil
}
