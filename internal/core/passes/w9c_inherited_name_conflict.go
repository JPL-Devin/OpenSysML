package passes

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Pilot KerMLValidator (2026-05) checkTypeDistinguishability: a type inheriting
// one member name from two supertypes, neither redefining the other's feature.
const msgW9CDuplicateInherited = "Duplicate of inherited member name"

// W9CInheritedNameConflictPass warns when two library definitions a declaration
// conforms to each supply a feature of one name: `action a : ABlock` inherits
// `self`, `start` and `done` from both Actions::Action and Parts::Part. The
// resolver's own rule covers only the document's own supertypes.
type W9CInheritedNameConflictPass struct{}

func (W9CInheritedNameConflictPass) Level() PassLevel { return LevelType }

func (W9CInheritedNameConflictPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &w9cConflictChecker{
		model:    ctx.Model(),
		idx:      ctx.Index,
		resolver: ctx.Resolver(),
		members:  map[*symbols.Symbol]map[string]w9cCandidate{},
	}
	if c.model == nil {
		return nil
	}
	w8dWalkSymbols(ctx, rootScope, c.check)
	return c.diags
}

type w9cConflictChecker struct {
	model    *semantics.Model
	idx      *symbols.Index
	resolver *resolve.Resolver
	// members memoizes the visible member of each library base, per name.
	members map[*symbols.Symbol]map[string]w9cCandidate
	diags   []Diagnostic
}

// w9cCandidate is one library base's contribution of a member name, with the
// type in that base's chain that declares it.
type w9cCandidate struct {
	declaredBy *symbols.Symbol
	member     *symbols.Symbol
}

func (c *w9cConflictChecker) check(sym *symbols.Symbol) {
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		// A variant only references a member of its variation; it declares no
		// specialization of its own for a diamond to be drawn through.
		if d.IsVariant {
			return
		}
	case *ast.Definition:
	default:
		return
	}
	if c.idx.Library(sym) {
		return
	}
	bases := c.libraryBases(sym)
	if len(bases) < 2 {
		return
	}
	byName := map[string][]w9cCandidate{}
	for _, base := range bases {
		for name, cand := range c.baseMembers(base) {
			byName[name] = append(byName[name], cand)
		}
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		if len(byName[name]) > 1 && !c.declares(sym, name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	spans := append([]source.Span{sym.DeclSpan}, c.chainSpans(sym)...)
	for _, name := range names {
		if from := c.conflictingBases(byName[name]); len(from) > 1 {
			for _, span := range spans {
				c.report(span, name, from)
			}
		}
	}
}

// chainSpans are the spans of the feature chains sym references. A chain is an
// owned feature of its own (KerML feature chaining), specialized as the
// referencing usage is, so it repeats that usage's conflicts.
func (c *w9cConflictChecker) chainSpans(sym *symbols.Symbol) []source.Span {
	var out []source.Span
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelReferences || rel.Target == nil {
			continue
		}
		if _, ok := rel.Target.(*ast.FeatureChainExpr); ok {
			out = append(out, rel.Target.Span())
			continue
		}
		qname := ast.AsQualifiedName(rel.Target)
		if qname == nil {
			continue
		}
		for _, part := range qname.Parts {
			if part.Chained {
				out = append(out, rel.Target.Span())
				break
			}
		}
	}
	return out
}

// libraryBases are the nearest library definitions sym conforms to, reached
// through the document's own types where those intervene.
func (c *w9cConflictChecker) libraryBases(sym *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{sym: true}
	var walk func(*symbols.Symbol)
	walk = func(cur *symbols.Symbol) {
		sups := c.model.DirectSupertypes(cur)
		for _, sup := range sups {
			if sup == nil || seen[sup] {
				continue
			}
			// A feature's implicit value typing is only added when nothing else
			// classifies it: a definition it is typed by, a subsetting or a
			// redefinition supplies its type, and a feature it is typed by does
			// not, so only then is a diamond drawn through Base::DataValue.
			if symbols.FQNOf(sup) == "Base::DataValue" && !isDefKind(cur.Kind) &&
				!c.typedByFeatureOnly(cur) {
				continue
			}
			seen[sup] = true
			// A feature contributes through the type that types it, not as a base.
			if c.idx.Library(sup) && (isDefKind(sup.Kind) || sup.Kind == symbols.SymbolKerMLType) {
				out = append(out, sup)
				continue
			}
			walk(sup)
		}
		// A reference subsetting (`::>`, `perform b.a`) is a subsetting, so the
		// referenced feature's type is a base of the referencing usage too.
		for _, ref := range c.referenceSubsettings(cur) {
			if seen[ref] {
				continue
			}
			seen[ref] = true
			walk(ref)
		}
	}
	walk(sym)
	return out
}

