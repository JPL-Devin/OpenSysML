package passes

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// ConstraintPass runs the depth-C semantic constraint checks over a document's
// symbol tree: specialization-cycle detection, multiplicity-range validity, and
// subsetting multiplicity conformance (design §4.1). It runs at LevelConstraint,
// after type checking; it relies on the shared semantic model for the resolved
// specialization graph and extracted multiplicities.
type ConstraintPass struct{}

func (ConstraintPass) Level() PassLevel { return LevelConstraint }

func (ConstraintPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	cc := &constraintChecker{
		model:    ctx.Model(),
		resolver: ctx.Resolver(),
		seen:     make(map[*symbols.Symbol]bool),
	}
	cc.walk(rootScope)
	return append(cc.diags, checkExposeOwners(root, root.Members)...)
}

type constraintChecker struct {
	model    *semantics.Model
	resolver *resolve.Resolver
	seen     map[*symbols.Symbol]bool
	diags    []Diagnostic
}

// walk visits every symbol in the scope subtree, deduping by pointer (a decl
// with short+primary names registers the same *Symbol under two keys), and
// recurses into each symbol's owned child scope.
func (cc *constraintChecker) walk(scope *symbols.Scope) {
	if scope == nil {
		return
	}
	// Walk named members
	for _, childName := range scope.MemberNames() {
		for _, sym := range scope.LookupLocalAll(childName) {
			if sym == nil || cc.seen[sym] {
				continue
			}
			cc.seen[sym] = true
			cc.check(sym)
			cc.walk(sym.Scope)
		}
	}
	// Walk anonymous members
	for _, sym := range scope.AllMembers() {
		if sym == nil || cc.seen[sym] {
			continue
		}
		if sym.Name == "" {
		}
		// AllMembers includes named + anonymous, so dedup via seen map
		cc.seen[sym] = true
		cc.check(sym)
		cc.walk(sym.Scope)
	}
}

// check runs the per-symbol constraint rules.
func (cc *constraintChecker) check(sym *symbols.Symbol) {
	cc.checkSpecializationCycle(sym)
	cc.checkMultiplicityRange(sym)
	cc.checkSubsettingMultiplicity(sym)
	cc.checkConnectorEnds(sym)
	cc.checkConnectorEndRedefinition(sym)
	cc.checkRedefinition(sym)
	cc.checkUnnamedRedefinitionValue(sym)
	cc.checkVariantOutsideVariation(sym)
}

// checkVariantOutsideVariation warns about a `variant` whose owner is not a
// variation: it offers no choice to anything (SysML v2 §7.20 VariantMembership),
// so it is an ordinary member spelled as if it were selectable.
func (cc *constraintChecker) checkVariantOutsideVariation(sym *symbols.Symbol) {
	if !semantics.DeclaresVariant(sym) || semantics.VariationOwning(sym) != nil {
		return
	}
	owner := "a namespace"
	if sym.OwnerScope != nil {
		if ownerSym := sym.OwnerScope.Owner(); ownerSym != nil && ownerSym.Name != "" {
			owner = ownerSym.Name
		}
	}
	cc.diags = append(cc.diags, Diagnostic{
		Severity: SeverityWarning,
		Span:     sym.Decl.Span(),
		Message: fmt.Sprintf(
			"variant %s is declared in %s, which is not a variation, so it offers no choice; declare its owner `variation` or drop `variant`",
			sym.Name, owner),
		Code:   "variant-outside-variation",
		Source: "constraint",
	})
}

// checkSpecializationCycle flags a symbol that participates in a specialization
// cycle. The diagnostic is anchored at the first generalization edge that leads
// back to sym so the error points at the offending clause.
func (cc *constraintChecker) checkSpecializationCycle(sym *symbols.Symbol) {
	if !cc.model.HasSpecializationCycle(sym) {
		return
	}
	span := sym.DeclSpan
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil || !semantics.GeneralizationKind(rel.Kind) {
			continue
		}
		span = rel.Target.Span()
		break
	}
	cc.diags = append(cc.diags, Diagnostic{
		Severity: SeverityError,
		Span:     span,
		Message:  fmt.Sprintf("%s participates in a specialization cycle", sym.Name),
		Code:     "specialization-cycle",
		Source:   "constraint",
	})
}

// checkMultiplicityRange flags a usage whose evaluable multiplicity has a lower
// bound greater than its upper bound. Non-evaluable bounds are skipped.
func (cc *constraintChecker) checkMultiplicityRange(sym *symbols.Symbol) {
	rng, ok := cc.model.MultiplicityOf(sym)
	if !ok {
		return
	}
	valid, evaluable := rng.LowerLeUpper()
	if !evaluable || valid {
		return
	}
	u, isUsage := sym.Decl.(*ast.Usage)
	span := sym.DeclSpan
	if isUsage && u.Multiplicity != nil {
		span = u.Multiplicity.Span()
	}
	cc.diags = append(cc.diags, Diagnostic{
		Severity: SeverityError,
		Span:     span,
		Message:  fmt.Sprintf("multiplicity lower bound exceeds upper bound on %s", sym.Name),
		Code:     "multiplicity-range",
		Source:   "constraint",
	})
}

