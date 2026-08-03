package symbols

import "github.com/Open-MBEE/Systemica/internal/core/source"

// RecordEntry is a minimal, AST-less description of a symbol, used to populate
// the index from a persisted cache record instead of a parsed document.
type RecordEntry struct {
	FQN             string
	Kind            SymbolKind
	Span            source.Span
	WildcardImports []string // for packages: FQNs of wildcard-imported targets
	AliasTarget     string   // for aliases: raw target text of "alias X for Y"
}

// AddRecords registers synthetic, AST-less symbols for a document directly by
// their fully-qualified names. It first removes any prior contributions for
// name (idempotent re-add), mirroring AddDocument. Symbols added this way carry
// no Decl/Scope and are keyed only by FQN, which is sufficient for
// qualified-name resolution against library content restored from cache.
func (idx *Index) AddRecords(name string, entries []RecordEntry) {
	idx.RemoveDocument(name)
	for _, e := range entries {
		sym := &Symbol{
			Name:           e.FQN,
			Kind:           e.Kind,
			DeclSpan:       e.Span,
			AliasTargetFQN: e.AliasTarget,
		}
		idx.fqn[e.FQN] = append(idx.fqn[e.FQN], sym)
		idx.contributions[name] = append(idx.contributions[name], fqnEntry{fqn: e.FQN, sym: sym})
		// Store wildcard import metadata
		if len(e.WildcardImports) > 0 {
			idx.wildcardMeta[e.FQN] = e.WildcardImports
		}
	}
}
