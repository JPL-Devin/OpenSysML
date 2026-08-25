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
	idx := markedStdlibIndex(t)
	stdlibPaths := stdlibDocumentPaths(t)
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

func TestLibraryMemberTableReferenceGuard(t *testing.T) {
	idx := markedStdlibIndex(t)
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
	actions := m.symbolByFQN("Actions")
	if actions == nil {
		t.Fatal("Actions is not in the standard library index")
	}

	userReference := &symbols.Symbol{Name: "userReference"}
	m.resolvingRef[userReference] = true
	if table := m.libraryMemberTable(actions); table == nil {
		t.Fatal("a user reference in flight blocked a library member table")
	}
	delete(m.resolvingRef, userReference)
	delete(m.memberTables, actions)

	m.resolvingRef[actions] = true
	if table := m.libraryMemberTable(actions); table != nil {
		t.Fatal("a library reference in flight produced a library member table")
	}
	if _, ok := m.memberTables[actions]; ok {
		t.Fatal("a transient library-reference failure was cached")
	}
	delete(m.resolvingRef, actions)
	if table := m.libraryMemberTable(actions); table == nil {
		t.Fatal("a library member table was not rebuilt after the transient failure")
	}
}

func TestLibraryMemberTableSourceAndReferenceFallbacks(t *testing.T) {
	idx := markedStdlibIndex(t)
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
	subject, contributor, name := findLibraryContributor(t, m, idx)

	m.memberTableGeneration = idx.Generation()
	m.memberTables[contributor] = &memberTable{
		entries: map[string]memberEntry{},
		sources: map[*symbols.Symbol]bool{
			contributor: true,
			subject:     true,
		},
	}
	if table := m.libraryMemberTable(subject); table != nil {
		t.Fatal("a cached table whose sources include the root was used")
	}

	want, wantOK := m.lookupContributedByWalk(subject, name)
	m.memberSources = make(map[*symbols.Symbol][]*symbols.Symbol)
	got, gotOK := m.LookupContributedMember(subject, name)
	if got != want || gotOK != wantOK {
		t.Fatalf("source-cycle fallback = (%v, %v), walk = (%v, %v)", got, gotOK, want, wantOK)
	}

	delete(m.memberTables, contributor)
	m.resolvingRef[contributor] = true
	got, gotOK = m.LookupContributedMember(subject, name)
	want, wantOK = m.lookupContributedByWalk(subject, name)
	if got != want || gotOK != wantOK {
		t.Fatalf("library-reference fallback = (%v, %v), walk = (%v, %v)", got, gotOK, want, wantOK)
	}
}

