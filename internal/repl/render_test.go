package repl

import (
	"strings"
	"testing"
)

func TestRenderSummary(t *testing.T) {
	s := NewSession()
	s.Submit("package P { }")
	r := s.Submit("namespace N;")
	lines := renderSummary(r.Members)
	joined := strings.Join(lines, "\n")
	// Both accumulated top-level members are summarized with kind + name.
	if !strings.Contains(joined, "package P") {
		t.Errorf("missing package P summary: %q", joined)
	}
	if !strings.Contains(joined, "namespace N") {
		t.Errorf("missing namespace N summary: %q", joined)
	}
}

func TestKindLabel(t *testing.T) {
	root := parseRoot("package P {} namespace N; alias A for X; import Y::Z;")
	labels := []string{}
	for _, m := range root.Members {
		labels = append(labels, renderMember(m))
	}
	want := []string{"package P", "namespace N", "alias A", "import Y::Z"}
	for i, w := range want {
		if labels[i] != w {
			t.Errorf("member %d = %q, want %q", i, labels[i], w)
		}
	}
}

func TestRenderDiagnostics(t *testing.T) {
	s := NewSession()
	r := s.Submit("namespace N { import Missing::X; }")
	if len(r.Diagnostics) == 0 {
		t.Fatal("importing a missing namespace produced no diagnostic to render")
	}
	out := renderDiagnostics(r.Diagnostics, r.Source)
	joined := strings.Join(out, "\n")
	// Expect: a "line:col: severity: message" header and a caret line.
	if !strings.Contains(joined, "error:") && !strings.Contains(joined, "warning:") {
		t.Errorf("missing severity header: %q", joined)
	}
	if !strings.Contains(joined, "^") {
		t.Errorf("missing caret line: %q", joined)
	}
}
