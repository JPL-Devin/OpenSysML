package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// TypeCheckPass validates that each def/usage relationship target has a symbol
// kind compatible with the source node and relationship kind (spec §6.3), and
// types expressions (operands, bound values, invocation arguments) against the
// stdlib scalar lattice.
// It runs at LevelType, after name resolution; unresolved targets are skipped.
type TypeCheckPass struct{}

func (TypeCheckPass) Level() PassLevel { return LevelType }

func (TypeCheckPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	// Initialize model to enable inheritance-aware resolution
	model := ctx.Model()
	tc := &typeChecker{
		resolver: ctx.Resolver(),
		expr:     &exprChecker{resolver: ctx.Resolver(), model: model},
		lang:     source.KindOf(name),
	}
	tc.walk(rootScope, root.Members)
	return append(tc.diags, tc.expr.diags...)
}

type typeChecker struct {
	resolver *resolve.Resolver
	expr     *exprChecker
	// lang is the document's language; a document of no known kind — the REPL
	// buffer — reads as SysML, the notation its prompt takes.
	lang  source.Kind
	diags []Diagnostic
}

func (tc *typeChecker) walk(scope *symbols.Scope, members []ast.Node) {
	for _, m := range members {
		switch d := unwrapType(m).(type) {
		case *ast.Definition:
			tc.checkRelationships(scope, d.Relationships, declKind{
				lang: tc.lang, isDef: true, defKind: d.Kind, keyword: d.Keyword,
			})
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		case *ast.Usage:
			tc.checkRelationships(scope, d.Relationships, declKind{
				lang:         tc.lang,
				useKind:      d.Kind,
				direction:    d.Direction,
				isEnd:        d.IsEnd,
				isIndividual: d.IsIndividual,
				portion:      d.Portion,
			})
			tc.expr.checkUsageValue(scope, d)
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		case *ast.Package:
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		case *ast.Namespace:
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		default:
			tc.checkBehaviorMember(scope, d)
		}
	}
}

// checkBehaviorMember types the expressions carried by behavior body members
// (calc results, constraints, guards, conditions, assignments).
func (tc *typeChecker) checkBehaviorMember(scope *symbols.Scope, n ast.Node) {
	switch m := n.(type) {
	case *ast.ResultMember:
		tc.expr.infer(scope, m.Expression)
	case *ast.ConstraintMember:
		tc.expr.checkBoolean(scope, m.Expression, "constraint expression")
		tc.walk(scope, m.Body)
	case *ast.AssumeMember:
		tc.expr.checkBoolean(scope, m.Expression, "assume expression")
		tc.walk(symbols.ConstraintBodyScope(scope, m), m.Body)
	case *ast.RequireMember:
		tc.expr.checkBoolean(scope, m.Expression, "require expression")
		tc.walk(symbols.ConstraintBodyScope(scope, m), m.Body)
	case *ast.IfActionNode:
		// The condition is evaluated before either branch is entered, so it is
		// checked outside them; each branch's body is checked in its own scope.
		tc.expr.checkBoolean(scope, m.Condition, "condition of 'if'")
		for _, branch := range m.Branches() {
			tc.checkBehaviorMember(scope, branch)
		}
	case *ast.IfBranchNode:
		body := scope
		if child := childScopeOf(scope, m); child != nil {
			body = child
		}
		tc.walk(body, m.Body)
	case *ast.WhileLoopActionNode:
		// The condition may read the loop's own body members, which live in the
		// scope the loop owns; the collection a `for` loop iterates over is
		// evaluated before the loop is entered, so it is checked outside it.
		body := scope
		if child := childScopeOf(scope, m); child != nil {
			body = child
		}
		tc.expr.infer(scope, m.Collection)
		tc.expr.checkBoolean(body, m.Condition, "condition of '"+m.Kind.String()+"'")
		tc.expr.checkBoolean(body, m.Until, "condition of 'until'")
		tc.walk(body, m.Body)
	case *ast.TransitionMember:
		tc.expr.checkBoolean(scope, m.Guard, "transition guard")
		tc.checkTrigger(scope, m.Trigger)
	case *ast.AssignmentActionNode:
		tc.expr.infer(scope, m.Value)
	case *ast.ActionExecutionNode:
		tc.expr.infer(scope, m.Expression)
	case *ast.SubjectMember:
		tc.checkSubjectMember(scope, m)
	default:
		// A body member that is an expression is a value the body computes — a
		// calc body whose result is its last expression — and is typed as one.
		tc.expr.infer(scope, n)
	}
}

