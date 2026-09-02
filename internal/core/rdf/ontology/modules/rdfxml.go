package modules

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ReadRDFXML reads an RDF/XML document into triples, in document order. It
// covers the striped syntax an OWL tool writes — typed node elements with
// rdf:about, property elements carrying rdf:resource, a literal (optionally
// with rdf:datatype) or one nested node element, and rdf:Description — and
// refuses what it does not cover (rdf:parseType, rdf:ID, property attributes)
// rather than reading it partially. Blank nodes are labelled b1, b2, ... in
// order of appearance.
func ReadRDFXML(r io.Reader) ([]Triple, error) {
	p := &rdfxmlReader{decoder: xml.NewDecoder(r)}
	for {
		token, err := p.decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Space+start.Name.Local != RDFNS+"RDF" {
			return nil, fmt.Errorf("rdfxml: root element is %s, want rdf:RDF", start.Name.Local)
		}
		p.base = attr(start, "http://www.w3.org/XML/1998/namespace", "base")
		if err := p.nodeElements(start); err != nil {
			return nil, err
		}
	}
	if len(p.triples) == 0 {
		return nil, errors.New("rdfxml: no triples")
	}
	return p.triples, nil
}

type rdfxmlReader struct {
	decoder *xml.Decoder
	base    string
	blanks  int
	triples []Triple
}

func (p *rdfxmlReader) emit(s, pr, o Node) {
	p.triples = append(p.triples, Triple{Subject: s, Predicate: pr, Object: o})
}

// nodeElements reads the node elements that are children of parent until
// parent's end tag.
func (p *rdfxmlReader) nodeElements(parent xml.StartElement) error {
	for {
		token, err := p.decoder.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.EndElement:
			return nil
		case xml.StartElement:
			if _, err := p.nodeElement(t); err != nil {
				return err
			}
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return fmt.Errorf("rdfxml: text %q where a node element was expected in %s",
					strings.TrimSpace(string(t)), parent.Name.Local)
			}
		}
	}
}

// nodeElement reads one node element and its property elements, returning the
// node it describes.
func (p *rdfxmlReader) nodeElement(start xml.StartElement) (Node, error) {
	var subject Node
	for _, a := range start.Attr {
		switch {
		case a.Name.Space == RDFNS && a.Name.Local == "about":
			subject = IRI(p.resolve(a.Value))
		case a.Name.Space == RDFNS && a.Name.Local == "nodeID":
			subject = Blank(a.Value)
		case a.Name.Space == "xmlns" || a.Name.Local == "xmlns":
		default:
			return Node{}, fmt.Errorf("rdfxml: unsupported attribute %s on node element %s",
				a.Name.Local, start.Name.Local)
		}
	}
	if subject.Value == "" {
		p.blanks++
		subject = Blank(fmt.Sprintf("b%d", p.blanks))
	}
	if name := start.Name.Space + start.Name.Local; name != RDFNS+"Description" {
		p.emit(subject, IRI(RDFNS+"type"), IRI(name))
	}
	for {
		token, err := p.decoder.Token()
		if err != nil {
			return Node{}, err
		}
		switch t := token.(type) {
		case xml.EndElement:
			return subject, nil
		case xml.StartElement:
			if err := p.propertyElement(subject, t); err != nil {
				return Node{}, err
			}
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return Node{}, fmt.Errorf("rdfxml: text %q where a property element was expected on %s",
					strings.TrimSpace(string(t)), subject)
			}
		}
	}
}

// propertyElement reads one property element of subject, emitting its triple.
func (p *rdfxmlReader) propertyElement(subject Node, start xml.StartElement) error {
	predicate := IRI(start.Name.Space + start.Name.Local)
	var (
		object   Node
		resolved bool
		datatype string
	)
	for _, a := range start.Attr {
		switch {
		case a.Name.Space == RDFNS && a.Name.Local == "resource":
			object, resolved = IRI(p.resolve(a.Value)), true
		case a.Name.Space == RDFNS && a.Name.Local == "nodeID":
			object, resolved = Blank(a.Value), true
		case a.Name.Space == RDFNS && a.Name.Local == "datatype":
			datatype = a.Value
		default:
			return fmt.Errorf("rdfxml: unsupported attribute %s on property element %s of %s",
				a.Name.Local, start.Name.Local, subject)
		}
	}
	var text strings.Builder
	for {
		token, err := p.decoder.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.EndElement:
			if !resolved {
				object = Literal(text.String(), datatype)
			}
			p.emit(subject, predicate, object)
			return nil
		case xml.CharData:
			text.Write(t)
		case xml.StartElement:
			if resolved {
				return fmt.Errorf("rdfxml: property %s of %s has both a reference and a nested node",
					start.Name.Local, subject)
			}
			if strings.TrimSpace(text.String()) != "" {
				return fmt.Errorf("rdfxml: property %s of %s mixes text and a nested node",
					start.Name.Local, subject)
			}
			nested, err := p.nodeElement(t)
			if err != nil {
				return err
			}
			object, resolved = nested, true
			text.Reset()
		}
	}
}

func (p *rdfxmlReader) resolve(ref string) string {
	if strings.Contains(ref, ":") || p.base == "" {
		return ref
	}
	return p.base + ref
}

func attr(element xml.StartElement, space, local string) string {
	for _, a := range element.Attr {
		if a.Name.Local == local && (a.Name.Space == space || a.Name.Space == "") {
			return a.Value
		}
	}
	return ""
}
