package lsp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/core/format"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
)

var spaces4 = protocol.FormattingOptions{TabSize: 4, InsertSpaces: true}

func openFormatDoc(t *testing.T, src string) (*Server, string) {
	t.Helper()
	ws := model.NewWorkspace()
	name := uri.File("/tmp/fmt.sysml").Filename()
	ws.Open(name, []byte(src), 1)
	return NewServer(ws), name
}

// formatEditsFor runs Formatting on src and returns its edits.
func formatEditsFor(t *testing.T, src string, opts protocol.FormattingOptions) []protocol.TextEdit {
	t.Helper()
	s, name := openFormatDoc(t, src)
	edits, err := s.Formatting(context.Background(), &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
		Options:      opts,
	})
	if err != nil {
		t.Fatalf("Formatting err = %v", err)
	}
	return edits
}

// rangeEditsFor runs RangeFormatting over r on src and returns its edits.
func rangeEditsFor(t *testing.T, src string, r protocol.Range) []protocol.TextEdit {
	t.Helper()
	s, name := openFormatDoc(t, src)
	edits, err := s.RangeFormatting(context.Background(), &protocol.DocumentRangeFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
		Range:        r,
		Options:      spaces4,
	})
	if err != nil {
		t.Fatalf("RangeFormatting err = %v", err)
	}
	return edits
}

func formatDoc(t *testing.T, src string, opts protocol.FormattingOptions) ([]protocol.TextEdit, string) {
	t.Helper()
	edits := formatEditsFor(t, src, opts)
	return edits, applyOrderedEdits(t, src, edits)
}

func pos(line, char int) protocol.Position {
	return protocol.Position{Line: uint32(line), Character: uint32(char)}
}

func lines(from, to int) protocol.Range {
	return protocol.Range{Start: pos(from, 0), End: pos(to, 0)}
}

// applyOrderedEdits checks edits are in document order and never overlap, then
// applies them back to front as a client does.
func applyOrderedEdits(t *testing.T, content string, edits []protocol.TextEdit) string {
	t.Helper()
	src := []byte(content)
	type span struct{ start, end int }
	spans := make([]span, len(edits))
	for i, e := range edits {
		start, end := positionToOffset(src, e.Range.Start), positionToOffset(src, e.Range.End)
		if offsetToPosition(src, start) != e.Range.Start || offsetToPosition(src, end) != e.Range.End {
			t.Fatalf("edit %d range %v does not land on a UTF-16 boundary of the document", i, e.Range)
		}
		if end < start {
			t.Fatalf("edit %d %v ends before it starts", i, e.Range)
		}
		if i > 0 && start < spans[i-1].end {
			t.Fatalf("edit %d %v overlaps or precedes edit %d %v", i, e.Range, i-1, edits[i-1].Range)
		}
		spans[i] = span{start, end}
	}
	for i := len(edits) - 1; i >= 0; i-- {
		src = append(src[:spans[i].start], append([]byte(edits[i].NewText), src[spans[i].end:]...)...)
	}
	return string(src)
}

func TestFormattingReindentsDocument(t *testing.T) {
	edits, got := formatDoc(t, "package P {\npart def Car;\n}\n", spaces4)
	if want := "package P {\n    part def Car;\n}\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	want := []protocol.TextEdit{{Range: protocol.Range{Start: pos(1, 0), End: pos(1, 0)}, NewText: "    "}}
	if len(edits) != 1 || edits[0] != want[0] {
		t.Fatalf("edits = %+v, want %+v", edits, want)
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

// Only the lines the formatter changes are edited, and only the characters
// that differ, so the editor can keep its undo history, cursor and folds.
func TestFormattingEditsOnlyChangedLines(t *testing.T) {
	src := "package P {\n    part def Car;\n  part def Bus;\n    part def Van;   \n}\n"
	edits, got := formatDoc(t, src, spaces4)
	if want := "package P {\n    part def Car;\n    part def Bus;\n    part def Van;\n}\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	want := []protocol.TextEdit{
		{Range: protocol.Range{Start: pos(2, 2), End: pos(2, 2)}, NewText: "  "},
		{Range: protocol.Range{Start: pos(3, 17), End: pos(3, 20)}, NewText: ""},
	}
	if len(edits) != len(want) || edits[0] != want[0] || edits[1] != want[1] {
		t.Fatalf("edits = %+v, want %+v", edits, want)
	}
}

// Blank lines the formatter collapses become one deletion, not a rewrite of
// the surrounding lines.
func TestFormattingDeletesCollapsedBlankLines(t *testing.T) {
	src := "package P {\n    part def Car;\n\n\n\n    part def Bus;\n}\n"
	edits, got := formatDoc(t, src, spaces4)
	want, err := format.Source("fmt.sysml", []byte(src), format.DefaultOptions)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("got %q, want %q", got, string(want))
	}
	if len(edits) != 1 || edits[0].NewText != "" || edits[0].Range.Start.Line < 2 || edits[0].Range.End.Line > 5 {
		t.Fatalf("edits = %+v, want one deletion inside the blank run", edits)
	}
}

