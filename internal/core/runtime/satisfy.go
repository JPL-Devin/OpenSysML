package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// SatisfyAssertion is one satisfaction assertion an element states:
// `assert satisfy <requirement> by <subject>` (SysML v2 §8.3.17.15). The
// assertion is a requirement usage of its own — it reference-subsets the
// requirement it satisfies and binds that requirement's subject parameter to
// the feature named by `by` — so it carries a verdict the requirement alone
// does not: one about the values that feature actually holds.
type SatisfyAssertion struct {
	// Symbol is the satisfy usage itself, which is anonymous in the usual
	// `assert satisfy r by p;` form.
	Symbol *symbols.Symbol

	// Owner is the element stating the assertion (the enclosing part or
	// package), or nil at the root of a document.
	Owner *symbols.Symbol

	// Requirement is the requirement the assertion satisfies: the target of the
	// usage's reference subsetting. It is nil when the reference names nothing
	// resolvable, and when the assertion declares the requirement itself
	// (`satisfy requirement r by p { ... }`), which states its conditions.
	Requirement *symbols.Symbol

	// RequirementRef is the requirement reference as written, so an unresolved
	// one can be reported by name.
	RequirementRef string

	// Subject is the feature named by `by`, whose values the requirement is
	// evaluated against. It is nil when the assertion names no subject, and
	// when the name resolves to nothing.
	Subject *symbols.Symbol

	// SubjectRef is the `by` operand as written.
	SubjectRef string

	// Negated is `assert not satisfy ...`: the assertion holds when the
	// requirement is not satisfied.
	Negated bool
}

// Text renders the assertion as it was written, so an anonymous one can be
// named in a verdict. A `satisfy requirement r by p` form declares the
// requirement rather than referencing one, so it is named by the usage itself.
func (a *SatisfyAssertion) Text() string {
	var b strings.Builder
	if a.Negated {
		b.WriteString("not ")
	}
	b.WriteString("satisfy ")
	switch {
	case a.RequirementRef != "":
		b.WriteString(a.RequirementRef)
	case a.Symbol != nil && a.Symbol.Name != "":
		b.WriteString(a.Symbol.Name)
	default:
		b.WriteString("?")
	}
	if a.SubjectRef != "" {
		b.WriteString(" by ")
		b.WriteString(a.SubjectRef)
	}
	return b.String()
}

// SatisfyAssertionsIn returns the satisfaction assertions stated in scope and,
// recursively, in the scopes nested within it, in declaration order. An
// assertion is anonymous in its usual form, so it is reached through the
// element that states it rather than by name.
func (ctx *Context) SatisfyAssertionsIn(scope *symbols.Scope) []*SatisfyAssertion {
	if scope == nil {
		return nil
	}
	var out []*SatisfyAssertion
	for _, sym := range scopeMemberSymbols(scope) {
		if a := ctx.satisfyAssertionOf(sym); a != nil {
			out = append(out, a)
		}
	}
	for _, child := range scope.Children() {
		out = append(out, ctx.SatisfyAssertionsIn(child)...)
	}
	return out
}

// scopeMemberSymbols returns the symbols declared directly in scope, named ones
// in declaration order followed by the anonymous ones, which is where an
// `assert satisfy ...` lives.
func scopeMemberSymbols(scope *symbols.Scope) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, name := range scope.MemberNames() {
		out = append(out, scope.LookupLocalAll(name)...)
	}
	return append(out, scope.AnonymousMembers()...)
}

// SatisfyAssertionOf returns the assertion sym declares, or an error when sym is
// not a satisfaction assertion. It names the one a `satisfy requirement r by p`
// form declares under a name.
func (ctx *Context) SatisfyAssertionOf(sym *symbols.Symbol) (*SatisfyAssertion, error) {
	a := ctx.satisfyAssertionOf(sym)
	if a == nil {
		return nil, fmt.Errorf("%s: %w", symbolLabel(sym), ErrNotASatisfaction)
	}
	return a, nil
}

// satisfyAssertionOf resolves the assertion sym declares, or nil when sym is not
// a satisfy usage.
func (ctx *Context) satisfyAssertionOf(sym *symbols.Symbol) *SatisfyAssertion {
	if sym == nil {
		return nil
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageSatisfy {
		return nil
	}
	a := &SatisfyAssertion{Symbol: sym, Negated: usage.IsNegated}
	if sym.OwnerScope != nil {
		a.Owner = sym.OwnerScope.Owner()
	}
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Target == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelSubsets:
			a.RequirementRef, a.Requirement = ctx.resolveRelationship(sym, rel)
		case ast.RelSubject:
			a.SubjectRef, a.Subject = ctx.resolveRelationship(sym, rel)
		}
	}
	return a
}

// resolveRelationship resolves a relationship target of sym, returning the name
// as written and the symbol it denotes, which is nil when it denotes none.
func (ctx *Context) resolveRelationship(sym *symbols.Symbol, rel *ast.Relationship) (string, *symbols.Symbol) {
	qn := ast.AsQualifiedName(rel.Target)
	if qn == nil {
		name, _ := ast.TargetName(rel.Target)
		return name, nil
	}
	target, ok := ctx.resolver.ResolveQualified(sym.OwnerScope, qn)
	if !ok {
		target = nil
	}
	return qualifiedNameToString(qn), target
}

