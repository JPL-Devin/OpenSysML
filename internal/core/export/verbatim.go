package export

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// Source text is printed only while it states what the graph states: the
// candidate notation is converted back and each disagreement demotes its text.

// render prints the roots from source text where it agrees with the graph and
// canonically where it does not; every pass demotes more text or is the last.
func (d *decoder) render(roots []*element) (string, error) {
	for {
		d.printed = map[*element]bool{}
		d.rebuilt = map[*element]bool{}
		d.usedExpr = map[string]bool{}
		if err := d.resolveExpressions(); err != nil {
			return "", err
		}
		var b strings.Builder
		for _, root := range roots {
			if err := d.print(&b, root, 0); err != nil {
				return "", err
			}
		}
		if len(d.printed)+len(d.usedExpr) == 0 {
			return b.String(), nil
		}
		before := len(d.demoted) + len(d.demotedExpr)
		d.demoteStale(b.String())
		if len(d.demoted)+len(d.demotedExpr) == before {
			return b.String(), nil
		}
	}
}

// verbatim returns the source text an element is printed as, if it carries
// one it has not been demoted from.
func (d *decoder) verbatim(el *element) (string, bool) {
	if d.demoted[el] {
		return "", false
	}
	return d.stringOf(el, rdf.OpenSysML+xSourceText)
}

// expressionText returns the source text an expression node is written as, if
// it carries one it has not been demoted from.
func (d *decoder) expressionText(node rdf.Term) (string, bool) {
	if d.demotedExpr[node.Value] {
		return "", false
	}
	text, ok := d.graph.Lexical(node, rdf.OpenSysML+xSourceText)
	if ok && text != "" {
		d.usedExpr[node.Value] = true
	}
	return text, ok && text != ""
}

// demoteStale demotes whatever verbatim text contradicts the graph; notation
// that does not convert, or whose disagreement cannot be placed, demotes all.
func (d *decoder) demoteStale(notation string) {
	file := source.New("<converted>.sysml", []byte(notation))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		d.demoteAll()
		return
	}
	check, err := ToRDF(file, root)
	if err != nil {
		d.demoteAll()
		return
	}
	for _, t := range disagreeingTriples(d.graph, check) {
		// A reference to an element only one graph has disagrees about that
		// element's identity, not about the element referring to it.
		var el *element
		var expr string
		if t.Object.IsIRI() && d.graph.HasProperty(t.Object, rdf.RDFType) != check.HasProperty(t.Object, rdf.RDFType) {
			el, expr = d.blame(t.Object.Value, check)
		}
		if el == nil && expr == "" {
			el, expr = d.blame(t.Subject.Value, check)
		}
		if el == nil && expr == "" && t.Object.IsIRI() {
			el, expr = d.blame(t.Object.Value, check)
		}
		// One landing on notation already rebuilt is the graph's own, not its text's.
		switch {
		case el != nil && d.printed[el]:
			d.demoted[el] = true
		case expr != "":
			d.demotedExpr[expr] = true
		case el == nil:
			d.demoteAll()
			return
		}
	}
}

func (d *decoder) demoteAll() {
	for el := range d.printed {
		d.demoted[el] = true
	}
	for iri := range d.usedExpr {
		d.demotedExpr[iri] = true
	}
}

// disagreeingTriples lists the structural triples only one of the two graphs
// states; source text differs from canonical notation by design and is skipped.
func disagreeingTriples(graph, check *rdf.Graph) []rdf.Triple {
	var out []rdf.Triple
	structural := func(t rdf.Triple) bool {
		return t.Predicate.Value != rdf.OpenSysML+xSourceText && t.Predicate.Value != rdf.OpenSysML+xSourceTail
	}
	for _, t := range graph.Triples() {
		if structural(t) && !check.Has(t) {
			out = append(out, t)
		}
	}
	for _, t := range check.Triples() {
		if structural(t) && !graph.Has(t) {
			out = append(out, t)
		}
	}
	return out
}

// blame returns the nearest verbatim element a disagreement over iri falls
// under, or else the outermost expression node written from its text.
func (d *decoder) blame(iri string, check *rdf.Graph) (*element, string) {
	var expr string
	visited := map[string]bool{}
	for iri != "" && !visited[iri] {
		visited[iri] = true
		if el, ok := d.byIRI[iri]; ok {
			return d.nearestVerbatim(el), expr
		}
		if d.usedExpr[iri] {
			expr = iri
		}
		holder := holderOf(d.graph, iri)
		if holder == "" {
			holder = holderOf(check, iri)
			if owner, ok := d.byIRI[holder]; ok && check.HasProperty(rdf.IRI(iri), rdf.SysML+pQualifiedName) {
				return d.verbatimMemberAt(owner, intOf(check, rdf.IRI(iri), rdf.OpenSysML+xMemberIndex)), ""
			}
		}
		iri = holder
	}
	return nil, expr
}

// nearestVerbatim returns the element whose notation a disagreement over el
// falls in: the nearest of el and its owners that was written at all. One
// already rebuilt canonically is returned as is, so the blame stops there.
func (d *decoder) nearestVerbatim(el *element) *element {
	for x := el; x != nil; x = x.owner {
		if d.printed[x] || d.rebuilt[x] {
			return x
		}
	}
	return nil
}

// verbatimMemberAt returns the verbatim member of owner at index, or else the
// nearest verbatim owner.
func (d *decoder) verbatimMemberAt(owner *element, index int) *element {
	for _, child := range owner.children {
		if child.memberIndex == index && (d.printed[child] || d.rebuilt[child]) {
			return child
		}
	}
	return d.nearestVerbatim(owner)
}

// holderOf returns the subject a node hangs off: the namespace owning an
// element, or the subject that points at a membership or expression node.
func holderOf(g *rdf.Graph, iri string) string {
	subject := rdf.IRI(iri)
	if g.HasProperty(subject, rdf.SysML+pQualifiedName) {
		if owner, ok := g.Object(subject, rdf.SysML+pOwningNamespace); ok && owner.IsIRI() {
			return owner.Value
		}
		return ""
	}
	if !g.HasProperty(subject, rdf.RDFType) {
		return ""
	}
	for _, t := range g.Triples() {
		if t.Object.IsIRI() && t.Object.Value == iri && t.Subject.Value != iri {
			return t.Subject.Value
		}
	}
	return ""
}
