package export

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/rdf"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// Property names in the SysML vocabulary.
const (
	pDeclaredName      = "declaredName"
	pDeclaredShortName = "declaredShortName"
	pQualifiedName     = "qualifiedName"
	pOwningNamespace   = "owningNamespace"
	pVisibility        = "visibility"
	pDirection         = "direction"
	pLowerBound        = "lowerBound"
	pUpperBound        = "upperBound"
	pValue             = "value"
	pImportedNamespace = "importedNamespace"
	pAliasFor          = "aliasedElement"
	pClient            = "client"
	pSupplier          = "supplier"
	pBody              = "body"
	pLanguage          = "language"
	pLocale            = "locale"
	pAnnotatedElement  = "annotatedElement"
	pIsImportAll       = "isImportAll"
	pSourceFeature     = "sourceFeature"
	pTargetFeature     = "targetFeature"
)

// Property names in the Systemica extension namespace: declaration order,
// body presence, and the source text of the constructs whose head this
// mapping keeps verbatim (see the package doc).
const (
	xMemberIndex     = "memberIndex"
	xHasBody         = "hasBody"
	xSourceText      = "sourceText"
	xPrefixMetadata  = "prefixMetadata"
	xFilter          = "filter"
	xNamespaceImport = "isNamespaceImport"
	xRecursive       = "isRecursive"
	xExpose          = "isExpose"
	xDeclaredKeyword = "declaredKeyword"
)

// Metaclass names for the constructs that have no SysML metaclass of their own
// in this mapping.
const (
	mAlias        = "Alias"
	mFilter       = "FilterMember"
	mMultiplicity = "MultiplicityDeclaration"
)

// boolProperty pairs an RDF property name with the AST flag it mirrors. Only
// true values are written, so an absent property reads as false.
type boolProperty struct {
	name  string
	value bool
}

// UnsupportedError reports a construct the conversion cannot represent. It
// names the element so the user can find it, rather than converting a model
// that silently lost part of itself.
type UnsupportedError struct {
	What string
	Note string
}

func (e *UnsupportedError) Error() string {
	if e.Note == "" {
		return fmt.Sprintf("cannot convert %s", e.What)
	}
	return fmt.Sprintf("cannot convert %s: %s", e.What, e.Note)
}

// ToRDF converts a parsed document into an RDF graph. file must be the source
// the tree was parsed from: expression-valued positions (feature values,
// multiplicity bounds, filter conditions) are carried as their source text, so
// the conversion needs the bytes as well as the tree.
func ToRDF(file *source.SourceFile, root *ast.RootNamespace) (*rdf.Graph, error) {
	if file == nil || root == nil {
		return nil, &UnsupportedError{What: "an empty document", Note: "nothing to convert"}
	}
	e := &encoder{file: file, graph: rdf.NewGraph(), declared: map[string]bool{}}
	// The first pass records which qualified names this document declares, so
	// the second can decide whether a relationship target is a link to an
	// element in the graph or a name that resolves outside it.
	if err := e.collect(root.Members, ""); err != nil {
		return nil, err
	}
	if err := e.encode(root.Members, "", rdf.Term{}); err != nil {
		return nil, err
	}
	return e.graph, nil
}

type encoder struct {
	file     *source.SourceFile
	graph    *rdf.Graph
	declared map[string]bool
}

// declaredKeyword records the kind keyword as written when it is a synonym of
// the canonical one, so the notation comes back as the author spelled it rather
// than rewritten. A synonym written in a shape the decoder cannot rebuild is
// refused instead: returning the canonical keyword would be a different model.
func (e *encoder) declaredKeyword(subject rdf.Term, node ast.Node, written, canonical, named string) error {
	if written == "" || written == canonical {
		return nil
	}
	// A two-word kind keyword such as `use case` is written one word at a time;
	// its last word is not a synonym of the whole.
	for _, word := range strings.Fields(canonical) {
		if written == word {
			return nil
		}
	}
	// The keyword introduces a declaration whose subject is its name. Without
	// one the keyword takes an inline reference instead (`perform a`), a shape
	// rebuilt from the relationship rather than the head.
	if named == "" {
		return &UnsupportedError{
			What: fmt.Sprintf("the `%s` declaration at %s", written, e.where(node)),
			Note: fmt.Sprintf("it names no element of its own, so the notation cannot be rebuilt from the graph and would come back as `%s`, a different declaration", canonical),
		}
	}
	e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(written))
	return nil
}

