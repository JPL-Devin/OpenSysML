package grpc

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/Open-MBEE/Systemica/api/proto"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// libraryModel names the standard library, so a model analysed against a
// prewarmed index has something to resolve there.
const libraryModel = `package Sweep {
	import ISQ::*;
	attribute def Reading {
		attribute magnitude : ScalarValues::Real;
	}
	part def Sensor {
		attribute reading : Reading;
	}
	part sensor : Sensor;
	calc def Doubled { in x : ScalarValues::Real; return : ScalarValues::Real = x * 2; }
}`

// prewarmedService returns a service whose index pool is full, so a request
// takes a prewarmed index rather than building one.
func prewarmedService(t *testing.T, cacheSize, poolSize int) *Service {
	t.Helper()
	t.Setenv(IndexPoolEnvVar, fmt.Sprint(poolSize))
	svc := mustNewService(t, cacheSize)
	svc.Prewarm()
	deadline := time.Now().Add(2 * time.Minute)
	for svc.libIndexes.warm() < poolSize {
		if time.Now().After(deadline) {
			t.Fatalf("pool warmed only %d of %d indexes", svc.libIndexes.warm(), poolSize)
		}
		time.Sleep(2 * time.Millisecond)
	}
	return svc
}

// parseContent parses content through the ParseFile RPC, returning the response
// and the model the service cached for it.
func parseContent(t *testing.T, svc *Service, content string) (*pb.ParseFileResponse, *CachedModel) {
	t.Helper()
	resp, err := svc.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: content},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	cached, ok := svc.cache.Get(resp.ModelHash)
	if !ok {
		t.Fatalf("model %s was not cached", resp.ModelHash)
	}
	return resp, cached
}

// diagLines renders diagnostics as comparable text.
func diagLines(diags []*pb.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		line, col := int32(0), int32(0)
		if d.Span != nil {
			line, col = d.Span.StartLine, d.Span.StartCol
		}
		out = append(out, fmt.Sprintf("%s %d:%d %s", d.Severity, line, col, d.Message))
	}
	return out
}

// lookupLines renders every name an index knows with the kinds it resolves to,
// which is what a model is analysed against.
func lookupLines(idx *symbols.Index) []string {
	fqns := idx.FQNs()
	sort.Strings(fqns)
	out := make([]string, 0, len(fqns))
	for _, fqn := range fqns {
		var kinds []string
		for _, sym := range idx.LookupQualified(fqn) {
			kinds = append(kinds, fmt.Sprintf("%s/%t", sym.Kind, idx.Library(sym)))
		}
		sort.Strings(kinds)
		out = append(out, fqn+" -> "+strings.Join(kinds, ","))
	}
	return out
}

// TestPooledIndexMatchesFreshlyBuiltIndex proves prewarming does not change what
// a model resolves against: the same model analysed through a prewarmed index and
// through one built on the request path produces the same diagnostics, the same
// root, and the same qualified lookups over the whole index.
func TestPooledIndexMatchesFreshlyBuiltIndex(t *testing.T) {
	demo, err := os.ReadFile("../../examples/combined-behavioral-demo.sysml")
	if err != nil {
		t.Fatalf("read demo model: %v", err)
	}
	models := map[string]string{
		"library_references": libraryModel,
		"behavioral_demo":    string(demo),
		"parse_errors":       "package Broken { part def (((;",
	}

	pooled := prewarmedService(t, 10, 4)
	defer pooled.Close()

	// A pool of no indexes is the pre-pool path: every request builds its own.
	t.Setenv(IndexPoolEnvVar, "0")
	inline := mustNewService(t, 10)
	defer inline.Close()

	for name, content := range models {
		t.Run(name, func(t *testing.T) {
			pooledResp, pooledModel := parseContent(t, pooled, content)
			inlineResp, inlineModel := parseContent(t, inline, content)

			if pooledResp.ModelHash != inlineResp.ModelHash {
				t.Fatalf("model hash differs: %s vs %s", pooledResp.ModelHash, inlineResp.ModelHash)
			}
			if got, want := diagLines(pooledResp.Diagnostics), diagLines(inlineResp.Diagnostics); !equalLines(got, want) {
				t.Errorf("diagnostics differ:\npooled: %v\ninline: %v", got, want)
			}
			if (pooledResp.Root == nil) != (inlineResp.Root == nil) {
				t.Fatalf("root differs: pooled %v, inline %v", pooledResp.Root, inlineResp.Root)
			}
			if pooledResp.Root != nil {
				pooledKids := append([]string(nil), pooledResp.Root.ChildIds...)
				inlineKids := append([]string(nil), inlineResp.Root.ChildIds...)
				sort.Strings(pooledKids)
				sort.Strings(inlineKids)
				if !equalLines(pooledKids, inlineKids) {
					t.Errorf("root children differ:\npooled: %v\ninline: %v", pooledKids, inlineKids)
				}
			}
			if got, want := lookupLines(pooledModel.Index), lookupLines(inlineModel.Index); !equalLines(got, want) {
				t.Errorf("qualified lookups differ: %s", firstDifference(got, want))
			}
		})
	}
}

