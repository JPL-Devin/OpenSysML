package parser

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
)

// parseActionBody parses the body of an action usage.
// Expects '{' already consumed, returns list of action nodes + edges.
func (p *Parser) parseActionBody() []ast.Node {
	var members []ast.Node
	
	for !p.at(lexer.RBrace) && !p.atEOF() {
		members = append(members, p.parseActionMember())
	}
	
	p.expect(lexer.RBrace, "expected '}' after action body")
	return members
}

// parseActionMember parses one action member: node or edge.
func (p *Parser) parseActionMember() ast.Node {
	start := p.peek().Span.Offset
	
	// Check for keyword dispatch
	if tok, ok := p.accept(lexer.Keyword); ok {
		kw := tok.KeywordID
		switch kw {
		case "first":
			return p.parseInitialNode(tok)
		case "done":
			return p.parseFinalNode(tok)
		case "fork":
			return p.parseForkNode(tok)
		case "join":
			return p.parseJoinNode(tok)
		case "merge":
			return p.parseMergeNode(tok)
		case "decision":
			return p.parseDecisionNode(tok)
		case "action":
			return p.parseActionExecutionNode(tok)
		case "then":
			return p.parseSuccessionEdge(tok)
		default:
			// Unknown keyword, return ErrorNode
			p.error(tok.Span, "unknown action keyword: "+kw)
			en := &ast.ErrorNode{Message: "unknown action keyword: " + kw}
			en.NodeSpan = p.spanFrom(start)
			return en
		}
	}
	
	// Not a keyword — for now, return ErrorNode (Task 11 will handle edges)
	p.error(p.peek().Span, "expected action node or edge keyword")
	en := &ast.ErrorNode{Message: "expected action node or edge keyword"}
	if !p.atEOF() && !p.at(lexer.RBrace) && !p.at(lexer.Semicolon) {
		p.advance() // ensure progress
	}
	en.NodeSpan = p.spanFrom(start)
	return en
}

// Stubs — Tasks 9-10 will implement

func (p *Parser) parseInitialNode(tok lexer.Token) ast.Node {
	return &ast.InitialNode{NodeBase: ast.NodeBase{NodeSpan: tok.Span}}
}

func (p *Parser) parseFinalNode(tok lexer.Token) ast.Node {
	return &ast.FinalNode{NodeBase: ast.NodeBase{NodeSpan: tok.Span}}
}

func (p *Parser) parseForkNode(tok lexer.Token) ast.Node {
	return &ast.ForkNode{NodeBase: ast.NodeBase{NodeSpan: tok.Span}}
}

func (p *Parser) parseJoinNode(tok lexer.Token) ast.Node {
	return &ast.JoinNode{NodeBase: ast.NodeBase{NodeSpan: tok.Span}}
}

func (p *Parser) parseMergeNode(tok lexer.Token) ast.Node {
	return &ast.MergeNode{NodeBase: ast.NodeBase{NodeSpan: tok.Span}}
}

func (p *Parser) parseDecisionNode(tok lexer.Token) ast.Node {
	return &ast.DecisionNode{NodeBase: ast.NodeBase{NodeSpan: tok.Span}}
}

func (p *Parser) parseActionExecutionNode(tok lexer.Token) ast.Node {
	return &ast.ActionExecutionNode{NodeBase: ast.NodeBase{NodeSpan: tok.Span}}
}

func (p *Parser) parseSuccessionEdge(tok lexer.Token) ast.Node {
	return &ast.SuccessionEdge{NodeBase: ast.NodeBase{NodeSpan: tok.Span}}
}
