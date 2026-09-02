package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// The function form of each operator answers what the operator notation does.
func TestOperatorFunctionValues(t *testing.T) {
	cases := []struct {
		fn   string
		args []Value
		want Value
	}{
		{"IntegerFunctions::+", []Value{constInt(1), constInt(2)}, constInt(3)},
		{"IntegerFunctions::+", []Value{constInt(5)}, constInt(5)},
		{"IntegerFunctions::-", []Value{constInt(5)}, constInt(-5)},
		{"IntegerFunctions::-", []Value{constInt(5), constInt(7)}, constInt(-2)},
		{"IntegerFunctions::*", []Value{constInt(3), constInt(4)}, constInt(12)},
		{"IntegerFunctions::/", []Value{constInt(1), constInt(4)}, constReal(0.25)},
		{"IntegerFunctions::%", []Value{constInt(7), constInt(3)}, constInt(1)},
		{"IntegerFunctions::**", []Value{constInt(2), constInt(10)}, constInt(1024)},
		{"IntegerFunctions::^", []Value{constInt(2), constInt(0)}, constInt(1)},
		{"IntegerFunctions::<", []Value{constInt(1), constInt(2)}, constBool(true)},
		{"IntegerFunctions::>=", []Value{constInt(1), constInt(2)}, constBool(false)},
		{"IntegerFunctions::==", []Value{constInt(2), constInt(2)}, constBool(true)},
		{"IntegerFunctions::==", []Value{constInt(2), constReal(2)}, constBool(true)},
		{"IntegerFunctions::==", nil, constBool(true)},
		{"IntegerFunctions::==", []Value{constInt(2)}, constBool(false)},
		{"NaturalFunctions::+", []Value{constInt(1), constInt(2)}, constInt(3)},
		{"NaturalFunctions::/", []Value{constInt(6), constInt(3)}, constReal(2)},
		{"RealFunctions::+", []Value{constReal(1.5), constInt(2)}, constReal(3.5)},
		{"RealFunctions::-", []Value{constReal(1.5)}, constReal(-1.5)},
		{"RealFunctions::**", []Value{constReal(2), constReal(-1)}, constReal(0.5)},
		{"RealFunctions::<=", []Value{constReal(2), constInt(2)}, constBool(true)},
		{"RationalFunctions::*", []Value{constReal(0.5), constReal(0.5)}, constReal(0.25)},
		{"RationalFunctions::>", []Value{constReal(0.5), constReal(0.25)}, constBool(true)},
		{"NumericalFunctions::+", []Value{constInt(1), constInt(2)}, constInt(3)},
		{"NumericalFunctions::%", []Value{constReal(7.5), constInt(2)}, constReal(1.5)},
		{"NumericalFunctions::*", []Value{cx(0, 1), cx(0, 1)}, cx(-1, 0)},
		{"ScalarFunctions::+", []Value{NewStringValue("a"), NewStringValue("b")}, NewStringValue("ab")},
		{"ScalarFunctions::<", []Value{NewStringValue("a"), NewStringValue("b")}, constBool(true)},
		{"DataFunctions::+", []Value{constInt(1), constReal(0.5)}, constReal(1.5)},
		{"DataFunctions::==", []Value{NewStringValue("a"), NewStringValue("a")}, constBool(true)},
		{"DataFunctions::===", []Value{constInt(2), constReal(2)}, constBool(false)},
		{"DataFunctions::===", []Value{constInt(2), constInt(2)}, constBool(true)},
		{"BaseFunctions::==", []Value{constInt(2), constReal(2)}, constBool(true)},
		{"BaseFunctions::!=", []Value{constInt(1), constInt(2)}, constBool(true)},
		{"BaseFunctions::!=", nil, constBool(false)},
		{"BaseFunctions::===", []Value{NewStringValue("x"), NewStringValue("x")}, constBool(true)},
		{"BaseFunctions::!==", []Value{constInt(2), constReal(2)}, constBool(true)},
		{"BooleanFunctions::not", []Value{constBool(true)}, constBool(false)},
		{"BooleanFunctions::xor", []Value{constBool(true), constBool(false)}, constBool(true)},
		{"BooleanFunctions::xor", []Value{constBool(true), constBool(true)}, constBool(false)},
		{"BooleanFunctions::|", []Value{constBool(false), constBool(true)}, constBool(true)},
		{"BooleanFunctions::&", []Value{constBool(false), constBool(true)}, constBool(false)},
		{"BooleanFunctions::==", []Value{constBool(true), constBool(true)}, constBool(true)},
		{"DataFunctions::not", []Value{constBool(false)}, constBool(true)},
		{"ScalarFunctions::xor", []Value{constBool(false), constBool(true)}, constBool(true)},
		{"DataFunctions::max", []Value{constInt(2), constInt(7)}, constInt(7)},
		{"DataFunctions::max", []Value{constInt(2), constReal(7.5)}, constReal(7.5)},
		{"DataFunctions::min", []Value{constReal(-1), constInt(3)}, constReal(-1)},
		{"DataFunctions::max", []Value{NewStringValue("apple"), NewStringValue("pear")}, NewStringValue("pear")},
		{"DataFunctions::min", []Value{NewStringValue("apple"), NewStringValue("pear")}, NewStringValue("apple")},
		{"ScalarFunctions::max", []Value{NewStringValue("b"), NewStringValue("b")}, NewStringValue("b")},
		{"ScalarFunctions::min", []Value{constInt(4), constInt(4)}, constInt(4)},
	}
	for _, tc := range cases {
		name := tc.fn
		if len(tc.args) > 0 {
			name += "/" + FormatValue(tc.args[0])
		}
		t.Run(name, func(t *testing.T) {
			got, err := applyLibrary(t, tc.fn, tc.args...)
			if err != nil {
				t.Fatalf("%s = error %v", tc.fn, err)
			}
			if !valueIdentical(got, tc.want) {
				t.Fatalf("%s = %s (%v), want %s (%v)", tc.fn, FormatValue(got), got.Kind, FormatValue(tc.want), tc.want.Kind)
			}
		})
	}
}

