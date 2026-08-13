package symbols

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

// buildBodyScopes links the scope each body expression declares its parameters
// into (`c->forAll { in i : Positive; f(i) }`) into the document scope tree.
// Body expressions sit inside expressions, which the declaration builder does
// not walk, so this second pass mirrors resolve/document.go's traversal: a form
// handled there but not here leaves its parameters out of the tree.
func buildBodyScopes(scope *Scope, members []ast.Node) {
	for _, m := range members {
		decl, _ := unwrapMember(m)
		if decl == nil {
			continue
		}
		bodyScopesInDecl(scope, decl)
	}
}

// BodyExprScope returns the scope holding body's parameters. A body without
// parameters declares nothing, so its result resolves in parent.
func BodyExprScope(parent *Scope, body *ast.BodyExpr) *Scope {
	if parent == nil {
		return nil
	}
	for _, ch := range parent.Children() {
		if ch.Node() == body {
			return ch
		}
	}
	return parent
}

// newBodyExprScope creates and links the scope body's parameters declare into.
func newBodyExprScope(parent *Scope, body *ast.BodyExpr) *Scope {
	if len(body.Params) == 0 {
		return parent
	}
	scope := NewScope(parent, body)
	scope.markBodyLocal()
	for i := range body.Params {
		p := &body.Params[i]
		if p.Name == "" {
			continue
		}
		// The parameter's own span, not the whole body's: the editor renames
		// through NameSpan and jumps to DeclSpan.
		scope.Define(p.Name, &Symbol{
			Name:       p.Name,
			Kind:       SymbolAttributeUsage,
			Decl:       body,
			DeclSpan:   p.Span,
			NameSpan:   p.Span,
			OwnerScope: scope,
		})
	}
	parent.AddChild(scope)
	return scope
}

// bodyScopeChild returns the child scope declared by decl, or nil.
func bodyScopeChild(scope *Scope, decl ast.Node) *Scope {
	for _, c := range scope.Children() {
		if c.Node() == decl {
			return c
		}
	}
	return nil
}

// bodyScopesInDecl walks the expressions a declaration carries, descending into
// the child scopes its body resolves against.
func bodyScopesInDecl(scope *Scope, decl ast.Node) {
	switch d := decl.(type) {
	case *ast.Package:
		if child := bodyScopeChild(scope, d); child != nil {
			buildBodyScopes(child, d.Members)
		}
	case *ast.Namespace:
		if child := bodyScopeChild(scope, d); child != nil {
			buildBodyScopes(child, d.Members)
		}
	case *ast.FilterMember:
		bodyScopesInExpr(scope, d.Condition)
	case *ast.Definition:
		bodyScopesInRelationships(scope, d.Relationships)
		if child := bodyScopeChild(scope, d); child != nil {
			buildBodyScopes(child, d.Members)
		}
	case *ast.Usage:
		bodyScopesInRelationships(scope, d.Relationships)
		bodyScopesInMultiplicity(scope, d.Multiplicity)
		bodyScopesInExpr(scope, d.Value)
		for _, end := range d.ConnectorEnds {
			if end == nil {
				continue
			}
			bodyScopesInExpr(scope, end.Target)
			bodyScopesInExpr(scope, end.Reference)
		}
		if d.FlowEnds != nil {
			bodyScopesInExpr(scope, d.FlowEnds.From)
			bodyScopesInExpr(scope, d.FlowEnds.To)
			bodyScopesInExpr(scope, d.FlowEnds.Payload)
		}
		if child := bodyScopeChild(scope, d); child != nil {
			buildBodyScopes(child, d.Members)
		}
	case *ast.SubjectMember:
		bodyScopesInMultiplicity(scope, d.Multiplicity)
		bodyScopesInExpr(scope, d.BindingExpr)
		if child := bodyScopeChild(scope, d); child != nil {
			buildBodyScopes(child, d.Body)
		}
	case *ast.InitialNode:
		bodyScopesInExpr(scope, d.Guard)
	case *ast.ResultMember:
		bodyScopesInExpr(scope, d.Expression)
	case *ast.ConstraintMember:
		bodyScopesInExpr(scope, d.Expression)
		buildBodyScopes(scope, d.Body)
	case *ast.AssumeMember:
		bodyScopesInExpr(scope, d.Expression)
		buildBodyScopes(scope, d.Body)
	case *ast.RequireMember:
		bodyScopesInExpr(scope, d.Expression)
		buildBodyScopes(scope, d.Body)
	case *ast.EntryMember:
		buildBodyScopes(scope, d.Actions)
	case *ast.DoMember:
		buildBodyScopes(scope, d.Actions)
	case *ast.ExitMember:
		buildBodyScopes(scope, d.Actions)
	case *ast.DeferMember:
		for _, trigger := range d.Triggers {
			bodyScopesInTrigger(scope, trigger)
		}
	case *ast.StateNode:
		body := scope
		if child := bodyScopeChild(scope, d); child != nil {
			body = child
		}
		buildBodyScopes(body, d.Entry)
		buildBodyScopes(body, d.Do)
		buildBodyScopes(body, d.Exit)
		buildBodyScopes(body, d.Substates)
		for _, region := range d.Regions {
			bodyScopesInDecl(body, region)
		}
	case *ast.StateRegion:
		states := scope
		if child := bodyScopeChild(scope, d); child != nil {
			states = child
		}
		buildBodyScopes(states, d.States)
	case *ast.TransitionMember:
		bodyScopesInTrigger(scope, d.Trigger)
		// The parameters a call trigger declares are visible to the transition's
		// own guard and effect, and nowhere else.
		body := newCallTriggerScope(scope, d)
		bodyScopesInExpr(body, d.Guard)
		buildBodyScopes(body, d.Effect)
	case *ast.SendStatement:
		bodyScopesInExpr(scope, d.Message)
		bodyScopesInExpr(scope, d.Target)
	case *ast.TerminateStatement:
		bodyScopesInExpr(scope, d.Target)
	case *ast.AssignmentActionNode:
		bodyScopesInExpr(scope, d.Target)
		bodyScopesInExpr(scope, d.Value)
	case *ast.ActionExecutionNode:
		bodyScopesInExpr(scope, d.Expression)
	case *ast.PerformActionNode:
		bodyScopesInExpr(scope, d.ActionRef)
	case *ast.WhileLoopActionNode:
		body := scope
		if child := bodyScopeChild(scope, d); child != nil {
			body = child
		}
		bodyScopesInExpr(scope, d.Collection)
		bodyScopesInExpr(body, d.Condition)
		buildBodyScopes(body, d.Body)
	case *ast.IfActionNode:
		bodyScopesInExpr(scope, d.Condition)
		for _, branch := range d.Branches() {
			bodyScopesInDecl(scope, branch)
		}
	case *ast.IfBranchNode:
		// A branch owns the scope its body declares into (see builder.buildDecl).
		body := scope
		if child := bodyScopeChild(scope, d); child != nil {
			body = child
		}
		buildBodyScopes(body, d.Body)
	}
}

