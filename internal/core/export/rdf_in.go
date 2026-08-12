package export

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/format"
	"github.com/Open-MBEE/Systemica/internal/core/rdf"
)

// annotationMetaclasses are the elements whose notation is a comment body,
// which terminates the declaration by itself.
var annotationMetaclasses = map[string]bool{
	"Comment":               true,
	"Documentation":         true,
	"TextualRepresentation": true,
}

// element is one subject of the graph, read into the shape the printer needs.
type element struct {
	iri         string
	metaclass   string
	memberIndex int
	// scope is the qualified name of the namespace this element is declared
	// in, which is what a reference written inside it is relative to.
	scope    string
	children []*element
}

// ToSysML converts an RDF graph back into SysML v2 source text. The result is
// run through the source formatter, so the output is indented the same way as a
// formatted file rather than in whatever order the graph happened to be in.
//
// A subject whose metaclass this mapping does not know, or which lacks the
// properties needed to rebuild its declaration, is reported as an
// UnsupportedError: a converted file that dropped an element would be worse
// than a failed conversion.
func ToSysML(graph *rdf.Graph) ([]byte, error) {
	if graph == nil || graph.Len() == 0 {
		return nil, &UnsupportedError{What: "an empty graph", Note: "nothing to convert"}
	}
	d := &decoder{graph: graph, byIRI: map[string]*element{}}
	roots, err := d.build()
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	for _, root := range roots {
		if err := d.print(&b, root, 0); err != nil {
			return nil, err
		}
	}
	out, err := format.Source("<converted>", []byte(b.String()), format.DefaultOptions)
	if err != nil {
		// The formatter only fails on input it cannot lex; return the
		// unformatted text with the reason so the user can see what was built.
		return nil, fmt.Errorf("converted source is not valid SysML: %w", err)
	}
	return out, nil
}

type decoder struct {
	graph *rdf.Graph
	byIRI map[string]*element
}

// build reads every subject into an element and links it to its owner,
// returning the elements that have no owner in the graph.
func (d *decoder) build() ([]*element, error) {
	var (
		order []*element
		roots []*element
	)
	for _, subject := range d.graph.Subjects() {
		metaclass := rdf.LocalName(d.graph.Type(subject))
		if metaclass == "" {
			return nil, &UnsupportedError{
				What: fmt.Sprintf("the subject <%s>", subject.Value),
				Note: "it has no rdf:type, so there is no way to tell what to write",
			}
		}
		el := &element{
			iri:         subject.Value,
			metaclass:   metaclass,
			memberIndex: d.intOf(subject, rdf.Systemica+xMemberIndex),
		}
		d.byIRI[el.iri] = el
		order = append(order, el)
	}
	for _, el := range order {
		owner, ok := d.graph.Object(rdf.IRI(el.iri), rdf.SysML+pOwningNamespace)
		if !ok {
			roots = append(roots, el)
			continue
		}
		parent, known := d.byIRI[owner.Value]
		if !known {
			return nil, &UnsupportedError{
				What: fmt.Sprintf("the element <%s>", el.iri),
				Note: fmt.Sprintf("its owning namespace <%s> is not in the graph", owner.Value),
			}
		}
		el.scope = d.qualifiedName(parent)
		parent.children = append(parent.children, el)
	}
	sortByIndex(roots)
	for _, el := range order {
		sortByIndex(el.children)
	}
	if err := d.checkReachable(roots, order); err != nil {
		return nil, err
	}
	return roots, nil
}

// checkReachable reports an element that no root owns, which happens when
// ownership forms a cycle. Printing walks down from the roots, so such an
// element would be left out of the output without this check.
func (d *decoder) checkReachable(roots, all []*element) error {
	seen := make(map[string]bool, len(all))
	var walk func(el *element)
	walk = func(el *element) {
		if seen[el.iri] {
			return
		}
		seen[el.iri] = true
		for _, child := range el.children {
			walk(child)
		}
	}
	for _, root := range roots {
		walk(root)
	}
	for _, el := range all {
		if !seen[el.iri] {
			return &UnsupportedError{
				What: fmt.Sprintf("the element <%s>", el.iri),
				Note: "no root owns it, so its sysml:owningNamespace chain forms a cycle",
			}
		}
	}
	return nil
}

