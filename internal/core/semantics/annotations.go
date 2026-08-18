package semantics

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// The metadata annotating an element is what an element filter classifies it by
// (`@Safety`), and SysML v2 7.27 lets it be written in three ways, all of which
// count here:
//
//   - prefix metadata on the declaration, `#Safety part def P;` and the
//     `@Safety{...}` form written inside the element's body;
//   - a metadata usage in the element's body, `part p { metadata safety : Safety; }`;
//   - a metadata usage annotating the element from elsewhere,
//     `metadata safety : Safety about p;`.
//
// An element is also classified by its own metaclass — a part usage by
// SysML::PartUsage — which is what the reflective metadata types in the standard
// library name, and what the corpus filters on (`filter @SysML::PartUsage`).
// That is answered by metaclassOf rather than collected here, since it is not a
// declared annotation.

// annotation is one metadata annotation of an element: the metadata type
// annotating it and the values its body binds that type's features to.
//
// A restored library's annotation names its type rather than pointing at it, so
// both are carried: typ where the annotation was read from a declaration, and
// typFQN where it was read from an index-cache record.
type annotation struct {
	typ    *symbols.Symbol
	typFQN string
	values map[string]symbols.FilterValue
}

// annotationsOf returns the metadata annotating sym, memoized: an element filter
// asks for it once per candidate and per import enumeration.
func (m *Model) annotationsOf(sym *symbols.Symbol) []annotation {
	if sym == nil {
		return nil
	}
	if cached, ok := m.annotations[sym]; ok {
		return cached
	}
	// Recorded first so that a value that resolves back to this element cannot
	// re-enter the collection of its own annotations.
	m.annotations[sym] = nil

	var out []annotation
	for _, facts := range sym.Annotations {
		out = append(out, m.annotationFromFacts(facts))
	}
	if sym.Decl != nil {
		out = append(out, m.declaredAnnotations(sym)...)
		out = append(out, m.aboutAnnotations(sym)...)
	}
	m.annotations[sym] = out
	return out
}

// AnnotationFactsOf states the metadata annotating sym in the declaration-free
// form an index cache record carries, so that an element filter classifies a
// restored library element the same way as a parsed one. The values an
// annotation binds are recorded as read; a binding whose value is not constant
// is recorded with an unknown value, which a condition reading it reports as
// unevaluable rather than silently treating as absent.
func (m *Model) AnnotationFactsOf(sym *symbols.Symbol) []symbols.AnnotationFacts {
	var out []symbols.AnnotationFacts
	for _, a := range m.annotationsOf(sym) {
		typFQN := a.typFQN
		if typFQN == "" && a.typ != nil {
			typFQN = m.fqnOf(a.typ)
		}
		if typFQN == "" {
			continue
		}
		facts := symbols.AnnotationFacts{TypeFQN: typFQN}
		for _, feature := range sortedFeatureNames(a.values) {
			facts.Values = append(facts.Values, symbols.AnnotationValueFacts{
				Feature: feature,
				Value:   a.values[feature],
			})
		}
		out = append(out, facts)
	}
	return out
}

