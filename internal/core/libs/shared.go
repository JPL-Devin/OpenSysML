package libs

import (
	"os"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// shared holds the frozen library index of each library source a process has
// loaded, keyed by the directory SYSML_LIBRARY_PATH names ("" for the bundled
// library), so a test pointing at its own library does not get another's.
var shared struct {
	mu   sync.Mutex
	base map[string]*symbols.Index
}

// SharedBase returns the frozen index holding the standard library, built on
// first use and shared by every model afterwards: the library is the same for
// every model and immutable once loaded, so one copy serves them all.
func SharedBase() *symbols.Index {
	key := os.Getenv("SYSML_LIBRARY_PATH")

	shared.mu.Lock()
	defer shared.mu.Unlock()
	if idx, ok := shared.base[key]; ok {
		return idx
	}
	idx := symbols.NewIndex()
	LoadInto(idx)
	idx.Freeze()
	if shared.base == nil {
		shared.base = map[string]*symbols.Index{}
	}
	shared.base[key] = idx
	return idx
}

// NewModelIndex returns an index for one model to add its documents to: an
// overlay over the shared standard library, which the model reads but cannot
// write to.
func NewModelIndex() *symbols.Index {
	return symbols.NewOverlay(SharedBase())
}
