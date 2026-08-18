package lsp

import (
	"context"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/Systemica/internal/core/model"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// References returns every location naming the symbol under the cursor, in all
// workspace documents, at whichever segment of a qualified name denotes it.
func (s *Server) References(ctx context.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	name := uriToName(params.TextDocument.URI)
	doc := s.ws.Document(name)
	if doc == nil || doc.Scope == nil {
		return nil, nil
	}
	target := s.referenceTarget(name, doc, params.Position)
	if target == nil {
		return nil, nil
	}

	var out []protocol.Location
	seen := map[protocol.Location]bool{}
	add := func(docName string, content []byte, span source.Span) {
		loc := protocol.Location{URI: nameToURI(docName), Range: spanToRange(content, span)}
		if seen[loc] {
			return
		}
		seen[loc] = true
		out = append(out, loc)
	}

	if params.Context.IncludeDeclaration {
		loc := s.symbolLocation(name, target)
		seen[loc] = true
		out = append(out, loc)
	}
	for _, docName := range s.ws.DocumentNames() {
		refDoc := s.ws.Document(docName)
		if refDoc == nil || refDoc.Scope == nil {
			continue
		}
		for _, ref := range collectRefs(refDoc.AST, refDoc.Scope) {
			for i, seg := range s.ws.ResolveReferenceSegmentsInDoc(docName, ref) {
				if sameSymbol(seg, target) {
					add(docName, refDoc.Content, ref.QN.Parts[i].Span)
				}
			}
		}
	}
	return out, nil
}

// referenceTarget returns the symbol named at pos: the one a reference under the
// cursor resolves to, or the declaration the cursor sits in.
func (s *Server) referenceTarget(name string, doc *model.Document, pos protocol.Position) *symbols.Symbol {
	offset := positionToOffset(doc.Content, pos)
	if ref := refAtOffset(collectRefs(doc.AST, doc.Scope), offset); ref != nil {
		if sym, ok := s.ws.ResolveReferenceInDoc(name, *ref); ok {
			return sym
		}
	}
	return symbolAtOffset(doc.Scope, offset)
}
