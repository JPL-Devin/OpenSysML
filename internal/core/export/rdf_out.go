package export

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
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

// Property names in the OpenSysML extension namespace: declaration order,
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
	xDeclaredPrefix  = "declaredPrefix"
	xImplicitKind    = "isKindImplicit"
	xCondition       = "condition"
	xRelatedFeature  = "relatedFeature"
	xEndIndex        = "endIndex"
	xEndRole         = "endRole"
	xEndForm         = "endForm"
	xEndVerb         = "endVerb"
	xSourceMember    = "sourceMember"
	xTargetMember    = "targetMember"
)

// The notations a head that binds ends writes its ends in, stated as
// sysx:endForm. See docs/reference/rdf-mapping.md § End-binding heads.
const (
	formTo        = "to"        // connect a to b
	formNary      = "nary"      // connect (a, b, c)
	formEquals    = "equals"    // bind a = b
	formFirstThen = "firstThen" // succession first a then b
	formFromTo    = "fromTo"    // flow of P from a to b
	formFlowTo    = "flowTo"    // flow a to b
	formThen      = "then"      // then b, whose source end is the member before it
	formSatisfy   = "satisfy"   // satisfy R by v, whose requirement is written bare
)

// dtExpression is the datatype of a relationship target that is not a name but
// an expression, carried as the text it was written as rather than as a name.
const dtExpression = "Expression"

