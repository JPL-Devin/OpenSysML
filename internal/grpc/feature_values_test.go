package grpc

import (
	"testing"

	"google.golang.org/protobuf/proto"

	pb "github.com/Open-MBEE/Systemica/api/proto"
)

// Instance.feature_values is the current spelling of Instance.slots, so every
// object carries both, holding the same thing under the same names.
func TestInstantiate_FeatureValuesMirrorTheDeprecatedSlots(t *testing.T) {
	resp := instantiate(t, unsetFeatureValueModel, "feature-values", "Demo::Vehicle")

	for _, inst := range append([]*pb.Instance{resp.GetInstance()}, resp.GetInstances()...) {
		//lint:ignore SA1019 the deprecated map is what this test is about.
		values, slots := inst.GetFeatureValues(), inst.GetSlots()
		if len(values) == 0 {
			t.Fatalf("instance %d carries no feature values", inst.GetId())
		}
		if len(values) != len(slots) {
			t.Fatalf("instance %d: %d feature values, %d slots", inst.GetId(), len(values), len(slots))
		}
		for name, fv := range values {
			slot, ok := slots[name]
			if !ok {
				t.Errorf("instance %d: slots is missing %q", inst.GetId(), name)
				continue
			}
			if fv.GetFeatureName() != slot.GetFeatureName() ||
				fv.GetMaterialized() != slot.GetMaterialized() ||
				fv.GetError() != slot.GetError() ||
				!proto.Equal(fv.GetValue(), slot.GetValue()) ||
				!sameValues(fv.GetValues(), slot.GetValues()) {
				t.Errorf("instance %d, %q: feature value %v does not match slot %v", inst.GetId(), name, fv, slot)
			}
		}
	}
}

// sameValues reports whether two value lists hold the same values in order.
func sameValues(a, b []*pb.Value) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !proto.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}

// The added field is wire-visible, so a client can require it rather than
// discover it.
func TestCapabilities_IncludeFeatureValues(t *testing.T) {
	for _, name := range Capabilities() {
		if name == CapabilityFeatureValues {
			return
		}
	}
	t.Errorf("capabilities = %v, want one named %q", Capabilities(), CapabilityFeatureValues)
}
