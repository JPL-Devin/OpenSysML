package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/Systemica/internal/core/model"
)

func TestHoverShowsKindAndName(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/h.sysml").Filename()
	src := "package P { namespace N; }"
	ws.Open(name, []byte(src), 1)

	// Cursor on "N" (offset of 'N' in src).
	off := strings.Index(src, "N")
	pos := offsetToPosition([]byte(src), off)

	res, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     pos,
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if res == nil {
		t.Fatal("Hover result = nil, want content")
	}
	if !strings.Contains(res.Contents.Value, "namespace") || !strings.Contains(res.Contents.Value, "N") {
		t.Errorf("hover value = %q, want kind+name", res.Contents.Value)
	}
}

func TestHoverIncludesDocComment(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/hd.sysml").Filename()
	src := "// hello docs\npackage P { namespace N; }"
	ws.Open(name, []byte(src), 1)

	// Cursor on "P" (the package declaration).
	off := strings.Index(src, "P")
	pos := offsetToPosition([]byte(src), off)

	res, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     pos,
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if res == nil {
		t.Fatal("Hover result = nil, want content")
	}
	if !strings.Contains(res.Contents.Value, "hello docs") {
		t.Errorf("hover value = %q, want doc-comment text", res.Contents.Value)
	}
}

func TestHoverMissWhenNoSymbol(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/h2.sysml").Filename()
	ws.Open(name, []byte("package P;"), 1)
	res, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     protocol.Position{Line: 5, Character: 0},
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if res != nil {
		t.Errorf("expected nil hover for out-of-range position, got %+v", res)
	}
}