func TestLibraryMemberPlans(t *testing.T) {
	idx := markedStdlibIndex(t)
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
	subject, contributor, name := findLibraryContributor(t, m, idx)

	want, wantOK := m.lookupContributedByWalk(subject, name)
	m.memberSources = make(map[*symbols.Symbol][]*symbols.Symbol)
	got, gotOK := m.LookupContributedMember(subject, name)
	if got != want || gotOK != wantOK {
		t.Fatalf("positive plan = (%v, %v), walk = (%v, %v)", got, gotOK, want, wantOK)
	}
	plan, ok := m.memberPlans[subject]
	if !ok || plan == nil {
		t.Fatalf("member plan for %s = (%v, %v), want positive plan", symbols.FQNOf(subject), plan, ok)
	}
	if _, ok := m.memberSources[subject]; ok {
		t.Fatalf("positive plan populated member sources for %s", symbols.FQNOf(subject))
	}
	if len(plan.tables) == 0 || plan.indices[0] < 0 {
		t.Fatalf("positive plan for %s = %+v, want contributor table", symbols.FQNOf(subject), plan)
	}
	if _, ok := m.memberTables[contributor]; !ok {
		t.Fatalf("positive plan did not build contributor table for %s", symbols.FQNOf(contributor))
	}

	root := parseMemberTableDocument(t, "memberplan_user.sysml", source.KindSysML, `package P {
		part def UserBase { feature userMember; }
		part def UserDerived :> UserBase;
	}`)
	idx.AddDocumentWithKind("memberplan_user.sysml", root, source.KindSysML)
	r.ResolveDocument("memberplan_user.sysml", root)
	pkg := sym(t, idx.DocumentRoot("memberplan_user.sysml"), "P")
	base := sym(t, pkg.Scope, "UserBase")
	derived := sym(t, pkg.Scope, "UserDerived")
	got, gotOK = m.LookupContributedMember(derived, "userMember")
	want, wantOK = m.lookupContributedByWalk(derived, "userMember")
	if got != want || gotOK != wantOK {
		t.Fatalf("negative plan = (%v, %v), walk = (%v, %v)", got, gotOK, want, wantOK)
	}
	plan, ok = m.memberPlans[derived]
	if !ok || plan != nil {
		t.Fatalf("member plan for %s = (%v, %v), want cached negative plan", symbols.FQNOf(derived), plan, ok)
	}
	if table, ok := m.memberTables[base]; !ok || table != nil {
		t.Fatalf("member table for %s = (%v, %v), want cached permanent negative", symbols.FQNOf(base), table, ok)
	}
	if table := m.libraryMemberTable(base); table != nil {
		t.Fatal("cached permanent negative unexpectedly produced a table")
	}

	idx2 := markedStdlibIndex(t)
	r2 := resolve.New(idx2)
	m2 := NewModel(r2)
	r2.SetModel(m2)
	subject2, _, name2 := findLibraryContributor(t, m2, idx2)
	if _, ok := m2.LookupContributedMember(subject2, name2); !ok {
		t.Fatalf("initial lookup for %s did not find %q", symbols.FQNOf(subject2), name2)
	}
	oldPlan := m2.memberPlans[subject2]
	oldGeneration := idx2.Generation()
	generationRoot := parseMemberTableDocument(t, "memberplan_generation.sysml", source.KindSysML, "package Generation {}")
	idx2.AddDocumentWithKind("memberplan_generation.sysml", generationRoot, source.KindSysML)
	if idx2.Generation() == oldGeneration {
		t.Fatal("adding a document did not advance the index generation")
	}
	if _, ok := m2.LookupContributedMember(subject2, name2); !ok {
		t.Fatalf("post-generation lookup for %s did not find %q", symbols.FQNOf(subject2), name2)
	}
	if m2.memberPlanGeneration != idx2.Generation() {
		t.Fatalf("member plan generation = %d, want %d", m2.memberPlanGeneration, idx2.Generation())
	}
	if m2.memberPlans[subject2] == oldPlan {
		t.Fatal("post-generation lookup reused the old member plan")
	}
}

func markedStdlibIndex(t *testing.T) *symbols.Index {
	t.Helper()
	idx := stdlibIndex(t)
	for _, path := range stdlibDocumentPaths(t) {
		idx.MarkLibrary(path)
	}
	return idx
}

func findLibraryContributor(t *testing.T, m *Model, idx *symbols.Index) (*symbols.Symbol, *symbols.Symbol, string) {
	t.Helper()
	for _, path := range stdlibDocumentPaths(t) {
		for _, subject := range walkMemberTableSymbols(idx.DocumentRoot(path)) {
			if !idx.Library(subject) {
				continue
			}
			for _, contributor := range m.contributors(subject) {
				if contributor == nil || !idx.Library(contributor) {
					continue
				}
				if contributor.Scope != nil {
					for _, name := range contributor.Scope.MemberNames() {
						return subject, contributor, name
					}
				}
				for _, member := range idx.LookupDirectChildren(contributor.Name) {
					name := member.Name
					if lastIdx := lastDoubleColon(name); lastIdx >= 0 {
						name = name[lastIdx+2:]
					}
					return subject, contributor, name
				}
			}
		}
	}
	t.Fatal("standard library has no library contributor with a member")
	return nil, nil, ""
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
