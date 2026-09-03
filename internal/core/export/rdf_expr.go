package export

// Expression-valued positions — values, bounds, guards, conditions, filters —
// are mapped to a graph of expression nodes, each keeping its notation as
// sysx:sourceText. See docs/reference/rdf-mapping.md § Expressions.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
)

// Metaclasses of the expression nodes, as SysML v2 8.4 names them.
const (
	mExpression       = "Expression"
	mLiteralBoolean   = "LiteralBoolean"
	mLiteralInteger   = "LiteralInteger"
	mLiteralRational  = "LiteralRational"
	mLiteralString    = "LiteralString"
	mLiteralInfinity  = "LiteralInfinity"
	mNullExpression   = "NullExpression"
	mFeatureReference = "FeatureReferenceExpression"
	mFeatureChain     = "FeatureChainExpression"
	mOperator         = "OperatorExpression"
	mInvocation       = "InvocationExpression"
	mCollect          = "CollectExpression"
	mSelect           = "SelectExpression"
	mMetadataAccess   = "MetadataAccessExpression"
)

// Properties of an expression node in the SysML vocabulary.
const (
	pOperator          = "operator"
	pArgument          = "argument"
	pOperand           = "operand"
	pReferent          = "referent"
	pFunction          = "function"
	pReferencedElement = "referencedElement"
)

// Properties this mapping adds: argument order, which RDF does not carry, and
// parts the 202407 metamodel rendering has no property for.
const (
	xArgumentIndex    = "argumentIndex"
	xArgumentName     = "argumentName"
	xTypeArgument     = "typeArgument"
	xIsConstructor    = "isConstructor"
	xBodyParameter    = "bodyParameter"
	xBodyMember       = "bodyMember"
	xResultExpression = "resultExpression"
)

// mBodyMember types a declaration an expression body makes between its
// parameters and its result, carried as its notation.
const mBodyMember = "BodyMember"

// Operator spellings whose notation is not the plain infix form.
const (
	opSequence = ","
	opIndex    = "[]"
	opAt       = "#"
	opIf       = "if"
)

// expression emits the graph of one expression-valued property. slot names the
// position, so each expression of an element has a distinct identity.
func (e *encoder) expression(subject, property rdf.Term, slot, owner string, node ast.Node) {
	if node == nil {
		return
	}
	e.graph.Prefixes[rdf.ExpressionPrefix] = rdf.Expression
	target := rdf.ExpressionIRI(subject, slot)
	e.graph.Add(subject, property, target)
	e.expressionNode(target, owner, node)
}

