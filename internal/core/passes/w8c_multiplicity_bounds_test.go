package passes

import "testing"

func TestW8CMultiplicityBoundNotNatural(t *testing.T) {
	cases := map[string]string{
		"boolean literal": "feature f [1..false];",
		"string literal":  "feature f [\"x\"..*];",
		"boolean valued":  "feature n = 0;\n\tfeature b = n > 3;\n\tfeature f [n..b];",
		"negative valued": "feature n = 0;\n\tfeature m = n - 1;\n\tfeature f [m..2];",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			msgs := w8cMessages(t, "package P {\n\t"+body+"\n}")
			if w8cCount(msgs, msgMultiplicityBoundNatural) != 1 {
				t.Errorf("want one %q, got %v", msgMultiplicityBoundNatural, msgs)
			}
		})
	}
}

func TestW8CMultiplicityBoundLegal(t *testing.T) {
	src := `package P {
	feature n = 0;
	feature m = n + 2;
	feature a [0..*];
	feature b [1];
	feature c [n..m];
	feature d [*];
}`
	if msgs := w8cMessages(t, src); len(msgs) != 0 {
		t.Errorf("expected no constraint diagnostics, got %v", msgs)
	}
}

func TestW8CMultiplicityBoundSpanIsTheBound(t *testing.T) {
	src := "package P {\n\tfeature f [1..false];\n}"
	var found bool
	for _, d := range constraintDiagsKerML(t, src) {
		if d.Message != msgMultiplicityBoundNatural {
			continue
		}
		found = true
		if got := src[d.Span.Offset:d.Span.End()]; got != "false" {
			t.Errorf("span covers %q, want %q", got, "false")
		}
	}
	if !found {
		t.Fatal("rule did not fire")
	}
}

// The corpus case semantic/k37-multiplicity-bound-not-natural.kerml and its
// relatives: a bound whose result is typed by a class, a data type or a
// non-integer scalar is rejected however the type was derived.
func TestW8CMultiplicityBoundResultTypeNotNatural(t *testing.T) {
	cases := map[string]string{
		"class typed":        "class C { feature n : C; feature f : C [n]; }",
		"class typed upper":  "class C { feature n : C; feature f : C [1..n]; }",
		"datatype typed":     "datatype D; class C { feature d : D; feature f [d]; }",
		"class typed lower":  "class C { feature n : C; feature f : C [n..*]; }",
		"real typed":         "class C { feature r : ScalarValues::Real; feature f [r]; }",
		"string typed":       "class C { feature s : ScalarValues::String; feature f [s]; }",
		"boolean typed":      "class C { feature b : ScalarValues::Boolean; feature f [b]; }",
		"real valued":        "class C { feature r = 3.5; feature f [r]; }",
		"class typed nested": "class C { feature n : C; feature f : C [n + 1]; }",
		"integer exponent":   "class C { feature i : ScalarValues::Integer; feature f [2 ** i]; }",
		"negated exponent":   "class C { feature n : ScalarValues::Natural; feature f [2 ** -n]; }",
		"real exponent":      "class C { feature r : ScalarValues::Real; feature f [2 ^ r]; }",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			msgs := w8cMessages(t, "package P {\n\t"+body+"\n}")
			if w8cCount(msgs, msgMultiplicityBoundNatural) != 1 {
				t.Errorf("want one %q, got %v", msgMultiplicityBoundNatural, msgs)
			}
		})
	}
}

// Integer-conforming results are accepted, and a bound whose type cannot be
// resolved or was never declared stays silent for the name-resolution tier.
func TestW8CMultiplicityBoundResultTypeSilent(t *testing.T) {
	cases := map[string]string{
		"natural typed":      "class C { feature n : ScalarValues::Natural; feature f [n]; }",
		"integer typed":      "class C { feature i : ScalarValues::Integer; feature f [0..i]; }",
		"positive typed":     "class C { feature p : ScalarValues::Positive; feature f [p..*]; }",
		"natural valued":     "class C { feature n : ScalarValues::Natural = 3; feature f [n]; }",
		"integer valued":     "class C { feature i = 3; feature f [i]; }",
		"integer arithmetic": "class C { feature n : ScalarValues::Natural; feature f [n + 1]; }",
		"natural exponent":   "class C { feature n : ScalarValues::Natural; feature f [2 ** n]; }",
		"positive exponent":  "class C { feature i : ScalarValues::Integer; feature p : ScalarValues::Positive; feature f [i ^ (p + 1)]; }",
		"literal exponent":   "class C { feature i : ScalarValues::Integer; feature f [i ** 2]; }",
		"untyped":            "class C { feature u; feature f [u]; }",
		"unresolved type":    "class C { feature q : Undeclared; feature f [q]; }",
		"unresolved bound":   "class C { feature f [nothere]; }",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			msgs := w8cMessages(t, "package P {\n\t"+body+"\n}")
			if n := w8cCount(msgs, msgMultiplicityBoundNatural); n != 0 {
				t.Errorf("want no %q, got %v", msgMultiplicityBoundNatural, msgs)
			}
		})
	}
}
