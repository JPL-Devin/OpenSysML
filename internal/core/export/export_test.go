package export_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/export"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/rdf"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

var update = flag.Bool("update", false, "rewrite the .golden.ttl and .golden.sysml files")

// TestGoldenConversions locks the exact Turtle written for each model in
// testdata/convert, and the exact notation that Turtle converts back to.
func TestGoldenConversions(t *testing.T) {
	for _, path := range modelFiles(t) {
		name := strings.TrimSuffix(filepath.Base(path), ".sysml")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			turtle, err := export.Convert(path, src, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := export.Convert(name+".ttl", turtle, export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v\n%s", err, turtle)
			}
			checkGolden(t, strings.TrimSuffix(path, ".sysml")+".golden.ttl", turtle)
			checkGolden(t, strings.TrimSuffix(path, ".sysml")+".golden.sysml", back)
		})
	}
}

// TestConvertedNotationParses checks that the notation written from a graph is
// valid SysML: it must parse without a single syntax error.
func TestConvertedNotationParses(t *testing.T) {
	for _, path := range modelFiles(t) {
		name := strings.TrimSuffix(filepath.Base(path), ".sysml")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			turtle, err := export.Convert(path, src, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := export.Convert(name+".ttl", turtle, export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			p := parser.New(source.New(name+".converted.sysml", back))
			p.ParseFile()
			if len(p.Diagnostics) > 0 {
				t.Errorf("converted notation does not parse: %v\n%s", p.Diagnostics, back)
			}
		})
	}
}

// TestRoundTripIsLossless is the fidelity contract: converting the notation a
// graph produced back to a graph gives the same graph. Notation and RDF say the
// same thing in different words, so this is what "no data lost" means — the
// notation itself may legitimately be spelled differently (a name written
// relative to its scope, a keyword written in place of its symbol).
func TestRoundTripIsLossless(t *testing.T) {
	for _, path := range modelFiles(t) {
		name := strings.TrimSuffix(filepath.Base(path), ".sysml")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			first, err := export.Convert(path, src, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := export.Convert(name+".ttl", first, export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			second, err := export.Convert(name+".sysml", back, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle again: %v", err)
			}
			if string(first) != string(second) {
				t.Errorf("round trip changed the graph\n--- first ---\n%s\n--- second ---\n%s", first, second)
			}
		})
	}
}

// TestSaveKeepsComments covers the notation-to-notation path a save uses: every
// lexeme survives, including the comments an AST printer would drop.
func TestSaveKeepsComments(t *testing.T) {
	src := `package P {
// a line note
part def Q; // trailing note
/* a comment */
part def R;
}`
	out, err := export.Convert("save.sysml", []byte(src), export.FormatSysML, export.FormatSysML)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	for _, want := range []string{"// a line note", "// trailing note", "/* a comment */"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("save dropped %q:\n%s", want, out)
		}
	}
}

func TestVerbatimHeadsRoundTrip(t *testing.T) {
	src := `package Connections {
    part def Engine;
    part def Vehicle {
        part engine : Engine;
        part spare : Engine;
        connect engine to spare;
    }
}`
	turtle, err := export.Convert("conn.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	if !strings.Contains(string(turtle), "sourceText") {
		t.Fatalf("expected the connect declaration to be carried as source text:\n%s", turtle)
	}
	back, err := export.Convert("conn.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "connect engine to spare") {
		t.Errorf("connect declaration lost:\n%s", back)
	}
}

func TestSyntaxErrorIsReported(t *testing.T) {
	_, err := export.Convert("bad.sysml", []byte("part def {"), export.FormatSysML, export.FormatTurtle)
	if err == nil {
		t.Fatal("expected a syntax error")
	}
	syntax, ok := err.(*export.SyntaxError)
	if !ok {
		t.Fatalf("expected a *export.SyntaxError, got %T: %v", err, err)
	}
	if len(syntax.Messages) == 0 {
		t.Error("expected at least one message")
	}
	if !strings.Contains(syntax.Error(), "bad.sysml") {
		t.Errorf("error should name the input: %v", syntax)
	}
}