// expressionNode emits one expression node and, recursively, its operands.
// Every node carries its notation, so the exact text always survives.
func (e *encoder) expressionNode(subject rdf.Term, owner string, node ast.Node) {
	e.graph.Add(subject, e.sysx(xSourceText), rdf.String(e.text(node)))
	// The id an API reader addresses the node by, as on an element: a node has no
	// qualified name, but its position in the model gives it a valid id.
	e.graph.Add(subject, e.sysml(pElementID), rdf.String(rdf.LocalName(subject.Value)))
	e.typed(subject, expressionMetaclass(node))
	switch n := node.(type) {
	case *ast.LiteralBool:
		e.graph.Add(subject, e.sysml(pValue), rdf.Bool(n.Value))

	case *ast.LiteralString:
		e.graph.Add(subject, e.sysml(pValue), rdf.String(lexer.StringValue(n.Value)))

	case *ast.LiteralInteger:
		e.graph.Add(subject, e.sysml(pValue), rdf.TypedLiteral(n.Value, rdf.XSD+"integer"))

	case *ast.LiteralReal:
		e.graph.Add(subject, e.sysml(pValue), rdf.TypedLiteral(n.Value, rdf.XSD+"decimal"))

	case *ast.QualifiedName:
		// A position whose notation is a bare name holds the feature it names.
		e.graph.Add(subject, e.sysml(pReferent), e.reference(owner, qualifiedText(n)))

	case *ast.FeatureReference:
		e.graph.Add(subject, e.sysml(pReferent), e.reference(owner, qualifiedText(n.Name)))

	case *ast.OperatorExpr:
		e.graph.Add(subject, e.sysml(pOperator), rdf.String(n.Operator.String()))
		e.arguments(subject, owner, n.Operands)
		if n.TypeRef != nil {
			e.graph.Add(subject, e.sysx(xTypeArgument), e.reference(owner, qualifiedText(n.TypeRef)))
		}

	case *ast.CastExpr:
		// `(as T[m])` is the classification operator with a type argument only,
		// its multiplicity written as bounds the way a usage's is.
		e.graph.Add(subject, e.sysml(pOperator), rdf.String(ast.OpAs.String()))
		e.graph.Add(subject, e.sysx(xTypeArgument), e.reference(owner, qualifiedText(n.TargetType)))
		e.multiplicity(subject, owner, n.Multiplicity)

	case *ast.FeatureChainExpr:
		e.arguments(subject, owner, []ast.Node{n.Operand})
		e.graph.Add(subject, e.sysml(pTargetFeature), e.reference(owner, qualifiedText(n.Member)))

	case *ast.IndexExpr:
		operator := opAt
		if n.Bracket {
			operator = opIndex
		}
		e.graph.Add(subject, e.sysml(pOperator), rdf.String(operator))
		e.arguments(subject, owner, []ast.Node{n.Operand, n.Index})

	case *ast.InvocationExpr:
		e.invocation(subject, owner, n.Type, n.Operand, n.Args, n.NamedArgs)

	case *ast.ConstructorExpr:
		// The 202407 rendering declares no ConstructorExpression, so `new` is a flag.
		e.graph.Add(subject, e.sysx(xIsConstructor), rdf.Bool(true))
		e.invocation(subject, owner, n.Type, nil, n.Args, nil)

	case *ast.CollectExpr:
		e.arguments(subject, owner, []ast.Node{n.Operand, n.Body})

	case *ast.SelectExpr:
		e.arguments(subject, owner, []ast.Node{n.Operand, n.Body})

	case *ast.SequenceExpr:
		e.graph.Add(subject, e.sysml(pOperator), rdf.String(opSequence))
		e.arguments(subject, owner, n.Elements)

	case *ast.MetadataAccessExpr:
		e.graph.Add(subject, e.sysml(pReferencedElement), e.reference(owner, qualifiedText(n.Ref)))

	case *ast.BodyExpr:
		// A body declares its own parameters and members, then a result expression;
		// sysx:hasBody marks it as one even when it declares nothing.
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(true))
		e.bodyDeclarations(subject, owner, n.Params, n.Members)
		if n.Result != nil {
			result := rdf.ExpressionIRI(subject, "result")
			e.graph.Add(subject, e.sysx(xResultExpression), result)
			e.expressionNode(result, owner, n.Result)
		}
	}
}

func (e *encoder) typed(subject rdf.Term, metaclass string) {
	e.graph.Add(subject, rdf.IRI(rdf.RDFType), rdf.SysMLTerm(metaclass))
}

// expressionMetaclass is the metaclass an expression node is typed with. A shape
// this mapping does not decompose still states it is an expression.
func expressionMetaclass(node ast.Node) string {
	switch node.(type) {
	case *ast.LiteralBool:
		return mLiteralBoolean
	case *ast.LiteralString:
		return mLiteralString
	case *ast.LiteralInteger:
		return mLiteralInteger
	case *ast.LiteralReal:
		return mLiteralRational
	case *ast.LiteralInfinity:
		return mLiteralInfinity
	case *ast.NullExpr:
		return mNullExpression
	case *ast.QualifiedName, *ast.FeatureReference:
		return mFeatureReference
	case *ast.OperatorExpr, *ast.CastExpr, *ast.IndexExpr, *ast.SequenceExpr:
		return mOperator
	case *ast.FeatureChainExpr:
		return mFeatureChain
	case *ast.InvocationExpr, *ast.ConstructorExpr:
		return mInvocation
	case *ast.CollectExpr:
		return mCollect
	case *ast.SelectExpr:
		return mSelect
	case *ast.MetadataAccessExpr:
		return mMetadataAccess
	}
	return mExpression
}

