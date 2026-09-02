// Package identity computes the effective repository identity of the symbols
// of an analyzed model: the id of an element's IdentityMetadata::ElementId
// annotation when it carries one, the id derived from its qualified name
// otherwise, plus the project scope its nearest enclosing ProjectRef declares.
// It is a side table keyed by symbol; the AST is never touched.
package identity

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// FQNs of the identity metadata definitions in the bundled library.
const (
	ElementIdFQN  = "IdentityMetadata::ElementId"
	ProjectRefFQN = "IdentityMetadata::ProjectRef"
)

// Scope is one ProjectRef annotation's project binding. Two scopes name the
// same project when org and projectId agree; branch selects a version of one
// project, never another identity, so it plays no part in equality.
type Scope struct {
	ProjectID string
	Branch    string
	Org       string
	// Symbol is the namespace carrying the ProjectRef annotation.
	Symbol *symbols.Symbol
	// Declarations lists every ProjectRef annotation of the namespace, inline
	// ones first, `about`-form ones after. The first supplies the binding.
	Declarations []ScopeDeclaration
}

// ScopeDeclaration is one ProjectRef annotation of a namespace, whether
// written inline or from elsewhere with an `about` clause.
type ScopeDeclaration struct {
	ProjectID string
	Branch    string
	Org       string
	// Node is the annotating node itself, so a consumer that absorbs the
	// annotation into provenance can identify it in the tree.
	Node ast.Node
	// Span locates the annotating node, for diagnostics.
	Span source.Span
	// Scope is where the annotating node is declared.
	Scope *symbols.Scope
}

// Key names the project a declaration binds to; branch selects a version of
// one project, never another identity, so it plays no part.
func (d ScopeDeclaration) Key() string {
	return d.Org + "\x00" + d.ProjectID
}

// Key names the project a scope binds to, for scope-equality grouping.
func (s *Scope) Key() string {
	if s == nil {
		return ""
	}
	return s.Org + "\x00" + s.ProjectID
}

// Declaration is one ElementId annotation of an element, whether written
// inline (`@ElementId {...}`) or from elsewhere (`metadata : ElementId about x;`).
type Declaration struct {
	// ID is the annotation's evaluated id; meaningful when Declared.
	ID string
	// Declared reports the id evaluated to a constant string.
	Declared bool
	// About marks an `about`-form annotation stated away from the element.
	About bool
	// Node is the annotating node itself, so a consumer that absorbs the
	// annotation into identity can identify it in the tree.
	Node ast.Node
	// Span locates the annotating node, for diagnostics.
	Span source.Span
	// Scope is where the annotating node is declared; for an `about`-form
	// annotation that may be another document than the element's.
	Scope *symbols.Scope
}

// Info is one symbol's identity: its effective id, whether an ElementId
// annotation declared it, and the project scope it resolves against.
type Info struct {
	Symbol *symbols.Symbol
	// FQN is the symbol's qualified name, which the derived id encodes.
	FQN string
	// EffectiveID is the annotated id when declared, the derived id otherwise.
	EffectiveID string
	// Annotated reports an ElementId annotation on the element.
	Annotated bool
	// Declared reports that the annotation's id evaluated to a constant
	// string; when false the derived id stands.
	Declared bool
	// DeclaredID is the evaluated ElementId id, possibly empty.
	DeclaredID string
	// AnnotationSpan locates the ElementId annotation, for diagnostics.
	AnnotationSpan source.Span
	// Declarations lists every ElementId annotation of the element, inline
	// ones first, `about`-form ones after.
	Declarations []Declaration
	// Scope is the nearest enclosing ProjectRef binding; nil when unbound.
	Scope *Scope
}

// Table is the identity side table of one analyzed scope tree.
type Table struct {
	infos map[*symbols.Symbol]*Info
	order []*symbols.Symbol
}

// Info returns the identity of sym, if the table holds it.
func (t *Table) Info(sym *symbols.Symbol) (*Info, bool) {
	info, ok := t.infos[sym]
	return info, ok
}

// Symbols returns the table's symbols in deterministic walk order.
func (t *Table) Symbols() []*symbols.Symbol { return t.order }

// Build computes the identity side table of every named symbol under the
// given roots, in root order.
func Build(model *semantics.Model, res *resolve.Resolver, roots ...*symbols.Scope) *Table {
	b := &builder{
		model:  model,
		res:    res,
		idx:    res.Index(),
		scopes: make(map[*symbols.Symbol]*Scope),
		known:  make(map[*symbols.Symbol]bool),
	}
	t := &Table{infos: make(map[*symbols.Symbol]*Info)}
	syms := collectSymbols(roots)
	// `about` annotations may target elements outside the roots — bundled
	// library ones included — and those targets still carry the identity
	// their annotations declare. A ProjectRef binds the target's whole
	// subtree to a project, so its descendants join the id space too.
	for _, sym := range model.AboutAnnotatedSymbols() {
		syms = append(syms, sym)
		if hasProjectRefSite(model, sym) {
			syms = append(syms, collectSymbols([]*symbols.Scope{sym.Scope})...)
		}
	}
	for _, sym := range syms {
		if _, ok := t.infos[sym]; ok {
			continue
		}
		info := b.infoOf(sym)
		if info == nil {
			continue
		}
		t.infos[sym] = info
		t.order = append(t.order, sym)
	}
	return t
}

