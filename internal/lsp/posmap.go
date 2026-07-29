package lsp

import (
	"unicode/utf8"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// offsetToPosition converts a byte offset in content to a 0-based LSP Position
// whose Character is a UTF-16 code-unit column.
func offsetToPosition(content []byte, offset int) protocol.Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(content) {
		offset = len(content)
	}
	line := 0
	lineStart := 0
	for i := 0; i < offset; i++ {
		if content[i] == '\n' {
			line++
			lineStart = i + 1
		}
	}
	char := utf16Len(content[lineStart:offset])
	return protocol.Position{Line: uint32(line), Character: uint32(char)}
}

// positionToOffset converts a 0-based LSP Position (UTF-16 column) to a byte
// offset in content. Out-of-range positions clamp to the end of content.
func positionToOffset(content []byte, pos protocol.Position) int {
	line := 0
	i := 0
	for line < int(pos.Line) && i < len(content) {
		if content[i] == '\n' {
			line++
		}
		i++
	}
	// i is now the byte offset of the start of the target line.
	units := 0
	for i < len(content) && content[i] != '\n' {
		if units >= int(pos.Character) {
			break
		}
		r, size := utf8.DecodeRune(content[i:])
		units += utf16RuneLen(r)
		i += size
	}
	return i
}

// spanToRange converts a core byte Span to an LSP Range.
func spanToRange(content []byte, sp source.Span) protocol.Range {
	return protocol.Range{
		Start: offsetToPosition(content, sp.Offset),
		End:   offsetToPosition(content, sp.End()),
	}
}

// rangeToSpan converts an LSP Range to a core byte Span.
func rangeToSpan(content []byte, r protocol.Range) source.Span {
	start := positionToOffset(content, r.Start)
	end := positionToOffset(content, r.End)
	if end < start {
		end = start
	}
	return source.Span{Offset: start, Len: end - start}
}

// utf16Len returns the number of UTF-16 code units in b.
func utf16Len(b []byte) int {
	n := 0
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		n += utf16RuneLen(r)
		i += size
	}
	return n
}

// utf16RuneLen returns the number of UTF-16 code units for r: 2 for
// astral-plane runes (> U+FFFF, encoded as a surrogate pair), else 1.
func utf16RuneLen(r rune) int {
	if r > 0xFFFF {
		return 2
	}
	return 1
}
