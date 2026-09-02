package modules

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const testOWL = `<?xml version="1.0"?>
<rdf:RDF xmlns="https://www.omg.org/spec/SysML#"
     xml:base="https://www.omg.org/spec/SysML"
     xmlns:owl="http://www.w3.org/2002/07/owl#"
     xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
     xmlns:xsd="http://www.w3.org/2001/XMLSchema#"
     xmlns:rdfs="http://www.w3.org/2000/01/rdf-schema#"
     xmlns:Ecore="http://www.eclipse.org/emf/2002/Ecore#">
    <owl:Ontology rdf:about="https://www.omg.org/spec/SysML">
        <owl:imports rdf:resource="http://www.eclipse.org/emf/2002/Ecore"/>
        <rdfs:label>sysml</rdfs:label>
    </owl:Ontology>
    <rdfs:Datatype rdf:about="https://www.omg.org/spec/SysML#VisibilityKind">
        <owl:equivalentClass>
            <rdfs:Datatype>
                <owl:oneOf>
                    <rdf:Description>
                        <rdf:type rdf:resource="http://www.w3.org/1999/02/22-rdf-syntax-ns#List"/>
                        <rdf:first>private</rdf:first>
                        <rdf:rest>
                            <rdf:Description>
                                <rdf:type rdf:resource="http://www.w3.org/1999/02/22-rdf-syntax-ns#List"/>
                                <rdf:first>public</rdf:first>
                                <rdf:rest rdf:resource="http://www.w3.org/1999/02/22-rdf-syntax-ns#nil"/>
                            </rdf:Description>
                        </rdf:rest>
                    </rdf:Description>
                </owl:oneOf>
            </rdfs:Datatype>
        </owl:equivalentClass>
    </rdfs:Datatype>
    <owl:ObjectProperty rdf:about="https://www.omg.org/spec/SysML#Element_owner">
        <rdfs:domain rdf:resource="https://www.omg.org/spec/SysML#Element"/>
        <rdfs:range rdf:resource="https://www.omg.org/spec/SysML#Element"/>
        <Ecore:isOrdered rdf:datatype="http://www.w3.org/2001/XMLSchema#boolean">false</Ecore:isOrdered>
    </owl:ObjectProperty>
    <owl:DatatypeProperty rdf:about="https://www.omg.org/spec/SysML#Membership_visibility">
        <rdfs:domain rdf:resource="https://www.omg.org/spec/SysML#Membership"/>
        <rdfs:range rdf:resource="https://www.omg.org/spec/SysML#VisibilityKind"/>
    </owl:DatatypeProperty>
    <owl:Class rdf:about="https://www.omg.org/spec/SysML#Element">
        <rdfs:subClassOf>
            <owl:Restriction>
                <owl:onProperty rdf:resource="https://www.omg.org/spec/SysML#Element_owner"/>
                <owl:maxCardinality rdf:datatype="http://www.w3.org/2001/XMLSchema#nonNegativeInteger">1</owl:maxCardinality>
            </owl:Restriction>
        </rdfs:subClassOf>
        <rdfs:label>Element</rdfs:label>
        <rdfs:comment>An &lt;code&gt;Element&lt;/code&gt; is a "constituent".
Second line.</rdfs:comment>
    </owl:Class>
    <owl:Class rdf:about="https://www.omg.org/spec/SysML#Membership">
        <rdfs:subClassOf rdf:resource="https://www.omg.org/spec/SysML#Element"/>
    </owl:Class>
</rdf:RDF>
`

