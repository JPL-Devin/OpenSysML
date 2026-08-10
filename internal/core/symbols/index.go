package symbols

import (
	"sort"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

// fqnEntry records one symbol registered under a fully-qualified name.
type fqnEntry struct {
	fqn string
	sym *Symbol
}

// Index aggregates symbol information across all documents in a workspace.
// It owns each document's root scope and a global map from fully-qualified
// name to the symbol(s) declared under it. Per-document contributions are
// tracked so a document can be removed or re-added without leaving stale
// entries.
type Index struct {
	docRoots      map[string]*Scope           // document name -> root scope
	fqn           map[string][]*Symbol        // fully-qualified name -> symbols
	contributions map[string][]fqnEntry       // document name -> entries it added
	wildcardMeta  map[string][]WildcardImport // package FQN -> its wildcard imports

	// reexported marks the (FQN, symbol) pairs that a wildcard import made
	// visible rather than the namespace declaring them, so a lookup can prefer
	// the declared member. hidden is the subset a *private* import surfaced,
	// which a further wildcard import must not carry on.
	reexported map[string]map[*Symbol]bool
	hidden     map[string]map[*Symbol]bool

	// declaredAt maps a symbol to the FQN its declaration gives it, which is the
	// only key its own members are registered under: a re-export registers the
	// symbol elsewhere but never copies its subtree.
	declaredAt map[*Symbol]string
}

// WildcardImport is one `import X::*` declaration: the target's raw qualified
// name text and whether the import was declared private.
type WildcardImport struct {
	Target  string
	Private bool
}

// NewIndex creates an empty index.
func NewIndex() *Index {
	return &Index{
		docRoots:      make(map[string]*Scope),
		fqn:           make(map[string][]*Symbol),
		contributions: make(map[string][]fqnEntry),
		wildcardMeta:  make(map[string][]WildcardImport),
		reexported:    make(map[string]map[*Symbol]bool),
		hidden:        make(map[string]map[*Symbol]bool),
		declaredAt:    make(map[*Symbol]string),
	}
}

// AddDocument builds the scope tree for root and records its symbols under
// their fully-qualified names. Re-adding the same document name first removes
// the document's previous contributions, so the index stays exact.
func (idx *Index) AddDocument(name string, root *ast.RootNamespace) {
	idx.RemoveDocument(name)
	rs := Build(root)
	SetDocName(rs, name)
	idx.docRoots[name] = rs
	idx.indexScope(name, rs, "")

	// Extract wildcard imports from root namespace itself
	// (root is not a symbol, so indexScope won't process its imports)
	if wildcards := extractWildcardImports(root); len(wildcards) > 0 {
		idx.wildcardMeta[""] = wildcards
	}
}

// ExpandWildcardImports adds re-exported symbols for every package with a
// wildcard import like `import ISQMechanics::*`, making the target's members
// visible through the importing package's FQN. Call it after all documents are
// indexed.
//
// Imports chain — KerML imports Kernel::*, which imports Core::*, which imports
// Root::* — so a single pass would only propagate one level and its result
// would depend on the order the importing packages happened to be visited in.
// Passes therefore repeat until nothing new is re-exported, over the importers
// in name order, which makes the outcome independent of both map iteration
// order and of whether a document was parsed or restored from cache.
func (idx *Index) ExpandWildcardImports() {
	for idx.expandWildcardImportsPass() {
	}
}

// expandWildcardImportsPass re-exports one level of wildcard imports and
// reports whether it registered anything new.
func (idx *Index) expandWildcardImportsPass() bool {
	added := false
	pkgFQNs := make([]string, 0, len(idx.wildcardMeta))
	for pkgFQN := range idx.wildcardMeta {
		pkgFQNs = append(pkgFQNs, pkgFQN)
	}
	sort.Strings(pkgFQNs)
	for _, pkgFQN := range pkgFQNs {
		targets := idx.wildcardMeta[pkgFQN]
		for _, target := range targets {
			// Resolve target FQN: may be absolute (ISQMechanics) or relative (Systems)
			targetFQN := idx.resolveWildcardTarget(pkgFQN, target.Target)
			if targetFQN == "" {
				continue // Target not found
			}

			targetChildren := idx.exportedChildren(targetFQN)
			for _, child := range targetChildren {
				// Extract child's primary name
				childName := child.Name
				if i := lastIndex(childName, "::"); i >= 0 {
					childName = childName[i+2:]
				}
				// Add child under importing package's FQN
				if idx.reexport(joinFQN(pkgFQN, childName), child, target.Private) {
					added = true
				}

				// Also re-export under short name if different from primary name
				if child.ShortName != "" && child.ShortName != childName {
					if idx.reexport(joinFQN(pkgFQN, child.ShortName), child, target.Private) {
						added = true
					}
				}
			}
		}
	}
	return added
}

// resolveWildcardTarget resolves a wildcard import target name to the
// fully-qualified name it names. Handles both absolute references
// (ISQMechanics) and references relative to the importing package (Systems
// within SysML). Returns "" if the target is unknown or ambiguous.
//
// The answer is the FQN the target was declared under, not the matched symbol's
// Name: a symbol built from a parsed document carries only its local name,
// while one restored from a cache record carries its fully-qualified one.
//
// A relative target is searched from the importing package outward through its
// enclosing packages before the global namespace, as KerML 8.2.3.5 resolves a
// name: KerML::Core's `import Root::*` names its sibling KerML::Root.
func (idx *Index) resolveWildcardTarget(pkgFQN, targetText string) string {
	for prefix := pkgFQN; prefix != ""; {
		if fqn, ok := idx.wildcardTargetAt(prefix + "::" + targetText); ok {
			return fqn
		}
		i := lastIndex(prefix, "::")
		if i < 0 {
			break
		}
		prefix = prefix[:i]
	}

	// Global namespace
	if fqn, ok := idx.wildcardTargetAt(targetText); ok {
		return fqn
	}

	// Target not found or ambiguous
	return ""
}

// wildcardTargetAt reports the FQN a wildcard import reads its members from
// when key names exactly one namespace, and whether it does.
//
// A namespace declared under key holds its members there, and shadows anything
// a wildcard import also re-exported under that name (SI::min is SI's minute).
// Either way the answer is the FQN the symbol was declared under, the only key
// its members are registered under: neither a re-export nor the short-name entry
// of `package <USCU> USCustomaryUnits` copies its subtree.
func (idx *Index) wildcardTargetAt(key string) (string, bool) {
	imported := idx.reexported[key]
	owned, reexports := 0, 0
	var soleOwned, soleImported *Symbol
	for _, sym := range idx.fqn[key] {
		if imported[sym] {
			reexports++
			soleImported = sym
			continue
		}
		owned++
		soleOwned = sym
	}
	switch {
	case owned == 1:
		if declared, ok := idx.declaredAt[soleOwned]; ok {
			return declared, true
		}
		return key, true
	case owned == 0 && reexports == 1:
		declared, ok := idx.declaredAt[soleImported]
		return declared, ok
	default:
		return "", false // unknown, or ambiguous between namespaces
	}
}

func (idx *Index) hasFQN(fqn string, sym *Symbol) bool {
	for _, s := range idx.fqn[fqn] {
		if s == sym {
			return true
		}
	}
	return false
}

func lastIndex(s, substr string) int {
	result := -1
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			result = i
		}
	}
	return result
}

