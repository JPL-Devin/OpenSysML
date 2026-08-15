package resolve

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/suggest"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// MemberLookup interface abstracts semantic model for inheritance-aware member resolution.
// Implemented by *semantics.Model.
type MemberLookup interface {
	// LookupMember searches for a member by name in sym's scope and inherited scopes.
	LookupMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool)
	// LookupContributedMember searches the inherited scopes only, skipping
	// what sym itself declares.
	LookupContributedMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool)
}

// supertypeLookup is the part of the semantic model that reports the features a
// declaration specializes, including the ones it redefines implicitly. A
// nameless parameter takes its name from the parameter it redefines, which only
// the model can match (KerML 7.3.4.5). *semantics.Model implements it.
type supertypeLookup interface {
	DirectSupertypes(sym *symbols.Symbol) []*symbols.Symbol
}

// resolution is a memoized lookup outcome.
type resolution struct {
	sym *symbols.Symbol
	ok  bool
}

// Resolver performs lazy name resolution over a symbol index, memoizing results
// keyed by the reference AST node and collecting diagnostics.
type Resolver struct {
	idx       *symbols.Index
	memo      map[ast.Node]resolution
	resolving map[ast.Node]bool // cycle detection
	parts     map[*ast.QualifiedName][]*symbols.Symbol
	// imports are the import declarations of a namespace-bearing node, found once
	// and kept: see (*Resolver).importsOf.
	imports     map[ast.Node][]*ast.Import
	Diagnostics []Diagnostic
	model       MemberLookup             // Optional *semantics.Model for inheritance-aware member lookup
	naming      map[*symbols.Symbol]bool // effective names being computed, for cycle detection
	// inheritedImports are the declarations whose supertypes' imports are being
	// searched, so a specialization cycle ends the walk.
	inheritedImports map[*symbols.Symbol]bool
	// constraintRefs are the requirements referenced by the require/assume
	// members whose bodies are being walked, innermost last: such a body may
	// redefine a feature of the requirement it references by plain name.
	constraintRefs []constraintRef
	// suggestions are the spellings an unresolvable name may have meant, kept
	// per name and scope; names is the index's name table they are looked up in.
	// suggesting holds the suggestions being scored, so scoring one cannot
	// recurse into scoring itself.
	suggestions map[suggestKey][]string
	names       *suggest.Table
	suggesting  map[suggestKey]bool
}

// New creates a resolver over the given index.
func New(idx *symbols.Index) *Resolver {
	return &Resolver{
		idx:       idx,
		memo:      map[ast.Node]resolution{},
		resolving: map[ast.Node]bool{},
		parts:     map[*ast.QualifiedName][]*symbols.Symbol{},
		imports:   map[ast.Node][]*ast.Import{},
		naming:    map[*symbols.Symbol]bool{},

		suggestions: map[suggestKey][]string{},
		suggesting:  map[suggestKey]bool{},

		inheritedImports: map[*symbols.Symbol]bool{},
	}
}

// recordPart stores the symbol a single segment of a qualified name resolves to.
// Per-segment results are kept here rather than on the AST, which stays
// immutable after parsing so that concurrent readers may share it.
func (r *Resolver) recordPart(qn *ast.QualifiedName, i int, sym *symbols.Symbol) {
	if qn == nil || i < 0 || i >= len(qn.Parts) {
		return
	}
	syms, ok := r.parts[qn]
	if !ok {
		syms = make([]*symbols.Symbol, len(qn.Parts))
		r.parts[qn] = syms
	}
	syms[i] = sym
}

// PartSymbol returns the symbol the i-th segment of qn resolved to during a
// previous resolution by this resolver. Callers that need intermediate
// resolutions of a qualified name (a hover over `A::B` in `A::B::C`, say) read
// them here.
func (r *Resolver) PartSymbol(qn *ast.QualifiedName, i int) (*symbols.Symbol, bool) {
	syms, ok := r.parts[qn]
	if !ok || i < 0 || i >= len(syms) || syms[i] == nil {
		return nil, false
	}
	return syms[i], true
}

// Index returns the symbol index this resolver operates over.
func (r *Resolver) Index() *symbols.Index {
	return r.idx
}

// lookupMember resolves name as a member of sym — declared by it or inherited
// from what it specializes or is typed by — when a semantic model is attached.
func (r *Resolver) lookupMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	if r.model == nil || sym == nil {
		return nil, false
	}
	return r.model.LookupMember(sym, name)
}

// lookupContributedMember resolves name as a member sym inherits or
// reference-subsets, ignoring the members sym declares itself.
func (r *Resolver) lookupContributedMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	if r.model == nil || sym == nil {
		return nil, false
	}
	return r.model.LookupContributedMember(sym, name)
}

// SetModel attaches a semantic model for inheritance-aware member resolution.
// Must be called before resolving feature chains if inherited members are needed.
func (r *Resolver) SetModel(model MemberLookup) {
	r.model = model
}

// ResolveQualified resolves a qualified-name reference against the given scope.
// scope may be nil to resolve purely from the global index / document root.
// Later tasks implement the walk; this skeleton reports unresolved.
func (r *Resolver) ResolveQualified(scope *symbols.Scope, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	return r.resolveQualified(scope, qn, nil)
}

// resolveQualified is ResolveQualified with an optional filter hiding the
// bindings a reference subsetting's own borrowed name contributes.
func (r *Resolver) resolveQualified(scope *symbols.Scope, qn *ast.QualifiedName, hide *refFilter) (*symbols.Symbol, bool) {
	if qn == nil {
		return nil, false
	}
	if res, done := r.memo[qn]; done {
		return res.sym, res.ok
	}
	// Detect resolution cycles
	if r.resolving[qn] {
		// Cycle detected, fail resolution
		return nil, false
	}
	r.resolving[qn] = true
	// A feature that took its name from qn is never what qn names.
	res := r.walkQualified(scope, qn, hide.hiding(qn))
	delete(r.resolving, qn)
	r.memo[qn] = res
	return res.sym, res.ok
}

// ResolveName resolves a single-segment (unqualified) reference from the given
// scope. The at node keys the memo table.
func (r *Resolver) ResolveName(scope *symbols.Scope, name string, at ast.Node) (*symbols.Symbol, bool) {
	if at != nil {
		if res, done := r.memo[at]; done {
			return res.sym, res.ok
		}
	}
	res := r.walkUnqualified(scope, name)
	if at != nil {
		r.memo[at] = res
	}
	if !res.ok {
		r.Diagnostics = append(r.Diagnostics, Diagnostic{
			Span:    spanOf(at),
			Message: r.unresolvedMessage(scope, name),
		})
	}
	return res.sym, res.ok
}

func spanOf(n ast.Node) source.Span {
	if n == nil {
		return source.Span{}
	}
	return n.Span()
}

// qnText renders a qualified name for diagnostics (segments joined by "::",
// "$::" prefix when global).
func qnText(qn *ast.QualifiedName) string {
	s := ""
	for i, part := range qn.Parts {
		if i > 0 {
			s += "::"
		}
		s += part.Text
	}
	if qn.Global {
		s = "$::" + s
	}
	return s
}
