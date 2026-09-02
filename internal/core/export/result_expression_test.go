package export_test

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
)

// convertFixture converts one fixture of testdata/convert to Turtle.
func convertFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", "convert", name+".sysml")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	turtle, err := export.Convert(path, src, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	return turtle
}

// memberResultLink matches the sysx:resultExpression a result member ends in,
// leaving the results of expression bodies in place.
var memberResultLink = regexp.MustCompile(` ;\n    sysx:resultExpression expr:[^ \n]+_presultExpression \.\n`)

// withoutMemberResultLinks drops the member-level sysx:resultExpression triples.
func withoutMemberResultLinks(t *testing.T, turtle []byte) []byte {
	t.Helper()
	out := memberResultLink.ReplaceAll(turtle, []byte(" .\n"))
	if string(out) == string(turtle) {
		t.Fatal("the graph has no result member links to drop")
	}
	return out
}

// A result expression is a member of its own, owned through a
// ResultExpressionMembership that states the expression, and it keeps its
// place among the body's other members.
func TestResultExpressionIsAResultExpressionMembership(t *testing.T) {
	turtle := string(convertFixture(t, "result_expressions"))
	for _, want := range []string{
		"elmt:Results__AfterMembers___403\n    a sysx:ResultExpressionMember ;",
		"sysx:memberIndex \"3\"^^xsd:integer ;\n    sysml:owningNamespace elmt:Results__AfterMembers ;",
		"sysx:resultExpression expr:Results__AfterMembers___403_presultExpression .",
		"elmt:Results__AfterMembers___403_om\n    a sysml:ResultExpressionMembership ;",
		"sysml:ownedResultExpression expr:Results__AfterMembers___403_presultExpression .",
		"expr:Results__AfterMembers___403_presultExpression\n    a sysml:OperatorExpression ;\n    sysx:sourceText \"y * y\" ;",
	} {
		if !strings.Contains(turtle, want) {
			t.Errorf("the graph lacks\n%s\n--- graph ---\n%s", want, turtle)
		}
	}
	back, err := export.Convert("m.ttl", []byte(turtle), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "attribute y : Real = x + 1;\n        y * y\n    }") {
		t.Errorf("the result expression did not come back after the members before it:\n%s", back)
	}
}

// The expression tree carries every result expression of the fixture — the
// bare operators, literals, invocations, chains and conditionals, the expression
// bodies with their parameters, nested bodies and `in expr` parameters — so the
// notation comes back the same with no sysx:sourceText in the graph at all.
func TestResultExpressionsComeBackFromTheGraphAlone(t *testing.T) {
	turtle := convertFixture(t, "result_expressions")
	withText, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	stripped := withoutTriples(t, turtle, "sysx:sourceText")
	if strings.Contains(string(stripped), "sysx:sourceText") {
		t.Fatal("the stripped graph still carries source text")
	}
	fromGraph, err := export.Convert("m.ttl", stripped, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the mapping alone: %v", err)
	}
	again, err := export.Convert("m.sysml", fromGraph, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	// Rebuilt notation parenthesizes every operator, so its text differs; the
	// model it states must not: the re-encoded graph is the first one.
	if string(withoutTriples(t, again, "sysx:sourceText")) != string(stripped) {
		t.Errorf("the structure alone did not carry the result expressions\n--- with text ---\n%s\n--- from the graph ---\n%s\n--- first ---\n%s\n--- second ---\n%s", withText, fromGraph, turtle, again)
	}
	for _, want := range []string{
		"{ in y : Real; (y + x) }",
		"{ in 'the input' : Real; ('the input' + x) }",
		"{ in y : Real; Double(x = { in z : Real; (z + y) }) }",
		"{ doc /* the parameter, unchanged */ in y : Real; y }",
		"in expr keep : Boolean {\n            in v : Real;\n            (v > x)\n        }",
		"Double(x = Double(x)).result",
		"if (x > 0) ? x else (- x)",
		"(as Real[2])",
		"(as Real[1..*])",
	} {
		if !strings.Contains(string(fromGraph), want) {
			t.Errorf("the notation rebuilt from the graph lacks %q:\n%s", want, fromGraph)
		}
	}
}

// A graph another tool wrote states the result only the metamodel's way, as the
// membership's ownedResultExpression; that is enough to write the body back.
func TestResultExpressionComesBackFromItsMembershipAlone(t *testing.T) {
	turtle := convertFixture(t, "result_expressions")
	stripped := withoutMemberResultLinks(t, withoutTriples(t, turtle, "sysx:sourceText"))
	back, err := export.Convert("m.ttl", stripped, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the membership alone: %v", err)
	}
	for _, want := range []string{"(x * 2)\n    }", "42\n    }", "(v.mass * 2)\n    }"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the notation lacks %q:\n%s", want, back)
		}
	}
	again, err := export.Convert("m.sysml", back, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	if string(withoutTriples(t, again, "sysx:sourceText")) != string(withoutTriples(t, turtle, "sysx:sourceText")) {
		t.Errorf("the membership alone did not carry the result expressions\n--- first ---\n%s\n--- second ---\n%s", turtle, again)
	}
}

// A result expression member whose graph states no expression at all cannot be
// written as a bare expression; it is refused by name rather than dropped.
func TestResultExpressionWithoutAnExpressionIsRefused(t *testing.T) {
	turtle := convertFixture(t, "result_expressions")
	stripped := withoutMemberResultLinks(t, withoutTriples(t, turtle, "sysx:sourceText"))
	stripped = withoutTriples(t, stripped, "sysml:ownedResultExpression")
	_, err := export.Convert("m.ttl", stripped, export.FormatTurtle, export.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("want an UnsupportedError for a result member without its expression, got %v", err)
	}
	if !strings.Contains(err.Error(), "urn:sysmlv2:element:Results__") || !strings.Contains(err.Error(), "sysx:resultExpression") {
		t.Errorf("the refusal should name the member and the property it lacks: %v", err)
	}
}

// A declaration inside an expression body is carried as its notation; a graph
// that states such a member without its text is refused, naming the member.
func TestExpressionBodyDeclarationNeedsItsText(t *testing.T) {
	turtle := convertFixture(t, "expression_body_members")
	if !strings.Contains(string(turtle), "a sysx:BodyMember ;") {
		t.Fatalf("the declaration inside the body should be a sysx:BodyMember:\n%s", turtle)
	}
	back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "{ in y : Real; private attribute k : Real = 2; y * k + x }") {
		t.Errorf("the body did not come back with its declaration:\n%s", back)
	}
	_, err = export.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("want an UnsupportedError for a body member without its text, got %v", err)
	}
	if !strings.Contains(err.Error(), "the body member <urn:opensysml:expr:Bodies__Scaled___401_presultExpression_pm1>") {
		t.Errorf("the refusal should name the body member: %v", err)
	}
}