// checkSubjectMember types a `subject` declared through the requirement body
// path, which yields a SubjectMember rather than a Usage and so would otherwise
// escape the usage-kind rules.
func (tc *typeChecker) checkSubjectMember(scope *symbols.Scope, m *ast.SubjectMember) {
	if m.TypeRef != nil {
		tc.checkTypeTarget(scope, m.TypeRef, ast.RelTyping, declKind{lang: tc.lang, useKind: ast.UsageSubject})
	}
	tc.checkRelationships(scope, m.Relationships, declKind{lang: tc.lang, useKind: ast.UsageSubject})
	if m.BindingExpr != nil {
		tc.expr.infer(scope, m.BindingExpr)
	}
	if len(m.Body) > 0 {
		body := scope
		if child := childScopeOf(scope, m); child != nil {
			body = child
		}
		tc.walk(body, m.Body)
	}
}

// checkTrigger types a transition trigger. A change event carries a Boolean
// condition and a time event a duration expression; a signal or call trigger
// names an event and has nothing to type here. `transition ... when <expr>`
// leaves the trigger as a bare expression, which is a change-event condition
// unless it is a bare name — that names a signal.
func (tc *typeChecker) checkTrigger(scope *symbols.Scope, trigger ast.Node) {
	switch t := trigger.(type) {
	case nil:
	case *ast.ChangeEvent:
		tc.expr.checkBoolean(scope, t.Condition, "change event condition")
	case *ast.TimeEvent:
		tc.expr.infer(scope, t.Duration)
	case *ast.FeatureReference, *ast.QualifiedName, *ast.AcceptEvent, *ast.CallEvent:
	default:
		tc.expr.checkBoolean(scope, trigger, "change event condition")
	}
}

// declKind describes the declaration a relationship is declared on: what the
// kind-compatibility rules are checked against.
type declKind struct {
	// lang is the language of the document the declaration is written in: the
	// KerML type layer has no definition/usage split to check against.
	lang      source.Kind
	isDef     bool
	defKind   ast.DefinitionKind
	useKind   ast.UsageKind
	direction ast.FeatureDirection
	// isEnd marks a feature declared with the `end` modifier, whose type is that
	// of the feature it connects and so escapes the usage-kind taxonomy.
	isEnd bool
	// isIndividual and portion carry the `individual` modifier and the
	// `snapshot`/`timeslice` portion prefix (SysML v2 §8.3.9.11:
	// `OccurrenceUsage::isIndividual` and `OccurrenceUsage::portionKind`).
	// Either makes the declaration an occurrence usage, whatever kind keyword
	// declares it.
	isIndividual bool
	portion      ast.PortionKind
	// keyword is the kind keyword as written, which tells apart the spellings a
	// single DefinitionKind carries (`classifier` and `class` are both DefClass).
	keyword string
}

// isPlainClassifier reports whether the declaration is written with `classifier`
// (or `subclassifier`), a KerML Classifier rather than the narrower `class`.
func (d declKind) isPlainClassifier() bool {
	return d.isDef && d.defKind == ast.DefClass &&
		(d.keyword == "classifier" || d.keyword == "subclassifier")
}

// isKerML reports whether the declaration is written in KerML, which has no
// definition/usage distinction to classify it against (KerML 1.0 §8.3).
func (d declKind) isKerML() bool { return d.lang == source.KindKerML }

// isOccurrenceUsage reports whether the `individual` modifier or a portion
// prefix makes the declaration an occurrence usage.
func (d declKind) isOccurrenceUsage() bool {
	return d.isIndividual || d.portion != ast.PortionNone
}

// occurrenceModifier names the occurrence modifier the declaration carries, for
// diagnostics.
func (d declKind) occurrenceModifier() string {
	if d.isIndividual {
		return "individual"
	}
	return d.portion.Keyword()
}

func (tc *typeChecker) checkRelationships(scope *symbols.Scope, rels []*ast.Relationship, decl declKind) {
	for _, rel := range rels {
		if rel == nil || rel.Target == nil {
			continue
		}
		tc.checkTypeTarget(scope, rel.Target, rel.Kind, decl)
		if rel.Conjugated {
			tc.checkConjugatedTyping(scope, rel, decl)
		}
	}
}

