package semantics

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func TestLibraryMemberLookupMatchesWalk(t *testing.T) {
	idx := stdlibIndex(t)
	stdlibPaths := stdlibDocumentPaths(t)
	for _, path := range stdlibPaths {
		idx.MarkLibrary(path)
	}
	sysmlRoot := parseMemberTableDocument(t, "membertable_user.sysml", source.KindSysML, `package P {
		part def UserBase { feature userMember; }
		part def UserDerived :> UserBase;
		part def CycleA :> CycleB { feature cycleAMember; }
		part def CycleB :> CycleA { feature cycleBMember; }
		alias UserAlias for UserBase;
		part usage : UserAlias;
	}`)
	kermlRoot := parseMemberTableDocument(t, "membertable_user.kerml", source.KindKerML, `package K {
		class UserClass { feature classMember; }
		feature userFeature;
	}`)
	idx.AddDocumentWithKind("membertable_user.sysml", sysmlRoot, source.KindSysML)
	idx.AddDocumentWithKind("membertable_user.kerml", kermlRoot, source.KindKerML)
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
	r.ResolveDocument("membertable_user.sysml", sysmlRoot)
	r.ResolveDocument("membertable_user.kerml", kermlRoot)

	var all []*symbols.Symbol
	for _, name := range []string{"membertable_user.sysml", "membertable_user.kerml"} {
		all = append(all, walkMemberTableSymbols(idx.DocumentRoot(name))...)
	}
	for _, path := range stdlibPaths {
		all = append(all, walkMemberTableSymbols(idx.DocumentRoot(path))...)
	}

	var found *symbols.Symbol
	for _, fqn := range idx.FQNs() {
		candidates := idx.LookupQualified(fqn)
		if len(candidates) > 0 && idx.Library(candidates[0]) {
			found = candidates[0]
			break
		}
	}
	if table := m.libraryMemberTable(found); table == nil || len(m.memberTables) == 0 {
		t.Fatalf("library member fast path did not build a table for %s", idx.GetFQN(found))
	}
	derived := sym(t, sym(t, idx.DocumentRoot("membertable_user.sysml"), "P").Scope, "UserDerived")
	if table := m.libraryMemberTable(derived); table != nil {
		t.Fatal("user type unexpectedly received a library member table")
	}

	for _, sym := range all {
		if sym == nil {
			continue
		}
		names := memberTableNames(m, sym)
		for _, name := range names {
			got, gotOK := m.LookupContributedMember(sym, name)
			want, wantOK := m.lookupContributedByWalk(sym, name)
			if got != want || gotOK != wantOK {
				t.Errorf("%s %q: fast=(%v, %v), walk=(%v, %v)",
					symbols.FQNOf(sym), name, got, gotOK, want, wantOK)
			}
		}
	}

	generation := idx.Generation()
	generationRoot := parseMemberTableDocument(t, "membertable_generation.sysml", source.KindSysML, "package Generation {}")
	idx.AddDocumentWithKind("membertable_generation.sysml", generationRoot, source.KindSysML)
	if idx.Generation() == generation {
		t.Fatal("adding a document did not advance the index generation")
	}
	actions := m.symbolByFQN("Actions")
	if actions == nil {
		t.Fatal("Actions is not in the standard library index")
	}
	if table := m.libraryMemberTable(actions); table == nil {
		t.Fatal("library member table was not rebuilt after index generation changed")
	}
	if m.memberTableGeneration != idx.Generation() || len(m.memberTables) != 1 {
		t.Fatalf("member table cache after generation change = generation %d, %d entries; want %d, one entry",
			m.memberTableGeneration, len(m.memberTables), idx.Generation())
	}
}

func parseMemberTableDocument(t *testing.T, name string, kind source.Kind, text string) *ast.RootNamespace {
	t.Helper()
	p := parser.New(source.New(name, []byte(text)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics for %s: %v", name, p.Diagnostics)
	}
	return root
}

func stdlibDocumentPaths(t *testing.T) []string {
	t.Helper()
	root := filepath.Join("..", "libs", "stdlib")
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext == ".sysml" || ext == ".kerml" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk standard library: %v", err)
	}
	return paths
}

func walkMemberTableSymbols(scope *symbols.Scope) []*symbols.Symbol {
	if scope == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, sym := range scope.Members() {
		out = append(out, sym)
		out = append(out, walkMemberTableSymbols(sym.Scope)...)
	}
	return out
}

func memberTableNames(m *Model, sym *symbols.Symbol) []string {
	seen := make(map[string]bool)
	var names []string
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	for _, source := range m.MemberSources(sym) {
		if source.Scope != nil {
			for _, name := range source.Scope.MemberNames() {
				add(name)
			}
			continue
		}
		for _, member := range m.resolver.Index().LookupDirectChildren(source.Name) {
			name := member.Name
			if lastIdx := lastDoubleColon(name); lastIdx >= 0 {
				name = name[lastIdx+2:]
			}
			add(name)
		}
	}
	return names
}
