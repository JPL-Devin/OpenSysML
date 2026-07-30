package libs

import (
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
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
	Supers []string // raw target text of specializes/subsets/redefines edges
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
			FQN:    fqn,
			Kind:   sym.Kind,
			Span:   sym.DeclSpan,
			Supers: supersOf(sym.Decl),
		})
		if sym.Scope != nil {
			collectScope(sym.Scope, fqn, rec)
		}
	}
}

// supersOf extracts the raw qualified-name text of the specialization edges
// (specializes/subsets/redefines) declared by a Definition or Usage. Typing,
// references, and crosses edges are not specializations and are excluded.
// Returns nil for any other node kind.
func supersOf(decl ast.Node) []string {
	var rels []*ast.Relationship
	switch d := decl.(type) {
	case *ast.Definition:
		rels = d.Relationships
	case *ast.Usage:
		rels = d.Relationships
	default:
		return nil
	}
	var out []string
	for _, r := range rels {
		switch r.Kind {
		case ast.RelSpecializes, ast.RelSubsets, ast.RelRedefines:
			out = append(out, qualifiedNameText(r.Target))
		}
	}
	return out
}

// qualifiedNameText renders a QualifiedName as "A::B::C" (no leading $:: marker;
// specialization targets are relative names). Returns "" for a nil name.
func qualifiedNameText(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	var b strings.Builder
	for i, seg := range qn.Parts {
		if i > 0 {
			b.WriteString("::")
		}
		b.WriteString(seg.Text)
	}
	return b.String()
}