// Metaclass names for the constructs that have no SysML metaclass of their own
// in this mapping.
const (
	mAlias        = "Alias"
	mFilter       = "FilterMember"
	mMultiplicity = "MultiplicityDeclaration"
	// The members that state a condition or a result rather than declaring a
	// feature: the conditions of a constraint body, a requirement's assumptions
	// and required conditions, and a calculation's result expression.
	mConstraint = "ConstraintMember"
	mAssume     = "AssumeMember"
	mRequire    = "RequireMember"
	mResult     = "ResultMember"
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
// the tree was parsed from: an expression-valued position carries the notation
// it was written as alongside the graph of the expression, so the conversion
// needs the bytes as well as the tree.
func ToRDF(file *source.SourceFile, root *ast.RootNamespace) (*rdf.Graph, error) {
	if file == nil || root == nil {
		return nil, &UnsupportedError{What: "an empty document", Note: "nothing to convert"}
	}
	e := &encoder{
		file:     file,
		graph:    rdf.NewGraph(),
		declared: map[string]bool{},
		fqn:      map[ast.Node]string{},
	}
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
	// fqn is the qualified name of each member node, which is how a succession
	// end the notation leaves unnamed addresses the member it binds.
	fqn map[ast.Node]string
}

// declaredKeyword records the kind keyword as written when it is a synonym of
// the canonical one, so the notation comes back as the author spelled it rather
// than rewritten. A synonym written in a shape the decoder cannot rebuild is
// refused instead: returning the canonical keyword would be a different model.
// referenced states whether the declaration names an existing feature, the
// shape a keyword with no name of its own is rebuilt from.
func (e *encoder) declaredKeyword(subject rdf.Term, node ast.Node, written, canonical, named string, referenced bool) error {
	if written == "" || written == canonical {
		return nil
	}
	// The keyword introduces a declaration whose subject is its name. Without
	// one the keyword takes an inline reference instead (`perform a`), a shape
	// rebuilt from the relationship rather than the head.
	if named == "" && !(referenced && referenceMemberKeyword(written)) {
		// A shorter spelling of a multi-word kind keyword (`verification` for
		// `verification case`) states the same kind, so it needs no name.
		for _, word := range strings.Fields(canonical) {
			if written == word {
				return nil
			}
		}
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
		e.fqn[node] = fqn
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

	// A metaclass name this mapping invents is typed in the OpenSysML namespace,
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
		if err := e.prefixes(subject, n, n.Prefixes); err != nil {
			return err
		}
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Members, fqn, subject)

	case *ast.Namespace:
		head(rdf.SysMLTerm("Namespace"))
		e.ident(subject, n.Ident)
		if err := e.prefixes(subject, n, n.Prefixes); err != nil {
			return err
		}
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Members, fqn, subject)

	case *ast.Definition:
		metaclass, ok := definitionMetaclass[n.Kind]
		if !ok {
			return &UnsupportedError{What: fmt.Sprintf("definition kind %q at %s", n.Kind, e.where(n))}
		}
		head(rdf.SysMLTerm(metaclass))
		e.ident(subject, n.Ident)
		if err := e.declaredKeyword(subject, n, n.Keyword, definitionKeyword(n.Kind), n.Ident.Name, false); err != nil {
			return err
		}
		e.flags(subject, []boolProperty{
			{"isAbstract", n.IsAbstract},
			{"isVariation", n.IsVariation},
			{"isAll", n.IsAll},
			{"isConstant", n.IsConstant},
			{"isEvent", n.IsEvent},
		})
		if err := e.prefixes(subject, n, n.Prefixes); err != nil {
			return err
		}
		e.relationships(subject, owner, n.Relationships)
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Members, fqn, subject)

	case *ast.Usage:
		metaclass, ok := usageMetaclass[n.Kind]
		if !ok {
			return &UnsupportedError{What: fmt.Sprintf("usage kind %q at %s", n.Kind, e.where(n))}
		}
		head(rdf.SysMLTerm(metaclass))
		if !shorthandRelationship(n) {
			e.ident(subject, n.Ident)
		}
		switch {
		case verbatimUsage(n):
			// A verbatim head is reproduced as written, so its keyword needs no
			// reconstructing and never has to be refused.
		case bareAcceptNode(n, e.text(n)):
			// The `action` of an accept node is optional and the parser records it
			// either way, so what the author wrote is read from the source.
			e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String("accept"))
		case n.Keyword == "" && !e.wroteKindKeyword(n):
			// A feature written with no kind keyword (`in x : Real;`) takes its kind
			// from its owner; writing that kind's keyword back would declare more.
			e.graph.Add(subject, e.sysx(xImplicitKind), rdf.Bool(true))
		default:
			if err := e.declaredKeyword(subject, n, n.Keyword, usageKeyword(n.Kind), n.Ident.Name, referencesFeature(n)); err != nil {
				return err
			}
		}
		// The prefix a kind keyword was qualified with (`assert constraint c`)
		// states what the usage is for, so it is part of the declaration.
		if n.PrefixKeyword != "" {
			e.graph.Add(subject, e.sysx(xDeclaredPrefix), rdf.String(n.PrefixKeyword))
		}
		e.flags(subject, []boolProperty{
			{"isAbstract", n.IsAbstract},
			{"isVariation", n.IsVariation},
			{"isVariant", n.IsVariant},
			{"isNegated", n.IsNegated},
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
		if err := e.prefixes(subject, n, n.Prefixes); err != nil {
			return err
		}
		if keyword := directionKeyword(n.Direction); keyword != "" {
			e.graph.Add(subject, e.sysml(pDirection), rdf.String(keyword))
		}
		e.relationships(subject, owner, n.Relationships)
		e.multiplicity(subject, owner, n.Multiplicity)
		e.expression(subject, e.sysml(pValue), pValue, owner, n.Value)
		// A declaration head that binds ends (connect/bind/flow/succession),
		// a transition, an accept action or a satisfy usage is kept as source
		// text: its head is not reconstructible from the properties above.
		if verbatimUsage(n) {
			e.graph.Add(subject, e.sysx(xSourceText), rdf.String(e.text(n)))
			e.bindingEnds(subject, owner, n)
			e.endForm(subject, n)
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
		e.expression(subject, e.sysx(xFilter), xFilter, owner, n.FilterExpr)
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Body, fqn, subject)

	case *ast.Alias:
		head(rdf.OpenSysMLTerm(mAlias))
		e.ident(subject, n.Ident)
		e.graph.Add(subject, e.sysml(pAliasFor), e.reference(owner, qualifiedText(n.For)))
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Body, fqn, subject)

	case *ast.RelationshipMember:
		form, ok := relationshipElementForm[n.Kind]
		if n.Conjugated {
			form, ok = conjugationForm, true
		}
		if !ok {
			return &UnsupportedError{What: fmt.Sprintf("the %s relationship at %s", n.Keyword, e.where(n))}
		}
		head(rdf.SysMLTerm(form.metaclass))
		e.ident(subject, n.Ident)
		e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(n.Keyword))
		if n.PrefixKeyword != "" {
			e.graph.Add(subject, e.sysx(xDeclaredPrefix), rdf.String(n.PrefixKeyword))
		}
		e.relationshipEnd(subject, owner, form.source, n.Source)
		e.relationshipEnd(subject, owner, form.target, n.Target)
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Members, fqn, subject)

	case *ast.Dependency:
		head(rdf.SysMLTerm("Dependency"))
		e.ident(subject, n.Ident)
		for _, client := range n.Clients {
			e.graph.Add(subject, e.sysml(pClient), e.reference(owner, qualifiedText(client)))
		}
		for _, supplier := range n.Suppliers {
			e.graph.Add(subject, e.sysml(pSupplier), e.reference(owner, qualifiedText(supplier)))
		}
		if err := e.prefixes(subject, n, n.Prefixes); err != nil {
			return err
		}
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
		head(rdf.OpenSysMLTerm(mMultiplicity))
		e.ident(subject, n.Ident)
		e.multiplicity(subject, owner, n.Range)
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Members, fqn, subject)

	case *ast.ConstraintMember:
		// A condition of a constraint body, written inline (`assert x > 0;`) or
		// as a nested constraint (`assert constraint [name] { … }`).
		head(rdf.OpenSysMLTerm(mConstraint))
		if n.Name != "" {
			e.graph.Add(subject, e.sysml(pDeclaredName), rdf.String(n.Name))
		}
		if n.Keyword != "" {
			e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(n.Keyword))
		}
		e.flags(subject, []boolProperty{{"isNegated", n.IsNegated}})
		return e.condition(subject, fqn, owner, n.Expression, nil, n.Body)

	case *ast.AssumeMember:
		head(rdf.OpenSysMLTerm(mAssume))
		return e.condition(subject, fqn, owner, n.Expression, n.Reference, n.Body)

	case *ast.RequireMember:
		head(rdf.OpenSysMLTerm(mRequire))
		return e.condition(subject, fqn, owner, n.Expression, n.Reference, n.Body)

	case *ast.ResultMember:
		head(rdf.OpenSysMLTerm(mResult))
		e.expression(subject, e.sysml(pValue), pValue, owner, n.Expression)
		return nil

	case *ast.SubjectMember:
		// A subject parameter is a usage of its own (SysML v2 8.2.2.16), so it
		// is mapped as the SubjectMembership a `subject s : X` declares.
		head(rdf.SysMLTerm(usageMetaclass[ast.UsageSubject]))
		if n.Name != "" {
			e.graph.Add(subject, e.sysml(pDeclaredName), rdf.String(n.Name))
		}
		if n.TypeRef != nil {
			e.graph.Add(subject, e.sysml(relationshipProperty[ast.RelTyping]), e.reference(owner, qualifiedText(n.TypeRef)))
		}
		e.relationships(subject, owner, n.Relationships)
		e.multiplicity(subject, owner, n.Multiplicity)
		e.expression(subject, e.sysml(pValue), pValue, owner, n.BindingExpr)
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Body, fqn, subject)

	case *ast.FilterMember:
		head(rdf.OpenSysMLTerm(mFilter))
		e.expression(subject, e.sysx(xFilter), xFilter, owner, n.Condition)
		return nil

	case *ast.ErrorNode:
		return &UnsupportedError{
			What: fmt.Sprintf("the malformed declaration at %s", e.where(n)),
			Note: "fix the syntax error before converting",
		}
	}
	// A behavioral node — a control node, statement, loop, conditional, state or
	// transition — is mapped by the behavior half of this encoder.
	if handled, err := e.encodeBehavior(node, head, subject, fqn, owner, index); handled {
		return err
	}
	return &UnsupportedError{
		What: fmt.Sprintf("the %s at %s", nodeDescription(node), e.where(node)),
		Note: rdfLimitationsNote,
	}
}

