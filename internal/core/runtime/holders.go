package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// holdingFeatures names the features of typ whose stated value may hold the named
// feature's objects, so their types classify them (KerML 1.0 §7.3.4.1); memoized per type.
func (ctx *Context) holdingFeatures(typ *symbols.Symbol, name string) []string {
	index, ok := ctx.holders[typ]
	if !ok {
		index = make(map[string][]string)
		features := ctx.FeaturesOf(typ)
		data := make(map[string]bool, len(features))
		for i := range features {
			data[features[i].Name] = ctx.holdsData(&features[i])
		}
		for i := range features {
			feat := &features[i]
			if feat.DefaultValue == nil || !ctx.valueBinds(feat) || data[feat.Name] {
				continue
			}
			for _, held := range ctx.mentionedFeatures(typ, feat) {
				if held != feat.Name && !data[held] {
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
	for _, ref := range ctx.passedFeatureReferences(scope, feat.DefaultValue) {
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

// passedFeatureReferences lists the feature references an expression written in scope
// may answer the values of as its own: a branch chosen, an element indexed or selected,
// an argument a call returns — not a chain's values, a condition or an operand computed from.
func (ctx *Context) passedFeatureReferences(scope *symbols.Scope, expr ast.Node) []*ast.FeatureReference {
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
				visit(ctx.returnedArguments(scope, n)...)
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

// returnedArguments lists the arguments of a call its result may consist of: those bound
// to parameters a calc's returns pass on; every one for a function whose body is not
// written in the model; none for a call that denotes nothing or computes no result.
func (ctx *Context) returnedArguments(scope *symbols.Scope, call *ast.InvocationExpr) []ast.Node {
	target := NewEvalContext(ctx, scope).invocationTarget(call)
	positional := call.Args
	if call.Operand != nil {
		positional = append([]ast.Node{call.Operand}, call.Args...)
	}
	var args []ast.Node
	switch {
	case target.shape != nil:
		passed := ctx.returnedParameters(target.shape)
		for i, arg := range positional {
			if i < len(target.shape.ParamNames) && passed[target.shape.ParamNames[i]] {
				args = append(args, arg)
			}
		}
		for _, named := range call.NamedArgs {
			if named.Name != nil && len(named.Name.Parts) != 0 && passed[named.Name.Parts[len(named.Name.Parts)-1].Text] {
				args = append(args, named.Value)
			}
		}
	case target.builtin != nil || target.library != nil:
		args = append(args, positional...)
		for _, named := range call.NamedArgs {
			args = append(args, named.Value)
		}
	}
	return args
}

// returnedParameters names the parameters of a calc whose values its result may consist
// of: those its return expressions pass on, a `for` variable standing for its collection.
// Memoized per shape; a recursive call reads the set as far as it is known.
func (ctx *Context) returnedParameters(shape *calcShape) map[string]bool {
	if passed, ok := ctx.returnedParams[shape]; ok {
		return passed
	}
	passed := make(map[string]bool)
	ctx.returnedParams[shape] = passed
	params := make(map[string]bool, len(shape.ParamNames))
	for _, name := range shape.ParamNames {
		params[name] = true
	}
	for {
		known := len(passed)
		ctx.collectReturnedParameters(shape, passed, params)
		if len(passed) == known {
			return passed
		}
	}
}

// collectReturnedParameters adds to passed the parameters of shape the returns of its body
// and its result binding pass on.
func (ctx *Context) collectReturnedParameters(shape *calcShape, passed, params map[string]bool) {
	type loopVariable struct {
		scope      *symbols.Scope
		collection ast.Node
	}
	variables := make(map[*symbols.Symbol]loopVariable)
	var follow func(scope *symbols.Scope, expr ast.Node)
	follow = func(scope *symbols.Scope, expr ast.Node) {
		if scope == nil {
			return
		}
		for _, ref := range ctx.passedFeatureReferences(scope, expr) {
			sym := ctx.referencedSymbol(scope, ref.Name)
			if sym == nil {
				continue
			}
			if variable, ok := variables[sym]; ok {
				follow(variable.scope, variable.collection)
				continue
			}
			if name, ok := ctx.denotedFeature(shape.Sym, sym); ok && params[name] {
				passed[name] = true
			}
		}
	}
	var walk func(stmts []lower.Statement)
	walk = func(stmts []lower.Statement) {
		for _, stmt := range stmts {
			switch s := stmt.(type) {
			case lower.Return:
				follow(s.Scope, s.Value)
			case lower.If:
				walk(s.Then.Steps())
				if s.Else != nil {
					walk(s.Else.Steps())
				}
			case lower.Loop:
				if s.Variable != "" {
					if sym, ok := ctx.resolver.LookupName(s.Body.Scope, s.Variable); ok {
						variables[sym] = loopVariable{scope: s.Scope, collection: s.Collection}
					}
				}
				walk(s.Body.Steps())
			case lower.Block:
				walk(s.Steps())
			}
		}
	}
	walk(shape.Body)
	for _, binding := range shape.Bindings {
		for i := range binding.Ends {
			if binding.Ends[i].Path == "result" && binding.Ends[1-i].Expr != nil {
				follow(binding.Scope, binding.Ends[1-i].Expr)
			}
		}
	}
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
