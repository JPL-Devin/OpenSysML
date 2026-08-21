package symbols

import "github.com/Open-MBEE/OpenSysML/internal/core/source"

// RecordEntry is a minimal, AST-less description of a symbol, used to populate
// the index from a persisted cache record instead of a parsed document.
type RecordEntry struct {
	FQN             string
	ShortName       string // short name (e.g., "kg" for "kilogram"), empty if none
	Kind            SymbolKind
	Span            source.Span
	Supers          []string         // FQNs of the specialization targets of a def/usage
	FeaturedBy      []string         // FQNs of the `featured by` targets of a def/usage
	WildcardImports []WildcardImport // for packages: its `import X::*` declarations
	AliasTarget     string           // for aliases: raw target text of "alias X for Y"
	Unit            *UnitFacts       // for measurement units: their reduction to base units
	Dimension       *DimensionFacts  // for measurement units: the quantity dimension they measure in
	Behavior        *BehaviorFacts   // for behaviors/steps: their parameter list in order

	// Annotations are the metadata annotating the symbol, which an element
	// filter classifies it by and its absent declaration would have stated.
	Annotations []AnnotationFacts

	// NamespaceFilters are the conditions of the namespace's `filter` members,
	// compiled: they restrict what it re-exports, and the declaration they were
	// written in is gone.
	NamespaceFilters []*FilterPredicate
}

// AddRecords registers synthetic, AST-less symbols for a document directly by
// their fully-qualified names. It first removes any prior contributions for
// name (idempotent re-add), mirroring AddDocument — the wildcard imports a
// record carries are expanded by a later ExpandWildcardImports, as a parsed
// document's are. Symbols added this way carry no Decl/Scope and are keyed only
// by FQN, which is sufficient for qualified-name resolution against library
// content restored from cache.
func (idx *Index) AddRecords(name string, entries []RecordEntry) {
	idx.RemoveDocument(name)
	for _, e := range entries {
		sym := &Symbol{
			Name:           e.FQN,
			ShortName:      e.ShortName,
			Kind:           e.Kind,
			DeclSpan:       e.Span,
			SuperFQNs:      e.Supers,
			FeaturedByFQNs: e.FeaturedBy,
			AliasTargetFQN: e.AliasTarget,
			Unit:           e.Unit,
			Dimension:      e.Dimension,
			Behavior:       e.Behavior,
			Annotations:    e.Annotations,
		}
		idx.register(e.FQN, sym)
		idx.declaredAt[sym] = e.FQN
		idx.contributions[name] = append(idx.contributions[name], fqnEntry{fqn: e.FQN, sym: sym})

		// Also index under short name FQN if different
		if e.ShortName != "" && e.ShortName != shortLeafName(e.FQN) {
			shortFQN := replaceLeafName(e.FQN, e.ShortName)
			idx.register(shortFQN, sym)
			idx.contributions[name] = append(idx.contributions[name], fqnEntry{fqn: shortFQN, sym: sym})
		}

		// Store wildcard import metadata
		if len(e.WildcardImports) > 0 {
			idx.setWildcardImports(e.FQN, name, e.WildcardImports)
		}
		idx.SetNamespaceFilters(e.FQN, name, filtersFromPredicates(e.NamespaceFilters))
	}
}

// shortLeafName extracts the leaf name from FQN ("A::B::C" → "C").
func shortLeafName(fqn string) string {
	lastIdx := -1
	for i := len(fqn) - 1; i >= 1; i-- {
		if fqn[i-1:i+1] == "::" {
			lastIdx = i + 1
			break
		}
	}
	if lastIdx < 0 {
		return fqn
	}
	return fqn[lastIdx:]
}

// replaceLeafName replaces the leaf name in FQN ("A::B::C", "D" → "A::B::D").
func replaceLeafName(fqn, newLeaf string) string {
	lastIdx := -1
	for i := len(fqn) - 1; i >= 1; i-- {
		if fqn[i-1:i+1] == "::" {
			lastIdx = i + 1
			break
		}
	}
	if lastIdx < 0 {
		return newLeaf
	}
	return fqn[:lastIdx] + newLeaf
}