// checkSubsettingMultiplicity flags a subsetting usage whose upper bound exceeds
// the upper bound of a usage it subsets (design §4.1): a subset may not admit
// more elements than its superset. Bounds that are not evaluable are skipped.
func (cc *constraintChecker) checkSubsettingMultiplicity(sym *symbols.Symbol) {
	subRange, ok := cc.model.MultiplicityOf(sym)
	if !ok || !subRange.Upper.Known {
		return
	}
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil || rel.Kind != ast.RelSubsets {
			continue
		}
		// Unwrap FeatureReference if needed
		targetNode := rel.Target
		if fr, ok := targetNode.(*ast.FeatureReference); ok {
			targetNode = fr.Name
		}
		qn, isQN := targetNode.(*ast.QualifiedName)
		if !isQN {
			continue
		}
		target, resolved := cc.resolver.ResolveQualified(sym.OwnerScope, qn)
		if !resolved || target == nil {
			continue
		}
		superRange, ok := cc.model.MultiplicityOf(target)
		if !ok || !superRange.Upper.Known {
			continue
		}
		if superRange.Upper.Infinite {
			continue // superset is unbounded: any subset upper conforms
		}
		if subRange.Upper.Infinite || subRange.Upper.Value > superRange.Upper.Value {
			cc.diags = append(cc.diags, Diagnostic{
				Severity: SeverityError,
				Span:     rel.Target.Span(),
				Message: fmt.Sprintf(
					"subsetting %s: upper bound exceeds subsetted %s",
					sym.Name, target.Name),
				Code:   "subsetting-multiplicity",
				Source: "constraint",
			})
		}
	}
}

// checkConnectorEnds validates the declared ends of connector-like usages
// (design §4.3/§4.6), using only the parsed end lists (no stdlib type model):
//
//   - a connection must declare at least two ends (adapts the pilot's
//     INVALID_CONNECTOR_RELATED_FEATURES rule);
//   - an interface or allocation is binary — exactly two ends when any are
//     declared (adapts the binary-specialization rules).
//
// Usages with no declared ends are treated as abstract and skipped. Flow ends
// are not checked: they are optional (SysML v2 §8.2.2.16) and must be absent for
// a message (§8.4.12.2), and a half-declared pair is a parse error.
func (cc *constraintChecker) checkConnectorEnds(sym *symbols.Symbol) {
	u, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return
	}
	switch u.Kind {
	case ast.UsageConnection, ast.UsageInterface, ast.UsageAllocation:
		n := len(u.ConnectorEnds)
		if n == 0 {
			return // no connector clause: abstract connector
		}
		switch u.Kind {
		case ast.UsageConnection:
			if n < 2 {
				cc.addConnectorEndsDiag(sym, u, "a connection must have at least two ends")
			}
		case ast.UsageInterface:
			if n != 2 {
				cc.addConnectorEndsDiag(sym, u, "an interface connection must be binary (exactly two ends)")
			}
		case ast.UsageAllocation:
			if n != 2 {
				cc.addConnectorEndsDiag(sym, u, "an allocation must be binary (exactly two ends)")
			}
		}
	}
}

// checkConnectorEndRedefinition flags an end a connector declares that redefines
// no end of the connector it specializes. Every end of a typed connector
// redefines the end at its own position of that type (SysML v2 §7.13.2), so an
// end beyond the last position of the type refines nothing.
func (cc *constraintChecker) checkConnectorEndRedefinition(sym *symbols.Symbol) {
	general, unmatched := cc.model.UnmatchedConnectorEnds(sym)
	if general == nil {
		return
	}
	declared := cc.model.ConnectorEndCount(general)
	for _, end := range unmatched {
		cc.diags = append(cc.diags, Diagnostic{
			Severity: SeverityError,
			Span:     end.DeclSpan,
			Message: fmt.Sprintf("end %s redefines no end of %s, which declares %d end(s)",
				end.Name, general.Name, declared),
			Code:   "connector-ends",
			Source: "constraint",
		})
	}
}

