package symbols

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// Build constructs the immutable scope tree for a parsed document.
func Build(root *ast.RootNamespace) *Scope {
	rootScope := NewScope(nil, nil)
	if root == nil {
		return rootScope
	}
	buildMembers(rootScope, root.Members)
	// Body-expression parameter scopes hang off the scopes their expressions
	// resolve against, so they are linked once the declarations exist.
	buildBodyScopes(rootScope, root.Members)
	return rootScope
}

// buildMembers processes a member list into the given scope.
func buildMembers(scope *Scope, members []ast.Node) {
	for _, m := range members {
		decl, vis := unwrapMember(m)
		if decl == nil {
			continue
		}
		// Trivia (doc comments) is attached to the member wrapper m, not to the
		// unwrapped inner decl; capture it here so the Symbol can carry it.
		buildDecl(scope, decl, vis, m.LeadingTrivia())
	}
}

// unwrapMember returns the underlying declaration node and its visibility.
// Membership wrappers carry visibility; directly-listed Import/Alias nodes
// carry their own.
func unwrapMember(m ast.Node) (ast.Node, ast.Visibility) {
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

// buildDecl registers a symbol (and child scope, where applicable) for a single
// declaration node. trivia is the leading trivia captured from the member
// wrapper before unwrap.
func buildDecl(scope *Scope, decl ast.Node, vis ast.Visibility, trivia []ast.Trivia) {
	switch d := decl.(type) {
	case *ast.Package:
		child := NewScope(scope, d)
		sym := newSymbol(d.Ident, SymbolPackage, d, vis, child, scope, trivia)
		defineIdent(scope, d.Ident, sym)
		scope.AddChild(child)
		buildMembers(child, d.Members)
	case *ast.Namespace:
		child := NewScope(scope, d)
		sym := newSymbol(d.Ident, SymbolNamespace, d, vis, child, scope, trivia)
		defineIdent(scope, d.Ident, sym)
		scope.AddChild(child)
		buildMembers(child, d.Members)
	case *ast.Alias:
		sym := newSymbol(d.Ident, SymbolAlias, d, vis, nil, scope, trivia)
		defineIdent(scope, d.Ident, sym)
	case *ast.Dependency:
		sym := newSymbol(d.Ident, SymbolDependency, d, vis, nil, scope, trivia)
		defineIdent(scope, d.Ident, sym)
	case *ast.Comment:
		sym := newSymbol(d.Ident, SymbolComment, d, vis, nil, scope, trivia)
		defineIdent(scope, d.Ident, sym)
	case *ast.Documentation:
		sym := newSymbol(d.Ident, SymbolDocumentation, d, vis, nil, scope, trivia)
		defineIdent(scope, d.Ident, sym)
	case *ast.TextualRepresentation:
		sym := newSymbol(d.Ident, SymbolTextualRepresentation, d, vis, nil, scope, trivia)
		defineIdent(scope, d.Ident, sym)
	case *ast.Definition:
		child := NewScope(scope, d)
		sym := newSymbol(d.Ident, definitionSymbolKind(d.Kind), d, vis, child, scope, trivia)
		defineIdent(scope, d.Ident, sym)
		scope.AddChild(child)
		buildMembers(child, d.Members)
	case *ast.Usage:
		child := NewScope(scope, d)
		// Phase 4: Parser treats 'datatype' uniformly as usage. Builder classifies based on context.
		// If usage is attribute kind with specializes/subsets but no typing, treat as definition.
		kind := classifyUsage(d)
		id, namingTarget := effectiveIdent(d)
		sym := newSymbol(id, kind, d, vis, child, scope, trivia)
		sym.EffectiveName = namingTarget != nil
		sym.NamingTarget = namingTarget
		defineIdent(scope, id, sym)
		scope.AddChild(child)
		buildMembers(child, d.Members)
		buildConnectorEnds(child, d)
	case *ast.SubstateMember:
		// SubstateMember represents simple state declaration: state <name>;
		// Create a state usage symbol for it
		id := ast.Identification{Name: d.Name}
		child := NewScope(scope, d)
		sym := newSymbol(id, SymbolStateUsage, d, vis, child, scope, trivia)
		defineIdent(scope, id, sym)
		scope.AddChild(child)
	case *ast.SubjectMember:
		// SubjectMember represents requirement subject: subject <name> : <Type>;
		// Create a part usage symbol (subject is structural usage like part)
		id := ast.Identification{Name: d.Name}
		child := NewScope(scope, d)
		sym := newSymbol(id, SymbolPartUsage, d, vis, child, scope, trivia)
		defineIdent(scope, id, sym)
		scope.AddChild(child)
		if len(d.Body) > 0 {
			buildMembers(child, d.Body)
		}
	case *ast.Import, *ast.FilterMember, *ast.ErrorNode:
		// Imports are processed during resolution; filters hold expressions;
		// error nodes have no declaration. Nothing to register here.
	case *ast.InitialNode:
		// Register initial node by name so transitions can reference it
		if d.Name != "" {
			id := ast.Identification{Name: d.Name}
			child := NewScope(scope, d)
			// Use attribute usage kind (control flow nodes are structural members)
			sym := newSymbol(id, SymbolAttributeUsage, d, vis, child, scope, trivia)
			defineIdent(scope, id, sym)
			scope.AddChild(child)
		}
	case *ast.FinalNode:
		// Register final node by name so transitions can reference it
		if d.Name != "" {
			id := ast.Identification{Name: d.Name}
			child := NewScope(scope, d)
			sym := newSymbol(id, SymbolAttributeUsage, d, vis, child, scope, trivia)
			defineIdent(scope, id, sym)
			scope.AddChild(child)
		}
	case *ast.StateNode:
		// Register state node by name (including initial/final pseudostates)
		// so transitions and successions can reference it
		if d.Name == "" {
			return
		}
		id := ast.Identification{Name: d.Name}
		child := NewScope(scope, d)
		sym := newSymbol(id, SymbolStateUsage, d, vis, child, scope, trivia)
		defineIdent(scope, id, sym)
		scope.AddChild(child)
		// Substates and regions declare the names the state's own body refers to.
		buildMembers(child, d.Substates)
		for _, region := range d.Regions {
			buildDecl(child, region, ast.VisibilityDefault, nil)
		}
	case *ast.TransitionMember:
		// A named transition is a feature of the state that declares it (SysML v2
		// §7.19.2: TransitionUsage specializes ActionUsage), and its effect
		// behaviors are features of the transition, so `t.effectAction` resolves.
		if d.Name == "" {
			return
		}
		id := ast.Identification{Name: d.Name}
		child := NewScope(scope, d)
		sym := newSymbol(id, SymbolActionUsage, d, vis, child, scope, trivia)
		defineIdent(scope, id, sym)
		scope.AddChild(child)
		buildMembers(child, d.Effect)
	case *ast.StateRegion:
		// A region is a namespace of its own: sibling regions routinely reuse
		// state names (each region declaring its own `initial start`), so their
		// states must not collide in the composite state's scope.
		regionScope := NewScope(scope, d)
		if d.Name != "" {
			id := ast.Identification{Name: d.Name}
			sym := newSymbol(id, SymbolStateUsage, d, vis, regionScope, scope, trivia)
			defineIdent(scope, id, sym)
		}
		scope.AddChild(regionScope)
		buildMembers(regionScope, d.States)
	case *ast.EntryMember:
		// An entry/do/exit action is a feature of the state declaring it, so a
		// named one (`entry action entryAction :>> 'entry';`) is a member of the
		// state's scope rather than of the wrapper the parser puts it in.
		buildMembers(scope, d.Actions)
	case *ast.DoMember:
		buildMembers(scope, d.Actions)
	case *ast.ExitMember:
		buildMembers(scope, d.Actions)
	case *ast.PseudostateNode:
		// fork/join/choice/junction/entry/exit named in a state body are
		// transition endpoints, so they must be referenceable.
		if d.Name != "" {
			id := ast.Identification{Name: d.Name}
			child := NewScope(scope, d)
			sym := newSymbol(id, SymbolStateUsage, d, vis, child, scope, trivia)
			defineIdent(scope, id, sym)
			scope.AddChild(child)
		}
	case *ast.WhileLoopActionNode:
		// A loop is an anonymous action namespace: its body's declarations (and a
		// `for` iteration variable) are members visible to the body and condition.
		child := NewScope(scope, d)
		child.markBodyLocal()
		scope.AddChild(child)
		if d.Kind == ast.LoopFor && d.Variable.Name != "" {
			// The variable's own span, not the whole loop's: the editor renames
			// through NameSpan and jumps to DeclSpan.
			child.Define(d.Variable.Name, &Symbol{
				Name:       d.Variable.Name,
				Kind:       SymbolAttributeUsage,
				Decl:       d,
				DeclSpan:   d.Variable.NameSpan,
				NameSpan:   d.Variable.NameSpan,
				OwnerScope: child,
			})
		}
		buildMembers(child, d.Body)
	case *ast.IfActionNode:
		// Each branch is a namespace of its own: `if c { action a; } else { action a; }`
		// declares two distinct actions, neither of them a member of the enclosing
		// behavior. The condition is evaluated outside both branches, so it is not
		// resolved against them.
		for _, branch := range d.Branches() {
			buildDecl(scope, branch, vis, trivia)
		}
	case *ast.IfBranchNode:
		child := NewScope(scope, d)
		child.markBodyLocal()
		scope.AddChild(child)
		buildMembers(child, d.Body)
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode:
		// Control flow nodes without explicit names in AST - skip indexing
		// (If these nodes gain name fields in future, register them here)
	}
}

// buildConnectorEnds registers a symbol for every end of a connector usage that
// declares a name (`connect bead references t.bead`). Such an end is an end
// feature of the connector itself (SysML v2 §7.13.2), so it is a member of the
// connector's own scope, never of the scope the connector is declared in.
func buildConnectorEnds(scope *Scope, u *ast.Usage) {
	for _, end := range u.ConnectorEnds {
		if end == nil {
			continue
		}
		id, ok := end.DeclaredName()
		if !ok {
			continue
		}
		child := NewScope(scope, end)
		sym := newSymbol(id, SymbolConnectorEnd, end, ast.VisibilityDefault, child, scope, nil)
		defineIdent(scope, id, sym)
		scope.AddChild(child)
	}
}

// newSymbol builds a Symbol from an identification. scope is the child scope the
// declaration owns (nil for leaf declarations); owner is the enclosing scope the
// declaration was declared in. trivia is the leading trivia from the member wrapper.
func newSymbol(id ast.Identification, kind SymbolKind, decl ast.Node, vis ast.Visibility, scope, owner *Scope, trivia []ast.Trivia) *Symbol {
	name := id.Name
	nameSpan := id.NameSpan
	if name == "" {
		name, nameSpan = id.ShortName, id.ShortNameSpan
	}
	sym := &Symbol{
		Name:          name,
		ShortName:     id.ShortName,
		Kind:          kind,
		Decl:          decl,
		Visibility:    vis,
		DeclSpan:      decl.Span(),
		NameSpan:      nameSpan,
		Scope:         scope,
		OwnerScope:    owner,
		LeadingTrivia: trivia,
	}
	// Set scope's owner back-reference for inheritance lookup
	if scope != nil {
		scope.SetOwner(sym)
	}
	return sym
}

// effectiveIdent returns the identification a usage is registered under, and
// the reference that supplied it: the name of the usage's naming feature
// (ast.NamingFeature) keeps its own declared short name.
//
// The naming feature's own effective name is approximated by the reference's
// last segment, since resolution has not run when scopes are built.
func effectiveIdent(u *ast.Usage) (ast.Identification, ast.Node) {
	rel := ast.NamingFeature(u)
	if rel == nil {
		return u.Ident, nil
	}
	name, span := ast.TargetName(rel.Target)
	if name == "" {
		return u.Ident, nil
	}
	id := u.Ident
	id.Name, id.NameSpan = name, span
	return id, namingTargetNode(rel.Target)
}

// namingTargetNode returns the node a relationship target is resolved as, so
// that the resolver can recognise the reference that named a feature: the
// qualified name a plain reference unwraps to, or the target itself.
func namingTargetNode(target ast.Node) ast.Node {
	if qn := ast.AsQualifiedName(target); qn != nil {
		return qn
	}
	return target
}

// defineIdent registers sym under its short and primary name keys, skipping
// any that are empty (e.g. anonymous usages).
func defineIdent(scope *Scope, id ast.Identification, sym *Symbol) {
	if id.ShortName != "" {
		scope.Define(id.ShortName, sym)
	}
	if id.Name != "" {
		scope.Define(id.Name, sym)
	}
	// If both names empty, register as anonymous
	if id.ShortName == "" && id.Name == "" {
		scope.DefineAnonymous(sym)
	}
}

// definitionSymbolKind maps an ast.DefinitionKind to its SymbolKind.
func definitionSymbolKind(k ast.DefinitionKind) SymbolKind {
	switch k {
	case ast.DefPart:
		return SymbolPartDef
	case ast.DefAttribute:
		return SymbolAttributeDef
	case ast.DefItem:
		return SymbolItemDef
	case ast.DefOccurrence:
		return SymbolOccurrenceDef
	case ast.DefIndividual:
		return SymbolIndividualDef
	case ast.DefMetadata:
		return SymbolMetadataDef
	case ast.DefMetaclass:
		return SymbolMetaclass
	case ast.DefEnumeration:
		return SymbolEnumerationDef
	case ast.DefView:
		return SymbolViewDef
	case ast.DefViewpoint:
		return SymbolViewpointDef
	case ast.DefRendering:
		return SymbolRenderingDef
	case ast.DefConcern:
		return SymbolConcernDef
	case ast.DefConnection:
		return SymbolConnectionDef
	case ast.DefFlow:
		return SymbolFlowDef
	case ast.DefPort:
		return SymbolPortDef
	case ast.DefInterface:
		return SymbolInterfaceDef
	case ast.DefAllocation:
		return SymbolAllocationDef
	case ast.DefAction:
		return SymbolActionDef
	case ast.DefState:
		return SymbolStateDef
	case ast.DefCalc:
		return SymbolCalcDef
	case ast.DefConstraint:
		return SymbolConstraintDef
	case ast.DefRequirement:
		return SymbolRequirementDef
	case ast.DefCase:
		return SymbolCaseDef
	case ast.DefAnalysisCase:
		return SymbolAnalysisCaseDef
	case ast.DefVerificationCase:
		return SymbolVerificationCaseDef
	case ast.DefUseCase:
		return SymbolUseCaseDef
	case ast.DefClass, ast.DefStruct, ast.DefAssoc, ast.DefBehavior, ast.DefPredicate:
		return SymbolKerMLType
	default:
		return SymbolUnknown
	}
}

// usageSymbolKind maps an ast.UsageKind to its SymbolKind.
func usageSymbolKind(k ast.UsageKind) SymbolKind {
	switch k {
	case ast.UsagePart:
		return SymbolPartUsage
	case ast.UsageAttribute:
		return SymbolAttributeUsage
	case ast.UsageItem:
		return SymbolItemUsage
	case ast.UsageOccurrence:
		return SymbolOccurrenceUsage
	case ast.UsageIndividual:
		return SymbolIndividualUsage
	case ast.UsageMetadata:
		return SymbolMetadataUsage
	case ast.UsageEnumeration:
		return SymbolEnumerationUsage
	case ast.UsageView:
		return SymbolViewUsage
	case ast.UsageViewpoint:
		return SymbolViewpointUsage
	case ast.UsageRendering, ast.UsageViewRendering:
		// A view's `render` member owns a rendering usage (SysML v2 §8.3.26).
		return SymbolRenderingUsage
	case ast.UsageConcern, ast.UsageFramedConcern:
		// A framed concern is a concern usage (SysML v2 §8.3.20).
		return SymbolConcernUsage
	case ast.UsageConnection, ast.UsageConnector:
		// A KerML `connector` is the connection usage of the kernel layer
		// (KerML 1.0 §7.4.6), so it is one kind of symbol.
		return SymbolConnectionUsage
	case ast.UsageFlow:
		return SymbolFlowUsage
	case ast.UsagePort:
		return SymbolPortUsage
	case ast.UsageInterface:
		return SymbolInterfaceUsage
	case ast.UsageAllocation:
		return SymbolAllocationUsage
	case ast.UsageAction:
		return SymbolActionUsage
	case ast.UsageState:
		return SymbolStateUsage
	case ast.UsageCalc:
		return SymbolCalcUsage
	case ast.UsageConstraint:
		return SymbolConstraintUsage
	case ast.UsageRequirement:
		return SymbolRequirementUsage
	case ast.UsageCase:
		return SymbolCaseUsage
	case ast.UsageAnalysisCase:
		return SymbolAnalysisCaseUsage
	case ast.UsageVerificationCase:
		return SymbolVerificationCaseUsage
	case ast.UsageUseCase:
		return SymbolUseCaseUsage
	case ast.UsageSubject:
		// Subject is a requirement parameter - treat as part usage for structural purposes
		return SymbolPartUsage
	case ast.UsageObjective:
		// Objective is a requirement parameter - treat as part usage for structural purposes
		return SymbolPartUsage
	case ast.UsageActor, ast.UsageStakeholder:
		// An actor and a stakeholder are part usages (SysML v2 §8.3.19).
		return SymbolPartUsage
	case ast.UsageClass, ast.UsageStruct, ast.UsageAssoc, ast.UsageBehavior,
		ast.UsagePredicate, ast.UsageInteraction:
		// The parser records a KerML type declaration as a usage; it declares a
		// type, not a feature (KerML 1.0 §8.3).
		return SymbolKerMLType
	case ast.UsageStep:
		// A KerML step is a feature typed by a behavior, which is what a SysML
		// action usage is (SysML v2 §8.3.14).
		return SymbolActionUsage
	default:
		return SymbolUnknown
	}
}

// classifyUsage determines the correct symbol kind for a usage AST node.
// Per Phase 4: Parser treats some keywords (like 'datatype') uniformly as usage.
// Builder classifies based on semantic context (relationships, body structure).
//
// Classification rules:
// - Attribute usage with specializes (but NO typing/subsets) → AttributeDef
// - Attribute usage with typing or subsets/redefines → AttributeUsage
// - All other usages → use usageSymbolKind directly
func classifyUsage(u *ast.Usage) SymbolKind {
	// A `datatype` declares a KerML DataType (KerML 1.0 §8.3.2): a definition whatever
	// it specializes, unlike the `attribute`/`feature` keywords classified below.
	if u.Keyword == "datatype" {
		return SymbolAttributeDef
	}
	// Only classify attribute usages (datatype, attribute, feature keywords)
	if u.Kind != ast.UsageAttribute {
		return usageSymbolKind(u.Kind)
	}

	// Check relationships to determine if this is def-like or usage-like
	hasTyping := false
	hasSpecializes := false
	hasSubsetsOrRedefines := false

	for _, rel := range u.Relationships {
		switch rel.Kind {
		case ast.RelTyping:
			hasTyping = true
		case ast.RelSpecializes:
			hasSpecializes = true
		case ast.RelSubsets, ast.RelRedefines:
			hasSubsetsOrRedefines = true
		}
	}

	// Attribute usage with ONLY specializes (no typing/subsets) → classify as definition
	// Pattern: datatype Real specializes Complex;
	// NOT: datatype MyReal :>> Real; (this has subsets, stays as usage)
	if hasSpecializes && !hasTyping && !hasSubsetsOrRedefines {
		return SymbolAttributeDef
	}

	// Default: treat as usage
	return SymbolAttributeUsage
}
