package parser

import (
	"fmt"
	
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
)

// definitionKindKeywords maps a single kind keyword to its DefinitionKind.
// The two-word `use case` is handled separately in parseDefUsage.
var definitionKindKeywords = map[string]ast.DefinitionKind{
	"part":      ast.DefPart,
	"attribute": ast.DefAttribute,
	"datatype":  ast.DefAttribute,
	"feature":   ast.DefAttribute,
	// Tier A.
	"item":       ast.DefItem,
	"occurrence": ast.DefOccurrence,
	"individual": ast.DefIndividual,
	"metaclass":  ast.DefMetaclass,
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
	"binding":    ast.DefBinding,
	// Tier C.
	"action":       ast.DefAction,
	"state":        ast.DefState,
	"calc":         ast.DefCalc,
	"function":     ast.DefCalc, // synonym for calc
	"constraint":   ast.DefConstraint,
	"requirement":  ast.DefRequirement,
	"case":         ast.DefCase,
	"analysis":     ast.DefAnalysisCase,
	"verification": ast.DefVerificationCase,
	// KerML structural.
	"behavior":  ast.DefBehavior,
	"assoc":     ast.DefAssoc,
	"struct":    ast.DefStruct,
	"class":     ast.DefClass,
	"predicate": ast.DefPredicate,
	"bool":      ast.DefBool,
}

// usageKindKeywords maps a single kind keyword to its UsageKind.
var usageKindKeywords = map[string]ast.UsageKind{
	"part":      ast.UsagePart,
	"attribute": ast.UsageAttribute,
	"datatype":  ast.UsageAttribute,
	"feature":   ast.UsageAttribute,
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
	"connector":  ast.UsageConnector,
	"succession": ast.UsageSuccession,
	"flow":       ast.UsageFlow,
	"port":       ast.UsagePort,
	"interface":  ast.UsageInterface,
	"allocation": ast.UsageAllocation,
	"binding":    ast.UsageBinding,
	// Tier C.
	"action":       ast.UsageAction,
	"state":        ast.UsageState,
	"calc":         ast.UsageCalc,
	"function":     ast.UsageCalc, // synonym for calc
	"constraint":   ast.UsageConstraint,
	"inv":          ast.UsageConstraint, // synonym for constraint (invariant)
	"requirement":  ast.UsageRequirement,
	"satisfy":      ast.UsageSatisfy,
	"subject":      ast.UsageSubject,
	"objective":    ast.UsageObjective,
	"case":         ast.UsageCase,
	"analysis":     ast.UsageAnalysisCase,
	"verification": ast.UsageVerificationCase,
	// KerML structural.
	"behavior":  ast.UsageBehavior,
	"assoc":     ast.UsageAssoc,
	"struct":    ast.UsageStruct,
	"class":     ast.UsageClass,
	"predicate": ast.UsagePredicate,
	"bool":      ast.UsageBool,
}

var featureModifierKeywords = map[string]bool{
	"abstract":  true,
	"variation": true,
	"ref":       true,
	"end":       true,
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
	"intersects":  ast.RelIntersects,
	"disjoint":    ast.RelDisjoint, // followed by 'from' keyword
}

