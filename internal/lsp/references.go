package lsp

import (
	"context"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// References returns all reference locations for the symbol under the cursor.
func (s *Server) References(ctx context.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	name := uriToName(params.TextDocument.URI)
	doc := s.ws.Document(name)
	if doc == nil || doc.Scope == nil {
		return nil, nil
	}
	content := doc.Content
	offset := positionToOffset(content, params.Position)

	refs := collectRefs(doc.AST, doc.Scope)

	// Determine the target symbol: either the cursor is on a reference (resolve it)
	// or on a declaration (find the symbol whose DeclSpan contains the cursor).
	var target *symbols.Symbol
	if ref := refAtOffset(refs, offset); ref != nil {
		if sym, ok := s.ws.ResolveReferenceInDoc(name, ref.Reference); ok {
			target = sym
		}
	}
	if target == nil {
		target = symbolAtOffset(doc.Scope, offset)
	}
	if target == nil {
		return nil, nil
	}

	var out []protocol.Location
	if params.Context.IncludeDeclaration {
		out = append(out, s.symbolLocation(name, target))
	}
	for _, ref := range refs {
		sym, ok := s.ws.ResolveReferenceInDoc(name, ref.Reference)
		// Pointer identity: same-document resolution stays inside the Document-tree
		// scope captured above (ref.Scope points into doc.Scope), so sym and target
		// are both Document-tree pointers and comparable. Cross-document references
		// (resolved through the global index tree) are out of scope for v1 and would
		// need FQN/DocName+DeclSpan equality instead of pointer identity.
		if !ok || sym != target {
			continue
		}
		// A QualifiedName resolves to the symbol of its terminal segment, so the
		// reference range highlights that segment, not the whole dotted name.
		refSpan := ref.QN.Span()
		if n := len(ref.QN.Parts); n > 0 {
			refSpan = ref.QN.Parts[n-1].Span
		}
		out = append(out, protocol.Location{
			URI:   nameToURI(name),
			Range: spanToRange(content, refSpan),
		})
	}
	return out, nil
}
