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
