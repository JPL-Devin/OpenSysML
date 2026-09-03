package modules

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	xmiNS = "http://www.omg.org/spec/XMI/20161101"
	umlNS = "http://www.omg.org/spec/UML/20161101"
)

// Package is one package of the OMG abstract syntax, identified by its path
// from the specification root ("KerML/Core/Features").
type Package struct {
	// Path is the slash-joined package names from the root package down.
	Path string
	// Comment is the package's ownedComment body, empty when it has none.
	Comment string
	// URI is the package's declared URI; only the root package has one.
	URI string
}

// Metamodel is the package structure of the normative XMI: which package each
// metaclass and enumeration is declared in.
type Metamodel struct {
	// Packages lists every package in document order.
	Packages []Package
	// Owner maps a metaclass or enumeration name to its package path.
	Owner map[string]string
}

// ReadXMI reads the packages, classes and enumerations of one OMG XMI
// document (KerML.xmi or SysML.xmi) into m, refusing a name m already holds.
func (m *Metamodel) ReadXMI(r io.Reader) error {
	if m.Owner == nil {
		m.Owner = make(map[string]string)
	}
	decoder := xml.NewDecoder(r)
	var path []string
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		switch t := token.(type) {
		case xml.EndElement:
			if isPackageElement(t.Name) && len(path) > 0 {
				path = path[:len(path)-1]
			}
		case xml.StartElement:
			if !isPackageElement(t.Name) {
				if t.Name.Local == "ownedComment" && len(path) > 0 && attr(t, xmiNS, "type") == "uml:Comment" {
					m.setComment(strings.Join(path, "/"), attr(t, "", "body"))
				}
				continue
			}
			name := attr(t, "", "name")
			kind := attr(t, xmiNS, "type")
			if t.Name.Space == umlNS && t.Name.Local == "Package" {
				kind = "uml:Package"
			}
			switch kind {
			case "uml:Package":
				path = append(path, name)
				m.Packages = append(m.Packages, Package{Path: strings.Join(path, "/"), URI: attr(t, "", "URI")})
			case "uml:Class", "uml:Enumeration":
				if len(path) == 0 {
					return fmt.Errorf("xmi: %s %s outside any package", kind, name)
				}
				if prior, dup := m.Owner[name]; dup {
					return fmt.Errorf("xmi: %s declared in both %s and %s", name, prior, strings.Join(path, "/"))
				}
				m.Owner[name] = strings.Join(path, "/")
				if err := decoder.Skip(); err != nil {
					return err
				}
			default:
				if err := decoder.Skip(); err != nil {
					return err
				}
			}
		}
	}
	if len(path) != 0 {
		return errors.New("xmi: unbalanced package elements")
	}
	return nil
}

// isPackageElement reports whether an element may be a package, class or
// enumeration: the root uml:Package or a packagedElement.
func isPackageElement(name xml.Name) bool {
	return (name.Space == umlNS && name.Local == "Package") || name.Local == "packagedElement"
}

func (m *Metamodel) setComment(path, body string) {
	for i := range m.Packages {
		if m.Packages[i].Path == path && m.Packages[i].Comment == "" {
			m.Packages[i].Comment = strings.TrimSpace(body)
		}
	}
}
