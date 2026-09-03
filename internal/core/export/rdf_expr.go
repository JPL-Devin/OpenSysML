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
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf/ontology"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// exprScope is where an expression's names are read: the element that owns
// the expression, the body-expression scope enclosing it if any, and whether
// it is a filter condition, whose names its own namespace's filters do not
// restrict.
type exprScope struct {
	owner     string
	scope     *symbols.Scope
	condition bool
}

// link resolves name as read in this scope.
func (s exprScope) link(e *encoder, name *ast.QualifiedName) rdf.Term {
	return s.linkReference(e, resolve.Reference{QN: name})
}

func (s exprScope) linkReference(e *encoder, ref resolve.Reference) rdf.Term {
	ref.Scope = s.scope
	ref.Condition = s.condition
	return e.linkReference(s.owner, ref)
}

// inBody is the scope of body's parameters and members, in which its result is read.
func (s exprScope) inBody(e *encoder, body *ast.BodyExpr) exprScope {
	parent := s.scope
	if parent == nil {
		parent = e.scopeOf(s.owner)
	}
	s.scope = symbols.BodyExprScope(parent, body)
	return s
}

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
	e.expressionIn(subject, property, slot, exprScope{owner: owner}, node)
}

// filterExpression is expression for a filter condition, whose names resolve
// unrestricted by the filters of the namespace declaring it.
func (e *encoder) filterExpression(subject, property rdf.Term, slot, owner string, node ast.Node) {
	e.expressionIn(subject, property, slot, exprScope{owner: owner, condition: true}, node)
}

func (e *encoder) expressionIn(subject, property rdf.Term, slot string, in exprScope, node ast.Node) {
	if node == nil {
		return
	}
	e.graph.Prefixes[rdf.ExpressionPrefix] = rdf.Expression
	target := rdf.ExpressionIRI(subject, slot)
	e.graph.Add(subject, property, target)
	e.expressionNode(target, in, node)
}

// expressionNode emits one expression node and, recursively, its operands.
// Every node carries its notation, so the exact text always survives.
func (e *encoder) expressionNode(subject rdf.Term, in exprScope, node ast.Node) {
	e.graph.Add(subject, e.sysx(xSourceText), rdf.String(e.text(node)))
	// The id an API reader addresses the node by, as on an element: a node has no
	// qualified name, but its position in the model gives it a valid id.
	e.graph.Add(subject, e.sysml(pElementID), rdf.String(rdf.LocalName(subject.Value)))
	e.expressionStructure(subject, in, node)
}

