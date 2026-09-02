package export

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/format"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// formattedSource maps spans of a parsed document onto the formatter's output
// for it, which is the notation the graph carries as sysx:sourceText.
type formattedSource struct {
	text []byte
	// The significant tokens of the parsed source and of text, in step: the
	// formatter keeps every lexeme and rewrites only the whitespace between.
	kinds      []lexer.Kind
	orig, fmtd []source.Span
}

func newFormattedSource(file *source.SourceFile) (*formattedSource, error) {
	text, err := format.Source(file.Name(), file.Bytes(), format.DefaultOptions)
	if err != nil {
		return nil, fmt.Errorf("formatting %s: %w", file.Name(), err)
	}
	s := &formattedSource{text: text}
	s.kinds, s.orig = significantTokens(file)
	_, s.fmtd = significantTokens(source.New(file.Name(), text))
	if len(s.orig) != len(s.fmtd) {
		return nil, fmt.Errorf("formatting %s: %w", file.Name(), format.ErrNotIdempotent)
	}
	return s, nil
}

func significantTokens(file *source.SourceFile) ([]lexer.Kind, []source.Span) {
	lx := lexer.New(file)
	var kinds []lexer.Kind
	var spans []source.Span
	for {
		tok := lx.Next()
		if tok.Kind == lexer.EOF {
			return kinds, spans
		}
		if tok.Kind != lexer.Whitespace {
			kinds = append(kinds, tok.Kind)
			spans = append(spans, tok.Span)
		}
	}
}

// index returns the position of the token holding an offset of the parsed
// source, or of the token before that offset when it falls between two.
func (s *formattedSource) index(offset int) int {
	return sort.Search(len(s.orig), func(i int) bool { return s.orig[i].Offset > offset }) - 1
}

// at maps an offset of the parsed source into text. An offset inside a token
// keeps its place in that token; one between tokens lands where the next starts.
func (s *formattedSource) at(offset int) int {
	k := s.index(offset)
	if k >= 0 && offset <= s.orig[k].End() {
		if offset == s.orig[k].End() {
			return s.fmtd[k].End()
		}
		return min(s.fmtd[k].Offset+offset-s.orig[k].Offset, s.fmtd[k].End())
	}
	if k+1 < len(s.fmtd) {
		return s.fmtd[k+1].Offset
	}
	return len(s.text)
}

// slice returns the formatted text of a span of the parsed source.
func (s *formattedSource) slice(span source.Span) string {
	return string(s.text[s.at(span.Offset):s.at(span.End())])
}

// region is a range of the formatted text, from start up to end.
type region struct{ start, end int }

// tile returns the contiguous lines of each member of one body: from the end
// of the one before (the first, from the trivia ahead of it) to its last line.
func (s *formattedSource) tile(spans []source.Span) []region {
	out := make([]region, len(spans))
	for i, span := range spans {
		first := sort.Search(len(s.orig), func(k int) bool { return s.orig[k].Offset >= span.Offset })
		// A member's span runs on over the trivia after it, up to the next token.
		last := s.index(span.End() - 1)
		for last > first && s.trivia(last) {
			last--
		}
		next := last + 1
		for next < len(s.kinds) && s.trivia(next) && !s.newlineBefore(next) {
			next++
		}
		end := len(s.text)
		if next < len(s.kinds) {
			end = s.lineStart(s.fmtd[next].Offset)
		}
		var start int
		if i > 0 {
			start = out[i-1].end
		} else {
			for first > 0 && s.trivia(first-1) && s.newlineBefore(first-1) {
				first--
			}
			start = s.lineStart(s.fmtd[first].Offset)
		}
		out[i] = region{start, max(start, end)}
	}
	return out
}

// split returns the text of an element apart from its members: the lines up to
// the first member, and those after the last.
func (s *formattedSource) split(own, members region) (head, tail string) {
	return string(s.text[own.start:members.start]), string(s.text[members.end:own.end])
}

func (s *formattedSource) region(r region) string { return string(s.text[r.start:r.end]) }

// newlineBefore reports whether token k starts on a later line than token k-1.
func (s *formattedSource) newlineBefore(k int) bool {
	if k == 0 {
		return true
	}
	prev := s.fmtd[k-1]
	return s.kinds[k-1] == lexer.SLNote || bytes.IndexByte(s.text[prev.End():s.fmtd[k].Offset], '\n') >= 0
}

// lineStart returns the offset of the indentation before pos on its line, or
// pos itself when something other than blanks precedes it there.
func (s *formattedSource) lineStart(pos int) int {
	for pos > 0 && (s.text[pos-1] == ' ' || s.text[pos-1] == '\t') {
		pos--
	}
	return pos
}

// trivia reports whether token k is one the parser skips: a note, or a `/* */`
// comment not following a declaration head, which would make it its body.
func (s *formattedSource) trivia(k int) bool {
	switch s.kinds[k] {
	case lexer.SLNote, lexer.MLNote:
		return true
	case lexer.RegularComment:
		if k == 0 {
			return true
		}
		switch s.kinds[k-1] {
		case lexer.Semicolon, lexer.LBrace, lexer.RBrace, lexer.SLNote, lexer.MLNote, lexer.RegularComment:
			return true
		}
	}
	return false
}
