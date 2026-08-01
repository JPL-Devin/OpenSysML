package parser

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
)

// parseCalcBody parses the body of a calc def/usage.
// Handles BOTH generic members (parameters like 'in x: Integer;') AND result members ('return expr;').
// Expects '{' already consumed.
func (p *Parser) parseCalcBody() []ast.Node {
	var members []ast.Node
	
	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset
		
		// Check for 'return' keyword → ResultMember
		if p.isResultKeyword() {
			members = append(members, p.parseResultMember())
		} else {
			// Try parsing as body member (parameters, doc, import, etc.)
			// Body member expects: visibility + declaration keyword, or special patterns
			// If we see expression-start that's NOT a declaration, parse as implicit return expression
			// Check if current position looks like expression start (name, literal, if, etc.)
			// but not a declaration pattern (name followed by colon/semicolon/keyword/bracket)
			peek1 := p.peek()
			peek2 := p.peekN(1)
			isNameDecl := (peek1.Kind == lexer.Identifier || peek1.Kind == lexer.UnrestrictedName) &&
				(peek2.Kind == lexer.Colon || peek2.Kind == lexer.Semicolon || 
				 peek2.Kind == lexer.Keyword || peek2.Kind == lexer.LBracket)
			
			// If expression-start but NOT name-declaration pattern, parse as implicit return
			if p.atExprStart() && !isNameDecl {
				// Parse as implicit return expression
				expr := p.ParseExpression()
				if expr != nil {
					members = append(members, expr)
				}
			} else {
				// Parse as generic body member (parameters, etc.)
				m := p.parseBodyMember()
				if m != nil {
					members = append(members, m)
				}
			}
		}
		
		// Guard against infinite loop
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.advance()
		}
	}
	
	p.expect(lexer.RBrace, "expected '}' after calc body")
	return members
}

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
	
	// Handle doc keyword specially (parseDocumentation consumes it)
	if p.atKeyword("doc") {
		return p.parseDocumentation(start)
	}
	
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

