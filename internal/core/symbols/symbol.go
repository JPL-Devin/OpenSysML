// Package symbols builds an immutable per-document scope tree and a global
// qualified-name index over a parsed ast.RootNamespace.
package symbols

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
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
	// SymbolKerMLType classifies a KerML type declaration — `class`,
	// `classifier`, `struct`, `assoc`, `behavior`, `predicate` — which the SysML
	// definition taxonomy has no counterpart for.
	SymbolKerMLType
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
	SymbolKerMLType:             "kermlType",
}

// String returns the display name of the kind.
func (k SymbolKind) String() string {
	if s, ok := symbolKindNames[k]; ok {
		return s
	}
	return "unknown"
}

// UnitFacts is a measurement unit reduced to a scale factor over base units,
// each named by its qualified name. It is what a cached library symbol carries
// in place of the declaration the reduction was computed from.
type UnitFacts struct {
	ScaleNum float64
	ScaleDen float64
	Factors  []UnitFactorFacts

	// Irreducible marks a measurement unit whose reduction is not derivable
	// from its declaration, so that it is still known to be a unit.
	Irreducible bool
}

// UnitFactorFacts is one base unit of a reduced measurement unit, named by its
// qualified name, and its exponent.
type UnitFactorFacts struct {
	FQN      string
	Exponent float64
}

// DimensionFacts is a quantity dimension as a cached library symbol carries it:
// the base quantities it is a product of powers of, each named by its qualified
// name. Empty factors are the dimension of a count or a ratio of like quantities.
type DimensionFacts struct {
	Factors []DimensionFactorFacts
}

// DimensionFactorFacts is one base quantity of a dimension, named by its
// qualified name, and its exponent.
type DimensionFactorFacts struct {
	FQN      string
	Exponent float64
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

	// SuperFQNs are the fully-qualified names of the specialization targets
	// (specializes/subsets/redefines), populated for cached library symbols
	// where Decl=nil. Empty for live-parsed symbols, which use Decl instead.
	SuperFQNs []string

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

	// Unit is the reduced measurement-unit form persisted for a cached library
	// symbol, whose declaration is absent: without it the conversion a unit
	// declares could not be read back. Nil for a symbol that is not a
	// measurement unit, and for live-parsed symbols, which use Decl instead.
	Unit *UnitFacts

	// Dimension is the quantity dimension persisted for a cached library symbol,
	// whose declaration is absent: the power factors a unit definition states are
	// declared members with bound values, which the declaration takes with it.
	// Nil for a symbol whose dimension is not determined, and for live-parsed
	// symbols, which are read from Decl instead.
	Dimension *DimensionFacts

	// NamingTarget is the reference that named this symbol when EffectiveName
	// is set: the target of its reference subsetting or redefinition. Resolving
	// that reference must not see the name it gave away, or it would resolve to
	// the feature that borrowed it (KerML 7.3.4.5).
	NamingTarget ast.Node

	// Annotations are the metadata annotations of a cached library symbol,
	// whose declaration is absent: an element filter classifies a candidate by
	// the metadata annotating it, so a restored symbol has to carry what its
	// declaration would have said. Empty for live-parsed symbols, which are read
	// from Decl instead.
	Annotations []AnnotationFacts
}

// AnnotationFacts is one metadata annotation of a cached library symbol: the
// fully-qualified name of the metadata type annotating it, and the values the
// annotation body binds its features to, as written.
type AnnotationFacts struct {
	TypeFQN string
	Values  []AnnotationValueFacts
}

// AnnotationValueFacts is one feature binding inside an annotation body
// (`@Safety{isMandatory = true;}`), holding the value already evaluated, so that
// a filter condition reading it decides the same way where the declaration it
// came from is gone.
type AnnotationValueFacts struct {
	Feature string
	Value   FilterValue
}
