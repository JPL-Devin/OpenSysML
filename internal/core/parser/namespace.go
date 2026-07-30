package parser

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
	"github.com/Open-MBEE/Systemica/internal/core/source"
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
	case p.atKeyword("filter"):
		return p.parseFilter(start)
	case p.atDefUsageStart():
		return p.parseDefUsage(start)
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

// declStartKeywords are keywords that begin a namespace-member declaration.
// They serve as recovery sync points so a malformed member does not swallow
// the declaration that follows it.
var declStartKeywords = map[string]bool{
	"package":    true,
	"namespace":  true,
	"library":    true,
	"standard":   true,
	"dependency": true,
	"comment":    true,
	"doc":        true,
	"rep":        true,
	"language":   true,
	"alias":      true,
	"import":     true,
	"public":     true,
	"private":    true,
	"protected":  true,
	"part":       true,
	"attribute":  true,
	"def":        true,
	"abstract":   true,
	"variation":  true,
	"ref":        true,
	"in":         true,
	"out":        true,
	"inout":      true,
	"composite":  true,
	"portion":    true,
	"derived":    true,
	"ordered":    true,
	"nonunique":  true,
}

// atMemberSync reports whether the parser sits at a recovery synchronization
// point: EOF, `;`, `}`, `#`, or a declaration-start keyword.
func (p *Parser) atMemberSync() bool {
	if p.atEOF() || p.at(lexer.Semicolon) || p.at(lexer.RBrace) || p.at(lexer.Hash) {
		return true
	}
	t := p.peek()
	return t.Kind == lexer.Keyword && declStartKeywords[t.KeywordID]
}

// errorNodeSkip builds an ErrorNode and skips tokens up to the next member
// sync point (`;`/`}`/`#`/declaration keyword/EOF), leaving that token for the
// enclosing loop. It always advances at least one token (unless already at
// EOF/`;`/`}`) so parsing makes progress.
func (p *Parser) errorNodeSkip(start int, msg string) *ast.ErrorNode {
	p.error(p.peek().Span, msg)
	if !p.atEOF() && !p.at(lexer.Semicolon) && !p.at(lexer.RBrace) {
		p.advance()
	}
	for !p.atMemberSync() {
		p.advance()
	}
	p.accept2(lexer.Semicolon) // consume the terminator if present
	en := &ast.ErrorNode{Message: msg}
	en.NodeSpan = p.spanFrom(start)
	return en
}

// expectCommentBody consumes a trailing /* */ regular comment and returns its
// span. It peeks first to force any trailing comment into the pending slot.
func (p *Parser) expectCommentBody(start int) source.Span {
	p.peek()
	if sp, ok := p.takePendingComment(); ok {
		return sp
	}
	p.error(p.peek().Span, "expected a /* ... */ comment body")
	return p.spanFrom(start)
}

func (p *Parser) parseComment(start int) ast.Node {
	p.advance() // 'comment'
	c := &ast.Comment{}
	if p.atName() && !p.atKeyword("about") && !p.atKeyword("locale") {
		c.Ident = p.parseIdentification()
	}
	if p.acceptKeyword("about") {
		c.About = p.parseQualifiedNameList()
	}
	if p.acceptKeyword("locale") {
		if tok, ok := p.expect(lexer.String, "expected locale string"); ok {
			c.Locale = p.src.Text(tok.Span)
		}
	}
	c.BodySpan = p.expectCommentBody(start)
	c.NodeSpan = p.spanFrom(start)
	return c
}

func (p *Parser) parseDocumentation(start int) ast.Node {
	p.advance() // 'doc'
	d := &ast.Documentation{}
	if p.atName() && !p.atKeyword("locale") {
		d.Ident = p.parseIdentification()
	}
	if p.acceptKeyword("locale") {
		if tok, ok := p.expect(lexer.String, "expected locale string"); ok {
			d.Locale = p.src.Text(tok.Span)
		}
	}
	d.BodySpan = p.expectCommentBody(start)
	d.NodeSpan = p.spanFrom(start)
	return d
}