// Columns are UTF-16 code units: an astral-plane character counts as two.
func TestFormattingPositionsAreUTF16(t *testing.T) {
	src := "package P {\n    /* 😀 */ part def Car;   \n  attribute s = \"🚗\";\n}\n"
	edits, got := formatDoc(t, src, spaces4)
	want, err := format.Source("fmt.sysml", []byte(src), format.DefaultOptions)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("got %q, want %q", got, string(want))
	}
	// "    /* 😀 */ part def Car;" is 26 UTF-16 units (28 bytes): the emoji
	// is two units, four bytes.
	wantEdits := []protocol.TextEdit{
		{Range: protocol.Range{Start: pos(1, 26), End: pos(1, 29)}, NewText: ""},
		{Range: protocol.Range{Start: pos(2, 2), End: pos(2, 2)}, NewText: "  "},
	}
	if len(edits) != len(wantEdits) || edits[0] != wantEdits[0] || edits[1] != wantEdits[1] {
		t.Fatalf("edits = %+v, want %+v", edits, wantEdits)
	}
}

// A CRLF document keeps its line endings, and each edit stays within its line.
func TestFormattingKeepsCRLF(t *testing.T) {
	src := "package P {\r\npart def Car;\r\n}\r\n"
	edits, got := formatDoc(t, src, spaces4)
	if want := "package P {\r\n    part def Car;\r\n}\r\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if len(edits) != 1 || edits[0].NewText != "    " {
		t.Fatalf("edits = %+v, want one indentation insert", edits)
	}
}

// Every corpus model formats to exactly what format.Source produces when the
// edits are applied, including files that need no edits or do not parse.
func TestFormattingEditsReproduceFormatterOutput(t *testing.T) {
	for _, path := range formattingCorpus(t) {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(filepath.Base(path), func(t *testing.T) {
			checkFormattingEdits(t, path, string(content))
			// A mangled copy changes many lines at once.
			checkFormattingEdits(t, path, mangleIndentation(string(content)))
		})
	}
}

func formattingCorpus(t *testing.T) []string {
	t.Helper()
	var paths []string
	for _, pattern := range []string{
		"../core/format/testdata/*.sysml",
		"../../examples/*.sysml",
		"../repl/testdata/*.sysml",
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) == 0 {
			t.Fatalf("no fixtures match %s", pattern)
		}
		paths = append(paths, matches...)
	}
	return paths
}

// mangleIndentation shifts every line's indentation and pads a few lines with
// trailing whitespace and extra blank lines.
func mangleIndentation(src string) string {
	var b strings.Builder
	for i, line := range strings.SplitAfter(src, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		switch i % 4 {
		case 0:
			b.WriteString(trimmed)
		case 1:
			b.WriteString("\t" + line)
		case 2:
			b.WriteString(strings.TrimSuffix(line, "\n") + "  \n")
		default:
			b.WriteString(line + "\n\n")
		}
	}
	return b.String()
}

func checkFormattingEdits(t *testing.T, path, src string) {
	t.Helper()
	s, name := openFormatDoc(t, src)
	edits, err := s.Formatting(context.Background(), &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
		Options:      spaces4,
	})
	if err != nil {
		t.Fatalf("%s: Formatting err = %v", path, err)
	}
	if doc := s.ws.Document(name); len(doc.ParseDiagnostics) > 0 {
		if len(edits) != 0 {
			t.Fatalf("%s: edits = %d for a document with parse errors, want none", path, len(edits))
		}
		return
	}
	want, err := format.Source(name, []byte(src), format.DefaultOptions)
	if err != nil {
		t.Fatalf("%s: format.Source: %v", path, err)
	}
	if string(want) == src && len(edits) != 0 {
		t.Fatalf("%s: edits = %d for an already formatted document, want none", path, len(edits))
	}
	if got := applyOrderedEdits(t, src, edits); got != string(want) {
		t.Fatalf("%s: applying %d edits gives\n%s\nwant\n%s", path, len(edits), got, want)
	}
}

