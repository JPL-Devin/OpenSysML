package export

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/format"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf/ontology"
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
	// qname is the element's sysml:qualifiedName, the only place its identity
	// is read from — never the IRI, whose id is an opaque encoding.
	qname string
	// scope is the qualified name of the namespace this element is declared
	// in, which is what a reference written inside it is relative to.
	scope    string
	children []*element
	// prefix is written ahead of the declaration, for a member a succession
	// attached itself to (`then send Show(x) to screen;`).
	prefix string
	// expressions holds the notation of each expression-valued property, keyed
	// by predicate, resolved from the expression graph the property points at.
	expressions map[string]string
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
	if err := checkExtensionNamespace(graph); err != nil {
		return nil, err
	}
	d := &decoder{
		graph:            graph,
		byIRI:            map[string]*element{},
		memberships:      map[string]membership{},
		owningMembership: map[string]membership{},
	}
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

// checkExtensionNamespace refuses a graph written with the pre-rename extension
// namespace. Its properties would otherwise read as absent and the elements they
// describe would be written back without them.
func checkExtensionNamespace(graph *rdf.Graph) error {
	for _, triple := range graph.Triples() {
		for _, term := range []rdf.Term{triple.Subject, triple.Predicate, triple.Object} {
			if term.Kind == rdf.TermIRI && strings.HasPrefix(term.Value, rdf.LegacyExtension) {
				return legacyNamespaceError(term.Value)
			}
		}
		if strings.HasPrefix(triple.Object.Datatype, rdf.LegacyExtension) {
			return legacyNamespaceError(triple.Object.Datatype)
		}
	}
	return nil
}

func legacyNamespaceError(iri string) error {
	return &UnsupportedError{
		What: fmt.Sprintf("the term <%s>", iri),
		Note: fmt.Sprintf("it is in the pre-rename extension namespace %s, which this version does not read; convert the model from source again to write %s", rdf.LegacyExtension, rdf.OpenSysML),
	}
}

// membership is one materialized membership of the graph: the namespace it
// belongs to and the member it owns. A membership is not written back as a
// declaration — the notation states it by nesting the member in its owner — so
// it is read as the ownership edge it stands for rather than as an element.
type membership struct {
	iri    string
	owner  string
	member string
}

type decoder struct {
	graph *rdf.Graph
	byIRI map[string]*element
	// memberships is keyed by membership IRI, and owningMembership by the IRI of
	// the member each one owns.
	memberships      map[string]membership
	owningMembership map[string]membership
}

