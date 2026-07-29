package lsp

import (
	"context"

	"go.lsp.dev/protocol"
)

// publishDiagnostics analyzes the named document and pushes diagnostics to the
// client. It is a no-op when no client is connected.
func (s *Server) publishDiagnostics(ctx context.Context, name string) {
	if s.client == nil {
		return
	}
	doc := s.ws.Document(name)
	if doc == nil {
		return
	}
	content := doc.Content
	diags := s.ws.Diagnostics(name)
	out := make([]protocol.Diagnostic, 0, len(diags))
	for _, d := range diags {
		out = append(out, protocol.Diagnostic{
			Range:    spanToRange(content, d.Span),
			Severity: protocol.DiagnosticSeverity(int(d.Severity) + 1),
			Message:  d.Message,
			Code:     d.Code,
			Source:   d.Source,
		})
	}
	// Best-effort push: a failed notification has no recovery path here and the
	// server currently threads no logger, so the error is intentionally dropped.
	_ = s.client.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{
		URI:         nameToURI(name),
		Diagnostics: out,
	})
}
