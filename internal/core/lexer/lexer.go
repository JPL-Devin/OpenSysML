package lexer

import "github.com/Open-MBEE/Systemica/internal/core/source"

// Lexer is a hand-written, pull-based scanner. Call Next() repeatedly.
type Lexer struct {
	sf    *source.SourceFile
	src   []byte
	pos   int // current byte offset
	atEOF bool
}

// New creates a Lexer over a source file.
func New(sf *source.SourceFile) *Lexer {
	return &Lexer{sf: sf, src: sf.Bytes()}
}

// Next returns the next token, including trivia tokens. At end of input it
// returns EOF repeatedly (idempotent).
func (lx *Lexer) Next() Token {
	if lx.pos >= len(lx.src) {
		lx.atEOF = true
		return Token{Kind: EOF, Span: source.Span{Offset: len(lx.src), Len: 0}}
	}
	start := lx.pos
	c := lx.src[lx.pos]
	_ = c
	// Dispatch is filled in subsequent tasks. For now, emit a single-byte
	// Error token so the loop always advances and terminates.
	lx.pos++
	return Token{Kind: Error, Span: source.Span{Offset: start, Len: 1}}
}

// peek returns the byte at pos+n without advancing, or 0 if out of range.
func (lx *Lexer) peek(n int) byte {
	i := lx.pos + n
	if i >= len(lx.src) {
		return 0
	}
	return lx.src[i]
}

// span builds a Span from a start offset to the current pos.
func (lx *Lexer) span(start int) source.Span {
	return source.Span{Offset: start, Len: lx.pos - start}
}