// RemoveDocument drops all of the named document's contributions from the
// global index and forgets its root scope. Unknown names are a no-op.
func (idx *Index) RemoveDocument(name string) {
	for _, e := range idx.contributions[name] {
		if idx.declaredAt[e.sym] == e.fqn {
			delete(idx.declaredAt, e.sym)
		}
		syms := idx.fqn[e.fqn]
		for i, s := range syms {
			if s == e.sym {
				syms = append(syms[:i], syms[i+1:]...)
				break
			}
		}
		if len(syms) == 0 {
			delete(idx.fqn, e.fqn)
			delete(idx.reexported, e.fqn)
			delete(idx.hidden, e.fqn)
		} else {
			idx.fqn[e.fqn] = syms
		}
	}
	delete(idx.contributions, name)
	delete(idx.docRoots, name)
}

// indexScope walks a scope, recording each distinct symbol under its FQN and
// recursing into child scopes. prefix is the FQN of the owning scope ("" at
// the document root). Every recorded (fqn, symbol) pair is also tracked as a
// contribution of the named document.
func (idx *Index) indexScope(doc string, scope *Scope, prefix string) {
	seen := make(map[*Symbol]bool)
	for _, syms := range scope.members {
		for _, sym := range syms {
			if seen[sym] {
				continue // symbol registered under both short and primary key
			}
			seen[sym] = true

			// Index under primary FQN
			fqn := joinFQN(prefix, sym.Name)
			idx.fqn[fqn] = append(idx.fqn[fqn], sym)
			idx.declaredAt[sym] = fqn
			idx.contributions[doc] = append(idx.contributions[doc], fqnEntry{fqn: fqn, sym: sym})

			// Also index under short name FQN if different
			// Try cached shortName first (for stdlib), fallback to extracting from Decl
			shortName := sym.ShortName
			if shortName == "" {
				shortName = shortNameOf(sym.Decl)
			}
			if shortName != "" && shortName != sym.Name {
				shortFQN := joinFQN(prefix, shortName)
				idx.fqn[shortFQN] = append(idx.fqn[shortFQN], sym)
				idx.contributions[doc] = append(idx.contributions[doc], fqnEntry{fqn: shortFQN, sym: sym})
			}

			// Extract wildcard imports from packages/namespaces
			if sym.Kind == SymbolPackage || sym.Kind == SymbolNamespace {
				if wildcards := extractWildcardImports(sym.Decl); len(wildcards) > 0 {
					idx.wildcardMeta[fqn] = wildcards
				}
			}

			if sym.Scope != nil {
				idx.indexScope(doc, sym.Scope, fqn)
			}
		}
	}
}

