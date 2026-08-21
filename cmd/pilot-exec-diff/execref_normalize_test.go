package main

import (
	"reflect"
	"testing"
)

func TestNormalizePilotSamples(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want normalized
	}{
		{"integer", "LiteralInteger 7 (fb98f88c-9172-45bc-8ed8-b78fe546719b)", normalized{Kind: kindInt, Value: "7"}},
		{"rational", "LiteralRational 2.5 (fb98f88c-9172-45bc-8ed8-b78fe546719b)", normalized{Kind: kindReal, Value: "2.5"}},
		{"exponent", "LiteralRational 1.099511627776E12 (fb98f88c-9172-45bc-8ed8-b78fe546719b)", normalized{Kind: kindReal, Value: "1.099511627776E12"}},
		{"sequence", "LiteralInteger 3 (fb98f88c-9172-45bc-8ed8-b78fe546719b)\nLiteralInteger 1 (fb98f88c-9172-45bc-8ed8-b78fe546719b)", normalized{Kind: kindSequence, Elements: []normalized{{Kind: kindInt, Value: "3"}, {Kind: kindInt, Value: "1"}}}},
		{"empty", "", normalized{}},
		{"unevaluated", "OperatorExpression + (fb98f88c-9172-45bc-8ed8-b78fe546719b)", normalized{Unevaluated: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := normalizePilot(test.raw)
			if got.Error || got.Value.Kind != test.want.Kind || got.Value.Value != test.want.Value ||
				got.Value.Unevaluated != test.want.Unevaluated || len(got.Value.Elements) != len(test.want.Elements) {
				t.Fatalf("normalizePilot(%q) = %+v, want %+v", test.raw, got, test.want)
			}
			for i := range test.want.Elements {
				if !reflect.DeepEqual(got.Value.Elements[i], test.want.Elements[i]) {
					t.Errorf("element %d = %+v, want %+v", i, got.Value.Elements[i], test.want.Elements[i])
				}
			}
		})
	}
}

func TestNormalizeOursSamples(t *testing.T) {
	tests := []struct {
		raw  string
		want normalized
	}{
		{"✓ 2 + 3\n  = 5", normalized{Kind: kindInt, Value: "5"}},
		{"✓ seq\n  = [3, 1, 2]", normalized{Kind: kindSequence, Elements: []normalized{{Kind: kindInt, Value: "3"}, {Kind: kindInt, Value: "1"}, {Kind: kindInt, Value: "2"}}}},
		{"✓ s\n  = \"abc\"", normalized{Kind: kindString, Value: "abc"}},
		{"✓ q\n  = 3.00 [SI::kg]", normalized{Kind: kindQuantity, Value: "3.00 [SI::kg]"}},
		{"", normalized{}},
	}
	for _, test := range tests {
		got := normalizeOurs(test.raw, false)
		if got.Error || got.Value.Kind != test.want.Kind || got.Value.Value != test.want.Value ||
			len(got.Value.Elements) != len(test.want.Elements) {
			t.Errorf("normalizeOurs(%q) = %+v, want %+v", test.raw, got, test.want)
		}
	}
}

func TestBucketResults(t *testing.T) {
	tests := []struct {
		name        string
		pilot, ours sideResult
		want        string
	}{
		{"agree", sideResult{Value: normalized{Kind: kindInt, Value: "7"}}, sideResult{Value: normalized{Kind: kindInt, Value: "7"}}, "agree"},
		{"kind", sideResult{Value: normalized{Kind: kindReal, Value: "1.0"}}, sideResult{Value: normalized{Kind: kindInt, Value: "1"}}, "kind-only"},
		{"order", sideResult{Value: normalized{Kind: kindSequence, Elements: []normalized{{Kind: kindInt, Value: "1"}, {Kind: kindInt, Value: "2"}}}}, sideResult{Value: normalized{Kind: kindSequence, Elements: []normalized{{Kind: kindInt, Value: "2"}, {Kind: kindInt, Value: "1"}}}}, "order-only"},
		{"pilot unevaluated", sideResult{Value: normalized{Unevaluated: true}}, sideResult{Value: normalized{Kind: kindQuantity, Value: "3.00 [kg]"}}, "pilot-unevaluated"},
		{"pilot error", sideResult{Error: true}, sideResult{Value: normalized{Kind: kindInt, Value: "1"}}, "pilot-error"},
		{"ours error", sideResult{Value: normalized{Kind: kindInt, Value: "1"}}, sideResult{Error: true}, "ours-error"},
		{"both error", sideResult{Error: true}, sideResult{Error: true}, "both-error"},
		{"empty", sideResult{}, sideResult{}, "agree"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := bucketResults(test.pilot, test.ours); got != test.want {
				t.Errorf("bucketResults() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRealRoundingAndExponent(t *testing.T) {
	pilot := normalized{Kind: kindReal, Value: "1.099511627776E12"}
	ours := normalized{Kind: kindInt, Value: "1099511627776"}
	if got := compareValues(pilot, ours); got != "kind-only" {
		t.Fatalf("compareValues() = %q, want kind-only", got)
	}
	if got := roundedReal(normalized{Kind: kindReal, Value: "0.3333333333333333"}); got != "0.33" {
		t.Fatalf("roundedReal() = %q, want 0.33", got)
	}
}

func TestCanonicalPilotStripsUUIDsFromEveryLine(t *testing.T) {
	raw := "LiteralInteger 1 (00000000-0000-0000-0000-000000000001)\n" +
		"LiteralInteger 2 (00000000-0000-0000-0000-000000000002)"
	want := "LiteralInteger 1\nLiteralInteger 2"
	if got := canonicalPilot(raw); got != want {
		t.Fatalf("canonicalPilot() = %q, want %q", got, want)
	}
}

func TestArtifactAbsentMessage(t *testing.T) {
	got := artifactAbsentMessage("/repo/build/pilot-evaluator/eval-sysml")
	want := "pilot execution artifact is absent at /repo/build/pilot-evaluator/eval-sysml; run ./scripts/download-pilot-evaluator.sh to provision it"
	if got != want {
		t.Fatalf("artifactAbsentMessage() = %q, want %q", got, want)
	}
}
