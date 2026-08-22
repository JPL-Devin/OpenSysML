package model

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// VisibleName is one spelling resolution would consider at a point: the dotted
// path a reference there may be written as, and the element it reaches.
type VisibleName struct {
	// Name is the path in the pilot's notation, segments joined by ".".
	Name string
	// FQN is the "::"-qualified name of the element the path reaches.
	FQN string
	// Kind classifies the element, an alias resolved to its target: a caller
	// restricting the set by metatype — a specialization names a type, a
	// redefinition names a feature — needs what the name reaches, not the name.
	Kind symbols.SymbolKind
	// Depth is the number of segments in Name.
	Depth int
}

// VisibleNamesOptions bounds a visible-name enumeration.
type VisibleNamesOptions struct {
	// Redefinition enumerates only what the anchor's owner inherits: a
	// redefinition's target is resolved against the generals of the owning type
	// rather than against its own members (KerML 8.3.3.3.6).
	Redefinition bool
	// LibraryRoots names the standard-library root packages the caller counts
	// as loaded: `Base` alone admits the implicit `self` and `that` it
	// contributes to a declaration. A library element is reachable through
	// inheritance and imports only when its root is named here, and a library
	// root package is never offered as a top-level name of its own.
	LibraryRoots []string
	// MaxDepth bounds the number of segments in an enumerated path.
	MaxDepth int
}

// defaultVisibleNameDepth bounds path length. Namespace nesting is truncated at
// a visited path rather than at a depth, so this is only a safety valve.
const defaultVisibleNameDepth = 6

// VisibleNames enumerates every name resolution would consider in scope: the
// members of the enclosing namespace chain, what those namespaces inherit and
// import, and the document roots the index holds, each under every path it can
// be reached by. It is the read-only enumeration behind the single-name lookup
// resolve performs, and it answers both halves of a visibility question — a
// name that should be reachable and is not, and one that is reachable and
// should not be. Results are sorted by name, so two runs agree byte for byte.
func (w *Workspace) VisibleNames(scope *symbols.Scope, opts VisibleNamesOptions) []VisibleName {
	if scope == nil {
		return nil
	}
	w.mu.RLock()
	defer w.mu.RUnlock()

	r, sem := w.newResolver()
	nw := &nameWalk{
		idx:      w.index,
		r:        r,
		sem:      sem,
		doc:      symbols.DocNameOf(scope),
		maxDepth: opts.MaxDepth,
		library:  map[string]bool{},
		seen:     map[string]bool{},
	}
	for _, root := range opts.LibraryRoots {
		nw.library[root] = true
	}
	if nw.maxDepth <= 0 {
		nw.maxDepth = defaultVisibleNameDepth
	}
	nw.walk(scope, opts.Redefinition)
	sort.Slice(nw.out, func(i, j int) bool { return nw.out[i].Name < nw.out[j].Name })
	return nw.out
}

// VisibleNamesAt is VisibleNames for the deepest scope of a document that
// contains offset.
func (w *Workspace) VisibleNamesAt(doc string, offset int, opts VisibleNamesOptions) []VisibleName {
	d := w.Document(doc)
	if d == nil || d.Scope == nil {
		return nil
	}
	return w.VisibleNames(scopeAt(d.Scope, offset), opts)
}

// ScopeAt is the deepest scope of a document whose declaration contains offset,
// which is the namespace a name written there is resolved in.
func (w *Workspace) ScopeAt(doc string, offset int) *symbols.Scope {
	d := w.Document(doc)
	if d == nil || d.Scope == nil {
		return nil
	}
	return scopeAt(d.Scope, offset)
}

// scopeAt returns the deepest scope whose owning declaration contains offset.
func scopeAt(root *symbols.Scope, offset int) *symbols.Scope {
	for _, sym := range root.Members() {
		if sym.Scope == nil {
			continue
		}
		if sp := sym.DeclSpan; offset >= sp.Offset && offset < sp.End() {
			return scopeAt(sym.Scope, offset)
		}
	}
	return root
}

