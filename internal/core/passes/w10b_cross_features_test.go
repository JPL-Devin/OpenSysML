package passes

import "testing"

const w10bCrossSrc = `package AssociationTest {
	class C1 { feature a : C2; }
	class C2 { feature b : C1; feature c subsets b; }
	assoc A1 {
		end x : C1 crosses y.b;
		end y : C2 crosses y.b;
	}
	assoc A2 specializes A1 {
		end x : C1 crosses y.c;
		end y : C2 crosses x.a;
	}
}`

// An end crossing a feature of its own type chain fails both crossing
// constraints, as in validation/AssociationTest_CrossFeatures_invalid.
func TestW10BCrossFeatureTypeAndChain(t *testing.T) {
	diags := constraintDiagsKerML(t, w10bCrossSrc)
	for _, tc := range []struct{ code, msg string }{
		{"cross-feature-type", msgCrossFeatureType},
		{"cross-subsetting-chain", msgCrossSubsettingChain},
	} {
		got := only(diags, tc.code)
		if len(got) != 1 {
			t.Fatalf("%s: got %d diagnostics, want 1: %v", tc.code, len(got), got)
		}
		if got[0].Message != tc.msg {
			t.Errorf("%s: message = %q, want %q", tc.code, got[0].Message, tc.msg)
		}
		if got[0].Severity != SeverityError {
			t.Errorf("%s: severity = %v, want an error", tc.code, got[0].Severity)
		}
		if text := spanText(w10bCrossSrc, got[0]); text != "y.b" {
			t.Errorf("%s: span text = %q, want %q", tc.code, text, "y.b")
		}
	}
}

// A specialized association's end must cross a feature specializing the cross
// feature of the end it redefines.
func TestW10BCrossFeatureSpecialization(t *testing.T) {
	diags := only(constraintDiagsKerML(t, w10bCrossSrc), "cross-feature-specialization")
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %v", len(diags), diags)
	}
	if diags[0].Message != msgCrossSpecialization {
		t.Errorf("message = %q, want %q", diags[0].Message, msgCrossSpecialization)
	}
	if text := spanText(w10bCrossSrc, diags[0]); text != "x.a" {
		t.Errorf("span text = %q, want %q", text, "x.a")
	}
}

// Crossings that name a feature of the opposite end's type with the end's own
// type, and refine the inherited cross feature, are well-formed.
func TestW10BCrossFeaturesClean(t *testing.T) {
	const src = `package AssociationTest {
		class C1 { feature a : C2; }
		class C2 { feature b : C1; feature c subsets b; }
		assoc A1 {
			end x : C1 crosses y.b;
			end y : C2 crosses x.a;
		}
		assoc A2 specializes A1 {
			end x : C1 crosses y.c;
			end y : C2 crosses x.a;
		}
	}`
	for _, code := range []string{"cross-feature-type", "cross-subsetting-chain", "cross-feature-specialization"} {
		if diags := only(constraintDiagsKerML(t, src), code); len(diags) != 0 {
			t.Fatalf("%s fired on a well-formed association: %v", code, diags)
		}
	}
}

// Malformed crossings must not panic and must not be adjudicated.
func TestW10BCrossFeaturesMalformedInput(t *testing.T) {
	const src = `package P {
		assoc A1 {
			end x crosses ;
			end y : crosses y.;
			end crosses ..b;
		}
	}`
	for _, code := range []string{"cross-feature-type", "cross-feature-specialization"} {
		if diags := only(constraintDiagsKerML(t, src), code); len(diags) != 0 {
			t.Fatalf("%s fired on malformed input: %v", code, diags)
		}
	}
}