func TestUnsupportedTurtleConstructs(t *testing.T) {
	const prefix = "@prefix sysml: <https://www.omg.org/spec/SysML#> .\n"
	cases := map[string]string{
		"blank node":       prefix + "_:x a sysml:Package .",
		"collection":       prefix + "<urn:x> sysml:client ( <urn:y> ) .",
		"unknown prefix":   "nope:x a nope:Thing .",
		"unterminated":     prefix + "<urn:x> a sysml:Package",
		"no rdf type":      prefix + "<urn:x> sysml:declaredName \"x\" .",
		"unterminated iri": prefix + "<urn:x a sysml:Package .",
		"missing owner":    prefix + "<urn:sysmlv2:element:A::B> a sysml:PartDefinition ; sysml:owningNamespace <urn:sysmlv2:element:A> .",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := export.Convert(name+".ttl", []byte(src), export.FormatTurtle, export.FormatSysML); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestUnknownMetaclassIsUnsupported(t *testing.T) {
	src := "@prefix sysml: <https://www.omg.org/spec/SysML#> .\n" +
		"<urn:sysmlv2:element:X> a sysml:NoSuchMetaclass ; sysml:declaredName \"X\" ."
	_, err := export.Convert("x.ttl", []byte(src), export.FormatTurtle, export.FormatSysML)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "NoSuchMetaclass") {
		t.Errorf("error should name the metaclass, got: %v", err)
	}
}

// TestForeignGraph covers a graph written by another tool: no memberIndex, no
// hasBody, no sourceText, and links between elements only.
func TestForeignGraph(t *testing.T) {
	src := `@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix elmt: <urn:sysmlv2:element:> .

<urn:sysmlv2:element:Demo> a sysml:Package ; sysml:declaredName "Demo" .
<urn:sysmlv2:element:Demo::Engine> a sysml:PartDefinition ;
    sysml:declaredName "Engine" ;
    sysml:owningNamespace elmt:Demo .
<urn:sysmlv2:element:Demo::engine> a sysml:PartUsage ;
    sysml:declaredName "engine" ;
    sysml:owningNamespace elmt:Demo ;
    sysml:type <urn:sysmlv2:element:Demo::Engine> ;
    sysml:upperBound "1" .`
	out, err := export.Convert("foreign.ttl", []byte(src), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	got := strings.Join(strings.Fields(string(out)), " ")
	want := "package Demo { part def Engine; part engine : Engine[1]; }"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestElementIRIsAreQualifiedNames(t *testing.T) {
	graph, err := export.SysMLToRDF("iri.sysml", []byte("package P { part def Q; }"))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	want := rdf.Element + "P::Q"
	for _, subject := range graph.Subjects() {
		if subject.Value == want {
			return
		}
	}
	t.Errorf("expected subject %s in:\n%s", want, rdf.WriteTurtle(graph))
}

func TestFormatDetection(t *testing.T) {
	cases := map[string]export.Format{
		"model.sysml":      export.FormatSysML,
		"model.kerml":      export.FormatSysML,
		"model.ttl":        export.FormatTurtle,
		"dir/model.turtle": export.FormatTurtle,
	}
	for path, want := range cases {
		got, err := export.FormatOfPath(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if got != want {
			t.Errorf("%s: got %v, want %v", path, got, want)
		}
	}
	if _, err := export.FormatOfPath("model.json"); err == nil {
		t.Error("expected an error for an unknown extension")
	}
	if _, err := export.FormatOfPath("model"); err == nil {
		t.Error("expected an error for a missing extension")
	}
	for _, name := range []string{"sysml", "SysML", "kerml", "ttl", " turtle ", "rdf"} {
		if _, err := export.ParseFormat(name); err != nil {
			t.Errorf("ParseFormat(%q): %v", name, err)
		}
	}
	if _, err := export.ParseFormat("xml"); err == nil {
		t.Error("expected an error for an unknown format name")
	}
}

func TestEmptyInputs(t *testing.T) {
	if _, err := export.ToSysML(nil); err == nil {
		t.Error("expected an error for a nil graph")
	}
	if _, err := export.ToRDF(nil, nil); err == nil {
		t.Error("expected an error for a nil document")
	}
}

func modelFiles(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "convert", "*.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, path := range paths {
		if strings.HasSuffix(path, ".golden.sysml") {
			continue
		}
		out = append(out, path)
	}
	if len(out) == 0 {
		t.Fatal("no models in testdata/convert")
	}
	return out
}

func checkGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if string(got) != string(want) {
		t.Errorf("%s differs\n--- want ---\n%s\n--- got ---\n%s", path, want, got)
	}
}
