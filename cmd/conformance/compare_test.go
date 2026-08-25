package main

import (
	"strings"
	"testing"
)

// response is a normalized response written as a scenario would read it.
func response() map[string]any {
	return map[string]any{
		"content": "part def Vehicle {\n\tattribute mass = 1500.0;\n}",
		"instance": map[string]any{
			"id":             "@1",
			"type_symbol_id": "Demo::Vehicle",
			"feature_values": map[string]any{
				"mass": map[string]any{"value": map[string]any{"real_value": 1500.0}},
			},
		},
		"elements": []any{
			map[string]any{"id": "Demo::vehicle", "type": "PartUsage"},
			map[string]any{"id": "Demo::spare", "type": "PartUsage"},
		},
		"capabilities": []any{"query", "convert"},
	}
}

// assertPasses fails the test unless the expectation holds.
func assertPasses(t *testing.T, expect *Expect) {
	t.Helper()
	if failures := check(expect, response()); len(failures) > 0 {
		t.Fatalf("expectation failed: %s", strings.Join(failures, "; "))
	}
}

// assertFails fails the test unless the expectation is reported as a mismatch.
func assertFails(t *testing.T, expect *Expect, wantMention string) {
	t.Helper()
	failures := check(expect, response())
	if len(failures) == 0 {
		t.Fatal("expectation held, want a mismatch")
	}
	if wantMention != "" && !strings.Contains(strings.Join(failures, "; "), wantMention) {
		t.Fatalf("failures = %v, want one mentioning %q", failures, wantMention)
	}
}

// TestResponseIsComparedOnlyWhereNamed verifies a partial expectation matches,
// so a field added to the schema later does not fail a scenario.
func TestResponseIsComparedOnlyWhereNamed(t *testing.T) {
	assertPasses(t, &Expect{Response: map[string]any{
		"instance": map[string]any{"type_symbol_id": "Demo::Vehicle"},
	}})
}

// TestANestedMismatchIsReportedByPath verifies a mismatch names where it is.
func TestANestedMismatchIsReportedByPath(t *testing.T) {
	assertFails(t, &Expect{Response: map[string]any{
		"instance": map[string]any{"feature_values": map[string]any{
			"mass": map[string]any{"value": map[string]any{"real_value": 1200.0}},
		}},
	}}, "instance.feature_values.mass.value.real_value")
}

// TestRealsCompareWithinTolerance verifies a difference far below any a model
// would state is the same value, and a real difference is not.
func TestRealsCompareWithinTolerance(t *testing.T) {
	within := map[string]any{"instance": map[string]any{"feature_values": map[string]any{
		"mass": map[string]any{"value": map[string]any{"real_value": 1500.0 + 1e-9}},
	}}}
	assertPasses(t, &Expect{Response: within})
	beyond := map[string]any{"instance": map[string]any{"feature_values": map[string]any{
		"mass": map[string]any{"value": map[string]any{"real_value": 1500.1}},
	}}}
	assertFails(t, &Expect{Response: beyond}, "1500.1")
}

// TestADefaultExpectationMatchesAnUnsetField verifies an unset field and one
// left at its default are the same thing, as they are on the wire.
func TestADefaultExpectationMatchesAnUnsetField(t *testing.T) {
	assertPasses(t, &Expect{Response: map[string]any{"error": "", "experimental": false}})
	assertFails(t, &Expect{Response: map[string]any{"error": "boom"}}, "error")
}

// TestAbsentAndNonEmptyReadNestedPaths verifies both address a field by path.
func TestAbsentAndNonEmptyReadNestedPaths(t *testing.T) {
	assertPasses(t, &Expect{
		NonEmpty: []string{"instance.type_symbol_id"},
		Absent:   []string{"error", "instance.feature_values.mass.error"},
	})
	assertFails(t, &Expect{NonEmpty: []string{"error"}}, "error")
	assertFails(t, &Expect{Absent: []string{"instance.id"}}, "instance.id")
}

// TestContainsAllReadsTextAsSubstrings verifies a text field is checked for
// every wanted substring.
func TestContainsAllReadsTextAsSubstrings(t *testing.T) {
	assertPasses(t, &Expect{ContainsAll: map[string][]string{
		"content": {"part def Vehicle", "attribute mass"},
	}})
	assertFails(t, &Expect{ContainsAll: map[string][]string{"content": {"part def Wheel"}}}, "part def Wheel")
}

// TestContainsAllReadsAListAndAWildcardPath verifies membership in a list, and
// in one field taken from every entry of a list.
func TestContainsAllReadsAListAndAWildcardPath(t *testing.T) {
	assertPasses(t, &Expect{ContainsAll: map[string][]string{
		"capabilities":  {"query", "convert"},
		"elements.*.id": {"Demo::vehicle", "Demo::spare"},
	}})
	assertFails(t, &Expect{ContainsAll: map[string][]string{"elements.*.id": {"Demo::missing"}}}, "Demo::missing")
}

// TestCountsAndMinCounts verifies exact and lower-bound sizes of lists and maps,
// and that nothing at a path is zero entries rather than a missing path.
func TestCountsAndMinCounts(t *testing.T) {
	assertPasses(t, &Expect{
		Counts:    map[string]int{"elements": 2, "instance.feature_values": 1, "diagnostics": 0},
		MinCounts: map[string]int{"elements": 1},
	})
	assertFails(t, &Expect{Counts: map[string]int{"elements": 3}}, "want 3")
	assertFails(t, &Expect{MinCounts: map[string]int{"elements": 5}}, "at least 5")
}

// TestAListLengthIsPartOfAnExactExpectation verifies a named list must have the
// entries named, so an extra element is a mismatch.
func TestAListLengthIsPartOfAnExactExpectation(t *testing.T) {
	assertFails(t, &Expect{Response: map[string]any{
		"elements": []any{map[string]any{"id": "Demo::vehicle"}},
	}}, "entries")
}
