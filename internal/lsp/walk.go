package lsp

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// qnRef is a qualified-name reference paired with the scope it resolves against.
type qnRef struct {
	qn    *ast.QualifiedName
	scope *symbols.Scope
}

// collectRefs walks a document's scope tree and AST, gathering every
// QualifiedName reference with its resolution scope. It mirrors
// resolve/document.go's traversal: a form handled there but not here is a
// reference the editor cannot find.
func collectRefs(root *ast.RootNamespace, rootScope *symbols.Scope) []qnRef {
	c := &refCollector{}
	if root != nil && rootScope != nil {
		c.walkMembers(rootScope, root.Members)
	}
	return c.refs
}

type refCollector struct {
	refs []qnRef
}

func (c *refCollector) add(scope *symbols.Scope, qn *ast.QualifiedName) {
	if qn != nil {
		c.refs = append(c.refs, qnRef{qn: qn, scope: scope})
	}
}

func (c *refCollector) childScope(scope *symbols.Scope, decl ast.Node) *symbols.Scope {
	for _, ch := range scope.Children() {
		if ch.Node() == decl {
			return ch
		}
	}
	return nil
}

func (c *refCollector) walkMembers(scope *symbols.Scope, members []ast.Node) {
	for _, m := range members {
		decl := m
		switch v := m.(type) {
		case *ast.Membership:
			decl = v.Member
		}
		c.resolveDecl(scope, decl)
	}
}

func (c *refCollector) resolveDecl(scope *symbols.Scope, decl ast.Node) {
	switch d := decl.(type) {
	case *ast.Package:
		c.prefixes(scope, d.Prefixes)
		if child := c.childScope(scope, d); child != nil {
			c.walkMembers(child, d.Members)
		}
	case *ast.Namespace:
		c.prefixes(scope, d.Prefixes)
		if child := c.childScope(scope, d); child != nil {
			c.walkMembers(child, d.Members)
		}
	case *ast.Import:
		c.add(scope, d.Imported)
	case *ast.Alias:
		c.add(scope, d.For)
	case *ast.Dependency:
		c.prefixes(scope, d.Prefixes)
		for _, cl := range d.Clients {
			c.add(scope, cl)
		}
		for _, sp := range d.Suppliers {
			c.add(scope, sp)
		}
	case *ast.Comment:
		for _, a := range d.About {
			c.add(scope, a)
		}
	case *ast.FilterMember:
		c.expr(scope, d.Condition)
	case *ast.Definition:
		c.prefixes(scope, d.Prefixes)
		c.relationships(scope, d, d.Relationships)
		if child := c.childScope(scope, d); child != nil {
			c.walkMembers(child, d.Members)
		}
	case *ast.Usage:
		c.prefixes(scope, d.Prefixes)
		c.relationships(scope, d, d.Relationships)
		c.multiplicity(scope, d.Multiplicity)
		c.expr(scope, d.Value)
		for _, end := range d.ConnectorEnds {
			if end == nil {
				continue
			}
			c.target(scope, end.Target)
			c.target(scope, end.Reference)
		}
		if d.FlowEnds != nil {
			c.expr(scope, d.FlowEnds.From)
			c.expr(scope, d.FlowEnds.To)
			c.expr(scope, d.FlowEnds.Payload)
		}
		if child := c.childScope(scope, d); child != nil {
			c.walkMembers(child, d.Members)
		}
	case *ast.SubjectMember:
		c.add(scope, d.TypeRef)
		c.multiplicity(scope, d.Multiplicity)
		c.expr(scope, d.BindingExpr)
		if child := c.childScope(scope, d); child != nil {
			c.walkMembers(child, d.Body)
		}
	case *ast.InitialNode:
		// The node's own name is a label, not a reference.
		c.add(scope, d.Successor)
		c.expr(scope, d.Guard)
	case *ast.ResultMember:
		c.expr(scope, d.Expression)
	case *ast.ConstraintMember:
		c.expr(scope, d.Expression)
	case *ast.AssumeMember:
		c.expr(scope, d.Expression)
	case *ast.RequireMember:
		c.expr(scope, d.Expression)
		c.walkMembers(scope, d.Body)
	case *ast.ActorMember:
		c.add(scope, d.TypeRef)
		c.expr(scope, d.BindingExpr)
	case *ast.EntryMember:
		c.walkMembers(scope, d.Actions)
	case *ast.DoMember:
		c.walkMembers(scope, d.Actions)
	case *ast.ExitMember:
		c.walkMembers(scope, d.Actions)
	case *ast.StateNode:
		body := scope
		if child := c.childScope(scope, d); child != nil {
			body = child
		}
		c.walkMembers(body, d.Entry)
		c.walkMembers(body, d.Do)
		c.walkMembers(body, d.Exit)
		c.walkMembers(body, d.Substates)
		for _, region := range d.Regions {
			c.resolveDecl(body, region)
		}
	case *ast.StateRegion:
		states := scope
		if child := c.childScope(scope, d); child != nil {
			states = child
		}
		c.walkMembers(states, d.States)
	case *ast.TransitionMember:
		c.add(scope, d.Source)
		c.add(scope, d.Target)
		c.trigger(scope, d.Trigger)
		c.expr(scope, d.Guard)
		c.walkMembers(scope, d.Effect)
	case *ast.SendStatement:
		c.expr(scope, d.Message)
		c.expr(scope, d.Target)
	case *ast.TerminateStatement:
		c.expr(scope, d.Target)
	case *ast.AssignmentActionNode:
		c.expr(scope, d.Target)
		c.expr(scope, d.Value)
	case *ast.ActionExecutionNode:
		c.add(scope, d.ActionRef)
		c.expr(scope, d.Expression)
	case *ast.PerformActionNode:
		c.expr(scope, d.ActionRef)
	case *ast.WhileLoopActionNode:
		// A loop owns the scope its body declares into (see symbols.buildDecl).
		body := scope
		if child := c.childScope(scope, d); child != nil {
			body = child
		}
		c.expr(body, d.Condition)
		c.walkMembers(body, d.Body)
	case *ast.IfActionNode:
		c.expr(scope, d.Condition)
		c.walkMembers(scope, d.ThenBody)
		c.walkMembers(scope, d.ElseBody)
	}
}

