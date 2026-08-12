package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// condition is one boolean check a constraint or requirement states, with the
// scope its expression resolves names in.
type condition struct {
	expr     ast.Node
	scope    *symbols.Scope
	negated  bool
	required bool // an assumption is trusted rather than required to hold
}

// scopedExpr is an expression with the scope its names resolve in.
type scopedExpr struct {
	expr  ast.Node
	scope *symbols.Scope
}

// conditionsOf returns the conditions the members state, inherited ones first.
// A member states its condition either directly (`require x < y;`, `assert x < y;`)
// or through the body of an anonymous nested constraint (`require constraint { x < y }`).
func conditionsOf(members []scopedMember) []condition {
	var out []condition
	for _, member := range members {
		out = appendConditions(out, member.node, member.scope, true)
	}
	return out
}

// appendConditions appends the conditions node states. required says whether the
// enclosing member requires them to hold or only assumes them.
func appendConditions(out []condition, node ast.Node, scope *symbols.Scope, required bool) []condition {
	switch m := node.(type) {
	case *ast.ConstraintMember:
		if m.Expression != nil {
			out = append(out, condition{expr: m.Expression, scope: scope, negated: m.IsNegated, required: required && m.IsAssert})
		}
		for _, nested := range m.Body {
			out = appendConditions(out, nested, scope, required && m.IsAssert)
		}
	case *ast.RequireMember:
		if m.Expression != nil {
			out = append(out, condition{expr: m.Expression, scope: scope, required: true})
		}
		for _, nested := range m.Body {
			out = appendConditions(out, nested, scope, true)
		}
	case *ast.AssumeMember:
		if m.Expression != nil {
			out = append(out, condition{expr: m.Expression, scope: scope})
		}
		for _, nested := range m.Body {
			out = appendConditions(out, nested, scope, false)
		}
	}
	return out
}

// evaluateConditions evaluates conds in order and reports whether every required
// one holds. kind and what name the checked element and its conditions in
// messages ("constraint"/"assertion", "requirement"/"require condition").
// bindings, when non-nil, are the names the element binds itself (subject, actor).
func (ctx *Context) evaluateConditions(sym *symbols.Symbol, kind, what string, conds []condition, self *Instance, bindings map[string]Value) (bool, error) {
	if len(conds) == 0 {
		return false, fmt.Errorf("%s %s: %w", kind, sym.Name, ErrNoConditions)
	}
	features := ctx.conditionFeatures(sym)
	for _, cond := range conds {
		ec := NewEvalContextIn(ctx, cond.scope, self)
		ec.features = features
		if bindings != nil {
			ec.Push(bindings)
		}
		result, err := ec.Eval(cond.expr)
		if err != nil {
			return false, fmt.Errorf("%s %s: %s evaluation failed: %w", kind, sym.Name, what, err)
		}
		if result.Kind != ValConst || result.Const.Kind != semantics.ValBool {
			return false, fmt.Errorf("%s %s: %s must evaluate to boolean, got %v", kind, sym.Name, what, result.Kind)
		}
		holds := result.Const.Bool
		if cond.negated {
			holds = !holds
		}
		if cond.required && !holds {
			return false, &ViolationError{Kind: kind, Element: sym.Name, What: what, Condition: conditionText(cond.expr)}
		}
	}
	return true, nil
}

// conditionFeatures returns the features the conditions of sym may name: its
// own, the ones it inherits, and the ones a typed usage rebinds, which mask the
// declaration they redefine. A feature carrying no value maps to a nil
// expression, so naming it reports an uninitialized feature rather than an
// unresolved one.
func (ctx *Context) conditionFeatures(sym *symbols.Symbol) map[string]scopedExpr {
	features := ctx.FeaturesOf(sym)
	if len(features) == 0 {
		return nil
	}
	out := make(map[string]scopedExpr, len(features))
	for _, feat := range features {
		if feat.Name == "" {
			continue
		}
		out[feat.Name] = scopedExpr{expr: feat.DefaultValue, scope: feat.DeclScope()}
	}
	return out
}

// conditionText renders a condition compactly, so a violation names the
// condition that failed rather than only the element that states it.
func conditionText(n ast.Node) string {
	switch e := n.(type) {
	case *ast.LiteralInteger:
		return e.Value
	case *ast.LiteralReal:
		return e.Value
	case *ast.LiteralString:
		return e.Value
	case *ast.LiteralBool:
		if e.Value {
			return "true"
		}
		return "false"
	case *ast.FeatureReference:
		return qualifiedNameToString(e.Name)
	case *ast.FeatureChainExpr:
		return conditionText(e.Operand) + "." + qualifiedNameToString(e.Member)
	case *ast.OperatorExpr:
		switch len(e.Operands) {
		case 1:
			return e.Operator.String() + " " + conditionText(e.Operands[0])
		case 2:
			return conditionText(e.Operands[0]) + " " + e.Operator.String() + " " + conditionText(e.Operands[1])
		}
	case *ast.InvocationExpr:
		args := make([]string, 0, len(e.Args))
		for _, arg := range e.Args {
			args = append(args, conditionText(arg))
		}
		return qualifiedNameToString(e.Type) + "(" + strings.Join(args, ", ") + ")"
	}
	return TraceLabel(n)
}
