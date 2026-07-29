// Package symbols builds an immutable per-document scope tree and a global
// qualified-name index over a parsed ast.RootNamespace.
package symbols

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// SymbolKind classifies a declared name.
type SymbolKind int

const (
	SymbolUnknown SymbolKind = iota
	SymbolPackage
	SymbolNamespace
	SymbolAlias
	SymbolDependency
	SymbolComment
	SymbolDocumentation
	SymbolTextualRepresentation
)

var symbolKindNames = map[SymbolKind]string{
	SymbolUnknown:               "unknown",
	SymbolPackage:               "package",
	SymbolNamespace:             "namespace",
	SymbolAlias:                 "alias",
	SymbolDependency:            "dependency",
	SymbolComment:               "comment",
	SymbolDocumentation:         "documentation",
	SymbolTextualRepresentation: "textualRepresentation",
}

// String returns the display name of the kind.
func (k SymbolKind) String() string {
	if s, ok := symbolKindNames[k]; ok {
		return s
	}
	return "unknown"
}

// Symbol describes one declared name. The same declaration may be reachable
// through more than one Symbol only when it declares both a short and a
// primary name; in that case a single Symbol is registered under both keys.
type Symbol struct {
	Name       string         // the key this Symbol was primarily created for
	Kind       SymbolKind     // classification
	Decl       ast.Node       // the declaring AST node
	Visibility ast.Visibility // declared visibility
	DeclSpan   source.Span    // span of the declaration (for diagnostics)
	Scope      *Scope         // the child scope this declaration owns, or nil for leaves
	OwnerScope *Scope         // the enclosing scope this declaration was declared in

	// LeadingTrivia is the comment/note trivia attached to the member wrapper
	// preceding this declaration (captured before unwrap, since wrappers carry
	// the trivia while the unwrapped inner Decl does not). Used for doc hover.
	LeadingTrivia []ast.Trivia

	DocName string // name of the document that declares this symbol (stamped after Build)
}
