package query

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

var metamodelTypeNames = map[symbols.SymbolKind]string{
	symbols.SymbolPackage: "Package", symbols.SymbolNamespace: "Namespace",
	symbols.SymbolAlias: "Membership", symbols.SymbolDependency: "Dependency",
	symbols.SymbolRelationship: "Relationship", symbols.SymbolComment: "Comment",
	symbols.SymbolDocumentation:         "Documentation",
	symbols.SymbolTextualRepresentation: "TextualRepresentation",
	symbols.SymbolPartDef:               "PartDefinition", symbols.SymbolAttributeDef: "AttributeDefinition",
	symbols.SymbolPartUsage: "PartUsage", symbols.SymbolAttributeUsage: "AttributeUsage",
	symbols.SymbolItemDef: "ItemDefinition", symbols.SymbolOccurrenceDef: "OccurrenceDefinition",
	symbols.SymbolIndividualDef: "OccurrenceDefinition", symbols.SymbolMetadataDef: "MetadataDefinition",
	symbols.SymbolMetaclass: "Metaclass", symbols.SymbolEnumerationDef: "EnumerationDefinition",
	symbols.SymbolViewDef: "ViewDefinition", symbols.SymbolViewpointDef: "ViewpointDefinition",
	symbols.SymbolRenderingDef: "RenderingDefinition", symbols.SymbolConcernDef: "ConcernDefinition",
	symbols.SymbolConnectionDef: "ConnectionDefinition", symbols.SymbolFlowDef: "FlowDefinition",
	symbols.SymbolPortDef: "PortDefinition", symbols.SymbolInterfaceDef: "InterfaceDefinition",
	symbols.SymbolAllocationDef: "AllocationDefinition", symbols.SymbolActionDef: "ActionDefinition",
	symbols.SymbolStateDef: "StateDefinition", symbols.SymbolCalcDef: "CalculationDefinition",
	symbols.SymbolConstraintDef: "ConstraintDefinition", symbols.SymbolRequirementDef: "RequirementDefinition",
	symbols.SymbolCaseDef: "CaseDefinition", symbols.SymbolAnalysisCaseDef: "AnalysisCaseDefinition",
	symbols.SymbolVerificationCaseDef: "VerificationCaseDefinition", symbols.SymbolUseCaseDef: "UseCaseDefinition",
	symbols.SymbolItemUsage: "ItemUsage", symbols.SymbolOccurrenceUsage: "OccurrenceUsage",
	symbols.SymbolIndividualUsage: "OccurrenceUsage", symbols.SymbolMetadataUsage: "MetadataUsage",
	symbols.SymbolEnumerationUsage: "EnumerationUsage", symbols.SymbolViewUsage: "ViewUsage",
	symbols.SymbolViewpointUsage: "ViewpointUsage", symbols.SymbolRenderingUsage: "RenderingUsage",
	symbols.SymbolConcernUsage: "ConcernUsage", symbols.SymbolConnectionUsage: "ConnectionUsage",
	symbols.SymbolSuccessionUsage: "SuccessionAsUsage", symbols.SymbolFlowUsage: "FlowUsage",
	symbols.SymbolPortUsage: "PortUsage", symbols.SymbolInterfaceUsage: "InterfaceUsage",
	symbols.SymbolAllocationUsage: "AllocationUsage", symbols.SymbolActionUsage: "ActionUsage",
	symbols.SymbolStateUsage: "StateUsage", symbols.SymbolCalcUsage: "CalculationUsage",
	symbols.SymbolConstraintUsage: "ConstraintUsage", symbols.SymbolRequirementUsage: "RequirementUsage",
	symbols.SymbolCaseUsage: "CaseUsage", symbols.SymbolAnalysisCaseUsage: "AnalysisCaseUsage",
	symbols.SymbolVerificationCaseUsage: "VerificationCaseUsage", symbols.SymbolUseCaseUsage: "UseCaseUsage",
	symbols.SymbolConnectorEnd:            "ReferenceUsage",
	symbols.SymbolSatisfyRequirementUsage: "SatisfyRequirementUsage",
}

var kermlTypeNames = map[ast.DefinitionKind]string{
	ast.DefClass: "Class", ast.DefStruct: "Structure", ast.DefAssoc: "Association",
	ast.DefBehavior: "Behavior", ast.DefPredicate: "Predicate",
}

var kermlUsageTypeNames = map[ast.UsageKind]string{
	ast.UsageClass: "Class", ast.UsageStruct: "Structure", ast.UsageAssoc: "Association",
	ast.UsageBehavior: "Behavior", ast.UsagePredicate: "Predicate",
	ast.UsageInteraction: "Interaction",
}

// MetamodelTypeNameOf returns the metamodel type name an element reports as
// @type, refining symbol kinds that span several metaclasses.
func MetamodelTypeNameOf(sym *symbols.Symbol) string {
	if sym == nil {
		return ""
	}
	switch sym.Kind {
	case symbols.SymbolConnectorEnd:
		if sym.OwnerScope != nil {
			if usage, ok := sym.OwnerScope.Node().(*ast.Usage); ok && usage.Kind == ast.UsageInterface {
				return "PortUsage"
			}
		}
	case symbols.SymbolKerMLType:
		switch decl := sym.Decl.(type) {
		case *ast.Definition:
			return kermlTypeNames[decl.Kind]
		case *ast.Usage:
			return kermlUsageTypeNames[decl.Kind]
		default:
			return ""
		}
	}
	return metamodelTypeNames[sym.Kind]
}

// MetamodelTypeName returns the metamodel type associated with a symbol kind.
func MetamodelTypeName(kind symbols.SymbolKind) string {
	return metamodelTypeNames[kind]
}
