package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

// ValueKind distinguishes runtime value types.
type ValueKind int

const (
	ValInvalid ValueKind = iota
	ValConst             // wraps semantics.Value (int/real/bool/infinity)
	ValNull
	ValString
	ValInstance
	ValSequence
	ValSet
	ValExpr // wraps unevaluated AST node for delayed evaluation (e.g., BodyExpr for select/collect)
)

// Value is a runtime-evaluable value.
type Value struct {
	Kind     ValueKind
	Const    semantics.Value // ValConst: reuse static evaluator
	Str      string          // ValString
	Instance int64           // ValInstance: instance ID
	Sequence *Sequence       // ValSequence
	Set      *Set            // ValSet
	Expr     ast.Node        // ValExpr: unevaluated AST for delayed evaluation
}

// Sequence is an ordered collection (slice-backed).
type Sequence struct {
	elements []Value
}

// NewSequence creates an empty Sequence.
func NewSequence() *Sequence {
	return &Sequence{elements: make([]Value, 0)}
}

// Append adds a value to the end of the sequence.
func (s *Sequence) Append(val Value) {
	s.elements = append(s.elements, val)
}

// At returns the element at the given index (0-based).
func (s *Sequence) At(index int) (Value, error) {
	if index < 0 || index >= len(s.elements) {
		return Value{}, fmt.Errorf("index %d out of range [0, %d)", index, len(s.elements))
	}
	return s.elements[index], nil
}

// Size returns the number of elements.
func (s *Sequence) Size() int {
	return len(s.elements)
}

// Elements returns the underlying slice (for iteration).
func (s *Sequence) Elements() []Value {
	return s.elements
}

// Set is an unordered unique collection (map-backed, using valueKey for equality).
type Set struct {
	elements map[valueKey]Value
}

// NewSet creates an empty Set.
func NewSet() *Set {
	return &Set{elements: make(map[valueKey]Value)}
}

// Add inserts a value into the set (deduplicates by valueKey).
func (s *Set) Add(val Value) {
	key := valueKeyFunc(val)
	s.elements[key] = val
}

// Contains checks if the value is in the set.
func (s *Set) Contains(val Value) bool {
	_, ok := s.elements[valueKeyFunc(val)]
	return ok
}

// Size returns the number of unique elements.
func (s *Set) Size() int {
	return len(s.elements)
}

// Elements returns all elements in arbitrary order.
func (s *Set) Elements() []Value {
	result := make([]Value, 0, len(s.elements))
	for _, v := range s.elements {
		result = append(result, v)
	}
	return result
}