// checkTypeTarget checks one relationship target against the kind rules for the
// declaration carrying it.
func (tc *typeChecker) checkTypeTarget(scope *symbols.Scope, target ast.Node, relKind ast.RelationshipKind, decl declKind) {
	// Unwrap FeatureReference if needed
	targetNode := target
	if fr, ok := targetNode.(*ast.FeatureReference); ok {
		targetNode = fr.Name
	}
	qn, isQN := targetNode.(*ast.QualifiedName)
	if !isQN {
		return
	}
	sym, ok := tc.resolver.ResolveQualified(scope, qn)
	if !ok || sym == nil {
		return // unresolved: name-resolution tier owns this
	}
	// Resolve aliases to their underlying types for the relationships whose
	// check depends on the target's kind: a typing and a generalization.
	targetSym := sym
	aliasMatters := relKind == ast.RelSpecializes ||
		relKind == ast.RelSubsets ||
		relKind == ast.RelRedefines ||
		relKind == ast.RelTyping
	if aliasMatters && sym.Kind == symbols.SymbolAlias {
		if resolved, ok := tc.resolver.ResolveAliasTarget(sym); ok && resolved != nil {
			targetSym = resolved
		}
	}
	if msg := compatMessage(decl, relKind, targetSym.Kind); msg != "" {
		tc.diags = append(tc.diags, Diagnostic{
			Severity: SeverityError,
			Span:     target.Span(),
			Message:  msg,
			Code:     "type",
			Source:   "type",
		})
	}
}

// checkConjugatedTyping checks a `~T` typing: conjugation names the conjugated
// port definition of a port definition, so T must be one (SysML v2 §7.12.3).
func (tc *typeChecker) checkConjugatedTyping(scope *symbols.Scope, rel *ast.Relationship, decl declKind) {
	target := rel.Target
	if fr, ok := target.(*ast.FeatureReference); ok {
		target = fr.Name
	}
	qn, isQN := target.(*ast.QualifiedName)
	if !isQN {
		return
	}
	sym, ok := tc.resolver.ResolveQualified(scope, qn)
	if !ok || sym == nil {
		return // unresolved: name-resolution tier owns this
	}
	if sym.Kind == symbols.SymbolAlias {
		if resolved, ok := tc.resolver.ResolveAliasTarget(sym); ok && resolved != nil {
			sym = resolved
		}
	}
	switch sym.Kind {
	case symbols.SymbolPortDef, symbols.SymbolPortUsage:
	default:
		tc.diags = append(tc.diags, Diagnostic{
			Severity: SeverityError,
			Span:     target.Span(),
			Message: fmt.Sprintf(
				"'~' names the conjugated port definition of a port definition, found %s", sym.Kind),
			Code:   "type",
			Source: "type",
		})
		return
	}
	if decl.isDef || (decl.useKind != ast.UsagePort && !decl.isEnd) {
		tc.diags = append(tc.diags, Diagnostic{
			Severity: SeverityError,
			Span:     target.Span(),
			Message:  "only a port usage or a connector end may be typed by a conjugated port definition",
			Code:     "type",
			Source:   "type",
		})
	}
}

