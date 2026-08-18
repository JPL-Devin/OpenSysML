package edit

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// target returns the declaration an operation names. Only a declaration of this
// model's own document can be edited: its source is the only one being rewritten.
func (m Model) target(i int, op Operation) (*symbols.Symbol, error) {
	var declaring []*symbols.Symbol
	for _, sym := range m.Index.LookupQualifiedFrom(op.Target, op.Target) {
		if sym != nil && m.Index.GetFQN(sym) == op.Target {
			declaring = append(declaring, sym)
		}
	}
	switch len(declaring) {
	case 0:
		return nil, &Error{
			Failure:        FailureUnknownTarget,
			OperationIndex: i,
			Message:        fmt.Sprintf("no element named %q in this model", op.Target),
		}
	case 1:
	default:
		return nil, &Error{
			Failure:        FailureAmbiguousTarget,
			OperationIndex: i,
			Message: fmt.Sprintf("%q names %d declarations; it does not say which to edit",
				op.Target, len(declaring)),
		}
	}
	sym := declaring[0]
	if doc := sym.DocName; doc != m.Source.Name() {
		return nil, &Error{
			Failure:        FailureUnknownTarget,
			OperationIndex: i,
			Message: fmt.Sprintf("%q is declared in %s, not in this model's source",
				op.Target, docLabel(doc)),
		}
	}
	return sym, nil
}

// docLabel names a document for a message, for the library declarations that
// carry no document name of their own.
func docLabel(doc string) string {
	if doc == "" {
		return "the standard library"
	}
	return doc
}

// valueSplice is the byte range a set-value operation rewrites: the expression
// of an existing `= <expr>`, or an insertion before the declaration's `;`.
func (m Model) valueSplice(i int, op Operation, sym *symbols.Symbol) (splice, error) {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return splice{}, &Error{
			Failure:        FailureNotValued,
			OperationIndex: i,
			Message: fmt.Sprintf("%s declares no feature that can carry a value (%s)",
				op.Target, sym.Kind),
		}
	}
	if err := m.checkValue(i, op); err != nil {
		return splice{}, err
	}
	if usage.Value != nil {
		return splice{
			span:    m.tokenSpan(usage.Value.Span()),
			text:    op.Value,
			opIndex: i,
			target:  op.Target,
		}, nil
	}
	if usage.ConnectorEnds != nil || usage.FlowEnds != nil {
		return splice{}, &Error{
			Failure:        FailureNotValued,
			OperationIndex: i,
			Message:        fmt.Sprintf("%s connects features rather than carrying a value", op.Target),
		}
	}
	semi, ok := m.terminator(usage)
	if !ok {
		return splice{}, &Error{
			Failure:        FailureNotValued,
			OperationIndex: i,
			Message: fmt.Sprintf("%s has no value and no terminating ';' to add one before"+
				" (a value cannot be added to a declaration with a body)", op.Target),
		}
	}
	text := "= " + op.Value
	if semi.Offset > 0 && !isSpace(m.Source.Bytes()[semi.Offset-1]) {
		text = " " + text
	}
	return splice{
		span:    source.Span{Offset: semi.Offset, Len: 0},
		text:    text,
		opIndex: i,
		target:  op.Target,
	}, nil
}

// tokenSpan narrows a node's span to the bytes its own tokens cover. A span ends
// at the next token's start, so whitespace and comments written after the node
// fall inside it and would be spliced away with it.
func (m Model) tokenSpan(span source.Span) source.Span {
	end := span.Offset
	lx := lexer.New(m.Source)
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.Span.Offset >= span.End() {
			break
		}
		if tok.Span.Offset < span.Offset || tok.IsTrivia() || tok.Kind == lexer.RegularComment {
			continue
		}
		if e := tok.Span.End(); e > end && e <= span.End() {
			end = e
		}
	}
	if end <= span.Offset {
		return span
	}
	return source.Span{Offset: span.Offset, Len: end - span.Offset}
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

// terminator returns the span of the `;` ending a declaration that has no body.
// Only such a declaration can take a value appended to it.
func (m Model) terminator(usage *ast.Usage) (source.Span, bool) {
	if usage.HasBody {
		return source.Span{}, false
	}
	span := usage.Span()
	var last source.Span
	found := false
	lx := lexer.New(m.Source)
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.Span.Offset >= span.End() {
			break
		}
		if tok.Span.Offset >= span.Offset && tok.Kind == lexer.Semicolon {
			last, found = tok.Span, true
		}
	}
	return last, found
}

