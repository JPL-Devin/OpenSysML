package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Binding is a lowered binding connector with its two endpoint expressions and
// the scope in which those expressions were declared.
type Binding struct {
	Ends  [2]BindingEnd
	Scope *symbols.Scope
	Decl  *ast.Usage
}

// BindingEnd is one binding endpoint. Path is the runtime lvalue path; Expr
// retains the lossless expression for diagnostics and calc evaluation.
type BindingEnd struct {
	Path string
	Expr ast.Node
}

// ToBindings lowers binding connectors directly declared by a type or usage.
// Namespace-owned bindings are intentionally left to callers to exclude.
func ToBindings(decl ast.Node, scope *symbols.Scope) []Binding {
	var members []ast.Node
	switch n := decl.(type) {
	case *ast.Usage:
		members = n.Members
	case *ast.Definition:
		members = n.Members
	default:
		return nil
	}

	var out []Binding
	for _, member := range members {
		u, ok := unwrapMembership(member).(*ast.Usage)
		if !ok || u.Kind != ast.UsageBinding {
			continue
		}
		binding, ok := lowerBinding(u, scope)
		if ok {
			out = append(out, binding)
		}
	}
	return out
}

func lowerBinding(u *ast.Usage, scope *symbols.Scope) (Binding, bool) {
	if u == nil {
		return Binding{}, false
	}

	var first ast.Node
	for _, rel := range u.Relationships {
		if rel != nil && rel.Kind == ast.RelReferences {
			first = rel.Target
			break
		}
	}
	if first == nil {
		return Binding{}, false
	}
	if u.Value == nil {
		return Binding{}, false
	}
	ends := [2]BindingEnd{
		{Path: FeaturePath(first), Expr: first},
		{Path: FeaturePath(u.Value), Expr: u.Value},
	}
	return Binding{Ends: ends, Scope: scope, Decl: u}, true
}
