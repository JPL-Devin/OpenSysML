package resolve

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ResolveDocument walks the document's references and resolves each, recording
// diagnostics on the Resolver. name identifies the document in the index.
func (r *Resolver) ResolveDocument(name string, root *ast.RootNamespace) {
	rootScope := r.idx.DocumentRoot(name)
	if rootScope == nil {
		return
	}
	saved := r.document
	r.document = name
	defer func() { r.document = saved }()
	r.walkMembers(rootScope, membersOf(root))
	r.checkDistinguishability(rootScope)
}

// membersOf returns the top-level members of a RootNamespace.
func membersOf(root *ast.RootNamespace) []ast.Node {
	if root == nil {
		return nil
	}
	return root.Members
}

// walkMembers resolves references in each member, descending into child scopes.
func (r *Resolver) walkMembers(scope *symbols.Scope, members []ast.Node) {
	for _, m := range members {
		decl, _ := unwrapForResolve(m)
		r.resolveDecl(scope, decl)
	}
}

// walkBody walks the members of a body decl owns in the scope built for it, or in
// scope itself when the builder made none (an empty body).
func (r *Resolver) walkBody(scope *symbols.Scope, decl ast.Node, members []ast.Node) {
	if child := r.childScope(scope, decl); child != nil {
		scope = child
	}
	r.walkMembers(scope, members)
}

// unwrapForResolve mirrors the builder's unwrapMember: it strips *ast.Membership
// wrappers so we resolve against the inner declaration.
func unwrapForResolve(m ast.Node) (ast.Node, ast.Visibility) {
	switch v := m.(type) {
	case *ast.Membership:
		return v.Member, v.Visibility
	case *ast.Import:
		return v, v.Visibility
	case *ast.Alias:
		return v, v.Visibility
	default:
		return m, ast.VisibilityDefault
	}
}

// resolveDecl resolves references contributed by a single declaration and
// recurses into declarations that own a child scope.
func (r *Resolver) resolveDecl(scope *symbols.Scope, decl ast.Node) {
	switch {
	case r.resolveNamespaceDecl(scope, decl):
	case r.resolveTypeDecl(scope, decl):
	case r.resolveBehaviorDecl(scope, decl):
	default:
		// A bare expression member is the body's result, as in a calc body
		// whose result is its last expression.
		r.resolveExpr(scope, decl)
	}
}

// resolveNamespaceDecl resolves a namespace-level declaration, reporting
// whether decl was one.
func (r *Resolver) resolveNamespaceDecl(scope *symbols.Scope, decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Package:
		r.resolvePrefixes(scope, d.Prefixes)
		if child := r.childScope(scope, d); child != nil {
			r.walkMembers(child, d.Members)
			r.checkDistinguishability(child)
		}
		return true
	case *ast.Namespace:
		r.resolvePrefixes(scope, d.Prefixes)
		if child := r.childScope(scope, d); child != nil {
			r.walkMembers(child, d.Members)
			r.checkDistinguishability(child)
		}
		return true
	case *ast.Import:
		r.resolveImportTarget(scope, d)
		if d.FilterExpr != nil {
			r.InCondition(func() { r.resolveExpr(scope, d.FilterExpr) })
		}
		return true
	case *ast.Alias:
		r.ResolveQualified(scope, d.For)
		return true
	case *ast.RelationshipMember:
		// Both ends of a keyword-first relationship name elements, in the scope
		// the relationship is a member of.
		r.resolveRelationshipEnd(scope, d.Source)
		r.resolveRelationshipEnd(scope, d.Target)
		if child := r.childScope(scope, d); child != nil {
			r.walkMembers(child, d.Members)
		}
		return true
	case *ast.Dependency:
		r.resolvePrefixes(scope, d.Prefixes)
		for _, c := range d.Clients {
			r.ResolveQualified(scope, c)
		}
		for _, s := range d.Suppliers {
			r.ResolveQualified(scope, s)
		}
		return true
	case *ast.Comment:
		for _, a := range d.About {
			r.ResolveQualified(scope, a)
		}
		return true
	case *ast.PrefixMetadata:
		// A metadata usage written as a member of its own names its type the same
		// way a prefix does.
		r.resolveMetadataPrefix(scope, d)
		return true
	case *ast.FilterMember:
		r.InCondition(func() { r.resolveExpr(scope, d.Condition) })
		return true
	default:
		return false
	}
}

