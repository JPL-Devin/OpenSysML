package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// evalLibraryCall evaluates expr as an attribute value of a model that loads
// the standard libraries, which is how a function-call form reaches a builtin.
func evalLibraryCall(t *testing.T, expr string) (Value, error) {
	t.Helper()
	return evalNamedAttribute(t, `package test {
	attribute xs = (1, 2, 3);
	attribute result = `+expr+`;
}`, "result")
}

// The function-call form of a control function keeps the library's
// short-circuit semantics: a branch it does not select is never evaluated.
func TestControlFunctionCallForms(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"ControlFunctions::'if'(true, 1, 2)", "1"},
		{"ControlFunctions::'if'(false, 1, 2)", "2"},
		{"ControlFunctions::'if'(false, 1)", "null"},
		{"ControlFunctions::'if'(true, 1, 1 / 0)", "1"},
		{"ControlFunctions::'if'(false, 1 / 0, 2)", "2"},
		{"ControlFunctions::'if'(true, {1 + 1}, 2)", "2"},
		{"ControlFunctions::'and'(false, 1 / 0 == 0)", "false"},
		{"ControlFunctions::'and'(true, true)", "true"},
		{"ControlFunctions::'and'(true, false)", "false"},
		{"ControlFunctions::'or'(true, 1 / 0 == 0)", "true"},
		{"ControlFunctions::'or'(false, true)", "true"},
		{"ControlFunctions::'or'(false, false)", "false"},
		{"ControlFunctions::'implies'(false, 1 / 0 == 0)", "true"},
		{"ControlFunctions::'implies'(true, false)", "false"},
		{"ControlFunctions::'implies'(true, true)", "true"},
		{"ControlFunctions::'??'(null, 5)", "5"},
		{"ControlFunctions::'??'(3, 1 / 0)", "3"},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			got, err := evalLibraryCall(t, tc.expr)
			if err != nil {
				t.Fatalf("%s = error %v", tc.expr, err)
			}
			if FormatValue(got) != tc.want {
				t.Fatalf("%s = %s, want %s", tc.expr, FormatValue(got), tc.want)
			}
		})
	}
}

// A control function reports a non-Boolean test and an error in the branch
// it does select.
func TestControlFunctionCallFormErrors(t *testing.T) {
	cases := []struct {
		expr string
		want error
	}{
		{"ControlFunctions::'if'(1, 1, 2)", ErrTypeMismatch},
		{"ControlFunctions::'if'(false, 1, 1 / 0)", ErrDivisionByZero},
		{"ControlFunctions::'and'(true, 1 / 0 == 0)", ErrDivisionByZero},
		{"ControlFunctions::'and'(true, 1)", ErrTypeMismatch},
		{"ControlFunctions::'or'(1, true)", ErrTypeMismatch},
		{"ControlFunctions::'??'(null, 1 / 0)", ErrDivisionByZero},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			_, err := evalLibraryCall(t, tc.expr)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s error = %v, want %v", tc.expr, err, tc.want)
			}
		})
	}
}

// sum0 and product1 fold a collection and answer the identity given for an
// empty one, in the kind of the elements or of the identity.
func TestAggregationsWithIdentity(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"NumericalFunctions::sum0((1, 2, 3), 0)", "6"},
		{"NumericalFunctions::sum0((1.5, 2), 0)", "3.5"},
		{"NumericalFunctions::sum0((), 0)", "0"},
		{"NumericalFunctions::sum0((), 0.0)", "0.0"},
		{"NumericalFunctions::sum0(test::xs, 0.0)", "6"},
		{"NumericalFunctions::product1((2, 3, 4), 1)", "24"},
		{"NumericalFunctions::product1((0.5, 4), 1)", "2.0"},
		{"NumericalFunctions::product1((), 1)", "1"},
		{"NumericalFunctions::product1((), 1.0)", "1.0"},
		{"sum0((1, 2), 0)", "3"},
		{"product1((2, 5), 1)", "10"},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			got, err := evalLibraryCall(t, tc.expr)
			if err != nil {
				t.Fatalf("%s = error %v", tc.expr, err)
			}
			if FormatValue(got) != tc.want {
				t.Fatalf("%s = %s, want %s", tc.expr, FormatValue(got), tc.want)
			}
		})
	}
}

// The identity argument must be the identity the library's invariant asserts,
// the elements must be numbers, and an overflowing fold is reported.
func TestAggregationsWithIdentityErrors(t *testing.T) {
	cases := []struct {
		expr string
		want error
		text string
	}{
		{"NumericalFunctions::sum0((1, 2), 1)", ErrTypeMismatch, "isZero(zero)"},
		{"NumericalFunctions::product1((1, 2), 0)", ErrTypeMismatch, "isUnit(one)"},
		{`NumericalFunctions::sum0(("a", "b"), 0)`, ErrTypeMismatch, "numeric elements"},
		{"NumericalFunctions::sum0((9223372036854775807, 1), 0)", semantics.ErrArithmeticOverflow, "Integer range"},
		{"NumericalFunctions::product1((1e200, 1e200), 1)", semantics.ErrArithmeticOverflow, "finite"},
		{"NumericalFunctions::sum0((1, 2))", ErrCalcArity, "sum0"},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			_, err := evalLibraryCall(t, tc.expr)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s error = %v, want %v", tc.expr, err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.text) {
				t.Fatalf("%s error %q does not mention %q", tc.expr, err, tc.text)
			}
		})
	}
}

// The sequence and range operators called by name answer as the notation does.
func TestSequenceOperatorCallForms(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"ScalarFunctions::'..'(1, 3)", "[1, 2, 3]"},
		{"DataFunctions::'..'(3, 1)", "[]"},
		{"IntegerFunctions::'..'(-1, 1)", "[-1, 0, 1]"},
		{"BaseFunctions::'#'((10, 20, 30), 2)", "20"},
		{"BaseFunctions::','((1, 2), (3))", "[1, 2, 3]"},
		{"CollectionFunctions::'=='((1, 2), (1, 2))", "true"},
		{"CollectionFunctions::'=='((1, 2), (2, 1))", "false"},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			got, err := evalLibraryCall(t, tc.expr)
			if err != nil {
				t.Fatalf("%s = error %v", tc.expr, err)
			}
			if FormatValue(got) != tc.want {
				t.Fatalf("%s = %s, want %s", tc.expr, FormatValue(got), tc.want)
			}
		})
	}
}
