package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/Systemica/internal/core/model"
)

func TestDefinitionJumpsToDeclaration(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	// Absolute name so uri.File(name).Filename() round-trips back to name.
	name := uri.File("/tmp/def.sysml").Filename()
	src := "package P { namespace N; }\nimport P::N;"
	ws.Open(name, []byte(src), 1)

	// Cursor on the "N" inside "import P::N;".
	off := strings.LastIndex(src, "N")
	pos := offsetToPosition([]byte(src), off)

	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     pos,
		},
	})
	if err != nil {
		t.Fatalf("Definition err = %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1", len(locs))
	}
	if locs[0].URI != uri.File(name) {
		t.Errorf("URI = %q, want %q", locs[0].URI, uri.File(name))
	}
	// Declaration "N" is on line 0.
	if locs[0].Range.Start.Line != 0 {
		t.Errorf("decl line = %d, want 0", locs[0].Range.Start.Line)
	}
}

func TestDefinitionCrossFile(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	// Two docs under distinct absolute round-trip names.
	libName := uri.File("/tmp/lib.sysml").Filename()
	useName := uri.File("/tmp/use.sysml").Filename()
	ws.Open(libName, []byte("package P { namespace N; }"), 1)
	useSrc := "import P::N;"
	ws.Open(useName, []byte(useSrc), 1)

	// Cursor on the "N" inside "import P::N;" in the using doc.
	off := strings.LastIndex(useSrc, "N")
	pos := offsetToPosition([]byte(useSrc), off)

	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(useName)},
			Position:     pos,
		},
	})
	if err != nil {
		t.Fatalf("Definition err = %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1", len(locs))
	}
	// The declaring file is libName, not the requesting useName.
	if locs[0].URI != uri.File(libName) {
		t.Errorf("URI = %q, want %q", locs[0].URI, uri.File(libName))
	}
	if locs[0].Range.Start.Line != 0 {
		t.Errorf("decl line = %d, want 0", locs[0].Range.Start.Line)
	}
}
