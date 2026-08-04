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
// parseActionBodyMixed handles action bodies with BOTH declarations and behavioral statements
// Syntax: { in item x; action nested {...}; first nested then ...; flow ...; }
func (p *Parser) parseActionBodyMixed() []ast.Node {
	var members []ast.Node
	
	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset
		
		// Try parsing as direction parameter (in/out/inout item/accept/via)
		if p.isDirectionKeyword() {
			members = append(members, p.parseDirectionParameter())
			continue
		}
		
		// Check for nested action DECLARATION (action name : Type {...} or action name {...})
		// vs behavioral action node (action ref or action {expr})
		if p.atKeyword("action") {
			// Lookahead to distinguish:
			// - action <id> : Type { ... } = declaration (typing)
			// - action <id> { stmts } = declaration (multi-statement body)
			// - action <id> { expr } = behavioral node (inline expression)
			// - action { expr } = behavioral node
			// - action <id>; = behavioral node (reference)
			// Check for typing colon OR declaration-like body content
			tok1 := p.peekN(1)
			if tok1.Kind == lexer.Identifier || tok1.Kind == lexer.Keyword {
				tok2 := p.peekN(2)
				// If colon after name → definitely declaration (typing)
				if tok2.Kind == lexer.Colon {
					m := p.parseBodyMember()
					if m != nil {
						if p.atKeyword("then") {
							p.advance()
							if mem, ok := m.(*ast.Membership); ok {
								mem.HasSuccession = true
							}
						}
						members = append(members, m)
					}
					continue
				}
				// If 'accept' keyword after name → declaration (accept action)
				// Pattern: action <name> accept <param> : Type [via <port>];
				if tok2.Kind == lexer.Keyword && tok2.KeywordID == "accept" {
					m := p.parseBodyMember()
					if m != nil {
						if p.atKeyword("then") {
							p.advance()
							if mem, ok := m.(*ast.Membership); ok {
								mem.HasSuccession = true
							}
						}
						members = append(members, m)
					}
					continue
				}
				// If behavioral keyword after name → declaration with inline statement
				// Pattern: action <name> send <msg> to <target>;
				//          action <name> perform <ref>;
				if tok2.Kind == lexer.Keyword && (tok2.KeywordID == "send" || tok2.KeywordID == "terminate" || 
					tok2.KeywordID == "perform" || tok2.KeywordID == "bind" || tok2.KeywordID == "assign") {
					m := p.parseBodyMember()
					if m != nil {
						if p.atKeyword("then") {
							p.advance()
							if mem, ok := m.(*ast.Membership); ok {
								mem.HasSuccession = true
							}
						}
						members = append(members, m)
					}
					continue
				}
				// If brace after name, peek inside
				if tok2.Kind == lexer.LBrace {
					firstInBrace := p.peekN(3)
					isDeclaration := false
					if firstInBrace.Kind == lexer.Keyword {
						kw := firstInBrace.KeywordID
						// Direction keywords, behavioral keywords, or nested declarations suggest declaration body
						if kw == "in" || kw == "out" || kw == "inout" || kw == "action" || 
							kw == "part" || kw == "item" || kw == "flow" || kw == "doc" ||
							kw == "perform" || kw == "send" || kw == "assign" || kw == "first" {
							isDeclaration = true
						}
					}
					if isDeclaration {
						m := p.parseBodyMember()
						if m != nil {
							if p.atKeyword("then") {
								p.advance()
								if mem, ok := m.(*ast.Membership); ok {
									mem.HasSuccession = true
								}
							}
							members = append(members, m)
						}
						continue
					}
				}
			}
			// Otherwise: treat as behavioral action node
			members = append(members, p.parseActionMember())
			continue
		}
		
		// Try parsing as behavioral statement
		// Special case: 'then action' could be succession OR declaration with succession
		// Check if it's 'then action name : Type' (declaration) vs 'then action ref' (behavioral)
		if p.atKeyword("then") && p.peekN(1).Kind == lexer.Keyword && p.peekN(1).KeywordID == "action" {
			// Lookahead: then action <id> : → declaration with succession
			if p.peekN(2).Kind == lexer.Identifier {
				tok3 := p.peekN(3)
				if tok3.Kind == lexer.Colon || (tok3.Kind == lexer.LBrace && p.peekN(4).Kind == lexer.Keyword) {
					// It's declaration: consume 'then', parse declaration, mark succession
					p.advance() // consume 'then'
					m := p.parseBodyMember()
					if m != nil {
						if mem, ok := m.(*ast.Membership); ok {
							mem.HasSuccession = true
						}
						members = append(members, m)
					}
					continue
				}
			}
		}
		
		if p.isBehavioralKeyword() {
			members = append(members, p.parseActionMember())
			continue
		}
		
		// Try parsing as body member (nested declarations)
		m := p.parseBodyMember()
		if m != nil {
			// Check for namespace-level succession: 'then' after member
			if p.atKeyword("then") {
				p.advance() // consume 'then'
				if mem, ok := m.(*ast.Membership); ok {
					mem.HasSuccession = true
				}
			}
			members = append(members, m)
		}
		
		// Ensure progress
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.error(p.peek().Span, "expected a body member")
			p.advance()
		}
	}
	
	p.expect(lexer.RBrace, "expected '}' after action body")
	return members
}

// parseActionBody handles pure behavioral bodies (legacy - for inline action statements)
func (p *Parser) parseActionBody() []ast.Node {
	var members []ast.Node
	
	for !p.at(lexer.RBrace) && !p.atEOF() {
		members = append(members, p.parseActionMember())
	}
	
	p.expect(lexer.RBrace, "expected '}' after action body")
	return members
}

// isDirectionKeyword checks if current token is in/out/inout
func (p *Parser) isDirectionKeyword() bool {
	if !p.at(lexer.Keyword) {
		return false
	}
	kw := p.peek().KeywordID
	return kw == "in" || kw == "out" || kw == "inout"
}

