package symbols

import "github.com/Open-MBEE/Systemica/internal/core/ast"

// Build constructs the immutable scope tree for a parsed document.
func Build(root *ast.RootNamespace) *Scope {
	rootScope := NewScope(nil, nil)
	if root == nil {
		return rootScope
	}
	buildMembers(rootScope, root.Members)
	return rootScope
}

// buildMembers processes a member list into the given scope.
func buildMembers(scope *Scope, members []ast.Node) {
	for _, m := range members {
		decl, vis := unwrapMember(m)
		if decl == nil {
			continue
		}
		buildDecl(scope, decl, vis)
	}
}

// unwrapMember returns the underlying declaration node and its visibility.
// Membership wrappers carry visibility; directly-listed Import/Alias nodes
// carry their own.
func unwrapMember(m ast.Node) (ast.Node, ast.Visibility) {
	switch v := m.(type) {
	case *ast.Membership:
		return v.Member, v.Visibility
	case *ast.Import:
		return v, v.Visibility
	case *ast.Alias:
		return v, v.Visibility
	default:
		return m, ast.VisibilityDefault
	}
}

// buildDecl registers a symbol (and child scope, where applicable) for a single
// declaration node.
func buildDecl(scope *Scope, decl ast.Node, vis ast.Visibility) {
	switch d := decl.(type) {
	case *ast.Package:
		child := NewScope(scope, d)
		sym := newSymbol(d.Ident, SymbolPackage, d, vis, child)
		defineIdent(scope, d.Ident, sym)
		scope.AddChild(child)
		buildMembers(child, d.Members)
	case *ast.Namespace:
		child := NewScope(scope, d)
		sym := newSymbol(d.Ident, SymbolNamespace, d, vis, child)
		defineIdent(scope, d.Ident, sym)
		scope.AddChild(child)
		buildMembers(child, d.Members)
	case *ast.Alias:
		sym := newSymbol(d.Ident, SymbolAlias, d, vis, scope)
		defineIdent(scope, d.Ident, sym)
	case *ast.Dependency:
		sym := newSymbol(d.Ident, SymbolDependency, d, vis, scope)
		defineIdent(scope, d.Ident, sym)
	case *ast.Comment:
		sym := newSymbol(d.Ident, SymbolComment, d, vis, scope)
		defineIdent(scope, d.Ident, sym)
	case *ast.Documentation:
		sym := newSymbol(d.Ident, SymbolDocumentation, d, vis, scope)
		defineIdent(scope, d.Ident, sym)
	case *ast.TextualRepresentation:
		sym := newSymbol(d.Ident, SymbolTextualRepresentation, d, vis, scope)
		defineIdent(scope, d.Ident, sym)
	case *ast.Import, *ast.FilterMember, *ast.ErrorNode:
		// Imports are processed during resolution; filters hold expressions;
		// error nodes have no declaration. Nothing to register here.
	}
}

// newSymbol builds a Symbol from an identification.
func newSymbol(id ast.Identification, kind SymbolKind, decl ast.Node, vis ast.Visibility, scope *Scope) *Symbol {
	name := id.Name
	if name == "" {
		name = id.ShortName
	}
	sym := &Symbol{
		Name:       name,
		Kind:       kind,
		Decl:       decl,
		Visibility: vis,
		DeclSpan:   decl.Span(),
		Scope:      scope,
	}
	return sym
}

// defineIdent registers sym under both the short and primary name keys.
func defineIdent(scope *Scope, id ast.Identification, sym *Symbol) {
	scope.Define(id.ShortName, sym)
	scope.Define(id.Name, sym)
}