// nameWalk enumerates the paths reachable from one anchor scope. Each path is
// recorded once, the first time it is reached, which truncates the circularity
// an import or a specialization cycle would otherwise spin in.
type nameWalk struct {
	idx      *symbols.Index
	r        *resolve.Resolver
	sem      *semantics.Model
	doc      string
	maxDepth int
	// library are the standard-library root packages counted as loaded.
	library map[string]bool
	seen    map[string]bool
	out     []VisibleName
	// expanding are the elements whose members are being enumerated on the
	// current path, so a namespace that contains itself ends the descent.
	expanding map[*symbols.Symbol]bool
	// traversed counts the types the current path's derivation went through, so
	// a path never inherits through the same type twice.
	traversed map[*symbols.Symbol]int
	// chains memoizes the source steps from an owner to a member's declarer.
	chains map[[2]*symbols.Symbol][]*symbols.Symbol
	// pending maps a type to the declaration being written in it, whose
	// redefinition masks nothing there (KerML 8.3.3.3.6).
	pending map[*symbols.Symbol]pendingDecl
}

// pendingDecl is the declaration being written in a type. sym is nil when the
// walk cannot tell which of the type's declarations it is; hide reports whether
// that declaration is not yet a member of the type.
type pendingDecl struct {
	sym  *symbols.Symbol
	hide bool
}

// markPending records the declaration being written in a type. A package
// declares no features, so nothing is pending in it.
func (nw *nameWalk) markPending(owner *symbols.Symbol, decl pendingDecl) {
	if owner == nil || owner.Kind == symbols.SymbolPackage || owner.Kind == symbols.SymbolNamespace {
		return
	}
	nw.pending[owner] = decl
}

// membersOf reports the members of owner as the walk sees them: the declaration
// being written in it is absent, and the redefinitions it declares mask nothing.
func (nw *nameWalk) membersOf(owner *symbols.Symbol) []*symbols.Symbol {
	if decl, ok := nw.pending[owner]; ok {
		return nw.sem.MembersOfDeclaring(owner, decl.sym)
	}
	return nw.sem.MembersOf(owner)
}

// walk enumerates the anchor scope and its ancestors, then the index roots.
func (nw *nameWalk) walk(anchor *symbols.Scope, redefinition bool) {
	nw.expanding = map[*symbols.Symbol]bool{}
	nw.pending = map[*symbols.Symbol]pendingDecl{}
	nw.traversed = map[*symbols.Symbol]int{}
	nw.chains = map[[2]*symbols.Symbol][]*symbols.Symbol{}
	if redefinition && anchor != nil {
		// The anchor is the namespace the redefinition is written in: one of
		// its declarations is the redefining one, and which is not knowable
		// from a scope alone.
		owner := anchor.Owner()
		nw.markPending(owner, pendingDecl{hide: true})
		// It may instead be the redefining declaration's own scope — a cursor
		// in its head — in which case the namespace is its owner, and there the
		// declaration is known and stays visible under its own name.
		if semantics.DeclaresRedefinition(owner) && owner.OwnerScope != nil {
			nw.markPending(owner.OwnerScope.Owner(), pendingDecl{sym: owner})
		}
	}
	for s := anchor; s != nil; s = s.Parent() {
		if s == anchor && redefinition {
			nw.inherited(s.Owner(), "", 1, true)
			continue
		}
		nw.locals(s, "", 1, true, true)
		nw.inherited(s.Owner(), "", 1, true)
		nw.imports(s, "", 1, true, false)
	}
	nw.roots()
}

// roots adds the names the index registers at its root: every non-library
// document's top-level declarations.
func (nw *nameWalk) roots() {
	for _, b := range nw.idx.TopLevelBindings(nw.doc) {
		if nw.idx.Library(b.Sym) {
			continue
		}
		nw.add("", b.Name, b.Sym, 1)
	}
}

