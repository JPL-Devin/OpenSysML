package parser

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
)

// atName reports whether the current token can begin a name segment.
func (p *Parser) atName() bool {
	k := p.peek().Kind
	return k == lexer.Identifier || k == lexer.UnrestrictedName
}

// parseNameSegment consumes one name token and returns its segment.
func (p *Parser) parseNameSegment() (ast.NameSegment, bool) {
	if !p.atName() {
		return ast.NameSegment{}, false
	}
	tok := p.advance()
	return ast.NameSegment{Text: p.src.Text(tok.Span), Span: tok.Span}, true
}

// parseQualifiedName parses `[$::] Name (:: Name)*`. It returns nil and
// records a diagnostic if no name is present.
func (p *Parser) parseQualifiedName() *ast.QualifiedName {
	start := p.peek().Span.Offset
	trivia := p.takeTrivia()

	global := false
	if p.at(lexer.Dollar) && p.peekN(1).Kind == lexer.ColonColon {
		p.advance() // $
		p.advance() // ::
		global = true
	}

	seg, ok := p.parseNameSegment()
	if !ok {
		if global {
			// `$::` with no following name — still a (degenerate) global name.
			qn := &ast.QualifiedName{Global: true}
			qn.NodeSpan = p.spanFrom(start)
			qn.SetLeadingTrivia(trivia)
			return qn
		}
		p.error(p.peek().Span, "expected a name")
		return nil
	}

	parts := []ast.NameSegment{seg}
	for p.at(lexer.ColonColon) {
		// Do not consume `::` if it introduces `*`/`**` (namespace import wildcard).
		if nk := p.peekN(1).Kind; nk == lexer.Star || nk == lexer.StarStar {
			break
		}
		p.advance() // ::
		next, ok := p.parseNameSegment()
		if !ok {
			p.error(p.peek().Span, "expected a name after '::'")
			break
		}
		parts = append(parts, next)
	}

	qn := &ast.QualifiedName{Global: global, Parts: parts}
	qn.NodeSpan = p.spanFrom(start)
	qn.SetLeadingTrivia(trivia)
	return qn
}

// parseIdentification parses `<shortName> name?` or `name` or nothing.
// A missing identification yields a zero-value Identification (no diagnostic).
func (p *Parser) parseIdentification() ast.Identification {
	var id ast.Identification
	if p.at(lexer.Lt) {
		p.advance() // <
		if seg, ok := p.parseNameSegment(); ok {
			id.ShortName = seg.Text
			id.ShortNameSpan = seg.Span
		} else {
			p.error(p.peek().Span, "expected short name after '<'")
		}
		p.expect(lexer.Gt, "expected '>'")
	}
	if seg, ok := p.parseNameSegment(); ok {
		id.Name = seg.Text
		id.NameSpan = seg.Span
	}
	return id
}