// addConnectorEndsDiag records a connector-ends diagnostic anchored at the first
// offending end (the third end when there are too many, otherwise the first
// declared end), falling back to the declaration span.
func (cc *constraintChecker) addConnectorEndsDiag(sym *symbols.Symbol, u *ast.Usage, msg string) {
	span := sym.DeclSpan
	switch {
	case len(u.ConnectorEnds) > 2:
		span = u.ConnectorEnds[2].Span()
	case len(u.ConnectorEnds) >= 1:
		span = u.ConnectorEnds[0].Span()
	}
	cc.diags = append(cc.diags, Diagnostic{
		Severity: SeverityError,
		Span:     span,
		Message:  msg,
		Code:     "connector-ends",
		Source:   "constraint",
	})
}

// checkUnnamedRedefinitionValue warns about a value on a member that redefines
// more than one feature: it derives no name (KerML 7.3.4.5), so the value
// reaches none of them.
func (cc *constraintChecker) checkUnnamedRedefinitionValue(sym *symbols.Symbol) {
	u, ok := sym.Decl.(*ast.Usage)
	if !ok || u.Value == nil || u.Ident.Name != "" {
		return
	}
	var targets []string
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		if name, _ := ast.TargetName(rel.Target); name != "" {
			targets = append(targets, name)
		}
	}
	if len(targets) < 2 || ast.NamingFeature(u) != nil {
		return
	}
	binding := "is not reachable by name"
	if u.Ident.ShortName != "" {
		binding = fmt.Sprintf("is bound to the short name <%s> only", u.Ident.ShortName)
	}
	cc.diags = append(cc.diags, Diagnostic{
		Severity: SeverityWarning,
		Span:     u.Value.Span(),
		Message: fmt.Sprintf(
			"a member redefining %s derives no name, so this value %s; declare a name or redefine one feature",
			strings.Join(targets, " and "), binding),
		Code:   "redefinition-no-derived-name",
		Source: "constraint",
	})
}

// checkRedefinition flags a usage that redefines a member without proper
// inheritance, type conformance, or multiplicity bounds. SysML constraints:
// (1) redefined member must be inherited, (2) redefining usage must have a type
// that conforms to the redefined usage's type, (3) multiplicity bounds must be
// compatible (lower >= redefined.lower, upper <= redefined.upper).
func (cc *constraintChecker) checkRedefinition(sym *symbols.Symbol) {
	rels := semantics.RelationshipsOf(sym)
	if len(rels) == 0 {
		return // No relationships
	}

	// Find owner symbol (the definition/usage that declares sym).
	// This traverses OwnerScope.Parent() to find the symbol whose Decl matches
	// sym's declaring scope node. If scope hierarchy is complex (nested blocks,
	// synthetic scopes), owner lookup may fail - we skip validation gracefully.
	var owner *symbols.Symbol
	if sym.OwnerScope != nil {
		ownerNode := sym.OwnerScope.Node()
		if ownerNode != nil {
			// Search parent scope for symbol with matching Decl
			parentScope := sym.OwnerScope.Parent()
			if parentScope != nil {
				for _, name := range parentScope.MemberNames() {
					for _, candidate := range parentScope.LookupLocalAll(name) {
						if candidate != nil && candidate.Decl == ownerNode {
							owner = candidate
							break
						}
					}
					if owner != nil {
						break
					}
				}
			}
		}
	}

	if owner == nil {
		// Cannot determine owner (complex scope hierarchy or internal structure).
		// Skip validation gracefully rather than reporting false positives.
		return
	}

	// Extract all redefines relationships
	for _, rel := range rels {
		if rel == nil {
			continue
		}
		if rel.Kind != ast.RelRedefines {
			continue
		}
		if rel.Target == nil {
			continue
		}

		targetNode := rel.Target
		if fr, ok := targetNode.(*ast.FeatureReference); ok {
			targetNode = fr.Name
		}
		qn, isQN := targetNode.(*ast.QualifiedName)
		if !isQN {
			continue
		}
		redefined, resolved := cc.resolveInheritedMember(owner, qn)
		if !resolved || redefined == nil {
			continue
		}

		// Check that redefined member is inherited (not locally declared in owner)
		inherited := false
		locallyDeclared := false

		// Check if redefined is locally declared in owner's scope
		if owner.Scope != nil {
			for _, memberName := range owner.Scope.MemberNames() {
				for _, localMember := range owner.Scope.LookupLocalAll(memberName) {
					if localMember == redefined {
						locallyDeclared = true
						break
					}
				}
				if locallyDeclared {
					break
				}
			}
		}

		// If not locally declared, check if it's inherited from supertypes
		if !locallyDeclared {
			for _, supertype := range cc.model.AllSupertypes(owner) {
				if supertype.Scope != nil {
					for _, memberName := range supertype.Scope.MemberNames() {
						for _, inheritedMember := range supertype.Scope.LookupLocalAll(memberName) {
							if inheritedMember == redefined {
								inherited = true
								break
							}
						}
						if inherited {
							break
						}
					}
				}
				if inherited {
					break
				}
			}
		}

		if !inherited {
			cc.diags = append(cc.diags, Diagnostic{
				Severity: SeverityError,
				Span:     rel.Target.Span(),
				Message: fmt.Sprintf(
					"%s redefines %s, but %s is not an inherited member of %s",
					sym.Name, redefined.Name, redefined.Name, owner.Name),
				Code:   "redefinition-no-inherited",
				Source: "constraint",
			})
			continue
		}

		// Check type conformance
		usageType := extractUsageType(cc, sym)
		redefinedType := extractUsageType(cc, redefined)

		if usageType != nil && redefinedType != nil {
			if !cc.model.Conforms(usageType, redefinedType) {
				cc.diags = append(cc.diags, Diagnostic{
					Severity: SeverityError,
					Span:     rel.Target.Span(),
					Message: fmt.Sprintf(
						"%s (typed by %s) redefines %s (typed by %s): types do not conform",
						sym.Name, usageType.Name, redefined.Name, redefinedType.Name),
					Code:   "redefinition-type-mismatch",
					Source: "constraint",
				})
			}
		}

		// Check multiplicity bounds (SysML: redefining multiplicity must tighten)
		symMult, symOk := cc.model.MultiplicityOf(sym)
		redefinedMult, redefinedOk := cc.model.MultiplicityOf(redefined)

		// Only validate if both multiplicities are known and evaluable.
		// MultiplicityOf returns ok=false for non-usages or missing multiplicity.
		// Bound.Known=false means expression is not model-level-evaluable.
		// This guards against nil/uninitialized bounds and non-evaluable expressions.
		if symOk && redefinedOk && symMult.Lower.Known && symMult.Upper.Known &&
			redefinedMult.Lower.Known && redefinedMult.Upper.Known {
			// Lower bound must be >= redefined lower bound
			lowerViolated := false
			if !symMult.Lower.Infinite && !redefinedMult.Lower.Infinite {
				lowerViolated = symMult.Lower.Value < redefinedMult.Lower.Value
			}

			// Upper bound must be <= redefined upper bound (or both unbounded)
			upperViolated := false
			if !redefinedMult.Upper.Infinite { // redefined has finite upper bound
				if symMult.Upper.Infinite { // sym is unbounded
					upperViolated = true
				} else if symMult.Upper.Value > redefinedMult.Upper.Value {
					upperViolated = true
				}
			}

			if lowerViolated || upperViolated {
				cc.diags = append(cc.diags, Diagnostic{
					Severity: SeverityError,
					Span:     rel.Target.Span(),
					Message: fmt.Sprintf(
						"%s [%s..%s] redefines %s [%s..%s]: multiplicity bounds incompatible",
						sym.Name, formatBound(symMult.Lower), formatBound(symMult.Upper),
						redefined.Name, formatBound(redefinedMult.Lower), formatBound(redefinedMult.Upper)),
					Code:   "redefinition-multiplicity",
					Source: "constraint",
				})
			}
		}
	}
}

