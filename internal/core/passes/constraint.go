package passes

import (
	"fmt"

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
	return cc.diags
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
}

// check runs the per-symbol constraint rules.
func (cc *constraintChecker) check(sym *symbols.Symbol) {
	cc.checkSpecializationCycle(sym)
	cc.checkMultiplicityRange(sym)
	cc.checkSubsettingMultiplicity(sym)
	cc.checkConnectorEnds(sym)
	cc.checkTypingConformance(sym)
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
//     declared (adapts the binary-specialization rules);
//   - a flow whose ends are declared must have both a source and a target.
//
// Usages with no declared ends are treated as abstract and skipped.
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
	case ast.UsageFlow:
		if u.FlowEnds != nil && (u.FlowEnds.From == nil || u.FlowEnds.To == nil) {
			cc.diags = append(cc.diags, Diagnostic{
				Severity: SeverityError,
				Span:     u.FlowEnds.Span(),
				Message:  fmt.Sprintf("flow %s must declare both a source and a target end", sym.Name),
				Code:     "flow-ends",
				Source:   "constraint",
			})
		}
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

// checkTypingConformance flags a usage whose type does not conform to the type
// of a usage it subsets (SysML spec: a subsetting usage must have a type that
// conforms to the type of the subsetted usage). Uses model.Conforms for type
// conformance checking.
func (cc *constraintChecker) checkTypingConformance(sym *symbols.Symbol) {
	// Extract usage typing (via typing relationship)
	var usageType *symbols.Symbol
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
				usageType = resolved
				break
			}
		}
	}
	
	if usageType == nil {
		return // No explicit type, skip
	}
	
	// Check all subsets relationships
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil || rel.Kind != ast.RelSubsets {
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
		subsetted, resolved := cc.resolver.ResolveQualified(sym.OwnerScope, qn)
		if !resolved || subsetted == nil {
			continue
		}
		
		// Extract type of subsetted usage
		var subsettedType *symbols.Symbol
		for _, subRel := range semantics.RelationshipsOf(subsetted) {
			if subRel == nil || subRel.Target == nil || subRel.Kind != ast.RelTyping {
				continue
			}
			subTargetNode := subRel.Target
			if fr, ok := subTargetNode.(*ast.FeatureReference); ok {
				subTargetNode = fr.Name
			}
			if subQn, ok := subTargetNode.(*ast.QualifiedName); ok {
				subResolved, ok := cc.resolver.ResolveQualified(subsetted.OwnerScope, subQn)
				if ok && subResolved != nil {
					subsettedType = subResolved
					break
				}
			}
		}
		
		if subsettedType == nil {
			continue // Subsetted usage has no explicit type, skip
		}
		
		// Check conformance: usageType must conform to subsettedType
		if !cc.model.Conforms(usageType, subsettedType) {
			cc.diags = append(cc.diags, Diagnostic{
				Severity: SeverityError,
				Span:     rel.Target.Span(),
				Message: fmt.Sprintf(
					"%s (typed by %s) subsets %s (typed by %s): types do not conform",
					sym.Name, usageType.Name, subsetted.Name, subsettedType.Name),
				Code:   "typing-conformance",
				Source: "constraint",
			})
		}
	}
}
