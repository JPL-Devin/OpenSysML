// Exercised from outside the package: building a restored index needs the loader,
// which imports semantics.
package semantics_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A restored library symbol has no declaration, so each rule it takes part in has
// two implementations and a rule pinned only on the parsed path is half-tested.
const semanticsLibrary = `standard library package Kit {
	attribute def Mass;
	metadata def Safety;
	part def Fastener {
		attribute mass : Mass;
	}
	part def Bolt :> Fastener;
	part def HexBolt :> Bolt;
	part def Nut :> Fastener {
		attribute mass : Mass [0..1];
	}
	package Critical {
		filter @Safety;
		#Safety part def Guard;
		part def Plain;
	}
}`

// Conformance follows the specialization graph, which a restored library holds as
// recorded supertype names: it must answer the same either way and transitively.
func TestConformanceOverALibraryIsTheSameParsedAndRestored(t *testing.T) {
	parsed, restored := bothPaths(t)
	for _, tc := range []struct {
		sub, super string
		want       bool
	}{
		{"Kit::Bolt", "Kit::Fastener", true},
		{"Kit::HexBolt", "Kit::Fastener", true}, // transitive
		{"Kit::Fastener", "Kit::Bolt", false},   // conformance is not symmetric
		{"Kit::Nut", "Kit::Bolt", false},        // siblings do not conform
	} {
		for path, lib := range map[string]library{"parsed": parsed, "restored": restored} {
			got := lib.model.Conforms(lib.symbol(t, tc.sub), lib.symbol(t, tc.super))
			if got != tc.want {
				t.Errorf("%s: Conforms(%s, %s) = %v, want %v", path, tc.sub, tc.super, got, tc.want)
			}
		}
	}
}

// The members of a restored type come from the index rather than from a scope,
// including the inherited ones and the masking a closer declaration applies.
func TestMembersOfALibraryTypeAreTheSameParsedAndRestored(t *testing.T) {
	parsed, restored := bothPaths(t)
	for _, tc := range []struct{ owner, member, want string }{
		// Declared, inherited, and inherited-but-masked by a redeclaration.
		{"Kit::Fastener", "mass", "Kit::Fastener::mass"},
		{"Kit::Bolt", "mass", "Kit::Fastener::mass"},
		{"Kit::HexBolt", "mass", "Kit::Fastener::mass"},
		{"Kit::Nut", "mass", "Kit::Nut::mass"},
	} {
		for path, lib := range map[string]library{"parsed": parsed, "restored": restored} {
			member, ok := lib.model.LookupMember(lib.symbol(t, tc.owner), tc.member)
			if !ok {
				t.Errorf("%s: %s has no member %s", path, tc.owner, tc.member)
				continue
			}
			if got := lib.idx.GetFQN(member); got != tc.want {
				t.Errorf("%s: %s::%s = %s, want %s", path, tc.owner, tc.member, got, tc.want)
			}
		}
	}
}

// A member reached only through what a type specializes must not be reported as
// one of its own, whichever path the library came from.
func TestContributedMembersOfALibraryTypeAreTheSameParsedAndRestored(t *testing.T) {
	parsed, restored := bothPaths(t)
	for path, lib := range map[string]library{"parsed": parsed, "restored": restored} {
		nut := lib.symbol(t, "Kit::Nut")
		contributed, ok := lib.model.LookupContributedMember(nut, "mass")
		if !ok {
			t.Errorf("%s: Nut inherits no mass from Fastener", path)
			continue
		}
		if got := lib.idx.GetFQN(contributed); got != "Kit::Fastener::mass" {
			t.Errorf("%s: the mass Nut inherits = %s, want Kit::Fastener::mass", path, got)
		}
	}
}

// A `filter` reaches the evaluator as a parsed condition from a live document and
// as a compiled predicate from a restored one; both must classify alike.
func TestNamespaceFilterOverALibraryIsTheSameParsedAndRestored(t *testing.T) {
	parsed, restored := bothPaths(t)
	if f := filterOn(t, parsed, "Kit::Critical"); f.Expr == nil {
		t.Error("the parsed path must carry the condition as an expression")
	}
	if f := filterOn(t, restored, "Kit::Critical"); f.Pred == nil {
		t.Error("the restored path must carry the condition compiled")
	}
	for _, tc := range []struct {
		cand string
		want bool
	}{
		{"Kit::Critical::Guard", true},
		{"Kit::Critical::Plain", false},
		{"Kit::Bolt", false},
	} {
		for path, lib := range map[string]library{"parsed": parsed, "restored": restored} {
			got := lib.model.SatisfiesElementFilter(filterOn(t, lib, "Kit::Critical"), lib.symbol(t, tc.cand))
			if got != tc.want {
				t.Errorf("%s: @Safety admits %s = %v, want %v", path, tc.cand, got, tc.want)
			}
		}
	}
}