func compatMessage(decl declKind, rel ast.RelationshipKind, target symbols.SymbolKind) string {
	isDef, defKind, useKind, direction := decl.isDef, decl.defKind, decl.useKind, decl.direction
	switch rel {
	case ast.RelSpecializes:
		want := defSymbolKind(defKind)
		if !isDef {
			if !decl.isKerML() {
				return "only a definition may specialize; found a usage"
			}
			// Every KerML declaration is a Type and specializes a Type; the
			// definition/usage taxonomy does not apply (KerML 1.0 §8.3.3).
			if !isTypeKind(target) {
				return fmt.Sprintf("a KerML type may specialize only a type, found %s", target)
			}
			return ""
		}
		if target == symbols.SymbolUnknown || target == symbols.SymbolKerMLType {
			return "" // a KerML type or an unclassified kind constrains nothing
		}
		if !isDefKind(target) {
			return fmt.Sprintf("%s cannot specialize %s (target is not a definition)", defKind, target)
		}
		// Enums can specialize attribute defs (per SysML v2 spec: GradePoints :> Real)
		if defKind == ast.DefEnumeration && target == symbols.SymbolAttributeDef {
			return ""
		}
		// Metadata defs can specialize metaclasses (per SysML v2 spec: situation :> SemanticMetadata)
		if defKind == ast.DefMetadata && target == symbols.SymbolMetaclass {
			return ""
		}
		// `individual def X` is an occurrence definition (equivalent to
		// `individual occurrence def X`) that individuates the definition it
		// specializes, so it may specialize an occurrence definition of any kind
		// (SysML v2 §7.9.4). It may not specialize an attribute definition:
		// Occurrences::Occurrence is disjoint with Base::DataValues (§8.4.5.1).
		if defKind == ast.DefIndividual && isOccurrenceDefKind(target) {
			return ""
		}
		// Every definition is a Classifier, a DataType among them (KerML §8.3.2), so a
		// classifier specializes any kind; `class`/`struct` stay constrained (§8.4.4.1).
		if decl.isPlainClassifier() {
			return ""
		}
		if !defKindsComparable(target, want) {
			return fmt.Sprintf("%s cannot specialize %s (kind mismatch)", defKind, target)
		}
	case ast.RelSubsets, ast.RelRedefines:
		if isDef {
			return fmt.Sprintf("a definition may not %s a feature", rel)
		}
		if target == symbols.SymbolUnknown {
			return "" // an unclassified target constrains nothing
		}
		// Usages can subset/redefine other usages OR definitions
		// Example: datatype MyReal :>> Real (usage redefines attributeDef)
		// The check for isUsageKind OR isDefKind allows both patterns
		if !isUsageKind(target) && !isDefKind(target) {
			return fmt.Sprintf("%s target must be a usage or definition, found %s", rel, target)
		}
		// `satisfy`/`verify <name>` is a reference subsetting of an existing
		// requirement usage; viewpoint and concern usages are requirement usages.
		if useKind == ast.UsageSatisfy && rel == ast.RelSubsets && !isRequirementUsageKind(target) {
			return fmt.Sprintf("satisfy target must be a requirement usage, found %s", target)
		}
	case ast.RelTyping:
		if isDef {
			return "" // typing on a definition is not produced by the parser; ignore
		}
		// A KerML FeatureTyping's type is any Type, a Feature among them (KerML
		// 1.0 §8.3.4.4); KerML has no usage-kind taxonomy to check further.
		if decl.isKerML() {
			if !isTypeKind(target) {
				return fmt.Sprintf("type must be a type, found %s", target)
			}
			return ""
		}
		if !isDefKind(target) {
			return fmt.Sprintf("type must be a definition, found %s", target)
		}
		// An end feature is a plain KerML feature typed by whatever the feature it
		// connects is typed by (`end supplierPort : FuelOutPort`), so the usage-kind
		// taxonomy does not constrain it.
		if decl.isEnd {
			return ""
		}
		// Every SysML definition specializes a KerML type (a part def is a
		// Structure, an action def a Behavior …), so a KerML type may type a usage
		// of any kind (KerML 1.0 §8.3.4).
		if target == symbols.SymbolKerMLType {
			return ""
		}
		// An `individual` or `snapshot` usage is an occurrence usage, and an
		// occurrence is disjoint with the data values an attribute or enumeration
		// definition classifies (SysML v2 §8.4.5.1), so name the modifier that
		// makes the typing wrong rather than the kind keyword.
		if decl.isOccurrenceUsage() && isDataTypeDefKind(target) {
			return fmt.Sprintf("%s usage cannot be typed by %s (an occurrence usage may not be typed by a data type)", decl.occurrenceModifier(), target)
		}
		if !isCompatibleTyping(useKind, direction, target, decl.isOccurrenceUsage()) {
			return fmt.Sprintf("%s cannot be typed by %s (kind mismatch)", useKind, target)
		}
	case ast.RelReferences, ast.RelCrosses, ast.RelVia, ast.RelAnnotates, ast.RelSubject:
		if !isDef && !isUsageKind(target) {
			return fmt.Sprintf("%s target must be a usage, found %s", rel, target)
		}
	}
	return ""
}

func defSymbolKind(k ast.DefinitionKind) symbols.SymbolKind {
	switch k {
	case ast.DefPart:
		return symbols.SymbolPartDef
	case ast.DefAttribute:
		return symbols.SymbolAttributeDef
	case ast.DefItem:
		return symbols.SymbolItemDef
	case ast.DefOccurrence:
		return symbols.SymbolOccurrenceDef
	case ast.DefIndividual:
		return symbols.SymbolIndividualDef
	case ast.DefMetadata:
		return symbols.SymbolMetadataDef
	case ast.DefMetaclass:
		// A Metaclass is a Class (KerML 1.0 §8.4.4), so it specializes a metaclass.
		return symbols.SymbolMetaclass
	case ast.DefEnumeration:
		return symbols.SymbolEnumerationDef
	case ast.DefView:
		return symbols.SymbolViewDef
	case ast.DefViewpoint:
		return symbols.SymbolViewpointDef
	case ast.DefRendering:
		return symbols.SymbolRenderingDef
	case ast.DefConcern:
		return symbols.SymbolConcernDef
	case ast.DefConnection:
		return symbols.SymbolConnectionDef
	case ast.DefFlow:
		return symbols.SymbolFlowDef
	case ast.DefPort:
		return symbols.SymbolPortDef
	case ast.DefInterface:
		return symbols.SymbolInterfaceDef
	case ast.DefAllocation:
		return symbols.SymbolAllocationDef
	case ast.DefAction:
		return symbols.SymbolActionDef
	case ast.DefState:
		return symbols.SymbolStateDef
	case ast.DefCalc:
		return symbols.SymbolCalcDef
	case ast.DefConstraint:
		return symbols.SymbolConstraintDef
	case ast.DefRequirement:
		return symbols.SymbolRequirementDef
	case ast.DefCase:
		return symbols.SymbolCaseDef
	case ast.DefAnalysisCase:
		return symbols.SymbolAnalysisCaseDef
	case ast.DefVerificationCase:
		return symbols.SymbolVerificationCaseDef
	case ast.DefUseCase:
		return symbols.SymbolUseCaseDef
	}
	return symbols.SymbolUnknown
}

