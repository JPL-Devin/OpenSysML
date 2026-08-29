package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
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

// initMarkdownHover initializes s as a client that renders Markdown hovers.
func initMarkdownHover(t *testing.T, s *Server) {
	t.Helper()
	_, err := s.Initialize(context.Background(), &protocol.InitializeParams{
		Capabilities: protocol.ClientCapabilities{
			TextDocument: &protocol.TextDocumentClientCapabilities{
				Hover: &protocol.HoverTextDocumentClientCapabilities{
					ContentFormat: []protocol.MarkupKind{protocol.Markdown, protocol.PlainText},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Initialize err = %v", err)
	}
}

func hoverInSrc(t *testing.T, s *Server, name, src string, off int) *protocol.Hover {
	t.Helper()
	res, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), off),
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if res == nil {
		t.Fatal("Hover result = nil, want content")
	}
	return res
}

func TestHoverRendersMarkdownWhenClientSupportsIt(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	initMarkdownHover(t, s)
	name := uri.File("/tmp/hm.sysml").Filename()
	src := "package P {\n    doc /*\n     * A wheel.\n     */\n    part def Wheel;\n}\n"
	ws.Open(name, []byte(src), 1)

	res := hoverInSrc(t, s, name, src, strings.Index(src, "Wheel"))
	if res.Contents.Kind != protocol.Markdown {
		t.Errorf("hover kind = %q, want %q", res.Contents.Kind, protocol.Markdown)
	}
	if !strings.Contains(res.Contents.Value, "```sysml") {
		t.Errorf("hover value = %q, want a fenced sysml block", res.Contents.Value)
	}
	if !strings.Contains(res.Contents.Value, "Wheel") {
		t.Errorf("hover value = %q, want the signature", res.Contents.Value)
	}
	if !strings.Contains(res.Contents.Value, "A wheel.") {
		t.Errorf("hover value = %q, want the doc text", res.Contents.Value)
	}
	for _, marker := range []string{"/*", "*/", " * "} {
		if strings.Contains(res.Contents.Value, marker) {
			t.Errorf("hover value = %q, still carries the comment marker %q", res.Contents.Value, marker)
		}
	}
}

func TestHoverSignatureUsesNotationKeywords(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	initMarkdownHover(t, s)
	name := uri.File("/tmp/hk.sysml").Filename()
	src := "package P {\n    part def Wheel;\n    part w : Wheel;\n}\n"
	ws.Open(name, []byte(src), 1)

	for _, tc := range []struct {
		at   string
		want string
	}{
		{at: "Wheel;", want: "```sysml\npart def Wheel\n```"},
		{at: "w :", want: "```sysml\npart w\n```"},
	} {
		res := hoverInSrc(t, s, name, src, strings.Index(src, tc.at))
		if res.Contents.Value != tc.want {
			t.Errorf("hover on %q = %q, want %q", tc.at, res.Contents.Value, tc.want)
		}
	}
}

func TestHoverStripsDelimitersOfEveryLeadingComment(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	initMarkdownHover(t, s)
	name := uri.File("/tmp/hc.sysml").Filename()
	src := "package P {\n    /* first */\n    /* second */\n    part def Wheel;\n}\n"
	ws.Open(name, []byte(src), 1)

	res := hoverInSrc(t, s, name, src, strings.Index(src, "Wheel;"))
	want := "```sysml\npart def Wheel\n```\n\nfirst\n\nsecond"
	if res.Contents.Value != want {
		t.Errorf("hover value = %q, want %q", res.Contents.Value, want)
	}
}

func TestHoverKeepsDocCommentLineBreaks(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	initMarkdownHover(t, s)
	name := uri.File("/tmp/hl.sysml").Filename()
	src := "package P {\n    doc /*\n     * First line.\n     * Second line.\n     */\n    part def Wheel;\n}\n"
	ws.Open(name, []byte(src), 1)

	res := hoverInSrc(t, s, name, src, strings.Index(src, "Wheel;"))
	want := "```sysml\npart def Wheel\n```\n\nFirst line.  \nSecond line."
	if res.Contents.Value != want {
		t.Errorf("hover value = %q, want %q", res.Contents.Value, want)
	}
}

func TestHoverFallsBackToPlainTextWithoutMarkdownCapability(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	// No initialize: a client that never advertised Markdown must get plaintext.
	name := uri.File("/tmp/hp.sysml").Filename()
	src := "package P { namespace N; }"
	ws.Open(name, []byte(src), 1)

	res := hoverInSrc(t, s, name, src, strings.Index(src, "N"))
	if res.Contents.Kind != protocol.PlainText {
		t.Errorf("hover kind = %q, want %q", res.Contents.Kind, protocol.PlainText)
	}
	if strings.Contains(res.Contents.Value, "```") {
		t.Errorf("plaintext hover value = %q, should carry no Markdown fence", res.Contents.Value)
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
