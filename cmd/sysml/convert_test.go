package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildCLI builds the sysml binary once so the conversion tests exercise the
// real command line, flag parsing included.
func buildCLI(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "sysml")
	build := exec.Command("go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return binary
}

const sampleModel = `package Demo {
    part def Engine;
    part def Vehicle {
        attribute mass : Real = 1200.0;
        part engine : Engine[1];
    }
}
`

func TestConvertRoundTripThroughCLI(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	model := filepath.Join(dir, "model.sysml")
	turtle := filepath.Join(dir, "model.ttl")
	back := filepath.Join(dir, "back.sysml")
	if err := os.WriteFile(model, []byte(sampleModel), 0o644); err != nil {
		t.Fatal(err)
	}

	run(t, binary, "-convert", model, "-o", turtle)
	data, err := os.ReadFile(turtle)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"@prefix sysml:", "elmt:Demo::Vehicle", "a sysml:PartDefinition"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("Turtle output is missing %q:\n%s", want, data)
		}
	}

	run(t, binary, "-convert", turtle, "-o", back)
	got, err := os.ReadFile(back)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(strings.Fields(string(got)), " ") != strings.Join(strings.Fields(sampleModel), " ") {
		t.Errorf("round trip changed the model:\n%s", got)
	}
}

func TestConvertToStdout(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	model := filepath.Join(dir, "model.sysml")
	if err := os.WriteFile(model, []byte(sampleModel), 0o644); err != nil {
		t.Fatal(err)
	}
	// With no -o and no -to, a notation input converts to Turtle.
	out := run(t, binary, "-convert", model)
	if !strings.Contains(out, "@prefix sysml:") {
		t.Errorf("expected Turtle on stdout, got:\n%s", out)
	}
}

func TestConvertExplicitFormats(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	// An extension the tool does not recognize, so the formats must be named.
	model := filepath.Join(dir, "model.txt")
	if err := os.WriteFile(model, []byte(sampleModel), 0o644); err != nil {
		t.Fatal(err)
	}
	out := run(t, binary, "-convert", model, "-from", "sysml", "-to", "turtle")
	if !strings.Contains(out, "@prefix sysml:") {
		t.Errorf("expected Turtle on stdout, got:\n%s", out)
	}
}

func TestConvertSameFormatReformats(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	model := filepath.Join(dir, "model.sysml")
	// Badly indented, with a comment that a re-print from the AST would lose.
	if err := os.WriteFile(model, []byte("package P {\n// keep me\npart def Q;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := run(t, binary, "-convert", model, "-to", "sysml")
	if !strings.Contains(out, "// keep me") {
		t.Errorf("the comment was dropped:\n%s", out)
	}
	if !strings.Contains(out, "    part def Q;") {
		t.Errorf("the output was not re-indented:\n%s", out)
	}
}

func TestConvertErrors(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()

	model := filepath.Join(dir, "model.sysml")
	if err := os.WriteFile(model, []byte(sampleModel), 0o644); err != nil {
		t.Fatal(err)
	}
	unknownExt := filepath.Join(dir, "model.txt")
	if err := os.WriteFile(unknownExt, []byte(sampleModel), 0o644); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(dir, "broken.sysml")
	if err := os.WriteFile(broken, []byte("part def {"), 0o644); err != nil {
		t.Fatal(err)
	}
	badTurtle := filepath.Join(dir, "bad.ttl")
	if err := os.WriteFile(badTurtle, []byte("_:blank <urn:p> \"v\" ."), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := map[string]struct {
		args []string
		want string
	}{
		"missing input":     {[]string{"-convert", filepath.Join(dir, "absent.sysml")}, "absent.sysml"},
		"unknown extension": {[]string{"-convert", unknownExt}, "cannot tell the format"},
		"unknown format":    {[]string{"-convert", model, "-to", "xml"}, "unknown format"},
		"extra argument":    {[]string{"-convert", model, filepath.Join(dir, "other.sysml")}, "unexpected extra argument"},
		"syntax error":      {[]string{"-convert", broken, "-to", "ttl"}, "syntax error"},
		"unsupported rdf":   {[]string{"-convert", badTurtle, "-to", "sysml"}, "blank node"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(binary, tc.args...)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("expected a non-zero exit, got:\n%s", out)
			}
			if !strings.Contains(string(out), tc.want) {
				t.Errorf("expected %q in the error, got:\n%s", tc.want, out)
			}
		})
	}
}

// run executes the binary and returns everything it wrote, failing the test if
// it exits non-zero.
func run(t *testing.T, binary string, args ...string) string {
	t.Helper()
	out, err := exec.Command(binary, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
	return string(out)
}