// sortedFeatureNames orders an annotation's bound features by name, so that what
// is written to a cache record does not depend on map iteration order.
func sortedFeatureNames(values map[string]symbols.FilterValue) []string {
	out := make([]string, 0, len(values))
	for name := range values {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// annotationFromFacts rebuilds an annotation a cache record carried.
func (m *Model) annotationFromFacts(facts symbols.AnnotationFacts) annotation {
	a := annotation{typFQN: facts.TypeFQN, typ: m.symbolByFQN(facts.TypeFQN), values: map[string]symbols.FilterValue{}}
	for _, v := range facts.Values {
		a.values[v.Feature] = v.Value
	}
	return a
}

// declaredAnnotations returns the annotations sym's own declaration states: its
// prefix metadata, the prefix metadata written among its members, and the
// metadata usages in its body.
func (m *Model) declaredAnnotations(sym *symbols.Symbol) []annotation {
	var prefixes []*ast.PrefixMetadata
	var members []ast.Node
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		prefixes, members = d.Prefixes, d.Members
	case *ast.Usage:
		prefixes, members = d.Prefixes, d.Members
	case *ast.Package:
		prefixes, members = d.Prefixes, d.Members
	case *ast.Namespace:
		prefixes, members = d.Prefixes, d.Members
	default:
		return nil
	}

	scope := sym.OwnerScope
	var out []annotation
	for _, p := range prefixes {
		if a, ok := m.prefixAnnotation(scope, p); ok {
			out = append(out, a)
		}
	}
	for _, member := range members {
		if mem, ok := member.(*ast.Membership); ok {
			member = mem.Member
		}
		switch decl := member.(type) {
		case *ast.PrefixMetadata:
			// `part seatBelt {@Safety{isMandatory = true;}}`: prefix metadata
			// written as a member annotates the element owning the body.
			if a, ok := m.prefixAnnotation(memberScope(sym, scope), decl); ok {
				out = append(out, a)
			}
		case *ast.Usage:
			if decl.Kind != ast.UsageMetadata || annotatesOthers(decl) {
				continue
			}
			if a, ok := m.usageAnnotation(memberScope(sym, scope), decl); ok {
				out = append(out, a)
			}
		}
	}
	return out
}

// memberScope is the scope the members of sym's body resolve names against,
// falling back to the scope sym itself was declared in.
func memberScope(sym *symbols.Symbol, outer *symbols.Scope) *symbols.Scope {
	if sym.Scope != nil {
		return sym.Scope
	}
	return outer
}

// prefixAnnotation reads one prefix-metadata annotation.
func (m *Model) prefixAnnotation(scope *symbols.Scope, p *ast.PrefixMetadata) (annotation, bool) {
	if p == nil || p.Type == nil {
		return annotation{}, false
	}
	typ, ok := m.resolver.ResolveQualified(scope, p.Type)
	if !ok || typ == nil {
		return annotation{}, false
	}
	return m.annotationOfType(typ, scope, p.Body), true
}

// usageAnnotation reads one metadata-usage annotation, whose type is what the
// usage is typed by.
func (m *Model) usageAnnotation(scope *symbols.Scope, u *ast.Usage) (annotation, bool) {
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelTyping {
			continue
		}
		qn, ok := rel.Target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		typ, ok := m.resolver.ResolveQualified(scope, qn)
		if !ok || typ == nil {
			continue
		}
		return m.annotationOfType(typ, bodyScope(u, scope), u.Members), true
	}
	return annotation{}, false
}

// annotationOfType is one annotation of metadata type typ, valued by what its
// body binds plus the defaults typ declares for what the body leaves unbound.
func (m *Model) annotationOfType(typ *symbols.Symbol, scope *symbols.Scope, body []ast.Node) annotation {
	values := m.annotationValues(scope, body)
	m.addTypeDefaults(typ, values)
	return annotation{typ: typ, typFQN: m.fqnOf(typ), values: values}
}

// addTypeDefaults adds the value the metadata type declares for each feature the
// annotation body leaves unbound, since an annotation inherits its type's values.
func (m *Model) addTypeDefaults(typ *symbols.Symbol, values map[string]symbols.FilterValue) {
	for _, member := range m.MembersOf(typ) {
		usage, ok := member.Decl.(*ast.Usage)
		if !ok || usage.Value == nil {
			continue
		}
		name := simpleSymbolName(member)
		if name == "" {
			continue
		}
		if _, bound := values[name]; bound {
			continue
		}
		values[name] = m.annotationValue(member.OwnerScope, usage.Value)
	}
}

// bodyScope is the scope a metadata usage's body resolves names against. The
// usage's own scope is not reachable from its declaration, so the annotation's
// values are read against the scope the usage was declared in, which is the
// scope its type reference resolved in too.
func bodyScope(_ *ast.Usage, declared *symbols.Scope) *symbols.Scope { return declared }

// aboutAnnotations returns the annotations that `metadata m about sym;`
// declarations elsewhere in the workspace state about sym.
func (m *Model) aboutAnnotations(sym *symbols.Symbol) []annotation {
	return m.annotationsAbout()[sym]
}