func usageWantsDefKind(k ast.UsageKind) symbols.SymbolKind {
	switch k {
	case ast.UsagePart:
		return symbols.SymbolPartDef
	case ast.UsageAttribute:
		return symbols.SymbolAttributeDef
	case ast.UsageItem:
		return symbols.SymbolItemDef
	case ast.UsageOccurrence:
		return symbols.SymbolOccurrenceDef
	case ast.UsageIndividual:
		return symbols.SymbolIndividualDef
	case ast.UsageMetadata:
		return symbols.SymbolMetadataDef
	case ast.UsageEnumeration:
		return symbols.SymbolEnumerationDef
	case ast.UsageView:
		return symbols.SymbolViewDef
	case ast.UsageViewpoint:
		return symbols.SymbolViewpointDef
	case ast.UsageRendering, ast.UsageViewRendering:
		return symbols.SymbolRenderingDef
	case ast.UsageConcern, ast.UsageFramedConcern:
		return symbols.SymbolConcernDef
	case ast.UsageActor, ast.UsageStakeholder:
		// An actor and a stakeholder are part usages, so a part definition types
		// them (SysML v2 §8.3.19).
		return symbols.SymbolPartDef
	case ast.UsageConnection:
		return symbols.SymbolConnectionDef
	case ast.UsageFlow:
		return symbols.SymbolFlowDef
	case ast.UsagePort:
		return symbols.SymbolPortDef
	case ast.UsageInterface:
		return symbols.SymbolInterfaceDef
	case ast.UsageAllocation:
		return symbols.SymbolAllocationDef
	case ast.UsageAction:
		return symbols.SymbolActionDef
	case ast.UsageState:
		return symbols.SymbolStateDef
	case ast.UsageCalc:
		return symbols.SymbolCalcDef
	case ast.UsageConstraint:
		return symbols.SymbolConstraintDef
	case ast.UsageRequirement:
		return symbols.SymbolRequirementDef
	case ast.UsageCase:
		return symbols.SymbolCaseDef
	case ast.UsageAnalysisCase:
		return symbols.SymbolAnalysisCaseDef
	case ast.UsageVerificationCase:
		return symbols.SymbolVerificationCaseDef
	case ast.UsageUseCase:
		return symbols.SymbolUseCaseDef
	}
	return symbols.SymbolUnknown
}

// defSymbolKinds is the set of SymbolKinds that classify a definition.
var defSymbolKinds = map[symbols.SymbolKind]bool{
	symbols.SymbolPartDef:             true,
	symbols.SymbolAttributeDef:        true,
	symbols.SymbolItemDef:             true,
	symbols.SymbolOccurrenceDef:       true,
	symbols.SymbolIndividualDef:       true,
	symbols.SymbolMetadataDef:         true,
	symbols.SymbolMetaclass:           true, // KerML metaclass definitions
	symbols.SymbolEnumerationDef:      true,
	symbols.SymbolViewDef:             true,
	symbols.SymbolViewpointDef:        true,
	symbols.SymbolRenderingDef:        true,
	symbols.SymbolConcernDef:          true,
	symbols.SymbolConnectionDef:       true,
	symbols.SymbolFlowDef:             true,
	symbols.SymbolPortDef:             true,
	symbols.SymbolInterfaceDef:        true,
	symbols.SymbolAllocationDef:       true,
	symbols.SymbolActionDef:           true,
	symbols.SymbolStateDef:            true,
	symbols.SymbolCalcDef:             true,
	symbols.SymbolConstraintDef:       true,
	symbols.SymbolRequirementDef:      true,
	symbols.SymbolCaseDef:             true,
	symbols.SymbolAnalysisCaseDef:     true,
	symbols.SymbolVerificationCaseDef: true,
	symbols.SymbolUseCaseDef:          true,
	symbols.SymbolKerMLType:           true, // KerML class/struct/assoc/behavior/predicate
	symbols.SymbolAlias:               true, // Aliases can be used as types
}

