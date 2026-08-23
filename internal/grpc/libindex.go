package grpc

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// IndexPoolEnvVar names the variable holding how many prewarmed library indexes
// the service keeps. Zero disables prewarming, so every cache miss builds its
// own index inline, as it did before the pool existed.
const IndexPoolEnvVar = "SYSML_GRPC_INDEX_POOL"

// DefaultIndexPoolSize is how many prewarmed library indexes a service keeps
// when IndexPoolEnvVar is unset.
const DefaultIndexPoolSize = 4

// libraryBuilder builds one index holding the standard library and nothing
// else. It is a field of indexPool so a test can count the indexes a service
// builds, and stand in a library of its own.
type libraryBuilder func() *symbols.Index

// buildLibraryIndex loads every standard library file into a fresh index and
// caches the records of whatever had to be parsed.
func buildLibraryIndex() *symbols.Index {
	idx := symbols.NewIndex()
	src := libs.DefaultSource()
	cache, _ := libs.NewCache()                 // a cache failure only costs speed
	_ = libs.NewLoader(src, cache).LoadAll(idx) // an unreadable library file only costs its names
	return idx
}

// indexPool keeps library indexes built ahead of the requests that need them.
//
// A symbols.Index is mutable and the model added to it stays in it, so an index
// is handed out once and then belongs to the cached model that took it: the pool
// shares nothing between requests and holds no index a model can still reach.
// Checkout never waits — an empty pool builds one inline, so a result never
// depends on how far prewarming got.
type indexPool struct {
	size  int
	build libraryBuilder

	mu       sync.Mutex
	ready    []*symbols.Index
	inflight int  // indexes being built in the background right now
	target   int  // how many warm indexes to keep ready
	closed   bool // no further background builds
	stats    poolStats

	wg sync.WaitGroup // background builders, awaited by close
}

// poolStats counts what the pool did, which is how a test tells a request served
// from a prewarmed index apart from one that loaded the library itself.
type poolStats struct {
	Pooled int // checkouts served from a prewarmed index
	Inline int // checkouts that had to build an index on the request path
	Built  int // indexes built, in the background or inline
}

// newIndexPool returns a pool of at most size prewarmed indexes built by build.
// Prewarming is off until prewarm is called, so a pool nobody prewarms builds
// exactly the indexes its requests need, one per request, as before.
func newIndexPool(size int, build libraryBuilder) *indexPool {
	if size < 0 {
		size = 0
	}
	return &indexPool{size: size, build: build}
}

// indexPoolSizeFromEnv returns the pool size the environment asks for: the
// non-negative integer IndexPoolEnvVar holds, or DefaultIndexPoolSize when it is
// unset or empty. An unusable value is an error naming the variable, rather than
// a silently kept default.
func indexPoolSizeFromEnv() (int, error) {
	raw := strings.TrimSpace(os.Getenv(IndexPoolEnvVar))
	if raw == "" {
		return DefaultIndexPoolSize, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("library index pool size must be an integer, got %q (%s)", raw, IndexPoolEnvVar)
	}
	if n < 0 {
		return 0, fmt.Errorf("library index pool size must not be negative, got %d (%s)", n, IndexPoolEnvVar)
	}
	return n, nil
}

// prewarm starts building indexes up to the pool's size, and keeps refilling as
// requests take them, so the requests after it are served from a warm index. It
// returns immediately: the building happens in the background.
func (p *indexPool) prewarm() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.target = p.size
	p.fillLocked()
}

// get returns an index carrying the standard library: a prewarmed one when the
// pool has one, otherwise one built here and now. Either way the caller owns it,
// and the pool starts building a replacement.
func (p *indexPool) get() *symbols.Index {
	p.mu.Lock()
	var idx *symbols.Index
	if n := len(p.ready); n > 0 {
		idx = p.ready[n-1]
		p.ready = p.ready[:n-1]
		p.stats.Pooled++
	} else {
		p.stats.Inline++
	}
	p.fillLocked()
	p.mu.Unlock()

	if idx != nil {
		return idx
	}
	built := p.build()
	p.mu.Lock()
	p.stats.Built++
	p.mu.Unlock()
	return built
}

// fillLocked starts a background build if the pool is short of a warm index and
// none is being built. One at a time is deliberate: the builds of one library
// hit the same records, so running them in parallel would have each parse what
// the one before it cached, and spend every core doing it. The caller holds the
// lock.
func (p *indexPool) fillLocked() {
	if p.closed || p.inflight > 0 || len(p.ready) >= p.target {
		return
	}
	p.inflight++
	p.wg.Add(1)
	go p.fillOne()
}

// fillOne builds one index in the background and adds it to the pool, dropping
// it if the pool closed or filled up while it was building, then starts the next
// build the pool is short of.
func (p *indexPool) fillOne() {
	defer p.wg.Done()
	idx := p.build()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.inflight--
	p.stats.Built++
	if p.closed || len(p.ready) >= p.size {
		return
	}
	p.ready = append(p.ready, idx)
	p.fillLocked()
}

// warm is how many prewarmed indexes are ready now.
func (p *indexPool) warm() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.ready)
}

// snapshot reports what the pool has done so far.
func (p *indexPool) snapshot() poolStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stats
}

// close stops prewarming, waits for the builds already running and releases the
// indexes nobody took. get still works afterwards, building inline.
func (p *indexPool) close() {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()

	p.wg.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()
	p.ready = nil
}