const testXMI = `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="20131001" xmlns:xmi="http://www.omg.org/spec/XMI/20161101" xmlns:uml="http://www.omg.org/spec/UML/20161101">
  <uml:Package xmi:id="k" name="KerML" URI="https://www.omg.org/spec/KerML/20240201">
    <ownedComment xmi:id="k.c" xmi:type="uml:Comment" body="The kernel modeling language."/>
    <packagedElement xmi:type="uml:Package" xmi:id="k.root" name="Root">
      <ownedComment xmi:id="k.root.c" xmi:type="uml:Comment" body="The Root layer."/>
      <packagedElement xmi:type="uml:Package" xmi:id="k.root.el" name="Elements">
        <packagedElement xmi:type="uml:Class" xmi:id="k.el" name="Element">
          <ownedComment xmi:id="k.el.c" xmi:type="uml:Comment" body="Class comments are not package comments."/>
        </packagedElement>
      </packagedElement>
      <packagedElement xmi:type="uml:Package" xmi:id="k.root.ns" name="Namespaces">
        <packagedElement xmi:type="uml:Class" xmi:id="k.ms" name="Membership"/>
        <packagedElement xmi:type="uml:Enumeration" xmi:id="k.vk" name="VisibilityKind"/>
        <packagedElement xmi:type="uml:Association" xmi:id="k.a" name="A_membership"/>
      </packagedElement>
    </packagedElement>
  </uml:Package>
</xmi:XMI>
`

func testMetamodel(t *testing.T) *Metamodel {
	t.Helper()
	m := &Metamodel{}
	if err := m.ReadXMI(strings.NewReader(testXMI)); err != nil {
		t.Fatal(err)
	}
	return m
}

func testTriples(t *testing.T) []Triple {
	t.Helper()
	triples, err := ReadRDFXML(strings.NewReader(testOWL))
	if err != nil {
		t.Fatal(err)
	}
	return triples
}

func TestReadRDFXML(t *testing.T) {
	triples := testTriples(t)
	has := func(want string) {
		t.Helper()
		for _, tr := range triples {
			if tr.String() == want {
				return
			}
		}
		t.Errorf("missing triple %s", want)
	}
	has(`<https://www.omg.org/spec/SysML> <http://www.w3.org/2002/07/owl#imports> <http://www.eclipse.org/emf/2002/Ecore>`)
	has(`<https://www.omg.org/spec/SysML#Element> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://www.w3.org/2002/07/owl#Class>`)
	has(`<https://www.omg.org/spec/SysML#Element_owner> <http://www.eclipse.org/emf/2002/Ecore#isOrdered> "false"^^<http://www.w3.org/2001/XMLSchema#boolean>`)
	has(`<https://www.omg.org/spec/SysML#Element> <http://www.w3.org/2000/01/rdf-schema#comment> "An <code>Element</code> is a \"constituent\".\nSecond line."`)
	blank := 0
	for _, tr := range triples {
		if tr.Subject.Kind == BlankNode {
			blank++
		}
	}
	// the datatype, the list's two cells (3 triples each) and the restriction
	if blank != 2+6+3 {
		t.Errorf("blank-subject triples = %d, want 11", blank)
	}
}

