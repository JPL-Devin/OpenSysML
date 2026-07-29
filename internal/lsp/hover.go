package lsp

import (
	"context"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Hover returns type/kind information for the declaration under the cursor.
func (s *Server) Hover(ctx context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	name := uriToName(params.TextDocument.URI)
	doc := s.ws.Document(name)
	if doc == nil || doc.Scope == nil {
		return nil, nil
	}
	content := doc.Content
	offset := positionToOffset(content, params.Position)
	sym := symbolAtOffset(doc.Scope, offset)
	if sym == nil {
		return nil, nil
	}

	var b strings.Builder
	b.WriteString(sym.Kind.String())
	if sym.Name != "" {
		b.WriteString(" ")
		b.WriteString(sym.Name)
	}
	if note := leadingDocText(content, sym.LeadingTrivia); note != "" {
		b.WriteString("\n\n")
		b.WriteString(note)
	}

	rng := spanToRange(content, sym.DeclSpan)
	return &protocol.Hover{
		Contents: protocol.MarkupContent{Kind: protocol.PlainText, Value: b.String()},
		Range:    &rng,
	}, nil
}

// symbolAtOffset finds the innermost symbol whose DeclSpan contains offset.
func symbolAtOffset(scope *symbols.Scope, offset int) *symbols.Symbol {
	for _, sym := range scope.Members() {
		sp := sym.DeclSpan
		if offset >= sp.Offset && offset < sp.End() {
			if sym.Scope != nil {
				if inner := symbolAtOffset(sym.Scope, offset); inner != nil {
					return inner
				}
			}
			return sym
		}
	}
	return nil
}

// leadingDocText returns the concatenated text of comment/note trivia
// preceding a declaration.
func leadingDocText(content []byte, trivia []ast.Trivia) string {
	if len(trivia) == 0 {
		return ""
	}
	var parts []string
	for _, tr := range trivia {
		switch tr.Kind {
		case ast.TriviaComment, ast.TriviaBlockNote, ast.TriviaLineNote:
			start, end := tr.Span.Offset, tr.Span.End()
			if start < 0 || end > len(content) || start > end {
				continue
			}
			parts = append(parts, strings.TrimSpace(string(content[start:end])))
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}