// EvaluateSatisfaction evaluates a satisfaction assertion against a fresh
// instance of its subject, so that the values the subject declares supply the
// requirement's own.
func (ctx *Context) EvaluateSatisfaction(a *SatisfyAssertion) (bool, error) {
	return ctx.EvaluateSatisfactionOn(a, nil)
}

// EvaluateSatisfactionOn evaluates a satisfaction assertion against a given
// object as its subject: the requirement's subject parameter is bound to that
// object, so a feature the requirement's conditions reach through the subject
// reads the value that object holds. A nil subject instantiates the feature the
// assertion names with `by`.
//
// A false verdict is returned as a *ViolationError, which unwraps to
// ErrViolated: it is an answer about the model, not a failure to evaluate.
func (ctx *Context) EvaluateSatisfactionOn(a *SatisfyAssertion, subject *Instance) (bool, error) {
	result, err := ctx.CheckSatisfactionOn(a, subject)
	return result.Holds, err
}

// CheckSatisfactionOn evaluates a satisfaction assertion as
// EvaluateSatisfactionOn does and also reports the object it turned out to be
// about.
func (ctx *Context) CheckSatisfactionOn(a *SatisfyAssertion, subject *Instance) (CheckResult, error) {
	defer ctx.beginRun()()

	if a == nil || a.Symbol == nil {
		return CheckResult{Subject: subject}, ErrNotASatisfaction
	}
	// The assertion is evaluated as the requirement usage it is: it inherits the
	// conditions of the requirement it references and the values that
	// requirement binds, plus any it rebinds itself.
	target := a.Symbol
	if a.Requirement == nil && a.RequirementRef != "" {
		return CheckResult{Subject: subject}, fmt.Errorf("%s: %w: %s", a.Text(), ErrNoRequirement, a.RequirementRef)
	}
	if a.Requirement == nil && !declaresConditions(a.Symbol) {
		return CheckResult{Subject: subject}, fmt.Errorf("%s: %w", a.Text(), ErrNoRequirement)
	}

	if subject == nil && a.SubjectRef != "" {
		inst, err := ctx.SatisfySubject(a)
		if err != nil {
			return CheckResult{}, err
		}
		subject = inst
	}

	// The requirement being satisfied chooses the object its conditions read the
	// same way `%requirement` does, so an object holding the carrier nested
	// answers about that nested object rather than about the declaration.
	// An assertion stating its own conditions has no requirement to resolve
	// against, so it resolves against itself.
	carrying := a.Requirement
	if carrying == nil {
		carrying = target
	}
	resolved, err := ctx.checkSubject("satisfaction", a.Text(), carrying, subject)
	if err != nil {
		return CheckResult{}, err
	}
	reached := resolved // the object resolved to, named by where it was reached from
	subject = resolved.instance

	scope := target.OwnerScope
	members := ctx.chainMembers(target, scope)

	// Every subject the chain declares names the object `by` supplies, which is
	// what satisfies the requirement (SysML v2 §8.3.17.15); the other values the
	// requirement binds by name are visible to its conditions here too, as they
	// are when the requirement is evaluated directly.
	bindings, err := ctx.memberBindings(target, a.Text(), members, subject, subject)
	if err != nil {
		return ctx.satisfactionResult(false, subject, reached), err
	}

	conds := conditionsOf(members)
	holds, err := ctx.evaluateConditions(conditionCheck{
		sym:  target,
		kind: "satisfaction",
		what: "require condition",
		// The object the assertion checks is the one `by` supplies, so a
		// requirement feature carrying no value of its own reads that object's,
		// the same way `%requirement` on an instance does.
		self:     subject,
		element:  a.Text(),
		bindings: bindings,
		negated:  a.Negated,
	}, conds)
	return ctx.satisfactionResult(holds, subject, reached), err
}

// satisfactionResult reports a verdict about subject, naming where a resolved
// nested one was reached from. An assertion whose requirement resolved nothing
// is still about the object `by` supplied.
func (ctx *Context) satisfactionResult(holds bool, subject *Instance, reached carrier) CheckResult {
	if reached.instance == nil {
		return CheckResult{Holds: holds, Subject: subject, SubjectRoot: subject}
	}
	return ctx.checkResultOf(holds, reached)
}

// SatisfySubject returns an object of the feature a satisfaction assertion names
// with `by`: the object its requirement is evaluated against when the caller
// supplies none.
func (ctx *Context) SatisfySubject(a *SatisfyAssertion) (*Instance, error) {
	if a == nil || a.Symbol == nil {
		return nil, ErrNotASatisfaction
	}
	if a.Subject == nil {
		return nil, fmt.Errorf("%s: %w: %s", a.Text(), ErrNoSubject, a.SubjectRef)
	}
	inst, err := ctx.Instantiate(a.Subject)
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", a.Text(), ErrNoSubject, err)
	}
	return inst, nil
}

// declaresConditions reports whether sym's own declaration states any condition,
// which is how a `satisfy requirement r by p { require ... }` form carries one
// without referencing another requirement.
func declaresConditions(sym *symbols.Symbol) bool {
	for _, node := range declMembers(sym.Decl) {
		if len(appendConditions(nil, node, nil, true, false)) > 0 {
			return true
		}
	}
	return false
}

// symbolLabel names a symbol in a message, falling back to its declaration kind
// for an anonymous one.
func symbolLabel(sym *symbols.Symbol) string {
	switch {
	case sym == nil:
		return "<nil>"
	case sym.Name != "":
		return sym.Name
	default:
		return fmt.Sprintf("anonymous %T", sym.Decl)
	}
}
