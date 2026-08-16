package runtime

import (
	"errors"
	"testing"
)

// A `for` visits a sequence in the order the expression that built it produced,
// a set in the order its canonical rendering sorts in, and a single value as the
// one element it is.
func TestForElementsOrder(t *testing.T) {
	set := NewSet()
	for _, n := range []int64{30, 4, 100, 4} {
		set.Add(integerValue(n))
	}

	cases := map[string]struct {
		value Value
		want  []int64
	}{
		"a sequence keeps its own order":    {sequenceOf([]Value{integerValue(3), integerValue(1), integerValue(2)}), []int64{3, 1, 2}},
		"an empty sequence visits nothing":  {sequenceOf(nil), nil},
		"a set sorts by its rendering":      {Value{Kind: ValSet, Set: set}, []int64{100, 30, 4}},
		"an empty set visits nothing":       {Value{Kind: ValSet, Set: NewSet()}, nil},
		"a single value is its one element": {integerValue(7), []int64{7}},
		"null visits nothing":               {nullValue(), nil},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			elements, err := forElements(tc.value)
			if err != nil {
				t.Fatalf("forElements: %v", err)
			}
			got := intsOf(t, sequenceOf(elements))
			if !equalInts(got, tc.want) {
				t.Errorf("visited %v, want %v", got, tc.want)
			}
		})
	}
}

// A value stating a computation rather than a collection is no collection in any
// order, so a `for` over it is reported with the kind it has.
func TestForElementsRejectsANonCollection(t *testing.T) {
	if _, err := forElements(Value{Kind: ValExpr}); !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("forElements over an expression failed with %v, want ErrTypeMismatch", err)
	}
}
