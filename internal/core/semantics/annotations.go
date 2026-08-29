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
type annotation struct {
	typ    *symbols.Symbol
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
	if sym.Decl != nil {
		out = append(out, m.declaredAnnotations(sym)...)
		out = append(out, m.aboutAnnotations(sym)...)
	}
	m.annotations[sym] = out
	return out
}

// AnnotationFactsOf states the metadata annotating sym as names and constants,
// so that how an element filter classifies it can be compared across loads. The
// values an annotation binds are reported as read; a binding whose value is not
// constant is reported with an unknown value, which a condition reading it
// reports as unevaluable rather than silently treating as absent.
func (m *Model) AnnotationFactsOf(sym *symbols.Symbol) []symbols.AnnotationFacts {
	var out []symbols.AnnotationFacts
	for _, a := range m.annotationsOf(sym) {
		var typFQN string
		if a.typ != nil {
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
// is reported does not depend on map iteration order.
func sortedFeatureNames(values map[string]symbols.FilterValue) []string {
	out := make([]string, 0, len(values))
	for name := range values {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
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
	if resolved, aliasOK := m.resolver.ResolveAliasTarget(typ); aliasOK {
		typ = resolved
	} else {
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
		if resolved, aliasOK := m.resolver.ResolveAliasTarget(typ); aliasOK {
			typ = resolved
		} else {
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
	return annotation{typ: typ, values: values}
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

// metaclassOf is the candidate's own metaclass — what `@@T` tests: a KerML
// declaration by its keyword (its kind cannot tell `struct` from `datatype`),
// anything else by its symbol kind, so a cached element classifies alike.
func (m *Model) metaclassOf(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil {
		return nil
	}
	// A relationship written keyword-first is classified by its own kind in
	// either language, since no symbol kind distinguishes its forms.
	if rel, ok := sym.Decl.(*ast.RelationshipMember); ok {
		if meta := m.kermlMetaclass(relationshipMetaclassName(rel)); meta != nil {
			return meta
		}
	}
	if meta := m.kermlMetaclass(kermlMetaclassName(sym, m.isKerMLDoc(sym))); meta != nil {
		return meta
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

// kermlMetaclass is the library element declaring the named KerML metaclass,
// or nil for an unnamed or undeclared one.
func (m *Model) kermlMetaclass(name string) *symbols.Symbol {
	if name == "" {
		return nil
	}
	for _, prefix := range kermlMetaclassPrefixes {
		if meta := m.symbolByFQN(prefix + name); meta != nil {
			return meta
		}
	}
	return nil
}

// relationshipMetaclassName is the metaclass of a keyword-first relationship,
// which conjugation writes as a form of its own (KerML §7.2).
func relationshipMetaclassName(rel *ast.RelationshipMember) string {
	if rel.Conjugated {
		return "Conjugation"
	}
	return relationshipMetaclassNames[rel.Kind]
}

// relationshipMetaclassNames maps the kind of a relationship written
// keyword-first to the KerML metaclass classifying it (KerML §7.2).
var relationshipMetaclassNames = map[ast.RelationshipKind]string{
	ast.RelSpecializes: "Specialization",
	ast.RelTyping:      "FeatureTyping",
	ast.RelSubsets:     "Subsetting",
	ast.RelRedefines:   "Redefinition",
	ast.RelInverseOf:   "FeatureInverting",
	ast.RelFeaturedBy:  "TypeFeaturing",
}

// sysmlMetaclassPrefix qualifies the reflective metadata types of the SysML
// abstract syntax, which the standard library declares in SysML::Systems and
// re-exports through SysML.
const sysmlMetaclassPrefix = "SysML::Systems::"

// kermlMetaclassPrefixes qualify the KerML abstract syntax metaclasses, which
// the library declares across KerML's three packages.
var kermlMetaclassPrefixes = []string{"KerML::Kernel::", "KerML::Core::", "KerML::Root::"}

// kermlMetaclassNames maps a KerML declaration keyword to the metaclass it
// implies (KerML 1.1 §8.2, §9.2). `assoc struct` is recorded as `struct` by the
// parser, so it is classified as a Structure rather than an AssociationStructure.
var kermlMetaclassNames = map[string]string{
	"type":         "Type",
	"classifier":   "Classifier",
	"class":        "Class",
	"struct":       "Structure",
	"assoc":        "Association",
	"association":  "Association",
	"datatype":     "DataType",
	"behavior":     "Behavior",
	"function":     "Function",
	"predicate":    "Predicate",
	"interaction":  "Interaction",
	"metaclass":    "Metaclass",
	"feature":      "Feature",
	"step":         "Step",
	"expr":         "Expression",
	"bool":         "BooleanExpression",
	"inv":          "Invariant",
	"connector":    "Connector",
	"binding":      "BindingConnector",
	"bind":         "BindingConnector",
	"flow":         "Flow",
	"message":      "Flow",
	"succession":   "Succession",
	"multiplicity": "Multiplicity",
}

// kermlMetaclassName is the metaclass the keyword of sym's KerML declaration
// implies, or "" for a declaration written in SysML or restored from a cache,
// which is classified by its symbol kind instead.
func kermlMetaclassName(sym *symbols.Symbol, isKerML bool) string {
	if !isKerML {
		return ""
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return kermlMetaclassNames[d.Keyword]
	case *ast.Usage:
		return kermlMetaclassNames[d.Keyword]
	}
	return ""
}

// reflectiveFeatureValue is what the candidate's declaration states for a
// metaclass feature of it, and whether that feature is derived here at all
// (KerML 1.1 §8.2.4); an underived one is unevaluable, not false.
func (m *Model) reflectiveFeatureValue(sym *symbols.Symbol, feature string) (symbols.FilterValue, bool) {
	switch feature {
	case "name", "declaredName":
		return stringOrEmpty(simpleSymbolName(sym)), true
	case "qualifiedName":
		return stringOrEmpty(m.fqnOf(sym)), true
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		switch feature {
		case "isAbstract":
			return boolValue(d.IsAbstract), true
		case "isSufficient":
			return boolValue(d.IsAll), true
		case "isConstant":
			return boolValue(d.IsConstant), true
		}
	case *ast.Usage:
		switch feature {
		case "isAbstract":
			return boolValue(d.IsAbstract), true
		case "isSufficient":
			return boolValue(d.IsAll), true
		case "isComposite":
			return boolValue(d.IsComposite), true
		case "isDerived":
			return boolValue(d.IsDerived), true
		case "isEnd":
			return boolValue(d.IsEnd), true
		case "isOrdered":
			return boolValue(d.IsOrdered), true
		case "isUnique":
			return boolValue(!d.IsNonunique), true
		case "isVariable":
			return boolValue(d.IsVariable), true
		case "isConstant":
			return boolValue(d.IsConstant), true
		case "isPortion":
			return boolValue(d.Portion != ast.PortionNone), true
		}
	}
	return symbols.FilterValue{}, false
}

// stringOrEmpty is a string value, or the empty sequence for a name the
// declaration does not have.
func stringOrEmpty(s string) symbols.FilterValue {
	if s == "" {
		return emptyValue()
	}
	return symbols.FilterValue{Kind: symbols.FilterValueString, Str: s}
}

// metaclassNames maps each declaration kind to its reflective SysML metadata
// type.
var metaclassNames = map[symbols.SymbolKind]string{
	symbols.SymbolPackage:               "Package",
	symbols.SymbolNamespace:             "Namespace",
	symbols.SymbolPartDef:               "PartDefinition",
	symbols.SymbolPartUsage:             "PartUsage",
	symbols.SymbolAttributeDef:          "AttributeDefinition",
	symbols.SymbolAttributeUsage:        "AttributeUsage",
	symbols.SymbolItemDef:               "ItemDefinition",
	symbols.SymbolItemUsage:             "ItemUsage",
	symbols.SymbolOccurrenceDef:         "OccurrenceDefinition",
	symbols.SymbolOccurrenceUsage:       "OccurrenceUsage",
	symbols.SymbolIndividualUsage:       "OccurrenceUsage",
	symbols.SymbolIndividualDef:         "OccurrenceDefinition",
	symbols.SymbolMetadataDef:           "MetadataDefinition",
	symbols.SymbolMetadataUsage:         "MetadataUsage",
	symbols.SymbolEnumerationDef:        "EnumerationDefinition",
	symbols.SymbolEnumerationUsage:      "EnumerationUsage",
	symbols.SymbolViewDef:               "ViewDefinition",
	symbols.SymbolViewUsage:             "ViewUsage",
	symbols.SymbolViewpointDef:          "ViewpointDefinition",
	symbols.SymbolViewpointUsage:        "ViewpointUsage",
	symbols.SymbolRenderingDef:          "RenderingDefinition",
	symbols.SymbolRenderingUsage:        "RenderingUsage",
	symbols.SymbolConcernDef:            "ConcernDefinition",
	symbols.SymbolConcernUsage:          "ConcernUsage",
	symbols.SymbolConnectionDef:         "ConnectionDefinition",
	symbols.SymbolConnectionUsage:       "ConnectionUsage",
	symbols.SymbolSuccessionUsage:       "SuccessionAsUsage",
	symbols.SymbolFlowDef:               "FlowDefinition",
	symbols.SymbolFlowUsage:             "FlowUsage",
	symbols.SymbolPortDef:               "PortDefinition",
	symbols.SymbolPortUsage:             "PortUsage",
	symbols.SymbolInterfaceDef:          "InterfaceDefinition",
	symbols.SymbolInterfaceUsage:        "InterfaceUsage",
	symbols.SymbolAllocationDef:         "AllocationDefinition",
	symbols.SymbolAllocationUsage:       "AllocationUsage",
	symbols.SymbolActionDef:             "ActionDefinition",
	symbols.SymbolActionUsage:           "ActionUsage",
	symbols.SymbolStateDef:              "StateDefinition",
	symbols.SymbolStateUsage:            "StateUsage",
	symbols.SymbolCalcDef:               "CalculationDefinition",
	symbols.SymbolCalcUsage:             "CalculationUsage",
	symbols.SymbolConstraintDef:         "ConstraintDefinition",
	symbols.SymbolConstraintUsage:       "ConstraintUsage",
	symbols.SymbolRequirementDef:        "RequirementDefinition",
	symbols.SymbolRequirementUsage:      "RequirementUsage",
	symbols.SymbolCaseDef:               "CaseDefinition",
	symbols.SymbolCaseUsage:             "CaseUsage",
	symbols.SymbolAnalysisCaseDef:       "AnalysisCaseDefinition",
	symbols.SymbolAnalysisCaseUsage:     "AnalysisCaseUsage",
	symbols.SymbolVerificationCaseDef:   "VerificationCaseDefinition",
	symbols.SymbolVerificationCaseUsage: "VerificationCaseUsage",
	symbols.SymbolUseCaseDef:            "UseCaseDefinition",
	symbols.SymbolUseCaseUsage:          "UseCaseUsage",
}

// metaclassName is the reflective SysML metadata type classifying a declaration
// of the given kind, or "" for a kind the abstract syntax has no metaclass for.
func metaclassName(kind symbols.SymbolKind) string {
	return metaclassNames[kind]
}
