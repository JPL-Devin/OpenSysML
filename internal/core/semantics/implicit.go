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

// implicitKerMLFeatureBases maps a KerML feature keyword to the base feature
// every feature of that kind subsets (KerML 1.1 §8.4.2).
var implicitKerMLFeatureBases = map[string]string{
	"type":        baseUsageFQN,
	"classifier":  baseUsageFQN,
	"feature":     baseUsageFQN,
	"class":       "Occurrences::occurrences",
	"struct":      "Objects::objects",
	"datatype":    "Base::dataValues",
	"assoc":       "Links::links",
	"association": "Links::links",
	"connector":   "Links::links",
	"binding":     "Links::selfLinks",
	"bind":        "Links::selfLinks",
	"succession":  "Occurrences::happensBeforeLinks",
	"behavior":    "Performances::performances",
	"step":        "Performances::performances",
	"function":    "Performances::evaluations",
	"expr":        "Performances::evaluations",
	"predicate":   "Performances::booleanEvaluations",
	"bool":        "Performances::booleanEvaluations",
	"inv":         "Performances::trueEvaluations",
	"interaction": "Transfers::transfers",
	"flow":        "Transfers::flowTransfers",
	"metaclass":   "Metaobjects::metaobjects",
}

// kindBaseFQN returns the standard-library base every declaration of sym's
// kind conforms to, implicitly or through its declared chain.
func kindBaseFQN(sym *symbols.Symbol, isKerML bool) (string, bool) {
	if sym == nil {
		return "", false
	}
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		if isKerML {
			fqn, ok := implicitKerMLBases[d.Keyword]
			return fqn, ok
		}
		if d.IsIndividual && d.Kind == ast.UsageOccurrence {
			// An individual occurrence is a life, not an arbitrary occurrence
			// (SysML v2 §7.9.4), however the modifier is spelled.
			fqn, ok := implicitUsageBases[ast.UsageIndividual]
			return fqn, ok
		}
		fqn, ok := implicitUsageBases[d.Kind]
		return fqn, ok
	case *ast.Definition:
		if isKerML {
			fqn, ok := implicitKerMLBases[d.Keyword]
			return fqn, ok
		}
		fqn, ok := implicitDefinitionBases[d.Kind]
		return fqn, ok
	case *ast.SubstateMember:
		// `state s;` in a state body: a bodyless, always-untyped state usage.
		fqn, ok := implicitUsageBases[ast.UsageState]
		return fqn, ok
	case *ast.TransitionMember:
		// A textual transition is a TransitionUsage (SysML v2 §7.19.2), so it
		// gets the same base as `transition` written as a usage.
		fqn, ok := implicitUsageBases[ast.UsageTransition]
		return fqn, ok
	}
	return "", false
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
	fqn, ok := kindBaseFQN(sym, isKerML)
	if !ok || m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	// A declaration keeps its kind's base unless a declared chain already reaches
	// it — the same rule for a usage and in either language (KerML §8.4.2).
	if m.declaredGeneralizationReaches(sym, fqn, nil) {
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
		target := m.relationshipTarget(sym, rel)
		if target == nil {
			continue
		}
		sameBase := m.resolver.Index() != nil && m.resolver.Index().GetFQN(target) == want
		if !sameBase {
			// A declaration conforms to its kind's base whether the edge is
			// declared or implicit, so reaching one of the same kind suffices.
			if base, ok := kindBaseFQN(target, m.isKerMLDoc(target)); ok && base == want {
				sameBase = true
			}
		}
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

// isKerMLTypeDecl reports whether sym declares a KerML type rather than a
// feature — `class`, `struct`, `assoc`, `behavior`, `predicate`, `interaction` —
// which the parser records as a usage node (KerML §8.3).
func isKerMLTypeDecl(sym *symbols.Symbol) bool {
	return sym != nil && sym.Kind == symbols.SymbolKerMLType
}

// implicitKerMLFeatureBase returns the base feature a KerML feature declaration
// subsets (KerML §8.4.2); it contributes members only, like implicitBaseUsage.
func (m *Model) implicitKerMLFeatureBase(sym *symbols.Symbol) *symbols.Symbol {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || m.resolver == nil || m.resolver.Index() == nil || !m.isKerMLDoc(sym) {
		return nil
	}
	// Only a feature subsets a base feature; a type specializes a base type.
	if isKerMLTypeDecl(sym) {
		return nil
	}
	fqn, ok := implicitKerMLFeatureBases[usage.Keyword]
	// A typed feature subsets the base feature of its type's kind, not of its own
	// keyword: `feature f : C` with C a class subsets Occurrences::occurrences.
	if typed, tok := m.declaredTypeFeatureBase(sym); tok {
		fqn, ok = typed, true
	}
	if !ok || m.declaredGeneralizationReaches(sym, fqn, nil) {
		return nil
	}
	for _, base := range m.resolver.Index().LookupQualified(fqn) {
		if base == nil || base == sym || enclosedBy(sym, base) {
			continue
		}
		return base
	}
	return nil
}

// relationshipTarget resolves the element rel names from sym's scope, following
// an alias to what it names.
func (m *Model) relationshipTarget(sym *symbols.Symbol, rel *ast.Relationship) *symbols.Symbol {
	if m.resolver == nil {
		return nil
	}
	node := rel.Target
	if fr, ok := node.(*ast.FeatureReference); ok {
		node = fr.Name
	}
	qn, ok := node.(*ast.QualifiedName)
	if !ok {
		return nil
	}
	target, ok := m.resolver.ResolveQualified(sym.OwnerScope, qn)
	if !ok || target == nil {
		return nil
	}
	resolved, ok := m.resolver.ResolveAliasTarget(target)
	if !ok {
		return nil
	}
	return resolved
}

// declaredTypeFeatureBase returns the base feature implied by the kind of the
// type sym is declared to have, if it declares one that is a KerML type.
func (m *Model) declaredTypeFeatureBase(sym *symbols.Symbol) (string, bool) {
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelTyping {
			continue
		}
		target := m.relationshipTarget(sym, rel)
		if target == nil || !isKerMLTypeDecl(target) {
			continue
		}
		if fqn, ok := implicitKerMLFeatureBases[keywordOf(target)]; ok {
			return fqn, true
		}
	}
	return "", false
}

// keywordOf returns the declaration keyword sym was written with.
func keywordOf(sym *symbols.Symbol) string {
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		return d.Keyword
	case *ast.Definition:
		return d.Keyword
	}
	return ""
}

// implicitBaseUsage returns Base::things for a usage element, or nil when sym is
// not a usage, is that base usage, or is owned by it.
func (m *Model) implicitBaseUsage(sym *symbols.Symbol) *symbols.Symbol {
	if _, ok := sym.Decl.(*ast.Usage); !ok {
		return nil
	}
	// A KerML type declaration is a classifier, not a usage of one.
	if isKerMLTypeDecl(sym) {
		return nil
	}
	if m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	// A KerML feature typed by a kind with its own base subsets that base instead.
	if typed, ok := m.declaredTypeFeatureBase(sym); ok && typed != baseUsageFQN && m.isKerMLDoc(sym) {
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

// ImplicitGenerals returns the general types sym has by its kind rather than by
// declaration. A scope reached through a recursive import does not traverse them
// (KerML 8.2.3.5).
func (m *Model) ImplicitGenerals(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, base := range []*symbols.Symbol{m.implicitBase(sym), m.implicitBaseUsage(sym), m.implicitKerMLFeatureBase(sym)} {
		if base != nil && base != sym {
			out = append(out, base)
		}
	}
	return out
}
