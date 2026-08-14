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
// the persisted shape changes, or the resolution a record captures changes; a
// mismatch invalidates all cached records.
const formatVersion = 13

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
	Unit            *unitFacts       // for measurement units: their reduction to base units

	// Annotations is the metadata annotating the symbol. An element filter
	// classifies a candidate by what annotates it, which a restored symbol has no
	// declaration left to state, so it is recorded here.
	Annotations []symbols.AnnotationFacts

	// NamespaceFilters holds the conditions of the namespace's `filter` members,
	// compiled. A compiled condition names every element it tests by
	// fully-qualified name, so it needs neither the expression it was parsed from
	// nor the scope that expression's names meant something in — which is what
	// lets a restored library filter what it re-exports the way a parsed one does.
	NamespaceFilters []*symbols.FilterPredicate
}

// unitFacts is the gob-encodable projection of a measurement unit reduced to
// base units. A restored symbol carries no declaration, so the conversion the
// unit declares can only be read back from here.
type unitFacts struct {
	ScaleNum    float64
	ScaleDen    float64
	Factors     []unitFactor
	Irreducible bool // a unit whose reduction its declaration does not yield
}

// unitFactor is one base unit of a reduced measurement unit and its exponent.
type unitFactor struct {
	FQN      string
	Exponent float64
}

// wildcardImport is the gob-encodable projection of symbols.WildcardImport.
type wildcardImport struct {
	Target  string
	Private bool
	// Filter is the condition of the import's `[...]` clause, compiled, or nil
	// for an unfiltered import.
	Filter *symbols.FilterPredicate
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
	complete := collectScope(root, "", rec, semantics.NewModel(r), idx, r)
	return rec, complete
}

// collectScope walks scope's members (and child scopes) appending reduced
// records. prefix is the fully-qualified name of scope's owner ("" at root).
// Reports whether every specialization target it met resolved.
func collectScope(scope *symbols.Scope, prefix string, rec *IndexRecord, model *semantics.Model, idx *symbols.Index, r *resolve.Resolver) bool {
	complete := true
	for _, sym := range scope.Members() {
		fqn := sym.Name
		if prefix != "" {
			fqn = prefix + "::" + sym.Name
		}
		supers, resolved := supersOf(sym, idx, r)
		complete = complete && resolved
		rec.Symbols = append(rec.Symbols, symRecord{
			FQN:              fqn,
			ShortName:        shortNameOf(sym.Decl),
			Kind:             sym.Kind,
			Span:             sym.DeclSpan,
			Supers:           supers,
			WildcardImports:  wildcardImportsOf(sym.Decl, sym.Scope, model),
			AliasTarget:      aliasTargetOf(sym.Decl),
			Unit:             unitFactsOf(sym, model, idx),
			Annotations:      model.AnnotationFactsOf(sym),
			NamespaceFilters: namespaceFiltersOf(sym, model),
		})
		if sym.Scope != nil {
			complete = collectScope(sym.Scope, fqn, rec, model, idx, r) && complete
		}
	}
	return complete
}

// unitFactsOf reduces a measurement unit to base units, so the reduction
// survives caching: a restored symbol has no declaration, and the conversion or
// unit expression the reduction was computed from would be lost with it. A unit
// whose reduction its declaration does not yield is recorded as irreducible,
// which is not the same as not being a unit.
func unitFactsOf(sym *symbols.Symbol, model *semantics.Model, idx *symbols.Index) *unitFacts {
	if !model.IsMeasurementUnit(sym) {
		return nil
	}
	term, err := model.UnitTermOf(sym)
	if err != nil {
		return &unitFacts{Irreducible: true}
	}
	facts := &unitFacts{ScaleNum: term.Scale.Num, ScaleDen: term.Scale.Den}
	for _, f := range term.Factors {
		baseFQN := idx.GetFQN(f.Unit)
		if baseFQN == "" {
			return &unitFacts{Irreducible: true}
		}
		facts.Factors = append(facts.Factors, unitFactor{FQN: baseFQN, Exponent: f.Exponent})
	}
	return facts
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
// Namespace: target text, visibility, and the compiled condition of a filter
// clause, whose names are resolved against scope while the declaration is still
// there to resolve them in. Returns nil for non-namespace nodes.
func wildcardImportsOf(decl ast.Node, scope *symbols.Scope, model *semantics.Model) []wildcardImport {
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
		entry := wildcardImport{
			Target:  qualifiedNameText(imp.Imported),
			Private: imp.Visibility == ast.VisibilityPrivate,
		}
		if imp.FilterExpr != nil {
			entry.Filter = model.CompileElementFilter(symbols.ElementFilter{
				Expr:  imp.FilterExpr,
				Scope: scope,
				Span:  imp.FilterExpr.Span(),
			})
		}
		out = append(out, entry)
	}
	return out
}

// namespaceFiltersOf compiles the conditions of the `filter` members the symbol's
// namespace declares, so that they restrict what a restored library re-exports.
func namespaceFiltersOf(sym *symbols.Symbol, model *semantics.Model) []*symbols.FilterPredicate {
	var out []*symbols.FilterPredicate
	for _, f := range symbols.NamespaceFiltersIn(sym.Scope) {
		if pred := model.CompileElementFilter(f); pred != nil {
			out = append(out, pred)
		}
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