// condition emits the three forms a condition member is written in: an inline
// expression, a reference to the constraint it states (`require R { … }`), or a
// nested constraint stating its conditions in a body.
func (e *encoder) condition(subject rdf.Term, fqn, owner string, expr ast.Node, ref *ast.QualifiedName, body []ast.Node) error {
	if expr != nil {
		e.expression(subject, e.sysx(xCondition), xCondition, owner, expr)
		return nil
	}
	if ref != nil {
		e.graph.Add(subject, e.sysml(relationshipProperty[ast.RelReferences]), e.reference(owner, qualifiedText(ref)))
	}
	// Both remaining forms — a nested constraint and the constraint a member
	// names — are written with a body, whether or not it has members.
	e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(true))
	return e.encode(body, fqn, subject)
}

// shorthandRelationship reports whether a usage's identification is the first
// end of a shorthand head (`bind x = y;`) rather than a name it declares; the
// named form spells the kind out (`binding b bind x = y;`).
func shorthandRelationship(n *ast.Usage) bool {
	return n.Kind == ast.UsageBinding && n.Keyword == "bind"
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

// bindingEnds states the features a binding head relates as structure beside the
// text it is kept as, so a consumer reads the ends without reading notation.
func (e *encoder) bindingEnds(subject rdf.Term, owner string, n *ast.Usage) {
	for i, end := range n.ConnectorEnds {
		if end == nil {
			continue
		}
		e.bindingEnd(subject, owner, fmt.Sprintf("end%d", i), i, "", end.Target)
	}
	if n.FlowEnds != nil {
		e.bindingEnd(subject, owner, "flowSource", 0, "source", n.FlowEnds.From)
		e.bindingEnd(subject, owner, "flowTarget", 1, "target", n.FlowEnds.To)
		e.bindingEnd(subject, owner, "flowPayload", -1, "payload", n.FlowEnds.Payload)
	}
}

// bindingEnd emits one end as an expression node, tagged with its position.
func (e *encoder) bindingEnd(subject rdf.Term, owner, slot string, index int, role string, target ast.Node) {
	if target == nil {
		return
	}
	e.expression(subject, e.sysx(xRelatedFeature), slot, owner, target)
	end := rdf.ExpressionIRI(subject, slot)
	if index >= 0 {
		e.graph.Add(end, e.sysx(xEndIndex), rdf.Int(index))
	}
	if role != "" {
		e.graph.Add(end, e.sysx(xEndRole), rdf.String(role))
	}
}

func (e *encoder) sysml(name string) rdf.Term { return rdf.SysMLTerm(name) }
func (e *encoder) sysx(name string) rdf.Term  { return rdf.OpenSysMLTerm(name) }

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

// isExtensionFlag reports whether a flag lives in the OpenSysML namespace
// because the SysML metamodel has no such property.
func isExtensionFlag(name string) bool {
	switch name {
	case "isLibraryPackage", "isStandardLibraryPackage", xNamespaceImport, xRecursive, xExpose:
		return true
	}
	return false
}

func (e *encoder) prefixes(subject rdf.Term, node ast.Node, prefixes []*ast.PrefixMetadata) error {
	for _, prefix := range prefixes {
		if prefix == nil {
			continue
		}
		written, err := e.prefixText(node, prefix)
		if err != nil {
			return err
		}
		e.graph.Add(subject, e.sysx(xPrefixMetadata), rdf.String(written))
	}
	return nil
}

// prefixText writes an annotation as the notation it was written as. The node
// carries no span of its own, so its sigil is read from ahead of the type it
// names; a prefix that cannot be rebuilt that way is refused, not dropped.
func (e *encoder) prefixText(node ast.Node, prefix *ast.PrefixMetadata) (string, error) {
	at := ast.Node(prefix)
	if prefix.Type != nil {
		at = prefix.Type
	}
	unsupported := &UnsupportedError{
		What: fmt.Sprintf("the metadata annotation at %s", e.where(at)),
		Note: "its notation cannot be rebuilt from the graph, and a graph without it would be a different model; " + rdfLimitationsNote,
	}
	// An annotation the parser records ahead of the declaration it belongs to
	// would be written back onto a different element.
	if prefix.Type == nil || len(prefix.Body) > 0 || prefix.Type.Span().Offset >= node.Span().End() {
		return "", unsupported
	}
	name := e.text(prefix.Type)
	sigil := e.sigilBefore(prefix.Type.Span().Offset)
	if name == "" || sigil == "" {
		return "", unsupported
	}
	return sigil + name, nil
}

// wroteKindKeyword reports whether the declaration spells its kind keyword out.
// A directed usage (`in attribute speed`) does not record the keyword it wrote,
// so the words ahead of the name are read from the source.
func (e *encoder) wroteKindKeyword(n *ast.Usage) bool {
	keyword := usageKeyword(n.Kind)
	if keyword == "" {
		return false
	}
	start, head := n.Span().Offset, n.Span().End()
	if name := n.Ident.NameSpan; name.Len > 0 && name.Offset < head {
		head = name.Offset
	}
	if short := n.Ident.ShortNameSpan; short.Len > 0 && short.Offset < head {
		head = short.Offset
	}
	if head <= start {
		return false
	}
	// A keyword inside a comment is trivia the declaration does not state, so the
	// comments are dropped before the words are read.
	text := withoutComments(e.file.Text(source.Span{Offset: start, Len: head - start}))
	// An unnamed declaration's keyword, if it wrote one, is ahead of everything
	// its head can state.
	if cut := strings.IndexAny(text, ":=;[{"); cut >= 0 {
		text = text[:cut]
	}
	written := strings.Fields(text)
	for _, word := range strings.Fields(keyword) {
		if !slices.Contains(written, word) {
			return false
		}
	}
	return true
}

// withoutComments replaces every comment with a space, told apart by the lexer
// the parser reads them with, so each shape it scans is excluded by construction.
func withoutComments(text string) string {
	var kept strings.Builder
	lx := lexer.New(source.New("head.sysml", []byte(text)))
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		switch tok.Kind {
		case lexer.SLNote, lexer.MLNote, lexer.RegularComment:
			kept.WriteByte(' ')
		default:
			kept.WriteString(text[tok.Span.Offset:tok.Span.End()])
		}
	}
	return kept.String()
}

