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
		},
		ServerInfo: &protocol.ServerInfo{
			Name:    "systemica-lsp",
			Version: "0.1.0",
		},
	}, nil
}

// Initialized is a no-op notification acknowledgement.
func (s *Server) Initialized(ctx context.Context, params *protocol.InitializedParams) error {
	return nil
}

// Shutdown prepares the server for exit.
func (s *Server) Shutdown(ctx context.Context) error { return nil }

// Exit terminates the server process handling.
func (s *Server) Exit(ctx context.Context) error { return nil }
