package symbols

import "github.com/Open-MBEE/Systemica/internal/core/ast"

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
	docRoots      map[string]*Scope     // document name -> root scope
	fqn           map[string][]*Symbol  // fully-qualified name -> symbols
	contributions map[string][]fqnEntry // document name -> entries it added
	wildcardMeta  map[string][]string   // package FQN -> target FQNs for wildcard imports
}

// NewIndex creates an empty index.
func NewIndex() *Index {
	return &Index{
		docRoots:      make(map[string]*Scope),
		fqn:           make(map[string][]*Symbol),
		contributions: make(map[string][]fqnEntry),
		wildcardMeta:  make(map[string][]string),
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

// ExpandWildcardImports performs a post-indexing pass to add re-exported symbols.
// For each package with wildcard imports like `import ISQMechanics::*`, this adds
// ISQMechanics members as visible through the importing package's FQN.
// Call this after all documents are indexed.
func (idx *Index) ExpandWildcardImports() {
	// Use metadata from wildcard imports
	for pkgFQN, targets := range idx.wildcardMeta {
		for _, targetFQN := range targets {
			targetChildren := idx.LookupDirectChildren(targetFQN)
			for _, child := range targetChildren {
				// Extract child's primary name
				childName := child.Name
				if i := lastIndex(childName, "::"); i >= 0 {
					childName = childName[i+2:]
				}
				// Add child under importing package's FQN
				reexportFQN := joinFQN(pkgFQN, childName)
				// Don't add duplicates
				if !idx.hasFQN(reexportFQN, child) {
					idx.fqn[reexportFQN] = append(idx.fqn[reexportFQN], child)
					// Note: not added to contributions - these are synthetic
				}
				
				// Also re-export under short name if different from primary name
				if child.ShortName != "" && child.ShortName != childName {
					shortReexportFQN := joinFQN(pkgFQN, child.ShortName)
					if !idx.hasFQN(shortReexportFQN, child) {
						idx.fqn[shortReexportFQN] = append(idx.fqn[shortReexportFQN], child)
					}
				}
			}
		}
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
		syms := idx.fqn[e.fqn]
		for i, s := range syms {
			if s == e.sym {
				syms = append(syms[:i], syms[i+1:]...)
				break
			}
		}
		if len(syms) == 0 {
			delete(idx.fqn, e.fqn)
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

// extractWildcardImports extracts the target names of wildcard imports from a Package, Namespace, or RootNamespace AST node.
// Returns the raw qualified name text (e.g., "ISQBase") for each `import <name>::*` statement.
func extractWildcardImports(decl ast.Node) []string {
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
	
	var out []string
	for _, m := range members {
		imp, ok := m.(*ast.Import)
		if !ok || imp.Kind != ast.ImportNamespace || imp.Imported == nil {
			continue
		}
		out = append(out, qualifiedNameText(imp.Imported))
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

// LookupQualified returns all symbols registered under the exact
// fully-qualified name.
func (idx *Index) LookupQualified(fqn string) []*Symbol {
	return idx.fqn[fqn]
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
