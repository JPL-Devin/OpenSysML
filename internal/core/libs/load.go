package libs

import (
	"log/slog"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// LoadInto loads every standard library file into idx, using the symbol cache
// when it is available. A failure is non-fatal: it is logged, and the index
// stays usable for a model that does not depend on the file that failed.
func LoadInto(idx *symbols.Index) {
	src := DefaultSource()
	cache, err := NewCache()
	if err != nil {
		slog.Warn("stdlib symbol cache unavailable, loading without cache", "error", err)
		cache = nil
	}
	loader := NewLoader(src, cache)

	loaded := true
	for _, name := range src.List() {
		if err := loader.Load(name, idx); err != nil {
			slog.Warn("failed to load stdlib file", "file", name, "error", err)
			loaded = false
		}
	}

	// A facade package re-exports what another library package declares.
	idx.ExpandWildcardImports()

	// Cache whatever had to be parsed, now that every library file is indexed
	// and a supertype declared in another file resolves. A record is keyed by
	// content alone, so nothing is cached from an incomplete library.
	if loaded {
		loader.Persist(idx)
	}
}