// Computing the edits costs far less than the formatter's own pass, even on the
// largest library file with every line changed.
func TestFormattingDiffIsCheaperThanFormatting(t *testing.T) {
	src := libs.DefaultSource()
	var largest []byte
	for _, name := range src.List() {
		content, err := src.Read(name)
		if err != nil {
			t.Fatal(err)
		}
		if len(content) > len(largest) {
			largest = content
		}
	}
	mangled := []byte(mangleIndentation(string(largest)))
	formatted, err := format.Source("largest.sysml", mangled, format.DefaultOptions)
	if err != nil {
		t.Fatal(err)
	}
	fastest := func(f func()) time.Duration {
		best := time.Duration(1<<63 - 1)
		for i := 0; i < 5; i++ {
			start := time.Now()
			f()
			if d := time.Since(start); d < best {
				best = d
			}
		}
		return best
	}
	formatTime := fastest(func() { _, _ = format.Source("largest.sysml", mangled, format.DefaultOptions) })
	diffTime := fastest(func() { formatEdits(mangled, formatted) })
	t.Logf("%d bytes: format %v, diff %v", len(mangled), formatTime, diffTime)
	if diffTime > formatTime {
		t.Fatalf("diff took %v, formatter %v; the diff must not dominate", diffTime, formatTime)
	}
	edits := formatEdits(mangled, formatted)
	if got := applyOrderedEdits(t, string(mangled), edits); got != string(formatted) {
		t.Fatal("edits do not reproduce the formatter output")
	}
}

func BenchmarkFormatEdits(b *testing.B) {
	content, err := os.ReadFile("../repl/testdata/compile_calcs.sysml")
	if err != nil {
		b.Fatal(err)
	}
	mangled := []byte(mangleIndentation(string(content)))
	formatted, err := format.Source("b.sysml", mangled, format.DefaultOptions)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		formatEdits(mangled, formatted)
	}
}

const rangeSource = "package P {\n    part def Car;\n  part def Bus;\n    part def Van;\n    part def Bike;\n}\n"

// A range over one mis-indented line yields exactly that line's edit.
func TestRangeFormattingSingleBadLine(t *testing.T) {
	edits := rangeEditsFor(t, rangeSource, lines(2, 3))
	want := protocol.TextEdit{Range: protocol.Range{Start: pos(2, 2), End: pos(2, 2)}, NewText: "  "}
	if len(edits) != 1 || edits[0] != want {
		t.Fatalf("edits = %+v, want [%+v]", edits, want)
	}
}

// A range over well-formatted lines yields nothing, even when the rest of the
// file needs work.
func TestRangeFormattingCleanRegion(t *testing.T) {
	if edits := rangeEditsFor(t, rangeSource, lines(3, 5)); len(edits) != 0 {
		t.Fatalf("edits = %+v, want none", edits)
	}
	if edits := rangeEditsFor(t, rangeSource, protocol.Range{Start: pos(0, 0), End: pos(1, 17)}); len(edits) != 0 {
		t.Fatalf("edits = %+v, want none", edits)
	}
}

// A range spanning the whole file yields the same edits as Formatting.
func TestRangeFormattingWholeFileMatchesFormatting(t *testing.T) {
	src := mangleIndentation(rangeSource)
	full := formatEditsFor(t, src, spaces4)
	if len(full) < 2 {
		t.Fatalf("full formatting produced %d edits, want several to compare", len(full))
	}
	end := offsetToPosition([]byte(src), len(src))
	ranged := rangeEditsFor(t, src, protocol.Range{Start: pos(0, 0), End: end})
	if len(ranged) != len(full) {
		t.Fatalf("range edits = %+v, want %+v", ranged, full)
	}
	for i := range full {
		if ranged[i] != full[i] {
			t.Fatalf("range edit %d = %+v, want %+v", i, ranged[i], full[i])
		}
	}
}

