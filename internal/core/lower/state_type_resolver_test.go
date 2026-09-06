package lower

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// lookupEndpoints resolves endpoints and type names from the scope tree, but
// reports the type declarations named in lookups as that lookup says.
type lookupEndpoints struct {
	scopeEndpoints
	lookups map[string]TypeLookup
	asked   []string
}

func (l *lookupEndpoints) TypeDecl(scope *symbols.Scope, qn *ast.QualifiedName) (ast.Node, *symbols.Scope, TypeLookup) {
	decl, body, ok := resolve.TypeDeclInScope(scope, qn)
	if !ok {
		return nil, nil, TypeUnresolved
	}
	name := declName(decl)
	l.asked = append(l.asked, name)
	switch l.lookups[name] {
	case TypeWithheld:
		return nil, nil, TypeWithheld
	case TypeUnresolved:
		return nil, nil, TypeUnresolved
	}
	return decl, body, TypeResolved
}

// stateTypeResolverSource holds Lib, whose content names Lib again the way the
// library's StateAction names itself, and Inner, whose content is finite.
const stateTypeResolverSource = `
	package test {
		state def Lib {
			state self : Lib;
		}
		state def Inner {
			entry; then i1;
			state i1;
		}
		state def Machine {
			entry; then nested;
			state nested : Inner;
			state lib : Lib;
		}
	}
`

// stateGraphWithLookups lowers Machine through a resolver answering lookups.
func stateGraphWithLookups(t *testing.T, lookups map[string]TypeLookup) (*StateGraph, *lookupEndpoints, error) {
	t.Helper()
	p := parser.New(source.New("test.sysml", []byte(stateTypeResolverSource)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc("test.sysml", root)
	decl := stateDefinition(t, root, "Machine")
	scope := scopeOfDecl(idx.DocumentRoot("test.sysml"), decl)
	if scope == nil {
		t.Fatal("scope for Machine missing")
	}
	endpoints := &lookupEndpoints{scopeEndpoints: scopeEndpoints{machine: scope}, lookups: lookups}
	graph, err := ToStateGraphWithEndpoints(decl, scope, endpoints)
	return graph, endpoints, err
}

// A withheld type contributes no content and is never looked up again through
// the scope tree, where Lib's self-reference would be found and recursed into.
func TestStateTypeResolverWithheldTypeContributesNothing(t *testing.T) {
	graph, endpoints, err := stateGraphWithLookups(t, map[string]TypeLookup{"Lib": TypeWithheld})
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	lib := stateNamed(graph, "lib")
	if lib == nil {
		t.Fatal("the usage typed by the withheld definition was not collected")
	}
	if self := stateNamed(graph, "self"); self != nil {
		t.Fatalf("withheld content was materialized under %v", graph.ParentState[self])
	}
	if stateNamed(graph, "i1") == nil {
		t.Fatal("a resolved type's content was not inherited alongside the withheld one")
	}
	if got := countOf(endpoints.asked, "Lib"); got != 1 {
		t.Fatalf("the resolver was asked about Lib %d times, want once", got)
	}
}

// An unresolved type falls back to the scope tree: finite content is inherited
// from there, and a self-referencing one is reported as recursive.
func TestStateTypeResolverUnresolvedTypeFallsBackToTheScopeTree(t *testing.T) {
	graph, _, err := stateGraphWithLookups(t, map[string]TypeLookup{"Inner": TypeUnresolved, "Lib": TypeWithheld})
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	i1 := stateNamed(graph, "i1")
	if i1 == nil {
		t.Fatal("content of the type the scope tree resolves was not inherited")
	}
	if parent := graph.ParentState[i1]; parent == nil || parent.Name != "nested" {
		t.Fatalf("parent of i1 = %v, want nested", parent)
	}

	_, _, err = stateGraphWithLookups(t, map[string]TypeLookup{"Lib": TypeUnresolved})
	if !errors.Is(err, ErrRecursiveStateTyping) {
		t.Fatalf("error = %v, want the scope tree's Lib recursed into", err)
	}
}

// A type the resolver resolves is inherited from the declaration it returns.
func TestStateTypeResolverResolvedTypeIsInherited(t *testing.T) {
	graph, endpoints, err := stateGraphWithLookups(t, map[string]TypeLookup{"Inner": TypeResolved, "Lib": TypeWithheld})
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	i1 := stateNamed(graph, "i1")
	if i1 == nil {
		t.Fatal("content of the resolved type was not inherited")
	}
	if !graph.IsInitial(i1) {
		t.Fatal("the inherited initial transition did not reach the usage")
	}
	if got := countOf(endpoints.asked, "Inner"); got != 1 {
		t.Fatalf("the resolver was asked about Inner %d times, want once", got)
	}
}

// countOf is how many times name occurs in names.
func countOf(names []string, name string) int {
	n := 0
	for _, candidate := range names {
		if candidate == name {
			n++
		}
	}
	return n
}