type builder struct {
	model  *semantics.Model
	res    *resolve.Resolver
	idx    *symbols.Index
	scopes map[*symbols.Symbol]*Scope
	known  map[*symbols.Symbol]bool // scope memo: sym has an entry in scopes
}

// infoOf computes one symbol's identity, or nil for a symbol that has no
// qualified name to derive an id from (an alias adds no element of its own).
func (b *builder) infoOf(sym *symbols.Symbol) *Info {
	if sym == nil || sym.Decl == nil || sym.Kind == symbols.SymbolAlias {
		return nil
	}
	fqn := b.idx.GetFQN(sym)
	if fqn == "" {
		return nil
	}
	info := &Info{Symbol: sym, FQN: fqn, EffectiveID: rdf.EncodeElementID(fqn)}
	for _, site := range b.model.AnnotationSitesOf(sym) {
		if site.TypeFQN != ElementIdFQN {
			continue
		}
		d := Declaration{About: site.About, Node: site.Node, Span: site.Node.Span(), Scope: site.Scope}
		if id, ok := siteString(site, "id"); ok {
			d.Declared = true
			d.ID = id
		}
		info.Declarations = append(info.Declarations, d)
	}
	if len(info.Declarations) > 0 {
		first := info.Declarations[0]
		info.Annotated = true
		info.AnnotationSpan = first.Span
		info.Declared = first.Declared
		info.DeclaredID = first.ID
		for _, d := range info.Declarations {
			if d.Declared && d.ID != "" {
				info.EffectiveID = d.ID
				break
			}
		}
	}
	info.Scope = b.scopeOf(sym)
	return info
}

// scopeOf resolves the nearest enclosing ProjectRef binding of sym, the
// symbol's own annotation included, memoized over the enclosing chain.
func (b *builder) scopeOf(sym *symbols.Symbol) *Scope {
	if sym == nil {
		return nil
	}
	if b.known[sym] {
		return b.scopes[sym]
	}
	b.known[sym] = true
	var decls []ScopeDeclaration
	for _, site := range b.model.AnnotationSitesOf(sym) {
		if site.TypeFQN != ProjectRefFQN {
			continue
		}
		d := ScopeDeclaration{Node: site.Node, Span: site.Node.Span(), Scope: site.Scope}
		d.ProjectID, _ = siteString(site, "projectId")
		d.Branch, _ = siteString(site, "branch")
		d.Org, _ = siteString(site, "org")
		decls = append(decls, d)
	}
	if len(decls) > 0 {
		first := decls[0]
		s := &Scope{
			ProjectID:    first.ProjectID,
			Branch:       first.Branch,
			Org:          first.Org,
			Symbol:       sym,
			Declarations: decls,
		}
		b.scopes[sym] = s
		return s
	}
	s := b.scopeOf(enclosingSymbol(sym))
	b.scopes[sym] = s
	return s
}

// hasProjectRefSite reports a ProjectRef annotation among sym's sites.
func hasProjectRefSite(model *semantics.Model, sym *symbols.Symbol) bool {
	for _, site := range model.AnnotationSitesOf(sym) {
		if site.TypeFQN == ProjectRefFQN {
			return true
		}
	}
	return false
}

// enclosingSymbol is the declaration sym is nested in, or nil at a root.
func enclosingSymbol(sym *symbols.Symbol) *symbols.Symbol {
	for sc := sym.OwnerScope; sc != nil; sc = sc.Parent() {
		if owner := sc.Owner(); owner != nil && owner != sym {
			return owner
		}
	}
	return nil
}

// siteString is the constant string value one annotation binds to feature.
func siteString(site semantics.AnnotationSite, feature string) (string, bool) {
	for _, v := range site.Values {
		if v.Feature == feature && v.Value.Kind == symbols.FilterValueString {
			return v.Value.Str, true
		}
	}
	return "", false
}

// collectSymbols visits every symbol of the scope subtrees exactly once, in
// declaration order.
func collectSymbols(roots []*symbols.Scope) []*symbols.Symbol {
	seenSyms := make(map[*symbols.Symbol]bool)
	seenScopes := make(map[*symbols.Scope]bool)
	var out []*symbols.Symbol
	var walk func(*symbols.Scope)
	walk = func(scope *symbols.Scope) {
		if scope == nil || seenScopes[scope] {
			return
		}
		seenScopes[scope] = true
		scope.ForEachMember(func(sym *symbols.Symbol) bool {
			if sym == nil || seenSyms[sym] {
				return true
			}
			seenSyms[sym] = true
			out = append(out, sym)
			walk(sym.Scope)
			return true
		})
		for _, child := range scope.Children() {
			walk(child)
		}
	}
	for _, root := range roots {
		walk(root)
	}
	return out
}