type featureMods struct {
	isAbstract  bool
	isVariation bool
	isReference bool
	isEnd       bool
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
	// Check for relationship tokens that can precede kind keyword (e.g., :>> num)
	if t.Kind == lexer.ColonGt || t.Kind == lexer.ColonGtGt || t.Kind == lexer.Colon || t.Kind == lexer.Tilde {
		return true
	}
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
	_, isUsage := usageKindKeywords[t.KeywordID]
	return isDef || isUsage
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
		case "end":
			m.isEnd = true
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

// parsePostModifiers parses feature modifiers that appear after typing/multiplicity.
// Currently only 'ordered' and 'nonunique' are allowed in this position.
// isPostModifierKeyword checks if token is a post-multiplicity modifier keyword
func isPostModifierKeyword(tok lexer.Token) bool {
	if tok.Kind != lexer.Keyword {
		return false
	}
	return tok.KeywordID == "ordered" || tok.KeywordID == "nonunique"
}

func (p *Parser) parsePostModifiers() featureMods {
	var m featureMods
	for {
		t := p.peek()
		if t.Kind != lexer.Keyword {
			return m
		}
		switch t.KeywordID {
		case "ordered":
			m.isOrdered = true
			p.advance()
		case "nonunique":
			m.isNonunique = true
			p.advance()
		default:
			return m
		}
	}
}

// parseDefUsage parses a definition or usage declaration. The caller has
// already established (via atDefUsageStart) that a def/usage begins here.
func (p *Parser) parseDefUsage(start int) ast.Node {
	// Check for relationship tokens before modifiers (e.g., :>> x)
	// These indicate anonymous usages (attribute is default kind)
	tok := p.peek()
	if tok.Kind == lexer.ColonGt || tok.Kind == lexer.ColonGtGt || tok.Kind == lexer.Colon {
		// No modifiers, no kind keyword - parse as anonymous attribute usage
		return p.parseUsage(start, ast.UsageAttribute, featureMods{}, false)
	}
	
	mods := p.parseFeatureModifiers()

	// Two-word `use case` kind keyword.
	if p.atUseCase() {
		p.advance() // 'use'
		p.advance() // 'case'
		if p.atKeyword("def") {
			p.advance() // 'def'
			return p.parseDefinition(start, ast.DefUseCase, mods, false)
		}
		return p.parseUsage(start, ast.UsageUseCase, mods, false)
	}

	t := p.peek()
	kw := ""
	if t.Kind == lexer.Keyword {
		kw = t.KeywordID
	}
	
	// Check for usage-only keywords (subject, objective, succession, inv, connector, satisfy) that never have def forms
	if kw == "subject" || kw == "objective" || kw == "succession" || kw == "inv" || kw == "connector" || kw == "satisfy" {
		p.advance() // consume the kind keyword
		isAll := p.acceptKeyword("all")
		return p.parseUsage(start, usageKindKeywords[kw], mods, isAll)
	}
	
	defKind, ok := definitionKindKeywords[kw]
	if !ok {
		// Fallback: if we have modifiers but no kind keyword, assume it's a generic usage (e.g., "in x: Integer;")
		// This is common for parameters in calc/action bodies.
		// Also check if name + multiplicity/modifiers follow (e.g., "in seq[1..*] ordered;")
		hasModifiers := mods.direction != ast.DirNone || mods.isReference || mods.isEnd || mods.isComposite || mods.isDerived
		hasNameWithMultOrMods := p.atNameOrKeyword() && (p.peekN(1).Kind == lexer.LBracket || p.peekN(1).Kind == lexer.Colon || isPostModifierKeyword(p.peekN(1)))
		if hasModifiers || hasNameWithMultOrMods {
			return p.parseUsage(start, ast.UsageAttribute, mods, false)
		}
		return nil
	}
	p.advance() // consume the kind keyword
	
	// Parse 'all' modifier if present (appears after keyword, before name)
	isAll := p.acceptKeyword("all")
	
	// Parse secondary keyword if present (e.g., 'assoc struct')
	// Check if next token is also a kind keyword
	t2 := p.peek()
	if t2.Kind == lexer.Keyword {
		if secondKind, ok := definitionKindKeywords[t2.KeywordID]; ok && t2.KeywordID != "def" {
			// Have secondary keyword - use it as primary kind
			defKind = secondKind
			p.advance() // consume secondary keyword
		}
	}

	if p.atKeyword("def") {
		p.advance() // consume 'def'
		return p.parseDefinition(start, defKind, mods, isAll)
	}
	return p.parseUsage(start, usageKindKeywords[kw], mods, isAll)
}

func (p *Parser) parseDefinition(start int, kind ast.DefinitionKind, mods featureMods, isAll bool) *ast.Definition {
	def := &ast.Definition{
		Kind:        kind,
		IsAbstract:  mods.isAbstract,
		IsVariation: mods.isVariation,
		IsAll:       isAll,
		Ident:       p.parseIdentification(),
	}
	def.Relationships, _ = p.parseRelationships(false)
	
	// Dispatch to specialized body parsers based on kind
	var members []ast.Node
	var hasBody bool
	switch kind {
	case ast.DefAction:
		// Action def bodies: behavioral OR generic
		// Lookahead: if body starts with behavioral keyword → parseActionBody
		// Otherwise → generic parseDefUsageBody
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			if p.isBehavioralKeyword() {
				members = p.parseActionBody()
			} else {
				// Generic body (e.g., { part p; })
				members = p.parseActionBodyGeneric()
			}
			hasBody = true
		}
	case ast.DefCalc:
		// Calculation def bodies: mixed (parameters + return statements)
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			members = p.parseCalcBody()
			hasBody = true
		}
	case ast.DefConstraint:
		// Constraint def bodies: constraint body OR generic
		// Lookahead: if body starts with 'assert'/'assume' → parseConstraintBody
		// Otherwise → generic parseDefUsageBody
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			if p.isConstraintKeyword() {
				members = p.parseConstraintBody()
			} else {
				members = p.parseActionBodyGeneric()
			}
			hasBody = true
		}
	case ast.DefRequirement:
		// Requirement def bodies: requirement body OR generic
		// Lookahead: if body starts with requirement keywords → parseRequirementBody
		// Otherwise → generic parseDefUsageBody
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			if p.isRequirementKeyword() {
				members = p.parseRequirementBody()
			} else {
				members = p.parseActionBodyGeneric()
			}
			hasBody = true
		}
	case ast.DefState:
		// State def bodies: state body OR generic
		// Lookahead: if body starts with state keywords → parseStateBody
		// Otherwise → generic parseDefUsageBody
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			if p.isStateKeyword() {
				members = p.parseStateBody()
			} else {
				members = p.parseActionBodyGeneric()
			}
			hasBody = true
		}
	default:
		members, hasBody = p.parseDefUsageBody()
	}
	
	def.Members = members
	def.HasBody = hasBody
	def.NodeSpan = p.spanFrom(start)
	return def
}

