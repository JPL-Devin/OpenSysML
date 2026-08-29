// Package provenance carries source locations through derived semantic artifacts.
package provenance

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Origin identifies the source declaration behind a derived artifact.
// Synthetic and cache-restored artifacts use the zero value.
type Origin struct {
	Doc  string
	Span source.Span
	Name source.Span
}

// Located reports whether the origin identifies a source location.
func (o Origin) Located() bool { return o.Doc != "" && o.Span.Len > 0 }

// Symbol returns the origin of sym, or the zero origin when it is synthetic.
func Symbol(sym *symbols.Symbol) Origin {
	if sym == nil {
		return Origin{}
	}
	origin := At(sym.DocName, sym.DeclSpan)
	if origin.Located() && sym.NameSpan.Len > 0 {
		origin.Name = sym.NameSpan
	}
	return origin
}

// Node returns the origin of node in doc.
func Node(doc string, node ast.Node) Origin {
	if node == nil {
		return Origin{}
	}
	return At(doc, node.Span())
}

// At returns the origin of span in doc.
func At(doc string, span source.Span) Origin {
	if doc == "" || span.Len <= 0 {
		return Origin{}
	}
	return Origin{Doc: doc, Span: span}
}
