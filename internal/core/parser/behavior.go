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
	if !p.atEOF() && !p.at(lexer.RBrace) {
		p.advance() // ensure progress (consume even semicolons to prevent infinite loop)
	}
	en.NodeSpan = p.spanFrom(start)
	return en
}

// Action node parsers — Task 9 complete. Task 10 (ActionExecutionNode + SuccessionEdge) below.

func (p *Parser) parseInitialNode(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	var name string
	
	if p.at(lexer.Identifier) {
		nameToken := p.peek()
		name = p.src.Text(nameToken.Span)
		p.advance()
	}
	
	p.expect(lexer.Semicolon, "expected ';' after initial node")
	
	node := &ast.InitialNode{
		Name: name,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

func (p *Parser) parseFinalNode(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	var name string
	
	if p.at(lexer.Identifier) {
		nameToken := p.peek()
		name = p.src.Text(nameToken.Span)
		p.advance()
	}
	
	p.expect(lexer.Semicolon, "expected ';' after final node")
	
	node := &ast.FinalNode{
		Name: name,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

func (p *Parser) parseForkNode(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	var name string
	
	if p.at(lexer.Identifier) {
		nameToken := p.peek()
		name = p.src.Text(nameToken.Span)
		p.advance()
	}
	
	p.expect(lexer.Semicolon, "expected ';' after fork node")
	
	node := &ast.ForkNode{
		Name: name,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

func (p *Parser) parseJoinNode(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	var name string
	
	if p.at(lexer.Identifier) {
		nameToken := p.peek()
		name = p.src.Text(nameToken.Span)
		p.advance()
	}
	
	p.expect(lexer.Semicolon, "expected ';' after join node")
	
	node := &ast.JoinNode{
		Name: name,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

func (p *Parser) parseMergeNode(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	var name string
	
	if p.at(lexer.Identifier) {
		nameToken := p.peek()
		name = p.src.Text(nameToken.Span)
		p.advance()
	}
	
	p.expect(lexer.Semicolon, "expected ';' after merge node")
	
	node := &ast.MergeNode{
		Name: name,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

func (p *Parser) parseDecisionNode(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	var name string
	
	if p.at(lexer.Identifier) {
		nameToken := p.peek()
		name = p.src.Text(nameToken.Span)
		p.advance()
	}
	
	p.expect(lexer.Semicolon, "expected ';' after decision node")
	
	node := &ast.DecisionNode{
		Name: name,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

func (p *Parser) parseActionExecutionNode(tok lexer.Token) ast.Node {
	// Syntax:
	//   action [name] actionRef ;
	//   action [name] { expression } ;
	start := tok.Span.Offset
	trivia := p.takeTrivia()
	
	var name string
	var actionRef *ast.QualifiedName
	var expression ast.Node
	
	// Disambiguate: name vs ref, inline vs reference mode
	if p.at(lexer.LBrace) {
		// Inline mode, no name: action { expr };
		p.advance() // consume '{'
		expression = p.ParseExpression()
		_, ok := p.expect(lexer.RBrace, "expected '}' after action expression")
		if !ok {
			return &ast.ErrorNode{
				NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)},
				Message:  "expected '}' after action expression",
			}
		}
	} else if p.at(lexer.Identifier) {
		// Could be:
		// - action name { expr }; (name + inline)
		// - action name ref; (name + ref)
		// - action ref; (ref only)
		// Use lookahead to decide
		nextTok := p.peekN(1)
		if nextTok.Kind == lexer.LBrace {
			// name + inline: action name { expr };
			nameToken := p.peek()
			name = p.src.Text(nameToken.Span)
			p.advance()
			p.advance() // consume '{'
			expression = p.ParseExpression()
			_, ok := p.expect(lexer.RBrace, "expected '}' after action expression")
			if !ok {
				return &ast.ErrorNode{
					NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)},
					Message:  "expected '}' after action expression",
				}
			}
		} else if nextTok.Kind == lexer.Identifier || nextTok.Kind == lexer.ColonColon {
			// Could be name + ref OR just ref (qualified name)
			// Parse first identifier
			firstIdToken := p.peek()
			firstIdSpan := firstIdToken.Span
			firstId := p.src.Text(firstIdSpan)
			p.advance()
			
			// Check what follows
			if p.at(lexer.ColonColon) {
				// It's a qualified name starting with firstId (no separate name)
				// Build QualifiedName manually since we consumed first part
				parts := []ast.NameSegment{{Text: firstId, Span: firstIdSpan}}
				for p.at(lexer.ColonColon) {
					p.advance() // consume '::'
					if !p.at(lexer.Identifier) {
						return &ast.ErrorNode{
							NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)},
							Message:  "expected identifier after '::'",
						}
					}
					seg := p.peek()
					parts = append(parts, ast.NameSegment{Text: p.src.Text(seg.Span), Span: seg.Span})
					p.advance()
				}
				actionRef = &ast.QualifiedName{Parts: parts}
				actionRef.NodeSpan = p.spanFrom(firstIdSpan.Offset)
			} else if p.at(lexer.Identifier) {
				// firstId is name, what follows is actionRef
				name = firstId
				actionRef = p.parseQualifiedName()
			} else {
				// firstId is a simple (non-qualified) actionRef
				actionRef = &ast.QualifiedName{
					Parts: []ast.NameSegment{{Text: firstId, Span: firstIdSpan}},
				}
				actionRef.NodeSpan = firstIdSpan
			}
		} else {
			// Single identifier followed by something else (likely ';')
			idToken := p.peek()
			actionRef = &ast.QualifiedName{
				Parts: []ast.NameSegment{{Text: p.src.Text(idToken.Span), Span: idToken.Span}},
			}
			actionRef.NodeSpan = idToken.Span
			p.advance()
		}
	} else {
		return &ast.ErrorNode{
			NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)},
			Message:  "expected action reference or '{' after 'action'",
		}
	}
	
	p.expect(lexer.Semicolon, "expected ';' after action execution node")
	
	node := &ast.ActionExecutionNode{
		Name:       name,
		ActionRef:  actionRef,
		Expression: expression,
	}
	node.NodeSpan = p.spanFrom(start)
	node.SetLeadingTrivia(trivia)
	return node
}

// parseSuccessionEdge parses: then source target [if guard] ;
func (p *Parser) parseSuccessionEdge(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	
	source := p.parseQualifiedName()
	target := p.parseQualifiedName()
	
	// Check for optional guard
	if p.acceptKeyword("if") {
		// 'if' keyword already consumed
		guard := p.ParseExpression()
		
		p.expect(lexer.Semicolon, "expected ';' after control flow edge")
		
		node := &ast.ControlFlowEdge{
			NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)},
			Source:   source,
			Target:   target,
			Guard:    guard,
		}
		return node
	}
	
	p.expect(lexer.Semicolon, "expected ';' after succession edge")
	
	node := &ast.SuccessionEdge{
		NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)},
		Source:   source,
		Target:   target,
	}
	return node
}

