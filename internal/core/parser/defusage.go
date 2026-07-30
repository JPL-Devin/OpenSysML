package parser

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
)

var definitionKindKeywords = map[string]ast.DefinitionKind{
	"part":      ast.DefPart,
	"attribute": ast.DefAttribute,
}

var usageKindKeywords = map[string]ast.UsageKind{
	"part":      ast.UsagePart,
	"attribute": ast.UsageAttribute,
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
	_, isDef := definitionKindKeywords[t.KeywordID]
	return isDef
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

	t := p.peek()
	kw := ""
	if t.Kind == lexer.Keyword {
		kw = t.KeywordID
	}
	if _, ok := definitionKindKeywords[kw]; !ok {
		return nil
	}
	p.advance() // consume the kind keyword

	if p.atKeyword("def") {
		p.advance() // consume 'def'
		return p.parseDefinition(start, definitionKindKeywords[kw], mods)
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
	def.Relationships = p.parseRelationships(false)
	members, hasBody := p.parseNamespaceBody() // placeholder; Task 5 replaces
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
		Ident:       p.parseIdentification(),
	}
	u.Relationships = p.parseRelationships(true)
	u.Multiplicity = p.parseMultiplicity()
	if p.accept2(lexer.Eq) {
		u.Value = p.ParseExpression()
	}
	members, hasBody := p.parseNamespaceBody() // placeholder; Task 5 replaces
	u.Members = members
	u.HasBody = hasBody
	u.NodeSpan = p.spanFrom(start)
	return u
}

// parseRelationships parses zero or more relationship clauses. isUsage selects
// the meaning of the symbolic `:>` operator (subsets on a usage, specializes on
// a definition). Each clause may carry a comma-separated target list; every
// target becomes its own Relationship sharing the clause kind.
func (p *Parser) parseRelationships(isUsage bool) []*ast.Relationship {
	var rels []*ast.Relationship
	for {
		kind, ok := p.relationshipClauseKind(isUsage)
		if !ok {
			return rels
		}
		for {
			start := p.peek().Span.Offset
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
