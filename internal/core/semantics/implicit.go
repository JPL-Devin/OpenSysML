package semantics

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// implicitUsageBases maps a usage kind to the qualified name of the standard
// library definition that every usage of that kind is implicitly typed by
// (SysML v2 §7: each usage specializes the base feature of its kind, which is
// itself typed by the base definition listed here). A usage that declares no
// type or specialization of its own gets this base, so members inherited from
// it — `done` on a state, `startShot` on an action — resolve through it.
var implicitUsageBases = map[ast.UsageKind]string{
	ast.UsagePart:             "Parts::Part",
	ast.UsageAttribute:        "Base::DataValue",
	ast.UsageEnumeration:      "Base::DataValue",
	ast.UsageItem:             "Items::Item",
	ast.UsageOccurrence:       "Occurrences::Occurrence",
	ast.UsageIndividual:       "Occurrences::Life",
	ast.UsageMetadata:         "Metadata::MetadataItem",
	ast.UsageView:             "Views::View",
	ast.UsageViewpoint:        "Views::ViewpointCheck",
	ast.UsageRendering:        "Views::Rendering",
	ast.UsageConcern:          "Requirements::ConcernCheck",
	ast.UsageConnection:       "Connections::Connection",
	ast.UsagePort:             "Ports::Port",
	ast.UsageInterface:        "Interfaces::Interface",
	ast.UsageAllocation:       "Allocations::Allocation",
	ast.UsageAction:           "Actions::Action",
	ast.UsageState:            "States::StateAction",
	ast.UsageTransition:       "Actions::TransitionAction",
	ast.UsageStep:             "Performances::Performance",
	ast.UsageCalc:             "Calculations::Calculation",
	ast.UsageExpr:             "Performances::Evaluation",
	ast.UsageConstraint:       "Constraints::ConstraintCheck",
	ast.UsageRequirement:      "Requirements::RequirementCheck",
	ast.UsageCase:             "Cases::Case",
	ast.UsageAnalysisCase:     "AnalysisCases::AnalysisCase",
	ast.UsageVerificationCase: "VerificationCases::VerificationCase",
	ast.UsageUseCase:          "UseCases::UseCase",
}

// implicitDefinitionBases maps a definition kind to the qualified name of the
// standard library definition every definition of that kind implicitly
// specializes: the base metadata definition is MetadataItem, which supplies
// `annotatedElement` from Metaobjects::Metaobject (SysML v2 §7.27.2, [KerML,
// 9.2.17]).
var implicitDefinitionBases = map[ast.DefinitionKind]string{
	ast.DefMetadata: "Metadata::MetadataItem",
}

// implicitBase returns the stdlib definition sym is implicitly typed by, or nil
// when sym is not an untyped usage or definition of a kind with a known base. A
// declaration that declares any generalization (typing, subsetting,
// redefinition, specialization) takes its supertypes from that declaration
// instead.
func (m *Model) implicitBase(sym *symbols.Symbol) *symbols.Symbol {
	var fqn string
	var ok bool
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		if declaresGeneralization(d.Relationships) {
			return nil
		}
		fqn, ok = implicitUsageBases[d.Kind]
		if d.IsIndividual && d.Kind == ast.UsageOccurrence {
			// An individual occurrence is a life, not an arbitrary occurrence
			// (SysML v2 §7.9.4), however the modifier is spelled.
			fqn, ok = implicitUsageBases[ast.UsageIndividual]
		}
	case *ast.Definition:
		if declaresGeneralization(d.Relationships) {
			return nil
		}
		fqn, ok = implicitDefinitionBases[d.Kind]
	case *ast.SubstateMember:
		// `state s;` in a state body: a bodyless, always-untyped state usage.
		fqn, ok = implicitUsageBases[ast.UsageState]
	default:
		return nil
	}
	if !ok || m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	for _, base := range m.resolver.Index().LookupQualified(fqn) {
		if base != nil && base != sym {
			return base
		}
	}
	return nil
}

// declaresGeneralization reports whether rels contain a conformance edge, which
// makes a declaration take its supertypes from the declaration itself.
func declaresGeneralization(rels []*ast.Relationship) bool {
	for _, rel := range rels {
		if rel != nil && GeneralizationKind(rel.Kind) {
			return true
		}
	}
	return false
}
