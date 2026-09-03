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
// is a noun (`allocation a allocate x to y`), by the form they introduce.
var endVerbs = map[string][]string{
	formTo:        {"connect", "allocate"},
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
// an end with a multiplicity or a `references` clause, an inline payload
// declaration, or a transition's trigger, guard and effect.
func (e *encoder) endShape(n *ast.Usage) (form string, ends []string, payload string) {
	switch {
	case len(n.ConnectorEnds) > 0:
		for _, end := range n.ConnectorEnds {
			if end == nil || end.Target == nil || end.Multiplicity != nil || end.ReferencedTarget() != nil {
				return "", nil, ""
			}
			ends = append(ends, e.text(end.Target))
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
	case n.Kind == ast.UsageBinding && n.Value != nil:
		// `bind a = b` states its ends as the feature it references and the
		// value bound to it, not as connector ends.
		bound := relationshipTarget(n, ast.RelReferences)
		if bound == nil {
			return "", nil, ""
		}
		return formEquals, []string{e.text(bound), e.text(n.Value)}, ""
	}
	return "", nil, ""
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
	head := e.file.Text(source.Span{Offset: start, Len: at - start})
	for _, verb := range endVerbs[form] {
		// A head whose own keyword is the verb (`connect a to b`) writes it
		// once; the keyword the graph already carries is that verb.
		if verb == n.Keyword {
			continue
		}
		if index := strings.LastIndex(head, verb); index >= 0 && isWord(head, index, len(verb)) {
			return verb, start + index
		}
	}
	if form == formNary {
		if index := strings.LastIndex(head, "("); index >= 0 {
			return "", start + index
		}
	}
	if form == formFromTo {
		if index := strings.LastIndex(head, "of "); index >= 0 && isWord(head, index, 2) {
			return "", start + index
		}
		if index := strings.LastIndex(head, "from"); index >= 0 && isWord(head, index, 4) {
			return "", start + index
		}
	}
	if form == formFlowTo && n.FlowEnds != nil && n.FlowEnds.Payload != nil {
		if index := strings.LastIndex(head, "of "); index >= 0 && isWord(head, index, 2) {
			return "", start + index
		}
	}
	return "", at
}

// firstEnd returns the node the ends notation begins with.
func (e *encoder) firstEnd(n *ast.Usage) ast.Node {
	switch {
	case len(n.ConnectorEnds) > 0 && n.ConnectorEnds[0] != nil:
		return n.ConnectorEnds[0].Target
	case n.FlowEnds != nil:
		if n.FlowEnds.Payload != nil {
			return n.FlowEnds.Payload
		}
		return n.FlowEnds.From
	case n.Kind == ast.UsageBinding:
		return relationshipTarget(n, ast.RelReferences)
	}
	return nil
}

// headTail returns the declaration text from an offset to the end of the head,
// which is what the ends notation has to reproduce. A head with a body is
// refused: the mapping keeps such a declaration whole, as text.
func (e *encoder) headTail(n *ast.Usage, from int) (string, bool) {
	end := n.Span().End()
	if from < n.Span().Offset || from >= end {
		return "", false
	}
	text := strings.TrimSpace(e.src.slice(source.Span{Offset: from, Len: end - from}))
	if strings.ContainsAny(text, "{}") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimSuffix(text, ";")), true
}

// isWord reports whether the match at index in text stands alone rather than
// inside a longer name.
func isWord(text string, index, length int) bool {
	isNameByte := func(b byte) bool {
		return b == '_' || b == '\'' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
	}
	if index > 0 && isNameByte(text[index-1]) {
		return false
	}
	after := index + length
	return after >= len(text) || !isNameByte(text[after])
}

// wroteBefore reports whether word appears in the head ahead of an end, which
// is what tells the parenthesized and `from` forms from the ones without them.
func (e *encoder) wroteBefore(n *ast.Usage, end ast.Node, word string) bool {
	start, at := n.Span().Offset, end.Span().Offset
	if at <= start {
		return false
	}
	head := e.file.Text(source.Span{Offset: start, Len: at - start})
	if word == "(" {
		return strings.Contains(head, word)
	}
	index := strings.LastIndex(head, word)
	return index >= 0 && isWord(head, index, len(word))
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
	if form == formEquals {
		// The bound feature and the value it is bound to are the ends of a
		// binding; the value is written by this notation, not after it.
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

// relatedEnds reads the ends of a head in the order they are written, with the
// payload of a flow kept apart: it is written ahead of them, after `of`.
func (d *decoder) relatedEnds(el *element) (ends []string, payload string, err error) {
	type end struct {
		index int
		text  string
	}
	var ordered []end
	for _, term := range d.graph.Objects(rdf.IRI(el.iri), rdf.OpenSysML+xRelatedFeature) {
		text, err := d.expressionNodeText(term, el.scope)
		if err != nil {
			return nil, "", err
		}
		if role, ok := d.graph.Lexical(term, rdf.OpenSysML+xEndRole); ok && role == "payload" {
			payload = text
			continue
		}
		ordered = append(ordered, end{index: intOf(d.graph, term, rdf.OpenSysML+xEndIndex), text: text})
	}
	slices.SortStableFunc(ordered, func(a, b end) int { return a.index - b.index })
	for _, end := range ordered {
		ends = append(ends, end.text)
	}
	return ends, payload, nil
}

// statesEnds reports whether an element relates ends of its own, the shape that
// needs a form to be written back.
func (d *decoder) statesEnds(el *element) bool {
	return len(d.graph.Objects(rdf.IRI(el.iri), rdf.OpenSysML+xRelatedFeature)) > 0
}
