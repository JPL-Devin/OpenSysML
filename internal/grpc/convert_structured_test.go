package grpc

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// The wire Value has no Array, Vector or measurement-reference arm, so such a
// value crosses as an unsupported null naming it, never as a sequence that has lost its shape.
func TestStructuredValuesCrossAsUnsupported(t *testing.T) {
	real := func(f float64) semantics.Value {
		return semantics.Value{Kind: semantics.ValReal, Real: f}
	}
	one := runtime.Value{Kind: runtime.ValConst, Const: real(1)}
	two := runtime.Value{Kind: runtime.ValConst, Const: real(2)}
	cases := []struct {
		val  runtime.Value
		want string
	}{
		{runtime.NewArrayValue([]int64{1, 2}, []runtime.Value{one, two}), "unsupported: array Array(1, 2)[1.0, 2.0]"},
		{runtime.NewVectorValue([]semantics.Value{real(1), real(2)}), "unsupported: vector ⟨1.0, 2.0⟩"},
		{runtime.NewMeasurementRefValue(semantics.Unit{Text: "m", Product: semantics.OpaqueUnitProduct("m", semantics.UnitTerm{Scale: semantics.UnitScale(1)})}), "unsupported: measurement reference m"},
	}
	for _, tc := range cases {
		pv := ValueToProto(tc.val, nil)
		if pv.GetSequence() != nil {
			t.Errorf("%s crossed as a sequence: %v", runtime.FormatValue(tc.val), pv)
		}
		if got := pv.GetNull(); got != tc.want {
			t.Errorf("ValueToProto(%s) = %v, want null %q", runtime.FormatValue(tc.val), pv, tc.want)
		}
	}
}