// build reads every subject into an element and links it to its owner,
// returning the elements that have no owner in the graph.
func (d *decoder) build() ([]*element, error) {
	var (
		order []*element
		roots []*element
	)
	for _, subject := range d.graph.Subjects() {
		if d.isExpressionNode(subject) {
			// A node of an expression graph belongs to the declaration that holds
			// the expression, not to an element of its own.
			continue
		}
		metaclass := rdf.LocalName(d.graph.Type(subject))
		if metaclass == "" {
			return nil, &UnsupportedError{
				What: fmt.Sprintf("the subject <%s>", subject.Value),
				Note: "it has no rdf:type, so there is no way to tell what to write",
			}
		}
		if ontology.IsAncestorOrSelf(metaclass, mOwningMembership) && !d.graph.HasProperty(subject, rdf.SysML+pQualifiedName) {
			// A membership with no qualified name states ownership rather than a
			// declaration of its own; one with a name, such as a state's entry
			// membership, is written as the member it is.
			if err := d.readMembership(subject); err != nil {
				return nil, err
			}
			continue
		}
		el := &element{
			iri:         subject.Value,
			metaclass:   metaclass,
			memberIndex: d.intOf(subject, rdf.OpenSysML+xMemberIndex),
		}
		el.qname, _ = d.stringOf(el, rdf.SysML+pQualifiedName)
		d.byIRI[el.iri] = el
		order = append(order, el)
	}
	for _, el := range order {
		parent, err := d.ownerOf(el)
		if err != nil {
			return nil, err
		}
		if parent == nil {
			roots = append(roots, el)
			continue
		}
		el.scope = parent.qname
		parent.children = append(parent.children, el)
	}
	if err := d.checkReferences(); err != nil {
		return nil, err
	}
	if err := d.resolveExpressions(); err != nil {
		return nil, err
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

// readMembership records the ownership edge an OwningMembership stands for. Both
// ends are stated twice in the abstract syntax — once under the membership's own
// name for the property and once under the Relationship's — and either spelling
// is accepted, since a graph from another tool may carry only one.
func (d *decoder) readMembership(subject rdf.Term) error {
	owner, hasOwner := d.firstObject(subject, pMembershipOwningNamespace, pOwningRelatedElement)
	member, hasMember := d.firstObject(subject, pMemberElement, pOwnedMemberElement, pOwnedMemberFeature, pOwnedRelatedElement)
	if !hasOwner || !hasMember {
		return &UnsupportedError{
			What: fmt.Sprintf("the membership <%s>", subject.Value),
			Note: "a membership states the namespace it belongs to in sysml:membershipOwningNamespace and the element it owns in sysml:memberElement, and this one states one of them or neither",
		}
	}
	m := membership{iri: subject.Value, owner: owner.Value, member: member.Value}
	d.memberships[m.iri] = m
	d.owningMembership[m.member] = m
	return nil
}

// firstObject returns the object of the first of properties the subject states.
func (d *decoder) firstObject(subject rdf.Term, properties ...string) (rdf.Term, bool) {
	for _, property := range properties {
		if object, ok := d.graph.Object(subject, rdf.SysML+property); ok {
			return object, true
		}
	}
	return rdf.Term{}, false
}

// ownerOf returns the element that owns el, or nil when it is a root. Ownership
// is read through the element's OwningMembership, which is where the abstract
// syntax puts it; a graph carrying only the compact sysml:owningNamespace shape
// this tool wrote before memberships were materialized is still read.
func (d *decoder) ownerOf(el *element) (*element, error) {
	ownerIRI := ""
	switch relationship, ok := d.firstObject(rdf.IRI(el.iri), pOwningMembership, pOwningRelationship); {
	case ok:
		// The owning relationship is either a membership standing between the
		// element and its owner, or the owner itself when a relationship owns
		// the element directly, as a state owns its entry action.
		if m, known := d.memberships[relationship.Value]; known {
			ownerIRI = m.owner
		} else {
			ownerIRI = relationship.Value
		}
	default:
		// A relationship a namespace declares — an import, a dependency, a
		// membership — states the element that owns it rather than a membership.
		owner, hasOwner := d.firstObject(rdf.IRI(el.iri), pOwningRelatedElement, pOwningNamespace, pOwner)
		if !hasOwner {
			return nil, nil
		}
		ownerIRI = owner.Value
	}
	parent, known := d.byIRI[ownerIRI]
	if !known {
		return nil, &UnsupportedError{
			What: fmt.Sprintf("the element <%s>", el.iri),
			Note: fmt.Sprintf("its owning namespace <%s> is not in the graph", ownerIRI),
		}
	}
	return parent, nil
}

// referenceProperties are the predicates whose IRI objects reference elements,
// which must be graph subjects carrying sysml:qualifiedName.
var referenceProperties = func() map[string]bool {
	set := map[string]bool{
		rdf.SysML + pOwningNamespace:  true,
		rdf.SysML + pOwner:            true,
		rdf.SysML + pOwnedMember:      true,
		rdf.SysML + pSourceFeature:    true,
		rdf.SysML + pTargetFeature:    true,
		rdf.SysML + pClient:           true,
		rdf.SysML + pSupplier:         true,
		rdf.SysML + pAliasFor:         true,
		rdf.SysML + pAnnotatedElement: true,
		// The ends a succession reaches by position, which are elements of the
		// graph rather than names a reference could be written from.
		rdf.OpenSysML + xSourceMember: true,
		rdf.OpenSysML + xTargetMember: true,
	}
	for _, property := range relationshipProperty {
		set[rdf.SysML+property] = true
	}
	return set
}()

// checkReferences refuses a graph whose element references cannot be named
// from the graph itself, which would otherwise be written back mis-named.
func (d *decoder) checkReferences() error {
	for _, triple := range d.graph.Triples() {
		if triple.Object.Kind != rdf.TermIRI || !referenceProperties[triple.Predicate.Value] {
			continue
		}
		if _, err := d.referencedElement(triple.Object.Value); err != nil {
			return err
		}
	}
	return nil
}

// referencedElement resolves a referenced IRI to the graph subject whose
// sysml:qualifiedName names it, reporting a reference no property can name.
func (d *decoder) referencedElement(iri string) (*element, error) {
	target, ok := d.byIRI[iri]
	if !ok {
		return nil, &UnsupportedError{
			What: fmt.Sprintf("the reference <%s>", iri),
			Note: "it is not a subject of the graph, so there is no sysml:qualifiedName to write its name back from",
		}
	}
	if target.qname == "" {
		return nil, &UnsupportedError{
			What: fmt.Sprintf("the element <%s>", target.iri),
			Note: "it is referenced but carries no sysml:qualifiedName, which is where a reference's name is read from",
		}
	}
	return target, nil
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

func sortByIndex(elements []*element) {
	sort.SliceStable(elements, func(i, j int) bool {
		return elements[i].memberIndex < elements[j].memberIndex
	})
}

// print writes one element and, recursively, its members.
func (d *decoder) print(b *strings.Builder, el *element, depth int) error {
	indent := strings.Repeat("    ", depth)
	lead := indent + el.prefix
	if text, ok := d.stringOf(el, rdf.OpenSysML+xSourceText); ok {
		// A declaration whose head this mapping keeps verbatim.
		b.WriteString(lead + strings.TrimSpace(text) + "\n")
		return nil
	}
	if handled, err := d.printBehavior(b, el, lead, depth); handled {
		return err
	}
	head, err := d.head(el)
	if err != nil {
		return err
	}
	b.WriteString(lead + head)
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
	children, err = d.positionalSuccessions(children)
	if err != nil {
		return err
	}
	// `parallel` marks a state's substates orthogonal, and only a body may
	// follow it, so a parallel state with none has no notation.
	parallel := d.boolOf(el, rdf.SysML+"isParallel")
	if len(children) == 0 && !d.boolOf(el, rdf.OpenSysML+xHasBody) {
		if parallel {
			return d.missing(el, "sysx:"+xHasBody, "a parallel state states its regions in a body")
		}
		b.WriteString(";\n")
		return nil
	}
	if parallel {
		b.WriteString(" parallel")
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
	case "Specialization", "FeatureTyping", "Subsetting", "Redefinition",
		"FeatureInverting", "TypeFeaturing", "Conjugation":
		return d.relationshipElementHead(el)
	case "Comment":
		return d.commentHead(el)
	case "Documentation":
		return d.documentationHead(el), nil
	case "TextualRepresentation":
		return d.representationHead(el)
	case mMultiplicity:
		return d.multiplicityHead(el)
	case mFilter:
		condition, ok := d.stringOf(el, rdf.OpenSysML+xFilter)
		if !ok {
			return "", d.missing(el, "sysx:"+xFilter, "a filter is its condition")
		}
		return "filter " + condition, nil
	case mConstraint:
		// A bare condition states no keyword of its own; it asserts implicitly.
		keyword, _ := d.stringOf(el, rdf.OpenSysML+xDeclaredKeyword)
		return d.conditionHead(el, keyword)
	case mAssume:
		return d.conditionHead(el, "assume")
	case mRequire:
		return d.conditionHead(el, "require")
	}
	// A succession carrying its ends as references is the one the parser builds
	// for a succession, written back as `succession first <source> then <target>;`.
	// sequences the two members it names wherever they are declared, so the
	// order survives the round trip. A `succession` declaration whose head was
	// kept verbatim never reaches here — print() writes its source text.
	// A `succession` declaration that states the form its ends are written in
	// is a head that binds ends, not an edge between two members.
	if el.metaclass == mSuccession && !d.statesEnds(el) {
		return d.successionHead(el)
	}
	// A control node, statement, state or region: the behavioral half of the
	// mapping writes the ones whose notation is a head and a terminator.
	if head, handled, err := d.behaviorHead(el); handled {
		return head, err
	}
	if kind, ok := metaclassDefinition[el.metaclass]; ok {
		return d.definitionHead(el, kind)
	}
	if kind, ok := metaclassUsage[el.metaclass]; ok {
		return d.usageHead(el, kind)
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
		if d.boolOf(el, rdf.OpenSysML+"isStandardLibraryPackage") {
			words = append(words, "standard")
		}
		if d.boolOf(el, rdf.OpenSysML+"isLibraryPackage") {
			words = append(words, "library")
		}
		words = append(words, "package")
	} else {
		words = append(words, "namespace")
	}
	words = append(words, d.identWords(el)...)
	return strings.Join(words, " ")
}

func (d *decoder) definitionHead(el *element, kind ast.DefinitionKind) (string, error) {
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
	relationships, err := d.relationshipWords(el, "")
	if err != nil {
		return "", err
	}
	words = append(words, relationships...)
	return strings.Join(words, " "), nil
}

func (d *decoder) usageHead(el *element, kind ast.UsageKind) (string, error) {
	// A head that binds ends is written from the form it states; one relating
	// ends without a form is refused rather than written back without them.
	endForm, hasEnds := d.stringOf(el, rdf.OpenSysML+xEndForm)
	// A satisfy head names the requirement it subsets bare rather than through
	// a relatedFeature end, so its form states no ends.
	if endForm == formSatisfy {
		hasEnds = false
	}
	if !hasEnds && d.statesEnds(el) {
		return "", d.missing(el, "sysx:"+xEndForm,
			"the ends it relates are written in the form the head states")
	}
	words := d.prefixWords(el)
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
	// An accept written without the `action` keyword its kind states carries
	// `accept` as the keyword it was written with; the shorthand writes it.
	if keyword == "accept" {
		keyword = ""
	}
	for _, flag := range []struct {
		property string
		keyword  string
	}{
		{"isVariation", "variation"},
		{"isVariant", "variant"},
		{"isComposite", "composite"},
		{"isDerived", "derived"},
		{"isConstant", "constant"},
		{"isIndividual", "individual"},
		{"isSnapshot", "snapshot"},
		{"isTimeslice", "timeslice"},
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
	// A prefix qualifies the kind keyword after it, and the `not` of
	// `assert not constraint c` negates the declaration that prefix introduces.
	// Negation on its own has no notation, so it is reported rather than dropped.
	prefix, hasPrefix := d.stringOf(el, rdf.OpenSysML+xDeclaredPrefix)
	negated := d.boolOf(el, rdf.SysML+"isNegated")
	switch {
	case hasPrefix:
		words = append(words, prefix)
		if negated {
			words = append(words, "not")
		}
	case negated:
		return "", d.missing(el, "sysx:"+xDeclaredPrefix, "the `not` of a negated declaration qualifies the prefix keyword it follows")
	}
	keywordAt := len(words)
	if keyword != "" {
		words = append(words, keyword)
	}
	// `chain` qualifies the kind keyword it follows, unlike the modifiers above.
	if d.boolOf(el, rdf.SysML+"isChain") {
		words = append(words, "chain")
	}
	if d.boolOf(el, rdf.SysML+"isAll") {
		words = append(words, "all")
	}
	// A `render`/`frame` reference writes its target as a bare name; without one the
	// member declares a usage, spelling out the kind keyword (SysML.xtext
	// ViewRenderingUsage, FramedConcernUsage) even when it declares no name.
	var skip []ast.RelationshipKind
	if endForm == formEquals {
		// The bound feature is an end of the binding, written by the ends
		// notation rather than as a `references` clause.
		skip = append(skip, ast.RelReferences)
	}
	// A satisfy head writes the requirement it subsets bare, after the keyword.
	if endForm == formSatisfy {
		targets, err := d.referenceList(el, rdf.SysML+relationshipProperty[ast.RelSubsets])
		if err != nil {
			return "", err
		}
		if len(targets) == 0 {
			return "", d.missing(el, "sysml:"+relationshipProperty[ast.RelSubsets],
				"a satisfy head names the requirement it satisfies")
		}
		words = append(words, strings.Join(targets, ", "))
		skip = append(skip, ast.RelSubsets)
	}
	if noun := memberDeclarationKeyword(kind); noun != "" {
		targets, err := d.referenceList(el, rdf.SysML+relationshipProperty[ast.RelReferences])
		if err != nil {
			return "", err
		}
		if len(targets) > 0 {
			words = append(words, strings.Join(targets, ", "))
			skip = append(skip, ast.RelReferences)
		} else {
			words = append(words, noun)
		}
	}
	// `include` states the use case a case performs: `include <ref>;` names an
	// existing one, `include use case <name> : T` declares one that includes T
	// (SysML.xtext PerformedUseCaseUsage). Both carry the inclusion as a
	// relationship, which the keyword itself writes.
	included, err := d.referenceList(el, rdf.SysML+relationshipProperty[ast.RelIncludes])
	if err != nil {
		return "", err
	}
	if len(included) > 0 {
		skip = append(skip, ast.RelIncludes)
		if len(d.identWords(el)) == 0 {
			// `include <ref>;` states no kind keyword and takes the inclusion in
			// its place; the typing the parser derives from it is that same target.
			words = append(words[:keywordAt:keywordAt], "include", strings.Join(included, ", "))
			skip = append(skip, ast.RelTyping)
		} else {
			words = append(words[:keywordAt:keywordAt], append([]string{"include"}, words[keywordAt:]...)...)
		}
	}
	// A `perform` or a state's `entry`/`do`/`exit` names the action it performs,
	// declaring no name of its own (SysML.xtext PerformActionUsageDeclaration).
	identWords := d.identWords(el)
	if len(identWords) == 0 && referenceMemberKeyword(keyword) {
		targets, err := d.referenceList(el, rdf.SysML+relationshipProperty[ast.RelReferences])
		if err != nil {
			return "", err
		}
		if len(targets) > 0 {
			words = append(words, strings.Join(targets, ", "))
			skip = append(skip, ast.RelReferences)
		}
	}
	words = append(words, identWords...)
	// The accept shorthand writes its parameter into the head, ahead of the
	// `via` clause the parent's relationships supply.
	if accept := d.acceptParam(el); accept != nil {
		words = append(words, "accept")
		words = append(words, d.identWords(accept)...)
		acceptWords, err := d.relationshipWords(accept, "")
		if err != nil {
			return "", err
		}
		words = append(words, acceptWords...)
		// A trigger (`when`/`at`/`after` …) is what the payload accepts, written
		// in place of a type rather than as a value clause.
		if trigger, ok := d.stringOf(accept, rdf.SysML+pValue); ok {
			words = append(words, trigger)
		}
	}
	// The multiplicity part (`[1] ordered nonunique`) qualifies the type it
	// follows, so it goes with the typing clause and ahead of any further
	// specialization; with no type it follows the name.
	multPart := d.multiplicityText(el)
	if d.boolOf(el, rdf.SysML+"isOrdered") {
		multPart += " ordered"
	}
	if d.boolOf(el, rdf.SysML+"isNonunique") {
		multPart += " nonunique"
	}
	typed, err := d.referenceList(el, rdf.SysML+relationshipProperty[ast.RelTyping])
	if err != nil {
		return "", err
	}
	typedPart := ""
	if len(typed) > 0 {
		typedPart, multPart = multPart, ""
	}
	relationships, err := d.relationshipWords(el, typedPart, skip...)
	if err != nil {
		return "", err
	}
	words = append(words, relationships...)
	if hasEnds {
		ends, err := d.endWords(el, endForm)
		if err != nil {
			return "", err
		}
		words = append(words, ends)
		// The `= value` of a binding is one of its ends, already written above.
		if endForm == formEquals {
			return strings.Join(words, " ") + multPart, nil
		}
	}
	head := strings.Join(words, " ") + multPart
	if value, ok := d.stringOf(el, rdf.SysML+pValue); ok {
		head += " = " + value
	}
	return head, nil
}

// conditionHead rebuilds a condition member from its properties: an inline
// condition (`assert x > 0`), the constraint it states (`require R`), or a
// nested constraint whose conditions are its body (`assume constraint { … }`).
func (d *decoder) conditionHead(el *element, keyword string) (string, error) {
	var words []string
	if keyword != "" {
		words = append(words, keyword)
	}
	if d.boolOf(el, rdf.SysML+"isNegated") {
		words = append(words, "not")
	}
	reference, err := d.referenceText(el, rdf.SysML+relationshipProperty[ast.RelReferences])
	if err != nil {
		return "", err
	}
	switch condition, ok := d.stringOf(el, rdf.OpenSysML+xCondition); {
	case ok:
		words = append(words, condition)
	case reference != "":
		words = append(words, reference)
	case d.boolOf(el, rdf.OpenSysML+xHasBody):
		// The nested-constraint form spells out the kind it declares, so the
		// braces that follow are read as a constraint body rather than a name.
		words = append(words, "constraint")
		words = append(words, d.identWords(el)...)
	default:
		return "", d.missing(el, "sysx:"+xCondition, "a condition member states a condition")
	}
	return strings.Join(words, " "), nil
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
	// An expose is always protected and always imports all (SysML v2 8.3.26.2),
	// so its keyword states both: writing them as well does not parse.
	expose := d.boolOf(el, rdf.OpenSysML+xExpose)
	if keyword := d.visibility(el); keyword != "" && !expose {
		words = append(words, keyword)
	}
	if expose {
		words = append(words, "expose")
	} else {
		words = append(words, "import")
		if d.boolOf(el, rdf.SysML+pIsImportAll) {
			words = append(words, "all")
		}
	}
	imported, ok := d.stringOf(el, rdf.SysML+pImportedNamespace)
	if !ok {
		return "", d.missing(el, "sysml:"+pImportedNamespace, "an import names the namespace it imports")
	}
	imported = qualifiedNameText(imported)
	switch {
	case d.boolOf(el, rdf.OpenSysML+xRecursive):
		imported += "::**"
	case d.boolOf(el, rdf.OpenSysML+xNamespaceImport):
		imported += "::*"
	}
	words = append(words, imported)
	if filter, ok := d.stringOf(el, rdf.OpenSysML+xFilter); ok {
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
	forName, err := d.referenceText(el, rdf.SysML+pAliasFor)
	if err != nil {
		return "", err
	}
	if forName == "" {
		return "", d.missing(el, "sysml:"+pAliasFor, "an alias names the element it stands for")
	}
	words = append(words, "for", forName)
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
	clients, err := d.referenceList(el, rdf.SysML+pClient)
	if err != nil {
		return "", err
	}
	suppliers, err := d.referenceList(el, rdf.SysML+pSupplier)
	if err != nil {
		return "", err
	}
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

// relationshipElementHead rebuilds a keyword-first relationship member from its
// ordered ends: `specialization Gen subtype A specializes B`.
func (d *decoder) relationshipElementHead(el *element) (string, error) {
	keyword, ok := d.stringOf(el, rdf.OpenSysML+xDeclaredKeyword)
	if !ok {
		return "", d.missing(el, "sysx:"+xDeclaredKeyword,
			"a relationship member is written keyword-first, and the keyword says which form")
	}
	form, ok := relationshipMemberSyntax[keyword]
	if !ok {
		return "", &UnsupportedError{What: fmt.Sprintf("the relationship keyword %q of %s", keyword, el.iri)}
	}
	source, err := d.relationshipEndName(el, form.source)
	if err != nil {
		return "", err
	}
	target, err := d.relationshipEndName(el, form.target)
	if err != nil {
		return "", err
	}
	var words []string
	if keyword := d.visibility(el); keyword != "" {
		words = append(words, keyword)
	}
	if prefix, ok := d.stringOf(el, rdf.OpenSysML+xDeclaredPrefix); ok {
		words = append(words, prefix)
		words = append(words, d.identWords(el)...)
		words = append(words, keyword)
	} else {
		words = append(words, keyword)
		words = append(words, d.identWords(el)...)
	}
	if keyword == "featuring" {
		// `featuring of f by T` names the relationship before its featured end.
		if len(d.identWords(el)) > 0 {
			words = append(words, "of")
		}
		return strings.Join(append(words, source, "by", target), " "), nil
	}
	return strings.Join(append(words, source, form.separator, target), " "), nil
}

// relationshipEndName reads one end of a relationship element, which the
// notation always writes.
func (d *decoder) relationshipEndName(el *element, property string) (string, error) {
	names, err := d.referenceList(el, rdf.SysML+property)
	if err != nil {
		return "", err
	}
	if len(names) != 1 {
		return "", d.missing(el, "sysml:"+property, "a relationship relates exactly one element at each end")
	}
	return names[0], nil
}

func (d *decoder) commentHead(el *element) (string, error) {
	// The keyword is what makes this a declared element rather than lexical
	// trivia, so it is written even when nothing identifies the comment.
	words := []string{"comment"}
	words = append(words, d.identWords(el)...)
	about, err := d.referenceList(el, rdf.SysML+pAnnotatedElement)
	if err != nil {
		return "", err
	}
	if len(about) > 0 {
		words = append(words, "about", strings.Join(about, ", "))
	}
	words = append(words, d.localeWords(el)...)
	body, _ := d.stringOf(el, rdf.SysML+pBody)
	return strings.Join(words, " ") + " /*" + body + "*/", nil
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

func (d *decoder) multiplicityHead(el *element) (string, error) {
	words := []string{"multiplicity"}
	words = append(words, d.identWords(el)...)
	if mult := d.multiplicityText(el); mult != "" {
		words = append(words, mult)
	}
	// A MultiplicitySubset states its bounds by subsetting instead of a range.
	subsets, err := d.referenceText(el, rdf.SysML+relationshipProperty[ast.RelSubsets])
	if err != nil {
		return "", err
	}
	if subsets != "" {
		words = append(words, "subsets", subsets)
	}
	return strings.Join(words, " "), nil
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
	if written, ok := d.stringOf(el, rdf.OpenSysML+xDeclaredKeyword); ok && written != "" {
		return written
	}
	// A declaration that wrote no kind keyword takes its kind from its owner.
	if d.boolOf(el, rdf.OpenSysML+xImplicitKind) {
		return ""
	}
	return canonical
}

func (d *decoder) identWords(el *element) []string {
	var words []string
	if short, ok := d.stringOf(el, rdf.SysML+pDeclaredShortName); ok {
		words = append(words, "<"+nameText(short)+">")
	}
	if name, ok := d.stringOf(el, rdf.SysML+pDeclaredName); ok {
		words = append(words, nameText(name))
	}
	return words
}

// nameText writes a name as the notation spells it: the graph carries the name
// itself, so one that is not a basic name needs its quotes back (KerML §8.2.2).
// A reserved word lexes as a keyword rather than a name, so a name spelling one
// needs the quotes too.
func nameText(name string) string {
	if lexer.IsIdentifier(name) && !lexer.IsKeyword(name) {
		return name
	}
	return "'" + escapeName(name) + "'"
}

// escapeName escapes a quote the name itself contains, which would otherwise
// close the unrestricted name early. The parser keeps the escapes a name was
// written with, so one already escaped is left alone.
func escapeName(name string) string {
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		switch name[i] {
		case '\\':
			b.WriteByte(name[i])
			if i+1 < len(name) {
				i++
				b.WriteByte(name[i])
			}
		case '\'':
			b.WriteString(`\'`)
		default:
			b.WriteByte(name[i])
		}
	}
	return b.String()
}

// qualifiedNameText writes a qualified name segment by segment, since each
// segment is a name of its own and is quoted on its own.
func qualifiedNameText(qname string) string {
	global := strings.HasPrefix(qname, "$::")
	if global {
		qname = strings.TrimPrefix(qname, "$::")
	}
	segments := strings.Split(qname, "::")
	for i, segment := range segments {
		segments[i] = nameText(segment)
	}
	out := strings.Join(segments, "::")
	if global {
		return "$::" + out
	}
	return out
}

func (d *decoder) prefixWords(el *element) []string {
	var words []string
	for _, term := range d.graph.Objects(rdf.IRI(el.iri), rdf.OpenSysML+xPrefixMetadata) {
		words = append(words, term.Value)
	}
	return words
}

// visibility reads the visibility a member was declared with, which the abstract
// syntax states on the membership rather than on the member — except on an
// import, which has a visibility of its own.
func (d *decoder) visibility(el *element) string {
	keyword, ok := d.stringOf(el, rdf.SysML+pVisibility)
	if !ok {
		if m, owned := d.owningMembership[el.iri]; owned {
			keyword, ok = d.graph.Lexical(rdf.IRI(m.iri), rdf.SysML+pVisibility)
		}
	}
	if !ok || keyword == "" {
		return ""
	}
	return visibilityKeyword(visibilityOf(keyword))
}

// relationshipWords renders the typing and specialization clauses of a
// declaration head, in the order the grammar expects. multPart, when given, is
// the multiplicity part the typing clause carries.
func (d *decoder) relationshipWords(el *element, multPart string, skip ...ast.RelationshipKind) ([]string, error) {
	var words []string
	for _, kind := range relationshipOrder {
		if slices.Contains(skip, kind) {
			continue
		}
		targets, err := d.referenceList(el, rdf.SysML+relationshipProperty[kind])
		if err != nil {
			return nil, err
		}
		if len(targets) == 0 {
			continue
		}
		// Conjugation qualifies the type a feature is typed by, not the feature
		// itself: the notation is `port p : ~P` (SysML v2 ConjugatedPortTyping).
		if kind == ast.RelTyping && d.boolOf(el, rdf.SysML+"isConjugated") {
			for i, target := range targets {
				targets[i] = "~" + target
			}
		}
		clause := strings.Join(targets, ", ")
		if kind == ast.RelTyping {
			clause += multPart
		}
		words = append(words, relationshipSyntax[kind], clause)
	}
	return words, nil
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
func (d *decoder) referenceText(el *element, property string) (string, error) {
	list, err := d.referenceList(el, property)
	if err != nil || len(list) == 0 {
		return "", err
	}
	return list[0], nil
}

func (d *decoder) referenceList(el *element, property string) ([]string, error) {
	var out []string
	for _, term := range d.graph.Objects(rdf.IRI(el.iri), property) {
		name, err := d.referenceName(term, el.scope)
		if err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, nil
}

// referenceName renders a reference term as the name to write in source: a
// literal as written, an IRI as its element's qualified name relative to scope.
func (d *decoder) referenceName(term rdf.Term, scope string) (string, error) {
	if term.IsLiteral() {
		if term.Datatype == rdf.OpenSysML+dtExpression {
			return term.Value, nil
		}
		return qualifiedNameText(term.Value), nil
	}
	target, err := d.referencedElement(term.Value)
	if err != nil {
		return "", err
	}
	qname := target.qname
	for {
		if scope == "" {
			return qualifiedNameText(qname), nil
		}
		if rest, found := strings.CutPrefix(qname, scope+"::"); found {
			return qualifiedNameText(rest), nil
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
	if text, ok := el.expressions[property]; ok {
		return text, text != ""
	}
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
