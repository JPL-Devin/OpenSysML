package parser

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
)

// parseCalcBody parses the body of a calc def/usage.
// Handles BOTH generic members (parameters like 'in x: Integer;') AND result members ('return expr;').
// Expects '{' already consumed.
func (p *Parser) parseCalcBody() []ast.Node {
	body := p.newBodyBuilder()
	p.calcBodyDepth++
	defer func() { p.calcBodyDepth-- }()

	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset

		// A calculation body carries the members of an action body
		// (SysML.xtext CalculationBodyItem), a member-attached `then` among them.
		if body.atSuccession() {
			body.takeSuccession()
			continue
		}
		// `then a b;` is the edge member a member-attached `then` desugars to,
		// and so the form a converted model is written back as.
		if p.atKeyword("then") {
			body.add(p.parseSuccessionEdge(p.advance()))
			continue
		}

		// A constraint body that declares parameters is read here, so its
		// asserted conditions are members of this body too.
		if p.atConstraintCondition() {
			body.add(p.parseConstraintMember())
			continue
		}

		// Check for 'return' keyword → ResultMember
		if p.isResultKeyword() {
			body.add(p.parseResultMember())
		} else if p.atCalcStatement() {
			// A behavioural item of a calculation body — a conditional, a loop, an
			// assignment — is read the way an action body reads it
			// (SysML.xtext CalculationBodyItem carries ActionBodyItem).
			body.add(p.parseActionMember())
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
				body.add(p.ParseExpression())
			} else {
				// Parse as generic body member (parameters, etc.)
				body.add(p.parseBodyMember())
			}
		}

		// Guard against infinite loop
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.advance()
		}
	}

	p.expect(lexer.RBrace, "expected '}' after calc body")
	return body.finish()
}

// parseActionBody parses the body of an action usage.
// Expects '{' already consumed, returns list of action nodes + edges.
// parseActionBodyMixed handles action bodies with BOTH declarations and behavioral statements
// Syntax: { in item x; action nested {...}; first nested then ...; flow ...; }
func (p *Parser) parseActionBodyMixed() []ast.Node {
	body := p.newBodyBuilder()

	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset

		// A member-attached `then` sequences the members either side of it, so
		// the keyword is taken here and the member it prefixes read next time
		// round, by whichever branch below that member needs (see
		// succession.go).
		if body.atSuccession() {
			body.takeSuccession()
			continue
		}

		// Try parsing as direction parameter (in/out/inout item/accept/via)
		if p.isDirectionKeyword() {
			body.add(p.parseDirectionParameter())
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
			// An accept node, however it is identified: `action accept …`
			// naming no node of its own, `action nm accept …`, and the short
			// name and name both.
			if p.atAcceptNode() {
				body.add(p.parseBodyMember())
				continue
			}
			if tok1.Kind == lexer.Identifier || tok1.Kind == lexer.Keyword {
				tok2 := p.peekN(2)
				// If colon after name → definitely declaration (typing)
				if tok2.Kind == lexer.Colon {
					body.add(p.parseBodyMember())
					continue
				}
				// If 'accept' keyword after name → declaration (accept action)
				// Pattern: action <name> accept <param> : Type [via <port>];
				if tok2.Kind == lexer.Keyword && tok2.KeywordID == "accept" {
					body.add(p.parseBodyMember())
					continue
				}
				// If behavioral keyword after name → declaration with inline statement
				// Pattern: action <name> send <msg> to <target>;
				//          action <name> perform <ref>;
				if tok2.Kind == lexer.Keyword && (tok2.KeywordID == "send" || tok2.KeywordID == "terminate" ||
					tok2.KeywordID == "perform" || tok2.KeywordID == "bind" || tok2.KeywordID == "assign") {
					body.add(p.parseBodyMember())
					continue
				}
				// If brace after name, peek inside
				if tok2.Kind == lexer.LBrace && startsActionBodyItem(p.peekN(3)) {
					body.add(p.parseBodyMember())
					continue
				}
			}
			// `action { <statements> }` is the anonymous ActionBodyParameter a loop
			// or branch body is written as (SysML.xtext ActionBodyParameter), not the
			// one expression an `action { <expr> }` node computes.
			if tok1.Kind == lexer.LBrace && startsActionBodyItem(p.peekN(2)) {
				body.add(p.parseBodyMember())
				continue
			}
			// Otherwise: treat as behavioral action node
			body.add(p.parseActionMember())
			continue
		}

		if p.isBehavioralKeyword() {
			body.add(p.parseActionMember())
			continue
		}

		// Try parsing as body member (nested declarations)
		body.add(p.parseBodyMember())

		// Ensure progress
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.error(p.peek().Span, "expected a body member")
			p.advance()
		}
	}

	p.expect(lexer.RBrace, "expected '}' after action body")
	return body.finish()
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

	// Check for optional individual/portion/event modifiers
	isIndividual := false
	portion := ast.PortionNone
	isEvent := false
	if p.atKeyword("individual") {
		p.advance()
		isIndividual = true
	} else if p.atKeyword("snapshot") {
		p.advance()
		portion = ast.PortionSnapshot
	} else if p.atKeyword("timeslice") {
		p.advance()
		portion = ast.PortionTimeslice
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
		rels := p.parseRelationships(true)
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
		IsIndividual:  isIndividual,
		Portion:       portion,
	}
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

	// A `return` in a statement position of a calculation body is an early
	// return: only the body's own members declare its result parameter.
	if p.calcBodyDepth > 0 && p.isResultKeyword() {
		return p.parseResultMemberIn(true)
	}

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
		case "decision", "decide":
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
		case "else":
			return p.parseDefaultTargetSuccession(tok)
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
			p.expectStatementEnd(start, "expected ';' after assignment")

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

// atChainedThen reports whether the parser is at a `then` chaining the preceding
// member to a keyword-introduced declaration or statement (`then action b {...}`,
// `then send s to t;`) rather than one starting a named succession edge
// (`then a b;`) over members of the enclosing body.
func (p *Parser) atChainedThen() bool {
	return p.atKeyword("then") && p.peekN(1).Kind == lexer.Keyword
}

// atNamespaceSuccession reports whether the parser is at a `then` chaining to a
// following declaration (`action a {...} then action b {...}`) rather than one
// chaining a behavioral statement or starting a succession edge member.
func (p *Parser) atNamespaceSuccession() bool {
	if !p.atChainedThen() {
		return false
	}
	next := p.peekN(1)
	switch next.KeywordID {
	case "public", "private", "protected":
		return true
	}
	if _, isDef := definitionKindKeywords[next.KeywordID]; isDef {
		return true
	}
	_, isUsage := usageKindKeywords[next.KeywordID]
	return isUsage
}

// startsActionBodyItem reports whether tok, the first token inside a braced
// action body, begins an action body item (SysML.xtext ActionBodyItem) rather
// than an expression. Only keywords that cannot begin an expression qualify.
func startsActionBodyItem(tok lexer.Token) bool {
	if tok.Kind != lexer.Keyword {
		return false
	}
	switch tok.KeywordID {
	case "in", "out", "inout",
		"action", "part", "item", "flow", "doc", "state", "port", "attribute",
		"perform", "send", "assign", "accept", "terminate",
		"first", "then", "done", "fork", "join", "merge", "decision", "decide",
		"while", "loop", "for":
		return true
	}
	return false
}

