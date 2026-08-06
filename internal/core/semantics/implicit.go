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

// implicitBase returns the stdlib definition sym is implicitly typed by, or nil
// when sym is not an untyped usage of a kind with a known base. A usage that
// declares any generalization (typing, subsetting, redefinition) takes its
// supertypes from that declaration instead.
func (m *Model) implicitBase(sym *symbols.Symbol) *symbols.Symbol {
	var kind ast.UsageKind
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		for _, rel := range d.Relationships {
			if rel != nil && GeneralizationKind(rel.Kind) {
				return nil
			}
		}
		kind = d.Kind
	case *ast.SubstateMember:
		// `state s;` in a state body: a bodyless, always-untyped state usage.
		kind = ast.UsageState
	default:
		return nil
	}
	fqn, ok := implicitUsageBases[kind]
	if !ok || m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	// A usage whose name matches a feature its owner inherits implicitly
	// redefines that feature, which supplies the type instead of the base.
	if sym.Name != "" && m.inheritedFeatureNamed(sym, sym.Name) != nil {
		return nil
	}
	for _, base := range m.resolver.Index().LookupQualified(fqn) {
		if base != nil && base != sym {
			return base
		}
	}
	return nil
}