// collect walks the tree recording every qualified name it declares. A name
// declared twice in one namespace is reported: the qualified name is an
// element's identity in the graph, so two such members would merge into one.
func (e *encoder) collect(members []ast.Node, owner string) error {
	for i, member := range members {
		node, _ := unwrapMember(member)
		name, children := declaredNameAndMembers(node)
		fqn := qualify(owner, name, i)
		if fqn == "" {
			continue
		}
		if e.declared[fqn] {
			return &UnsupportedError{
				What: fmt.Sprintf("the duplicate declaration of %q at %s", name, e.where(node)),
				Note: "a name identifies an element in the graph, so two members of one namespace cannot share it",
			}
		}
		e.declared[fqn] = true
		if err := e.collect(children, fqn); err != nil {
			return err
		}
	}
	return nil
}

// encode walks the members of one namespace, emitting the triples for each.
func (e *encoder) encode(members []ast.Node, owner string, ownerTerm rdf.Term) error {
	for i, member := range members {
		node, visibility := unwrapMember(member)
		if node == nil {
			continue
		}
		if err := e.encodeMember(node, visibility, owner, ownerTerm, i); err != nil {
			return err
		}
	}
	return nil
}

func (e *encoder) encodeMember(node ast.Node, visibility ast.Visibility, owner string, ownerTerm rdf.Term, index int) error {
	name, _ := declaredNameAndMembers(node)
	fqn := qualify(owner, name, index)
	subject := rdf.ElementIRI(fqn)

	// A metaclass name this mapping invents is typed in the Systemica namespace,
	// so a consumer can tell it from the standard OMG vocabulary.
	head := func(metaclass rdf.Term) {
		e.graph.Add(subject, rdf.IRI(rdf.RDFType), metaclass)
		e.graph.Add(subject, e.sysml(pQualifiedName), rdf.String(fqn))
		if ownerTerm.Value != "" {
			e.graph.Add(subject, e.sysml(pOwningNamespace), ownerTerm)
		}
		e.graph.Add(subject, e.sysx(xMemberIndex), rdf.Int(index))
		if keyword := visibilityKeyword(visibility); keyword != "" {
			e.graph.Add(subject, e.sysml(pVisibility), rdf.String(keyword))
		}
	}

	switch n := node.(type) {
	case *ast.Package:
		head(rdf.SysMLTerm("Package"))
		e.ident(subject, n.Ident)
		e.flags(subject, []boolProperty{
			{"isLibraryPackage", n.IsLibrary},
			{"isStandardLibraryPackage", n.IsStandard},
		})
		e.prefixes(subject, n.Prefixes)
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Members, fqn, subject)

	case *ast.Namespace:
		head(rdf.SysMLTerm("Namespace"))
		e.ident(subject, n.Ident)
		e.prefixes(subject, n.Prefixes)
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Members, fqn, subject)

	case *ast.Definition:
		metaclass, ok := definitionMetaclass[n.Kind]
		if !ok {
			return &UnsupportedError{What: fmt.Sprintf("definition kind %q at %s", n.Kind, e.where(n))}
		}
		head(rdf.SysMLTerm(metaclass))
		e.ident(subject, n.Ident)
		if err := e.declaredKeyword(subject, n, n.Keyword, definitionKeyword(n.Kind), n.Ident.Name); err != nil {
			return err
		}
		e.flags(subject, []boolProperty{
			{"isAbstract", n.IsAbstract},
			{"isVariation", n.IsVariation},
			{"isAll", n.IsAll},
			{"isConstant", n.IsConstant},
			{"isEvent", n.IsEvent},
		})
		e.prefixes(subject, n.Prefixes)
		e.relationships(subject, owner, n.Relationships)
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Members, fqn, subject)

	case *ast.Usage:
		metaclass, ok := usageMetaclass[n.Kind]
		if !ok {
			return &UnsupportedError{What: fmt.Sprintf("usage kind %q at %s", n.Kind, e.where(n))}
		}
		head(rdf.SysMLTerm(metaclass))
		e.ident(subject, n.Ident)
		// A verbatim head is reproduced as written, so its keyword needs no
		// reconstructing and never has to be refused.
		if !verbatimUsage(n) {
			if err := e.declaredKeyword(subject, n, n.Keyword, usageKeyword(n.Kind), n.Ident.Name); err != nil {
				return err
			}
		}
		e.flags(subject, []boolProperty{
			{"isAbstract", n.IsAbstract},
			{"isReference", n.IsReference},
			{"isAll", n.IsAll},
			{"isEnd", n.IsEnd},
			{"isChain", n.IsChain},
			{"isConstant", n.IsConstant},
			{"isEvent", n.IsEvent},
			{"isIndividual", n.IsIndividual},
			{"isSnapshot", n.Portion == ast.PortionSnapshot},
			{"isTimeslice", n.Portion == ast.PortionTimeslice},
			{"isComposite", n.IsComposite},
			{"isDerived", n.IsDerived},
			{"isOrdered", n.IsOrdered},
			{"isNonunique", n.IsNonunique},
			{"isConjugated", n.HasConjugatedTyping()},
			{"isAccept", n.IsAccept},
			{"isResult", n.IsResult},
		})
		e.prefixes(subject, n.Prefixes)
		if keyword := directionKeyword(n.Direction); keyword != "" {
			e.graph.Add(subject, e.sysml(pDirection), rdf.String(keyword))
		}
		e.relationships(subject, owner, n.Relationships)
		e.multiplicity(subject, n.Multiplicity)
		if n.Value != nil {
			e.graph.Add(subject, e.sysml(pValue), rdf.String(e.text(n.Value)))
		}
		// A declaration head that binds ends (connect/bind/flow/succession),
		// a transition, an accept action or a satisfy usage is kept as source
		// text: its head is not reconstructible from the properties above.
		if verbatimUsage(n) {
			e.graph.Add(subject, e.sysx(xSourceText), rdf.String(e.text(n)))
			return nil
		}
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Members, fqn, subject)

	case *ast.Import:
		head(rdf.SysMLTerm("Import"))
		e.graph.Add(subject, e.sysml(pImportedNamespace), rdf.String(qualifiedText(n.Imported)))
		e.flags(subject, []boolProperty{
			{pIsImportAll, n.IsAll},
			{xNamespaceImport, n.Kind == ast.ImportNamespace},
			{xRecursive, n.IsRecursive},
			{xExpose, n.IsExpose},
		})
		if n.FilterExpr != nil {
			e.graph.Add(subject, e.sysx(xFilter), rdf.String(e.text(n.FilterExpr)))
		}
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Body, fqn, subject)

	case *ast.Alias:
		head(rdf.SystemicaTerm(mAlias))
		e.ident(subject, n.Ident)
		e.graph.Add(subject, e.sysml(pAliasFor), e.reference(owner, qualifiedText(n.For)))
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Body, fqn, subject)

	case *ast.Dependency:
		head(rdf.SysMLTerm("Dependency"))
		e.ident(subject, n.Ident)
		for _, client := range n.Clients {
			e.graph.Add(subject, e.sysml(pClient), e.reference(owner, qualifiedText(client)))
		}
		for _, supplier := range n.Suppliers {
			e.graph.Add(subject, e.sysml(pSupplier), e.reference(owner, qualifiedText(supplier)))
		}
		e.prefixes(subject, n.Prefixes)
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Body, fqn, subject)

	case *ast.Comment:
		head(rdf.SysMLTerm("Comment"))
		e.ident(subject, n.Ident)
		for _, about := range n.About {
			e.graph.Add(subject, e.sysml(pAnnotatedElement), e.reference(owner, qualifiedText(about)))
		}
		if n.Locale != "" {
			e.graph.Add(subject, e.sysml(pLocale), rdf.String(unquote(n.Locale)))
		}
		e.graph.Add(subject, e.sysml(pBody), rdf.String(commentBody(e.file.Text(n.BodySpan))))
		return nil

	case *ast.Documentation:
		head(rdf.SysMLTerm("Documentation"))
		e.ident(subject, n.Ident)
		if n.Locale != "" {
			e.graph.Add(subject, e.sysml(pLocale), rdf.String(unquote(n.Locale)))
		}
		e.graph.Add(subject, e.sysml(pBody), rdf.String(commentBody(e.file.Text(n.BodySpan))))
		return nil

	case *ast.TextualRepresentation:
		head(rdf.SysMLTerm("TextualRepresentation"))
		e.ident(subject, n.Ident)
		e.graph.Add(subject, e.sysml(pLanguage), rdf.String(unquote(n.Language)))
		e.graph.Add(subject, e.sysml(pBody), rdf.String(commentBody(e.file.Text(n.BodySpan))))
		return nil

	case *ast.MultiplicityDecl:
		head(rdf.SystemicaTerm(mMultiplicity))
		e.ident(subject, n.Ident)
		e.multiplicity(subject, n.Range)
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Members, fqn, subject)

	case *ast.FilterMember:
		head(rdf.SystemicaTerm(mFilter))
		e.graph.Add(subject, e.sysx(xFilter), rdf.String(e.text(n.Condition)))
		return nil

	case *ast.SuccessionEdge:
		// A succession, however it was written: as its own member (`then a b;`)
		// or as a `then` attached to a member, which the parser desugars to this
		// same node. Its ends are what carries execution order, so they are
		// mapped as references rather than as the text of the declaration.
		source, target := qualifiedText(n.Source), qualifiedText(n.Target)
		if source == "" || target == "" {
			return &UnsupportedError{
				What: fmt.Sprintf("the succession at %s", e.where(n)),
				Note: "it does not name both of the members it sequences, so the order it declares cannot be written back",
			}
		}
		head(rdf.SysMLTerm("SuccessionAsUsage"))
		e.graph.Add(subject, e.sysml(pSourceFeature), e.reference(owner, source))
		e.graph.Add(subject, e.sysml(pTargetFeature), e.reference(owner, target))
		return nil

	case *ast.ErrorNode:
		return &UnsupportedError{
			What: fmt.Sprintf("the malformed declaration at %s", e.where(n)),
			Note: "fix the syntax error before converting",
		}
	}
	return &UnsupportedError{What: fmt.Sprintf("the %T at %s", node, e.where(node))}
}

