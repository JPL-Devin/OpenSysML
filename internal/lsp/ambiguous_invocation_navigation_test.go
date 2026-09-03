package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
)

// ambiguousNavigationSrc imports two equally specific `pick(Integer)` overloads,
// so `pick(2)` is ambiguous while `pick("s")` selects the String one.
const ambiguousNavigationSrc = `package A {
	private import ScalarValues::*;
	calc def pick { in x : Integer; return : Integer = 1; }
}
package B {
	private import ScalarValues::*;
	calc def pick { in x : Integer; return : Integer = 2; }
	calc def pick { in x : String; return : Integer = 3; }
}
package P {
	private import ScalarValues::*;
	private import A::*;
	private import B::*;
	attribute i : Integer = pick(2);
	attribute s : Integer = pick("s");
}
`

func openAmbiguousNavigation(t *testing.T) (*model.Workspace, *Server, string) {
	t.Helper()
	ws := model.NewWorkspace()
	name := uri.File("/tmp/ambiguous_nav.sysml").Filename()
	ws.Open(name, []byte(ambiguousNavigationSrc), 1)
	return ws, NewServer(ws), name
}

// lineOfSrc is the zero-based line the first occurrence of needle is on.
func lineOfSrc(t *testing.T, src, needle string) uint32 {
	t.Helper()
	off := strings.Index(src, needle)
	if off < 0 {
		t.Fatalf("%q not in source", needle)
	}
	return uint32(strings.Count(src[:off], "\n"))
}

func referencesAt(t *testing.T, s *Server, name, src, needle string) []protocol.Location {
	t.Helper()
	off := strings.Index(src, needle)
	if off < 0 {
		t.Fatalf("%q not in source", needle)
	}
	locs, err := s.References(context.Background(), &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), off),
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: false},
	})
	if err != nil {
		t.Fatalf("References err = %v", err)
	}
	return locs
}

// Go-to-definition on an ambiguous call lists every overload the arguments
// leave tied, in declaration order; a call that selects one still goes to it.
func TestDefinitionListsAmbiguousOverloads(t *testing.T) {
	_, s, name := openAmbiguousNavigation(t)
	src := ambiguousNavigationSrc

	locs := definitionOf(t, s, name, src, "pick(2)")
	if len(locs) != 2 {
		t.Fatalf("pick(2): locations = %d, want both Integer overloads: %v", len(locs), locs)
	}
	wantLines := []uint32{
		lineOfSrc(t, src, "calc def pick { in x : Integer; return : Integer = 1;"),
		lineOfSrc(t, src, "calc def pick { in x : Integer; return : Integer = 2;"),
	}
	for i, loc := range locs {
		if loc.URI != uri.File(name) {
			t.Errorf("location %d URI = %q, want %q", i, loc.URI, uri.File(name))
		}
		if loc.Range.Start.Line != wantLines[i] {
			t.Errorf("location %d line = %d, want %d", i, loc.Range.Start.Line, wantLines[i])
		}
	}

	locs = definitionOf(t, s, name, src, `pick("s")`)
	if len(locs) != 1 {
		t.Fatalf(`pick("s"): locations = %d, want 1`, len(locs))
	}
	if got, want := locs[0].Range.Start.Line, lineOfSrc(t, src, "in x : String"); got != want {
		t.Errorf(`pick("s"): line = %d, want %d`, got, want)
	}
}

// Find-references never counts an ambiguous call for any of its overloads, and
// from the ambiguous call itself there is no one declaration to list uses of.
func TestReferencesSkipAmbiguousCalls(t *testing.T) {
	_, s, name := openAmbiguousNavigation(t)
	src := ambiguousNavigationSrc

	for _, decl := range []string{
		"pick { in x : Integer; return : Integer = 1;",
		"pick { in x : Integer; return : Integer = 2;",
	} {
		if locs := referencesAt(t, s, name, src, decl); len(locs) != 0 {
			t.Errorf("%s: references = %v, want none (the only call is ambiguous)", decl, locs)
		}
	}
	locs := referencesAt(t, s, name, src, "pick { in x : String")
	if len(locs) != 1 {
		t.Fatalf("String overload: references = %d, want the one selecting call: %v", len(locs), locs)
	}
	if at := positionToOffset([]byte(src), locs[0].Range.Start); !strings.HasPrefix(src[at:], `pick("s")`) {
		t.Errorf("reference at %q, want the String call", src[at:at+9])
	}
	if locs := referencesAt(t, s, name, src, "pick(2)"); len(locs) != 0 {
		t.Errorf("from the ambiguous call: references = %v, want none", locs)
	}
}

// Rename leaves an ambiguous call as written, whichever tied overload is renamed,
// and refuses to start from the call itself.
func TestRenameSkipsAmbiguousCalls(t *testing.T) {
	ws, _, name := openAmbiguousNavigation(t)

	out, err := applyRename(t, ws, name, "pick { in x : Integer; return : Integer = 1;", "choose")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	got := out[name]
	for _, want := range []string{
		"calc def choose { in x : Integer; return : Integer = 1;",
		"calc def pick { in x : Integer; return : Integer = 2;",
		"= pick(2);",
		`= pick("s");`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renamed source lacks %q:\n%s", want, got)
		}
	}

	out, err = applyRename(t, ws, name, "pick { in x : String", "text")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	got = out[name]
	for _, want := range []string{"calc def text { in x : String", `= text("s");`, "= pick(2);"} {
		if !strings.Contains(got, want) {
			t.Errorf("renamed source lacks %q:\n%s", want, got)
		}
	}

	if _, err := applyRename(t, ws, name, "pick(2)", "choose"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("rename from the ambiguous call: err = %v, want an ambiguity error", err)
	}
}

// Hover on an ambiguous call names each tied overload rather than one of them
// or the declaration the call sits in.
func TestHoverNamesAmbiguousOverloads(t *testing.T) {
	_, s, name := openAmbiguousNavigation(t)
	src := ambiguousNavigationSrc
	off := strings.Index(src, "pick(2)")
	hov, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), off),
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if hov == nil {
		t.Fatal("Hover = nil, want the tied overloads")
	}
	for _, want := range []string{"A::pick", "B::pick", "Ambiguous call"} {
		if !strings.Contains(hov.Contents.Value, want) {
			t.Errorf("hover %q lacks %q", hov.Contents.Value, want)
		}
	}
	if strings.Contains(hov.Contents.Value, "attribute") {
		t.Errorf("hover %q describes the enclosing attribute", hov.Contents.Value)
	}
	if hov.Range == nil || hov.Range.Start.Line != lineOfSrc(t, src, "pick(2)") {
		t.Errorf("hover range = %v, want the call's name", hov.Range)
	}
}
