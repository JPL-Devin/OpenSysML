package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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
	ast.UsageViewRendering:    "Views::Rendering",
	ast.UsageConcern:          "Requirements::ConcernCheck",
	ast.UsageFramedConcern:    "Requirements::ConcernCheck",
	ast.UsageActor:            "Parts::Part",
	ast.UsageStakeholder:      "Parts::Part",
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
// specializes, so the members that base supplies resolve inside the
// definition's body: MetadataItem supplies `annotatedElement` from
// Metaobjects::Metaobject (SysML v2 §7.27.2, [KerML, 9.2.17]), and Parts::Part
// supplies `start` and `done` inside every `part def`.
var implicitDefinitionBases = map[ast.DefinitionKind]string{
	ast.DefPart:        "Parts::Part",
	ast.DefAttribute:   "Base::DataValue",
	ast.DefEnumeration: "Base::DataValue",
	ast.DefItem:        "Items::Item",
	ast.DefOccurrence:  "Occurrences::Occurrence",
	ast.DefIndividual:  "Occurrences::Life",
	ast.DefMetaclass:   "Metaobjects::Metaobject",
	ast.DefMetadata:    "Metadata::MetadataItem",
	ast.DefView:        "Views::View",
	ast.DefViewpoint:   "Views::ViewpointCheck",
	ast.DefRendering:   "Views::Rendering",
	ast.DefConcern:     "Requirements::ConcernCheck",
	ast.DefConnection:  "Connections::Connection",
	ast.DefFlow:        "Flows::Flow",
	ast.DefPort:        "Ports::Port",
	ast.DefInterface:   "Interfaces::Interface",
	ast.DefAllocation:  "Allocations::Allocation",
	// A behavior definition specializes the base behavior of its kind, which is
	// what makes an occurrence's own features — `self`, `start`, `done` — visible
	// inside the definition's body the same way they are inside a usage's
	// (SysML v2 §7.16.2, §7.17.2).
	ast.DefAction:           "Actions::Action",
	ast.DefState:            "States::StateAction",
	ast.DefCalc:             "Calculations::Calculation",
	ast.DefConstraint:       "Constraints::ConstraintCheck",
	ast.DefRequirement:      "Requirements::RequirementCheck",
	ast.DefCase:             "Cases::Case",
	ast.DefAnalysisCase:     "AnalysisCases::AnalysisCase",
	ast.DefVerificationCase: "VerificationCases::VerificationCase",
	ast.DefUseCase:          "UseCases::UseCase",
}

var implicitKerMLBases = map[string]string{
	"classifier":  "Base::Anything",
	"class":       "Occurrences::Occurrence",
	"struct":      "Objects::Object",
	"assoc":       "Links::Link",
	"association": "Links::Link",
	"behavior":    "Performances::Performance",
	"function":    "Performances::Evaluation",
	"predicate":   "Performances::BooleanEvaluation",
	"interaction": "Transfers::Transfer",
	"metaclass":   "Metaobjects::Metaobject",
	"datatype":    "Base::DataValue",
	"type":        "Base::Anything",
}

// isKerMLDoc reports whether sym is declared by a KerML document, as recorded
// by the index rather than inferred from the document name.
func (m *Model) isKerMLDoc(sym *symbols.Symbol) bool {
	if m.resolver == nil || m.resolver.Index() == nil {
		return false
	}
	return m.resolver.Index().DocumentKind(sym.DocName) == source.KindKerML
}

// implicitBase returns the stdlib definition sym is implicitly typed by, or nil
// when sym is not a declaration of a kind with a known base.
func (m *Model) implicitBase(sym *symbols.Symbol) *symbols.Symbol {
	isKerML := m.isKerMLDoc(sym)
	var fqn string
	var ok bool
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		fqn, ok = implicitUsageBases[d.Kind]
		if d.IsIndividual && d.Kind == ast.UsageOccurrence {
			// An individual occurrence is a life, not an arbitrary occurrence
			// (SysML v2 §7.9.4), however the modifier is spelled.
			fqn, ok = implicitUsageBases[ast.UsageIndividual]
		}
	case *ast.Definition:
		fqn, ok = implicitDefinitionBases[d.Kind]
	case *ast.SubstateMember:
		// `state s;` in a state body: a bodyless, always-untyped state usage.
		fqn, ok = implicitUsageBases[ast.UsageState]
	default:
		return nil
	}
	if isKerML {
		switch d := sym.Decl.(type) {
		case *ast.Usage:
			fqn, ok = implicitKerMLBases[d.Keyword]
		case *ast.Definition:
			fqn, ok = implicitKerMLBases[d.Keyword]
		}
	} else if declaresGeneralization(RelationshipsOf(sym)) {
		return nil
	}
	if isKerML && m.declaredGeneralizationReaches(sym, fqn, nil) {
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

func (m *Model) declaredGeneralizationReaches(sym *symbols.Symbol, want string, visiting map[*symbols.Symbol]bool) bool {
	if sym == nil {
		return false
	}
	if visiting == nil {
		visiting = make(map[*symbols.Symbol]bool)
	}
	if visiting[sym] {
		return false
	}
	visiting[sym] = true
	defer delete(visiting, sym)

	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || !GeneralizationKind(rel.Kind) {
			continue
		}
		targetNode := rel.Target
		if fr, ok := targetNode.(*ast.FeatureReference); ok {
			targetNode = fr.Name
		}
		qn, ok := targetNode.(*ast.QualifiedName)
		if !ok {
			continue
		}
		target, ok := m.resolver.ResolveQualified(sym.OwnerScope, qn)
		if !ok || target == nil {
			continue
		}
		if resolved, aliasOK := m.resolver.ResolveAliasTarget(target); aliasOK {
			target = resolved
		} else {
			continue
		}
		sameBase := m.resolver.Index() != nil && m.resolver.Index().GetFQN(target) == want
		if sameBase || m.declaredGeneralizationReaches(target, want, visiting) {
			return true
		}
	}
	return false
}

// baseUsageFQN is the most general base usage every usage element subsets,
// directly or indirectly (SysML v2 §7.6, [KerML, 8.4.2]). Its only member is
// `that`, the featuring instance of a usage's value, so subsetting it is what
// makes an unqualified `that` resolve inside a usage body.
const baseUsageFQN = "Base::things"

// implicitBaseUsage returns Base::things for a usage element, or nil when sym is
// not a usage, is that base usage, or is owned by it.
func (m *Model) implicitBaseUsage(sym *symbols.Symbol) *symbols.Symbol {
	if _, ok := sym.Decl.(*ast.Usage); !ok {
		return nil
	}
	if m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	for _, base := range m.resolver.Index().LookupQualified(baseUsageFQN) {
		if base == nil || base == sym || enclosedBy(sym, base) {
			continue
		}
		return base
	}
	return nil
}

// enclosedBy reports whether owner's own scope encloses sym.
func enclosedBy(sym, owner *symbols.Symbol) bool {
	if owner.Scope == nil {
		return false
	}
	for s := sym.OwnerScope; s != nil; s = s.Parent() {
		if s == owner.Scope {
			return true
		}
	}
	return false
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
