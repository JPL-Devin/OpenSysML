package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const (
	msgCrossFeatureType     = "Cross feature must have same type as feature"
	msgCrossSubsettingChain = "Cross subsetting must chain through an opposite end feature"
	msgCrossSpecialization  = "Cross feature must specialized redefined-end cross features"
	// Feature::crossFeature is single-valued (KerML 8.3.4.5), so an end that
	// declares its cross feature inline states no other one with `crosses`.
	msgMustBeCrossFeature = "Must be the cross feature"
)

// checkW10BCrossFeatures implements the KerML crossing constraints on an
// association's end features (KerML §8.3.3.3 validateFeatureCrossFeature*,
// validateCrossSubsettingFeatureChain): a `crosses` chain must start at an
// opposite end, name a feature of that end's type, and specialize the cross
// feature of the end it redefines in a specialized association.
func (cc *constraintChecker) checkW10BCrossFeatures(sym *symbols.Symbol) {
	ends := cc.w10bAssociationEnds(sym)
	if len(ends) == 0 {
		return
	}
	for _, end := range ends {
		rel, chain := w10bCrossChain(end)
		if rel == nil {
			continue
		}
		cc.checkDeclaredCrossFeature(end)
		base, ok := cc.w10bEndNamed(ends, chain.base)
		if !ok || base == end {
			cc.addRedefineDiag(rel.Target, msgCrossSubsettingChain, "cross-subsetting-chain")
		}
		cross, found := cc.w10bCrossFeature(base, chain.member)
		if !found {
			continue
		}
		if !cc.w10bSameTypes(cross, end) {
			cc.addRedefineDiag(rel.Target, msgCrossFeatureType, "cross-feature-type")
		}
		if !cc.w10bSpecializesInheritedCross(sym, end, cross) {
			cc.addRedefineDiag(rel.Target, msgCrossSpecialization, "cross-feature-specialization")
		}
	}
}

// checkDeclaredCrossFeature reports an end that declares its cross feature
// inline and also crosses another one: an end's crossFeature is single-valued
// (KerML 8.3.4.5), and a `crosses` chain names a feature of the opposite end's
// type, never the feature the end owns.
func (cc *constraintChecker) checkDeclaredCrossFeature(end *symbols.Symbol) {
	u, ok := end.Decl.(*ast.Usage)
	if !ok || u.CrossFeature == nil {
		return
	}
	cc.diags = append(cc.diags, Diagnostic{
		Severity: SeverityError,
		Span:     u.CrossFeature.Span(),
		Message:  msgMustBeCrossFeature,
		Code:     "declared-cross-feature",
		Source:   "constraint",
	})
}

type w10bCrossRef struct {
	base   string
	member string
}

// w10bCrossChain returns sym's `crosses` relationship and the two-segment chain
// it names; only a chain rooted at another end can be checked.
func w10bCrossChain(sym *symbols.Symbol) (*ast.Relationship, w10bCrossRef) {
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelCrosses || rel.Target == nil {
			continue
		}
		chain, ok := w10bChainOf(rel.Target)
		if !ok {
			continue
		}
		return rel, chain
	}
	return nil, w10bCrossRef{}
}

func w10bChainOf(target ast.Node) (w10bCrossRef, bool) {
	fce, ok := target.(*ast.FeatureChainExpr)
	if !ok || fce.Member == nil || len(fce.Member.Parts) != 1 {
		return w10bCrossRef{}, false
	}
	member := fce.Member.Parts[0].Text
	switch base := fce.Operand.(type) {
	case *ast.FeatureReference:
		if base.Name != nil && len(base.Name.Parts) == 1 {
			return w10bCrossRef{base: base.Name.Parts[0].Text, member: member}, true
		}
	case *ast.QualifiedName:
		if len(base.Parts) == 1 {
			return w10bCrossRef{base: base.Parts[0].Text, member: member}, true
		}
	}
	return w10bCrossRef{}, false
}

// w10bAssociationEnds returns the end features an association declares, in
// declaration order, or nil when sym is not an association.
func (cc *constraintChecker) w10bAssociationEnds(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || sym.Scope == nil {
		return nil
	}
	u, ok := sym.Decl.(*ast.Usage)
	if !ok || (u.Keyword != "assoc" && u.Keyword != "association") {
		return nil
	}
	var ends []*symbols.Symbol
	for _, name := range sym.Scope.MemberNames() {
		member, ok := sym.Scope.LookupLocal(name)
		if !ok || member == nil || !w10bIsEndFeature(member) {
			continue
		}
		ends = append(ends, member)
	}
	return ends
}

func (cc *constraintChecker) w10bEndNamed(ends []*symbols.Symbol, name string) (*symbols.Symbol, bool) {
	for _, end := range ends {
		if end.Name == name {
			return end, true
		}
	}
	return nil, false
}

// w10bCrossFeature resolves the feature the chain names on the type of its base
// end.
func (cc *constraintChecker) w10bCrossFeature(base *symbols.Symbol, member string) (*symbols.Symbol, bool) {
	if base == nil || member == "" {
		return nil, false
	}
	baseType := extractUsageType(cc, base)
	if baseType == nil {
		return nil, false
	}
	found, ok := cc.model.LookupMember(baseType, member)
	if !ok || found == nil {
		return nil, false
	}
	return found, true
}

// w10bSameTypes reports whether a cross feature and its end declare the same
// type, which is what the crossing requires.
func (cc *constraintChecker) w10bSameTypes(cross, end *symbols.Symbol) bool {
	crossType := extractUsageType(cc, cross)
	endType := extractUsageType(cc, end)
	if crossType == nil || endType == nil {
		return true // nothing declared to compare
	}
	return crossType == endType
}

// w10bSpecializesInheritedCross reports whether end's cross feature specializes
// the cross feature of the same-named end in every association sym specializes.
func (cc *constraintChecker) w10bSpecializesInheritedCross(sym, end, cross *symbols.Symbol) bool {
	for _, general := range cc.model.AllSupertypes(sym) {
		if general == sym {
			continue
		}
		inherited, ok := cc.w10bEndNamed(cc.w10bAssociationEnds(general), end.Name)
		if !ok {
			continue
		}
		rel, chain := w10bCrossChain(inherited)
		if rel == nil {
			continue
		}
		base, ok := cc.w10bEndNamed(cc.w10bAssociationEnds(general), chain.base)
		if !ok {
			continue
		}
		inheritedCross, found := cc.w10bCrossFeature(base, chain.member)
		if !found || inheritedCross == cross {
			continue
		}
		if !cc.model.Conforms(cross, inheritedCross) {
			return false
		}
	}
	return true
}
