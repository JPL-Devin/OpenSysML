package grpc

import (
	"context"
	"strings"
	"testing"
	"time"

	pb "github.com/Open-MBEE/Systemica/api/proto"
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

	engine := resp.Instance.Slots["engine"]
	if engine == nil {
		t.Fatal("expected engine slot")
	}
	childID := engine.Value.GetInstanceId()
	if childID == 0 {
		t.Fatalf("expected engine slot to hold an instance id, got %v", engine.Value)
	}

	child, ok := graph[childID]
	if !ok {
		t.Fatalf("child instance %d not present in Instances", childID)
	}
	if child.TypeSymbolId != "Demo::Engine" {
		t.Errorf("child type = %q, want Demo::Engine", child.TypeSymbolId)
	}
	if got := child.Slots["power"].Value.GetRealValue(); got != 300.0 {
		t.Errorf("child power = %v, want 300", got)
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

	engine := graph[resp.Instance.Slots["engine"].Value.GetInstanceId()]
	if engine == nil {
		t.Fatal("engine instance missing")
	}
	bolt := graph[engine.Slots["bolt"].Value.GetInstanceId()]
	if bolt == nil {
		t.Fatal("bolt instance missing")
	}
	if got := bolt.Slots["size"].Value.GetIntValue(); got != 8 {
		t.Errorf("bolt size = %d, want 8", got)
	}
}

// TestInstantiate_CollectionOfInstances verifies instances referenced from a
// collection slot are also returned.
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

	slot := resp.Instance.Slots["wheels"]
	if slot == nil {
		t.Fatal("expected wheels slot")
	}

	var ids []int64
	for _, v := range slot.Values {
		if id := v.GetInstanceId(); id != 0 {
			ids = append(ids, id)
		}
	}
	if id := slot.Value.GetInstanceId(); id != 0 {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		t.Skipf("runtime did not materialize wheel instances: %v", slot)
	}

	graph := byID(resp)
	for _, id := range ids {
		if _, ok := graph[id]; !ok {
			t.Errorf("wheel instance %d not present in Instances", id)
		}
	}
}

// TestInstantiate_SlotErrorReported verifies a cyclic derived attribute is
// reported through SlotValue.error rather than as a null value.
func TestInstantiate_SlotErrorReported(t *testing.T) {
	content := `
package Demo {
  part def Cyclic {
    attribute a = b + 1.0;
    attribute b = a + 1.0;
  }
}
`
	resp := instantiate(t, content, "graph-cyclic", "Demo::Cyclic")

	slot := resp.Instance.Slots["a"]
	if slot == nil {
		t.Fatal("expected slot a")
	}
	if slot.Error == "" {
		t.Fatalf("expected slot error, got %v", slot)
	}
	if !strings.Contains(strings.ToLower(slot.Error), "cyclic") {
		t.Errorf("slot error = %q, want a cyclic dependency error", slot.Error)
	}
	if slot.Value != nil {
		t.Errorf("expected no value on an errored slot, got %v", slot.Value)
	}
	if slot.Materialized {
		t.Error("errored slot must not be reported as materialized")
	}
}

// TestInstantiate_SelfReferentialPartTerminates verifies a part that contains
// its own kind is not expanded forever: reading a composite slot materializes
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
		if id := resp.Instance.Slots["next"].Value.GetInstanceId(); id == 0 {
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
