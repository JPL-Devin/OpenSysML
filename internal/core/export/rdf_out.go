package export

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf/ontology"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// Property names in the SysML vocabulary.
const (
	pDeclaredName      = "declaredName"
	pDeclaredShortName = "declaredShortName"
	pQualifiedName     = "qualifiedName"
	pElementID         = "elementId"
	pOwningNamespace   = "owningNamespace"
	pVisibility        = "visibility"
	// The ownership properties of the abstract syntax: an element's owner and
	// the OwningMembership between them, written from both ends.
	pOwner                     = "owner"
	pOwningRelationship        = "owningRelationship"
	pOwningMembership          = "owningMembership"
	pOwnedRelationship         = "ownedRelationship"
	pOwnedMembership           = "ownedMembership"
	pOwnedMember               = "ownedMember"
	pMemberElement             = "memberElement"
	pOwnedMemberElement        = "ownedMemberElement"
	pOwnedRelatedElement       = "ownedRelatedElement"
	pOwningRelatedElement      = "owningRelatedElement"
	pMembershipOwningNamespace = "membershipOwningNamespace"
	pOwnedMemberFeature        = "ownedMemberFeature"
	pOwningType                = "owningType"
	pOwnedFeature              = "ownedFeature"
	pOwnedFeatureMembership    = "ownedFeatureMembership"
	pOwnedImport               = "ownedImport"
	pImportOwningNamespace     = "importOwningNamespace"
	pDirection                 = "direction"
	pLowerBound                = "lowerBound"
	pUpperBound                = "upperBound"
	pValue                     = "value"
	pImportedNamespace         = "importedNamespace"
	pAliasFor                  = "aliasedElement"
	pClient                    = "client"
	pSupplier                  = "supplier"
	pBody                      = "body"
	pLanguage                  = "language"
	pLocale                    = "locale"
	pAnnotatedElement          = "annotatedElement"
	pIsImportAll               = "isImportAll"
	pSourceFeature             = "sourceFeature"
	pTargetFeature             = "targetFeature"
)

