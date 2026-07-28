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
}

// NewIndex creates an empty index.
func NewIndex() *Index {
	return &Index{
		docRoots:      make(map[string]*Scope),
		fqn:           make(map[string][]*Symbol),
		contributions: make(map[string][]fqnEntry),
	}
}

// AddDocument builds the scope tree for root and records its symbols under
// their fully-qualified names. Re-adding the same document name first removes
// the document's previous contributions, so the index stays exact.
func (idx *Index) AddDocument(name string, root *ast.RootNamespace) {
	idx.RemoveDocument(name)
	rs := Build(root)
	idx.docRoots[name] = rs
	idx.indexScope(name, rs, "")
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
			fqn := joinFQN(prefix, sym.Name)
			idx.fqn[fqn] = append(idx.fqn[fqn], sym)
			idx.contributions[doc] = append(idx.contributions[doc], fqnEntry{fqn: fqn, sym: sym})
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

// LookupQualified returns all symbols registered under the exact
// fully-qualified name.
func (idx *Index) LookupQualified(fqn string) []*Symbol {
	return idx.fqn[fqn]
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