// qualifiedName returns the qualified name of an element: the one it carries,
// or the one its IRI encodes for a graph written by another tool.
func (d *decoder) qualifiedName(el *element) string {
	if name, ok := d.stringOf(el, rdf.SysML+pQualifiedName); ok {
		return name
	}
	if name, ok := rdf.QualifiedNameOf(el.iri); ok {
		return name
	}
	return ""
}

func sortByIndex(elements []*element) {
	sort.SliceStable(elements, func(i, j int) bool {
		return elements[i].memberIndex < elements[j].memberIndex
	})
}

// print writes one element and, recursively, its members.
func (d *decoder) print(b *strings.Builder, el *element, depth int) error {
	indent := strings.Repeat("    ", depth)
	if text, ok := d.stringOf(el, rdf.Systemica+xSourceText); ok {
		// A declaration whose head this mapping keeps verbatim.
		b.WriteString(indent + strings.TrimSpace(text) + "\n")
		return nil
	}
	head, err := d.head(el)
	if err != nil {
		return err
	}
	b.WriteString(indent + head)
	if annotationMetaclasses[el.metaclass] {
		// A comment, doc or rep declaration ends with its comment body, and
		// takes no terminator.
		b.WriteString("\n")
		return nil
	}
	// An accept parameter is written into its parent's head, not its body.
	children := el.children
	if accept := d.acceptParam(el); accept != nil {
		children = nil
		for _, child := range el.children {
			if child != accept {
				children = append(children, child)
			}
		}
	}
	if len(children) == 0 && !d.boolOf(el, rdf.Systemica+xHasBody) {
		b.WriteString(";\n")
		return nil
	}
	b.WriteString(" {\n")
	for _, child := range children {
		if err := d.print(b, child, depth+1); err != nil {
			return err
		}
	}
	b.WriteString(indent + "}\n")
	return nil
}

// head builds the declaration text up to the body or terminator.
func (d *decoder) head(el *element) (string, error) {
	switch el.metaclass {
	case "Package", "Namespace":
		return d.namespaceHead(el), nil
	case "Import":
		return d.importHead(el)
	case mAlias:
		return d.aliasHead(el)
	case "Dependency":
		return d.dependencyHead(el)
	case "Comment":
		return d.commentHead(el), nil
	case "Documentation":
		return d.documentationHead(el), nil
	case "TextualRepresentation":
		return d.representationHead(el)
	case mMultiplicity:
		return d.multiplicityHead(el), nil
	case mFilter:
		condition, ok := d.stringOf(el, rdf.Systemica+xFilter)
		if !ok {
			return "", d.missing(el, "sysx:"+xFilter, "a filter is its condition")
		}
		return "filter " + condition, nil
	}
	// A succession carrying its ends as references is the one the parser builds
	// for a `then`, written back as the edge form: `then <source> <target>;`
	// sequences the two members it names wherever they are declared, so the
	// order survives the round trip. A `succession` declaration whose head was
	// kept verbatim never reaches here — print() writes its source text.
	if el.metaclass == "SuccessionAsUsage" {
		source := d.referenceText(el, rdf.SysML+pSourceFeature)
		target := d.referenceText(el, rdf.SysML+pTargetFeature)
		if source != "" && target != "" {
			return "then " + source + " " + target, nil
		}
	}
	if kind, ok := metaclassDefinition[el.metaclass]; ok {
		return d.definitionHead(el, kind), nil
	}
	if kind, ok := metaclassUsage[el.metaclass]; ok {
		return d.usageHead(el, kind), nil
	}
	return "", &UnsupportedError{
		What: fmt.Sprintf("the element <%s> of type sysml:%s", el.iri, el.metaclass),
		Note: "this metaclass has no SysML notation in the conversion mapping",
	}
}

func (d *decoder) namespaceHead(el *element) string {
	var words []string
	words = append(words, d.prefixWords(el)...)
	if keyword := d.visibility(el); keyword != "" {
		words = append(words, keyword)
	}
	if el.metaclass == "Package" {
		if d.boolOf(el, rdf.Systemica+"isStandardLibraryPackage") {
			words = append(words, "standard")
		}
		if d.boolOf(el, rdf.Systemica+"isLibraryPackage") {
			words = append(words, "library")
		}
		words = append(words, "package")
	} else {
		words = append(words, "namespace")
	}
	words = append(words, d.identWords(el)...)
	return strings.Join(words, " ")
}