// usageSymbolKinds is the set of SymbolKinds that classify a usage.
var usageSymbolKinds = map[symbols.SymbolKind]bool{
	symbols.SymbolPartUsage:             true,
	symbols.SymbolAttributeUsage:        true,
	symbols.SymbolItemUsage:             true,
	symbols.SymbolOccurrenceUsage:       true,
	symbols.SymbolIndividualUsage:       true,
	symbols.SymbolMetadataUsage:         true,
	symbols.SymbolEnumerationUsage:      true,
	symbols.SymbolViewUsage:             true,
	symbols.SymbolViewpointUsage:        true,
	symbols.SymbolRenderingUsage:        true,
	symbols.SymbolConcernUsage:          true,
	symbols.SymbolConnectionUsage:       true,
	symbols.SymbolFlowUsage:             true,
	symbols.SymbolPortUsage:             true,
	symbols.SymbolInterfaceUsage:        true,
	symbols.SymbolAllocationUsage:       true,
	symbols.SymbolActionUsage:           true,
	symbols.SymbolStateUsage:            true,
	symbols.SymbolCalcUsage:             true,
	symbols.SymbolConstraintUsage:       true,
	symbols.SymbolRequirementUsage:      true,
	symbols.SymbolCaseUsage:             true,
	symbols.SymbolAnalysisCaseUsage:     true,
	symbols.SymbolVerificationCaseUsage: true,
	symbols.SymbolUseCaseUsage:          true,
	symbols.SymbolConnectorEnd:          true, // An end of a connect clause is a feature
	symbols.SymbolAlias:                 true, // Aliases can be subsetting targets
}

func isDefKind(k symbols.SymbolKind) bool {
	return defSymbolKinds[k]
}

func isUsageKind(k symbols.SymbolKind) bool {
	return usageSymbolKinds[k]
}

// typeSymbolKinds is the set of SymbolKinds that classify a Type: every
// definition and usage kind, plus a KerML type declaration. Enumerated rather
// than derived by negation so a kind added later is rejected until classified.
var typeSymbolKinds = func() map[symbols.SymbolKind]bool {
	m := map[symbols.SymbolKind]bool{symbols.SymbolKerMLType: true}
	for k := range defSymbolKinds {
		m[k] = true
	}
	for k := range usageSymbolKinds {
		m[k] = true
	}
	// An alias is a naming, not a Type; a resolvable one is resolved upstream.
	delete(m, symbols.SymbolAlias)
	return m
}()

// isTypeKind reports whether k classifies a Type, which alone may be a KerML
// specialization or typing target — a Feature is one too (KerML 1.0 §8.3.3).
// An unclassified kind constrains nothing, as on the definition path.
func isTypeKind(k symbols.SymbolKind) bool {
	return k == symbols.SymbolUnknown || typeSymbolKinds[k]
}

// occurrenceDefSymbolKinds is the set of SymbolKinds that classify a definition
// that is an OccurrenceDefinition in the SysML v2 abstract syntax (§8.3.9.3):
// the occurrence definition itself, an individual definition, and every kind
// whose metaclass directly or indirectly specializes OccurrenceDefinition —
// items and parts (§8.3.10.2, §8.3.11.2), ports (§8.3.12.5), connections with
// their interface and allocation specializations (§8.3.13.3, §8.3.14.2,
// §8.3.15.2), actions and everything derived from them (§8.3.16.2, §8.3.17.3,
// §8.3.18.5, §8.3.19.2, §8.3.22.2, §8.3.23.2, §8.3.24.3, §8.3.25.3),
// constraints and requirements (§8.3.20.3, §8.3.21.3, §8.3.21.8, §8.3.26.8),
// views and renderings (§8.3.26.7, §8.3.26.5) and metadata definitions
// (§8.3.27.2). Attribute and enumeration definitions are data types, not
// occurrence definitions (§8.3.7.2, §8.3.8.2).
var occurrenceDefSymbolKinds = map[symbols.SymbolKind]bool{
	symbols.SymbolOccurrenceDef:       true,
	symbols.SymbolIndividualDef:       true,
	symbols.SymbolItemDef:             true,
	symbols.SymbolPartDef:             true,
	symbols.SymbolPortDef:             true,
	symbols.SymbolConnectionDef:       true,
	symbols.SymbolInterfaceDef:        true,
	symbols.SymbolAllocationDef:       true,
	symbols.SymbolFlowDef:             true,
	symbols.SymbolActionDef:           true,
	symbols.SymbolStateDef:            true,
	symbols.SymbolCalcDef:             true,
	symbols.SymbolCaseDef:             true,
	symbols.SymbolAnalysisCaseDef:     true,
	symbols.SymbolVerificationCaseDef: true,
	symbols.SymbolUseCaseDef:          true,
	symbols.SymbolConstraintDef:       true,
	symbols.SymbolRequirementDef:      true,
	symbols.SymbolConcernDef:          true,
	symbols.SymbolViewpointDef:        true,
	symbols.SymbolViewDef:             true,
	symbols.SymbolRenderingDef:        true,
	symbols.SymbolMetadataDef:         true,
}

func isOccurrenceDefKind(k symbols.SymbolKind) bool {
	return occurrenceDefSymbolKinds[k]
}

