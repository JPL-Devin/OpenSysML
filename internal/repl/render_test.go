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

// A definition or usage is the common thing to type at the prompt, so it gets
// the same kind + name confirmation as a package: silence reads as no-op.
func TestDefinitionsAndUsagesAreSummarized(t *testing.T) {
	root := parseRoot("part def Wheel { } attribute wheelCount = 4; calc def area { } action step;")
	got := []string{}
	for _, m := range root.Members {
		got = append(got, renderMember(m))
	}
	want := []string{"part def Wheel", "attribute wheelCount", "calc def area", "action step"}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("member %d = %q, want %q", i, got[i], w)
		}
	}
}

func TestRenderDiagnostics(t *testing.T) {
	s := NewSession()
	r := s.Submit("namespace N { import Missing::X; }")
	if len(r.Diagnostics) == 0 {
		t.Fatal("importing a missing namespace produced no diagnostic to render")
	}
	out := renderDiagnostics(r.Diagnostics, r.Source, r.diagLocation, false)
	joined := strings.Join(out, "\n")
	// Expect: a "line:col: severity: message" header and a caret line.
	if !strings.Contains(joined, "error:") && !strings.Contains(joined, "warning:") {
		t.Errorf("missing severity header: %q", joined)
	}
	if !strings.Contains(joined, "^") {
		t.Errorf("missing caret line: %q", joined)
	}
}

// An import confirmation echoes the wildcard too: "import A::B" and
// "import A::B::*" bring in different things.
func TestImportSummaryKeepsWildcards(t *testing.T) {
	root := parseRoot("import A::B; import A::B::*; import A::B::**; import A::B::*::**;")
	want := []string{"import A::B", "import A::B::*", "import A::B::**", "import A::B::*::**"}
	for i, w := range want {
		if got := renderMember(root.Members[i]); got != w {
			t.Errorf("member %d = %q, want %q", i, got, w)
		}
	}
}

// D2 changed the severity of a bare import, not what the model means, so a
// submission whose only error is notation still confirms what it declared.
func TestNotationErrorKeepsTheDeclaredSummary(t *testing.T) {
	s := NewSession()
	s.Submit("package Q { part def A; }")
	r := s.Submit("package P { import Q::*; part def X; }")
	found, declared := renderSplit(r, s.verbosity)
	if !strings.Contains(strings.Join(found, "\n"), "error:") {
		t.Fatalf("the bare import produced no error to report: %q", found)
	}
	if !strings.Contains(strings.Join(declared, "\n"), "package P") {
		t.Errorf("the submission declared package P but summarized nothing: %q", declared)
	}
}
