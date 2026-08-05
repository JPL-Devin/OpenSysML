package lsp

import (
	"context"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/Systemica/internal/core/model"
)

func formatDoc(t *testing.T, src string, opts protocol.FormattingOptions) ([]protocol.TextEdit, string) {
	t.Helper()
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/fmt.sysml").Filename()
	ws.Open(name, []byte(src), 1)

	edits, err := s.Formatting(context.Background(), &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
		Options:      opts,
	})
	if err != nil {
		t.Fatalf("Formatting err = %v", err)
	}
	if len(edits) == 0 {
		return edits, src
	}
	return edits, applyEdits(t, src, edits)
}

var spaces4 = protocol.FormattingOptions{TabSize: 4, InsertSpaces: true}

func TestFormattingReindentsDocument(t *testing.T) {
	edits, got := formatDoc(t, "package P {\npart def Car;\n}\n", spaces4)
	if len(edits) != 1 {
		t.Fatalf("edits = %d, want 1 whole-document edit", len(edits))
	}
	if want := "package P {\n    part def Car;\n}\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormattingHonoursClientIndentSettings(t *testing.T) {
	if _, got := formatDoc(t, "package P {\npart def Car;\n}\n", protocol.FormattingOptions{TabSize: 2, InsertSpaces: true}); got != "package P {\n  part def Car;\n}\n" {
		t.Errorf("two-space indent: got %q", got)
	}
	if _, got := formatDoc(t, "package P {\npart def Car;\n}\n", protocol.FormattingOptions{TabSize: 4}); got != "package P {\n\tpart def Car;\n}\n" {
		t.Errorf("tab indent: got %q", got)
	}
}

func TestFormattingReturnsNoEditsForFormattedDocument(t *testing.T) {
	if edits, _ := formatDoc(t, "package P {\n    part def Car;\n}\n", spaces4); len(edits) != 0 {
		t.Fatalf("edits = %v, want none", edits)
	}
}

// A file whose last line is a comment with no newline after it still formats.
func TestFormattingDocumentEndingInUnterminatedComment(t *testing.T) {
	_, got := formatDoc(t, "package P {\npart def Car;\n}\n// note", spaces4)
	if want := "package P {\n    part def Car;\n}\n// note\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormattingSkipsDocumentThatDoesNotParse(t *testing.T) {
	// Brace depth is meaningless here, so the file must be left untouched.
	if edits, _ := formatDoc(t, "package P { part def\n", spaces4); len(edits) != 0 {
		t.Fatalf("edits = %v, want none for an unparseable document", edits)
	}
}

func TestFormattingUnknownDocument(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	edits, err := s.Formatting(context.Background(), &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File("/tmp/missing.sysml")},
		Options:      spaces4,
	})
	if err != nil || edits != nil {
		t.Fatalf("Formatting = %v, %v; want nil, nil", edits, err)
	}
}