// isExpressionMember reports whether a body member is a bare expression: the
// result a calculation or case body ends in.
func isExpressionMember(node ast.Node) bool {
	switch node.(type) {
	case *ast.LiteralBool, *ast.LiteralString, *ast.LiteralInteger, *ast.LiteralReal,
		*ast.LiteralInfinity, *ast.NullExpr, *ast.FeatureReference, *ast.OperatorExpr,
		*ast.CastExpr, *ast.FeatureChainExpr, *ast.IndexExpr, *ast.InvocationExpr,
		*ast.ConstructorExpr, *ast.CollectExpr, *ast.SelectExpr, *ast.SequenceExpr,
		*ast.MetadataAccessExpr, *ast.BodyExpr:
		return true
	}
	return false
}

// bodyDeclarations emits what an expression body declares ahead of its result,
// parameters and members alike, indexed in the one order they were written.
func (e *encoder) bodyDeclarations(subject rdf.Term, owner string, params []ast.BodyParam, members []ast.Node) {
	type declaration struct {
		offset int
		param  *ast.BodyParam
		member ast.Node
	}
	declarations := make([]declaration, 0, len(params)+len(members))
	for i := range params {
		declarations = append(declarations, declaration{offset: params[i].Span.Offset, param: &params[i]})
	}
	for _, member := range members {
		declarations = append(declarations, declaration{offset: member.Span().Offset, member: member})
	}
	sort.SliceStable(declarations, func(i, j int) bool {
		return declarations[i].offset < declarations[j].offset
	})
	for i, decl := range declarations {
		if decl.param != nil {
			e.bodyParameter(subject, owner, i, *decl.param)
		} else {
			e.bodyMember(subject, i, decl.member)
		}
	}
}

// bodyParameter emits one parameter of an expression body as a node of its
// own, so its type, value and bounds are structure, not text.
func (e *encoder) bodyParameter(subject rdf.Term, owner string, index int, param ast.BodyParam) {
	node := rdf.ExpressionIRI(subject, fmt.Sprintf("in%d", index))
	e.graph.Add(subject, e.sysx(xBodyParameter), node)
	e.graph.Add(node, rdf.IRI(rdf.RDFType), rdf.SysMLTerm(keywordMetaclass["ref"]))
	e.graph.Add(node, e.sysml(pElementID), rdf.String(rdf.LocalName(node.Value)))
	e.graph.Add(node, e.sysx(xMemberIndex), rdf.Int(index))
	e.graph.Add(node, e.sysml(pDirection), rdf.String(directionKeyword(ast.DirIn)))
	e.name(node, param.Name)
	e.flags(node, []boolProperty{{"isReference", param.IsReference}})
	if param.Type != nil {
		e.graph.Add(node, e.sysml(relationshipProperty[ast.RelTyping]), e.reference(owner, qualifiedText(param.Type)))
	}
	e.relationships(node, owner, param.Relationships)
	e.multiplicity(node, owner, param.Multiplicity)
	e.expression(node, e.sysml(pValue), pValue, owner, param.Value)
	e.bodyDeclarations(node, owner, nil, param.Members)
}

// bodyMember carries one declaration of an expression body, or of one of its
// parameters. Documentation is structure; any other declaration is its notation.
func (e *encoder) bodyMember(subject rdf.Term, index int, member ast.Node) {
	node := rdf.ExpressionIRI(subject, fmt.Sprintf("m%d", index))
	e.graph.Add(subject, e.sysx(xBodyMember), node)
	e.graph.Add(node, e.sysml(pElementID), rdf.String(rdf.LocalName(node.Value)))
	e.graph.Add(node, e.sysx(xMemberIndex), rdf.Int(index))
	e.graph.Add(node, e.sysx(xSourceText), rdf.String(e.text(member)))
	if doc, ok := member.(*ast.Documentation); ok {
		e.typed(node, mDocumentation)
		e.documentation(node, doc)
		return
	}
	e.graph.Add(node, rdf.IRI(rdf.RDFType), rdf.OpenSysMLTerm(mBodyMember))
}

