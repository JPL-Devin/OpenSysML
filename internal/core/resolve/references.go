package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// References walks a document's scope tree and AST, gathering every
// QualifiedName reference with the scope it resolves in. It mirrors
// document.go's traversal: a form handled there but not here is a reference a
// consumer walking the document — the editor's navigation and highlighting —
// cannot find.
func References(root *ast.RootNamespace, rootScope *symbols.Scope) []Reference {
	c := &refCollector{}
	if root != nil && rootScope != nil {
		c.walkMembers(rootScope, root.Members)
	}
	return c.refs
}

type refCollector struct {
	refs []Reference
	// condition is set while the names of an element-filter condition are walked.
	condition bool
}

// push records a reference, marking it as a filter condition's own name when one
// is being walked: those names resolve unfiltered, as in resolve/document.go.
func (c *refCollector) push(ref Reference) {
	ref.Condition = c.condition
	c.refs = append(c.refs, ref)
}

// conditionExpr walks the names of a filter condition.
func (c *refCollector) conditionExpr(scope *symbols.Scope, e ast.Node) {
	prev := c.condition
	c.condition = true
	defer func() { c.condition = prev }()
	c.expr(scope, e)
}

func (c *refCollector) add(scope *symbols.Scope, qn *ast.QualifiedName) {
	if qn != nil {
		c.push(Reference{Scope: scope, QN: qn})
	}
}

// addReference records the target of a reference subsetting owned by decl.
func (c *refCollector) addReference(scope *symbols.Scope, decl ast.Node, qn *ast.QualifiedName) {
	if qn != nil {
		c.push(Reference{Scope: scope, QN: qn, Referrer: decl})
	}
}

// addRedefinition records a redefinition's target, which names a feature of the
// owning scope's generals rather than a member of the scope itself.
func (c *refCollector) addRedefinition(scope *symbols.Scope, qn *ast.QualifiedName) {
	if qn != nil {
		c.push(Reference{Scope: scope, QN: qn, Redefines: true})
	}
}

// addChainMember records the member segments of a feature chain, which name
// members of the operand rather than elements of scope.
func (c *refCollector) addChainMember(scope *symbols.Scope, decl ast.Node, chain *ast.FeatureChainExpr) {
	if chain != nil && chain.Member != nil {
		c.push(Reference{Scope: scope, QN: chain.Member, Referrer: decl, Chain: chain})
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
	switch {
	case c.namespaceDecl(scope, decl):
	case c.typeDecl(scope, decl):
	case c.behaviorDecl(scope, decl):
	default:
		// A bare expression member is the body's result, as in a calc body
		// whose result is its last expression.
		c.expr(scope, decl)
	}
}

// namespaceDecl collects the references of a namespace-level declaration, reporting whether
// decl was one.
func (c *refCollector) namespaceDecl(scope *symbols.Scope, decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Package:
		c.prefixes(scope, d.Prefixes)
		if child := c.childScope(scope, d); child != nil {
			c.walkMembers(child, d.Members)
		}
		return true
	case *ast.Namespace:
		c.prefixes(scope, d.Prefixes)
		if child := c.childScope(scope, d); child != nil {
			c.walkMembers(child, d.Members)
		}
		return true
	case *ast.Import:
		c.add(scope, d.Imported)
		c.conditionExpr(scope, d.FilterExpr)
		return true
	case *ast.Alias:
		c.add(scope, d.For)
		return true
	case *ast.RelationshipMember:
		c.target(scope, d.Source)
		c.target(scope, d.Target)
		if child := c.childScope(scope, d); child != nil {
			c.walkMembers(child, d.Members)
		}
		return true
	case *ast.Dependency:
		c.prefixes(scope, d.Prefixes)
		for _, cl := range d.Clients {
			c.add(scope, cl)
		}
		for _, sp := range d.Suppliers {
			c.add(scope, sp)
		}
		return true
	case *ast.Comment:
		for _, a := range d.About {
			c.add(scope, a)
		}
		return true
	case *ast.PrefixMetadata:
		c.metadataPrefix(scope, d)
		return true
	case *ast.FilterMember:
		c.conditionExpr(scope, d.Condition)
		return true
	default:
		return false
	}
}