// resolveTypeDecl resolves a definition, usage or constraint member, reporting
// whether decl was one.
func (r *Resolver) resolveTypeDecl(scope *symbols.Scope, decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		r.resolvePrefixes(scope, d.Prefixes)
		child := r.childScope(scope, d)
		r.resolveHeaderRelationships(scope, child, d, d.Relationships)
		if child != nil {
			r.walkMembers(child, d.Members)
			r.checkDistinguishability(child)
		}
		return true
	case *ast.Usage:
		r.resolvePrefixes(scope, d.Prefixes)
		child := r.childScope(scope, d)
		r.resolveHeaderRelationships(scope, child, d, d.Relationships)
		if d.Multiplicity != nil {
			r.resolveExpr(scope, d.Multiplicity.Lower)
			r.resolveExpr(scope, d.Multiplicity.Upper)
		}
		// An accept node keeps its trigger in the usage's value, and a trigger's
		// names are not all references (see resolveTrigger).
		if d.IsAccept {
			r.resolveTrigger(scope, d.Value)
		} else if d.Kind == ast.UsageBinding && isImplicitCalcResult(scope, d.Value) {
			// A calc binding may name its implicit result feature as the value end.
		} else {
			r.resolveExpr(scope, d.Value)
		}
		for _, end := range d.ConnectorEnds {
			// ConnectorEnd has both Target and Reference fields
			// Target: primary connector target (part being connected)
			// Reference: optional "references X" clause, or state transition target
			if end == nil {
				continue
			}
			// An end that reference-subsets the feature it attaches to declares
			// its own name (`connect bead references t.bead`), so that name is a
			// declaration, not a reference to resolve.
			_, declaresName := end.DeclaredName()
			// A redefinition names an end of the connector's own type, so it
			// resolves in the connector's scope; everything else an end names —
			// the feature it attaches to, its type — is a feature of the
			// connector's owner and resolves in the enclosing scope.
			endScope := scope
			redefinitionScope := scope
			if inner := r.childScope(scope, d); inner != nil {
				redefinitionScope = inner
			}
			redefines, others := ast.SplitRedefinitions(end.Relationships)
			r.resolveRelationships(redefinitionScope, end, redefines)
			r.resolveRelationships(endScope, end, others)
			endpointKind := d.Kind == ast.UsageSuccession || d.Kind == ast.UsageTransition
			resolveAsEndpoint := endpointKind && inStateMachine(endScope) && !declaresName
			resolveEnd := func(target ast.Node) {
				// A machine succession/transition end names a vertex like a transition endpoint.
				if qn, ok := target.(*ast.QualifiedName); ok {
					if resolveAsEndpoint {
						r.ResolveEndpoint(endScope, qn)
					} else {
						r.ResolveQualified(endScope, qn)
					}
				} else {
					r.resolveExpr(endScope, target)
				}
			}
			if end.Target != nil && !declaresName {
				resolveEnd(end.Target)
			}
			if end.Reference != nil {
				resolveEnd(end.Reference)
			}
		}
		if d.FlowEnds != nil {
			r.resolveExpr(scope, d.FlowEnds.From)
			r.resolveExpr(scope, d.FlowEnds.To)
			// A declared payload (`of name : Type`) names a member of the flow
			// itself, not an element of the enclosing scope.
			payloadScope := scope
			if d.FlowEnds.PayloadDecl != nil && child != nil {
				payloadScope = child
			}
			r.resolveExpr(payloadScope, d.FlowEnds.Payload)
		}
		if child != nil {
			r.walkMembers(child, d.Members)
			r.checkDistinguishability(child)
		}
		return true
	case *ast.SubjectMember:
		// Resolve subject type reference
		if d.TypeRef != nil {
			r.ResolveQualified(scope, d.TypeRef)
		}
		if d.Multiplicity != nil {
			r.resolveExpr(scope, d.Multiplicity.Lower)
			r.resolveExpr(scope, d.Multiplicity.Upper)
		}
		r.resolveRelationships(scope, d, d.Relationships)
		r.resolveExpr(scope, d.BindingExpr)
		if child := r.childScope(scope, d); child != nil {
			r.walkMembers(child, d.Body)
		}
		return true
	case *ast.InitialNode:
		// Only resolve successor, not the name (which is just a label)
		if d.Successor != nil {
			r.ResolveQualified(scope, d.Successor)
		}
		r.resolveExpr(scope, d.Guard)
		r.walkBody(scope, d, d.Members)
		return true
	case *ast.SuccessionEdge:
		r.resolveSuccessionEdge(scope, d)
		r.walkBody(scope, d, d.Members)
		return true
	case *ast.ControlFlowEdge:
		r.resolveControlFlowEdge(scope, d)
		return true
	case *ast.FinalNode:
		// Final nodes have no references
		return true
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode:
		// Control flow nodes have no references to resolve (names are just labels)
		return true
	case *ast.ConstraintMember:
		r.resolveExpr(scope, d.Expression)
		r.walkMembers(scope, d.Body)
		return true
	case *ast.AssumeMember:
		r.resolvePrefixes(scope, d.Prefixes)
		r.resolveExpr(scope, d.Expression)
		r.resolveRelationships(scope, d, d.Relationships)
		r.resolveMultiplicity(scope, d.Multiplicity)
		r.resolveExpr(scope, d.Value)
		r.walkConstraintBody(scope, d, r.resolveConstraintReference(scope, d.Reference), d.Body)
		return true
	case *ast.RequireMember:
		r.resolvePrefixes(scope, d.Prefixes)
		r.resolveExpr(scope, d.Expression)
		r.resolveRelationships(scope, d, d.Relationships)
		r.resolveMultiplicity(scope, d.Multiplicity)
		r.resolveExpr(scope, d.Value)
		r.walkConstraintBody(scope, d, r.resolveConstraintReference(scope, d.Reference), d.Body)
		return true
	default:
		return false
	}
}

// resolveBehaviorDecl resolves a behavioral member — a state, transition or
// action node — reporting whether decl was one.
func (r *Resolver) resolveBehaviorDecl(scope *symbols.Scope, decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.EntryMember:
		r.walkMembers(scope, d.Actions)
		return true
	case *ast.DoMember:
		r.walkMembers(scope, d.Actions)
		return true
	case *ast.ExitMember:
		r.walkMembers(scope, d.Actions)
		return true
	case *ast.DeferMember:
		// A deferred event is a trigger like a transition's, so it resolves the
		// same way: bare signal names are left to lowering.
		for _, trigger := range d.Triggers {
			r.resolveTrigger(scope, trigger)
		}
		return true
	case *ast.StateNode:
		// The state's own name is a declaration, not a reference. Its body
		// resolves in the scope the state owns, which holds its substates and
		// regions; features it reads still resolve outward from there.
		body := scope
		if child := r.childScope(scope, d); child != nil {
			body = child
		}
		r.walkMembers(body, d.Entry)
		r.walkMembers(body, d.Do)
		r.walkMembers(body, d.Exit)
		r.walkMembers(body, d.Substates)
		for _, region := range d.Regions {
			r.resolveDecl(body, region)
		}
		return true
	case *ast.StateRegion:
		states := scope
		if child := r.childScope(scope, d); child != nil {
			states = child
		}
		r.walkMembers(states, d.States)
		return true
	case *ast.TransitionMember:
		// Source and target name vertices of the enclosing machine: resolved here,
		// so a misspelled endpoint reports with the other name diagnostics rather
		// than at lowering, which consumes what this resolved.
		// The guard and effect resolve against the parameters the transition's
		// call trigger declares, which live in a scope of their own.
		r.ResolveEndpoint(scope, d.Source)
		r.ResolveEndpoint(scope, d.Target)
		r.resolveTrigger(scope, d.Trigger)
		body := symbols.TriggerScope(scope, d)
		r.resolveExpr(body, d.Guard)
		r.walkMembers(body, d.Effect)
		return true
	case *ast.SendStatement:
		r.resolveExpr(scope, d.Message)
		r.resolveExpr(scope, d.Target)
		return true
	case *ast.TerminateStatement:
		r.resolveExpr(scope, d.Target)
		return true
	case *ast.AssignmentActionNode:
		r.resolveExpr(scope, d.Target)
		r.resolveExpr(scope, d.Value)
		return true
	case *ast.ActionExecutionNode:
		if d.ActionRef != nil {
			r.ResolveQualified(scope, d.ActionRef)
		}
		r.resolveExpr(scope, d.Expression)
		return true
	case *ast.PerformActionNode:
		r.resolveExpr(scope, d.ActionRef)
		return true
	case *ast.WhileLoopActionNode:
		// The loop owns its body's declarations, and its condition is checked
		// against them: `loop { action charging; } until charging.done`. The
		// collection a `for` loop iterates over is evaluated before the loop is
		// entered, so it resolves outside the body.
		body := scope
		if child := r.childScope(scope, d); child != nil {
			body = child
		}
		r.resolveExpr(scope, d.Collection)
		r.resolveExpr(body, d.Condition)
		r.resolveExpr(body, d.Until)
		r.walkMembers(body, d.Body)
		return true
	case *ast.IfActionNode:
		// The condition is evaluated before either branch is entered, so it sees
		// the enclosing scope only; each branch owns its body's declarations.
		r.resolveExpr(scope, d.Condition)
		for _, branch := range d.Branches() {
			r.resolveDecl(scope, branch)
		}
		return true
	case *ast.IfBranchNode:
		body := scope
		if child := r.childScope(scope, d); child != nil {
			body = child
		}
		r.walkMembers(body, d.Body)
		return true
	default:
		return false
	}
}