// referenceSubsettings are the features sym references (`::>` or the reference
// form of `perform`/`exhibit`/`include`).
func (c *w9cConflictChecker) referenceSubsettings(sym *symbols.Symbol) []*symbols.Symbol {
	if c.resolver == nil || sym == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelReferences || rel.Target == nil {
			continue
		}
		target, ok := c.resolver.ResolveTarget(sym.OwnerScope, rel.Target)
		if !ok || target == nil || target == sym {
			continue
		}
		out = append(out, target)
	}
	return out
}

// conflictingBases names the types declaring the features one name reaches
// that survive: a feature another candidate redefines is not inherited, and
// two candidates naming the same feature are one member reached twice.
func (c *w9cConflictChecker) conflictingBases(cands []w9cCandidate) []string {
	seen := map[*symbols.Symbol]bool{}
	names := map[string]bool{}
	for _, cand := range cands {
		if seen[cand.member] || c.redefinedByOther(cand.member, cands) ||
			c.declaredBySubtype(cand, cands) {
			continue
		}
		seen[cand.member] = true
		names[leafOf(symbols.FQNOf(cand.declaredBy))] = true
	}
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// declaredBySubtype reports whether another candidate declares the name in a
// subtype of this one's declaring type, which supplies the nearer member.
func (c *w9cConflictChecker) declaredBySubtype(cand w9cCandidate, cands []w9cCandidate) bool {
	for _, other := range cands {
		if other.declaredBy != cand.declaredBy && c.specializes(other.declaredBy, cand.declaredBy) {
			return true
		}
	}
	return false
}

// redefinedByOther reports whether another candidate's feature specializes
// member, directly or transitively: a redefinition hides what it redefines.
func (c *w9cConflictChecker) redefinedByOther(member *symbols.Symbol, cands []w9cCandidate) bool {
	for _, other := range cands {
		if other.member != member && c.specializes(other.member, member) {
			return true
		}
	}
	return false
}

// specializes reports whether sub reaches sup through its supertypes, which for
// a feature includes the features it redefines and subsets.
func (c *w9cConflictChecker) specializes(sub, sup *symbols.Symbol) bool {
	seen := map[*symbols.Symbol]bool{sub: true}
	queue := c.model.DirectSupertypes(sub)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == nil || seen[cur] {
			continue
		}
		if cur == sup {
			return true
		}
		seen[cur] = true
		queue = append(queue, c.model.DirectSupertypes(cur)...)
	}
	return false
}

// baseMembers is the member a library base makes visible under each name, the
// closest one when several of its supertypes supply it. Memoized per base.
func (c *w9cConflictChecker) baseMembers(base *symbols.Symbol) map[string]w9cCandidate {
	if cached, ok := c.members[base]; ok {
		return cached
	}
	out := map[string]w9cCandidate{}
	c.members[base] = out
	seen := map[*symbols.Symbol]bool{}
	queue := []*symbols.Symbol{base}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == nil || seen[cur] {
			continue
		}
		seen[cur] = true
		// The index carries a library type's nested names in both cache states;
		// its scope exists only when the library was parsed.
		for _, member := range c.idx.LookupDirectChildren(symbols.FQNOf(cur)) {
			name := leafOf(member.Name)
			if name == "" || member.Visibility == ast.VisibilityPrivate {
				continue
			}
			if _, taken := out[name]; taken {
				continue
			}
			out[name] = w9cCandidate{declaredBy: cur, member: member}
		}
		queue = append(queue, c.model.DirectSupertypes(cur)...)
	}
	return out
}

// declares reports whether sym binds name itself, which masks what it inherits.
func (c *w9cConflictChecker) declares(sym *symbols.Symbol, name string) bool {
	if sym.Scope == nil {
		return false
	}
	_, ok := sym.Scope.LookupLocal(name)
	return ok
}

func (c *w9cConflictChecker) report(span source.Span, name string, from []string) {
	c.diags = append(c.diags, Diagnostic{
		Severity: SeverityWarning,
		Span:     span,
		Message:  fmt.Sprintf("%s '%s' from %s", msgW9CDuplicateInherited, name, strings.Join(from, ", ")),
		Code:     "name-conflict",
		Source:   "name-resolution",
	})
}

func leafOf(fqn string) string {
	if i := strings.LastIndex(fqn, "::"); i >= 0 {
		return fqn[i+2:]
	}
	return fqn
}

// typedByFeatureOnly reports whether sym is typed by at least one feature and
// classified by nothing else: no definition types it, and it neither subsets
// nor redefines another feature.
func (c *w9cConflictChecker) typedByFeatureOnly(sym *symbols.Symbol) bool {
	typedByFeature := false
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelSubsets, ast.RelRedefines, ast.RelReferences, ast.RelSpecializes:
			return false
		case ast.RelTyping:
			target, ok := c.resolver.ResolveTarget(sym.OwnerScope, rel.Target)
			if !ok || target == nil || isDefKind(target.Kind) {
				return false
			}
			typedByFeature = true
		}
	}
	return typedByFeature
}
