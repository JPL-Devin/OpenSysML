package lsp

import (
	"context"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// DocumentSymbol returns the hierarchical symbol tree for a document.
func (s *Server) DocumentSymbol(ctx context.Context, params *protocol.DocumentSymbolParams) ([]interface{}, error) {
	name := uriToName(params.TextDocument.URI)
	doc := s.ws.Document(name)
	if doc == nil || doc.Scope == nil {
		return nil, nil
	}
	syms := walkDocumentSymbols(doc.Content, doc.Scope)
	out := make([]interface{}, 0, len(syms))
	for _, ds := range syms {
		out = append(out, ds)
	}
	return out, nil
}

// walkDocumentSymbols converts a scope's members (and their child scopes) into
// a DocumentSymbol tree.
func walkDocumentSymbols(content []byte, scope *symbols.Scope) []protocol.DocumentSymbol {
	var out []protocol.DocumentSymbol
	for _, sym := range scope.Members() {
		if sym.Name == "" {
			continue
		}
		ds := protocol.DocumentSymbol{
			Name:           sym.Name,
			Kind:           lspSymbolKind(sym.Kind),
			Range:          spanToRange(content, sym.DeclSpan),
			SelectionRange: spanToRange(content, sym.DeclSpan),
		}
		if sym.Scope != nil {
			ds.Children = walkDocumentSymbols(content, sym.Scope)
		}
		out = append(out, ds)
	}
	return out
}

// lspSymbolKind maps a core SymbolKind to the LSP SymbolKind.
func lspSymbolKind(k symbols.SymbolKind) protocol.SymbolKind {
	switch k {
	case symbols.SymbolPackage:
		return protocol.SymbolKindPackage
	case symbols.SymbolNamespace:
		return protocol.SymbolKindNamespace
	case symbols.SymbolAlias:
		return protocol.SymbolKindVariable
	case symbols.SymbolDependency:
		return protocol.SymbolKindModule
	case symbols.SymbolComment, symbols.SymbolDocumentation, symbols.SymbolTextualRepresentation:
		return protocol.SymbolKindString
	default:
		return protocol.SymbolKindObject
	}
}
