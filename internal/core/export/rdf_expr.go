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
	xResultExpression = "resultExpression"
)

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
	switch n := node.(type) {
	case *ast.LiteralBool:
		e.typed(subject, mLiteralBoolean)
		e.graph.Add(subject, e.sysml(pValue), rdf.Bool(n.Value))

	case *ast.LiteralString:
		e.typed(subject, mLiteralString)
		e.graph.Add(subject, e.sysml(pValue), rdf.String(unquote(n.Value)))

	case *ast.LiteralInteger:
		e.typed(subject, mLiteralInteger)
		e.graph.Add(subject, e.sysml(pValue), rdf.TypedLiteral(n.Value, rdf.XSD+"integer"))

	case *ast.LiteralReal:
		e.typed(subject, mLiteralRational)
		e.graph.Add(subject, e.sysml(pValue), rdf.TypedLiteral(n.Value, rdf.XSD+"decimal"))

	case *ast.LiteralInfinity:
		e.typed(subject, mLiteralInfinity)

	case *ast.NullExpr:
		e.typed(subject, mNullExpression)

	case *ast.QualifiedName:
		// A position whose notation is a bare name holds the feature it names.
		e.typed(subject, mFeatureReference)
		e.graph.Add(subject, e.sysml(pReferent), e.reference(n))

	case *ast.FeatureReference:
		e.typed(subject, mFeatureReference)
		e.graph.Add(subject, e.sysml(pReferent), e.reference(n.Name))

	case *ast.OperatorExpr:
		e.typed(subject, mOperator)
		e.graph.Add(subject, e.sysml(pOperator), rdf.String(n.Operator.String()))
		e.arguments(subject, owner, n.Operands)
		if n.TypeRef != nil {
			e.graph.Add(subject, e.sysx(xTypeArgument), e.reference(n.TypeRef))
		}

	case *ast.CastExpr:
		// `(as T)` is the classification operator with a type argument only.
		e.typed(subject, mOperator)
		e.graph.Add(subject, e.sysml(pOperator), rdf.String(ast.OpAs.String()))
		e.graph.Add(subject, e.sysx(xTypeArgument), e.reference(n.TargetType))

	case *ast.FeatureChainExpr:
		e.typed(subject, mFeatureChain)
		e.arguments(subject, owner, []ast.Node{n.Operand})
		e.graph.Add(subject, e.sysml(pTargetFeature), e.reference(n.Member))

	case *ast.IndexExpr:
		e.typed(subject, mOperator)
		operator := opAt
		if n.Bracket {
			operator = opIndex
		}
		e.graph.Add(subject, e.sysml(pOperator), rdf.String(operator))
		e.arguments(subject, owner, []ast.Node{n.Operand, n.Index})

	case *ast.InvocationExpr:
		e.typed(subject, mInvocation)
		e.invocation(subject, owner, n.Type, n.Operand, n.Args, n.NamedArgs)

	case *ast.ConstructorExpr:
		// The 202407 rendering declares no ConstructorExpression, so `new` is a flag.
		e.typed(subject, mInvocation)
		e.graph.Add(subject, e.sysx(xIsConstructor), rdf.Bool(true))
		e.invocation(subject, owner, n.Type, nil, n.Args, nil)

	case *ast.CollectExpr:
		e.typed(subject, mCollect)
		e.arguments(subject, owner, []ast.Node{n.Operand, n.Body})

	case *ast.SelectExpr:
		e.typed(subject, mSelect)
		e.arguments(subject, owner, []ast.Node{n.Operand, n.Body})

	case *ast.SequenceExpr:
		e.typed(subject, mOperator)
		e.graph.Add(subject, e.sysml(pOperator), rdf.String(opSequence))
		e.arguments(subject, owner, n.Elements)

	case *ast.MetadataAccessExpr:
		e.typed(subject, mMetadataAccess)
		e.graph.Add(subject, e.sysml(pReferencedElement), e.reference(n.Ref))

	case *ast.BodyExpr:
		// A body declares its own parameters and a result expression.
		e.typed(subject, mExpression)
		for _, param := range n.Params {
			if param.Name != "" {
				e.graph.Add(subject, e.sysx(xBodyParameter), rdf.String(param.Name))
			}
		}
		if n.Result != nil {
			result := rdf.ExpressionIRI(subject, "result")
			e.graph.Add(subject, e.sysx(xResultExpression), result)
			e.expressionNode(result, owner, n.Result)
		}

	default:
		// A shape this mapping does not decompose still states it is an expression.
		e.typed(subject, mExpression)
	}
}

