package lsp

import (
	"context"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/Systemica/internal/core/lexer"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Completion implements textDocument/completion: in-scope member names plus
// SysML/KerML keywords. Prefix filtering is left to the client.
func (s *Server) Completion(ctx context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	seen := map[string]bool{}
	var items []protocol.CompletionItem

	add := func(label string, kind protocol.CompletionItemKind, detail string) {
		if label == "" || seen[label] {
			return
		}
		seen[label] = true
		items = append(items, protocol.CompletionItem{
			Label:  label,
			Kind:   kind,
			Detail: detail,
		})
	}

	name := uriToName(params.TextDocument.URI)
	if doc := s.ws.Document(name); doc != nil && doc.Scope != nil {
		offset := positionToOffset(doc.Content, params.Position)
		scope := enclosingScope(doc.Scope, offset)
		for scope != nil {
			for _, n := range scope.MemberNames() {
				add(n, protocol.CompletionItemKindModule, "member")
			}
			scope = scope.Parent()
		}
	}

	for _, kw := range lexer.Keywords() {
		add(kw, protocol.CompletionItemKindKeyword, "keyword")
	}

	return &protocol.CompletionList{IsIncomplete: false, Items: items}, nil
}

// enclosingScope returns the deepest scope whose owning declaration span
// contains offset, starting from root. Falls back to root.
func enclosingScope(root *symbols.Scope, offset int) *symbols.Scope {
	best := root
	for _, sym := range root.Members() {
		if sym.Scope == nil {
			continue
		}
		sp := sym.DeclSpan
		if offset >= sp.Offset && offset < sp.End() {
			return enclosingScope(sym.Scope, offset)
		}
	}
	return best
}
