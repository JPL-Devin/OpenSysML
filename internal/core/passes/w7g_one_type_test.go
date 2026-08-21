package passes

import "testing"

func TestW7GTwoTypesOnAOneTypeUsageIsAnError(t *testing.T) {
	const src = `package T {
		case def C1;
		case def C2;
		use case def U1;
		use case def U2;
		case cTwo : C1, C2;
		use case uTwo : U1, U2;
	}`
	diags := only(typeDiags(t, src), "one-type")
	if len(diags) != 2 {
		t.Fatalf("expected one diagnostic per over-typed usage, got %v", diags)
	}
	want := map[string]bool{
		"A case must be typed by one case definition.":         true,
		"A use case must be typed by one use case definition.": true,
	}
	for _, d := range diags {
		if !want[d.Message] {
			t.Fatalf("unexpected message %q", d.Message)
		}
		if d.Severity != SeverityError {
			t.Fatalf("the reference reports an error, got %v", d.Severity)
		}
	}
}

// Every usage kind the reference holds to a single definition, each with the
// message that kind's rule words.
func TestW7GEveryOneTypeUsageKindIsReported(t *testing.T) {
	cases := []struct{ keyword, def, want string }{
		{"calc", "calc def", "A calculation must be typed by one calculation definition."},
		{"constraint", "constraint def", "A constraint must be typed by one constraint definition."},
		{"requirement", "requirement def", "A requirement must be typed by one requirement definition."},
		{"case", "case def", "A case must be typed by one case definition."},
		{"analysis case", "analysis case def", "An analysis case must be typed by one analysis case definition."},
		{"verification case", "verification case def", "A verification case must be typed by one verification case definition."},
		{"use case", "use case def", "A use case must be typed by one use case definition."},
		{"enum", "enum def", "An enumeration must be typed by one enumeration definition."},
		{"rendering", "rendering def", "A rendering must be typed by one rendering definition."},
		{"viewpoint", "viewpoint def", "A viewpoint must be typed by one viewpoint definition."},
		{"view", "view def", "A view must be typed by one view definition."},
		{"metadata", "metadata def", "A metadata usage must be typed by one metadata definition."},
	}
	for _, c := range cases {
		t.Run(c.keyword, func(t *testing.T) {
			src := "package T {\n" +
				c.def + " D1;\n" + c.def + " D2;\n" +
				c.keyword + " u : D1, D2;\n}"
			diags := only(typeDiags(t, src), "one-type")
			if len(diags) != 1 || diags[0].Message != c.want {
				t.Fatalf("want %q, got %v", c.want, diags)
			}
		})
	}
}

func TestW7GOneOrNoDeclaredTypeIsSilent(t *testing.T) {
	const src = `package T {
		case def C1;
		case cNone;
		case cOne : C1;
	}`
	if diags := only(typeDiags(t, src), "one-type"); len(diags) != 0 {
		t.Fatalf("a usage with at most one declared type is silent, got %v", diags)
	}
}

func TestW7GTwoTypesOnAPartIsNotAOneTypeError(t *testing.T) {
	const src = `package T {
		part def P1;
		part def P2;
		part p : P1, P2;
	}`
	if diags := only(typeDiags(t, src), "one-type"); len(diags) != 0 {
		t.Fatalf("a part may be typed by several definitions, got %v", diags)
	}
}

func TestW7GAnAttributeTypedByAnEnumerationTakesNoOtherType(t *testing.T) {
	const src = `package T {
		enum def E { enum a; }
		attribute def A;
		attribute withEnum : E, A;
		attribute plain : A, A;
	}`
	diags := only(typeDiags(t, src), "one-type")
	if len(diags) != 1 || diags[0].Message != msgEnumerationAttributeTypes {
		t.Fatalf("expected only the enumeration attribute to be reported, got %v", diags)
	}
}
