package parser

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
)

// definitionKindKeywords maps a single kind keyword to its DefinitionKind.
// The two-word `use case` is handled separately in parseDefUsage.
var definitionKindKeywords = map[string]ast.DefinitionKind{
	"part":      ast.DefPart,
	"attribute": ast.DefAttribute,
	// Tier A.
	"item":       ast.DefItem,
	"occurrence": ast.DefOccurrence,
	"individual": ast.DefIndividual,
	"metadata":   ast.DefMetadata,
	"enum":       ast.DefEnumeration,
	"view":       ast.DefView,
	"viewpoint":  ast.DefViewpoint,
	"rendering":  ast.DefRendering,
	"concern":    ast.DefConcern,
	// Tier B.
	"connection": ast.DefConnection,
	"flow":       ast.DefFlow,
	"port":       ast.DefPort,
	"interface":  ast.DefInterface,
	"allocation": ast.DefAllocation,
	// Tier C.
	"action":       ast.DefAction,
	"state":        ast.DefState,
	"calc":         ast.DefCalc,
	"constraint":   ast.DefConstraint,
	"requirement":  ast.DefRequirement,
	"case":         ast.DefCase,
	"analysis":     ast.DefAnalysisCase,
	"verification": ast.DefVerificationCase,
}

// usageKindKeywords maps a single kind keyword to its UsageKind.
var usageKindKeywords = map[string]ast.UsageKind{
	"part":      ast.UsagePart,
	"attribute": ast.UsageAttribute,
	// Tier A.
	"item":       ast.UsageItem,
	"occurrence": ast.UsageOccurrence,
	"individual": ast.UsageIndividual,
	"metadata":   ast.UsageMetadata,
	"enum":       ast.UsageEnumeration,
	"view":       ast.UsageView,
	"viewpoint":  ast.UsageViewpoint,
	"rendering":  ast.UsageRendering,
	"concern":    ast.UsageConcern,
	// Tier B.
	"connection": ast.UsageConnection,
	"flow":       ast.UsageFlow,
	"port":       ast.UsagePort,
	"interface":  ast.UsageInterface,
	"allocation": ast.UsageAllocation,
	// Tier C.
	"action":       ast.UsageAction,
	"state":        ast.UsageState,
	"calc":         ast.UsageCalc,
	"constraint":   ast.UsageConstraint,
	"requirement":  ast.UsageRequirement,
	"case":         ast.UsageCase,
	"analysis":     ast.UsageAnalysisCase,
	"verification": ast.UsageVerificationCase,
}

var featureModifierKeywords = map[string]bool{
	"abstract":  true,
	"variation": true,
	"ref":       true,
	"in":        true,
	"out":       true,
	"inout":     true,
	"composite": true,
	"portion":   true,
	"derived":   true,
	"ordered":   true,
	"nonunique": true,
}

// relationshipKeywords maps a spelled-out relationship keyword to its kind.
var relationshipKeywords = map[string]ast.RelationshipKind{
	"specializes": ast.RelSpecializes,
	"subsets":     ast.RelSubsets,
	"redefines":   ast.RelRedefines,
	"references":  ast.RelReferences,
	"crosses":     ast.RelCrosses,
}

type featureMods struct {
	isAbstract  bool
	isVariation bool
	isReference bool
	direction   ast.FeatureDirection
	isComposite bool
	isDerived   bool
	isOrdered   bool
	isNonunique bool
}

// atDefUsageStart reports whether the current token begins a def/usage
// declaration: a feature-modifier keyword or a kind keyword.
func (p *Parser) atDefUsageStart() bool {
	t := p.peek()
	if t.Kind != lexer.Keyword {
		return false
	}
	if featureModifierKeywords[t.KeywordID] {
		return true
	}
	if t.KeywordID == "use" {
		return p.atUseCase()
	}
	_, isDef := definitionKindKeywords[t.KeywordID]
	return isDef
}

// atUseCase reports whether the current token is `use` immediately followed by
// `case` (the two-word use-case kind keyword).
func (p *Parser) atUseCase() bool {
	if !p.atKeyword("use") {
		return false
	}
	n := p.peekN(1)
	return n.Kind == lexer.Keyword && n.KeywordID == "case"
}