// sigilBefore reads the `#` or `@` that introduces an annotation: the character
// ahead of the type it names.
func (e *encoder) sigilBefore(offset int) string {
	for at := offset - 1; at >= 0; at-- {
		switch text := e.file.Text(source.Span{Offset: at, Len: 1}); text {
		case "#", "@":
			return text
		case " ", "\t", "\n", "\r":
			continue
		default:
			return ""
		}
	}
	return ""
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
		// A name is mapped as a reference, which links it when this document
		// declares it; a feature chain or other expression is not a name, so it
		// is carried as the text it was written as.
		if name, ok := rel.Target.(*ast.QualifiedName); ok {
			e.graph.Add(subject, e.sysml(property), e.reference(owner, qualifiedText(name)))
			continue
		}
		e.graph.Add(subject, e.sysml(property), rdf.TypedLiteral(e.text(rel.Target), rdf.OpenSysML+dtExpression))
	}
}

// relationshipEnd writes one end of a keyword-first relationship, as a link
// when it names an element of this graph and as its written text otherwise.
func (e *encoder) relationshipEnd(subject rdf.Term, owner, property string, end ast.Node) {
	if end == nil {
		return
	}
	if name, ok := end.(*ast.QualifiedName); ok {
		e.graph.Add(subject, e.sysml(property), e.reference(owner, qualifiedText(name)))
		return
	}
	e.graph.Add(subject, e.sysml(property), rdf.TypedLiteral(e.text(end), rdf.OpenSysML+dtExpression))
}

