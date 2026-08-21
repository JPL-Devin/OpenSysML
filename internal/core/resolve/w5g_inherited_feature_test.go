package resolve_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// w5gLibrary exercises the shapes the inherited-feature walk must follow
// through restored symbols: a multi-hop specialization chain, a specialization
// cycle, and a `featured by` edge.
const w5gLibrary = `standard library package Rigs {
	classifier Base {
		feature anchor;
	}
	classifier Mid specializes Base;
	classifier Rig specializes Mid;
	classifier Loop1 specializes Loop2 {
		feature strand;
	}
	classifier Loop2 specializes Loop1;
	classifier Host {
		feature slot;
	}
	feature widget featured by Host;
}`

// resolveW5GUser parses src over idx and returns the resolver's diagnostics.
func resolveW5GUser(t *testing.T, idx *symbols.Index, src string) []string {
	t.Helper()
	const name = "user.kerml"
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	m := semantics.NewModel(r)
	r.SetModel(m)
	r.ResolveDocument(name, root)
	var msgs []string
	for _, d := range r.Diagnostics {
		msgs = append(msgs, d.Message)
	}
	return msgs
}

// w5gLibraryIndexes loads w5gLibrary twice against one cache: the first load
// parses it, the second restores the persisted record.
func w5gLibraryIndexes(t *testing.T) (parsed, restored *symbols.Index) {
	t.Helper()
	dir, cacheDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rigs.kerml"), []byte(w5gLibrary), 0o600); err != nil {
		t.Fatalf("write library: %v", err)
	}
	parsed = loadLibrary(t, dir, cacheDir)
	restored = loadLibrary(t, dir, cacheDir)
	if sym := libSymbol(t, parsed, "Rigs::Rig"); sym.Decl == nil {
		t.Fatal("the first load must parse the library")
	}
	if sym := libSymbol(t, restored, "Rigs::Rig"); sym.Decl != nil {
		t.Fatal("the second load must restore the library from cache")
	}
	return parsed, restored
}

// A redefinition whose target sits several specialization hops up a restored
// chain resolves exactly as it does over the parsed library.
func TestW5GRedefinitionThroughRestoredChain(t *testing.T) {
	const user = `package App {
	private import Rigs::*;
	classifier Custom specializes Rig {
		feature redefines anchor;
	}
}`
	parsed, restored := w5gLibraryIndexes(t)
	if msgs := resolveW5GUser(t, parsed, user); len(msgs) != 0 {
		t.Fatalf("parsed library reports %v, want clean", msgs)
	}
	if msgs := resolveW5GUser(t, restored, user); len(msgs) != 0 {
		t.Fatalf("restored library reports %v, parsed was clean", msgs)
	}
}

// A specialization cycle in the library terminates the walk in both cache
// states: a feature on the cycle resolves, a missing one reports cleanly.
func TestW5GWalkIsCycleSafe(t *testing.T) {
	const resolves = `package App {
	private import Rigs::*;
	classifier Cyclic specializes Loop2 {
		feature redefines strand;
	}
}`
	const missing = `package App {
	private import Rigs::*;
	classifier Cyclic specializes Loop2 {
		feature redefines notThere;
	}
}`
	parsed, restored := w5gLibraryIndexes(t)
	for _, idx := range []*symbols.Index{parsed, restored} {
		if msgs := resolveW5GUser(t, idx, resolves); len(msgs) != 0 {
			t.Errorf("a feature on the cycle reports %v, want clean", msgs)
		}
		if msgs := resolveW5GUser(t, idx, missing); len(msgs) == 0 {
			t.Error("a feature absent from the cycle must stay unresolved")
		}
	}
}

// A `featured by` edge contributes inherited features identically whether the
// library was parsed or restored.
func TestW5GFeaturedByEdgeSurvivesRestore(t *testing.T) {
	const user = `package App {
	private import Rigs::*;
	feature gadget specializes widget {
		feature redefines slot;
	}
}`
	parsed, restored := w5gLibraryIndexes(t)
	if msgs := resolveW5GUser(t, parsed, user); len(msgs) != 0 {
		t.Fatalf("parsed library reports %v, want clean", msgs)
	}
	if msgs := resolveW5GUser(t, restored, user); len(msgs) != 0 {
		t.Fatalf("restored library reports %v, parsed was clean", msgs)
	}
}
