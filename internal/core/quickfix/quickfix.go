// Package quickfix carries the source edits that resolve a diagnostic, attached
// by the layer that reported it and rendered by an editor as a quick fix.
package quickfix

import "github.com/Open-MBEE/Systemica/internal/core/source"

// Edit replaces the text a span covers. A zero-length span is an insertion at
// its offset.
type Edit struct {
	Span    source.Span
	NewText string
	// OwnLine makes the edit a whole line inserted before Span.Offset, indented
	// like the line that offset sits on (see Render).
	OwnLine bool
}

// Fix is one unambiguous way to resolve a diagnostic: a title to offer it under
// and the edits applying it, all within the document the diagnostic belongs to.
// Preferred marks the fix an editor may apply without asking.
type Fix struct {
	Title     string
	Edits     []Edit
	Preferred bool
}

// Insert returns an insertion of text at offset.
func Insert(offset int, text string) Edit {
	return Edit{Span: source.Span{Offset: offset}, NewText: text}
}

// Replace returns a replacement of the text span covers with text.
func Replace(span source.Span, text string) Edit {
	return Edit{Span: span, NewText: text}
}

// InsertLine returns an insertion of text as its own line before offset.
func InsertLine(offset int, text string) Edit {
	return Edit{Span: source.Span{Offset: offset}, NewText: text, OwnLine: true}
}

// Render resolves an edit against the document it applies to, returning the span
// to replace and the text to replace it with. An own-line edit is terminated by
// a newline and followed by the indentation of the line it is inserted before,
// so the declaration that line holds keeps its place.
func (e Edit) Render(content []byte) (source.Span, string) {
	if !e.OwnLine {
		return e.Span, e.NewText
	}
	return e.Span, e.NewText + "\n" + indentAt(content, e.Span.Offset)
}

// indentAt returns the leading whitespace of the line offset sits on.
func indentAt(content []byte, offset int) string {
	if offset < 0 || offset > len(content) {
		return ""
	}
	start := offset
	for start > 0 && content[start-1] != '\n' {
		start--
	}
	end := start
	for end < len(content) && (content[end] == ' ' || content[end] == '\t') {
		end++
	}
	if end > offset {
		end = offset
	}
	return string(content[start:end])
}
