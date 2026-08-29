package view

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/provenance"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Origin is the source declaration that produced a rendered artifact.
type Origin = provenance.Origin

// symbolOrigin is where a symbol was declared, the zero Origin for a symbol
// carrying no declaration of its own.
func symbolOrigin(sym *symbols.Symbol) Origin {
	return provenance.Symbol(sym)
}

// nodeOrigin is where an AST node of a lowered graph was written, in the
// document the element that lowered to it was declared in.
func nodeOrigin(doc string, node ast.Node) Origin {
	return provenance.Node(doc, node)
}
