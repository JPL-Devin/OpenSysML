package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// The cached semantic model reads documentation through the cache's own source
// lookup; a request's runtime context must neither replace it nor be retained
// by the model once the request is over.
func TestRuntimeLeavesCachedSourceLookupInPlace(t *testing.T) {
	const model = `
package Demo {
	requirement def <'R1'> Safe {
		doc /* The crew shall return safely. */
	}
}
`
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: model},
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	cached, ok := srv.cache.Get(parsed.ModelHash)
	if !ok {
		t.Fatal("parsed model not cached")
	}
	syms := cached.Index.LookupQualified("Demo::Safe")
	if len(syms) != 1 {
		t.Fatalf("Demo::Safe resolved to %d symbols, want 1", len(syms))
	}

	rs, release := cached.RuntimeSemantics()
	before := rs.Model.SourceText()
	release()
	if before == nil {
		t.Fatal("cached runtime model has no source lookup")
	}

	for i := 0; i < 3; i++ {
		ctx, sem, release := srv.newRuntime(cached)
		if ctx.Model() != sem {
			t.Fatalf("runtime %d: context model differs from the cached model", i)
		}
		if got := sem.DocumentationOf(syms[0]); len(got) != 1 || got[0] != "The crew shall return safely." {
			t.Fatalf("runtime %d: DocumentationOf = %q, want the doc body", i, got)
		}
		release()
	}

	rs, release = cached.RuntimeSemantics()
	defer release()
	if got := rs.Model.DocumentationOf(syms[0]); len(got) != 1 {
		t.Fatalf("after requests: DocumentationOf = %q, want the doc body", got)
	}
}