// parseResultMember parses one result member: 
//   return <expr>;         -- computed result
//   return : Type[mult];   -- result parameter (anonymous, type-only)
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
	
	// Parse optional usage kind keyword (e.g., 'attribute')
	// Default to UsageAttribute if not specified
	usageKind := ast.UsageAttribute
	if p.at(lexer.Keyword) {
		if kind, ok := usageKindKeywords[p.peek().KeywordID]; ok {
			usageKind = kind
			p.advance() // consume kind keyword
		}
	}
	
	// Parse optional feature modifiers after kind keyword
	mods := p.parseFeatureModifiers()
	
	// Check for named or anonymous result parameter syntax
	// Pattern 1: return [modifiers] name: Type[mult];  (named result parameter)
	// Pattern 2: return [modifiers] : Type[mult];      (anonymous result parameter)
	// Pattern 3: return expr;              (computed result)
	// Pattern 4: return name = expr;       (computed result with binding)
	// Pattern 5: return [modifiers] name;  (named result parameter, no type)
	// Pattern 6: return [modifiers] name : Type { body } (with body)
	// Use lookahead to distinguish Pattern 1 from Pattern 4
	if p.at(lexer.Colon) || (p.atName() && p.peekN(1).Kind == lexer.Colon) {
		// Parse as result parameter (named or anonymous usage with typing)
		u := &ast.Usage{
			Kind:        usageKind,
			Direction:   ast.DirOut,
			IsAbstract:  mods.isAbstract,
			IsReference: mods.isReference,
			IsEnd:       mods.isEnd,
			IsConstant:  mods.isConstant,
			IsComposite: mods.isComposite,
			IsDerived:   mods.isDerived,
			IsOrdered:   mods.isOrdered,
			IsNonunique: mods.isNonunique,
		}
		
		// Check if named (identifier before colon)
		if p.atName() {
			u.Ident = p.parseIdentification()
		}
		
		// Parse typing relationship ': Type'
		if !p.at(lexer.Colon) {
			p.error(p.peek().Span, "expected ':' after result parameter name")
		} else {
			p.advance() // consume ':'
			
			// Parse type name directly (QualifiedName)
			qn := p.parseQualifiedName()
			if qn != nil {
				rel := &ast.Relationship{Kind: ast.RelTyping, Target: qn}
				rel.NodeSpan = qn.NodeSpan
				u.Relationships = append(u.Relationships, rel)
			}
		}
		
		// Parse additional relationships (e.g., :>> redefines)
		additionalRels, _ := p.parseRelationships(false)
		u.Relationships = append(u.Relationships, additionalRels...)
		
		// Parse optional multiplicity '[n..m]'
		if p.at(lexer.LBracket) {
			u.Multiplicity = p.parseMultiplicity()
		}
		
		// Parse additional feature modifiers after multiplicity (e.g., 'nonunique')
		// Stdlib pattern: return : Type[mult] nonunique;
		mods2 := p.parseFeatureModifiers()
		if mods2.isAbstract {
			u.IsAbstract = true
		}
		if mods2.isReference {
			u.IsReference = true
		}
		if mods2.isEnd {
			u.IsEnd = true
		}
		if mods2.isComposite {
			u.IsComposite = true
		}
		if mods2.isDerived {
			u.IsDerived = true
		}
		if mods2.isOrdered {
			u.IsOrdered = true
		}
		if mods2.isNonunique {
			u.IsNonunique = true
		}
		
		// Parse optional default value 'default expr' or '= expr'
		if p.acceptKeyword("default") {
			u.Value = p.ParseExpression()
		} else if p.accept2(lexer.Eq) {
			u.Value = p.ParseExpression()
		}
		
		// Check for body or semicolon
		if p.at(lexer.LBrace) || p.at(lexer.Semicolon) {
			members, hasBody := p.parseDefUsageBody()
			u.Members = members
			u.HasBody = hasBody
		} else {
			// Neither body nor semicolon → error
			p.error(p.peek().Span, "expected '{' or ';' after return parameter")
		}
		
		u.NodeSpan = p.spanFrom(start)
		return u
	}
	
	// Check for Pattern 5: return [kind] [modifiers] name [mult] [body/semicolon] (no type, no value)
	if p.atName() && (p.peekN(1).Kind == lexer.Semicolon || p.peekN(1).Kind == lexer.LBracket) {
		u := &ast.Usage{
			Kind:        usageKind,
			Direction:   ast.DirOut,
			IsAbstract:  mods.isAbstract,
			IsReference: mods.isReference,
			IsEnd:       mods.isEnd,
			IsConstant:  mods.isConstant,
			IsComposite: mods.isComposite,
			IsDerived:   mods.isDerived,
			IsOrdered:   mods.isOrdered,
			IsNonunique: mods.isNonunique,
		}
		u.Ident = p.parseIdentification()
		
		// Parse optional multiplicity '[n..m]'
		if p.at(lexer.LBracket) {
			u.Multiplicity = p.parseMultiplicity()
		}
		
		// If followed by '=', this is Pattern 4 (value), not Pattern 5 (no value)
		if p.at(lexer.Eq) {
			p.advance() // consume '='
			u.Value = p.ParseExpression()
		}
		
		// Check for body or semicolon
		if p.at(lexer.LBrace) {
			bodyMembers, hasBody := p.parseDefUsageBody()
			u.Members = bodyMembers
			if !hasBody {
				p.expect(lexer.Semicolon, "expected ';' after return parameter")
			}
		} else {
			p.expect(lexer.Semicolon, "expected ';' after return parameter")
		}
		
		u.NodeSpan = p.spanFrom(start)
		return u
	}
	
	// Check for Pattern 4: return [kind] [modifiers] name = expr [body] (result parameter with initializer, no type, no mult)
	// Lookahead: name followed by '=' directly
	if p.atName() && p.peekN(1).Kind == lexer.Eq {
		u := &ast.Usage{
			Kind:        usageKind,
			Direction:   ast.DirOut,
			IsAbstract:  mods.isAbstract,
			IsReference: mods.isReference,
			IsEnd:       mods.isEnd,
			IsConstant:  mods.isConstant,
			IsComposite: mods.isComposite,
			IsDerived:   mods.isDerived,
			IsOrdered:   mods.isOrdered,
			IsNonunique: mods.isNonunique,
		}
		u.Ident = p.parseIdentification()
		p.advance() // consume '='
		u.Value = p.ParseExpression()
		
		// Check for optional body or semicolon
		if p.at(lexer.LBrace) {
			bodyMembers, hasBody := p.parseDefUsageBody()
			u.Members = bodyMembers
			if !hasBody {
				p.expect(lexer.Semicolon, "expected ';' after return parameter")
			}
		} else {
			p.expect(lexer.Semicolon, "expected ';' after return parameter")
		}
		u.NodeSpan = p.spanFrom(start)
		return u
	}
	
	// Otherwise parse as computed result (expression)
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
		before := p.peek().Span.Offset
		
		// Check for doc keyword → parse as documentation
		if p.atKeyword("doc") {
			members = append(members, p.parseDocumentation(before))
		} else {
			// Parse constraint expression (assert/assume/bare expression)
			members = append(members, p.parseConstraintMember())
		}
		
		// Safety check: if position hasn't advanced, force progress to avoid infinite loop
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.advance()
		}
	}
	
	p.expect(lexer.RBrace, "expected '}' after constraint body")
	return members
}