// joinFQN joins a prefix and a name with "::".
func joinFQN(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "::" + name
}

// extractWildcardImports extracts the wildcard imports of a Package, Namespace,
// or RootNamespace AST node: the raw qualified name text (e.g. "ISQBase") and
// declared visibility of each `import <name>::*` statement.
func extractWildcardImports(decl ast.Node) []WildcardImport {
	var members []ast.Node
	switch d := decl.(type) {
	case *ast.Package:
		members = d.Members
	case *ast.Namespace:
		members = d.Members
	case *ast.RootNamespace:
		members = d.Members
	default:
		return nil
	}

	var out []WildcardImport
	for _, m := range members {
		imp, ok := m.(*ast.Import)
		if !ok || imp.Kind != ast.ImportNamespace || imp.Imported == nil {
			continue
		}
		out = append(out, WildcardImport{
			Target:  qualifiedNameText(imp.Imported),
			Private: imp.Visibility == ast.VisibilityPrivate,
		})
	}
	return out
}

// qualifiedNameText renders a QualifiedName as "A::B::C".
func qualifiedNameText(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	var parts []string
	for _, seg := range qn.Parts {
		parts = append(parts, seg.Text)
	}
	return joinQualifiedName(parts)
}

// joinQualifiedName joins parts with "::".
func joinQualifiedName(parts []string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += "::"
		}
		result += part
	}
	return result
}

// shortNameOf extracts the short name from a declaration's Identification.
// Returns "" if the node has no Identification or no short name.
func shortNameOf(decl ast.Node) string {
	switch d := decl.(type) {
	case *ast.Package:
		return d.Ident.ShortName
	case *ast.Namespace:
		return d.Ident.ShortName
	case *ast.Definition:
		return d.Ident.ShortName
	case *ast.Usage:
		return d.Ident.ShortName
	case *ast.Alias:
		return d.Ident.ShortName
	default:
		return ""
	}
}

// LookupQualified returns the symbols a qualified reference from outside the
// naming namespace reaches under the exact fully-qualified name. A namespace's
// own member shadows one of the same name that a wildcard import re-exported
// through it, as in SI::min, which is SI's minute and not the imported min
// function, and a name only a *private* import surfaced is not reachable at all:
// it is a member of the namespace, but not a visible one (KerML 8.2.3.3).
func (idx *Index) LookupQualified(fqn string) []*Symbol {
	return idx.LookupQualifiedFrom(fqn, "")
}