// equalLines reports whether two renderings are the same line for line.
func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// firstDifference describes where two renderings first disagree.
func firstDifference(a, b []string) string {
	for i := range a {
		if i >= len(b) {
			return fmt.Sprintf("extra line %d: %q", i, a[i])
		}
		if a[i] != b[i] {
			return fmt.Sprintf("line %d: %q vs %q", i, a[i], b[i])
		}
	}
	if len(b) > len(a) {
		return fmt.Sprintf("missing line %d: %q", len(a), b[len(a)])
	}
	return "no difference"
}

// TestParseFileServesModelsFromThePrewarmedPool is the regression guard against
// going back to loading the standard library on the request path: with the pool
// warm, no cache miss builds an index itself.
func TestParseFileServesModelsFromThePrewarmedPool(t *testing.T) {
	const models = 4
	svc := prewarmedService(t, 10, models)
	defer svc.Close()

	for i := 0; i < models; i++ {
		parseContent(t, svc, fmt.Sprintf("package P%d { part def Thing%d; }", i, i))
	}

	stats := svc.libIndexes.snapshot()
	if stats.Inline != 0 {
		t.Errorf("%d of %d models loaded the library on the request path, want 0 (stats %+v)", stats.Inline, models, stats)
	}
	if stats.Pooled != models {
		t.Errorf("served %d models from the pool, want %d (stats %+v)", stats.Pooled, models, stats)
	}
}

// TestParseFileTakesNoIndexOnACacheHit keeps the LRU doing its job: the same
// content twice costs one index, not two.
func TestParseFileTakesNoIndexOnACacheHit(t *testing.T) {
	svc := prewarmedService(t, 10, 2)
	defer svc.Close()

	first, _ := parseContent(t, svc, libraryModel)
	second, _ := parseContent(t, svc, libraryModel)
	if first.ModelHash != second.ModelHash {
		t.Fatalf("same content hashed differently: %s vs %s", first.ModelHash, second.ModelHash)
	}

	stats := svc.libIndexes.snapshot()
	if got := stats.Pooled + stats.Inline; got != 1 {
		t.Errorf("two requests for one model took %d indexes, want 1 (stats %+v)", got, stats)
	}
}

// TestCachedModelsOwnTheirIndex holds the rule that makes the pool safe: an index
// is handed out once, so no two cached models share one — including when the LRU
// evicts and when requests arrive concurrently.
func TestCachedModelsOwnTheirIndex(t *testing.T) {
	const models = 8
	svc := prewarmedService(t, 3, 4) // cache smaller than the model count, so it evicts
	defer svc.Close()

	var wg sync.WaitGroup
	hashes := make([]string, models)
	for i := 0; i < models; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, err := svc.ParseFile(context.Background(), &pb.ParseFileRequest{
				Source: &pb.ParseFileRequest_Content{Content: fmt.Sprintf("package P%d { part def Thing%d; }", i, i)},
			})
			if err != nil {
				t.Errorf("ParseFile %d: %v", i, err)
				return
			}
			hashes[i] = resp.ModelHash
		}(i)
	}
	wg.Wait()

	seen := map[*symbols.Index]string{}
	for _, hash := range hashes {
		cached, ok := svc.cache.Get(hash)
		if !ok {
			continue // evicted, which is the LRU working
		}
		if other, dup := seen[cached.Index]; dup {
			t.Fatalf("models %s and %s share one index", other, hash)
		}
		seen[cached.Index] = hash
	}
	if len(seen) == 0 {
		t.Fatal("no model survived in the cache")
	}
}

