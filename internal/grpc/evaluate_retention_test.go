package grpc

import (
	"context"
	"fmt"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// Each Evaluate parses its own expression; the resolver and semantics shared
// per model hash must not retain those trees, or unique expressions grow them
// without bound.
func TestEvaluateDoesNotRetainRequestExpressions(t *testing.T) {
	const model = `
package Demo {
  calc def Twice { in x : ScalarValues::Real; return : ScalarValues::Real = x * 2.0; }
  part def Vehicle {
    attribute mass = 1500.0;
    part engine { attribute power = 100.0; }
  }
  part sedan : Vehicle {
    attribute :>> mass = 1200.0;
  }
}
`
	srv := mustNewService(t, 10)
	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: model},
		ContentHash: "evaluate-retention",
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	cached, ok := srv.cache.Get(parseResp.ModelHash)
	if !ok {
		t.Fatal("parsed model not cached")
	}
	memoSize := func() (int, int) {
		rs, release := cached.RuntimeSemantics()
		defer release()
		return rs.Resolver.MemoSize(), rs.Model.MemoSize()
	}

	evaluate := func(expr string) string {
		resp, err := srv.Evaluate(context.Background(), &pb.EvaluateRequest{
			ModelHash: parseResp.ModelHash, Expression: expr, SubjectSymbolId: "Demo::sedan",
		})
		if err != nil {
			t.Fatalf("Evaluate(%q): %v", expr, err)
		}
		return resp.GetError()
	}
	mustEvaluate := func(expr string) {
		if msg := evaluate(expr); msg != "" {
			t.Fatalf("Evaluate(%q): %s", expr, msg)
		}
	}
	// The first request resolves the model's own syntax the expressions reach
	// (Twice's parameters, the parts' types); that is the model's and stays.
	mustEvaluate("Twice(mass) + engine.power + Demo::sedan::mass + 1.0")
	before, beforeSel := memoSize()
	for i := 0; i < 50; i++ {
		mustEvaluate(fmt.Sprintf("Twice(mass) + engine.power + Demo::sedan::mass + %d.0", i))
	}
	if got, gotSel := memoSize(); got != before || gotSel != beforeSel {
		t.Fatalf("shared memo grew from %d+%d to %d+%d over 50 unique expressions",
			before, beforeSel, got, gotSel)
	}

	// A name that resolves to nothing is spell-checked, and the spellings are
	// memoized by name: a request's misspelling is the request's too.
	for i := 0; i < 50; i++ {
		if msg := evaluate(fmt.Sprintf("nosuch%d(1.0) + masss%d", i, i)); msg == "" {
			t.Fatalf("expression %d: unresolved names evaluated", i)
		}
	}
	if got, gotSel := memoSize(); got != before || gotSel != beforeSel {
		t.Fatalf("shared memo grew from %d+%d to %d+%d over 50 unresolved expressions",
			before, beforeSel, got, gotSel)
	}
}
