package ast

// IsSuccessionSource reports whether a body member is one a positional `then`
// after it sequences from: a feature that is not an edge. A definition, package,
// import, alias, annotation or filter is not a feature, and an edge relates
// other members (UsageKind.IsEdge), so a `then` passes over both to the nearest
// member before them that is one. The parser reads `then` by this rule and the
// RDF writer folds a succession back into `then` by it.
func IsSuccessionSource(member Node) bool {
	switch n := member.(type) {
	case *Membership:
		return n.Member != nil && IsSuccessionSource(n.Member)
	case *Usage:
		return !n.Kind.IsEdge()
	case *SuccessionEdge, *ControlFlowEdge, *ObjectFlowEdge, *TransitionEdge, *TransitionMember:
		return false
	case *Definition, *Package, *Namespace, *Import, *Alias, *Dependency,
		*RelationshipMember, *Comment, *Documentation, *TextualRepresentation, *FilterMember:
		return false
	}
	return true
}