func isImplicitCalcResult(scope *symbols.Scope, node ast.Node) bool {
	owner := scope.Owner()
	if owner == nil {
		return false
	}
	switch decl := owner.Decl.(type) {
	case *ast.Definition:
		if decl.Kind != ast.DefCalc {
			return false
		}
	case *ast.Usage:
		if decl.Kind != ast.UsageCalc {
			return false
		}
	default:
		return false
	}
	var name string
	switch ref := node.(type) {
	case *ast.FeatureReference:
		if ref.Name == nil || len(ref.Name.Parts) != 1 {
			return false
		}
		name = ref.Name.Parts[0].Text
	case *ast.QualifiedName:
		if len(ref.Parts) != 1 {
			return false
		}
		name = ref.Parts[0].Text
	default:
		return false
	}
	return name == "result"
}

// resolveTrigger resolves the references a transition trigger carries.
//
// A bare name after `when` is classified by lowering as a signal, and signals
// are injected by the event source rather than declared in the model, so bare
// names are left unresolved here; resolving them would report every signal-
// triggered transition as an unresolved reference.
func (r *Resolver) resolveTrigger(scope *symbols.Scope, trigger ast.Node) {
	switch t := trigger.(type) {
	case nil:
		return
	case *ast.TimeEvent:
		r.resolveExpr(scope, t.Duration)
	case *ast.ChangeEvent:
		r.resolveExpr(scope, t.Condition)
	case *ast.AcceptEvent:
		// The payload of an accept names a type, and `:> f` an event feature,
		// as the pinned validator resolves them; a bare `when` name does not.
		if t.SignalType != nil {
			r.resolveQualified(scope, t.SignalType, nil)
		}
		if t.Subsets != nil {
			r.resolveQualified(scope, t.Subsets, nil)
		}
		if t.Payload != nil {
			r.resolveDecl(scope, t.Payload)
		}
	case *ast.Usage:
		// A named payload (`accept m : Warning`) declares a parameter, so its
		// typing resolves like any other declaration's.
		r.resolveDecl(scope, t)
	case *ast.QualifiedName, *ast.FeatureReference, *ast.CallEvent:
		// Signal and call triggers name events, not model elements.
	default:
		r.resolveExpr(scope, trigger)
	}
}

// ParameterizedByName reports whether sym is a case or requirement — including
// a concern or viewpoint, which are requirements — whose subject, actors and
// stakeholders redefine the inherited ones by name (SysML 7.18.4, 7.19.4). That
// is not modelled and not distinguishable from an ordinary feature here, so the
// conflict rule skips such a body entirely.
func ParameterizedByName(sym *symbols.Symbol) bool {
	switch decl := sym.Decl.(type) {
	case *ast.Usage:
		switch decl.Kind {
		case ast.UsageRequirement, ast.UsageSatisfy, ast.UsageConcern,
			ast.UsageFramedConcern, ast.UsageViewpoint,
			ast.UsageCase, ast.UsageAnalysisCase,
			ast.UsageVerificationCase, ast.UsageUseCase:
			return true
		}
	case *ast.Definition:
		switch decl.Kind {
		case ast.DefRequirement, ast.DefConcern, ast.DefViewpoint, ast.DefCase,
			ast.DefAnalysisCase, ast.DefVerificationCase, ast.DefUseCase:
			return true
		}
	}
	return false
}

// childScope finds the child scope whose node is decl.
func (r *Resolver) childScope(scope *symbols.Scope, decl ast.Node) *symbols.Scope {
	return scope.ChildFor(decl)
}

func (r *Resolver) resolvePrefixes(scope *symbols.Scope, prefixes []*ast.PrefixMetadata) {
	for _, p := range prefixes {
		r.resolveMetadataPrefix(scope, p)
	}
}

func (r *Resolver) resolveMetadataPrefix(scope *symbols.Scope, prefix *ast.PrefixMetadata) {
	if prefix == nil {
		return
	}
	owner, ok := r.ResolveQualified(scope, prefix.Type)
	if !ok || owner == nil || len(prefix.Body) == 0 {
		return
	}
	if target, aliasOK := r.ResolveAliasTarget(owner); aliasOK {
		owner = target
	}
	body := scope.ChildFor(prefix)
	if body == nil {
		return
	}
	// Body values resolve against the metadata definition, not the annotated element.
	if body.Owner() == nil {
		body.SetOwner(owner)
	}
	r.resolveMetadataBody(body, prefix.Body)
}

func (r *Resolver) resolveMetadataBody(scope *symbols.Scope, members []ast.Node) {
	for _, member := range members {
		decl, _ := unwrapForResolve(member)
		if decl == nil {
			continue
		}
		r.resolveDecl(scope, decl)
	}
}

