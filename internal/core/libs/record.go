package libs

import (
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// formatVersion is the on-disk index record format version. Bump it whenever
// the persisted shape changes; a mismatch invalidates all cached records.
const formatVersion = 1

// symRecord is the reduced, gob-encodable projection of a symbols.Symbol.
// It deliberately excludes the AST-backed Decl and the Scope/OwnerScope
// pointers, persisting only the fields the resolver needs to answer
// qualified-name lookups.
type symRecord struct {
	FQN    string
	Kind   symbols.SymbolKind
	Span   source.Span
	Supers []string // specialization-edge placeholder; empty until def/usage grammar lands
}

// IndexRecord is the serializable snapshot of one library document's symbols.
type IndexRecord struct {
	Name    string
	Symbols []symRecord
}

// recordFromIndex extracts a reduced, serializable record of every symbol the
// named document contributed to idx. Returns nil if the document is unknown.
func recordFromIndex(name string, idx *symbols.Index) *IndexRecord {
	root := idx.DocumentRoot(name)
	if root == nil {
		return nil
	}
	rec := &IndexRecord{Name: name}
	collectScope(root, "", rec)
	return rec
}

// collectScope walks scope's members (and child scopes) appending reduced
// records. prefix is the fully-qualified name of scope's owner ("" at root).
func collectScope(scope *symbols.Scope, prefix string, rec *IndexRecord) {
	for _, sym := range scope.Members() {
		fqn := sym.Name
		if prefix != "" {
			fqn = prefix + "::" + sym.Name
		}
		rec.Symbols = append(rec.Symbols, symRecord{
			FQN:  fqn,
			Kind: sym.Kind,
			Span: sym.DeclSpan,
		})
		if sym.Scope != nil {
			collectScope(sym.Scope, fqn, rec)
		}
	}
}
