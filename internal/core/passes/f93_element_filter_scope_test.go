package passes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// f93Blocked names the layer the fix belongs to. The two filters in the fixtures
// are decided false by `semantics.Model.metaclassOf` (no metaclass is mapped for
// a KerML declaration kind) and by `annotationFeatureValue` (a metaclass feature
// is read from annotating metadata, of which a candidate carries none), neither
// of which `passes/` or `resolve/` can reach.
const f93Blocked = "blocked on internal/core/semantics: metaclassName maps no KerML kind and a metaclass feature is not read reflectively"

// f93LibraryDiags analyzes a library-backed fixture as the document it is named,
// whose extension decides the language. The reflective metaclasses a filter
// condition names (`Element`, `Type`, `Feature`, `Structure`) are library
// elements, so the standard library has to be loaded for the condition to compile.
func f93LibraryDiags(t *testing.T, file string) []Diagnostic {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "passes", file))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	idx := symbols.NewIndex()
	libSrc := libs.DefaultSource()
	cache, err := libs.NewCache()
	if err != nil {
		cache = nil
	}
	loader := libs.NewLoader(libSrc, cache)
	for _, name := range libSrc.List() {
		if err := loader.Load(name, idx); err != nil {
			t.Fatalf("load library %s: %v", name, err)
		}
	}
	root := parser.New(source.New(file, data)).ParseFile()
	idx.AddDocument(file, root)
	idx.ExpandWildcardImports()
	return Analyze(file, root, nil, idx)
}

// A filter condition is a predicate over a candidate element, so `@Structure`
// holds for a KerML `struct` and a metaclass feature (`Element::name`) reads the
// candidate itself. The pinned validate-kerml is silent on this fixture; we drop
// the candidates and then report their names unresolved.
func TestF93ElementFilterOverKerMLCandidates(t *testing.T) {
	t.Skip(f93Blocked)
	if diags := f93LibraryDiags(t, "f93_element_filter.kerml"); len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
}

// The same in SysML, where `@SysML::PartDefinition` already selects by metaclass:
// what remains is the metaclass feature read.
func TestF93ElementFilterOverSysMLCandidates(t *testing.T) {
	t.Skip(f93Blocked)
	if diags := f93LibraryDiags(t, "f93_element_filter.sysml"); len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
}

// The known limitation itself, pinned so it cannot move unnoticed: each dropped
// candidate surfaces as its name being unresolved, and nothing else is reported.
// Delete this test with the skips above, whose contract replaces it.
func TestF93DroppedCandidatesAreTheOnlyDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		file string
		want int
	}{
		{"f93_element_filter.kerml", 3}, // ByMetaclass::Test, ByMetaFeature::Test and its feature
		{"f93_element_filter.sysml", 1}, // ByMetaFeature::Test
	} {
		diags := f93LibraryDiags(t, tc.file)
		if len(diags) != tc.want {
			t.Fatalf("%s: got %d diagnostics, want %d: %v", tc.file, len(diags), tc.want, diags)
		}
		for _, d := range diags {
			if d.Code != "unresolved" {
				t.Errorf("%s: unexpected diagnostic %v", tc.file, d)
			}
		}
	}
}
