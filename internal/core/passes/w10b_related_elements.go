package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// msgRelatedElements is the reference's message for an association or connector
// that relates fewer than two elements (pilot KerMLValidator checkAssociation,
// validateAssociationRelatedTypes).
const msgRelatedElements = "Must have at least two related elements"

// W10BRelatedElementsPass reports a concrete association or connector with
// fewer than two ends: a link needs two participants (KerML §8.4.3.4). An
// abstract declaration is exempt, as it is in the reference — its ends are the
// specialization's to supply.
type W10BRelatedElementsPass struct{}

func (W10BRelatedElementsPass) Level() PassLevel { return LevelConstraint }

func (W10BRelatedElementsPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	model := ctx.Model()
	var diags []Diagnostic
	w8dWalkSymbols(ctx, rootScope, func(sym *symbols.Symbol) {
		if !w10bRelatesElements(sym) {
			return
		}
		if w10bEndCount(model, sym, make(map[*symbols.Symbol]bool)) >= 2 {
			return
		}
		diags = append(diags, Diagnostic{
			Severity: SeverityError,
			Span:     sym.Decl.Span(),
			Message:  msgRelatedElements,
			Code:     "related-elements",
			Source:   "constraint",
		})
	})
	return diags
}

// w10bEndCount counts the ends of sym, its own or those it inherits from what
// it types by, subsets or redefines. A variation supplies its ends through the
// variant selected for it, so it counts as related.
func w10bEndCount(model *semantics.Model, sym *symbols.Symbol, visited map[*symbols.Symbol]bool) int {
	if sym == nil || visited[sym] {
		return 0
	}
	visited[sym] = true
	if semantics.DeclaresVariation(sym) {
		return 2
	}
	count := model.ConnectorEndCount(sym)
	for _, target := range model.DirectSupertypes(sym) {
		if count >= 2 {
			break
		}
		if inherited := w10bEndCount(model, target, visited); inherited > count {
			count = inherited
		}
	}
	return count
}

// w10bRelatesElements reports whether sym declares an association or connector
// whose ends this rule counts.
func w10bRelatesElements(sym *symbols.Symbol) bool {
	if sym == nil || semantics.DeclaresVariation(sym) {
		return false
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		if d.IsAbstract {
			return false
		}
		switch d.Kind {
		case ast.DefConnection, ast.DefInterface, ast.DefAllocation, ast.DefAssoc:
			return true
		}
	case *ast.Usage:
		if d.IsAbstract {
			return false
		}
		switch d.Kind {
		case ast.UsageConnection, ast.UsageInterface, ast.UsageAllocation:
			return true
		}
	}
	return false
}
