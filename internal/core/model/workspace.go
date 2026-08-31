package model

import (
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/conformance"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Workspace is the single source of truth for a server/REPL session: the
// document set plus the global symbol index. Mutations are serialized under a
// write lock; reads take a read lock.
type Workspace struct {
	mu        sync.RWMutex
	docs      map[string]*Document
	onDisk    map[string][]byte // last-known on-disk bytes, used when a doc is not open
	open      map[string]bool   // names with an authoritative open buffer
	index     *symbols.Index
	diagCache map[string][]passes.Diagnostic
	// analysis is the options every document of this workspace is analyzed under,
	// so one session asks one question of all its files.
	analysis passes.Options
}

// Option configures a workspace at construction.
type Option func(*Workspace)

// WithConformanceMode analyzes this workspace's documents at mode.
func WithConformanceMode(mode conformance.Mode) Option {
	return func(w *Workspace) { w.analysis.Conformance = mode }
}

// NewWorkspace returns a workspace with stdlib pre-loaded into the global index.
// Stdlib files are loaded from embedded sources (or OPENSYSML_LIBRARY_PATH if set).
// Without options it analyzes in the default conformance mode.
func NewWorkspace(opts ...Option) *Workspace {
	return NewWorkspaceWithIndex(libs.NewModelIndex(), opts...)
}

// NewWorkspaceWithIndex returns a workspace over a caller-built index, for a
// consumer whose resource set is not the bundled standard library. The options
// travel with the resource set, so any index is analyzed under the asked mode.
func NewWorkspaceWithIndex(idx *symbols.Index, opts ...Option) *Workspace {
	w := &Workspace{
		docs:      map[string]*Document{},
		onDisk:    map[string][]byte{},
		open:      map[string]bool{},
		index:     idx,
		diagCache: map[string][]passes.Diagnostic{},
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// ConformanceMode reports the strictness this workspace judges notation at.
func (w *Workspace) ConformanceMode() conformance.Mode {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.analysis.Conformance
}

// SetConformanceMode switches the mode for a live session — an LSP client
// changing its setting, a REPL user asking the strict question — and drops the
// cached diagnostics, which answered the other question.
func (w *Workspace) SetConformanceMode(mode conformance.Mode) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.analysis.Conformance == mode {
		return
	}
	w.analysis.Conformance = mode
	w.invalidateLocked()
}

// NewIndexWithStdlib returns an index carrying the standard library for a
// consumer that resolves library names outside a workspace — the REPL's
// runtime, which has to resolve the measurement unit a quantity expression
// names. It shares the one library index every model reads.
func NewIndexWithStdlib() *symbols.Index {
	return libs.NewModelIndex()
}

// Open registers an authoritative open buffer for name and reindexes.
func (w *Workspace) Open(name string, content []byte, version int) {
	w.setOpenBuffer(name, content, version)
}

// setOpenBuffer records an open buffer and reindexes it.
func (w *Workspace) setOpenBuffer(name string, content []byte, version int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.open[name] = true
	w.reindexLocked(name, content, version)
}

// Update replaces the open buffer content for name and reindexes.
func (w *Workspace) Update(name string, content []byte, version int) {
	w.setOpenBuffer(name, content, version)
}

// SetOnDisk records the bytes a file holds on disk. If the document is not open,
// it becomes the active content and the document is reindexed.
func (w *Workspace) SetOnDisk(name string, content []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onDisk[name] = content
	if !w.open[name] {
		w.reindexLocked(name, content, 0)
	}
}

// DeleteOnDisk forgets the on-disk bytes recorded for name. An open document
// keeps its authoritative buffer; a closed one leaves the document set.
func (w *Workspace) DeleteOnDisk(name string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.onDisk, name)
	if !w.open[name] {
		w.removeLocked(name)
	}
}

// IsOpen reports whether name has an authoritative open buffer.
func (w *Workspace) IsOpen(name string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.open[name]
}

// OpenNames returns a snapshot of the names with an open buffer.
func (w *Workspace) OpenNames() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	names := make([]string, 0, len(w.open))
	for name := range w.open {
		names = append(names, name)
	}
	return names
}

// Close drops the open buffer for name; the document reverts to on-disk content
// if any, otherwise it is removed.
func (w *Workspace) Close(name string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.open, name)
	if disk, ok := w.onDisk[name]; ok {
		w.reindexLocked(name, disk, 0)
		return
	}
	w.removeLocked(name)
}

// Remove deletes the document entirely (open buffer, on-disk cache, and index).
func (w *Workspace) Remove(name string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.open, name)
	delete(w.onDisk, name)
	w.removeLocked(name)
}

