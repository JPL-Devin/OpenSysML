package lsp

import (
	"context"

	"go.lsp.dev/protocol"
)

// DidOpen registers a newly opened document with the workspace.
func (s *Server) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
	name := uriToName(params.TextDocument.URI)
	s.ws.Open(name, []byte(params.TextDocument.Text), int(params.TextDocument.Version))
	return nil
}

// DidChange applies content changes (full-replace or incremental) then updates
// the workspace document.
//
// NOTE: real transport traffic is intercepted by (*Server).changeHandler, which
// decodes the raw params with a pointer-valued Range so an omitted range
// (full-document replace) is distinguishable from an incremental insertion at
// {0,0}. This typed method is retained to satisfy protocol.Server and for
// direct unit testing; because protocol.TextDocumentContentChangeEvent.Range is
// a value type, this path treats a zero Range as an incremental splice at
// [0,0) and never as a full replace.
func (s *Server) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	raw := make([]rawContentChange, len(params.ContentChanges))
	for i, ch := range params.ContentChanges {
		rng := ch.Range
		raw[i] = rawContentChange{Range: &rng, RangeLength: ch.RangeLength, Text: ch.Text}
	}
	s.applyDidChange(uriToName(params.TextDocument.URI), raw, int(params.TextDocument.Version))
	return nil
}

// DidClose marks the document closed in the workspace.
func (s *Server) DidClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) error {
	s.ws.Close(uriToName(params.TextDocument.URI))
	return nil
}

// DidSave is a no-op for now; diagnostics are refreshed on change.
func (s *Server) DidSave(ctx context.Context, params *protocol.DidSaveTextDocumentParams) error {
	return nil
}