func (d *decoder) definitionHead(el *element, kind ast.DefinitionKind) string {
	var words []string
	words = append(words, d.prefixWords(el)...)
	if keyword := d.visibility(el); keyword != "" {
		words = append(words, keyword)
	}
	if d.boolOf(el, rdf.SysML+"isAbstract") {
		words = append(words, "abstract")
	}
	if d.boolOf(el, rdf.SysML+"isVariation") {
		words = append(words, "variation")
	}
	if d.boolOf(el, rdf.SysML+"isConstant") {
		words = append(words, "constant")
	}
	if d.boolOf(el, rdf.SysML+"isEvent") {
		words = append(words, "event")
	}
	words = append(words, d.keywordOr(el, definitionKeyword(kind)))
	if d.boolOf(el, rdf.SysML+"isAll") {
		words = append(words, "all")
	}
	// Every definition kind but `metaclass` shares its keyword with a usage
	// form, and is told apart from it by `def`.
	if kind != ast.DefMetaclass {
		words = append(words, "def")
	}
	words = append(words, d.identWords(el)...)
	words = append(words, d.relationshipWords(el)...)
	return strings.Join(words, " ")
}

func (d *decoder) usageHead(el *element, kind ast.UsageKind) string {
	var words []string
	words = append(words, d.prefixWords(el)...)
	if keyword := d.visibility(el); keyword != "" {
		words = append(words, keyword)
	}
	if d.boolOf(el, rdf.SysML+"isAbstract") {
		words = append(words, "abstract")
	}
	// A result parameter is declared with `return`, which carries its out
	// direction: writing both would not parse.
	isResult := d.boolOf(el, rdf.SysML+"isResult")
	if isResult {
		words = append(words, "return")
	} else if direction, ok := d.stringOf(el, rdf.SysML+pDirection); ok {
		words = append(words, direction)
	}
	keyword := d.keywordOr(el, usageKeyword(kind))
	for _, flag := range []struct {
		property string
		keyword  string
	}{
		{"isComposite", "composite"},
		{"isDerived", "derived"},
		{"isConstant", "constant"},
		{"isIndividual", "individual"},
		{"isSnapshot", "snapshot"},
		{"isEvent", "event"},
		{"isEnd", "end"},
		{"isReference", "ref"},
	} {
		// A keyword such as `snapshot` is both a modifier and a kind keyword;
		// writing it here as well as below would declare it twice.
		if flag.keyword == keyword {
			continue
		}
		if d.boolOf(el, rdf.SysML+flag.property) {
			words = append(words, flag.keyword)
		}
	}
	if d.boolOf(el, rdf.SysML+"isConjugated") {
		keyword = "~" + keyword
	}
	words = append(words, keyword)
	// `chain` qualifies the kind keyword it follows, unlike the modifiers above.
	if d.boolOf(el, rdf.SysML+"isChain") {
		words = append(words, "chain")
	}
	if d.boolOf(el, rdf.SysML+"isAll") {
		words = append(words, "all")
	}
	words = append(words, d.identWords(el)...)
	// The accept shorthand writes its parameter into the head, ahead of the
	// `via` clause the parent's relationships supply.
	if accept := d.acceptParam(el); accept != nil {
		words = append(words, "accept")
		words = append(words, d.identWords(accept)...)
		words = append(words, d.relationshipWords(accept)...)
	}
	words = append(words, d.relationshipWords(el)...)
	// A multiplicity binds to the name or type it follows, with no space
	// before the bracket.
	head := strings.Join(words, " ") + d.multiplicityText(el)
	var tail []string
	if d.boolOf(el, rdf.SysML+"isOrdered") {
		tail = append(tail, "ordered")
	}
	if d.boolOf(el, rdf.SysML+"isNonunique") {
		tail = append(tail, "nonunique")
	}
	if value, ok := d.stringOf(el, rdf.SysML+pValue); ok {
		tail = append(tail, "=", value)
	}
	if len(tail) > 0 {
		head += " " + strings.Join(tail, " ")
	}
	return head
}

// acceptParam returns the synthetic parameter of an accept shorthand, whose
// notation belongs in its parent's declaration head.
func (d *decoder) acceptParam(el *element) *element {
	for _, child := range el.children {
		if d.boolOf(child, rdf.SysML+"isAccept") {
			return child
		}
	}
	return nil
}

