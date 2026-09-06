package export

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// endNotation writes the ends of an end-binding head — `connect a to b`,
// `bind a = b`, `first a then b`, `of P from a to b` — from the form the graph
// states. Both halves of the mapping build the text here, so what the encoder
// checks it can rebuild is exactly what the decoder rebuilds.
type endNotation struct {
	form string
	// keyword is a verb written ahead of the ends by a head whose own keyword
	// is the noun form (`connection c connect a to b`), or "" when the head
	// keyword introduces the ends itself.
	keyword string
	ends    []string
	payload string
}

func (n endNotation) text() (string, error) {
	bad := func(why string) error {
		return &UnsupportedError{
			What: fmt.Sprintf("an end-binding head written as sysx:%s %q", xEndForm, n.form),
			Note: why,
		}
	}
	var words []string
	if n.keyword != "" {
		words = append(words, n.keyword)
	}
	switch n.form {
	case formTo:
		if len(n.ends) < 2 {
			return "", bad("it states fewer than the two ends this form connects")
		}
		words = append(words, n.ends[0], "to", strings.Join(n.ends[1:], ", "))
	case formNary:
		if len(n.ends) < 2 {
			return "", bad("it states fewer than the two ends this form connects")
		}
		words = append(words, "("+strings.Join(n.ends, ", ")+")")
	case formSatisfy:
		if len(n.ends) != 1 {
			return "", bad("it states other than the one requirement this form names")
		}
		words = append(words, n.ends[0])
	case formEquals, formFirstThen, formFromTo, formFlowTo:
		if len(n.ends) != 2 {
			return "", bad("it states other than the two ends this form binds")
		}
		switch n.form {
		case formEquals:
			words = append(words, n.ends[0], "=", n.ends[1])
		case formFirstThen:
			words = append(words, n.ends[0], "then", n.ends[1])
		default:
			if n.payload != "" {
				words = append(words, "of", n.payload)
			}
			if n.form == formFromTo {
				words = append(words, "from")
			}
			words = append(words, n.ends[0], "to", n.ends[1])
		}
	default:
		return "", bad("this mapping writes no such form; see docs/reference/rdf-mapping.md § End-binding heads")
	}
	return strings.Join(words, " "), nil
}

// endVerbs are the verbs a head writes ahead of its ends when its own keyword
// is a noun (`allocation a allocate x to y`, `connector c from x to y`), by the
// form they introduce.
var endVerbs = map[string][]string{
	formTo:        {"connect", "allocate", "from"},
	formNary:      {"connect", "allocate"},
	formEquals:    {"bind", "of"},
	formFirstThen: {"first"},
}

// endForm states the form an end-binding head writes its ends in, so a graph
// without the head's source text can be written back from its structure. The
// form is only claimed when rebuilding it reproduces the head as written: a
// head this mapping cannot rebuild exactly stays readable as text alone.
func (e *encoder) endForm(subject rdf.Term, n *ast.Usage) {
	if n.Kind == ast.UsageSatisfy {
		e.satisfyForm(subject, n)
		return
	}
	form, ends, payload := e.endShape(n)
	if form == "" {
		return
	}
	keyword, from := e.endVerb(n, form)
	written, ok := e.headTail(n, from)
	if !ok {
		return
	}
	rebuilt, err := (endNotation{form: form, keyword: keyword, ends: ends, payload: payload}).text()
	if err != nil || !sameSpelling(rebuilt, written) {
		return
	}
	e.graph.Add(subject, e.sysx(xEndForm), rdf.String(form))
	if keyword != "" {
		e.graph.Add(subject, e.sysx(xEndVerb), rdf.String(keyword))
	}
	if n.Keyword != "" && n.Keyword != usageKeyword(n.Kind) {
		e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(n.Keyword))
	}
}