// defKindParents is the definition metaclass taxonomy (SysML v2 §8.3): each kind
// maps to the kinds it specializes.
var defKindParents = map[symbols.SymbolKind][]symbols.SymbolKind{
	symbols.SymbolItemDef:             {symbols.SymbolOccurrenceDef},
	symbols.SymbolIndividualDef:       {symbols.SymbolOccurrenceDef},
	symbols.SymbolPartDef:             {symbols.SymbolItemDef},
	symbols.SymbolMetadataDef:         {symbols.SymbolItemDef},
	symbols.SymbolConnectionDef:       {symbols.SymbolPartDef},
	symbols.SymbolInterfaceDef:        {symbols.SymbolConnectionDef},
	symbols.SymbolAllocationDef:       {symbols.SymbolConnectionDef},
	symbols.SymbolViewDef:             {symbols.SymbolPartDef},
	symbols.SymbolRenderingDef:        {symbols.SymbolPartDef},
	symbols.SymbolActionDef:           {symbols.SymbolOccurrenceDef},
	symbols.SymbolFlowDef:             {symbols.SymbolActionDef, symbols.SymbolConnectionDef},
	symbols.SymbolStateDef:            {symbols.SymbolActionDef},
	symbols.SymbolCalcDef:             {symbols.SymbolActionDef},
	symbols.SymbolCaseDef:             {symbols.SymbolCalcDef},
	symbols.SymbolAnalysisCaseDef:     {symbols.SymbolCaseDef},
	symbols.SymbolVerificationCaseDef: {symbols.SymbolCaseDef},
	symbols.SymbolUseCaseDef:          {symbols.SymbolCaseDef},
	symbols.SymbolConstraintDef:       {symbols.SymbolOccurrenceDef},
	symbols.SymbolRequirementDef:      {symbols.SymbolConstraintDef},
	symbols.SymbolConcernDef:          {symbols.SymbolRequirementDef},
	symbols.SymbolViewpointDef:        {symbols.SymbolRequirementDef},
	symbols.SymbolPortDef:             {symbols.SymbolOccurrenceDef},
	symbols.SymbolEnumerationDef:      {symbols.SymbolAttributeDef},
}

// defKindSpecializes reports whether kind k is want or one of its
// specializations in that taxonomy.
func defKindSpecializes(k, want symbols.SymbolKind) bool {
	if k == want {
		return true
	}
	for _, parent := range defKindParents[k] {
		if defKindSpecializes(parent, want) {
			return true
		}
	}
	return false
}

// defKindsComparable reports whether one of two definition kinds specializes the
// other, as an item and a part definition do and a part and an attribute
// definition do not.
func defKindsComparable(a, b symbols.SymbolKind) bool {
	return defKindSpecializes(a, b) || defKindSpecializes(b, a)
}

// isDataTypeDefKind reports whether k classifies data values rather than
// occurrences: an attribute definition is a DataType and an enumeration
// definition an AttributeDefinition (SysML v2 §8.3.7.2, §8.3.8.2). Occurrences
// are disjoint with data values (§8.4.5.1).
func isDataTypeDefKind(k symbols.SymbolKind) bool {
	return k == symbols.SymbolAttributeDef || k == symbols.SymbolEnumerationDef
}

// isRequirementUsageKind reports whether k is a RequirementUsage or one of its
// specializations (ViewpointUsage, ConcernUsage).
func isRequirementUsageKind(k symbols.SymbolKind) bool {
	switch k {
	case symbols.SymbolRequirementUsage, symbols.SymbolViewpointUsage, symbols.SymbolConcernUsage:
		return true
	}
	return false
}

// isRequirementDefKind reports whether k is a RequirementDefinition or one of
// its specializations (ConcernDefinition, ViewpointDefinition).
func isRequirementDefKind(k symbols.SymbolKind) bool {
	switch k {
	case symbols.SymbolRequirementDef, symbols.SymbolConcernDef, symbols.SymbolViewpointDef:
		return true
	}
	return false
}

// isCompatibleTyping checks if a usage kind can be typed by a definition kind.
// Allows structural compatibility: part/attribute/item/occurrence can cross-type
// since they're all structural classifiers in SysML.
// isOccurrenceUsage marks a usage carrying the `individual` or `snapshot`
// modifier. Such a usage is an OccurrenceUsage whatever kind keyword declares it
// (SysML v2 §8.3.9.11), so it may be typed by an occurrence definition of any
// kind, on top of whatever its kind keyword admits.
func isCompatibleTyping(useKind ast.UsageKind, direction ast.FeatureDirection, defKind symbols.SymbolKind, isOccurrenceUsage bool) bool {
	if isOccurrenceUsage && isOccurrenceDefKind(defKind) {
		return true
	}
	if compatibleTyping(useKind, direction, defKind) {
		return true
	}
	// An individual definition is an occurrence definition that individuates the
	// definition it specializes (SysML v2 §7.9.4), so a usage may be typed by
	// one wherever it may be typed by an occurrence definition.
	if defKind == symbols.SymbolIndividualDef {
		return compatibleTyping(useKind, direction, symbols.SymbolOccurrenceDef)
	}
	return false
}

