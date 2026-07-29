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
	for _, name := range src.List() {
		if err := loader.Load(name, idx); err != nil {
			return err
		}
	}
	return nil
}
