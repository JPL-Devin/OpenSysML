package libs

import (
	"errors"
	"log/slog"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/envvar"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// shared holds the frozen library index of each library source a process has
// loaded, keyed by the directory LibraryPathEnvVar names ("" for the bundled
// library), so a test pointing at its own library does not get another's.
var shared struct {
	mu   sync.Mutex
	base map[string]*symbols.Index
}

// SharedBase returns the frozen index holding the standard library, built on
// first use and shared by every model afterwards: the library is the same for
// every model and immutable once loaded, so one copy serves them all.
func SharedBase() *symbols.Index {
	key := envvar.Lookup(LibraryPathEnvVar)

	shared.mu.Lock()
	defer shared.mu.Unlock()
	if idx, ok := shared.base[key]; ok {
		return idx
	}
	idx := FrozenLibrary()
	if shared.base == nil {
		shared.base = map[string]*symbols.Index{}
	}
	shared.base[key] = idx
	return idx
}

// FrozenLibrary returns a frozen index holding the standard library: the
// embedded snapshot decoded when it matches the library files, else the files
// loaded and frozen. A snapshot that fails to decode is logged, since only a
// build defect makes one.
func FrozenLibrary() *symbols.Index {
	idx, err := SnapshotIndex()
	if err == nil {
		return idx
	}
	if !errors.Is(err, ErrSnapshotStale) {
		slog.Warn("stdlib snapshot unreadable, loading the library files", "error", err)
	}
	idx = symbols.NewIndex()
	LoadInto(idx)
	idx.Freeze()
	return idx
}

// NewModelIndex returns an index for one model to add its documents to: an
// overlay over the shared standard library, which the model reads but cannot
// write to.
func NewModelIndex() *symbols.Index {
	return symbols.NewOverlay(SharedBase())
}