func compatibleTyping(useKind ast.UsageKind, direction ast.FeatureDirection, defKind symbols.SymbolKind) bool {
	// Exact match always allowed
	if defKind == usageWantsDefKind(useKind) {
		return true
	}

	// SysML v2 §7.27.2: a MetadataDefinition is an ItemDefinition, so it types
	// whatever an item definition types (`:> annotatedElement : SysML::PartDefinition`).
	if defKind == symbols.SymbolMetadataDef {
		defKind = symbols.SymbolItemDef
	}

	// Attributes can be typed by any structural def (for parameters, properties)
	// Also allow enumDef for typed enumerations
	// This allows: in scene : Scene (attribute : itemDef), verdict : VerdictKind (attribute : enumDef)
	if useKind == ast.UsageAttribute {
		return defKind == symbols.SymbolPartDef ||
			defKind == symbols.SymbolAttributeDef ||
			defKind == symbols.SymbolItemDef ||
			defKind == symbols.SymbolOccurrenceDef ||
			defKind == symbols.SymbolEnumerationDef
	}

	// Parameters (in/out/inout) can cross-type to any structural def
	// Also allow enumDef for typed enumerations
	// This allows: in power : PowerValue (part : attributeDef), out verdict : VerdictKind (part : enumDef)
	hasDirection := direction != ast.DirNone
	if hasDirection {
		return defKind == symbols.SymbolPartDef ||
			defKind == symbols.SymbolAttributeDef ||
			defKind == symbols.SymbolItemDef ||
			defKind == symbols.SymbolOccurrenceDef ||
			defKind == symbols.SymbolEnumerationDef
	}

	// An occurrence, item or part may be typed by an occurrence definition of
	// any kind (SysML v2 §8.3.9.7 validateOccurrenceUsageType).
	if useKind == ast.UsagePart {
		return isOccurrenceDefKind(defKind)
	}
	// Items and occurrences keep the table's attribute-def leniency for values.
	if useKind == ast.UsageItem || useKind == ast.UsageOccurrence || useKind == ast.UsageIndividual {
		return isOccurrenceDefKind(defKind) || defKind == symbols.SymbolAttributeDef
	}

	// An action must be typed by action definitions, i.e. Behaviors (SysML v2
	// §8.3.16.6 validateActionUsageType), so any behavior-family def works.
	if useKind == ast.UsageAction {
		return defKindSpecializes(defKind, symbols.SymbolActionDef)
	}

	// A case may be typed by a case definition of any kind (SysML v2 §8.3.24.4
	// validateCaseUsageType); analysis and verification keep their exact kinds.
	if useKind == ast.UsageCase {
		return defKindSpecializes(defKind, symbols.SymbolCaseDef)
	}

	// A succession is a binary connector (SysML v2 §8.3.13), so a connection
	// definition of any kind types it.
	if useKind == ast.UsageSuccession {
		return defKindSpecializes(defKind, symbols.SymbolConnectionDef)
	}

	// SysML v2 §8.3.22.4: an ObjectiveMembership's ownedObjectiveRequirement is a
	// RequirementUsage, so an objective is typed by a RequirementDefinition or one
	// of its specializations (`objective : MaximizeObjective`, a requirement def in
	// Domain Libraries/Analysis/TradeStudies.sysml).
	if useKind == ast.UsageObjective {
		return isRequirementDefKind(defKind)
	}

	// A SatisfyRequirementUsage is a RequirementUsage (SysML v2 §8.3.19), so the
	// declaration form `satisfy requirement r : Req1 by v` is typed by a
	// RequirementDefinition or one of its specializations.
	if useKind == ast.UsageSatisfy {
		return isRequirementDefKind(defKind)
	}

	// A SubjectMembership's ownedSubjectParameter is an unconstrained Usage (SysML
	// v2 §8.3.21), so any definition types a subject — the OMG training models
	// subject a `port def` and an `action def` as well as structural definitions.
	if useKind == ast.UsageSubject {
		return true
	}

	return false
}

func unwrapType(n ast.Node) ast.Node {
	if m, ok := n.(*ast.Membership); ok {
		return m.Member
	}
	return n
}

func childScopeOf(scope *symbols.Scope, decl ast.Node) *symbols.Scope {
	return scope.ChildFor(decl)
}
