package semantics

import (
	"errors"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// A view's exposed elements (SysML v2 8.3.26 Expose, 7.24 Views and Viewpoints)
// are what its Expose relationships import, enumerated through the same import
// admission name resolution makes — element filters included.

// ErrNotAView is returned when the exposed set is asked of an element that is no
// view; only a view usage or a view definition exposes elements.
var ErrNotAView = errors.New("not a view")

// ExposedElements returns what view exposes, in declaration order and once each:
// its own `expose` relationships followed by those of the views it specializes,
// since an Expose is protected. An empty set is no error, a non-view is
// ErrNotAView, and a nested view's own set is asked of it directly.
func (m *Model) ExposedElements(view *symbols.Symbol) ([]*symbols.Symbol, error) {
	if view == nil || !IsView(view) {
		return nil, ErrNotAView
	}
	out := &exposedSet{seen: map[*symbols.Symbol]bool{}}
	m.addExposed(view, out)
	for _, super := range m.AllSupertypes(view) {
		if IsView(super) {
			m.addExposed(super, out)
		}
	}
	return out.elems, nil
}

// NestedViews returns the views declared in view's body, in declaration order,
// so a caller can walk a view tree.
func (m *Model) NestedViews(view *symbols.Symbol) ([]*symbols.Symbol, error) {
	if view == nil || !IsView(view) {
		return nil, ErrNotAView
	}
	if view.Scope == nil {
		return nil, nil
	}
	var out []*symbols.Symbol
	for _, member := range view.Scope.Members() {
		if member != view && IsView(member) {
			out = append(out, member)
		}
	}
	return out, nil
}

// addExposed adds what the `expose` relationships of one view import.
func (m *Model) addExposed(view *symbols.Symbol, out *exposedSet) {
	for _, imp := range exposesIn(view.Decl) {
		for _, elem := range m.resolver.ImportedElements(view.Scope, imp) {
			out.add(elem)
		}
	}
}

// IsView reports whether sym is a view usage or a view definition, the two
// elements that own Expose relationships.
func IsView(sym *symbols.Symbol) bool {
	switch sym.Kind {
	case symbols.SymbolViewUsage, symbols.SymbolViewDef:
		return true
	}
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		return decl.Kind == ast.DefView
	case *ast.Usage:
		return decl.Kind == ast.UsageView
	}
	return false
}

// exposesIn returns the `expose` relationships declared directly in a view's
// body; a plain `import` there exposes nothing.
func exposesIn(decl ast.Node) []*ast.Import {
	var members []ast.Node
	switch n := decl.(type) {
	case *ast.Definition:
		members = n.Members
	case *ast.Usage:
		members = n.Members
	default:
		return nil
	}
	var out []*ast.Import
	for _, member := range members {
		if imp, ok := member.(*ast.Import); ok && imp.IsExpose {
			out = append(out, imp)
		}
	}
	return out
}

// exposedSet collects exposed elements in the order they were reached, once each.
type exposedSet struct {
	elems []*symbols.Symbol
	seen  map[*symbols.Symbol]bool
}

func (s *exposedSet) add(sym *symbols.Symbol) {
	if sym == nil || s.seen[sym] {
		return
	}
	s.seen[sym] = true
	s.elems = append(s.elems, sym)
}
