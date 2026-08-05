package lsp

import (
	"bytes"
	"context"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/Systemica/internal/core/format"
)

// Formatting re-indents the whole document. A document that does not parse is
// left alone: its brace structure is unreliable, and reformatting a file the
// author is midway through editing is worse than doing nothing.
func (s *Server) Formatting(ctx context.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	name := uriToName(params.TextDocument.URI)
	doc := s.ws.Document(name)
	if doc == nil || len(doc.ParseDiagnostics) > 0 {
		return nil, nil
	}
	out, err := format.Source(name, doc.Content, formatOptions(params.Options))
	if err != nil {
		return nil, err
	}
	if bytes.Equal(out, doc.Content) {
		return nil, nil
	}
	// One edit replacing the document: the formatter reflows whitespace
	// throughout, so a minimal diff would not be meaningfully smaller.
	return []protocol.TextEdit{{
		Range:   protocol.Range{Start: protocol.Position{}, End: offsetToPosition(doc.Content, len(doc.Content))},
		NewText: string(out),
	}}, nil
}

// formatOptions maps the client's editor settings onto the formatter's.
func formatOptions(opts protocol.FormattingOptions) format.Options {
	out := format.Options{IndentWidth: int(opts.TabSize), UseTabs: !opts.InsertSpaces}
	if out.IndentWidth <= 0 {
		out.IndentWidth = format.DefaultOptions.IndentWidth
	}
	return out
}
