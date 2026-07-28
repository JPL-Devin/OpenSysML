package parser

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
)

// atName reports whether the current token can begin a name segment.
func (p *Parser) atName() bool {
	k := p.peek().Kind
	return k == lexer.Identifier || k == lexer.UnrestrictedName
}

// parseNameSegment consumes one name token and returns its segment.
func (p *Parser) parseNameSegment() (ast.NameSegment, bool) {
	if !p.atName() {
		return ast.NameSegment{}, false
	}
	tok := p.advance()
	return ast.NameSegment{Text: p.src.Text(tok.Span), Span: tok.Span}, true
}

// parseQualifiedName parses `[$::] Name (:: Name)*`. It returns nil and
// records a diagnostic if no name is present.
func (p *Parser) parseQualifiedName() *ast.QualifiedName {
	start := p.peek().Span.Offset
	trivia := p.takeTrivia()

	global := false
	if p.at(lexer.Dollar) && p.peekN(1).Kind == lexer.ColonColon {
		p.advance() // $
		p.advance() // ::
		global = true
	}

	seg, ok := p.parseNameSegment()
	if !ok {
		if global {
			// `$::` with no following name — still a (degenerate) global name.
			qn := &ast.QualifiedName{Global: true}
			qn.NodeSpan = p.spanFrom(start)
			qn.SetLeadingTrivia(trivia)
			return qn
		}
		p.error(p.peek().Span, "expected a name")
		return nil
	}

	parts := []ast.NameSegment{seg}
	for p.at(lexer.ColonColon) {
		// Do not consume `::` if it introduces `*`/`**` (namespace import wildcard).
		if nk := p.peekN(1).Kind; nk == lexer.Star || nk == lexer.StarStar {
			break
		}
		p.advance() // ::
		next, ok := p.parseNameSegment()
		if !ok {
			p.error(p.peek().Span, "expected a name after '::'")
			break
		}
		parts = append(parts, next)
	}

	qn := &ast.QualifiedName{Global: global, Parts: parts}
	qn.NodeSpan = p.spanFrom(start)
	qn.SetLeadingTrivia(trivia)
	return qn
}

// parseIdentification parses `<shortName> name?` or `name` or nothing.
// A missing identification yields a zero-value Identification (no diagnostic).
func (p *Parser) parseIdentification() ast.Identification {
	var id ast.Identification
	if p.at(lexer.Lt) {
		p.advance() // <
		if seg, ok := p.parseNameSegment(); ok {
			id.ShortName = seg.Text
			id.ShortNameSpan = seg.Span
		} else {
			p.error(p.peek().Span, "expected short name after '<'")
		}
		p.expect(lexer.Gt, "expected '>'")
	}
	if seg, ok := p.parseNameSegment(); ok {
		id.Name = seg.Text
		id.NameSpan = seg.Span
	}
	return id
}

// parseVisibility reads an optional public/private/protected prefix.
func (p *Parser) parseVisibility() ast.Visibility {
	switch {
	case p.acceptKeyword("public"):
		return ast.VisibilityPublic
	case p.acceptKeyword("private"):
		return ast.VisibilityPrivate
	case p.acceptKeyword("protected"):
		return ast.VisibilityProtected
	default:
		return ast.VisibilityDefault
	}
}

// parseMember parses one namespace member: an optional visibility prefix
// followed by a declaration. Import/Alias carry their own visibility and are
// returned directly; other declarations are wrapped in a Membership.
func (p *Parser) parseMember() ast.Node {
	start := p.peek().Span.Offset
	trivia := p.takeTrivia()
	vis := p.parseVisibility()

	// Import and Alias hold visibility internally and are not wrapped.
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
		// No declaration recognized. Emit an error node spanning the skip.
		en := p.errorNodeSkip(start, "expected a namespace member")
		en.SetLeadingTrivia(trivia)
		return en
	}
	m := &ast.Membership{Visibility: vis, Member: inner}
	m.NodeSpan = p.spanFrom(start)
	m.SetLeadingTrivia(trivia)
	return m
}