// verbatimUsage reports whether a usage's declaration head has to be carried as
// source text rather than rebuilt from properties.
//
// An accept parameter is not verbatim: it is a synthetic member of the accept
// shorthand, fully described by its direction, type and isAccept flag, and the
// printer rebuilds the shorthand from those.
func verbatimUsage(n *ast.Usage) bool {
	if n.IsAccept {
		return false
	}
	if len(n.ConnectorEnds) > 0 || n.FlowEnds != nil {
		return true
	}
	switch n.Kind {
	case ast.UsageConnector, ast.UsageSuccession, ast.UsageBinding, ast.UsageFlow,
		ast.UsageTransition, ast.UsageSatisfy:
		return true
	}
	return false
}

func (e *encoder) sysml(name string) rdf.Term { return rdf.SysMLTerm(name) }
func (e *encoder) sysx(name string) rdf.Term  { return rdf.SystemicaTerm(name) }

func (e *encoder) ident(subject rdf.Term, ident ast.Identification) {
	if ident.Name != "" {
		e.graph.Add(subject, e.sysml(pDeclaredName), rdf.String(ident.Name))
	}
	if ident.ShortName != "" {
		e.graph.Add(subject, e.sysml(pDeclaredShortName), rdf.String(ident.ShortName))
	}
}

