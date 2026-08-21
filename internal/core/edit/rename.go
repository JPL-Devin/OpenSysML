package edit

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// renameOccurrence is one name occurrence a rename rewrites: the segment's span,
// the reference it belongs to, and where in this source it was written.
type renameOccurrence struct {
	span  source.Span
	scope *symbols.Scope
	qn    *ast.QualifiedName
	part  int
	site  string
}

// renameSplices are the byte ranges a rename rewrites: the declaration's name
// token and every reference to it in this source. A rename that would capture or
// shadow another name is refused.
func (m Model) renameSplices(i int, op Operation, sym *symbols.Symbol) ([]splice, error) {
	ident, ok := declIdent(sym.Decl)
	if !ok || ident.Name == "" || ident.NameSpan.Len == 0 {
		return nil, &Error{
			Failure:        FailureNotNamed,
			OperationIndex: i,
			Message: fmt.Sprintf("%s declares no name of its own to rename"+
				" (a shorthand redefinition names the feature it redefines)", op.Target),
		}
	}
	if err := checkName(i, op.NewName); err != nil {
		return nil, err
	}
	if taken, ok := m.nameTaken(sym, op.NewName); ok {
		return nil, &Error{
			Failure:        FailureInvalidName,
			OperationIndex: i,
			Message: fmt.Sprintf("%s cannot be renamed to %q: that name already means %s where"+
				" %s is declared, so the rename would make it ambiguous or silently rebind"+
				" what reads that name",
				op.Target, op.NewName, taken, op.Target),
		}
	}
	occurrences, err := m.renameOccurrences(i, op, sym, ident)
	if err != nil {
		return nil, err
	}
	out := make([]splice, 0, len(occurrences)+1)
	out = append(out, splice{span: ident.NameSpan, text: op.NewName, opIndex: i, target: op.Target})
	for _, occ := range occurrences {
		out = append(out, splice{span: occ.span, text: op.NewName, opIndex: i, target: op.Target})
	}
	return out, nil
}

// renameOccurrences returns every reference spelling sym's declared name, in
// source order. A reference written with the short name is left alone: the
// rename does not change the short name.
func (m Model) renameOccurrences(i int, op Operation, sym *symbols.Symbol,
	ident ast.Identification) ([]renameOccurrence, error) {

	rootScope := m.Index.DocumentRoot(m.Source.Name())
	if rootScope == nil {
		return nil, nil
	}
	r := m.resolver()
	seen := map[int]bool{}
	var out []renameOccurrence
	for _, ref := range resolve.References(m.Root, rootScope) {
		if ref.QN == nil {
			continue
		}
		r.ResolveReference(ref)
		for part, segment := range ref.QN.Parts {
			if segment.Span == ident.NameSpan || segment.Text != ident.Name ||
				seen[segment.Span.Offset] {
				continue
			}
			seg, ok := r.PartSymbol(ref.QN, part)
			if !ok || !symbols.SameElement(seg, sym) {
				continue
			}
			seen[segment.Span.Offset] = true
			out = append(out, renameOccurrence{
				span:  segment.Span,
				scope: ref.Scope,
				qn:    ref.QN,
				part:  part,
				site:  referenceSite(r, ref, segment.Text),
			})
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].span.Offset < out[b].span.Offset })
	for _, occ := range out {
		if captured, ok := m.capturedAt(r, sym, occ, op.NewName); ok {
			return nil, &Error{
				Failure:        FailureInvalidName,
				OperationIndex: i,
				Referring:      []string{occ.site},
				Message: fmt.Sprintf("%s cannot be renamed to %q: the reference to it in %s"+
					" would read %s instead, so the rename would change what that reference means",
					op.Target, op.NewName, occ.site, captured),
			}
		}
	}
	return out, nil
}

// capturedAt names what a rewritten reference would read instead of sym: an
// unqualified name is checked in its own scope, a qualified segment in the
// namespace the segment before it named.
func (m Model) capturedAt(r *resolve.Resolver, sym *symbols.Symbol,
	occ renameOccurrence, newName string) (string, bool) {

	if occ.part > 0 {
		parent, ok := r.PartSymbol(occ.qn, occ.part-1)
		if !ok {
			return "", false
		}
		prefix := m.Index.GetFQN(parent)
		if prefix == "" {
			return "", false
		}
		candidate := prefix + "::" + newName
		for _, other := range m.Index.LookupQualifiedFrom(candidate, prefix) {
			if other != nil && !symbols.SameElement(other, sym) {
				return candidate, true
			}
		}
		return "", false
	}
	if occ.scope == nil {
		return "", false
	}
	other, ok := r.LookupNameExcluding(occ.scope, newName, sym.Decl)
	if !ok || symbols.SameElement(other, sym) {
		return "", false
	}
	if fqn := m.Index.GetFQN(other); fqn != "" {
		return fqn, true
	}
	return other.Name, true
}