// annotationsAbout indexes every `about` metadata usage in the workspace by the
// element it annotates. It is built once: an `about` annotation is stated away
// from the element it applies to, so there is no way to it from the element
// itself.
func (m *Model) annotationsAbout() map[*symbols.Symbol][]annotation {
	if m.aboutAnnots != nil {
		return m.aboutAnnots
	}
	m.aboutAnnots = make(map[*symbols.Symbol][]annotation)
	idx := m.resolver.Index()
	if idx == nil {
		return m.aboutAnnots
	}
	seen := make(map[*symbols.Symbol]bool)
	for _, fqn := range idx.FQNs() {
		for _, sym := range idx.LookupQualified(fqn) {
			if sym == nil || seen[sym] || sym.Kind != symbols.SymbolMetadataUsage {
				continue
			}
			seen[sym] = true
			usage, ok := sym.Decl.(*ast.Usage)
			if !ok || !annotatesOthers(usage) {
				continue
			}
			a, ok := m.usageAnnotation(sym.OwnerScope, usage)
			if !ok {
				continue
			}
			for _, target := range m.annotatedElements(sym.OwnerScope, usage) {
				m.aboutAnnots[target] = append(m.aboutAnnots[target], a)
			}
		}
	}
	return m.aboutAnnots
}

// annotatedElements resolves the elements a metadata usage's `about` clause
// names.
func (m *Model) annotatedElements(scope *symbols.Scope, u *ast.Usage) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelAnnotates {
			continue
		}
		qn, ok := rel.Target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		if target, ok := m.resolver.ResolveQualified(scope, qn); ok && target != nil {
			out = append(out, target)
		}
	}
	return out
}

// annotatesOthers reports whether a metadata usage states what it annotates
// (`metadata m about p;`), rather than annotating the element owning it.
func annotatesOthers(u *ast.Usage) bool {
	for _, rel := range u.Relationships {
		if rel != nil && rel.Kind == ast.RelAnnotates {
			return true
		}
	}
	return false
}

// annotationValues reads the feature values an annotation body binds, as in
// `@Safety{isMandatory = true;}`. A binding whose value is not a constant or an
// element reference is recorded with an unknown value, which a condition reading
// it reports as unevaluable rather than treating as absent.
func (m *Model) annotationValues(scope *symbols.Scope, body []ast.Node) map[string]symbols.FilterValue {
	values := make(map[string]symbols.FilterValue)
	for _, member := range body {
		if mem, ok := member.(*ast.Membership); ok {
			member = mem.Member
		}
		usage, ok := member.(*ast.Usage)
		if !ok || usage.Value == nil {
			continue
		}
		name := boundFeatureName(usage)
		if name == "" {
			continue
		}
		values[name] = m.annotationValue(scope, usage.Value)
	}
	return values
}

// boundFeatureName is the annotation feature a body member binds: the name it
// declares, or the feature it redefines (`:>> isMandatory = true`).
func boundFeatureName(u *ast.Usage) string {
	if u.Ident.Name != "" {
		return u.Ident.Name
	}
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		qn, ok := rel.Target.(*ast.QualifiedName)
		if !ok || len(qn.Parts) == 0 {
			continue
		}
		return qn.Parts[len(qn.Parts)-1].Text
	}
	return ""
}

// annotationValue evaluates one value an annotation binds: a constant, or a
// reference to an element such as an enumeration literal, which is compared by
// identity.
func (m *Model) annotationValue(scope *symbols.Scope, value ast.Node) symbols.FilterValue {
	if v, ok := evalConst(value); ok {
		return constValue(v)
	}
	switch e := value.(type) {
	case *ast.LiteralString:
		return symbols.FilterValue{Kind: symbols.FilterValueString, Str: unquote(e.Value)}
	case *ast.FeatureReference:
		if sym, ok := m.resolver.ResolveQualified(scope, e.Name); ok && sym != nil {
			if fqn := m.fqnOf(sym); fqn != "" {
				return symbols.FilterValue{Kind: symbols.FilterValueRef, RefFQN: fqn}
			}
		}
	}
	return symbols.FilterValue{}
}

// metaclassOf returns the reflective metadata type of the candidate's own
// metaclass: `SysML::PartUsage` for a part usage. It is what `@@T` tests, and
// what makes `@SysML::PartUsage` — the form the training corpus filters views
// with — select by what an element *is* rather than by what annotates it.
//
// The metaclass follows the symbol's kind rather than its declaration, so an
// element restored from an index cache is classified the same way as a parsed
// one.
func (m *Model) metaclassOf(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil {
		return nil
	}
	name := metaclassName(sym.Kind)
	if name == "" {
		return nil
	}
	if meta := m.symbolByFQN(sysmlMetaclassPrefix + name); meta != nil {
		return meta
	}
	return m.symbolByFQN(name)
}

