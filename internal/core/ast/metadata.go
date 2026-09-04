package ast

// DeclaredMetadata returns the prefix metadata a declaration is written with and
// the body whose metadata members annotate it (KerML 8.2.4 PrefixMetadataMember;
// SysML.xtext UsageExtensionKeyword on SubjectUsage and RequirementConstraintUsage).
// A document root's members annotate the root namespace. ok is false for a node
// that carries neither.
func DeclaredMetadata(node Node) (prefixes []*PrefixMetadata, body []Node, ok bool) {
	switch d := node.(type) {
	case *RootNamespace:
		return nil, d.Members, true
	case *Definition:
		return d.Prefixes, d.Members, true
	case *Usage:
		return d.Prefixes, d.Members, true
	case *Package:
		return d.Prefixes, d.Members, true
	case *Namespace:
		return d.Prefixes, d.Members, true
	case *SubjectMember:
		return d.Prefixes, d.Body, true
	case *AssumeMember:
		return d.Prefixes, d.Body, true
	case *RequireMember:
		return d.Prefixes, d.Body, true
	default:
		return nil, nil, false
	}
}
