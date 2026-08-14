package resolve

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// stubJudge stands in for the semantic model when the question under test is
// where resolution consults a filter, not what a condition means: it admits the
// elements whose name carries the marker, as `@Safety` admits the annotated
// ones. The evaluator itself is tested in semantics.
type stubJudge struct{ marker string }

func (stubJudge) LookupMember(*symbols.Symbol, string) (*symbols.Symbol, bool) {
	return nil, false
}

func (stubJudge) LookupContributedMember(*symbols.Symbol, string) (*symbols.Symbol, bool) {
	return nil, false
}

func (j stubJudge) SatisfiesElementFilter(_ symbols.ElementFilter, cand *symbols.Symbol) bool {
	return cand != nil && strings.Contains(cand.Name, j.marker)
}

// expandedIndexOf indexes docs and expands their wildcard imports, so the names
// an import surfaces are registered the way a workspace registers them.
func expandedIndexOf(t *testing.T, docs map[string]string) *symbols.Index {
	t.Helper()
	idx := indexOf(t, docs)
	idx.ExpandWildcardImports()
	return idx
}

// judgingResolver resolves over idx with a filter judge attached, the way a
// workspace attaches its semantic model.
func judgingResolver(idx *symbols.Index) *Resolver {
	r := New(idx)
	r.SetModel(stubJudge{marker: "Safe"})
	return r
}

const filterLib = `package Lib {
	part def SafeBelt;
	part def Radio;
}`

// An import's filter clause restricts what that import brings in (SysML v2
// 7.4.4): a name it rejects is not resolvable through the importing namespace,
// by either the unqualified or the qualified route.
func TestFilteredImportHidesARejectedElement(t *testing.T) {
	idx := expandedIndexOf(t, map[string]string{
		"lib.sysml": filterLib,
		"app.sysml": "package App { public import Lib::*[@Safety]; }",
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	if _, ok := judgingResolver(idx).ResolveName(app, "SafeBelt", ident("SafeBelt")); !ok {
		t.Error("the filter admits SafeBelt, which must resolve through the filtered import")
	}
	if _, ok := judgingResolver(idx).ResolveName(app, "Radio", ident("Radio")); ok {
		t.Error("the filter rejects Radio, which must not resolve through the filtered import")
	}
	if _, ok := judgingResolver(idx).ResolveQualified(nil, qn(false, "App", "Radio")); ok {
		t.Error("App::Radio names an element the import filtered out, so it must not resolve")
	}
	if _, ok := judgingResolver(idx).ResolveQualified(nil, qn(false, "Lib", "Radio")); !ok {
		t.Error("Lib::Radio is a declared member of Lib and no filter applies to it")
	}
}

// A namespace's `filter` members restrict the memberships its imports bring in,
// and leave the members it declares itself alone (KerML 8.2.4).
func TestNamespaceFilterKeepsDeclaredMembers(t *testing.T) {
	idx := expandedIndexOf(t, map[string]string{
		"lib.sysml": filterLib,
		"app.sysml": `package App {
			public import Lib::*;
			filter @Safety;
			part def Radiator;
		}`,
	})

	if _, ok := judgingResolver(idx).ResolveQualified(nil, qn(false, "App", "Radiator")); !ok {
		t.Error("a filter must not hide Radiator, which App declares itself")
	}
	if _, ok := judgingResolver(idx).ResolveQualified(nil, qn(false, "App", "Radio")); ok {
		t.Error("App's filter must hide Radio, which App only imports")
	}
	if _, ok := judgingResolver(idx).ResolveQualified(nil, qn(false, "App", "SafeBelt")); !ok {
		t.Error("App's filter admits SafeBelt, which it imports")
	}
}

// A filtered namespace re-exports onward only what its filter selects, so an
// outside importer of it sees the same subset — the filter travels with the
// membership rather than with one lookup route.
func TestNamespaceFilterAppliesToOnwardImporters(t *testing.T) {
	idx := expandedIndexOf(t, map[string]string{
		"lib.sysml": filterLib,
		"mid.sysml": "package Mid { public import Lib::*; filter @Safety; }",
		"app.sysml": "package App { public import Mid::*; }",
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	if _, ok := judgingResolver(idx).ResolveName(app, "SafeBelt", ident("SafeBelt")); !ok {
		t.Error("SafeBelt passes Mid's filter, so importing Mid must reach it")
	}
	if _, ok := judgingResolver(idx).ResolveName(app, "Radio", ident("Radio")); ok {
		t.Error("Radio does not pass Mid's filter, so importing Mid must not reach it")
	}
}

// An expose is an import (SysML v2 8.3.26.2), so its filter clause restricts
// what the view body sees.
func TestFilteredExposeSurfacesASubset(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": filterLib,
		"app.sysml": "package App { view v { expose Lib::*[@Safety]; } }",
	})
	view := scopeOf(t, scopeOf(t, idx.DocumentRoot("app.sysml"), "App"), "v")

	if _, ok := judgingResolver(idx).ResolveName(view, "SafeBelt", ident("SafeBelt")); !ok {
		t.Error("the filtered expose admits SafeBelt, which must be visible in the view body")
	}
	if _, ok := judgingResolver(idx).ResolveName(view, "Radio", ident("Radio")); ok {
		t.Error("the filtered expose rejects Radio, which must not be visible in the view body")
	}
}

// Without a semantic model there is nothing to judge a condition with, and an
// unevaluable filter keeps the element rather than silently hiding it.
func TestFilterWithoutAModelKeepsEveryElement(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": filterLib,
		"app.sysml": "package App { public import Lib::*[@Safety]; }",
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")

	if _, ok := New(idx).ResolveName(app, "Radio", ident("Radio")); !ok {
		t.Error("with no model attached the filter cannot be judged, so Radio must stay visible")
	}
}