// parseDeclaration dispatches on the leading keyword to a declaration parser.
// Returns nil if the current token starts no known (in-scope) declaration.
func (p *Parser) parseDeclaration(start int) ast.Node {
	switch {
	case p.atKeyword("package"), p.atKeyword("library"), p.atKeyword("standard"):
		return p.parsePackage(start)
	case p.atKeyword("namespace"):
		return p.parseNamespace(start)
	case p.atKeyword("dependency"):
		return p.parseDependency(start)
	case p.atKeyword("comment"):
		return p.parseComment(start)
	case p.atKeyword("doc"):
		return p.parseDocumentation(start)
	case p.atKeyword("rep"), p.atKeyword("language"):
		return p.parseTextualRepresentation(start)
	case p.at(lexer.Hash):
		// Look past `# QualifiedName ...` prefixes for the declaration keyword.
		if p.leadingPrefixIsPackage() {
			return p.parsePackage(start)
		}
		if p.leadingPrefixIsNamespace() {
			return p.parseNamespace(start)
		}
		return nil
	default:
		return nil
	}
}

// parseNamespaceBody parses `{ member* }` or `;`. Returns (members, hasBody).
// The caller has already consumed the declaration head up to this point.
func (p *Parser) parseNamespaceBody() ([]ast.Node, bool) {
	if p.accept2(lexer.Semicolon) {
		return nil, false
	}
	if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); !ok {
		return nil, false
	}
	var members []ast.Node
	for !p.atEOF() && !p.at(lexer.RBrace) {
		before := p.peek().Span.Offset
		m := p.parseMember()
		if m != nil {
			members = append(members, m)
		}
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.advance()
		}
	}
	p.expect(lexer.RBrace, "expected '}'")
	return members, true
}

// accept2 is accept that discards the token (convenience for punctuation).
func (p *Parser) accept2(k lexer.Kind) bool {
	_, ok := p.accept(k)
	return ok
}

// errorNodeSkip builds an ErrorNode and skips tokens to the next `;`/`}`/EOF.
func (p *Parser) errorNodeSkip(start int, msg string) *ast.ErrorNode {
	p.error(p.peek().Span, msg)
	for !p.atEOF() && !p.at(lexer.Semicolon) && !p.at(lexer.RBrace) {
		p.advance()
	}
	p.accept2(lexer.Semicolon) // consume the terminator if present
	en := &ast.ErrorNode{Message: msg}
	en.NodeSpan = p.spanFrom(start)
	return en
}

// Temporary stubs (replaced in Tasks 8-11).
func (p *Parser) parseDependency(start int) ast.Node {
	return p.errorNodeSkip(start, "dependency: not yet implemented")
}
func (p *Parser) parseComment(start int) ast.Node {
	return p.errorNodeSkip(start, "comment: not yet implemented")
}
func (p *Parser) parseDocumentation(start int) ast.Node {
	return p.errorNodeSkip(start, "doc: not yet implemented")
}
func (p *Parser) parseTextualRepresentation(start int) ast.Node {
	return p.errorNodeSkip(start, "rep: not yet implemented")
}
// parseImport parses `import [all] QualifiedName [::*|::**] body`.
// Visibility has already been consumed by the caller.
func (p *Parser) parseImport(start int, vis ast.Visibility) *ast.Import {
	p.advance() // 'import' (guaranteed by caller)
	isAll := p.acceptKeyword("all")

	qn := p.parseQualifiedName()
	imp := &ast.Import{
		Visibility: vis,
		IsAll:      isAll,
		Kind:       ast.ImportMembership,
		Imported:   qn,
	}

	// Wildcard tail: `:: *` (namespace) then optional `:: **` (recursive),
	// or `:: **` directly.
	if p.at(lexer.ColonColon) {
		nk := p.peekN(1).Kind
		if nk == lexer.Star {
			p.advance() // ::
			p.advance() // *
			imp.Kind = ast.ImportNamespace
			if p.at(lexer.ColonColon) && p.peekN(1).Kind == lexer.StarStar {
				p.advance() // ::
				p.advance() // **
				imp.IsRecursive = true
			}
		} else if nk == lexer.StarStar {
			p.advance() // ::
			p.advance() // **
			imp.Kind = ast.ImportNamespace
			imp.IsRecursive = true
		}
	}

	imp.Body, imp.HasBody = p.parseNamespaceBody()
	imp.NodeSpan = p.spanFrom(start)
	return imp
}
func (p *Parser) parseAlias(start int, vis ast.Visibility) *ast.Alias {
	en := p.errorNodeSkip(start, "alias: not yet implemented")
	al := &ast.Alias{Visibility: vis}
	al.NodeSpan = en.NodeSpan
	return al
}

