package repl

import "github.com/Open-MBEE/Systemica/internal/core/ast"

// declaredNames returns the replaceable top-level names introduced by a parsed
// submission, in source order. A named declaration is replaceable: re-typing it
// supersedes the earlier snippet and invalidates anything debugging it.
// Imports/comments/etc. declare nothing replaceable.
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
		case *ast.Definition:
			name = d.Ident.Name
		case *ast.Usage:
			// A usage that declares no name of its own answers to its
			// reference's or redefinition's, which is the name re-typing it
			// supersedes (ast.EffectiveName).
			name, _ = ast.EffectiveName(d)
		}
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}
