package view

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Origin is where a node or an edge was declared: the document it was declared
// in and the span of the declaration. It is a core location, not a protocol one:
// a caller speaking a protocol converts it.
//
// An element with no locatable declaration — a cached standard-library symbol, a
// step a lowering sequenced without a declaration of its own — carries the zero
// Origin rather than a fabricated one.
type Origin struct {
	// Doc is the name of the document the declaration is in, "" when unknown.
	Doc string
	// Span is the span of the declaration within that document.
	Span source.Span
}

// Located reports whether the origin names a place in a document.
func (o Origin) Located() bool { return o.Doc != "" && o.Span.Len > 0 }

// symbolOrigin is where a symbol was declared, the zero Origin for a symbol
// carrying no declaration of its own.
func symbolOrigin(sym *symbols.Symbol) Origin {
	if sym == nil {
		return Origin{}
	}
	return originAt(sym.DocName, sym.DeclSpan)
}

// nodeOrigin is where an AST node of a lowered graph was written, in the
// document the element that lowered to it was declared in.
func nodeOrigin(doc string, node ast.Node) Origin {
	if node == nil {
		return Origin{}
	}
	return originAt(doc, node.Span())
}

// originAt is the origin of a span in a document, the zero Origin when either is
// unknown.
func originAt(doc string, span source.Span) Origin {
	if doc == "" || span.Len <= 0 {
		return Origin{}
	}
	return Origin{Doc: doc, Span: span}
}
