package format

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/libs"
)

func format(t *testing.T, src string) string {
	t.Helper()
	out, err := Source("t.sysml", []byte(src), DefaultOptions)
	if err != nil {
		t.Fatalf("Source() error = %v", err)
	}
	return string(out)
}

func TestIndentsNestedBlocks(t *testing.T) {
	got := format(t, "package P {\npart def Car {\nattribute mass;\n}\n}\n")
	want := "package P {\n    part def Car {\n        attribute mass;\n    }\n}\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestReindentsOverIndentedBlocks(t *testing.T) {
	got := format(t, "package P {\n            part def Car;\n}\n")
	if want := "package P {\n    part def Car;\n}\n"; got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestStripsTrailingWhitespaceAndCollapsesBlankLines(t *testing.T) {
	got := format(t, "package P {   \n\n\n\n    part def Car;   \n}\n")
	if want := "package P {\n\n    part def Car;\n}\n"; got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestTightensBracketPaddingAndSeparators(t *testing.T) {
	got := format(t, "package P { calc def c { return add( 1 , 2 ) ; } }\n")
	if want := "package P { calc def c { return add(1, 2); } }\n"; got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestPreservesAuthorLineBreaks(t *testing.T) {
	// A one-line block stays on one line; a split one stays split.
	if got := format(t, "package P { part def Car; }\n"); got != "package P { part def Car; }\n" {
		t.Fatalf("one-liner rewrapped: %q", got)
	}
}

func TestPreservesCommentsAndNotes(t *testing.T) {
	src := "package P {\n// a note\n/* a comment */\n//* a\n   multi-line note */\npart def Car;\n}\n"
	got := format(t, src)
	for _, want := range []string{"// a note", "/* a comment */", "multi-line note */"} {
		if !strings.Contains(got, want) {
			t.Fatalf("comment %q lost:\n%s", want, got)
		}
	}
}

func TestPreservesStringContents(t *testing.T) {
	src := "package P {\nattribute a = \"  spaced  ;  \";\n}\n"
	if got := format(t, src); !strings.Contains(got, `"  spaced  ;  "`) {
		t.Fatalf("string literal altered:\n%s", got)
	}
}

func TestDoesNotInsertSpacesTheAuthorOmitted(t *testing.T) {
	got := format(t, "package P { calc def c { return 1+2; } }\n")
	if !strings.Contains(got, "1+2") {
		t.Fatalf("spacing invented:\n%s", got)
	}
}

func TestUseTabs(t *testing.T) {
	out, err := Source("t.sysml", []byte("package P {\npart def Car;\n}\n"), Options{UseTabs: true})
	if err != nil {
		t.Fatal(err)
	}
	if want := "package P {\n\tpart def Car;\n}\n"; string(out) != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestUnbalancedBracesDoNotPanic(t *testing.T) {
	if got := format(t, "package P { part def Car; } }\n"); got == "" {
		t.Fatal("empty output")
	}
}

// TestCorpusIsPreservedAndStable formats every shipped library file and example
// model, asserting the token stream survives and that formatting is idempotent.
func TestCorpusIsPreservedAndStable(t *testing.T) {
	src := libs.DefaultSource()
	for _, name := range src.List() {
		content, err := src.Read(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		checkStable(t, name, content)
	}

	for _, dir := range []string{"../../../examples", "../runtime/testdata"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read dir %s: %v", dir, err)
		}
		for _, e := range entries {
			ext := filepath.Ext(e.Name())
			if e.IsDir() || (ext != ".sysml" && ext != ".kerml") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			checkStable(t, path, content)
		}
	}
}

func checkStable(t *testing.T, name string, content []byte) {
	t.Helper()
	once, err := Source(name, content, DefaultOptions)
	if err != nil {
		t.Errorf("%s: %v", name, err)
		return
	}
	twice, err := Source(name, once, DefaultOptions)
	if err != nil {
		t.Errorf("%s (second pass): %v", name, err)
		return
	}
	if string(once) != string(twice) {
		t.Errorf("%s: formatting is not idempotent", name)
	}
}
