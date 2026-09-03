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

// A result expression is the Expression element itself, owned through a
// ResultExpressionMembership whose member it is, and it keeps its place among
// the body's other members.
func TestResultExpressionIsAResultExpressionMembership(t *testing.T) {
	turtle := string(convertFixture(t, "result_expressions"))
	for _, want := range []string{
		"elmt:Results__AfterMembers___403\n    a sysml:OperatorExpression ;",
		"sysx:memberIndex \"3\"^^xsd:integer ;\n    sysml:owningNamespace elmt:Results__AfterMembers ;",
		"sysx:sourceText \"y * y\" ;\n    sysml:operator \"*\" ;",
		"elmt:Results__AfterMembers___403_om\n    a sysml:ResultExpressionMembership ;",
		"sysml:ownedMemberElement elmt:Results__AfterMembers___403 ;",
		"sysml:ownedMemberFeature elmt:Results__AfterMembers___403 ;",
		"sysml:ownedResultExpression elmt:Results__AfterMembers___403 .",
	} {
		if !strings.Contains(turtle, want) {
			t.Errorf("the graph lacks\n%s\n--- graph ---\n%s", want, turtle)
		}
	}
	for _, unwanted := range []string{"ResultExpressionMember ", "ResultExpressionMember;", "_presultExpression"} {
		if strings.Contains(turtle, unwanted) {
			t.Errorf("the graph still wraps the result in a member of its own (%q):\n%s", unwanted, turtle)
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

// membershipMember matches the member properties of a membership.
var membershipMember = regexp.MustCompile(`    sysml:(memberElement|ownedMemberElement|ownedRelatedElement|ownedMemberFeature) elmt:[^ \n]+ ;\n`)

// withoutResultMembers drops the member properties of every result membership,
// leaving its sysml:ownedResultExpression and every other membership in place.
func withoutResultMembers(t *testing.T, turtle []byte) []byte {
	t.Helper()
	blocks := strings.Split(string(turtle), "\n\n")
	stripped := 0
	for i, block := range blocks {
		if strings.Contains(block, "a sysml:ResultExpressionMembership ;") {
			blocks[i] = membershipMember.ReplaceAllString(block, "")
			stripped++
		}
	}
	if stripped == 0 {
		t.Fatal("the graph has no result memberships to strip")
	}
	return []byte(strings.Join(blocks, "\n\n"))
}

// A graph another tool wrote states the result only the metamodel's way, as the
// membership's ownedResultExpression; that is enough to write the body back.
func TestResultExpressionComesBackFromItsMembershipAlone(t *testing.T) {
	turtle := convertFixture(t, "result_expressions")
	stripped := withoutResultMembers(t, withoutTriples(t, turtle, "sysx:sourceText"))
	if !strings.Contains(string(stripped), "sysml:owningType elmt:Results__AfterMembers ;\n    sysml:ownedResultExpression elmt:Results__AfterMembers___403 .") {
		t.Fatalf("expected the result membership to keep only its ownedResultExpression:\n%s", stripped)
	}
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

// A result expression whose graph states no structure to rebuild it from
// cannot be written as a bare expression; it is refused by name rather than
// dropped.
func TestResultExpressionWithoutAnExpressionIsRefused(t *testing.T) {
	turtle := string(withoutTriples(t, convertFixture(t, "result_expressions"), "sysx:sourceText"))
	const operands = "    sysml:operator \"*\" ;\n    sysml:argument expr:Results__AfterMembers___403_pa0, expr:Results__AfterMembers___403_pa1 .\n"
	if !strings.Contains(turtle, operands) {
		t.Fatalf("expected the operands of the AfterMembers result in the graph:\n%s", turtle)
	}
	turtle = strings.Replace(turtle, operands, "    sysml:operator \"*\" .\n", 1)
	_, err := export.Convert("m.ttl", []byte(turtle), export.FormatTurtle, export.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("want an UnsupportedError for a result expression without its operands, got %v", err)
	}
	if !strings.Contains(err.Error(), "the expression <urn:sysmlv2:element:Results__AfterMembers___403>") {
		t.Errorf("the refusal should name the result expression: %v", err)
	}
}

// A graph a conforming tool wrote owns a body's result the abstract syntax way
// alone — an Expression with no qualified name, no sysx: property and no
// notation, reached from its ResultExpressionMembership — and is written back.
func TestResultExpressionOwnedTheStandardWayIsRead(t *testing.T) {
	const turtle = `@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix ex: <urn:example:> .

ex:P a sysml:Package ; sysml:qualifiedName "P" ; sysml:declaredName "P" .
ex:Real a sysml:AttributeDefinition ; sysml:qualifiedName "P::Real" ; sysml:declaredName "Real" ; sysml:owningMembership ex:Real_m .
ex:Real_m a sysml:OwningMembership ; sysml:membershipOwningNamespace ex:P ; sysml:memberElement ex:Real .
ex:C a sysml:CalculationDefinition ; sysml:qualifiedName "P::C" ; sysml:declaredName "C" ; sysml:owningMembership ex:C_m .
ex:C_m a sysml:OwningMembership ; sysml:membershipOwningNamespace ex:P ; sysml:memberElement ex:C .
ex:x a sysml:ReferenceUsage ; sysml:qualifiedName "P::C::x" ; sysml:declaredName "x" ; sysml:direction "in" ; sysml:type ex:Real ; sysml:owningMembership ex:x_m .
ex:x_m a sysml:ParameterMembership ; sysml:membershipOwningNamespace ex:C ; sysml:memberElement ex:x .
ex:r a sysml:OperatorExpression ; sysml:operator "*" ; sysml:argument ex:r_a, ex:r_b ; sysml:owningMembership ex:r_m .
ex:r_m a sysml:ResultExpressionMembership ; sysml:membershipOwningNamespace ex:C ; sysml:ownedMemberFeature ex:r ; sysml:ownedResultExpression ex:r .
ex:r_a a sysml:FeatureReferenceExpression ; sysml:referent ex:x .
ex:r_b a sysml:LiteralInteger ; sysml:value "2"^^xsd:integer .
`
	back, err := export.Convert("m.ttl", []byte(turtle), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from a standard graph: %v", err)
	}
	if want := "x : Real;\n        (x * 2)\n    }"; !strings.Contains(string(back), want) {
		t.Errorf("the notation lacks %q:\n%s", want, back)
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
	if !strings.Contains(err.Error(), "the body member <urn:opensysml:expr:Bodies__Scaled___401_pm1>") {
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
	const node = "sysx:bodyParameter expr:Results__Quoted___401_pin0 ;"
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

// A name is a string; a literal of any other datatype or with a language tag is
// a different term, so a graph stating one as a parameter is refused with the
// literal and the subject that states it named.
func TestNonStringLiteralsAreRefused(t *testing.T) {
	turtle := string(withoutTriples(t, convertFixture(t, "result_expressions"), "sysx:sourceText"))
	const node = "sysx:bodyParameter expr:Results__Quoted___401_pin0 ;"
	if !strings.Contains(turtle, node) {
		t.Fatalf("expected the parameter node of the Quoted body in the graph:\n%s", turtle)
	}
	cases := []struct {
		name, literal, want string
	}{
		{"typed", `sysx:bodyParameter "3"^^xsd:integer ;`, `the literal "3"^^xsd:integer stated by <urn:sysmlv2:element:Results__Quoted___401> sysx:bodyParameter: sysx:bodyParameter takes a string`},
		{"language-tagged", `sysx:bodyParameter "eingabe"@de ;`, `the literal "eingabe"@de stated by <urn:sysmlv2:element:Results__Quoted___401> sysx:bodyParameter: a language-tagged literal is an rdf:langString`},
		{"explicit string", `sysx:bodyParameter "the input"^^xsd:string ;`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			graph := strings.Replace(turtle, node, tc.literal, 1)
			back, err := export.Convert("m.ttl", []byte(graph), export.FormatTurtle, export.FormatSysML)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("an xsd:string literal is a string: %v", err)
				}
				if want := "{ in 'the input'; ('the input' + x) }"; !strings.Contains(string(back), want) {
					t.Errorf("the notation rebuilt from the graph lacks %q:\n%s", want, back)
				}
				return
			}
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("want an UnsupportedError for %s, got %v", tc.literal, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the refusal should name the literal and its subject:\n got %v\nwant %s", err, tc.want)
			}
		})
	}
}

// The check is decoder-wide: a typed literal where the metamodel has a String
// is refused whichever property states it.
func TestTypedLiteralNamesAreRefusedEverywhere(t *testing.T) {
	turtle := string(convertFixture(t, "result_expressions"))
	for _, tc := range []struct{ from, to, want string }{
		{`sysml:declaredName "Quoted" ;`, `sysml:declaredName "42"^^xsd:integer ;`, "sysml:declaredName takes a string"},
		{`sysml:operator "+" ;`, `sysml:operator "+"@en ;`, "a language-tagged literal is an rdf:langString"},
		{`sysx:memberIndex "0"^^xsd:integer ;`, `sysx:memberIndex "0"^^xsd:decimal ;`, "sysx:memberIndex takes a string or xsd:integer"},
	} {
		if !strings.Contains(turtle, tc.from) {
			t.Fatalf("expected %q in the graph:\n%s", tc.from, turtle)
		}
		_, err := export.Convert("m.ttl", []byte(strings.Replace(turtle, tc.from, tc.to, 1)), export.FormatTurtle, export.FormatSysML)
		var unsupported *export.UnsupportedError
		if !errors.As(err, &unsupported) {
			t.Fatalf("want an UnsupportedError for %s, got %v", tc.to, err)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("for %s:\n got %v\nwant %s", tc.to, err, tc.want)
		}
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