// parseDirectionParameter parses: <direction> [ref] [<kind>] [<name>] [: <type>] [= <value>];
// Examples: in item scene; out feature x; in ref item x : Foo = bar; in item; in target;
// Kind keyword is optional - defaults to generic feature
func (p *Parser) parseDirectionParameter() ast.Node {
	start := p.peek().Span.Offset
	
	// Parse direction (in/out/inout)
	dirTok, _ := p.expect(lexer.Keyword, "expected direction keyword")
	var direction ast.FeatureDirection
	switch dirTok.KeywordID {
	case "in":
		direction = ast.DirIn
	case "out":
		direction = ast.DirOut
	case "inout":
		direction = ast.DirInOut
	default:
		direction = ast.DirNone
	}
	
	// Check for optional 'ref' modifier
	isRef := false
	if p.atKeyword("ref") {
		p.advance()
		isRef = true
	}
	
	// Check for optional individual/snapshot/event modifiers
	isIndividual := false
	isSnapshot := false
	isEvent := false
	if p.atKeyword("individual") {
		p.advance()
		isIndividual = true
	} else if p.atKeyword("snapshot") {
		p.advance()
		isSnapshot = true
	} else if p.atKeyword("event") {
		p.advance()
		isEvent = true
	}
	
	// Parse optional kind keyword (item, feature, port, etc)
	// If next token is keyword and recognized kind, consume it
	// Otherwise treat next token as name (kind defaults to generic feature)
	var kind ast.UsageKind = ast.UsagePart // default to generic feature
	if p.at(lexer.Keyword) {
		kindKeyword := p.peek().KeywordID
		recognized := false
		switch kindKeyword {
		case "item":
			kind = ast.UsageItem
			recognized = true
		case "feature":
			kind = ast.UsagePart
			recognized = true
		case "port":
			kind = ast.UsagePort
			recognized = true
		case "part":
			kind = ast.UsagePart
			recognized = true
		case "attribute":
			kind = ast.UsageAttribute
			recognized = true
		case "occurrence":
			kind = ast.UsageOccurrence
			recognized = true
		case "action":
			kind = ast.UsageAction
			recognized = true
		}
		if recognized {
			p.advance() // consume kind keyword
		}
		// If not recognized, leave it as name (kind stays default)
	}
	
	// Optional name (can be anonymous: "in item;")
	var ident ast.Identification
	// Skip name if next token is relationship operator, value assignment, or semicolon
	isRelationshipNext := p.at(lexer.Colon) || p.at(lexer.ColonGt) || p.at(lexer.ColonGtGt) || 
		p.at(lexer.ColonColonGt) || p.at(lexer.ColonEq)
	if p.atName() && !isRelationshipNext && !p.at(lexer.Eq) && !p.at(lexer.Semicolon) {
		ident = p.parseIdentification()
	}
	
	// Optional multiplicity before relationships (e.g., name[mult]: Type)
	var multiplicity *ast.Multiplicity
	if p.at(lexer.LBracket) {
		multiplicity = p.parseMultiplicity()
	}
	
	// Optional typing and relationships (: Type, :> SuperType, ::> Redefines, etc)
	var relationships []*ast.Relationship
	if p.at(lexer.Colon) || p.at(lexer.ColonGt) || p.at(lexer.ColonGtGt) || 
		p.at(lexer.ColonColonGt) || p.at(lexer.ColonEq) {
		rels, _ := p.parseRelationships(true)
		relationships = append(relationships, rels...)
	}
	
	// Optional multiplicity after relationships if not already parsed (e.g., :> target[mult])
	if multiplicity == nil && p.at(lexer.LBracket) {
		multiplicity = p.parseMultiplicity()
	}
	
	// Parse post-multiplicity modifiers (ordered/nonunique)
	postMods := p.parsePostModifiers()
	
	// Optional value (= expr, := expr, or default expr)
	var value ast.Node
	if p.accept2(lexer.Eq) || p.accept2(lexer.ColonEq) || p.acceptKeyword("default") {
		value = p.ParseExpression()
	}
	
	// Optional body or semicolon
	var members []ast.Node
	var hasBody bool
	if p.accept2(lexer.Semicolon) {
		hasBody = false
	} else if p.at(lexer.LBrace) {
		p.advance() // consume '{'
		// Parse body members generically
		for !p.at(lexer.RBrace) && !p.atEOF() {
			m := p.parseBodyMember()
			if m != nil {
				members = append(members, m)
			}
		}
		p.expect(lexer.RBrace, "expected '}'")
		hasBody = true
	} else {
		p.error(p.peek().Span, "expected ';' or '{' after parameter")
	}
	
	// Create Usage node with direction
	usage := &ast.Usage{
		Kind:          kind,
		Ident:         ident,
		Relationships: relationships,
		Multiplicity:  multiplicity,
		Value:         value,
		Members:       members,
		HasBody:       hasBody,
		IsReference:   isRef,
		Direction:     direction,
		IsOrdered:     postMods.isOrdered,
		IsNonunique:   postMods.isNonunique,
		IsEvent:       isEvent,
	}
	// Note: isIndividual, isSnapshot consumed but not stored in AST (no fields yet)
	_, _ = isIndividual, isSnapshot
	usage.NodeSpan = p.spanFrom(start)
	
	// Wrap in Membership
	membership := &ast.Membership{
		Member: usage,
	}
	membership.NodeSpan = usage.NodeSpan
	
	return membership
}

// parseActionMember parses one action member: node, edge, or nested declaration.
func (p *Parser) parseActionMember() ast.Node {
	start := p.peek().Span.Offset
	
	// Try general declaration first (nested actions, features, etc.)
	if node := p.tryParseDeclaration(); node != nil {
		return node
	}
	
	// Handle doc keyword specially (parseDocumentation consumes it)
	if p.atKeyword("doc") {
		return p.parseDocumentation(start)
	}
	
	// Check for keyword dispatch
	if tok, ok := p.accept(lexer.Keyword); ok {
		kw := tok.KeywordID
		switch kw {
		case "first", "initial":
			return p.parseInitialNode(tok)
		case "done", "final":
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
		case "assign":
			return p.parseAssignmentAction(tok)
		case "perform":
			return p.parsePerformAction(tok)
		case "while":
			return p.parseWhileLoopAction(tok)
		case "loop":
			return p.parseLoopAction(tok)
		case "for":
			return p.parseForAction(tok)
		case "if":
			return p.parseIfAction(tok)
		case "send":
			return p.parseSendStatement(tok)
		case "terminate":
			return p.parseTerminateStatement(tok)
		default:
			// Unknown keyword - try as general body member
			// Restore checkpoint since we consumed keyword
			p.error(tok.Span, "unknown action keyword: "+kw)
			en := &ast.ErrorNode{Message: "unknown action keyword: " + kw}
			en.NodeSpan = p.spanFrom(start)
			return en
		}
	}
	
	// Check for bare assignment: identifier = expr; or identifier := expr;
	// This is shorthand for assign identifier = expr;
	if p.at(lexer.Identifier) {
		nextTok := p.peekN(1)
		if nextTok.Kind == lexer.Eq || nextTok.Kind == lexer.ColonEq {
			// Parse as assignment
			target := p.parseQualifiedName()
			p.advance() // consume = or :=
			value := p.ParseExpression()
			p.expect(lexer.Semicolon, "expected ';' after assignment")
			
			node := &ast.AssignmentActionNode{
				Target: target,
				Value:  value,
			}
			node.NodeSpan = p.spanFrom(start)
			return node
		}
	}
	
	// Graceful fallback: try parseBodyMember for remaining constructs
	if node := p.parseBodyMember(); node != nil {
		return node
	}
	
	// Last resort: report error but don't crash
	p.error(p.peek().Span, "expected action member")
	en := &ast.ErrorNode{Message: "expected action member"}
	if !p.atEOF() && !p.at(lexer.RBrace) {
		p.advance() // ensure progress
	}
	en.NodeSpan = p.spanFrom(start)
	return en
}

// Action node parsers — Task 9 complete. Task 10 (ActionExecutionNode + SuccessionEdge) below.