func (e *encoder) multiplicity(subject rdf.Term, owner string, mult *ast.Multiplicity) {
	if mult == nil {
		return
	}
	// The parser puts the single bound of `[n]` in Lower; the language reads
	// that as lower and upper both being n, so it is written as the upper bound
	// alone and the printer renders it back as `[n]`.
	if !mult.IsRange {
		e.expression(subject, e.sysml(pUpperBound), pUpperBound, owner, mult.Lower)
		return
	}
	e.expression(subject, e.sysml(pLowerBound), pLowerBound, owner, mult.Lower)
	e.expression(subject, e.sysml(pUpperBound), pUpperBound, owner, mult.Upper)
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
	// A name that links to nothing is carried as the plain name; the quotes an
	// unrestricted name needs are notation, added when it is written back out.
	return rdf.String(name)
}

func (e *encoder) text(node ast.Node) string {
	if node == nil {
		return ""
	}
	return strings.TrimSpace(e.file.Text(node.Span()))
}

// rdfLimitationsNote is the remedy for a construct the RDF mapping does not
// represent, as docs/reference/rdf-mapping.md states it.
const rdfLimitationsNote = "save to .sysml or .kerml instead, which writes the source exactly; " +
	"see docs/reference/rdf-mapping.md § Limitations"

// nodeDescription names a construct the way the notation does — "part def",
// "substate member" — so an error about one prints no Go type name.
func nodeDescription(node ast.Node) string {
	switch n := node.(type) {
	case nil:
		return "declaration"
	case *ast.Definition:
		return n.Kind.String() + " def"
	case *ast.Usage:
		return n.Kind.String() + " usage"
	}
	return spacedWords(strings.TrimPrefix(fmt.Sprintf("%T", node), "*ast."))
}

