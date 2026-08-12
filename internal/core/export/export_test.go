package export_test

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

// Several kind keywords are synonyms that the AST records as one kind, so the
// keyword as written is carried through the graph rather than normalized.
func TestKindKeywordSynonymsSurviveRDF(t *testing.T) {
	for _, decl := range []string{
		"datatype D;",
		"feature f;",
		"function def F;",
		"message m;",
		"allocate al;",
		"timeslice ts;",
		"snapshot sn;",
	} {
		src := "package P {\n\t" + decl + "\n}"
		turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		if err != nil {
			t.Fatalf("%s: to turtle: %v", decl, err)
		}
		back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
		if err != nil {
			t.Fatalf("%s: back to notation: %v", decl, err)
		}
		if !strings.Contains(string(back), decl) {
			t.Errorf("the keyword of %q was rewritten:\n%s", decl, back)
		}
	}
}

// A keyword sitting in a comment inside a declaration head is trivia, not the
// declaration's kind, so it must not become the keyword written back.
func TestCommentInHeadDoesNotChangeKeyword(t *testing.T) {
	for _, src := range []string{
		"package P {\n\tattribute // the flow rate\n\t\trate : Real;\n}",
		"package P {\n\tpart /* a state */ def X;\n}",
	} {
		turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		if err != nil {
			t.Fatalf("to turtle: %v", err)
		}
		if strings.Contains(string(turtle), "declaredKeyword") {
			t.Errorf("a comment word was recorded as the kind keyword:\n%s", turtle)
		}
		back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
		if err != nil {
			t.Fatalf("back to notation: %v", err)
		}
		for _, keyword := range []string{"flow ", "state "} {
			if strings.Contains(string(back), keyword) {
				t.Errorf("the declaration came back as a %sdeclaration:\n%s", keyword, back)
			}
		}
	}
}

// A usage whose head is kept verbatim comes back as written, so a synonym
// keyword on it needs no rebuilding and is not refused.
func TestVerbatimSynonymConverts(t *testing.T) {
	src := "package P {\n\trequirement def R;\n\tverify R;\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "verify R;") {
		t.Errorf("`verify R;` did not survive the round trip:\n%s", back)
	}
}

// A synonym keyword whose declaration names no element of its own takes an
// inline reference, a shape the graph cannot rebuild, so it is reported rather
// than written back as the canonical keyword — a different declaration.
func TestUnrebuildableSynonymIsUnsupported(t *testing.T) {
	src := "package P {\n\taction def A;\n\tpart def Q {\n\t\tperform a : A;\n\t}\n}"
	_, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("want an UnsupportedError for a `perform` reference, got %v", err)
	}
	if !strings.Contains(err.Error(), "perform") {
		t.Errorf("the error should name the keyword: %v", err)
	}
}

// Every direction rejects notation the parser cannot read, including the
// notation-to-notation save: formatting broken input would suggest it is valid.
func TestSysMLToSysMLChecksSyntax(t *testing.T) {
	_, err := export.Convert("bad.sysml", []byte("package P {\n\tpart ((( ;\n}"), export.FormatSysML, export.FormatSysML)
	var syntax *export.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("want a SyntaxError, got %v", err)
	}
}

// A succession cannot be placed from the AST flag alone, so it is reported
// rather than written back in a position that would change execution order.
func TestSuccessionIsUnsupported(t *testing.T) {
	src := `package P {
	action def A;
	action def B;
	action def Move {
		action a : A;
		then action b : B;
	}
}`
	_, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("want an UnsupportedError for a `then` succession, got %v", err)
	}
	if !strings.Contains(err.Error(), "then") {
		t.Errorf("the error should name the construct: %v", err)
	}
}

// A `then` prefix marks whatever membership follows it, which need not wrap a
// usage, so the refusal has to reach every member kind rather than usages only.
func TestSuccessionOnNonUsageIsUnsupported(t *testing.T) {
	for _, src := range []string{
		"package P {\n\tpart def Q {\n\t\tthen part def B;\n\t}\n}",
		"package P {\n\tpart def Q {\n\t\tthen package Inner { }\n\t}\n}",
	} {
		_, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		var unsupported *export.UnsupportedError
		if !errors.As(err, &unsupported) {
			t.Errorf("want an UnsupportedError for %q, got %v", src, err)
		}
	}
}