// A partial-line selection widens to its whole lines: a cursor in the middle
// of the bad line still fixes it, and a selection ending at column 0 of the
// next line does not reach into that line.
func TestRangeFormattingWidensToWholeLines(t *testing.T) {
	src := "package P {\n  part def Car;\n  part def Bus;\n}\n"
	edits := rangeEditsFor(t, src, protocol.Range{Start: pos(1, 5), End: pos(1, 5)})
	if len(edits) != 1 || edits[0].Range.Start.Line != 1 {
		t.Fatalf("edits = %+v, want the edit for line 1 only", edits)
	}
	edits = rangeEditsFor(t, src, protocol.Range{Start: pos(1, 5), End: pos(2, 0)})
	if len(edits) != 1 || edits[0].Range.Start.Line != 1 {
		t.Fatalf("edits = %+v, want the edit for line 1 only", edits)
	}
	edits = rangeEditsFor(t, src, protocol.Range{Start: pos(1, 5), End: pos(2, 1)})
	if len(edits) != 2 {
		t.Fatalf("edits = %+v, want the edits for lines 1 and 2", edits)
	}
}

// Indentation inside the range follows the whole file's structure, so a
// selection deep in a nested block is indented to its real depth.
func TestRangeFormattingUsesWholeFileStructure(t *testing.T) {
	src := "package P {\n    part def Car {\n        part def Wheel {\nattribute n;\n        }\n    }\n}\n"
	edits := rangeEditsFor(t, src, lines(3, 4))
	want := protocol.TextEdit{Range: protocol.Range{Start: pos(3, 0), End: pos(3, 0)}, NewText: "            "}
	if len(edits) != 1 || edits[0] != want {
		t.Fatalf("edits = %+v, want [%+v]", edits, want)
	}
}

// A deletion that spans lines is kept when any of those lines is selected.
func TestRangeFormattingKeepsMultiLineEditTouchingRange(t *testing.T) {
	src := "package P {\n    part def Car;\n\n\n\n    part def Bus;\n}\n"
	full := formatEditsFor(t, src, spaces4)
	if len(full) != 1 {
		t.Fatalf("full edits = %+v, want one deletion", full)
	}
	if edits := rangeEditsFor(t, src, lines(4, 5)); len(edits) != 1 || edits[0] != full[0] {
		t.Fatalf("edits = %+v, want %+v", edits, full)
	}
	if edits := rangeEditsFor(t, src, lines(0, 1)); len(edits) != 0 {
		t.Fatalf("edits = %+v, want none outside the blank run", edits)
	}
}

func TestRangeFormattingSkipsDocumentThatDoesNotParse(t *testing.T) {
	if edits := rangeEditsFor(t, "package P { part def\n", lines(0, 1)); len(edits) != 0 {
		t.Fatalf("edits = %v, want none for an unparseable document", edits)
	}
}

func TestRangeFormattingUnknownDocument(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	edits, err := s.RangeFormatting(context.Background(), &protocol.DocumentRangeFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File("/tmp/missing.sysml")},
		Range:        lines(0, 1),
		Options:      spaces4,
	})
	if err != nil || edits != nil {
		t.Fatalf("RangeFormatting = %v, %v; want nil, nil", edits, err)
	}
}

// A formatter that declines with ErrNotIdempotent leaves the document alone,
// for both requests, while any other failure is reported.
func TestFormattingHonoursNotIdempotentGuard(t *testing.T) {
	real := formatSource
	t.Cleanup(func() { formatSource = real })

	formatSource = func(name string, src []byte, opts format.Options) ([]byte, error) {
		return src, format.ErrNotIdempotent
	}
	if edits := formatEditsFor(t, "package P {\npart def Car;\n}\n", spaces4); edits != nil {
		t.Fatalf("Formatting = %+v, want nil on ErrNotIdempotent", edits)
	}
	if edits := rangeEditsFor(t, "package P {\npart def Car;\n}\n", lines(0, 3)); edits != nil {
		t.Fatalf("RangeFormatting = %+v, want nil on ErrNotIdempotent", edits)
	}

	boom := errors.New("boom")
	formatSource = func(name string, src []byte, opts format.Options) ([]byte, error) { return nil, boom }
	s, name := openFormatDoc(t, "package P {\npart def Car;\n}\n")
	if _, err := s.Formatting(context.Background(), &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)}, Options: spaces4,
	}); !errors.Is(err, boom) {
		t.Fatalf("Formatting err = %v, want %v", err, boom)
	}
	if _, err := s.RangeFormatting(context.Background(), &protocol.DocumentRangeFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)}, Range: lines(0, 3), Options: spaces4,
	}); !errors.Is(err, boom) {
		t.Fatalf("RangeFormatting err = %v, want %v", err, boom)
	}
}