func (p *Parser) parseTextualRepresentation(start int) ast.Node {
	r := &ast.TextualRepresentation{}
	if p.acceptKeyword("rep") {
		if p.atName() && !p.atKeyword("language") {
			r.Ident = p.parseIdentification()
		}
	}
	if !p.acceptKeyword("language") {
		return p.errorNodeSkip(start, "expected 'language'")
	}
	if tok, ok := p.expect(lexer.String, "expected representation language string"); ok {
		r.Language = p.src.Text(tok.Span)
	}
	r.BodySpan = p.expectCommentBody(start)
	r.NodeSpan = p.spanFrom(start)
	return r
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

	// Wildcard tail (per KerML.xtext ImportedMembership/ImportedNamespace):
	//   `:: *`          -> namespace import (may then take `:: **` recursive)
	//   `:: **` directly -> recursive MEMBERSHIP import (no `::*`)
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
			p.advance()            // ::
			p.advance()            // **
			imp.IsRecursive = true // recursive membership import; Kind stays ImportMembership
		}
	}

	imp.Body, imp.HasBody = p.parseNamespaceBody()
	imp.NodeSpan = p.spanFrom(start)
	return imp
}
func (p *Parser) parseAlias(start int, vis ast.Visibility) *ast.Alias {
	p.advance() // 'alias'
	id := p.parseIdentification()
	al := &ast.Alias{Visibility: vis, Ident: id}
	if !p.acceptKeyword("for") {
		p.error(p.peek().Span, "expected 'for' in alias")
	} else {
		al.For = p.parseQualifiedName()
	}
	al.Body, al.HasBody = p.parseNamespaceBody()
	al.NodeSpan = p.spanFrom(start)
	return al
}

// parseQualifiedNameList parses `QualifiedName (, QualifiedName)*`.
func (p *Parser) parseQualifiedNameList() []*ast.QualifiedName {
	var list []*ast.QualifiedName
	if qn := p.parseQualifiedName(); qn != nil {
		list = append(list, qn)
	}
	for p.at(lexer.Comma) {
		p.advance() // ,
		if qn := p.parseQualifiedName(); qn != nil {
			list = append(list, qn)
		}
	}
	return list
}

// parseDependency parses
// `[# prefixes] dependency [<id> from] clients to suppliers body`.
func (p *Parser) parseDependency(start int) ast.Node {
	prefixes := p.parsePrefixMetadata()
	if !p.acceptKeyword("dependency") {
		return p.errorNodeSkip(start, "expected 'dependency'")
	}
	dep := &ast.Dependency{Prefixes: prefixes}

	// Optional `<id> [name] from`. The `from` keyword disambiguates: an
	// identification is present only if a `from` follows it.
	if p.identificationThenFrom() {
		dep.Ident = p.parseIdentification()
		p.acceptKeyword("from") // guaranteed
	}

	dep.Clients = p.parseQualifiedNameList()
	if !p.acceptKeyword("to") {
		p.error(p.peek().Span, "expected 'to' in dependency")
	} else {
		dep.Suppliers = p.parseQualifiedNameList()
	}
	dep.Body, dep.HasBody = p.parseNamespaceBody()
	dep.NodeSpan = p.spanFrom(start)
	return dep
}

// identificationThenFrom reports whether the upcoming tokens form an
// identification (`<x> y` / `y`) immediately followed by `from`.
func (p *Parser) identificationThenFrom() bool {
	i := 0
	if p.peekN(i).Kind == lexer.Lt {
		i++ // <
		if k := p.peekN(i).Kind; k == lexer.Identifier || k == lexer.UnrestrictedName {
			i++
		}
		if p.peekN(i).Kind == lexer.Gt {
			i++
		}
	}
	if k := p.peekN(i).Kind; k == lexer.Identifier || k == lexer.UnrestrictedName {
		i++
	}
	t := p.peekN(i)
	return t.Kind == lexer.Keyword && t.KeywordID == "from"
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

// parseFilter parses `filter OwnedExpression ;` (ElementFilterMember).
func (p *Parser) parseFilter(start int) ast.Node {
	p.advance() // filter
	expr := p.ParseExpression()
	p.expect(lexer.Semicolon, "expected ';' after filter expression")
	f := &ast.FilterMember{Condition: expr}
	f.NodeSpan = p.spanFrom(start)
	return f
}
