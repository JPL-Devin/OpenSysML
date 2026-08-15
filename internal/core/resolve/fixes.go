package resolve

import (
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/quickfix"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/suggest"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// unresolvedFixes returns the edits that resolve an unresolved simple name
// written in scope: spelling it as one of the candidates the suggestion machinery
// ranked, and, where a candidate is that very name declared elsewhere, importing
// the namespace declaring it so the name resolves as written.
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

// importFix imports the namespace declaring cand, which resolves the reference
// only where cand is the written name declared elsewhere: a misspelled name
// needs its spelling fixed too, and that is a second correction.
func (r *Resolver) importFix(scope *symbols.Scope, name, cand string) (quickfix.Fix, bool) {
	cut := strings.LastIndex(cand, "::")
	if cut < 0 || suggest.LastSegment(cand) != name || !r.importable(cand) {
		return quickfix.Fix{}, false
	}
	at, ok := importAnchor(scope)
	if !ok {
		return quickfix.Fix{}, false
	}
	stmt := "import " + cand[:cut] + "::*;"
	return quickfix.Fix{
		Title:     "Import '" + cand[:cut] + "::*'",
		Edits:     []quickfix.Edit{quickfix.InsertLine(at, stmt)},
		Preferred: false,
	}, true
}

// importAnchor is the offset an import declaration is inserted at: the start of
// the first member of the nearest enclosing namespace, which is a position an
// import is a legal member of. A namespace with no members at all has no
// reference to resolve.
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
			// The document root scope carries no node, and an import is a member
			// of the root namespace, so the top of the file is where it goes.
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