// resolveMultiplicity resolves the bounds of a multiplicity, which may name
// features rather than state literals (`[n..m]`).
func (r *Resolver) resolveMultiplicity(scope *symbols.Scope, mult *ast.Multiplicity) {
	if mult == nil {
		return
	}
	r.resolveExpr(scope, mult.Lower)
	r.resolveExpr(scope, mult.Upper)
}

// resolveRelationships resolves each relationship target of decl as a qualified
// name. Redefinitions resolve in the inherited scope, and reference subsettings
// resolve outside decl's own name binding (see refFilter).
func (r *Resolver) resolveRelationships(scope *symbols.Scope, decl ast.Node, rels []*ast.Relationship) {
	for _, rel := range rels {
		if rel != nil && rel.Target != nil {
			// Unwrap FeatureReference if needed (relationship targets parsed as expressions)
			target := rel.Target
			if fr, ok := target.(*ast.FeatureReference); ok {
				target = fr.Name
			}

			// Special case: redefinitions should resolve in inherited scope
			// Self-subsetting must resolve in the declaration scope so cycle checks see `p4 :> p4`.
			if rel.Kind == ast.RelRedefines || (rel.Kind == ast.RelSubsets && !relationshipTargetsDecl(rel, decl)) {
				if qn, ok := target.(*ast.QualifiedName); ok {
					r.resolveRedefinition(scope, qn, decl)
					continue
				}
			}
			if rel.Kind == ast.RelReferences && isImplicitCalcResult(scope, target) {
				continue
			}

			// A reference subsetting resolves its leading segment past the
			// name decl borrows from it; memoizing that result makes the
			// chain walk below see the referenced feature, not decl.
			if rel.Kind == ast.RelReferences {
				hide := &refFilter{
					decl: decl,
				}
				// A connector end's participant is featured where the connector
				// is, so a feature of the connector itself is not one
				// (KerML 8.3.4.5).
				if u, ok := decl.(*ast.Usage); ok && u.IsEnd && declaresConnector(scope) {
					hide.featuredBy = scope
				}
				if _, ok := target.(*ast.FeatureChainExpr); ok {
					hide = hide.forPrefix()
				}
				if _, ok := target.(*ast.QualifiedName); ok {
					r.resolveTarget(scope, target, hide)
					continue
				}
				r.resolveTarget(scope, leadingName(target), hide)
			}

			// Standard resolution in current scope
			if qn, ok := target.(*ast.QualifiedName); ok {
				r.ResolveQualified(scope, qn)
			} else if fc, ok := target.(*ast.FeatureChainExpr); ok {
				r.resolveFeatureChain(scope, fc)
			}
		}
	}
}

// resolveRelationshipEnd resolves one end of a keyword-first relationship,
// which the notation writes as a name or a feature chain.
func (r *Resolver) resolveRelationshipEnd(scope *symbols.Scope, end ast.Node) {
	switch e := end.(type) {
	case *ast.QualifiedName:
		r.ResolveQualified(scope, e)
	case *ast.FeatureReference:
		r.ResolveQualified(scope, e.Name)
	case *ast.FeatureChainExpr:
		r.resolveFeatureChain(scope, e)
	}
}

