package libs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// countingSource wraps a Source and counts Read calls so we can prove a cache
// hit skips the parse path on the second Load.
type countingSource struct {
	inner Source
	reads int
}

func (c *countingSource) List() []string { return c.inner.List() }
func (c *countingSource) Read(name string) ([]byte, error) {
	c.reads++
	return c.inner.Read(name)
}

func TestLoaderCacheMissThenHit(t *testing.T) {
	cacheDir := t.TempDir()
	cache := &Cache{dir: cacheDir}
	cs := &countingSource{inner: DefaultSource()}
	ld := NewLoader(cs, cache)

	idx1 := symbols.NewIndex()
	if err := ld.Load("Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml", idx1); err != nil {
		t.Fatalf("first Load: %v", err)
	}
	if cs.reads != 1 {
		t.Fatalf("reads after first load = %d, want 1", cs.reads)
	}
	if len(idx1.LookupQualified("ScalarValues::Boolean")) != 1 {
		t.Fatal("first load did not index ScalarValues::Boolean")
	}
	ld.Persist(idx1)
	entries, _ := os.ReadDir(cacheDir)
	found := false
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".idx" {
			found = true
		}
	}
	if !found {
		t.Fatal("no .idx file written after cache miss")
	}

	idx2 := symbols.NewIndex()
	if err := ld.Load("Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml", idx2); err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if len(idx2.LookupQualified("ScalarValues")) != 1 ||
		len(idx2.LookupQualified("ScalarValues::Boolean")) != 1 {
		t.Fatal("cached load did not repopulate index")
	}

	// A symbol restored from the cache keeps its specialization targets: it has
	// no Decl, so those edges are the only way its inherited members are found.
	boolean := idx2.LookupQualified("ScalarValues::Boolean")[0]
	if len(boolean.SuperFQNs) != 1 || boolean.SuperFQNs[0] != "ScalarValues::ScalarValue" {
		t.Fatalf("supertypes of the cached Boolean = %v, want [ScalarValues::ScalarValue]", boolean.SuperFQNs)
	}
}