func (p *Parser) parseFeatureModifiers() featureMods {
	var m featureMods
	for {
		t := p.peek()
		if t.Kind != lexer.Keyword {
			return m
		}
		switch t.KeywordID {
		case "abstract":
			m.isAbstract = true
		case "variation":
			m.isVariation = true
		case "ref":
			m.isReference = true
		case "in":
			m.direction = ast.DirIn
		case "out":
			m.direction = ast.DirOut
		case "inout":
			m.direction = ast.DirInOut
		case "composite", "portion":
			m.isComposite = true
		case "derived":
			m.isDerived = true
		case "ordered":
			m.isOrdered = true
		case "nonunique":
			m.isNonunique = true
		default:
			return m
		}
		p.advance()
	}
}

// parseDefUsage parses a definition or usage declaration. The caller has
// already established (via atDefUsageStart) that a def/usage begins here.
func (p *Parser) parseDefUsage(start int) ast.Node {
	mods := p.parseFeatureModifiers()

	// Two-word `use case` kind keyword.
	if p.atUseCase() {
		p.advance() // 'use'
		p.advance() // 'case'
		if p.atKeyword("def") {
			p.advance() // 'def'
			return p.parseDefinition(start, ast.DefUseCase, mods)
		}
		return p.parseUsage(start, ast.UsageUseCase, mods)
	}

	t := p.peek()
	kw := ""
	if t.Kind == lexer.Keyword {
		kw = t.KeywordID
	}
	defKind, ok := definitionKindKeywords[kw]
	if !ok {
		return nil
	}
	p.advance() // consume the kind keyword

	if p.atKeyword("def") {
		p.advance() // consume 'def'
		return p.parseDefinition(start, defKind, mods)
	}
	return p.parseUsage(start, usageKindKeywords[kw], mods)
}

func (p *Parser) parseDefinition(start int, kind ast.DefinitionKind, mods featureMods) *ast.Definition {
	def := &ast.Definition{
		Kind:        kind,
		IsAbstract:  mods.isAbstract,
		IsVariation: mods.isVariation,
		Ident:       p.parseIdentification(),
	}
	def.Relationships, _ = p.parseRelationships(false)
	members, hasBody := p.parseDefUsageBody()
	def.Members = members
	def.HasBody = hasBody
	def.NodeSpan = p.spanFrom(start)
	return def
}

func (p *Parser) parseUsage(start int, kind ast.UsageKind, mods featureMods) *ast.Usage {
	u := &ast.Usage{
		Kind:        kind,
		IsAbstract:  mods.isAbstract,
		IsReference: mods.isReference,
		Direction:   mods.direction,
		IsComposite: mods.isComposite,
		IsDerived:   mods.isDerived,
		IsOrdered:   mods.isOrdered,
		IsNonunique: mods.isNonunique,
	}
	// A bare flow shorthand `flow x to y` has no declaration name; the first
	// name is the `from` end, parsed later by parseFlowEnds.
	if !(kind == ast.UsageFlow && p.atFlowShorthand()) {
		u.Ident = p.parseIdentification()
	}
	rels, conjugated := p.parseRelationships(true)
	u.Relationships = rels
	u.IsConjugated = conjugated
	u.Multiplicity = p.parseMultiplicity()
	if p.accept2(lexer.Eq) {
		u.Value = p.ParseExpression()
	}
	p.parseTierBEnds(u, kind)
	
	// Dispatch to specialized body parsers based on kind
	var members []ast.Node
	var hasBody bool
	switch kind {
	case ast.UsageAction:
		// Action bodies: { first x; action y; then x y; ... }
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			members = p.parseActionBody() // parseActionBody expects '{' already consumed
			hasBody = true
		}
	case ast.UsageState:
		// TODO: Phase C4 — implement parseStateBody
		members, hasBody = p.parseDefUsageBody()
	default:
		members, hasBody = p.parseDefUsageBody()
	}
	
	u.Members = members
	u.HasBody = hasBody
	u.NodeSpan = p.spanFrom(start)
	return u
}