// parseActionBodyGeneric parses generic action def body (same as parseDefUsageBody internals)
func (p *Parser) parseActionBodyGeneric() []ast.Node {
	var members []ast.Node
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
	return members
}

// isBehavioralKeyword checks if next token is a behavioral keyword
func (p *Parser) isBehavioralKeyword() bool {
	if !p.at(lexer.Keyword) {
		return false
	}
	kw := p.peek().KeywordID
	switch kw {
	case "first", "done", "fork", "join", "merge", "decision", "action", "then":
		return true
	}
	return false
}

// isResultKeyword checks if next token is 'return'
func (p *Parser) isResultKeyword() bool {
	return p.at(lexer.Keyword) && p.peek().KeywordID == "return"
}

// isConstraintKeyword checks if next token is 'assert' or 'assume'
func (p *Parser) isConstraintKeyword() bool {
	if !p.at(lexer.Keyword) {
		return false
	}
	kw := p.peek().KeywordID
	return kw == "assert" || kw == "assume"
}

// isRequirementKeyword checks if next token is requirement-related
func (p *Parser) isRequirementKeyword() bool {
	if !p.at(lexer.Keyword) {
		return false
	}
	kw := p.peek().KeywordID
	return kw == "subject" || kw == "assume" || kw == "require" || kw == "actor"
}

// isStateKeyword checks if next token is state body keyword
func (p *Parser) isStateKeyword() bool {
	if !p.at(lexer.Keyword) {
		return false
	}
	kw := p.peek().KeywordID
	return kw == "entry" || kw == "do" || kw == "exit" || kw == "state" || kw == "transition"
}

