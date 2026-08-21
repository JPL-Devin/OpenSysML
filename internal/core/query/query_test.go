package query

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

type testModel struct {
	symbols []*symbols.Symbol
	values  map[*symbols.Symbol]map[string][]string
}

func (m *testModel) Candidates([]string) ([]*symbols.Symbol, error) { return m.symbols, nil }
func (m *testModel) Value(sym *symbols.Symbol, property string) ([]string, bool) {
	values, ok := m.values[sym][property]
	return values, ok
}
func (m *testModel) Identity(sym *symbols.Symbol) string { return m.values[sym][PropertyID][0] }
func (m *testModel) Type(sym *symbols.Symbol) string {
	if values := m.values[sym][PropertyType]; len(values) != 0 {
		return values[0]
	}
	return ""
}

func TestEvaluateRichPredicatesAndOrdering(t *testing.T) {
	first, second := &symbols.Symbol{}, &symbols.Symbol{}
	model := &testModel{
		symbols: []*symbols.Symbol{first, second},
		values: map[*symbols.Symbol]map[string][]string{
			first: {
				PropertyID:                {"A"},
				PropertyType:              {"PartUsage"},
				PropertyName:              {"a"},
				PropertyMultiplicityLower: {"1"},
			},
			second: {
				PropertyID:                {"B"},
				PropertyType:              {"PartUsage"},
				PropertyName:              {"b"},
				PropertyMultiplicityLower: {"4"},
			},
		},
	}
	results, err := Evaluate(model, Query{
		Where: []Predicate{{
			Property: PropertyMultiplicityLower,
			Operator: GreaterEqual,
			Values:   []string{"1"},
		}},
		Select:  []string{PropertyName},
		OrderBy: []OrderTerm{{Property: PropertyName, Desc: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{results[0].ID, results[1].ID}; !slices.Equal(got, []string{"B", "A"}) {
		t.Fatalf("IDs = %v", got)
	}
}

func TestEvaluateOrdersByUnselectedProperty(t *testing.T) {
	first, second := &symbols.Symbol{}, &symbols.Symbol{}
	model := &testModel{
		symbols: []*symbols.Symbol{first, second},
		values: map[*symbols.Symbol]map[string][]string{
			first:  {PropertyID: {"A"}, PropertyName: {"a"}, PropertyMultiplicityLower: {"9"}},
			second: {PropertyID: {"B"}, PropertyName: {"b"}, PropertyMultiplicityLower: {"2"}},
		},
	}
	results, err := Evaluate(model, Query{
		Select:  []string{PropertyName},
		OrderBy: []OrderTerm{{Property: PropertyMultiplicityLower}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{results[0].ID, results[1].ID}; !slices.Equal(got, []string{"B", "A"}) {
		t.Fatalf("IDs = %v", got)
	}
	if _, ok := results[0].Properties[PropertyMultiplicityLower]; ok {
		t.Fatalf("unselected order property leaked into result: %#v", results[0].Properties)
	}
}

func TestEvaluateNotEqualAndIn(t *testing.T) {
	one, two := &symbols.Symbol{}, &symbols.Symbol{}
	model := &testModel{
		symbols: []*symbols.Symbol{one, two},
		values: map[*symbols.Symbol]map[string][]string{
			one: {PropertyID: {"one"}, PropertyName: {"alpha"}},
			two: {PropertyID: {"two"}, PropertyName: {"beta"}},
		},
	}
	for _, query := range []Query{
		{Where: []Predicate{{Property: PropertyName, Operator: In, Values: []string{"beta"}}}},
		{Where: []Predicate{{Property: PropertyName, Operator: NotEqual, Values: []string{"alpha"}}}},
	} {
		results, err := Evaluate(model, query)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 || results[0].ID != "two" {
			t.Fatalf("results = %#v", results)
		}
	}
}

func TestEvaluateRejectsUnorderedAndMultiOperandOrderedPredicates(t *testing.T) {
	tests := []Query{
		{Where: []Predicate{{Property: PropertyName, Operator: Greater, Values: []string{"1"}}}},
		{Where: []Predicate{{Property: PropertyMultiplicityLower, Operator: Greater, Values: []string{"1", "2"}}}},
	}
	for _, query := range tests {
		if err := func() error {
			_, err := Evaluate(&testModel{}, query)
			return err
		}(); err == nil {
			t.Errorf("Evaluate(%#v) succeeded", query)
		}
	}
}