// TestIndexPoolFallsBackToBuildingInline pins that a result never depends on how
// far prewarming got: with prewarming off, every request builds its own index and
// the answers are the same ones.
func TestIndexPoolFallsBackToBuildingInline(t *testing.T) {
	t.Setenv(IndexPoolEnvVar, "0")
	svc := mustNewService(t, 10)
	defer svc.Close()
	svc.Prewarm() // a pool of no indexes has nothing to warm

	if warm := svc.libIndexes.warm(); warm != 0 {
		t.Errorf("pool of size 0 warmed %d indexes", warm)
	}
	resp, cached := parseContent(t, svc, libraryModel)
	if len(resp.Diagnostics) != 0 {
		t.Errorf("model reported diagnostics without the pool: %v", diagLines(resp.Diagnostics))
	}
	if syms := cached.Index.LookupQualified("ScalarValues::Real"); len(syms) == 0 {
		t.Error("inline index does not carry the standard library")
	}
	if stats := svc.libIndexes.snapshot(); stats.Inline != 1 || stats.Pooled != 0 {
		t.Errorf("stats %+v, want one inline checkout and no pooled one", stats)
	}
}

// TestIndexPoolCloseStopsPrewarmingAndStillServes covers shutdown: closing waits
// for the builds in flight, drops what nobody took, and leaves the service able
// to answer by building inline.
func TestIndexPoolCloseStopsPrewarmingAndStillServes(t *testing.T) {
	svc := prewarmedService(t, 10, 2)
	svc.Close()
	svc.Close() // closing twice is not an error

	if warm := svc.libIndexes.warm(); warm != 0 {
		t.Errorf("%d indexes still held after close", warm)
	}
	_, cached := parseContent(t, svc, libraryModel)
	if syms := cached.Index.LookupQualified("ScalarValues::Real"); len(syms) == 0 {
		t.Error("index built after close does not carry the standard library")
	}
	if stats := svc.libIndexes.snapshot(); stats.Inline != 1 {
		t.Errorf("stats %+v, want the request after close to have built inline", stats)
	}
	if warm := svc.libIndexes.warm(); warm != 0 {
		t.Errorf("close did not stop prewarming: %d indexes ready", warm)
	}
}

// TestIndexPoolSizeFromEnv covers the pool size a deployment asks for, including
// the values that are a typo rather than a size.
func TestIndexPoolSizeFromEnv(t *testing.T) {
	cases := []struct {
		raw     string
		want    int
		wantErr bool
	}{
		{raw: "", want: DefaultIndexPoolSize},
		{raw: "   ", want: DefaultIndexPoolSize},
		{raw: "0", want: 0},
		{raw: "2", want: 2},
		{raw: " 6 ", want: 6},
		{raw: "-1", wantErr: true},
		{raw: "many", wantErr: true},
		{raw: "1.5", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%q", tc.raw), func(t *testing.T) {
			t.Setenv(IndexPoolEnvVar, tc.raw)
			got, err := indexPoolSizeFromEnv()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("%q was accepted as size %d", tc.raw, got)
				}
				if !strings.Contains(err.Error(), IndexPoolEnvVar) {
					t.Errorf("error does not name %s: %v", IndexPoolEnvVar, err)
				}
				if _, serr := NewService(4, "test"); serr == nil {
					t.Error("NewService accepted an unusable pool size")
				}
				return
			}
			if err != nil {
				t.Fatalf("indexPoolSizeFromEnv(%q): %v", tc.raw, err)
			}
			if got != tc.want {
				t.Errorf("size %d, want %d", got, tc.want)
			}
		})
	}
}