// expressionStructure emits an expression's type and operands; an Expression
// element states its text as an element does, so it takes none here.
func (e *encoder) expressionStructure(subject rdf.Term, in exprScope, node ast.Node) {
	e.typed(subject, expressionMetaclass(node))
	switch n := node.(type) {
	case *ast.LiteralBool:
		e.graph.Add(subject, e.sysml(pValue), rdf.Bool(n.Value))

	case *ast.LiteralString:
		e.graph.Add(subject, e.sysml(pValue), rdf.String(lexer.StringValue(n.Value)))

	case *ast.LiteralInteger:
		e.graph.Add(subject, e.sysml(pValue), rdf.TypedLiteral(n.Value, rdf.XSD+"integer"))

	case *ast.LiteralReal:
		e.graph.Add(subject, e.sysml(pValue), rdf.TypedLiteral(n.Value, realDatatype(n.Value)))

	case *ast.QualifiedName:
		// A position whose notation is a bare name holds the feature it names.
		e.graph.Add(subject, e.sysml(pReferent), in.link(e, n))

	case *ast.FeatureReference:
		e.graph.Add(subject, e.sysml(pReferent), in.link(e, n.Name))

	case *ast.OperatorExpr:
		e.graph.Add(subject, e.sysml(pOperator), rdf.String(n.Operator.String()))
		e.arguments(subject, in, n.Operands)
		if n.TypeRef != nil {
			e.graph.Add(subject, e.sysx(xTypeArgument), in.link(e, n.TypeRef))
		}

	case *ast.CastExpr:
		// `(as T[m])` is the classification operator with a type argument only,
		// its multiplicity written as bounds the way a usage's is.
		e.graph.Add(subject, e.sysml(pOperator), rdf.String(ast.OpAs.String()))
		e.graph.Add(subject, e.sysx(xTypeArgument), in.link(e, n.TargetType))
		e.multiplicityIn(subject, in, n.Multiplicity)

	case *ast.FeatureChainExpr:
		e.arguments(subject, in, []ast.Node{n.Operand})
		e.graph.Add(subject, e.sysml(pTargetFeature), in.linkReference(e, resolve.Reference{QN: n.Member, Chain: n}))

	case *ast.IndexExpr:
		operator := opAt
		if n.Bracket {
			operator = opIndex
		}
		e.graph.Add(subject, e.sysml(pOperator), rdf.String(operator))
		e.arguments(subject, in, []ast.Node{n.Operand, n.Index})

	case *ast.InvocationExpr:
		e.invocation(subject, in, n.Type, n.Operand, n.Args, n.NamedArgs)

	case *ast.ConstructorExpr:
		// The 202407 rendering declares no ConstructorExpression, so `new` is a flag.
		e.graph.Add(subject, e.sysx(xIsConstructor), rdf.Bool(true))
		e.invocation(subject, in, n.Type, nil, n.Args, n.NamedArgs)

	case *ast.CollectExpr:
		e.arguments(subject, in, []ast.Node{n.Operand, n.Body})

	case *ast.SelectExpr:
		e.arguments(subject, in, []ast.Node{n.Operand, n.Body})

	case *ast.SequenceExpr:
		e.graph.Add(subject, e.sysml(pOperator), rdf.String(opSequence))
		e.arguments(subject, in, n.Elements)

	case *ast.MetadataAccessExpr:
		e.graph.Add(subject, e.sysml(pReferencedElement), in.link(e, n.Ref))

	case *ast.BodyExpr:
		// A body declares its own parameters and members, then a result expression;
		// sysx:hasBody marks it as one even when it declares nothing. The
		// parameters' annotations are read outside the body, the result inside.
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(true))
		e.bodyDeclarations(subject, in, n.Params, n.Members)
		if n.Result != nil {
			result := rdf.ExpressionIRI(subject, "result")
			e.graph.Add(subject, e.sysx(xResultExpression), result)
			e.expressionNode(result, in.inBody(e, n), n.Result)
		}
	}
}

// realDatatype is the datatype whose lexical space holds a REAL_VALUE token:
// xsd:decimal, or xsd:double when the token has an exponent.
func realDatatype(value string) string {
	if strings.ContainsAny(value, "eE") {
		return rdf.XSD + "double"
	}
	return rdf.XSD + "decimal"
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
// in is the scope enclosing the body, where a parameter's own names are read.
func (e *encoder) bodyDeclarations(subject rdf.Term, in exprScope, params []ast.BodyParam, members []ast.Node) {
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
			e.bodyParameter(subject, in, i, *decl.param)
		} else {
			e.bodyMember(subject, i, decl.member)
		}
	}
}