// locals adds what a scope declares directly. inside admits its private
// members, as a reference written within the namespace sees them; inheriting
// admits protected ones, which a specialization sees (KerML 8.2.3.3).
func (nw *nameWalk) locals(s *symbols.Scope, prefix string, depth int, inside, inheriting bool) {
	if s == nil {
		return
	}
	decl := nw.pending[s.Owner()]
	// The binding name is the key, not the symbol's own name: an alias member
	// binds a name to another element (KerML 7.3.4.4).
	for _, name := range s.MemberNames() {
		for _, sym := range s.LookupLocalAll(name) {
			if !symbols.VisibleAs(sym.Visibility, inside, inheriting) {
				continue
			}
			if decl.hide && semantics.NotYetMember(sym, decl.sym) {
				continue // not yet a member of the namespace it is written in
			}
			nw.add(prefix, leafName(name), sym, depth)
			nw.add(prefix, sym.ShortName, sym, depth)
		}
	}
}

// inherited adds the members owner takes from what it specializes, is typed by
// or reference-subsets, which the semantic model reports with masking applied.
func (nw *nameWalk) inherited(owner *symbols.Symbol, prefix string, depth int, inheriting bool) {
	// Only a Type has generals to inherit from: a package or a namespace has
	// no implicit specialization, so it contributes nothing here.
	if owner == nil || owner.Kind == symbols.SymbolPackage || owner.Kind == symbols.SymbolNamespace {
		return
	}
	// One filter chain: the semantic member view drops what a redefinition
	// masks, then membership visibility drops what cannot be named here, then
	// the path's own derivation drops what it would inherit twice over.
	for _, sym := range nw.membersOf(owner) {
		if !symbols.VisibleAs(sym.Visibility, false, inheriting) {
			continue
		}
		chain, ok := nw.enter(nw.chainTo(owner, declarerOf(sym)))
		if !ok {
			continue
		}
		nw.addInherited(prefix, leafName(sym.Name), sym, depth)
		nw.addInherited(prefix, sym.ShortName, sym, depth)
		nw.leave(chain)
	}
	// MembersOf reads a source's scope, which a cached standard-library symbol
	// does not carry, so its inherited members come from the index instead.
	for _, src := range nw.sem.MemberSources(owner) {
		if src == nil || src.Scope != nil {
			continue
		}
		chain, ok := nw.enter(nw.chainTo(owner, src))
		if !ok {
			continue
		}
		for _, child := range nw.idx.LookupDirectChildrenFrom(nw.fqnOf(src), "") {
			if !symbols.VisibleAs(child.Visibility, false, inheriting) {
				continue
			}
			nw.addInherited(prefix, leafName(child.Name), child, depth)
		}
		nw.leave(chain)
	}
	nw.inheritedImports(owner, prefix, depth, map[*symbols.Symbol]bool{})
}

func (nw *nameWalk) inheritedImports(owner *symbols.Symbol, prefix string, depth int, seen map[*symbols.Symbol]bool) {
	for _, src := range nw.sem.DirectMemberSources(owner) {
		if src == nil || seen[src] {
			continue
		}
		seen[src] = true
		if src.Scope != nil {
			nw.imports(src.Scope, prefix, depth, false, true)
		}
		nw.inheritedImports(src, prefix, depth, seen)
	}
}

// declarerOf is the element whose scope declares sym, which is the type a path
// inheriting sym went through.
func declarerOf(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	return sym.OwnerScope.Owner()
}

