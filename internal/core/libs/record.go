package libs

import (
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// formatVersion is the on-disk index record format version. Bump it whenever
// the persisted shape changes; a mismatch invalidates all cached records.
const formatVersion = 8

// symRecord is the reduced, gob-encodable projection of a symbols.Symbol.
// It deliberately excludes the AST-backed Decl and the Scope/OwnerScope
// pointers, persisting only the fields the resolver needs to answer
// qualified-name lookups.
type symRecord struct {
	FQN             string
	ShortName       string // short name (e.g., "kg" for "kilogram"), empty if none
	Kind            symbols.SymbolKind
	Span            source.Span
	Supers          []string         // FQNs of the generalization targets (see supersOf)
	WildcardImports []wildcardImport // for packages: its `import X::*` declarations
	AliasTarget     string           // for aliases: raw target text of "alias X for Y"
}

// wildcardImport is the gob-encodable projection of symbols.WildcardImport.
type wildcardImport struct {
	Target  string
	Private bool
}

// IndexRecord is the serializable snapshot of one library document's symbols.
type IndexRecord struct {
	Name    string
	Symbols []symRecord
}

// recordFromIndex extracts a reduced, serializable record of every symbol the
// named document contributed to idx, and reports whether every specialization
// target it declares resolved. Returns nil if the document is unknown.
// Targets are resolved through r, so the record holds the fully-qualified name
// of each supertype rather than text that only means something in its own file.
func recordFromIndex(name string, idx *symbols.Index, r *resolve.Resolver) (*IndexRecord, bool) {
	root := idx.DocumentRoot(name)
	if root == nil {
		return nil, false
	}
	rec := &IndexRecord{Name: name}
	complete := collectScope(root, "", rec, idx, r)
	return rec, complete
}

// collectScope walks scope's members (and child scopes) appending reduced
// records. prefix is the fully-qualified name of scope's owner ("" at root).
// Reports whether every specialization target it met resolved.
func collectScope(scope *symbols.Scope, prefix string, rec *IndexRecord, idx *symbols.Index, r *resolve.Resolver) bool {
	complete := true
	for _, sym := range scope.Members() {
		fqn := sym.Name
		if prefix != "" {
			fqn = prefix + "::" + sym.Name
		}
		supers, resolved := supersOf(sym, idx, r)
		complete = complete && resolved
		rec.Symbols = append(rec.Symbols, symRecord{
			FQN:             fqn,
			ShortName:       shortNameOf(sym.Decl),
			Kind:            sym.Kind,
			Span:            sym.DeclSpan,
			Supers:          supers,
			WildcardImports: wildcardImportsOf(sym.Decl),
			AliasTarget:     aliasTargetOf(sym.Decl),
		})
		if sym.Scope != nil {
			complete = collectScope(sym.Scope, fqn, rec, idx, r) && complete
		}
	}
	return complete
}

// supersOf resolves the generalization edges declared by a Definition or Usage
// to the fully-qualified names of their targets, reporting whether every one of
// them resolved. The edge kinds are exactly semantics.GeneralizationKind, since
// a restored record replaces that graph; a feature chain target has no
// qualified name to record and counts as unresolved.
func supersOf(sym *symbols.Symbol, idx *symbols.Index, r *resolve.Resolver) ([]string, bool) {
	var rels []*ast.Relationship
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		rels = d.Relationships
	case *ast.Usage:
		rels = d.Relationships
	default:
		return nil, true
	}
	var out []string
	seen := map[string]bool{}
	complete := true
	for _, rel := range rels {
		if !semantics.GeneralizationKind(rel.Kind) {
			continue
		}
		// Unwrap FeatureReference to get underlying QualifiedName
		target := rel.Target
		if ref, ok := target.(*ast.FeatureReference); ok {
			target = ref.Name
		}
		qn, ok := target.(*ast.QualifiedName)
		if !ok {
			complete = false
			continue
		}
		super, ok := r.ResolveQualified(sym.OwnerScope, qn)
		if !ok || super == nil {
			complete = false
			continue
		}
		if super == sym {
			continue
		}
		superFQN := idx.GetFQN(super)
		if superFQN == "" {
			complete = false
			continue
		}
		if seen[superFQN] {
			continue
		}
		seen[superFQN] = true
		out = append(out, superFQN)
	}
	return out, complete
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

// wildcardImportsOf extracts the `import X::*` declarations of a Package or
// Namespace, target text and visibility. Returns nil for non-namespace nodes.
func wildcardImportsOf(decl ast.Node) []wildcardImport {
	var members []ast.Node
	switch d := decl.(type) {
	case *ast.Package:
		members = d.Members
	case *ast.Namespace:
		members = d.Members
	default:
		return nil
	}

	var out []wildcardImport
	for _, m := range members {
		imp, ok := m.(*ast.Import)
		if !ok || imp.Kind != ast.ImportNamespace || imp.Imported == nil {
			continue
		}
		out = append(out, wildcardImport{
			Target:  qualifiedNameText(imp.Imported),
			Private: imp.Visibility == ast.VisibilityPrivate,
		})
	}
	return out
}

// aliasTargetOf extracts the raw qualified-name text of an alias's target
// ("alias X for Y" → "Y"). Returns "" for non-alias nodes.
func aliasTargetOf(decl ast.Node) string {
	al, ok := decl.(*ast.Alias)
	if !ok || al.For == nil {
		return ""
	}
	return qualifiedNameText(al.For)
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
