package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// holdingFeatures names the features of typ whose stated value may hold the named
// feature's objects, so their types classify them (KerML 1.0 §7.3.4.1); memoized per type.
func (ctx *Context) holdingFeatures(typ *symbols.Symbol, name string) []string {
	index, ok := ctx.holders[typ]
	if !ok {
		index = make(map[string][]string)
		for _, feat := range ctx.FeaturesOf(typ) {
			if feat.DefaultValue == nil || !ctx.valueBinds(&feat) || ctx.holdsData(&feat) {
				continue
			}
			for _, held := range ctx.mentionedFeatures(typ, &feat) {
				if held != feat.Name {
					index[held] = append(index[held], feat.Name)
				}
			}
		}
		ctx.holders[typ] = index
	}
	return index[name]
}

// holdsData reports a feature whose values are data, never objects: an attribute, or
// one typed by a data type. It classifies nothing.
func (ctx *Context) holdsData(feat *EffectiveFeature) bool {
	if feat.Symbol != nil {
		if usage, ok := feat.Symbol.Decl.(*ast.Usage); ok && usage.Kind == ast.UsageAttribute {
			return true
		}
	}
	return ctx.model.IsDataType(feat.Type)
}

// mentionedFeatures names the features of typ whose values feat's value may pass
// on as its own, wherever its expression refers to them.
func (ctx *Context) mentionedFeatures(typ *symbols.Symbol, feat *EffectiveFeature) []string {
	scope := feat.DefaultScope()
	if scope == nil {
		return nil
	}
	var names []string
	seen := make(map[string]bool)
	for _, ref := range passedFeatureReferences(feat.DefaultValue) {
		sym := ctx.referencedSymbol(scope, ref.Name)
		if sym == nil {
			continue
		}
		if name, ok := ctx.denotedFeature(typ, sym); ok && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

// passedFeatureReferences lists the feature references an expression may answer the
// values of as its own: a branch chosen, an element indexed or selected, a receiver or
// argument a function may return — not a chain's values, a condition or an operand computed from.
func passedFeatureReferences(expr ast.Node) []*ast.FeatureReference {
	var refs []*ast.FeatureReference
	var visit func(nodes ...ast.Node)
	visit = func(nodes ...ast.Node) {
		for _, n := range nodes {
			switch n := n.(type) {
			case *ast.FeatureReference:
				refs = append(refs, n)
			case *ast.SequenceExpr:
				visit(n.Elements...)
			case *ast.OperatorExpr:
				switch n.Operator {
				case ast.OpConditional:
					if len(n.Operands) > 1 {
						visit(n.Operands[1:]...)
					}
				case ast.OpNullCoalesce, ast.OpAs:
					visit(n.Operands...)
				}
			case *ast.IndexExpr:
				visit(n.Operand)
			case *ast.InvocationExpr:
				visit(n.Operand)
				visit(n.Args...)
				visit(namedArgValues(n.NamedArgs)...)
			case *ast.CollectExpr:
				visit(n.Body)
			case *ast.SelectExpr:
				visit(n.Operand)
			case *ast.BodyExpr:
				visit(n.Result)
			case *ast.Usage:
				visit(n.Value)
			}
		}
	}
	visit(expr)
	return refs
}

// namedArgValues lists the value expressions of named arguments.
func namedArgValues(args []ast.NamedArg) []ast.Node {
	values := make([]ast.Node, 0, len(args))
	for _, a := range args {
		values = append(values, a.Value)
	}
	return values
}

// referencedSymbol resolves a name the way the evaluator does: a single part by
// lookup in the scope, more through the qualified-name reader.
func (ctx *Context) referencedSymbol(scope *symbols.Scope, qn *ast.QualifiedName) *symbols.Symbol {
	if qn == nil || len(qn.Parts) == 0 {
		return nil
	}
	if len(qn.Parts) == 1 && !qn.Global {
		if sym, ok := ctx.resolver.LookupName(scope, qn.Parts[0].Text); ok {
			return sym
		}
		return nil
	}
	sym, _ := ctx.resolver.ReadQualified(scope, qn).Symbol()
	return sym
}

// materializeHolders reads the features of inst whose value lists the named feature before its
// objects are made, so every holding feature classifies them whichever is read first.
func (ctx *Context) materializeHolders(inst *Instance, name string) error {
	for _, typ := range inst.types() {
		for _, holder := range ctx.holdingFeatures(typ, name) {
			fv, ok := inst.FeatureValues[holder]
			if !ok || fv.Materialized ||
				ctx.derivingFeatureValues[featureValueRef{instance: inst.ID, feature: holder}] {
				continue
			}
			if _, err := inst.GetFeatureValue(ctx, holder); err != nil {
				return fmt.Errorf("feature %s holding %s: %w", holder, name, err)
			}
		}
	}
	return nil
}
