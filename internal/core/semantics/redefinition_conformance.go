package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Feature conformance (KerML 8.3.3.3): a subsetting feature restricts the
// subsetted one and may not widen it; a redefinition replaces the redefined
// feature in the owning type, so its direction must agree too.

// ConformanceViolationKind names the conformance rule a relationship breaks.
type ConformanceViolationKind int

const (
	// ViolationDirection: a redefining feature's direction differs from the
	// direction the redefined feature has in the owning type.
	ViolationDirection ConformanceViolationKind = iota
	// ViolationUniqueness: a nonunique feature subsets or redefines a unique one.
	ViolationUniqueness
	// ViolationConstancy: a variable feature subsets or redefines a constant one.
	ViolationConstancy
)

// ConformanceViolation is one broken conformance rule: the relationship that
// breaks it, and the reference to report it at.
type ConformanceViolation struct {
	Kind ConformanceViolationKind
	// Feature is the subsetting or redefining feature.
	Feature *symbols.Symbol
	// Target is the subsetted or redefined feature.
	Target *symbols.Symbol
	// Ref is the node the diagnostic is reported at: the target reference of an
	// explicit clause, or the declaration itself for an implicit redefinition.
	Ref ast.Node
}

// ConformanceViolations returns the feature-conformance rules the declaration of
// sym breaks against the features it subsets and redefines, explicitly or as a
// parameter of its owning behavior.
func (m *Model) ConformanceViolations(sym *symbols.Symbol) []ConformanceViolation {
	usage, ok := usageOf(sym)
	if !ok {
		return nil
	}
	var out []ConformanceViolation
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil {
			continue
		}
		if rel.Kind != ast.RelSubsets && rel.Kind != ast.RelRedefines {
			continue
		}
		target := m.conformanceTarget(sym, rel)
		if target == nil || target == sym {
			continue
		}
		ref := ast.Node(rel.Target)
		if rel.Kind == ast.RelRedefines {
			out = append(out, m.directionViolations(sym, usage, target, ref)...)
		}
		out = append(out, m.restrictionViolations(sym, usage, target, ref)...)
	}
	// A parameter redefines the parameter at its position implicitly, and the
	// direction it declares must be the redefined one's (KerML 7.4.7.2).
	for _, target := range m.implicitParameterCounterparts(sym) {
		out = append(out, m.directionViolations(sym, usage, target, sym.Decl)...)
	}
	return out
}

// conformanceTarget resolves the feature a subsetting or redefinition names. A
// redefinition names what the owner inherits, which RedefinedFeatures already
// resolves; a subsetting is resolved like any other reference.
func (m *Model) conformanceTarget(sym *symbols.Symbol, rel *ast.Relationship) *symbols.Symbol {
	if rel.Kind == ast.RelRedefines {
		return m.redefinitionTarget(sym, rel.Target)
	}
	target := rel.Target
	if ref, ok := target.(*ast.FeatureReference); ok {
		target = ref.Name
	}
	found, ok := m.resolver.ResolveTarget(sym.OwnerScope, target)
	if !ok || found == nil {
		return nil
	}
	if resolved, aliasOK := m.resolver.ResolveAliasTarget(found); aliasOK {
		return resolved
	}
	return found
}

// directionViolations reports a redefinition whose declared direction differs
// from the redefined feature's. An undeclared direction takes the redefined
// one's, and a redefined `inout` admits any direction.
func (m *Model) directionViolations(sym *symbols.Symbol, usage *ast.Usage,
	target *symbols.Symbol, ref ast.Node) []ConformanceViolation {
	if usage.Direction == ast.DirNone {
		return nil
	}
	targetDir := m.directionThrough(owningTypeOf(sym), target)
	if targetDir == ast.DirNone || targetDir == ast.DirInOut || targetDir == usage.Direction {
		return nil
	}
	return []ConformanceViolation{{
		Kind: ViolationDirection, Feature: sym, Target: target, Ref: ref,
	}}
}

// restrictionViolations reports the uniqueness and constancy rules: a subsetting
// or redefining feature may not be nonunique where the target is unique, nor
// variable where the target is constant (KerML 8.3.3.3).
func (m *Model) restrictionViolations(sym *symbols.Symbol, usage *ast.Usage,
	target *symbols.Symbol, ref ast.Node) []ConformanceViolation {
	targetUsage, ok := usageOf(target)
	if !ok {
		return nil
	}
	var out []ConformanceViolation
	if usage.IsNonunique && !targetUsage.IsNonunique {
		out = append(out, ConformanceViolation{
			Kind: ViolationUniqueness, Feature: sym, Target: target, Ref: ref,
		})
	}
	if usage.IsVariable && m.isConstantFeature(target, targetUsage) {
		out = append(out, ConformanceViolation{
			Kind: ViolationConstancy, Feature: sym, Target: target, Ref: ref,
		})
	}
	return out
}

// isConstantFeature reports whether target is constant, declared so or through a
// feature it subsets or redefines: constancy is inherited by restriction.
func (m *Model) isConstantFeature(target *symbols.Symbol, usage *ast.Usage) bool {
	if usage.IsConstant {
		return true
	}
	if usage.IsVariable {
		return false
	}
	for _, rel := range RelationshipsOf(target) {
		if rel == nil || rel.Target == nil {
			continue
		}
		if rel.Kind != ast.RelSubsets && rel.Kind != ast.RelRedefines {
			continue
		}
		next := m.conformanceTarget(target, rel)
		if next == nil || next == target {
			continue
		}
		nextUsage, ok := usageOf(next)
		if ok && m.isConstantFeature(next, nextUsage) {
			return true
		}
	}
	return false
}

// directionThrough returns the direction feature has as seen through owner:
// reversed when owner reaches the feature's owning type by a conjugation
// (SysML v2 7.12.2).
func (m *Model) directionThrough(owner, feature *symbols.Symbol) ast.FeatureDirection {
	usage, ok := usageOf(feature)
	if !ok {
		return ast.DirNone
	}
	dir := usage.Direction
	source := owningTypeOf(feature)
	if owner == nil || source == nil {
		return dir
	}
	for _, sup := range m.conjugatedSupertypes(owner) {
		if sup.sym == source && sup.conjugated {
			return ConjugateDirection(dir)
		}
	}
	return dir
}

// owningTypeOf returns the type declaring sym, or nil at the top level.
func owningTypeOf(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	return sym.OwnerScope.Owner()
}

// usageOf returns the usage declaring sym, if sym is declared by one.
func usageOf(sym *symbols.Symbol) (*ast.Usage, bool) {
	if sym == nil {
		return nil, false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	return usage, ok
}
