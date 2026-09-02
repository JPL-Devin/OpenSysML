package export_test

import (
	"strings"
	"testing"
)

// A model the formatter leaves alone, so what the graph carries as source text
// is the notation itself: comments, notes, blank lines and synonyms included.
const commented = `// The rover, as modelled.
package Rover {
    /* Definitions come first. */
    part def Wheel :> Part; // a synonym the printer would spell out
    //* A note the parser skips
       over two lines. */
    part def Hub;

    part def Vehicle {
        doc /* what a vehicle is for */
        part wheels : Wheel[4]; // four of them
        part hub : Hub;
        // connected at the hub
        connect wheels to hub;
    }
}
`

// commentedTurtle converts commented to Turtle, with the notation it formats to
// (which is commented itself) for comparison.
func commentedTurtle(t *testing.T) []byte {
	t.Helper()
	if got := formatted(t, commented); got != commented {
		t.Fatalf("the fixture is not fixed under the formatter:\n%s", got)
	}
	return idTurtle(t, commented)
}

// editTurtle rewrites one line of a Turtle document, failing when it is absent.
func editTurtle(t *testing.T, turtle []byte, old, new string) []byte {
	t.Helper()
	if !strings.Contains(string(turtle), old) {
		t.Fatalf("%q is not in the graph:\n%s", old, turtle)
	}
	return []byte(strings.Replace(string(turtle), old, new, 1))
}

