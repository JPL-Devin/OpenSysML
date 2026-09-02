package modules

import (
	"fmt"
	"sort"
	"strings"
)

// prefixes are the bindings a module document may use, in the order written.
var prefixes = []struct{ prefix, ns string }{
	{"sysml", SysMLNS},
	{"owl", OWLNS},
	{"rdf", RDFNS},
	{"rdfs", RDFSNS},
	{"xsd", XSDNS},
	{"ecore", EcoreNS},
	{"dc", DCNS},
}

// predicateOrder is the order predicates are written under a subject; any
// other predicate follows, alphabetically.
var predicateOrder = []string{
	RDFNS + "type",
	OWLNS + "imports",
	OWLNS + "versionInfo",
	DCNS + "title",
	DCNS + "description",
	DCNS + "source",
	RDFSNS + "subClassOf",
	RDFSNS + "subPropertyOf",
	OWLNS + "inverseOf",
	OWLNS + "equivalentClass",
	RDFSNS + "domain",
	RDFSNS + "range",
	EcoreNS + "isOrdered",
	OWLNS + "onProperty",
	OWLNS + "cardinality",
	OWLNS + "minCardinality",
	OWLNS + "maxCardinality",
	OWLNS + "oneOf",
	RDFNS + "first",
	RDFNS + "rest",
	RDFSNS + "label",
	RDFSNS + "comment",
	RDFSNS + "seeAlso",
}

// WriteTurtle serializes triples as a Turtle document: the prefixes the
// document uses, then one block per named subject in IRI order, with the
// ontology declaration first. Blank nodes are written inline where they are
// referred to, bare rdf:List chains as collections.
func WriteTurtle(triples []Triple) ([]byte, error) {
	w := &turtleWriter{
		bySubject: make(map[Node][]Triple),
		used:      make(map[string]bool),
		written:   make(map[string]bool),
	}
	var subjects []Node
	for _, t := range triples {
		if _, seen := w.bySubject[t.Subject]; !seen && t.Subject.Kind == IRINode {
			subjects = append(subjects, t.Subject)
		}
		w.bySubject[t.Subject] = append(w.bySubject[t.Subject], t)
	}
	sort.Slice(subjects, func(i, j int) bool {
		oi, oj := w.isOntology(subjects[i]), w.isOntology(subjects[j])
		if oi != oj {
			return oi
		}
		return subjects[i].Value < subjects[j].Value
	})
	var body strings.Builder
	for _, s := range subjects {
		body.WriteString(w.term(s))
		if err := w.predicateList(&body, s, 1); err != nil {
			return nil, err
		}
		body.WriteString(" .\n\n")
	}
	for label := range w.bySubject {
		if label.Kind == BlankNode && !w.written[label.Value] {
			return nil, fmt.Errorf("turtle: blank node %s is the object of no triple", label)
		}
	}
	var out strings.Builder
	for _, p := range prefixes {
		if w.used[p.prefix] {
			fmt.Fprintf(&out, "@prefix %s: <%s> .\n", p.prefix, p.ns)
		}
	}
	out.WriteString("\n")
	out.WriteString(strings.TrimRight(body.String(), "\n"))
	out.WriteString("\n")
	return []byte(out.String()), nil
}

type turtleWriter struct {
	bySubject map[Node][]Triple
	used      map[string]bool
	written   map[string]bool
}

func (w *turtleWriter) isOntology(s Node) bool {
	for _, t := range w.bySubject[s] {
		if t.Predicate.Value == RDFNS+"type" && t.Object.Value == OWLNS+"Ontology" {
			return true
		}
	}
	return false
}

