package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// w8cWalker visits every symbol of a document's scope tree once, deduping by
// pointer: one declaration may be registered under several keys.
type w8cWalker struct {
	ctx *Context
	// walked is what the first walk visited, which w8cSymbols already dedupes;
	// seen is built only when a later walk must dedupe against it.
	walked []*symbols.Symbol
	seen   map[*symbols.Symbol]bool
}

func (w *w8cWalker) walk(scope *symbols.Scope, visit func(*symbols.Symbol)) {
	if w == nil || scope == nil {
		return
	}
	syms := w8cSymbols(w.ctx, scope)
	if w.walked == nil {
		w.walked = syms
		for _, sym := range syms {
			visit(sym)
		}
		return
	}
	if w.seen == nil {
		w.seen = make(map[*symbols.Symbol]bool, len(w.walked))
		for _, sym := range w.walked {
			w.seen[sym] = true
		}
	}
	for _, sym := range syms {
		if w.seen[sym] {
			continue
		}
		w.seen[sym] = true
		visit(sym)
	}
}

// w8cSymbols lists the scope tree's symbols once each, in visiting order, and
// caches the list on ctx for the passes that share it.
func w8cSymbols(ctx *Context, root *symbols.Scope) []*symbols.Symbol {
	if ctx != nil {
		if cached, ok := ctx.w8cCache[root]; ok {
			return cached
		}
	}
	out := w8cCollectSymbols(root)
	if ctx != nil {
		if ctx.w8cCache == nil {
			ctx.w8cCache = make(map[*symbols.Scope][]*symbols.Symbol)
		}
		ctx.w8cCache[root] = out
	}
	return out
}

func w8cCollectSymbols(root *symbols.Scope) []*symbols.Symbol {
	seen := make(map[*symbols.Symbol]bool)
	var out []*symbols.Symbol
	var walk func(*symbols.Scope)
	walk = func(scope *symbols.Scope) {
		if scope == nil {
			return
		}
		scope.ForEachMember(func(sym *symbols.Symbol) bool {
			if sym == nil || seen[sym] {
				return true
			}
			seen[sym] = true
			out = append(out, sym)
			walk(sym.Scope)
			return true
		})
	}
	walk(root)
	return out
}

// w8cScopeOf returns the scope a declaration's own references resolve in.
func w8cScopeOf(sym *symbols.Symbol) *symbols.Scope {
	if sym == nil {
		return nil
	}
	if sym.OwnerScope != nil {
		return sym.OwnerScope
	}
	return sym.Scope
}

// w8cMultiplicityOf returns the multiplicity a declaration declares, or nil.
func w8cMultiplicityOf(sym *symbols.Symbol) *ast.Multiplicity {
	if sym == nil {
		return nil
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.Multiplicity
	case *ast.ConnectorEnd:
		return d.Multiplicity
	default:
		return semantics.UsageMultiplicityOf(sym)
	}
}

// w8cIsReference reports whether n names a feature rather than computing a value.
func w8cIsReference(n ast.Node) bool {
	switch n.(type) {
	case *ast.QualifiedName, *ast.FeatureReference, *ast.FeatureChainExpr:
		return true
	default:
		return false
	}
}

// w8cEvalConst evaluates n as a model-level constant, reading through features
// it names to their bound values. seen guards a value that names itself.
func w8cEvalConst(r *resolve.Resolver, m *semantics.Model, scope *symbols.Scope, n ast.Node, seen map[*symbols.Symbol]bool) (semantics.Value, bool) {
	if n == nil || m == nil {
		return semantics.Value{}, false
	}
	switch e := n.(type) {
	case *ast.QualifiedName, *ast.FeatureReference, *ast.FeatureChainExpr:
		return w8cEvalFeature(r, m, scope, n, seen)
	case *ast.OperatorExpr:
		return w8cEvalOperator(r, m, scope, e, seen)
	default:
		return m.Eval(n)
	}
}

// w8cEvalFeature evaluates the value a named feature is bound to.
func w8cEvalFeature(r *resolve.Resolver, m *semantics.Model, scope *symbols.Scope, ref ast.Node, seen map[*symbols.Symbol]bool) (semantics.Value, bool) {
	if r == nil {
		return semantics.Value{}, false
	}
	sym, ok := r.ResolveTarget(scope, ref)
	if !ok || sym == nil || seen[sym] {
		return semantics.Value{}, false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Value == nil {
		return semantics.Value{}, false
	}
	next := make(map[*symbols.Symbol]bool, len(seen)+1)
	for s := range seen {
		next[s] = true
	}
	next[sym] = true
	return w8cEvalConst(r, m, w8cScopeOf(sym), usage.Value, next)
}

// w8cEvalOperator evaluates an operator expression over evaluable operands.
func w8cEvalOperator(r *resolve.Resolver, m *semantics.Model, scope *symbols.Scope, e *ast.OperatorExpr, seen map[*symbols.Symbol]bool) (semantics.Value, bool) {
	switch e.Operator {
	case ast.OpNeg, ast.OpPos, ast.OpNot:
		if len(e.Operands) != 1 {
			return semantics.Value{}, false
		}
		v, ok := w8cEvalConst(r, m, scope, e.Operands[0], seen)
		if !ok {
			return semantics.Value{}, false
		}
		return semantics.EvalUnary(e.Operator, v)
	case ast.OpConditional:
		if len(e.Operands) != 3 {
			return semantics.Value{}, false
		}
		cond, ok := w8cEvalConst(r, m, scope, e.Operands[0], seen)
		if !ok || cond.Kind != semantics.ValBool {
			return semantics.Value{}, false
		}
		if cond.Bool {
			return w8cEvalConst(r, m, scope, e.Operands[1], seen)
		}
		return w8cEvalConst(r, m, scope, e.Operands[2], seen)
	default:
		if len(e.Operands) != 2 {
			return semantics.Value{}, false
		}
		l, lok := w8cEvalConst(r, m, scope, e.Operands[0], seen)
		rv, rok := w8cEvalConst(r, m, scope, e.Operands[1], seen)
		if !lok || !rok {
			return semantics.Value{}, false
		}
		return semantics.EvalBinary(e.Operator, l, rv)
	}
}

// w8cChainStep is one chaining feature: the node resolving to it and the span
// of the segment naming it.
type w8cChainStep struct {
	Node ast.Node
	Span source.Span
}

// w8cChainSteps splits a chain target into chaining features: only a `.` starts
// a new one, a `::`-qualified name is a single chaining feature.
func w8cChainSteps(target ast.Node) []w8cChainStep {
	switch t := target.(type) {
	case nil:
		return nil
	case *ast.FeatureReference:
		if t.Name == nil {
			return nil
		}
		return []w8cChainStep{{Node: t, Span: t.Name.Span()}}
	case *ast.FeatureChainExpr:
		steps := w8cChainSteps(t.Operand)
		span := t.Span()
		if t.Member != nil {
			span = t.Member.Span()
		}
		return append(steps, w8cChainStep{Node: t, Span: span})
	default:
		return []w8cChainStep{{Node: target, Span: target.Span()}}
	}
}
