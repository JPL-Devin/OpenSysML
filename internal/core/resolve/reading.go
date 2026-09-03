package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// readingKey keys a reading of a qualified name in a scope its reader chose,
// which may not be the one the name was written in.
type readingKey struct {
	scope *symbols.Scope
	qn    *ast.QualifiedName
}

// Reading is what a qualified name named when read in a chosen scope, with the
// per-segment records and ambiguity a consumer of ResolveQualified would ask
// the resolver for afterwards: see ReadQualified.
type Reading struct {
	sym       *symbols.Symbol
	ok        bool
	parts     []*symbols.Symbol
	ambiguity int
}

// Symbol returns the element the name reached, or false when it reached none.
func (rd Reading) Symbol() (*symbols.Symbol, bool) {
	return rd.sym, rd.ok
}

// Part returns the element segment i named, as PartSymbol does for a written name.
func (rd Reading) Part(i int) (*symbols.Symbol, bool) {
	if i < 0 || i >= len(rd.parts) || rd.parts[i] == nil {
		return nil, false
	}
	return rd.parts[i], true
}

// Ambiguity returns how many elements the name named when it failed for naming
// several, as Ambiguity does for a written name.
func (rd Reading) Ambiguity() (int, bool) {
	return rd.ambiguity, rd.ambiguity > 0
}

// ReadQualified resolves qn as if it were written in scope, the way an
// expression evaluated in a chosen scope reads its names. A written name is
// read in one scope, so ResolveQualified memoizes by node; an expression may be
// evaluated in several, so a reading is memoized by scope and node, and leaves
// the node's own records untouched.
func (r *Resolver) ReadQualified(scope *symbols.Scope, qn *ast.QualifiedName) Reading {
	if qn == nil {
		return Reading{}
	}
	key := readingKey{scope: scope, qn: qn}
	if rd, done := r.readings[key]; done {
		return rd
	}
	if r.resolving[qn] {
		return Reading{}
	}
	r.resolving[qn] = true
	// The walk records its segments where the written name keeps its own, so
	// those are set aside, read off once it is done, and put back.
	saved := r.saveSegments(qn)
	r.clearSegments(qn)
	var res resolution
	r.aside(func() { res = r.walkQualified(scope, qn, (*refFilter)(nil).hiding(qn)) })
	rd := Reading{sym: res.sym, ok: res.ok, parts: r.parts[qn], ambiguity: r.ambiguities[qn]}
	r.restoreSegments(qn, saved)
	delete(r.resolving, qn)
	if r.inCondition == 0 && r.allVisible == 0 {
		r.readings[key] = rd
	}
	return rd
}
