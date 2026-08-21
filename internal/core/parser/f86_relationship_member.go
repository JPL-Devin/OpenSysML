package parser

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// relationshipMemberForm describes a KerML relationship written keyword-first,
// as a member of its own rather than as a clause of a declaration
// (KerML.xtext NonFeatureElement): `subtype A specializes B;`.
type relationshipMemberForm struct {
	kind ast.RelationshipKind
	// sepKeyword and sepToken are the two spellings of the separator between
	// the two ends; sepToken is lexer.EOF when only the keyword exists.
	sepKeyword string
	sepToken   lexer.Kind
	// sepKeyword2 is the second word of a two-word separator (`typed by`).
	sepKeyword2 string
	// conjugated marks a Conjugation, recorded as a conjugated generalization.
	conjugated bool
	// prefix is the optional keyword that may precede this one and carry the
	// relationship's own identification.
	prefix string
}

// relationshipMemberForms are the keyword-first relationship members, keyed by
// the keyword introducing the first end (KerML.xtext:390, 408, 486, 634, 665,
// 683, 712).
var relationshipMemberForms = map[string]relationshipMemberForm{
	"subtype":       {kind: ast.RelSpecializes, sepKeyword: "specializes", sepToken: lexer.ColonGt, prefix: "specialization"},
	"subclassifier": {kind: ast.RelSpecializes, sepKeyword: "specializes", sepToken: lexer.ColonGt, prefix: "specialization"},
	"typing":        {kind: ast.RelTyping, sepKeyword: "typed", sepKeyword2: "by", sepToken: lexer.Colon, prefix: "specialization"},
	"subset":        {kind: ast.RelSubsets, sepKeyword: "subsets", sepToken: lexer.ColonGt, prefix: "specialization"},
	"redefinition":  {kind: ast.RelRedefines, sepKeyword: "redefines", sepToken: lexer.ColonGtGt, prefix: "specialization"},
	"conjugate":     {kind: ast.RelSpecializes, sepKeyword: "conjugates", sepToken: lexer.Tilde, conjugated: true, prefix: "conjugation"},
	"inverse":       {kind: ast.RelInverseOf, sepKeyword: "of", sepToken: lexer.EOF, prefix: "inverting"},
}

// atRelationshipMember reports whether the cursor is at a keyword-first KerML
// relationship member. The forms are KerML-only, so SysML notation is untouched.
func (p *Parser) atRelationshipMember() bool {
	if p.src.Kind() != source.KindKerML || !p.at(lexer.Keyword) {
		return false
	}
	kw := p.peek().KeywordID
	if kw == "featuring" {
		return true
	}
	if _, ok := relationshipMemberForms[kw]; ok {
		return p.atRelationshipMemberFirstEnd(1)
	}
	// A prefix keyword only introduces a member when its own keyword follows,
	// possibly behind the relationship's identification.
	for name, form := range relationshipMemberForms {
		if form.prefix != kw {
			continue
		}
		for off := 1; off <= 5; off++ {
			t := p.peekN(off)
			if t.Kind == lexer.Keyword && t.KeywordID == name {
				return p.atRelationshipMemberFirstEnd(off + 1)
			}
			if t.Kind != lexer.Identifier && t.Kind != lexer.UnrestrictedName &&
				t.Kind != lexer.Lt && t.Kind != lexer.Gt {
				break
			}
		}
	}
	return false
}

// atRelationshipMemberKeyword reports whether the cursor is at the keyword
// naming a relationship kind rather than at the relationship's own name.
func (p *Parser) atRelationshipMemberKeyword() bool {
	if !p.at(lexer.Keyword) {
		return false
	}
	_, ok := relationshipMemberForms[p.peek().KeywordID]
	return ok
}

// atRelationshipMemberFirstEnd reports whether a name starts at off, which is
// what tells `inverse f of g;` (a member) from the `inverse of g` clause of a
// feature declaration.
func (p *Parser) atRelationshipMemberFirstEnd(off int) bool {
	t := p.peekN(off)
	return t.Kind == lexer.Identifier || t.Kind == lexer.UnrestrictedName
}