// LookupQualifiedFrom is LookupQualified as seen from the namespace named by
// fromFQN. A private import is visible inside the namespace that declares it and
// inside everything nested in it, so a reference made from there — including the
// target of an alias the namespace declares — still reaches a privately imported
// name that the same lookup from anywhere else does not (KerML 8.2.3.3).
//
// fromFQN is the FQN of the referring namespace; "" means "from outside", which
// is what an ordinary qualified reference elsewhere in the workspace gets.
func (idx *Index) LookupQualifiedFrom(fqn, fromFQN string) []*Symbol {
	syms := idx.fqn[fqn]
	imported := idx.reexported[fqn]
	if len(imported) == 0 {
		return syms
	}
	hidden := idx.hidden[fqn]
	if len(hidden) > 0 && !withinNamespace(fromFQN, namespaceOf(fqn)) {
		visible := make([]*Symbol, 0, len(syms))
		for _, sym := range syms {
			if !hidden[sym] {
				visible = append(visible, sym)
			}
		}
		syms = visible
	}
	owned := make([]*Symbol, 0, len(syms))
	for _, sym := range syms {
		if !imported[sym] {
			owned = append(owned, sym)
		}
	}
	if len(owned) == 0 {
		return syms
	}
	return owned
}

// HiddenFrom reports whether every symbol registered under fqn is one only a
// private import surfaced there, seen from the namespace fromFQN. It is the
// reason LookupQualifiedFrom found nothing, so a caller that falls back to
// another lookup route — the qualified walk's inheritance-aware member search,
// which reaches cached symbols through LookupDirectChildren — asks here first
// and stops, rather than resurfacing a name KerML 8.2.3.3 hides.
func (idx *Index) HiddenFrom(fqn, fromFQN string) bool {
	hidden := idx.hidden[fqn]
	if len(hidden) == 0 || withinNamespace(fromFQN, namespaceOf(fqn)) {
		return false
	}
	for _, sym := range idx.fqn[fqn] {
		if !hidden[sym] {
			return false
		}
	}
	return true
}

// namespaceOf returns the FQN of the namespace a qualified name names a member
// of: "A::B::C" -> "A::B", and "" for a top-level name.
func namespaceOf(fqn string) string {
	i := lastIndex(fqn, "::")
	if i < 0 {
		return ""
	}
	return fqn[:i]
}

// withinNamespace reports whether a reference made from the namespace fromFQN
// sees ns's private memberships, which it does when it *is* ns or is nested
// inside it. A reference from outside any namespace ("") never does, and neither
// does one from a namespace that merely shares a name prefix ("A::BC" is not in
// "A::B").
func withinNamespace(fromFQN, ns string) bool {
	if fromFQN == "" {
		return false
	}
	if ns == "" {
		return false
	}
	if fromFQN == ns {
		return true
	}
	return len(fromFQN) > len(ns)+2 && fromFQN[:len(ns)] == ns && fromFQN[len(ns):len(ns)+2] == "::"
}

// FQNs returns every fully-qualified name registered in the index, sorted.
func (idx *Index) FQNs() []string {
	out := make([]string, 0, len(idx.fqn))
	for fqn := range idx.fqn {
		out = append(out, fqn)
	}
	sort.Strings(out)
	return out
}

// WildcardImportsOf returns the wildcard-import targets recorded for the
// namespace registered under fqn ("" for a document root).
func (idx *Index) WildcardImportsOf(fqn string) []WildcardImport {
	return idx.wildcardMeta[fqn]
}

// reexport registers sym under fqn on behalf of a wildcard import and reports
// whether anything changed. An entry the importing namespace declares itself is
// left alone: a cycle of wildcard imports brings a package its own members back,
// and they are not borrowed.
func (idx *Index) reexport(fqn string, sym *Symbol, private bool) bool {
	if !idx.hasFQN(fqn, sym) {
		// Note: not added to contributions - these are synthetic
		idx.fqn[fqn] = append(idx.fqn[fqn], sym)
		idx.markReexported(fqn, sym, private)
		return true
	}
	if !idx.reexported[fqn][sym] {
		return false
	}
	return idx.markReexported(fqn, sym, private)
}

