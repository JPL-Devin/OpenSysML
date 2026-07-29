package lsp

import (
	"context"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/Systemica/internal/core/model"
)

func TestCompletionIncludesMembersAndKeywords(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	// Use an absolute name so uri.File(name).Filename() round-trips to the
	// same key the document was opened under.
	name := uri.File("/tmp/c.sysml").Filename()
	src := "package Alpha { namespace Nested; }\n"
	ws.Open(name, []byte(src), 1)

	// Cursor at end of file (top-level scope).
	pos := offsetToPosition([]byte(src), len(src))
	list, err := s.Completion(context.Background(), &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     pos,
		},
	})
	if err != nil {
		t.Fatalf("Completion err = %v", err)
	}
	if list == nil {
		t.Fatal("Completion returned nil list")
	}

	labels := map[string]protocol.CompletionItemKind{}
	for _, it := range list.Items {
		labels[it.Label] = it.Kind
	}
	if _, ok := labels["Alpha"]; !ok {
		t.Error("completion missing member 'Alpha'")
	}
	if k := labels["package"]; k != protocol.CompletionItemKindKeyword {
		t.Errorf("'package' kind = %v, want Keyword", k)
	}
	if list.IsIncomplete {
		t.Error("IsIncomplete = true, want false")
	}
}

func TestCompletionUnknownDocStillReturnsKeywords(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	list, err := s.Completion(context.Background(), &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File("missing.sysml")},
		},
	})
	if err != nil {
		t.Fatalf("Completion err = %v", err)
	}
	if list == nil || len(list.Items) == 0 {
		t.Fatal("expected keyword completions even for unknown doc")
	}
	found := false
	for _, it := range list.Items {
		if it.Label == "package" && it.Kind == protocol.CompletionItemKindKeyword {
			found = true
		}
	}
	if !found {
		t.Error("keyword 'package' not offered for unknown doc")
	}
}
