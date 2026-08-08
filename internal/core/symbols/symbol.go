// Package symbols builds an immutable per-document scope tree and a global
// qualified-name index over a parsed ast.RootNamespace.
package symbols

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// SymbolKind classifies a declared name.
type SymbolKind int

const (
	SymbolUnknown SymbolKind = iota
	SymbolPackage
	SymbolNamespace
	SymbolAlias
	SymbolDependency
	SymbolComment
	SymbolDocumentation
	SymbolTextualRepresentation
	SymbolPartDef
	SymbolAttributeDef
	SymbolPartUsage
	SymbolAttributeUsage
	// Tier A definitions.
	SymbolItemDef
	SymbolOccurrenceDef
	SymbolIndividualDef
	SymbolMetadataDef
	SymbolMetaclass // KerML metaclass (similar to metadata def)
	SymbolEnumerationDef
	SymbolViewDef
	SymbolViewpointDef
	SymbolRenderingDef
	SymbolConcernDef
	// Tier B definitions.
	SymbolConnectionDef
	SymbolFlowDef
	SymbolPortDef
	SymbolInterfaceDef
	SymbolAllocationDef
	// Tier C definitions.
	SymbolActionDef
	SymbolStateDef
	SymbolCalcDef
	SymbolConstraintDef
	SymbolRequirementDef
	SymbolCaseDef
	SymbolAnalysisCaseDef
	SymbolVerificationCaseDef
	SymbolUseCaseDef
	// Tier A usages.
	SymbolItemUsage
	SymbolOccurrenceUsage
	SymbolIndividualUsage
	SymbolMetadataUsage
	SymbolEnumerationUsage
	SymbolViewUsage
	SymbolViewpointUsage
	SymbolRenderingUsage
	SymbolConcernUsage
	// Tier B usages.
	SymbolConnectionUsage
	SymbolFlowUsage
	SymbolPortUsage
	SymbolInterfaceUsage
	SymbolAllocationUsage
	// Tier C usages.
	SymbolActionUsage
	SymbolStateUsage
	SymbolCalcUsage
	SymbolConstraintUsage
	SymbolRequirementUsage
	SymbolCaseUsage
	SymbolAnalysisCaseUsage
	SymbolVerificationCaseUsage
	SymbolUseCaseUsage
	// SymbolConnectorEnd is an end feature a connector usage declares in its
	// connect clause (`connect bead references t.bead`).
	SymbolConnectorEnd
)

var symbolKindNames = map[SymbolKind]string{
	SymbolUnknown:               "unknown",
	SymbolPackage:               "package",
	SymbolNamespace:             "namespace",
	SymbolAlias:                 "alias",
	SymbolDependency:            "dependency",
	SymbolComment:               "comment",
	SymbolDocumentation:         "documentation",
	SymbolTextualRepresentation: "textualRepresentation",
	SymbolPartDef:               "partDef",
	SymbolAttributeDef:          "attributeDef",
	SymbolPartUsage:             "partUsage",
	SymbolAttributeUsage:        "attributeUsage",
	SymbolItemDef:               "itemDef",
	SymbolOccurrenceDef:         "occurrenceDef",
	SymbolIndividualDef:         "individualDef",
	SymbolMetadataDef:           "metadataDef",
	SymbolMetaclass:             "metaclass",
	SymbolEnumerationDef:        "enumDef",
	SymbolViewDef:               "viewDef",
	SymbolViewpointDef:          "viewpointDef",
	SymbolRenderingDef:          "renderingDef",
	SymbolConcernDef:            "concernDef",
	SymbolConnectionDef:         "connectionDef",
	SymbolFlowDef:               "flowDef",
	SymbolPortDef:               "portDef",
	SymbolInterfaceDef:          "interfaceDef",
	SymbolAllocationDef:         "allocationDef",
	SymbolActionDef:             "actionDef",
	SymbolStateDef:              "stateDef",
	SymbolCalcDef:               "calcDef",
	SymbolConstraintDef:         "constraintDef",
	SymbolRequirementDef:        "requirementDef",
	SymbolCaseDef:               "caseDef",
	SymbolAnalysisCaseDef:       "analysisCaseDef",
	SymbolVerificationCaseDef:   "verificationCaseDef",
	SymbolUseCaseDef:            "useCaseDef",
	SymbolItemUsage:             "itemUsage",
	SymbolOccurrenceUsage:       "occurrenceUsage",
	SymbolIndividualUsage:       "individualUsage",
	SymbolMetadataUsage:         "metadataUsage",
	SymbolEnumerationUsage:      "enumUsage",
	SymbolViewUsage:             "viewUsage",
	SymbolViewpointUsage:        "viewpointUsage",
	SymbolRenderingUsage:        "renderingUsage",
	SymbolConcernUsage:          "concernUsage",
	SymbolConnectionUsage:       "connectionUsage",
	SymbolFlowUsage:             "flowUsage",
	SymbolPortUsage:             "portUsage",
	SymbolInterfaceUsage:        "interfaceUsage",
	SymbolAllocationUsage:       "allocationUsage",
	SymbolActionUsage:           "actionUsage",
	SymbolStateUsage:            "stateUsage",
	SymbolCalcUsage:             "calcUsage",
	SymbolConstraintUsage:       "constraintUsage",
	SymbolRequirementUsage:      "requirementUsage",
	SymbolCaseUsage:             "caseUsage",
	SymbolAnalysisCaseUsage:     "analysisCaseUsage",
	SymbolVerificationCaseUsage: "verificationCaseUsage",
	SymbolUseCaseUsage:          "useCaseUsage",
	SymbolConnectorEnd:          "connectorEnd",
}

// String returns the display name of the kind.
func (k SymbolKind) String() string {
	if s, ok := symbolKindNames[k]; ok {
		return s
	}
	return "unknown"
}

// Symbol describes one declared name. The same declaration may be reachable
// through more than one Symbol only when it declares both a short and a
// primary name; in that case a single Symbol is registered under both keys.
type Symbol struct {
	Name       string         // the key this Symbol was primarily created for
	Kind       SymbolKind     // classification
	Decl       ast.Node       // the declaring AST node
	Visibility ast.Visibility // declared visibility
	DeclSpan   source.Span    // span of the declaration (for diagnostics)
	NameSpan   source.Span    // span of the declared identifier alone (for rename); zero when unknown
	Scope      *Scope         // the child scope this declaration owns, or nil for leaves
	OwnerScope *Scope         // the enclosing scope this declaration was declared in

	// LeadingTrivia is the comment/note trivia attached to the member wrapper
	// preceding this declaration (captured before unwrap, since wrappers carry
	// the trivia while the unwrapped inner Decl does not). Used for doc hover.
	LeadingTrivia []ast.Trivia

	DocName string // name of the document that declares this symbol (stamped after Build)

	// AliasTargetFQN is the raw qualified name text of the alias target
	// ("alias X for Y" → "Y"), populated for cached stdlib aliases where Decl=nil.
	// Empty for non-aliases or live-parsed aliases (which use Decl instead).
	AliasTargetFQN string

	// ShortName is the short name from Identification (e.g., "kg" for "kilogram").
	// Populated for cached symbols where Decl=nil. Empty if no short name.
	ShortName string

	// EffectiveName reports that Name was taken from the feature this
	// declaration references rather than declared (KerML Feature::effectiveName).
	EffectiveName bool
}
