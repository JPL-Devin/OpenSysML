package lsp

import (
	"context"
	"reflect"
	"testing"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/Systemica/internal/core/model"
)

func TestInitializeAdvertisesCapabilities(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	res, err := s.Initialize(context.Background(), &protocol.InitializeParams{})
	if err != nil {
		t.Fatalf("Initialize error: %v", err)
	}
	sync, ok := res.Capabilities.TextDocumentSync.(*protocol.TextDocumentSyncOptions)
	if !ok {
		t.Fatalf("TextDocumentSync = %T, want *protocol.TextDocumentSyncOptions", res.Capabilities.TextDocumentSync)
	}
	if !sync.OpenClose {
		t.Error("OpenClose = false, want true")
	}
	if sync.Change != protocol.TextDocumentSyncKindIncremental {
		t.Errorf("Change = %v, want Incremental", sync.Change)
	}
	if res.Capabilities.HoverProvider != true {
		t.Error("HoverProvider not advertised")
	}
	if res.Capabilities.DefinitionProvider != true {
		t.Error("DefinitionProvider not advertised")
	}
	if res.Capabilities.ReferencesProvider != true {
		t.Error("ReferencesProvider not advertised")
	}
	if res.Capabilities.DocumentSymbolProvider != true {
		t.Error("DocumentSymbolProvider not advertised")
	}
	if res.Capabilities.WorkspaceSymbolProvider != true {
		t.Error("WorkspaceSymbolProvider not advertised")
	}
	if cp := res.Capabilities.CompletionProvider; cp == nil {
		t.Error("CompletionProvider not advertised")
	} else if !reflect.DeepEqual(cp.TriggerCharacters, []string{":", "."}) {
		t.Errorf("TriggerCharacters = %v, want [: .]", cp.TriggerCharacters)
	}
}

func TestShutdownAndExit(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	if err := s.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown error: %v", err)
	}
	if err := s.Exit(context.Background()); err != nil {
		t.Errorf("Exit error: %v", err)
	}
}