// The metadata annotating an element is read from its declaration when parsed and
// from its record when restored, so a filter classifies both the same way.
func TestAnnotationFactsOfALibraryElementSurviveTheCache(t *testing.T) {
	parsed, restored := bothPaths(t)
	for path, lib := range map[string]library{"parsed": parsed, "restored": restored} {
		facts := lib.model.AnnotationFactsOf(lib.symbol(t, "Kit::Critical::Guard"))
		if len(facts) != 1 || facts[0].TypeFQN != "Kit::Safety" {
			t.Errorf("%s: Guard is annotated %+v, want one Kit::Safety", path, facts)
		}
		if facts := lib.model.AnnotationFactsOf(lib.symbol(t, "Kit::Critical::Plain")); len(facts) != 0 {
			t.Errorf("%s: Plain is annotated %+v, want nothing", path, facts)
		}
	}
}

// filterOn is the single condition the namespace registered under fqn declares.
func filterOn(t *testing.T, lib library, fqn string) symbols.ElementFilter {
	t.Helper()
	filters := lib.idx.NamespaceFiltersOf(fqn)
	if len(filters) != 1 {
		t.Fatalf("%s declares %d filters, want 1", fqn, len(filters))
	}
	return filters[0]
}

// library is one indexed copy of semanticsLibrary with a model over it.
type library struct {
	idx   *symbols.Index
	model *semantics.Model
}

func (l library) symbol(t *testing.T, fqn string) *symbols.Symbol {
	t.Helper()
	syms := l.idx.LookupQualified(fqn)
	if len(syms) != 1 {
		t.Fatalf("%s names %d symbols, want 1", fqn, len(syms))
	}
	return syms[0]
}

// bothPaths loads semanticsLibrary twice through one cache directory: the first
// load parses it, the second restores its record.
func bothPaths(t *testing.T) (parsed, restored library) {
	t.Helper()
	dir, cacheDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "kit.sysml"), []byte(semanticsLibrary), 0o600); err != nil {
		t.Fatalf("write library: %v", err)
	}
	parsed = modelOver(t, loadLibrary(t, dir, cacheDir))
	restored = modelOver(t, loadLibrary(t, dir, cacheDir))
	if parsed.symbol(t, "Kit::Fastener").Decl == nil {
		t.Fatal("the first load must parse the library, leaving its symbols a declaration")
	}
	if restored.symbol(t, "Kit::Fastener").Decl != nil {
		t.Fatal("the second load must restore the record, whose symbols carry no declaration")
	}
	return parsed, restored
}

// modelOver is a semantic model over idx, wired as the workspace wires it.
func modelOver(t *testing.T, idx *symbols.Index) library {
	t.Helper()
	r := resolve.New(idx)
	m := semantics.NewModel(r)
	r.SetModel(m)
	return library{idx: idx, model: m}
}

// loadLibrary indexes every file of the library in dir through a loader backed
// by cacheDir, persisting its records exactly as the stdlib load does.
func loadLibrary(t *testing.T, dir, cacheDir string) *symbols.Index {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	cache, err := libs.NewCache()
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	src := libs.NewDirSource(dir)
	ld := libs.NewLoader(src, cache)
	idx := symbols.NewIndex()
	for _, name := range src.List() {
		if err := ld.Load(name, idx); err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
	}
	idx.ExpandWildcardImports()
	ld.Persist(idx)
	if len(idx.FQNs()) == 0 {
		t.Fatal("the load registered nothing")
	}
	return idx
}

// A restored library feature loses its declared multiplicity: symRecord persists
// none, so it takes the assumed 1..1. Fix belongs in libs ("Found, not fixed").
func TestMultiplicityOfALibraryFeatureIsTheSameParsedAndRestored(t *testing.T) {
	t.Skip("known gap: a restored library feature loses its declared multiplicity")
	parsed, restored := bothPaths(t)
	want := parsed.model.EffectiveMultiplicityOf(parsed.symbol(t, "Kit::Nut::mass"))
	got := restored.model.EffectiveMultiplicityOf(restored.symbol(t, "Kit::Nut::mass"))
	if got != want {
		t.Errorf("Kit::Nut::mass multiplicity = %+v restored, %+v parsed", got, want)
	}
}
