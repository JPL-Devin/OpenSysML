package lsp

import (
	"context"

	"go.lsp.dev/protocol"
)

// DidOpen registers a newly opened document with the workspace.
func (s *Server) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
	name := uriToName(params.TextDocument.URI)
	s.ws.Open(name, []byte(params.TextDocument.Text), int(params.TextDocument.Version))
	s.publishDiagnostics(ctx, name)
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
	s.applyDidChange(ctx, uriToName(params.TextDocument.URI), raw, int(params.TextDocument.Version))
	return nil
}

// DidClose marks the document closed, re-reading its on-disk content first so
// closing a tab does not unindex names other documents resolve through it.
func (s *Server) DidClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) error {
	name := uriToName(params.TextDocument.URI)
	s.loadFromDisk(name)
	s.ws.Close(name)
	s.refreshOpenDiagnostics(ctx)
	return nil
}

// DidSave refreshes diagnostics for every open document, since an edit to one
// file changes what the others resolve.
func (s *Server) DidSave(ctx context.Context, params *protocol.DidSaveTextDocumentParams) error {
	name := uriToName(params.TextDocument.URI)
	if !s.ws.IsOpen(name) {
		s.publishDiagnostics(ctx, name)
	}
	s.refreshOpenDiagnostics(ctx)
	return nil
}
