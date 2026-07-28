package model

import (
	"sync"

	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Workspace is the single source of truth for a server/REPL session: the
// document set plus the global symbol index. Mutations are serialized under a
// write lock (and, in Task 5, through a single owner goroutine); reads take a
// read lock.
type Workspace struct {
	mu       sync.RWMutex
	docs     map[string]*Document
	onDisk   map[string][]byte // last-known on-disk bytes, used when a doc is not open
	open     map[string]bool   // names with an authoritative open buffer
	index    *symbols.Index
	diagChan map[string]bool // reserved for Task 4 cache; unused here
}

// NewWorkspace returns an empty workspace with a fresh index.
func NewWorkspace() *Workspace {
	return &Workspace{
		docs:     map[string]*Document{},
		onDisk:   map[string][]byte{},
		open:     map[string]bool{},
		index:    symbols.NewIndex(),
		diagChan: map[string]bool{},
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

// invalidateLocked is the cache-invalidation hook; filled in Task 4.
func (w *Workspace) invalidateLocked() {}

// Index returns the global symbol index. Callers must not mutate it directly.
func (w *Workspace) Index() *symbols.Index {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.index
}

// Document returns the current parsed document for name, or nil.
func (w *Workspace) Document(name string) *Document {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.docs[name]
}
