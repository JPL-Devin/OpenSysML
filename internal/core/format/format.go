// Package format re-indents SysML v2 / KerML source text.
//
// The formatter works on the token stream rather than on the AST. Printing an
// AST back to text would drop everything the AST does not model — comments,
// notes, and any construct the parser records as an ErrorNode — so a formatter
// built that way cannot safely be pointed at a user's file. Working on tokens
// keeps every lexeme and only rewrites the whitespace between them.
//
// The rules are deliberately conservative: line breaks the author wrote are
// preserved, so the formatter never joins or splits a line. It fixes
// indentation, trailing whitespace, runs of blank lines, and the padding just
// inside brackets and before separators.
package format

import (
	"bytes"
	"errors"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/lexer"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// Options controls the emitted indentation.
type Options struct {
	// IndentWidth is the number of columns per nesting level (default 4).
	IndentWidth int
	// UseTabs indents with one tab per level instead of spaces.
	UseTabs bool
}

// DefaultOptions matches the indentation used by the models in examples/.
var DefaultOptions = Options{IndentWidth: 4}

// ErrNotIdempotent reports that formatting would have changed the token stream.
// It signals a bug in the formatter, never bad input: callers should present the
// source unchanged rather than a mangled file.
var ErrNotIdempotent = errors.New("format: rewrite would change the token stream")

// Source formats src, which must be the contents of name. It returns the
// formatted bytes, or ErrNotIdempotent (with src unchanged) if the result would
// not lex back to the same tokens.
func Source(name string, src []byte, opts Options) ([]byte, error) {
	if opts.IndentWidth <= 0 {
		opts.IndentWidth = DefaultOptions.IndentWidth
	}
	out := (&formatter{src: src, opts: opts}).run(tokenize(name, src))
	if !sameTokens(name, src, out) {
		return src, ErrNotIdempotent
	}
	return out, nil
}

func tokenize(name string, src []byte) []lexer.Token {
	lx := lexer.New(source.New(name, src))
	var toks []lexer.Token
	for {
		tok := lx.Next()
		if tok.Kind == lexer.EOF {
			return toks
		}
		toks = append(toks, tok)
	}
}

type formatter struct {
	src  []byte
	opts Options

	buf   bytes.Buffer
	depth int

	atLineStart bool // nothing written on the current line yet
	pendingNL   int  // newlines seen since the last emitted token
	pendingWS   bool // same-line whitespace seen since the last emitted token
	prev        lexer.Token
	started     bool
}

func (f *formatter) run(toks []lexer.Token) []byte {
	f.atLineStart = true
	for _, tok := range toks {
		if tok.Kind == lexer.Whitespace {
			text := f.text(tok)
			if n := strings.Count(text, "\n"); n > 0 {
				f.pendingNL += n
			} else {
				f.pendingWS = true
			}
			continue
		}
		f.emit(tok)
	}
	if f.started {
		f.buf.WriteByte('\n')
	}
	return f.buf.Bytes()
}

// emit writes one non-whitespace token, together with the whitespace that
// should precede it.
func (f *formatter) emit(tok lexer.Token) {
	if tok.Kind == lexer.RBrace {
		f.depth--
		if f.depth < 0 {
			f.depth = 0
		}
	}

	switch {
	case f.pendingNL > 0:
		if f.started {
			// At most one blank line between tokens.
			f.buf.WriteByte('\n')
			if f.pendingNL > 1 {
				f.buf.WriteByte('\n')
			}
		}
		f.atLineStart = true
	case f.pendingWS && f.started && !f.atLineStart && f.wantSpace(tok):
		f.buf.WriteByte(' ')
	}
	f.pendingNL, f.pendingWS = 0, false

	if f.atLineStart {
		f.writeIndent()
	}
	text := f.text(tok)
	f.buf.WriteString(text)

	if tok.Kind == lexer.LBrace {
		f.depth++
	}
	f.prev, f.started, f.atLineStart = tok, true, false
	if i := strings.LastIndexByte(text, '\n'); i >= 0 {
		// A multi-line note or block comment is emitted verbatim; the tokens
		// after it start from wherever it ended.
		f.atLineStart = strings.TrimSpace(text[i+1:]) == ""
	}
}

// wantSpace reports whether whitespace the author wrote before tok is kept.
// Only pairs that cannot lex differently once joined are tightened, so removing
// the space can never merge two tokens into one.
func (f *formatter) wantSpace(tok lexer.Token) bool {
	switch tok.Kind {
	case lexer.Semicolon, lexer.Comma, lexer.RParen, lexer.RBracket:
		return false
	}
	switch f.prev.Kind {
	case lexer.LParen, lexer.LBracket:
		return false
	}
	return true
}

func (f *formatter) writeIndent() {
	if f.opts.UseTabs {
		f.buf.WriteString(strings.Repeat("\t", f.depth))
		return
	}
	f.buf.WriteString(strings.Repeat(" ", f.depth*f.opts.IndentWidth))
}

func (f *formatter) text(tok lexer.Token) string {
	return string(f.src[tok.Span.Offset:tok.Span.End()])
}

// sameTokens reports whether before and after lex to the same sequence of
// non-whitespace tokens with the same text: the formatter's safety net.
func sameTokens(name string, before, after []byte) bool {
	a, b := significant(name, before), significant(name, after)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func significant(name string, src []byte) []string {
	toks := tokenize(name, src)
	out := make([]string, 0, len(toks))
	for _, tok := range toks {
		if tok.Kind == lexer.Whitespace {
			continue
		}
		out = append(out, tok.Kind.String()+" "+string(src[tok.Span.Offset:tok.Span.End()]))
	}
	return out
}
