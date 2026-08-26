package symbols

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
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
	if len(body.Params) == 0 && len(body.Members) == 0 {
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
	buildMembers(scope, body.Members)
	parent.AddChild(scope)
	return scope
}

// bodyScopeChild returns the child scope declared by decl, or nil.
func bodyScopeChild(scope *Scope, decl ast.Node) *Scope {
	return scope.ChildFor(decl)
}

// nodeBodyScope returns the scope the body of an action node resolves against:
// its own where the builder gave it one, and the enclosing scope otherwise.
func nodeBodyScope(scope *Scope, node ast.Node) *Scope {
	if child := bodyScopeChild(scope, node); child != nil {
		return child
	}
	return scope
}

// bodyScopesInDecl walks the expressions a declaration carries, descending into
// the child scopes its body resolves against.
func bodyScopesInDecl(scope *Scope, decl ast.Node) {
	if prefixes := prefixMetadataOf(decl); len(prefixes) > 0 {
		for _, prefix := range prefixes {
			if prefix == nil || len(prefix.Body) == 0 {
				continue
			}
			if child := bodyScopeChild(scope, prefix); child != nil {
				buildBodyScopes(child, prefix.Body)
			}
		}
	}
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
	case *ast.PrefixMetadata:
		if child := bodyScopeChild(scope, d); child != nil {
			buildBodyScopes(child, d.Body)
		}
	case *ast.Definition:
		bodyScopesInRelationships(scope, d.Relationships)
		if child := bodyScopeChild(scope, d); child != nil {
			buildBodyScopes(child, d.Members)
		}
	case *ast.Usage:
		bodyScopesInRelationships(scope, d.Relationships)
		bodyScopesInMultiplicity(scope, d.Multiplicity)
		// An accept node keeps its trigger in the usage's value.
		if d.IsAccept {
			bodyScopesInTrigger(scope, d.Value)
		} else {
			bodyScopesInExpr(scope, d.Value)
		}
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
		buildBodyScopes(nodeBodyScope(scope, d), d.Members)
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode:
		buildBodyScopes(nodeBodyScope(scope, d), ast.NodeBodyMembers(d))
	case *ast.ConstraintMember:
		bodyScopesInExpr(scope, d.Expression)
		buildBodyScopes(scope, d.Body)
	case *ast.AssumeMember:
		bodyScopesInExpr(scope, d.Expression)
		buildBodyScopes(ConstraintBodyScope(scope, d), d.Body)
	case *ast.RequireMember:
		bodyScopesInExpr(scope, d.Expression)
		buildBodyScopes(ConstraintBodyScope(scope, d), d.Body)
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
		// The parameters a trigger declares are visible to the transition's own
		// guard and effect, and nowhere else.
		body := newTriggerScope(scope, d)
		bodyScopesInExpr(body, d.Guard)
		buildBodyScopes(body, d.Effect)
		buildBodyScopes(body, d.Members)
	case *ast.SendStatement:
		bodyScopesInExpr(scope, d.Message)
		bodyScopesInExpr(scope, d.Target)
		bodyScopesInExpr(scope, d.Receiver)
		buildBodyScopes(nodeBodyScope(scope, d), d.Members)
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
		bodyScopesInExpr(body, d.Until)
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
	default:
		// Bare expression members can contain nested body expressions.
		bodyScopesInExpr(scope, decl)
	}
}

// TriggerScope returns the innermost scope trans's guard and effect resolve
// against: the one holding the parameters its trigger declares when it declares
// any, the transition's own scope when it has one, and parent otherwise.
func TriggerScope(parent *Scope, trans *ast.TransitionMember) *Scope {
	if parent == nil {
		return nil
	}
	scope := parent
	if child := bodyScopeChild(parent, trans); child != nil {
		scope = child
		if params := triggerParameterScope(child, trans); params != nil {
			scope = params
		}
	}
	return scope
}

// newTriggerScope creates and links the scope a trigger's parameters declare
// into, so `accept setSpeed(value) if value > 0` resolves `value` and
// `accept w : Warning do assign level := w` resolves `w`.
func newTriggerScope(parent *Scope, trans *ast.TransitionMember) *Scope {
	// A named transition already owns a scope holding its effect members
	// (builder.buildDecl); an unnamed one owns none until its trigger needs it.
	scope := bodyScopeChild(parent, trans)
	define := triggerParameterDefiner(trans.Trigger)
	if define == nil {
		if scope != nil {
			return scope
		}
		return parent
	}
	if scope == nil {
		scope = NewScope(parent, trans)
		scope.markBodyLocal()
		parent.AddChild(scope)
	}
	// The parameters go in a scope of their own, nested in the transition's: they
	// are visible to the guard and effect that resolve through it, but are not
	// features of the transition the way its effect behaviors are.
	params := triggerParameterScope(scope, trans)
	if params == nil {
		params = NewScope(scope, trans.Trigger)
		params.markBodyLocal()
		scope.AddChild(params)
	}
	define(params)
	return params
}

// triggerParameterScope returns the scope holding the parameters trans's
// trigger declares, or nil when it has none.
func triggerParameterScope(transScope *Scope, trans *ast.TransitionMember) *Scope {
	if trans.Trigger == nil {
		return nil
	}
	return bodyScopeChild(transScope, trans.Trigger)
}

// triggerParameterDefiner returns a function defining the parameters trigger
// declares, or nil when it declares none.
func triggerParameterDefiner(trigger ast.Node) func(*Scope) {
	switch t := trigger.(type) {
	case *ast.CallEvent:
		if len(t.Parameters) == 0 {
			return nil
		}
		return func(scope *Scope) {
			for _, param := range t.Parameters {
				scope.Define(param.Text, &Symbol{
					Name:       param.Text,
					Kind:       SymbolAttributeUsage,
					Decl:       t,
					DeclSpan:   param.Span,
					NameSpan:   param.Span,
					OwnerScope: scope,
				})
			}
		}
	case *ast.AcceptEvent:
		return payloadParameterDefiner(t.Payload)
	case *ast.Usage:
		// The parser gives an accept's payload parameter as the usage it was
		// written as (`accept w : Warning`); lowering classifies it. A usage
		// carrying a value carries a time or change event instead, which declares
		// no parameter.
		if t.Value != nil {
			return nil
		}
		return payloadParameterDefiner(t)
	default:
		return nil
	}
}

// payloadParameterDefiner returns a function defining the payload parameter an
// accept names, or nil when it names none. The parameter holds the received
// occurrence and is visible to the transition's guard and effect only.
func payloadParameterDefiner(payload *ast.Usage) func(*Scope) {
	if payload == nil {
		return nil
	}
	name, nameSpan := ast.EffectiveName(payload)
	if name == "" {
		return nil
	}
	return func(scope *Scope) {
		scope.Define(name, &Symbol{
			Name:       name,
			Kind:       SymbolItemUsage,
			Decl:       payload,
			DeclSpan:   payload.Span(),
			NameSpan:   nameSpan,
			OwnerScope: scope,
		})
	}
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
		body := newBodyExprScope(scope, v)
		buildBodyScopes(body, v.Members)
		bodyScopesInExpr(body, v.Result)
	case *ast.SequenceExpr:
		for _, el := range v.Elements {
			bodyScopesInExpr(scope, el)
		}
	}
}
