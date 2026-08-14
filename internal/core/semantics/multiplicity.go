package semantics

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Bound is one end of a multiplicity range. Known is false when the bound
// expression is not model-level-evaluable (checks then skip it). Infinite marks
// the `*` unbounded upper.
type Bound struct {
	Value    int64
	Infinite bool
	Known    bool
}

// Range is an extracted multiplicity [lower..upper]. For the single-bound form
// `[n]`, Lower and Upper are both n, except for `[*]`, whose lower bound is 0
// (KerML 1.0 §8.2.5.11, multiplicity textual notation).
type Range struct {
	Lower Bound
	Upper Bound
}

// boundOf evaluates a multiplicity bound expression. `*` (LiteralInfinity)
// becomes an infinite bound; an evaluable integer becomes a known bound;
// anything else is unknown.
func (m *Model) boundOf(n ast.Node) Bound {
	if n == nil {
		return Bound{}
	}
	if _, isInf := n.(*ast.LiteralInfinity); isInf {
		return Bound{Infinite: true, Known: true}
	}
	v, ok := m.Eval(n)
	if !ok {
		return Bound{}
	}
	switch v.Kind {
	case ValInt:
		return Bound{Value: v.Int, Known: true}
	case ValInfinity:
		return Bound{Infinite: true, Known: true}
	default:
		return Bound{}
	}
}

// multiplicityRange extracts a Range from a parsed *ast.Multiplicity. ok is
// false when the multiplicity is nil.
func (m *Model) multiplicityRange(mult *ast.Multiplicity) (Range, bool) {
	if mult == nil {
		return Range{}, false
	}
	if mult.IsRange {
		return Range{Lower: m.boundOf(mult.Lower), Upper: m.boundOf(mult.Upper)}, true
	}
	// Single-bound `[n]`: the parser stores the sole bound in Lower. The bound is
	// both bounds, unless it is unbounded, where the lower bound is 0.
	b := m.boundOf(mult.Lower)
	if b.Infinite {
		return Range{Lower: Bound{Value: 0, Known: true}, Upper: b}, true
	}
	return Range{Lower: b, Upper: b}, true
}

// RangeOf extracts the multiplicity range declared on a usage node, or ok=false
// when it declares none.
func (m *Model) RangeOf(mult *ast.Multiplicity) (Range, bool) {
	return m.multiplicityRange(mult)
}

// MultiplicityOf returns the extracted multiplicity range of a usage symbol, or
// ok=false when the symbol is not a usage or declares no multiplicity.
func (m *Model) MultiplicityOf(sym *symbols.Symbol) (Range, bool) {
	if sym == nil {
		return Range{}, false
	}
	u, isUsage := sym.Decl.(*ast.Usage)
	if !isUsage || u.Multiplicity == nil {
		return Range{}, false
	}
	return m.multiplicityRange(u.Multiplicity)
}

// LowerLeUpper reports whether a range's lower bound does not exceed its upper
// bound. It returns ok=false when either bound is unknown (not evaluable), so
// callers can skip the check. An infinite upper always satisfies the ordering;
// an infinite lower is only valid when the upper is also infinite.
func (r Range) LowerLeUpper() (valid bool, ok bool) {
	if !r.Lower.Known || !r.Upper.Known {
		return false, false
	}
	if r.Upper.Infinite {
		return true, true
	}
	if r.Lower.Infinite {
		// finite upper with infinite lower is invalid.
		return false, true
	}
	return r.Lower.Value <= r.Upper.Value, true
}