// arguments emits the operands of an expression, each carrying the position it
// was written in: RDF states no order between the objects of one property.
func (e *encoder) arguments(subject rdf.Term, owner string, args []ast.Node) {
	for i, arg := range args {
		if arg == nil {
			continue
		}
		child := rdf.ExpressionIRI(subject, fmt.Sprintf("a%d", i))
		e.graph.Add(subject, e.sysml(pArgument), child)
		e.expressionNode(child, owner, arg)
		e.graph.Add(child, e.sysx(xArgumentIndex), rdf.Int(i))
	}
}

// invocation emits the parts an invocation and a constructor share: the function
// invoked, the receiver of a `->` form, and the arguments.
func (e *encoder) invocation(subject rdf.Term, owner string, function *ast.QualifiedName, operand ast.Node, args []ast.Node, named []ast.NamedArg) {
	if function != nil {
		e.graph.Add(subject, e.sysml(pFunction), e.reference(owner, qualifiedText(function)))
	}
	if operand != nil {
		receiver := rdf.ExpressionIRI(subject, "operand")
		e.graph.Add(subject, e.sysml(pOperand), receiver)
		e.expressionNode(receiver, owner, operand)
	}
	e.arguments(subject, owner, args)
	for i, arg := range named {
		if arg.Value == nil {
			continue
		}
		child := rdf.ExpressionIRI(subject, fmt.Sprintf("n%d", i))
		e.graph.Add(subject, e.sysml(pArgument), child)
		e.expressionNode(child, owner, arg.Value)
		e.graph.Add(child, e.sysx(xArgumentIndex), rdf.Int(i))
		e.graph.Add(child, e.sysx(xArgumentName), rdf.String(qualifiedText(arg.Name)))
	}
}

// expressionMetaclasses are the metaclasses of an expression node, which is a
// part of an element's declaration rather than an element the printer writes.
var expressionMetaclasses = map[string]bool{
	mExpression: true, mLiteralBoolean: true, mLiteralInteger: true,
	mLiteralRational: true, mLiteralString: true, mLiteralInfinity: true,
	mNullExpression: true, mFeatureReference: true, mFeatureChain: true,
	mOperator: true, mInvocation: true, mCollect: true, mSelect: true,
	mMetadataAccess: true,
}

// isExpressionNode reports whether a subject is an expression node rather than
// an element: it is in the expression namespace, or its metaclass is one and it
// has no qualified name (an `expr` usage is typed sysml:Expression too) and no
// membership owns it (a body's result is an Expression element). Memberships
// are read before elements, so the ownership index is complete here.
func (d *decoder) isExpressionNode(subject rdf.Term) bool {
	if !subject.IsIRI() {
		return false
	}
	if strings.HasPrefix(subject.Value, rdf.Expression) {
		return true
	}
	_, owned := d.owningMembership[subject.Value]
	return expressionMetaclasses[rdf.LocalName(d.graph.Type(subject))] &&
		!d.graph.HasProperty(subject, rdf.SysML+pQualifiedName) &&
		!d.graph.HasProperty(subject, rdf.SysML+pOwningMembership) && !owned
}

// resolveExpressions renders every element's expression-valued properties as
// notation, so the printer reads one text per property.
func (d *decoder) resolveExpressions() error {
	for _, triple := range d.graph.Triples() {
		if !d.isExpressionNode(triple.Object) {
			continue
		}
		el, ok := d.byIRI[triple.Subject.Value]
		if !ok || d.isResultExpression(el) {
			// The subject is an expression node; its parts are written with it.
			continue
		}
		text, err := d.expressionNodeText(triple.Object, el.scope)
		if err != nil {
			return err
		}
		if el.expressions == nil {
			el.expressions = map[string]string{}
		}
		el.expressions[triple.Predicate.Value] = text
	}
	return nil
}

