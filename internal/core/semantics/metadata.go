package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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
// Any other combination contributes nothing. The second result is false when an
// annotation named a type that could not be resolved yet, so the answer is
// provisional and must not be memoized.
func (m *Model) semanticMetadataBases(sym *symbols.Symbol) ([]*symbols.Symbol, bool) {
	switch sym.Decl.(type) {
	case *ast.Definition, *ast.Usage:
	default:
		return nil, true
	}
	isDef := !isFeature(sym)

	var out []*symbols.Symbol
	complete := true
	for _, a := range MetadataAnnotationsOf(sym.Decl) {
		p := a.Node
		if p.Type == nil {
			continue
		}
		def, ok := m.resolveAnnotationType(sym, a)
		if !ok || def == nil {
			complete = false
			continue
		}
		if resolved, aliasOK := m.resolver.ResolveAliasTarget(def); aliasOK {
			def = resolved
		} else {
			continue
		}
		if !m.isSemanticMetadata(def) {
			continue
		}
		base := m.baseTypeOf(def, sym)
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
	return out, complete
}

// MetadataAnnotation is a metadata feature annotating an element, written either
// as a prefix (`#A part p`) or as a member of the element's body (`@A;`).
type MetadataAnnotation struct {
	Node   *ast.PrefixMetadata
	Prefix bool
}

// resolveAnnotationType resolves the metadata type an annotation names. An
// annotation written as a member is inside the annotated element's body, so it
// is looked up there first and in the owning namespace after.
func (m *Model) resolveAnnotationType(sym *symbols.Symbol, a MetadataAnnotation) (*symbols.Symbol, bool) {
	scopes := []*symbols.Scope{sym.OwnerScope}
	if !a.Prefix && sym.Scope != nil {
		scopes = []*symbols.Scope{sym.Scope, sym.OwnerScope}
	}
	for _, scope := range scopes {
		if def, ok := m.resolver.ResolveQualified(scope, a.Node.Type); ok && def != nil {
			return def, true
		}
	}
	return nil, false
}

// MetadataAnnotationsOf returns the metadata features annotating a declaration,
// in declaration order. `@A about x` annotates other elements, so it is not one.
func MetadataAnnotationsOf(decl ast.Node) []MetadataAnnotation {
	var prefixes []*ast.PrefixMetadata
	var members []ast.Node
	switch d := decl.(type) {
	case *ast.Definition:
		prefixes, members = d.Prefixes, d.Members
	case *ast.Usage:
		prefixes, members = d.Prefixes, d.Members
	case *ast.Package:
		prefixes, members = d.Prefixes, d.Members
	case *ast.Namespace:
		prefixes, members = d.Prefixes, d.Members
	default:
		return nil
	}
	var out []MetadataAnnotation
	for _, p := range prefixes {
		if p != nil && len(p.About) == 0 {
			out = append(out, MetadataAnnotation{Node: p, Prefix: true})
		}
	}
	for _, member := range members {
		if mem, ok := member.(*ast.Membership); ok {
			member = mem.Member
		}
		if p, ok := member.(*ast.PrefixMetadata); ok && len(p.About) == 0 {
			out = append(out, MetadataAnnotation{Node: p})
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

// baseTypeOf returns the type bound to def's baseType feature for annotated,
// resolving the meta-cast operand of `:>> baseType = causes meta SysML::Usage`
// (§7.27.3). The binding is model-level evaluated, so a conditional binding is
// decided against the element being annotated. Returns nil when def binds no
// baseType or the binding does not name a type.
func (m *Model) baseTypeOf(def, annotated *symbols.Symbol) *symbols.Symbol {
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
		name := metaCastOperand(m.baseTypeBinding(def, annotated, usage.Value))
		if name == nil {
			continue
		}
		if base, ok := m.resolver.ResolveQualified(def.Scope, name); ok {
			if resolved, aliasOK := m.resolver.ResolveAliasTarget(base); aliasOK {
				return resolved
			}
		}
	}
	return nil
}

// baseTypeBinding reduces a baseType binding to the branch that applies to the
// annotated element: the binding is model-level evaluated (KerML §7.4.9), so
// `if annotatedElement istype S ? SS meta KerML::Type else CC meta KerML::Class`
// names one type per annotation. Returns nil when no branch can be decided.
func (m *Model) baseTypeBinding(def, annotated *symbols.Symbol, value ast.Node) ast.Node {
	op, ok := value.(*ast.OperatorExpr)
	if !ok || op.Operator != ast.OpConditional || len(op.Operands) != 3 {
		return value
	}
	cond, decided := m.evalAnnotatedElementTest(def, annotated, op.Operands[0])
	if !decided {
		return nil
	}
	if cond {
		return m.baseTypeBinding(def, annotated, op.Operands[1])
	}
	return m.baseTypeBinding(def, annotated, op.Operands[2])
}

// evalAnnotatedElementTest evaluates `annotatedElement istype T` (or `hastype`)
// against the metaclass of the annotated element, the value annotatedElement
// holds for this annotation ([KerML, 8.3.4.9]).
func (m *Model) evalAnnotatedElementTest(def, annotated *symbols.Symbol, cond ast.Node) (bool, bool) {
	op, ok := cond.(*ast.OperatorExpr)
	if !ok || len(op.Operands) == 0 {
		return false, false
	}
	if op.Operator != ast.OpIsType && op.Operator != ast.OpHasType {
		return false, false
	}
	if !readsAnnotatedElement(op.Operands[0]) {
		return false, false
	}
	qn := op.TypeRef
	if qn == nil && len(op.Operands) > 1 {
		if fr, isRef := op.Operands[1].(*ast.FeatureReference); isRef {
			qn = fr.Name
		}
	}
	target, resolved := m.resolveExprTarget(def.Scope, qn)
	meta := m.metaclassOf(annotated)
	if !resolved || meta == nil {
		return false, false
	}
	if op.Operator == ast.OpHasType {
		return meta == target, true
	}
	return m.Conforms(meta, target), true
}

// readsAnnotatedElement reports whether expr reads the annotatedElement feature.
func readsAnnotatedElement(expr ast.Node) bool {
	fr, ok := expr.(*ast.FeatureReference)
	if !ok || fr.Name == nil || len(fr.Name.Parts) == 0 {
		return false
	}
	return fr.Name.Parts[len(fr.Name.Parts)-1].Text == annotatedElementName
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

// isFeature reports whether sym declares a feature rather than a type. A KerML
// type is recorded as a usage node, so the symbol kind decides (KerML §8.3).
func isFeature(sym *symbols.Symbol) bool {
	if _, ok := sym.Decl.(*ast.Usage); !ok {
		return false
	}
	return !isTypeDecl(sym)
}

// isTypeDecl reports whether sym declares a type rather than a feature: a SysML
// definition, or a KerML classifier the parser records as a usage (`datatype`).
func isTypeDecl(sym *symbols.Symbol) bool {
	return sym != nil && sym.Kind.IsDefinition()
}
