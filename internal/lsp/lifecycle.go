package lsp

import (
	"context"

	"go.lsp.dev/protocol"
)

// Initialize advertises the server capabilities this plan implements.
func (s *Server) Initialize(ctx context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				OpenClose: true,
				Change:    protocol.TextDocumentSyncKindIncremental,
			},
			HoverProvider:           true,
			DefinitionProvider:      true,
			ReferencesProvider:      true,
			DocumentSymbolProvider:  true,
			WorkspaceSymbolProvider: true,
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{":", "."},
			},
			DocumentFormattingProvider: true,
			RenameProvider:             &protocol.RenameOptions{PrepareProvider: true},
			SemanticTokensProvider: &semanticTokensProvider{
				Legend: semanticTokensLegend(),
				Full:   true,
				Range:  true,
			},
			CodeActionProvider: &protocol.CodeActionOptions{
				CodeActionKinds: []protocol.CodeActionKind{protocol.QuickFix},
			},
		},
		ServerInfo: &protocol.ServerInfo{
			Name:    "sysml-lsp",
			Version: "0.1.0",
		},
	}, nil
}

// Initialized is a no-op notification acknowledgement.
func (s *Server) Initialized(ctx context.Context, params *protocol.InitializedParams) error {
	return nil
}

// Shutdown prepares the server for exit.
func (s *Server) Shutdown(ctx context.Context) error {
	s.shutdownDone = true
	return nil
}

// Exit terminates the server process. Per LSP spec, exit notification ends the process.
func (s *Server) Exit(ctx context.Context) error {
	// Close the jsonrpc2 connection to trigger graceful shutdown
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