// extractUsageType extracts the type of a usage via its RelTyping relationship.
// Returns nil if no explicit type is found.
func extractUsageType(cc *constraintChecker, sym *symbols.Symbol) *symbols.Symbol {
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil || rel.Kind != ast.RelTyping {
			continue
		}
		targetNode := rel.Target
		if fr, ok := targetNode.(*ast.FeatureReference); ok {
			targetNode = fr.Name
		}
		if qn, ok := targetNode.(*ast.QualifiedName); ok {
			resolved, ok := cc.resolver.ResolveQualified(sym.OwnerScope, qn)
			if ok && resolved != nil {
				return resolved
			}
		}
	}
	return nil
}

// resolveInheritedMember resolves a qualified name from owner's inherited scopes only,
// excluding locally declared members. Used for redefines validation where target
// must be inherited, not local.
func (cc *constraintChecker) resolveInheritedMember(owner *symbols.Symbol, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	// For single-part names, search inherited scopes directly
	if len(qn.Parts) == 1 {
		name := qn.Parts[0].Text
		// Search all supertypes for the member
		for _, supertype := range cc.model.AllSupertypes(owner) {
			if supertype.Scope != nil {
				if members := supertype.Scope.LookupLocalAll(name); len(members) > 0 {
					return members[0], true
				}
			}
		}
		return nil, false
	}

	// For multi-part names, resolve normally (qualifiers won't be local members)
	return cc.resolver.ResolveQualified(owner.Scope, qn)
}

// formatBound formats a Bound for display (infinite = "*", else numeric value).
func formatBound(b semantics.Bound) string {
	if b.Infinite {
		return "*"
	}
	if b.Known {
		return fmt.Sprintf("%d", b.Value)
	}
	return "?"
}
