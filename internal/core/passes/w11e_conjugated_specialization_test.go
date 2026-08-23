package passes

import "testing"

// A conjugated type inverts the directions of its features, so specializing it
// is meaningless (KerML validateSpecializationSpecificNotConjugated).
func TestConjugatedTypeIsNotSpecialized(t *testing.T) {
	const src = `package p {
		classifier A;
		classifier B;
		classifier C conjugates A;
		subtype C specializes B;
	}`
	if !hasMessage(diagsIn(t, "a.kerml", src, "type"), msgConjugatedSpecific) {
		t.Fatalf("expected %q", msgConjugatedSpecific)
	}
}

// The rule shares the type tier with the specialization metaclass rules, so a
// metaclass error elsewhere in the file does not suppress it.
func TestConjugatedRuleSurvivesAMetaclassError(t *testing.T) {
	const src = `package p {
		classifier A;
		classifier B;
		classifier C conjugates A;
		subtype C specializes B;
		datatype D1;
		class C1;
		datatype D2 specializes D1, C1;
	}`
	diags := diagsIn(t, "a.kerml", src, "type")
	if !hasMessage(diags, msgConjugatedSpecific) {
		t.Fatalf("expected %q alongside the metaclass error, got %v", msgConjugatedSpecific, diags)
	}
	if !hasMessage(diags, msgW11ASpecializeClassOrAssoc) {
		t.Fatalf("expected %q, got %v", msgW11ASpecializeClassOrAssoc, diags)
	}
}

func hasMessage(diags []Diagnostic, msg string) bool {
	for _, d := range diags {
		if d.Message == msg {
			return true
		}
	}
	return false
}
