package lsp

import (
	"context"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Definition returns the declaration location of the reference under the cursor.
func (s *Server) Definition(ctx context.Context, params *protocol.DefinitionParams) ([]protocol.Location, error) {
	name := uriToName(params.TextDocument.URI)
	doc := s.ws.Document(name)
	if doc == nil || doc.Scope == nil {
		return nil, nil
	}
	content := doc.Content
	offset := positionToOffset(content, params.Position)

	refs := collectRefs(doc.AST, doc.Scope)
	ref := refAtOffset(refs, offset)
	if ref == nil {
		return nil, nil
	}
	sym, ok := s.ws.ResolveReferenceInDoc(name, *ref)
	if !ok || sym == nil {
		return nil, nil
	}
	return []protocol.Location{s.symbolLocation(name, sym)}, nil
}

// symbolLocation builds a Location for a resolved symbol using the symbol's own
// declaring document (DocName), so cross-file definitions point at the correct
// file/bytes. Falls back to the requesting document name if the symbol was not
// stamped (should not happen for resolved symbols).
func (s *Server) symbolLocation(reqName string, sym *symbols.Symbol) protocol.Location {
	declName := sym.DocName
	if declName == "" {
		declName = reqName // fallback: symbol not stamped (shouldn't happen for resolved symbols)
	}
	var content []byte
	if doc := s.ws.Document(declName); doc != nil {
		content = doc.Content
	}
	return protocol.Location{URI: nameToURI(declName), Range: spanToRange(content, sym.DeclSpan)}
}