// markReexported records that fqn only names sym by way of a wildcard import,
// hidden while every import that surfaced it was private, and reports whether
// anything changed. A public import of a name a private one already brought in
// exports it, so the mark only ever clears.
func (idx *Index) markReexported(fqn string, sym *Symbol, private bool) bool {
	if idx.reexported[fqn] == nil {
		idx.reexported[fqn] = make(map[*Symbol]bool)
	}
	switch {
	case !idx.reexported[fqn][sym]:
		idx.reexported[fqn][sym] = true
		if private {
			if idx.hidden[fqn] == nil {
				idx.hidden[fqn] = make(map[*Symbol]bool)
			}
			idx.hidden[fqn][sym] = true
		}
		return true
	case private:
		return false
	case idx.hidden[fqn][sym]:
		delete(idx.hidden[fqn], sym)
		return true
	default:
		return false
	}
}

// exportedChildren returns the direct children of prefix that a wildcard import
// of prefix surfaces: everything but what prefix's own private imports brought
// in, which stays visible only inside prefix (KerML 8.2.3.3).
func (idx *Index) exportedChildren(prefix string) []*Symbol {
	var out []*Symbol
	seen := make(map[*Symbol]bool)
	targetPrefix := prefix + "::"
	for fqn, syms := range idx.fqn {
		if len(fqn) <= len(targetPrefix) || fqn[:len(targetPrefix)] != targetPrefix {
			continue
		}
		if containsString(fqn[len(targetPrefix):], "::") {
			continue
		}
		hidden := idx.hidden[fqn]
		for _, sym := range syms {
			if seen[sym] || hidden[sym] {
				continue
			}
			seen[sym] = true
			out = append(out, sym)
		}
	}
	return out
}

// LookupDirectChildren returns all symbols whose FQN is exactly prefix::name
// (direct children of the given prefix). This supports wildcard imports from
// packages that don't have populated Scopes.
func (idx *Index) LookupDirectChildren(prefix string) []*Symbol {
	var out []*Symbol
	seen := make(map[*Symbol]bool)
	targetPrefix := prefix + "::"
	for fqn, syms := range idx.fqn {
		// Check if this FQN starts with prefix:: and has no further :: after that
		if len(fqn) > len(targetPrefix) && fqn[:len(targetPrefix)] == targetPrefix {
			remainder := fqn[len(targetPrefix):]
			// Only include if remainder has no "::" (direct child)
			if !containsString(remainder, "::") {
				for _, sym := range syms {
					if !seen[sym] {
						seen[sym] = true
						out = append(out, sym)
					}
				}
			}
		}
	}
	return out
}

func containsString(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetFQN returns the fully-qualified name for a symbol by walking its owner scope chain.
// Returns the local name if the symbol has no owner scope (root-level symbol).
func (idx *Index) GetFQN(sym *Symbol) string {
	if sym == nil {
		return ""
	}

	// Collect scope chain from symbol up to root
	var parts []string
	parts = append(parts, sym.Name)

	scope := sym.OwnerScope
	for scope != nil && scope.Owner() != nil {
		owner := scope.Owner()
		parts = append(parts, owner.Name)
		scope = owner.OwnerScope
	}

	// Reverse parts (collected from leaf to root)
	for i := 0; i < len(parts)/2; i++ {
		j := len(parts) - 1 - i
		parts[i], parts[j] = parts[j], parts[i]
	}

	// Join with "::"
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += "::" + parts[i]
	}
	return result
}

// DocumentRoot returns the root scope for the named document, or nil.
func (idx *Index) DocumentRoot(name string) *Scope {
	return idx.docRoots[name]
}

// NewIndexFromDoc builds an Index containing a single document.
func NewIndexFromDoc(name string, root *ast.RootNamespace) *Index {
	idx := NewIndex()
	idx.AddDocument(name, root)
	return idx
}
