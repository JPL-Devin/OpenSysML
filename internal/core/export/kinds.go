package export

import "github.com/Open-MBEE/OpenSysML/internal/core/ast"

// The tables below are the single source of truth for the correspondence
// between a SysML declaration keyword, the AST kind the parser produced for it,
// and the metaclass name written into RDF. Conversion runs in both directions,
// so keeping one table per direction in sync by hand would drift: the reverse
// maps are derived here instead.

// definitionMetaclass maps a definition kind to its SysML metaclass name.
var definitionMetaclass = map[ast.DefinitionKind]string{
	ast.DefPart:             "PartDefinition",
	ast.DefAttribute:        "AttributeDefinition",
	ast.DefItem:             "ItemDefinition",
	ast.DefOccurrence:       "OccurrenceDefinition",
	ast.DefIndividual:       "IndividualDefinition",
	ast.DefMetaclass:        "Metaclass",
	ast.DefMetadata:         "MetadataDefinition",
	ast.DefEnumeration:      "EnumerationDefinition",
	ast.DefView:             "ViewDefinition",
	ast.DefViewpoint:        "ViewpointDefinition",
	ast.DefRendering:        "RenderingDefinition",
	ast.DefConcern:          "ConcernDefinition",
	ast.DefConnection:       "ConnectionDefinition",
	ast.DefFlow:             "FlowDefinition",
	ast.DefPort:             "PortDefinition",
	ast.DefInterface:        "InterfaceDefinition",
	ast.DefAllocation:       "AllocationDefinition",
	ast.DefBinding:          "BindingConnectorDefinition",
	ast.DefAction:           "ActionDefinition",
	ast.DefState:            "StateDefinition",
	ast.DefCalc:             "CalculationDefinition",
	ast.DefConstraint:       "ConstraintDefinition",
	ast.DefRequirement:      "RequirementDefinition",
	ast.DefCase:             "CaseDefinition",
	ast.DefAnalysisCase:     "AnalysisCaseDefinition",
	ast.DefVerificationCase: "VerificationCaseDefinition",
	ast.DefUseCase:          "UseCaseDefinition",
	ast.DefBehavior:         "Behavior",
	ast.DefAssoc:            "Association",
	ast.DefStruct:           "Structure",
	ast.DefClass:            "Class",
	ast.DefPredicate:        "Predicate",
	ast.DefBool:             "BooleanExpression",
}

// usageMetaclass maps a usage kind to its SysML metaclass name.
var usageMetaclass = map[ast.UsageKind]string{
	ast.UsagePart:             "PartUsage",
	ast.UsageAttribute:        "AttributeUsage",
	ast.UsageItem:             "ItemUsage",
	ast.UsageOccurrence:       "OccurrenceUsage",
	ast.UsageIndividual:       "IndividualUsage",
	ast.UsageMetadata:         "MetadataUsage",
	ast.UsageEnumeration:      "EnumerationUsage",
	ast.UsageView:             "ViewUsage",
	ast.UsageViewpoint:        "ViewpointUsage",
	ast.UsageRendering:        "RenderingUsage",
	ast.UsageViewRendering:    "ViewRenderingMembership",
	ast.UsageConcern:          "ConcernUsage",
	ast.UsageFramedConcern:    "FramedConcernMembership",
	ast.UsageConnection:       "ConnectionUsage",
	ast.UsageConnector:        "ConnectorAsUsage",
	ast.UsageSuccession:       "SuccessionAsUsage",
	ast.UsageFlow:             "FlowUsage",
	ast.UsagePort:             "PortUsage",
	ast.UsageInterface:        "InterfaceUsage",
	ast.UsageInteraction:      "InteractionUsage",
	ast.UsageAllocation:       "AllocationUsage",
	ast.UsageBinding:          "BindingConnectorAsUsage",
	ast.UsageAction:           "ActionUsage",
	ast.UsageState:            "StateUsage",
	ast.UsageTransition:       "TransitionUsage",
	ast.UsageStep:             "Step",
	ast.UsageCalc:             "CalculationUsage",
	ast.UsageExpr:             "Expression",
	ast.UsageConstraint:       "ConstraintUsage",
	ast.UsageRequirement:      "RequirementUsage",
	ast.UsageSatisfy:          "SatisfyRequirementUsage",
	ast.UsageSubject:          "SubjectMembership",
	ast.UsageActor:            "ActorMembership",
	ast.UsageStakeholder:      "StakeholderMembership",
	ast.UsageObjective:        "ObjectiveMembership",
	ast.UsageCase:             "CaseUsage",
	ast.UsageAnalysisCase:     "AnalysisCaseUsage",
	ast.UsageVerificationCase: "VerificationCaseUsage",
	ast.UsageUseCase:          "UseCaseUsage",
	ast.UsageBehavior:         "BehaviorUsage",
	ast.UsageAssoc:            "AssociationUsage",
	ast.UsageStruct:           "StructureUsage",
	ast.UsageClass:            "ClassUsage",
	ast.UsagePredicate:        "PredicateUsage",
	ast.UsageBool:             "BooleanUsage",
}

