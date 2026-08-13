package passes

import "testing"

// exposeDiags returns the expose-owning-namespace findings of src.
func exposeDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	var out []Diagnostic
	for _, d := range constraintDiags(t, src) {
		if d.Code == "expose-owning-namespace" {
			out = append(out, d)
		}
	}
	return out
}

// An expose is owned by a view usage (SysML v2 8.3.26.2); anywhere else is
// reported, and a view def body is the warning case Systemica still resolves.
// A package or namespace body rejects `expose` in the parser already.
func TestExposeOwningNamespace(t *testing.T) {
	cases := []struct {
		name     string
		src      string
		count    int
		severity Severity
	}{
		{"view usage", "package P { part p; view v { expose P::**; } }", 0, SeverityError},
		{"view def", "package P { part p; view def V { expose P::**; } }", 1, SeverityWarning},
		{"part def", "package P { part p; part def D { expose P::**; } }", 1, SeverityError},
		{"part usage", "package P { part p; part q { expose P::**; } }", 1, SeverityError},
		{"nested in a view usage", "package P { part p; view v { view w { expose P::**; } } }", 0, SeverityError},
		{"plain import is untouched", "package P { part p; part def D { import P::*; } }", 0, SeverityError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diags := exposeDiags(t, tc.src)
			if len(diags) != tc.count {
				t.Fatalf("got %d expose-owning-namespace diagnostics, want %d: %v",
					len(diags), tc.count, diags)
			}
			if tc.count > 0 {
				if diags[0].Severity != tc.severity {
					t.Errorf("severity = %v, want %v", diags[0].Severity, tc.severity)
				}
				if diags[0].Span.Len == 0 {
					t.Errorf("diagnostic has no span: %v", diags[0])
				}
			}
		})
	}
}
