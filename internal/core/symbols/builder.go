package symbols

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

// Build constructs the immutable scope tree for a parsed document.
func Build(root *ast.RootNamespace) *Scope {
	rootScope := NewScope(nil, nil)
	if root == nil {
		return rootScope
	}
	buildMembers(rootScope, root.Members)
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
		sym := newSymbol(d.Ident, kind, d, vis, child, scope, trivia)
		defineIdent(scope, d.Ident, sym)
		scope.AddChild(child)
		buildMembers(child, d.Members)
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
	case *ast.ResultMember:
		// ResultMember represents calc/function result: return <expr>; or return <name>;
		// If expression is a simple identifier/reference, create attribute usage symbol
		// so the result can be accessed via feature chain (e.g., calc.resultName)
		name := extractResultName(d.Expression)
		if name != "" {
			id := ast.Identification{Name: name}
			child := NewScope(scope, d)
			sym := newSymbol(id, SymbolAttributeUsage, d, vis, child, scope, trivia)
			defineIdent(scope, id, sym)
			scope.AddChild(child)
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
		if d.Name != "" {
			id := ast.Identification{Name: d.Name}
			child := NewScope(scope, d)
			sym := newSymbol(id, SymbolStateUsage, d, vis, child, scope, trivia)
			defineIdent(scope, id, sym)
			scope.AddChild(child)
		}
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode:
		// Control flow nodes without explicit names in AST - skip indexing
		// (If these nodes gain name fields in future, register them here)
	}
}

// extractResultName attempts to extract a simple identifier name from a result expression.
// Returns empty string if expression is complex or not a simple name reference.
func extractResultName(expr ast.Node) string {
	if expr == nil {
		return ""
	}

	// Handle FeatureReference wrapper
	if fr, ok := expr.(*ast.FeatureReference); ok {
		expr = fr.Name
	}

	// Extract name from QualifiedName
	if qn, ok := expr.(*ast.QualifiedName); ok {
		// Only accept single-part names (simple identifiers)
		if len(qn.Parts) == 1 {
			return qn.Parts[0].Text
		}
	}

	return ""
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
	case ast.UsageRendering:
		return SymbolRenderingUsage
	case ast.UsageConcern:
		return SymbolConcernUsage
	case ast.UsageConnection:
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