// startsInlineSuccessionStatement reports whether tok, the token after a `then`,
// starts an inline statement succession (`then assign x := 1;`) rather than a
// named edge (`then source target;`) over members of the enclosing body.
func startsInlineSuccessionStatement(tok lexer.Token) bool {
	if tok.Kind != lexer.Keyword {
		return false
	}
	switch tok.KeywordID {
	// `loop` and `for` head the same action node forms `while` does
	// (SysML.xtext WhileLoopNode, ForLoopNode), so a `then` before one chains a
	// statement rather than naming an edge end.
	case "assign", "perform", "while", "loop", "for", "if", "action":
		return true
	}
	return false
}

// parseSuccessionEdge parses:
// 1. then source target [if guard] ; (control flow edge between named nodes)
// 2. then statement (inline statement succession)
func (p *Parser) parseSuccessionEdge(tok lexer.Token) ast.Node {
	start := tok.Span.Offset

	// Check if this is inline statement succession (then followed by behavioral keyword)
	// Pattern: then assign x := 1; OR then perform foo;
	if startsInlineSuccessionStatement(p.peek()) {
		return p.parseActionMember()
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

// expectStatementEnd terminates the behavioral statement that starts at start.
// Only a statement written as a transition's `do` effect is ended by the
// transition itself, either by its next clause or by its own ';'; elsewhere a
// missing ';' stays a syntax error.
func (p *Parser) expectStatementEnd(start int, msg string) {
	if p.atTransitionEffectStatement(start) && p.atEffectEnd() {
		return
	}
	if p.effectDepth > 0 && (p.atKeyword("then") || p.atKeyword("if") || p.atKeyword("do")) {
		return
	}
	p.expect(lexer.Semicolon, msg)
}

// atTransitionEffectStatement reports whether the member starting at start is the
// innermost transition's `do` effect rather than a statement nested in its body.
func (p *Parser) atTransitionEffectStatement(start int) bool {
	return p.effectDepth > 0 && p.effectStmtStart == start
}

// atEffectEnd reports whether the cursor is where a transition's effect ends: at
// the transition's own ';', or at the clause written after the effect.
func (p *Parser) atEffectEnd() bool {
	return p.at(lexer.Semicolon) || p.atKeyword("then") || p.atKeyword("if") || p.atKeyword("do")
}

// enterTransitionEffect records that the statement at the cursor is a
// transition's `do` effect and returns the function that leaves it.
func (p *Parser) enterTransitionEffect() func() {
	savedDepth, savedStart := p.effectDepth, p.effectStmtStart
	p.effectDepth++
	p.effectStmtStart = p.peek().Span.Offset
	return func() { p.effectDepth, p.effectStmtStart = savedDepth, savedStart }
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

	p.expectStatementEnd(start, "expected ';' after assignment")

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

	p.expectStatementEnd(start, "expected ';' after perform statement")

	node := &ast.PerformActionNode{
		ActionRef: actionRef,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseActionBodyParameter parses the `action [<name>] { <items> }` form of an
// ActionBodyParameter (SysML.xtext), with the `action` keyword at the cursor.
// The parameter stays an action usage member so a name it declares keeps
// scoping the body: `loop action charging { … } until charging.done`.
func (p *Parser) parseActionBodyParameter() []ast.Node {
	m := p.parseBodyMember()
	if m == nil {
		return nil
	}
	// The usage is marked as the body itself, so lowering makes its members the
	// block's statements whether or not it was given a name.
	member := m
	if membership, ok := member.(*ast.Membership); ok {
		member = membership.Member
	}
	if usage, ok := member.(*ast.Usage); ok && usage.Kind == ast.UsageAction {
		usage.IsBodyParameter = true
	}
	return []ast.Node{m}
}

// parseWhileLoopAction parses `while <condition> <action-body> ['until' <c>;']`
// (SysML.xtext WhileLoopNode).
func (p *Parser) parseWhileLoopAction(tok lexer.Token) ast.Node {
	start := tok.Span.Offset

	// Parse condition expression
	condition := p.ParseExpression()

	var body []ast.Node
	if p.atKeyword("action") {
		body = p.parseActionBodyParameter()
	} else {
		if !p.at(lexer.LBrace) {
			p.error(p.peek().Span, "expected '{' after while condition")
			return &ast.ErrorNode{
				NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)},
				Message:  "expected '{' after while condition",
			}
		}
		p.advance() // consume '{'

		for !p.at(lexer.RBrace) && !p.atEOF() {
			body = append(body, p.parseActionMember())
		}

		p.expect(lexer.RBrace, "expected '}' after while body")
	}

	// A `while` loop may also carry an `until` clause, tested after each
	// iteration (SysML.xtext WhileLoopNode).
	var until ast.Node
	if p.acceptKeyword("until") {
		until = p.ParseExpression()
		p.expect(lexer.Semicolon, "expected ';' after 'until' condition")
	}

	node := &ast.WhileLoopActionNode{
		Kind:      ast.LoopWhile,
		Condition: condition,
		Until:     until,
		Body:      body,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

func (p *Parser) parseLoopAction(tok lexer.Token) ast.Node {
	start := tok.Span.Offset

	// Loop syntax: `loop { <body> } until <condition>;`, or the unbraced
	// `loop <body-members> until <condition>;`. A braced body ends at its own
	// '}', so the members following the loop stay with the enclosing body.
	// Body can contain action declarations, succession, etc.
	// Parse body as mixed content (declarations + behavioral statements)
	_, braced := p.accept(lexer.LBrace)
	var body []ast.Node

	// The unbraced body is an ActionBodyParameter: `loop action [<name>] { … }`.
	if !braced && p.atKeyword("action") {
		body = p.parseActionBodyParameter()
	}

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

	if braced {
		p.expect(lexer.RBrace, "expected '}' after loop body")
	}

	// Parse optional 'until' condition
	var condition ast.Node
	if p.acceptKeyword("until") {
		condition = p.ParseExpression()
	}

	// An `until` clause is terminated by a semicolon in either form; a braced
	// body without one is already complete.
	if condition != nil || !braced {
		p.expect(lexer.Semicolon, "expected ';' after loop")
	} else {
		p.accept(lexer.Semicolon)
	}

	// The condition an `until` clause carries is tested after each iteration,
	// which is what LoopUntil records.
	node := &ast.WhileLoopActionNode{
		Kind:      ast.LoopUntil,
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

	// Parse body as an action body parameter (SysML.xtext ForLoopNode).
	if p.atKeyword("action") {
		node := &ast.WhileLoopActionNode{
			Kind: ast.LoopFor,
			Body: p.parseActionBodyParameter(),
			Variable: ast.Identification{
				Name:     p.src.Text(varTok.Span),
				NameSpan: varTok.Span,
			},
			Collection: collection,
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}
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

	// A `for` loop has no condition: iteration is driven by the collection, and
	// the variable it binds each element to is a member of the loop's body scope.
	node := &ast.WhileLoopActionNode{
		Kind: ast.LoopFor,
		Body: body,
		Variable: ast.Identification{
			Name:     p.src.Text(varTok.Span),
			NameSpan: varTok.Span,
		},
		Collection: collection,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseDefaultTargetSuccession parses `else <target>;` (SysML.xtext
// DefaultTargetSuccession): the branch of the preceding decision taken when no
// guarded branch is, which an unguarded edge is how the executor routes.
func (p *Parser) parseDefaultTargetSuccession(tok lexer.Token) ast.Node {
	start := tok.Span.Offset

	target := p.parseQualifiedName()
	p.expect(lexer.Semicolon, "expected ';' after else branch")

	node := &ast.ControlFlowEdge{
		Source: &ast.QualifiedName{}, // empty source = the member before it
		Target: target,
		IsElse: true,
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

	thenStart := p.peek().Span.Offset
	var thenBranch *ast.IfBranchNode
	switch {
	case p.atKeyword("action"):
		thenBranch = &ast.IfBranchNode{Kind: ast.IfBranchThen, Body: p.parseActionBodyParameter()}
		thenBranch.NodeSpan = p.spanFrom(thenStart)
	case p.at(lexer.LBrace):
		p.advance() // consume '{'
		thenBranch = p.parseIfBranch(ast.IfBranchThen, thenStart, "expected '}' after if body")
	default:
		p.error(p.peek().Span, "expected '{' after if condition")
		return &ast.ErrorNode{
			NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)},
			Message:  "expected '{' after if condition",
		}
	}

	// Check for optional 'else' clause
	var elseBranch *ast.IfBranchNode
	elseStart := p.peek().Span.Offset
	if p.acceptKeyword("else") {
		switch {
		case p.atKeyword("action"):
			elseBranch = &ast.IfBranchNode{Kind: ast.IfBranchElse, Body: p.parseActionBodyParameter()}
			elseBranch.NodeSpan = p.spanFrom(elseStart)
		case p.atKeyword("if"):
			// `else if …` is an if node in the else parameter (SysML.xtext
			// IfNodeParameterMember), so the nested node is the branch's body.
			elseBranch = &ast.IfBranchNode{Kind: ast.IfBranchElse, Body: []ast.Node{p.parseIfAction(p.advance())}}
			elseBranch.NodeSpan = p.spanFrom(elseStart)
		case p.at(lexer.LBrace):
			p.advance() // consume '{'
			elseBranch = p.parseIfBranch(ast.IfBranchElse, elseStart, "expected '}' after else body")
		default:
			p.error(p.peek().Span, "expected '{' after else")
		}
	}

	node := &ast.IfActionNode{
		Condition: condition,
		Then:      thenBranch,
		Else:      elseBranch,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseIfBranch parses the members of one branch body of an if action, with the
// opening '{' already consumed, and consumes the closing '}'. start is the
// offset the branch's span begins at ('{' for the then branch, 'else' for the
// else branch). Both declarations and behavioral statements are accepted, so
// `if <cond> { action x : Type { body }; first x then y; }` parses.
func (p *Parser) parseIfBranch(kind ast.IfBranchKind, start int, closeMsg string) *ast.IfBranchNode {
	var body []ast.Node
	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset

		// Direction parameters first.
		if p.isDirectionKeyword() {
			body = append(body, p.parseDirectionParameter())
			continue
		}

		// Declarations (action/part/etc).
		if p.atDefUsageStart() {
			if m := p.parseBodyMember(); m != nil {
				body = append(body, m)
			}
			// No progress: advance to avoid an infinite loop.
			if p.peek().Span.Offset == before {
				p.advance()
			}
			continue
		}

		body = append(body, p.parseActionMember())
	}
	p.expect(lexer.RBrace, closeMsg)

	branch := &ast.IfBranchNode{Kind: kind, Body: body}
	branch.NodeSpan = p.spanFrom(start)
	return branch
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
//
//	return <expr>;         -- computed result
//	return : Type[mult];   -- result parameter (anonymous, type-only)
func (p *Parser) parseResultMember() ast.Node {
	return p.parseResultMemberIn(false)
}

// atCalcStatement reports whether the calculation body continues with a
// behavioural statement rather than a member declaration or a result
// expression. `if` also starts a conditional expression, which is told apart by
// the '?' that follows its condition.
func (p *Parser) atCalcStatement() bool {
	if p.at(lexer.Identifier) {
		next := p.peekN(1).Kind
		return next == lexer.Eq || next == lexer.ColonEq
	}
	if !p.at(lexer.Keyword) {
		return false
	}
	switch p.peek().KeywordID {
	case "assign", "while", "loop", "for", "send", "perform", "terminate":
		return true
	case "if":
		return !p.atConditionalExpression()
	}
	return false
}

// atConditionalExpression reports whether the `if` at the cursor starts a
// conditional expression (`if c ? a else b`) rather than an if statement, by
// looking for the '?' that ends its condition. A brace group is scanned past
// only while the condition goes on after it, so a body expression
// (`xs->exists{...}`) is read as part of the condition while the block of an if
// statement ends the scan.
func (p *Parser) atConditionalExpression() bool {
	depth := 0
	for i := 1; ; i++ {
		tok := p.peekN(i)
		switch tok.Kind {
		case lexer.EOF:
			return false
		case lexer.LParen, lexer.LBracket, lexer.LBrace:
			depth++
		case lexer.RParen, lexer.RBracket, lexer.RBrace:
			if depth == 0 {
				return false
			}
			depth--
			if depth == 0 && tok.Kind == lexer.RBrace && !continuesCondition(p.peekN(i+1)) {
				return false
			}
		case lexer.Question:
			if depth == 0 {
				return true
			}
		case lexer.Semicolon:
			if depth == 0 {
				return false
			}
		}
	}
}

// continuesCondition reports whether tok can go on the condition of a
// conditional expression once a brace group of it has closed.
func continuesCondition(tok lexer.Token) bool {
	switch tok.Kind {
	case lexer.Question, lexer.Arrow, lexer.Dot, lexer.LBracket,
		lexer.Comma, lexer.RParen, lexer.RBracket:
		return true
	}
	_, isOperator := binaryOpForToken(tok)
	return isOperator
}

// parseResultMemberIn parses a `return` member. In a statement position
// (inStatement) a lone name after `return` is the value returned, since a
// result parameter is declared among the body's own members, not inside a
// branch or a loop.
func (p *Parser) parseResultMemberIn(inStatement bool) ast.Node {
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
			IsResult:    true,
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
		additionalRels := p.parseRelationships(false)
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
		postModRels := p.parseRelationships(true)
		u.Relationships = append(u.Relationships, postModRels...)

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
	// `return` introduces a return parameter, so a lone name after it declares
	// that parameter (`calc acc : Acceleration { return a; }`) rather than
	// referencing one.
	if !inStatement && p.atName() && (p.peekN(1).Kind == lexer.Semicolon || p.peekN(1).Kind == lexer.LBracket) {
		u := &ast.Usage{
			Kind:        usageKind,
			Direction:   ast.DirOut,
			IsResult:    true,
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
			IsResult:    true,
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
		} else if p.atDefUsageStart() || p.atRelationshipKeyword() {
			// A declaration, or a member that states a relationship where its
			// name would go (`redefines partMasses = expr;`, `:>> x = value;`):
			// both spellings reach parseBodyMember, which reads them as one form
			// and diagnoses the relationships that are not member forms.
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

// atConstraintCondition reports whether an `assert`/`assume` condition follows,
// as against a named constraint usage (`assert constraint { … }`).
func (p *Parser) atConstraintCondition() bool {
	if !p.atKeyword("assert") && !p.atKeyword("assume") {
		return false
	}
	n := 1
	if t := p.peekN(n); t.Kind == lexer.Keyword && t.KeywordID == "not" {
		n++
	}
	return p.peekN(n).Kind != lexer.Keyword
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

	// A nested constraint states its conditions in a body rather than inline:
	// assert constraint [<name>] { <expr> }
	if node := p.tryParseNestedConstraint(start, isAssert, isNegated); node != nil {
		return node
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
	body := p.newBodyBuilder()

	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset
		// A requirement body carries the members of a definition body
		// (SysML.xtext RequirementBodyItem), a member-attached `then` among them.
		if body.atSuccession() {
			body.takeSuccession()
			continue
		}
		// `then a b;` is the edge member a member-attached `then` desugars to,
		// and so the form a converted model is written back as.
		if p.atKeyword("then") {
			body.add(p.parseSuccessionEdge(p.advance()))
			continue
		}
		body.add(p.parseRequirementMember())
		// Force progress: a member that consumed nothing would spin the loop.
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.advance()
		}
	}

	p.expect(lexer.RBrace, "expected '}' after requirement body")
	return body.finish()
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

	// Try general declaration (nested requirements, features, etc.)
	if node := p.tryParseDeclaration(); node != nil {
		// Validate that tryParseDeclaration didn't just accept garbage
		// Example: "require ;" gets parsed as anonymous constraint usage by tryParseDeclaration
		if usage, ok := node.(*ast.Usage); !ok || usageIsSubstantive(usage) {
			return node // Valid declaration, use it
		}
		// Nothing meaningful: likely a keyword we should handle specially, so
		// fall through to the fallback below.
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
	if usage, ok := node.(*ast.Usage); ok && !usageIsSubstantive(usage) {
		p.error(node.Span(), "expected 'subject', 'assume', 'require', 'actor', or a valid body member")
		return &ast.ErrorNode{Message: "unexpected requirement member"}
	}

	return node
}

// usageIsSubstantive reports whether a usage declares anything: a name, a
// relationship of any kind (`ref concern :>> self : ConcernCheck` takes its name
// from its redefinition), a value or a body. A usage with none of those came
// from a keyword the requirement body parser handles itself.
func usageIsSubstantive(u *ast.Usage) bool {
	return u.Ident.Name != "" || len(u.Relationships) > 0 || u.Value != nil || len(u.Members) > 0
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

	// A bare `subject;` declares the subject parameter without naming or typing
	// it, as the OMG viewpoint examples write it.
	if p.at(lexer.Semicolon) {
		p.advance()
		node := &ast.SubjectMember{}
		node.NodeSpan = p.spanFrom(start)
		return node
	}

	// A subject parameter is a full usage (SysML v2 8.2.2.16): an optional name,
	// an optional specialization part, a multiplicity, a value and a body.
	var name string
	if seg, ok := p.parseNameSegment(); ok {
		name = seg.Text

		// Named binding: subject <name> = <expr>;
		if p.at(lexer.Eq) {
			p.advance()
			value := p.ParseExpression()
			p.expect(lexer.Semicolon, "expected ';' after subject binding")

			node := &ast.SubjectMember{
				Name:        name,
				BindingExpr: value,
			}
			node.NodeSpan = p.spanFrom(start)
			return node
		}
	}

	var typeRef *ast.QualifiedName
	if p.at(lexer.Colon) {
		p.advance()
		typeRef = p.parseQualifiedName()
	}

	var mult *ast.Multiplicity
	if p.at(lexer.LBracket) {
		mult = p.parseMultiplicity()
	}

	// A subject may redefine the one it inherits: subject subj : View[1] :>> RequirementCheck::subj;
	rels := p.parseRelationships(true)

	if name == "" && typeRef == nil && len(rels) == 0 {
		p.error(p.peek().Span, "expected a name, ':' or a specialization after 'subject'")
		en := &ast.ErrorNode{Message: "expected a name, ':' or a specialization after 'subject'"}
		if !p.atEOF() && !p.at(lexer.RBrace) {
			p.advance()
		}
		en.NodeSpan = p.spanFrom(start)
		return en
	}

	// Value part: `= expr` or `default expr`.
	var value ast.Node
	if p.at(lexer.Eq) {
		p.advance()
		value = p.ParseExpression()
	} else if p.acceptKeyword("default") {
		if p.at(lexer.Eq) {
			p.advance()
		}
		value = p.ParseExpression()
	}

	node := &ast.SubjectMember{
		Name:          name,
		TypeRef:       typeRef,
		Multiplicity:  mult,
		Relationships: rels,
		BindingExpr:   value,
	}

	if p.at(lexer.LBrace) {
		p.advance()
		node.Body = p.parseRequirementBody()
	} else {
		p.expect(lexer.Semicolon, "expected ';' or '{' after subject declaration")
	}
	node.NodeSpan = p.spanFrom(start)
	return node
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

		node := &ast.AssumeMember{
			Body: p.parseNestedConstraintConditions(),
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}

	// Reference-subsetting form: assume <QualifiedName> { body }
	if ref, members, ok := p.tryParseConstraintReference(); ok {
		node := &ast.AssumeMember{
			Reference: ref,
			Body:      members,
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

		node := &ast.RequireMember{
			Body: p.parseNestedConstraintConditions(),
		}
		node.NodeSpan = p.spanFrom(start)
		return node
	}

	// Reference-subsetting form: require <QualifiedName> { body }
	if ref, members, ok := p.tryParseConstraintReference(); ok {
		node := &ast.RequireMember{
			Reference: ref,
			Body:      members,
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

// tryParseConstraintReference parses the reference-subsetting form of a
// requirement constraint member — a (possibly qualified) requirement name
// followed by a body (SysML.xtext RequirementConstraintUsage). It returns
// ok=false, consuming nothing, when the member is not that form.
func (p *Parser) tryParseConstraintReference() (*ast.QualifiedName, []ast.Node, bool) {
	if !p.atName() {
		return nil, nil, false
	}
	cp := p.checkpoint()
	ref := p.parseQualifiedName()
	if ref == nil || len(ref.Parts) == 0 || !p.at(lexer.LBrace) {
		p.restore(cp)
		return nil, nil, false
	}
	p.advance() // consume '{'
	return ref, p.parseRequirementBody(), true
}

// tryParseNestedConstraint parses `constraint [<name>] { <conditions> }`, the
// nested-constraint form of a constraint member, and returns nil when the member
// is not that form — `constraint` is a valid feature name, so an expression may
// legitimately start with it.
func (p *Parser) tryParseNestedConstraint(start int, isAssert, isNegated bool) ast.Node {
	if !p.atKeyword("constraint") {
		return nil
	}
	cp := p.checkpoint()
	p.advance() // consume 'constraint'
	var name string
	if p.at(lexer.Identifier) {
		name = p.src.Text(p.peek().Span)
		p.advance()
	}
	if !p.at(lexer.LBrace) {
		p.restore(cp)
		return nil
	}
	p.advance() // consume '{'
	node := &ast.ConstraintMember{
		IsAssert:  isAssert,
		IsNegated: isNegated,
		Name:      name,
		Body:      p.parseNestedConstraintConditions(),
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseNestedConstraintConditions parses the body of the anonymous constraint an
// `assume`/`require constraint { … }` member owns, through its closing brace,
// and returns its ConstraintMembers. Every condition is kept: a constraint body
// may state more than one.
func (p *Parser) parseNestedConstraintConditions() []ast.Node {
	var conditions []ast.Node
	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset
		if p.atKeyword("doc") {
			p.parseDocumentation(before)
			continue
		}
		member := p.parseConstraintMember()
		if c, ok := member.(*ast.ConstraintMember); ok && (c.Expression != nil || len(c.Body) > 0) {
			conditions = append(conditions, c)
		}
		// Force progress: a member that consumed nothing would spin the loop.
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.advance()
		}
	}
	p.expect(lexer.RBrace, "expected '}' after constraint body")
	return conditions
}

// Phase C4: State Body Parsing

// parseStateBody parses the body of a state usage.
// Expects '{' already consumed, returns list of state members.
func (p *Parser) parseStateBody() []ast.Node {
	body := p.newBodyBuilder()

	for !p.at(lexer.RBrace) && !p.atEOF() {
		// A member-attached `then` sequences the members either side of it; a
		// `then` naming states (`then idle done;`) is a member of its own,
		// which parseStateMember reads (see succession.go).
		if body.atSuccession() {
			body.takeSuccession()
			continue
		}
		body.add(p.parseStateMember())
	}

	p.expect(lexer.RBrace, "expected '}' after state body")
	return body.finish()
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
			// `entry point <name>;` declares an entry point pseudostate; `entry ...`
			// anything else is the state's entry action.
			if p.atPointPseudostate() {
				p.advance() // consume 'entry'
				p.advance() // consume 'point'
				return p.parsePseudostate(start, "entry point", ast.PseudostateEntry)
			}
			p.advance()
			return p.parseEntryMember(start)
		case "do":
			p.advance()
			return p.parseDoMember(start)
		case "exit":
			if p.atPointPseudostate() {
				p.advance() // consume 'exit'
				p.advance() // consume 'point'
				return p.parsePseudostate(start, "exit point", ast.PseudostateExit)
			}
			p.advance()
			return p.parseExitMember(start)
		case "defer":
			p.advance()
			return p.parseDeferMember(start)
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
			return p.parsePseudostate(start, kw, ast.PseudostateChoice)
		case "junction":
			p.advance()
			return p.parsePseudostate(start, kw, ast.PseudostateJunction)
		case "fork":
			p.advance()
			return p.parsePseudostate(start, kw, ast.PseudostateFork)
		case "join":
			p.advance()
			return p.parsePseudostate(start, kw, ast.PseudostateJoin)
		case "history":
			// Bare `history <name>;` is shallow, as in UML's H vs H*.
			p.advance()
			return p.parsePseudostate(start, kw, ast.PseudostateShallowHistory)
		case "shallow", "deep":
			kind := ast.PseudostateShallowHistory
			if kw == "deep" {
				kind = ast.PseudostateDeepHistory
			}
			p.advance() // consume 'shallow' / 'deep'
			if !p.acceptKeyword("history") {
				p.error(p.peek().Span, fmt.Sprintf("expected 'history' after '%s'", kw))
				en := &ast.ErrorNode{Message: fmt.Sprintf("expected 'history' after '%s'", kw)}
				en.NodeSpan = p.spanFrom(start)
				return en
			}
			return p.parsePseudostate(start, kw+" history", kind)
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
			// Standalone succession (`then <source> <target>;`, `then <target>;`):
			// the same edge node a member-attached `then` desugars to, so a state
			// body carries one representation of both spellings.
			return p.parseSuccessionEdge(p.advance())
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

// atAcceptNode reports whether the parser is at an accept node declaration
// (SysML.xtext `AcceptNode`): the `accept` keyword, optionally preceded by the
// `action` keyword and the node's own name.
func (p *Parser) atAcceptNode() bool {
	if p.atKeyword("accept") {
		return true
	}
	if !p.atKeyword("action") {
		return false
	}
	if p.peekN(1).Kind == lexer.Keyword && p.peekN(1).KeywordID == "accept" {
		return true
	}
	switch p.peekN(1).Kind {
	case lexer.Identifier, lexer.UnrestrictedName, lexer.Lt:
	default:
		return false
	}
	// `action <name> accept …`, and `action <shortName> name accept …`, whose
	// identification spends four tokens before the keyword.
	for i := 1; i <= 5; i++ {
		tok := p.peekN(i)
		if tok.Kind == lexer.Keyword {
			return tok.KeywordID == "accept"
		}
		switch tok.Kind {
		case lexer.Identifier, lexer.UnrestrictedName, lexer.Lt, lexer.Gt:
			continue
		default:
			return false
		}
	}
	return false
}

// parseAcceptNode parses an accept node: an action that waits for the occurrence
// its payload parameter describes, optionally at a port (`via`), and may carry a
// body like any other action node.
//
//	('action' <name>?)? accept <payload> ('via' <port>)? (';' | '{' … '}')
func (p *Parser) parseAcceptNode(start int, vis ast.Visibility, trivia []ast.Trivia) ast.Node {
	var ident ast.Identification
	// `action accept …` names no node of its own, so the keyword must not be read
	// as the declaration's name.
	if p.acceptKeyword("action") && !p.atKeyword("accept") {
		ident = p.parseIdentification()
	}
	p.advance() // consume 'accept'

	action := &ast.Usage{
		Kind:    ast.UsageAction,
		Keyword: "action",
		Ident:   ident,
	}

	param := p.parsePayloadParameter()
	member := &ast.Membership{Member: param}
	member.NodeSpan = param.Span()
	action.Members = append(action.Members, member)

	// `via <port>`: the port the occurrence must arrive at. It relates the accept
	// to a port rather than specializing it, so it is its own relationship kind.
	if p.acceptKeyword("via") {
		portStart := p.peek().Span.Offset
		rel := &ast.Relationship{Kind: ast.RelVia, Target: p.parseQualifiedName()}
		rel.NodeSpan = p.spanFrom(portStart)
		action.Relationships = append(action.Relationships, rel)
	}

	if p.at(lexer.LBrace) {
		p.advance()
		action.Members = append(action.Members, p.parseActionBodyMixed()...)
		action.HasBody = true
	} else if !p.atKeyword("then") {
		p.expect(lexer.Semicolon, "expected ';' after accept action")
	}

	action.NodeSpan = p.spanFrom(start)
	action.SetLeadingTrivia(trivia)

	m := &ast.Membership{Visibility: vis, Member: action}
	m.NodeSpan = action.Span()
	m.SetLeadingTrivia(trivia)
	return m
}

// atPayloadSpecialization reports whether the parser is at a payload parameter
// declared with a specialization part: `msg : Data`, `:> shutDown`,
// `p :>> Base::event` (SysML.xtext `PayloadFeatureSpecializationPart`). A bare
// name is not one — it types the payload and is read as a signal reference.
func (p *Parser) atPayloadSpecialization() bool {
	i := 0
	if p.atNameOrKeyword() {
		i = 1
	}
	switch p.peekN(i).Kind {
	case lexer.Colon, lexer.ColonGt, lexer.ColonGtGt, lexer.ColonColonGt:
		return true
	}
	return false
}

// parsePayloadParameter parses the payload parameter of an accept — the feature
// the accepted occurrence binds to (SysML.xtext `PayloadParameter`). Its
// specialization part says what is accepted: a typing (`: Data`) accepts
// occurrences of a type, a subsetting (`:> shutDown`) occurrences of that event
// feature. A trigger value (`at`/`after`/`when`) is kept as the parameter's
// value, which is what `TriggerValuePart` declares it to be.
func (p *Parser) parsePayloadParameter() *ast.Usage {
	start := p.peek().Span.Offset
	param := &ast.Usage{
		Kind:        ast.UsageAttribute,
		IsReference: true,
		// The payload is what the accept yields to whatever follows it, so the
		// parameter is an output (SysML v2 §8.3.17: AcceptActionUsage's payload
		// parameter).
		Direction: ast.DirOut,
		IsAccept:  true,
	}

	switch {
	case p.namesPayloadType():
		// `accept Data`, `accept ISQ::Time`: the one name types the payload
		// rather than naming it (SysML.xtext `Payload` third alternative, an
		// OwnedFeatureTyping).
		typeStart := p.peek().Span.Offset
		rel := &ast.Relationship{Kind: ast.RelTyping, Target: p.parseQualifiedName()}
		rel.NodeSpan = p.spanFrom(typeStart)
		param.Relationships = append(param.Relationships, rel)
	default:
		if p.atName() || p.at(lexer.Lt) {
			param.Ident = p.parseIdentification()
		}
		if p.atPayloadOperator() {
			rels := p.parseRelationships(true)
			param.Relationships = append(param.Relationships, rels...)
		}
		if p.atTriggerKeyword() {
			param.Value = p.parseTriggerExpression()
		}
	}

	if len(param.Relationships) == 0 && param.Value == nil && param.Ident.Name == "" {
		p.error(p.peek().Span, "expected the payload of the accept: a type (`accept Warning`), a named parameter (`accept w : Warning`), an event (`accept :> shutDown`) or a trigger (`accept when x > 1`)")
	}

	param.NodeSpan = p.spanFrom(start)
	return param
}

// namesPayloadType reports whether the payload is written as a bare name, which
// types it rather than naming it.
func (p *Parser) namesPayloadType() bool {
	if !p.atName() {
		return false
	}
	for i := 1; ; i += 2 {
		if p.peekN(i).Kind != lexer.ColonColon {
			// A name followed by anything but `::` ends the payload only when no
			// specialization, trigger or name follows it.
			switch tok := p.peekN(i); tok.Kind {
			case lexer.Colon, lexer.ColonGt, lexer.ColonGtGt, lexer.ColonColonGt,
				lexer.Identifier, lexer.UnrestrictedName:
				return false
			case lexer.Keyword:
				return !isTriggerKeyword(tok.KeywordID)
			}
			return true
		}
		if k := p.peekN(i + 1).Kind; k != lexer.Identifier && k != lexer.UnrestrictedName {
			return false
		}
	}
}

// atTriggerKeyword reports whether the parser is at the keyword of a trigger
// expression.
func (p *Parser) atTriggerKeyword() bool {
	tok := p.peek()
	return tok.Kind == lexer.Keyword && isTriggerKeyword(tok.KeywordID)
}

// isTriggerKeyword reports whether a keyword introduces a trigger expression
// (SysML.xtext `TimeTriggerKind` and `ChangeTriggerKind`).
func isTriggerKeyword(kw string) bool {
	switch kw {
	case "at", "after", "when":
		return true
	}
	return false
}

// atPayloadOperator reports whether the parser is at a specialization operator,
// which a payload parameter may begin with when it declares no name.
func (p *Parser) atPayloadOperator() bool {
	switch p.peek().Kind {
	case lexer.Colon, lexer.ColonGt, lexer.ColonGtGt, lexer.ColonColonGt:
		return true
	}
	return false
}

// parseTriggerExpression parses the trigger a payload parameter takes its value
// from (SysML.xtext `TriggerExpression`): `at <instant>` and `after <duration>`
// give a time event, `when <condition>` a change event.
func (p *Parser) parseTriggerExpression() ast.Node {
	kwStart := p.peek().Span.Offset
	if p.atKeyword("when") {
		p.advance()
		evt := &ast.ChangeEvent{Condition: p.ParseExpression()}
		evt.NodeSpan = p.spanFrom(kwStart)
		return evt
	}
	absolute := p.atKeyword("at")
	p.advance() // consume 'at' / 'after'
	evt := &ast.TimeEvent{Duration: p.ParseExpression(), Absolute: absolute}
	evt.NodeSpan = p.spanFrom(kwStart)
	return evt
}

// parseTriggerEvent parses the event of a transition trigger, the part after
// `accept`: a time event (`at <instant>` / `after <duration>`), a change event
// (`when <condition>`), a call event (`<operation>(<params>)`), a payload
// parameter (`<name> : <Type>`, `:> <event>`) or a bare signal name. The event
// kind is decided here so lowering never has to re-derive it.
func (p *Parser) parseTriggerEvent() ast.Node {
	if p.atKeyword("at") || p.atKeyword("after") || p.atKeyword("when") {
		return p.parseTriggerExpression()
	}

	// A payload parameter declared with a specialization: `accept msg : Data`,
	// `accept :> shutDown`, `accept p :> shutDown` (SysML.xtext
	// `PayloadParameter` → `Payload` → `PayloadFeatureSpecializationPart`).
	if p.atPayloadSpecialization() {
		return p.parsePayloadParameter()
	}

	// Bare name: a signal reference, or a call event when an argument list follows.
	nameStart := p.peek().Span.Offset
	name := p.parseQualifiedNameRelaxed()
	if name == nil {
		// An unnamed trigger is an error node, not a nil trigger: a nil one would
		// read as a completion transition and fire on its own.
		en := &ast.ErrorNode{Message: "expected an event after 'accept'"}
		en.NodeSpan = p.spanFrom(nameStart)
		return en
	}
	if p.at(lexer.LParen) {
		return p.parseCallEvent(nameStart, name)
	}
	return name
}

// parseCallEvent parses the argument list of a call trigger, `(<name>, ...)`,
// after its operation name. The names are matched against the arguments of the
// invocation, so `accept setSpeed(value)` fires only for a call carrying a
// `value` argument, while `accept setSpeed()` fires for any call to it.
func (p *Parser) parseCallEvent(start int, operation *ast.QualifiedName) ast.Node {
	p.advance() // consume '('

	var params []ast.NameSegment
	for !p.at(lexer.RParen) && !p.atEOF() {
		seg, ok := p.parseNameSegmentRelaxed()
		if !ok {
			p.error(p.peek().Span, "expected parameter name in call trigger")
			en := &ast.ErrorNode{Message: "expected parameter name in call trigger"}
			en.NodeSpan = p.spanFrom(start)
			return en
		}
		params = append(params, seg)
		if !p.accept2(lexer.Comma) {
			break
		}
	}

	if !p.accept2(lexer.RParen) {
		p.error(p.peek().Span, "expected ')' after call trigger parameters")
		en := &ast.ErrorNode{Message: "expected ')' after call trigger parameters"}
		en.NodeSpan = p.spanFrom(start)
		return en
	}

	evt := &ast.CallEvent{Operation: operation, Parameters: params}
	evt.NodeSpan = p.spanFrom(start)
	return evt
}

// parseAcceptTransition parses a transition stated by its trigger alone, whose
// source is the state containing it (SysML.xtext `TargetTransitionUsage`):
//
//	accept <trigger> [via <port>] [if <guard>] [do <effect>] then <target>;
func (p *Parser) parseAcceptTransition(start int) ast.Node {
	return p.parseTransitionTail(start, "", nil, nil)
}

// stateSubactionKind names which of a state's subactions is being parsed: the
// StateSubactionKind of SysML.xtext (/* STATES */).
type stateSubactionKind string

const (
	subactionEntry stateSubactionKind = "entry"
	subactionDo    stateSubactionKind = "do"
	subactionExit  stateSubactionKind = "exit"
)

// parseEntryMember parses a state's entry subaction; the keyword is consumed.
func (p *Parser) parseEntryMember(start int) ast.Node {
	return p.parseStateSubaction(start, subactionEntry)
}

// parseDoMember parses a state's do subaction; the keyword is consumed.
func (p *Parser) parseDoMember(start int) ast.Node {
	return p.parseStateSubaction(start, subactionDo)
}

// parseExitMember parses a state's exit subaction; the keyword is consumed.
func (p *Parser) parseExitMember(start int) ast.Node {
	return p.parseStateSubaction(start, subactionExit)
}

// parseStateSubaction parses the action of an entry/do/exit subaction, whose
// keyword is already consumed:
//
//	StateActionUsage : EmptyActionUsage ';' | PerformedActionUsage ActionBody
//
// (SysML.xtext, /* STATES */). A performed action is either an inline action
// usage (`entry action warmUp;`), a behavioral statement (`entry assign x := 1;`)
// or a reference to an action declared elsewhere (`entry warmUp;`), the last
// being the reference-subsetting form of PerformActionUsageDeclaration. The
// braced form (`entry { ... }`) is a Systemica extension over that grammar.
func (p *Parser) parseStateSubaction(start int, kind stateSubactionKind) ast.Node {
	actions, err := p.parseStateSubactionActions(start, kind)
	if err != nil {
		return err
	}
	var node ast.Node
	switch kind {
	case subactionEntry:
		node = &ast.EntryMember{NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)}, Actions: actions}
	case subactionDo:
		node = &ast.DoMember{NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)}, Actions: actions}
	case subactionExit:
		node = &ast.ExitMember{NodeBase: ast.NodeBase{NodeSpan: p.spanFrom(start)}, Actions: actions}
	}
	return node
}

// parseStateSubactionActions parses the action itself, returning an ErrorNode
// instead when the subaction is followed by something that starts no action.
func (p *Parser) parseStateSubactionActions(start int, kind stateSubactionKind) ([]ast.Node, ast.Node) {
	// EmptyActionUsage ';'
	if p.at(lexer.Semicolon) {
		p.advance()
		return nil, nil
	}

	// `<kind> { ... }`, and the `entry do { ... }` spelling of it.
	if p.at(lexer.LBrace) {
		return p.parseStateSubactionBlock(kind), nil
	}
	if kind != subactionDo && p.atKeyword("do") && p.peekN(1).Kind == lexer.LBrace {
		p.advance() // consume 'do'
		return p.parseStateSubactionBlock(kind), nil
	}

	// An inline action usage or definition: `<kind> action warmUp : WarmUp;`.
	if p.atKeyword("action") {
		return []ast.Node{p.parseBodyMember()}, nil
	}

	// A behavioral statement: `<kind> assign x := 1;`, `<kind> send s to t;`.
	if p.isBehavioralKeyword() {
		return []ast.Node{p.parseActionMember()}, nil
	}

	// PerformActionUsageDeclaration by reference: `<kind> warmUp;`.
	if p.atName() {
		return []ast.Node{p.parsePerformedActionReference(p.peek().Span.Offset, featureMods{}, string(kind))}, nil
	}

	msg := fmt.Sprintf("expected an action, an action reference or '{' after '%s'", kind)
	p.error(p.peek().Span, msg)
	en := &ast.ErrorNode{Message: msg}
	en.NodeSpan = p.spanFrom(start)
	return nil, en
}

// parseStateSubactionBlock parses the braced action sequence of a subaction;
// the '{' is at the cursor.
func (p *Parser) parseStateSubactionBlock(kind stateSubactionKind) []ast.Node {
	p.advance() // consume '{'
	var actions []ast.Node
	for !p.at(lexer.RBrace) && !p.atEOF() {
		actions = append(actions, p.parseActionMember())
	}
	p.expect(lexer.RBrace, fmt.Sprintf("expected '}' after %s actions", kind))
	return actions
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

	// A region carries the members of a state body, a member-attached `then`
	// among them (SysML.xtext StateBodyItem).
	body := p.newBodyBuilder()
	for !p.at(lexer.RBrace) && !p.at(lexer.EOF) {
		if body.atSuccession() {
			body.takeSuccession()
			continue
		}
		body.add(p.parseStateMember())
	}
	states := body.finish()

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

// atPointPseudostate reports whether the `entry`/`exit` keyword at the cursor
// starts an entry/exit point pseudostate — `entry point <name>;` — rather than
// an entry/exit action. `point` is matched contextually rather than reserved as
// a keyword, because models routinely name features `point`.
func (p *Parser) atPointPseudostate() bool {
	point := p.peekN(1)
	if point.Kind != lexer.Identifier || p.src.Text(point.Span) != "point" {
		return false
	}
	name := p.peekN(2)
	if name.Kind != lexer.Identifier && name.Kind != lexer.Keyword {
		return false
	}
	return p.peekN(3).Kind == lexer.Semicolon
}

// parseDeferMember parses `defer <event> [, <event>]* ;` in a state body: the
// events the state retains while it is active instead of dropping them. Each
// event is parsed exactly like a transition trigger, so a signal name and a
// call event (`defer setSpeed(value)`) both work.
func (p *Parser) parseDeferMember(start int) ast.Node {
	// 'defer' already consumed
	if p.at(lexer.Semicolon) || p.atEOF() || p.at(lexer.RBrace) {
		p.error(p.peek().Span, "expected an event after 'defer'")
		en := &ast.ErrorNode{Message: "expected an event after 'defer'"}
		if p.at(lexer.Semicolon) {
			p.advance()
		}
		en.NodeSpan = p.spanFrom(start)
		return en
	}

	var triggers []ast.Node
	for {
		triggers = append(triggers, p.parseTriggerEvent())
		if !p.accept2(lexer.Comma) {
			break
		}
	}

	p.expect(lexer.Semicolon, "expected ';' after deferred events")

	node := &ast.DeferMember{Triggers: triggers}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parsePseudostate parses a pseudostate declaration in a state body:
// choice/junction/fork/join <name>;. The keyword is already consumed.
func (p *Parser) parsePseudostate(start int, keyword string, kind ast.PseudostateKind) ast.Node {
	if !p.at(lexer.Identifier) && !p.at(lexer.Keyword) {
		p.error(p.peek().Span, fmt.Sprintf("expected name after '%s'", keyword))
		en := &ast.ErrorNode{Message: fmt.Sprintf("expected %s name", keyword)}
		en.NodeSpan = p.spanFrom(start)
		return en
	}

	name := p.src.Text(p.peek().Span)
	p.advance()
	p.expect(lexer.Semicolon, fmt.Sprintf("expected ';' after %s name", keyword))

	ps := &ast.PseudostateNode{
		Kind: kind,
		Name: name,
	}
	ps.NodeSpan = p.spanFrom(start)
	return ps
}

// parseTransitionMember parses a transition of a state machine, in either
// spelling, with the `transition` keyword already consumed:
//
//	transition [<name>] first <source> [accept <trigger> [via <port>]]
//	    [if <guard>] [do <effect>] then <target>;
//	transition [<name>] <source> to <target> [accept …] [if …] [do …];
//
// The first is SysML.xtext `TransitionUsage`, which states the source with
// `first` and the target with `then`; the second is the `to` spelling Systemica
// also accepts. Both describe the same transition and give the same node.
func (p *Parser) parseTransitionMember(start int) ast.Node {
	var name string
	var source, target *ast.QualifiedName

	// A name of the transition's own, which only the `first` spelling can have:
	// in the `to` spelling the first name is the source state.
	if p.atName() && p.peekN(1).Kind == lexer.Keyword && p.peekN(1).KeywordID == "first" {
		if seg, ok := p.parseNameSegment(); ok {
			name = seg.Text
		}
	}

	switch {
	case p.atKeyword("first") && !p.peekIsKeyword(1, "to"):
		// `first` marks the source, unless it is itself the source: `first` is a
		// legal state name, so `transition first to second;` names a state.
		p.advance() // consume 'first'
		source = p.parseQualifiedNameRelaxed()
	case p.atName() || p.at(lexer.Keyword):
		source = p.parseQualifiedNameRelaxed()
		if !p.acceptKeyword("to") {
			p.error(p.peek().Span, "expected 'to' after transition source, or 'first' before it: a transition is written `transition first <source> … then <target>;` or `transition <source> to <target>;`")
			en := &ast.ErrorNode{Message: "expected 'to' after transition source"}
			en.NodeSpan = p.spanFrom(start)
			return en
		}
		target = p.parseQualifiedNameRelaxed()
	}

	return p.parseTransitionTail(start, name, source, target)
}

// parseTransitionTail parses the clauses a transition carries after its source —
// trigger, guard, effect and, when the target was not already stated with `to`,
// the `then` naming it — and the terminating ';'. The clauses are read in the
// order they were written so a misordered transition is reported once, at the
// clause that is out of place, rather than silently dropped.
func (p *Parser) parseTransitionTail(start int, name string, source, target *ast.QualifiedName) ast.Node {
	node := &ast.TransitionMember{
		Name:   name,
		Source: source,
		Target: target,
	}

	for {
		switch {
		case p.atKeyword("accept"):
			if node.Trigger != nil {
				p.error(p.peek().Span, "a transition accepts one trigger: write a second transition for the other event")
			}
			p.advance() // consume 'accept'
			node.Trigger = p.parseTriggerEvent()
			// `via <port>` belongs to the accept, naming the port the accepted
			// occurrence must arrive at (SysML.xtext `AcceptParameterPart`).
			if p.acceptKeyword("via") {
				node.Via = p.parseQualifiedNameRelaxed()
			}
			continue
		case p.atKeyword("when"):
			// `when <event>`: the trigger spelling Systemica accepts alongside the
			// standard `accept`. What follows is read as an expression and
			// classified when lowered, so a name states a signal and a condition a
			// change, as it did before the standard spelling was added.
			if node.Trigger != nil {
				p.error(p.peek().Span, "a transition accepts one trigger: write a second transition for the other event")
			}
			p.advance() // consume 'when'
			node.Trigger = p.ParseExpression()
			continue
		case p.atKeyword("if"):
			p.advance() // consume 'if'
			node.Guard = p.ParseExpression()
			continue
		case p.atKeyword("do"):
			p.advance() // consume 'do'
			effect, err := p.parseTransitionEffect(start)
			if err != nil {
				return err
			}
			node.Effect = effect
			continue
		case p.atKeyword("then"):
			p.advance() // consume 'then'
			if node.Target != nil {
				p.error(p.peek().Span, "a transition has one target: it is named either after 'to' or after 'then', not both")
			}
			node.Target = p.parseQualifiedNameRelaxed()
			continue
		}
		break
	}

	if node.Target == nil {
		// A transition without a target names no edge, so it is an error node
		// rather than a member the later tiers would read a missing target from.
		msg := "expected the target of the transition after 'then'"
		p.error(p.peek().Span, msg)
		p.accept(lexer.Semicolon)
		en := &ast.ErrorNode{Message: msg}
		en.NodeSpan = p.spanFrom(start)
		return en
	}

	p.expect(lexer.Semicolon, "expected ';' after transition")
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseTransitionEffect parses the effect of a transition, whose `do` is already
// consumed: a single action (`do action alarm send Alert() to op`, `do assign
// x := 1`) as SysML.xtext `TransitionUsage` states it, or a braced sequence,
// which Systemica also accepts.
func (p *Parser) parseTransitionEffect(start int) ([]ast.Node, ast.Node) {
	if p.at(lexer.LBrace) {
		p.advance() // consume '{'
		var effect []ast.Node
		for !p.at(lexer.RBrace) && !p.atEOF() {
			effect = append(effect, p.parseActionMember())
		}
		p.expect(lexer.RBrace, "expected '}' after effect actions")
		return effect, nil
	}
	if p.atKeyword("action") || p.atKeyword("perform") {
		leave := p.enterTransitionEffect()
		defer leave()
		return []ast.Node{p.parseBodyMember()}, nil
	}
	if p.isBehavioralKeyword() {
		leave := p.enterTransitionEffect()
		defer leave()
		return []ast.Node{p.parseActionMember()}, nil
	}
	msg := "expected an action after 'do': an action declaration (`do action alarm send Alert() to operator`), a behavioral statement or '{'"
	p.error(p.peek().Span, msg)
	en := &ast.ErrorNode{Message: msg}
	en.NodeSpan = p.spanFrom(start)
	return nil, en
}

// parseSendStatement parses: send <message> to <target>;
// parseSendStatement parses: send <message> to <target>; OR send <message> via <port>;
func (p *Parser) parseSendStatement(tok lexer.Token) ast.Node {
	start := tok.Span.Offset

	// Parse message expression
	message := p.ParseExpression()

	// Expect 'to' or 'via' keyword. Which one was written decides how the
	// target is interpreted: a receiver, or a port to route through.
	isVia := false
	if p.acceptKeyword("via") {
		isVia = true
	} else if !p.acceptKeyword("to") {
		p.error(p.peek().Span, "expected 'to' or 'via' after send message")
	}

	// Parse target expression
	target := p.ParseExpression()

	p.expectStatementEnd(start, "expected ';' after send statement")

	node := &ast.SendStatement{
		Message: message,
		Target:  target,
		IsVia:   isVia,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}

// parseTerminateStatement parses: terminate [<target>];
func (p *Parser) parseTerminateStatement(tok lexer.Token) ast.Node {
	start := tok.Span.Offset

	// The occurrence to terminate is optional (SysML.xtext TerminateActionUsage:
	// a `terminate` with no parameter terminates the performing occurrence).
	var target ast.Node
	if !p.at(lexer.Semicolon) && !p.at(lexer.RBrace) &&
		!p.atKeyword("then") && !p.atKeyword("if") && !p.atKeyword("do") {
		target = p.ParseExpression()
	}

	p.expectStatementEnd(start, "expected ';' after terminate statement")

	node := &ast.TerminateStatement{
		Target: target,
	}
	node.NodeSpan = p.spanFrom(start)
	return node
}