// satisfyForm states that a satisfy head writes the requirement it subsets
// bare, right after its keyword (`satisfy R by v`), which is what tells that
// clause from a `subsets` one written out. The keyword goes with it, since
// `verify` is the same kind spelled differently.
func (e *encoder) satisfyForm(subject rdf.Term, n *ast.Usage) {
	requirement := relationshipTarget(n, ast.RelSubsets)
	if requirement == nil || n.Ident.Name != "" || n.Value != nil {
		return
	}
	for _, rel := range n.Relationships {
		if rel != nil && rel.Kind != ast.RelSubsets && rel.Kind != ast.RelSubject {
			return
		}
	}
	head, ok := e.headTail(n, n.Span().Offset)
	if !ok || !sameSpelling(head, n.Keyword+" "+e.text(requirement)+" "+e.subjectClause(n)) {
		return
	}
	e.graph.Add(subject, e.sysx(xEndForm), rdf.String(formSatisfy))
	if n.Keyword != "" && n.Keyword != usageKeyword(n.Kind) {
		e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(n.Keyword))
	}
}

// subjectClause is the `by` clause of a satisfy head, or "" where it names no
// subject.
func (e *encoder) subjectClause(n *ast.Usage) string {
	subject := relationshipTarget(n, ast.RelSubject)
	if subject == nil {
		return ""
	}
	return "by " + e.text(subject)
}

// endShape reads the form a head writes its ends in, with the end texts the
// graph carries beside it, or "" for a head whose ends the graph cannot state:
// an end that redefines, an inline payload declaration, or a transition's
// trigger, guard and effect.
func (e *encoder) endShape(n *ast.Usage) (form string, ends []string, payload string) {
	switch {
	case len(n.ConnectorEnds) > 0:
		for _, end := range n.ConnectorEnds {
			text, ok := e.connectorEndText(end)
			if !ok {
				return "", nil, ""
			}
			ends = append(ends, text)
		}
		switch {
		case n.Kind == ast.UsageSuccession:
			if len(ends) != 2 {
				return "", nil, ""
			}
			return formFirstThen, ends, ""
		case e.wroteBefore(n, n.ConnectorEnds[0].Target, "("):
			return formNary, ends, ""
		case len(ends) == 2:
			return formTo, ends, ""
		}
		return "", nil, ""
	case n.FlowEnds != nil:
		flow := n.FlowEnds
		if flow.From == nil || flow.To == nil || flow.PayloadDecl != nil || flow.PayloadMultiplicity != nil {
			return "", nil, ""
		}
		ends = []string{e.text(flow.From), e.text(flow.To)}
		if e.wroteBefore(n, flow.From, "from") {
			return formFromTo, ends, e.text(flow.Payload)
		}
		return formFlowTo, ends, e.text(flow.Payload)
	case n.Kind == ast.UsageBinding:
		// `bind [m] a = [n] b` states its ends as the features it references and
		// the value bound to them, not as connector ends.
		for _, rel := range n.Relationships {
			if rel != nil && rel.Kind == ast.RelReferences && rel.Target != nil {
				ends = append(ends, e.endText(rel.Multiplicity, rel.Target))
			}
		}
		if n.Value != nil {
			ends = append(ends, e.endText(n.ValueMultiplicity, n.Value))
		}
		if len(ends) != 2 {
			return "", nil, ""
		}
		return formEquals, ends, ""
	}
	return "", nil, ""
}

// endText is one end as its notation writes it: the multiplicity ahead of the
// feature it names, when the end states one.
func (e *encoder) endText(mult *ast.Multiplicity, target ast.Node) string {
	if mult == nil {
		return e.text(target)
	}
	return e.text(mult) + " " + e.text(target)
}

// connectorEndText is one connector end as the graph can state it: `[1] a.p`
// or `[1] bead ::> t.bead`; an end saying more than that is not stated.
func (e *encoder) connectorEndText(end *ast.ConnectorEnd) (string, bool) {
	if end == nil || end.Target == nil {
		return "", false
	}
	if _, named := end.DeclaredName(); !named {
		if end.ReferencedTarget() != nil {
			return "", false
		}
		return e.endText(end.Multiplicity, end.Target), true
	}
	var reference *ast.Relationship
	for _, rel := range end.Relationships {
		if rel == nil || rel.Kind != ast.RelReferences || reference != nil {
			return "", false
		}
		reference = rel
	}
	if reference == nil {
		return "", false
	}
	return e.endText(end.Multiplicity, end.Target) + " " + e.referencesKeyword(end.Target, reference) + " " + e.text(reference), true
}