// A qualified name identifies an element, so two members of one namespace
// sharing a name would merge into a single subject.
func TestDuplicateNameIsUnsupported(t *testing.T) {
	src := "package P {\n\tpart def A;\n\tpart def A;\n}"
	_, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("want an UnsupportedError for a duplicate name, got %v", err)
	}
}

// Ownership that forms a cycle leaves no root to print from, which would
// otherwise write an empty document and report success.
func TestOwnershipCycleIsUnsupported(t *testing.T) {
	const turtle = `@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix elmt: <urn:sysmlv2:element:> .
elmt:A a sysml:Package ; sysml:declaredName "A" ; sysml:qualifiedName "A" ;
  sysml:owningNamespace elmt:B .
elmt:B a sysml:Package ; sysml:declaredName "B" ; sysml:qualifiedName "B" ;
  sysml:owningNamespace elmt:A .
`
	out, err := export.Convert("cycle.ttl", []byte(turtle), export.FormatTurtle, export.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("want an UnsupportedError for a containment cycle, got %v (output %q)", err, out)
	}
}

// A round trip through RDF drops lexical trivia, which no element owns, but
// keeps `doc` and `comment` because those are declarations.
func TestCommentsThroughRDF(t *testing.T) {
	src := `package Demo {
	// a lexical line comment
	doc /* what this package is for */
	comment about Wheel /* a note on wheels */
	part def Wheel;
}`
	ttl, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := export.Convert("m.ttl", ttl, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("to sysml: %v", err)
	}
	got := string(back)
	for _, want := range []string{
		"doc /* what this package is for */",
		"comment about Wheel /* a note on wheels */",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("round trip dropped the declaration %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "a lexical line comment") {
		t.Errorf("trivia unexpectedly survived; update the documented limitation:\n%s", got)
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

// A declaration built from a property the graph does not carry cannot be
// written: reporting it beats emitting notation that will not parse.
func TestMissingRequiredPropertyIsUnsupported(t *testing.T) {
	const head = `@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix sysx: <urn:systemica:sysml:> .
@prefix elmt: <urn:sysmlv2:element:> .

<urn:sysmlv2:element:P> a sysml:Package ; sysml:declaredName "P" .
`
	for name, subject := range map[string]string{
		"alias without aliasedElement": `<urn:sysmlv2:element:P::X> a sysx:Alias ;
    sysml:declaredName "X" ; sysml:owningNamespace elmt:P .`,
		"dependency without supplier": `<urn:sysmlv2:element:P::D> a sysml:Dependency ;
    sysml:declaredName "D" ; sysml:owningNamespace elmt:P ; sysml:client "A" .`,
		"representation without language": `<urn:sysmlv2:element:P::R> a sysml:TextualRepresentation ;
    sysml:declaredName "R" ; sysml:owningNamespace elmt:P ; sysml:body "x" .`,
		"import without importedNamespace": `<urn:sysmlv2:element:P::I> a sysml:Import ;
    sysml:owningNamespace elmt:P .`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := export.Convert("m.ttl", []byte(head+subject), export.FormatTurtle, export.FormatSysML)
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("want an UnsupportedError, got %v", err)
			}
		})
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

// A save replaces the previous file only once the new bytes are safely written,
// and the result is an ordinary readable document.
func TestWriteFileIsAtomicAndReadable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "model.sysml")

	replaced, err := export.WriteFile(path, []byte("package P;\n"))
	if err != nil || replaced {
		t.Fatalf("first write: replaced=%v err=%v", replaced, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("mode = %o, want 644", got)
	}
	replaced, err = export.WriteFile(path, []byte("package Q;\n"))
	if err != nil || !replaced {
		t.Fatalf("second write: replaced=%v err=%v", replaced, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "package Q;\n" {
		t.Errorf("content = %q", data)
	}
	// No temporary file is left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("leftover files in %s: %v", dir, entries)
	}
}

// A missing parent directory is named rather than surfacing as a bare open(2)
// failure.
func TestWriteFileNamesTheMissingDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "absent")
	_, err := export.WriteFile(filepath.Join(dir, "model.sysml"), []byte("package P;\n"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), dir) || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("unhelpful error: %v", err)
	}
}

