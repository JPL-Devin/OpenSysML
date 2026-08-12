package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// condition is one boolean check a constraint or requirement states, with the
// scope its expression resolves names in. A condition with a group holds when
// every condition in it does, which is what a negation denies as a whole
// (Invariant::isNegated denies the constraint, not each of its conditions).
type condition struct {
	expr     ast.Node
	scope    *symbols.Scope
	negated  bool
	required bool // an assumption is trusted rather than required to hold
	group    []condition
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
// enclosing member requires them to hold or only assumes them. A negation denies
// the conditions its member states together rather than one by one.
func appendConditions(out []condition, node ast.Node, scope *symbols.Scope, required bool) []condition {
	switch m := node.(type) {
	case *ast.ConstraintMember:
		required = required && m.IsAssert
		var stated []condition
		if m.Expression != nil {
			stated = append(stated, condition{expr: m.Expression, scope: scope, required: required})
		}
		for _, nested := range m.Body {
			stated = appendConditions(stated, nested, scope, required)
		}
		switch {
		case !m.IsNegated:
			out = append(out, stated...)
		case len(stated) == 1:
			stated[0].negated = !stated[0].negated
			out = append(out, stated[0])
		case len(stated) > 1:
			out = append(out, condition{scope: scope, negated: true, required: required, group: stated})
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
	for _, cond := range conds {
		// An assumption is evaluated too — a broken one is still an error — but
		// only a required condition has to hold.
		holds, err := ctx.conditionHolds(check, cond, features)
		if err != nil {
			return false, err
		}
		if cond.required && !holds {
			// A negated element asserts exactly this: one required condition
			// failing makes the negated assertion hold.
			if check.negated {
				return true, nil
			}
			return false, &ViolationError{Kind: check.kind, Element: check.name(), What: check.what, Condition: conditionTextOf(cond)}
		}
	}
	if check.negated {
		return false, &ViolationError{
			Kind:      check.kind,
			Element:   check.name(),
			What:      check.what,
			Condition: negatedText(conds),
		}
	}
	return true, nil
}

// conditionHolds reports whether one condition holds, its own negation applied.
// A condition stating a group holds when every required condition in the group
// does, so denying the group denies their conjunction.
func (ctx *Context) conditionHolds(check conditionCheck, cond condition, features map[string]scopedExpr) (bool, error) {
	holds := true
	if cond.group != nil {
		for _, nested := range cond.group {
			nestedHolds, err := ctx.conditionHolds(check, nested, features)
			if err != nil {
				return false, err
			}
			if nested.required && !nestedHolds {
				holds = false
			}
		}
	} else {
		ec := NewEvalContextIn(ctx, cond.scope, check.self)
		ec.features = features
		if check.bindings != nil {
			ec.Push(check.bindings)
		}
		result, err := ec.Eval(cond.expr)
		if err != nil {
			return false, fmt.Errorf("%s %s: %s evaluation failed: %w", check.kind, check.name(), check.what, err)
		}
		if result.Kind != ValConst || result.Const.Kind != semantics.ValBool {
			return false, fmt.Errorf("%s %s: %s must evaluate to boolean, got %v", check.kind, check.name(), check.what, result.Kind)
		}
		holds = result.Const.Bool
	}
	if cond.negated {
		holds = !holds
	}
	return holds, nil
}

// conditionTextOf renders a condition as it was written, negation included.
func conditionTextOf(cond condition) string {
	if cond.group == nil {
		if cond.negated {
			return "not " + conditionText(cond.expr)
		}
		return conditionText(cond.expr)
	}
	var texts []string
	for _, nested := range cond.group {
		if nested.required {
			texts = append(texts, conditionTextOf(nested))
		}
	}
	text := strings.Join(texts, " and ")
	if !cond.negated {
		return text
	}
	if len(texts) > 1 {
		return "not (" + text + ")"
	}
	return "not " + text
}

// negatedText renders what a negated element asserted and did not get: that not
// every required condition holds.
func negatedText(conds []condition) string {
	var texts []string
	for _, cond := range conds {
		if !cond.required {
			continue
		}
		texts = append(texts, conditionTextOf(cond))
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
