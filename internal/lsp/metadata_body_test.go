package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
)

// metadataBodySrc annotates item p with @Anno: `own` is Anno's own feature,
// `inherited` comes from Base, `outer` is a feature of the enclosing item.
const metadataBodySrc = `metadata def Base {
	attribute inherited;
}
metadata def Anno :> Base {
	attribute own : ScalarValues::Integer;
}
item p {
	attribute outer;
	@Anno {
		own = outer;
		inherited = 2;
	}
}
item q {
	@Missing {
		ghost = 1;
	}
}
`

func openMetadataBodyDoc(t *testing.T) (*Server, string) {
	t.Helper()
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/metabody.sysml").Filename()
	ws.Open(name, []byte(metadataBodySrc), 1)
	return s, name
}

func hoverAt(t *testing.T, s *Server, name string, offset int) *protocol.Hover {
	t.Helper()
	h, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(metadataBodySrc), offset),
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	return h
}

func definitionAt(t *testing.T, s *Server, name string, offset int) []protocol.Location {
	t.Helper()
	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(metadataBodySrc), offset),
		},
	})
	if err != nil {
		t.Fatalf("Definition err = %v", err)
	}
	return locs
}

func bodyCompletionAt(t *testing.T, s *Server, name string, offset int) map[string]bool {
	t.Helper()
	list, err := s.Completion(context.Background(), &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(metadataBodySrc), offset),
		},
	})
	if err != nil {
		t.Fatalf("Completion err = %v", err)
	}
	labels := map[string]bool{}
	for _, item := range list.Items {
		labels[item.Label] = true
	}
	return labels
}

func TestMetadataBodyHoverNamesRedefinedFeature(t *testing.T) {
	s, name := openMetadataBodyDoc(t)

	h := hoverAt(t, s, name, strings.Index(metadataBodySrc, "own = outer"))
	if h == nil {
		t.Fatal("hover on body declaration returned nil")
	}
	text := h.Contents.Value
	if !strings.Contains(text, "redefines Anno::own") {
		t.Errorf("hover = %q, want it to name Anno::own", text)
	}
	if !strings.Contains(text, ": ScalarValues::Integer") {
		t.Errorf("hover = %q, want it to name the feature's type", text)
	}
}

func TestMetadataBodyDefinitionJumpsToMetadataFeature(t *testing.T) {
	s, name := openMetadataBodyDoc(t)

	locs := definitionAt(t, s, name, strings.Index(metadataBodySrc, "own = outer"))
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1", len(locs))
	}
	want := strings.Index(metadataBodySrc, "attribute own")
	if got := positionToOffset([]byte(metadataBodySrc), locs[0].Range.Start); got != want {
		t.Errorf("definition offset = %d, want %d (Anno's own declaration)", got, want)
	}
}

func TestMetadataBodyDefinitionJumpsToInheritedFeature(t *testing.T) {
	s, name := openMetadataBodyDoc(t)

	locs := definitionAt(t, s, name, strings.Index(metadataBodySrc, "inherited = 2"))
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1", len(locs))
	}
	want := strings.Index(metadataBodySrc, "attribute inherited")
	if got := positionToOffset([]byte(metadataBodySrc), locs[0].Range.Start); got != want {
		t.Errorf("definition offset = %d, want %d (Base's inherited declaration)", got, want)
	}
}

func TestMetadataBodyValueResolvesInEnclosingScope(t *testing.T) {
	s, name := openMetadataBodyDoc(t)

	locs := definitionAt(t, s, name, strings.Index(metadataBodySrc, "outer;\n\t\tinherited"))
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1", len(locs))
	}
	want := strings.Index(metadataBodySrc, "attribute outer")
	if got := positionToOffset([]byte(metadataBodySrc), locs[0].Range.Start); got != want {
		t.Errorf("definition offset = %d, want %d (the enclosing item's feature)", got, want)
	}
}