// definitionKeyword and usageKeyword give the source keyword for a kind. The
// AST's own String() is the keyword for every kind, which is what makes the
// printer able to reconstruct a declaration head from the metaclass alone.
func definitionKeyword(kind ast.DefinitionKind) string { return kind.String() }
func usageKeyword(kind ast.UsageKind) string           { return kind.String() }

// memberDeclarationKeyword gives the kind keyword a member usage states after
// its own keyword when it declares an element rather than referencing one, or
// "" for a kind with no such form: `render rendering r : AsTree` declares a
// rendering where `render r` names one (SysML.xtext ViewRenderingUsage,
// FramedConcernUsage).
func memberDeclarationKeyword(kind ast.UsageKind) string {
	switch kind {
	case ast.UsageViewRendering:
		return "rendering"
	case ast.UsageFramedConcern:
		return "concern"
	}
	return ""
}

// relationshipProperty maps a declaration-head relationship to its RDF
// predicate name in the SysML vocabulary.
var relationshipProperty = map[ast.RelationshipKind]string{
	ast.RelTyping:      "type",
	ast.RelSpecializes: "specializes",
	ast.RelSubsets:     "subsets",
	ast.RelRedefines:   "redefines",
	ast.RelReferences:  "references",
	ast.RelCrosses:     "crosses",
	ast.RelDisjoint:    "disjointFrom",
	ast.RelIntersects:  "intersects",
	ast.RelInverseOf:   "inverseOf",
	ast.RelUnions:      "unions",
	ast.RelChains:      "chains",
	ast.RelIncludes:    "includes",
	ast.RelVia:         "via",
	ast.RelAnnotates:   "annotates",
	ast.RelSubject:     "subject",
}

// relationshipSyntax gives the source syntax that introduces a relationship
// when a declaration head is rebuilt from RDF.
var relationshipSyntax = map[ast.RelationshipKind]string{
	ast.RelTyping:      ":",
	ast.RelSpecializes: "specializes",
	ast.RelSubsets:     "subsets",
	ast.RelRedefines:   "redefines",
	ast.RelReferences:  "references",
	ast.RelCrosses:     "crosses",
	ast.RelDisjoint:    "disjoint from",
	ast.RelIntersects:  "intersects",
	ast.RelInverseOf:   "inverse of",
	ast.RelUnions:      "unions",
	ast.RelChains:      "chains",
	ast.RelIncludes:    "includes",
	ast.RelVia:         "via",
	ast.RelAnnotates:   "about",
	ast.RelSubject:     "by",
}

// Reverse lookups, derived once so the two directions cannot disagree.
var (
	metaclassDefinition = map[string]ast.DefinitionKind{}
	metaclassUsage      = map[string]ast.UsageKind{}
)

func init() {
	for kind, name := range definitionMetaclass {
		metaclassDefinition[name] = kind
	}
	for kind, name := range usageMetaclass {
		metaclassUsage[name] = kind
	}
}

// relationshipOrder is the order relationships are written back into a
// declaration head; typing comes first because it is the ':' clause.
var relationshipOrder = []ast.RelationshipKind{
	ast.RelTyping,
	ast.RelSpecializes,
	ast.RelSubsets,
	ast.RelRedefines,
	ast.RelReferences,
	ast.RelCrosses,
	ast.RelDisjoint,
	ast.RelIntersects,
	ast.RelInverseOf,
	ast.RelUnions,
	ast.RelChains,
	ast.RelIncludes,
	ast.RelVia,
	ast.RelAnnotates,
	ast.RelSubject,
}

// visibilityKeyword renders a declared visibility, or "" for the default.
func visibilityKeyword(v ast.Visibility) string {
	switch v {
	case ast.VisibilityPublic:
		return "public"
	case ast.VisibilityPrivate:
		return "private"
	case ast.VisibilityProtected:
		return "protected"
	}
	return ""
}

// visibilityOf reverses visibilityKeyword.
func visibilityOf(keyword string) ast.Visibility {
	switch keyword {
	case "public":
		return ast.VisibilityPublic
	case "private":
		return ast.VisibilityPrivate
	case "protected":
		return ast.VisibilityProtected
	}
	return ast.VisibilityDefault
}

// directionKeyword renders a feature direction, or "" for none.
func directionKeyword(d ast.FeatureDirection) string {
	if d == ast.DirNone {
		return ""
	}
	return d.String()
}