func (p *Parser) parseUsage(start int, kind ast.UsageKind, mods featureMods, isAll bool) *ast.Usage {
	u := &ast.Usage{
		Kind:        kind,
		IsAbstract:  mods.isAbstract,
		IsReference: mods.isReference,
		IsAll:       isAll,
		IsEnd:       mods.isEnd,
		Direction:   mods.direction,
		IsComposite: mods.isComposite,
		IsDerived:   mods.isDerived,
		IsOrdered:   mods.isOrdered,
		IsNonunique: mods.isNonunique,
	}
	
	// Handle UsageSatisfy special syntax: satisfy requirement <name> by <name> { body }
	if kind == ast.UsageSatisfy {
		// Expect: requirement <name> by <name>
		if !p.acceptKeyword("requirement") {
			p.error(p.peek().Span, "expected 'requirement' keyword after 'satisfy'")
			u.NodeSpan = p.spanFrom(start)
			return u
		}
		reqName := p.parseQualifiedName()
		if reqName != nil {
			// Store as typing relationship
			u.Relationships = append(u.Relationships, &ast.Relationship{
				Kind:   ast.RelTyping,
				Target: reqName,
			})
		}
		if !p.acceptKeyword("by") {
			p.error(p.peek().Span, "expected 'by' keyword after requirement reference")
			u.NodeSpan = p.spanFrom(start)
			return u
		}
		subjName := p.parseQualifiedName()
		if subjName != nil {
			// Store subject as identification (or could be relationship)
			// For now, use as identification name
			if len(subjName.Parts) > 0 {
				u.Ident.Name = subjName.Parts[0].Text
				u.Ident.NameSpan = subjName.Parts[0].Span
			}
		}
		// Parse body (requirement body)
		members, hasBody := p.parseDefUsageBody()
		u.Members = members
		u.HasBody = hasBody
		u.NodeSpan = p.spanFrom(start)
		return u
	}
	
	// Handle shorthand: `feature redefines x` means `feature x redefines x`
	// Check if relationship keyword followed by name (not symbolic operator)
	var preRels []*ast.Relationship
	var conjugated bool
	if p.at(lexer.Keyword) {
		if relKind, ok := relationshipKeywords[p.peek().KeywordID]; ok {
			// Peek ahead to see if name follows
			nextTok := p.peekN(1)
			if nextTok.Kind == lexer.Identifier || nextTok.Kind == lexer.UnrestrictedName {
				// Shorthand: relationship keyword + name
				p.advance() // consume relationship keyword
				u.Ident = p.parseIdentification()
				// Create implicit relationship targeting same name
				rel := &ast.Relationship{
					Kind:   relKind,
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: u.Ident.Name, Span: u.Ident.NameSpan}}},
				}
				preRels = append(preRels, rel)
			} else {
				// Normal relationship parsing
				preRels, conjugated = p.parseRelationships(true)
				// A bare flow shorthand `flow x to y` and succession `succession x then y` have no declaration name
				if !(kind == ast.UsageFlow && p.atFlowShorthand()) && kind != ast.UsageSuccession {
					u.Ident = p.parseIdentification()
				}
			}
		} else {
			// Not a relationship keyword
			preRels, conjugated = p.parseRelationships(true)
			// A bare flow shorthand `flow x to y` and succession `succession x then y` have no declaration name
			if !(kind == ast.UsageFlow && p.atFlowShorthand()) && kind != ast.UsageSuccession {
				u.Ident = p.parseIdentification()
			}
		}
	} else {
		// No relationship shorthand
		preRels, conjugated = p.parseRelationships(true)
		// A bare flow shorthand `flow x to y` and succession `succession x then y` have no declaration name
		if !(kind == ast.UsageFlow && p.atFlowShorthand()) && kind != ast.UsageSuccession {
			u.Ident = p.parseIdentification()
		}
	}
	
	// Parse post-identification relationships (e.g., : Type)
	postIdRels, postConj := p.parseRelationships(true)
	u.Relationships = append(preRels, postIdRels...)
	u.IsConjugated = conjugated || postConj
	u.Multiplicity = p.parseMultiplicity()
	
	// Parse post-multiplicity modifiers (ordered/nonunique)
	postMods := p.parsePostModifiers()
	if postMods.isOrdered {
		u.IsOrdered = true
	}
	if postMods.isNonunique {
		u.IsNonunique = true
	}
	
	// Parse additional relationships after modifiers (e.g., :> target)
	postRels, _ := p.parseRelationships(true)
	u.Relationships = append(u.Relationships, postRels...)
	
	if p.accept2(lexer.Eq) || p.acceptKeyword("default") {
		u.Value = p.ParseExpression()
	}
	p.parseTierBEnds(u, kind)
	
	// Dispatch to specialized body parsers based on kind
	var members []ast.Node
	var hasBody bool
	switch kind {
	case ast.UsageAction:
		// Action usage bodies: behavioral OR generic
		// Lookahead: if body starts with behavioral keyword → parseActionBody
		// Otherwise → generic parseActionBodyGeneric
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			if p.isBehavioralKeyword() {
				members = p.parseActionBody()
			} else {
				// Generic body (e.g., { doc /* ... */; })
				members = p.parseActionBodyGeneric()
			}
			hasBody = true
		}
	case ast.UsageCalc:
		// Calculation usage bodies: mixed (parameters + return statements)
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			members = p.parseCalcBody()
			hasBody = true
		}
	case ast.UsageConstraint:
		// Constraint bodies: { assert/assume expr; ... }
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			members = p.parseConstraintBody()
			hasBody = true
		}
	case ast.UsageRequirement:
		// Requirement bodies: { subject/assume/require/actor ... }
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			members = p.parseRequirementBody()
			hasBody = true
		}
	case ast.UsageState:
		// State usage bodies: state body OR generic
		// Lookahead: if body starts with state keywords → parseStateBody
		// Otherwise → generic parseActionBodyGeneric
		if p.accept2(lexer.Semicolon) {
			hasBody = false
		} else if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); ok {
			if p.isStateKeyword() {
				members = p.parseStateBody()
			} else {
				// Generic body (e.g., { doc /* ... */; })
				members = p.parseActionBodyGeneric()
			}
			hasBody = true
		}
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
	
	// Check for enum literal pattern: identifier = expr; OR identifier;
	// Examples: low = 0.25; or pass;
	nextKind := p.peekN(1).Kind
	if p.atName() && (nextKind == lexer.Eq || nextKind == lexer.Semicolon) {
		var id ast.Identification
		tok := p.advance()
		if tok.Kind == lexer.Identifier || tok.Kind == lexer.UnrestrictedName {
			id.Name = p.src.Text(tok.Span)
			id.NameSpan = tok.Span
		}
		
		var value ast.Node
		if p.at(lexer.Eq) {
			p.advance() // consume '='
			value = p.ParseExpression()
		}
		p.expect(lexer.Semicolon, "expected ';' after enum literal")
		
		u := &ast.Usage{
			Kind:  ast.UsageEnumeration,
			Ident: id,
			Value: value,
		}
		u.NodeSpan = p.spanFrom(start)
		
		mem := &ast.Membership{Visibility: vis, Member: u}
		mem.NodeSpan = p.spanFrom(start)
		mem.SetLeadingTrivia(trivia)
		return mem
	}
	
	// Check for name-before-keyword pattern: <name> <keyword> { ... }
	// Example: assert constraint { ... }, require constraint { ... }
	// <name> can be identifier OR keyword used as name
	// But exclude usage-only keywords (they're declarations, not names)
	isUsageOnlyKw := p.at(lexer.Keyword) && (p.peek().KeywordID == "subject" || p.peek().KeywordID == "objective" || 
		p.peek().KeywordID == "succession" || p.peek().KeywordID == "inv" || p.peek().KeywordID == "connector" || 
		p.peek().KeywordID == "satisfy")
	if !isUsageOnlyKw && (p.atName() || p.at(lexer.Keyword)) {
		next := p.peekN(1)
		if next.Kind == lexer.Keyword {
			_, isDef := definitionKindKeywords[next.KeywordID]
			_, isUsage := usageKindKeywords[next.KeywordID]
			if isDef || isUsage {
				// Parse as named usage: consume name token, then proceed with keyword
				var id ast.Identification
				tok := p.advance()
				if tok.Kind == lexer.Identifier || tok.Kind == lexer.UnrestrictedName {
					id.Name = p.src.Text(tok.Span)
					id.NameSpan = tok.Span
				} else if tok.Kind == lexer.Keyword {
					// Keyword used as name (e.g., 'assert' in 'assert constraint')
					id.Name = tok.KeywordID
					id.NameSpan = tok.Span
				}
				inner := p.parseDeclaration(start)
				if u, ok := inner.(*ast.Usage); ok {
					u.Ident = id
				} else if d, ok := inner.(*ast.Definition); ok {
					d.Ident = id
				}
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
		}
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
	case ast.UsageConnector:
		p.parseConnectorFromTo(u)
	case ast.UsageSuccession:
		p.parseConnectorEnds(u, "") // succession has no intermediate keyword
	case ast.UsageAllocation:
		p.parseConnectorEnds(u, "allocate")
	case ast.UsageFlow:
		p.parseFlowEnds(u)
	}
}

