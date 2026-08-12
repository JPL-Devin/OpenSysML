package parser

// Member-attached `then` (SysML.xtext EmptySuccessionMember): the keyword
// sequences the member before it with the member after it, so it describes
// neither and is desugared into the *ast.SuccessionEdge the `then a b;` form
// builds — the node lowering and RDF already honour.
//
// The grammar admits it only before an occurrence usage, so anything else after
// it is an error; an edge names its ends, so a `then` beside a member with no
// name warns instead.

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// nonOccurrenceUsageKeywords are the usage keywords a `then` may not precede:
// SysML.xtext lists them under NonOccurrenceUsageElement rather than under
// StructureUsageElement or BehaviorUsageElement.
var nonOccurrenceUsageKeywords = map[string]string{
	"attribute":  "an attribute usage",
	"datatype":   "an attribute usage",
	"feature":    "a feature",
	"ref":        "a reference usage",
	"enum":       "an enumeration usage",
	"binding":    "a binding",
	"bind":       "a binding",
	"succession": "a succession",
}

// namespaceMemberKeywords are the members a `then` may not precede because they
// are namespace members rather than usages.
var namespaceMemberKeywords = map[string]string{
	"package":   "a package",
	"namespace": "a namespace",
	"import":    "an import",
	"alias":     "an alias",
	"doc":       "a documentation comment",
	"comment":   "a comment",
	"rep":       "a textual representation",
	"filter":    "a filter",
}

// bodyBuilder accumulates one body's member list, desugaring a member-attached
// `then` as the two members it joins are parsed. A body loop takes the keyword
// (atSuccession/takeSuccession) and parses the member after it as it normally
// would, so the desugaring is independent of the members a body admits.
type bodyBuilder struct {
	p       *Parser
	members []ast.Node

	// last names the last member a succession can reference, the source a `then`
	// taken next sequences from. An edge or an unnamed member is not one.
	last      string
	lastSpan  source.Span
	hasMember bool // whether any member precedes, named or not

	// pending is set between taking a `then` and adding the member it prefixes;
	// valid is false once it has been diagnosed, so a bad succession is reported
	// rather than also synthesised.
	pending    bool
	pendingAt  source.Span
	valid      bool
	source     string
	sourceSpan source.Span
}

func (p *Parser) newBodyBuilder() *bodyBuilder {
	return &bodyBuilder{p: p}
}

// atSuccession reports whether the parser is at a `then` prefixing a body
// member, as opposed to one starting a succession edge member (`then a b;`) or
// chaining an inline statement (`then assign x := 1;`).
func (b *bodyBuilder) atSuccession() bool {
	p := b.p
	if !p.atKeyword("then") {
		return false
	}
	next := p.peekN(1)
	if next.Kind != lexer.Keyword {
		// `then a b;` and `then a;` name members; the edge parser reads them.
		return false
	}
	kw := next.KeywordID
	if _, ok := namespaceMemberKeywords[kw]; ok {
		// Diagnosed as an illegal target rather than left to parse as a
		// namespace member with the keyword dropped.
		return true
	}
	if featureModifierKeywords[kw] {
		// A modifier cannot be the name of a member, so `then private action
		// whileLoop …` can only be a prefixed declaration.
		return true
	}
	if kw == "send" {
		// A send statement has no name of its own: taken here so the `then` is
		// accounted for rather than read as an edge naming the keyword.
		return true
	}
	// The name follows the kind keyword, which `use case` spells in two words.
	nameAt := 2
	if kw == "use" {
		if word := p.peekN(2); word.Kind != lexer.Keyword || word.KeywordID != "case" {
			return false
		}
		nameAt = 3
	} else {
		_, isUsage := usageKindKeywords[kw]
		_, isDef := definitionKindKeywords[kw]
		if !isUsage && !isDef {
			// `then done;`, `then first x;`, `then accept …`: node and edge
			// forms with their own parsers.
			return false
		}
		if startsInlineSuccessionStatement(next) && kw != "action" {
			// `then assign …`, `then perform …`, `then while …`, `then if …`
			// sequence statements inside one node body, where the lowered block
			// already runs its statements in order.
			return false
		}
	}
	switch name := p.peekN(nameAt); name.Kind {
	case lexer.Identifier, lexer.Keyword, lexer.UnrestrictedName:
	default:
		// `then part;` and `then action { … }` declare an anonymous member,
		// which an edge end cannot name: taken so the succession is diagnosed
		// rather than read as an edge naming the kind keyword.
		return !b.declares(kw)
	}
	// `then <kw> <name>;` is ambiguous: a two-name edge whose source is a
	// keyword used as a member name (`then flow end;`), or a declaration of
	// <name> prefixed with `then` (`then action b;`). Only a name already
	// declared in this body can be an edge's source, so the edge reading wins
	// exactly when the keyword names such a member.
	if p.peekN(nameAt+1).Kind == lexer.Semicolon && b.declares(kw) {
		return false
	}
	return true
}

