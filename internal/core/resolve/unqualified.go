package resolve

import "github.com/Open-MBEE/Systemica/internal/core/symbols"

// walkUnqualified searches the scope and its ancestors for a local match,
// then falls back to the document root. Import-aware search is added in Task 8.
func (r *Resolver) walkUnqualified(scope *symbols.Scope, name string) resolution {
	for s := scope; s != nil; s = s.Parent() {
		if sym, ok := s.LookupLocal(name); ok {
			return resolution{sym: sym, ok: true}
		}
	}
	if root := rootOf(scope); root != nil {
		if sym, ok := root.LookupLocal(name); ok {
			return resolution{sym: sym, ok: true}
		}
	}
	return resolution{}
}