// parseConnectorEnds parses `<kw> end to end` (binary) or
// `<kw> ( end , end , ... )` (n-ary), where <kw> is `connect` or `allocate`.
// For succession, kw is empty and the pattern is directly `end then end`.
// Each end can optionally have a multiplicity: `[mult] end`.
// The connector clause is optional. On a malformed end, it records a diagnostic,
// keeps the ends parsed so far, and stops (the declaration remains a Usage).
func (p *Parser) parseConnectorEnds(u *ast.Usage, kw string) {
	// For connection/allocation, expect intermediate keyword ('connect'/'allocate')
	// For succession, no intermediate keyword (kw is empty)
	if kw != "" {
		if !p.acceptKeyword(kw) {
			return
		}
	}
	if p.at(lexer.LParen) {
		p.advance() // '('
		for {
			ce := p.parseConnectorEnd()
			if ce == nil {
				return // parseConnectorEnd recorded the diagnostic; keep partial ends
			}
			u.ConnectorEnds = append(u.ConnectorEnds, ce)
			if !p.accept2(lexer.Comma) {
				break
			}
		}
		p.expect(lexer.RParen, "expected ')' to close connector ends")
		return
	}
	// Binary form: end keyword end (where keyword is "to" for connection, "then" for succession).
	from := p.parseConnectorEnd()
	if from == nil {
		return
	}
	u.ConnectorEnds = append(u.ConnectorEnds, from)
	
	// Determine expected keyword based on usage kind
	var expectedKeyword string
	switch u.Kind {
	case ast.UsageSuccession:
		expectedKeyword = "then"
	default:
		expectedKeyword = "to"
	}
	
	if !p.acceptKeyword(expectedKeyword) {
		p.error(p.peek().Span, fmt.Sprintf("expected '%s' between connector ends", expectedKeyword))
		return
	}
	to := p.parseConnectorEnd()
	if to == nil {
		return
	}
	u.ConnectorEnds = append(u.ConnectorEnds, to)
}

