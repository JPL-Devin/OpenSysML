// Package edit rewrites the source a model was parsed from. An operation names
// an element the way symbols name it, the spans of the parse say which bytes
// carry that element's value or name, and only those bytes are replaced — so
// comments, blank lines and indentation come back byte-identical. The edited
// notation is re-parsed and re-analyzed before it is returned: an edit that
// would make the model unreadable is refused rather than written.
package edit

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/passes"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// OpKind is which of the two source-preserving changes an Operation makes.
type OpKind int

const (
	// OpSetValue sets the value of a feature that already exists.
	OpSetValue OpKind = iota
	// OpRename rewrites the name token of a declaration.
	OpRename
)

// Operation is one change to make to a model's source.
type Operation struct {
	Kind OpKind
	// Target is the element to edit, by FQN, as symbols name it.
	Target string
	// Value is the new value in SysML notation, for OpSetValue.
	Value string
	// NewName is the new declared name, for OpRename.
	NewName string
}

// SetValue is an operation setting target's value to the expression value.
func SetValue(target, value string) Operation {
	return Operation{Kind: OpSetValue, Target: target, Value: value}
}

// Rename is an operation rewriting target's declared name to newName.
func Rename(target, newName string) Operation {
	return Operation{Kind: OpRename, Target: target, NewName: newName}
}

// Model is a parsed model to edit: the source that was read, its parse, and the
// index it was analyzed in.
type Model struct {
	Source *source.SourceFile
	Root   *ast.RootNamespace
	Index  *symbols.Index
	// ParseDiags and SemDiags are what the original was found to have, so a
	// refusal reports the errors an edit introduced and not ones it inherited.
	ParseDiags []parser.Diagnostic
	SemDiags   []passes.Diagnostic
	// NewIndex hands out an index carrying the libraries the model was analyzed
	// against and no document of its own, for analyzing the edited notation.
	// Nil checks syntax alone.
	NewIndex func() *symbols.Index
}

// Applied is one byte range of the original source that an operation replaced.
type Applied struct {
	OperationIndex int
	Target         string
	// Span is the range of the original source replaced; Len is 0 for an
	// insertion.
	Span    source.Span
	OldText string
	NewText string
}

// Result is the edited notation and what each operation changed.
type Result struct {
	Content []byte
	Applied []Applied
}

// Apply applies every operation to m's source, or none of them, and returns the
// edited notation. Every refusal is an *Error naming its kind.
func Apply(m Model, ops []Operation) (*Result, error) {
	if m.Source == nil || m.Root == nil || m.Index == nil {
		return nil, &Error{Failure: FailureResultInvalid, Message: "no parsed model to edit"}
	}
	if len(ops) == 0 {
		return nil, &Error{Failure: FailureNoOperations, Message: "no edit operations requested"}
	}

	splices := make([]splice, 0, len(ops))
	for i, op := range ops {
		sp, err := m.spliceFor(i, op)
		if err != nil {
			return nil, err
		}
		splices = append(splices, sp)
	}
	if err := checkOverlap(splices); err != nil {
		return nil, err
	}

	content := m.splice(splices)
	if err := m.validate(content); err != nil {
		return nil, err
	}

	applied := make([]Applied, len(splices))
	for i, sp := range splices {
		applied[i] = Applied{
			OperationIndex: sp.opIndex,
			Target:         sp.target,
			Span:           sp.span,
			OldText:        m.Source.Text(sp.span),
			NewText:        sp.text,
		}
	}
	return &Result{Content: content, Applied: applied}, nil
}

// splice is one byte range of the original source to replace with text.
type splice struct {
	span    source.Span
	text    string
	opIndex int
	target  string
}

// spliceFor turns one operation into the byte range it rewrites.
func (m Model) spliceFor(i int, op Operation) (splice, error) {
	sym, err := m.target(i, op)
	if err != nil {
		return splice{}, err
	}
	switch op.Kind {
	case OpSetValue:
		return m.valueSplice(i, op, sym)
	case OpRename:
		return m.renameSplice(i, op, sym)
	default:
		return splice{}, &Error{
			Failure:        FailureResultInvalid,
			OperationIndex: i,
			Message:        fmt.Sprintf("unknown edit operation kind %d", op.Kind),
		}
	}
}

// splice rewrites the source, applying the ranges right-to-left so that each
// offset still names the bytes the parse of the original found there.
func (m Model) splice(splices []splice) []byte {
	ordered := make([]splice, len(splices))
	copy(ordered, splices)
	sort.SliceStable(ordered, func(a, b int) bool {
		return ordered[a].span.Offset > ordered[b].span.Offset
	})

	src := m.Source.Bytes()
	out := make([]byte, len(src))
	copy(out, src)
	for _, sp := range ordered {
		edited := make([]byte, 0, len(out)-sp.span.Len+len(sp.text))
		edited = append(edited, out[:sp.span.Offset]...)
		edited = append(edited, sp.text...)
		edited = append(edited, out[sp.span.End():]...)
		out = edited
	}
	return out
}

// checkOverlap refuses edits covering the same bytes, which no order of
// application can satisfy. Two insertions at one offset overlap as well: their
// result would depend on the order they happened to be applied in.
func checkOverlap(splices []splice) error {
	ordered := make([]splice, len(splices))
	copy(ordered, splices)
	sort.SliceStable(ordered, func(a, b int) bool {
		return ordered[a].span.Offset < ordered[b].span.Offset
	})
	for i := 1; i < len(ordered); i++ {
		prev, cur := ordered[i-1], ordered[i]
		if cur.span.Offset < prev.span.End() || (cur.span.Offset == prev.span.Offset && prev.span.Len == 0) {
			return &Error{
				Failure:        FailureOverlappingEdits,
				OperationIndex: cur.opIndex,
				Message: fmt.Sprintf("edits to %s and %s cover the same source bytes",
					prev.target, cur.target),
			}
		}
	}
	return nil
}