// parseConstraintMember parses one constraint member: assert/assume [not] <expr>;
// Also supports bare expressions (implicit assert): inv name { expr }
func (p *Parser) parseConstraintMember() ast.Node {
	start := p.peek().Span.Offset
	
	var isAssert bool
	var isNegated bool
	
	// Check for 'assert' or 'assume' keyword
	if p.acceptKeyword("assert") {
		isAssert = true
	} else if p.acceptKeyword("assume") {
		isAssert = false
	} else {
		// Bare expression (implicit assert) - common in invariants
		// Example: inv piPrecision { RealFunctions::round(pi * 1E20) == 314159265358979323846.0 }
		isAssert = true // Default to assert for bare expressions
	}
	
	// Check for optional 'not' keyword
	if p.acceptKeyword("not") {
		isNegated = true
	}
	
	// Parse expression
	expr := p.ParseExpression()
	
	// Semicolon is optional for constraint expressions (especially in inv bodies)
	p.accept2(lexer.Semicolon)
	
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

// parseRequirementMember parses one requirement member: subject/assume/require/actor/doc
func (p *Parser) parseRequirementMember() ast.Node {
	start := p.peek().Span.Offset
	
	// Check for doc keyword
	if p.atKeyword("doc") {
		return p.parseDocumentation(start)
	}
	
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
	
	// Parse optional multiplicity
	var mult *ast.Multiplicity
	if p.at(lexer.LBracket) {
		mult = p.parseMultiplicity()
	}
	
	// Parse optional body or expect semicolon
	if p.at(lexer.LBrace) {
		// Body present - parse requirement body recursively
		p.advance() // consume '{'
		members := p.parseRequirementBody()
		
		node := &ast.SubjectMember{
			Name:         name,
			TypeRef:      typeRef,
			Multiplicity: mult,
			Body:         members,
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	} else {
		// No body - expect semicolon
		p.expect(lexer.Semicolon, "expected ';' or '{' after subject declaration")
		
		node := &ast.SubjectMember{
			Name:         name,
			TypeRef:      typeRef,
			Multiplicity: mult,
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}
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
	
	// Check for 'require name { body }' pattern
	// If next token is name and peek+1 is '{', parse as named requirement with body
	if p.atName() && p.peekN(1).Kind == lexer.LBrace {
		nameToken := p.peek()
		name := p.src.Text(nameToken.Span)
		p.advance() // consume name
		
		// Parse body
		p.expect(lexer.LBrace, "expected '{' after require name")
		members := p.parseRequirementBody()
		
		node := &ast.RequireMember{
			Name:    name,
			Body:    members,
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}
	
	// Otherwise parse as expression: require <expr>;
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

// Phase C4: State Body Parsing

// parseStateBody parses the body of a state usage.
// Expects '{' already consumed, returns list of state members.
func (p *Parser) parseStateBody() []ast.Node {
	var members []ast.Node
	
	for !p.at(lexer.RBrace) && !p.atEOF() {
		members = append(members, p.parseStateMember())
	}
	
	p.expect(lexer.RBrace, "expected '}' after state body")
	return members
}

// parseStateMember parses one state member: entry/do/exit/state/transition, or general body member.
func (p *Parser) parseStateMember() ast.Node {
	start := p.peek().Span.Offset
	
	// Handle doc keyword specially (parseDocumentation consumes it)
	if p.atKeyword("doc") {
		return p.parseDocumentation(start)
	}
	
	// Check for state-specific keywords first
	if p.at(lexer.Keyword) {
		tok := p.peek()
		kw := tok.KeywordID
		
		switch kw {
		case "entry":
			p.advance()
			return p.parseEntryMember(start)
		case "do":
			p.advance()
			return p.parseDoMember(start)
		case "exit":
			p.advance()
			return p.parseExitMember(start)
		case "state":
			p.advance()
			return p.parseSubstateMember(start)
		case "transition":
			p.advance()
			return p.parseTransitionMember(start)
		}
	}
	
	// Not a state-specific keyword - try parsing as general body member
	// This allows succession, binding, feature declarations, etc. in state bodies
	return p.parseBodyMember()
}

// parseEntryMember parses: entry { <actions> }
func (p *Parser) parseEntryMember(start int) ast.Node {
	// 'entry' already consumed
	
	// Expect '{'
	if !p.at(lexer.LBrace) {
		p.error(p.peek().Span, "expected '{' after 'entry'")
		en := &ast.ErrorNode{Message: "expected '{' after 'entry'"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	p.advance() // consume '{'
	
	// Parse action sequence (reuse action body parsing logic)
	var actions []ast.Node
	for !p.at(lexer.RBrace) && !p.atEOF() {
		actions = append(actions, p.parseActionMember())
	}
	
	p.expect(lexer.RBrace, "expected '}' after entry actions")
	
	node := &ast.EntryMember{
		Actions: actions,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseDoMember parses: do { <actions> }
func (p *Parser) parseDoMember(start int) ast.Node {
	// 'do' already consumed
	
	// Expect '{'
	if !p.at(lexer.LBrace) {
		p.error(p.peek().Span, "expected '{' after 'do'")
		en := &ast.ErrorNode{Message: "expected '{' after 'do'"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	p.advance() // consume '{'
	
	// Parse action sequence
	var actions []ast.Node
	for !p.at(lexer.RBrace) && !p.atEOF() {
		actions = append(actions, p.parseActionMember())
	}
	
	p.expect(lexer.RBrace, "expected '}' after do actions")
	
	node := &ast.DoMember{
		Actions: actions,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseExitMember parses: exit { <actions> }
func (p *Parser) parseExitMember(start int) ast.Node {
	// 'exit' already consumed
	
	// Expect '{'
	if !p.at(lexer.LBrace) {
		p.error(p.peek().Span, "expected '{' after 'exit'")
		en := &ast.ErrorNode{Message: "expected '{' after 'exit'"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	p.advance() // consume '{'
	
	// Parse action sequence
	var actions []ast.Node
	for !p.at(lexer.RBrace) && !p.atEOF() {
		actions = append(actions, p.parseActionMember())
	}
	
	p.expect(lexer.RBrace, "expected '}' after exit actions")
	
	node := &ast.ExitMember{
		Actions: actions,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseSubstateMember parses: state <name>;
func (p *Parser) parseSubstateMember(start int) ast.Node {
	// 'state' already consumed
	
	// Expect identifier
	if !p.at(lexer.Identifier) {
		p.error(p.peek().Span, "expected identifier after 'state'")
		en := &ast.ErrorNode{Message: "expected identifier after 'state'"}
		if !p.atEOF() && !p.at(lexer.RBrace) {
			p.advance()
		}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	
	nameToken := p.peek()
	name := p.src.Text(nameToken.Span)
	p.advance()
	
	p.expect(lexer.Semicolon, "expected ';' after state name")
	
	node := &ast.SubstateMember{
		Name: name,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseTransitionMember parses: transition <source> to <target> [when <trigger>] [if <guard>] [do { <effect> }];
func (p *Parser) parseTransitionMember(start int) ast.Node {
	// 'transition' already consumed
	
	// Parse source state
	source := p.parseQualifiedName()
	
	// Expect 'to'
	if !p.atKeyword("to") {
		p.error(p.peek().Span, "expected 'to' after transition source")
		en := &ast.ErrorNode{Message: "expected 'to' after transition source"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	p.advance() // consume 'to'
	
	// Parse target state
	target := p.parseQualifiedName()
	
	// Optional: when <trigger>
	var trigger ast.Node
	if p.atKeyword("when") {
		p.advance() // consume 'when'
		trigger = p.ParseExpression() // simplified: parse as expression (could be time/change/accept/call)
	}
	
	// Optional: if <guard>
	var guard ast.Node
	if p.atKeyword("if") {
		p.advance() // consume 'if'
		guard = p.ParseExpression()
	}
	
	// Optional: do { <effect> }
	var effect []ast.Node
	if p.atKeyword("do") {
		p.advance() // consume 'do'
		
		if !p.at(lexer.LBrace) {
			p.error(p.peek().Span, "expected '{' after 'do'")
			en := &ast.ErrorNode{Message: "expected '{' after 'do'"}
			en.NodeSpan = p.spanFrom(start)
			return en
		}
		p.advance() // consume '{'
		
		// Parse effect actions
		for !p.at(lexer.RBrace) && !p.atEOF() {
			effect = append(effect, p.parseActionMember())
		}
		
		p.expect(lexer.RBrace, "expected '}' after effect actions")
	}
	
	p.expect(lexer.Semicolon, "expected ';' after transition")
	
	node := &ast.TransitionMember{
		Source:  source,
		Target:  target,
		Trigger: trigger,
		Guard:   guard,
		Effect:  effect,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}
