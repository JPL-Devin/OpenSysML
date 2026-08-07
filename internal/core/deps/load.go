package deps

import (
	"github.com/Open-MBEE/Systemica/internal/core/libs"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// loadDir parses every .sysml/.kerml file in dir and registers its symbols into
// idx, reusing cache (may be nil) to skip re-parsing already-cached content.
func loadDir(dir string, idx *symbols.Index, cache *libs.Cache) error {
	src := libs.NewDirSource(dir)
	loader := libs.NewLoader(src, cache)
	// Dependencies load before the stdlib, so a record whose supertypes are not
	// all reachable yet must not be cached under its content-only key.
	loader.RequireResolved = true
	for _, name := range src.List() {
		if err := loader.Load(name, idx); err != nil {
			return err
		}
	}
	// Records are written once the whole directory is indexed, so a
	// specialization target in a sibling file resolves.
	loader.Persist(idx)
	return nil
}