func (r *Resolver) resolveHeaderRelationships(parent, header *symbols.Scope, decl ast.Node, rels []*ast.Relationship) {
	if header == nil || header == parent {
		r.resolveRelationships(parent, decl, rels)
		return
	}
	for _, rel := range rels {
		if rel == nil || rel.Target == nil {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		resolvesInHeader := false
		switch target := target.(type) {
		case *ast.QualifiedName:
			if len(target.Parts) > 0 {
				resolvesInHeader = rel.Kind != ast.RelTyping &&
					rel.Kind != ast.RelSpecializes &&
					rel.Kind != ast.RelSubsets &&
					r.headerHasName(header, target.Parts[0].Text, rel.Kind)
			}
		case *ast.FeatureChainExpr:
			if len(target.Member.Parts) > 0 {
				resolvesInHeader = r.headerHasName(header, target.Member.Parts[0].Text, rel.Kind)
			}
		}
		if resolvesInHeader {
			r.resolveRelationships(header, decl, []*ast.Relationship{rel})
		} else {
			r.resolveRelationships(parent, decl, []*ast.Relationship{rel})
		}
	}
}

func (r *Resolver) headerHasName(scope *symbols.Scope, name string, kind ast.RelationshipKind) bool {
	if scope == nil {
		return false
	}
	if _, ok := scope.LookupLocal(name); ok {
		return true
	}
	for _, imp := range r.importsOf(scope.Node()) {
		if !r.importPrefixAvailable(scope, imp, name) {
			continue
		}
		if _, ok := r.matchImport(scope, imp, name); ok {
			return true
		}
	}
	// A featuring or crossing name may be one the declaration inherits from its
	// type; a redefinition or subsetting target may not, as it would find itself.
	if kind == ast.RelFeaturedBy || kind == ast.RelCrosses {
		if _, ok := r.lookupContributedMember(scope.Owner(), name); ok {
			return true
		}
	}
	return false
}

func relationshipTargetsDecl(rel *ast.Relationship, decl ast.Node) bool {
	// Keep a declaration's self-reference out of inherited lookup; cycle detection handles it.
	if rel == nil || decl == nil || rel.Target == nil {
		return false
	}
	target := rel.Target
	if fr, ok := target.(*ast.FeatureReference); ok {
		target = fr.Name
	}
	qn, ok := target.(*ast.QualifiedName)
	if !ok || len(qn.Parts) != 1 {
		return false
	}
	name := ""
	switch d := decl.(type) {
	case *ast.Definition:
		name = d.Ident.Name
	case *ast.Usage:
		name, _ = ast.EffectiveName(d)
	}
	return name != "" && qn.Parts[0].Text == name
}

// resolveConstraintReference resolves the requirement a require/assume member
// subsets by reference (SysML.xtext RequirementConstraintUsage).
func (r *Resolver) resolveConstraintReference(scope *symbols.Scope, ref *ast.QualifiedName) *symbols.Symbol {
	if ref == nil || len(ref.Parts) == 0 {
		return nil
	}
	sym, ok := r.ResolveQualified(scope, ref)
	if !ok {
		return nil
	}
	return sym
}

// walkConstraintBody walks the body of a require/assume member, in the scope its
// declarations were built into so that nested bodies resolve too. The member
// reference-subsets the requirement ref, so the body may redefine that
// requirement's features by plain name (SysML.xtext RequirementConstraintUsage).
func (r *Resolver) walkConstraintBody(scope *symbols.Scope, decl ast.Node, ref *symbols.Symbol, body []ast.Node) {
	scope = symbols.ConstraintBodyScope(scope, decl)
	if scope == nil {
		return
	}
	if ref != nil {
		members := make(map[ast.Node]bool, len(body))
		for _, m := range body {
			member, _ := unwrapForResolve(m)
			members[member] = true
		}
		r.constraintRefs = append(r.constraintRefs, constraintRef{ref: ref, members: members})
		defer func() { r.constraintRefs = r.constraintRefs[:len(r.constraintRefs)-1] }()
	}
	r.walkMembers(scope, body)
}

// constraintRef is a requirement referenced by a require/assume member, with
// the direct members of that member's body, which redefine its features.
type constraintRef struct {
	ref     *symbols.Symbol
	members map[ast.Node]bool
}

// lookupConstraintRefFeature finds name among the features of the requirement
// referenced by the require/assume member whose body declares decl.
func (r *Resolver) lookupConstraintRefFeature(name string, decl ast.Node) (*symbols.Symbol, bool) {
	for i := len(r.constraintRefs) - 1; i >= 0; i-- {
		if !r.constraintRefs[i].members[decl] {
			continue // a nested declaration inherits from its own type, not the reference
		}
		return r.featureOf(r.constraintRefs[i].ref, name, map[*symbols.Symbol]bool{})
	}
	return nil, false
}

// featureOf finds the feature named name declared by sym or inherited through
// its typings, specializations and featurings, walking live-parsed and
// cache-restored symbols alike. seen makes the walk cycle-safe: the standard
// library holds specialization cycles, so every visited symbol is recorded.
func (r *Resolver) featureOf(sym *symbols.Symbol, name string, seen map[*symbols.Symbol]bool) (*symbols.Symbol, bool) {
	if sym == nil || seen[sym] {
		return nil, false
	}
	seen[sym] = true
	if sym.Scope != nil {
		if found, ok := sym.Scope.LookupLocal(name); ok && inheritableMember(found) {
			return found, true
		}
	} else if sym.Decl == nil && r.idx != nil {
		// A restored library symbol has no scope; its members are indexed.
		for _, found := range r.idx.LookupQualified(sym.Name + "::" + name) {
			if resolved, ok := r.ResolveAliasTarget(found); ok && inheritableMember(resolved) {
				return resolved, true
			}
		}
	}
	for _, general := range r.generalsOf(sym) {
		// A feature of sym redefining what a general declares means sym does not
		// inherit it, under that name or any other (KerML 8.3.3.3).
		if found, ok := r.featureOf(general, name, seen); ok && !r.inheritanceMasked(sym, found) {
			return found, true
		}
	}
	return nil, false
}

// inheritableMember reports whether a member can be reached through a
// specialization: KerML 8.2.3.5 excludes private memberships from
// inheritedMembership, so only public and protected members are inherited.
func inheritableMember(sym *symbols.Symbol) bool {
	return sym != nil && symbols.VisibleAs(sym.Visibility, false, true)
}

// inheritanceMasked reports whether sym does not inherit found because one of
// sym's own features redefines it.
func (r *Resolver) inheritanceMasked(sym, found *symbols.Symbol) bool {
	model, ok := r.model.(maskChecker)
	return ok && model.InheritanceMasked(sym, found)
}

// inheritanceMaskedDeclaring is inheritanceMasked as the declaration named
// declName, being written in sym, sees it: its own redefinition still names its
// target, and so does the inherited namesake it redefines.
func (r *Resolver) inheritanceMaskedDeclaring(sym, found *symbols.Symbol, declName string) bool {
	model, ok := r.model.(maskChecker)
	return ok && model.InheritanceMaskedDeclaring(sym, found, declName)
}

// declaredNameIn returns the name decl binds in sym's own scope, if any.
func declaredNameIn(sym *symbols.Symbol, decl ast.Node) string {
	if sym == nil || sym.Scope == nil || decl == nil {
		return ""
	}
	var found *symbols.Symbol
	sym.Scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member.Decl == decl {
			found = member
			return false
		}
		return true
	})
	if found != nil {
		if i := strings.LastIndex(found.Name, "::"); i >= 0 {
			return found.Name[i+2:]
		}
		return found.Name
	}
	return ""
}

// generalsOf returns the symbols sym inherits features from: the resolved
// specialization, typing and featuring targets of its declaration.
func (r *Resolver) generalsOf(sym *symbols.Symbol) []*symbols.Symbol {
	var rels []*ast.Relationship
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		rels = decl.Relationships
	case *ast.Usage:
		rels = decl.Relationships
	default:
		return nil
	}
	scope := sym.OwnerScope
	generals := r.findSpecializationTargets(scope, rels)
	generals = append(generals, r.findTypingTargets(scope, rels)...)
	return append(generals, r.findFeaturedByTargets(scope, rels)...)
}