// bodyParameter emits one parameter of an expression body as a node of its
// own, so its type, value and bounds are structure, not text.
func (e *encoder) bodyParameter(subject rdf.Term, in exprScope, index int, param ast.BodyParam) {
	node := rdf.ExpressionIRI(subject, fmt.Sprintf("in%d", index))
	e.graph.Add(subject, e.sysx(xBodyParameter), node)
	e.graph.Add(node, rdf.IRI(rdf.RDFType), rdf.SysMLTerm(keywordMetaclass["ref"]))
	e.graph.Add(node, e.sysml(pElementID), rdf.String(rdf.LocalName(node.Value)))
	e.graph.Add(node, e.sysx(xMemberIndex), rdf.Int(index))
	e.graph.Add(node, e.sysml(pDirection), rdf.String(directionKeyword(ast.DirIn)))
	e.name(node, param.Name)
	e.flags(node, []boolProperty{{"isReference", param.IsReference}})
	if param.Type != nil {
		e.graph.Add(node, e.sysml(relationshipProperty[ast.RelTyping]), in.link(e, param.Type))
	}
	e.relationshipsIn(node, nil, in, param.Relationships)
	e.multiplicityIn(node, in, param.Multiplicity)
	e.expressionIn(node, e.sysml(pValue), pValue, in, param.Value)
	e.bodyDeclarations(node, in, nil, param.Members)
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
func (e *encoder) arguments(subject rdf.Term, in exprScope, args []ast.Node) {
	for i, arg := range args {
		if arg == nil {
			continue
		}
		child := rdf.ExpressionIRI(subject, fmt.Sprintf("a%d", i))
		e.graph.Add(subject, e.sysml(pArgument), child)
		e.expressionNode(child, in, arg)
		e.graph.Add(child, e.sysx(xArgumentIndex), rdf.Int(i))
	}
}

// invocation emits the parts an invocation and a constructor share: the function
// invoked, the receiver of a `->` form, and the arguments.
func (e *encoder) invocation(subject rdf.Term, in exprScope, function *ast.QualifiedName, operand ast.Node, args []ast.Node, named []ast.NamedArg) {
	if function != nil {
		e.graph.Add(subject, e.sysml(pFunction), in.link(e, function))
	}
	if operand != nil {
		receiver := rdf.ExpressionIRI(subject, "operand")
		e.graph.Add(subject, e.sysml(pOperand), receiver)
		e.expressionNode(receiver, in, operand)
	}
	e.arguments(subject, in, args)
	for i, arg := range named {
		if arg.Value == nil {
			continue
		}
		child := rdf.ExpressionIRI(subject, fmt.Sprintf("n%d", i))
		e.graph.Add(subject, e.sysml(pArgument), child)
		e.expressionNode(child, in, arg.Value)
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

// isExpressionNode reports whether a subject is an expression node rather than an
// element: unowned, and in the expression namespace or an unnamed expression class.
func (d *decoder) isExpressionNode(subject rdf.Term) bool {
	if !subject.IsIRI() {
		return false
	}
	if _, owned := d.owningMembership[subject.Value]; owned || d.graph.HasProperty(subject, rdf.SysML+pOwningMembership) {
		return false
	}
	return strings.HasPrefix(subject.Value, rdf.Expression) ||
		expressionMetaclasses[d.metaclass(subject)] && !d.graph.HasProperty(subject, rdf.SysML+pQualifiedName)
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
		text, err := d.expressionNodeText(triple.Object, expressionScope(el, triple.Predicate.Value))
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

// realValueText spells a number as a REAL_VALUE token (`3` becomes `3.0`);
// a signed or non-finite form has no such token.
func realValueText(lexical string) (string, bool) {
	if lexer.IsRealValue(lexical) {
		return lexical, true
	}
	mantissa, exponent := lexical, ""
	if i := strings.IndexAny(lexical, "eE"); i >= 0 {
		mantissa, exponent = lexical[:i], lexical[i:]
	}
	switch {
	case lexer.IsDecimalValue(mantissa):
		mantissa += ".0"
	case lexer.IsDecimalValue(strings.TrimSuffix(mantissa, ".")):
		mantissa += "0"
	default:
		return "", false
	}
	text := mantissa + exponent
	return text, lexer.IsRealValue(text)
}

// expressionScope is what a reference in el's property is written relative to:
// a loop's conditions are read in the loop's own scope, whose body they test.
func expressionScope(el *element, property string) string {
	switch property {
	case rdf.OpenSysML + xWhileCondition, rdf.OpenSysML + xUntilCondition:
		return el.qname
	case rdf.OpenSysML + xGuard:
		if el.metaclass == mTransition {
			return el.qname
		}
	}
	return el.scope
}

// expressionNodeText writes an expression node back as notation: the notation it
// kept, or notation rebuilt from its structure when it kept none it can use.
func (d *decoder) expressionNodeText(node rdf.Term, scope string) (string, error) {
	if text, ok := d.expressionText(node); ok {
		return text, nil
	}
	metaclass := d.metaclass(node)
	unsupported := func(note string) error {
		return &UnsupportedError{
			What: fmt.Sprintf("the expression <%s>", node.Value),
			Note: note,
		}
	}
	switch metaclass {
	case mLiteralBoolean:
		if !d.graph.HasProperty(node, rdf.SysML+pValue) {
			return "", unsupported("a literal expression states the value it evaluates to")
		}
		return strconv.FormatBool(d.graph.BoolValue(node, rdf.SysML+pValue)), nil
	case mLiteralInteger:
		value, ok := d.graph.Lexical(node, rdf.SysML+pValue)
		if !ok {
			return "", unsupported("a literal expression states the value it evaluates to")
		}
		if !lexer.IsDecimalValue(value) {
			return "", unsupported(fmt.Sprintf("the notation spells an integer literal as digits alone, not %q; a sign is an OperatorExpression applied to it", value))
		}
		return value, nil
	case mLiteralRational:
		value, ok := d.graph.Lexical(node, rdf.SysML+pValue)
		if !ok {
			return "", unsupported("a literal expression states the value it evaluates to")
		}
		text, ok := realValueText(value)
		if !ok {
			return "", unsupported(fmt.Sprintf("the notation spells a rational literal as an unsigned finite number, not %q; a sign is an OperatorExpression applied to it", value))
		}
		return text, nil
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
		member, err := d.chainMember(node, scope)
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
	what := fmt.Sprintf("the body parameter <%s>", param.Value)
	// A body parameter is written `in name`, so the node must be a Feature whose
	// direction, when stated, is in; any other shape would be rewritten, not kept.
	if metaclass := d.metaclass(param); !ontology.IsAncestorOrSelf(metaclass, "Feature") {
		return "", &UnsupportedError{
			What: what,
			Note: fmt.Sprintf("a parameter of an expression body is a Feature, and this one is %s", typeDescription(metaclass)),
		}
	}
	if direction, ok := d.stringOf(el, rdf.SysML+pDirection); ok && direction != directionKeyword(ast.DirIn) {
		return "", &UnsupportedError{
			What: what,
			Note: fmt.Sprintf("a parameter of an expression body is written `in`, and its sysml:direction is %q", direction),
		}
	}
	name, ok := d.stringOf(el, rdf.SysML+pDeclaredName)
	if !ok {
		return "", &UnsupportedError{
			What: what,
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

func typeDescription(metaclass string) string {
	if metaclass == "" {
		return "of no rdf:type"
	}
	return "a " + curie(rdf.SysML+metaclass)
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
		return intOf(d.graph, declarations[i].term, rdf.OpenSysML+xMemberIndex) < intOf(d.graph, declarations[j].term, rdf.OpenSysML+xMemberIndex)
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
	if d.metaclass(member) == mDocumentation {
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

// chainMember names the feature a chain reaches. A linked one is written by its
// own name, which the operand looks up among its members, unless the operand
// reaches another feature so named too; then the qualified spelling keeps it.
func (d *decoder) chainMember(node rdf.Term, scope string) (string, error) {
	object, ok := d.graph.Object(node, rdf.SysML+pTargetFeature)
	if !ok {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the expression <%s>", node.Value),
			Note: "a feature chain names the feature it reaches",
		}
	}
	if object.IsLiteral() {
		return d.referenceName(object, scope)
	}
	target, err := d.referencedElement(object.Value)
	if err != nil {
		return "", err
	}
	name, ok := d.stringOf(target, rdf.SysML+pDeclaredName)
	if !ok {
		return d.referenceName(object, scope)
	}
	operand, ok := d.chainOperand(node)
	if !ok {
		return d.referenceName(object, scope)
	}
	if named := d.operandMembersNamed(operand, name); len(named) != 1 || named[0] != target {
		return d.referenceName(object, scope)
	}
	return qualifiedNameText(name), nil
}

// chainOperand is the element a chain's operand refers to: the feature a
// reference links, or the one an inner chain reaches; another shape is unknown.
func (d *decoder) chainOperand(node rdf.Term) (*element, bool) {
	arg, ok := d.graph.Object(node, rdf.SysML+pArgument)
	if !ok {
		return nil, false
	}
	var linked rdf.Term
	ok = false
	switch d.metaclass(arg) {
	case mFeatureReference:
		linked, ok = d.graph.Object(arg, rdf.SysML+pReferent)
	case mFeatureChain:
		linked, ok = d.graph.Object(arg, rdf.SysML+pTargetFeature)
	}
	if !ok || linked.IsLiteral() {
		return nil, false
	}
	el, ok := d.byIRI[linked.Value]
	return el, ok
}

// operandMembersNamed collects the features named name a chain operand reaches:
// its own member of that name, or else those its types declare or inherit.
func (d *decoder) operandMembersNamed(operand *element, name string) []*element {
	if own := d.memberNamed(operand, name); own != nil {
		return []*element{own}
	}
	var found []*element
	for _, term := range d.graph.Objects(rdf.IRI(operand.iri), rdf.SysML+relationshipProperty[ast.RelTyping]) {
		if term.IsLiteral() {
			continue
		}
		typ, ok := d.byIRI[term.Value]
		if !ok {
			continue
		}
		if own := d.memberNamed(typ, name); own != nil {
			found = append(found, own)
			continue
		}
		found = append(found, d.inheritedNamed(typ, name)...)
	}
	return found
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
