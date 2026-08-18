package solve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// fixture indexes a model over the standard library and returns a runtime
// context and the index to look symbols up in. path names the document, which
// appears in the provenance a script records.
func fixture(t *testing.T, path, src string) (*runtime.Context, *symbols.Index) {
	t.Helper()
	idx := symbols.NewIndex()
	loadLibraries(t, idx)
	sf := source.New(path, []byte(src))
	idx.AddDocument(path, parser.New(sf).ParseFile())
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	ctx := runtime.NewContext(semantics.NewModel(resolver), resolver, 10000)
	ctx.RegisterSource(sf)
	return ctx, idx
}

// fixtureFile indexes a .sysml file from testdata, named by its base name so a
// script's provenance does not carry the checkout's path.
func fixtureFile(t *testing.T, name string) (*runtime.Context, *symbols.Index) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return fixture(t, name, string(src))
}

// loadLibraries indexes the bundled standard library, which is what makes units,
// quantity value types and the scalar value types resolve.
func loadLibraries(t *testing.T, idx *symbols.Index) {
	t.Helper()
	cache, err := libs.NewCache()
	if err != nil {
		t.Fatalf("library cache: %v", err)
	}
	src := libs.DefaultSource()
	loader := libs.NewLoader(src, cache)
	for _, name := range src.List() {
		if err := loader.Load(name, idx); err != nil {
			t.Fatalf("load library %s: %v", name, err)
		}
	}
}

// symbolNamed returns the single symbol with that qualified name.
func symbolNamed(t *testing.T, idx *symbols.Index, fqn string) *symbols.Symbol {
	t.Helper()
	matches := idx.LookupQualified(fqn)
	if len(matches) != 1 {
		t.Fatalf("%s matched %d symbols, want 1", fqn, len(matches))
	}
	return matches[0]
}
