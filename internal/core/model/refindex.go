package model

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ReferenceLocation is one written name segment in a workspace document, with
// the document text its span addresses (the bytes the index was built from).
type ReferenceLocation struct {
	Doc     string
	Content []byte
	Span    source.Span
}

// refEntry is a segment listed under an element: one it reaches, one whose name it
// writes (an alias's own name), or both when the name is the element's own.
type refEntry struct {
	ReferenceLocation
	reached bool
	named   bool
}

// refIndex maps every element (by symbols.KeyOf) to the segments in the
// workspace's documents — never the library's — that reach it or write its name.
type refIndex struct {
	entries map[symbols.ElementKey][]refEntry
}

// add records one segment that reaches element and writes name (either may be nil).
func (x *refIndex) add(doc *Document, span source.Span, element, name *symbols.Symbol) {
	loc := ReferenceLocation{Doc: doc.Name, Content: doc.Content, Span: span}
	switch {
	case element == nil && name == nil:
		return
	case symbols.SameElement(element, name):
		key := symbols.KeyOf(element)
		x.entries[key] = append(x.entries[key], refEntry{ReferenceLocation: loc, reached: true, named: true})
		return
	}
	if element != nil {
		key := symbols.KeyOf(element)
		x.entries[key] = append(x.entries[key], refEntry{ReferenceLocation: loc, reached: true})
	}
	if name != nil {
		key := symbols.KeyOf(name)
		x.entries[key] = append(x.entries[key], refEntry{ReferenceLocation: loc, named: true})
	}
}

// referenceIndexLocked returns the reverse reference index, building it over every
// document with one resolver when a change has dropped it. Caller holds the write lock.
func (w *Workspace) referenceIndexLocked() *refIndex {
	if w.refs != nil {
		return w.refs
	}
	idx := &refIndex{entries: map[symbols.ElementKey][]refEntry{}}
	r, sem := w.newResolver()
	names := make([]string, 0, len(w.docs))
	for name := range w.docs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		doc := w.docs[name]
		if doc.Scope == nil {
			continue
		}
		for _, ref := range resolve.References(doc.AST, doc.Scope) {
			if ref.QN == nil || len(ref.QN.Parts) == 0 {
				continue
			}
			r.ResolveReference(ref)
			sel := invocationSelection(r, sem, ref)
			elements := segmentElements(r, ref, sel)
			written := segmentNames(r, ref, sel)
			for i, part := range ref.QN.Parts {
				idx.add(doc, part.Span, elements[i], written[i])
			}
		}
	}
	w.refs = idx
	return idx
}

// ReferencesTo returns every segment in the workspace's documents that reaches
// target or writes its name (an alias use counts for both), in document then position order.
func (w *Workspace) ReferencesTo(target *symbols.Symbol) []ReferenceLocation {
	return w.referenceLocations(target, func(refEntry) bool { return true })
}

// NameReferencesTo returns every segment in the workspace's documents that writes
// target's own name — what a rename edits; an alias use is edited via the alias.
func (w *Workspace) NameReferencesTo(target *symbols.Symbol) []ReferenceLocation {
	return w.referenceLocations(target, func(e refEntry) bool { return e.named })
}

// referenceLocations answers a reverse-index query for target, building the index
// first when it is stale; the answer is a copy the caller owns.
func (w *Workspace) referenceLocations(target *symbols.Symbol, keep func(refEntry) bool) []ReferenceLocation {
	if target == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	entries := w.referenceIndexLocked().entries[symbols.KeyOf(target)]
	out := make([]ReferenceLocation, 0, len(entries))
	for _, e := range entries {
		if keep(e) {
			out = append(out, e.ReferenceLocation)
		}
	}
	return out
}

// segmentElements is the element each segment of a resolved ref reaches (nil where
// unresolved); the last is the overload a call selects, nothing when several tie.
func segmentElements(r *resolve.Resolver, ref resolve.Reference, sel *semantics.InvocationSelection) []*symbols.Symbol {
	out := make([]*symbols.Symbol, len(ref.QN.Parts))
	for i := range ref.QN.Parts {
		if sym, ok := r.PartSymbol(ref.QN, i); ok {
			out[i] = sym
		}
	}
	last := len(out) - 1
	out[last] = calledDeclaration(sel, out[last])
	return out
}

// segmentNames is what each segment of a resolved ref writes rather than reaches,
// so a segment written as an alias name is the alias.
func segmentNames(r *resolve.Resolver, ref resolve.Reference, sel *semantics.InvocationSelection) []*symbols.Symbol {
	out := make([]*symbols.Symbol, len(ref.QN.Parts))
	for i := range ref.QN.Parts {
		if sym, ok := r.PartAlias(ref.QN, i); ok {
			out[i] = sym
			continue
		}
		if sym, ok := r.PartSymbol(ref.QN, i); ok {
			out[i] = sym
		}
	}
	// A written alias stays the name — the alias naming the overload the call selects,
	// when same-named aliases name several — unless the call is tied: then none is.
	last := len(out) - 1
	if _, aliased := r.PartAlias(ref.QN, last); !aliased || (sel != nil && sel.Ambiguous) {
		out[last] = calledDeclaration(sel, out[last])
	} else if element, _ := r.PartSymbol(ref.QN, last); sel != nil && sel.Selected != nil && element != sel.Selected {
		out[last] = selectedName(r, ref, sel.Selected)
	}
	return out
}
