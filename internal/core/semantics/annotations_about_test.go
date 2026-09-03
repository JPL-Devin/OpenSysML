package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// buildOverlayModel indexes libSrc as a frozen base — the shape the shared
// standard library index has — and userSrc as the workspace document of an
// overlay over it, returning the model and both document root scopes.
func buildOverlayModel(t *testing.T, libSrc, userSrc string) (*Model, *symbols.Scope, *symbols.Scope) {
	t.Helper()
	base := symbols.NewIndex()
	addTestDoc(t, base, "lib.sysml", libSrc)
	base.MarkLibrary("lib.sysml")
	base.Freeze()

	over := symbols.NewOverlay(base)
	addTestDoc(t, over, "user.sysml", userSrc)
	over.ExpandWildcardImports()

	r := resolve.New(over)
	m := NewModel(r)
	r.SetModel(m)
	return m, over.DocumentRoot("lib.sysml"), over.DocumentRoot("user.sysml")
}

func addTestDoc(t *testing.T, idx *symbols.Index, name, src string) {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics for %s: %v", name, p.Diagnostics)
	}
	idx.AddDocument(name, root)
}

// annotationTypes reports the FQNs of the metadata types annotating sym.
func annotationTypes(m *Model, sym *symbols.Symbol) []string {
	var out []string
	for _, facts := range m.AnnotationFactsOf(sym) {
		out = append(out, facts.TypeFQN)
	}
	return out
}

// An `about` annotation declared by a frozen library document is read from the
// cache its Freeze built and found exactly as walking the document found it.
func TestAboutAnnotationDeclaredByAFrozenLibraryDocument(t *testing.T) {
	m, lib, _ := buildOverlayModel(t, `
		metadata def Safety;
		part def Belt;
		metadata s : Safety about Belt;
	`, "part x;")

	belt := sym(t, lib, "Belt")
	types := annotationTypes(m, belt)
	if len(types) != 1 || types[0] != "Safety" {
		t.Fatalf("annotations of Belt = %v, want [Safety]", types)
	}
	annotated := m.AboutAnnotatedSymbols()
	if len(annotated) != 1 || annotated[0] != belt {
		t.Fatalf("AboutAnnotatedSymbols = %v, want [Belt]", annotated)
	}
}

// A workspace document's `about` annotation targeting a frozen library element
// is still found: the workspace document is walked, cache or no cache.
func TestAboutAnnotationFromWorkspaceTargetsALibraryElement(t *testing.T) {
	m, lib, _ := buildOverlayModel(t, `
		metadata def Safety;
		part def Belt;
	`, "metadata s : Safety about Belt;")

	types := annotationTypes(m, sym(t, lib, "Belt"))
	if len(types) != 1 || types[0] != "Safety" {
		t.Fatalf("annotations of Belt = %v, want [Safety]", types)
	}
}

// A workspace document's `about` annotation targeting another workspace
// element is found over an overlay, and combines with the library's own.
func TestAboutAnnotationsFromLibraryAndWorkspaceCombine(t *testing.T) {
	m, lib, user := buildOverlayModel(t, `
		metadata def Safety;
		part def Belt;
		metadata s : Safety about Belt;
	`, `
		metadata def Comfort;
		part radio;
		metadata c : Comfort about radio;
	`)

	if types := annotationTypes(m, sym(t, user, "radio")); len(types) != 1 || types[0] != "Comfort" {
		t.Fatalf("annotations of radio = %v, want [Comfort]", types)
	}
	if types := annotationTypes(m, sym(t, lib, "Belt")); len(types) != 1 || types[0] != "Safety" {
		t.Fatalf("annotations of Belt = %v, want [Safety]", types)
	}
}