// reindexLocked reparses name and incrementally updates the global index.
// Caller must hold the write lock.
func (w *Workspace) reindexLocked(name string, content []byte, version int) {
	doc := newDocument(name, content, version)
	w.docs[name] = doc
	w.index.AddDocument(name, doc.AST) // AddDocument removes stale entries first
	w.index.ExpandWildcardImports()    // Expand new document's wildcard imports
	w.invalidateLocked()
}

// removeLocked drops name from the document set and index. Caller holds the lock.
func (w *Workspace) removeLocked(name string) {
	delete(w.docs, name)
	w.index.RemoveDocument(name)
	w.invalidateLocked()
}

// invalidateLocked clears all cached diagnostics. Caller holds the write lock.
func (w *Workspace) invalidateLocked() {
	// Conservative: any change clears all cached diagnostics. Correctness first;
	// fine-grained cross-document dependency tracking is a later optimization.
	w.diagCache = map[string][]passes.Diagnostic{}
}

// Diagnostics returns the analysis diagnostics for name, computing them lazily
// (and caching) on first request after a change. Returns nil for unknown docs.
func (w *Workspace) Diagnostics(name string) []passes.Diagnostic {
	w.mu.Lock()
	defer w.mu.Unlock()
	doc := w.docs[name]
	if doc == nil {
		return nil
	}
	return w.diagnosticsLocked(name, doc)
}

// AnalyzedContent returns a document's diagnostics together with the content
// they were computed against, so an edit cannot split the two. Reports whether
// the document exists.
func (w *Workspace) AnalyzedContent(name string) ([]byte, []passes.Diagnostic, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	doc := w.docs[name]
	if doc == nil {
		return nil, nil, false
	}
	return doc.Content, w.diagnosticsLocked(name, doc), true
}

// diagnosticsLocked analyzes doc, caching the result. Caller holds the lock.
func (w *Workspace) diagnosticsLocked(name string, doc *Document) []passes.Diagnostic {
	if cached, ok := w.diagCache[name]; ok {
		return cached
	}
	parseDiags := make([]passes.Diagnostic, 0, len(doc.ParseDiagnostics)+len(doc.ParseWarnings))
	for _, pd := range doc.ParseDiagnostics {
		parseDiags = append(parseDiags, passes.Diagnostic{
			Severity: passes.SeverityError,
			Span:     pd.Span,
			Message:  pd.Message,
			Code:     "syntax",
			Source:   "syntax",
			Fixes:    pd.Fixes,
		})
	}
	for _, pw := range doc.ParseWarnings {
		parseDiags = append(parseDiags, passes.Diagnostic{
			Severity: passes.SeverityWarning,
			Span:     pw.Span,
			Message:  pw.Message,
			Code:     pw.Code,
			Source:   "syntax",
			Fixes:    pw.Fixes,
		})
	}
	diags := passes.AnalyzeWithOptions(name, source.KindOf(name), doc.AST, parseDiags, w.index, w.analysis)
	w.diagCache[name] = diags
	return diags
}

// LookupQualified resolves a fully-qualified name against the global index under
// the read lock and returns a copy of the matching symbols, so callers never
// touch the shared index concurrently with a reindex. This is the safe read path
// for consumers (LSP/REPL); the raw index is intentionally not exposed.
func (w *Workspace) LookupQualified(fqn string) []*symbols.Symbol {
	w.mu.RLock()
	defer w.mu.RUnlock()
	syms := w.index.LookupQualified(fqn)
	if len(syms) == 0 {
		return nil
	}
	out := make([]*symbols.Symbol, len(syms))
	copy(out, syms)
	return out
}

// TopLevelSymbols returns the symbols declared at the root of the index as seen
// from the document named doc: the standard library's top-level packages and
// every document's top-level declarations. This is the read path for completion,
// which offers library names that no open document declares.
func (w *Workspace) TopLevelSymbols(doc string) []*symbols.Symbol {
	w.mu.RLock()
	defer w.mu.RUnlock()
	resolver, _ := w.newResolver()
	return resolver.AdmittedTopLevel(doc, w.index.TopLevelBindings(doc))
}

