package modules

import (
	"fmt"
	"sort"
	"strings"
)

// Module is one package of the metamodel as an OWL ontology of its own.
type Module struct {
	// Path is the package path ("KerML/Core/Features"); the module's file is
	// Path + ".ttl" and its IRI is the base IRI + Path.
	Path string
	// Comment is the OMG package comment, when the XMI states one.
	Comment string
	// Triples are the source triples assigned to this module, in source order.
	Triples []Triple
	// Terms are the SysML-namespace IRIs this module declares, sorted.
	Terms []string
	// Imports are the paths of the modules declaring the terms this module
	// refers to but does not declare, sorted; and Ecore when it uses that
	// vocabulary.
	Imports []string
	// ImportsEcore reports whether the module uses the Ecore vocabulary.
	ImportsEcore bool
}

// Layer is a package that holds packages rather than terms: the KerML layers
// Root, Core and Kernel, SysML's Systems, and the two specification roots.
type Layer struct {
	Path    string
	Comment string
	// Children are the paths of the packages and layers directly inside.
	Children []string
}

// Partition is the modularized ontology.
type Partition struct {
	Modules []Module
	Layers  []Layer
	// Header holds the source ontology's own triples (its owl:Ontology
	// declaration), which no module carries.
	Header []Triple
}

// Partition splits the triples of the source ontology by the packages of the
// metamodel. Every triple lands in exactly one module or in the header, and
// an error names any term the metamodel does not place.
func (m *Metamodel) Partition(source []Triple) (*Partition, error) {
	// A blank node belongs to the subject whose property introduced it, and
	// so on transitively (a list cell to the list head to the datatype).
	blankOwner := make(map[string]Node)
	for _, t := range source {
		if t.Object.Kind == BlankNode {
			if _, seen := blankOwner[t.Object.Value]; seen {
				return nil, fmt.Errorf("blank node %s is reached from more than one subject", t.Object)
			}
			blankOwner[t.Object.Value] = t.Subject
		}
	}
	root := func(n Node) (Node, error) {
		visited := map[string]bool{}
		for n.Kind == BlankNode {
			if visited[n.Value] {
				return n, fmt.Errorf("blank node %s is in a cycle reached from no named subject", n)
			}
			visited[n.Value] = true
			owner, ok := blankOwner[n.Value]
			if !ok {
				return n, fmt.Errorf("blank node %s is the object of no triple", n)
			}
			n = owner
		}
		return n, nil
	}

	// The module of a named subject: a class or enumeration from the
	// metamodel directly, a property from its rdfs:domain.
	domain := make(map[string]string)
	isOntology := make(map[string]bool)
	for _, t := range source {
		if t.Subject.Kind != IRINode {
			continue
		}
		switch {
		case t.Predicate.Value == RDFSNS+"domain" && t.Object.Kind == IRINode:
			domain[t.Subject.Value] = t.Object.Value
		case t.Predicate.Value == RDFNS+"type" && t.Object.Value == OWLNS+"Ontology":
			isOntology[t.Subject.Value] = true
		}
	}
	placement := func(iri string) (string, error) {
		local, ok := strings.CutPrefix(iri, SysMLNS)
		if !ok {
			return "", fmt.Errorf("%s is outside the SysML namespace", iri)
		}
		if path, ok := m.Owner[local]; ok {
			return path, nil
		}
		if d, ok := domain[iri]; ok {
			if path, ok := m.Owner[strings.TrimPrefix(d, SysMLNS)]; ok {
				return path, nil
			}
			return "", fmt.Errorf("%s: domain %s is not a class of the metamodel", iri, d)
		}
		return "", fmt.Errorf("%s is neither a class or enumeration of the metamodel nor a property with a domain", iri)
	}

	byPath := make(map[string]*Module)
	terms := make(map[string]map[string]bool)
	var header []Triple
	for _, t := range source {
		subject, err := root(t.Subject)
		if err != nil {
			return nil, err
		}
		if isOntology[subject.Value] {
			header = append(header, t)
			continue
		}
		path, err := placement(subject.Value)
		if err != nil {
			return nil, err
		}
		mod := byPath[path]
		if mod == nil {
			mod = &Module{Path: path}
			byPath[path] = mod
			terms[path] = make(map[string]bool)
		}
		mod.Triples = append(mod.Triples, t)
		terms[path][subject.Value] = true
	}
	if len(byPath) == 0 {
		return nil, fmt.Errorf("no triple of the source is placed by the metamodel")
	}

	declaring := make(map[string]string)
	for path, set := range terms {
		for iri := range set {
			declaring[iri] = path
		}
	}
	p := &Partition{Header: header}
	for _, path := range sortedKeys(byPath) {
		mod := byPath[path]
		mod.Comment = m.comment(path)
		mod.Terms = sortedKeys(terms[path])
		imports := make(map[string]bool)
		for _, t := range mod.Triples {
			for _, n := range []Node{t.Predicate, t.Object} {
				if n.Kind != IRINode {
					continue
				}
				if strings.HasPrefix(n.Value, EcoreNS) {
					mod.ImportsEcore = true
				}
				if !strings.HasPrefix(n.Value, SysMLNS) {
					continue
				}
				other, ok := declaring[n.Value]
				if !ok {
					return nil, fmt.Errorf("%s refers to %s, which no module declares", path, n.Value)
				}
				if other != path {
					imports[other] = true
				}
			}
		}
		mod.Imports = sortedKeys(imports)
		p.Modules = append(p.Modules, *mod)
	}
	p.Layers = m.layers(byPath)
	return p, nil
}

// layers builds the container packages above the modules: each imports its
// direct children, so a layer's import closure is the whole layer.
func (m *Metamodel) layers(modules map[string]*Module) []Layer {
	children := make(map[string]map[string]bool)
	for _, pkg := range m.Packages {
		if _, isModule := modules[pkg.Path]; isModule {
			continue
		}
		if _, ok := children[pkg.Path]; !ok {
			children[pkg.Path] = make(map[string]bool)
		}
	}
	for _, pkg := range m.Packages {
		parent, _, ok := cutLast(pkg.Path)
		if !ok {
			continue
		}
		if set, isLayer := children[parent]; isLayer {
			set[pkg.Path] = true
		}
	}
	var layers []Layer
	for _, path := range sortedKeys(children) {
		if len(children[path]) == 0 {
			continue
		}
		layers = append(layers, Layer{Path: path, Comment: m.comment(path), Children: sortedKeys(children[path])})
	}
	return layers
}

func (m *Metamodel) comment(path string) string {
	for _, pkg := range m.Packages {
		if pkg.Path == path {
			return pkg.Comment
		}
	}
	return ""
}

func cutLast(path string) (parent, name string, ok bool) {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return "", path, false
	}
	return path[:i], path[i+1:], true
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