// declares reports whether a member of this body was declared with this name.
func (b *bodyBuilder) declares(name string) bool {
	for _, m := range b.members {
		if memberDeclaredName(m) == name {
			return true
		}
	}
	return false
}

// takeSuccession consumes a member-attached `then`, reporting the forms the
// grammar does not allow. The member it prefixes is parsed by the caller.
func (b *bodyBuilder) takeSuccession() {
	p := b.p
	tok := p.advance() // consume 'then'
	if b.pending {
		p.error(tok.Span, "`then` cannot follow another `then`: a succession sequences two members, so each keyword needs a member between it and the next")
		return
	}
	b.pending, b.pendingAt, b.valid = true, tok.Span, true
	b.source, b.sourceSpan = b.last, b.lastSpan

	// One diagnostic per keyword: the first thing wrong with it is enough to
	// say why no succession was built.
	switch what, illegal := b.illegalTarget(); {
	case illegal:
		p.error(tok.Span, fmt.Sprintf("`then` cannot sequence %s: it sequences the members either side of it, which the notation allows only before an occurrence usage such as a part, item, action or state", what))
	case !b.hasMember:
		p.error(tok.Span, "`then` has no member before it to sequence from: it sequences the member after it with the member before it, so a body cannot begin with one")
	case b.source == "":
		p.warn(tok.Span, unnamedEndWarning("from"), codeUnnamedSuccessionEnd)
	default:
		return
	}
	b.valid = false
}

// illegalTarget describes the member beginning at the parser's position, just
// past a taken `then`, when the grammar does not allow a succession before it.
func (b *bodyBuilder) illegalTarget() (string, bool) {
	p := b.p
	for i := 0; ; i++ {
		tok := p.peekN(i)
		if tok.Kind != lexer.Keyword {
			if tok.Kind == lexer.Identifier {
				// A bare name is a DefaultReferenceUsage.
				return "a reference usage", true
			}
			return "", false
		}
		kw := tok.KeywordID
		if what, ok := namespaceMemberKeywords[kw]; ok {
			return what, true
		}
		if _, isDef := definitionKindKeywords[kw]; isDef && p.peekN(i+1).Kind == lexer.Keyword && p.peekN(i+1).KeywordID == "def" {
			return "a definition", true
		}
		if _, isUsage := usageKindKeywords[kw]; isUsage {
			// `ref` is both a modifier and a usage keyword: `then ref part x;`
			// is a part usage, `then ref x;` a reference usage.
			if what, isNon := nonOccurrenceUsageKeywords[kw]; isNon && !(kw == "ref" && b.kindFollows(i+1)) {
				return what, true
			}
			return "", false
		}
		if !featureModifierKeywords[kw] {
			return "", false
		}
	}
}

// kindFollows reports whether the token at offset i is a usage kind keyword,
// which tells a modifier apart from the kind it qualifies.
func (b *bodyBuilder) kindFollows(i int) bool {
	tok := b.p.peekN(i)
	if tok.Kind != lexer.Keyword {
		return false
	}
	_, ok := usageKindKeywords[tok.KeywordID]
	return ok
}

