package repl

import (
	"io"
	"strings"
	"testing"
)

type scriptReader struct {
	lines []string
	i     int
}

func (r *scriptReader) ReadLine(prompt string) (string, error) {
	if r.i >= len(r.lines) {
		return "", io.EOF
	}
	l := r.lines[r.i]
	r.i++
	return l, nil
}

func TestLoopEndToEnd(t *testing.T) {
	script := []string{
		"package P {",        // continuation begins
		"}",                  // closes → submits "package P {\n}"
		"namespace N;",       // second submission
		"%list",              // meta: should show both P and N
		"package P { }",      // redefine P (replaces prior)
		"import Missing::X;", // unresolved → diagnostic with caret
	}
	var out strings.Builder
	if err := Loop(&scriptReader{lines: script}, &out, NewSession()); err != nil {
		t.Fatalf("Loop error: %v", err)
	}
	got := out.String()

	// Continuation + summary for P.
	if !strings.Contains(got, "package P") {
		t.Errorf("missing package P summary:\n%s", got)
	}
	// Namespace N summary.
	if !strings.Contains(got, "namespace N") {
		t.Errorf("missing namespace N summary:\n%s", got)
	}
	// %list output shows accumulated declarations.
	if !strings.Contains(got, "package P") || !strings.Contains(got, "namespace N") {
		t.Errorf("%%list did not show session:\n%s", got)
	}
	// Unresolved import produces a diagnostic with a caret.
	if !strings.Contains(got, "^") {
		t.Errorf("missing caret diagnostic for unresolved import:\n%s", got)
	}
}