// typeDecl collects the references of a definition, usage or constraint member, reporting whether
// decl was one.
func (c *refCollector) typeDecl(scope *symbols.Scope, decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		c.prefixes(scope, d.Prefixes)
		c.relationships(scope, d, d.Relationships)
		if child := c.childScope(scope, d); child != nil {
			c.walkMembers(child, d.Members)
		}
		return true
	case *ast.Usage:
		c.prefixes(scope, d.Prefixes)
		c.relationships(scope, d, d.Relationships)
		c.multiplicity(scope, d.Multiplicity)
		// An accept node keeps its trigger in the usage's value.
		if d.IsAccept {
			c.trigger(scope, d.Value)
		} else if inv := d.PerformedInvocation(); inv != nil {
			c.invocation(scope, inv, true)
		} else {
			c.expr(scope, d.Value)
		}
		child := c.childScope(scope, d)
		for _, end := range d.ConnectorEnds {
			if end == nil {
				continue
			}
			// An end that reference-subsets what it attaches to declares its
			// own name, so that name is a declaration, not a reference. A `:>>`
			// on it names an end of the connector's type and so resolves in the
			// connector's scope; everything else resolves in the enclosing one.
			_, declaresName := end.DeclaredName()
			endScope := scope
			if child != nil {
				endScope = child
			}
			redefines, others := ast.SplitRedefinitions(end.Relationships)
			c.relationships(endScope, end, redefines)
			c.relationships(scope, end, others)
			if !declaresName {
				c.target(scope, end.Target)
			}
			c.target(scope, end.Reference)
		}
		if d.FlowEnds != nil {
			c.expr(scope, d.FlowEnds.From)
			c.expr(scope, d.FlowEnds.To)
			// A declared payload (`of name : Type`) names a member of the flow
			// itself, not an element of the enclosing scope.
			payloadScope := scope
			if d.FlowEnds.PayloadDecl != nil && child != nil {
				payloadScope = child
			}
			c.expr(payloadScope, d.FlowEnds.Payload)
		}
		if child != nil {
			c.walkMembers(child, d.Members)
		}
		return true
	case *ast.SubjectMember:
		c.add(scope, d.TypeRef)
		c.multiplicity(scope, d.Multiplicity)
		c.relationships(scope, d, d.Relationships)
		c.expr(scope, d.BindingExpr)
		if child := c.childScope(scope, d); child != nil {
			c.walkMembers(child, d.Body)
		}
		return true
	case *ast.InitialNode:
		// The node's own name is a label, not a reference.
		c.add(scope, d.Successor)
		c.expr(scope, d.Guard)
		return true
	case *ast.ConstraintMember:
		c.expr(scope, d.Expression)
		c.walkMembers(scope, d.Body)
		return true
	case *ast.AssumeMember:
		c.prefixes(scope, d.Prefixes)
		c.expr(scope, d.Expression)
		c.add(scope, d.Reference)
		c.relationships(scope, d, d.Relationships)
		c.multiplicity(scope, d.Multiplicity)
		c.expr(scope, d.Value)
		c.walkMembers(symbols.ConstraintBodyScope(scope, d), d.Body)
		return true
	case *ast.RequireMember:
		c.prefixes(scope, d.Prefixes)
		c.expr(scope, d.Expression)
		c.add(scope, d.Reference)
		c.relationships(scope, d, d.Relationships)
		c.multiplicity(scope, d.Multiplicity)
		c.expr(scope, d.Value)
		c.walkMembers(symbols.ConstraintBodyScope(scope, d), d.Body)
		return true
	default:
		return false
	}
}

