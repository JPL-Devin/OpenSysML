package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const msgW9CBoundFeatureTypes = "Bound features should have conforming types"

// W9CBoundFeatureTypesPass checks that the two features a binding connector
// binds have conforming types (SysML 8.3.3.1, KerML 8.3.4.3): binding features
// of unrelated types cannot be satisfied.
type W9CBoundFeatureTypesPass struct{}

func (W9CBoundFeatureTypesPass) Level() PassLevel { return LevelType }

func (W9CBoundFeatureTypesPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &w9cBindingChecker{model: ctx.Model(), resolver: ctx.Resolver(), idx: ctx.Index}
	if c.model == nil || c.resolver == nil {
		return nil
	}
	w8dWalkSymbols(ctx, rootScope, c.check)
	return c.diags
}

type w9cBindingChecker struct {
	model    *semantics.Model
	resolver *resolve.Resolver
	idx      *symbols.Index
	diags    []Diagnostic
}

func (c *w9cBindingChecker) check(sym *symbols.Symbol) {
	u, ok := sym.Decl.(*ast.Usage)
	if !ok || u.Kind != ast.UsageBinding || c.idx.Library(sym) {
		return
	}
	ends := c.bindingEnds(sym, u)
	if len(ends) != 2 {
		return
	}
	left, right := c.endTypes(ends[0]), c.endTypes(ends[1])
	if len(left) == 0 || len(right) == 0 || w9cTypesConform(c.model, left, right) {
		return
	}
	c.diags = append(c.diags, Diagnostic{
		Severity: SeverityWarning,
		Span:     u.Span(),
		Message:  msgW9CBoundFeatureTypes,
		Code:     "bound-feature-types",
		Source:   "type",
	})
}

// bindingEnds resolves the two features a `bind a = b;` names.
func (c *w9cBindingChecker) bindingEnds(sym *symbols.Symbol, u *ast.Usage) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelReferences || rel.Target == nil {
			continue
		}
		target, ok := c.resolver.ResolveTarget(w8cScopeOf(sym), rel.Target)
		if !ok || target == nil {
			return nil
		}
		out = append(out, target)
	}
	// The right-hand side of `bind a = b` is the usage's value, not a second end.
	if u.Value != nil {
		target, ok := c.resolver.ResolveTarget(w8cScopeOf(sym), u.Value)
		if !ok || target == nil {
			return nil
		}
		out = append(out, target)
	}
	return out
}

// endTypes returns an end's declared types, following a single untyped
// reference subsetting so `ref x ::> y` is judged by y's types.
func (c *w9cBindingChecker) endTypes(end *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, rel := range semantics.RelationshipsOf(end) {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		if t, ok := c.resolver.ResolveTarget(w8cScopeOf(end), rel.Target); ok && t != nil {
			out = append(out, t)
		}
	}
	return w8cMostSpecific(c.model, out)
}

// w9cTypesConform reports whether some type on one side conforms to one on the
// other: a binding needs only one compatible pairing.
func w9cTypesConform(model *semantics.Model, left, right []*symbols.Symbol) bool {
	for _, l := range left {
		for _, r := range right {
			if l == r || model.Conforms(l, r) || model.Conforms(r, l) {
				return true
			}
		}
	}
	return false
}
