package lsp

import (
	"context"
	"sort"
	"strings"
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

const completionSrc = `package P {
	part def Wheel {
		attribute pressure;
	}
	// A vehicle under test.
	part def Vehicle {
		part wheel : Wheel;
		action drive;
	}
	part v : Vehicle;
	part done : Vehicle {
		v.
	}
}
`

// completionAt returns the completion items offered with the cursor placed
// right after the first occurrence of marker in src, keyed by label.
func completionAt(t *testing.T, src, marker string) map[string]protocol.CompletionItem {
	t.Helper()
	cut := strings.Index(src, marker)
	if cut < 0 {
		t.Fatalf("marker %q not found in source", marker)
	}
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/completion.sysml").Filename()
	ws.Open(name, []byte(src), 1)

	list, err := s.Completion(context.Background(), &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), cut+len(marker)),
		},
	})
	if err != nil {
		t.Fatalf("Completion err = %v", err)
	}
	if list == nil {
		t.Fatal("Completion returned nil list")
	}
	items := map[string]protocol.CompletionItem{}
	for _, it := range list.Items {
		items[it.Label] = it
	}
	return items
}

func TestCompletionCarriesKindDetailAndDocumentation(t *testing.T) {
	items := completionAt(t, completionSrc, "part v : Vehicle;")

	def, ok := items["Vehicle"]
	if !ok {
		t.Fatal("completion missing 'Vehicle'")
	}
	if def.Kind != protocol.CompletionItemKindClass {
		t.Errorf("'Vehicle' kind = %v, want Class", def.Kind)
	}
	if def.Detail != "partDef" {
		t.Errorf("'Vehicle' detail = %q, want %q", def.Detail, "partDef")
	}
	if doc, _ := def.Documentation.(protocol.MarkupContent); !strings.Contains(doc.Value, "A vehicle under test.") {
		t.Errorf("'Vehicle' documentation = %#v, want the declaration's comment", def.Documentation)
	}

	usage, ok := items["v"]
	if !ok {
		t.Fatal("completion missing 'v'")
	}
	if usage.Kind != protocol.CompletionItemKindField {
		t.Errorf("'v' kind = %v, want Field", usage.Kind)
	}
	if usage.Detail != "partUsage : Vehicle" {
		t.Errorf("'v' detail = %q, want %q", usage.Detail, "partUsage : Vehicle")
	}
}

func TestCompletionOnDotOffersMembersOfType(t *testing.T) {
	items := completionAt(t, completionSrc, "\t\tv.")

	wheel, ok := items["wheel"]
	if !ok {
		t.Fatalf("completion after 'v.' missing 'wheel'; got %v", labelsOf(items))
	}
	if wheel.Detail != "partUsage : Wheel" {
		t.Errorf("'wheel' detail = %q, want %q", wheel.Detail, "partUsage : Wheel")
	}
	if drive, ok := items["drive"]; !ok {
		t.Error("completion after 'v.' missing 'drive'")
	} else if drive.Kind != protocol.CompletionItemKindMethod {
		t.Errorf("'drive' kind = %v, want Method", drive.Kind)
	}
	if _, ok := items["package"]; ok {
		t.Error("completion after 'v.' offered the keyword 'package'")
	}
	if _, ok := items["Vehicle"]; ok {
		t.Error("completion after 'v.' offered the in-scope name 'Vehicle'")
	}
}

func TestCompletionOnDotOffersNestedMembers(t *testing.T) {
	src := strings.Replace(completionSrc, "\t\tv.\n", "\t\tv.wheel.\n", 1)
	items := completionAt(t, src, "v.wheel.")
	if _, ok := items["pressure"]; !ok {
		t.Errorf("completion after 'v.wheel.' missing 'pressure'; got %v", labelsOf(items))
	}
}

func TestCompletionOnDotStillFiltersOnTheTypedPrefix(t *testing.T) {
	src := strings.Replace(completionSrc, "\t\tv.\n", "\t\tv.whe\n", 1)
	items := completionAt(t, src, "v.whe")
	if _, ok := items["wheel"]; !ok {
		t.Errorf("completion after 'v.whe' missing 'wheel'; got %v", labelsOf(items))
	}
}

func TestCompletionOffersLibrarySymbols(t *testing.T) {
	items := completionAt(t, completionSrc, "part v : Vehicle;")
	if _, ok := items["ScalarValues"]; !ok {
		t.Errorf("completion missing library package 'ScalarValues'; got %v", labelsOf(items))
	}
}