// referencesKeyword is the ReferencesKeyword written between a connector end's
// name and its target, `::>` or `references` (KerML.xtext:856).
func (e *encoder) referencesKeyword(name ast.Node, reference *ast.Relationship) string {
	between := source.Span{Offset: name.Span().End(), Len: reference.Span().Offset - name.Span().End()}
	if between.Len > 0 {
		if written := words(e.src.slice(between)); len(written) == 1 && written[0] == "references" {
			return "references"
		}
	}
	return "::>"
}

// endVerb returns the verb written ahead of the ends and the offset the ends
// notation starts at, which is that verb's if there is one.
func (e *encoder) endVerb(n *ast.Usage, form string) (string, int) {
	first := e.firstEnd(n)
	if first == nil {
		return "", -1
	}
	start, at := n.Span().Offset, first.Span().Offset
	if form == formEquals {
		// The `of` of `binding b of a = c` names the bound feature the way
		// `bind` does, and both precede it.
		start = n.Span().Offset
	}
	if at <= start {
		return "", at
	}
	head := source.Span{Offset: start, Len: at - start}
	for _, verb := range endVerbs[form] {
		// A head whose own keyword is the verb (`connect a to b`) writes it
		// once; the keyword the graph already carries is that verb.
		if verb == n.Keyword {
			continue
		}
		if index := e.src.lastToken(head, verb); index >= 0 {
			return verb, index
		}
	}
	if form == formNary {
		if index := e.src.lastToken(head, "("); index >= 0 {
			return "", index
		}
	}
	if form == formFromTo {
		if index := e.src.lastToken(head, "of"); index >= 0 {
			return "", index
		}
		if index := e.src.lastToken(head, "from"); index >= 0 {
			return "", index
		}
	}
	if form == formFlowTo && n.FlowEnds != nil && n.FlowEnds.Payload != nil {
		if index := e.src.lastToken(head, "of"); index >= 0 {
			return "", index
		}
	}
	return "", at
}

// firstEnd returns the node the ends notation begins with: the first end's
// multiplicity when it states one, else the feature it names.
func (e *encoder) firstEnd(n *ast.Usage) ast.Node {
	switch {
	case len(n.ConnectorEnds) > 0 && n.ConnectorEnds[0] != nil:
		if n.ConnectorEnds[0].Multiplicity != nil {
			return n.ConnectorEnds[0].Multiplicity
		}
		return n.ConnectorEnds[0].Target
	case n.FlowEnds != nil:
		if n.FlowEnds.Payload != nil {
			return n.FlowEnds.Payload
		}
		return n.FlowEnds.From
	case n.Kind == ast.UsageBinding:
		for _, rel := range n.Relationships {
			if rel == nil || rel.Kind != ast.RelReferences || rel.Target == nil {
				continue
			}
			if rel.Multiplicity != nil {
				return rel.Multiplicity
			}
			return rel.Target
		}
	}
	return nil
}

// headTail returns the declaration text from an offset to the end of the head,
// which is what the ends notation has to reproduce.
func (e *encoder) headTail(n *ast.Usage, from int) (string, bool) {
	end := e.headEnd(n)
	if from < n.Span().Offset || from >= end {
		return "", false
	}
	text := strings.TrimSpace(e.src.code(source.Span{Offset: from, Len: end - from}))
	if strings.ContainsAny(text, "{}") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimSuffix(text, ";")), true
}