func TestReadRDFXMLRejectsUnsupported(t *testing.T) {
	for name, doc := range map[string]string{
		"parseType":  `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description rdf:about="x:a"><rdf:value rdf:parseType="Literal"><b/></rdf:value></rdf:Description></rdf:RDF>`,
		"mixed":      `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description rdf:about="x:a"><rdf:value>text<rdf:Description/></rdf:value></rdf:Description></rdf:RDF>`,
		"notRDF":     `<owl:Ontology xmlns:owl="http://www.w3.org/2002/07/owl#"/>`,
		"aboutAndID": `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description rdf:about="x:a" rdf:nodeID="b"/></rdf:RDF>`,
		"noBase":     `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description rdf:about="#a"><rdf:value>1</rdf:value></rdf:Description></rdf:RDF>`,
		"badBase":    `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xml:base="relative/base"><rdf:Description rdf:about="#a"><rdf:value>1</rdf:value></rdf:Description></rdf:RDF>`,
		// A property element takes exactly one of: reference, nested node, literal.
		"resourceText":      `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description rdf:about="x:a"><rdf:value rdf:resource="x:b">text</rdf:value></rdf:Description></rdf:RDF>`,
		"resourceDatatype":  `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description rdf:about="x:a"><rdf:value rdf:resource="x:b" rdf:datatype="x:d"/></rdf:Description></rdf:RDF>`,
		"datatypeResource":  `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description rdf:about="x:a"><rdf:value rdf:datatype="x:d" rdf:resource="x:b"/></rdf:Description></rdf:RDF>`,
		"nodeIDDatatype":    `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description rdf:about="x:a"><rdf:value rdf:nodeID="n" rdf:datatype="x:d"/></rdf:Description></rdf:RDF>`,
		"resourceAndNodeID": `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description rdf:about="x:a"><rdf:value rdf:resource="x:b" rdf:nodeID="n"/></rdf:Description></rdf:RDF>`,
		"resourceNested":    `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description rdf:about="x:a"><rdf:value rdf:resource="x:b"><rdf:Description/></rdf:value></rdf:Description></rdf:RDF>`,
		"datatypeNested":    `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description rdf:about="x:a"><rdf:value rdf:datatype="x:d"><rdf:Description/></rdf:value></rdf:Description></rdf:RDF>`,
		"nestedTrailing":    `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description rdf:about="x:a"><rdf:value><rdf:Description/>text</rdf:value></rdf:Description></rdf:RDF>`,
		"twoNested":         `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description rdf:about="x:a"><rdf:value><rdf:Description/><rdf:Description/></rdf:value></rdf:Description></rdf:RDF>`,
	} {
		if _, err := ReadRDFXML(strings.NewReader(doc)); err == nil {
			t.Errorf("%s: parsed without error", name)
		}
	}
}

func TestReadRDFXMLResolvesReferences(t *testing.T) {
	const doc = `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xml:base="https://example.org/spec/SysML.owl?v=1">
  <rdf:Description rdf:about="#frag"><rdf:value rdf:resource="sibling"/></rdf:Description>
  <rdf:Description rdf:about="/root"><rdf:value rdf:resource="?q"/></rdf:Description>
  <rdf:Description rdf:about=""><rdf:value rdf:resource="urn:x:abs"/></rdf:Description>
</rdf:RDF>`
	triples, err := ReadRDFXML(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"<https://example.org/spec/SysML.owl?v=1#frag> <" + RDFNS + "value> <https://example.org/spec/sibling>",
		"<https://example.org/root> <" + RDFNS + "value> <https://example.org/spec/SysML.owl?q>",
		"<https://example.org/spec/SysML.owl?v=1> <" + RDFNS + "value> <urn:x:abs>",
	}
	for i, w := range want {
		if got := triples[i].String(); got != w {
			t.Errorf("triple %d = %s, want %s", i, got, w)
		}
	}
}

