package model

import (
	"sync"

	"github.com/Open-MBEE/Systemica/internal/core/passes"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Workspace is the single source of truth for a server/REPL session: the
// document set plus the global symbol index. Mutations are serialized under a
// write lock (and, in Task 5, through a single owner goroutine); reads take a
// read lock.
type Workspace struct {
	mu        sync.RWMutex
	docs      map[string]*Document
	onDisk    map[string][]byte // last-known on-disk bytes, used when a doc is not open
	open      map[string]bool   // names with an authoritative open buffer
	index     *symbols.Index
	diagCache map[string][]passes.Diagnostic
}

// NewWorkspace returns an empty workspace with a fresh index.
func NewWorkspace() *Workspace {
	return &Workspace{
		docs:      map[string]*Document{},
		onDisk:    map[string][]byte{},
		open:      map[string]bool{},
		index:     symbols.NewIndex(),
		diagCache: map[string][]passes.Diagnostic{},
	}
}

// Open registers an authoritative open buffer for name and reindexes.
func (w *Workspace) Open(name string, content []byte, version int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.open[name] = true
	w.reindexLocked(name, content, version)
}

// Update replaces the open buffer content for name and reindexes.
func (w *Workspace) Update(name string, content []byte, version int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.open[name] = true
	w.reindexLocked(name, content, version)
}

// SetOnDisk records on-disk bytes (from fsnotify). If the document is not open,
// it becomes the active content and the document is reindexed.
func (w *Workspace) SetOnDisk(name string, content []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onDisk[name] = content
	if !w.open[name] {
		w.reindexLocked(name, content, 0)
	}
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
	if cached, ok := w.diagCache[name]; ok {
		return cached
	}
	doc := w.docs[name]
	if doc == nil {
		return nil
	}
	parseDiags := make([]passes.Diagnostic, len(doc.ParseDiagnostics))
	for i, pd := range doc.ParseDiagnostics {
		parseDiags[i] = passes.Diagnostic{
			Severity: passes.SeverityError,
			Span:     pd.Span,
			Message:  pd.Message,
			Code:     "syntax",
			Source:   "syntax",
		}
	}
	diags := passes.Analyze(name, doc.AST, parseDiags, w.index)
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

// Document returns the current parsed document for name, or nil.
func (w *Workspace) Document(name string) *Document {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.docs[name]
}
