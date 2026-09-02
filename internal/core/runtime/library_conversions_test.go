package runtime

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

func constBool(b bool) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: b}}
}

// The conversions compute the value their declaration promises, keeping the
// kind the declared result type names.
func TestConversionFunctionValues(t *testing.T) {
	cases := []struct {
		fn   string
		args []Value
		want Value
	}{
		{"BaseFunctions::ToString", []Value{constInt(42)}, NewStringValue("42")},
		{"BaseFunctions::ToString", []Value{constReal(2.5)}, NewStringValue("2.5")},
		{"BaseFunctions::ToString", []Value{constReal(1)}, NewStringValue("1.0")},
		{"BaseFunctions::ToString", []Value{constReal(math.Nextafter(0.3, 1))}, NewStringValue("0.30000000000000004")},
		{"BaseFunctions::ToString", []Value{constReal(1e21)}, NewStringValue("1e+21")},
		{"BaseFunctions::ToString", []Value{constBool(true)}, NewStringValue("true")},
		{"BaseFunctions::ToString", []Value{NewStringValue("as is")}, NewStringValue("as is")},
		{"BooleanFunctions::ToString", []Value{constBool(false)}, NewStringValue("false")},
		{"IntegerFunctions::ToString", []Value{constInt(-7)}, NewStringValue("-7")},
		{"NaturalFunctions::ToString", []Value{constInt(7)}, NewStringValue("7")},
		{"RationalFunctions::ToString", []Value{constReal(0.5)}, NewStringValue("0.5")},
		{"RealFunctions::ToString", []Value{constReal(-1.5)}, NewStringValue("-1.5")},
		{"RealFunctions::ToString", []Value{constInt(3)}, NewStringValue("3")},

		{"BooleanFunctions::ToBoolean", []Value{NewStringValue("true")}, constBool(true)},
		{"BooleanFunctions::ToBoolean", []Value{NewStringValue(" false ")}, constBool(false)},
		{"IntegerFunctions::ToInteger", []Value{NewStringValue("7")}, constInt(7)},
		{"IntegerFunctions::ToInteger", []Value{NewStringValue("-9223372036854775808")}, constInt(math.MinInt64)},
		{"NaturalFunctions::ToNatural", []Value{NewStringValue("0")}, constInt(0)},
		{"RealFunctions::ToReal", []Value{NewStringValue("1.5")}, constReal(1.5)},
		{"RealFunctions::ToReal", []Value{NewStringValue("-2")}, constReal(-2)},
		{"RealFunctions::ToReal", []Value{NewStringValue("1e21")}, constReal(1e21)},
		{"RealFunctions::ToReal", []Value{NewStringValue(".5")}, constReal(0.5)},
		{"RationalFunctions::ToRational", []Value{NewStringValue("0.25")}, constReal(0.25)},

		{"IntegerFunctions::ToNatural", []Value{constInt(5)}, constInt(5)},
		{"RealFunctions::ToInteger", []Value{constReal(2.7)}, constInt(2)},
		{"RealFunctions::ToInteger", []Value{constReal(-2.7)}, constInt(-2)},
		{"RealFunctions::ToInteger", []Value{constInt(4)}, constInt(4)},
		{"RationalFunctions::ToInteger", []Value{constReal(0.5)}, constInt(0)},
		{"RealFunctions::ToRational", []Value{constReal(0.75)}, constReal(0.75)},

		{"RealFunctions::re", []Value{constReal(2.5)}, constReal(2.5)},
		{"RealFunctions::re", []Value{constInt(2)}, constReal(2)},
		{"RealFunctions::im", []Value{constReal(2.5)}, constReal(0)},
		{"RealFunctions::arg", []Value{constReal(-2.5)}, constReal(0)},

		{"RationalFunctions::floor", []Value{constReal(-0.5)}, constInt(-1)},
		{"RationalFunctions::round", []Value{constReal(2.5)}, constInt(3)},
		{"RationalFunctions::gcd", []Value{constInt(12), constInt(18)}, constInt(6)},
		{"RationalFunctions::gcd", []Value{constInt(-12), constInt(18)}, constInt(6)},
		{"RationalFunctions::gcd", []Value{constInt(0), constInt(0)}, constInt(0)},
		{"RationalFunctions::gcd", []Value{constInt(0), constInt(-5)}, constInt(5)},
		{"RationalFunctions::gcd", []Value{constReal(12), constInt(8)}, constInt(4)},
	}
	for _, tc := range cases {
		t.Run(tc.fn+"/"+FormatValue(tc.args[0]), func(t *testing.T) {
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

// ToString and ToReal round-trip: a Real reads back as the same Real.
func TestRealToStringRoundTrips(t *testing.T) {
	for _, x := range []float64{0, 1, -1.5, 0.1 + 0.2, 1e21, 1e-7, math.MaxFloat64, math.SmallestNonzeroFloat64, 123456789.125} {
		text, err := applyLibrary(t, "RealFunctions::ToString", constReal(x))
		if err != nil {
			t.Fatalf("ToString(%v) = error %v", x, err)
		}
		back, err := applyLibrary(t, "RealFunctions::ToReal", text)
		if err != nil {
			t.Fatalf("ToReal(%q) = error %v", text.Str(), err)
		}
		if back.Const.Kind != semantics.ValReal || back.Const.Real != x {
			t.Fatalf("ToReal(ToString(%v)) = %s, want %v", x, FormatValue(back), x)
		}
	}
}

// Every conversion failure is a typed error naming the function.
func TestConversionFunctionErrors(t *testing.T) {
	cases := []struct {
		fn   string
		args []Value
		want error
	}{
		{"BooleanFunctions::ToBoolean", []Value{NewStringValue("yes")}, ErrInvalidNotation},
		{"BooleanFunctions::ToBoolean", []Value{NewStringValue("True")}, ErrInvalidNotation},
		{"BooleanFunctions::ToBoolean", []Value{constInt(1)}, ErrTypeMismatch},
		{"IntegerFunctions::ToInteger", []Value{NewStringValue("7.0")}, ErrInvalidNotation},
		{"IntegerFunctions::ToInteger", []Value{NewStringValue("")}, ErrInvalidNotation},
		{"IntegerFunctions::ToInteger", []Value{NewStringValue("0x10")}, ErrInvalidNotation},
		{"IntegerFunctions::ToInteger", []Value{NewStringValue("9223372036854775808")}, semantics.ErrArithmeticOverflow},
		{"IntegerFunctions::ToInteger", []Value{constInt(7)}, ErrTypeMismatch},
		{"NaturalFunctions::ToNatural", []Value{NewStringValue("-3")}, semantics.ErrArithmeticDomain},
		{"NaturalFunctions::ToNatural", []Value{NewStringValue("three")}, ErrInvalidNotation},
		{"IntegerFunctions::ToNatural", []Value{constInt(-1)}, semantics.ErrArithmeticDomain},
		{"IntegerFunctions::ToNatural", []Value{constReal(1)}, ErrTypeMismatch},
		{"RealFunctions::ToReal", []Value{NewStringValue("abc")}, ErrInvalidNotation},
		{"RealFunctions::ToReal", []Value{NewStringValue("1.5.2")}, ErrInvalidNotation},
		{"RealFunctions::ToReal", []Value{NewStringValue("1e")}, ErrInvalidNotation},
		{"RealFunctions::ToReal", []Value{NewStringValue("Inf")}, ErrInvalidNotation},
		{"RealFunctions::ToReal", []Value{NewStringValue("NaN")}, ErrInvalidNotation},
		{"RealFunctions::ToReal", []Value{NewStringValue("0x1p-2")}, ErrInvalidNotation},
		{"RealFunctions::ToReal", []Value{NewStringValue("1e400")}, semantics.ErrArithmeticOverflow},
		{"RealFunctions::ToReal", []Value{constReal(1)}, ErrTypeMismatch},
		{"RationalFunctions::ToRational", []Value{NewStringValue("1/3")}, ErrInvalidNotation},
		{"RealFunctions::ToInteger", []Value{constReal(1e19)}, semantics.ErrArithmeticOverflow},
		{"RealFunctions::ToInteger", []Value{NewStringValue("1")}, ErrTypeMismatch},
		{"RealFunctions::re", []Value{NewStringValue("1")}, ErrTypeMismatch},
		{"NaturalFunctions::ToString", []Value{constInt(-1)}, ErrTypeMismatch},
		{"IntegerFunctions::ToString", []Value{constReal(1)}, ErrTypeMismatch},
		{"BaseFunctions::ToString", []Value{NewSequenceValue(NewSequence())}, ErrTypeMismatch},
		{"RationalFunctions::gcd", []Value{constReal(0.5), constInt(2)}, semantics.ErrArithmeticDomain},
		{"RationalFunctions::gcd", []Value{constInt(math.MinInt64), constInt(2)}, semantics.ErrArithmeticOverflow},
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
