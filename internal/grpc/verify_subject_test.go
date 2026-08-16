package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/Systemica/api/proto"
)

// nestedSubjectModelSource redefines a nested feature on the object, so the
// object a check is about is the nested one rather than the one named.
const nestedSubjectModelSource = `package Demo {
	part def Wheel {
		attribute pressure = 30.0;
		constraint inflated {
			assert pressure > 20.0;
		}
	}
	part def Car {
		part wheel : Wheel;
	}
	part flat : Car {
		part :>> wheel {
			attribute :>> pressure = 5.0;
		}
	}
}
`

// A verdict about a condition reached through a nested redefinition names the
// nested object it was evaluated against, so a client reading the reported slots
// sees the values behind the verdict.
func TestVerifyConstraintNamesTheNestedSubject(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, nestedSubjectModelSource, "verify-nested-subject")

	resp, err := srv.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{
		ModelHash:       hash,
		SymbolId:        "Demo::Wheel::inflated",
		SubjectSymbolId: "Demo::flat",
	})
	if err != nil {
		t.Fatalf("VerifyConstraint: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("VerifyConstraint reported %q", resp.Error)
	}
	if resp.Verdict.Holds {
		t.Error("inflated on flat: holds = true, want the nested redefinition to violate it")
	}
	if resp.Verdict.InstanceTypeId == "Demo::Car" {
		t.Errorf("instance_type_id = %q, want the nested wheel object rather than the car named", resp.Verdict.InstanceTypeId)
	}
	var named *pb.Instance
	for _, inst := range resp.Instances {
		if inst.Id == resp.Verdict.InstanceId {
			named = inst
		}
	}
	if named == nil {
		t.Fatalf("verdict names instance %d, which the response does not carry", resp.Verdict.InstanceId)
	}
	pressure, ok := named.Slots["pressure"]
	if !ok {
		t.Fatalf("the object the verdict names has no pressure slot: %v", named.Slots)
	}
	if pressure.Value.GetRealValue() != 5.0 {
		t.Errorf("pressure = %v, want the redefined 5", pressure)
	}
}