// MembersOnPath returns the members visible on the element that path names from
// scope — the members of a usage's type included, since typing is a
// generalization edge — so that completion after `v.` offers what `v` has.
// Segments after the first are looked up as members of the previous one; an
// unresolved segment yields no members.
func (w *Workspace) MembersOnPath(scope *symbols.Scope, path []string) []*symbols.Symbol {
	if scope == nil || len(path) == 0 {
		return nil
	}
	w.mu.RLock()
	defer w.mu.RUnlock()

	resolver, sem := w.newResolver()

	sym, ok := resolver.ResolveName(scope, path[0], nil)
	if !ok || sym == nil {
		return nil
	}
	for _, seg := range path[1:] {
		if sym, ok = sem.LookupMember(sym, seg); !ok || sym == nil {
			return nil
		}
	}
	if target, ok := resolver.ResolveAliasTarget(sym); ok {
		sym = target
	}
	return w.memberSymbolsLocked(resolver, sem, scope, sym)
}

// memberSymbolsLocked returns the members visible on sym as seen from scope.
// Callers hold the read lock.
func (w *Workspace) memberSymbolsLocked(resolver *resolve.Resolver, sem *semantics.Model,
	scope *symbols.Scope, sym *symbols.Symbol) []*symbols.Symbol {
	members := sem.MembersOf(sym)
	// A cached library symbol has no scope, and a package's own scope does not
	// hold what its imports brought in; both are reachable through the index,
	// as seen from the namespace the completion is requested in so that another
	// namespace's private imports stay hidden. An element the namespace's
	// filters reject is no member of it, so it is not offered either.
	if fqn := w.index.GetFQN(sym); fqn != "" {
		from := resolver.ReferringNamespaceFQN(scope)
		children := w.index.LookupDirectChildrenFrom(fqn, from)
		members = append(members, resolver.AdmittedChildrenOf(scope, fqn, children)...)
	}
	return members
}

// newResolver is a resolver over the index with a semantic model attached: an
// inherited member and the element filters gating an import are both answered by
// the model, so a read path without one resolves differently to a checked one.
func (w *Workspace) newResolver() (*resolve.Resolver, *semantics.Model) {
	resolver := resolve.New(w.index)
	sem := semantics.NewModel(resolver)
	resolver.SetModel(sem)
	return resolver, sem
}

// Document returns the current parsed document for name, or nil.
func (w *Workspace) Document(name string) *Document {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.docs[name]
}

// DocumentNames returns a snapshot of the names of all known documents.
func (w *Workspace) DocumentNames() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	names := make([]string, 0, len(w.docs))
	for name := range w.docs {
		names = append(names, name)
	}
	return names
}

// ResolveQualifiedInDoc resolves a qualified name against the given scope using
// the workspace's symbol index. Used by the LSP layer for go-to-definition.
func (w *Workspace) ResolveQualifiedInDoc(name string, scope *symbols.Scope, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	return w.ResolveReferenceInDoc(name, resolve.Reference{Scope: scope, QN: qn})
}

// ResolveReferenceInDoc resolves one name occurrence, which may be the target of
// a reference subsetting or a feature chain's member segment (see
// resolve.Reference).
func (w *Workspace) ResolveReferenceInDoc(name string, ref resolve.Reference) (*symbols.Symbol, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	resolver, _ := w.newResolver()
	return resolver.ResolveReference(ref)
}

// ResolveQualifiedSegmentsInDoc resolves a qualified name and returns the symbol
// each of its segments denotes, so `A::B::C` yields the symbols for A, B and C.
// Entries are nil where a segment did not resolve. Used by rename, which must
// edit a name wherever it appears, qualifier positions included.
func (w *Workspace) ResolveQualifiedSegmentsInDoc(name string, scope *symbols.Scope, qn *ast.QualifiedName) []*symbols.Symbol {
	return w.ResolveReferenceSegmentsInDoc(name, resolve.Reference{Scope: scope, QN: qn})
}

// ResolveReferenceSegmentsInDoc is ResolveQualifiedSegmentsInDoc for one name
// occurrence (see ResolveReferenceInDoc).
func (w *Workspace) ResolveReferenceSegmentsInDoc(name string, ref resolve.Reference) []*symbols.Symbol {
	if ref.QN == nil {
		return nil
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	r, _ := w.newResolver()
	r.ResolveReference(ref)
	out := make([]*symbols.Symbol, len(ref.QN.Parts))
	for i := range ref.QN.Parts {
		if sym, ok := r.PartSymbol(ref.QN, i); ok {
			out[i] = sym
		}
	}
	return out
}

// ResolveReferenceNameSegmentsInDoc is ResolveReferenceSegmentsInDoc reporting
// what each segment's name *is* rather than the element it reaches, so a segment
// written as an alias name is the alias. Rename edits names, not elements.
func (w *Workspace) ResolveReferenceNameSegmentsInDoc(name string, ref resolve.Reference) []*symbols.Symbol {
	if ref.QN == nil {
		return nil
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	r, _ := w.newResolver()
	r.ResolveReference(ref)
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
	return out
}