// Parameters and declarations share one index, so a body written as parameter,
// declaration, parameter comes back in that order when the graph alone orders
// it — only the declarations keep their text; a body of declarations alone is
// written too.
func TestExpressionBodyKeepsTheOrderOfItsDeclarations(t *testing.T) {
	turtle := string(withoutTriples(t, convertFixture(t, "expression_body_order"), "sysx:sourceText"))
	const member = "    a sysx:BodyMember ;\n"
	if strings.Count(turtle, member) != 2 {
		t.Fatalf("expected the declaration of two bodies in the graph:\n%s", turtle)
	}
	turtle = strings.ReplaceAll(turtle, member, member+`    sysx:sourceText "private attribute k : Real = 1;" ;`+"\n")
	back, err := export.Convert("m.ttl", []byte(turtle), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the body's structure: %v", err)
	}
	for _, want := range []string{
		"{ in a : Real; private attribute k : Real = 1; in b : Real; ((a + b) + (k * x)) }",
		"{ private attribute k : Real = 1; }",
	} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the notation rebuilt from the graph lacks %q:\n%s", want, back)
		}
	}
}

// A graph written before parameters became nodes states each one as a name
// literal; an unrestricted name among them comes back quoted.
func TestExpressionBodyParameterLiteralIsQuoted(t *testing.T) {
	turtle := string(withoutTriples(t, convertFixture(t, "result_expressions"), "sysx:sourceText"))
	const node = "sysx:bodyParameter expr:Results__Quoted___401_presultExpression_pin0 ;"
	if !strings.Contains(turtle, node) {
		t.Fatalf("expected the parameter node of the Quoted body in the graph:\n%s", turtle)
	}
	turtle = strings.Replace(turtle, node, `sysx:bodyParameter "the input" ;`, 1)
	back, err := export.Convert("m.ttl", []byte(turtle), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from a literal parameter: %v", err)
	}
	if want := "{ in 'the input'; ('the input' + x) }"; !strings.Contains(string(back), want) {
		t.Errorf("the notation rebuilt from the graph lacks %q:\n%s", want, back)
	}
	if _, err := export.Convert("m.sysml", back, export.FormatSysML, export.FormatTurtle); err != nil {
		t.Fatalf("the rebuilt notation should parse: %v", err)
	}
}

// A body parameter is named; a graph stating one without a name is refused,
// naming the parameter, since `in ;` declares nothing the result could read.
func TestExpressionBodyParameterNeedsItsName(t *testing.T) {
	stripped := string(withoutTriples(t, convertFixture(t, "result_expressions"), "sysx:sourceText"))
	const name = "    sysml:declaredName \"y\" ;\n"
	if strings.Count(stripped, name) != 4 {
		t.Fatalf("expected the parameter y of four bodies in the graph:\n%s", stripped)
	}
	stripped = strings.ReplaceAll(stripped, name, "")
	_, err := export.Convert("m.ttl", []byte(stripped), export.FormatTurtle, export.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("want an UnsupportedError for a nameless body parameter, got %v", err)
	}
	if !strings.Contains(err.Error(), "the body parameter <urn:opensysml:expr:Results__") {
		t.Errorf("the refusal should name the parameter: %v", err)
	}
}
