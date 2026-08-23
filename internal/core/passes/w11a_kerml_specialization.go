package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Messages of the KerML classifier specialization rules (KerML 1.0 §8.4.4:
// validateDataTypeSpecialization, validateClassSpecialization,
// validateStructureSpecialization, validateBehaviorSpecialization).
const (
	msgW11ASpecializeClassOrAssoc    = "Cannot specialize class or association"
	msgW11ASpecializeDataTypeOrAssoc = "Cannot specialize data type or association"
	msgW11ASpecializeBehavior        = "Cannot specialize behavior"
	msgW11ASpecializeStructure       = "Cannot specialize structure"
)

// w11aFamily is the KerML metaclass family a type declaration belongs to. A
// declaration may belong to several: an `assoc struct` is both.
type w11aFamily struct {
	dataType    bool
	class       bool
	association bool
	structure   bool
	behavior    bool
}

func (f w11aFamily) any() bool {
	return f.dataType || f.class || f.association || f.structure || f.behavior
}

// w11aFamilies maps a KerML type keyword to its metaclass family: a Structure
// and a Behavior are Classes, an Interaction an Association and a Behavior, and
// an AssociationStructure an Association and a Structure (KerML 1.0 §8.4, §9.2).
var w11aFamilies = map[string]w11aFamily{
	"datatype":           {dataType: true},
	"class":              {class: true},
	"struct":             {class: true, structure: true},
	"assoc":              {association: true},
	"association":        {association: true},
	"assoc struct":       {association: true, class: true, structure: true},
	"association struct": {association: true, class: true, structure: true},
	"behavior":           {class: true, behavior: true},
	"function":           {class: true, behavior: true},
	"interaction":        {association: true, class: true, behavior: true},
	"predicate":          {class: true, behavior: true},
}

// W11AKerMLSpecializationPass checks the kind of a KerML classifier's
// supertypes: a data type specializes neither a class nor an association, a
// class neither a data type nor an association, and a structure and a behavior
// do not specialize each other (KerML 1.0 §8.4.4).
type W11AKerMLSpecializationPass struct{}

func (W11AKerMLSpecializationPass) Level() PassLevel { return LevelType }

func (W11AKerMLSpecializationPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil || ctx.Kind != source.KindKerML {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &w11aSpecializationChecker{resolver: ctx.Resolver()}
	w8dWalkSymbols(rootScope, c.check)
	return c.diags
}

type w11aSpecializationChecker struct {
	resolver *resolve.Resolver
	diags    []Diagnostic
}

func (c *w11aSpecializationChecker) check(sym *symbols.Symbol) {
	sub, ok := w11aFamilies[w11aKeywordOf(sym)]
	if !ok {
		return
	}
	for _, rel := range w11aRelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelSpecializes || rel.Target == nil {
			continue
		}
		target, ok := c.resolver.ResolveTarget(w8dScopeOf(sym), rel.Target)
		if !ok || target == nil {
			continue
		}
		if target.Kind == symbols.SymbolAlias {
			if resolved, ok := c.resolver.ResolveAliasTarget(target); ok && resolved != nil {
				target = resolved
			}
		}
		sup := w11aFamilies[w11aKeywordOf(target)]
		if !sup.any() {
			continue
		}
		if msg, bad := w11aSpecializationMessage(sub, sup); bad {
			c.diags = append(c.diags, Diagnostic{
				Severity: SeverityError,
				Span:     rel.Target.Span(),
				Message:  msg,
				Code:     "specialization-kind",
				Source:   "type",
			})
		}
	}
}

// w11aSpecializationMessage reports which rule a supertype of this family
// breaks, if any. An association may specialize an association whatever else it
// is, so the class rule exempts one.
func w11aSpecializationMessage(sub, sup w11aFamily) (string, bool) {
	if sub.dataType && (sup.class || sup.association) {
		return msgW11ASpecializeClassOrAssoc, true
	}
	if sub.class && !sub.association && (sup.dataType || sup.association) {
		return msgW11ASpecializeDataTypeOrAssoc, true
	}
	if sub.structure && sup.behavior && !sup.structure {
		return msgW11ASpecializeBehavior, true
	}
	if sub.behavior && sup.structure && !sup.behavior {
		return msgW11ASpecializeStructure, true
	}
	return "", false
}

// w11aKeywordOf is the kind keyword a declaration is written with.
func w11aKeywordOf(sym *symbols.Symbol) string {
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		return d.Keyword
	case *ast.Definition:
		return d.Keyword
	}
	return ""
}

// w11aRelationshipsOf are the relationships a type declaration states.
func w11aRelationshipsOf(sym *symbols.Symbol) []*ast.Relationship {
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		return d.Relationships
	case *ast.Definition:
		return d.Relationships
	}
	return nil
}