func TestReadRDFXMLKeepsNodeIDsDistinct(t *testing.T) {
	const doc = `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="x:a">
    <rdf:value><rdf:Description><rdf:value>anonymous</rdf:value></rdf:Description></rdf:value>
    <rdf:value rdf:nodeID="b1"/>
    <rdf:value rdf:nodeID="b1"/>
  </rdf:Description>
  <rdf:Description rdf:nodeID="b1"><rdf:value>named</rdf:value></rdf:Description>
</rdf:RDF>`
	triples, err := ReadRDFXML(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	var anonymous, named Node
	for _, tr := range triples {
		if tr.Object.Kind == LiteralNode && tr.Object.Value == "anonymous" {
			anonymous = tr.Subject
		}
		if tr.Object.Kind == LiteralNode && tr.Object.Value == "named" {
			named = tr.Subject
		}
	}
	if anonymous == named || anonymous.Kind != BlankNode || named.Kind != BlankNode {
		t.Fatalf("anonymous node %s and nodeID node %s should be distinct blank nodes", anonymous, named)
	}
	if triples[2].Object != named || triples[3].Object != named {
		t.Errorf("repeated rdf:nodeID=\"b1\" references do not name one node: %s, %s", triples[2].Object, triples[3].Object)
	}
}

func TestReadXMI(t *testing.T) {
	m := testMetamodel(t)
	want := map[string]string{"Element": "KerML/Root/Elements", "Membership": "KerML/Root/Namespaces", "VisibilityKind": "KerML/Root/Namespaces"}
	if len(m.Owner) != len(want) {
		t.Errorf("Owner = %v, want %v", m.Owner, want)
	}
	for name, path := range want {
		if m.Owner[name] != path {
			t.Errorf("Owner[%s] = %q, want %q", name, m.Owner[name], path)
		}
	}
	var paths []string
	for _, p := range m.Packages {
		paths = append(paths, p.Path)
	}
	if got := strings.Join(paths, " "); got != "KerML KerML/Root KerML/Root/Elements KerML/Root/Namespaces" {
		t.Errorf("Packages = %q", got)
	}
	if m.Packages[1].Comment != "The Root layer." || m.Packages[0].URI != "https://www.omg.org/spec/KerML/20240201" {
		t.Errorf("package metadata = %+v", m.Packages[:2])
	}
	dup := &Metamodel{}
	if err := dup.ReadXMI(strings.NewReader(testXMI)); err != nil {
		t.Fatal(err)
	}
	if err := dup.ReadXMI(strings.NewReader(testXMI)); err == nil {
		t.Error("reading the same classes twice did not fail")
	}
}

func TestPartition(t *testing.T) {
	p, err := testMetamodel(t).Partition(testTriples(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Header) != 3 {
		t.Errorf("header has %d triples, want 3", len(p.Header))
	}
	if len(p.Modules) != 2 {
		t.Fatalf("modules = %d, want 2", len(p.Modules))
	}
	el, ns := p.Modules[0], p.Modules[1]
	if el.Path != "KerML/Root/Elements" || ns.Path != "KerML/Root/Namespaces" {
		t.Fatalf("module paths = %s, %s", el.Path, ns.Path)
	}
	if got := strings.Join(el.Terms, " "); got != SysMLNS+"Element "+SysMLNS+"Element_owner" {
		t.Errorf("Elements terms = %q", got)
	}
	if got := strings.Join(ns.Terms, " "); got != SysMLNS+"Membership "+SysMLNS+"Membership_visibility "+SysMLNS+"VisibilityKind" {
		t.Errorf("Namespaces terms = %q", got)
	}
	if len(el.Imports) != 0 || !el.ImportsEcore {
		t.Errorf("Elements imports = %v ecore=%v", el.Imports, el.ImportsEcore)
	}
	if strings.Join(ns.Imports, " ") != "KerML/Root/Elements" || ns.ImportsEcore {
		t.Errorf("Namespaces imports = %v ecore=%v", ns.Imports, ns.ImportsEcore)
	}
	total := len(p.Header)
	for _, m := range p.Modules {
		total += len(m.Triples)
	}
	if src := testTriples(t); total != len(src) {
		t.Errorf("partition holds %d triples, source %d", total, len(src))
	}
	var layers []string
	for _, l := range p.Layers {
		layers = append(layers, l.Path+"["+strings.Join(l.Children, ",")+"]")
	}
	if got := strings.Join(layers, " "); got != "KerML[KerML/Root] KerML/Root[KerML/Root/Elements,KerML/Root/Namespaces]" {
		t.Errorf("layers = %q", got)
	}
	if p.Layers[0].Comment != "The kernel modeling language." {
		t.Errorf("KerML comment = %q", p.Layers[0].Comment)
	}
}

func TestPartitionRejectsUnplacedTerms(t *testing.T) {
	triples := append(testTriples(t), Triple{IRI(SysMLNS + "Orphan"), IRI(RDFNS + "type"), IRI(OWLNS + "Class")})
	if _, err := testMetamodel(t).Partition(triples); err == nil || !strings.Contains(err.Error(), "Orphan") {
		t.Errorf("err = %v, want one naming Orphan", err)
	}
	triples = append(testTriples(t), Triple{IRI(SysMLNS + "Element"), IRI(RDFSNS + "seeAlso"), IRI(SysMLNS + "Missing")})
	if _, err := testMetamodel(t).Partition(triples); err == nil || !strings.Contains(err.Error(), "Missing") {
		t.Errorf("err = %v, want one naming Missing", err)
	}
}

func TestPartitionRejectsBlankNodeCycle(t *testing.T) {
	triples := append(testTriples(t),
		Triple{Blank("c1"), IRI("x:p"), Blank("c2")},
		Triple{Blank("c2"), IRI("x:p"), Blank("c1")},
	)
	done := make(chan error, 1)
	go func() {
		_, err := testMetamodel(t).Partition(triples)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "cycle") {
			t.Errorf("err = %v, want a cycle error", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Partition did not terminate on a blank-node cycle")
	}
}

func TestWriteTurtle(t *testing.T) {
	p, err := testMetamodel(t).Partition(testTriples(t))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := WriteTurtle(p.Modules[1].Triples)
	if err != nil {
		t.Fatal(err)
	}
	want := `@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .

sysml:Membership
    a owl:Class ;
    rdfs:subClassOf sysml:Element .

sysml:Membership_visibility
    a owl:DatatypeProperty ;
    rdfs:domain sysml:Membership ;
    rdfs:range sysml:VisibilityKind .

sysml:VisibilityKind
    a rdfs:Datatype ;
    owl:equivalentClass [
        a rdfs:Datatype ;
        owl:oneOf [
            a rdf:List ;
            rdf:first "private" ;
            rdf:rest [
                a rdf:List ;
                rdf:first "public" ;
                rdf:rest rdf:nil
            ]
        ]
    ] .
`
	if string(doc) != want {
		t.Errorf("turtle:\n%s\nwant:\n%s", doc, want)
	}
	doc, err = WriteTurtle(p.Modules[0].Triples)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"ecore:isOrdered false", `owl:maxCardinality "1"^^xsd:nonNegativeInteger`,
		"rdfs:comment \"\"\"An <code>Element</code> is a \\\"constituent\\\".\nSecond line.\"\"\" .",
	} {
		if !strings.Contains(string(doc), want) {
			t.Errorf("Elements turtle lacks %q:\n%s", want, doc)
		}
	}
}

func TestWriteTurtleCollections(t *testing.T) {
	doc, err := WriteTurtle([]Triple{
		{IRI("x:a"), IRI("x:p"), Blank("l1")},
		{Blank("l1"), IRI(RDFNS + "first"), Literal("1", XSDNS+"integer")},
		{Blank("l1"), IRI(RDFNS + "rest"), Blank("l2")},
		{Blank("l2"), IRI(RDFNS + "first"), IRI("x:b")},
		{Blank("l2"), IRI(RDFNS + "rest"), IRI(RDFNS + "nil")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := "<x:a>\n    <x:p> ( 1 <x:b> ) .\n"; !strings.HasSuffix(string(doc), want) {
		t.Errorf("turtle:\n%s\nwant suffix:\n%s", doc, want)
	}
	_, err = WriteTurtle([]Triple{{Blank("orphan"), IRI("x:p"), IRI("x:b")}})
	if err == nil {
		t.Error("orphan blank node did not fail")
	}
	_, err = WriteTurtle([]Triple{{IRI("x:a"), IRI("x:p"), Blank("s")}, {IRI("x:b"), IRI("x:p"), Blank("s")}, {Blank("s"), IRI("x:q"), IRI("x:c")}})
	if err == nil {
		t.Error("shared blank node did not fail")
	}
}

func TestWriteTurtleKeepsRepeatedListPredicates(t *testing.T) {
	// A cell with two rdf:first or rdf:rest values is not a well-formed list;
	// it is written as a blank node so no triple is lost.
	doc, err := WriteTurtle([]Triple{
		{IRI("x:a"), IRI("x:p"), Blank("l1")},
		{Blank("l1"), IRI(RDFNS + "first"), IRI("x:b")},
		{Blank("l1"), IRI(RDFNS + "first"), IRI("x:c")},
		{Blank("l1"), IRI(RDFNS + "rest"), IRI(RDFNS + "nil")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := "<x:p> [\n        rdf:first <x:b>, <x:c> ;\n        rdf:rest rdf:nil\n    ] .\n"; !strings.HasSuffix(string(doc), want) {
		t.Errorf("turtle:\n%s\nwant suffix:\n%s", doc, want)
	}
	doc, err = WriteTurtle([]Triple{
		{IRI("x:a"), IRI("x:p"), Blank("l1")},
		{Blank("l1"), IRI(RDFNS + "first"), IRI("x:b")},
		{Blank("l1"), IRI(RDFNS + "rest"), IRI(RDFNS + "nil")},
		{Blank("l1"), IRI(RDFNS + "rest"), Blank("l2")},
		{Blank("l2"), IRI(RDFNS + "first"), IRI("x:c")},
		{Blank("l2"), IRI(RDFNS + "rest"), IRI(RDFNS + "nil")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := "<x:p> [\n        rdf:first <x:b> ;\n        rdf:rest rdf:nil, ( <x:c> )\n    ] .\n"; !strings.HasSuffix(string(doc), want) {
		t.Errorf("turtle:\n%s\nwant suffix:\n%s", doc, want)
	}
}

func TestWriteTurtleMultilineLiterals(t *testing.T) {
	for lexical, want := range map[string]string{
		"a\nb":            "\"\"\"a\nb\"\"\"",
		"say \"hi\"\n":    "\"\"\"say \\\"hi\\\"\n\"\"\"",
		"x\n\"\"\"":       "\"\"\"x\n\\\"\\\"\\\"\"\"\"",
		"\"\"\"\nq\"":     "\"\"\"\\\"\\\"\\\"\nq\\\"\"\"\"",
		"back\\slash\n\"": "\"\"\"back\\\\slash\n\\\"\"\"\"",
	} {
		doc, err := WriteTurtle([]Triple{{IRI("x:a"), IRI("x:p"), Literal(lexical, "")}})
		if err != nil {
			t.Fatal(err)
		}
		if want := "<x:p> " + want + " .\n"; !strings.HasSuffix(string(doc), want) {
			t.Errorf("%q: got %s want suffix %s", lexical, doc, want)
		}
	}
}

func TestWriteTurtleRejectsListCycle(t *testing.T) {
	// l2 -> l3 -> l2: the cycle does not pass through the head l1.
	done := make(chan error, 1)
	go func() {
		_, err := WriteTurtle([]Triple{
			{IRI("x:a"), IRI("x:p"), Blank("l1")},
			{Blank("l1"), IRI(RDFNS + "first"), IRI("x:b")},
			{Blank("l1"), IRI(RDFNS + "rest"), Blank("l2")},
			{Blank("l2"), IRI(RDFNS + "first"), IRI("x:b")},
			{Blank("l2"), IRI(RDFNS + "rest"), Blank("l3")},
			{Blank("l3"), IRI(RDFNS + "first"), IRI("x:b")},
			{Blank("l3"), IRI(RDFNS + "rest"), Blank("l2")},
		})
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Error("cyclic list did not fail")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("WriteTurtle did not terminate on a cyclic list")
	}
}

func TestGenerateAndCheck(t *testing.T) {
	p, err := testMetamodel(t).Partition(testTriples(t))
	if err != nil {
		t.Fatal(err)
	}
	src := Sources{OntologyRepo: "https://example.org/onto", OntologyCommit: "abc123", XMIRelease: "20240201"}
	if _, err := Generate(p, "https://example.org/modules", src); err == nil {
		t.Error("base IRI without trailing slash accepted")
	}
	outputs, err := Generate(p, "https://example.org/modules/", src)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	byPath := make(map[string]string)
	for _, o := range outputs {
		paths = append(paths, o.Path)
		byPath[o.Path] = string(o.Content)
	}
	if got := strings.Join(paths, " "); got != "KerML.ttl KerML/Root.ttl KerML/Root/Elements.ttl KerML/Root/Namespaces.ttl VERSION catalog-v001.xml catalog.tsv" {
		t.Errorf("outputs = %q", got)
	}
	ns := byPath["KerML/Root/Namespaces.ttl"]
	for _, want := range []string{
		"<https://example.org/modules/KerML/Root/Namespaces>\n    a owl:Ontology ;\n    owl:imports <https://example.org/modules/KerML/Root/Elements> ;",
		`owl:versionInfo "abc123+xmi20240201"`,
		`dc:source "OMG KerML/SysML XMI 20240201", "https://example.org/onto@abc123"`,
	} {
		if !strings.Contains(ns, want) {
			t.Errorf("Namespaces.ttl lacks %q:\n%s", want, ns)
		}
	}
	if el := byPath["KerML/Root/Elements.ttl"]; !strings.Contains(el, "owl:imports <http://www.eclipse.org/emf/2002/Ecore> ;") {
		t.Errorf("Elements.ttl does not import Ecore:\n%s", el)
	}
	if root := byPath["KerML/Root.ttl"]; !strings.Contains(root, "owl:imports <https://example.org/modules/KerML/Root/Elements>, <https://example.org/modules/KerML/Root/Namespaces> ;") ||
		!strings.Contains(root, `dc:description "The Root layer."`) {
		t.Errorf("Root.ttl:\n%s", root)
	}
	if got := byPath["catalog.tsv"]; got != "term\tmodule\nElement\tKerML/Root/Elements\nElement_owner\tKerML/Root/Elements\nMembership\tKerML/Root/Namespaces\nMembership_visibility\tKerML/Root/Namespaces\nVisibilityKind\tKerML/Root/Namespaces\n" {
		t.Errorf("catalog.tsv:\n%s", got)
	}
	if got := byPath["VERSION"]; !strings.Contains(got, "ontology-commit\tabc123\n") {
		t.Errorf("VERSION:\n%s", got)
	}
	if got := byPath[CatalogFile]; !strings.Contains(got, `<uri name="https://example.org/modules/KerML/Root/Elements" uri="KerML/Root/Elements.ttl"></uri>`) ||
		!strings.Contains(got, `<uri name="https://example.org/modules/KerML" uri="KerML.ttl"></uri>`) {
		t.Errorf("%s:\n%s", CatalogFile, got)
	}

	dir := t.TempDir()
	stale, err := CheckOutputs(dir, outputs)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != len(outputs) || !strings.HasSuffix(stale[0], "(missing)") {
		t.Errorf("empty dir stale = %v", stale)
	}
	if err := WriteOutputs(dir, outputs); err != nil {
		t.Fatal(err)
	}
	if stale, err = CheckOutputs(dir, outputs); err != nil || len(stale) != 0 {
		t.Errorf("fresh dir stale = %v, err = %v", stale, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "KerML", "Root", "Old.ttl"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale, err = CheckOutputs(dir, outputs)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(stale, ";"); got != "KerML/Root/Old.ttl (not generated);VERSION (differs)" {
		t.Errorf("stale = %q", got)
	}
	if err := WriteOutputs(dir, outputs); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "KerML", "Root", "Old.ttl")); !os.IsNotExist(err) {
		t.Errorf("stale module not removed: %v", err)
	}
	if got := loadClosure(t, dir, "KerML.ttl"); len(got) != 4 {
		t.Errorf("closure from KerML.ttl = %v, want all 4 documents", got)
	}

	// A write that fails while staging leaves the previous tree untouched and
	// no staging directory behind.
	bad := append([]Output{{Path: "KerML.ttl/nested.ttl", Content: []byte("x")}}, outputs...)
	if err := WriteOutputs(dir, bad); err == nil {
		t.Fatal("WriteOutputs with an unwritable output did not fail")
	}
	if stale, err := CheckOutputs(dir, outputs); err != nil || len(stale) != 0 {
		t.Errorf("after failed write stale = %v, err = %v", stale, err)
	}
	siblings, err := os.ReadDir(filepath.Dir(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(siblings) != 1 {
		t.Errorf("staging directory left behind: %v", siblings)
	}

	// A swap that fails after the old files have started moving is undone:
	// Blocked.ttl is a non-generated directory, so the staged file cannot
	// take its place.
	if err := os.MkdirAll(filepath.Join(dir, "Blocked.ttl"), 0o750); err != nil {
		t.Fatal(err)
	}
	blocked := append([]Output{{Path: "Blocked.ttl", Content: []byte("x")}}, outputs...)
	if err := WriteOutputs(dir, blocked); err == nil {
		t.Fatal("WriteOutputs onto a blocked destination did not fail")
	}
	if stale, err := CheckOutputs(dir, outputs); err != nil || len(stale) != 0 {
		t.Errorf("after failed swap stale = %v, err = %v", stale, err)
	}
	if fi, err := os.Stat(filepath.Join(dir, "Blocked.ttl")); err != nil || !fi.IsDir() {
		t.Errorf("non-generated entry disturbed: %v, %v", fi, err)
	}
	if siblings, err = os.ReadDir(filepath.Dir(dir)); err != nil || len(siblings) != 1 {
		t.Errorf("staging directory left behind: %v, %v", siblings, err)
	}
}

// TestCommittedModulesResolveOffline opens the committed root ontologies from
// disk and follows every owl:imports through catalog-v001.xml, as an OWL tool
// would, proving the tree loads without network access.
func TestCommittedModulesResolveOffline(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "..", "..", "ontology", "sysmlv2")
	ttl, err := filepath.Glob(filepath.Join(dir, "*", "*", "*.ttl"))
	if err != nil {
		t.Fatal(err)
	}
	leaves := len(ttl)
	if leaves == 0 {
		t.Fatalf("no leaf modules under %s", dir)
	}
	all := make(map[string]bool)
	for _, root := range []string{"KerML.ttl", "SysML.ttl"} {
		for _, doc := range loadClosure(t, dir, root) {
			all[doc] = true
		}
	}
	if want := leaves + 6; len(all) != want {
		t.Errorf("closure of both roots reaches %d documents, want %d (%d leaves + 6 layers)", len(all), want, leaves)
	}
}

// loadClosure resolves start's transitive owl:imports through the XML
// catalog in dir and returns every document reached, failing on an IRI the
// catalog cannot map (Ecore is external and excluded).
func loadClosure(t *testing.T, dir, start string) []string {
	t.Helper()
	catData, err := os.ReadFile(filepath.Join(dir, CatalogFile))
	if err != nil {
		t.Fatal(err)
	}
	var cat oasisCatalog
	if err := xml.Unmarshal(catData, &cat); err != nil {
		t.Fatal(err)
	}
	files := make(map[string]string, len(cat.URIs))
	for _, e := range cat.URIs {
		files[e.Name] = e.URI
	}
	seen := map[string]bool{}
	var visit func(file string)
	visit = func(file string) {
		if seen[file] {
			return
		}
		seen[file] = true
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(file)))
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "owl:imports ") {
				continue
			}
			for _, ref := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(line, "owl:imports "), " ;"), ", ") {
				iri := strings.Trim(ref, "<>")
				if iri == EcoreOntology {
					continue
				}
				target, ok := files[iri]
				if !ok {
					t.Fatalf("%s imports %s, which %s does not map", file, iri, CatalogFile)
				}
				visit(target)
			}
		}
	}
	visit(start)
	docs := make([]string, 0, len(seen))
	for f := range seen {
		docs = append(docs, f)
	}
	sort.Strings(docs)
	return docs
}
