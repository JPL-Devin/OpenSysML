package grpc

import (
	"context"
	"errors"
	"testing"

	pb "github.com/Open-MBEE/Systemica/api/proto"
)

const unsetSlotModel = `
package Demo {
  private import ScalarValues::*;
  part def Engine {
    attribute power : Real = 120.0;
  }
  part def Vehicle {
    attribute d : Real;
    attribute ds : Real[2];
    attribute k : Real = 2.0;
    part engine : Engine;
  }
}
`

// A slot holding no value is sent as such, so a client reads "no value" rather
// than the id of an object with nothing in it. A valued slot and an object-valued
// one are unaffected.
func TestInstantiate_ValuelessValueTypedSlotIsSentUnset(t *testing.T) {
	resp := instantiate(t, unsetSlotModel, "unset-slot", "Demo::Vehicle")
	slots := resp.GetInstance().GetSlots()

	d := slots["d"]
	if !d.GetMaterialized() {
		t.Errorf("slot d: materialized = false, want true: the object is created either way")
	}
	if _, ok := d.GetValue().GetKind().(*pb.Value_Unset); !ok {
		kind, value := describeValue(d.GetValue())
		t.Errorf("slot d: %s %v, want unset", kind, value)
	}

	for i, elem := range slots["ds"].GetValues() {
		if _, ok := elem.GetKind().(*pb.Value_Unset); !ok {
			kind, value := describeValue(elem)
			t.Errorf("slot ds[%d]: %s %v, want unset", i, kind, value)
		}
	}

	if got := slots["k"].GetValue().GetRealValue(); got != 2.0 {
		kind, value := describeValue(slots["k"].GetValue())
		t.Errorf("slot k: %s %v, want real 2", kind, value)
	}
	if _, ok := slots["engine"].GetValue().GetKind().(*pb.Value_InstanceId); !ok {
		kind, value := describeValue(slots["engine"].GetValue())
		t.Errorf("slot engine: %s %v, want an instance id", kind, value)
	}
}

// The empty object a valueless value-typed feature materializes is not reachable
// as a value, so it is not sent as one of the graph's instances either.
func TestInstantiate_UnsetSlotContributesNoInstance(t *testing.T) {
	resp := instantiate(t, unsetSlotModel, "unset-slot-graph", "Demo::Vehicle")
	if len(resp.Instances) != 2 {
		ids := make([]int64, 0, len(resp.Instances))
		for _, inst := range resp.Instances {
			ids = append(ids, inst.Id)
		}
		t.Errorf("instances = %v, want the object and its engine", ids)
	}
}

// Unset says what a slot holds, which is something to read and not to supply, so
// a caller sending it is told so rather than having it read as some value.
func TestProtoToValue_RejectsUnset(t *testing.T) {
	_, err := ProtoToValueIn(&pb.Value{Kind: &pb.Value_Unset{Unset: true}}, nil, nil)
	if !errors.Is(err, ErrUnsetNotAccepted) {
		t.Errorf("err = %v, want %v", err, ErrUnsetNotAccepted)
	}

	seq := &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{
		Elements: []*pb.Value{{Kind: &pb.Value_IntValue{IntValue: 1}}, {Kind: &pb.Value_Unset{Unset: true}}},
	}}}
	if _, err := ProtoToValueIn(seq, nil, nil); !errors.Is(err, ErrUnsetNotAccepted) {
		t.Errorf("in a sequence: err = %v, want %v", err, ErrUnsetNotAccepted)
	}
}

// A calc answers with the same spelling as every other RPC: a result or output
// that holds no value is sent unset rather than as an object reference.
func TestEvaluateCalc_UnsetOutputIsSentUnset(t *testing.T) {
	srv := mustNewService(t, 10)
	source := `package Demo {
	private import ScalarValues::*;
	part def Q {
		attribute d : Real;
	}
	part q : Q;
	calc def Read {
		out r = q.d;
	}
	calc reading : Read;
}
`
	hash := mustVerifyModel(t, srv, source, "verify-calc-unset")

	resp, err := srv.EvaluateCalc(context.Background(), &pb.EvaluateCalcRequest{
		ModelHash: hash,
		SymbolId:  "Demo::reading",
	})
	if err != nil {
		t.Fatalf("EvaluateCalc: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("EvaluateCalc reported %q", resp.Error)
	}
	if len(resp.Outputs) != 1 {
		t.Fatalf("got %d outputs, want r: %v", len(resp.Outputs), resp.Outputs)
	}
	if _, ok := resp.Outputs[0].GetValue().GetKind().(*pb.Value_Unset); !ok {
		kind, value := describeValue(resp.Outputs[0].GetValue())
		t.Errorf("output r: %s %v, want unset", kind, value)
	}

	invoked, err := srv.EvaluateCalc(context.Background(), &pb.EvaluateCalcRequest{
		ModelHash: hash,
		SymbolId:  "Demo::Read",
	})
	if err != nil {
		t.Fatalf("EvaluateCalc of the definition: %v", err)
	}
	if invoked.Error != "" {
		t.Fatalf("EvaluateCalc of the definition reported %q", invoked.Error)
	}
	if _, ok := invoked.GetResult().GetKind().(*pb.Value_Unset); !ok {
		kind, value := describeValue(invoked.GetResult())
		t.Errorf("result: %s %v, want unset", kind, value)
	}
}

// The unset arm is a wire-visible addition, so it is advertised as a capability
// a client can require rather than left for a client to discover.
func TestCapabilities_IncludeUnsetValue(t *testing.T) {
	for _, name := range Capabilities() {
		if name == CapabilityUnsetValue {
			return
		}
	}
	t.Errorf("capabilities = %v, want one named %q", Capabilities(), CapabilityUnsetValue)
}
