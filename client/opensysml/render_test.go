package opensysml_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

func TestAStatusErrorNamesItsCodeCanonically(t *testing.T) {
	err := error(&opensysml.StatusError{Code: opensysml.CodeNotFound, Message: "model not found: abc"})
	const want = "opensysml: NOT_FOUND: model not found: abc"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if got := fmt.Sprintf("%v", err); got != want {
		t.Errorf("%%v = %q, want %q", got, want)
	}
	if !errors.Is(err, opensysml.CodeNotFound) {
		t.Error("errors.Is did not match the code")
	}
}

func TestValuesRenderAsTheirNotation(t *testing.T) {
	for _, testcase := range []struct {
		value opensysml.Value
		want  string
	}{
		{opensysml.Int(4), "4"},
		{opensysml.Real(1800), "1800"},
		{opensysml.Quantity{Magnitude: opensysml.Real(5.4), Unit: "km/h"}, "5.4 km/h"},
		{opensysml.Quantity{Magnitude: opensysml.Int(4200), Unit: "SI::kg",
			Term: &opensysml.UnitTerm{ScaleNum: 1, ScaleDen: 1}}, "4200 SI::kg"},
		{opensysml.Quantity{Magnitude: opensysml.Int(3)}, "3"},
		{opensysml.EnumLiteral{LiteralID: "D::Color::red", EnumerationID: "D::Color", Name: "Color::red"}, "Color::red"},
		{opensysml.Null(""), "null"},
		{opensysml.Null("unsupported: variant selection"), "null: unsupported: variant selection"},
		{opensysml.Unset{}, "unset"},
	} {
		if got := fmt.Sprintf("%v", testcase.value); got != testcase.want {
			t.Errorf("%#v renders as %q, want %q", testcase.value, got, testcase.want)
		}
	}
}