// spacedWords turns a node type's name into lower-case words, so
// "SubstateMember" reads as "substate member".
func spacedWords(name string) string {
	if name == "" {
		return "declaration"
	}
	var b strings.Builder
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte(' ')
			}
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
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
	case *ast.RelationshipMember:
		return n, n.Visibility
	}
	return member, ast.VisibilityDefault
}

// referencesFeature reports whether a usage names an existing feature rather
// than declaring one of its own (`perform doIt;`).
func referencesFeature(n *ast.Usage) bool {
	for _, rel := range n.Relationships {
		if rel.Kind == ast.RelReferences && rel.Target != nil {
			return true
		}
	}
	return false
}

// declaredNameAndMembers returns the name a declaration introduces and the
// members it owns, for the node kinds that have either.
func declaredNameAndMembers(node ast.Node) (string, []ast.Node) {
	if name, members, ok := behaviorNameAndMembers(node); ok {
		return name, members
	}
	switch n := node.(type) {
	case *ast.Package:
		return n.Ident.Name, n.Members
	case *ast.Namespace:
		return n.Ident.Name, n.Members
	case *ast.Definition:
		return n.Ident.Name, n.Members
	case *ast.Usage:
		if verbatimUsage(n) {
			if shorthandRelationship(n) {
				return "", nil
			}
			return n.Ident.Name, nil
		}
		return n.Ident.Name, n.Members
	case *ast.Import:
		return "", n.Body
	case *ast.Alias:
		return n.Ident.Name, n.Body
	case *ast.Dependency:
		return n.Ident.Name, n.Body
	case *ast.RelationshipMember:
		return n.Ident.Name, n.Members
	case *ast.MultiplicityDecl:
		return n.Ident.Name, n.Members
	case *ast.ConstraintMember:
		return n.Name, n.Body
	case *ast.AssumeMember:
		return "", n.Body
	case *ast.RequireMember:
		return "", n.Body
	case *ast.SubjectMember:
		return n.Name, n.Body
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
