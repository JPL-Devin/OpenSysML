package semantics

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// semanticMetadataFQN is the KerML metaclass a metadata definition specializes
// to declare that annotating an element implicitly specializes its baseType
// ([KerML, 9.2.16], SysML v2 §7.27.3).
const semanticMetadataFQN = "Metaobjects::SemanticMetadata"

// baseTypeFeature is the SemanticMetadata feature holding the type annotated
// elements implicitly specialize.
const baseTypeFeature = "baseType"

// semanticMetadataBases returns the types sym implicitly specializes because of
// the semantic metadata annotating it. SysML v2 §7.27.4 makes a user-defined
// keyword (`#cause part p`) a metadata annotation, and §7.27.3 gives the
// implicit specialization it contributes:
//   - a usage annotated with a usage baseType subsets that baseType;
//   - a definition annotated with a definition baseType subclassifies it;
//   - a definition annotated with a usage baseType subclassifies the types of
//     that baseType.
//
// Any other combination contributes nothing.
func (m *Model) semanticMetadataBases(sym *symbols.Symbol) []*symbols.Symbol {
	var prefixes []*ast.PrefixMetadata
	isDef := false
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		prefixes, isDef = d.Prefixes, true
	case *ast.Usage:
		prefixes = d.Prefixes
	default:
		return nil
	}

	var out []*symbols.Symbol
	for _, p := range prefixes {
		if p == nil || p.Type == nil {
			continue
		}
		def, ok := m.resolver.ResolveQualified(sym.OwnerScope, p.Type)
		if !ok || def == nil || !m.isSemanticMetadata(def) {
			continue
		}
		base := m.baseTypeOf(def)
		if base == nil {
			continue
		}
		switch {
		case isDef && isFeature(base):
			// A definition cannot subclassify a feature: it subclassifies
			// what the feature is typed by.
			out = append(out, m.DirectSupertypes(base)...)
		case isDef || isFeature(base):
			// Definition with a definition baseType, or usage with a usage
			// baseType. A usage with a definition baseType adds nothing.
			out = append(out, base)
		}
	}
	return out
}

// isSemanticMetadata reports whether def is a direct or indirect specialization
// of the KerML SemanticMetadata metaclass.
func (m *Model) isSemanticMetadata(def *symbols.Symbol) bool {
	if m.resolver.Index() == nil {
		return false
	}
	for _, meta := range m.resolver.Index().LookupQualified(semanticMetadataFQN) {
		if meta == nil {
			continue
		}
		if def == meta {
			return true
		}
		for _, sup := range m.AllSupertypes(def) {
			if sup == meta {
				return true
			}
		}
	}
	return false
}

// baseTypeOf returns the type bound to def's baseType feature, resolving the
// meta-cast operand of `:>> baseType = causes meta SysML::Usage` (§7.27.3).
// Returns nil when def binds no baseType or the binding does not name a type.
func (m *Model) baseTypeOf(def *symbols.Symbol) *symbols.Symbol {
	decl, ok := def.Decl.(*ast.Definition)
	if !ok || def.Scope == nil {
		return nil
	}
	// The binding is an anonymous member, so it is reached through the AST
	// rather than through the (name-keyed) scope.
	for _, member := range decl.Members {
		if mem, ok := member.(*ast.Membership); ok {
			member = mem.Member
		}
		usage, ok := member.(*ast.Usage)
		if !ok || !redefinesBaseType(usage) || usage.Value == nil {
			continue
		}
		name := metaCastOperand(usage.Value)
		if name == nil {
			continue
		}
		if base, ok := m.resolver.ResolveQualified(def.Scope, name); ok {
			return base
		}
	}
	return nil
}

// redefinesBaseType reports whether usage redefines SemanticMetadata::baseType.
func redefinesBaseType(usage *ast.Usage) bool {
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		qn, ok := target.(*ast.QualifiedName)
		if !ok || len(qn.Parts) == 0 {
			continue
		}
		if qn.Parts[len(qn.Parts)-1].Text == baseTypeFeature {
			return true
		}
	}
	return false
}

// metaCastOperand returns the qualified name of the type a baseType binding
// denotes: the left operand of a `meta` cast, or the reference itself.
func metaCastOperand(value ast.Node) *ast.QualifiedName {
	if op, ok := value.(*ast.OperatorExpr); ok && op.Operator == ast.OpMeta && len(op.Operands) > 0 {
		value = op.Operands[0]
	}
	if fr, ok := value.(*ast.FeatureReference); ok {
		return fr.Name
	}
	if qn, ok := value.(*ast.QualifiedName); ok {
		return qn
	}
	return nil
}

// isFeature reports whether sym declares a usage (a KerML feature) rather than
// a definition (a classifier).
func isFeature(sym *symbols.Symbol) bool {
	_, ok := sym.Decl.(*ast.Usage)
	return ok
}
