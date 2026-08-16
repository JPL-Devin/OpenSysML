package runtime

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// ErrAmbiguousSubject is returned when more than one object carries the checked
// element, so which one the verdict would be about is a question, not an answer.
var ErrAmbiguousSubject = errors.New("ambiguous subject")

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
// one holds, or — for a negated element — whether one of them fails. check.self
// is the subject already, as checkSubject resolved it.
func (ctx *Context) evaluateConditions(check conditionCheck, conds []condition) (bool, error) {
	if len(conds) == 0 {
		return false, fmt.Errorf("%s %s: %w", check.kind, check.name(), ErrNoConditions)
	}
	features := ctx.conditionFeatures(check.sym)
	self := check.self
	// One check is one evaluation: its conditions share what a calc usage they
	// read answers, and the next check reads it again.
	activation, endStep := ctx.beginStep()
	defer endStep()
	required := false
	for _, cond := range conds {
		required = required || cond.required
		holds, err := ctx.conditionHolds(activation, cond, features, self, check.bindings)
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

// conditionSubject is the object a check is about: the one supplied when it
// carries the checked element, else the single object of this runtime that does.
// A nested object counts, since a redefinition on an object gives a nested
// feature values of its own; no such object leaves the check about the
// declaration.
func (ctx *Context) conditionSubject(sym *symbols.Symbol, self *Instance) (carrier, error) {
	owner := declaringType(sym)
	if owner == nil {
		return carrier{instance: self, root: self}, nil
	}
	roots := []*Instance{self}
	if self == nil {
		roots = ctx.rootInstances()
	} else if ctx.model.Conforms(self.Type, owner) {
		return carrier{instance: self, root: self}, nil
	}
	carriers := ctx.carriersUnder(roots, owner)
	switch len(carriers) {
	case 0:
		return carrier{instance: self, root: self}, nil
	case 1:
		return carriers[0], nil
	}
	return carrier{}, fmt.Errorf("%w: %s is carried by %s: check it on one of them",
		ErrAmbiguousSubject, sym.Name, strings.Join(ctx.carrierLabels(carriers), ", "))
}

// checkSubject resolves the object a check is about before its bindings are
// evaluated, so the bindings and the conditions read one object. An ambiguity is
// named after the checked element, as a verdict is.
func (ctx *Context) checkSubject(kind, element string, sym *symbols.Symbol, self *Instance) (carrier, error) {
	subject, err := ctx.conditionSubject(sym, self)
	if err != nil {
		return carrier{}, fmt.Errorf("%s %s: %w", kind, element, err)
	}
	return subject, nil
}

// declaringType is the type whose objects carry sym, nil when sym is declared
// somewhere that has no objects — a package, a library namespace.
func declaringType(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil {
		return nil
	}
	switch owner.Decl.(type) {
	case *ast.Definition, *ast.Usage:
		return owner
	}
	return nil
}

// nestedFeature reports whether sym is a feature a type declares. A definition
// nested in another is not one: objects materialize it in their own right.
func nestedFeature(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if _, ok := sym.Decl.(*ast.Usage); !ok {
		return false
	}
	return declaringType(sym) != nil
}

// readThrough reports whether inst is the object a value expression materialized
// to read a declaration through, which is an occurrence of nothing.
func (ctx *Context) readThrough(inst *Instance) bool {
	id, ok := ctx.occurrences[inst.Type]
	return ok && id == inst.ID
}

// rootInstances returns the objects this runtime holds that stand on their own,
// in identity order: an object a slot holds is reached through its holder, and one
// materialized to read a nested declaration through is an occurrence of nothing,
// while an object a caller asked for is a root whatever it materializes. One
// declaration materialized twice is one object here, the latest.
func (ctx *Context) rootInstances() []*Instance {
	held := ctx.heldObjectIDs()
	latest := make(map[*symbols.Symbol]*Instance, len(ctx.instances))
	for _, inst := range ctx.instances {
		if inst == nil || held[inst.ID] || (nestedFeature(inst.Type) && ctx.readThrough(inst)) {
			continue
		}
		if kept, ok := latest[inst.Type]; ok && kept.ID > inst.ID {
			continue
		}
		latest[inst.Type] = inst
	}
	out := make([]*Instance, 0, len(latest))
	for _, inst := range latest {
		out = append(out, inst)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// heldObjectIDs returns the identities a slot of another object already holds, so
// an object reached through its holder is no root of its own. Slots are read as
// they stand, since materializing one is the search that asked for these.
func (ctx *Context) heldObjectIDs() map[int64]bool {
	held := make(map[int64]bool)
	for _, inst := range ctx.instances {
		if inst == nil {
			continue
		}
		for _, slot := range inst.Slots {
			for _, id := range heldObjects(slot.HeldValue()) {
				held[id] = true
			}
		}
	}
	return held
}

// carriersUnder returns the objects reachable from roots whose type carries the
// features owner declares, roots included, in identity order. A declaration is
// descended into once per path, so recursive composition is a finite search, and
// one object stands for each declaration reached, so objects a multiplicity
// repeated are one candidate however deep the named declaration sits in them.
func (ctx *Context) carriersUnder(roots []*Instance, owner *symbols.Symbol) []carrier {
	var out []carrier
	seen := make(map[int64]bool, len(roots))
	declared := make(map[carrierOccurrence]bool)
	path := make(map[*symbols.Symbol]bool)
	var descend func(root, inst *Instance, through, features string)
	descend = func(root, inst *Instance, through, features string) {
		if inst == nil || seen[inst.ID] {
			return
		}
		seen[inst.ID] = true
		occurrence := carrierOccurrence{through: through, decl: inst.Type}
		if ctx.model.Conforms(inst.Type, owner) && !declared[occurrence] {
			declared[occurrence] = true
			out = append(out, carrier{instance: inst, root: root, features: features})
		}
		if inst.Type != nil {
			if path[inst.Type] {
				return
			}
			path[inst.Type] = true
			defer delete(path, inst.Type)
		}
		for _, child := range ctx.nestedObjects(inst) {
			nested := child.feature
			if features != "" {
				nested = features + "::" + child.feature
			}
			descend(root, child.instance, through+"::"+child.feature, nested)
		}
	}
	for _, root := range roots {
		descend(root, root, strconv.FormatInt(root.ID, 10), "")
	}
	sort.Slice(out, func(i, j int) bool { return out[i].instance.ID < out[j].instance.ID })
	return out
}

// carrier is an object a search reached: the object the search started from and
// the features walked from it name a nested one, which has no name of its own.
type carrier struct {
	instance *Instance
	root     *Instance
	features string
}

// carrierOccurrence identifies the declaration an object occurs as: the features
// walked through to reach it and the declaration it materializes. Objects a
// multiplicity repeated share both, while ones a collection gathers from
// different declarations — the features subsetting it — do not.
type carrierOccurrence struct {
	through string
	decl    *symbols.Symbol
}

// heldObject is an object a feature of another object holds, named by that
// feature.
type heldObject struct {
	feature  string
	instance *Instance
}

// nestedObjects returns the objects the object-valued features of inst hold,
// materializing a lazy one as reading its slot does. A slot that cannot be read
// yields no object: one that is not there is no subject either.
func (ctx *Context) nestedObjects(inst *Instance) []heldObject {
	features := ctx.FeaturesOf(inst.Type)
	var out []heldObject
	for i := range features {
		feat := &features[i]
		if feat.Name == "" || !holdsObjects(feat) {
			continue
		}
		slot, err := inst.GetSlot(ctx, feat.Name)
		if err != nil || slot == nil {
			continue
		}
		for _, id := range heldObjects(slot.HeldValue()) {
			if child, ok := ctx.instances[id]; ok {
				out = append(out, heldObject{feature: feat.Name, instance: child})
			}
		}
	}
	return out
}

// holdsObjects reports whether a feature holds objects rather than values: a
// nested part has features and conditions of its own, an attribute has neither.
func holdsObjects(feat *EffectiveFeature) bool {
	if feat.Symbol == nil {
		return false
	}
	usage, ok := feat.Symbol.Decl.(*ast.Usage)
	if !ok {
		return false
	}
	switch usage.Kind {
	case ast.UsagePart, ast.UsageItem, ast.UsageOccurrence, ast.UsageIndividual, ast.UsagePort:
		return true
	}
	return false
}

// heldObjects returns the identities a slot value denotes, a collection's
// elements included.
func heldObjects(val Value) []int64 {
	if id, ok := val.Object(); ok {
		return []int64{id}
	}
	var elements []Value
	switch {
	case val.Kind == ValSequence && val.Sequence != nil:
		elements = val.Sequence.Elements()
	case val.Kind == ValSet && val.Set != nil:
		elements = val.Set.Elements()
	}
	var out []int64
	for _, element := range elements {
		if id, ok := element.Object(); ok {
			out = append(out, id)
		}
	}
	return out
}

// carrierLabels names carriers as a diagnostic can quote them: the definition
// each is an object of, its identity, and the feature path telling two apart.
func (ctx *Context) carrierLabels(carriers []carrier) []string {
	out := make([]string, 0, len(carriers))
	for _, c := range carriers {
		inst := c.instance
		name := "object"
		if def := ctx.definitionOf(inst.Type); def != nil && def.Name != "" {
			name = def.Name
		}
		label := fmt.Sprintf("%s #%d", name, inst.ID)
		if path := carrierPath(c, name); path != "" {
			label += " (" + path + ")"
		}
		out = append(out, label)
	}
	return out
}

// carrierPath names a carrier apart from its siblings: the features walked to
// it, ending in the declaration it materializes — which differs from the feature
// holding it when a collection gathers objects of several declarations.
func carrierPath(c carrier, definition string) string {
	decl := ""
	if c.instance.Type != nil && c.instance.Type.Name != definition {
		decl = c.instance.Type.Name
	}
	if c.features == "" {
		return decl
	}
	walked := strings.Split(c.features, "::")
	if decl != "" && walked[len(walked)-1] != decl {
		walked[len(walked)-1] = decl
	}
	return strings.Join(walked, "::")
}

// carrierFeatures is the feature path to a nested carrier as a caller can quote
// it, corrected the way carrierLabels names an ambiguity's carriers.
func (ctx *Context) carrierFeatures(c carrier) string {
	if c.features == "" || c.instance == nil {
		return ""
	}
	name := "object"
	if def := ctx.definitionOf(c.instance.Type); def != nil && def.Name != "" {
		name = def.Name
	}
	return carrierPath(c, name)
}

// definitionOf is the definition objects of sym are objects of: sym itself when
// it declares one, else the nearest definition it specializes.
func (ctx *Context) definitionOf(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil {
		return nil
	}
	if _, ok := sym.Decl.(*ast.Definition); ok {
		return sym
	}
	for _, super := range ctx.model.AllSupertypes(sym) {
		if _, ok := super.Decl.(*ast.Definition); ok {
			return super
		}
	}
	return sym
}

// conditionHolds evaluates one condition: an expression, or a group that holds
// when all of its conditions hold. Its negation, if any, is applied last.
func (ctx *Context) conditionHolds(activation int64, cond condition, features map[string]scopedExpr, self *Instance, bindings map[string]Value) (bool, error) {
	holds := true
	if cond.group != nil {
		for _, sub := range cond.group {
			subHolds, err := ctx.conditionHolds(activation, sub, features, self, bindings)
			if err != nil {
				return false, err
			}
			holds = holds && subHolds
		}
	} else {
		ec := NewEvalContextIn(ctx, cond.scope, self)
		ec.activation = activation
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
		out[feat.Name] = scopedExpr{expr: feat.DefaultValue, scope: feat.DefaultScope()}
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
