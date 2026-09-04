package edit

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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

// resolver is a resolver with a semantic model attached, as every other caller
// builds one: without one, members reached through inheritance or a reference
// subsetting are invisible both to a lookup and to a reference's resolution.
func (m Model) resolver() *resolve.Resolver {
	r := resolve.New(m.Index)
	r.SetModel(semantics.NewModel(r))
	return r
}