// sysmlMetaclassPrefix qualifies the reflective metadata types of the SysML
// abstract syntax, which the standard library declares in SysML::Systems and
// re-exports through SysML.
const sysmlMetaclassPrefix = "SysML::Systems::"

// metaclassName is the reflective SysML metadata type classifying a declaration
// of the given kind, or "" for a kind the abstract syntax has no metaclass for.
func metaclassName(kind symbols.SymbolKind) string {
	switch kind {
	case symbols.SymbolPackage:
		return "Package"
	case symbols.SymbolNamespace:
		return "Namespace"
	case symbols.SymbolPartDef:
		return "PartDefinition"
	case symbols.SymbolPartUsage:
		return "PartUsage"
	case symbols.SymbolAttributeDef:
		return "AttributeDefinition"
	case symbols.SymbolAttributeUsage:
		return "AttributeUsage"
	case symbols.SymbolItemDef:
		return "ItemDefinition"
	case symbols.SymbolItemUsage:
		return "ItemUsage"
	case symbols.SymbolOccurrenceDef:
		return "OccurrenceDefinition"
	case symbols.SymbolOccurrenceUsage, symbols.SymbolIndividualUsage:
		return "OccurrenceUsage"
	case symbols.SymbolIndividualDef:
		return "OccurrenceDefinition"
	case symbols.SymbolMetadataDef:
		return "MetadataDefinition"
	case symbols.SymbolMetadataUsage:
		return "MetadataUsage"
	case symbols.SymbolEnumerationDef:
		return "EnumerationDefinition"
	case symbols.SymbolEnumerationUsage:
		return "EnumerationUsage"
	case symbols.SymbolViewDef:
		return "ViewDefinition"
	case symbols.SymbolViewUsage:
		return "ViewUsage"
	case symbols.SymbolViewpointDef:
		return "ViewpointDefinition"
	case symbols.SymbolViewpointUsage:
		return "ViewpointUsage"
	case symbols.SymbolRenderingDef:
		return "RenderingDefinition"
	case symbols.SymbolRenderingUsage:
		return "RenderingUsage"
	case symbols.SymbolConcernDef:
		return "ConcernDefinition"
	case symbols.SymbolConcernUsage:
		return "ConcernUsage"
	case symbols.SymbolConnectionDef:
		return "ConnectionDefinition"
	case symbols.SymbolConnectionUsage:
		return "ConnectionUsage"
	case symbols.SymbolFlowDef:
		return "FlowDefinition"
	case symbols.SymbolFlowUsage:
		return "FlowUsage"
	case symbols.SymbolPortDef:
		return "PortDefinition"
	case symbols.SymbolPortUsage:
		return "PortUsage"
	case symbols.SymbolInterfaceDef:
		return "InterfaceDefinition"
	case symbols.SymbolInterfaceUsage:
		return "InterfaceUsage"
	case symbols.SymbolAllocationDef:
		return "AllocationDefinition"
	case symbols.SymbolAllocationUsage:
		return "AllocationUsage"
	case symbols.SymbolActionDef:
		return "ActionDefinition"
	case symbols.SymbolActionUsage:
		return "ActionUsage"
	case symbols.SymbolStateDef:
		return "StateDefinition"
	case symbols.SymbolStateUsage:
		return "StateUsage"
	case symbols.SymbolCalcDef:
		return "CalculationDefinition"
	case symbols.SymbolCalcUsage:
		return "CalculationUsage"
	case symbols.SymbolConstraintDef:
		return "ConstraintDefinition"
	case symbols.SymbolConstraintUsage:
		return "ConstraintUsage"
	case symbols.SymbolRequirementDef:
		return "RequirementDefinition"
	case symbols.SymbolRequirementUsage:
		return "RequirementUsage"
	case symbols.SymbolCaseDef:
		return "CaseDefinition"
	case symbols.SymbolCaseUsage:
		return "CaseUsage"
	case symbols.SymbolAnalysisCaseDef:
		return "AnalysisCaseDefinition"
	case symbols.SymbolAnalysisCaseUsage:
		return "AnalysisCaseUsage"
	case symbols.SymbolVerificationCaseDef:
		return "VerificationCaseDefinition"
	case symbols.SymbolVerificationCaseUsage:
		return "VerificationCaseUsage"
	case symbols.SymbolUseCaseDef:
		return "UseCaseDefinition"
	case symbols.SymbolUseCaseUsage:
		return "UseCaseUsage"
	default:
		return ""
	}
}
