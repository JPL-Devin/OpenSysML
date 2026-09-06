package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// Reasons the calculations over a vector measurement reference or a coordinate
// frame are not evaluated with: the runtime holds scalar references only.
const (
	noVectorMeasurementRef = "a VectorMeasurementReference is a CoordinateFrame, whose mRefs and " +
		"basisDirections the runtime holds no value of; a scalar quantity is written `num [unit]` or `'['(num, unit)`"
	noCoordinateFrameValue = "a CoordinateFrame is a library declaration whose origin and basisDirections " +
		"the runtime holds no value of, so no frame is scaled by a unit"
)

// measurementRefArg reads a ScalarMeasurementReference (MeasurementUnit) parameter.
func measurementRefArg(name, param string, val Value) (*MeasurementRef, error) {
	if val.Kind != ValMeasurementRef || val.MeasurementRef() == nil {
		return nil, fmt.Errorf("%w: function %s parameter %q requires a measurement reference such as SI::m, got %s",
			ErrTypeMismatch, name, param, describeValue(val))
	}
	return val.MeasurementRef(), nil
}

// quantityOf is QuantityCalculations::'[': the number `num` in the reference `mRef`.
func quantityOf(name string, _ *Context, args []Value) (Value, error) {
	num, err := scalarArg(name, "num", args[0])
	if err != nil {
		return Value{}, err
	}
	ref, err := measurementRefArg(name, "mRef", args[1])
	if err != nil {
		return Value{}, err
	}
	return inUnit(num, ref.Unit)
}

// convertQuantity is QuantityCalculations::ConvertQuantity: x expressed in
// targetMRef, which must be commensurable with x's own reference.
func convertQuantity(name string, _ *Context, args []Value) (Value, error) {
	x, err := quantityArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	ref, err := measurementRefArg(name, "targetMRef", args[1])
	if err != nil {
		return Value{}, err
	}
	val, err := quantityResult(semantics.ConvertQuantity(*x, ref.Unit))
	return val, functionError(name, err)
}

// measurementRefArithmetic is MeasurementRefCalculations::'*', '/', '**' and '^'
// over MeasurementUnits: the composed unit, as a quantity's unit composes.
func measurementRefArithmetic(op ast.OperatorKind) libraryApply {
	return func(name string, _ *Context, args []Value) (Value, error) {
		if _, err := measurementRefArg(name, "x", args[0]); err != nil {
			return Value{}, err
		}
		if op == ast.OpPow {
			if _, err := scalarArg(name, "y", args[1]); err != nil {
				return Value{}, err
			}
		} else if _, err := measurementRefArg(name, "y", args[1]); err != nil {
			return Value{}, err
		}
		ref, ok := composeMeasurementRefs(op, args[0], args[1])
		if !ok {
			return Value{}, fmt.Errorf("%w: function %s is not defined over %s and %s",
				ErrTypeMismatch, name, describeValue(args[0]), describeValue(args[1]))
		}
		return ref, nil
	}
}

// measurementRefToString is MeasurementRefCalculations::ToString: the symbol
// the reference is written by, `m` or `km/h`.
func measurementRefToString(name string, _ *Context, args []Value) (Value, error) {
	ref, err := measurementRefArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return NewStringValue(ref.String()), nil
}