func TestMetadataBodyCompletionOffersDefinitionFeatures(t *testing.T) {
	s, name := openMetadataBodyDoc(t)

	// Declaration position: the start of the `inherited = 2;` statement.
	labels := bodyCompletionAt(t, s, name, strings.Index(metadataBodySrc, "inherited = 2"))
	if !labels["own"] || !labels["inherited"] {
		t.Errorf("completion = %v, want Anno's own and inherited features", labels)
	}
	if labels["outer"] || labels["p"] {
		t.Errorf("completion = %v, must not offer the enclosing namespace's names", labels)
	}
}

func TestMetadataBodyCompletionValuePositionUsesEnclosingScope(t *testing.T) {
	s, name := openMetadataBodyDoc(t)

	// Value position: just after `own = `.
	labels := bodyCompletionAt(t, s, name, strings.Index(metadataBodySrc, "outer;\n\t\tinherited"))
	if !labels["outer"] {
		t.Errorf("completion = %v, want the enclosing item's feature outer", labels)
	}
}

func TestMetadataBodyUnresolvedMetaclassDegradesQuietly(t *testing.T) {
	s, name := openMetadataBodyDoc(t)
	ghost := strings.Index(metadataBodySrc, "ghost = 1")

	if h := hoverAt(t, s, name, ghost); h != nil {
		text := h.Contents.Value
		if strings.Contains(text, "redefines") {
			t.Errorf("hover under unresolved metaclass = %q, want no redefinition", text)
		}
	}
	if locs := definitionAt(t, s, name, ghost); len(locs) != 0 {
		t.Errorf("definition under unresolved metaclass = %v, want none", locs)
	}
	// Completion must not error; whatever it offers, nothing comes from a
	// metadata definition that does not exist.
	bodyCompletionAt(t, s, name, ghost)
}

func TestMetadataBodyNestedAnnotation(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/metanest.sysml").Filename()
	src := `metadata def Tag {
	attribute label;
}
metadata def Anno {
	attribute own;
}
item p {
	@Anno {
		@Tag {
			label = 3;
		}
		own = 1;
	}
}
`
	ws.Open(name, []byte(src), 1)

	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), strings.Index(src, "label = 3")),
		},
	})
	if err != nil {
		t.Fatalf("Definition err = %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1", len(locs))
	}
	want := strings.Index(src, "attribute label")
	if got := positionToOffset([]byte(src), locs[0].Range.Start); got != want {
		t.Errorf("nested definition offset = %d, want %d (Tag's label)", got, want)
	}
}

func TestMetadataBodyDocumentSymbols(t *testing.T) {
	s, name := openMetadataBodyDoc(t)

	out, err := s.DocumentSymbol(context.Background(), &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
	})
	if err != nil {
		t.Fatalf("DocumentSymbol err = %v", err)
	}
	var names []string
	var walk func([]protocol.DocumentSymbol)
	walk = func(syms []protocol.DocumentSymbol) {
		for _, ds := range syms {
			names = append(names, ds.Name)
			walk(ds.Children)
		}
	}
	for _, v := range out {
		ds := v.(protocol.DocumentSymbol)
		names = append(names, ds.Name)
		walk(ds.Children)
	}
	joined := strings.Join(names, " ")
	if !strings.Contains(joined, "own") || !strings.Contains(joined, "inherited") {
		t.Errorf("document symbols = %v, want the body declarations own and inherited", names)
	}
}

func TestMetadataBodySemanticTokens(t *testing.T) {
	s, name := openMetadataBodyDoc(t)

	toks, err := s.SemanticTokensFull(context.Background(), &protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
	})
	if err != nil {
		t.Fatalf("SemanticTokensFull err = %v", err)
	}
	content := []byte(metadataBodySrc)
	covered := map[int]bool{}
	line, char := uint32(0), uint32(0)
	for i := 0; i+4 < len(toks.Data); i += 5 {
		line += toks.Data[i]
		if toks.Data[i] == 0 {
			char += toks.Data[i+1]
		} else {
			char = toks.Data[i+1]
		}
		covered[positionToOffset(content, protocol.Position{Line: line, Character: char})] = true
	}
	for _, anchor := range []string{"own = outer", "outer;\n\t\tinherited"} {
		if !covered[strings.Index(metadataBodySrc, anchor)] {
			t.Errorf("no semantic token at %q", anchor)
		}
	}
}