// missing reports a graph element that cannot be written back as notation
// because a property its declaration is built from is absent.
func (d *decoder) missing(el *element, property, why string) error {
	return &UnsupportedError{
		What: fmt.Sprintf("the element <%s>", el.iri),
		Note: fmt.Sprintf("it has no %s, and %s, so no valid declaration can be written for it", property, why),
	}
}

func (d *decoder) importHead(el *element) (string, error) {
	var words []string
	if keyword := d.visibility(el); keyword != "" {
		words = append(words, keyword)
	}
	if d.boolOf(el, rdf.Systemica+xExpose) {
		words = append(words, "expose")
	} else {
		words = append(words, "import")
	}
	if d.boolOf(el, rdf.SysML+pIsImportAll) {
		words = append(words, "all")
	}
	imported, ok := d.stringOf(el, rdf.SysML+pImportedNamespace)
	if !ok {
		return "", d.missing(el, "sysml:"+pImportedNamespace, "an import names the namespace it imports")
	}
	switch {
	case d.boolOf(el, rdf.Systemica+xRecursive):
		imported += "::**"
	case d.boolOf(el, rdf.Systemica+xNamespaceImport):
		imported += "::*"
	}
	words = append(words, imported)
	if filter, ok := d.stringOf(el, rdf.Systemica+xFilter); ok {
		words = append(words, "["+filter+"]")
	}
	return strings.Join(words, " "), nil
}

func (d *decoder) aliasHead(el *element) (string, error) {
	var words []string
	if keyword := d.visibility(el); keyword != "" {
		words = append(words, keyword)
	}
	words = append(words, "alias")
	words = append(words, d.identWords(el)...)
	for_ := d.referenceText(el, rdf.SysML+pAliasFor)
	if for_ == "" {
		return "", d.missing(el, "sysml:"+pAliasFor, "an alias names the element it stands for")
	}
	words = append(words, "for", for_)
	return strings.Join(words, " "), nil
}

func (d *decoder) dependencyHead(el *element) (string, error) {
	var words []string
	words = append(words, d.prefixWords(el)...)
	if keyword := d.visibility(el); keyword != "" {
		words = append(words, keyword)
	}
	words = append(words, "dependency")
	words = append(words, d.identWords(el)...)
	clients := d.referenceList(el, rdf.SysML+pClient)
	suppliers := d.referenceList(el, rdf.SysML+pSupplier)
	if len(clients) == 0 {
		return "", d.missing(el, "sysml:"+pClient, "a dependency runs from at least one client")
	}
	if len(suppliers) == 0 {
		return "", d.missing(el, "sysml:"+pSupplier, "a dependency runs to at least one supplier")
	}
	// `from` is what separates the clients from a name: without it, the first
	// client would be read as the dependency's own name.
	words = append(words, "from", strings.Join(clients, ", "))
	words = append(words, "to", strings.Join(suppliers, ", "))
	return strings.Join(words, " "), nil
}

func (d *decoder) commentHead(el *element) string {
	// The keyword is what makes this a declared element rather than lexical
	// trivia, so it is written even when nothing identifies the comment.
	words := []string{"comment"}
	words = append(words, d.identWords(el)...)
	if about := d.referenceList(el, rdf.SysML+pAnnotatedElement); len(about) > 0 {
		words = append(words, "about", strings.Join(about, ", "))
	}
	words = append(words, d.localeWords(el)...)
	body, _ := d.stringOf(el, rdf.SysML+pBody)
	return strings.Join(words, " ") + " /*" + body + "*/"
}

func (d *decoder) documentationHead(el *element) string {
	words := []string{"doc"}
	words = append(words, d.identWords(el)...)
	words = append(words, d.localeWords(el)...)
	body, _ := d.stringOf(el, rdf.SysML+pBody)
	return strings.Join(words, " ") + " /*" + body + "*/"
}

func (d *decoder) representationHead(el *element) (string, error) {
	words := []string{"rep"}
	words = append(words, d.identWords(el)...)
	language, ok := d.stringOf(el, rdf.SysML+pLanguage)
	if !ok {
		return "", d.missing(el, "sysml:"+pLanguage, "a textual representation states the language it is written in")
	}
	body, _ := d.stringOf(el, rdf.SysML+pBody)
	words = append(words, "language", strconv.Quote(language))
	return strings.Join(words, " ") + " /*" + body + "*/", nil
}

