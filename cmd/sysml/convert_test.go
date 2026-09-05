package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
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

// refusedModel declares one name twice in a namespace, which the RDF mapping
// refuses: a name identifies an element in the graph.
const refusedModel = `package Demo {
    part def Seat;
    part seat : Seat;
    part seat : Seat;
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

	run(t, binary, model, "-convert", "ttl", "-o", turtle)
	data, err := os.ReadFile(turtle)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"@prefix sysml:", "elmt:Demo__Vehicle", "a sysml:PartDefinition"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("Turtle output is missing %q:\n%s", want, data)
		}
	}

	run(t, binary, turtle, "-convert", "sysml", "-o", back)
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
	// With no -o, the conversion is written to stdout.
	out := run(t, binary, model, "-convert", "ttl")
	if !strings.Contains(out, "@prefix sysml:") {
		t.Errorf("expected Turtle on stdout, got:\n%s", out)
	}
}

// TestConvertFlagOrder checks that the model may be named before or after the
// flags that apply to it, since Go's flag package stops at the first file name
// unless the arguments are reordered.
func TestConvertFlagOrder(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	model := filepath.Join(dir, "model.sysml")
	if err := os.WriteFile(model, []byte(sampleModel), 0o644); err != nil {
		t.Fatal(err)
	}

	orders := map[string][]string{
		"model first":  {model, "-convert", "ttl"},
		"flags first":  {"-convert", "ttl", model},
		"model middle": {"-from", "sysml", model, "-convert", "ttl"},
	}
	for name, args := range orders {
		t.Run(name, func(t *testing.T) {
			if out := run(t, binary, args...); !strings.Contains(out, "@prefix sysml:") {
				t.Errorf("expected Turtle on stdout, got:\n%s", out)
			}
		})
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
	out := run(t, binary, model, "-convert", "turtle", "-from", "sysml")
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
	out := run(t, binary, model, "-convert", "sysml")
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
		"missing input":     {[]string{filepath.Join(dir, "absent.sysml"), "-convert", "ttl"}, "absent.sysml"},
		"no input":          {[]string{"-convert", "ttl"}, "no model to convert"},
		"unknown extension": {[]string{unknownExt, "-convert", "ttl"}, "cannot tell the format"},
		"unknown format":    {[]string{model, "-convert", "xml"}, "unknown format"},
		"file as format":    {[]string{"-convert", model}, "-convert names the format"},
		"extra argument":    {[]string{model, filepath.Join(dir, "other.sysml"), "-convert", "ttl"}, "unexpected extra argument"},
		"replaced -to flag": {[]string{model, "-convert", "ttl", "-to", "sysml"}, "-to has been replaced by -convert"},
		"forgotten value":   {[]string{model, "-convert", "ttl", "-o"}, "flag needs an argument: -o"},
		"syntax error":      {[]string{broken, "-convert", "ttl"}, "syntax error"},
		"unsupported rdf":   {[]string{badTurtle, "-convert", "sysml"}, "blank node"},
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

// TestConvertRDFIsMarkedExperimental checks every RDF conversion says so on
// stderr — including one the mapping refuses — and that the notice never lands
// in the converted model on stdout.
func TestConvertRDFIsMarkedExperimental(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	model := filepath.Join(dir, "model.sysml")
	behavior := filepath.Join(dir, "refused.sysml")
	if err := os.WriteFile(model, []byte(sampleModel), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(behavior, []byte(refusedModel), 0o644); err != nil {
		t.Fatal(err)
	}

	to := runCommand(t, exec.Command(binary, model, "-convert", "ttl"))
	if to.status != 0 {
		t.Fatalf("converting to Turtle failed: %s%s", to.stdout, to.stderr)
	}
	if !strings.Contains(to.stderr, "RDF conversion is experimental") {
		t.Errorf("no experimental notice on stderr:\n%s", to.stderr)
	}
	if strings.Contains(to.stdout, "experimental") {
		t.Errorf("the notice landed in the converted model:\n%s", to.stdout)
	}

	turtle := filepath.Join(dir, "model.ttl")
	run(t, binary, model, "-convert", "ttl", "-o", turtle)
	from := runCommand(t, exec.Command(binary, turtle, "-convert", "sysml"))
	if !strings.Contains(from.stderr, "RDF conversion is experimental") {
		t.Errorf("reading RDF is experimental too, but was not marked:\n%s", from.stderr)
	}

	notation := runCommand(t, exec.Command(binary, model, "-convert", "sysml"))
	if strings.Contains(notation.output(), "experimental") {
		t.Errorf("a notation conversion is stable, but was marked:\n%s", notation.output())
	}

	refused := runCommand(t, exec.Command(binary, behavior, "-convert", "ttl"))
	if refused.status == 0 {
		t.Fatalf("expected the mapping to refuse the duplicate declaration:\n%s", refused.stdout)
	}
	if !strings.Contains(refused.stderr, "RDF conversion is experimental") {
		t.Errorf("a refusal is the experimental behavior, but was not marked:\n%s", refused.stderr)
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

// TestConvertMigratesXMI drives a SysML v1 migration through the command line:
// the .xmi extension picks the input format, -migration-report writes text or
// JSON by extension, the summary goes to stderr otherwise, and XMI is never a
// target.
func TestConvertMigratesXMI(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	xmi := filepath.Join("..", "..", "internal", "core", "migrate", "testdata", "cameo", "vehicle.xmi")
	model := filepath.Join(dir, "model.sysml")
	if err := os.WriteFile(model, []byte(sampleModel), 0o644); err != nil {
		t.Fatal(err)
	}

	migrated := runCommand(t, exec.Command(binary, xmi, "-convert", "sysml"))
	if migrated.status != 0 {
		t.Fatalf("migrating failed: %s%s", migrated.stdout, migrated.stderr)
	}
	if !strings.Contains(migrated.stdout, "part def Vehicle") {
		t.Errorf("no migrated notation on stdout:\n%s", migrated.stdout)
	}
	if !strings.Contains(migrated.stderr, "migration: migrated") || strings.Contains(migrated.stdout, "migration:") {
		t.Errorf("the migration summary belongs on stderr:\nstdout: %s\nstderr: %s", migrated.stdout, migrated.stderr)
	}

	textReport := filepath.Join(dir, "report.txt")
	jsonReport := filepath.Join(dir, "report.json")
	turtle := filepath.Join(dir, "model.ttl")
	run(t, binary, xmi, "-convert", "ttl", "-o", turtle, "-migration-report", textReport)
	run(t, binary, xmi, "-from", "xmi", "-convert", "sysml", "-o", model, "-migration-report", jsonReport)
	text, err := os.ReadFile(textReport)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "unmapped") || !strings.Contains(string(text), "Vehicle") {
		t.Errorf("text report lacks its verdicts:\n%s", text)
	}
	var report migrate.Report
	raw, err := os.ReadFile(jsonReport)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("JSON report does not decode: %v\n%s", err, raw)
	}
	if report.Exporter != "MagicDraw UML" || len(report.Entries) == 0 {
		t.Errorf("JSON report is incomplete: exporter %q, %d entries", report.Exporter, len(report.Entries))
	}
	// The written notation must be what the ttl was built from.
	run(t, binary, model, "-convert", "ttl")

	for name, tc := range map[string]struct {
		args []string
		want string
	}{
		"xmi as target":          {[]string{model, "-convert", "xmi"}, "cannot write xmi"},
		"report without xmi":     {[]string{model, "-convert", "ttl", "-migration-report", textReport}, "-migration-report describes a SysML v1 migration"},
		"report without convert": {[]string{model, "-migration-report", textReport}, "-migration-report accompanies -convert"},
		"notation read as xmi":   {[]string{model, "-from", "xmi", "-convert", "sysml"}, "model.sysml"},
	} {
		t.Run(name, func(t *testing.T) {
			out, err := exec.Command(binary, tc.args...).CombinedOutput()
			if err == nil {
				t.Fatalf("expected a non-zero exit, got:\n%s", out)
			}
			if !strings.Contains(string(out), tc.want) {
				t.Errorf("expected %q in the error, got:\n%s", tc.want, out)
			}
		})
	}
}