// renameSplice is the byte range a rename rewrites: the declaration's own name
// token. References are not rewritten, so a referenced declaration is refused.
func (m Model) renameSplice(i int, op Operation, sym *symbols.Symbol) (splice, error) {
	ident, ok := declIdent(sym.Decl)
	if !ok || ident.Name == "" || ident.NameSpan.Len == 0 {
		return splice{}, &Error{
			Failure:        FailureNotNamed,
			OperationIndex: i,
			Message: fmt.Sprintf("%s declares no name of its own to rename"+
				" (a shorthand redefinition names the feature it redefines)", op.Target),
		}
	}
	if err := checkName(i, op.NewName); err != nil {
		return splice{}, err
	}
	if taken, ok := m.nameTaken(sym, op.NewName); ok {
		return splice{}, &Error{
			Failure:        FailureInvalidName,
			OperationIndex: i,
			Message: fmt.Sprintf("%s cannot be renamed to %q: that name already means %s where"+
				" %s is declared, so the rename would make it ambiguous or silently rebind"+
				" what reads that name",
				op.Target, op.NewName, taken, op.Target),
		}
	}
	if referring := m.referringTo(sym, ident.NameSpan); len(referring) > 0 {
		return splice{}, &Error{
			Failure:        FailureRenameReferenced,
			OperationIndex: i,
			Referring:      referring,
			Message: fmt.Sprintf("%s is referenced by %s; renaming it would break those"+
				" references, which this edit does not rewrite",
				op.Target, strings.Join(referring, ", ")),
		}
	}
	return splice{span: ident.NameSpan, text: op.NewName, opIndex: i, target: op.Target}, nil
}

// nameTaken names what the new name already means where sym is declared, with
// sym's own binding hidden — a sibling, or a name reached through an enclosing
// namespace, an import or inheritance. Renaming onto such a name says something
// the caller did not ask for: a sibling makes the qualified name ambiguous, and
// anything else is shadowed, so every expression reading that name silently
// starts reading the renamed element. Neither is diagnosed by re-analysis, since
// the name still resolves.
func (m Model) nameTaken(sym *symbols.Symbol, newName string) (string, bool) {
	if sym.OwnerScope == nil {
		return "", false
	}
	other, ok := resolve.New(m.Index).LookupNameExcluding(sym.OwnerScope, newName, sym.Decl)
	if !ok || sameSymbol(other, sym) {
		return "", false
	}
	if fqn := m.Index.GetFQN(other); fqn != "" {
		return fqn, true
	}
	return other.Name, true
}

// declIdent returns the identification a declaration node carries.
func declIdent(node ast.Node) (ast.Identification, bool) {
	switch d := node.(type) {
	case *ast.Definition:
		return d.Ident, true
	case *ast.Usage:
		return d.Ident, true
	case *ast.Package:
		return d.Ident, true
	case *ast.Namespace:
		return d.Ident, true
	case *ast.Alias:
		return d.Ident, true
	case *ast.MultiplicityDecl:
		return d.Ident, true
	case *ast.Dependency:
		return d.Ident, true
	case *ast.Comment:
		return d.Ident, true
	case *ast.Documentation:
		return d.Ident, true
	case *ast.TextualRepresentation:
		return d.Ident, true
	default:
		return ast.Identification{}, false
	}
}

// referringTo returns where this document refers to sym, by the FQN of each
// referring namespace. The declaration's own name occurrence is not a reference:
// a shorthand redefinition is collected at the very span being rewritten.
func (m Model) referringTo(sym *symbols.Symbol, nameSpan source.Span) []string {
	rootScope := m.Index.DocumentRoot(m.Source.Name())
	if rootScope == nil {
		return nil
	}
	r := resolve.New(m.Index)
	seen := map[string]bool{}
	var out []string
	for _, ref := range resolve.References(m.Root, rootScope) {
		if ref.QN == nil {
			continue
		}
		r.ResolveReference(ref)
		for i, part := range ref.QN.Parts {
			if part.Span == nameSpan {
				continue
			}
			seg, ok := r.PartSymbol(ref.QN, i)
			if !ok || !sameSymbol(seg, sym) {
				continue
			}
			site := referenceSite(r, ref, part.Text)
			if !seen[site] {
				seen[site] = true
				out = append(out, site)
			}
		}
	}
	sort.Strings(out)
	return out
}

// referenceSite names where a reference is made: the namespace containing it,
// or the name as written when that namespace has no FQN.
func referenceSite(r *resolve.Resolver, ref resolve.Reference, text string) string {
	if fqn := r.ReferringNamespaceFQN(ref.Scope); fqn != "" {
		return fqn
	}
	return text
}

// sameSymbol compares two symbols by identity, falling back to the declaration
// they were built from: resolution inside a document and through the index yield
// different pointers for one declaration.
func sameSymbol(a, b *symbols.Symbol) bool {
	switch {
	case a == nil || b == nil:
		return false
	case a == b:
		return true
	case a.DocName == "" || a.DocName != b.DocName:
		return false
	}
	return a.DeclSpan == b.DeclSpan
}