// parseConnectorEnd parses a single connector end: optional multiplicity followed by qualified name.
func (p *Parser) parseConnectorEnd() *ast.ConnectorEnd {
	start := p.peek().Span.Offset
	ce := &ast.ConnectorEnd{}
	
	// Optional multiplicity
	if p.at(lexer.LBracket) {
		ce.Multiplicity = p.parseMultiplicity()
	}
	
	// Target expression (qualified name or feature chain)
	ce.Target = p.ParseExpression()
	if ce.Target == nil {
		return nil
	}
	
	ce.NodeSpan = p.spanFrom(start)
	return ce
}

// parseConnectorFromTo parses the `from x to y` pattern for connector usages.
// Pattern: `from <end> to <end>` (binary form only).
func (p *Parser) parseConnectorFromTo(u *ast.Usage) {
	if !p.acceptKeyword("from") {
		return // Optional connector clause
	}
	
	from := p.parseConnectorEnd()
	if from == nil {
		return
	}
	u.ConnectorEnds = append(u.ConnectorEnds, from)
	
	if !p.acceptKeyword("to") {
		p.error(p.peek().Span, "expected 'to' between connector ends")
		return
	}
	
	to := p.parseConnectorEnd()
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
			// 'disjoint' requires 'from' keyword after it
			if k == ast.RelDisjoint {
				p.expect2Keyword("from")
			}
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