// wroteBefore reports whether word is a token of the head ahead of an end, which
// is what tells the parenthesized and `from` forms from the ones without them.
func (e *encoder) wroteBefore(n *ast.Usage, end ast.Node, word string) bool {
	start, at := n.Span().Offset, end.Span().Offset
	if at <= start {
		return false
	}
	return e.src.lastToken(source.Span{Offset: start, Len: at - start}, word) >= 0
}

// relationshipTarget returns the target of the first relationship of a kind.
func relationshipTarget(n *ast.Usage, kind ast.RelationshipKind) ast.Node {
	for _, rel := range n.Relationships {
		if rel != nil && rel.Kind == kind && rel.Target != nil {
			return rel.Target
		}
	}
	return nil
}

// endWords rebuilds the ends of an end-binding head from the graph: the form it
// states, the verb it writes them after, and the features it relates.
func (d *decoder) endWords(el *element, form string) (string, error) {
	ends, payload, err := d.relatedEnds(el)
	if err != nil {
		return "", err
	}
	verb, _ := d.stringOf(el, rdf.OpenSysML+xEndVerb)
	if form == formEquals && len(ends) == 0 {
		// A binding that relates no end nodes binds the feature it references to
		// its value; the value is written by this notation, not after it.
		reference, err := d.referenceText(el, rdf.SysML+relationshipProperty[ast.RelReferences])
		if err != nil {
			return "", err
		}
		value, ok := d.stringOf(el, rdf.SysML+pValue)
		if reference == "" || !ok {
			return "", d.missing(el, "sysml:"+pValue+" and sysml:references",
				"a binding written as `= ` binds the feature it references to a value")
		}
		ends = []string{reference, value}
	}
	if len(ends) == 0 {
		return "", d.missing(el, "sysx:"+xRelatedFeature, "a head that states sysx:"+xEndForm+" relates the ends it binds")
	}
	return endNotation{form: form, keyword: verb, ends: ends, payload: payload}.text()
}

// relatedEnds reads the ends of a head in the order they are written, each
// behind the multiplicity it states, with the payload of a flow kept apart: it
// is written ahead of them, after `of`.
func (d *decoder) relatedEnds(el *element) (ends []string, payload string, err error) {
	type end struct {
		index int
		text  string
	}
	var ordered []end
	for _, term := range d.graph.Objects(rdf.IRI(el.iri), rdf.OpenSysML+xRelatedFeature) {
		text, err := d.expressionNodeText(term, el)
		if err != nil {
			return nil, "", err
		}
		if role, ok := d.graph.Lexical(term, rdf.OpenSysML+xEndRole); ok && role == "payload" {
			payload = text
			continue
		}
		if name, ok := d.graph.Lexical(term, rdf.OpenSysML+xEndName); ok {
			text = nameText(name) + " ::> " + text
		}
		mult, err := d.endMultiplicity(term, el)
		if err != nil {
			return nil, "", err
		}
		if mult != "" {
			text = mult + " " + text
		}
		ordered = append(ordered, end{index: intOf(d.graph, term, rdf.OpenSysML+xEndIndex), text: text})
	}
	slices.SortStableFunc(ordered, func(a, b end) int { return a.index - b.index })
	for _, end := range ordered {
		ends = append(ends, end.text)
	}
	return ends, payload, nil
}

// endMultiplicity writes the bounds an end node states (`connect [1] a to b`),
// or "" for an end written bare.
func (d *decoder) endMultiplicity(end rdf.Term, in *element) (string, error) {
	lower, hasLower, err := d.boundText(end, rdf.SysML+pLowerBound, in)
	if err != nil {
		return "", err
	}
	upper, hasUpper, err := d.boundText(end, rdf.SysML+pUpperBound, in)
	if err != nil {
		return "", err
	}
	return multiplicityNotation(lower, upper, hasLower, hasUpper), nil
}

// statesEnds reports whether an element relates ends of its own, the shape that
// needs a form to be written back.
func (d *decoder) statesEnds(el *element) bool {
	return len(d.graph.Objects(rdf.IRI(el.iri), rdf.OpenSysML+xRelatedFeature)) > 0
}
