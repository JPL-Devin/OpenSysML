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
	// Systemica namespaces the few properties this tool needs that the SysML
	// metamodel does not define: declaration order, and the source text of
	// expressions. They are namespaced separately so a consumer can tell our
	// additions from the standard vocabulary and ignore them.
	Systemica = "urn:systemica:sysml:"
)

// RDFType is the rdf:type predicate IRI.
const RDFType = RDFNS + "type"

// DefaultPrefixes are the prefix bindings written on every serialized graph.
var DefaultPrefixes = map[string]string{
	"rdf":   RDFNS,
	"xsd":   XSD,
	"sysml": SysML,
	"elmt":  Element,
	"sysx":  Systemica,
}

// ElementIRI returns the element IRI for a fully-qualified SysML name.
// Qualified names are kept legible (`urn:sysmlv2:element:Demo::Vehicle`) and
// only the characters an IRI cannot carry are percent-escaped, so the IRI is
// deterministic and reversible.
func ElementIRI(qualifiedName string) Term {
	return IRI(Element + escapeIRIPath(qualifiedName))
}

// QualifiedNameOf reverses ElementIRI, returning the qualified name an element
// IRI encodes and whether iri is in the element namespace at all.
func QualifiedNameOf(iri string) (string, bool) {
	if !strings.HasPrefix(iri, Element) {
		return "", false
	}
	return unescapeIRIPath(strings.TrimPrefix(iri, Element)), true
}

// SysMLTerm returns the IRI of a name in the SysML vocabulary, used for both
// metaclasses (`sysml:PartUsage`) and properties (`sysml:declaredName`).
func SysMLTerm(name string) Term { return IRI(SysML + name) }

// SystemicaTerm returns the IRI of a name in the Systemica extension namespace.
func SystemicaTerm(name string) Term { return IRI(Systemica + name) }

// LocalName returns the part of iri after the last '#', '/' or ':' — the
// metaclass or property name for a vocabulary IRI. An element IRI returns the
// whole qualified name it encodes, since the segments of a qualified name are
// one name and not a path.
func LocalName(iri string) string {
	if qname, ok := QualifiedNameOf(iri); ok {
		return qname
	}
	if cut := strings.LastIndexAny(iri, "#/"); cut >= 0 {
		return iri[cut+1:]
	}
	if cut := strings.LastIndex(iri, ":"); cut >= 0 {
		return iri[cut+1:]
	}
	return iri
}

const iriUnreserved = "-._~:@!$&'()*+,;=" // safe in an IRI path segment

func escapeIRIPath(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case strings.ContainsRune(iriUnreserved, r):
			b.WriteRune(r)
		case r == '%':
			b.WriteString("%25")
		default:
			for _, octet := range []byte(string(r)) {
				b.WriteString("%")
				const hex = "0123456789ABCDEF"
				b.WriteByte(hex[octet>>4])
				b.WriteByte(hex[octet&0x0f])
			}
		}
	}
	return b.String()
}

func unescapeIRIPath(value string) string {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] == '%' && i+2 < len(value) {
			hi, ok1 := hexDigit(value[i+1])
			lo, ok2 := hexDigit(value[i+2])
			if ok1 && ok2 {
				b.WriteByte(hi<<4 | lo)
				i += 2
				continue
			}
		}
		b.WriteByte(value[i])
	}
	return b.String()
}

func hexDigit(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
