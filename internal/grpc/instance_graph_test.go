package grpc

import (
	"context"
	"strings"
	"testing"
	"time"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// instantiate parses content and instantiates symbolID, returning the response.
func instantiate(t *testing.T, content, hash, symbolID string) *pb.InstantiateResponse {
	t.Helper()
	srv := mustNewService(t, 10)

	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: content},
		ContentHash: hash,
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	resp, err := srv.Instantiate(context.Background(), &pb.InstantiateRequest{
		ModelHash: parseResp.ModelHash,
		SymbolId:  symbolID,
	})
	if err != nil {
		t.Fatalf("Instantiate failed: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("Instantiate returned error: %s", resp.Error)
	}
	return resp
}

// byID indexes the reachable instances of a response.
func byID(resp *pb.InstantiateResponse) map[int64]*pb.Instance {
	m := make(map[int64]*pb.Instance, len(resp.Instances))
	for _, inst := range resp.Instances {
		m[inst.Id] = inst
	}
	return m
}

// TestInstantiate_ReturnsNestedInstances verifies a nested part is reachable
// through InstantiateResponse.Instances rather than only as a bare id.
func TestInstantiate_ReturnsNestedInstances(t *testing.T) {
	content := `
package Demo {
  part def Engine {
    attribute power = 300.0;
  }
  part def Vehicle {
    attribute mass = 1500.0;
    part engine : Engine;
  }
}
`
	resp := instantiate(t, content, "graph-nested", "Demo::Vehicle")

	graph := byID(resp)
	if len(graph) != 2 {
		t.Fatalf("expected 2 reachable instances, got %d", len(graph))
	}
	if _, ok := graph[resp.Instance.Id]; !ok {
		t.Error("root instance missing from Instances")
	}

	engine := resp.Instance.FeatureValues["engine"]
	if engine == nil {
		t.Fatal("expected engine feature value")
	}
	childID := engine.Value.GetInstanceId()
	if childID == 0 {
		t.Fatalf("expected engine feature value to hold an instance id, got %v", engine.Value)
	}

	child, ok := graph[childID]
	if !ok {
		t.Fatalf("child instance %d not present in Instances", childID)
	}
	if child.TypeSymbolId != "Demo::Engine" {
		t.Errorf("child type = %q, want Demo::Engine", child.TypeSymbolId)
	}
	if got := child.FeatureValues["power"].Value.GetRealValue(); got != 300.0 {
		t.Errorf("child power = %v, want 300", got)
	}
}

// TestInstantiate_ReturnsExhibitedStateValues verifies that an exhibited
// performance occurrence carries the values its machine writes.
func TestInstantiate_ReturnsExhibitedStateValues(t *testing.T) {
	content := `
package Demo {
  state def Counting {
    attribute count : Integer = 0;
    entry; then active;
    state active {
      entry action increment { assign count := count + 1; }
    }
  }
  part def Counter {
    attribute count : Integer = 10;
    exhibit state modes : Counting;
  }
}
`
	resp := instantiate(t, content, "graph-exhibited-state", "Demo::Counter")

	graph := byID(resp)
	modesID := resp.Instance.FeatureValues["modes"].Value.GetInstanceId()
	modes, ok := graph[modesID]
	if !ok {
		t.Fatalf("modes occurrence %d not present in Instances", modesID)
	}
	if got := resp.Instance.FeatureValues["count"].Value.GetIntValue(); got != 10 {
		t.Errorf("performer count = %d, want 10", got)
	}
	if got := modes.FeatureValues["count"].Value.GetIntValue(); got != 1 {
		t.Errorf("modes count = %d, want 1", got)
	}
}

// TestInstantiate_ReturnsDeepNestedInstances verifies the graph is walked
// recursively, not just one level deep.
func TestInstantiate_ReturnsDeepNestedInstances(t *testing.T) {
	content := `
package Demo {
  part def Bolt {
    attribute size = 8;
  }
  part def Engine {
    part bolt : Bolt;
  }
  part def Vehicle {
    part engine : Engine;
  }
}
`
	resp := instantiate(t, content, "graph-deep", "Demo::Vehicle")

	graph := byID(resp)
	if len(graph) != 3 {
		t.Fatalf("expected 3 reachable instances, got %d", len(graph))
	}

	engine := graph[resp.Instance.FeatureValues["engine"].Value.GetInstanceId()]
	if engine == nil {
		t.Fatal("engine instance missing")
	}
	bolt := graph[engine.FeatureValues["bolt"].Value.GetInstanceId()]
	if bolt == nil {
		t.Fatal("bolt instance missing")
	}
	if got := bolt.FeatureValues["size"].Value.GetIntValue(); got != 8 {
		t.Errorf("bolt size = %d, want 8", got)
	}
}

// TestInstantiate_CollectionOfInstances verifies instances referenced from a
// collection feature value are also returned.
func TestInstantiate_CollectionOfInstances(t *testing.T) {
	content := `
package Demo {
  part def Wheel {
    attribute radius = 0.3;
  }
  part def Vehicle {
    part wheels : Wheel[4];
  }
}
`
	resp := instantiate(t, content, "graph-collection", "Demo::Vehicle")

	fv := resp.Instance.FeatureValues["wheels"]
	if fv == nil {
		t.Fatal("expected wheels feature value")
	}

	var ids []int64
	for _, v := range fv.Values {
		if id := v.GetInstanceId(); id != 0 {
			ids = append(ids, id)
		}
	}
	if id := fv.Value.GetInstanceId(); id != 0 {
		ids = append(ids, id)
	}
	if len(ids) != 4 {
		t.Fatalf("expected 4 wheel instances, got %d: %v", len(ids), fv)
	}

	graph := byID(resp)
	for _, id := range ids {
		if _, ok := graph[id]; !ok {
			t.Errorf("wheel instance %d not present in Instances", id)
		}
	}
}

// TestInstantiate_UnmaterializedScalarFeatureValueHasNoValue verifies a scalar feature
// with nothing to materialize is reported as an empty feature value rather than as a
// value the wire format could not represent.
func TestInstantiate_UnmaterializedScalarFeatureValueHasNoValue(t *testing.T) {
	content := `
package Demo {
  part def Sensor {
    attribute reading : Real;
  }
}
`
	resp := instantiate(t, content, "graph-unmaterialized", "Demo::Sensor")

	fv := resp.Instance.FeatureValues["reading"]
	if fv == nil {
		t.Fatal("expected reading feature value")
	}
	if fv.Materialized {
		t.Fatalf("expected an unmaterialized feature value, got %v", fv)
	}
	if fv.Value != nil {
		t.Errorf("expected no value on an unmaterialized feature value, got %v", fv.Value)
	}
}

// TestInstantiate_FeatureValueErrorReported verifies a cyclic derived attribute is
// reported through FeatureValue.error rather than as a null value.
func TestInstantiate_FeatureValueErrorReported(t *testing.T) {
	content := `
package Demo {
  part def Cyclic {
    attribute a = b + 1.0;
    attribute b = a + 1.0;
  }
}
`
	resp := instantiate(t, content, "graph-cyclic", "Demo::Cyclic")

	fv := resp.Instance.FeatureValues["a"]
	if fv == nil {
		t.Fatal("expected feature value a")
	}
	if fv.Error == "" {
		t.Fatalf("expected feature value error, got %v", fv)
	}
	if !strings.Contains(strings.ToLower(fv.Error), "cyclic") {
		t.Errorf("feature value error = %q, want a cyclic dependency error", fv.Error)
	}
	if fv.Value != nil {
		t.Errorf("expected no value on an errored feature value, got %v", fv.Value)
	}
	if fv.Materialized {
		t.Error("errored feature value must not be reported as materialized")
	}
}

// TestInstantiate_SelfReferentialPartTerminates verifies a part that contains
// its own kind is not expanded forever: reading a composite feature value materializes
// the object it holds, so the walk must stop at a type already on the path.
func TestInstantiate_SelfReferentialPartTerminates(t *testing.T) {
	content := `
package Demo {
  part def Node {
    attribute v = 1;
    part next : Node;
  }
}
`
	done := make(chan *pb.InstantiateResponse, 1)
	go func() {
		done <- instantiate(t, content, "graph-self-referential", "Demo::Node")
	}()

	select {
	case resp := <-done:
		if len(resp.Instances) != 1 {
			t.Errorf("expected only the root instance, got %d", len(resp.Instances))
		}
		if id := resp.Instance.FeatureValues["next"].Value.GetInstanceId(); id == 0 {
			t.Error("expected the unexpanded child to stay a bare instance id")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Instantiate did not terminate on a self-referential part")
	}
}

// TestInstantiate_MutuallyRecursivePartsTerminate verifies the path check also
// breaks a cycle that spans two definitions.
func TestInstantiate_MutuallyRecursivePartsTerminate(t *testing.T) {
	content := `
package Demo {
  part def A {
    part b : B;
  }
  part def B {
    part a : A;
  }
}
`
	done := make(chan *pb.InstantiateResponse, 1)
	go func() {
		done <- instantiate(t, content, "graph-mutual-recursion", "Demo::A")
	}()

	select {
	case resp := <-done:
		if len(resp.Instances) != 2 {
			t.Errorf("expected A and B only, got %d instances", len(resp.Instances))
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Instantiate did not terminate on mutually recursive parts")
	}
}