// behaviorDecl collects the references of a behavioral member — a state, transition or action node, reporting whether
// decl was one.
func (c *refCollector) behaviorDecl(scope *symbols.Scope, decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.EntryMember:
		c.walkMembers(scope, d.Actions)
		return true
	case *ast.DoMember:
		c.walkMembers(scope, d.Actions)
		return true
	case *ast.ExitMember:
		c.walkMembers(scope, d.Actions)
		return true
	case *ast.DeferMember:
		for _, trigger := range d.Triggers {
			c.trigger(scope, trigger)
		}
		return true
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
		return true
	case *ast.StateRegion:
		states := scope
		if child := c.childScope(scope, d); child != nil {
			states = child
		}
		c.walkMembers(states, d.States)
		return true
	case *ast.TransitionMember:
		c.add(scope, d.Source)
		c.add(scope, d.Target)
		c.trigger(scope, d.Trigger)
		c.expr(scope, d.Guard)
		c.walkMembers(scope, d.Effect)
		return true
	case *ast.SendStatement:
		c.expr(scope, d.Message)
		c.expr(scope, d.Target)
		return true
	case *ast.TerminateStatement:
		c.expr(scope, d.Target)
		return true
	case *ast.AssignmentActionNode:
		c.expr(scope, d.Target)
		c.expr(scope, d.Value)
		return true
	case *ast.ActionExecutionNode:
		c.add(scope, d.ActionRef)
		c.expr(scope, d.Expression)
		return true
	case *ast.PerformActionNode:
		c.expr(scope, d.ActionRef)
		return true
	case *ast.WhileLoopActionNode:
		// A loop owns the scope its body declares into (see symbols.buildDecl).
		body := scope
		if child := c.childScope(scope, d); child != nil {
			body = child
		}
		c.expr(scope, d.Collection)
		c.expr(body, d.Condition)
		c.expr(body, d.Until)
		c.walkMembers(body, d.Body)
		return true
	case *ast.IfActionNode:
		c.expr(scope, d.Condition)
		for _, branch := range d.Branches() {
			c.resolveDecl(scope, branch)
		}
		return true
	case *ast.IfBranchNode:
		// A branch owns the scope its body declares into (see symbols.buildDecl).
		body := scope
		if child := c.childScope(scope, d); child != nil {
			body = child
		}
		c.walkMembers(body, d.Body)
		return true
	default:
		return false
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
// subsetting records decl as the referrer, so it resolves past decl's own
// binding of the name it references.
func (c *refCollector) relationships(scope *symbols.Scope, decl ast.Node, rels []*ast.Relationship) {
	for _, rel := range rels {
		if rel == nil {
			continue
		}
		if rel.Kind == ast.RelReferences {
			c.referenceTarget(scope, decl, rel.Target)
			continue
		}
		// A target parsed as an expression wraps the name it denotes.
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		if rel.Kind == ast.RelRedefines {
			if qn, ok := target.(*ast.QualifiedName); ok {
				c.addRedefinition(scope, qn)
				continue
			}
		}
		c.target(scope, target)
	}
}

// referenceTarget collects a reference subsetting's target, tagging the leading
// qualified name with the declaration that refers to it.
func (c *refCollector) referenceTarget(scope *symbols.Scope, decl ast.Node, target ast.Node) {
	if fr, ok := target.(*ast.FeatureReference); ok {
		target = fr.Name
	}
	if qn, ok := target.(*ast.QualifiedName); ok {
		c.addReference(scope, decl, qn)
		return
	}
	if chain, ok := target.(*ast.FeatureChainExpr); ok {
		c.referenceTarget(scope, decl, chain.Operand)
		c.addChainMember(scope, decl, chain)
		return
	}
	c.expr(scope, target)
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
			c.metadataPrefix(scope, p)
		}
	}
}

// metadataPrefix collects an annotation's metaclass name and the references its
// body carries, in the body's own scope — the same scope the resolver resolves
// them in (see resolveMetadataPrefix).
func (c *refCollector) metadataPrefix(scope *symbols.Scope, p *ast.PrefixMetadata) {
	c.add(scope, p.Type)
	if len(p.Body) == 0 {
		return
	}
	if body := c.childScope(scope, p); body != nil {
		c.walkMembers(body, p.Body)
	}
}

// invocation collects a call's receiver, name and arguments; performed marks the
// call an action usage runs as its value.
func (c *refCollector) invocation(scope *symbols.Scope, v *ast.InvocationExpr, performed bool) {
	c.expr(scope, v.Operand)
	if v.Type != nil {
		c.push(Reference{Scope: scope, QN: v.Type, Invocation: v, Performed: performed})
	}
	for _, a := range v.Args {
		c.expr(scope, a)
	}
	for _, na := range v.NamedArgs {
		c.add(scope, na.Name)
		c.expr(scope, na.Value)
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
		c.addChainMember(scope, nil, v)
	case *ast.IndexExpr:
		c.expr(scope, v.Operand)
		c.expr(scope, v.Index)
	case *ast.InvocationExpr:
		c.invocation(scope, v, false)
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
			c.relationships(scope, v, p.Relationships)
			c.expr(scope, p.Value)
		}
		// The same scope the resolver uses, so a reference to a parameter or a
		// body declaration and its declaration denote one symbol.
		inner := symbols.BodyExprScope(scope, v)
		c.walkMembers(inner, v.Members)
		c.expr(inner, v.Result)
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
