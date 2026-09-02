package export

import (
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// Stored notation is written back verbatim only while it still states the
// graph: the graph is authoritative, the text is layout. A head is checked by
// spelling (keepsText), an expression by reading it back (textStatesGraph);
// docs/reference/rdf-mapping.md § Limitations defines both.

// sameSpelling reports whether two spellings differ only in what the graph does
// not state: whitespace, notes and comments, name quoting, and a relationship
// clause's symbol against its keyword (`:>` against `subsets`).
func sameSpelling(a, b string) bool {
	return slices.Equal(spellingOf(a), spellingOf(b))
}

type token struct {
	kind lexer.Kind
	text string
}

// clauseKeyword is the keyword each relationship symbol spells in a usage head.
var clauseKeyword = map[lexer.Kind]string{
	lexer.ColonGt:      "subsets",
	lexer.ColonGtGt:    "redefines",
	lexer.ColonColonGt: "references",
}

func spellingOf(text string) []token {
	sf := source.New("<spelling>", []byte(text))
	lx := lexer.New(sf)
	var out []token
	for {
		tok := lx.Next()
		if tok.Kind == lexer.EOF {
			return out
		}
		if tok.IsTrivia() || tok.Kind == lexer.RegularComment {
			continue
		}
		t := token{kind: tok.Kind, text: sf.Text(tok.Span)}
		switch {
		case tok.Kind == lexer.UnrestrictedName:
			if name := strings.Trim(t.text, "'"); lexer.IsIdentifier(name) && !lexer.IsKeyword(name) {
				t = token{kind: lexer.Identifier, text: name}
			}
		case clauseKeyword[tok.Kind] != "":
			t = token{kind: lexer.Keyword, text: clauseKeyword[tok.Kind]}
		case tok.Kind == lexer.Keyword && t.text == "by" && len(out) > 0 &&
			out[len(out)-1].kind == lexer.Keyword && (out[len(out)-1].text == "typed" || out[len(out)-1].text == "defined"):
			out = out[:len(out)-1]
			t = token{kind: lexer.Colon, text: ":"}
		}
		out = append(out, t)
	}
}

// lexesClean reports whether text closes every comment, note, string and
// quoted name it opens; one that does not would swallow what is written after it.
func lexesClean(text string) bool {
	lx := lexer.New(source.New("<spelling>", []byte(text)))
	for {
		tok := lx.Next()
		if tok.Kind == lexer.Error || tok.Unterminated {
			return false
		}
		if tok.Kind == lexer.EOF {
			return true
		}
	}
}

// keepsText reports whether an element kept as source text is written as that
// text: it lexes clean, and the graph states no form to rebuild its head from
// or the head rebuilt from the form is the same spelling as the text. A form
// the graph states but cannot rebuild is reported from the graph, not papered
// over by the text.
func (d *decoder) keepsText(el *element, text string) bool {
	if !lexesClean(text) {
		return false
	}
	if !d.graph.HasProperty(rdf.IRI(el.iri), rdf.OpenSysML+xEndForm) {
		return true
	}
	head, err := d.head(el)
	if err != nil {
		return false
	}
	return sameSpelling(text, head+";")
}

// textStatesGraph reports whether an expression node's stored notation states
// exactly the structure the graph carries for it: the text is parsed and mapped
// with the same encoder, and the two graphs must make the same statements about
// the node. A node the graph keeps as text alone is not contradicted by any text.
func (d *decoder) textStatesGraph(node rdf.Term, text, scope string) bool {
	if d.textOnly(node) {
		return true
	}
	sf := source.New("<expression>", []byte(text))
	p := parser.New(sf)
	expr := p.ParseExpression()
	if expr == nil || len(p.Diagnostics) > 0 || p.Offset() != len(text) {
		return false
	}
	stated := &encoder{
		file:     sf,
		graph:    rdf.NewGraph(),
		declared: d.declaredNames(),
		ids:      &identityFacts{},
		subjects: map[string]string{},
	}
	stated.expressionNode(node, scope, expr)
	return d.sameStatements(node, stated.graph, true)
}

// textOnly reports whether the graph states nothing about an expression node
// beyond that it is an expression: no operator, operands, referent or value.
func (d *decoder) textOnly(node rdf.Term) bool {
	for _, predicate := range d.graph.Predicates(node) {
		if layoutPredicate(predicate, true) {
			continue
		}
		if predicate != rdf.RDFType || d.graph.Type(node) != rdf.SysMLTerm(mExpression).Value {
			return false
		}
	}
	return true
}

// layoutPredicate reports whether a predicate carries layout or identity rather
// than structure: the notation itself, the id, and — on the node being checked,
// whose position its owner states — the position among its siblings.
func layoutPredicate(predicate string, root bool) bool {
	switch predicate {
	case rdf.OpenSysML + xSourceText, rdf.SysML + pElementID:
		return true
	case rdf.OpenSysML + xArgumentIndex, rdf.OpenSysML + xEndIndex, rdf.OpenSysML + xEndRole:
		return root
	}
	return false
}

// declaredNames is the set of qualified names the graph declares, which is what
// a name in an expression resolves against, as it did when the graph was written.
func (d *decoder) declaredNames() map[string]bool {
	if d.declared == nil {
		d.declared = make(map[string]bool, len(d.byIRI))
		for _, el := range d.byIRI {
			if el.qname != "" {
				d.declared[el.qname] = true
			}
		}
	}
	return d.declared
}

// sameStatements reports whether the graph and stated make the same structural
// statements about an expression node, nested nodes included. Objects are
// compared as sets: RDF states no order among the objects of one property.
func (d *decoder) sameStatements(node rdf.Term, stated *rdf.Graph, root bool) bool {
	predicates := map[string]bool{}
	for _, predicate := range d.graph.Predicates(node) {
		predicates[predicate] = true
	}
	for _, predicate := range stated.Predicates(node) {
		predicates[predicate] = true
	}
	for predicate := range predicates {
		if layoutPredicate(predicate, root) {
			continue
		}
		if !d.sameObjects(d.graph.Objects(node, predicate), stated.Objects(node, predicate), stated) {
			return false
		}
	}
	return true
}

// sameObjects reports whether two object lists match one to one, in any order.
func (d *decoder) sameObjects(have, want []rdf.Term, stated *rdf.Graph) bool {
	if len(have) != len(want) {
		return false
	}
	matched := make([]bool, len(want))
	for _, h := range have {
		found := false
		for i, w := range want {
			if !matched[i] && d.sameObject(h, w, stated) {
				matched[i], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// sameObject compares one object the graph states with the one the text
// states: a nested node recursively, an element by the element it names, and
// anything else by value.
func (d *decoder) sameObject(have, want rdf.Term, stated *rdf.Graph) bool {
	if have.IsLiteral() || want.IsLiteral() {
		return have == want
	}
	if d.isExpressionNode(have) {
		return have.Value == want.Value && d.sameStatements(have, stated, false)
	}
	if target, err := d.referencedElement(have.Value); err == nil {
		named, ok := rdf.DecodeElementID(rdf.LocalName(want.Value))
		return ok && named == target.qname
	}
	return have.Value == want.Value
}