// chainTo returns the source steps from owner to declarer, breadth-first over
// the member-contributing edges: the types a path reaching declarer's members
// through owner inherits through. It is empty when owner declares them itself
// or when no chain of sources reaches declarer.
func (nw *nameWalk) chainTo(owner, declarer *symbols.Symbol) []*symbols.Symbol {
	if owner == nil || declarer == nil || declarer == owner {
		return nil
	}
	key := [2]*symbols.Symbol{owner, declarer}
	if chain, ok := nw.chains[key]; ok {
		return chain
	}
	from := map[*symbols.Symbol]*symbols.Symbol{owner: nil}
	queue := []*symbols.Symbol{owner}
	var found bool
	for len(queue) > 0 && !found {
		cur := queue[0]
		queue = queue[1:]
		for _, src := range nw.sem.DirectMemberSources(cur) {
			if _, seen := from[src]; seen {
				continue
			}
			from[src] = cur
			if src == declarer {
				found = true
				break
			}
			queue = append(queue, src)
		}
	}
	var chain []*symbols.Symbol
	if found {
		for s := declarer; s != nil && s != owner; s = from[s] {
			chain = append(chain, s)
		}
	}
	nw.chains[key] = chain
	return chain
}

// enter marks a derivation's source steps as traversed on the current path, or
// reports false when one of them already is: the pilot's enumeration stops
// there rather than inheriting through a type twice (KerML 8.2.3.5).
func (nw *nameWalk) enter(chain []*symbols.Symbol) ([]*symbols.Symbol, bool) {
	for _, src := range chain {
		if nw.traversed[src] > 0 {
			return nil, false
		}
	}
	for _, src := range chain {
		nw.traversed[src]++
	}
	return chain, true
}

// leave undoes enter.
func (nw *nameWalk) leave(chain []*symbols.Symbol) {
	for _, src := range chain {
		nw.traversed[src]--
	}
}

// imports adds the imports visible inside a namespace or through inheritance.
func (nw *nameWalk) imports(s *symbols.Scope, prefix string, depth int, inside, inheriting bool) {
	if s == nil {
		return
	}
	for _, imp := range importsIn(s.Node()) {
		if !symbols.VisibleAs(imp.Visibility, inside, inheriting) {
			continue
		}
		elements := nw.r.ImportedElements(s, imp)
		for i, sym := range elements {
			// A membership import surfaces only the name written last in it,
			// which for an alias membership is the alias name, not the target's;
			// its recursive form adds the subtree under their own names.
			if imp.Kind == ast.ImportMembership && i == 0 {
				written := importedName(imp)
				nw.addPath(prefix, written, sym, depth, inheriting)
				// An element imported by one of its own two names keeps both;
				// an alias name belongs to the alias, not to its target.
				if written == leafName(sym.Name) || written == sym.ShortName {
					nw.addPath(prefix, leafName(sym.Name), sym, depth, inheriting)
					nw.addPath(prefix, sym.ShortName, sym, depth, inheriting)
				}
				continue
			}
			nw.addPath(prefix, leafName(sym.Name), sym, depth, inheriting)
			nw.addPath(prefix, sym.ShortName, sym, depth, inheriting)
		}
	}
}

// add records one path and then the paths that lead through it.
func (nw *nameWalk) add(prefix, name string, sym *symbols.Symbol, depth int) {
	nw.addPath(prefix, name, sym, depth, false)
}

func (nw *nameWalk) addInherited(prefix, name string, sym *symbols.Symbol, depth int) {
	nw.addPath(prefix, name, sym, depth, true)
}

func (nw *nameWalk) addPath(prefix, name string, sym *symbols.Symbol, depth int, recordCycle bool) {
	if name == "" || sym == nil || depth > nw.maxDepth {
		return
	}
	qn := name
	if prefix != "" {
		qn = prefix + "." + name
	}
	if nw.seen[qn] {
		return
	}
	nw.seen[qn] = true
	target := sym
	if t, ok := nw.r.ResolveAliasTarget(sym); ok && t != nil {
		target = t
	}
	if !nw.loaded(target) {
		return
	}
	if nw.expanding[target] && !recordCycle {
		return
	}
	nw.out = append(nw.out, VisibleName{
		Name:  qn,
		FQN:   nw.fqnOf(target),
		Kind:  target.Kind,
		Depth: depth,
	})
	if nw.expanding[target] {
		return
	}
	nw.expand(qn, target, depth+1)
}

