package symbols

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// ElementFilter is one element-filter condition, as it applies to the elements
// an enumeration would surface: the condition of a `filter <expr>;` member of a
// namespace (KerML 8.2.4 ElementFilterMembership) or of an import's `[...]`
// clause (`import P::*[@Safety]`, SysML v2 7.4.4).
//
// A condition is a predicate over one candidate element, not a value expression,
// and evaluating it needs the metadata annotating the candidate — knowledge the
// symbol layer does not have. So a filter is carried here in the two forms the
// evaluator in semantics can run:
//
//   - Expr, the condition as parsed, together with the Scope its names resolve
//     against. This is what a live-parsed document holds.
//   - Pred, the condition compiled to element references already resolved to
//     fully-qualified names. This is what a library restored from an index cache
//     holds, whose declarations — and the scopes that gave its names meaning —
//     are gone. It is what makes a filtered import behave the same way for a
//     parsed and for a cache-restored library.
type ElementFilter struct {
	Expr  ast.Node
	Scope *Scope
	Pred  *FilterPredicate
	Span  source.Span
}

// IsZero reports whether there is no condition to apply.
func (f ElementFilter) IsZero() bool { return f.Expr == nil && f.Pred == nil }

// String describes a filter for a snapshot of the index. It deliberately
// renders only whether a condition applies: the same condition reaches a parsed
// document as an expression and a restored library as a compiled predicate, and
// the contract between the two is that they select the same elements, which is
// asserted over resolution rather than over this text.
func (f ElementFilter) String() string {
	if f.IsZero() {
		return "unfiltered"
	}
	return "filtered"
}

// FilterOp is the operation a compiled filter predicate node performs.
type FilterOp uint8

const (
	// FilterUnsupported is a condition, or part of one, outside the subset the
	// filter evaluator implements. It has no truth value: it is reported, and
	// never silently taken to be true or false.
	FilterUnsupported FilterOp = iota
	FilterAnd
	FilterOr
	FilterXor
	FilterImplies
	FilterNot
	// FilterClassify is `@T`: the candidate is annotated by metadata
	// conforming to TypeFQN.
	FilterClassify
	// FilterMetaClassify is `@@T`: the candidate's own metaclass conforms to
	// TypeFQN.
	FilterMetaClassify
	FilterEq
	FilterNeq
	FilterLt
	FilterLe
	FilterGt
	FilterGe
	// FilterConst is a literal, or an element reference used as one (an
	// enumeration literal), held in Value.
	FilterConst
	// FilterFeature is the value the candidate's annotation of TypeFQN binds
	// Feature to.
	FilterFeature
)

// FilterPredicate is an element-filter condition compiled to the form the
// evaluator runs: a tree over the candidate element, with every element the
// condition names resolved to its fully-qualified name. Compiling before
// evaluating is what lets one condition be evaluated against many candidates,
// and lets a condition outlive the declaration it was written in.
type FilterPredicate struct {
	Op       FilterOp
	Operands []*FilterPredicate

	// TypeFQN is the metadata type a classification tests against, or whose
	// annotation a feature is read from.
	TypeFQN string

	// Feature is the annotation feature FilterFeature reads.
	Feature string

	// Value is the constant FilterConst yields.
	Value FilterValue

	// Reason says why a FilterUnsupported node cannot be evaluated, phrased for
	// a diagnostic message.
	Reason string

	// Span locates the part of the condition this node came from.
	Span source.Span
}

// FilterValueKind discriminates a constant a filter predicate compares.
type FilterValueKind uint8

const (
	FilterValueUnknown FilterValueKind = iota
	FilterValueBool
	FilterValueInt
	FilterValueReal
	FilterValueString
	// FilterValueRef is a reference to an element used as a value, such as an
	// enumeration literal, compared by identity of the element it names.
	FilterValueRef
	// FilterValueEmpty is the absence of a value — the empty sequence a feature
	// bound to nothing has. It is a result, unlike FilterValueUnknown, which
	// says a value could not be determined.
	FilterValueEmpty
)

// FilterValue is a constant a filter predicate yields or compares: a literal, or
// the element an enumeration-literal reference names.
type FilterValue struct {
	Kind   FilterValueKind
	Bool   bool
	Int    int64
	Real   float64
	Str    string
	RefFQN string
}

// Same reports whether two filters are the same condition, so that two routes to
// one re-exported name can be compared. A condition is identified by the
// declaration it was written in: the expression node and the scope its names
// resolve against, or the compiled predicate a restored library carries.
func (f ElementFilter) Same(g ElementFilter) bool {
	return f.Expr == g.Expr && f.Scope == g.Scope && f.Pred == g.Pred
}

// filtersFromPredicates wraps compiled conditions as the filters a restored
// namespace applies, which carry no expression and no scope.
func filtersFromPredicates(preds []*FilterPredicate) []ElementFilter {
	out := make([]ElementFilter, 0, len(preds))
	for _, pred := range preds {
		if pred == nil {
			continue
		}
		out = append(out, ElementFilter{Pred: pred, Span: pred.Span})
	}
	return out
}