func TestSourceTextComesBackByteForByte(t *testing.T) {
	turtle := commentedTurtle(t)
	if back := toNotation(t, turtle); back != commented {
		t.Errorf("round trip changed the notation:\n--- want ---\n%s--- got ---\n%s", commented, back)
	}
	// Unformatted notation comes back as the formatter writes it, since that
	// is the text the graph carries.
	loose := strings.ReplaceAll(commented, "    ", "\t")
	if back, want := toNotation(t, idTurtle(t, loose)), formatted(t, loose); back != want {
		t.Errorf("round trip changed the formatted notation:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
}

// Without source text the graph converts to canonical notation as before: the
// structural triples alone carry the model, and the trivia is gone.
func TestStrippedGraphPrintsCanonically(t *testing.T) {
	back := toNotation(t, withoutTriples(t, commentedTurtle(t), "sysx:sourceText"))
	want := `package Rover {
    part def Wheel specializes Part;
    part def Hub;
    part def Vehicle {
        doc /* what a vehicle is for */
        part wheels : Wheel[4];
        part hub : Hub;
        connect wheels to hub;
    }
}
`
	if back != want {
		t.Errorf("canonical notation changed:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
}

// An edit after export wins over the stale text of the member it touched, and
// only that member's lines are replaced with canonical notation.
func TestEditedGraphDoesNotResurrectStaleText(t *testing.T) {
	turtle := editTurtle(t, commentedTurtle(t),
		"    sysml:declaredName \"Hub\" ;\n",
		"    sysml:declaredName \"Hub\" ;\n    sysml:isAbstract \"true\"^^xsd:boolean ;\n")
	back := toNotation(t, turtle)
	want := `// The rover, as modelled.
package Rover {
    /* Definitions come first. */
    part def Wheel :> Part; // a synonym the printer would spell out
    abstract part def Hub;
    part def Vehicle {
        doc /* what a vehicle is for */
        part wheels : Wheel[4]; // four of them
        part hub : Hub;
        // connected at the hub
        connect wheels to hub;
    }
}
`
	if back != want {
		t.Errorf("stale text was not replaced by the edit:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
	// Converting the result again reproduces the edited graph, so the notation
	// states exactly what the graph states.
	again := idTurtle(t, back)
	if got, want := withoutTriples(t, again, "sysx:sourceText"), withoutTriples(t, turtle, "sysx:sourceText"); string(got) != string(want) {
		t.Errorf("the notation does not state the edited graph:\n--- want ---\n%s--- got ---\n%s", want, got)
	}
}

// A removed member takes its text with it: the owner's own text does not
// state its members, so the owner and the remaining member print as written.
func TestRemovedMemberIsNotResurrected(t *testing.T) {
	turtle := idTurtle(t, `package P {
    part def A; // the first
    part def B; // the second
}
`)
	var kept []string
	for _, line := range strings.Split(string(turtle), "\n") {
		if strings.Contains(line, "P__B") && !strings.HasPrefix(line, "elmt:P__B") {
			line = strings.ReplaceAll(line, ", elmt:P__B_om", "")
			line = strings.ReplaceAll(line, ", elmt:P__B", "")
		}
		kept = append(kept, line)
	}
	turtle = withoutSubjects(t, []byte(strings.Join(kept, "\n")), "elmt:P__B", "elmt:P__B_om")
	back := toNotation(t, turtle)
	want := "package P {\n    part def A; // the first\n}\n"
	if back != want {
		t.Errorf("the removed member came back:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
}

// withoutSubjects drops every block of a Turtle document describing one of the
// given subjects.
func withoutSubjects(t *testing.T, turtle []byte, subjects ...string) []byte {
	t.Helper()
	var kept []string
	for _, block := range strings.Split(string(turtle), "\n\n") {
		dropped := false
		for _, subject := range subjects {
			dropped = dropped || strings.HasPrefix(block, subject+"\n")
		}
		if !dropped {
			kept = append(kept, block)
		}
	}
	return []byte(strings.Join(kept, "\n\n"))
}

// Identity the text no longer carries is re-materialized as annotations; text
// that still carries them is kept as written.
func TestIdentityIsRematerializedIntoStaleText(t *testing.T) {
	const projectRef = `@IdentityMetadata::ProjectRef { projectId = "proj-1"; org = "acme"; }`
	const elementID = `@IdentityMetadata::ElementId { id = "shared"; }`
	src := "package P {\n    " + projectRef + "\n    part def A {\n        " + elementID + "\n    }\n    part a : A; // uses A\n}\n"
	turtle := idTurtle(t, src)
	if back := toNotation(t, turtle); back != src {
		t.Errorf("annotated notation changed:\n--- want ---\n%s--- got ---\n%s", src, back)
	}
	// The scope root's text loses its ProjectRef: the root is rebuilt with the
	// annotation, its members are kept as written.
	stale := editTurtle(t, turtle, "    "+strings.ReplaceAll(projectRef, `"`, `\"`)+`\n`, "")
	want := "package P {\n    " + projectRef + "\n    part def A {\n        " + elementID + "\n    }\n    part a : A; // uses A\n}\n"
	if back := toNotation(t, stale); back != want {
		t.Errorf("ProjectRef was not re-materialized:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
	// Both annotations gone from the text: everything but the untouched
	// member is rebuilt.
	stale = editTurtle(t, stale, "        "+strings.ReplaceAll(elementID, `"`, `\"`)+`\n`, "")
	if back := toNotation(t, stale); back != want {
		t.Errorf("ElementId was not re-materialized:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
	// Without any source text the same notation is built from the graph alone.
	if back := toNotation(t, withoutTriples(t, turtle, "sysx:sourceText")); back != strings.Replace(want, " // uses A", "", 1) {
		t.Errorf("stripped graph printed differently:\n%s", back)
	}
}

// An expression's text is checked like an element's: an edited expression graph
// wins over the text of the declaration holding it and over the node's own.
func TestEditedExpressionDoesNotResurrectStaleText(t *testing.T) {
	turtle := idTurtle(t, `package P {
    attribute mass : Real = 4; // the mass
    attribute count : Integer = 2; // and the count
}
`)
	// The literal's value is edited; the text of the literal and of the
	// attribute still say 4.
	turtle = editTurtle(t, turtle, `sysml:value "4"^^xsd:integer`, `sysml:value "5"^^xsd:integer`)
	back := toNotation(t, turtle)
	want := `package P {
    attribute mass : Real = 5;
    attribute count : Integer = 2; // and the count
}
`
	if back != want {
		t.Errorf("the edited expression did not win:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
}

// A syntax error cannot be checked against the graph or pinned on one element,
// so the whole document falls back to canonical notation.
func TestUnparsableSourceTextIsIgnored(t *testing.T) {
	turtle := editTurtle(t, commentedTurtle(t), `part def Hub;\n`, `part def Hub\n`)
	back := toNotation(t, turtle)
	if want := toNotation(t, withoutTriples(t, turtle, "sysx:sourceText")); back != want {
		t.Errorf("unparsable text was not replaced:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
	if strings.Contains(back, "part def Hub\n") || !strings.Contains(back, "part def Hub;") {
		t.Errorf("the notation is not valid:\n%s", back)
	}
}
