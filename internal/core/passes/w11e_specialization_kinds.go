package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// KerML specialization-kind constraint messages, quoted from KerMLValidator.
const (
	msgSpecializeClassOrAssoc    = "Cannot specialize class or association"
	msgSpecializeDataTypeOrAssoc = "Cannot specialize data type or association"
	msgSpecializeBehavior        = "Cannot specialize behavior"
	msgSpecializeStructure       = "Cannot specialize structure"
	msgConjugatedSpecific        = "Conjugated type cannot be a specialized type"
)

// metaKind is the set of KerML metaclasses a declaration is an instance of: the
// taxonomy is not a chain — an `assoc struct` is an Association and a Structure.
type metaKind uint8

const (
	metaDataType metaKind = 1 << iota
	metaClass
	metaStructure
	metaBehavior
	metaAssociation
)

// kermlMetaKinds maps a KerML type keyword to the metaclasses it instantiates
// (KerML 1.0 §8.3.2, §8.4.4: Structure, Behavior and Metaclass are Classes).
var kermlMetaKinds = map[string]metaKind{
	"datatype":    metaDataType,
	"class":       metaClass,
	"struct":      metaClass | metaStructure,
	"metaclass":   metaClass | metaStructure,
	"behavior":    metaClass | metaBehavior,
	"function":    metaClass | metaBehavior,
	"predicate":   metaClass | metaBehavior,
	"assoc":       metaAssociation,
	"association": metaAssociation,
	"interaction": metaAssociation | metaClass | metaBehavior,
}

// SpecializationKindsPass checks that a KerML type does not specialize a type of
// a disjoint metaclass, and that a conjugated type is not specialized (KerML 1.0
// validateDataTypeSpecialization, validateClassSpecialization,
// validateStructureSpecialization, validateBehaviorSpecialization,
// validateSpecializationSpecificNotConjugated).
type SpecializationKindsPass struct{}

func (SpecializationKindsPass) Level() PassLevel { return LevelConstraint }

func (SpecializationKindsPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil || ctx.Kind != source.KindKerML {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	w := &w8cWalker{seen: make(map[*symbols.Symbol]bool)}
	c := &specializationKindsChecker{resolver: ctx.Resolver()}
	w.walk(rootScope, c.check)
	return c.diags
}

type specializationKindsChecker struct {
	resolver *resolve.Resolver
	diags    []Diagnostic
}

func (c *specializationKindsChecker) check(sym *symbols.Symbol) {
	if rel, ok := sym.Decl.(*ast.RelationshipMember); ok {
		c.checkRelationshipMember(sym, rel)
		return
	}
	specific := w11eMetaKindOf(sym)
	conjugated := w11eHasConjugation(sym)
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelSpecializes || rel.Conjugated || rel.Target == nil {
			continue
		}
		if conjugated {
			c.report(w11eNameSpan(sym), msgConjugatedSpecific, "specialization-specific-conjugated")
		}
		c.checkPair(specific, sym, rel.Target)
	}
}

// checkRelationshipMember checks `subtype A specializes B`, whose specific type
// is named rather than owning the relationship.
func (c *specializationKindsChecker) checkRelationshipMember(sym *symbols.Symbol, rel *ast.RelationshipMember) {
	if rel.Kind != ast.RelSpecializes || rel.Conjugated || rel.Source == nil || rel.Target == nil {
		return
	}
	scope := w8cScopeOf(sym)
	specific, ok := c.resolver.ResolveTarget(scope, rel.Source)
	if !ok || specific == nil {
		return
	}
	if w11eHasConjugation(specific) {
		c.report(rel.Source.Span(), msgConjugatedSpecific, "specialization-specific-conjugated")
	}
	c.checkPairIn(scope, w11eMetaKindOf(specific), rel.Target)
}

func (c *specializationKindsChecker) checkPair(specific metaKind, sym *symbols.Symbol, target ast.Node) {
	c.checkPairIn(w8cScopeOf(sym), specific, target)
}

func (c *specializationKindsChecker) checkPairIn(scope *symbols.Scope, specific metaKind, target ast.Node) {
	if specific == 0 {
		return
	}
	general, ok := c.resolver.ResolveTarget(scope, target)
	if !ok || general == nil {
		return
	}
	if resolved, ok := c.resolver.ResolveAliasTarget(general); ok && resolved != nil {
		general = resolved
	}
	got := w11eMetaKindOf(general)
	switch {
	case specific&metaDataType != 0 && got&(metaClass|metaAssociation) != 0:
		c.report(target.Span(), msgSpecializeClassOrAssoc, "datatype-specialization")
	case specific&metaClass != 0 && (got&metaDataType != 0 ||
		(got&metaAssociation != 0 && specific&metaAssociation == 0)):
		c.report(target.Span(), msgSpecializeDataTypeOrAssoc, "class-specialization")
	}
	if specific&metaStructure != 0 && got&metaBehavior != 0 {
		c.report(target.Span(), msgSpecializeBehavior, "structure-specialization")
	}
	if specific&metaBehavior != 0 && got&metaStructure != 0 {
		c.report(target.Span(), msgSpecializeStructure, "behavior-specialization")
	}
}

// w11eMetaKindOf returns the KerML metaclasses sym instantiates, by the keyword
// it was declared with; a `classifier` or `type` constrains nothing.
func w11eMetaKindOf(sym *symbols.Symbol) metaKind {
	if sym == nil {
		return 0
	}
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		return kermlMetaKinds[d.Keyword]
	case *ast.Definition:
		return kermlMetaKinds[d.Keyword]
	}
	return 0
}

// w11eHasConjugation reports whether sym declares itself the conjugate of
// another type, which makes it unspecializable. A conjugation is recorded as a
// conjugated generalization edge.
func w11eHasConjugation(sym *symbols.Symbol) bool {
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel != nil && rel.Conjugated && rel.Kind == ast.RelSpecializes {
			return true
		}
	}
	return false
}

// w11eNameSpan returns the span of the declared name of sym, or of its whole
// declaration when it is unnamed.
func w11eNameSpan(sym *symbols.Symbol) source.Span {
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		if d.Ident.Name != "" {
			return d.Ident.NameSpan
		}
	case *ast.Definition:
		if d.Ident.Name != "" {
			return d.Ident.NameSpan
		}
	}
	return sym.Decl.Span()
}

func (c *specializationKindsChecker) report(span source.Span, msg, code string) {
	for _, have := range c.diags {
		if have.Span.Offset == span.Offset && have.Message == msg {
			return
		}
	}
	c.diags = append(c.diags, Diagnostic{
		Severity: SeverityError,
		Span:     span,
		Message:  msg,
		Code:     code,
		Source:   "constraint",
	})
}
