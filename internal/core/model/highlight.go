package model

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/highlight"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// HighlightTokens returns the semantic tokens of a document, ordered by source
// position. Returns nil for unknown documents.
func (w *Workspace) HighlightTokens(name string) []highlight.Token {
	doc := w.Document(name)
	if doc == nil {
		return nil
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	resolver, _ := w.newResolver()
	return highlight.Tokens(doc.Content, doc.AST, doc.Scope, resolution{r: resolver})
}

// resolution answers highlighting queries from one resolver, so the memoized
// resolution of a name is shared across the document's references.
type resolution struct{ r *resolve.Resolver }

func (res resolution) SegmentSymbols(ref resolve.Reference) []*symbols.Symbol {
	if ref.QN == nil {
		return nil
	}
	res.r.ResolveReference(ref)
	out := make([]*symbols.Symbol, len(ref.QN.Parts))
	for i := range ref.QN.Parts {
		if sym, ok := res.r.PartSymbol(ref.QN, i); ok {
			out[i] = sym
		}
	}
	return out
}