func (e *encoder) flags(subject rdf.Term, flags []boolProperty) {
	for _, flag := range flags {
		if !flag.value {
			continue
		}
		property := e.sysml(flag.name)
		if strings.HasPrefix(flag.name, "is") && isExtensionFlag(flag.name) {
			property = e.sysx(flag.name)
		}
		e.graph.Add(subject, property, rdf.Bool(true))
	}
}

// isExtensionFlag reports whether a flag lives in the Systemica namespace
// because the SysML metamodel has no such property.
func isExtensionFlag(name string) bool {
	switch name {
	case "isLibraryPackage", "isStandardLibraryPackage", xNamespaceImport, xRecursive, xExpose:
		return true
	}
	return false
}

func (e *encoder) prefixes(subject rdf.Term, prefixes []*ast.PrefixMetadata) {
	for _, prefix := range prefixes {
		if prefix == nil {
			continue
		}
		e.graph.Add(subject, e.sysx(xPrefixMetadata), rdf.String(e.text(prefix)))
	}
}

func (e *encoder) relationships(subject rdf.Term, owner string, rels []*ast.Relationship) {
	for _, rel := range rels {
		if rel == nil || rel.Target == nil {
			continue
		}
		property, ok := relationshipProperty[rel.Kind]
		if !ok {
			continue
		}
		e.graph.Add(subject, e.sysml(property), e.reference(owner, e.text(rel.Target)))
	}
}

func (e *encoder) multiplicity(subject rdf.Term, mult *ast.Multiplicity) {
	if mult == nil {
		return
	}
	// The parser puts the single bound of `[n]` in Lower; the language reads
	// that as lower and upper both being n, so it is written as the upper bound
	// alone and the printer renders it back as `[n]`.
	if !mult.IsRange {
		if mult.Lower != nil {
			e.graph.Add(subject, e.sysml(pUpperBound), rdf.String(e.text(mult.Lower)))
		}
		return
	}
	if mult.Lower != nil {
		e.graph.Add(subject, e.sysml(pLowerBound), rdf.String(e.text(mult.Lower)))
	}
	if mult.Upper != nil {
		e.graph.Add(subject, e.sysml(pUpperBound), rdf.String(e.text(mult.Upper)))
	}
}