// expressionNodeText writes an expression node back as notation: the notation it
// kept, or notation rebuilt from its structure when it kept none.
func (d *decoder) expressionNodeText(node rdf.Term, scope string) (string, error) {
	if text, ok := d.graph.Lexical(node, rdf.OpenSysML+xSourceText); ok && text != "" {
		return text, nil
	}
	metaclass := rdf.LocalName(d.graph.Type(node))
	unsupported := func(note string) error {
		return &UnsupportedError{
			What: fmt.Sprintf("the expression <%s>", node.Value),
			Note: note,
		}
	}
	switch metaclass {
	case mLiteralBoolean, mLiteralInteger, mLiteralRational:
		value, ok := d.graph.Lexical(node, rdf.SysML+pValue)
		if !ok {
			return "", unsupported("a literal expression states the value it evaluates to")
		}
		return value, nil
	case mLiteralString:
		value, ok := d.graph.Lexical(node, rdf.SysML+pValue)
		if !ok {
			return "", unsupported("a literal expression states the value it evaluates to")
		}
		return lexer.StringText(value), nil
	case mLiteralInfinity:
		return "*", nil
	case mNullExpression:
		return "null", nil
	case mFeatureReference:
		return d.expressionReference(node, rdf.SysML+pReferent, scope,
			"a feature reference names the feature it reads")
	case mMetadataAccess:
		name, err := d.expressionReference(node, rdf.SysML+pReferencedElement, scope,
			"a metadata access names the element it reads the metadata of")
		if err != nil {
			return "", err
		}
		return name + ".metadata", nil
	case mFeatureChain:
		operands, err := d.expressionArguments(node, scope)
		if err != nil {
			return "", err
		}
		member, err := d.expressionReference(node, rdf.SysML+pTargetFeature, scope,
			"a feature chain names the feature it reaches")
		if err != nil {
			return "", err
		}
		if len(operands) != 1 {
			return "", unsupported("a feature chain applies to exactly one operand")
		}
		return operands[0] + "." + member, nil
	case mCollect, mSelect:
		operands, err := d.expressionArguments(node, scope)
		if err != nil {
			return "", err
		}
		if len(operands) != 2 {
			return "", unsupported("a collect or select expression applies a body to one operand")
		}
		separator := "."
		if metaclass == mSelect {
			separator = ".?"
		}
		return operands[0] + separator + operands[1], nil
	case mOperator:
		return d.operatorText(node, scope)
	case mInvocation:
		return d.invocationText(node, scope)
	case mExpression:
		if d.graph.BoolValue(node, rdf.OpenSysML+xHasBody) ||
			d.graph.HasProperty(node, rdf.OpenSysML+xResultExpression) ||
			d.graph.HasProperty(node, rdf.OpenSysML+xBodyParameter) ||
			d.graph.HasProperty(node, rdf.OpenSysML+xBodyMember) {
			return d.expressionBodyText(node, scope)
		}
	}
	return "", unsupported("this expression states no notation and no structure to write one from; " + rdfLimitationsNote)
}

// expressionBodyText rebuilds an expression body: its declarations and its
// result, in the order the graph records.
func (d *decoder) expressionBodyText(node rdf.Term, scope string) (string, error) {
	parts, err := d.bodyDeclarationsText(node, scope)
	if err != nil {
		return "", err
	}
	if result, ok := d.graph.Object(node, rdf.OpenSysML+xResultExpression); ok {
		text, err := d.expressionNodeText(result, scope)
		if err != nil {
			return "", err
		}
		parts = append(parts, text)
	}
	if len(parts) == 0 {
		return "{}", nil
	}
	return "{ " + strings.Join(parts, " ") + " }", nil
}

