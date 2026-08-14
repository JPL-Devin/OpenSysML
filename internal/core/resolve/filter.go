package resolve

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Element filters (KerML 8.2.4, SysML v2 7.4.4) restrict which elements a
// namespace's imports bring in: the `filter` members of the importing namespace,
// and the `[...]` clause of one import. Both are conditions over one candidate
// element, and both are applied here, where a lookup enumerates the candidates
// an import surfaced — rather than in the index, which records the conditions a
// re-export is subject to but cannot judge them: a condition classifies a
// candidate by the metadata annotating it, which only the semantic model knows.
//
// A namespace's own declared members are never filtered: a filter restricts
// imported memberships, not declared ones.
//
// A namespace's `filter` members restrict what it re-exports, which is what an
// outside lookup reaches (admitsUnderName), not what resolves inside its own
// body. Gating the body too would make a filter unable to name a metadata type
// the namespace itself imports — the condition's own names would be filtered by
// the condition.

// importAdmits returns the test an element an import surfaces has to pass: the
// import's own filter clause (`import P::*[@Safety]`).
func (r *Resolver) importAdmits(scope *symbols.Scope, imp *ast.Import) func(*symbols.Symbol) bool {
	if imp.FilterExpr == nil {
		return func(*symbols.Symbol) bool { return true }
	}
	filters := []symbols.ElementFilter{{
		Expr:  imp.FilterExpr,
		Scope: scope,
		Span:  imp.FilterExpr.Span(),
	}}
	return func(sym *symbols.Symbol) bool { return r.admits(filters, sym) }
}

// admitsUnderName reports whether cand is reachable under the name fqn: a name a
// wildcard import surfaced is subject to the conditions restricting that import
// and the namespace it imported into, which the index recorded when it
// re-exported the name (symbols.Index.ReexportGates).
//
// This is the route an outside lookup takes into a filtered namespace: what
// `import 'Safety Features'::*` re-exports onward, and what
// `'Safety Features'::seatBelt` names, are both what that namespace's filters
// select.
//
// A name several imports surfaced is admitted when any one of those routes
// admits it: an unfiltered import re-exports an element whatever a filtered
// import of the same namespace rejects.
func (r *Resolver) admitsUnderName(fqn string, cand *symbols.Symbol) bool {
	if r.idx == nil {
		return true
	}
	routes := r.idx.ReexportGates(fqn, cand)
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
func (r *Resolver) admittedUnder(fqn string, cands []*symbols.Symbol) []*symbols.Symbol {
	kept := cands
	for i, sym := range cands {
		if r.admitsUnderName(fqn, sym) {
			continue
		}
		// Copy on first rejection, so an unfiltered lookup keeps its slice.
		kept = make([]*symbols.Symbol, 0, len(cands))
		kept = append(kept, cands[:i]...)
		for _, s := range cands[i+1:] {
			if r.admitsUnderName(fqn, s) {
				kept = append(kept, s)
			}
		}
		return kept
	}
	return kept
}

// admits reports whether cand satisfies every one of filters. A condition the
// model cannot judge — because no model is attached, or because the condition is
// outside the subset the evaluator implements — keeps the candidate, so that an
// unevaluable filter never silently hides model content; the corresponding
// diagnostic is reported by the filter pass.
func (r *Resolver) admits(filters []symbols.ElementFilter, cand *symbols.Symbol) bool {
	if len(filters) == 0 || cand == nil {
		return true
	}
	judge, ok := r.model.(elementFilterJudge)
	if !ok {
		return true
	}
	for _, f := range filters {
		if f.IsZero() {
			continue
		}
		if !judge.SatisfiesElementFilter(f, cand) {
			return false
		}
	}
	return true
}