// parsePrefixMetadata parses zero or more `# QualifiedName` prefix annotations.
func (p *Parser) parsePrefixMetadata() []*ast.PrefixMetadata {
	var prefixes []*ast.PrefixMetadata
	for p.at(lexer.Hash) {
		start := p.peek().Span.Offset
		p.advance() // #
		qn := p.parseQualifiedName()
		pm := &ast.PrefixMetadata{Type: qn}
		pm.NodeSpan = p.spanFrom(start)
		prefixes = append(prefixes, pm)
	}
	return prefixes
}

// parsePackage parses `[standard] [library] package <id> body`.
// Prefix metadata may precede `package`; it is consumed here.
func (p *Parser) parsePackage(start int) ast.Node {
	prefixes := p.parsePrefixMetadata()
	isStandard := p.acceptKeyword("standard")
	isLibrary := p.acceptKeyword("library")
	if !p.acceptKeyword("package") {
		return p.errorNodeSkip(start, "expected 'package'")
	}
	id := p.parseIdentification()
	members, hasBody := p.parseNamespaceBody()
	pkg := &ast.Package{
		Prefixes:   prefixes,
		Ident:      id,
		IsLibrary:  isLibrary,
		IsStandard: isStandard,
		Members:    members,
		HasBody:    hasBody,
	}
	pkg.NodeSpan = p.spanFrom(start)
	return pkg
}

// parseNamespace parses `namespace <id> body`.
func (p *Parser) parseNamespace(start int) ast.Node {
	prefixes := p.parsePrefixMetadata()
	if !p.acceptKeyword("namespace") {
		return p.errorNodeSkip(start, "expected 'namespace'")
	}
	id := p.parseIdentification()
	members, hasBody := p.parseNamespaceBody()
	ns := &ast.Namespace{Prefixes: prefixes, Ident: id, Members: members, HasBody: hasBody}
	ns.NodeSpan = p.spanFrom(start)
	return ns
}

// prefixLookahead returns the buffer index of the token following all
// leading `# QualifiedName` prefixes, without consuming anything.
func (p *Parser) prefixLookahead() int {
	i := 0
	for p.peekN(i).Kind == lexer.Hash {
		i++ // '#'
		// QualifiedName: Name (:: Name)*
		if k := p.peekN(i).Kind; k != lexer.Identifier && k != lexer.UnrestrictedName {
			return i
		}
		i++
		for p.peekN(i).Kind == lexer.ColonColon {
			i++
			if k := p.peekN(i).Kind; k != lexer.Identifier && k != lexer.UnrestrictedName {
				return i
			}
			i++
		}
	}
	return i
}

func (p *Parser) leadingPrefixIsPackage() bool {
	t := p.peekN(p.prefixLookahead())
	return t.Kind == lexer.Keyword && (t.KeywordID == "package" || t.KeywordID == "library" || t.KeywordID == "standard")
}

func (p *Parser) leadingPrefixIsNamespace() bool {
	t := p.peekN(p.prefixLookahead())
	return t.Kind == lexer.Keyword && t.KeywordID == "namespace"
}