// resolveRedefinition resolves a redefinition target by looking up the inheritance chain.
// Searches for the feature in parent definitions (following specialization relationships).
//
// decl owns the redefinition. An unnamed redefining feature takes the redefined
// feature's name (KerML 7.3.4.5), so that borrowed binding is hidden from the
// target, which names the redefined feature itself.
func (r *Resolver) resolveRedefinition(scope *symbols.Scope, qn *ast.QualifiedName, decl ast.Node) {
	// If already resolved, skip
	if qn == nil || len(qn.Parts) == 0 {
		return
	}

	hide := &refFilter{
		decl:             decl,
		skipBorrowedName: true,
	}

	if len(qn.Parts) == 1 {
		featureName := qn.Parts[0].Text
		if sym, ok := r.lookupConstraintRefFeature(featureName, decl); ok {
			r.recordRedefined(qn, sym)
			return
		}
	}

	ownerNode := scope.Node()
	var ownerRels []*ast.Relationship
	switch owner := ownerNode.(type) {
	case *ast.Definition:
		ownerRels = owner.Relationships
	case *ast.Usage:
		ownerRels = owner.Relationships
	case *ast.Package:
		r.resolveQualified(scope, qn, hide)
		return
	default:
		r.resolveQualified(scope, qn, hide)
		return
	}

	var parents []*symbols.Symbol
	if model, ok := r.model.(supertypeProvider); ok {
		parents = model.DirectSupertypes(scope.Owner())
	} else {
		parents = r.findSpecializationTargets(scope, ownerRels)
		if _, ok := ownerNode.(*ast.Usage); ok {
			parents = append(parents, r.findTypingTargets(scope, ownerRels)...)
		}
		parents = append(parents, r.findFeaturedByTargets(scope, ownerRels)...)
	}
	if def, ok := ownerNode.(*ast.Definition); ok {
		parents = append(parents, r.findImplicitSpecializations(scope, def)...)
	}

	if len(qn.Parts) == 1 {
		featureName := qn.Parts[0].Text

		// Search each parent's inheritance chain, cached and live alike.
		seen := make(map[*symbols.Symbol]bool)
		for _, parentSym := range parents {
			if sym, ok := r.featureOf(parentSym, featureName, seen); ok {
				r.recordRedefined(qn, sym)
				return
			}
		}

		// The walk above follows declared specializations only. The semantic
		// model also knows the implicit ones — a library base, or the baseType
		// a semantic-metadata keyword contributes (SysML v2 §7.27.3).
		owner := scope.Owner()
		if sym, ok := r.lookupContributedMember(owner, featureName); ok &&
			visibleAsInheritedMember(owner, sym) &&
			!r.inheritanceMaskedDeclaring(owner, sym, declaredNameIn(owner, decl)) {
			r.recordRedefined(qn, sym)
			return
		}
	} else {
		first := qn.Parts[0].Text
		for _, parent := range parents {
			if parent == nil || (parent.Name != first && parent.ShortName != first) {
				continue
			}
			p := parent
			var result resolution
			if r.probe(qn, func() bool {
				p = r.resolvedPart(qn, 0, p)
				result = r.walkQualifiedTail(scope, qn, p, 1)
				return result.ok
			}) {
				r.memoize(qn, result)
				return
			}
		}
	}

	// Fall back to standard resolution if not found in parents
	r.resolveQualified(scope, qn, hide)
}

// declaresConnector reports whether scope is the body of a connector, whose
// ends relate features of the type featuring it (KerML 8.3.4.5).
func declaresConnector(scope *symbols.Scope) bool {
	u, ok := scope.Node().(*ast.Usage)
	if !ok {
		return false
	}
	switch u.Kind {
	case ast.UsageConnector, ast.UsageConnection, ast.UsageBinding,
		ast.UsageSuccession, ast.UsageFlow, ast.UsageAllocation:
		return true
	}
	return false
}

// recordRedefined records sym as what the redefinition target qn names, and
// memoizes it, so a later unfiltered query — the semantic model reading the
// same relationship — does not find the borrowed name instead.
func (r *Resolver) recordRedefined(qn *ast.QualifiedName, sym *symbols.Symbol) {
	r.recordPart(qn, 0, sym)
	r.memoize(qn, resolution{sym: sym, ok: true})
}

// findSpecializationTargets returns symbols for all specialization targets in the relationship list.
func (r *Resolver) findSpecializationTargets(scope *symbols.Scope, rels []*ast.Relationship) []*symbols.Symbol {
	var parents []*symbols.Symbol

	for _, rel := range rels {
		if rel == nil || rel.Kind != ast.RelSpecializes {
			continue
		}

		// Extract target name
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}

		if qn, ok := target.(*ast.QualifiedName); ok {
			// Resolve the specialization target
			if sym, ok := r.ResolveQualified(scope, qn); ok && sym != nil {
				if resolved, aliasOK := r.ResolveAliasTarget(sym); aliasOK {
					sym = resolved
				} else {
					continue
				}
				parents = append(parents, sym)
			}
		}
	}

	return parents
}