// add appends a member, synthesising the succession edge of a pending `then`
// after it. A succession that cannot be built is reported rather than dropped,
// since dropping it would silently change execution order.
func (b *bodyBuilder) add(m ast.Node) {
	if m == nil {
		return
	}
	pending, at, valid := b.pending, b.pendingAt, b.valid
	b.pending = false

	// `then <target>;` leaves its source to the member before it, the same member
	// a member-attached `then` sequences from, rather than to a consumer's guess.
	if edge, ok := m.(*ast.SuccessionEdge); ok && len(sourceParts(edge)) == 0 && b.last != "" {
		edge.Source = memberReference(b.last, b.lastSpan)
	}

	target := memberDeclaredName(m)
	b.members = append(b.members, m)
	if !isEdgeMember(m) {
		// An unnamed member clears the source too: keeping an older name would
		// sequence from a member other than the one before the keyword.
		b.hasMember = true
		b.last, b.lastSpan = target, m.Span()
	}

	if !pending || !valid {
		return
	}
	if target == "" {
		b.p.warn(at, unnamedEndWarning("to"), codeUnnamedSuccessionEnd)
		return
	}
	b.members = append(b.members, synthesizeSuccession(b.source, b.sourceSpan, target, m.Span(), at))
}

// unnamedEndWarning reports a succession this representation cannot carry: a
// succession edge names its ends, and the member on this end declares no name.
func unnamedEndWarning(end string) string {
	return fmt.Sprintf("`then` sequences %s a member with no name, so no succession is recorded for it: name that member, or write the succession as its own member", end)
}

// sourceParts returns the name segments an edge's source end names, which the
// one-name notation (`then b;`) leaves empty.
func sourceParts(edge *ast.SuccessionEdge) []ast.NameSegment {
	if edge.Source == nil {
		return nil
	}
	return edge.Source.Parts
}

// isEdgeMember reports whether a member is an edge between other members, which
// declares no name of its own for a succession to reference.
func isEdgeMember(m ast.Node) bool {
	switch n := m.(type) {
	case *ast.Membership:
		return n.Member != nil && isEdgeMember(n.Member)
	case *ast.SuccessionEdge, *ast.ControlFlowEdge, *ast.ObjectFlowEdge, *ast.TransitionMember:
		return true
	case *ast.Usage:
		// `a then b;` and the connector forms are usages of an edge kind.
		switch n.Kind {
		case ast.UsageSuccession, ast.UsageTransition, ast.UsageConnector,
			ast.UsageFlow, ast.UsageBinding:
			return true
		}
	}
	return false
}

// finish returns the body's members, reporting a `then` that no member follows.
func (b *bodyBuilder) finish() []ast.Node {
	if b.pending {
		b.p.error(b.pendingAt, "`then` has no member after it to sequence to: the keyword sequences the member after it with the member before it")
		b.pending = false
	}
	return b.members
}

// synthesizeSuccession builds the edge a member-attached `then` desugars to,
// spanned at the keyword so diagnostics point at what the author wrote.
func synthesizeSuccession(source string, sourceSpan source.Span, target string, targetSpan, at source.Span) *ast.SuccessionEdge {
	edge := &ast.SuccessionEdge{
		Source: memberReference(source, sourceSpan),
		Target: memberReference(target, targetSpan),
	}
	edge.NodeSpan = at
	return edge
}

// memberReference names a member of the same body, spanned at its declaration.
func memberReference(name string, sp source.Span) *ast.QualifiedName {
	qn := &ast.QualifiedName{Parts: []ast.NameSegment{{Text: name, Span: sp}}}
	qn.NodeSpan = sp
	return qn
}

// memberDeclaredName returns the name a succession can reference a member by,
// or "" when it declares none.
func memberDeclaredName(member ast.Node) string {
	switch n := member.(type) {
	case *ast.Membership:
		if n.Member == nil {
			return ""
		}
		return memberDeclaredName(n.Member)
	case *ast.Usage:
		return identifierName(n.Ident)
	case *ast.Definition:
		return identifierName(n.Ident)
	case *ast.Package:
		return identifierName(n.Ident)
	case *ast.Namespace:
		return identifierName(n.Ident)
	case *ast.ActionExecutionNode:
		return n.Name
	case *ast.InitialNode:
		return n.Name
	case *ast.FinalNode:
		return n.Name
	case *ast.ForkNode:
		return n.Name
	case *ast.JoinNode:
		return n.Name
	case *ast.MergeNode:
		return n.Name
	case *ast.DecisionNode:
		return n.Name
	case *ast.StateNode:
		return n.Name
	case *ast.SubstateMember:
		return n.Name
	case *ast.PseudostateNode:
		return n.Name
	}
	return ""
}

// identifierName falls back to the short name, the other spelling a reference
// can use.
func identifierName(id ast.Identification) string {
	if id.Name != "" {
		return id.Name
	}
	return id.ShortName
}