// trigger collects the references a transition trigger carries. Bare signal and
// call event names are not model references (see resolve/document.go), so they
// are skipped here too: renaming a declaration must not rewrite them.
func (c *refCollector) trigger(scope *symbols.Scope, trigger ast.Node) {
	switch t := trigger.(type) {
	case nil:
		return
	case *ast.TimeEvent:
		c.expr(scope, t.Duration)
	case *ast.ChangeEvent:
		c.expr(scope, t.Condition)
	case *ast.QualifiedName, *ast.FeatureReference, *ast.AcceptEvent, *ast.CallEvent:
		// Event names, not model elements.
	default:
		c.expr(scope, trigger)
	}
}

// relationships collects the targets of typings, specializations, subsettings
// and redefinitions (`: T`, `:> T`, `:>> T`) owned by decl. A reference
// subsetting is collected in the scope it resolves in, which skips decl's own
// binding of the name it references (see resolve.ReferenceScope).
func (c *refCollector) relationships(scope *symbols.Scope, decl ast.Node, rels []*ast.Relationship) {
	for _, rel := range rels {
		if rel == nil {
			continue
		}
		at := scope
		if rel.Kind == ast.RelReferences {
			at = resolve.ReferenceScope(scope, decl, rel.Target)
		}
		c.target(at, rel.Target)
	}
}

// target collects a node that names something, whether it was parsed as a
// qualified name or wrapped in an expression.
func (c *refCollector) target(scope *symbols.Scope, target ast.Node) {
	if fr, ok := target.(*ast.FeatureReference); ok {
		target = fr.Name
	}
	if qn, ok := target.(*ast.QualifiedName); ok {
		c.add(scope, qn)
		return
	}
	c.expr(scope, target)
}

func (c *refCollector) multiplicity(scope *symbols.Scope, m *ast.Multiplicity) {
	if m == nil {
		return
	}
	c.expr(scope, m.Lower)
	c.expr(scope, m.Upper)
}

func (c *refCollector) prefixes(scope *symbols.Scope, prefixes []*ast.PrefixMetadata) {
	for _, p := range prefixes {
		if p != nil {
			c.add(scope, p.Type)
		}
	}
}

func (c *refCollector) expr(scope *symbols.Scope, e ast.Node) {
	switch v := e.(type) {
	case nil:
		return
	case *ast.FeatureReference:
		c.add(scope, v.Name)
	case *ast.OperatorExpr:
		for _, op := range v.Operands {
			c.expr(scope, op)
		}
		c.add(scope, v.TypeRef)
	case *ast.FeatureChainExpr:
		c.expr(scope, v.Operand)
		c.add(scope, v.Member)
	case *ast.IndexExpr:
		c.expr(scope, v.Operand)
		c.expr(scope, v.Index)
	case *ast.InvocationExpr:
		c.expr(scope, v.Operand)
		c.add(scope, v.Type)
		for _, a := range v.Args {
			c.expr(scope, a)
		}
		for _, na := range v.NamedArgs {
			c.add(scope, na.Name)
			c.expr(scope, na.Value)
		}
	case *ast.CollectExpr:
		c.expr(scope, v.Operand)
		c.expr(scope, v.Body)
	case *ast.SelectExpr:
		c.expr(scope, v.Operand)
		c.expr(scope, v.Body)
	case *ast.ConstructorExpr:
		c.add(scope, v.Type)
		for _, a := range v.Args {
			c.expr(scope, a)
		}
	case *ast.BodyExpr:
		for i := range v.Params {
			p := &v.Params[i]
			c.add(scope, p.Type)
			c.expr(scope, p.Value)
		}
		// The same scope the resolver uses, so a reference to a parameter and
		// its declaration denote one symbol.
		c.expr(symbols.BodyExprScope(scope, v), v.Result)
	case *ast.SequenceExpr:
		for _, el := range v.Elements {
			c.expr(scope, el)
		}
	case *ast.MetadataAccessExpr:
		c.add(scope, v.Ref)
	case *ast.QualifiedName:
		// A bare name in expression position parses straight to a qualified
		// name rather than to a FeatureReference wrapper: `return r = speed;`
		// and `return (speed);` reach here, while `return speed + 1;` arrives
		// as an operand of an OperatorExpr.
		c.add(scope, v)
	}
}

// refAtOffset returns the qnRef whose qualified-name span contains offset.
func refAtOffset(refs []qnRef, offset int) *qnRef {
	for i := range refs {
		sp := refs[i].qn.Span()
		if offset >= sp.Offset && offset < sp.End() {
			return &refs[i]
		}
	}
	return nil
}
