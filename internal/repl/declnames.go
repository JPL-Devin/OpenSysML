package repl

import "github.com/Open-MBEE/Systemica/internal/core/ast"

// declaredNames returns the replaceable top-level names introduced by a parsed
// submission, in source order. Only Package/Namespace/Alias declarations carry a
// name that the redefine-replaces logic acts on; imports/comments/etc. declare
// nothing replaceable.
func declaredNames(root *ast.RootNamespace) []string {
	if root == nil {
		return nil
	}
	var out []string
	for _, m := range root.Members {
		node := m
		if mem, ok := m.(*ast.Membership); ok {
			node = mem.Member
		}
		var name string
		switch d := node.(type) {
		case *ast.Package:
			name = d.Ident.Name
		case *ast.Namespace:
			name = d.Ident.Name
		case *ast.Alias:
			name = d.Ident.Name
		}
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}