// An operator function refuses an argument its package's parameter type does
// not admit, and reports the operator's own errors, all typed and named.
func TestOperatorFunctionErrors(t *testing.T) {
	cases := []struct {
		fn   string
		args []Value
		want error
	}{
		{"IntegerFunctions::+", []Value{constInt(1), constReal(2)}, ErrTypeMismatch},
		{"IntegerFunctions::+", []Value{NewStringValue("a"), NewStringValue("b")}, ErrTypeMismatch},
		{"IntegerFunctions::**", []Value{constInt(2), constInt(-1)}, ErrTypeMismatch},
		{"IntegerFunctions::*", []Value{constInt(1 << 62), constInt(4)}, semantics.ErrArithmeticOverflow},
		{"IntegerFunctions::/", []Value{constInt(1), constInt(0)}, ErrDivisionByZero},
		{"IntegerFunctions::%", []Value{constInt(1), constInt(0)}, ErrDivisionByZero},
		{"NaturalFunctions::+", []Value{constInt(-1), constInt(2)}, ErrTypeMismatch},
		{"NaturalFunctions::<", []Value{constInt(1), constReal(2)}, ErrTypeMismatch},
		{"RealFunctions::+", []Value{cx(1, 1), constReal(2)}, ErrTypeMismatch},
		{"RealFunctions::+", []Value{NewStringValue("1"), constReal(2)}, ErrTypeMismatch},
		{"RealFunctions::/", []Value{constReal(1), constReal(0)}, ErrDivisionByZero},
		{"RealFunctions::*", []Value{constReal(1e200), constReal(1e200)}, semantics.ErrArithmeticOverflow},
		{"RationalFunctions::<", []Value{constReal(1), NewStringValue("2")}, ErrTypeMismatch},
		{"NumericalFunctions::+", []Value{NewStringValue("a"), constInt(1)}, ErrTypeMismatch},
		{"ScalarFunctions::+", []Value{NewStringValue("a"), constInt(1)}, ErrTypeMismatch},
		{"DataFunctions::<", []Value{NewStringValue("a"), constInt(1)}, ErrTypeMismatch},
		{"DataFunctions::-", []Value{NewStringValue("a")}, ErrTypeMismatch},
		{"BooleanFunctions::not", []Value{constInt(1)}, ErrTypeMismatch},
		{"BooleanFunctions::xor", []Value{constBool(true), constInt(1)}, ErrTypeMismatch},
		{"DataFunctions::not", []Value{NewStringValue("true")}, ErrTypeMismatch},
		{"DataFunctions::max", []Value{constBool(true), constBool(false)}, ErrTypeMismatch},
		{"DataFunctions::max", []Value{constInt(1), NewStringValue("1")}, ErrTypeMismatch},
		{"ScalarFunctions::min", []Value{NewSequenceValue(NewSequence()), constInt(1)}, ErrTypeMismatch},
		{"IntegerFunctions::+", []Value{constInt(1), constInt(2), constInt(3)}, ErrCalcArity},
		{"IntegerFunctions::*", []Value{constInt(1)}, ErrCalcArity},
	}
	for _, tc := range cases {
		t.Run(tc.fn+"/"+FormatValue(tc.args[0]), func(t *testing.T) {
			_, err := applyLibrary(t, tc.fn, tc.args...)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s error = %v, want %v", tc.fn, err, tc.want)
			}
			if !strings.Contains(err.Error(), writtenName(tc.fn)) {
				t.Fatalf("%s error %q does not name the function", tc.fn, err)
			}
		})
	}
}
