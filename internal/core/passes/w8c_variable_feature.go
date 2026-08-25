package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// KerMLValidator's validateFeatureIsVariable message.
const msgVariableFeatureOwner = "Must be owned by an occurrence type"

// KerMLValidator's validateFeaturePortionNotVariable message.
const msgPortionFeatureVariable = "A portion cannot be variable"

// VariableFeaturePass checks that a `var` feature is owned by an occurrence
// type (KerML 8.3.3.1.5, validateFeatureIsVariable): a variable feature is a
// snapshot-varying feature, which only an occurrence has.
type VariableFeaturePass struct{}

func (VariableFeaturePass) Level() PassLevel { return LevelConstraint }

func (VariableFeaturePass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	model := ctx.Model()
	occurrence := w8cLibraryType(ctx, "Occurrences::Occurrence")
	var diags []Diagnostic
	w := &w8cWalker{ctx: ctx, seen: make(map[*symbols.Symbol]bool)}
	w.walk(rootScope, func(sym *symbols.Symbol) {
		u, ok := sym.Decl.(*ast.Usage)
		if !ok || !u.IsVariable {
			return
		}
		if u.IsPortion {
			diags = append(diags, Diagnostic{
				Severity: SeverityError,
				Span:     u.Span(),
				Message:  msgPortionFeatureVariable,
				Code:     "feature-portion-not-variable",
				Source:   "constraint",
			})
		}
		if occurrence == nil || sym.OwnerScope == nil {
			return
		}
		owner := sym.OwnerScope.Owner()
		if owner != nil && model.Conforms(owner, occurrence) {
			return
		}
		diags = append(diags, Diagnostic{
			Severity: SeverityError,
			Span:     u.Span(),
			Message:  msgVariableFeatureOwner,
			Code:     "variable-feature-owner",
			Source:   "constraint",
		})
	})
	return diags
}

// w8cLibraryType returns the single library symbol for fqn, or nil.
func w8cLibraryType(ctx *Context, fqn string) *symbols.Symbol {
	if ctx == nil || ctx.Index == nil {
		return nil
	}
	syms := ctx.Index.LookupQualified(fqn)
	if len(syms) != 1 {
		return nil
	}
	return syms[0]
}
