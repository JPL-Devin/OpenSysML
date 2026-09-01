// Package identity computes the effective repository identity of the symbols
// of an analyzed model: the id of an element's IdentityMetadata::ElementId
// annotation when it carries one, the id derived from its qualified name
// otherwise, plus the project scope its nearest enclosing ProjectRef declares.
// It is a side table keyed by symbol; the AST is never touched.
package identity

import (
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
}

// Key names the project a scope binds to, for scope-equality grouping.
func (s *Scope) Key() string {
	if s == nil {
		return ""
	}
	return s.Org + "\x00" + s.ProjectID
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
	for _, sym := range collectSymbols(roots) {
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
	if span, ok := b.annotationSpan(sym, ElementIdFQN); ok {
		info.Annotated = true
		info.AnnotationSpan = span
		if id, ok := b.stringFact(sym, ElementIdFQN, "id"); ok {
			info.Declared = true
			info.DeclaredID = id
			if id != "" {
				info.EffectiveID = id
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
	if _, ok := b.annotationSpan(sym, ProjectRefFQN); ok {
		s := &Scope{Symbol: sym}
		s.ProjectID, _ = b.stringFact(sym, ProjectRefFQN, "projectId")
		s.Branch, _ = b.stringFact(sym, ProjectRefFQN, "branch")
		s.Org, _ = b.stringFact(sym, ProjectRefFQN, "org")
		b.scopes[sym] = s
		return s
	}
	s := b.scopeOf(enclosingSymbol(sym))
	b.scopes[sym] = s
	return s
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

// annotationSpan reports whether sym's declaration carries an annotation of
// the given metadata type, and where it is written.
func (b *builder) annotationSpan(sym *symbols.Symbol, typeFQN string) (source.Span, bool) {
	for _, a := range semantics.MetadataAnnotationsOf(sym.Decl) {
		if a.Node == nil || a.Node.Type == nil {
			continue
		}
		scope := sym.OwnerScope
		if !a.Prefix && sym.Scope != nil {
			scope = sym.Scope
		}
		typ, ok := b.res.ResolveQualified(scope, a.Node.Type)
		if !ok || typ == nil {
			continue
		}
		if resolved, aliasOK := b.res.ResolveAliasTarget(typ); aliasOK {
			typ = resolved
		}
		if b.idx.GetFQN(typ) == typeFQN {
			return a.Node.Span(), true
		}
	}
	return source.Span{}, false
}

// stringFact is the constant string value an annotation of typeFQN binds to
// feature, evaluated by the model.
func (b *builder) stringFact(sym *symbols.Symbol, typeFQN, feature string) (string, bool) {
	for _, facts := range b.model.AnnotationFactsOf(sym) {
		if facts.TypeFQN != typeFQN {
			continue
		}
		for _, v := range facts.Values {
			if v.Feature == feature && v.Value.Kind == symbols.FilterValueString {
				return v.Value.Str, true
			}
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
