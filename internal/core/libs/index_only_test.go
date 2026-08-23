package libs

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// bodyLibrary declares what only a declaration exposes: members, a declared
// value, and a condition body.
const bodyLibrary = `package Lib {
	part def Space {
		part voids [0..*];
		attribute isSolid = isEmpty(voids);
		assert constraint { outerDimension >= innerDimension }
		attribute innerDimension;
		attribute outerDimension;
	}
	part def Room :> Space;
}`

// A library is index-only, so a cold load leaves no declaration to walk.
func TestColdLoadLeavesNoLibraryDeclarations(t *testing.T) {
	idx := loadWholeLibrary(t, writeLibrary(t, bodyLibrary), t.TempDir())
	for _, fqn := range idx.FQNs() {
		for _, sym := range idx.LookupQualified(fqn) {
			if sym.Decl != nil {
				t.Fatalf("%s keeps its declaration after a cold load, so the cache is observable", fqn)
			}
		}
	}
	if len(idx.LookupQualified("Lib::Space::isSolid")) != 1 {
		t.Fatal("the cold load did not index Lib::Space::isSolid")
	}
}

// The same contract over the bundled library every session actually loads.
func TestColdStandardLibraryLoadLeavesNoDeclarations(t *testing.T) {
	idx := symbols.NewIndex()
	if err := NewLoader(DefaultSource(), &Cache{dir: t.TempDir()}).LoadAll(idx); err != nil {
		t.Fatalf("load the standard library: %v", err)
	}
	for _, fqn := range idx.FQNs() {
		for _, sym := range idx.LookupQualified(fqn) {
			if sym.Decl != nil {
				t.Fatalf("%s keeps its declaration after a cold load of the bundled library", fqn)
			}
		}
	}
}

// A cache is a performance choice, so loading without one must reduce too:
// disabling persistence is not a way to see library declarations.
func TestLoadWithoutCacheIsIndexOnly(t *testing.T) {
	idx := symbols.NewIndex()
	if err := NewLoader(NewDirSource(writeLibrary(t, bodyLibrary)), nil).LoadAll(idx); err != nil {
		t.Fatalf("load the library: %v", err)
	}
	if len(idx.LookupQualified("Lib::Room")) != 1 {
		t.Fatal("loading without a cache did not index Lib::Room")
	}
	for _, fqn := range idx.FQNs() {
		for _, sym := range idx.LookupQualified(fqn) {
			if sym.Decl != nil {
				t.Fatalf("%s keeps its declaration when loaded without a cache", fqn)
			}
		}
	}
}

// Inherited library members are what consumers diverged over: a part listed its
// own attributes warm and dozens of library ones cold. Now neither path sees any.
func TestLibraryMembersAreInvisibleOnBothPaths(t *testing.T) {
	dir := writeLibrary(t, bodyLibrary)
	cacheDir := t.TempDir()

	for _, tc := range []struct {
		path string
		idx  *symbols.Index
	}{
		{"cold", loadWholeLibrary(t, dir, cacheDir)},
		{"warm", loadWholeLibrary(t, dir, cacheDir)},
	} {
		if got := membersOf(t, tc.idx, "Lib::Room"); len(got) != 0 {
			t.Errorf("a %s load exposes members of the library type Lib::Room: %v", tc.path, got)
		}
	}

	// Control: the same query over a non-library document, so the emptiness above
	// is the contract rather than a broken lookup.
	idx := symbols.NewIndex()
	idx.AddDocument("user.sysml", parser.New(source.New("user.sysml", []byte(bodyLibrary))).ParseFile())
	if got := membersOf(t, idx, "Lib::Room"); len(got) == 0 {
		t.Error("a parsed non-library document exposes no members either, so the check proves nothing")
	}
}

// membersOf names the members fqn exposes, its supertypes' included.
func membersOf(t *testing.T, idx *symbols.Index, fqn string) []string {
	t.Helper()
	syms := idx.LookupQualified(fqn)
	if len(syms) != 1 {
		t.Fatalf("%s names %d symbols, want 1", fqn, len(syms))
	}
	model := semantics.NewModel(resolve.New(idx))
	var names []string
	for _, member := range model.MembersOfIncludingRedefined(syms[0]) {
		names = append(names, member.Name)
	}
	sort.Strings(names)
	return names
}

// writeLibrary writes src as the only file of a fresh library directory.
func writeLibrary(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lib.sysml"), []byte(src), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	return dir
}