// Phase C1: Calculation and Constraint Bodies

// parseResultBody parses the body of a calculation usage.
// Expects '{' already consumed, returns list of ResultMember nodes.
// Syntax: calc example { return x + 5; }
func (p *Parser) parseResultBody() []ast.Node {
	var members []ast.Node
	
	for !p.at(lexer.RBrace) && !p.atEOF() {
		members = append(members, p.parseResultMember())
	}
	
	p.expect(lexer.RBrace, "expected '}' after result body")
	return members
}

// parseResultMember parses one result member: return <expr>;
func (p *Parser) parseResultMember() ast.Node {
	start := p.peek().Span.Offset
	
	// Expect 'return' keyword
	if !p.acceptKeyword("return") {
		p.error(p.peek().Span, "expected 'return' in calculation body")
		en := &ast.ErrorNode{Message: "expected 'return' in calculation body"}
		if !p.atEOF() && !p.at(lexer.RBrace) {
			p.advance() // ensure progress
		}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	
	// Parse expression
	expr := p.ParseExpression()
	
	p.expect(lexer.Semicolon, "expected ';' after return expression")
	
	node := &ast.ResultMember{
		Expression: expr,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseConstraintBody parses the body of a constraint usage.
// Expects '{' already consumed, returns list of ConstraintMember nodes.
// Syntax: constraint example { assert x > 0; assume y != null; assert not z < 0; }
func (p *Parser) parseConstraintBody() []ast.Node {
	var members []ast.Node
	
	for !p.at(lexer.RBrace) && !p.atEOF() {
		members = append(members, p.parseConstraintMember())
	}
	
	p.expect(lexer.RBrace, "expected '}' after constraint body")
	return members
}

// parseConstraintMember parses one constraint member: assert/assume [not] <expr>;
func (p *Parser) parseConstraintMember() ast.Node {
	start := p.peek().Span.Offset
	
	var isAssert bool
	var isNegated bool
	
	// Expect 'assert' or 'assume' keyword
	if p.acceptKeyword("assert") {
		isAssert = true
	} else if p.acceptKeyword("assume") {
		isAssert = false
	} else {
		p.error(p.peek().Span, "expected 'assert' or 'assume' in constraint body")
		en := &ast.ErrorNode{Message: "expected 'assert' or 'assume' in constraint body"}
		if !p.atEOF() && !p.at(lexer.RBrace) {
			p.advance() // ensure progress
		}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	
	// Check for optional 'not' keyword
	if p.acceptKeyword("not") {
		isNegated = true
	}
	
	// Parse expression
	expr := p.ParseExpression()
	
	p.expect(lexer.Semicolon, "expected ';' after constraint expression")
	
	node := &ast.ConstraintMember{
		IsAssert:   isAssert,
		IsNegated:  isNegated,
		Expression: expr,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// Phase C2: Requirement Bodies

// parseRequirementBody parses the body of a requirement usage.
// Expects '{' already consumed, returns list of requirement members.
// Syntax: requirement example { subject x : Type; assume x > 0; require x.valid; actor a : Actor; }
func (p *Parser) parseRequirementBody() []ast.Node {
	var members []ast.Node
	
	for !p.at(lexer.RBrace) && !p.atEOF() {
		members = append(members, p.parseRequirementMember())
	}
	
	p.expect(lexer.RBrace, "expected '}' after requirement body")
	return members
}

// parseRequirementMember parses one requirement member: subject/assume/require/actor
func (p *Parser) parseRequirementMember() ast.Node {
	start := p.peek().Span.Offset
	
	// Check for keyword dispatch
	if p.acceptKeyword("subject") {
		return p.parseSubjectMember(start)
	} else if p.acceptKeyword("assume") {
		return p.parseAssumeMember(start)
	} else if p.acceptKeyword("require") {
		return p.parseRequireMember(start)
	} else if p.acceptKeyword("actor") {
		return p.parseActorMember(start)
	}
	
	// Unknown member type
	p.error(p.peek().Span, "expected 'subject', 'assume', 'require', or 'actor' in requirement body")
	en := &ast.ErrorNode{Message: "expected requirement member keyword"}
	if !p.atEOF() && !p.at(lexer.RBrace) {
		p.advance() // ensure progress
	}
	en.NodeSpan = p.spanFrom(start)
	return en
}

// parseSubjectMember parses: subject <name> : <Type>;
func (p *Parser) parseSubjectMember(start int) ast.Node {
	// 'subject' already consumed
	
	// Expect identifier
	if !p.at(lexer.Identifier) {
		p.error(p.peek().Span, "expected identifier after 'subject'")
		en := &ast.ErrorNode{Message: "expected identifier after 'subject'"}
		if !p.atEOF() && !p.at(lexer.RBrace) {
			p.advance()
		}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	
	nameToken := p.peek()
	name := p.src.Text(nameToken.Span)
	p.advance()
	
	// Expect ':'
	if !p.at(lexer.Colon) {
		p.error(p.peek().Span, "expected ':' after subject name")
		en := &ast.ErrorNode{Message: "expected ':' after subject name"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	p.advance() // consume ':'
	
	// Parse type
	typeRef := p.parseQualifiedName()
	
	p.expect(lexer.Semicolon, "expected ';' after subject declaration")
	
	node := &ast.SubjectMember{
		Name:    name,
		TypeRef: typeRef,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseAssumeMember parses: assume <expr>;
func (p *Parser) parseAssumeMember(start int) ast.Node {
	// 'assume' already consumed
	
	expr := p.ParseExpression()
	
	p.expect(lexer.Semicolon, "expected ';' after assume expression")
	
	node := &ast.AssumeMember{
		Expression: expr,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseRequireMember parses: require <expr>;
func (p *Parser) parseRequireMember(start int) ast.Node {
	// 'require' already consumed
	
	expr := p.ParseExpression()
	
	p.expect(lexer.Semicolon, "expected ';' after require expression")
	
	node := &ast.RequireMember{
		Expression: expr,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseActorMember parses: actor <name> : <Type>;
func (p *Parser) parseActorMember(start int) ast.Node {
	// 'actor' already consumed
	
	// Expect identifier
	if !p.at(lexer.Identifier) {
		p.error(p.peek().Span, "expected identifier after 'actor'")
		en := &ast.ErrorNode{Message: "expected identifier after 'actor'"}
		if !p.atEOF() && !p.at(lexer.RBrace) {
			p.advance()
		}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	
	nameToken := p.peek()
	name := p.src.Text(nameToken.Span)
	p.advance()
	
	// Expect ':'
	if !p.at(lexer.Colon) {
		p.error(p.peek().Span, "expected ':' after actor name")
		en := &ast.ErrorNode{Message: "expected ':' after actor name"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	p.advance() // consume ':'
	
	// Parse type
	typeRef := p.parseQualifiedName()
	
	p.expect(lexer.Semicolon, "expected ';' after actor declaration")
	
	node := &ast.ActorMember{
		Name:    name,
		TypeRef: typeRef,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}