// parseDefUsageBody parses a definition/usage body: `;` (no body) or
// `{ member* }`. Body members may be nested def/usage declarations or ordinary
// namespace members, each carrying optional visibility.
func (p *Parser) parseDefUsageBody() (members []ast.Node, hasBody bool) {
	if p.accept2(lexer.Semicolon) {
		return nil, false
	}
	if _, ok := p.expect(lexer.LBrace, "expected '{' or ';' after declaration"); !ok {
		return nil, false
	}
	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset
		m := p.parseBodyMember()
		if m != nil {
			members = append(members, m)
		}
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.advance()
		}
	}
	p.expect(lexer.RBrace, "expected '}' to close body")
	return members, true
}

// parseBodyMember parses one body member: an optional visibility prefix
// followed by a declaration (which may be a nested def/usage). Import/Alias
// carry their own visibility and are returned directly; other declarations are
// wrapped in a Membership. Mirrors parseMember.
func (p *Parser) parseBodyMember() ast.Node {
	start := p.peek().Span.Offset
	trivia := p.takeTrivia()
	vis := p.parseVisibility()

	if p.atKeyword("import") {
		imp := p.parseImport(start, vis)
		imp.SetLeadingTrivia(trivia)
		return imp
	}
	if p.atKeyword("alias") {
		al := p.parseAlias(start, vis)
		al.SetLeadingTrivia(trivia)
		return al
	}

	inner := p.parseDeclaration(start)
	if inner == nil {
		en := p.errorNodeSkip(start, "expected a body member")
		en.SetLeadingTrivia(trivia)
		return en
	}
	mem := &ast.Membership{Visibility: vis, Member: inner}
	mem.NodeSpan = p.spanFrom(start)
	mem.SetLeadingTrivia(trivia)
	return mem
}

// parseRelationships parses zero or more relationship clauses. isUsage selects
// the meaning of the symbolic `:>` operator (subsets on a usage, specializes on
// a definition). Each clause may carry a comma-separated target list; every
// target becomes its own Relationship sharing the clause kind.
func (p *Parser) parseRelationships(isUsage bool) (rels []*ast.Relationship, conjugated bool) {
	for {
		kind, ok := p.relationshipClauseKind(isUsage)
		if !ok {
			return rels, conjugated
		}
		for {
			start := p.peek().Span.Offset
			// A leading `~` on a typing target is conjugation (`: ~ Type`).
			if p.accept2(lexer.Tilde) && kind == ast.RelTyping {
				conjugated = true
			}
			qn := p.parseQualifiedName()
			r := &ast.Relationship{Kind: kind, Target: qn}
			r.NodeSpan = p.spanFrom(start)
			rels = append(rels, r)
			if !p.accept2(lexer.Comma) {
				break
			}
		}
	}
}

// parseTierBEnds parses the distinctive Tier B usage grammar following the
// declaration head: connector ends (connection/interface/allocation) and flow
// ends + payload (flow). Other kinds contribute nothing.
func (p *Parser) parseTierBEnds(u *ast.Usage, kind ast.UsageKind) {
	switch kind {
	case ast.UsageConnection, ast.UsageInterface:
		p.parseConnectorEnds(u, "connect")
	case ast.UsageAllocation:
		p.parseConnectorEnds(u, "allocate")
	case ast.UsageFlow:
		p.parseFlowEnds(u)
	}
}

// parseConnectorEnds parses `<kw> end to end` (binary) or
// `<kw> ( end , end , ... )` (n-ary), where <kw> is `connect` or `allocate`.
// The connector clause is optional. On a malformed end, it records a diagnostic,
// keeps the ends parsed so far, and stops (the declaration remains a Usage).
func (p *Parser) parseConnectorEnds(u *ast.Usage, kw string) {
	if !p.acceptKeyword(kw) {
		return
	}
	if p.at(lexer.LParen) {
		p.advance() // '('
		for {
			qn := p.parseQualifiedName()
			if qn == nil {
				return // parseQualifiedName recorded the diagnostic; keep partial ends
			}
			u.ConnectorEnds = append(u.ConnectorEnds, qn)
			if !p.accept2(lexer.Comma) {
				break
			}
		}
		p.expect(lexer.RParen, "expected ')' to close connector ends")
		return
	}
	// Binary form: end to end.
	from := p.parseQualifiedName()
	if from == nil {
		return
	}
	u.ConnectorEnds = append(u.ConnectorEnds, from)
	if !p.acceptKeyword("to") {
		p.error(p.peek().Span, "expected 'to' between connector ends")
		return
	}
	to := p.parseQualifiedName()
	if to == nil {
		return
	}
	u.ConnectorEnds = append(u.ConnectorEnds, to)
}

