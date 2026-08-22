package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Pilot SysMLValidator (2026-05) checkDefinition/checkUsage, constraints
// validateDefinitionVariationMembership/validateUsageVariationMembership and
// validateDefinitionVariationSpecialization/validateUsageVariationSpecialization.
const (
	msgVariationMemberNotVariant = "An owned usage of a variation must be a variant."
	msgVariationSpecialization   = "A variation must not specialize another variation."
	msgVariantOutsideVariation   = "A variant must be an owned member of a variation."
)

// W8DVariabilityPass checks that every usage a variation owns is a variant and
// that a variation does not specialize another variation (SysML v2 §7.20).
type W8DVariabilityPass struct{}

func (W8DVariabilityPass) Level() PassLevel { return LevelConstraint }

func (W8DVariabilityPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	vc := &w8dVariabilityChecker{resolver: ctx.Resolver()}
	w8dWalkSymbols(rootScope, vc.check)
	return vc.diags
}

type w8dVariabilityChecker struct {
	resolver *resolve.Resolver
	diags    []Diagnostic
}

func (vc *w8dVariabilityChecker) check(sym *symbols.Symbol) {
	if !semantics.DeclaresVariation(sym) {
		return
	}
	vc.checkMembers(sym)
	vc.checkSpecializations(sym)
}

// checkMembers reports every owned usage of a variation that is not a variant;
// only a variation usage's objective is exempt, as in the reference.
func (vc *w8dVariabilityChecker) checkMembers(sym *symbols.Symbol) {
	_, isUsage := sym.Decl.(*ast.Usage)
	for _, member := range declMembers(sym.Decl) {
		node := unwrapType(member)
		if _, ok := node.(*ast.SubjectMember); ok {
			vc.report(member.Span())
			continue
		}
		u, ok := node.(*ast.Usage)
		if !ok || u.IsVariant {
			continue
		}
		if isUsage && u.Kind == ast.UsageObjective {
			continue
		}
		vc.report(member.Span())
	}
}

func (vc *w8dVariabilityChecker) report(span source.Span) {
	vc.diags = append(vc.diags, Diagnostic{
		Severity: SeverityError,
		Span:     span,
		Message:  msgVariationMemberNotVariant,
		Code:     "variation-member-not-variant",
		Source:   "constraint",
	})
}

// checkSpecializations reports a variation whose declared general is itself a
// variation; a typing counts, since FeatureTyping is a Specialization.
func (vc *w8dVariabilityChecker) checkSpecializations(sym *symbols.Symbol) {
	scope := sym.OwnerScope
	if sym.Scope != nil {
		scope = sym.Scope
	}
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil || !w8dSpecializationKind(rel.Kind) {
			continue
		}
		targetNode := rel.Target
		if fr, ok := targetNode.(*ast.FeatureReference); ok {
			targetNode = fr.Name
		}
		qn, ok := targetNode.(*ast.QualifiedName)
		if !ok {
			continue
		}
		general, ok := vc.resolver.ResolveQualified(scope, qn)
		if !ok || general == sym || !semantics.DeclaresVariation(general) {
			continue
		}
		vc.diags = append(vc.diags, Diagnostic{
			Severity: SeverityError,
			Span:     rel.Target.Span(),
			Message:  msgVariationSpecialization,
			Code:     "variation-specialization",
			Source:   "constraint",
		})
	}
}

// w8dSpecializationKind reports whether kind is an owned specialization of the
// declaration: a typing is one (FeatureTyping specializes Specialization).
func w8dSpecializationKind(kind ast.RelationshipKind) bool {
	switch kind {
	case ast.RelTyping, ast.RelSpecializes, ast.RelSubsets, ast.RelRedefines:
		return true
	}
	return false
}
