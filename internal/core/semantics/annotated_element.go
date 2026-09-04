package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// AnnotatedElementViolation names the metaclass of annotated when the metadata
// type typeRef names may not annotate it (validateMetadataFeatureAnnotatedElement).
func (m *Model) AnnotatedElementViolation(annotated *symbols.Symbol, scope *symbols.Scope, typeRef *ast.QualifiedName) (string, bool) {
	if m == nil || annotated == nil {
		return "", false
	}
	return m.annotatedElementViolationOf(annotated, m.resolveMetadataType(scope, typeRef))
}

// AboutAnnotatedElementViolations names, in order, the metaclass of each element
// an `about` clause names that the metadata type typeRef names may not annotate.
func (m *Model) AboutAnnotatedElementViolations(scope *symbols.Scope, typeRef *ast.QualifiedName, about []*ast.QualifiedName) []string {
	if m == nil || len(about) == 0 {
		return nil
	}
	def := m.resolveMetadataType(scope, typeRef)
	if def == nil {
		return nil
	}
	var out []string
	for _, qn := range about {
		if qn == nil {
			continue
		}
		annotated, ok := m.resolver.ResolveQualified(scope, qn)
		if !ok || annotated == nil {
			continue
		}
		if resolved, aliasOK := m.resolver.ResolveAliasTarget(annotated); aliasOK {
			annotated = resolved
		}
		if metaclass, bad := m.annotatedElementViolationOf(annotated, def); bad {
			out = append(out, metaclass)
		}
	}
	return out
}

// resolveMetadataType resolves the type an annotation names, through aliases.
func (m *Model) resolveMetadataType(scope *symbols.Scope, typeRef *ast.QualifiedName) *symbols.Symbol {
	if typeRef == nil {
		return nil
	}
	def, ok := m.resolver.ResolveQualified(scope, typeRef)
	if !ok || def == nil {
		return nil
	}
	if resolved, aliasOK := m.resolver.ResolveAliasTarget(def); aliasOK {
		return resolved
	}
	return def
}

func (m *Model) annotatedElementViolationOf(annotated, def *symbols.Symbol) (string, bool) {
	if def == nil {
		return "", false
	}
	alternatives := m.annotatedElementFeatures(def)
	if len(alternatives) == 0 {
		return "", false
	}
	meta := m.metaclassOf(annotated)
	if meta == nil {
		return "", false
	}
	for _, feature := range alternatives {
		if m.conformsToTypesOf(meta, feature) {
			return "", false
		}
	}
	return leafName(meta.Name), true
}

// annotatedElementFeatures are the features the annotation inherits from def that
// specialize annotatedElement, each an alternative; a concrete one supersedes the abstract ones.
func (m *Model) annotatedElementFeatures(def *symbols.Symbol) []*symbols.Symbol {
	var all, concrete []*symbols.Symbol
	for _, member := range m.MembersOf(def) {
		if !symbols.VisibleAs(member.Visibility, false, true) || !m.specializesAnnotatedElement(member) {
			continue
		}
		all = append(all, member)
		if !symbols.IsAbstract(member) {
			concrete = append(concrete, member)
		}
	}
	if len(concrete) > 0 {
		return concrete
	}
	return all
}

// specializesAnnotatedElement reports whether a feature is the annotatedElement
// feature or specializes it, by redefinition or subsetting at any distance.
func (m *Model) specializesAnnotatedElement(feature *symbols.Symbol) bool {
	if feature == nil || !isFeature(feature) {
		return false
	}
	if leafName(feature.Name) == annotatedElementName {
		return true
	}
	for _, super := range m.AllSupertypes(feature) {
		if super != nil && isFeature(super) && leafName(super.Name) == annotatedElementName {
			return true
		}
	}
	return false
}

// conformsToTypesOf reports whether meta conforms to every type of feature: the
// ones it declares and those of the features it redefines or subsets.
func (m *Model) conformsToTypesOf(meta, feature *symbols.Symbol) bool {
	if !m.conformsToDeclaredTypesOf(meta, feature) {
		return false
	}
	for _, super := range m.AllSupertypes(feature) {
		if isFeature(super) && !m.conformsToDeclaredTypesOf(meta, super) {
			return false
		}
	}
	return true
}

// conformsToDeclaredTypesOf reports whether meta conforms to every type feature
// declares; a feature declaring none restricts nothing.
func (m *Model) conformsToDeclaredTypesOf(meta, feature *symbols.Symbol) bool {
	for _, rel := range RelationshipsOf(feature) {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		qn, ok := target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		typ, ok := m.resolver.ResolveQualified(feature.OwnerScope, qn)
		if !ok || typ == nil {
			continue
		}
		if canonical, aliasOK := m.resolver.ResolveAliasTarget(typ); aliasOK {
			typ = canonical
		}
		if !m.Conforms(meta, typ) {
			return false
		}
	}
	return true
}

const annotatedElementName = "annotatedElement"
