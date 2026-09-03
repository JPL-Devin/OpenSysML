package rdf

import (
	"strings"
	"testing"
)

// The Turtle writer declares the annotation namespace under the service's own
// prefix, so an annotated graph reads back with the prefix Flexo uses.
func TestAnnotationPrefixIsDeclared(t *testing.T) {
	g := NewGraph()
	g.Add(IRI(Element+"P"), AnnotationJSONTerm("ownedMember"), String(`[{"@id":"P__A"},{"@id":"P__B"}]`))
	turtle := string(WriteTurtle(g))
	if !strings.Contains(turtle, "@prefix json: <urn:sysmlv2:annotation:json:> .") {
		t.Errorf("the json prefix is not declared:\n%s", turtle)
	}
	if !strings.Contains(turtle, `json:ownedMember "[{\"@id\":\"P__A\"},{\"@id\":\"P__B\"}]"`) {
		t.Errorf("the annotation is not written under the prefix:\n%s", turtle)
	}
}

// A subject stating a sysml: property more than once gets one annotation holding
// the members as JSON in triple order; single values and other namespaces do not.
func TestAnnotateCollections(t *testing.T) {
	g := NewGraph()
	p := IRI(Element + "P")
	g.Add(p, IRI(RDFType), SysMLTerm("Package"))
	g.Add(p, SysMLTerm("declaredName"), String("P"))
	g.Add(p, SysMLTerm("ownedMember"), IRI(Element+"P__B"))
	g.Add(p, SysMLTerm("ownedMember"), IRI(Element+"P__A"))
	g.Add(p, SysMLTerm("ownedMember"), IRI(Expression+"P__A_pa0"))
	g.Add(p, SysMLTerm("aliasIds"), String("x"))
	g.Add(p, SysMLTerm("aliasIds"), String("y"))
	g.Add(p, OpenSysMLTerm("memberIndex"), Int(0))
	g.Add(p, OpenSysMLTerm("memberIndex"), Int(1))
	if err := AnnotateCollections(g); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"ownedMember": `[{"@id":"P__B"},{"@id":"P__A"},{"@id":"P__A_pa0"}]`,
		"aliasIds":    `["x","y"]`,
	}
	for key, text := range want {
		objects := g.Objects(p, AnnotationJSON+key)
		if len(objects) != 1 || objects[0] != String(text) {
			t.Errorf("json:%s = %v, want %q", key, objects, text)
		}
	}
	for _, key := range []string{"declaredName", "memberIndex"} {
		if g.HasProperty(p, AnnotationJSON+key) {
			t.Errorf("json:%s was written for a property that is not a sysml: collection", key)
		}
	}
	if got := len(g.Objects(p, SysML+"ownedMember")); got != 3 {
		t.Errorf("the typed triples were not kept: %d ownedMember objects", got)
	}
	// Annotating again changes nothing.
	before := g.Len()
	if err := AnnotateCollections(g); err != nil {
		t.Fatal(err)
	}
	if g.Len() != before {
		t.Errorf("annotating twice added %d triples", g.Len()-before)
	}
}

// Literal members are spelled as the JSON value of their datatype: booleans and
// numbers bare, everything else as a string.
func TestCollectionJSONLiterals(t *testing.T) {
	text, err := CollectionJSON([]Term{
		Bool(true), Int(3), TypedLiteral("2.50", XSD+"decimal"), String("a <b>"), TypedLiteral("2024-01-01", XSD+"date"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := `[true,3,2.50,"a <b>","2024-01-01"]`; text != want {
		t.Errorf("got %s, want %s", text, want)
	}
	if _, err := CollectionJSON([]Term{TypedLiteral("three", XSD+"integer")}); err == nil {
		t.Error("a non-numeric integer literal was accepted")
	}
}

// ParseCollectionJSON reads references and primitives back and refuses anything
// the service does not store.
func TestParseCollectionJSON(t *testing.T) {
	members, err := ParseCollectionJSON(` [{"@id":"P__A"}, "s", true, 3, 2.50, 1e3] `)
	if err != nil {
		t.Fatal(err)
	}
	want := []CollectionMember{
		{ID: "P__A"},
		{Literal: String("s")},
		{Literal: Bool(true)},
		{Literal: TypedLiteral("3", XSD+"integer")},
		{Literal: TypedLiteral("2.50", XSD+"decimal")},
		{Literal: TypedLiteral("1e3", XSD+"double")},
	}
	if len(members) != len(want) {
		t.Fatalf("got %d members, want %d: %v", len(members), len(want), members)
	}
	for i := range want {
		if members[i] != want[i] {
			t.Errorf("member %d = %#v, want %#v", i, members[i], want[i])
		}
	}
	for name, text := range map[string]string{
		"an object":        `{"@id":"P__A"}`,
		"a string":         `"P__A"`,
		"two values":       `[{"@id":"P__A"}] [{"@id":"P__B"}]`,
		"trailing text":    `[{"@id":"P__A"}] x`,
		"not JSON":         `[{"@id":]`,
		"a null member":    `[null]`,
		"a nested array":   `[[{"@id":"P__A"}]]`,
		"an empty id":      `[{"@id":""}]`,
		"a wider object":   `[{"@id":"P__A","name":"A"}]`,
		"a foreign object": `[{"name":"A"}]`,
	} {
		if _, err := ParseCollectionJSON(text); err == nil {
			t.Errorf("%s was accepted: %s", name, text)
		}
	}
}