// A restored symbol's supertypes must match what the live-parsed AST yields, so
// the typing edge of a feature survives the round trip too.
func TestLoaderCacheKeepsTypingEdge(t *testing.T) {
	dir := t.TempDir()
	src := "package Lib { part def Engine; part e : Engine; }"
	if err := os.WriteFile(filepath.Join(dir, "lib.sysml"), []byte(src), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	ld := NewLoader(NewDirSource(dir), &Cache{dir: t.TempDir()})

	idx1 := symbols.NewIndex()
	if err := ld.Load("lib.sysml", idx1); err != nil {
		t.Fatalf("first Load: %v", err)
	}
	ld.Persist(idx1)

	idx2 := symbols.NewIndex()
	if err := ld.Load("lib.sysml", idx2); err != nil {
		t.Fatalf("second Load: %v", err)
	}
	e := idx2.LookupQualified("Lib::e")
	if len(e) != 1 {
		t.Fatalf("cached load did not register Lib::e")
	}
	if len(e[0].SuperFQNs) != 1 || e[0].SuperFQNs[0] != "Lib::Engine" {
		t.Fatalf("supertypes of the cached e = %v, want [Lib::Engine]", e[0].SuperFQNs)
	}
}

// A record whose supertypes are not all reachable yet must not be cached when
// the loader requires resolution: its key is the content alone, so it would be
// restored — minus that edge — in a context where the target is present.
func TestLoaderRequireResolvedSkipsUnresolvedRecord(t *testing.T) {
	dir := t.TempDir()
	// Specializes ScalarValues::Real, which this directory does not declare.
	src := filepath.Join(dir, "lib.sysml")
	if err := os.WriteFile(src, []byte("package Lib { attribute def Mass :> ScalarValues::Real; }"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	cacheDir := t.TempDir()
	ld := NewLoader(NewDirSource(dir), &Cache{dir: cacheDir})
	ld.RequireResolved = true

	idx := symbols.NewIndex()
	if err := ld.Load("lib.sysml", idx); err != nil {
		t.Fatalf("Load: %v", err)
	}
	ld.Persist(idx)

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("read cache dir: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".idx" {
			t.Fatalf("cached %s despite the unresolved supertype ScalarValues::Real", e.Name())
		}
	}
}

// equivalenceLibrary is a library exercising every piece of index state a
// record round-trip has to preserve: a chain of wildcard imports across sibling
// packages, a short name, an alias, and a specialization across files.
const equivalenceLibrary = `package Lib {
	public import Core::*;
	package Root {
		part def Element;
		attribute def <kg> Kilogram;
	}
	package Core {
		public import Root::*;
		part def Type :> Element;
	}
	alias Elem for Root::Element;
}`

// A persistent cache is a performance optimisation, so indexing a library by
// parsing it and indexing it by restoring its record must leave the resolver
// looking at the same thing: the same fully-qualified names, and the same
// supertypes, wildcard imports and alias targets under each of them.
func TestParsedAndRestoredIndexesAreEquivalent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lib.sysml"), []byte(equivalenceLibrary), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	cacheDir := t.TempDir()

	parsed := loadWholeLibrary(t, dir, cacheDir)   // cache miss: parses, then persists
	restored := loadWholeLibrary(t, dir, cacheDir) // cache hit: restores the record

	// A pair of equally empty indexes would compare equal, so pin the content
	// that only the full import chain Lib -> Core -> Root can produce.
	for _, fqn := range []string{"Lib::Root::Element", "Lib::Core::Element", "Lib::Element", "Lib::kg"} {
		if len(parsed.LookupQualified(fqn)) != 1 {
			t.Fatalf("the parsed index does not register %s", fqn)
		}
	}

	want := snapshotIndex(parsed)
	got := snapshotIndex(restored)
	for _, fqn := range want.fqns {
		if got.entries[fqn] != want.entries[fqn] {
			t.Errorf("%s:\n  parsed:   %s\n  restored: %s", fqn, want.entries[fqn], got.entries[fqn])
		}
	}
	for _, fqn := range got.fqns {
		if _, ok := want.entries[fqn]; !ok {
			t.Errorf("%s is registered only after a cache restore: %s", fqn, got.entries[fqn])
		}
	}
}

// loadWholeLibrary indexes every file of the library in dir through a loader
// backed by cacheDir, expanding imports and persisting records exactly as
// model.loadStdlib does.
func loadWholeLibrary(t *testing.T, dir, cacheDir string) *symbols.Index {
	t.Helper()
	src := NewDirSource(dir)
	ld := NewLoader(src, &Cache{dir: cacheDir})
	idx := symbols.NewIndex()
	for _, name := range src.List() {
		if err := ld.Load(name, idx); err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
	}
	idx.ExpandWildcardImports()
	ld.Persist(idx)
	return idx
}

// indexView is a description of an index that does not depend on how it was
// populated: every registered fully-qualified name, and the resolver-visible
// state of the symbols under it. It deliberately ignores symbols.Symbol.Name,
// which holds a local name on the parse path and a qualified one on the
// restore path.
type indexView struct {
	fqns    []string
	entries map[string]string
}

func snapshotIndex(idx *symbols.Index) indexView {
	r := resolve.New(idx)
	view := indexView{fqns: idx.FQNs(), entries: map[string]string{}}
	for _, fqn := range view.fqns {
		var descs []string
		for _, sym := range idx.LookupQualified(fqn) {
			descs = append(descs, describeSymbol(sym, idx, r))
		}
		sort.Strings(descs)
		view.entries[fqn] = fmt.Sprintf("%v imports=%v", descs, idx.WildcardImportsOf(fqn))
	}
	return view
}

// describeSymbol renders the state a symbol contributes to name resolution.
// A parsed symbol carries it in its declaration, a restored one in the fields
// its record populated; both must describe the same thing.
func describeSymbol(sym *symbols.Symbol, idx *symbols.Index, r *resolve.Resolver) string {
	supers, alias, short := sym.SuperFQNs, sym.AliasTargetFQN, sym.ShortName
	if sym.Decl != nil {
		supers, _ = supersOf(sym, idx, r)
		alias = aliasTargetOf(sym.Decl)
	}
	sort.Strings(supers)
	return fmt.Sprintf("kind=%v short=%q supers=%v alias=%q", sym.Kind, short, supers, alias)
}

func TestIndexAddRecordsRemovable(t *testing.T) {
	idx := symbols.NewIndex()
	idx.AddRecords("lib.kerml", []symbols.RecordEntry{
		{FQN: "P", Kind: symbols.SymbolPackage},
		{FQN: "P::N", Kind: symbols.SymbolNamespace},
	})
	if len(idx.LookupQualified("P::N")) != 1 {
		t.Fatal("AddRecords did not register P::N")
	}
	idx.RemoveDocument("lib.kerml")
	if len(idx.LookupQualified("P::N")) != 0 {
		t.Fatal("RemoveDocument did not drop record-added symbols")
	}
}