// bodyParameterText rebuilds one `in` parameter of an expression body. A
// parameter a graph states as a bare name literal is that name alone.
func (d *decoder) bodyParameterText(param rdf.Term, scope string) (string, error) {
	if !param.IsIRI() {
		return "in " + nameText(param.Value) + ";", nil
	}
	el := &element{iri: param.Value, scope: scope, expressions: map[string]string{}}
	name, ok := d.stringOf(el, rdf.SysML+pDeclaredName)
	if !ok {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the body parameter <%s>", param.Value),
			Note: "a parameter of an expression body is named, and this one states no sysml:declaredName",
		}
	}
	// The bounds and the value are expressions, resolved here since the
	// parameter is not an element of the model.
	for _, property := range []string{pLowerBound, pUpperBound, pValue} {
		object, ok := d.graph.Object(param, rdf.SysML+property)
		if !ok {
			continue
		}
		text, err := d.expressionNodeText(object, scope)
		if err != nil {
			return "", err
		}
		el.expressions[rdf.SysML+property] = text
	}
	words := []string{"in"}
	if d.boolOf(el, rdf.SysML+"isReference") {
		words = append(words, "ref")
	}
	words = append(words, nameText(name))
	relationships, err := d.relationshipWords(el, d.multiplicityText(el))
	if err != nil {
		return "", err
	}
	words = append(words, relationships...)
	head := strings.Join(words, " ")
	if value, ok := d.stringOf(el, rdf.SysML+pValue); ok {
		head += " = " + value
	}
	members, err := d.bodyDeclarationsText(param, scope)
	if err != nil {
		return "", err
	}
	if len(members) == 0 {
		return head + ";", nil
	}
	return head + " { " + strings.Join(members, " ") + " }", nil
}

// bodyDeclarationsText writes what an expression body declares, parameters and
// members merged by the one sysx:memberIndex they were written in.
func (d *decoder) bodyDeclarationsText(node rdf.Term, scope string) ([]string, error) {
	type declaration struct {
		term  rdf.Term
		param bool
	}
	var declarations []declaration
	for _, param := range d.graph.Objects(node, rdf.OpenSysML+xBodyParameter) {
		declarations = append(declarations, declaration{term: param, param: true})
	}
	for _, member := range d.graph.Objects(node, rdf.OpenSysML+xBodyMember) {
		declarations = append(declarations, declaration{term: member})
	}
	sort.SliceStable(declarations, func(i, j int) bool {
		return d.intOf(declarations[i].term, rdf.OpenSysML+xMemberIndex) < d.intOf(declarations[j].term, rdf.OpenSysML+xMemberIndex)
	})
	var out []string
	for _, decl := range declarations {
		var (
			text string
			err  error
		)
		if decl.param {
			text, err = d.bodyParameterText(decl.term, scope)
		} else {
			text, err = d.bodyMemberText(decl.term, scope)
		}
		if err != nil {
			return nil, err
		}
		out = append(out, text)
	}
	return out, nil
}

// bodyMemberText writes one declaration of an expression body: documentation
// from its structure, anything else from its notation, or reports it by name.
func (d *decoder) bodyMemberText(member rdf.Term, scope string) (string, error) {
	if rdf.LocalName(d.graph.Type(member)) == mDocumentation {
		return d.documentationHead(&element{iri: member.Value, scope: scope, expressions: map[string]string{}}), nil
	}
	text, ok := d.graph.Lexical(member, rdf.OpenSysML+xSourceText)
	if !ok || text == "" {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the body member <%s>", member.Value),
			Note: "a declaration inside an expression body is carried as its notation in sysx:sourceText, and this one has none; " + rdfLimitationsNote,
		}
	}
	return strings.TrimSpace(text), nil
}

// operatorText rebuilds an operator expression, parenthesized so the notation
// means the tree the graph states without recording precedence.
func (d *decoder) operatorText(node rdf.Term, scope string) (string, error) {
	operator, ok := d.graph.Lexical(node, rdf.SysML+pOperator)
	if !ok {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the expression <%s>", node.Value),
			Note: "an operator expression states the operator it applies",
		}
	}
	args, err := d.expressionArguments(node, scope)
	if err != nil {
		return "", err
	}
	typeArgument, hasType, err := d.expressionTypeArgument(node, scope)
	if err != nil {
		return "", err
	}
	switch {
	case operator == opSequence:
		return "(" + strings.Join(args, ", ") + ")", nil
	case operator == opIf && len(args) == 3:
		return "if " + args[0] + " ? " + args[1] + " else " + args[2], nil
	case operator == opIndex && len(args) == 2:
		return args[0] + "[" + args[1] + "]", nil
	case operator == opAt && len(args) == 2:
		return args[0] + "#(" + args[1] + ")", nil
	case hasType && len(args) == 1:
		return "(" + args[0] + " " + operator + " " + typeArgument + ")", nil
	case hasType && len(args) == 0:
		multiplicity, err := d.expressionMultiplicityText(node, scope)
		if err != nil {
			return "", err
		}
		return "(" + operator + " " + typeArgument + multiplicity + ")", nil
	case len(args) == 1:
		return "(" + operator + " " + args[0] + ")", nil
	case len(args) == 2:
		return "(" + args[0] + " " + operator + " " + args[1] + ")", nil
	}
	return "", &UnsupportedError{
		What: fmt.Sprintf("the expression <%s>", node.Value),
		Note: fmt.Sprintf("the operator %q is written with %d operand(s), which has no notation", operator, len(args)),
	}
}

