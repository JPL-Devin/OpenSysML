package rdf

import "strings"

// Namespace IRIs. The SysML vocabulary, element base and prefix labels match
// the ones the Flexo MMS SysML v2 service writes into its triplestore
// (Namespaces.kt), so a graph produced here loads into that service and a graph
// read out of it converts back to source here.
const (
	// RDFNS is the RDF syntax namespace.
	RDFNS = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	// XSD is the XML Schema datatype namespace.
	XSD = "http://www.w3.org/2001/XMLSchema#"
	// SysML is the OMG SysML vocabulary namespace, used for metaclasses and
	// metamodel properties.
	SysML = "https://www.omg.org/spec/SysML#"
	// Element is the base IRI for the elements of a converted model.
	Element = "urn:sysmlv2:element:"
	// OpenSysML namespaces the few properties this tool needs that the SysML
	// metamodel does not define: declaration order, and the source text of
	// expressions. They are namespaced separately so a consumer can tell our
	// additions from the standard vocabulary and ignore them.
	OpenSysML = "urn:opensysml:sysml:"
	// Expression is the base IRI for the nodes of an expression graph: the
	// subexpressions of a value, bound, guard or condition, which are elements
	// of the abstract syntax without a qualified name of their own.
	Expression = "urn:opensysml:expr:"
	// LegacyExtension is the extension namespace this tool wrote before the
	// project was renamed. It is recognized only so a graph carrying it can be
	// refused rather than read as if its properties were absent.
	LegacyExtension = "urn:systemica:sysml:"
)

// RDFType is the rdf:type predicate IRI.
const RDFType = RDFNS + "type"

// DefaultPrefixes are the prefix bindings written on every serialized graph.
var DefaultPrefixes = map[string]string{
	"rdf":   RDFNS,
	"xsd":   XSD,
	"sysml": SysML,
	"elmt":  Element,
	"sysx":  OpenSysML,
}

// ElementIRI returns the element IRI for a fully-qualified SysML name: the
// element namespace followed by EncodeElementID of the name, so the id after
// the last ':' is readable, deterministic, and valid for a consumer that
// restricts ids to [A-Za-z0-9_-]+.
func ElementIRI(qualifiedName string) Term {
	return IRI(Element + EncodeElementID(qualifiedName))
}

// OwningMembershipIRI returns the IRI of the OwningMembership through which a
// namespace owns the member named by qualifiedName. It is in the element
// namespace, since a membership is an element of the abstract syntax in its own
// right, and its id can never collide with an element's.
func OwningMembershipIRI(qualifiedName string) Term {
	return IRI(Element + OwningMembershipID(qualifiedName))
}

// ExpressionPrefix is the prefix label bound to the expression namespace. It is
// written only on a graph that carries an expression graph.
const ExpressionPrefix = "expr"

// ExpressionIRI returns the IRI of one node of an expression graph: the id of
// the term it hangs off — an element or an outer subexpression — with the
// position it holds, so its identity comes from where it sits in the model
// rather than being minted, and its id is valid where an element id is.
func ExpressionIRI(owner Term, path string) Term {
	return IRI(Expression + ExpressionNodeID(LocalName(owner.Value), path))
}

// SysMLTerm returns the IRI of a name in the SysML vocabulary, used for both
// metaclasses (`sysml:PartUsage`) and properties (`sysml:declaredName`).
func SysMLTerm(name string) Term { return IRI(SysML + name) }

// OpenSysMLTerm returns the IRI of a name in the OpenSysML extension namespace.
func OpenSysMLTerm(name string) Term { return IRI(OpenSysML + name) }

// LocalName returns the part of iri after the last '#', '/' or ':' — the
// metaclass or property name for a vocabulary IRI, or the encoded id for an
// element IRI.
func LocalName(iri string) string {
	if cut := strings.LastIndexAny(iri, "#/"); cut >= 0 {
		return iri[cut+1:]
	}
	if cut := strings.LastIndex(iri, ":"); cut >= 0 {
		return iri[cut+1:]
	}
	return iri
}