// parseRelationshipMember parses a keyword-first KerML relationship member into
// an anonymous feature carrying one relationship per end, as the `disjoint`
// member does.
func (p *Parser) parseRelationshipMember(start int, vis ast.Visibility, trivia []ast.Trivia) ast.Node {
	if p.atKeyword("featuring") {
		return p.parseTypeFeaturingMember(start, vis, trivia)
	}
	var ident ast.Identification
	kw := p.peek().KeywordID
	form, ok := relationshipMemberForms[kw]
	if !ok {
		// A prefix keyword: `specialization Gen subtype A specializes B;`,
		// whose identification is optional.
		p.advance()
		if !p.atRelationshipMemberKeyword() {
			ident = p.parseIdentification()
		}
		kw = p.peek().KeywordID
		form, ok = relationshipMemberForms[kw]
		if !ok {
			en := p.errorNodeSkip(start, "expected a relationship keyword after '"+kw+"'")
			en.SetLeadingTrivia(trivia)
			return en
		}
	}
	p.advance() // the relationship keyword

	sourceEnd := p.parseRelationshipTarget()
	if !p.acceptRelationshipSeparator(form) {
		en := p.errorNodeSkip(start, "expected '"+form.sepKeyword+"' between the ends of a "+kw+" relationship")
		en.SetLeadingTrivia(trivia)
		return en
	}
	targetEnd := p.parseRelationshipTarget()

	u := &ast.Usage{
		Kind:  ast.UsageAttribute,
		Ident: ident,
		Relationships: []*ast.Relationship{
			{Kind: form.kind, Target: sourceEnd},
			{Kind: form.kind, Target: targetEnd, Conjugated: form.conjugated},
		},
	}
	u.Members, u.HasBody = p.parseDefUsageBody()
	u.NodeSpan = p.spanFrom(start)
	u.SetLeadingTrivia(trivia)

	m := &ast.Membership{Visibility: vis, Member: u}
	m.NodeSpan = u.Span()
	m.SetLeadingTrivia(trivia)
	return m
}

// acceptRelationshipSeparator consumes the separator between the two ends, in
// either its keyword or its symbol spelling.
func (p *Parser) acceptRelationshipSeparator(form relationshipMemberForm) bool {
	if p.acceptKeyword(form.sepKeyword) {
		if form.sepKeyword2 != "" {
			p.expect2Keyword(form.sepKeyword2)
		}
		return true
	}
	if form.sepToken != lexer.EOF && p.at(form.sepToken) {
		p.advance()
		return true
	}
	return false
}

// parseTypeFeaturingMember parses `featuring ( <id> of )? f by T ;`
// (KerML.xtext TypeFeaturing:652).
func (p *Parser) parseTypeFeaturingMember(start int, vis ast.Visibility, trivia []ast.Trivia) ast.Node {
	p.advance() // 'featuring'
	var ident ast.Identification
	if !p.atKeyword("of") {
		cp := p.checkpoint()
		ident = p.parseIdentification()
		if !p.acceptKeyword("of") {
			// No `of`: what was read is the featured feature, not a name.
			p.restore(cp)
			ident = ast.Identification{}
		}
	} else {
		p.advance() // 'of'
	}
	featured := p.parseRelationshipTarget()
	if !p.acceptKeyword("by") {
		en := p.errorNodeSkip(start, "expected 'by' after the featured feature of a featuring relationship")
		en.SetLeadingTrivia(trivia)
		return en
	}
	featuring := p.parseRelationshipTarget()

	u := &ast.Usage{
		Kind:  ast.UsageAttribute,
		Ident: ident,
		Relationships: []*ast.Relationship{
			{Kind: ast.RelFeaturedBy, Target: featured},
			{Kind: ast.RelFeaturedBy, Target: featuring},
		},
	}
	u.Members, u.HasBody = p.parseDefUsageBody()
	u.NodeSpan = p.spanFrom(start)
	u.SetLeadingTrivia(trivia)

	m := &ast.Membership{Visibility: vis, Member: u}
	m.NodeSpan = u.Span()
	m.SetLeadingTrivia(trivia)
	return m
}