func TestCompletionOnQualifiedNameOffersLibraryMembers(t *testing.T) {
	src := strings.Replace(completionSrc, "\t\tv.\n", "\t\tattribute x : ScalarValues::\n", 1)
	items := completionAt(t, src, "ScalarValues::")
	real, ok := items["Real"]
	if !ok {
		t.Fatalf("completion after 'ScalarValues::' missing 'Real'; got %v", labelsOf(items))
	}
	if real.Kind == protocol.CompletionItemKindKeyword {
		t.Errorf("'Real' kind = %v, want a declaration kind", real.Kind)
	}
}

func TestCompletionUnresolvedPathOffersNothing(t *testing.T) {
	src := strings.Replace(completionSrc, "\t\tv.\n", "\t\tnosuch.\n", 1)
	if items := completionAt(t, src, "nosuch."); len(items) != 0 {
		t.Errorf("completion after an unresolved path offered %v", labelsOf(items))
	}
}

// A private import surfaces names at the root of the index; KerML 8.2.3.3 keeps
// them out of every other document, so completion must not offer them there.
func TestCompletionHidesAnotherDocumentsPrivateImport(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	importer := uri.File("/tmp/importer.sysml").Filename()
	ws.Open(importer, []byte("private import ScalarValues::*;\npackage Importer;\n"), 1)

	other := uri.File("/tmp/other.sysml").Filename()
	src := "package Other {\n\t\n}\n"
	ws.Open(other, []byte(src), 1)

	list, err := s.Completion(context.Background(), &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(other)},
			Position:     offsetToPosition([]byte(src), strings.Index(src, "\t")+1),
		},
	})
	if err != nil {
		t.Fatalf("Completion err = %v", err)
	}
	for _, it := range list.Items {
		if it.Label == "Real" {
			t.Error("completion offered 'Real', which only another document's private import surfaced")
		}
	}
}

// The dot in a numeric literal is not a member access, so completion there must
// still offer the ordinary scope and keyword list.
func TestCompletionAfterNumericLiteralOffersTheScopeList(t *testing.T) {
	src := strings.Replace(completionSrc, "\t\tv.\n", "\t\tattribute mass = 1.\n", 1)
	items := completionAt(t, src, "= 1.")
	if _, ok := items["package"]; !ok {
		t.Errorf("completion after '1.' missing keywords; got %v", labelsOf(items))
	}
	if _, ok := items["Vehicle"]; !ok {
		t.Errorf("completion after '1.' missing in-scope name 'Vehicle'; got %v", labelsOf(items))
	}
}

func TestMemberPathBefore(t *testing.T) {
	cases := []struct {
		text string
		want []string
	}{
		{"v.", []string{"v"}},
		{"v.whe", []string{"v"}},
		{"a.b.c.", []string{"a", "b", "c"}},
		{"A::B::", []string{"A", "B"}},
		{"A::b.", []string{"A", "b"}},
		{"= 1.", nil},
		{"= 2.5", nil},
		{"x1.", []string{"x1"}},
		{"part x", nil},
		{"", nil},
		{".", nil},
	}
	for _, tc := range cases {
		got, ok := memberPathBefore([]byte(tc.text), len(tc.text))
		if ok != (tc.want != nil) {
			t.Errorf("memberPathBefore(%q) ok = %v, want %v", tc.text, ok, tc.want != nil)
			continue
		}
		if len(got) != len(tc.want) {
			t.Errorf("memberPathBefore(%q) = %v, want %v", tc.text, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("memberPathBefore(%q) = %v, want %v", tc.text, got, tc.want)
				break
			}
		}
	}
}

// A short name is a name the element is referable by, so completion must offer
// it alongside the long name, both in scope and as a member.
func TestCompletionOffersShortNames(t *testing.T) {
	src := strings.Replace(completionSrc, "part def Vehicle {", "part def <veh> Vehicle {", 1)
	src = strings.Replace(src, "part wheel : Wheel;", "part <w> wheel : Wheel;", 1)

	items := completionAt(t, src, "part v : Vehicle;")
	short, ok := items["veh"]
	if !ok {
		t.Fatalf("completion missing short name 'veh'; got %v", labelsOf(items))
	}
	if short.Kind != protocol.CompletionItemKindClass || short.Detail != "partDef" {
		t.Errorf("'veh' = kind %v detail %q, want the same as 'Vehicle'", short.Kind, short.Detail)
	}

	members := completionAt(t, src, "v.")
	if _, ok := members["w"]; !ok {
		t.Errorf("member completion missing short name 'w'; got %v", labelsOf(members))
	}
}

func labelsOf(items map[string]protocol.CompletionItem) []string {
	out := make([]string, 0, len(items))
	for label := range items {
		out = append(out, label)
	}
	sort.Strings(out)
	return out
}
