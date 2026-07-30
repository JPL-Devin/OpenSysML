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
	members, hasBody := p.parseNamespaceBody() // placeholder; Task 5 replaces
	u.Members = members
	u.HasBody = hasBody
	u.NodeSpan = p.spanFrom(start)
	return u
}
