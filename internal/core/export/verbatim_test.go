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
