package resolve

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Element filters (KerML 8.2.4, SysML v2 7.4.4) restrict which imported
// elements a namespace surfaces, never its declared ones. They are applied
// where a lookup enumerates the candidates an import surfaced, since only the
// semantic model can judge a condition (docs/SPEC_COMPLIANCE.md).

// importAdmits returns the test an element an import surfaces has to pass: the
// import's own filter clause (`import P::*[@Safety]`) and the `filter` members
// of the namespace declaring the import, which restrict every membership it
// imports — including a `filter` beside the `expose` lines of a view.
func (r *Resolver) importAdmits(scope *symbols.Scope, imp *ast.Import) func(*symbols.Symbol) bool {
	filters := r.namespaceFilters(scope)
	if imp.FilterExpr != nil {
		filters = append(append([]symbols.ElementFilter{}, filters...), symbols.ElementFilter{
			Expr:  imp.FilterExpr,
			Scope: scope,
			Span:  imp.FilterExpr.Span(),
		})
	}
	if len(filters) == 0 {
		return func(*symbols.Symbol) bool { return true }
	}
	return func(sym *symbols.Symbol) bool { return r.admits(filters, sym) }
}

// namespaceFilters are the conditions the namespace owning scope declares,
// extracted once per scope because every import lookup consults them.
func (r *Resolver) namespaceFilters(scope *symbols.Scope) []symbols.ElementFilter {
	if scope == nil {
		return nil
	}
	if filters, ok := r.nsFilters[scope]; ok {
		return filters
	}
	filters := symbols.NamespaceFiltersIn(scope)
	r.nsFilters[scope] = filters
	return filters
}

// documentOf names the document scope belongs to, which decides the routes to a
// root-level name: each document's root namespace is its own.
func (r *Resolver) documentOf(scope *symbols.Scope) string {
	if r.idx == nil {
		return ""
	}
	return r.idx.DocumentOfRoot(rootOf(scope))
}

// admitsUnderName reports whether cand is reachable under the name fqn, given
// the conditions the index recorded for each route re-exporting it
// (symbols.Index.ReexportGates). One route's conditions all have to hold; any
// admitting route admits the name.
func (r *Resolver) admitsUnderName(doc, fqn string, cand *symbols.Symbol) bool {
	if r.idx == nil {
		return true
	}
	routes := r.idx.ReexportGates(doc, fqn, cand)
	if len(routes) == 0 {
		return true
	}
	for _, route := range routes {
		if r.admits(route, cand) {
			return true
		}
	}
	return false
}

// admittedUnder keeps the candidates a lookup of the name fqn may reach, i.e.
// the ones the conditions gating that name select (see admitsUnderName).
func (r *Resolver) admittedUnder(doc, fqn string, cands []*symbols.Symbol) []*symbols.Symbol {
	kept := cands
	for i, sym := range cands {
		if r.admitsUnderName(doc, fqn, sym) {
			continue
		}
		// Copy on first rejection, so an unfiltered lookup keeps its slice.
		kept = make([]*symbols.Symbol, 0, len(cands))
		kept = append(kept, cands[:i]...)
		for _, s := range cands[i+1:] {
			if r.admitsUnderName(doc, fqn, s) {
				kept = append(kept, s)
			}
		}
		return kept
	}
	return kept
}

// admits reports whether cand satisfies every one of filters. A condition the
// model cannot judge keeps the candidate, so an unevaluable filter never
// silently hides model content; the filter pass reports it.
func (r *Resolver) admits(filters []symbols.ElementFilter, cand *symbols.Symbol) bool {
	if len(filters) == 0 || cand == nil {
		return true
	}
	// A condition's own names are resolved unfiltered, or the condition would
	// filter itself: judging a candidate compiles the condition, which cannot
	// resolve a name whose resolution is what asked.
	if r.inCondition > 0 {
		return true
	}
	judge, ok := r.model.(elementFilterJudge)
	if !ok {
		return true
	}
	admitted := true
	// Evaluating a condition reaches declarations of other documents, which this
	// document does not report on.
	r.aside(func() {
		for _, f := range filters {
			if f.IsZero() {
				continue
			}
			if !judge.SatisfiesElementFilter(f, cand) {
				admitted = false
				return
			}
		}
	})
	return admitted
}