// CallTriggerScope returns the scope holding the parameters trans's call
// trigger declares, or parent when it declares none.
func CallTriggerScope(parent *Scope, trans *ast.TransitionMember) *Scope {
	if parent == nil {
		return nil
	}
	for _, ch := range parent.Children() {
		if ch.Node() == trans {
			return ch
		}
	}
	return parent
}

// newCallTriggerScope creates and links the scope a call trigger's parameters
// declare into, so `accept setSpeed(value) if value > 0` resolves `value`.
func newCallTriggerScope(parent *Scope, trans *ast.TransitionMember) *Scope {
	callEvent, ok := trans.Trigger.(*ast.CallEvent)
	if !ok || len(callEvent.Parameters) == 0 {
		return parent
	}
	if existing := bodyScopeChild(parent, trans); existing != nil {
		return existing
	}
	scope := NewScope(parent, trans)
	scope.markBodyLocal()
	for _, param := range callEvent.Parameters {
		scope.Define(param.Text, &Symbol{
			Name:       param.Text,
			Kind:       SymbolAttributeUsage,
			Decl:       callEvent,
			DeclSpan:   param.Span,
			NameSpan:   param.Span,
			OwnerScope: scope,
		})
	}
	parent.AddChild(scope)
	return scope
}

// bodyScopesInTrigger mirrors resolve's trigger handling: only the payload
// expressions of time and change events can hold a body expression.
func bodyScopesInTrigger(scope *Scope, trigger ast.Node) {
	switch t := trigger.(type) {
	case nil:
		return
	case *ast.TimeEvent:
		bodyScopesInExpr(scope, t.Duration)
	case *ast.ChangeEvent:
		bodyScopesInExpr(scope, t.Condition)
	case *ast.QualifiedName, *ast.FeatureReference, *ast.AcceptEvent, *ast.CallEvent:
		// Event names, not expressions.
	default:
		bodyScopesInExpr(scope, trigger)
	}
}

func bodyScopesInRelationships(scope *Scope, rels []*ast.Relationship) {
	for _, rel := range rels {
		if rel != nil {
			bodyScopesInExpr(scope, rel.Target)
		}
	}
}

func bodyScopesInMultiplicity(scope *Scope, m *ast.Multiplicity) {
	if m == nil {
		return
	}
	bodyScopesInExpr(scope, m.Lower)
	bodyScopesInExpr(scope, m.Upper)
}

// bodyScopesInExpr walks an expression subtree, creating a scope for every body
// expression that declares parameters and nesting inner bodies within it.
func bodyScopesInExpr(scope *Scope, e ast.Node) {
	switch v := e.(type) {
	case nil:
		return
	case *ast.OperatorExpr:
		for _, op := range v.Operands {
			bodyScopesInExpr(scope, op)
		}
	case *ast.FeatureChainExpr:
		bodyScopesInExpr(scope, v.Operand)
	case *ast.IndexExpr:
		bodyScopesInExpr(scope, v.Operand)
		bodyScopesInExpr(scope, v.Index)
	case *ast.InvocationExpr:
		bodyScopesInExpr(scope, v.Operand)
		for _, a := range v.Args {
			bodyScopesInExpr(scope, a)
		}
		for _, na := range v.NamedArgs {
			bodyScopesInExpr(scope, na.Value)
		}
	case *ast.CollectExpr:
		bodyScopesInExpr(scope, v.Operand)
		bodyScopesInExpr(scope, v.Body)
	case *ast.SelectExpr:
		bodyScopesInExpr(scope, v.Operand)
		bodyScopesInExpr(scope, v.Body)
	case *ast.ConstructorExpr:
		for _, a := range v.Args {
			bodyScopesInExpr(scope, a)
		}
	case *ast.BodyExpr:
		for i := range v.Params {
			bodyScopesInExpr(scope, v.Params[i].Value)
		}
		bodyScopesInExpr(newBodyExprScope(scope, v), v.Result)
	case *ast.SequenceExpr:
		for _, el := range v.Elements {
			bodyScopesInExpr(scope, el)
		}
	}
}
