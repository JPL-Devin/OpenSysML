package resolve

import (
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/quickfix"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/suggest"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// unresolvedFixes returns the edits resolving an unresolved simple name: writing
// it as a ranked candidate, or importing the namespace declaring that name.
func (r *Resolver) unresolvedFixes(scope *symbols.Scope, name string, span source.Span) []quickfix.Fix {
	if span.Len == 0 {
		return nil
	}
	cands := r.suggestFor(scope, name)
	var fixes []quickfix.Fix
	for _, cand := range cands {
		if cand == name {
			continue
		}
		fixes = append(fixes, quickfix.Fix{
			Title:     "Change '" + name + "' to '" + cand + "'",
			Edits:     []quickfix.Edit{quickfix.Replace(span, cand)},
			Preferred: len(cands) == 1,
		})
		if fix, ok := r.importFix(scope, name, cand); ok {
			fixes = append(fixes, fix)
		}
	}
	return fixes
}

// importFix imports the namespace declaring cand, offered only where cand is the
// written name declared elsewhere, so the import alone resolves the reference.
// The import is written private: it serves the namespace importing it without
// re-exporting its names onward ([SysML, 7.2] over [KerML, 8.2.3.3]).
func (r *Resolver) importFix(scope *symbols.Scope, name, cand string) (quickfix.Fix, bool) {
	cut := strings.LastIndex(cand, "::")
	if cut < 0 || suggest.LastSegment(cand) != name || !r.importable(cand) {
		return quickfix.Fix{}, false
	}
	at, ok := importAnchor(scope)
	if !ok {
		return quickfix.Fix{}, false
	}
	stmt := "private import " + cand[:cut] + "::*;"
	return quickfix.Fix{
		Title:     "Import '" + cand[:cut] + "::*'",
		Edits:     []quickfix.Edit{quickfix.InsertLine(at, stmt)},
		Preferred: false,
	}, true
}

// importAnchor is where an import is inserted: before the first member of the
// nearest enclosing namespace, a position an import is a legal member at.
func importAnchor(scope *symbols.Scope) (int, bool) {
	for s := scope; s != nil; s = s.Parent() {
		var members []ast.Node
		switch node := s.Node().(type) {
		case *ast.Package:
			members = node.Members
		case *ast.Namespace:
			members = node.Members
		case *ast.RootNamespace:
			members = node.Members
		case nil:
			// The document root scope carries no node, so the top of the file is
			// where a root-namespace import goes.
			return 0, true
		default:
			continue
		}
		for _, m := range members {
			if sp := m.Span(); sp.Len > 0 {
				return sp.Offset, true
			}
		}
	}
	return 0, false
}
