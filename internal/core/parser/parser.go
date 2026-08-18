package parser

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/quickfix"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// Parser is a hand-written recursive-descent parser over a lexer token
// stream. It buffers non-trivia tokens for lookahead and collects
// diagnostics; it always produces a tree (ErrorNodes for bad input).
type Parser struct {
	src *source.SourceFile
	lx  *lexer.Lexer
	// buf holds every non-trivia token read so far and is only appended to;
	// pos is the read cursor into it. Consuming a token moves the cursor, so a
	// checkpoint can rewind over what a try-parse consumed.
	buf  []lexer.Token
	pos  int
	triv []ast.Trivia // trivia pending attachment to the next node
	// Diagnostics are syntax errors: input the parser could not read as
	// well-formed SysML.
	Diagnostics []Diagnostic
	// Warnings are findings on input that parsed into the tree the author
	// intended but is not well-formed SysML, such as a reserved keyword written
	// as a declaration name.
	Warnings []Diagnostic

	pendingComment    source.Span // span of the most recent /* */ regular comment
	hasPendingComment bool

	// calcBodyDepth counts the calculation bodies being parsed, so a `return`
	// reached in a statement position inside one is read as an early return
	// rather than as a result parameter declaration.
	calcBodyDepth int

	// effectDepth counts the transition effects being parsed, whose statement is
	// closed by the transition's next clause rather than by ';'.
	effectDepth int

	// effectStmtStart is the source offset of the statement written as the
	// innermost transition effect, which the transition's own ';' terminates
	// (SysML.xtext EffectBehaviorUsage carries no ';'). It is only meaningful
	// while effectDepth > 0, and distinguishes that statement from the ones
	// nested inside its body, which end with their own ';'.
	effectStmtStart int

	// bodyCtx is the stack of enclosing body notations; only the innermost
	// matters (see bodyContext).
	bodyCtx []bodyContext
}

// bodyContext is the notation of the body being parsed, for members whose
// grammar depends on it: an interface body's default end is a port usage and
// may be anonymous (SysML v2 8.2.2.14, DefaultInterfaceEnd).
type bodyContext int

const (
	bodyOther bodyContext = iota
	bodyInterface
)

// pushBodyContext enters a body of the given notation and returns the function
// that leaves it.
func (p *Parser) pushBodyContext(c bodyContext) func() {
	depth := len(p.bodyCtx)
	p.bodyCtx = append(p.bodyCtx, c)
	return func() { p.bodyCtx = p.bodyCtx[:depth] }
}

// bodyContext returns the notation of the innermost body being parsed.
func (p *Parser) bodyContext() bodyContext {
	if len(p.bodyCtx) == 0 {
		return bodyOther
	}
	return p.bodyCtx[len(p.bodyCtx)-1]
}

// parseCheckpoint captures parser state for backtracking.
type parseCheckpoint struct {
	pos           int
	diagnosticLen int
	warningLen    int
	pendingSpan   source.Span
	hadPending    bool
}

// New creates a Parser for the given source file.
func New(sf *source.SourceFile) *Parser {
	return &Parser{src: sf, lx: lexer.New(sf)}
}