// expand adds the paths through a name: the public members of the element it
// reaches, which are what a qualified reference may name (KerML 8.2.3.3).
func (nw *nameWalk) expand(prefix string, sym *symbols.Symbol, depth int) {
	if depth > nw.maxDepth || nw.expanding[sym] {
		return
	}
	nw.expanding[sym] = true
	defer delete(nw.expanding, sym)

	if sym.Scope != nil {
		nw.locals(sym.Scope, prefix, depth, false, false)
		nw.inherited(sym, prefix, depth, false)
		nw.imports(sym.Scope, prefix, depth, false, false)
		return
	}
	// A cached library symbol carries no scope; its members are the children
	// the index registers under it.
	if fqn := nw.fqnOf(sym); fqn != "" {
		for _, child := range nw.idx.LookupDirectChildrenFrom(fqn, "") {
			nw.add(prefix, leafName(child.Name), child, depth)
		}
	}
	nw.inherited(sym, prefix, depth, false)
}

// loaded reports whether the caller's resource set holds an element: a library
// element counts only when its root package is named as loaded.
func (nw *nameWalk) loaded(sym *symbols.Symbol) bool {
	if !nw.idx.Library(sym) {
		return true
	}
	root := nw.fqnOf(sym)
	if i := strings.Index(root, "::"); i >= 0 {
		root = root[:i]
	}
	return nw.library[root]
}

// fqnOf is the qualified name the index registers sym under, falling back to
// the one its scope chain spells out.
func (nw *nameWalk) fqnOf(sym *symbols.Symbol) string {
	if fqn := nw.idx.GetFQN(sym); fqn != "" {
		return fqn
	}
	return sym.Name
}

// importedName is the last name segment a membership import writes.
func importedName(imp *ast.Import) string {
	if imp.Imported == nil || len(imp.Imported.Parts) == 0 {
		return ""
	}
	return imp.Imported.Parts[len(imp.Imported.Parts)-1].Text
}

// leafName is a name without the qualification an indexed symbol carries.
func leafName(name string) string {
	if i := strings.LastIndex(name, "::"); i >= 0 {
		return name[i+2:]
	}
	return name
}

// importsIn returns the imports declared directly in a namespace-bearing node.
func importsIn(node ast.Node) []*ast.Import {
	var members []ast.Node
	switch n := node.(type) {
	case *ast.Package:
		members = n.Members
	case *ast.Namespace:
		members = n.Members
	case *ast.RootNamespace:
		members = n.Members
	case *ast.Definition:
		members = n.Members
	case *ast.Usage:
		members = n.Members
	default:
		return nil
	}
	var out []*ast.Import
	for _, m := range members {
		if imp, ok := m.(*ast.Import); ok {
			out = append(out, imp)
		}
	}
	return out
}

// ElementOnPath resolves a dotted path from a scope to the element it names:
// the first segment as an unqualified name, each later one as a member of the
// previous, an alias replaced by its target.
func (w *Workspace) ElementOnPath(scope *symbols.Scope, path []string) (*symbols.Symbol, bool) {
	if scope == nil || len(path) == 0 {
		return nil, false
	}
	w.mu.RLock()
	defer w.mu.RUnlock()

	r, sem := w.newResolver()
	sym, ok := r.ResolveName(scope, path[0], nil)
	if !ok || sym == nil {
		return nil, false
	}
	for _, seg := range path[1:] {
		if sym, ok = sem.LookupMember(sym, seg); !ok || sym == nil {
			return nil, false
		}
	}
	if target, ok := r.ResolveAliasTarget(sym); ok && target != nil {
		sym = target
	}
	return sym, true
}

// FQNOf is the qualified name the index registers an element under, which is
// how a caller comparing enumerations tells two paths to one element apart.
func (w *Workspace) FQNOf(sym *symbols.Symbol) string {
	if sym == nil {
		return ""
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	if fqn := w.index.GetFQN(sym); fqn != "" {
		return fqn
	}
	return sym.Name
}
