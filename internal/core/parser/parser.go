package parser

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// Parser is a hand-written recursive-descent parser over a lexer token
// stream. It buffers non-trivia tokens for lookahead and collects
// diagnostics; it always produces a tree (ErrorNodes for bad input).
type Parser struct {
	src         *source.SourceFile
	lx          *lexer.Lexer
	buf         []lexer.Token // lookahead ring of non-trivia tokens
	triv        []ast.Trivia  // trivia pending attachment to the next node
	Diagnostics []Diagnostic

	pendingComment    source.Span // span of the most recent /* */ regular comment
	hasPendingComment bool
}

// New creates a Parser for the given source file.
func New(sf *source.SourceFile) *Parser {
	return &Parser{src: sf, lx: lexer.New(sf)}
}

// fill ensures buf has at least n+1 tokens (pulling from the lexer, skipping
// and recording trivia). The final EOF token is sticky (re-returned).
func (p *Parser) fill(n int) {
	for len(p.buf) <= n {
		tok := p.lx.Next()
		for tok.IsTrivia() || tok.Kind == lexer.RegularComment {
			p.triv = append(p.triv, triviaOf(tok))
			if tok.Kind == lexer.RegularComment {
				p.pendingComment = tok.Span
				p.hasPendingComment = true
			}
			if tok.Kind == lexer.EOF {
				break
			}
			tok = p.lx.Next()
		}
		p.buf = append(p.buf, tok)
		if tok.Kind == lexer.EOF {
			// keep EOF sticky: stop growing further with real tokens
			return
		}
	}
}

func triviaOf(tok lexer.Token) ast.Trivia {
	var k ast.TriviaKind
	switch tok.Kind {
	case lexer.SLNote:
		k = ast.TriviaLineNote
	case lexer.MLNote:
		k = ast.TriviaBlockNote
	case lexer.RegularComment:
		k = ast.TriviaComment
	default:
		k = ast.TriviaWhitespace
	}
	return ast.Trivia{Kind: k, Span: tok.Span}
}

// peek returns the current non-trivia token without consuming it.
func (p *Parser) peek() lexer.Token { return p.peekN(0) }

// peekN returns the token n positions ahead (0 = current).
func (p *Parser) peekN(n int) lexer.Token {
	p.fill(n)
	if n >= len(p.buf) {
		return p.buf[len(p.buf)-1] // EOF (sticky)
	}
	return p.buf[n]
}

// advance consumes and returns the current token.
func (p *Parser) advance() lexer.Token {
	p.fill(0)
	tok := p.buf[0]
	if tok.Kind != lexer.EOF {
		p.buf = p.buf[1:]
	}
	return tok
}

// atEOF reports whether the current token is EOF.
func (p *Parser) atEOF() bool { return p.peek().Kind == lexer.EOF }

// at reports whether the current token has the given kind.
func (p *Parser) at(k lexer.Kind) bool { return p.peek().Kind == k }

// atKeyword reports whether the current token is the given keyword literal.
func (p *Parser) atKeyword(kw string) bool {
	t := p.peek()
	return t.Kind == lexer.Keyword && t.KeywordID == kw
}

// accept consumes the current token if it matches kind, reporting success.
func (p *Parser) accept(k lexer.Kind) (lexer.Token, bool) {
	if p.at(k) {
		return p.advance(), true
	}
	return p.peek(), false
}

// acceptKeyword consumes the current token if it is the given keyword.
func (p *Parser) acceptKeyword(kw string) bool {
	if p.atKeyword(kw) {
		p.advance()
		return true
	}
	return false
}

// expect consumes a token of the given kind or records a diagnostic at the
// current position and returns ok=false (without consuming).
func (p *Parser) expect(k lexer.Kind, msg string) (lexer.Token, bool) {
	if p.at(k) {
		return p.advance(), true
	}
	p.error(p.peek().Span, msg)
	return p.peek(), false
}

// error records a diagnostic.
func (p *Parser) error(sp source.Span, msg string) {
	p.Diagnostics = append(p.Diagnostics, Diagnostic{Span: sp, Message: msg})
}

// takeTrivia returns and clears the pending leading trivia.
func (p *Parser) takeTrivia() []ast.Trivia {
	t := p.triv
	p.triv = nil
	return t
}

// takePendingComment returns and clears the most recent regular-comment span.
func (p *Parser) takePendingComment() (source.Span, bool) {
	if !p.hasPendingComment {
		return source.Span{}, false
	}
	sp := p.pendingComment
	p.hasPendingComment = false
	return sp, true
}

// spanFrom builds a span from a start offset to the end of the previously
// consumed token region (current token's start).
func (p *Parser) spanFrom(start int) source.Span {
	end := p.peek().Span.Offset
	if end < start {
		end = start
	}
	return source.Span{Offset: start, Len: end - start}
}

// ParseFile parses the whole source as a RootNamespace (brace-less member list).
func (p *Parser) ParseFile() *ast.RootNamespace {
	start := p.peek().Span.Offset
	root := &ast.RootNamespace{}
	for !p.atEOF() {
		before := len(p.buf)
		beforeOff := p.peek().Span.Offset
		m := p.parseMember()
		if m != nil {
			root.Members = append(root.Members, m)
		}
		// Guarantee progress: if nothing was consumed, skip a token.
		if len(p.buf) == before && p.peek().Span.Offset == beforeOff && !p.atEOF() {
			p.advance()
		}
	}
	root.NodeSpan = p.spanFrom(start)
	return root
}