// findTypingTargets resolves the types named by the typing relationships in
// rels, which are the generals a usage inherits members from.
func (r *Resolver) findTypingTargets(scope *symbols.Scope, rels []*ast.Relationship) []*symbols.Symbol {
	var parents []*symbols.Symbol
	for _, rel := range rels {
		if rel == nil || rel.Kind != ast.RelTyping {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		if qn, ok := target.(*ast.QualifiedName); ok {
			if sym, ok := r.ResolveQualified(scope, qn); ok && sym != nil {
				if resolved, aliasOK := r.ResolveAliasTarget(sym); aliasOK {
					sym = resolved
				} else {
					continue
				}
				parents = append(parents, sym)
			}
		}
	}
	return parents
}

func (r *Resolver) findFeaturedByTargets(scope *symbols.Scope, rels []*ast.Relationship) []*symbols.Symbol {
	var parents []*symbols.Symbol
	for _, rel := range rels {
		if rel == nil || rel.Kind != ast.RelFeaturedBy {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		if qn, ok := target.(*ast.QualifiedName); ok {
			if sym, ok := r.ResolveQualified(scope, qn); ok && sym != nil {
				if resolved, aliasOK := r.ResolveAliasTarget(sym); aliasOK {
					sym = resolved
				} else {
					continue
				}
				parents = append(parents, sym)
			}
		}
	}
	return parents
}

// findImplicitSpecializations returns implicit base types for a definition based on its kind.
// For example, 'flow def' implicitly specializes 'Flow' from kernel library.
func (r *Resolver) findImplicitSpecializations(scope *symbols.Scope, def *ast.Definition) []*symbols.Symbol {
	var parents []*symbols.Symbol

	// Map definition kind to base type FQN
	var baseFQN string
	switch def.Kind {
	case ast.DefPart:
		baseFQN = "Parts::Part"
	case ast.DefItem:
		baseFQN = "Items::Item"
	case ast.DefFlow:
		baseFQN = "Flows::Flow"
	case ast.DefConnection:
		baseFQN = "Connections::Connection"
	case ast.DefInterface:
		baseFQN = "Interfaces::Interface"
	case ast.DefAllocation:
		baseFQN = "Allocations::Allocation"
	// Add more as needed
	default:
		return nil
	}

	// Look up base type in index
	if r.idx != nil {
		candidates := r.idx.LookupQualified(baseFQN)
		if len(candidates) == 1 {
			parents = append(parents, candidates[0])
		}
	}

	return parents
}

// resolveExpr walks an expression subtree resolving feature references and
// classification type references.
func (r *Resolver) resolveExpr(scope *symbols.Scope, e ast.Node) {
	switch v := e.(type) {
	case nil:
		return
	case *ast.FeatureReference:
		r.ResolveQualified(scope, v.Name)
	case *ast.OperatorExpr:
		for _, op := range v.Operands {
			r.resolveExpr(scope, op)
		}
		if v.TypeRef != nil {
			r.ResolveQualified(scope, v.TypeRef)
		}
	case *ast.FeatureChainExpr:
		r.resolveFeatureChain(scope, v)
	case *ast.IndexExpr:
		r.resolveExpr(scope, v.Operand)
		r.resolveExpr(scope, v.Index)
	case *ast.InvocationExpr:
		r.resolveExpr(scope, v.Operand)
		if v.Type != nil {
			r.ResolveQualified(scope, v.Type)
		}
		for _, a := range v.Args {
			r.resolveExpr(scope, a)
		}
		for _, na := range v.NamedArgs {
			// Named argument names are parameter identifiers, not references
			// Don't resolve na.Name - it's looked up in callee's parameter list
			r.resolveExpr(scope, na.Value)
		}
	case *ast.CollectExpr:
		r.resolveExpr(scope, v.Operand)
		r.resolveExpr(scope, v.Body)
	case *ast.SelectExpr:
		r.resolveExpr(scope, v.Operand)
		r.resolveExpr(scope, v.Body)
	case *ast.ConstructorExpr:
		if v.Type != nil {
			r.ResolveQualified(scope, v.Type)
		}
		for _, a := range v.Args {
			r.resolveExpr(scope, a)
		}
	case *ast.BodyExpr:
		for i := range v.Params {
			p := &v.Params[i]
			if p.Type != nil {
				r.ResolveQualified(scope, p.Type)
			}
			r.resolveRelationships(scope, v, p.Relationships)
			r.resolveExpr(scope, p.Value)
		}
		// A body expression's parameters and declarations live in a scope of its
		// own, and its declarations are members of it (F64).
		inner := symbols.BodyExprScope(scope, v)
		r.walkMembers(inner, v.Members)
		r.resolveExpr(inner, v.Result)
	case *ast.SequenceExpr:
		for _, el := range v.Elements {
			r.resolveExpr(scope, el)
		}
	case *ast.MetadataAccessExpr:
		r.ResolveQualified(scope, v.Ref)
	case *ast.QualifiedName:
		// A bare name in expression position parses straight to a qualified
		// name rather than to a FeatureReference wrapper.
		r.ResolveQualified(scope, v)
	}
	// Literals (LiteralBool/String/Integer/Real/Infinity, NullExpr) have no refs.
}

// resolveFeatureChain resolves a FeatureChainExpr and returns its final symbol.
func (r *Resolver) resolveFeatureChain(scope *symbols.Scope, fc *ast.FeatureChainExpr) *symbols.Symbol {
	if fc == nil {
		return nil
	}
	key := featureChainKey{scope: scope, node: fc}
	if res, done := r.featureChains[key]; done {
		return res.sym
	}

	// Get the operand symbol without following its type for inline members.
	operandSym := r.getOperandSymbol(scope, fc.Operand)
	if operandSym == nil || fc.Member == nil {
		r.memoizeFeatureChain(scope, fc, resolution{})
		return nil
	}
	if featuring := r.featuringOf(scope, operandSym); featuring != nil {
		operandSym = featuring
	}
	// A chain from `this` reads the object the body is owned by, which the
	// library declares as its context occurrence rather than as its type.
	if r.IsOccurrenceThis(operandSym) {
		if object := r.ThisContext(scope); object != nil {
			operandSym = object
		}
	}

	// A chaining feature spelled as a qualified name resolves outward through the
	// enclosing namespaces when the previous element has no such member (KerML
	// §7.2.5): in `A::B.C::D`, `C::D` names a declaration, not a member of `B`.
	// The outward reading is probed, so that a chain the walk below reads instead
	// keeps its own diagnostics and per-segment symbols (see probe).
	if len(fc.Member.Parts) > 1 {
		if _, ok := r.chainMember(operandSym, fc.Member.Parts[0].Text); !ok {
			var outwardSym *symbols.Symbol
			outward := r.probe(fc.Member, func() bool {
				var ok bool
				outwardSym, ok = r.ResolveQualified(scope, fc.Member)
				return ok
			})
			if outward {
				outwardSym = r.followChainMemberType(outwardSym)
				r.memoizeFeatureChain(scope, fc, resolution{sym: outwardSym, ok: outwardSym != nil})
				return outwardSym
			}
		}
	}

	memberSym := r.resolveMemberChain(operandSym, fc.Member)
	memberSym = r.followChainMemberType(memberSym)
	r.memoizeFeatureChain(scope, fc, resolution{sym: memberSym, ok: memberSym != nil})
	return memberSym
}

// chainMember looks a chain segment up as the member walk does: as a member of
// sym when a model is attached, else in sym's own scope.
func (r *Resolver) chainMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	if found, ok := r.lookupMember(sym, name); ok && r.namedThroughNamespace(found) {
		return found, true
	}
	if sym == nil || sym.Scope == nil {
		return nil, false
	}
	found, ok := sym.Scope.LookupLocal(name)
	if !ok || !r.namedThroughNamespace(found) {
		return nil, false
	}
	return found, true
}

// resolveMemberChain walks a qualified name member-by-member in the given scope,
// assigning each part's symbol explicitly (for feature chain member access).
func (r *Resolver) resolveMemberChain(parentSym *symbols.Symbol, qn *ast.QualifiedName) *symbols.Symbol {
	if qn == nil || len(qn.Parts) == 0 {
		return nil
	}

	// Resolve first part using model.LookupMember if available, else
	// scope.LookupLocal. A chained feature names a member of what precedes it,
	// so it reaches only the visible ones (KerML 8.2.3.5).
	cur, ok := r.chainMember(parentSym, qn.Parts[0].Text)

	if !ok {
		msg := "unresolved member: " + qn.Parts[0].Text
		if parentSym.Scope == nil {
			msg = "no scope for member lookup in " + parentSym.Name
		}
		r.Diagnostics = append(r.Diagnostics, Diagnostic{Span: qn.Parts[0].Span, Message: msg})
		return nil
	}
	r.recordPart(qn, 0, cur)

	// Walk remaining parts via member lookup
	for i := 1; i < len(qn.Parts); i++ {
		next, found := r.chainMember(cur, qn.Parts[i].Text)
		if !found && cur.Scope == nil {
			r.Diagnostics = append(r.Diagnostics, Diagnostic{
				Span:    qn.Parts[i].Span,
				Message: "no members in " + cur.Name,
			})
			return nil
		}

		if !found {
			r.Diagnostics = append(r.Diagnostics, Diagnostic{
				Span:    qn.Parts[i].Span,
				Message: "unresolved member: " + qn.Parts[i].Text + " in " + cur.Name,
			})
			return nil
		}

		r.recordPart(qn, i, next)
		cur = next
	}

	// Store final resolution in memo
	r.memoize(qn, resolution{cur, true})
	return cur
}

// getOperandSymbol returns the symbol of an expression operand WITHOUT following
// type relationships. Used in feature chains to access usage inline members.
func (r *Resolver) getOperandSymbol(scope *symbols.Scope, e ast.Node) *symbols.Symbol {
	switch v := e.(type) {
	case *ast.FeatureReference:
		if v.Name == nil {
			return nil
		}
		var sym *symbols.Symbol
		var ok bool
		if len(v.Name.Parts) == 1 && !v.Name.Global {
			sym, ok = r.LookupName(scope, v.Name.Parts[0].Text)
			if !ok {
				sym, ok = r.ResolveQualified(scope, v.Name)
			}
		} else {
			sym, ok = r.ResolveQualified(scope, v.Name)
		}
		if !ok {
			return nil
		}
		// If usage has inline members (scope), return it to access those members
		// Otherwise follow type for inherited members
		if usage, isUsage := sym.Decl.(*ast.Usage); isUsage && !r.IsBaseThat(sym) && !r.IsOccurrenceThis(sym) {
			if sym.Scope != nil &&
				(len(sym.Scope.Members()) > 0 || len(r.importsOf(sym.Scope.Node())) > 0) {
				// Usage has inline members, return usage symbol
				return sym
			}
			// No inline members, follow type
			typeSym := r.getUsageType(sym.OwnerScope, usage)
			if typeSym != nil {
				return typeSym
			}
		}
		return sym
	case *ast.FeatureChainExpr:
		return r.resolveFeatureChain(scope, v)
	default:
		r.resolveExpr(scope, e)
		return nil
	}
}

// followChainMemberType follows a usage's type when it has no inline members.
func (r *Resolver) followChainMemberType(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil {
		return nil
	}
	usage, isUsage := sym.Decl.(*ast.Usage)
	if !isUsage {
		return sym
	}
	if sym.Scope != nil && len(sym.Scope.Members()) > 0 {
		return sym
	}
	if typeSym := r.getUsageType(sym.OwnerScope, usage); typeSym != nil {
		return typeSym
	}
	return sym
}

// baseThatFQN is the implicit `that` feature every usage takes from the base
// usage Base::things ([KerML, 8.4.2]).
const baseThatFQN = "Base::things::that"

// featuringOf returns what a member chain from `that` reads its members from:
// the usage enclosing the expression, whose value features the value being
// written. It is nil for any other operand, and where no usage encloses the
// expression — `that` is typed Anything, which has no members of its own.
func (r *Resolver) featuringOf(scope *symbols.Scope, operand *symbols.Symbol) *symbols.Symbol {
	if !r.IsBaseThat(operand) {
		return nil
	}
	for s := scope; s != nil; s = s.Parent() {
		owner := s.Owner()
		if owner == nil {
			continue
		}
		if _, ok := owner.Decl.(*ast.Usage); ok {
			return owner
		}
		return nil
	}
	return nil
}

// IsBaseThat reports whether sym is the implicit `that` feature of the base
// usage: it names the object featuring a usage's values, so it is reachable in
// any usage body and its declared type Anything is not what a chain reads.
func (r *Resolver) IsBaseThat(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	return sym.Name == baseThatFQN || r.registeredFQN(sym) == baseThatFQN
}

// getUsageType returns the type symbol of a usage by resolving its typing relationship.
func (r *Resolver) getUsageType(scope *symbols.Scope, usage *ast.Usage) *symbols.Symbol {
	for _, rel := range usage.Relationships {
		if rel.Kind == ast.RelTyping && rel.Target != nil {
			// Unwrap FeatureReference if needed
			target := rel.Target
			if fr, ok := target.(*ast.FeatureReference); ok {
				target = fr.Name
			}
			if qn, ok := target.(*ast.QualifiedName); ok {
				typeSym, _ := r.ResolveQualified(scope, qn)
				return typeSym
			}
		}
	}
	// A feature with no declared type takes the type of the value bound to it
	// (KerML 1.0 §7.4.9 FeatureValue), so its members are the value's members.
	return r.valueType(scope, usage)
}

// valueType returns what a usage's value expression names, for the member
// lookups a chain through the usage makes. Only the forms that denote a feature
// are followed; anything else has no members to reach.
func (r *Resolver) valueType(scope *symbols.Scope, usage *ast.Usage) *symbols.Symbol {
	if usage.Value == nil || r.valuesInProgress[usage] {
		return nil
	}
	r.valuesInProgress[usage] = true
	defer delete(r.valuesInProgress, usage)

	expr := usage.Value
	var sym *symbols.Symbol
	// The value is read on behalf of a member lookup, so its own diagnostics
	// belong to the reference that wrote it, not to this one.
	r.aside(func() {
		if found, ok := r.ResolveTarget(scope, expr); ok {
			sym = found
		}
	})
	if sym == nil {
		return nil
	}
	return r.followChainMemberType(sym)
}
