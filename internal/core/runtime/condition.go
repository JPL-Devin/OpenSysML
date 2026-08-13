package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// condition is one boolean check a constraint or requirement states, with the
// scope its expression resolves names in. A condition states either an
// expression or a group, which holds when all of its conditions hold.
type condition struct {
	expr     ast.Node
	scope    *symbols.Scope
	group    []condition
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
		out = appendConditions(out, member.node, member.scope, true, false)
	}
	return out
}

// appendConditions appends the conditions node states. required says whether the
// enclosing member requires them to hold or only assumes them; negated is the
// negation the enclosing member wrote, which a nested body inherits.
func appendConditions(out []condition, node ast.Node, scope *symbols.Scope, required, negated bool) []condition {
	switch m := node.(type) {
	case *ast.ConstraintMember:
		negated = negated != m.IsNegated
		required = required && m.IsAssert
		if m.Expression != nil {
			out = append(out, condition{expr: m.Expression, scope: scope, negated: negated, required: required})
		}
		if len(m.Body) == 0 {
			return out
		}
		var body []condition
		for _, nested := range m.Body {
			body = appendConditions(body, nested, scope, true, false)
		}
		if !negated {
			for _, c := range body {
				c.required = c.required && required
				out = append(out, c)
			}
			return out
		}
		// A body means the conjunction of its conditions, so negating it negates
		// that conjunction rather than each condition (De Morgan). A conjunction
		// of one is that one condition.
		if len(body) == 1 {
			only := body[0]
			only.negated = !only.negated
			only.required = only.required && required
			return append(out, only)
		}
		out = append(out, condition{group: body, negated: true, required: required})
	case *ast.RequireMember:
		if m.Expression != nil {
			out = append(out, condition{expr: m.Expression, scope: scope, required: true})
		}
		for _, nested := range m.Body {
			out = appendConditions(out, nested, scope, true, false)
		}
	case *ast.AssumeMember:
		if m.Expression != nil {
			out = append(out, condition{expr: m.Expression, scope: scope})
		}
		for _, nested := range m.Body {
			out = appendConditions(out, nested, scope, false, false)
		}
	}
	return out
}

// conditionCheck is one evaluation of the conditions an element states: the
// element itself, how it is named in messages, and what its conditions are
// evaluated against.
type conditionCheck struct {
	sym  *symbols.Symbol
	kind string // "constraint", "requirement", "satisfaction"
	what string // "assertion", "require condition"

	// element names the checked element in messages. Empty takes sym's name,
	// which an anonymous declaration such as a satisfaction assertion lacks.
	element string

	// self is the object a feature name resolves against, nil when unbound.
	self *Instance

	// bindings are the names the element binds itself (subject, actor); nil
	// binds nothing.
	bindings map[string]Value

	// negated inverts the verdict: the element asserts that its required
	// conditions do not all hold (`assert not …`, Invariant::isNegated).
	negated bool
}

// name returns how the checked element is named in messages.
func (c conditionCheck) name() string {
	if c.element != "" {
		return c.element
	}
	return c.sym.Name
}

// evaluateConditions evaluates conds in order and reports whether every required
// one holds, or — for a negated element — whether one of them fails.
func (ctx *Context) evaluateConditions(check conditionCheck, conds []condition) (bool, error) {
	if len(conds) == 0 {
		return false, fmt.Errorf("%s %s: %w", check.kind, check.name(), ErrNoConditions)
	}
	features := ctx.conditionFeatures(check.sym)
	required := false
	for _, cond := range conds {
		required = required || cond.required
		holds, err := ctx.conditionHolds(cond, features, check.self, check.bindings)
		if err != nil {
			return false, fmt.Errorf("%s %s: %s evaluation failed: %w", check.kind, check.name(), check.what, err)
		}
		if cond.required && !holds {
			// A negated element asserts exactly this: one required condition
			// failing makes the negated assertion hold.
			if check.negated {
				return true, nil
			}
			return false, &ViolationError{Kind: check.kind, Element: check.name(), What: check.what, Condition: conditionLabel(cond)}
		}
	}
	if check.negated {
		// An assumption is trusted rather than checked, so a negated element
		// stating only assumptions denies nothing.
		if !required {
			return false, fmt.Errorf("%s %s: %w", check.kind, check.name(), ErrNoConditions)
		}
		return false, &ViolationError{Kind: check.kind, Element: check.name(), What: check.what, Condition: negatedText(conds)}
	}
	return true, nil
}

// conditionHolds evaluates one condition: an expression, or a group that holds
// when all of its conditions hold. Its negation, if any, is applied last.
func (ctx *Context) conditionHolds(cond condition, features map[string]scopedExpr, self *Instance, bindings map[string]Value) (bool, error) {
	holds := true
	if cond.group != nil {
		for _, sub := range cond.group {
			subHolds, err := ctx.conditionHolds(sub, features, self, bindings)
			if err != nil {
				return false, err
			}
			holds = holds && subHolds
		}
	} else {
		ec := NewEvalContextIn(ctx, cond.scope, self)
		ec.features = features
		if bindings != nil {
			ec.Push(bindings)
		}
		result, err := ec.Eval(cond.expr)
		if err != nil {
			return false, err
		}
		if result.Kind != ValConst || result.Const.Kind != semantics.ValBool {
			return false, fmt.Errorf("condition must evaluate to boolean, got %v", result.Kind)
		}
		holds = result.Const.Bool
	}
	if cond.negated {
		holds = !holds
	}
	return holds, nil
}

// negatedText renders what a negated element asserted and did not get: that not
// every required condition holds.
func negatedText(conds []condition) string {
	var texts []string
	for _, cond := range conds {
		if !cond.required {
			continue
		}
		texts = append(texts, conditionLabel(cond))
	}
	if len(texts) == 1 {
		return "not " + texts[0]
	}
	return "not (" + strings.Join(texts, " and ") + ")"
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

// conditionLabel renders a condition as written, so a violation names the
// condition that failed, negation and grouping included.
func conditionLabel(cond condition) string {
	text := conditionText(cond.expr)
	if cond.group != nil {
		parts := make([]string, 0, len(cond.group))
		for _, sub := range cond.group {
			parts = append(parts, conditionLabel(sub))
		}
		text = "{ " + strings.Join(parts, "; ") + " }"
	}
	if cond.negated {
		text = "not " + text
	}
	return text
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
	case *ast.IndexExpr:
		// The bracket form is a quantity, `1.0 [m]`; `#` indexes a sequence.
		if e.Bracket {
			unit := semantics.UnitExprText(e.Index)
			if unit == "" {
				unit = conditionText(e.Index)
			}
			return conditionText(e.Operand) + " [" + unit + "]"
		}
		return conditionText(e.Operand) + "#(" + conditionText(e.Index) + ")"
	}
	return TraceLabel(n)
}