// reference renders a name reference as a link when it names an element this
// document declares, and as the written name otherwise — a type from the
// standard library is a name, not an element of this graph.
//
// The name is written relative to the referring element, so resolution walks
// outwards from its owner the way the language's own scoping does; the link is
// only made when the walk finds a declaration, which keeps the graph from
// claiming an element that is really an import from elsewhere.
func (e *encoder) reference(owner, name string) rdf.Term {
	if name == "" {
		return rdf.String("")
	}
	for scope := owner; ; {
		candidate := name
		if scope != "" {
			candidate = scope + "::" + name
		}
		if e.declared[candidate] {
			return rdf.ElementIRI(candidate)
		}
		if scope == "" {
			break
		}
		cut := strings.LastIndex(scope, "::")
		if cut < 0 {
			scope = ""
			continue
		}
		scope = scope[:cut]
	}
	return rdf.String(name)
}

func (e *encoder) text(node ast.Node) string {
	if node == nil {
		return ""
	}
	return strings.TrimSpace(e.file.Text(node.Span()))
}

func (e *encoder) where(node ast.Node) string {
	pos := e.file.Lines().PosAt(node.Span().Offset)
	return fmt.Sprintf("%s:%d:%d", e.file.Name(), pos.Line, pos.Col)
}

// unwrapMember returns the declaration inside a membership wrapper together
// with the visibility the wrapper declared.
func unwrapMember(member ast.Node) (ast.Node, ast.Visibility) {
	switch n := member.(type) {
	case *ast.Membership:
		if n.Member == nil {
			return nil, n.Visibility
		}
		return n.Member, n.Visibility
	case *ast.Import:
		return n, n.Visibility
	case *ast.Alias:
		return n, n.Visibility
	}
	return member, ast.VisibilityDefault
}

// declaredNameAndMembers returns the name a declaration introduces and the
// members it owns, for the node kinds that have either.
func declaredNameAndMembers(node ast.Node) (string, []ast.Node) {
	switch n := node.(type) {
	case *ast.Package:
		return n.Ident.Name, n.Members
	case *ast.Namespace:
		return n.Ident.Name, n.Members
	case *ast.Definition:
		return n.Ident.Name, n.Members
	case *ast.Usage:
		if verbatimUsage(n) {
			return n.Ident.Name, nil
		}
		return n.Ident.Name, n.Members
	case *ast.Import:
		return "", n.Body
	case *ast.Alias:
		return n.Ident.Name, n.Body
	case *ast.Dependency:
		return n.Ident.Name, n.Body
	case *ast.MultiplicityDecl:
		return n.Ident.Name, n.Members
	case *ast.Comment:
		return n.Ident.Name, nil
	case *ast.Documentation:
		return n.Ident.Name, nil
	case *ast.TextualRepresentation:
		return n.Ident.Name, nil
	}
	return "", nil
}

// qualify builds the qualified name of a member. An unnamed declaration is
// addressed by its position in its owner, which keeps every element in the
// graph identifiable and the mapping reversible.
func qualify(owner, name string, index int) string {
	if name == "" {
		name = fmt.Sprintf("@%d", index)
	}
	if owner == "" {
		return name
	}
	return owner + "::" + name
}

func qualifiedText(name *ast.QualifiedName) string {
	if name == nil {
		return ""
	}
	parts := make([]string, 0, len(name.Parts))
	for _, part := range name.Parts {
		parts = append(parts, part.Text)
	}
	out := strings.Join(parts, "::")
	if name.Global {
		return "$::" + out
	}
	return out
}

// unquote strips the quotes the parser keeps on a string token, so the graph
// carries the value rather than its notation.
func unquote(text string) string {
	if len(text) >= 2 && strings.HasPrefix(text, `"`) && strings.HasSuffix(text, `"`) {
		return text[1 : len(text)-1]
	}
	return text
}

// commentBody strips the /* */ delimiters from a comment token, leaving the
// text the printer re-wraps.
func commentBody(raw string) string {
	raw = strings.TrimPrefix(raw, "/*")
	raw = strings.TrimSuffix(raw, "*/")
	return raw
}