func (p *Parser) parseInitialNode(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	var name string
	
	if p.at(lexer.Identifier) || p.atNameOrKeyword() {
		nameToken := p.peek()
		name = p.src.Text(nameToken.Span)
		p.advance()
	}
	
	// Check for succession edge continuation: first X [if <expr>] then Y;
	var guard ast.Node
	if p.atKeyword("if") {
		p.advance() // consume 'if'
		guard = p.ParseExpression()
	}
	
	var successor *ast.QualifiedName
	if p.atKeyword("then") {
		p.advance() // consume 'then'
		successor = p.parseQualifiedName()
	}
	
	// If guard present but no then, error
	if guard != nil && successor == nil {
		p.error(p.peek().Span, "expected 'then' after guard condition")
	}
	
	p.expect(lexer.Semicolon, "expected ';' after initial node")
	
	node := &ast.InitialNode{
		Name:      name,
		Successor: successor,
		Guard:     guard,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

func (p *Parser) parseFinalNode(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	var name string
	
	if p.atNameOrKeyword() {
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
		} else if nextTok.Kind == lexer.Identifier || nextTok.Kind == lexer.ColonColon || nextTok.Kind == lexer.Keyword {
			// Could be name + ref OR just ref (qualified name)
			// Also handle keywords as refs (e.g., action stop terminate;)
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
			} else if p.at(lexer.Identifier) || p.at(lexer.Keyword) {
				// firstId is name, what follows is actionRef (identifier or keyword)
				name = firstId
				if p.at(lexer.Keyword) {
					// Allow keywords as action refs (e.g., 'terminate')
					kw := p.peek()
					actionRef = &ast.QualifiedName{
						Parts: []ast.NameSegment{{Text: kw.KeywordID, Span: kw.Span}},
					}
					actionRef.NodeSpan = kw.Span
					p.advance()
				} else {
					actionRef = p.parseQualifiedName()
				}
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

// parseSuccessionEdge parses: 
// 1. then source target [if guard] ; (control flow edge between named nodes)
// 2. then statement (inline statement succession)
func (p *Parser) parseSuccessionEdge(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	
	// Check if this is inline statement succession (then followed by behavioral keyword)
	// Pattern: then assign x := 1; OR then perform foo;
	if p.at(lexer.Keyword) {
		kw := p.peek().KeywordID
		if kw == "assign" || kw == "perform" || kw == "while" || kw == "if" || kw == "action" {
			// This is inline succession: parse following statement
			return p.parseActionMember()
		}
	}
	
	// Otherwise, parse as named edge: then [source] target;
	// If only one name before semicolon, it's the target (source implicit)
	first := p.parseQualifiedNameRelaxed() // allow keywords like 'fork', 'join'
	
	// Check if there's a second name (explicit source + target)
	var source, target *ast.QualifiedName
	if !p.at(lexer.Semicolon) && !p.atKeyword("if") && (p.at(lexer.Identifier) || p.at(lexer.Keyword)) {
		// Two-name form: then source target;
		source = first
		target = p.parseQualifiedNameRelaxed()
	} else {
		// One-name form: then target; (source implicit)
		source = &ast.QualifiedName{} // empty source
		target = first
	}
	
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

// parseAssignmentAction parses: assign target := value;
func (p *Parser) parseAssignmentAction(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	
	// Parse target (feature reference or qualified name)
	target := p.ParseExpression()
	
	// Expect ':=' operator
	if !p.at(lexer.ColonEq) {
		p.error(p.peek().Span, "expected ':=' after assignment target")
		return &ast.ErrorNode{
			NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)},
			Message:  "expected ':=' after assignment target",
		}
	}
	p.advance() // consume ':='
	
	// Parse value expression
	value := p.ParseExpression()
	
	p.expect(lexer.Semicolon, "expected ';' after assignment")
	
	node := &ast.AssignmentActionNode{
		Target: target,
		Value:  value,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parsePerformAction parses: perform action;
func (p *Parser) parsePerformAction(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	
	// Parse action reference (qualified name or invocation)
	actionRef := p.ParseExpression()
	
	p.expect(lexer.Semicolon, "expected ';' after perform statement")
	
	node := &ast.PerformActionNode{
		ActionRef: actionRef,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseWhileLoopAction parses: while condition { statements }
func (p *Parser) parseWhileLoopAction(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	
	// Parse condition expression
	condition := p.ParseExpression()
	
	// Expect '{'
	if !p.at(lexer.LBrace) {
		p.error(p.peek().Span, "expected '{' after while condition")
		return &ast.ErrorNode{
			NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)},
			Message:  "expected '{' after while condition",
		}
	}
	p.advance() // consume '{'
	
	// Parse body statements
	var body []ast.Node
	for !p.at(lexer.RBrace) && !p.atEOF() {
		body = append(body, p.parseActionMember())
	}
	
	p.expect(lexer.RBrace, "expected '}' after while body")
	
	node := &ast.WhileLoopActionNode{
		Condition: condition,
		Body:      body,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

func (p *Parser) parseLoopAction(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	
	// Loop syntax: loop <body-members> until <condition>;
	// Body can contain action declarations, succession, etc.
	// Parse body as mixed content (declarations + behavioral statements)
	var body []ast.Node
	
	// Parse loop body members until 'until' keyword or closing brace
	for !p.atKeyword("until") && !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset
		
		// Try direction parameters first
		if p.isDirectionKeyword() {
			body = append(body, p.parseDirectionParameter())
			continue
		}
		
		// Try declarations (action/part/etc)
		if p.atDefUsageStart() {
			m := p.parseBodyMember()
			if m != nil {
				body = append(body, m)
			}
			// Check if no progress (prevent infinite loop)
			if p.peek().Span.Offset == before {
				p.advance()
			}
			continue
		}
		
		// Parse behavioral statements
		body = append(body, p.parseActionMember())
		
		// Ensure progress
		if p.peek().Span.Offset == before {
			p.advance()
		}
	}
	
	// Parse optional 'until' condition
	var condition ast.Node
	if p.acceptKeyword("until") {
		condition = p.ParseExpression()
	}
	
	p.expect(lexer.Semicolon, "expected ';' after loop")
	
	// Reuse WhileLoopActionNode with condition (will be post-condition for 'until')
	// Alternatively, could add separate LoopActionNode with UntilCondition field
	node := &ast.WhileLoopActionNode{
		Condition: condition,
		Body:      body,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseForAction parses: for <variable> in <collection> { <body> }
func (p *Parser) parseForAction(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	
	// Parse iteration variable name
	varTok, ok := p.expect(lexer.Identifier, "expected variable name after 'for'")
	if !ok {
		en := &ast.ErrorNode{Message: "expected variable name"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	
	// Expect 'in' keyword
	if !p.acceptKeyword("in") {
		p.error(p.peek().Span, "expected 'in' keyword after for variable")
		en := &ast.ErrorNode{Message: "expected 'in'"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	
	// Parse collection expression
	collection := p.ParseExpression()
	
	// Parse body (braced)
	if _, ok := p.expect(lexer.LBrace, "expected '{' for for-loop body"); !ok {
		en := &ast.ErrorNode{Message: "expected '{'"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	
	// Parse body as mixed content (declarations + behavioral statements)
	var body []ast.Node
	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset
		
		// Try direction parameters
		if p.isDirectionKeyword() {
			body = append(body, p.parseDirectionParameter())
			continue
		}
		
		// Try declarations
		if p.atDefUsageStart() {
			m := p.parseBodyMember()
			if m != nil {
				body = append(body, m)
			}
			if p.peek().Span.Offset == before {
				p.advance()
			}
			continue
		}
		
		// Parse behavioral statements
		body = append(body, p.parseActionMember())
		
		// Ensure progress
		if p.peek().Span.Offset == before {
			p.advance()
		}
	}
	
	p.expect(lexer.RBrace, "expected '}'")
	
	// Create ForLoopActionNode AST node
	// Since this doesn't exist yet, reuse WhileLoopActionNode structure
	// Store variable name and collection in condition (hack - proper impl would add ForLoopActionNode)
	// For now, create simple FeatureReference for variable and store as body member
	varUsage := &ast.Usage{
		Kind: ast.UsageAttribute,
		Ident: ast.Identification{
			Name:     p.src.Text(varTok.Span),
			NameSpan: varTok.Span,
		},
		Value: collection, // Store collection as value
	}
	varUsage.NodeSpan = varTok.Span
	
	// Prepend variable usage to body
	body = append([]ast.Node{varUsage}, body...)
	
	node := &ast.WhileLoopActionNode{
		Condition: nil, // No explicit condition (iteration driven)
		Body:      body,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseIfAction parses: if condition { thenBody } [else { elseBody }]
func (p *Parser) parseIfAction(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	
	// Parse condition expression
	condition := p.ParseExpression()
	
	// Check for shorthand guard succession: if <expr> then <target>;
	if p.atKeyword("then") {
		p.advance() // consume 'then'
		target := p.parseQualifiedName()
		p.expect(lexer.Semicolon, "expected ';' after guard succession")
		
		// Return ControlFlowEdge with implicit source (decision node) and guard
		node := &ast.ControlFlowEdge{
			Source: &ast.QualifiedName{}, // empty source = implicit decision node
			Target: target,
			Guard:  condition,
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}
	
	// Expect '{'
	if !p.at(lexer.LBrace) {
		p.error(p.peek().Span, "expected '{' after if condition")
		return &ast.ErrorNode{
			NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)},
			Message:  "expected '{' after if condition",
		}
	}
	p.advance() // consume '{'
	
	// Parse then body statements
	var thenBody []ast.Node
	for !p.at(lexer.RBrace) && !p.atEOF() {
		// Use parseActionBodyMixed to support both declarations and behavioral statements
		// This allows: if <cond> { action x : Type { body }; first x then y; }
		before := p.peek().Span.Offset
		
		// Try direction parameters first
		if p.isDirectionKeyword() {
			thenBody = append(thenBody, p.parseDirectionParameter())
			continue
		}
		
		// Try declarations (action/part/etc)
		if p.atDefUsageStart() {
			m := p.parseBodyMember()
			if m != nil {
				thenBody = append(thenBody, m)
			}
			// Check if no progress (prevent infinite loop)
			if p.peek().Span.Offset == before {
				p.advance()
			}
			continue
		}
		
		// Parse behavioral statements
		thenBody = append(thenBody, p.parseActionMember())
	}
	
	p.expect(lexer.RBrace, "expected '}' after if body")
	
	// Check for optional 'else' clause
	var elseBody []ast.Node
	if p.acceptKeyword("else") {
		if !p.at(lexer.LBrace) {
			p.error(p.peek().Span, "expected '{' after else")
		} else {
			p.advance() // consume '{'
			for !p.at(lexer.RBrace) && !p.atEOF() {
				before := p.peek().Span.Offset
				
				// Try direction parameters first
				if p.isDirectionKeyword() {
					elseBody = append(elseBody, p.parseDirectionParameter())
					continue
				}
				
				// Try declarations
				if p.atDefUsageStart() {
					m := p.parseBodyMember()
					if m != nil {
						elseBody = append(elseBody, m)
					}
					if p.peek().Span.Offset == before {
						p.advance()
					}
					continue
				}
				
				// Parse behavioral statements
				elseBody = append(elseBody, p.parseActionMember())
			}
			p.expect(lexer.RBrace, "expected '}' after else body")
		}
	}
	
	node := &ast.IfActionNode{
		Condition: condition,
		ThenBody:  thenBody,
		ElseBody:  elseBody,
	}
	node.NodeSpan = p.spanFrom(start)
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
	
	// Check for leading relationship operators before name (e.g., return ::> result : Type)
	// In this context, relationships apply to the parameter being defined (not to external target)
	// So we consume the operator but DON'T parse target yet
	var leadingRels []*ast.Relationship
	for p.at(lexer.ColonGtGt) || p.at(lexer.ColonGt) || p.at(lexer.ColonColonGt) || p.at(lexer.ColonColon) {
		tok := p.peek()
		p.advance() // consume relationship operator
		
		// Map token to relationship kind
		var kind ast.RelationshipKind
		switch tok.Kind {
		case lexer.ColonGtGt:
			kind = ast.RelRedefines
		case lexer.ColonGt:
			kind = ast.RelSpecializes
		case lexer.ColonColonGt:
			kind = ast.RelReferences
		case lexer.ColonColon:
			kind = ast.RelTyping // or namespace qualification - context dependent
		}
		
		// Create relationship with nil target (will be filled by parent scope context)
		rel := &ast.Relationship{Kind: kind, Target: nil}
		leadingRels = append(leadingRels, rel)
	}
	
	// Check for named or anonymous result parameter syntax
	// Pattern 1: return [modifiers] name: Type[mult];  (named result parameter)
	// Pattern 2: return [modifiers] : Type[mult];      (anonymous result parameter)
	// Pattern 3: return expr;              (computed result)
	// Pattern 4: return name = expr;       (computed result with binding)
	// Pattern 5: return [modifiers] name;  (named result parameter, no type)
	// Pattern 6: return [modifiers] name : Type { body } (with body)
	// Pattern 7: return :>> name : Type = expr; (leading relationships before name)
	// Use lookahead to distinguish Pattern 1 from Pattern 4
	if len(leadingRels) > 0 || p.at(lexer.Colon) || (p.atName() && p.peekN(1).Kind == lexer.Colon) {
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
		
		// Add leading relationships if any (e.g., ::> from `return ::> result`)
		u.Relationships = append(u.Relationships, leadingRels...)
		
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
		
		// Parse additional relationships after post-modifiers (e.g., redefines result redefines values)
		postModRels, postConj := p.parseRelationships(true)
		u.Relationships = append(u.Relationships, postModRels...)
		if postConj {
			u.IsConjugated = true
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
	// Only match if modifiers present OR followed by multiplicity - bare 'return name;' is computed result (Pattern 3)
	hasModifiers := mods.isAbstract || mods.isReference || mods.isEnd || mods.isConstant ||
		mods.isComposite || mods.isDerived || mods.isReadonly || mods.isOrdered || mods.isNonunique
	hasMultiplicity := p.peekN(1).Kind == lexer.LBracket
	if (hasModifiers || hasMultiplicity) && p.atName() && (p.peekN(1).Kind == lexer.Semicolon || p.peekN(1).Kind == lexer.LBracket) {
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
		} else if p.atKeyword("assert") || p.atKeyword("assume") {
			// Parse constraint expression (assert/assume)
			members = append(members, p.parseConstraintMember())
		} else if p.atKeyword("return") {
			// Parse return member (for constraint defs that return result)
			// Example: return result = expr { doc }
			members = append(members, p.parseBodyMember())
		} else if p.atDefUsageStart() {
			// Definition/usage keyword - parse as body member
			members = append(members, p.parseBodyMember())
		} else if p.atKeyword("redefines") || p.atKeyword("subsets") || p.atKeyword("specializes") || p.atKeyword("references") {
			// Relationship keyword (e.g., redefines partMasses = expr;)
			// Parse as body member (handles bare relationship statements)
			members = append(members, p.parseBodyMember())
		} else if p.at(lexer.Colon) || p.at(lexer.ColonGt) || p.at(lexer.ColonGtGt) || p.at(lexer.ColonColonGt) {
			// Relationship keyword without name (e.g., :>> x = value;)
			// Parse as anonymous feature with relationship
			members = append(members, p.parseBodyMember())
		} else {
			// Default: parse as constraint expression (bare expression)
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

// parseRequirementMember parses one requirement member: subject/assume/require/actor/doc or general body members
func (p *Parser) parseRequirementMember() ast.Node {
	start := p.peek().Span.Offset
	
	// Check for requirement-specific keywords FIRST (before tryParseDeclaration)
	// These keywords have special meaning in requirement context that differs from general usage
	if p.acceptKeyword("subject") {
		return p.parseSubjectMember(start)
	}
	if p.acceptKeyword("assume") {
		return p.parseAssumeMember(start)
	}
	if p.acceptKeyword("require") {
		return p.parseRequireMember(start)
	}
	if p.acceptKeyword("actor") {
		return p.parseActorMember(start)
	}
	
	// Try general declaration (nested requirements, features, etc.)
	if node := p.tryParseDeclaration(); node != nil {
		// Validate that tryParseDeclaration didn't just accept garbage
		// Example: "require ;" gets parsed as anonymous constraint usage by tryParseDeclaration
		if usage, ok := node.(*ast.Usage); ok {
			hasName := usage.Ident.Name != ""
			hasType := len(usage.Relationships) > 0
			if hasType && usage.Relationships[0].Kind != ast.RelTyping {
				hasType = false
			}
			hasValue := usage.Value != nil
			hasMembers := len(usage.Members) > 0
			
			// Anonymous usage with nothing meaningful - likely a keyword we should handle specially
			if !hasName && !hasType && !hasValue && !hasMembers {
				// Don't accept this, fall through to fallback below
			} else {
				return node // Valid declaration, use it
			}
		} else {
			return node // Non-usage node, trust it
		}
	}
	
	// Graceful fallback: parse as general body member (expression, statement, etc.)
	// This handles any legal construct we haven't explicitly enumerated
	node := p.parseBodyMember()
	
	// Validate fallback didn't just accept garbage
	// If we got an ErrorNode, the parser already diagnosed it
	if _, isError := node.(*ast.ErrorNode); isError {
		return node
	}
	
	// If fallback produced something suspicious, diagnose it
	// Example: anonymous usage with no relationships (like bare "require ;")
	if usage, ok := node.(*ast.Usage); ok {
		hasName := usage.Ident.Name != ""
		hasType := len(usage.Relationships) > 0
		if hasType && usage.Relationships[0].Kind != ast.RelTyping {
			hasType = false // Only count typing relationships
		}
		hasValue := usage.Value != nil
		hasMembers := len(usage.Members) > 0
		
		// An anonymous usage with no type, no value, no members is likely garbage
		if !hasName && !hasType && !hasValue && !hasMembers {
			p.error(node.Span(), "expected 'subject', 'assume', 'require', 'actor', or a valid body member")
			return &ast.ErrorNode{Message: "unexpected requirement member"}
		}
	}
	
	return node
}

// parseSubjectMember parses: subject <name> : <Type>; OR subject = <expr>; OR subject <name> = <expr>;
func (p *Parser) parseSubjectMember(start int) ast.Node {
	// 'subject' already consumed
	
	// Check for binding pattern: subject = <expr>; OR subject <name> = <expr>;
	if p.at(lexer.Eq) {
		// Anonymous binding: subject = <expr>;
		p.advance() // consume '='
		
		// Parse value expression
		value := p.ParseExpression()
		
		// Expect semicolon
		p.expect(lexer.Semicolon, "expected ';' after subject binding")
		
		node := &ast.SubjectMember{
			Name:        "", // Empty name means binding inherited subject
			BindingExpr: value,
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}
	
	// Check if next is identifier (could be name + binding or typed declaration)
	var name string
	if p.at(lexer.Identifier) {
		nameToken := p.peek()
		name = p.src.Text(nameToken.Span)
		p.advance()
		
		// Check if followed by '=' (named binding: subject <name> = <expr>;)
		if p.at(lexer.Eq) {
			p.advance() // consume '='
			
			// Parse value expression
			value := p.ParseExpression()
			
			// Expect semicolon
			p.expect(lexer.Semicolon, "expected ';' after subject binding")
			
			node := &ast.SubjectMember{
				Name:        name,
				BindingExpr: value,
			}
			node.NodeSpan = p.spanFrom(start)
			return node
		}
		
		// Otherwise expect ':' for typed declaration
		if !p.at(lexer.Colon) {
			p.error(p.peek().Span, "expected ':' or '=' after subject name")
			en := &ast.ErrorNode{Message: "expected ':' or '=' after subject name"}
			en.NodeSpan = p.spanFrom(start)
			return en
		}
	} else if p.at(lexer.Colon) {
		// Anonymous typed subject: subject: <Type>;
		name = ""
	} else {
		p.error(p.peek().Span, "expected identifier or ':' after 'subject'")
		en := &ast.ErrorNode{Message: "expected identifier or ':' after 'subject'"}
		if !p.atEOF() && !p.at(lexer.RBrace) {
			p.advance()
		}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	
	// Typed declaration: subject [<name>] : <Type>;
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
	
	// Check for 'assume constraint { body }' pattern
	if p.atKeyword("constraint") {
		p.advance() // consume 'constraint'
		
		// Parse constraint body (expect '{')
		if !p.at(lexer.LBrace) {
			p.error(p.peek().Span, "expected '{' after 'assume constraint'")
			en := &ast.ErrorNode{Message: "expected '{' after assume constraint"}
			en.NodeSpan = p.spanFrom(start)
			return en
		}
		p.advance() // consume '{'
		
		// Parse constraint body members (expressions, doc, etc.)
		var expr ast.Node
		for !p.at(lexer.RBrace) && !p.atEOF() {
			// Allow doc comments
			if p.atKeyword("doc") {
				p.parseDocumentation(p.peek().Span.Offset)
				continue
			}
			// Parse constraint expression (store last one)
			constraintMember := p.parseConstraintMember()
			if c, ok := constraintMember.(*ast.ConstraintMember); ok && c.Expression != nil {
				expr = c.Expression
			}
		}
		p.expect(lexer.RBrace, "expected '}' after constraint body")
		
		node := &ast.AssumeMember{
			Expression: expr,
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}
	
	// Otherwise parse as simple expression
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
	
	// Check for 'require constraint { body }' pattern
	if p.atKeyword("constraint") {
		p.advance() // consume 'constraint'
		
		// Parse constraint body (expect '{')
		if !p.at(lexer.LBrace) {
			p.error(p.peek().Span, "expected '{' after 'require constraint'")
			en := &ast.ErrorNode{Message: "expected '{' after require constraint"}
			en.NodeSpan = p.spanFrom(start)
			return en
		}
		p.advance() // consume '{'
		
		// Parse constraint body members (expressions, doc, etc.)
		var expr ast.Node
		for !p.at(lexer.RBrace) && !p.atEOF() {
			// Allow doc comments
			if p.atKeyword("doc") {
				p.parseDocumentation(p.peek().Span.Offset)
				continue
			}
			// Parse constraint expression (store last one)
			constraintMember := p.parseConstraintMember()
			if c, ok := constraintMember.(*ast.ConstraintMember); ok && c.Expression != nil {
				expr = c.Expression
			}
		}
		p.expect(lexer.RBrace, "expected '}' after constraint body")
		
		node := &ast.RequireMember{
			Expression: expr,
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}
	
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

// parseActorMember parses: actor <name> : <Type>; OR actor <name> = <expr>;
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
	
	// Check if followed by '=' (binding form: actor <name> = <expr>;)
	if p.at(lexer.Eq) {
		p.advance() // consume '='
		
		// Parse value expression
		value := p.ParseExpression()
		
		// Expect semicolon
		p.expect(lexer.Semicolon, "expected ';' after actor binding")
		
		node := &ast.ActorMember{
			Name:        name,
			BindingExpr: value,
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}
	
	// Otherwise expect ':' for typed declaration
	if !p.at(lexer.Colon) {
		p.error(p.peek().Span, "expected ':' or '=' after actor name")
		en := &ast.ErrorNode{Message: "expected ':' or '=' after actor name"}
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
		case "initial":
			p.advance()
			return p.parseInitialState(start)
		case "final":
			p.advance()
			return p.parseFinalState(start)
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
			// Check if this is a simple declaration (state name;) or full usage (state name { ... })
			// Lookahead: state followed by name/keyword and semicolon → SubstateMember
			// Otherwise → full state usage declaration
			nextTok := p.peekN(1)
			isNameOrKeyword := nextTok.Kind == lexer.Identifier || nextTok.Kind == lexer.Keyword
			if isNameOrKeyword && p.peekN(2).Kind == lexer.Semicolon {
				p.advance()
				return p.parseSubstateMember(start)
			}
			// Full state usage - parse as body member
			return p.parseBodyMember()
		case "region":
			p.advance()
			return p.parseRegionMember(start)
		case "choice":
			p.advance()
			return p.parseChoicePseudostate(start)
		case "junction":
			p.advance()
			return p.parseJunctionPseudostate(start)
		case "transition":
			// Lookahead to distinguish:
			// 1. State machine transition: transition source to target (no name)
			// 2. Transition usage: transition name first ... (has name + connector syntax)
			// Check if followed by identifier + "first" keyword
			if p.peekN(1).Kind == lexer.Identifier && p.peekN(2).Kind == lexer.Keyword && p.peekN(2).KeywordID == "first" {
				// Transition usage - parse as general body member (declaration)
				return p.parseBodyMember()
			}
			// State machine transition
			p.advance()
			return p.parseTransitionMember(start)
		case "first":
			// Initial node: first <name> then <target>;
			p.advance() // consume 'first'
			return p.parseInitialNode(p.peek())
		case "then":
			// Standalone succession: then <source> <target>; or then <target>; (implicit source)
			p.advance() // consume 'then'
			
			// Parse first qualified name
			firstState := p.parseQualifiedName()
			
			// Check if there's a second state name (explicit source → target)
			var sourceState, targetState *ast.QualifiedName
			if p.at(lexer.Identifier) || p.atNameOrKeyword() {
				// Two states: then source target;
				sourceState = firstState
				targetState = p.parseQualifiedName()
			} else {
				// One state: then target; (implicit source)
				sourceState = nil
				targetState = firstState
			}
			
			p.expect(lexer.Semicolon, "expected ';' after succession target")
			
			// Create succession
			succession := &ast.Usage{
				Kind: ast.UsageSuccession,
				ConnectorEnds: []*ast.ConnectorEnd{
					{Target: sourceState},
					{Target: targetState},
				},
			}
			succession.NodeSpan = p.spanFrom(start)
			return succession
		case "accept":
			// Accept transition: accept <signal> then <state>;
			return p.parseAcceptTransition(start)
		}
	}
	
	// Check for succession statement: <name> then <name>;
	// Lookahead: identifier followed by 'then' keyword
	if p.at(lexer.Identifier) && p.peekN(1).Kind == lexer.Keyword && p.peekN(1).KeywordID == "then" {
		return p.parseSuccessionStatement(start)
	}
	
	// Not a state-specific keyword - try parsing as general body member
	// This allows succession, binding, feature declarations, etc. in state bodies
	return p.parseBodyMember()
}

// parseSuccessionStatement parses: first <state> then <state>;
// This is a succession statement in state body context (defines initial state flow)
func (p *Parser) parseSuccessionStatement(start int) ast.Node {
	// 'first' keyword should be consumed by caller, but check if we're at it
	if p.atKeyword("first") {
		p.advance()
	}
	
	// Parse first state reference (use relaxed parsing to allow keywords like 'off' as names)
	firstState := p.parseQualifiedNameRelaxed()
	
	// Expect 'then' keyword
	if !p.acceptKeyword("then") {
		p.error(p.peek().Span, "expected 'then' after first state")
		en := &ast.ErrorNode{Message: "expected 'then' keyword"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	
	// Parse second state reference (use relaxed parsing to allow keywords like 'on' as names)
	secondState := p.parseQualifiedNameRelaxed()
	
	// Expect semicolon
	p.expect(lexer.Semicolon, "expected ';' after succession statement")
	
	// Create succession usage (reuse existing AST node)
	succession := &ast.Usage{
		Kind: ast.UsageSuccession,
		ConnectorEnds: []*ast.ConnectorEnd{
			{Reference: firstState},
			{Reference: secondState},
		},
	}
	succession.NodeSpan = p.spanFrom(start)
	return succession
}

// parseAcceptTransition parses: accept <signal> then <state>;
// This is a state transition triggered by accepting a signal
func (p *Parser) parseAcceptTransition(start int) ast.Node {
	// 'accept' keyword should be consumed by caller
	if p.atKeyword("accept") {
		p.advance() // consume 'accept' if not already consumed
	}
	
	// Parse signal type reference (use relaxed parsing to allow keywords as names)
	// Could be event type OR temporal expression (at <time>) OR change trigger (when <cond>) OR relative time (after <duration>)
	// OR typed parameter (name : Type)
	var signalType ast.Node
	var isTemporalTransition bool
	var isChangeTransition bool
	if p.atKeyword("at") {
		// Temporal transition: accept at <timeExpr> then <state>
		p.advance() // consume 'at'
		signalType = p.ParseExpression()
		isTemporalTransition = true
	} else if p.atKeyword("after") {
		// Relative time transition: accept after <duration> then <state>
		p.advance() // consume 'after'
		signalType = p.ParseExpression()
		isTemporalTransition = true
	} else if p.atKeyword("when") {
		// Change transition: accept when <condition> then <state>
		p.advance() // consume 'when'
		signalType = p.ParseExpression()
		isChangeTransition = true
	} else {
		// Event transition: accept <signal> OR accept <name> : Type
		// Lookahead: if identifier followed by colon, parse as typed parameter
		if (p.at(lexer.Identifier) || p.at(lexer.Keyword)) && p.peekN(1).Kind == lexer.Colon {
			// Typed trigger: parse name + typing
			paramStart := p.peek().Span.Offset
			ident := p.parseIdentification()
			
			// Parse typing
			rels, _ := p.parseRelationships(true)
			
			// Create attribute usage to represent typed trigger
			usage := &ast.Usage{
				Kind:          ast.UsageAttribute,
				Ident:         ident,
				Relationships: rels,
			}
			usage.NodeSpan = p.spanFrom(paramStart)
			signalType = usage
		} else {
			// Simple signal reference
			signalType = p.parseQualifiedNameRelaxed()
		}
	}
	_ = isTemporalTransition // might be useful for AST differentiation
	_ = isChangeTransition
	
	// Optional guard condition: if <expr>
	var guardExpr ast.Node
	if p.acceptKeyword("if") {
		guardExpr = p.ParseExpression()
	}
	
	// Optional effect action: do <action>
	var effectAction ast.Node
	if p.acceptKeyword("do") {
		// Parse effect action - could be:
		// 1. send statement: do send <message> to <target>
		// 2. action invocation: do actionName(args)
		// 3. assignment: do x = expr
		// Use parseActionMember to handle all behavioral statements
		effectAction = p.parseActionMember()
	}
	
	// Optional via clause: via <port>
	var viaPort ast.Node
	if p.acceptKeyword("via") {
		viaPort = p.parseQualifiedNameRelaxed()
	}
	
	// Expect 'then' keyword
	if !p.acceptKeyword("then") {
		p.error(p.peek().Span, "expected 'then' after signal type")
		en := &ast.ErrorNode{Message: "expected 'then' keyword"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	
	// Parse target state reference (use relaxed parsing to allow keywords like 'on' as names)
	targetState := p.parseQualifiedNameRelaxed()
	
	// Expect semicolon
	p.expect(lexer.Semicolon, "expected ';' after accept transition")
	
	// Create transition usage with accept trigger
	// For now, represent as transition usage (specialized connector)
	transition := &ast.Usage{
		Kind: ast.UsageTransition,
		// Store signal type as first connector end, target state as second
		ConnectorEnds: []*ast.ConnectorEnd{
			{Reference: signalType},  // trigger
			{Reference: targetState}, // target
		},
	}
	
	// Store guard, effect, and via in Members (could extend AST for proper transition fields)
	if guardExpr != nil {
		// Wrap guard in membership for now
		transition.Members = append(transition.Members, guardExpr)
	}
	if effectAction != nil {
		transition.Members = append(transition.Members, effectAction)
	}
	if viaPort != nil {
		// Store via port as member (could use relationship or dedicated field)
		transition.Members = append(transition.Members, viaPort)
	}
	
	transition.NodeSpan = p.spanFrom(start)
	return transition
}

// parseEntryMember parses: entry { <actions> } OR entry <actionRef> OR entry action <def>
func (p *Parser) parseEntryMember(start int) ast.Node {
	// 'entry' already consumed
	
	// Check for action reference or inline definition
	// Patterns:
	// 1. entry { ... } - action block
	// 2. entry actionName { ... } - action reference with invocation
	// 3. entry action name { ... } - inline action definition
	
	if p.at(lexer.LBrace) {
		// Pattern 1: entry { ... }
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
	
	// Check for 'action' keyword (inline definition)
	if p.atKeyword("action") {
		// Pattern 3: entry action name { ... }
		// Parse as action usage/definition
		action := p.parseBodyMember()
		node := &ast.EntryMember{
			Actions: []ast.Node{action},
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}
	
	// Check for bare semicolon (empty entry action)
	if p.at(lexer.Semicolon) {
		p.advance() // consume ';'
		node := &ast.EntryMember{
			Actions: nil, // empty
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}
	
	// Check for behavioral statement (assign, send, bind, etc.)
	// Pattern: entry assign x := expr;
	if p.isBehavioralKeyword() {
		stmt := p.parseActionMember()
		node := &ast.EntryMember{
			Actions: []ast.Node{stmt},
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}
	
	// Check for 'do' keyword followed by braced block: entry do { ... }
	if p.atKeyword("do") {
		p.advance() // consume 'do'
		if p.at(lexer.LBrace) {
			p.advance() // consume '{'
			
			// Parse action sequence
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
		// Fall through to parse action reference after 'do'
	}
	
	// Pattern 2: entry actionName { ... } - action reference
	// Parse action reference (qualified name) and optional invocation arguments
	actionRef := p.parseQualifiedName()
	
	// Check for invocation arguments body
	if p.at(lexer.LBrace) {
		// Parse invocation body (feature bindings): { in vehicle = operatingVehicle; }
		// For now, skip the body (semantic layer will handle invocation)
		p.advance() // consume '{'
		
		for !p.at(lexer.RBrace) && !p.atEOF() {
			// Parse and discard feature bindings
			p.parseBodyMember()
		}
		p.expect(lexer.RBrace, "expected '}' after action invocation")
	}
	
	// Create entry member with action reference
	node := &ast.EntryMember{
		Actions: []ast.Node{actionRef}, // Store reference directly for now
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseDoMember parses: do { <actions> } OR do action <def>
func (p *Parser) parseDoMember(start int) ast.Node {
	// 'do' already consumed
	
	// Check for 'action' keyword (inline definition)
	if p.atKeyword("action") {
		// do action name { ... } - inline action definition
		action := p.parseBodyMember()
		node := &ast.DoMember{
			Actions: []ast.Node{action},
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}
	
	// Check for bare semicolon (empty do action)
	if p.at(lexer.Semicolon) {
		p.advance() // consume ';'
		node := &ast.DoMember{
			Actions: nil, // empty
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}
	
	// Check for action reference: do actionName;
	if p.atName() {
		actionRef := p.parseQualifiedName()
		p.expect(lexer.Semicolon, "expected ';' after action reference")
		node := &ast.DoMember{
			Actions: []ast.Node{actionRef},
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}
	
	// Otherwise expect block: do { ... }
	if !p.at(lexer.LBrace) {
		p.error(p.peek().Span, "expected '{' or 'action' after 'do'")
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

// parseExitMember parses: exit { <actions> } OR exit action <def>
func (p *Parser) parseExitMember(start int) ast.Node {
	// 'exit' already consumed
	
	// Check for 'action' keyword (inline definition)
	if p.atKeyword("action") {
		// exit action name { ... } - inline action definition
		action := p.parseBodyMember()
		node := &ast.ExitMember{
			Actions: []ast.Node{action},
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}
	
	// Check for bare semicolon (empty exit action)
	if p.at(lexer.Semicolon) {
		p.advance() // consume ';'
		node := &ast.ExitMember{
			Actions: nil, // empty
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}
	
	// Check for 'do' keyword followed by braced block: exit do { ... }
	if p.atKeyword("do") {
		p.advance() // consume 'do'
		if p.at(lexer.LBrace) {
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
		// Fall through if no brace after 'do'
	}
	
	// Otherwise expect block: exit { ... }
	if !p.at(lexer.LBrace) {
		p.error(p.peek().Span, "expected '{' or 'action' after 'exit'")
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
	
	// Expect identifier or keyword (for state names like 'off', 'on', 'normal')
	if !p.atNameOrKeyword() {
		p.error(p.peek().Span, "expected identifier or keyword after 'state'")
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

// parseInitialState parses: initial <name>;
func (p *Parser) parseInitialState(start int) ast.Node {
	// 'initial' already consumed
	
	// Expect identifier or keyword for state name
	if !p.atNameOrKeyword() {
		p.error(p.peek().Span, "expected identifier after 'initial'")
		en := &ast.ErrorNode{Message: "expected identifier after 'initial'"}
		if !p.atEOF() && !p.at(lexer.RBrace) {
			p.advance()
		}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	
	nameToken := p.peek()
	name := p.src.Text(nameToken.Span)
	p.advance()
	
	p.expect(lexer.Semicolon, "expected ';' after initial state name")
	
	node := &ast.StateNode{
		Name:      name,
		IsInitial: true,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseFinalState parses: final <name>;
func (p *Parser) parseFinalState(start int) ast.Node {
	// 'final' already consumed
	
	// Expect identifier or keyword for state name
	if !p.atNameOrKeyword() {
		p.error(p.peek().Span, "expected identifier after 'final'")
		en := &ast.ErrorNode{Message: "expected identifier after 'final'"}
		if !p.atEOF() && !p.at(lexer.RBrace) {
			p.advance()
		}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	
	nameToken := p.peek()
	name := p.src.Text(nameToken.Span)
	p.advance()
	
	p.expect(lexer.Semicolon, "expected ';' after final state name")
	
	node := &ast.StateNode{
		Name:    name,
		IsFinal: true,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseRegionMember parses: region <name> { <states> }
func (p *Parser) parseRegionMember(start int) ast.Node {
	// 'region' already consumed
	
	// Expect region name
	if !p.at(lexer.Identifier) && !p.at(lexer.Keyword) {
		p.error(p.peek().Span, "expected region name after 'region'")
		en := &ast.ErrorNode{Message: "expected region name"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	
	nameToken := p.peek()
	name := p.src.Text(nameToken.Span)
	p.advance()
	
	// Expect opening brace
	if !p.at(lexer.LBrace) {
		p.error(p.peek().Span, "expected '{' after region name")
		en := &ast.ErrorNode{Message: "expected '{'"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	p.advance() // consume '{'
	
	// Parse state members within region
	var states []ast.Node
	for !p.at(lexer.RBrace) && !p.at(lexer.EOF) {
		member := p.parseStateMember()
		if member != nil {
			states = append(states, member)
		}
	}
	
	if !p.at(lexer.RBrace) {
		p.error(p.peek().Span, "expected '}' to close region body")
		en := &ast.ErrorNode{Message: "expected '}'"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	p.advance() // consume '}'
	
	region := &ast.StateRegion{
		Name:   name,
		States: states,
	}
	region.NodeSpan = p.spanFrom(start)
	return region
}

// parseChoicePseudostate parses: choice <name>;
func (p *Parser) parseChoicePseudostate(start int) ast.Node {
	// 'choice' already consumed
	
	// Expect pseudostate name
	if !p.at(lexer.Identifier) && !p.at(lexer.Keyword) {
		p.error(p.peek().Span, "expected name after 'choice'")
		en := &ast.ErrorNode{Message: "expected choice name"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	
	nameToken := p.peek()
	name := p.src.Text(nameToken.Span)
	p.advance()
	
	// Expect semicolon
	p.expect(lexer.Semicolon, "expected ';' after choice name")
	
	ps := &ast.PseudostateNode{
		Kind: ast.PseudostateChoice,
		Name: name,
	}
	ps.NodeSpan = p.spanFrom(start)
	return ps
}

// parseJunctionPseudostate parses: junction <name>;
func (p *Parser) parseJunctionPseudostate(start int) ast.Node {
	// 'junction' already consumed
	
	// Expect pseudostate name
	if !p.at(lexer.Identifier) && !p.at(lexer.Keyword) {
		p.error(p.peek().Span, "expected name after 'junction'")
		en := &ast.ErrorNode{Message: "expected junction name"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	
	nameToken := p.peek()
	name := p.src.Text(nameToken.Span)
	p.advance()
	
	// Expect semicolon
	p.expect(lexer.Semicolon, "expected ';' after junction name")
	
	ps := &ast.PseudostateNode{
		Kind: ast.PseudostateJunction,
		Name: name,
	}
	ps.NodeSpan = p.spanFrom(start)
	return ps
}

// parseTransitionMember parses: transition <source> to <target> [when <trigger>] [if <guard>] [do { <effect> }];
func (p *Parser) parseTransitionMember(start int) ast.Node {
	// 'transition' already consumed
	
	// Parse source state (allow keywords like 'done', 'active' as state names)
	source := p.parseQualifiedNameRelaxed()
	
	// Expect 'to'
	if !p.atKeyword("to") {
		p.error(p.peek().Span, "expected 'to' after transition source")
		en := &ast.ErrorNode{Message: "expected 'to' after transition source"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}
	p.advance() // consume 'to'
	
	// Parse target state (allow keywords like 'done', 'active' as state names)
	target := p.parseQualifiedNameRelaxed()
	
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

// parseSendStatement parses: send <message> to <target>;
// parseSendStatement parses: send <message> to <target>; OR send <message> via <port>;
func (p *Parser) parseSendStatement(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	
	// Parse message expression
	message := p.ParseExpression()
	
	// Expect 'to' or 'via' keyword
	if !p.acceptKeyword("to") && !p.acceptKeyword("via") {
		p.error(p.peek().Span, "expected 'to' or 'via' after send message")
	}
	
	// Parse target expression
	target := p.ParseExpression()
	
	// Semicolon is optional if followed by transition keyword (then/if/do)
	if !p.atKeyword("then") && !p.atKeyword("if") && !p.atKeyword("do") {
		p.expect(lexer.Semicolon, "expected ';' after send statement")
	}
	
	node := &ast.SendStatement{
		Message: message,
		Target:  target,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseTerminateStatement parses: terminate <target>;
func (p *Parser) parseTerminateStatement(tok lexer.Token) ast.Node {
	start := tok.Span.Offset
	
	// Parse target expression
	target := p.ParseExpression()
	
	// Semicolon is optional if followed by transition keyword (then/if/do)
	if !p.atKeyword("then") && !p.atKeyword("if") && !p.atKeyword("do") {
		p.expect(lexer.Semicolon, "expected ';' after terminate statement")
	}
	
	node := &ast.TerminateStatement{
		Target: target,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}