// The REPL's tolerant save writes notation it could not fully parse and reports
// the syntax errors; every other direction still refuses.
func TestConvertTolerant(t *testing.T) {
	broken := []byte("package P { part x; }\npart 3x;\n")
	out, syntax, err := export.ConvertTolerant("<session>", broken, export.FormatSysML, export.FormatSysML)
	if err != nil {
		t.Fatalf("sysml to sysml: %v", err)
	}
	if syntax == nil {
		t.Error("expected the syntax errors to be reported")
	}
	if !strings.Contains(string(out), "part 3x;") {
		t.Errorf("the unreadable text was dropped:\n%s", out)
	}
	if _, _, err := export.ConvertTolerant("<session>", broken, export.FormatSysML, export.FormatTurtle); err == nil {
		t.Error("Turtle should still refuse a broken model")
	}
}

// Saving is an edit of the user's file, so a model they had kept private does
// not become world-readable because they saved it again.
func TestWriteFileKeepsExistingPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private.sysml")
	if err := os.WriteFile(path, []byte("package P;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := export.WriteFile(path, []byte("package Q;\n")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o, want 600", got)
	}
}

// A pipe or a device is a stream, not a file with contents to protect, so it is
// written as it stands rather than replaced by a rename.
func TestWriteFileWritesThroughAPipe(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "pipe.sysml")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	read := make(chan string, 1)
	go func() {
		data, err := os.ReadFile(fifo)
		if err != nil {
			t.Error(err)
		}
		read <- string(data)
	}()
	replaced, err := export.WriteFile(fifo, []byte("package Q;\n"))
	if err != nil {
		t.Fatal(err)
	}
	if replaced {
		t.Error("a pipe is not an existing file that was replaced")
	}
	if got := <-read; got != "package Q;\n" {
		t.Errorf("read %q from the pipe", got)
	}
	if info, err := os.Stat(fifo); err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		t.Errorf("the pipe was replaced by a regular file (%v)", err)
	}
}

// An existing file inside a directory the user cannot add entries to is still
// written: the temporary file is impossible there, but the save is not.
func TestWriteFileFallsBackWhenTheDirectoryIsClosed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := filepath.Join(t.TempDir(), "closed")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "model.sysml")
	if err := os.WriteFile(path, []byte("package Longer;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(dir, 0o700) }()
	replaced, err := export.WriteFile(path, []byte("package Q;\n"))
	if err != nil || !replaced {
		t.Fatalf("replaced=%v err=%v", replaced, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "package Q;\n" {
		t.Errorf("file = %q, want the new model with nothing of the old one left", data)
	}
}

// A symlink is a pointer to the model, so saving over it updates the model
// rather than replacing the link with a regular file.
func TestWriteFileWritesThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.sysml")
	link := filepath.Join(dir, "link.sysml")
	if err := os.WriteFile(real, []byte("package P;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	replaced, err := export.WriteFile(link, []byte("package Q;\n"))
	if err != nil || !replaced {
		t.Fatalf("replaced=%v err=%v", replaced, err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the symlink was replaced by a regular file (%v)", err)
	}
	data, err := os.ReadFile(real)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "package Q;\n" {
		t.Errorf("the linked model was not updated: %q", data)
	}
}

// A failed save names the file the user asked for, never the temporary file
// this package made up.
func TestWriteFileErrorNamesTheRequestedPath(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "model.sysml")
	_, err := export.WriteFile(path, []byte("package P;\n"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error does not name %s: %v", path, err)
	}
	if strings.Contains(err.Error(), ".model.sysml.") {
		t.Errorf("error leaks the temporary file: %v", err)
	}
}
