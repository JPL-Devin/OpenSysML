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

	switch {
	case c == ' ' || c == '\t' || c == '\r' || c == '\n':
		return lx.scanWhitespace(start)
	case c == '/' && lx.peek(1) == '/' && lx.peek(2) == '*':
		return lx.scanMLNote(start) // //* ... */
	case c == '/' && lx.peek(1) == '/':
		return lx.scanSLNote(start) // // ...
	case c == '/' && lx.peek(1) == '*':
		return lx.scanBlockComment(start) // /* ... */
	case isIdentStart(c):
		return lx.scanIdentOrKeyword(start)
	case c == '\'':
		return lx.scanQuoted(start, '\'', UnrestrictedName)
	}

	// Not trivia: emit a single-byte Error for now; later tasks add cases
	// BEFORE this fallthrough point.
	lx.pos++
	return Token{Kind: Error, Span: source.Span{Offset: start, Len: 1}}
}

func (lx *Lexer) scanWhitespace(start int) Token {
	for lx.pos < len(lx.src) {
		c := lx.src[lx.pos]
		if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
			break
		}
		lx.pos++
	}
	return Token{Kind: Whitespace, Span: lx.span(start)}
}

func (lx *Lexer) scanSLNote(start int) Token {
	lx.pos += 2 // consume "//"
	for lx.pos < len(lx.src) && lx.src[lx.pos] != '\n' && lx.src[lx.pos] != '\r' {
		lx.pos++
	}
	// SL_NOTE includes the trailing line terminator: KerMLExpressions.xtext
	// SL_NOTE: '//' (...)? ('\r'? '\n')?  -- so \r?\n is part of the token span.
	if lx.pos < len(lx.src) && lx.src[lx.pos] == '\r' {
		lx.pos++
	}
	if lx.pos < len(lx.src) && lx.src[lx.pos] == '\n' {
		lx.pos++
	}
	return Token{Kind: SLNote, Span: lx.span(start)}
}

func (lx *Lexer) scanMLNote(start int) Token {
	lx.pos += 3 // consume "//*"
	lx.consumeUntilStarSlash()
	return Token{Kind: MLNote, Span: lx.span(start)}
}

func (lx *Lexer) scanBlockComment(start int) Token {
	lx.pos += 2 // consume "/*"
	lx.consumeUntilStarSlash()
	return Token{Kind: RegularComment, Span: lx.span(start)}
}

// scanQuoted scans a quoted literal delimited by quote (' or "), honoring the
// backslash escape set. On success emits kind; on unterminated (newline/EOF
// before closing quote) emits Error covering what was consumed.
func (lx *Lexer) scanQuoted(start int, quote byte, kind Kind) Token {
	lx.pos++ // opening quote
	for lx.pos < len(lx.src) {
		c := lx.src[lx.pos]
		switch {
		case c == '\\':
			// escape: consume backslash + next char if present
			lx.pos++
			if lx.pos < len(lx.src) {
				lx.pos++
			}
		case c == quote:
			lx.pos++ // closing quote
			return Token{Kind: kind, Span: lx.span(start)}
		case c == '\n' || c == '\r':
			// unterminated on this line
			return Token{Kind: Error, Span: lx.span(start)}
		default:
			lx.pos++
		}
	}
	// reached EOF without closing quote
	return Token{Kind: Error, Span: lx.span(start)}
}

// consumeUntilStarSlash advances until it consumes a closing "*/", or to EOF
// if unterminated.
func (lx *Lexer) consumeUntilStarSlash() {
	for lx.pos < len(lx.src) {
		if lx.src[lx.pos] == '*' && lx.peek(1) == '/' {
			lx.pos += 2
			return
		}
		lx.pos++
	}
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentCont(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

func (lx *Lexer) scanIdentOrKeyword(start int) Token {
	lx.pos++ // first char already known to be identStart
	for lx.pos < len(lx.src) && isIdentCont(lx.src[lx.pos]) {
		lx.pos++
	}
	sp := lx.span(start)
	text := string(lx.src[start:lx.pos])
	if _, ok := keywords[text]; ok {
		return Token{Kind: Keyword, Span: sp, KeywordID: text}
	}
	return Token{Kind: Identifier, Span: sp}
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
