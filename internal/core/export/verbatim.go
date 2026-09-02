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
// not state: whitespace, notes, name quoting, and a relationship clause's
// symbol against its keyword (`:>` against `subsets`).
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
		if tok.IsTrivia() {
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

// keepsText reports whether an element kept as source text is written as that
// text: the graph states no form to rebuild its head from, or the head rebuilt
// from the form is the same spelling as the text.
func (d *decoder) keepsText(el *element, text string) bool {
	if !d.graph.HasProperty(rdf.IRI(el.iri), rdf.OpenSysML+xEndForm) {
		return true
	}
	head, err := d.head(el)
	if err != nil {
		return true
	}
	return sameSpelling(text, head+";")
}

// textStatesGraph reports whether an expression node's stored notation states
// the structure the graph carries for it: the text is parsed and mapped with the
// same encoder, and every statement the graph makes about the node must hold in
// the result. A graph that states no structure is not contradicted by any text.
func (d *decoder) textStatesGraph(node rdf.Term, text, scope string) bool {
	if d.statesNoMore(node, rdf.NewGraph()) {
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
	return d.statesNoMore(node, stated.graph)
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

// statesNoMore reports whether every statement the graph makes about an
// expression node also holds in stated, nested nodes included. Notation, id
// and position are left out: the notation is what is being checked, and the
// id and position follow from where the node sits in its owner.
func (d *decoder) statesNoMore(node rdf.Term, stated *rdf.Graph) bool {
	for _, predicate := range d.graph.Predicates(node) {
		switch predicate {
		case rdf.OpenSysML + xSourceText, rdf.SysML + pElementID,
			rdf.OpenSysML + xArgumentIndex, rdf.OpenSysML + xEndIndex, rdf.OpenSysML + xEndRole:
			continue
		case rdf.RDFType:
			// The generic metaclass states only that the node is an expression.
			if d.graph.Type(node) == rdf.SysMLTerm(mExpression).Value {
				continue
			}
		}
		have := d.graph.Objects(node, predicate)
		want := stated.Objects(node, predicate)
		if len(have) != len(want) {
			return false
		}
		for i := range have {
			if !d.sameObject(have[i], want[i], stated) {
				return false
			}
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
		return have.Value == want.Value && d.statesNoMore(have, stated)
	}
	if target, err := d.referencedElement(have.Value); err == nil {
		named, ok := rdf.DecodeElementID(rdf.LocalName(want.Value))
		return ok && named == target.qname
	}
	return have.Value == want.Value
}
