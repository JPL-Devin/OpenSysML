package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// KerMLValidator's validateFeatureIsVariable message.
const msgVariableFeatureOwner = "Must be owned by an occurrence type"

// KerMLValidator's validateFeaturePortionNotVariable message.
const msgPortionFeatureVariable = "A portion cannot be variable"

// KerMLValidator's validateFeatureValueIsInitial message.
const msgInitialValueNotVariable = "Initialized feature must be variable"

// KerMLValidator's validateFeatureConstantIsVariable message.
const msgConstantNotVariable = "Only a variable feature can be constant"

// VariableFeaturePass checks feature variability (KerML 8.3.3.1.5): a `var` feature is owned
// by an occurrence type and is no portion; only a variable feature is initialized or constant.
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
	report := func(span source.Span, message, code string) {
		diags = append(diags, Diagnostic{
			Severity: SeverityError,
			Span:     span,
			Message:  message,
			Code:     code,
			Source:   "constraint",
		})
	}
	w := &w8cWalker{ctx: ctx}
	w.walk(rootScope, func(sym *symbols.Symbol) {
		u, ok := sym.Decl.(*ast.Usage)
		if !ok {
			return
		}
		if u.ValueIsInitial && u.Value != nil && !model.FeatureIsVariable(sym) {
			report(w8cValueSpan(u), msgInitialValueNotVariable, "initial-value-not-variable")
		}
		if u.IsConstant && !model.FeatureIsVariable(sym) {
			report(u.Span(), msgConstantNotVariable, "constant-feature-not-variable")
		}
		if !u.IsVariable {
			return
		}
		if u.IsPortion {
			report(u.Span(), msgPortionFeatureVariable, "feature-portion-not-variable")
		}
		if occurrence == nil || sym.OwnerScope == nil {
			return
		}
		owner := sym.OwnerScope.Owner()
		if owner != nil && model.Conforms(owner, occurrence) {
			return
		}
		report(u.Span(), msgVariableFeatureOwner, "variable-feature-owner")
	})
	return diags
}

// w8cValueSpan is the span of a usage's feature value, operator through expression.
func w8cValueSpan(u *ast.Usage) source.Span {
	end := u.Value.Span().End()
	if u.ValueOperatorSpan.Len == 0 || end < u.ValueOperatorSpan.Offset {
		return u.Value.Span()
	}
	return source.Span{Offset: u.ValueOperatorSpan.Offset, Len: end - u.ValueOperatorSpan.Offset}
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
