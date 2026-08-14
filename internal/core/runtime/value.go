package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
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
	ValExpr     // wraps unevaluated AST node for delayed evaluation (e.g., BodyExpr for select/collect)
	ValQuantity // a magnitude and the measurement unit it is expressed in
	ValVariant  // the variant selected for a variation, and the object it materializes
)

// String names the kind, so diagnostics quoting it read as more than an index.
func (k ValueKind) String() string {
	switch k {
	case ValConst:
		return "constant"
	case ValNull:
		return "null"
	case ValString:
		return "string"
	case ValInstance:
		return "instance"
	case ValSequence:
		return "sequence"
	case ValSet:
		return "set"
	case ValExpr:
		return "expression"
	case ValQuantity:
		return "quantity"
	case ValVariant:
		return "variant"
	default:
		return "invalid"
	}
}

// Value is a runtime-evaluable value.
type Value struct {
	Kind     ValueKind
	Const    semantics.Value // ValConst: reuse static evaluator
	Str      string          // ValString
	Instance int64           // ValInstance: instance ID
	Sequence *Sequence       // ValSequence
	Set      *Set            // ValSet
	Expr     ast.Node        // ValExpr: unevaluated AST for delayed evaluation
	Quantity *Quantity       // ValQuantity: magnitude and measurement unit
	// Variant is the variant a variation was bound to (ValVariant). Instance
	// holds the object materialized for it, 0 when it materializes none.
	Variant *symbols.Symbol
}

// Object returns the object a value denotes: an instance, or the object a
// selected variant materialized.
func (v Value) Object() (int64, bool) {
	switch v.Kind {
	case ValInstance:
		return v.Instance, true
	case ValVariant:
		return v.Instance, v.Instance != 0
	default:
		return 0, false
	}
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

// Set is a unique collection (map-backed, using valueKey for equality). A set
// has no inherent order, but enumerating one has to answer in some order, and
// insertion order is the one order a set does carry: it makes a sequence
// derived from a set — what `select` and `collect` over a set return —
// reproducible instead of dependent on map iteration.
type Set struct {
	elements map[valueKey]Value
	order    []valueKey
}

// NewSet creates an empty Set.
func NewSet() *Set {
	return &Set{elements: make(map[valueKey]Value)}
}

// Add inserts a value into the set (deduplicates by valueKey).
func (s *Set) Add(val Value) {
	key := valueKeyFunc(val)
	if _, exists := s.elements[key]; !exists {
		s.order = append(s.order, key)
	}
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

// Elements returns all elements, in the order they were added.
func (s *Set) Elements() []Value {
	result := make([]Value, 0, len(s.elements))
	for _, key := range s.order {
		result = append(result, s.elements[key])
	}
	return result
}
