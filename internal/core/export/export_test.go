package export_test

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
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

// One element of a document is written through the same notation path a whole
// save goes through: the source at the span, comments included, re-indented.
func TestSysMLElementWritesOneElement(t *testing.T) {
	src := `package P {
	// which wheel
	part def Q {
attribute d = 16.0;
}
	part def R;
}`
	file := source.New("session.sysml", []byte(src))
	span := source.Span{Offset: strings.Index(src, "// which wheel")}
	span.Len = strings.Index(src, "\tpart def R;") - span.Offset

	out, syntax, err := export.SysMLElement(file, span)
	if err != nil || syntax != nil {
		t.Fatalf("element: err=%v syntax=%v", err, syntax)
	}
	got := string(out)
	for _, want := range []string{"// which wheel", "part def Q {", "    attribute d = 16.0;"} {
		if !strings.Contains(got, want) {
			t.Errorf("element output is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "part def R") || strings.Contains(got, "package P") {
		t.Errorf("element output covers more than the element:\n%s", got)
	}
}

// A span running past the element into the comments written for what follows
// writes the element alone: trailing trivia is not part of it.
func TestSysMLElementDropsTrailingComments(t *testing.T) {
	src := "part def Q { attribute d = 16.0; }\n\n// which wheel\n/* and another */\npart def R;"
	file := source.New("session.sysml", []byte(src))
	span := source.Span{Offset: 0, Len: strings.Index(src, "part def R;")}

	out, _, err := export.SysMLElement(file, span)
	if err != nil {
		t.Fatalf("element: %v", err)
	}
	if got := strings.TrimRight(string(out), "\n"); got != "part def Q { attribute d = 16.0; }" {
		t.Errorf("element output carries trailing trivia:\n%q", got)
	}
	if _, _, err := export.SysMLElement(file, source.Span{
		Offset: strings.Index(src, "// which wheel"),
		Len:    len("// which wheel\n"),
	}); !errors.Is(err, export.ErrNoNotation) {
		t.Errorf("a span holding only a comment: err=%v, want ErrNoNotation", err)
	}
}

// A span naming no source is reported as such rather than written as an empty
// document, so a caller can explain it instead of printing nothing.
func TestSysMLElementWithoutSource(t *testing.T) {
	file := source.New("session.sysml", []byte("part def Q;"))
	for _, span := range []source.Span{{}, {Offset: 0, Len: 99}, {Offset: -1, Len: 2}} {
		if _, _, err := export.SysMLElement(file, span); !errors.Is(err, export.ErrNoNotation) {
			t.Errorf("span %+v: err=%v, want ErrNoNotation", span, err)
		}
	}
	if _, _, err := export.SysMLElement(nil, source.Span{Len: 1}); !errors.Is(err, export.ErrNoNotation) {
		t.Errorf("no file: err=%v, want ErrNoNotation", err)
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
		"allocate a to b;",
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

// A condition member states a condition rather than declaring a feature, so
// each form it is written in has to come back as written: the keyword it states
// the condition with, its negation, and the nesting of a braced condition.
func TestConditionMembersSurviveRDF(t *testing.T) {
	for _, member := range []string{
		"assert mass > 0;",
		"assume mass != 0;",
		"assert not mass > 1000;",
		"mass < 1000;",
		"assert constraint inner {\n            mass < 1000;\n        }",
	} {
		src := "package P {\n\tattribute mass;\n\tconstraint c {\n\t\t" + member + "\n\t}\n}"
		turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		if err != nil {
			t.Fatalf("%s: to turtle: %v", member, err)
		}
		back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
		if err != nil {
			t.Fatalf("%s: back to notation: %v", member, err)
		}
		if !strings.Contains(string(back), member) {
			t.Errorf("the condition %q was rewritten:\n%s", member, back)
		}
	}
}

// A requirement's assumptions and required conditions are members of the same
// kind, written as a condition, as the constraint they state, or as a body.
func TestRequirementConditionsSurviveRDF(t *testing.T) {
	for _, member := range []string{
		"assume mass > 0;",
		"require mass < 100;",
		"require Light;",
		"require constraint {\n            mass < 100;\n        }",
	} {
		src := "package P {\n\tattribute mass;\n\tconstraint def Light;\n\trequirement r {\n\t\t" + member + "\n\t}\n}"
		turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		if err != nil {
			t.Fatalf("%s: to turtle: %v", member, err)
		}
		back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
		if err != nil {
			t.Fatalf("%s: back to notation: %v", member, err)
		}
		if !strings.Contains(string(back), member) {
			t.Errorf("the requirement member %q was rewritten:\n%s", member, back)
		}
	}
}

// `assert` before a kind keyword says what the declaration it qualifies is for,
// so dropping it would come back as a plain constraint — a different model.
func TestAssertedUsagePrefixSurvivesRDF(t *testing.T) {
	for _, decl := range []string{
		"assert constraint ok : Light;",
		"assert not constraint bad : Light;",
	} {
		src := "package P {\n\tconstraint def Light;\n\tpart def Q {\n\t\t" + decl + "\n\t}\n}"
		turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		if err != nil {
			t.Fatalf("%s: to turtle: %v", decl, err)
		}
		back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
		if err != nil {
			t.Fatalf("%s: back to notation: %v", decl, err)
		}
		if !strings.Contains(string(back), decl) {
			t.Errorf("the prefix of %q was dropped:\n%s", decl, back)
		}
	}
}

// The graph carries a name rather than the notation it was written in, so a
// name that is not a basic name (KerML §8.2.2) has to be written back with the
// quotes of an unrestricted name — without them it is two names, or none.
func TestQuotedNamesSurviveRDF(t *testing.T) {
	for _, decl := range []string{
		"package 'Package Example';",
		"part def <'1'> 'Lander Model';",
		"part 'my rover' : 'Rover Model';",
		"alias 'the rover' for 'Rover Model';",
		"part 'off model' : 'Not Declared Here';",
		"import 'Other Package'::*;",
		// A reserved word and a quote are names a graph can carry, and both
		// need quoting to lex as the name again.
		"part def 'state';",
		"part def 'it\\'s';",
	} {
		src := "package P {\n\tpart def 'Rover Model';\n\t" + decl + "\n}"
		turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		if err != nil {
			t.Fatalf("%s: to turtle: %v", decl, err)
		}
		back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
		if err != nil {
			t.Fatalf("%s: back to notation: %v", decl, err)
		}
		if !strings.Contains(string(back), decl) {
			t.Errorf("the quoted names of %q were rewritten:\n%s", decl, back)
		}
		p := parser.New(source.New("m.converted.sysml", back))
		p.ParseFile()
		if len(p.Diagnostics) > 0 {
			t.Errorf("%s: converted notation does not parse: %v\n%s", decl, p.Diagnostics, back)
		}
	}
}

// Empty braces are part of what a declaration says, so a subject written with
// them must not come back terminated by a semicolon.
func TestSubjectBodySurvivesRDF(t *testing.T) {
	for member, want := range map[string]string{
		"subject r : Rover;":    "subject r : Rover;",
		"subject r : Rover { }": "subject r : Rover {",
		"subject s : Rover { }": "subject s : Rover {",
	} {
		src := "package P {\n\tpart def Rover;\n\trequirement req {\n\t\t" + member + "\n\t}\n}"
		turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		if err != nil {
			t.Fatalf("%s: to turtle: %v", member, err)
		}
		back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
		if err != nil {
			t.Fatalf("%s: back to notation: %v", member, err)
		}
		if !strings.Contains(string(back), want) {
			t.Errorf("the subject %q was rewritten:\n%s", member, back)
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

// A directed usage records no keyword, so whether it wrote its kind out is read
// from the source — where a comment naming a kind must not count as one written.
func TestCommentedKindKeywordIsNotWrittenBack(t *testing.T) {
	// One case per comment shape the lexer distinguishes, plus a keyword the
	// declaration really does write.
	for name, tt := range map[string]struct{ src, want string }{
		"regular comment":     {"package P {\n\tpart p {\n\t\tin /* attribute */ x : Real;\n\t}\n}", "in x : Real;"},
		"single-line note":    {"package P {\n\tpart p {\n\t\tout // attribute\n\t\t\ty : Real;\n\t}\n}", "out y : Real;"},
		"multi-line note":     {"package P {\n\tpart p {\n\t\tin //* a note\n\t\t\tattribute */ w : Real;\n\t}\n}", "in w : Real;"},
		"note over two lines": {"package P {\n\tpart p {\n\t\tin /* a note\n\t\t\tattribute */ v : Real;\n\t}\n}", "in v : Real;"},
		"keyword written":     {"package P {\n\tpart p {\n\t\tin attribute z : Real;\n\t}\n}", "in attribute z : Real;"},
	} {
		t.Run(name, func(t *testing.T) {
			turtle, err := export.Convert("m.sysml", []byte(tt.src), export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			if !strings.Contains(string(back), tt.want) {
				t.Errorf("wanted %q written back from %q:\n%s", tt.want, tt.src, back)
			}
		})
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

// A `perform` declares an action of its own, so the graph carries the keyword
// it was written with: the canonical `action` would be a different declaration.
func TestPerformedActionKeepsItsKeyword(t *testing.T) {
	src := "package P {\n\taction def A;\n\tpart def Q {\n\t\tperform a : A;\n\t}\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "perform a : A;") {
		t.Errorf("the `perform` did not survive the round trip:\n%s", back)
	}
}

// A kind keyword written as one word of a two-word kind (`verification def`
// for a verification case) comes back as written: the canonical spelling
// reparses as a plain `case`, a different kind.
func TestShortKindKeywordSurvivesTheRoundTrip(t *testing.T) {
	src := "package P {\n\tverification def V;\n\tanalysis def A;\n\tanalysis a : A;\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	for _, want := range []string{"verification def V;", "analysis def A;", "analysis a : A;"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("`%s` did not survive the round trip:\n%s", want, back)
		}
	}
}

// A metadata annotation states what the element it prefixes is, so the graph
// carries the notation it was written as rather than dropping it.
func TestPrefixMetadataSurvivesTheRoundTrip(t *testing.T) {
	src := "package P {\n\tmetadata def Safety;\n\t#Safety part def Car;\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "#Safety part def Car;") {
		t.Errorf("the annotation did not survive the round trip:\n%s", back)
	}
}

// A feature that wrote no kind keyword takes its kind from its owner, so the
// graph must not put a keyword back that the author never wrote.
func TestImplicitKindStaysImplicitThroughTheRoundTrip(t *testing.T) {
	src := "package P {\n\taction def Drive {\n\t\tin x : Real;\n\t\tin attribute y : Real;\n\t\tout result : Real;\n\t}\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	for _, want := range []string{"in x : Real;", "in attribute y : Real;", "out result : Real;"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("`%s` did not survive the round trip:\n%s", want, back)
		}
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

// A member-attached `then` is a succession edge whose ends the graph states,
// and whose form says the source end is the member before it, so the notation
// it was written in comes back as written. Member order alone would not carry
// the sequencing: the declaration order here is the reverse.
func TestSuccessionRoundTrips(t *testing.T) {
	src := `package P {
	action def A;
	action def B;
	action def Move {
		action b : B;
		action a : A;
		then action c : A;
	}
}`
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	for _, want := range []string{"sysml:sourceFeature", "sysml:targetFeature", "SuccessionAsUsage"} {
		if !strings.Contains(string(turtle), want) {
			t.Errorf("the graph should carry the succession as %s:\n%s", want, turtle)
		}
	}
	back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "then action c : A;") {
		t.Fatalf("the succession should come back as the form it was written in:\n%s", back)
	}
	// The notation that came back declares the same succession: converting it
	// again yields the same graph, which member order could not have done.
	again, err := export.Convert("m.sysml", back, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	if string(again) != string(turtle) {
		t.Errorf("round trip changed the graph\n--- first ---\n%s\n--- second ---\n%s", turtle, again)
	}
}

// A succession is written back as the edge form, which every body that can carry
// a succession has to read for the notation to survive the round trip.
func TestSuccessionRoundTripsInEveryBody(t *testing.T) {
	bodies := map[string]string{
		"definition":  "part def Q {\n\t\tpart a;\n\t\tthen part b;\n\t}",
		"action":      "action def Q {\n\t\taction a;\n\t\tthen action b;\n\t}",
		"state":       "state def Q {\n\t\tstate a : S;\n\t\tthen state b : S;\n\t}",
		"calculation": "calc def Q {\n\t\tpart a;\n\t\tthen part b;\n\t}",
		"requirement": "requirement def Q {\n\t\tpart a;\n\t\tthen part b;\n\t}",
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			src := "package P {\n\tstate def S;\n\t" + body + "\n}"
			turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			if !strings.Contains(string(turtle), "sysml:sourceFeature") {
				t.Fatalf("the graph should carry the succession's ends:\n%s", turtle)
			}
			back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			// The notation that came back has to parse, and to declare the same
			// succession: a body that cannot read the edge form loses the order.
			again, err := export.Convert("m.sysml", back, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle again (%s):\n%s\n%v", name, back, err)
			}
			if string(again) != string(turtle) {
				t.Errorf("round trip changed the graph\n--- first ---\n%s\n--- second ---\n%s", turtle, again)
			}
		})
	}
}

// A succession is its two ends, so a graph from elsewhere that names only one of
// them declares no order: that is reported rather than written back as notation
// (`succession;`) that says nothing.
func TestHalfNamedSuccessionInAGraphIsReported(t *testing.T) {
	const graph = `@prefix elmt: <urn:sysmlv2:element:> .
@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix sysx: <urn:opensysml:sysml:> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

elmt:P
    a sysml:Package ;
    sysml:qualifiedName "P" ;
    sysx:memberIndex "0"^^xsd:integer ;
    sysml:declaredName "P" ;
    sysx:hasBody "true"^^xsd:boolean .

elmt:P::a
    a sysml:ActionUsage ;
    sysml:qualifiedName "P::a" ;
    sysml:owningNamespace elmt:P ;
    sysx:memberIndex "0"^^xsd:integer ;
    sysml:declaredName "a" ;
    sysx:hasBody "false"^^xsd:boolean .

<urn:sysmlv2:element:P::@1>
    a sysml:SuccessionAsUsage ;
    sysml:qualifiedName "P::@1" ;
    sysml:owningNamespace elmt:P ;
    sysx:memberIndex "1"^^xsd:integer ;
    sysml:sourceFeature elmt:P::a .
`
	out, err := export.Convert("m.ttl", []byte(graph), export.FormatTurtle, export.FormatSysML)
	if err == nil {
		t.Fatalf("a succession naming one end converted to:\n%s", out)
	}
	if !strings.Contains(err.Error(), "does not name both of the members it sequences") {
		t.Errorf("error %q should say why the order cannot be written back", err)
	}
}

// A `then` before a member the notation does not allow a succession in front of
// is a syntax error, so no graph is built from a model whose order is unclear.
func TestSuccessionOnNonUsageIsASyntaxError(t *testing.T) {
	for _, src := range []string{
		"package P {\n\tpart def Q {\n\t\tpart a;\n\t\tthen part def B;\n\t}\n}",
		"package P {\n\tpart def Q {\n\t\tpart a;\n\t\tthen package Inner { }\n\t}\n}",
		"package P {\n\tpart def Q {\n\t\tpart a;\n\t\tthen attribute x;\n\t}\n}",
	} {
		_, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		var syntax *export.SyntaxError
		if !errors.As(err, &syntax) {
			t.Errorf("want a SyntaxError for %q, got %v", src, err)
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

// withoutTriples writes the graph again without the named extension property,
// the shape a graph from another tool has: the head is then rebuilt from the
// mapping rather than read back from the text it was written as.
func withoutTriples(t *testing.T, turtle []byte, property string) []byte {
	t.Helper()
	var blocks []string
	for _, block := range strings.Split(string(turtle), "\n\n") {
		var kept []string
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "sysx:"+property+" ") {
				continue
			}
			kept = append(kept, line)
		}
		// Dropping the last triple of a block leaves the one before it to end it.
		for i := len(kept) - 1; i >= 0; i-- {
			if trimmed := strings.TrimRight(kept[i], "\n"); strings.HasSuffix(trimmed, " ;") {
				kept[i] = strings.TrimSuffix(trimmed, " ;") + " ."
				break
			} else if trimmed != "" {
				break
			}
		}
		blocks = append(blocks, strings.Join(kept, "\n"))
	}
	return []byte(strings.Join(blocks, "\n\n"))
}

// Every end-binding head states the form its ends are written in, so the
// declaration comes back as written from the mapping alone — without the source
// text that our own graphs also carry. Converting that notation again gives the
// original graph back, which is what proves the second hop loses nothing.
func TestEndBindingHeadsComeBackFromTheGraphAlone(t *testing.T) {
	heads := []string{
		"connect left to right;",
		"connect (left, right);",
		"connection c connect left to right;",
		"bind a = b;",
		"allocate a to b;",
		"flow left to right;",
		"flow of Bus from left to right;",
		"succession first left then right;",
		"satisfy R by v;",
		"verify R;",
	}
	for _, head := range heads {
		t.Run(head, func(t *testing.T) {
			src := "package P {\n\tport def Bus;\n\trequirement def R;\n\tpart v;\n\tpart def Car {\n\t\tport left : Bus;\n\t\tport right : Bus;\n\t\tattribute a : Integer;\n\t\tattribute b : Integer;\n\t\t" + head + "\n\t}\n}"
			turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := export.Convert("m.ttl", withoutTriples(t, turtle, "sourceText"), export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			if !strings.Contains(string(back), head) {
				t.Fatalf("the head should come back as written:\n%s", back)
			}
			again, err := export.Convert("m.sysml", back, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle again: %v", err)
			}
			if string(again) != string(turtle) {
				t.Errorf("the second hop changed the graph\n--- first ---\n%s\n--- second ---\n%s", turtle, again)
			}
		})
	}
}

// A transition and an accept state their trigger and payload in the head too,
// inside the bodies that allow them.
func TestBehavioralHeadsComeBackFromTheGraphAlone(t *testing.T) {
	bodies := map[string]string{
		"transition": "state def M {\n\t\tstate s1;\n\t\tstate s2;\n\t\ttransition first s1 accept e then s2;\n\t}",
		"accept":     "action def A {\n\t\taccept x : Bus;\n\t}",
		"send":       "action def A {\n\t\tsend x to y;\n\t}",
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			src := "package P {\n\tport def Bus;\n\tpart x;\n\tpart y;\n\t" + body + "\n}"
			turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := export.Convert("m.ttl", withoutTriples(t, turtle, "sourceText"), export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			again, err := export.Convert("m.sysml", back, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle again: %v", err)
			}
			if string(again) != string(turtle) {
				t.Errorf("the head did not come back as written\n--- notation ---\n%s\n--- first ---\n%s\n--- second ---\n%s", back, turtle, again)
			}
		})
	}
}

// A `then` beside a member the notation leaves unnamed states its source end as
// that member, so it comes back as written where a name could not have said it.
func TestUnnamedSuccessionEndComesBackFromTheGraph(t *testing.T) {
	src := `package P {
	state def S;
	state def M {
		entry;
		then s1;
		state s1 : S;
	}
}`
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "then s1;") {
		t.Fatalf("the succession beside the unnamed entry should come back:\n%s", back)
	}
	again, err := export.Convert("m.sysml", back, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	if string(again) != string(turtle) {
		t.Errorf("the second hop changed the graph\n--- first ---\n%s\n--- second ---\n%s", turtle, again)
	}
	// The form and the member it names carry the succession without the text.
	fromGraph, err := export.Convert("m.ttl", withoutTriples(t, turtle, "sourceText"), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the mapping alone: %v", err)
	}
	if !strings.Contains(string(fromGraph), "then s1;") {
		t.Errorf("the mapping alone should say which member the `then` follows:\n%s", fromGraph)
	}
}

// A graph that relates ends but states no form for them is refused: the ends
// alone do not say which keyword and notation the head was written in.
func TestEndsWithoutTheirFormAreReported(t *testing.T) {
	src := "package P {\n\tpart def Car {\n\t\tpart left;\n\t\tpart right;\n\t\tconnect left to right;\n\t}\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	stripped := withoutTriples(t, withoutTriples(t, turtle, "sourceText"), "endForm")
	_, err = export.Convert("m.ttl", stripped, export.FormatTurtle, export.FormatSysML)
	if err == nil {
		t.Fatal("a head whose form the graph does not state should be reported")
	}
	if !strings.Contains(err.Error(), "sysx:endForm") {
		t.Errorf("the report should name the property it needs: %v", err)
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

// TestForeignGraph covers a graph written by another tool: UUID element IRIs,
// no memberIndex, no hasBody, no sourceText, and links between elements only.
// Every name comes from a property; nothing is recovered from an IRI.
func TestForeignGraph(t *testing.T) {
	src := `@prefix sysml: <https://www.omg.org/spec/SysML#> .

<urn:uuid:aaaa-1> a sysml:Package ;
    sysml:declaredName "Demo" ;
    sysml:qualifiedName "Demo" .
<urn:uuid:aaaa-2> a sysml:PartDefinition ;
    sysml:declaredName "Engine" ;
    sysml:qualifiedName "Demo::Engine" ;
    sysml:owningNamespace <urn:uuid:aaaa-1> .
<urn:uuid:aaaa-3> a sysml:PartUsage ;
    sysml:declaredName "engine" ;
    sysml:qualifiedName "Demo::engine" ;
    sysml:owningNamespace <urn:uuid:aaaa-1> ;
    sysml:type <urn:uuid:aaaa-2> ;
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

// A graph whose referenced elements carry no sysml:qualifiedName cannot be
// written back: the name is never recovered from the IRI, so it is reported.
func TestForeignGraphWithoutQualifiedNamesIsReported(t *testing.T) {
	src := `@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix elmt: <urn:sysmlv2:element:> .

elmt:Demo a sysml:Package ; sysml:declaredName "Demo" .
elmt:Demo__Engine a sysml:PartDefinition ;
    sysml:declaredName "Engine" ;
    sysml:owningNamespace elmt:Demo .`
	out, err := export.Convert("foreign.ttl", []byte(src), export.FormatTurtle, export.FormatSysML)
	if err == nil {
		t.Fatalf("a graph without qualified names converted to:\n%s", out)
	}
	if !strings.Contains(err.Error(), "sysml:qualifiedName") {
		t.Errorf("error %q should name the missing property", err)
	}
}

// A reference whose target cannot be named from the graph — an IRI that is not
// a subject, or a subject without sysml:qualifiedName — is reported, not named.
func TestUnnameableReferencesAreReported(t *testing.T) {
	const head = `@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix elmt: <urn:sysmlv2:element:> .

elmt:P a sysml:Package ; sysml:declaredName "P" ; sysml:qualifiedName "P" .
elmt:P__u a sysml:PartUsage ; sysml:declaredName "u" ; sysml:qualifiedName "P::u" ;
    sysml:owningNamespace elmt:P ;
`
	for name, tail := range map[string]string{
		"reference to an IRI that is not a subject": `    sysml:type <urn:uuid:absent> .`,
		"reference to a subject without qualifiedName": `    sysml:type elmt:P__T .
elmt:P__T a sysml:PartDefinition ; sysml:declaredName "T" ; sysml:owningNamespace elmt:P .`,
	} {
		t.Run(name, func(t *testing.T) {
			out, err := export.Convert("m.ttl", []byte(head+tail), export.FormatTurtle, export.FormatSysML)
			if err == nil {
				t.Fatalf("an unnameable reference converted to:\n%s", out)
			}
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("want an UnsupportedError, got %v", err)
			}
			if !strings.Contains(err.Error(), "sysml:qualifiedName") {
				t.Errorf("error %q should name the missing property", err)
			}
		})
	}
}

// A declaration built from a property the graph does not carry cannot be
// written: reporting it beats emitting notation that will not parse.
func TestMissingRequiredPropertyIsUnsupported(t *testing.T) {
	const head = `@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix sysx: <urn:opensysml:sysml:> .
@prefix elmt: <urn:sysmlv2:element:> .

<urn:sysmlv2:element:P> a sysml:Package ; sysml:declaredName "P" ; sysml:qualifiedName "P" .
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
		// `not` negates the declaration a prefix introduces (`assert not
		// constraint c`), so negation without one has no notation.
		"negation without declaredPrefix": `@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
<urn:sysmlv2:element:P::c> a sysml:ConstraintUsage ; sysml:declaredName "c" ;
    sysml:owningNamespace elmt:P ; sysml:isNegated "true"^^xsd:boolean .`,
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

func TestElementIRIsEncodeQualifiedNames(t *testing.T) {
	graph, err := export.SysMLToRDF("iri.sysml", []byte("package P { part def Q; }"))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	want := rdf.Element + rdf.EncodeElementID("P::Q")
	for _, subject := range graph.Subjects() {
		if subject.Value == want {
			return
		}
	}
	t.Errorf("expected subject %s in:\n%s", want, rdf.WriteTurtle(graph))
}

// Every element IRI in the convert fixtures is the encoding of the qualified
// name the element carries, and the encoding decodes back to that name.
func TestFixtureElementIDsRoundTrip(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "convert", "*.golden.ttl"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fixtures found: %v", err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		graph, err := rdf.ParseTurtle(data)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, subject := range graph.Subjects() {
			if strings.HasPrefix(subject.Value, rdf.Expression) {
				// An expression node is named for the element and slot it
				// belongs to, not by a qualified name of its own.
				id, _, ok := strings.Cut(strings.TrimPrefix(subject.Value, rdf.Expression), ".")
				if owner, decoded := rdf.DecodeElementID(id); !ok || !decoded || owner == "" {
					t.Errorf("%s: expression %s is not named for an element", path, subject.Value)
				}
				continue
			}
			qname, ok := graph.Lexical(subject, rdf.SysML+"qualifiedName")
			if !ok {
				t.Errorf("%s: subject %s has no qualified name", path, subject.Value)
				continue
			}
			if want := rdf.Element + rdf.EncodeElementID(qname); subject.Value != want {
				t.Errorf("%s: subject %s should be %s for %q", path, subject.Value, want, qname)
			}
			id := strings.TrimPrefix(subject.Value, rdf.Element)
			if got, ok := rdf.DecodeElementID(id); !ok || got != qname {
				t.Errorf("%s: id %q decoded to %q, %v, want %q", path, id, got, ok, qname)
			}
		}
	}
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

// A graph written before the rename carries the old extension namespace, whose
// properties this version would read as absent; it must be refused instead.
func TestLegacyExtensionNamespaceIsRefused(t *testing.T) {
	for name, src := range map[string]string{
		"property": "@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .\n" +
			"@prefix sysml: <https://www.omg.org/spec/SysML#> .\n" +
			"@prefix elmt: <urn:sysmlv2:element:> .\n" +
			"@prefix sysx: <urn:systemica:sysml:> .\n" +
			"@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .\n" +
			"elmt:Demo rdf:type sysml:Package ; sysml:declaredName \"Demo\" ;\n" +
			"    sysx:memberIndex \"0\"^^xsd:integer .\n",
		"metaclass": "@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .\n" +
			"@prefix sysml: <https://www.omg.org/spec/SysML#> .\n" +
			"@prefix elmt: <urn:sysmlv2:element:> .\n" +
			"@prefix sysx: <urn:systemica:sysml:> .\n" +
			"elmt:Demo rdf:type sysx:InitialNode ; sysml:declaredName \"Demo\" .\n",
	} {
		graph, err := rdf.ParseTurtle([]byte(src))
		if err != nil {
			t.Fatalf("%s: parse: %v", name, err)
		}
		_, err = export.ToSysML(graph)
		if err == nil {
			t.Fatalf("%s: expected the legacy namespace to be refused", name)
		}
		if !strings.Contains(err.Error(), rdf.LegacyExtension) {
			t.Errorf("%s: error does not name the legacy namespace: %v", name, err)
		}
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
