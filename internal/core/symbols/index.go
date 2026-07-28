package symbols

import "github.com/Open-MBEE/Systemica/internal/core/ast"

// Index aggregates symbol information across all documents in a workspace.
// It owns each document's root scope and a global map from fully-qualified
// name to the symbol(s) declared under it.
type Index struct {
	docRoots map[string]*Scope    // document name -> root scope
	fqn      map[string][]*Symbol // fully-qualified name -> symbols
}

// NewIndex creates an empty index.
func NewIndex() *Index {
	return &Index{
		docRoots: make(map[string]*Scope),
		fqn:      make(map[string][]*Symbol),
	}
}

// AddDocument builds the scope tree for root and records its symbols under
// their fully-qualified names. Re-adding the same document name replaces its
// previous root scope but leaves stale global entries; callers rebuild the
// whole index on reparse (per-doc incremental invalidation is Plan 5).
func (idx *Index) AddDocument(name string, root *ast.RootNamespace) {
	rs := Build(root)
	idx.docRoots[name] = rs
	idx.indexScope(rs, "")
}

// indexScope walks a scope, recording each distinct symbol under its FQN and
// recursing into child scopes. prefix is the FQN of the owning scope ("" at
// the document root).
func (idx *Index) indexScope(scope *Scope, prefix string) {
	seen := make(map[*Symbol]bool)
	for _, syms := range scope.members {
		for _, sym := range syms {
			if seen[sym] {
				continue // symbol registered under both short and primary key
			}
			seen[sym] = true
			fqn := joinFQN(prefix, sym.Name)
			idx.fqn[fqn] = append(idx.fqn[fqn], sym)
			if sym.Scope != nil {
				idx.indexScope(sym.Scope, fqn)
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

// LookupQualified returns all symbols registered under the exact
// fully-qualified name.
func (idx *Index) LookupQualified(fqn string) []*Symbol {
	return idx.fqn[fqn]
}

// DocumentRoot returns the root scope for the named document, or nil.
func (idx *Index) DocumentRoot(name string) *Scope {
	return idx.docRoots[name]
}
