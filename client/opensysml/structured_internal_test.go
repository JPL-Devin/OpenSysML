package opensysml

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// A structured value is refused before it leaves the client when the service
// lacks structured_values, since such a service would read it as null: whether
// the service predates the capability or the GetServerInfo RPC itself.
func TestStructuredInputIsNotSentWithoutStructuredValues(t *testing.T) {
	ctx := context.Background()
	model := &Model{Hash: "h"}
	vector := Vector{Real(3), Real(4)}
	for name, old := range map[string]*oldCaller{
		"predates structured_values": {t: t, capabilities: []string{CapabilityFeatureValues, CapabilityComplexValues}},
		"predates GetServerInfo":     {t: t, infoErr: &StatusError{Code: CodeUnimplemented, Message: "unknown method"}},
	} {
		t.Run(name, func(t *testing.T) {
			old.t = t
			c := &client{caller: old}
			for label, input := range map[string]Value{
				"vector":          vector,
				"array":           Array{Dimensions: []int64{1}, Elements: []Value{Int(1)}},
				"vector quantity": VectorQuantity{{Magnitude: Real(1), Unit: "m"}},
				"nested":          Sequence{Int(1), Sequence{vector}},
				"in an array":     Array{Dimensions: []int64{1}, Elements: []Value{vector}},
			} {
				_, err := c.ExecuteAction(ctx, model, "A", map[string]Value{"x": input})
				wantUnimplemented(t, "ExecuteAction "+label, err)
				_, err = c.EvaluateCalc(ctx, model, "f", Int(1), input)
				wantUnimplemented(t, "EvaluateCalc "+label, err)
			}
		})
	}
}

// An array carrying a Complex needs complex_values, as a sequence does.
func TestComplexInArrayNeedsComplexValues(t *testing.T) {
	old := &oldCaller{t: t, capabilities: []string{CapabilityStructuredValues}}
	c := &client{caller: old}
	input := Array{Dimensions: []int64{1}, Elements: []Value{Complex(complex(1, 2))}}
	_, err := c.ExecuteAction(context.Background(), &Model{Hash: "h"}, "A", map[string]Value{"z": input})
	wantUnimplemented(t, "ExecuteAction array of complex", err)
}

func TestAValueArmThisClientPredatesIsANullNamingIt(t *testing.T) {
	// A newer service's arm parses as an unknown field: a Value with no kind.
	got := valueFromProto(&pb.Value{})
	null, ok := got.(Null)
	if !ok || !strings.Contains(string(null), "does not know") {
		t.Fatalf("unknown arm read as %#v, want a Null naming it", got)
	}
	if plain := valueFromProto(&pb.Value{Kind: &pb.Value_Null{}}); plain != Null("") {
		t.Fatalf("plain null read as %#v", plain)
	}
}