// atFlowShorthand reports whether the parser sits at a bare flow shorthand
// `x to y` (a name immediately followed by the `to` keyword), which has no
// declaration name.
func (p *Parser) atFlowShorthand() bool {
	if !p.atName() {
		return false
	}
	n := p.peekN(1)
	return n.Kind == lexer.Keyword && n.KeywordID == "to"
}

// parseFlowEnds parses an optional `of <payload>` followed by either
// `from <x> to <y>` or the shorthand `<x> to <y>`. On a malformed end it records
// a diagnostic and keeps whatever ends were parsed so far.
func (p *Parser) parseFlowEnds(u *ast.Usage) {
	start := p.peek().Span.Offset
	var fe *ast.FlowEnds
	hasOf := p.acceptKeyword("of")
	if hasOf {
		fe = &ast.FlowEnds{}
		fe.Payload = p.parseQualifiedName()
	}
	switch {
	case p.acceptKeyword("from"):
		if fe == nil {
			fe = &ast.FlowEnds{}
		}
		fe.From = p.parseQualifiedName()
		p.parseFlowTo(fe)
	case !hasOf && p.atName():
		// Shorthand `x to y`.
		fe = &ast.FlowEnds{}
		fe.From = p.parseQualifiedName()
		p.parseFlowTo(fe)
	}
	if fe != nil {
		fe.NodeSpan = p.spanFrom(start)
		u.FlowEnds = fe
	}
}

// parseFlowTo consumes the `to <end>` tail of a flow, recording a diagnostic if
// `to` is absent.
func (p *Parser) parseFlowTo(fe *ast.FlowEnds) {
	if p.acceptKeyword("to") {
		fe.To = p.parseQualifiedName()
		return
	}
	p.error(p.peek().Span, "expected 'to' between flow ends")
}

// relationshipClauseKind consumes the operator/keyword that begins a
// relationship clause and returns its kind. Reports ok=false (consuming
// nothing) when the current token does not begin a relationship clause.
func (p *Parser) relationshipClauseKind(isUsage bool) (ast.RelationshipKind, bool) {
	if t := p.peek(); t.Kind == lexer.Keyword {
		if k, ok := relationshipKeywords[t.KeywordID]; ok {
			p.advance()
			return k, true
		}
		if t.KeywordID == "defined" {
			p.advance()
			p.expect2Keyword("by")
			return ast.RelTyping, true
		}
	}
	switch p.peek().Kind {
	case lexer.Colon:
		p.advance()
		return ast.RelTyping, true
	case lexer.ColonGt:
		p.advance()
		if isUsage {
			return ast.RelSubsets, true
		}
		return ast.RelSpecializes, true
	case lexer.ColonGtGt:
		p.advance()
		return ast.RelRedefines, true
	case lexer.ColonColonGt:
		p.advance()
		return ast.RelReferences, true
	case lexer.EqGt:
		p.advance()
		return ast.RelCrosses, true
	}
	return 0, false
}

// parseMultiplicity parses `[ lower ( .. upper )? ]` when a `[` is present.
func (p *Parser) parseMultiplicity() *ast.Multiplicity {
	if p.peek().Kind != lexer.LBracket {
		return nil
	}
	start := p.peek().Span.Offset
	p.advance() // '['
	m := &ast.Multiplicity{}
	m.Lower = p.parseMultiplicityBound()
	if p.accept2(lexer.DotDot) {
		m.IsRange = true
		m.Upper = p.parseMultiplicityBound()
	}
	p.expect(lexer.RBracket, "expected ']' to close multiplicity")
	m.NodeSpan = p.spanFrom(start)
	return m
}

// parseMultiplicityBound parses a single bound: `*` (infinity) or an expression.
// The bound is parsed above range precedence so the multiplicity's own `..`
// separator is not swallowed as a range operator.
func (p *Parser) parseMultiplicityBound() ast.Node {
	if p.peek().Kind == lexer.Star {
		star := p.peek()
		p.advance()
		inf := &ast.LiteralInfinity{}
		inf.NodeSpan = star.Span
		return inf
	}
	return p.parseBinary(precAdditive)
}
