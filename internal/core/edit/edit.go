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

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// OpKind is which of the two source-preserving changes an Operation makes.
type OpKind int

const (
	// OpSetValue sets the value of a feature that already exists.
	OpSetValue OpKind = iota
	// OpRename rewrites the name token of a declaration.
	OpRename
	// OpAddMember inserts a declaration into a namespace.
	OpAddMember
	// OpDelete removes a declaration and its owned trivia.
	OpDelete
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
	// Owner is the namespace receiving an OpAddMember; empty means the root.
	Owner string
	// Declaration details for OpAddMember.
	MemberKind   string
	MemberName   string
	Type         string
	Multiplicity string
	Specializes  []string
	Cascade      bool
}

// SetValue is an operation setting target's value to the expression value.
func SetValue(target, value string) Operation {
	return Operation{Kind: OpSetValue, Target: target, Value: value}
}

// Rename is an operation rewriting target's declared name to newName.
func Rename(target, newName string) Operation {
	return Operation{Kind: OpRename, Target: target, NewName: newName}
}

// AddMember creates an operation inserting a declaration into owner.
func AddMember(owner, kind, name string) Operation {
	return Operation{Kind: OpAddMember, Owner: owner, MemberKind: kind, MemberName: name}
}

// Delete creates an operation removing target, optionally including referrers.
func Delete(target string, cascade bool) Operation {
	return Operation{Kind: OpDelete, Target: target, Cascade: cascade}
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

// Applied describes one replacement. Batch spans use the original source;
// sequential spans use the intermediate source seen by that operation.
type Applied struct {
	OperationIndex int
	Target         string
	// Span is the range replaced; Len is 0 for an insertion.
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
	if err := duplicateAddNames(ops); err != nil {
		return nil, err
	}
	if !needsSequential(ops) {
		return applyBatch(m, ops)
	}

	current := m
	content := append([]byte(nil), m.Source.Bytes()...)
	applied := make([]Applied, 0, len(ops))
	for i, op := range ops {
		var splices []splice
		if op.Kind == OpDelete {
			deletes, err := current.deleteSplices(i, op)
			if err != nil {
				return nil, err
			}
			splices = deletes
		} else {
			sp, err := current.spliceFor(i, op)
			if err != nil {
				return nil, err
			}
			splices = []splice{sp}
		}
		if err := checkOverlap(splices); err != nil {
			return nil, err
		}
		next := current.splice(splices)
		for _, sp := range splices {
			applied = append(applied, Applied{
				OperationIndex: sp.opIndex,
				Target:         sp.target,
				Span:           sp.span,
				OldText:        current.Source.Text(sp.span),
				NewText:        sp.text,
			})
		}
		content = next
		var err error
		current, err = reparseModel(m, content)
		if err != nil {
			return nil, err
		}
	}
	if err := m.validate(content); err != nil {
		return nil, err
	}
	return &Result{Content: content, Applied: applied}, nil
}

func duplicateAddNames(ops []Operation) error {
	seen := make(map[string]int)
	for i, op := range ops {
		if op.Kind != OpAddMember {
			continue
		}
		key := op.Owner + "\x00" + op.MemberName
		if previous, ok := seen[key]; ok {
			return &Error{
				Failure:        FailureMemberNameTaken,
				OperationIndex: i,
				Message: fmt.Sprintf("%q is declared more than once in owner %q (operations %d and %d)",
					op.MemberName, op.Owner, previous, i),
			}
		}
		seen[key] = i
	}
	return nil
}

func needsSequential(ops []Operation) bool {
	// This conservative trigger reparses when a later add may target an earlier add.
	addSeen := false
	for _, op := range ops {
		if op.Kind == OpAddMember {
			if addSeen && op.Owner != "" {
				return true
			}
			addSeen = true
		}
	}
	return false
}

func applyBatch(m Model, ops []Operation) (*Result, error) {
	splices := make([]splice, 0, len(ops))
	for i, op := range ops {
		if op.Kind == OpDelete {
			deletes, err := m.deleteSplices(i, op)
			if err != nil {
				return nil, err
			}
			splices = append(splices, deletes...)
			continue
		}
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

func reparseModel(base Model, content []byte) (Model, error) {
	sf := source.NewWithKind(base.Source.Name(), content, base.Source.Kind())
	p := parser.New(sf)
	root := p.ParseFile()
	var idx *symbols.Index
	if base.NewIndex != nil {
		idx = base.NewIndex()
		idx.AddDocument(sf.Name(), root)
	} else {
		idx = symbols.NewIndex()
		idx.AddDocument(sf.Name(), root)
	}
	var semDiags []passes.Diagnostic
	if len(p.Diagnostics) == 0 {
		semDiags = passes.AnalyzeWithKind(sf.Name(), sf.Kind(), root, nil, idx)
	}
	return Model{
		Source: sf, Root: root, Index: idx,
		ParseDiags: p.Diagnostics, SemDiags: semDiags,
		NewIndex: base.NewIndex,
	}, nil
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
	if op.Kind == OpAddMember {
		return m.addMemberSplice(i, op)
	}
	sym, err := m.target(i, op)
	if err != nil {
		return splice{}, err
	}
	switch op.Kind {
	case OpSetValue:
		return m.valueSplice(i, op, sym)
	case OpRename:
		return m.renameSplice(i, op, sym)
	case OpDelete:
		deletes, err := m.deleteSplices(i, op)
		if err != nil {
			return splice{}, err
		}
		if len(deletes) == 0 {
			return splice{}, &Error{Failure: FailureResultInvalid, OperationIndex: i,
				Message: "delete selected no declaration"}
		}
		return deletes[0], nil
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
		if ordered[a].span.Offset != ordered[b].span.Offset {
			return ordered[a].span.Offset > ordered[b].span.Offset
		}
		// At one insertion point, apply later requests first so the result
		// retains request order.
		return ordered[a].opIndex > ordered[b].opIndex
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

// checkOverlap refuses edits covering the same non-empty source bytes.
func checkOverlap(splices []splice) error {
	ordered := make([]splice, len(splices))
	copy(ordered, splices)
	sort.SliceStable(ordered, func(a, b int) bool {
		return ordered[a].span.Offset < ordered[b].span.Offset
	})
	for i := 1; i < len(ordered); i++ {
		prev, cur := ordered[i-1], ordered[i]
		if cur.span.Offset < prev.span.End() && (cur.span.Len > 0 || prev.span.Len > 0) {
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
