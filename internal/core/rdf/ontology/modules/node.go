// Package modules partitions the Open-MBEE OWL rendering of the SysML v2
// metamodel into one OWL module per package of the normative OMG KerML and
// SysML abstract syntax, and writes the modules as a Turtle tree with an
// import graph and a catalog. cmd/ontology-modules is its command line.
//
// The ontology (sysml2/owl/www.omg.org/spec/SysML.owl) is one file of 172
// classes and 411 properties in one namespace. Its terms are the metaclasses
// of the OMG XMI, which groups them into layers (KerML Root, Core, Kernel;
// SysML Systems) and packages (Elements, Namespaces, Types, Features, Actions,
// States, Requirements, ...). A term goes to the package the XMI declares it
// in: a class or enumeration directly, a property with the class that is its
// rdfs:domain, and an anonymous axiom (a cardinality restriction, an
// enumeration's value list) with the subject it hangs off. Every triple of the
// source lands in exactly one module, and each module imports the modules
// declaring the terms it refers to, so any module's import closure is a
// self-contained ontology.
package modules

import "fmt"

// Well-known namespaces.
const (
	SysMLNS = "https://www.omg.org/spec/SysML#"
	RDFNS   = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	RDFSNS  = "http://www.w3.org/2000/01/rdf-schema#"
	OWLNS   = "http://www.w3.org/2002/07/owl#"
	XSDNS   = "http://www.w3.org/2001/XMLSchema#"
	EcoreNS = "http://www.eclipse.org/emf/2002/Ecore#"
	DCNS    = "http://purl.org/dc/elements/1.1/"

	// EcoreOntology is the ontology SysML.owl imports for Ecore:isOrdered.
	EcoreOntology = "http://www.eclipse.org/emf/2002/Ecore"
)

// NodeKind discriminates the three RDF node kinds an OWL ontology needs.
type NodeKind int

const (
	// IRINode is a named node.
	IRINode NodeKind = iota
	// BlankNode is an anonymous node (a restriction, a list cell).
	BlankNode
	// LiteralNode is a literal, plain or typed.
	LiteralNode
)

// Node is one RDF term.
type Node struct {
	Kind NodeKind
	// Value is the IRI, the blank node label, or the literal's lexical form.
	Value string
	// Datatype is the literal's datatype IRI; empty for a plain literal.
	Datatype string
}

// IRI returns a named node.
func IRI(iri string) Node { return Node{Kind: IRINode, Value: iri} }

// Blank returns an anonymous node with the given label.
func Blank(label string) Node { return Node{Kind: BlankNode, Value: label} }

// Literal returns a literal, typed when datatype is not empty.
func Literal(value, datatype string) Node {
	return Node{Kind: LiteralNode, Value: value, Datatype: datatype}
}

func (n Node) String() string {
	switch n.Kind {
	case BlankNode:
		return "_:" + n.Value
	case LiteralNode:
		if n.Datatype != "" {
			return fmt.Sprintf("%q^^<%s>", n.Value, n.Datatype)
		}
		return fmt.Sprintf("%q", n.Value)
	default:
		return "<" + n.Value + ">"
	}
}

// Triple is one statement.
type Triple struct {
	Subject, Predicate, Object Node
}

func (t Triple) String() string {
	return t.Subject.String() + " " + t.Predicate.String() + " " + t.Object.String()
}