func (e *encoder) typed(subject rdf.Term, metaclass string) {
	e.graph.Add(subject, rdf.IRI(rdf.RDFType), rdf.SysMLTerm(metaclass))
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
		e.graph.Add(subject, e.sysml(pFunction), e.reference(function))
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
// an element: it is in the expression namespace, or its metaclass is one.
func (d *decoder) isExpressionNode(subject rdf.Term) bool {
	if !subject.IsIRI() {
		return false
	}
	if strings.HasPrefix(subject.Value, rdf.Expression) {
		return true
	}
	return expressionMetaclasses[rdf.LocalName(d.graph.Type(subject))]
}

// resolveExpressions renders every element's expression-valued properties as
// notation, so the printer reads one text per property.
func (d *decoder) resolveExpressions() error {
	for _, triple := range d.graph.Triples() {
		if !d.isExpressionNode(triple.Object) {
			continue
		}
		el, ok := d.byIRI[triple.Subject.Value]
		if !ok {
			// The subject is an expression node; its parts are written with it.
			continue
		}
		text, err := d.expressionNodeText(triple.Object, el)
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
func (d *decoder) expressionNodeText(node rdf.Term, in *element) (string, error) {
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
		return `"` + value + `"`, nil
	case mLiteralInfinity:
		return "*", nil
	case mNullExpression:
		return "null", nil
	case mFeatureReference:
		return d.expressionReference(node, rdf.SysML+pReferent, in,
			"a feature reference names the feature it reads")
	case mMetadataAccess:
		name, err := d.expressionReference(node, rdf.SysML+pReferencedElement, in,
			"a metadata access names the element it reads the metadata of")
		if err != nil {
			return "", err
		}
		return name + ".metadata", nil
	case mFeatureChain:
		operands, err := d.expressionArguments(node, in)
		if err != nil {
			return "", err
		}
		object, ok := d.graph.Object(node, rdf.SysML+pTargetFeature)
		if !ok {
			return "", unsupported("a feature chain names the feature it reaches")
		}
		member, err := d.memberName(object)
		if err != nil {
			return "", err
		}
		if len(operands) != 1 {
			return "", unsupported("a feature chain applies to exactly one operand")
		}
		return operands[0] + "." + member, nil
	case mCollect, mSelect:
		operands, err := d.expressionArguments(node, in)
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
		return d.operatorText(node, in)
	case mInvocation:
		return d.invocationText(node, in)
	}
	return "", unsupported("this expression states no notation and no structure to write one from; " + rdfLimitationsNote)
}

// operatorText rebuilds an operator expression, parenthesized so the notation
// means the tree the graph states without recording precedence.
func (d *decoder) operatorText(node rdf.Term, in *element) (string, error) {
	operator, ok := d.graph.Lexical(node, rdf.SysML+pOperator)
	if !ok {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the expression <%s>", node.Value),
			Note: "an operator expression states the operator it applies",
		}
	}
	args, err := d.expressionArguments(node, in)
	if err != nil {
		return "", err
	}
	typeArgument, hasType, err := d.expressionTypeArgument(node, in)
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
		return "(" + operator + " " + typeArgument + ")", nil
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

func (d *decoder) invocationText(node rdf.Term, in *element) (string, error) {
	function, err := d.expressionReference(node, rdf.SysML+pFunction, in,
		"an invocation names the function it invokes")
	if err != nil {
		return "", err
	}
	args, err := d.expressionArguments(node, in)
	if err != nil {
		return "", err
	}
	call := function + "(" + strings.Join(args, ", ") + ")"
	if d.graph.BoolValue(node, rdf.OpenSysML+xIsConstructor) {
		call = "new " + call
	}
	if operand, ok := d.graph.Object(node, rdf.SysML+pOperand); ok {
		receiver, err := d.expressionNodeText(operand, in)
		if err != nil {
			return "", err
		}
		return receiver + "->" + call, nil
	}
	return call, nil
}

// expressionArguments writes the operands in the order sysx:argumentIndex records.
func (d *decoder) expressionArguments(node rdf.Term, in *element) ([]string, error) {
	type argument struct {
		index int
		text  string
		name  string
	}
	objects := d.graph.Objects(node, rdf.SysML+pArgument)
	args := make([]argument, 0, len(objects))
	for i, object := range objects {
		text, err := d.expressionNodeText(object, in)
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
			arg.name = name
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
func (d *decoder) expressionReference(node rdf.Term, property string, in *element, why string) (string, error) {
	object, ok := d.graph.Object(node, property)
	if !ok {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the expression <%s>", node.Value),
			Note: why,
		}
	}
	return d.referenceName(object, in)
}

// expressionTypeArgument names the type a classification operator applies.
func (d *decoder) expressionTypeArgument(node rdf.Term, in *element) (string, bool, error) {
	object, ok := d.graph.Object(node, rdf.OpenSysML+xTypeArgument)
	if !ok {
		return "", false, nil
	}
	name, err := d.referenceName(object, in)
	if err != nil {
		return "", false, err
	}
	return name, true, nil
}