// fill ensures buf holds the token n positions ahead of the cursor (pulling
// from the lexer, skipping and recording trivia). The final EOF token is
// sticky (re-returned).
func (p *Parser) fill(n int) {
	for len(p.buf) <= p.pos+n {
		tok := p.lx.Next()
		for tok.IsTrivia() || tok.Kind == lexer.RegularComment {
			p.triv = append(p.triv, triviaOf(tok))
			if tok.Unterminated {
				// Everything after the opener is inside it, so the declarations
				// that follow are not in the tree at all.
				p.error(tok.Span, "unterminated comment: missing */")
			}
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
	if p.pos+n >= len(p.buf) {
		return p.buf[len(p.buf)-1] // EOF (sticky)
	}
	return p.buf[p.pos+n]
}

// advance consumes and returns the current token.
func (p *Parser) advance() lexer.Token {
	p.fill(0)
	tok := p.buf[p.pos]
	if tok.Kind != lexer.EOF {
		p.pos++
	}
	return tok
}

// atEOF reports whether the current token is EOF.
func (p *Parser) atEOF() bool { return p.peek().Kind == lexer.EOF }

// Offset returns the source offset of the next unconsumed token, which is where
// a caller parsing a sequence of productions continues from. A node's span is
// not that position: a parenthesized expression's span is its contents.
func (p *Parser) Offset() int { return p.peek().Span.Offset }

// at reports whether the current token has the given kind.
func (p *Parser) at(k lexer.Kind) bool { return p.peek().Kind == k }

// atKeyword reports whether the current token is the given keyword literal.
func (p *Parser) atKeyword(kw string) bool {
	t := p.peek()
	return t.Kind == lexer.Keyword && t.KeywordID == kw
}

// peekIsKeyword reports whether the token n ahead of the cursor is the given
// keyword literal.
func (p *Parser) peekIsKeyword(n int, kw string) bool {
	t := p.peekN(n)
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
	if k == lexer.Semicolon {
		// The statement ends where the last token consumed for it ends, which is
		// where the missing ';' goes; the diagnostic sits on the token that
		// should have followed it.
		p.errorWithFixes(p.peek().Span, msg, quickfix.Fix{
			Title:     "Insert ';'",
			Edits:     []quickfix.Edit{quickfix.Insert(p.lastEnd(), ";")},
			Preferred: true,
		})
		return p.peek(), false
	}
	p.error(p.peek().Span, msg)
	return p.peek(), false
}

// lastEnd returns the end offset of the last consumed non-trivia token, or the
// start of the current one when nothing has been consumed.
func (p *Parser) lastEnd() int {
	if p.pos > 0 && p.pos <= len(p.buf) {
		return p.buf[p.pos-1].Span.End()
	}
	return p.peek().Span.Offset
}

// error records a diagnostic that makes the parse ill-formed.
func (p *Parser) error(sp source.Span, msg string) {
	p.Diagnostics = append(p.Diagnostics, Diagnostic{Span: sp, Message: msg})
}

// errorWithFixes records an ill-formed-parse diagnostic that unambiguous edits
// resolve.
func (p *Parser) errorWithFixes(sp source.Span, msg string, fixes ...quickfix.Fix) {
	p.Diagnostics = append(p.Diagnostics, Diagnostic{Span: sp, Message: msg, Fixes: fixes})
}

// warn records a diagnostic for input that parses to the tree the author meant
// but is not well-formed SysML, under the code a consumer reports it by. It is
// kept apart from Diagnostics so that callers gating on a clean parse are not
// blocked by it.
func (p *Parser) warn(sp source.Span, msg, code string) {
	p.Warnings = append(p.Warnings, Diagnostic{Span: sp, Message: msg, Code: code})
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
		before := p.pos
		beforeOff := p.peek().Span.Offset
		root.Members = append(root.Members, p.parseMember())
		// Guarantee progress: if nothing was consumed, skip a token.
		if p.pos == before && p.peek().Span.Offset == beforeOff && !p.atEOF() {
			p.advance()
		}
	}
	root.NodeSpan = p.spanFrom(start)
	return root
}

// checkpoint captures current parser state for backtracking.
func (p *Parser) checkpoint() parseCheckpoint {
	return parseCheckpoint{
		pos:           p.pos,
		diagnosticLen: len(p.Diagnostics),
		warningLen:    len(p.Warnings),
		pendingSpan:   p.pendingComment,
		hadPending:    p.hasPendingComment,
	}
}

// restore rewinds parser to a previous checkpoint, un-consuming the tokens the
// abandoned attempt read and dropping the findings it reported. Used for
// try-parse patterns.
//
// Trivia collected during the attempt is deliberately kept: the lexer yields
// each trivia token once, so dropping it would lose a comment from the tree.
func (p *Parser) restore(cp parseCheckpoint) {
	p.pos = cp.pos
	p.Diagnostics = p.Diagnostics[:cp.diagnosticLen]
	p.Warnings = p.Warnings[:cp.warningLen]
	p.pendingComment = cp.pendingSpan
	p.hasPendingComment = cp.hadPending
}
