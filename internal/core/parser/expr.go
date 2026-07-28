package parser

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
)

// ParseExpression is the public entry to the expression sub-parser.
// Task 12 delegates to parsePrimary; Tasks 13-14 layer the operator ladder
// and postfix chain above it.
func (p *Parser) ParseExpression() ast.Node {
	return p.parsePrimary()
}

// parsePrimary parses a base expression (Task 14 extends it with postfixes).
func (p *Parser) parsePrimary() ast.Node {
	return p.parseBase()
}

// parseBase parses a leaf/base expression.
func (p *Parser) parseBase() ast.Node {
	start := p.peek().Span.Offset
	trivia := p.takeTrivia()

	setBase := func(n ast.Node) ast.Node {
		if nb, ok := n.(interface{ SetLeadingTrivia([]ast.Trivia) }); ok {
			nb.SetLeadingTrivia(trivia)
		}
		return n
	}

	switch {
	case p.atKeyword("null"):
		p.advance()
		n := &ast.NullExpr{}
		n.NodeSpan = p.spanFrom(start)
		return setBase(n)

	case p.atKeyword("true"), p.atKeyword("false"):
		tok := p.advance()
		n := &ast.LiteralBool{Value: tok.KeywordID == "true"}
		n.NodeSpan = p.spanFrom(start)
		return setBase(n)

	case p.atKeyword("new"):
		return setBase(p.parseConstructor(start))

	case p.at(lexer.Decimal):
		tok := p.advance()
		n := &ast.LiteralInteger{Value: p.src.Text(tok.Span)}
		n.NodeSpan = p.spanFrom(start)
		return setBase(n)

	case p.at(lexer.Real):
		tok := p.advance()
		n := &ast.LiteralReal{Value: p.src.Text(tok.Span)}
		n.NodeSpan = p.spanFrom(start)
		return setBase(n)

	case p.at(lexer.String):
		tok := p.advance()
		n := &ast.LiteralString{Value: p.src.Text(tok.Span)}
		n.NodeSpan = p.spanFrom(start)
		return setBase(n)

	case p.at(lexer.Star):
		// Infinity literal in expression position.
		p.advance()
		n := &ast.LiteralInfinity{}
		n.NodeSpan = p.spanFrom(start)
		return setBase(n)

	case p.at(lexer.LParen):
		return setBase(p.parseParenOrSequence(start))

	case p.at(lexer.LBrace):
		return setBase(p.parseBodyExpr(start))

	case p.atName():
		qn := p.parseQualifiedName()
		// A bare `Type(args)` invocation with no receiver is recognized here.
		if p.at(lexer.LParen) {
			return setBase(p.parseInvocationTail(start, nil, qn))
		}
		fr := &ast.FeatureReference{Name: qn}
		fr.NodeSpan = p.spanFrom(start)
		return setBase(fr)

	default:
		p.error(p.peek().Span, "expected an expression")
		en := &ast.ErrorNode{Message: "expected an expression"}
		if !p.atEOF() && !p.at(lexer.RParen) && !p.at(lexer.RBrace) && !p.at(lexer.Semicolon) {
			p.advance() // ensure progress
		}
		en.NodeSpan = p.spanFrom(start)
		return setBase(en)
	}
}

// parseParenOrSequence parses `( )`, `( expr )`, or `( expr, expr, ... )`.
func (p *Parser) parseParenOrSequence(start int) ast.Node {
	p.advance() // (
	var elems []ast.Node
	if !p.at(lexer.RParen) {
		elems = append(elems, p.ParseExpression())
		for p.at(lexer.Comma) {
			p.advance() // ,
			elems = append(elems, p.ParseExpression())
		}
	}
	p.expect(lexer.RParen, "expected ')'")
	if len(elems) == 1 {
		return elems[0]
	}
	seq := &ast.SequenceExpr{Elements: elems}
	seq.NodeSpan = p.spanFrom(start)
	return seq
}

// parseConstructor parses `new QualifiedName ( args )`.
func (p *Parser) parseConstructor(start int) ast.Node {
	p.advance() // new
	qn := p.parseQualifiedName()
	c := &ast.ConstructorExpr{Type: qn}
	if p.at(lexer.LParen) {
		c.Args, _ = p.parseArgList()
	}
	c.NodeSpan = p.spanFrom(start)
	return c
}

// parseArgList parses `( )`, positional `( a, b )`, or named `( n=a, m=b )`.
// Returns positional args and named args (one slice empty).
func (p *Parser) parseArgList() ([]ast.Node, []ast.NamedArg) {
	p.expect(lexer.LParen, "expected '('")
	var pos []ast.Node
	var named []ast.NamedArg
	if p.at(lexer.RParen) {
		p.advance()
		return pos, named
	}
	// Named if the first token is a name immediately followed by '='.
	if p.namedArgAhead() {
		for {
			name := p.parseQualifiedName()
			p.expect(lexer.Eq, "expected '=' in named argument")
			val := p.ParseExpression()
			named = append(named, ast.NamedArg{Name: name, Value: val})
			if !p.at(lexer.Comma) {
				break
			}
			p.advance()
		}
	} else {
		for {
			pos = append(pos, p.ParseExpression())
			if !p.at(lexer.Comma) {
				break
			}
			p.advance()
		}
	}
	p.expect(lexer.RParen, "expected ')'")
	return pos, named
}

// namedArgAhead reports whether the arg list is `name = ...` (named form).
func (p *Parser) namedArgAhead() bool {
	if !p.atName() {
		return false
	}
	// Skip a qualified name, then check for '='.
	i := 1
	for p.peekN(i).Kind == lexer.ColonColon {
		i++
		if k := p.peekN(i).Kind; k != lexer.Identifier && k != lexer.UnrestrictedName {
			return false
		}
		i++
	}
	return p.peekN(i).Kind == lexer.Eq
}

// parseInvocationTail parses `( args )` after a receiver/type has been read.
func (p *Parser) parseInvocationTail(start int, recv ast.Node, typ *ast.QualifiedName) ast.Node {
	args, named := p.parseArgList()
	inv := &ast.InvocationExpr{Operand: recv, Type: typ, Args: args, NamedArgs: named}
	inv.NodeSpan = p.spanFrom(start)
	return inv
}

// parseBodyExpr parses `{ (in param ;)* resultExpr }`.
func (p *Parser) parseBodyExpr(start int) ast.Node {
	p.advance() // {
	b := &ast.BodyExpr{}
	for p.atKeyword("in") {
		p.advance() // in
		if seg, ok := p.parseNameSegment(); ok {
			b.Params = append(b.Params, ast.BodyParam{Name: seg.Text, Span: seg.Span})
		}
		p.expect(lexer.Semicolon, "expected ';' after body parameter")
	}
	if !p.at(lexer.RBrace) {
		b.Result = p.ParseExpression()
	}
	p.expect(lexer.RBrace, "expected '}'")
	b.NodeSpan = p.spanFrom(start)
	return b
}