// Property names in the OpenSysML extension namespace: declaration order,
// body presence, and the source text of the constructs whose head this
// mapping keeps verbatim (see the package doc).
const (
	xMemberIndex     = "memberIndex"
	xHasBody         = "hasBody"
	xSourceText      = "sourceText"
	xSourceTail      = "sourceTail"
	xSourceLanguage  = "sourceLanguage"
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
	// The identity properties: whether an element's id came from an explicit
	// ElementId annotation, and the ProjectRef provenance of a scope root.
	xDeclaredID = "declaredId"
	xProjectID  = "projectId"
	xBranch     = "branch"
	xOrg        = "org"
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

// The metaclasses of the membership elements ownership is materialized as: a
// type owns a feature through a FeatureMembership, and every other namespace
// member is owned through an OwningMembership. Both are concrete in KerML.
const (
	mOwningMembership  = "OwningMembership"
	mFeatureMembership = "FeatureMembership"
)

// Metaclass names for the constructs that have no SysML metaclass of their own
// in this mapping.
const (
	mAlias        = "Alias"
	mFilter       = "FilterMember"
	mMultiplicity = "MultiplicityDeclaration"
	// The members that state a condition rather than declaring a feature: the
	// conditions of a constraint body and a requirement's assumptions and
	// required conditions.
	mConstraint = "ConstraintMember"
	mAssume     = "AssumeMember"
	mRequire    = "RequireMember"
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
// the tree was parsed from: every element and expression carries the notation
// it was written as alongside its structural triples, so the conversion needs
// the bytes as well as the tree.
func ToRDF(file *source.SourceFile, root *ast.RootNamespace) (*rdf.Graph, error) {
	e, err := encodeDocument(file, root)
	if err != nil {
		return nil, err
	}
	return e.graph, nil
}

// encodeDocument converts a parsed document, returning the encoder that holds
// the graph and where in file each element was written.
func encodeDocument(file *source.SourceFile, root *ast.RootNamespace) (*encoder, error) {
	if file == nil || root == nil {
		return nil, &UnsupportedError{What: "an empty document", Note: "nothing to convert"}
	}
	ids, err := documentIdentity(file.Name(), root)
	if err != nil {
		return nil, err
	}
	e := &encoder{
		file:           file,
		src:            newAuthoredSource(file),
		graph:          rdf.NewGraph(),
		declared:       map[string]bool{},
		metadataBodies: map[string]bool{},
		fqn:            map[ast.Node]string{},
		ids:            ids,
		subjects:       map[string]string{},
		regions:        map[rdf.Term]region{},
		bodies:         map[rdf.Term]region{},
		offsets:        map[string]int{},
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
	if e.idErr != nil {
		return nil, e.idErr
	}
	e.sourceText()
	return e, nil
}

// languageName is the name a document's grammar is recorded under on its roots,
// so the text is read back in the grammar it was written in. A file with no
// model extension is read as neither grammar exactly, so none is recorded.
func languageName(kind source.Kind) string {
	switch kind {
	case source.KindKerML:
		return "kerml"
	case source.KindSysML:
		return "sysml"
	}
	return ""
}

// sourceText gives every element its lines as written, comments included; one
// with members carries the lines before them and, as its tail, those after.
func (e *encoder) sourceText() {
	for _, subject := range e.graph.Subjects() {
		own, ok := e.regions[subject]
		if !ok {
			continue
		}
		members, ok := e.bodies[subject]
		if !ok {
			e.graph.Add(subject, e.sysx(xSourceText), rdf.String(e.src.region(own)))
			continue
		}
		head, tail := e.src.split(own, members)
		e.graph.Add(subject, e.sysx(xSourceText), rdf.String(head))
		e.graph.Add(subject, e.sysx(xSourceTail), rdf.String(tail))
	}
}

type encoder struct {
	file *source.SourceFile
	// src is the text of file as written, which is what sysx:sourceText carries.
	src      *authoredSource
	graph    *rdf.Graph
	declared map[string]bool
	// metadataBodies holds the qualified names of the metadata usages and the nested
	// members of their bodies: a keywordless member there is a ReferenceUsage.
	metadataBodies map[string]bool
	// fqn is the qualified name of each member node, which is how a succession
	// end the notation leaves unnamed addresses the member it binds.
	fqn map[ast.Node]string
	// ids is the document's identity side table: effective ids, declaredness,
	// scopes, and the annotation nodes consumed into it.
	ids *identityFacts
	// subjects maps each minted IRI — element or membership — to what it
	// stands for, so two ids landing on one IRI are refused rather than merged.
	subjects map[string]string
	// idErr holds a collision found where no error can propagate directly.
	idErr error
	// regions holds each element's lines, bodies the lines its members tile.
	regions map[rdf.Term]region
	bodies  map[rdf.Term]region
	// offsets holds where in file each element's declaration starts.
	offsets map[string]int
}

// claim reserves an IRI for what it stands for, returning the holder it
// collides with, if any.
func (e *encoder) claim(iri, standsFor string) (string, bool) {
	if prior, taken := e.subjects[iri]; taken && prior != standsFor {
		return prior, true
	}
	e.subjects[iri] = standsFor
	return "", false
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

// provenance emits the ProjectRef binding of a scope root as sysx: triples,
// so a graph carries the project its ids are stable within.
func (e *encoder) provenance(subject rdf.Term, node ast.Node) {
	scope := e.ids.provenance[node]
	if scope == nil {
		return
	}
	if scope.ProjectID != "" {
		e.graph.Add(subject, e.sysx(xProjectID), rdf.String(scope.ProjectID))
	}
	if scope.Branch != "" {
		e.graph.Add(subject, e.sysx(xBranch), rdf.String(scope.Branch))
	}
	if scope.Org != "" {
		e.graph.Add(subject, e.sysx(xOrg), rdf.String(scope.Org))
	}
}

// collect walks the tree recording every qualified name it declares. A name
// declared twice in one namespace is reported: the qualified name is an
// element's identity in the graph, so two such members would merge into one.
func (e *encoder) collect(members []ast.Node, owner string) error {
	for i, member := range e.kept(members) {
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
	kept := e.kept(members)
	spans := make([]source.Span, len(kept))
	for i, member := range kept {
		spans[i] = member.Span()
	}
	regions := e.src.tile(spans)
	// Members written on their owner's own lines, such as an accept's payload,
	// are part of its text: the owner is written whole or rebuilt whole.
	inline := !e.src.wholeLines(regions)
	if ownerTerm.Value == "" && len(regions) > 0 {
		// The document has no subject: roots sharing a line each keep their
		// slice of it, and what follows the last root is that root's.
		inline = false
		e.src.shareLines(regions, spans)
		regions[len(regions)-1].end = len(e.src.text)
	}
	if !inline && len(regions) > 0 && ownerTerm.Value != "" {
		body := region{regions[0].start, regions[len(regions)-1].end}
		if prior, ok := e.bodies[ownerTerm]; ok {
			body = region{min(prior.start, body.start), max(prior.end, body.end)}
		}
		e.bodies[ownerTerm] = body
	}
	for i, member := range kept {
		node, visibility := unwrapMember(member)
		if node == nil {
			continue
		}
		if err := e.encodeMember(node, visibility, regions[i], inline, owner, ownerTerm, i); err != nil {
			return err
		}
	}
	return nil
}

// kept filters out the identity annotations consumed into the graph's
// identity, so member positions count only what is actually exported.
func (e *encoder) kept(members []ast.Node) []ast.Node {
	out := make([]ast.Node, 0, len(members))
	for _, member := range members {
		if node, _ := unwrapMember(member); node != nil && e.ids.skip(node) {
			continue
		}
		out = append(out, member)
	}
	return out
}

// mint reserves the subject IRI of the element node declares as fqn.
func (e *encoder) mint(node ast.Node, fqn string) (rdf.Term, error) {
	subject := e.ids.subjectForNode(node, fqn)
	if prior, taken := e.claim(subject.Value, fqn); taken {
		return rdf.Term{}, &UnsupportedError{
			What: fmt.Sprintf("the declaration of %s at %s", fqn, e.where(node)),
			Note: fmt.Sprintf("its id lands on the same IRI as %s, and merging two elements into one subject would be a different model", prior),
		}
	}
	return subject, nil
}

// head types an element and states its identity, position and membership in
// its owner; a metaclass this mapping invents is typed in the OpenSysML namespace.
// lines is the text of the member, wrapper and all, unless inline: one written
// on its owner's own lines is part of the owner's text.
func (e *encoder) head(subject rdf.Term, node ast.Node, visibility ast.Visibility, fqn string, ownerTerm rdf.Term, index int, metaclass rdf.Term, lines region, inline bool) {
	e.graph.Add(subject, rdf.IRI(rdf.RDFType), metaclass)
	e.graph.Add(subject, e.sysml(pQualifiedName), rdf.String(fqn))
	// The id an API reader addresses the element by, which is the id its own
	// IRI ends in, so the two cannot disagree.
	e.graph.Add(subject, e.sysml(pElementID), rdf.String(rdf.LocalName(subject.Value)))
	e.graph.Add(subject, e.sysx(xMemberIndex), rdf.Int(index))
	e.offsets[subject.Value] = node.Span().Offset
	if !inline {
		e.regions[subject] = lines
	}
	if language := languageName(e.file.Kind()); ownerTerm.Value == "" && language != "" {
		e.graph.Add(subject, e.sysx(xSourceLanguage), rdf.String(language))
	}
	if e.ids.declaredIDAt(node) {
		e.graph.Add(subject, e.sysx(xDeclaredID), rdf.Bool(true))
	}
	e.provenance(subject, node)
	membership := rdf.Term{}
	if ownerTerm.Value != "" {
		// Only a namespace is an owningNamespace; a relationship owner is
		// stated by sysml:owner and the ownership owningMembership wires.
		if !isRelationship(e.metaclassOf(ownerTerm)) {
			e.graph.Add(subject, e.sysml(pOwningNamespace), ownerTerm)
		}
		membership = e.owningMembership(subject, ownerTerm, fqn)
	}
	if keyword := visibilityKeyword(visibility); keyword != "" {
		// The membership states the visibility a member is declared with; a
		// relationship, such as an import, states its own.
		visible := subject
		if membership.Value != "" {
			visible = membership
		}
		e.graph.Add(visible, e.sysml(pVisibility), rdf.String(keyword))
	}
}

// encodeMember maps one member: node is the declaration inside its membership
// wrapper, and lines the text of the member, wrapper and all, unless inline.
func (e *encoder) encodeMember(node ast.Node, visibility ast.Visibility, lines region, inline bool, owner string, ownerTerm rdf.Term, index int) error {
	name, _ := declaredNameAndMembers(node)
	fqn := qualify(owner, name, index)
	subject, err := e.mint(node, fqn)
	if err != nil {
		return err
	}
	head := func(metaclass rdf.Term) {
		e.head(subject, node, visibility, fqn, ownerTerm, index, metaclass, lines, inline)
	}

	switch n := node.(type) {
	case *ast.Package:
		head(rdf.SysMLTerm("Package"))
		e.ident(subject, n.Ident)
		e.flags(subject, []boolProperty{
			{"isLibraryPackage", n.IsLibrary},
			{"isStandardLibraryPackage", n.IsStandard},
		})
		if err := e.prefixes(subject, fqn, n.Prefixes, n.Members); err != nil {
			return err
		}
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Members, fqn, subject)

	case *ast.Namespace:
		head(rdf.SysMLTerm("Namespace"))
		e.ident(subject, n.Ident)
		if err := e.prefixes(subject, fqn, n.Prefixes, n.Members); err != nil {
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
			{"isParallel", n.IsParallel},
		})
		if err := e.prefixes(subject, fqn, n.Prefixes, n.Members); err != nil {
			return err
		}
		e.relationships(subject, owner, n.Relationships)
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Members, fqn, subject)

	case *ast.Usage:
		inBody := e.metadataBodies[owner]
		metaclass, ok := usageMetaclassOf(n, inBody)
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
			{"isParallel", n.IsParallel},
		})
		if err := e.prefixes(subject, fqn, n.Prefixes, n.Members); err != nil {
			return err
		}
		if keyword := directionKeyword(n.Direction); keyword != "" {
			e.graph.Add(subject, e.sysml(pDirection), rdf.String(keyword))
		}
		e.relationships(subject, owner, n.Relationships)
		e.multiplicity(subject, owner, n.Multiplicity)
		e.expression(subject, e.sysml(pValue), pValue, owner, n.Value)
		// A declaration head that binds ends (connect/bind/flow/succession),
		// a transition, an accept action or a satisfy usage states its ends
		// and form structurally, since the properties above do not.
		if verbatimUsage(n) {
			e.bindingEnds(subject, owner, n)
			e.endForm(subject, n)
			return nil
		}
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		if inBody || n.Kind == ast.UsageMetadata {
			e.metadataBodies[fqn] = true
		}
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
		if err := e.prefixes(subject, fqn, n.Prefixes, n.Body); err != nil {
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
			e.graph.Add(subject, e.sysml(pLocale), rdf.String(lexer.StringValue(n.Locale)))
		}
		e.graph.Add(subject, e.sysml(pBody), rdf.String(commentBody(e.src.slice(n.BodySpan))))
		return nil

	case *ast.Documentation:
		head(rdf.SysMLTerm("Documentation"))
		e.ident(subject, n.Ident)
		if n.Locale != "" {
			e.graph.Add(subject, e.sysml(pLocale), rdf.String(lexer.StringValue(n.Locale)))
		}
		e.graph.Add(subject, e.sysml(pBody), rdf.String(commentBody(e.src.slice(n.BodySpan))))
		return nil

	case *ast.TextualRepresentation:
		head(rdf.SysMLTerm("TextualRepresentation"))
		e.ident(subject, n.Ident)
		e.graph.Add(subject, e.sysml(pLanguage), rdf.String(lexer.StringValue(n.Language)))
		e.graph.Add(subject, e.sysml(pBody), rdf.String(commentBody(e.src.slice(n.BodySpan))))
		return nil

	case *ast.MultiplicityDecl:
		head(rdf.OpenSysMLTerm(mMultiplicity))
		e.ident(subject, n.Ident)
		e.multiplicity(subject, owner, n.Range)
		// A MultiplicitySubset states its bounds by subsetting, not as a range.
		if n.Subsets != nil {
			e.graph.Add(subject, e.sysml(relationshipProperty[ast.RelSubsets]),
				e.reference(owner, qualifiedText(n.Subsets)))
		}
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
		return e.encode(n.Members, fqn, subject)

	case *ast.ConstraintMember:
		// A condition of a constraint body, written bare (`x > 0`) or as a
		// nested constraint (`assert constraint [name] { … }`).
		head(rdf.OpenSysMLTerm(mConstraint))
		if n.Name != "" {
			e.graph.Add(subject, e.sysml(pDeclaredName), rdf.String(n.Name))
		}
		if n.Keyword != "" {
			e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(n.Keyword))
		}
		e.flags(subject, []boolProperty{{"isNegated", n.IsNegated}})
		return e.condition(subject, fqn, owner, n.Expression, nil, true, n.Body)

	case *ast.AssumeMember:
		head(rdf.OpenSysMLTerm(mAssume))
		return e.requirementCondition(subject, fqn, owner, requirementConditionDecl{
			prefixes: n.Prefixes, name: n.Name, relationships: n.Relationships, multiplicity: n.Multiplicity,
			value: n.Value, expression: n.Expression, reference: n.Reference, hasBody: n.HasBody, body: n.Body,
		})

	case *ast.RequireMember:
		head(rdf.OpenSysMLTerm(mRequire))
		return e.requirementCondition(subject, fqn, owner, requirementConditionDecl{
			prefixes: n.Prefixes, name: n.Name, relationships: n.Relationships, multiplicity: n.Multiplicity,
			value: n.Value, expression: n.Expression, reference: n.Reference, hasBody: n.HasBody, body: n.Body,
		})

	case *ast.PrefixMetadata:
		// `@M { … }` written as a member annotates its owner, or what its
		// `about` names (SysML v2 MetadataUsage).
		head(rdf.SysMLTerm(usageMetaclass[ast.UsageMetadata]))
		return e.metadataUsage(subject, fqn, owner, n, "@")

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
		if err := e.prefixes(subject, fqn, n.Prefixes, n.Body); err != nil {
			return err
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

// owningMembership wires a member to its owner the way the abstract syntax does,
// returning the membership minted between them, or the empty term when no
// membership stands between the two. The API's payloads reach a member through
// its membership, so a compact owner triple alone leaves a client walking down
// from a root with nothing to follow.
func (e *encoder) owningMembership(member, owner rdf.Term, memberFQN string) rdf.Term {
	ownerClass, memberClass := e.metaclassOf(owner), e.metaclassOf(member)
	// A metadata usage annotates its owner through an OwningMembership whatever
	// the owner is, a relationship included (SysML.xtext PrefixMetadataMember).
	metadata := memberClass == "MetadataUsage"
	switch {
	case isRelationship(ownerClass) && !metadata:
		// A relationship owns its related element itself, as a state's entry
		// membership owns the action it states.
		e.relationshipOwnership(member, owner, ownerClass, memberClass)
		return rdf.Term{}
	case isRelationship(memberClass):
		// A namespace owns a relationship it declares — an import, a dependency,
		// a membership — as an owned relationship, with no membership between.
		e.graph.Add(member, e.sysml(pOwner), owner)
		e.graph.Add(member, e.sysml(pOwningRelatedElement), owner)
		e.graph.Add(owner, e.sysml(pOwnedRelationship), member)
		if ontology.IsAncestorOrSelf(memberClass, "Import") {
			e.graph.Add(member, e.sysml(pImportOwningNamespace), owner)
			e.graph.Add(owner, e.sysml(pOwnedImport), member)
		}
		return rdf.Term{}
	}
	// A type owns a feature through a FeatureMembership, which is the membership
	// the API's payloads carry for it; anything else, a metadata usage included,
	// through an OwningMembership.
	feature := ontology.IsAncestorOrSelf(memberClass, "Feature") && ontology.IsAncestorOrSelf(ownerClass, "Type") && !metadata
	membership := rdf.OwningMembershipIRIOf(member)
	// The membership shares the element namespace, so its IRI is reserved too.
	if prior, taken := e.claim(membership.Value, memberFQN+"'s owning membership"); taken && e.idErr == nil {
		e.idErr = &UnsupportedError{
			What: fmt.Sprintf("the owning membership of %s", memberFQN),
			Note: fmt.Sprintf("its id lands on the same IRI as %s, and merging two elements into one subject would be a different model", prior),
		}
	}
	e.graph.Add(member, e.sysml(pOwner), owner)
	e.graph.Add(member, e.sysml(pOwningRelationship), membership)
	e.graph.Add(member, e.sysml(pOwningMembership), membership)

	metaclass := mOwningMembership
	if feature {
		metaclass = mFeatureMembership
	}
	e.graph.Add(membership, rdf.IRI(rdf.RDFType), e.sysml(metaclass))
	e.graph.Add(membership, e.sysml(pElementID), rdf.String(rdf.LocalName(membership.Value)))
	// The namespace owns the membership too, so it is not read as a root.
	e.graph.Add(membership, e.sysml(pOwner), owner)
	e.graph.Add(membership, e.sysml(pMemberElement), member)
	e.graph.Add(membership, e.sysml(pOwnedMemberElement), member)
	e.graph.Add(membership, e.sysml(pOwnedRelatedElement), member)
	e.graph.Add(membership, e.sysml(pOwningRelatedElement), owner)
	// Only a namespace has members; a relationship owner just owns the membership.
	if !isRelationship(ownerClass) {
		e.graph.Add(membership, e.sysml(pMembershipOwningNamespace), owner)
		e.graph.Add(owner, e.sysml(pOwnedMember), member)
		e.graph.Add(owner, e.sysml(pOwnedMembership), membership)
	}
	e.graph.Add(owner, e.sysml(pOwnedRelationship), membership)
	if feature {
		e.graph.Add(membership, e.sysml(pOwnedMemberFeature), member)
		e.graph.Add(membership, e.sysml(pOwningType), owner)
		e.graph.Add(owner, e.sysml(pOwnedFeature), member)
		e.graph.Add(owner, e.sysml(pOwnedFeatureMembership), membership)
	}
	return membership
}

// relationshipOwnership wires a member owned by a relationship rather than by a
// namespace, such as a state's entry action. A relationship owns its related
// element itself, so no membership is minted between them.
func (e *encoder) relationshipOwnership(member, owner rdf.Term, ownerClass, memberClass string) {
	e.graph.Add(member, e.sysml(pOwner), owner)
	e.graph.Add(owner, e.sysml(pOwnedRelatedElement), member)
	if isRelationship(memberClass) {
		// A relationship states the element that owns it, not an owning
		// relationship of its own.
		e.graph.Add(member, e.sysml(pOwningRelatedElement), owner)
		e.graph.Add(owner, e.sysml(pOwnedRelationship), member)
		return
	}
	e.graph.Add(member, e.sysml(pOwningRelationship), owner)
	if ontology.IsAncestorOrSelf(ownerClass, "Membership") {
		e.graph.Add(member, e.sysml(pOwningMembership), owner)
		e.graph.Add(owner, e.sysml(pMemberElement), member)
	}
	if ontology.IsAncestorOrSelf(ownerClass, "OwningMembership") {
		e.graph.Add(owner, e.sysml(pOwnedMemberElement), member)
	}
	if ontology.IsAncestorOrSelf(ownerClass, mFeatureMembership) && ontology.IsAncestorOrSelf(memberClass, "Feature") {
		e.graph.Add(owner, e.sysml(pOwnedMemberFeature), member)
	}
}

// metaclassOf is the ontology name of the metaclass a subject is typed with,
// which is empty for a metaclass this mapping invents.
func (e *encoder) metaclassOf(subject rdf.Term) string {
	return ontology.LocalName(e.graph.Type(subject))
}

// isRelationship reports whether a metaclass relates elements rather than
// containing them, which decides whether ownership needs a membership.
func isRelationship(metaclass string) bool {
	return ontology.IsAncestorOrSelf(metaclass, "Relationship") && !ontology.IsAncestorOrSelf(metaclass, "Namespace")
}

// condition emits the three forms a condition member is written in: an inline
// expression, a reference to the constraint it states (`require R { … }`), or a
// nested constraint stating its conditions in a body.
func (e *encoder) condition(subject rdf.Term, fqn, owner string, expr ast.Node, ref *ast.QualifiedName, hasBody bool, body []ast.Node) error {
	if expr != nil {
		e.expression(subject, e.sysx(xCondition), xCondition, owner, expr)
		return nil
	}
	if ref != nil {
		e.graph.Add(subject, e.sysml(relationshipProperty[ast.RelReferences]), e.reference(owner, qualifiedText(ref)))
	}
	e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(hasBody))
	return e.encode(body, fqn, subject)
}

// requirementConditionDecl is the head an `assume`/`require` member declares
// for the constraint usage it owns, besides the condition itself.
type requirementConditionDecl struct {
	prefixes      []*ast.PrefixMetadata
	name          string
	relationships []*ast.Relationship
	multiplicity  *ast.Multiplicity
	value         ast.Node
	expression    ast.Node
	reference     *ast.QualifiedName
	hasBody       bool
	body          []ast.Node
}

// requirementCondition emits an `assume`/`require` member together with its
// constraint usage's declaration (`require #goal constraint c : C [1] = true`).
func (e *encoder) requirementCondition(subject rdf.Term, fqn, owner string, n requirementConditionDecl) error {
	if n.name != "" {
		e.graph.Add(subject, e.sysml(pDeclaredName), rdf.String(n.name))
	}
	if err := e.prefixes(subject, fqn, n.prefixes, n.body); err != nil {
		return err
	}
	e.relationships(subject, owner, n.relationships)
	e.multiplicity(subject, owner, n.multiplicity)
	e.expression(subject, e.sysml(pValue), pValue, owner, n.value)
	return e.condition(subject, fqn, owner, n.expression, n.reference, n.hasBody, n.body)
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

// prefixes maps the `#M` annotations ahead of a declaration as metadata usages
// it owns after its body members (PrefixMetadataMember), keyed `#` for the writer.
func (e *encoder) prefixes(subject rdf.Term, fqn string, prefixes []*ast.PrefixMetadata, members []ast.Node) error {
	index := len(e.kept(members))
	for _, prefix := range prefixes {
		if prefix == nil || e.ids.skip(prefix) {
			continue
		}
		prefixFQN := qualify(fqn, "", index)
		// A body member may be named `'@N'`, the position name the prefix takes.
		if e.declared[prefixFQN] {
			return &UnsupportedError{
				What: fmt.Sprintf("the prefix annotation at %s", e.where(prefix)),
				Note: fmt.Sprintf("it is identified by its position as %s, which a body member is named, and merging two elements into one subject would be a different model", prefixFQN),
			}
		}
		prefixSubject, err := e.mint(prefix, prefixFQN)
		if err != nil {
			return err
		}
		// A prefix is written in its owner's head, so its text is the owner's.
		e.head(prefixSubject, prefix, ast.VisibilityDefault, prefixFQN, subject, index,
			rdf.SysMLTerm(usageMetaclass[ast.UsageMetadata]), region{}, true)
		if err := e.metadataUsage(prefixSubject, prefixFQN, fqn, prefix, "#"); err != nil {
			return err
		}
		index++
	}
	return nil
}

// metadataUsage states an annotation's definition, the elements it is about and
// its body; the sigil it was written with is its declared keyword.
func (e *encoder) metadataUsage(subject rdf.Term, fqn, owner string, n *ast.PrefixMetadata, sigil string) error {
	if n.Type == nil {
		return &UnsupportedError{
			What: fmt.Sprintf("the metadata annotation at %s", e.where(n)),
			Note: "it names no metadata definition, and an annotation of nothing would be a different model",
		}
	}
	e.ident(subject, n.Ident)
	e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(sigil))
	e.graph.Add(subject, e.sysml(relationshipProperty[ast.RelTyping]), e.reference(owner, qualifiedText(n.Type)))
	for _, about := range n.About {
		e.graph.Add(subject, e.sysml(pAnnotatedElement), e.reference(owner, qualifiedText(about)))
	}
	e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(n.HasBody))
	e.metadataBodies[fqn] = true
	return e.encode(n.Body, fqn, subject)
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
			return e.ids.subjectFor(candidate)
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

// text is the notation of a node as written, without the trivia its span runs
// on over.
func (e *encoder) text(node ast.Node) string {
	if node == nil {
		return ""
	}
	return strings.TrimSpace(e.src.code(node.Span()))
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
		return n.Name, n.Body
	case *ast.RequireMember:
		return n.Name, n.Body
	case *ast.SubjectMember:
		return n.Name, n.Body
	case *ast.PrefixMetadata:
		return n.Ident.Name, n.Body
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

// commentBody strips the /* */ delimiters from a comment token, leaving the
// text the printer re-wraps.
func commentBody(raw string) string {
	raw = strings.TrimPrefix(raw, "/*")
	raw = strings.TrimSuffix(raw, "*/")
	return raw
}
