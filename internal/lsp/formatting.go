package lsp

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/core/format"
)

// formatSource is the formatter behind both formatting requests; tests swap it
// to exercise the error paths the real one only takes on a formatter bug.
var formatSource = format.Source

// Formatting re-indents the whole document as one edit per changed region, so
// the editor keeps its undo history, cursor and folds.
func (s *Server) Formatting(ctx context.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	return s.formattingEdits(params.TextDocument.URI, params.Options)
}

// RangeFormatting formats the whole document as Formatting does and keeps only
// the edits touching the requested lines, so nothing outside them moves.
func (s *Server) RangeFormatting(ctx context.Context, params *protocol.DocumentRangeFormattingParams) ([]protocol.TextEdit, error) {
	edits, err := s.formattingEdits(params.TextDocument.URI, params.Options)
	if err != nil || len(edits) == 0 {
		return nil, err
	}
	first, last := lineSpan(params.Range)
	var kept []protocol.TextEdit
	for _, edit := range edits {
		if from, to := lineSpan(edit.Range); from <= last && to >= first {
			kept = append(kept, edit)
		}
	}
	return kept, nil
}

// formattingEdits returns the edits that turn the named document into its
// formatted form, or none when there is nothing to do. A document that does not
// parse is left alone: its brace structure is unreliable, and reformatting a
// file the author is midway through editing is worse than doing nothing.
func (s *Server) formattingEdits(uri protocol.DocumentURI, opts protocol.FormattingOptions) ([]protocol.TextEdit, error) {
	name := uriToName(uri)
	doc := s.ws.Document(name)
	if doc == nil || len(doc.ParseDiagnostics) > 0 {
		return nil, nil
	}
	out, err := formatSource(name, doc.Content, formatOptions(opts))
	if errors.Is(err, format.ErrNotIdempotent) {
		// The formatter declined to rewrite the file. That is its bug, not the
		// author's, so leave the document alone rather than raise it in the editor.
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if bytes.Equal(out, doc.Content) {
		return nil, nil
	}
	return formatEdits(doc.Content, out), nil
}

// lineSpan returns the first and last line a range touches; a range ending at
// column 0 of a later line stops before that line.
func lineSpan(r protocol.Range) (first, last uint32) {
	first, last = r.Start.Line, r.End.Line
	if last > first && r.End.Character == 0 {
		last--
	}
	if last < first {
		last = first
	}
	return first, last
}

// formatOptions maps the client's editor settings onto the formatter's.
func formatOptions(opts protocol.FormattingOptions) format.Options {
	out := format.Options{IndentWidth: int(opts.TabSize), UseTabs: !opts.InsertSpaces}
	if out.IndentWidth <= 0 {
		out.IndentWidth = format.DefaultOptions.IndentWidth
	}
	return out
}

// formatEdits returns the ordered, non-overlapping edits that turn content into
// formatted. Lines are matched on their non-whitespace text, since that is all
// the formatter preserves, and each changed line becomes one edit.
func formatEdits(content, formatted []byte) []protocol.TextEdit {
	oldLines, oldOffsets := cutLines(content)
	newLines, _ := cutLines(formatted)
	table := newLineTable(content, oldOffsets)

	var edits []protocol.TextEdit
	replace := func(start, end int, text string) {
		edits = append(edits, protocol.TextEdit{
			Range:   protocol.Range{Start: table.position(start), End: table.position(end)},
			NewText: text,
		})
	}
	// Lines the diff pairs up may still differ in whitespace; edit each one.
	pairLines := func(oldFrom, oldTo, newFrom int) {
		for i, j := oldFrom, newFrom; i < oldTo; i, j = i+1, j+1 {
			if start, end, text, changed := trimCommon(oldLines[i], newLines[j]); changed {
				replace(oldOffsets[i]+start, oldOffsets[i]+end, text)
			}
		}
	}
	oldNext, newNext := 0, 0
	keys := lineKeys{ids: make(map[string]int, len(oldLines))}
	hunks := diffLines(keys.of(oldLines, len(content)), keys.of(newLines, len(formatted)))
	for _, h := range append(hunks, hunk{len(oldLines), len(oldLines), len(newLines), len(newLines)}) {
		pairLines(oldNext, h.oldStart, newNext)
		oldNext, newNext = h.oldEnd, h.newEnd
		if h.oldEnd-h.oldStart == h.newEnd-h.newStart {
			pairLines(h.oldStart, h.oldEnd, h.newStart)
			continue
		}
		oldText := string(content[oldOffsets[h.oldStart]:oldOffsets[h.oldEnd]])
		newText := strings.Join(newLines[h.newStart:h.newEnd], "")
		if start, end, text, changed := trimCommon(oldText, newText); changed {
			replace(oldOffsets[h.oldStart]+start, oldOffsets[h.oldStart]+end, text)
		}
	}
	return edits
}

// cutLines splits text after each newline, keeping terminators so the lines
// concatenate back to text; offsets[i] is where lines[i] starts, offsets[len(lines)] is len(text).
func cutLines(text []byte) (lines []string, offsets []int) {
	s := string(text)
	offsets = append(offsets, 0)
	for start := 0; start < len(s); {
		end := strings.IndexByte(s[start:], '\n')
		if end < 0 {
			end = len(s)
		} else {
			end += start + 1
		}
		lines = append(lines, s[start:end])
		offsets = append(offsets, end)
		start = end
	}
	return lines, offsets
}

// lineKeys numbers lines by their text with whitespace removed: two lines get
// the same id exactly when they differ only in whitespace.
type lineKeys struct {
	ids map[string]int
}

// of returns the ids of lines, whose total length is size.
func (k *lineKeys) of(lines []string, size int) []int {
	// Strip every line into one buffer, then intern substrings of it.
	var stripped strings.Builder
	stripped.Grow(size)
	ends := make([]int, len(lines))
	for i, line := range lines {
		for j := 0; j < len(line); j++ {
			switch c := line[j]; c {
			case ' ', '\t', '\r', '\n', '\f', '\v':
			default:
				stripped.WriteByte(c)
			}
		}
		ends[i] = stripped.Len()
	}
	all := stripped.String()
	out := make([]int, len(lines))
	start := 0
	for i, end := range ends {
		key := all[start:end]
		start = end
		id, ok := k.ids[key]
		if !ok {
			id = len(k.ids)
			k.ids[key] = id
		}
		out[i] = id
	}
	return out
}

// trimCommon strips the shared prefix and suffix on rune boundaries and reports
// the byte range of old that remains and the text replacing it.
func trimCommon(old, new string) (start, end int, text string, changed bool) {
	if old == new {
		return 0, 0, "", false
	}
	start = 0
	for start < len(old) && start < len(new) && old[start] == new[start] {
		start++
	}
	for start > 0 && ((start < len(old) && !utf8.RuneStart(old[start])) || (start < len(new) && !utf8.RuneStart(new[start]))) {
		start--
	}
	end, newEnd := len(old), len(new)
	for end > start && newEnd > start && old[end-1] == new[newEnd-1] {
		end--
		newEnd--
	}
	for (end < len(old) && !utf8.RuneStart(old[end])) || (newEnd < len(new) && !utf8.RuneStart(new[newEnd])) {
		end++
		newEnd++
	}
	return start, end, new[start:newEnd], true
}

// lineTable converts byte offsets to protocol positions in O(log lines) each.
type lineTable struct {
	content []byte
	starts  []int // byte offset of each line start
}

func newLineTable(content []byte, offsets []int) lineTable {
	starts := offsets
	if n := len(content); n > 0 && content[n-1] != '\n' {
		starts = offsets[:len(offsets)-1]
	}
	return lineTable{content: content, starts: starts}
}

// position converts a byte offset to a position exactly as offsetToPosition does.
func (t lineTable) position(offset int) protocol.Position {
	line := sort.SearchInts(t.starts, offset+1) - 1
	if line < 0 {
		line = 0
	}
	return protocol.Position{
		Line:      uint32Clamp(line),
		Character: uint32Clamp(utf16Len(t.content[t.starts[line]:offset])),
	}
}
