package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const msgFeatureNoType = "Features must have at least one type"

// ImplicitBasePass checks that a KerML classifier reaches the standard-library
// supertype implied by its kind and that a feature has at least one type
// (KerML 1.0 validateClassifierDefaultSupertype, validateFeatureHasType). Both
// hold implicitly once the library is in the resource set, so they report a
// model whose declared resources do not supply it.
type ImplicitBasePass struct{}

func (ImplicitBasePass) Level() PassLevel { return LevelConstraint }

func (ImplicitBasePass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil || ctx.Kind != source.KindKerML {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &implicitBaseChecker{model: ctx.Model(), index: ctx.Index}
	(&w8cWalker{seen: make(map[*symbols.Symbol]bool)}).walk(rootScope, c.check)
	return c.diags
}

type implicitBaseChecker struct {
	model *semantics.Model
	index *symbols.Index
	diags []Diagnostic
}

func (c *implicitBaseChecker) check(sym *symbols.Symbol) {
	if _, ok := sym.Decl.(*ast.Usage); !ok {
		if _, ok := sym.Decl.(*ast.Definition); !ok {
			return
		}
	}
	if sym.Kind == symbols.SymbolKerMLType {
		c.checkDefaultSupertype(sym)
		return
	}
	c.checkFeatureHasType(sym)
}

// checkDefaultSupertype reports a type whose kind's library supertype cannot be
// reached: the library element is absent from the model, or the type conjugates
// itself, so neither the conjugated type nor the implicit specialization it
// replaces supplies one.
func (c *implicitBaseChecker) checkDefaultSupertype(sym *symbols.Symbol) {
	fqn, ok := semantics.KindBaseFQN(sym, true)
	if !ok {
		return
	}
	base := c.libraryType(fqn)
	if base != nil && (!c.selfConjugating(sym) || c.reaches(sym, base)) {
		return
	}
	c.report(sym.Decl.Span(), "Must directly or indirectly specialize "+fqn, "classifier-default-supertype")
}

// checkFeatureHasType reports a feature no type reaches, directly or through
// what it specializes or the base feature its kind implies.
func (c *implicitBaseChecker) checkFeatureHasType(sym *symbols.Symbol) {
	if c.hasType(sym) {
		return
	}
	for _, sup := range c.model.AllSupertypes(sym) {
		if c.hasType(sup) {
			return
		}
	}
	// A feature with no declared type takes the one its implicit base feature
	// carries, so it lacks a type only when that base is absent from the model or
	// conjugation stands in for the implicit subsetting that would supply it.
	fqn, ok := c.model.FeatureBaseFQN(sym)
	if !ok {
		return
	}
	if c.libraryType(fqn) != nil && !c.selfConjugating(sym) {
		return
	}
	c.report(sym.Decl.Span(), msgFeatureNoType, "feature-has-type")
}

// selfConjugating reports whether sym's conjugation chain returns to sym, which
// leaves it with neither a conjugated type nor the implicit specialization that
// conjugation replaces.
func (c *implicitBaseChecker) selfConjugating(sym *symbols.Symbol) bool {
	seen := map[*symbols.Symbol]bool{}
	for cur := sym; cur != nil && !seen[cur]; {
		seen[cur] = true
		next := c.conjugationTarget(cur)
		if next == sym {
			return true
		}
		cur = next
	}
	return false
}

// conjugationTarget returns the type sym conjugates, or nil.
func (c *implicitBaseChecker) conjugationTarget(sym *symbols.Symbol) *symbols.Symbol {
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || !rel.Conjugated {
			continue
		}
		if target := c.model.RelationshipTarget(sym, rel); target != nil {
			return target
		}
	}
	return nil
}

// reaches reports whether base is in sym's supertype closure.
func (c *implicitBaseChecker) reaches(sym, base *symbols.Symbol) bool {
	for _, sup := range c.model.AllSupertypes(sym) {
		if sup == base {
			return true
		}
	}
	return false
}

func (c *implicitBaseChecker) hasType(sym *symbols.Symbol) bool {
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelTyping {
			continue
		}
		if c.model.RelationshipTarget(sym, rel) != nil {
			return true
		}
	}
	return false
}

func (c *implicitBaseChecker) libraryType(fqn string) *symbols.Symbol {
	for _, sym := range c.index.LookupQualified(fqn) {
		if sym != nil {
			return sym
		}
	}
	return nil
}

func (c *implicitBaseChecker) report(span source.Span, msg, code string) {
	c.diags = append(c.diags, Diagnostic{
		Severity: SeverityError,
		Span:     span,
		Message:  msg,
		Code:     code,
		Source:   "constraint",
	})
}
