package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// AnnotatedElementViolation names the metaclass of annotated that the metadata
// type of the annotation may not be applied to: the metaclass of an annotated
// element conforms to every type of the annotatedElement feature of the
// metadata type (KerML 1.0 §8.3.4.9, validateMetadataFeatureAnnotatedElement).
func (m *Model) AnnotatedElementViolation(annotated *symbols.Symbol, scope *symbols.Scope, prefix *ast.PrefixMetadata) (string, bool) {
	if m == nil || annotated == nil || prefix == nil || prefix.Type == nil {
		return "", false
	}
	def, ok := m.resolver.ResolveQualified(scope, prefix.Type)
	if !ok || def == nil {
		return "", false
	}
	if resolved, aliasOK := m.resolver.ResolveAliasTarget(def); aliasOK {
		def = resolved
	}
	required := m.annotatedElementTypes(def)
	if len(required) == 0 {
		return "", false
	}
	meta := m.metaclassOf(annotated)
	if meta == nil {
		return "", false
	}
	for _, r := range required {
		if !m.Conforms(meta, r) {
			return leafName(meta.Name), true
		}
	}
	return "", false
}

// annotatedElementTypes are the types restricting what a metadata type may
// annotate: the typings of the annotatedElement feature it restates.
func (m *Model) annotatedElementTypes(def *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, member := range m.MembersOfIncludingRedefined(def) {
		if !m.restatesAnnotatedElement(member) {
			continue
		}
		if t := m.featureType(member); t != nil {
			out = append(out, t)
		}
	}
	return out
}

// restatesAnnotatedElement reports whether a member is the annotatedElement
// feature, declared under that name or redefining it.
func (m *Model) restatesAnnotatedElement(member *symbols.Symbol) bool {
	if member == nil {
		return false
	}
	if leafName(member.Name) == annotatedElementName {
		return true
	}
	for _, redefined := range m.RedefinedFeatures(member) {
		if redefined != nil && leafName(redefined.Name) == annotatedElementName {
			return true
		}
	}
	return false
}

const annotatedElementName = "annotatedElement"