func (d *decoder) invocationText(node rdf.Term, scope string) (string, error) {
	function, err := d.expressionReference(node, rdf.SysML+pFunction, scope,
		"an invocation names the function it invokes")
	if err != nil {
		return "", err
	}
	args, err := d.expressionArguments(node, scope)
	if err != nil {
		return "", err
	}
	call := function + "(" + strings.Join(args, ", ") + ")"
	if d.graph.BoolValue(node, rdf.OpenSysML+xIsConstructor) {
		call = "new " + call
	}
	if operand, ok := d.graph.Object(node, rdf.SysML+pOperand); ok {
		receiver, err := d.expressionNodeText(operand, scope)
		if err != nil {
			return "", err
		}
		return receiver + "->" + call, nil
	}
	return call, nil
}

// expressionArguments writes the operands in the order sysx:argumentIndex records.
func (d *decoder) expressionArguments(node rdf.Term, scope string) ([]string, error) {
	type argument struct {
		index int
		text  string
		name  string
	}
	objects := d.graph.Objects(node, rdf.SysML+pArgument)
	args := make([]argument, 0, len(objects))
	for i, object := range objects {
		text, err := d.expressionNodeText(object, scope)
		if err != nil {
			return nil, err
		}
		arg := argument{index: i, text: text}
		if written, ok := d.graph.Lexical(object, rdf.OpenSysML+xArgumentIndex); ok {
			if parsed, err := strconv.Atoi(written); err == nil {
				arg.index = parsed
			}
		}
		if name, ok := d.graph.Lexical(object, rdf.OpenSysML+xArgumentName); ok {
			arg.name = qualifiedNameText(name)
		}
		args = append(args, arg)
	}
	sort.SliceStable(args, func(i, j int) bool { return args[i].index < args[j].index })
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if arg.name != "" {
			out = append(out, arg.name+" = "+arg.text)
			continue
		}
		out = append(out, arg.text)
	}
	return out, nil
}

// expressionReference names the element an expression property points at.
func (d *decoder) expressionReference(node rdf.Term, property, scope, why string) (string, error) {
	object, ok := d.graph.Object(node, property)
	if !ok {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the expression <%s>", node.Value),
			Note: why,
		}
	}
	return d.referenceName(object, scope)
}

// expressionMultiplicityText writes the `[lower..upper]` a cast states, its
// bounds being expressions of their own.
func (d *decoder) expressionMultiplicityText(node rdf.Term, scope string) (string, error) {
	el := &element{iri: node.Value, scope: scope, expressions: map[string]string{}}
	for _, property := range []string{pLowerBound, pUpperBound} {
		object, ok := d.graph.Object(node, rdf.SysML+property)
		if !ok {
			continue
		}
		text, err := d.expressionNodeText(object, scope)
		if err != nil {
			return "", err
		}
		el.expressions[rdf.SysML+property] = text
	}
	return d.multiplicityText(el), nil
}

// expressionTypeArgument names the type a classification operator applies.
func (d *decoder) expressionTypeArgument(node rdf.Term, scope string) (string, bool, error) {
	object, ok := d.graph.Object(node, rdf.OpenSysML+xTypeArgument)
	if !ok {
		return "", false, nil
	}
	name, err := d.referenceName(object, scope)
	if err != nil {
		return "", false, err
	}
	return name, true, nil
}