func (d *decoder) multiplicityHead(el *element) string {
	words := []string{"multiplicity"}
	words = append(words, d.identWords(el)...)
	if mult := d.multiplicityText(el); mult != "" {
		words = append(words, mult)
	}
	return strings.Join(words, " ")
}

// A comment or doc declaration whose head carries a locale needs the `locale`
// keyword written back out.
func (d *decoder) localeWords(el *element) []string {
	locale, ok := d.stringOf(el, rdf.SysML+pLocale)
	if !ok {
		return nil
	}
	return []string{"locale", strconv.Quote(locale)}
}

// keywordOr returns the kind keyword the author wrote, falling back to the
// canonical one when the graph does not record a synonym.
func (d *decoder) keywordOr(el *element, canonical string) string {
	if written, ok := d.stringOf(el, rdf.Systemica+xDeclaredKeyword); ok && written != "" {
		return written
	}
	return canonical
}

func (d *decoder) identWords(el *element) []string {
	var words []string
	if short, ok := d.stringOf(el, rdf.SysML+pDeclaredShortName); ok {
		words = append(words, "<"+short+">")
	}
	if name, ok := d.stringOf(el, rdf.SysML+pDeclaredName); ok {
		words = append(words, name)
	}
	return words
}

func (d *decoder) prefixWords(el *element) []string {
	var words []string
	for _, term := range d.graph.Objects(rdf.IRI(el.iri), rdf.Systemica+xPrefixMetadata) {
		words = append(words, term.Value)
	}
	return words
}

func (d *decoder) visibility(el *element) string {
	keyword, ok := d.stringOf(el, rdf.SysML+pVisibility)
	if !ok {
		return ""
	}
	return visibilityKeyword(visibilityOf(keyword))
}

// relationshipWords renders the typing and specialization clauses of a
// declaration head, in the order the grammar expects.
func (d *decoder) relationshipWords(el *element) []string {
	var words []string
	for _, kind := range relationshipOrder {
		targets := d.referenceList(el, rdf.SysML+relationshipProperty[kind])
		if len(targets) == 0 {
			continue
		}
		words = append(words, relationshipSyntax[kind], strings.Join(targets, ", "))
	}
	return words
}

func (d *decoder) multiplicityText(el *element) string {
	lower, hasLower := d.stringOf(el, rdf.SysML+pLowerBound)
	upper, hasUpper := d.stringOf(el, rdf.SysML+pUpperBound)
	switch {
	case hasLower && hasUpper:
		return "[" + lower + ".." + upper + "]"
	case hasUpper:
		return "[" + upper + "]"
	case hasLower:
		return "[" + lower + "..*]"
	}
	return ""
}

// referenceText renders a single reference property: an element IRI becomes the
// qualified name it encodes, a literal is the name as written.
func (d *decoder) referenceText(el *element, property string) string {
	list := d.referenceList(el, property)
	if len(list) == 0 {
		return ""
	}
	return list[0]
}

func (d *decoder) referenceList(el *element, property string) []string {
	var out []string
	for _, term := range d.graph.Objects(rdf.IRI(el.iri), property) {
		out = append(out, d.referenceName(term, el.scope))
	}
	return out
}

// referenceName renders a reference term as the name to write in source.
//
// A literal is a name that resolved outside this model, and is written as it
// was. An element IRI is written relative to the scope the reference appears
// in — the same name the author would have written — falling back to the full
// qualified name when the target is not in scope.
func (d *decoder) referenceName(term rdf.Term, scope string) string {
	if term.IsLiteral() {
		return term.Value
	}
	qname, ok := rdf.QualifiedNameOf(term.Value)
	if !ok {
		return rdf.LocalName(term.Value)
	}
	for {
		if scope == "" {
			return qname
		}
		if rest, found := strings.CutPrefix(qname, scope+"::"); found {
			return rest
		}
		cut := strings.LastIndex(scope, "::")
		if cut < 0 {
			scope = ""
			continue
		}
		scope = scope[:cut]
	}
}

func (d *decoder) stringOf(el *element, property string) (string, bool) {
	value, ok := d.graph.Lexical(rdf.IRI(el.iri), property)
	if !ok || value == "" {
		return "", false
	}
	return value, true
}

func (d *decoder) boolOf(el *element, property string) bool {
	return d.graph.BoolValue(rdf.IRI(el.iri), property)
}

func (d *decoder) intOf(subject rdf.Term, property string) int {
	value, ok := d.graph.Lexical(subject, property)
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return n
}