// predicateList writes " pred obj, obj ;\n pred obj" for a subject, indented
// by depth; the caller writes the subject before and the terminator after.
func (w *turtleWriter) predicateList(b *strings.Builder, s Node, depth int) error {
	triples := w.bySubject[s]
	groups := make(map[Node][]Node)
	var preds []Node
	for _, t := range triples {
		if _, seen := groups[t.Predicate]; !seen {
			preds = append(preds, t.Predicate)
		}
		groups[t.Predicate] = append(groups[t.Predicate], t.Object)
	}
	sort.Slice(preds, func(i, j int) bool {
		ri, rj := predicateRank(preds[i].Value), predicateRank(preds[j].Value)
		if ri != rj {
			return ri < rj
		}
		return preds[i].Value < preds[j].Value
	})
	indent := strings.Repeat("    ", depth)
	for i, p := range preds {
		objects := groups[p]
		sort.SliceStable(objects, func(i, j int) bool {
			if objects[i].Kind != objects[j].Kind {
				return objects[i].Kind < objects[j].Kind
			}
			if objects[i].Kind == BlankNode {
				return false
			}
			return objects[i].Value < objects[j].Value
		})
		if i > 0 {
			b.WriteString(" ;")
		}
		b.WriteString("\n" + indent + w.predicate(p) + " ")
		for j, o := range objects {
			if j > 0 {
				b.WriteString(", ")
			}
			if err := w.object(b, o, depth); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *turtleWriter) object(b *strings.Builder, o Node, depth int) error {
	if o.Kind != BlankNode {
		b.WriteString(w.term(o))
		return nil
	}
	if w.written[o.Value] {
		return fmt.Errorf("turtle: blank node %s is referred to twice", o)
	}
	w.written[o.Value] = true
	if items, ok := w.list(o); ok {
		b.WriteString("( ")
		for _, item := range items {
			if err := w.object(b, item, depth); err != nil {
				return err
			}
			b.WriteString(" ")
		}
		b.WriteString(")")
		return nil
	}
	b.WriteString("[")
	if err := w.predicateList(b, o, depth+1); err != nil {
		return err
	}
	b.WriteString("\n" + strings.Repeat("    ", depth) + "]")
	return nil
}

// list returns the items of the rdf:List starting at head when every cell
// states exactly rdf:first and rdf:rest, ending at rdf:nil. Cells that also
// state rdf:type rdf:List are written as blank nodes so no triple is lost.
func (w *turtleWriter) list(head Node) ([]Node, bool) {
	var items []Node
	var cells []Node
	visited := map[string]bool{head.Value: true}
	cell := head
	for {
		var first, rest Node
		var hasFirst, hasRest bool
		for _, t := range w.bySubject[cell] {
			switch t.Predicate.Value {
			case RDFNS + "first":
				first, hasFirst = t.Object, true
			case RDFNS + "rest":
				rest, hasRest = t.Object, true
			default:
				return nil, false
			}
		}
		if !hasFirst || !hasRest {
			return nil, false
		}
		items = append(items, first)
		if rest.Kind == IRINode && rest.Value == RDFNS+"nil" {
			for _, c := range cells {
				w.written[c.Value] = true
			}
			return items, true
		}
		if rest.Kind != BlankNode || w.written[rest.Value] || visited[rest.Value] {
			return nil, false
		}
		visited[rest.Value] = true
		cells = append(cells, rest)
		cell = rest
	}
}

func (w *turtleWriter) predicate(p Node) string {
	if p.Value == RDFNS+"type" {
		return "a"
	}
	return w.term(p)
}

func (w *turtleWriter) term(n Node) string {
	switch n.Kind {
	case LiteralNode:
		return w.literal(n)
	case BlankNode:
		return "_:" + n.Value
	}
	for _, p := range prefixes {
		if local, ok := strings.CutPrefix(n.Value, p.ns); ok && isLocalName(local) {
			w.used[p.prefix] = true
			return p.prefix + ":" + local
		}
	}
	return "<" + n.Value + ">"
}

func (w *turtleWriter) literal(n Node) string {
	lexical := n.Value
	var s string
	if strings.ContainsAny(lexical, "\n\r") {
		escaped := strings.NewReplacer(`\`, `\\`, `"""`, `\"\"\"`).Replace(lexical)
		if strings.HasSuffix(escaped, `"`) {
			escaped = escaped[:len(escaped)-1] + `\"`
		}
		s = `"""` + escaped + `"""`
	} else {
		s = `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\t", `\t`).Replace(lexical) + `"`
	}
	switch n.Datatype {
	case "":
		return s
	case XSDNS + "boolean":
		if lexical == "true" || lexical == "false" {
			return lexical
		}
	case XSDNS + "integer":
		if isInteger(lexical) {
			return lexical
		}
	}
	return s + "^^" + w.term(IRI(n.Datatype))
}

func isInteger(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if i == 0 && (c == '-' || c == '+') && len(s) > 1 {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// isLocalName reports whether local can follow a prefix in Turtle without
// escaping: letters, digits, '_' and '-', not starting with a digit or '-'.
func isLocalName(local string) bool {
	if local == "" {
		return false
	}
	for i, c := range local {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_':
		case c >= '0' && c <= '9', c == '-':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func predicateRank(iri string) int {
	for i, p := range predicateOrder {
		if p == iri {
			return i
		}
	}
	return len(predicateOrder)
}