// NamespaceFiltersOf returns the filter conditions declared by the namespace
// registered under fqn, over the documents declaring it in name order.
func (idx *Index) NamespaceFiltersOf(fqn string) []ElementFilter {
	byDoc := idx.nsFilters[fqn]
	if len(byDoc) == 0 {
		return nil
	}
	var out []ElementFilter
	for _, doc := range sortedKeys(byDoc) {
		out = append(out, byDoc[doc]...)
	}
	return out
}

// namespaceFiltersGating returns the conditions gating an import doc states into
// the namespace under fqn: every document's for a named namespace, only doc's
// for a root namespace, which each document owns separately.
func (idx *Index) namespaceFiltersGating(fqn, doc string) []ElementFilter {
	if fqn != "" {
		return idx.NamespaceFiltersOf(fqn)
	}
	return idx.nsFilters[fqn][doc]
}

// SetNamespaceFilters records the filter conditions doc declares for the
// namespace registered under fqn. A library restored from an index cache states
// them this way, having no declaration left to read them from.
func (idx *Index) SetNamespaceFilters(fqn, doc string, filters []ElementFilter) {
	if len(filters) == 0 {
		idx.forgetNamespaceFilters(fqn, doc)
		return
	}
	if idx.nsFilters[fqn] == nil {
		idx.nsFilters[fqn] = make(map[string][]ElementFilter)
	}
	idx.nsFilters[fqn][doc] = filters
	idx.refilter(fqn)
}

// dropNamespaceFilters forgets the filters doc declared, for every namespace.
func (idx *Index) dropNamespaceFilters(doc string) {
	for fqn := range idx.nsFilters {
		idx.forgetNamespaceFilters(fqn, doc)
	}
}

// forgetNamespaceFilters drops the filters doc declared for the namespace
// registered under fqn, if it declared any.
func (idx *Index) forgetNamespaceFilters(fqn, doc string) {
	byDoc := idx.nsFilters[fqn]
	if _, had := byDoc[doc]; !had {
		return
	}
	delete(byDoc, doc)
	if len(byDoc) == 0 {
		delete(idx.nsFilters, fqn)
	}
	idx.refilter(fqn)
}

// refilter drops what the namespace registered under fqn re-exports, because the
// conditions gating those routes have changed: the next expansion records them
// under the filters the namespace now declares, and the members it takes back
// meanwhile mark the namespaces importing it onward for expansion too.
func (idx *Index) refilter(fqn string) {
	delete(idx.lastTargets, fqn)
	idx.purgeReexportsUnder(fqn)
}

// NamespaceFiltersIn returns the filter conditions the namespace owning scope
// declares. It is what a lookup through an import consults: the namespace's
// `filter` members restrict its imported memberships, whichever route reaches
// them (KerML 8.2.4).
//
// Unlike NamespaceFiltersOf, which answers for a namespace registered in the
// index, this reads the declaration a scope was built from, so it also answers
// for a `filter` member of a definition or usage body — where the training
// corpus puts one, in a view definition restricting what its views expose.
func NamespaceFiltersIn(scope *Scope) []ElementFilter {
	if scope == nil {
		return nil
	}
	return extractNamespaceFilters(scope.Node(), scope)
}

// ImportFiltersIn returns the filter clauses of the imports the namespace owning
// scope declares (`import P::*[@Safety]`), in declaration order. It is what a
// validation pass reads: the conditions an import states are checked where they
// are written, whichever lookup later applies them.
func ImportFiltersIn(scope *Scope) []ElementFilter {
	if scope == nil {
		return nil
	}
	var out []ElementFilter
	for _, m := range namespaceMembers(scope.Node()) {
		if mem, ok := m.(*ast.Membership); ok {
			m = mem.Member
		}
		imp, ok := m.(*ast.Import)
		if !ok || imp.FilterExpr == nil {
			continue
		}
		out = append(out, ElementFilter{
			Expr:  imp.FilterExpr,
			Scope: scope,
			Span:  imp.FilterExpr.Span(),
		})
	}
	return out
}

// extractNamespaceFilters returns the conditions of the `filter` members a
// namespace-bearing declaration states, in declaration order.
func extractNamespaceFilters(decl ast.Node, scope *Scope) []ElementFilter {
	var out []ElementFilter
	for _, m := range namespaceMembers(decl) {
		if mem, ok := m.(*ast.Membership); ok {
			m = mem.Member
		}
		f, ok := m.(*ast.FilterMember)
		if !ok || f.Condition == nil {
			continue
		}
		out = append(out, ElementFilter{Expr: f.Condition, Scope: scope, Span: f.Span()})
	}
	return out
}

// namespaceMembers returns the members of a namespace-bearing declaration. A
// definition or usage body is a namespace too (KerML 7.2), and can hold both
// imports and filters.
func namespaceMembers(decl ast.Node) []ast.Node {
	switch d := decl.(type) {
	case *ast.Package:
		return d.Members
	case *ast.Namespace:
		return d.Members
	case *ast.RootNamespace:
		return d.Members
	case *ast.Definition:
		return d.Members
	case *ast.Usage:
		return d.Members
	default:
		return nil
	}
}

// sortedKeys returns a map's keys in name order, so that reading it does not
// depend on map iteration order.
func sortedKeys(m map[string][]ElementFilter) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