// TestIndexPoolStopsAtItsSize keeps the pool's memory bounded: it never holds
// more indexes than it was sized for, however many requests it has served.
func TestIndexPoolStopsAtItsSize(t *testing.T) {
	var built int
	var mu sync.Mutex
	pool := newIndexPool(2, func() *symbols.Index {
		mu.Lock()
		built++
		mu.Unlock()
		return symbols.NewIndex()
	})
	defer pool.close()

	pool.prewarm()
	deadline := time.Now().Add(30 * time.Second)
	for pool.warm() < 2 {
		if time.Now().After(deadline) {
			t.Fatal("pool never warmed")
		}
		time.Sleep(time.Millisecond)
	}
	pool.prewarm() // asking again does not grow it
	time.Sleep(20 * time.Millisecond)
	if warm := pool.warm(); warm > 2 {
		t.Errorf("pool holds %d indexes, above its size of 2", warm)
	}
	for i := 0; i < 4; i++ {
		if pool.get() == nil {
			t.Fatal("pool handed out no index")
		}
	}
	if warm := pool.warm(); warm > 2 {
		t.Errorf("pool holds %d indexes after refilling, above its size of 2", warm)
	}
}

// TestIndexPoolBuildsOneIndexAtATime pins that filling the pool does not run the
// builds in parallel: the builds of one library hit the same cache records, so
// overlapping them has each build parse what its peers were about to cache.
func TestIndexPoolBuildsOneIndexAtATime(t *testing.T) {
	var mu sync.Mutex
	building, peak := 0, 0
	pool := newIndexPool(4, func() *symbols.Index {
		mu.Lock()
		building++
		if building > peak {
			peak = building
		}
		mu.Unlock()
		time.Sleep(5 * time.Millisecond)
		mu.Lock()
		building--
		mu.Unlock()
		return symbols.NewIndex()
	})
	defer pool.close()

	pool.prewarm()
	deadline := time.Now().Add(30 * time.Second)
	for pool.warm() < 4 {
		if time.Now().After(deadline) {
			t.Fatal("pool never warmed")
		}
		time.Sleep(time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if peak > 1 {
		t.Errorf("%d library builds ran at once, want them one at a time", peak)
	}
}

// BenchmarkParseFileColdPooled measures a first-time model against a prewarmed
// pool; BenchmarkParseFileColdInline measures the same work with prewarming off.
// The gap between them is the library build the pool takes off the request path.
func BenchmarkParseFileColdPooled(b *testing.B) {
	benchmarkParseFileCold(b, DefaultIndexPoolSize)
}

func BenchmarkParseFileColdInline(b *testing.B) {
	benchmarkParseFileCold(b, 0)
}

func benchmarkParseFileCold(b *testing.B, poolSize int) {
	b.Setenv(IndexPoolEnvVar, fmt.Sprint(poolSize))
	svc, err := NewService(b.N+1, "bench")
	if err != nil {
		b.Fatalf("NewService: %v", err)
	}
	defer svc.Close()
	svc.Prewarm()
	if poolSize > 0 {
		for svc.libIndexes.warm() < poolSize {
			time.Sleep(time.Millisecond)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Waiting for the refill is not part of a request's latency: a service
		// prewarms while it is idle, not while it is answering.
		if poolSize > 0 {
			b.StopTimer()
			for svc.libIndexes.warm() == 0 {
				time.Sleep(time.Millisecond)
			}
			b.StartTimer()
		}
		content := fmt.Sprintf("%s\n// model %d\n", libraryModel, i)
		if _, err := svc.ParseFile(context.Background(), &pb.ParseFileRequest{
			Source: &pb.ParseFileRequest_Content{Content: content},
		}); err != nil {
			b.Fatalf("ParseFile: %v", err)
		}
	}
}
