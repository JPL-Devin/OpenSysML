package grpc

import (
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// The wire Value has no Complex arm: a Complex crosses as an unsupported null
// naming the value, never as a pair of Reals a client would read as two values.
func TestComplexToProtoIsNamedUnsupported(t *testing.T) {
	idx := symbols.NewIndex()
	pv := ValueToProto(runtime.NewComplex(complex(0, 1)), idx)
	null, ok := pv.GetKind().(*pb.Value_Null)
	if !ok {
		t.Fatalf("got kind %T, want null", pv.GetKind())
	}
	if want := "unsupported: complex number 0.0 + 1.0i"; null.Null != want {
		t.Errorf("null marker: got %q, want %q", null.Null, want)
	}

	seq := runtime.NewSequence()
	seq.Append(runtime.NewComplex(complex(1, 2)))
	seq.Append(runtime.NewComplex(complex(3, 4)))
	pv = ValueToProto(runtime.NewSequenceValue(seq), idx)
	elements := pv.GetSequence().GetElements()
	if len(elements) != 2 {
		t.Fatalf("a sequence of two Complex values crossed as %d elements", len(elements))
	}
	for _, elem := range elements {
		if _, ok := elem.GetKind().(*pb.Value_Null); !ok {
			t.Errorf("element kind %T, want null", elem.GetKind())
		}
	}
}
