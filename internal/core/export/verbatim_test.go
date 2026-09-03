package export_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
)

// The graph is authoritative and the stored text is layout: a head or expression
// is written back byte-for-byte as stored while its spelling still states the
// graph, and from the graph alone once it does not.

// graphOf converts notation and fails the test on refusal.
func graphOf(t *testing.T, src string) string {
	t.Helper()
	turtle, err := export.Convert("model.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	return string(turtle)
}

// restated replaces one stored sysx:sourceText literal with another, the graph
// otherwise untouched, as a tool that edits the text but not the structure would.
func restated(t *testing.T, turtle, stored, instead string) string {
	t.Helper()
	from := `sysx:sourceText ` + stored
	if !strings.Contains(turtle, from) {
		t.Fatalf("graph stores no %s\n%s", from, turtle)
	}
	return strings.Replace(turtle, from, `sysx:sourceText `+instead, 1)
}

// Layout inside a stored condition, value, body or head survives both hops
// untouched: tabs, blank lines, odd continuation indents, CRLF line ends and
// escaped newlines in strings are part of the text, not of the graph.
func TestStoredLayoutIsWrittenAsStored(t *testing.T) {
	src := "package P {\r\n" +
		"    doc /* first line\r\n" +
		"\t\tsecond line */\r\n" +
		"    part def A;\n" +
		"    part a : A;\n" +
		"    part b : A;\n" +
		"    connect\ta\n" +
		"\n" +
		"          to b;\n" +
		"    part def R {\n" +
		"        attribute x : ScalarValues::Real =\n" +
		"\t\t1\n" +
		"\n" +
		"              + 2;\n" +
		"        attribute s : ScalarValues::String = \"two\\nlines\";\n" +
		"        constraint c {\n" +
		"            x > 0\r\n" +
		"\t  and\n" +
		"                  x < 10\n" +
		"        }\n" +
		"    }\n" +
		"}\n"
	first := graphOf(t, src)
	back := toNotation(t, []byte(first))
	for _, kept := range []string{
		"doc /* first line\r\n\t\tsecond line */",
		"connect\ta\n\n          to b;",
		"= 1\n\n              + 2;",
		`"two\nlines"`,
		"x > 0\r\n\t  and\n                  x < 10",
	} {
		if !strings.Contains(back, kept) {
			t.Errorf("written notation lost the stored layout %q\n%s", kept, back)
		}
	}
	second := graphOf(t, back)
	if first != second {
		t.Errorf("second conversion moved the graph\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

// Stored text that spells the graph differently but not otherwise — spacing,
// line breaks, quoted names, a comment — is still the author's layout and is kept.
func TestStoredTextIsKeptUpToSpelling(t *testing.T) {
	src := `package P {
    part def A;
    part a : A;
    part b : A;
    connect a to b;
}
`
	for _, instead := range []string{
		`"connect  a\n\t\tto   b;"`,
		`"connect 'a' to 'b';"`,
		`"connect a /* which */ to b;"`,
	} {
		turtle := restated(t, graphOf(t, src), `"connect a to b;"`, instead)
		back := toNotation(t, []byte(turtle))
		want := strings.NewReplacer(`\n`, "\n", `\t`, "\t").Replace(strings.Trim(instead, `"`))
		if !strings.Contains(back, want) {
			t.Errorf("stored %s was not written as stored\n%s", instead, back)
		}
	}
}

// Stored text that states other ends, or another kind, than the graph does is
// not written: the head is spelled from the graph instead.
func TestStoredHeadThatDisagreesWithTheGraphIsRebuilt(t *testing.T) {
	src := `package P {
    part def A;
    part a : A;
    part b : A;
    connect a to b;
}
`
	for _, instead := range []string{
		`"connect b to a;"`,
		`"connect a to a;"`,
		`"bind a = b;"`,
	} {
		turtle := restated(t, graphOf(t, src), `"connect a to b;"`, instead)
		back := toNotation(t, []byte(turtle))
		if !strings.Contains(back, "connect a to b;") {
			t.Errorf("stored %s did not give way to the graph's ends\n%s", instead, back)
		}
	}
}

// Stored text that leaves a comment, note, string or quoted name open would
// swallow every declaration written after it; it is spelled from the graph.
func TestStoredTextThatDoesNotLexCleanIsRebuilt(t *testing.T) {
	src := `package P {
    part def A;
    part a : A;
    part b : A;
    connect a to b;
    part c : A;
}
`
	for _, instead := range []string{
		`"connect a to b; /* rest"`,
		`"connect a to b; //* rest"`,
		`"connect 'a to b;"`,
		`"connect a to b; \"rest"`,
	} {
		turtle := restated(t, graphOf(t, src), `"connect a to b;"`, instead)
		back := toNotation(t, []byte(turtle))
		if !strings.Contains(back, "connect a to b;\n    part c : A;") {
			t.Errorf("stored %s was not rebuilt from the graph\n%s", instead, back)
		}
	}
}

// A stored expression is written as stored while reading it gives the operator
// and operands the graph states, and from the graph once it does not.
func TestStoredExpressionThatDisagreesWithTheGraphIsRebuilt(t *testing.T) {
	src := `package P {
    part def R {
        attribute x : ScalarValues::Real;
        constraint c {
            x > 0
        }
    }
}
`
	first := graphOf(t, src)
	kept := toNotation(t, []byte(restated(t, first, `"x > 0"`, `"x  >\n\t\t0"`)))
	if !strings.Contains(kept, "x  >\n\t\t0") {
		t.Errorf("relaid expression was not written as stored\n%s", kept)
	}
	for _, instead := range []string{`"x > 1"`, `"x < 0"`, `"0 > x"`, `"x >"`, `"x > 0 and true"`} {
		back := toNotation(t, []byte(restated(t, first, `"x > 0"`, instead)))
		if !strings.Contains(back, "x > 0") || strings.Contains(back, strings.Trim(instead, `"`)+"\n") {
			t.Errorf("stored %s did not give way to the graph's expression\n%s", instead, back)
		}
	}
}

// A string literal's sysml:value is the string itself, not its notation: editing
// it to one with quotes, backslashes and line breaks, the stale stored text left
// in place, writes a literal that reads back as the edited value.
func TestEditedStringValueIsWrittenEscaped(t *testing.T) {
	src := `package P {
    attribute s = "plain";
}
`
	first := graphOf(t, src)
	if !strings.Contains(first, `sysml:value "plain" .`) {
		t.Fatalf("graph does not carry the string's value\n%s", first)
	}
	edited := strings.Replace(first, `sysml:value "plain" .`,
		"sysml:value \"\"\"say \\\"hi\\\"\\\\\ttab\nnext\"\"\" .", 1)
	back := toNotation(t, []byte(edited))
	want := `attribute s = "say \"hi\"\\\ttab\nnext";`
	if !strings.Contains(back, want) {
		t.Fatalf("edited value was not written as a valid escaped literal\nwant %s\n%s", want, back)
	}
	again := graphOf(t, back)
	if !strings.Contains(again, "sysml:value \"\"\"say \\\"hi\\\"\\\\\ttab\nnext\"\"\" .") {
		t.Errorf("written literal does not read back as the edited value\n%s", again)
	}
}

// The order in which a Turtle document lists the objects of one property states
// nothing, so listing an expression's operands the other way round keeps the
// stored text: sysx:argumentIndex carries their order.
func TestStoredExpressionSurvivesReorderedOperandTriples(t *testing.T) {
	src := `package P {
    part def R {
        attribute x : ScalarValues::Real;
        constraint c {
            x > 0
        }
    }
}
`
	first := restated(t, graphOf(t, src), `"x > 0"`, `"x  >\n\t\t0"`)
	listed := regexp.MustCompile(`sysml:argument (expr:\S+_pa0), (expr:\S+_pa1)`)
	if !listed.MatchString(first) {
		t.Fatalf("graph lists no operands\n%s", first)
	}
	reordered := listed.ReplaceAllString(first, "sysml:argument $2, $1")
	if !strings.Contains(toNotation(t, []byte(reordered)), "x  >\n\t\t0") {
		t.Errorf("relaid expression gave way over operand order alone\n%s", reordered)
	}
}

// Stored text that states more than the graph does — an operator, referent or
// end the graph no longer carries — is not written either: what the graph does
// not state is reported, never revived from the text.
func TestStoredTextDoesNotReviveDeletedStructure(t *testing.T) {
	expression := `package P {
    part def R {
        attribute x : ScalarValues::Real;
        constraint c {
            x > 0
        }
    }
}
`
	connector := `package P {
    part def A;
    part a : A;
    part b : A;
    connect a to b;
}
`
	for _, tc := range []struct {
		name, src, deleted, instead, reported string
	}{
		{"operator", expression, `\n    sysml:operator ">" ;`, "", "states the operator"},
		{"referent", expression, `\n    sysml:referent elmt:P__R__x ;`, "", "feature reference"},
		{"end", connector, `, expr:\S+_pend1 ;`, " ;", "end"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			turtle := graphOf(t, tc.src)
			deleted := regexp.MustCompile(tc.deleted)
			if !deleted.MatchString(turtle) {
				t.Fatalf("graph does not state %q\n%s", tc.deleted, turtle)
			}
			edited := deleted.ReplaceAllLiteralString(turtle, tc.instead)
			back, err := export.Convert("m.ttl", []byte(edited), export.FormatTurtle, export.FormatSysML)
			if err == nil {
				t.Fatalf("deleted %s was revived from the stored text\n%s", tc.name, back)
			}
			if !strings.Contains(err.Error(), tc.reported) {
				t.Errorf("error does not name the missing %s: %v", tc.name, err)
			}
		})
	}
}

// A declaration the graph keeps as text alone — no sysx:endForm to rebuild its
// head from — is written as stored while the text, read back as a declaration,
// states the graph, and gives way once the graph's metaclass or keyword is
// edited under it: the edit is written or refused, never overridden by the text.
func TestStoredDeclarationGivesWayToEditedGraph(t *testing.T) {
	sysml := `package P {
    port def Bus;
    part def Car {
        port left : Bus;
        port right : Bus;
        connect left to right {
            doc /* wired */
        }
        bind left = right { doc /* same */ }
    }
}
`
	kerml := `package P {
    feature a;
    feature b;
    binding ab of a = b;
}
`
	toKerML, err := export.Convert("model.kerml", []byte(kerml), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	connector := `"""connect left to right {
            doc /* wired */
        }"""`
	for _, tc := range []struct {
		name, turtle, stored, edit, instead string
	}{
		{"connector metaclass", graphOf(t, sysml), connector, "a sysml:ConnectionUsage", "a sysml:InterfaceUsage"},
		{"connector keyword", graphOf(t, sysml), connector, `sysx:declaredKeyword "connect"`, `sysx:declaredKeyword "connection"`},
		{"binding metaclass", graphOf(t, sysml), `"bind left = right { doc /* same */ }"`, "a sysml:BindingConnectorAsUsage", "a sysml:ConnectionUsage"},
		{"binding keyword", graphOf(t, sysml), `"bind left = right { doc /* same */ }"`, `sysx:declaredKeyword "bind"`, `sysx:declaredKeyword "binding"`},
		{"kerml binding metaclass", string(toKerML), `"binding ab of a = b;"`, "a sysml:BindingConnectorAsUsage", "a sysml:ConnectorAsUsage"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stored := strings.Trim(tc.stored, `"`)
			if !strings.Contains(toNotation(t, []byte(tc.turtle)), stored) {
				t.Fatalf("untouched graph did not write %q as stored", stored)
			}
			if !strings.Contains(tc.turtle, tc.edit) {
				t.Fatalf("graph does not state %s\n%s", tc.edit, tc.turtle)
			}
			edited := strings.Replace(tc.turtle, tc.edit, tc.instead, 1)
			back, err := export.Convert("m.ttl", []byte(edited), export.FormatTurtle, export.FormatSysML)
			if err == nil && strings.Contains(string(back), stored) {
				t.Errorf("stored text overrode the edit %s -> %s\n%s", tc.edit, tc.instead, back)
			}
		})
	}
	relaid := restated(t, graphOf(t, sysml), `"bind left = right { doc /* same */ }"`, `"bind  left=right {\n\tdoc /* same */ }"`)
	if back := toNotation(t, []byte(relaid)); !strings.Contains(back, "bind  left=right {\n\tdoc /* same */ }") {
		t.Errorf("relaid declaration was not written as stored\n%s", back)
	}
	relaid = restated(t, string(toKerML), `"binding ab of a = b;"`, `"binding ab  of a =\n\tb;"`)
	if back := toNotation(t, []byte(relaid)); !strings.Contains(back, "binding ab  of a =\n\tb;") {
		t.Errorf("relaid KerML declaration was not written as stored\n%s", back)
	}
}

// An expression the graph keeps as text alone is still held to lexing clean and
// parsing whole: one leaving a comment open, or one cut short (`x >`), is
// refused as stating nothing, not written.
func TestTextOnlyExpressionThatDoesNotLexCleanIsRefused(t *testing.T) {
	src := `package P {
    part def R {
        attribute x : ScalarValues::Real;
        attribute y = x->select {in v; v > 0};
        part c;
    }
}
`
	turtle := graphOf(t, src)
	structure := regexp.MustCompile(`\n    sysx:(bodyParameter|resultExpression) [^\n]*;`)
	if len(structure.FindAllString(turtle, -1)) != 2 {
		t.Fatalf("graph does not state the body's parameter and result\n%s", turtle)
	}
	textOnly := structure.ReplaceAllLiteralString(turtle, "")
	if !strings.Contains(toNotation(t, []byte(textOnly)), "{in v; v > 0}") {
		t.Fatal("text-only body expression was not written from its text")
	}
	for name, stale := range map[string]string{
		"open comment": `"{in v; v > 0} /* rest"`,
		"cut short":    `"{in v; v > }"`,
		"trailing":     `"{in v; v > 0} x"`,
		"trigger":      `"when v > 0"`,
	} {
		edited := restated(t, textOnly, `"{in v; v > 0}"`, stale)
		back, err := export.Convert("m.ttl", []byte(edited), export.FormatTurtle, export.FormatSysML)
		if err == nil {
			t.Fatalf("%s in a text-only expression was written\n%s", name, back)
		}
		if !strings.Contains(err.Error(), "states no notation and no structure") {
			t.Errorf("%s: error does not report the expression as stating nothing: %v", name, err)
		}
	}
	// Text alone is not a node: with its type deleted the graph states nothing to write.
	untyped := strings.Replace(textOnly, "\n    a sysml:Expression ;\n    sysx:sourceText \"{in v; v > 0}\"", "\n    sysx:sourceText \"{in v; v > 0}\"", 1)
	if untyped == textOnly {
		t.Fatalf("body expression node not found to untype\n%s", textOnly)
	}
	if back, err := export.Convert("m.ttl", []byte(untyped), export.FormatTurtle, export.FormatSysML); err == nil {
		t.Fatalf("untyped text-only expression was written from stale text\n%s", back)
	}
	// An accept payload's value is a trigger expression, and only there is one read.
	trigger := graphOf(t, "package P {\n    action def A {\n        action a accept when  x > 0;\n    }\n}\n")
	if !strings.Contains(trigger, `sysx:sourceText "when  x > 0"`) {
		t.Fatalf("trigger is not stored as text\n%s", trigger)
	}
	if back := toNotation(t, []byte(trigger)); !strings.Contains(back, "accept when  x > 0;") {
		t.Errorf("stored trigger was not written as stored\n%s", back)
	}
	for name, stale := range map[string]string{"cut short": `"when  x >"`, "no trigger": `"x > 0"`} {
		edited := restated(t, trigger, `"when  x > 0"`, stale)
		if back, err := export.Convert("m.ttl", []byte(edited), export.FormatTurtle, export.FormatSysML); err == nil {
			t.Fatalf("%s trigger was written\n%s", name, back)
		}
	}
}

// Without any stored text the writer spells every head and expression from
// the graph, and the graph it converts to states the same structure.
func TestNotationComesBackFromTheGraphAlone(t *testing.T) {
	src := "package P {\n" +
		"    part def A;\n" +
		"    part a : A;\n" +
		"    part b : A;\n" +
		"    connect a\n" +
		"        to b;\n" +
		"    part def R {\n" +
		"        attribute x : ScalarValues::Real =\n" +
		"            1 + 2;\n" +
		"        constraint c {\n" +
		"            x > 0\n" +
		"                and x < 10\n" +
		"        }\n" +
		"    }\n" +
		"}\n"
	first := graphOf(t, src)
	stripped := withoutTriples(t, []byte(first), "sysx:sourceText")
	if strings.Contains(string(stripped), "sourceText") {
		t.Fatal("sourceText not stripped")
	}
	back := toNotation(t, stripped)
	for _, want := range []string{"connect a to b;", "= (1 + 2);", "((x > 0) and (x < 10));"} {
		if !strings.Contains(back, want) {
			t.Errorf("graph alone did not spell %q\n%s", want, back)
		}
	}
	second := withoutTriples(t, []byte(graphOf(t, back)), "sysx:sourceText")
	if string(stripped) != string(second) {
		t.Errorf("structural spelling changed the graph\n--- stripped ---\n%s\n--- second ---\n%s", stripped, second)
	}
}
