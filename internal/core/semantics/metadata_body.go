package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// The body of a metadata annotation restates features of the annotated metadata
// type: each declaration implicitly redefines the feature of that name, so a
// name the type does not offer redefines nothing (KerML 7.4.7, 8.3.3.3).

// MetadataBodyViolations returns the declarations in the body of the annotation
// that redefine no feature of the metadata type it names, at any nesting depth.
// It returns nothing when the type does not resolve: an unresolved reference is
// reported by name resolution, and every name under it would be a false report.
func (m *Model) MetadataBodyViolations(scope *symbols.Scope, prefix *ast.PrefixMetadata) []ast.Node {
	if m == nil || prefix == nil || prefix.Type == nil || len(prefix.Body) == 0 {
		return nil
	}
	def, ok := m.resolver.ResolveQualified(scope, prefix.Type)
	if !ok || def == nil {
		return nil
	}
	if resolved, aliasOK := m.resolver.ResolveAliasTarget(def); aliasOK {
		def = resolved
	}
	var out []ast.Node
	m.collectMetadataBodyViolations(def, prefix.Body, &out)
	return out
}

// MetadataBodyInevaluableValues returns the values written in the body of the
// annotation that are not model-level evaluable, at any nesting depth. A
// metadata feature is a model-level element, so its value must be one the model
// alone decides (KerML 7.4.7, Expression::isModelLevelEvaluable).
func (m *Model) MetadataBodyInevaluableValues(scope *symbols.Scope, prefix *ast.PrefixMetadata) []ast.Node {
	if m == nil || prefix == nil || len(prefix.Body) == 0 {
		return nil
	}
	var out []ast.Node
	m.collectInevaluableValues(valueScope(scope, prefix), prefix.Body, &out)
	return out
}

// valueScope is the scope a body's values resolve in: the one built for it, which
// sees the metadata type's members before the annotated element's, else scope.
func valueScope(scope *symbols.Scope, node ast.Node) *symbols.Scope {
	if scope == nil {
		return nil
	}
	if child := scope.ChildFor(node); child != nil {
		return child
	}
	return scope
}

// collectInevaluableValues walks a metadata annotation body, collecting the
// values of its declarations that the model cannot evaluate.
func (m *Model) collectInevaluableValues(scope *symbols.Scope, body []ast.Node, out *[]ast.Node) {
	for _, node := range body {
		if mem, ok := node.(*ast.Membership); ok {
			node = mem.Member
		}
		usage, ok := node.(*ast.Usage)
		if !ok {
			continue
		}
		if usage.Value != nil && !m.ModelLevelEvaluable(scope, usage.Value) {
			*out = append(*out, usage.Value)
		}
		m.collectInevaluableValues(valueScope(scope, usage), usage.Members, out)
	}
}

// collectMetadataBodyViolations walks one body level against the features owner
// offers, descending into the body of each declaration that does name one.
func (m *Model) collectMetadataBodyViolations(owner *symbols.Symbol, body []ast.Node, out *[]ast.Node) {
	if owner == nil {
		return
	}
	for _, node := range body {
		if mem, ok := node.(*ast.Membership); ok {
			node = mem.Member
		}
		usage, ok := node.(*ast.Usage)
		if !ok {
			continue
		}
		if declaresRedefinitionAST(usage) {
			continue // an explicit redefinition states its own target
		}
		target := symbols.MetadataBodyTarget(m, owner, usage.Ident)
		if target == nil {
			*out = append(*out, usage)
			continue
		}
		m.collectMetadataBodyViolations(target, usage.Members, out)
	}
}

// declaresRedefinitionAST reports whether the declaration carries a
// `redefines`/`:>>` clause.
func declaresRedefinitionAST(usage *ast.Usage) bool {
	for _, rel := range usage.Relationships {
		if rel != nil && rel.Kind == ast.RelRedefines {
			return true
		}
	}
	return false
}
