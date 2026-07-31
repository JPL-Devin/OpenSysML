# Plan 5a: Workspace & Reindex Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a `model` package with a `Workspace` (single source of truth: document set + global symbol index + diagnostic cache) and a one-path reindex pipeline that incrementally updates the index on document add/update/remove, with single-owner-goroutine concurrency and fsnotify-backed on-disk watching.

**Architecture:** New `internal/core/model` package sits above `parser`, `symbols`, `resolve`, `passes`. A `Document` holds source bytes, version, parsed AST, parse diagnostics, and its per-doc root scope. A `Workspace` owns a `map[name]*Document` and a `*symbols.Index`, and serializes all mutations through a single owner goroutine consuming a channel of change events; reads take a snapshot under a read-lock. Reindex is incremental: `symbols.Index` gains `RemoveDocument` so a reparsed doc's stale FQN entries are dropped before re-adding. Analysis (resolve + passes) is lazy and computed on demand with a fresh resolver per call.

**Tech Stack:** Go 1.25 stdlib + github.com/fsnotify/fsnotify (first external dep). Consumes internal/core/{parser,ast,source,symbols,resolve,passes}.

---

## Scope

Implements spec §9 (Workspace & Reindex Orchestration) ONLY. Deferred to later sub-plans: §10 bundled stdlib + persistent cache (Plan 5b); §11 external dependencies + `sysml.toml`/`sysml.lock` manifest (Plan 5c).

**In scope:**

- `Document`: source bytes, version, parsed AST, parse diagnostics, per-doc root scope.
- `Workspace`: owns document set + global `*symbols.Index` + per-doc diagnostic cache. Single source of truth, one per session.
- Two content sources per document: an open-buffer (authoritative when "open", modeling LSP didOpen/didChange/didClose) OR on-disk bytes (via fsnotify). Open-buffer wins while a document is open.
- One-path reindex pipeline fed by all change sources: reparse changed doc → new AST + syntax diagnostics + fresh scope tree → incrementally update the global index (drop the doc's stale FQN entries, re-add) → invalidate cached diagnostics for that doc (and, conservatively, all docs, since cross-doc dependents may reference it) → recompute analysis lazily on demand, not eagerly.
- Incremental index update: add `symbols.Index.RemoveDocument(name)` so a reparsed doc no longer leaves stale global FQN entries (fixes the limitation documented in `index.go` today).
- Concurrency: all mutations serialized through a single owner goroutine consuming a channel of change events; read queries take a snapshot under an `RWMutex` read-lock. Avoids races between the fsnotify goroutine and caller/LSP threads.
- Debounce: coalesce bursts of change events per document over a short window (e.g. a git checkout touching many files).
- fsnotify watcher: watch workspace root(s) for create/modify/delete of `.sysml`/`.kerml` files, feeding the same reindex pipeline. Open-buffer documents ignore on-disk events until closed.

**Deferred / not in scope:**

- Stdlib bundling, embed.FS, `SYSML_LIBRARY_PATH`, persistent `<hash>.idx` cache (Plan 5b).
- `sysml.toml`/`sysml.lock`, git/local external deps, import resolution order workspace→deps→stdlib (Plan 5c).
- Fine-grained cross-doc dependency tracking (spec §9 step 5 "cross-document dependents via resolution-layer dependency tracking"). v1 here uses a **conservative invalidation**: any document change invalidates the *entire* diagnostic cache. Precise dependency tracking is a later optimization; correctness is preserved because analysis recomputes lazily against the current index.
- Type/constraint passes (Plan 4 roadmap), per-node lazy invocation (Plan 6 LSP).

## File Structure

New package `internal/core/model/`:

- `document.go` — `Document` type (source, version, AST, parse diagnostics, scope) + constructor that parses.
- `workspace.go` — `Workspace` type, document set + index, `Open`/`Update`/`Close`/`SetOnDisk`/`Remove`, incremental reindex, lazy `Diagnostics(name)`, snapshot reads under RWMutex.
- `event.go` — `ChangeEvent` type + `EventKind` enum; the owner-goroutine loop (`Run`/`Post`/`Close`) that serializes mutations.
- `debounce.go` — per-document debounce coalescing helper.
- `watcher.go` — fsnotify-backed watcher translating filesystem events into `ChangeEvent`s, respecting open-buffer precedence.

Modified: `internal/core/symbols/index.go` — add `RemoveDocument(name)` + refactor `indexScope` accounting so removal is exact.

Tests: `*_test.go` per file + `integration_test.go` (scripted open→edit→save→external-modify sequences) + reused fixtures under `testdata/` where helpful.

**Dependency direction:** `model` → `{parser, passes, resolve, symbols, ast, source}` + `fsnotify`. Nothing imports `model` yet (Plan 6 LSP + Plan 7 REPL will).

## Package Reference (delivered APIs consumed)

Disk-verified as of HEAD e752bf3 (Plans 1-4 complete):

- `source.New(name string, content []byte) *source.SourceFile`; `(*SourceFile).Text(source.Span) string`; `(*SourceFile).Lines().PosAt(offset int) source.Pos{Line,Col int}` (1-based). `source.Span{Offset,Len int}` + `End()`.
- `parser.New(sf *source.SourceFile) *parser.Parser`; `(*Parser).ParseFile() *ast.RootNamespace`; `(*Parser).Diagnostics []parser.Diagnostic{Span source.Span; Message string}`.
- `ast.RootNamespace{ ...; Members []ast.Node }`; `ast.Node` interface getters ONLY `Span()/LeadingTrivia()/TrailingTrivia()` (setters on `*NodeBase` only).
- `symbols.NewIndex() *Index`; `(*Index).AddDocument(name string, root *ast.RootNamespace)`; `(*Index).LookupQualified(fqn string) []*Symbol`; `(*Index).DocumentRoot(name string) *Scope`; `symbols.Build(root *ast.RootNamespace) *Scope`. **Current gap:** `AddDocument` re-add leaves stale FQN entries; Task 1 adds `RemoveDocument`.
- `resolve.New(idx *symbols.Index) *resolve.Resolver`; `(*Resolver).ResolveDocument(name string, root *ast.RootNamespace)` APPENDS to exported `Resolver.Diagnostics []resolve.Diagnostic{Span,Message}` (never resets → use a **fresh resolver per analysis call**).
- `passes.Analyze(name string, root *ast.RootNamespace, parseDiags []passes.Diagnostic, idx *symbols.Index) []passes.Diagnostic`; `passes.Diagnostic{Severity Severity; Span source.Span; Message, Code, Source string}` (zero-value `Severity==SeverityError`). Adapt `parser.Diagnostic` → `passes.Diagnostic{Severity:SeverityError, Span, Message, Code:"syntax", Source:"syntax"}` before calling.

**Gotchas carried forward:** zero-value `passes.Severity` is `SeverityError` — always set Severity explicitly for non-error test fixtures; `T{}.Method()` in a statement/if-condition must be parenthesized `(T{}).Method()`; first `go test` run is slow (build) — use `go build ./... ` first then `go test ./internal/core/model/ -run <filter> -count=1 -timeout 120s`.

### Task 1: symbols.Index incremental RemoveDocument

**Files:**
- Modify: `internal/core/symbols/index.go`
- Test: `internal/core/symbols/index_test.go` (append)

Today `Index` cannot remove a document's contributions, so re-adding a reparsed doc leaves stale FQN entries. Track, per document, the exact `(fqn, symbol)` pairs it contributed so removal is precise, then make `AddDocument` remove-before-add (idempotent re-add).

- [ ] **Step 1: Write the failing test**

Append to `internal/core/symbols/index_test.go`:

```go
func TestIndexRemoveDocument(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { namespace N; }")
	if got := idx.LookupQualified("P::N"); len(got) != 1 {
		t.Fatalf("before remove: P::N = %d symbols, want 1", len(got))
	}
	idx.RemoveDocument("a.sysml")
	if got := idx.LookupQualified("P::N"); len(got) != 0 {
		t.Fatalf("after remove: P::N = %d symbols, want 0", len(got))
	}
	if idx.DocumentRoot("a.sysml") != nil {
		t.Fatalf("after remove: DocumentRoot should be nil")
	}
}

func TestIndexReAddReplacesStaleEntries(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { namespace Old; }")
	addDoc(t, idx, "a.sysml", "package P { namespace New; }")
	if got := idx.LookupQualified("P::Old"); len(got) != 0 {
		t.Fatalf("P::Old = %d symbols after re-add, want 0 (stale)", len(got))
	}
	if got := idx.LookupQualified("P::New"); len(got) != 1 {
		t.Fatalf("P::New = %d symbols after re-add, want 1", len(got))
	}
	if got := idx.LookupQualified("P"); len(got) != 1 {
		t.Fatalf("P = %d symbols after re-add, want 1 (not doubled)", len(got))
	}
}

func TestIndexRemoveUnknownDocumentNoop(t *testing.T) {
	idx := NewIndex()
	idx.RemoveDocument("missing.sysml") // must not panic
	addDoc(t, idx, "a.sysml", "package P;")
	idx.RemoveDocument("b.sysml") // unrelated doc untouched
	if got := idx.LookupQualified("P"); len(got) != 1 {
		t.Fatalf("P = %d after removing unrelated doc, want 1", len(got))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/symbols/ -run 'TestIndexRemoveDocument|TestIndexReAdd|TestIndexRemoveUnknown' -v`
Expected: FAIL — `idx.RemoveDocument undefined`.

- [ ] **Step 3: Write minimal implementation**

Rewrite `internal/core/symbols/index.go` to track per-document contributions. Replace the whole file with:

```go
package symbols

import "github.com/Open-MBEE/Systemica/internal/core/ast"

// fqnEntry records one symbol registered under a fully-qualified name.
type fqnEntry struct {
	fqn string
	sym *Symbol
}

// Index aggregates symbol information across all documents in a workspace.
// It owns each document's root scope and a global map from fully-qualified
// name to the symbol(s) declared under it. Per-document contributions are
// tracked so a document can be removed or re-added without leaving stale
// entries.
type Index struct {
	docRoots     map[string]*Scope     // document name -> root scope
	fqn          map[string][]*Symbol  // fully-qualified name -> symbols
	contributions map[string][]fqnEntry // document name -> entries it added
}

// NewIndex creates an empty index.
func NewIndex() *Index {
	return &Index{
		docRoots:      make(map[string]*Scope),
		fqn:           make(map[string][]*Symbol),
		contributions: make(map[string][]fqnEntry),
	}
}

// AddDocument builds the scope tree for root and records its symbols under
// their fully-qualified names. Re-adding the same document name first removes
// the document's previous contributions, so the index stays exact.
func (idx *Index) AddDocument(name string, root *ast.RootNamespace) {
	idx.RemoveDocument(name)
	rs := Build(root)
	idx.docRoots[name] = rs
	idx.indexScope(name, rs, "")
}

// RemoveDocument drops all of the named document's contributions from the
// global index and forgets its root scope. Unknown names are a no-op.
func (idx *Index) RemoveDocument(name string) {
	for _, e := range idx.contributions[name] {
		syms := idx.fqn[e.fqn]
		for i, s := range syms {
			if s == e.sym {
				syms = append(syms[:i], syms[i+1:]...)
				break
			}
		}
		if len(syms) == 0 {
			delete(idx.fqn, e.fqn)
		} else {
			idx.fqn[e.fqn] = syms
		}
	}
	delete(idx.contributions, name)
	delete(idx.docRoots, name)
}

// indexScope walks a scope, recording each distinct symbol under its FQN and
// recursing into child scopes. prefix is the FQN of the owning scope ("" at
// the document root). Every recorded (fqn, symbol) pair is also tracked as a
// contribution of the named document.
func (idx *Index) indexScope(doc string, scope *Scope, prefix string) {
	seen := make(map[*Symbol]bool)
	for _, syms := range scope.members {
		for _, sym := range syms {
			if seen[sym] {
				continue // symbol registered under both short and primary key
			}
			seen[sym] = true
			fqn := joinFQN(prefix, sym.Name)
			idx.fqn[fqn] = append(idx.fqn[fqn], sym)
			idx.contributions[doc] = append(idx.contributions[doc], fqnEntry{fqn: fqn, sym: sym})
			if sym.Scope != nil {
				idx.indexScope(doc, sym.Scope, fqn)
			}
		}
	}
}

// joinFQN joins a prefix and a name with "::".
func joinFQN(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "::" + name
}

// LookupQualified returns all symbols registered under the exact
// fully-qualified name.
func (idx *Index) LookupQualified(fqn string) []*Symbol {
	return idx.fqn[fqn]
}

// DocumentRoot returns the root scope for the named document, or nil.
func (idx *Index) DocumentRoot(name string) *Scope {
	return idx.docRoots[name]
}

// NewIndexFromDoc builds an Index containing a single document.
func NewIndexFromDoc(name string, root *ast.RootNamespace) *Index {
	idx := NewIndex()
	idx.AddDocument(name, root)
	return idx
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go build ./internal/core/... && go test ./internal/core/symbols/ -count=1`
Expected: PASS (new tests + all pre-existing symbols tests, including the two-doc ambiguity test that relies on two docs contributing the same FQN — removal is per-symbol so the other doc's entry survives).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/symbols/index.go internal/core/symbols/index_test.go
go vet ./internal/core/symbols/
git add internal/core/symbols/index.go internal/core/symbols/index_test.go
git commit -m "feat(symbols): add incremental Index.RemoveDocument"
```

### Task 2: model.Document type

**Files:**
- Create: `internal/core/model/document.go`
- Test: `internal/core/model/document_test.go`

A `Document` is the parsed state of one source file: its bytes, a version counter, the AST, the parse diagnostics, and its local scope tree. Parsing happens once in `newDocument`; the `Workspace` (Task 3) owns lifecycle.

- [ ] **Step 1: Write the failing test**

```go
package model

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

func TestNewDocumentParses(t *testing.T) {
	d := newDocument("a.sysml", []byte("package P { namespace N; }"), 1)
	if d.Name != "a.sysml" {
		t.Fatalf("Name = %q, want a.sysml", d.Name)
	}
	if d.Version != 1 {
		t.Fatalf("Version = %d, want 1", d.Version)
	}
	if d.AST == nil {
		t.Fatal("AST is nil")
	}
	if len(d.AST.Members) != 1 {
		t.Fatalf("len(Members) = %d, want 1", len(d.AST.Members))
	}
	if len(d.ParseDiagnostics) != 0 {
		t.Fatalf("ParseDiagnostics = %d, want 0", len(d.ParseDiagnostics))
	}
	if d.Scope == nil {
		t.Fatal("Scope is nil")
	}
	if _, ok := d.Scope.LookupLocal("P"); !ok {
		t.Fatal("P not in scope")
	}
	var _ = ast.Node(nil)
}

func TestNewDocumentReportsParseDiagnostics(t *testing.T) {
	d := newDocument("bad.sysml", []byte("package"), 1)
	if len(d.ParseDiagnostics) == 0 {
		t.Fatal("expected parse diagnostics for incomplete package")
	}
	if d.AST == nil {
		t.Fatal("AST should still be non-nil after recovery")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/model/ -run 'TestNewDocument' -v`
Expected: FAIL — package `model` and `newDocument` do not exist.

- [ ] **Step 3: Write minimal implementation**

```go
// Package model owns the workspace: the set of documents, the global symbol
// index, and the reindex pipeline that keeps them consistent.
package model

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Document is the parsed state of one source file.
type Document struct {
	Name             string
	Content          []byte
	Version          int
	AST              *ast.RootNamespace
	ParseDiagnostics []parser.Diagnostic
	Scope            *symbols.Scope
	sf               *source.SourceFile
}

// newDocument parses content and builds the document's local scope tree.
func newDocument(name string, content []byte, version int) *Document {
	sf := source.New(name, content)
	p := parser.New(sf)
	root := p.ParseFile()
	return &Document{
		Name:             name,
		Content:          content,
		Version:          version,
		AST:              root,
		ParseDiagnostics: p.Diagnostics,
		Scope:            symbols.Build(root),
		sf:               sf,
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go build ./internal/core/... && go test ./internal/core/model/ -run 'TestNewDocument' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/model/document.go internal/core/model/document_test.go
go vet ./internal/core/model/
git add internal/core/model/document.go internal/core/model/document_test.go
git commit -m "feat(model): add Document type parsing source into AST and scope"
```

### Task 3: model.Workspace core (add/update/remove, incremental reindex)

**Files:**
- Create: `internal/core/model/workspace.go`
- Test: `internal/core/model/workspace_test.go`

The `Workspace` is the single source of truth: it owns the document set and the global `symbols.Index`. Each document mutation reparses that one document and incrementally updates the global index via `RemoveDocument` + `AddDocument` (Task 1). Documents have two content sources: an **open buffer** (authoritative while open, modeling LSP `didOpen`/`didChange`) and **on-disk** bytes (from fsnotify). While a document is open, on-disk updates are stored but do not change the active content.

All exported methods take a write lock; read queries take a read lock. (The single-owner goroutine in Task 5 wraps these; the mutex makes the methods safe to call directly too, e.g. from tests.)

- [ ] **Step 1: Write the failing test**

```go
package model

import "testing"

func TestWorkspaceOpenIndexesDocument(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte("package P { namespace N; }"), 1)
	if syms := ws.Index().LookupQualified("P::N"); len(syms) != 1 {
		t.Fatalf("P::N = %d symbols, want 1", len(syms))
	}
}

func TestWorkspaceUpdateReindexesIncrementally(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte("package P { namespace Old; }"), 1)
	ws.Update("a.sysml", []byte("package P { namespace New; }"), 2)
	if syms := ws.Index().LookupQualified("P::Old"); len(syms) != 0 {
		t.Fatalf("P::Old = %d, want 0 (stale entry not cleared)", len(syms))
	}
	if syms := ws.Index().LookupQualified("P::New"); len(syms) != 1 {
		t.Fatalf("P::New = %d, want 1", len(syms))
	}
	if syms := ws.Index().LookupQualified("P"); len(syms) != 1 {
		t.Fatalf("P = %d, want 1 (not doubled)", len(syms))
	}
}

func TestWorkspaceCloseKeepsOnDiskContent(t *testing.T) {
	ws := NewWorkspace()
	ws.SetOnDisk("a.sysml", []byte("package Disk { namespace D; }"))
	ws.Open("a.sysml", []byte("package Buf { namespace B; }"), 1)
	if syms := ws.Index().LookupQualified("Buf::B"); len(syms) != 1 {
		t.Fatal("open buffer should be authoritative")
	}
	ws.Close("a.sysml")
	if syms := ws.Index().LookupQualified("Disk::D"); len(syms) != 1 {
		t.Fatal("closing should revert to on-disk content")
	}
	if syms := ws.Index().LookupQualified("Buf::B"); len(syms) != 0 {
		t.Fatal("buffer content should be gone after close")
	}
}

func TestWorkspaceRemoveDropsFromIndex(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte("package P;"), 1)
	ws.Remove("a.sysml")
	if syms := ws.Index().LookupQualified("P"); len(syms) != 0 {
		t.Fatalf("P = %d, want 0 after remove", len(syms))
	}
	if ws.Document("a.sysml") != nil {
		t.Fatal("document should be gone after remove")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/model/ -run 'TestWorkspace' -v`
Expected: FAIL — `NewWorkspace`, `Open`, `Update`, `Close`, `SetOnDisk`, `Remove`, `Index`, `Document` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
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
```

Remove the unused `diagChan` field if `go vet`/compiler complains; it is a placeholder kept only if Task 4 uses it (Task 4 replaces it with a real cache map, so leaving it is fine but delete if it triggers an unused warning — struct fields do not, so it is safe).

- [ ] **Step 4: Run test to verify it passes**

Run: `go build ./internal/core/... && go test ./internal/core/model/ -run 'TestWorkspace' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/model/workspace.go internal/core/model/workspace_test.go
go vet ./internal/core/model/
git add internal/core/model/workspace.go internal/core/model/workspace_test.go
git commit -m "feat(model): add Workspace with incremental reindex and open-buffer precedence"
```

### Task 4: Lazy analysis + diagnostic cache

**Files:**
- Modify: `internal/core/model/workspace.go`
- Test: `internal/core/model/diagnostics_test.go`

Diagnostics are computed lazily on request via `passes.Analyze`, using a **fresh resolver each call** (a new `symbols.Index`-backed `resolve.Resolver` is created inside `Analyze`, so no accumulation — but the workspace also creates a fresh analysis each time, never reusing a `Resolver`, per the b77-M2 note). Results are cached per document name. Any workspace mutation clears the **entire** cache (conservative invalidation — correctness first; fine-grained cross-document dependency tracking is a later optimization, spec §9 step 5). Recompute is lazy: nothing runs until `Diagnostics(name)` is called.

Parse diagnostics are adapted to `passes.Diagnostic` with `Severity: SeverityError`, `Code: "syntax"`, `Source: "syntax"` and handed to `Analyze` as the syntax-level input.

- [ ] **Step 1: Write the failing test**

```go
package model

import "testing"

func TestWorkspaceDiagnosticsClean(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte("package P { namespace N; alias A for P::N; }"), 1)
	if d := ws.Diagnostics("a.sysml"); len(d) != 0 {
		t.Fatalf("diagnostics = %d, want 0: %+v", len(d), d)
	}
}

func TestWorkspaceDiagnosticsReportsUnresolved(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte("package P { alias A for P::Missing; }"), 1)
	d := ws.Diagnostics("a.sysml")
	if len(d) == 0 {
		t.Fatal("expected an unresolved diagnostic")
	}
	if d[0].Source != "name-resolution" {
		t.Fatalf("Source = %q, want name-resolution", d[0].Source)
	}
}

func TestWorkspaceDiagnosticsRecomputeAfterEdit(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte("package P { alias A for P::Missing; }"), 1)
	if len(ws.Diagnostics("a.sysml")) == 0 {
		t.Fatal("expected diagnostics before fix")
	}
	ws.Update("a.sysml", []byte("package P { namespace Missing; alias A for P::Missing; }"), 2)
	if d := ws.Diagnostics("a.sysml"); len(d) != 0 {
		t.Fatalf("diagnostics after fix = %d, want 0: %+v", len(d), d)
	}
}

func TestWorkspaceDiagnosticsUnknownDocumentNil(t *testing.T) {
	ws := NewWorkspace()
	if d := ws.Diagnostics("nope.sysml"); d != nil {
		t.Fatalf("unknown doc diagnostics = %+v, want nil", d)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/model/ -run 'TestWorkspaceDiagnostics' -v`
Expected: FAIL — `Diagnostics` method undefined.

- [ ] **Step 3: Write minimal implementation**

Add to `workspace.go` the `passes` import, a cache field, the `Diagnostics` method, and wire `invalidateLocked` to clear the cache. Replace the placeholder `diagChan` field and the empty `invalidateLocked`:

```go
// add to imports:
//   "github.com/Open-MBEE/Systemica/internal/core/passes"

// in the Workspace struct, replace the diagChan placeholder with:
	diagCache map[string][]passes.Diagnostic

// in NewWorkspace, replace diagChan init with:
		diagCache: map[string][]passes.Diagnostic{},

// replace invalidateLocked with:
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
```

Note: `Diagnostics` takes the **write** lock because it populates the cache (a lazy write). This keeps it simple and race-free; a read/upgrade split is a later optimization.

- [ ] **Step 4: Run test to verify it passes**

Run: `go build ./internal/core/... && go test ./internal/core/model/ -run 'TestWorkspace' -v`
Expected: PASS (diagnostics tests + Task 3 tests).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/model/workspace.go internal/core/model/diagnostics_test.go
go vet ./internal/core/model/
git add internal/core/model/workspace.go internal/core/model/diagnostics_test.go
git commit -m "feat(model): add lazy diagnostics with conservative cache invalidation"
```

### Task 5: Single-owner-goroutine concurrency (change-event channel)

**Files:**
- Create: `internal/core/model/event.go`
- Test: `internal/core/model/event_test.go`

Per spec §9, workspace mutations are serialized through a single owner goroutine consuming a channel of change events; read queries take the read lock (already true — the `Workspace` methods are mutex-guarded). This task adds the event type and the owner loop. The loop simply dispatches each event to the existing mutex-guarded methods, giving a single serialized writer regardless of how many producers (fsnotify thread, LSP thread) post events.

- [ ] **Step 1: Write the failing test**

```go
package model

import "testing"

func TestEventLoopAppliesEvents(t *testing.T) {
	ws := NewWorkspace()
	loop := NewEventLoop(ws)
	go loop.Run()
	defer loop.Close()

	done := make(chan struct{})
	loop.Post(ChangeEvent{Kind: EventOpen, Name: "a.sysml", Content: []byte("package P { namespace N; }"), Version: 1, ack: done})
	<-done

	if syms := ws.Index().LookupQualified("P::N"); len(syms) != 1 {
		t.Fatalf("P::N = %d, want 1 after event applied", len(syms))
	}
}

func TestEventKindString(t *testing.T) {
	cases := map[EventKind]string{
		EventOpen: "open", EventChange: "change", EventClose: "close",
		EventCreate: "create", EventModify: "modify", EventDelete: "delete",
		EventKind(999): "unknown",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", k, got, want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/model/ -run 'TestEvent' -v`
Expected: FAIL — `NewEventLoop`, `ChangeEvent`, `EventKind`, event constants undefined.

- [ ] **Step 3: Write minimal implementation**

```go
package model

// EventKind classifies a change event feeding the reindex pipeline.
type EventKind int

const (
	EventOpen EventKind = iota // LSP didOpen: register authoritative buffer
	EventChange                // LSP didChange: replace buffer content
	EventClose                 // LSP didClose: drop buffer, revert to on-disk
	EventCreate                // fsnotify: file created on disk
	EventModify                // fsnotify: file modified on disk
	EventDelete                // fsnotify: file deleted on disk
)

var eventKindNames = map[EventKind]string{
	EventOpen:   "open",
	EventChange: "change",
	EventClose:  "close",
	EventCreate: "create",
	EventModify: "modify",
	EventDelete: "delete",
}

func (k EventKind) String() string {
	if s, ok := eventKindNames[k]; ok {
		return s
	}
	return "unknown"
}

// ChangeEvent is one mutation posted to the workspace owner goroutine.
// ack, when non-nil, is closed after the event has been applied (test/sync aid).
type ChangeEvent struct {
	Kind    EventKind
	Name    string
	Content []byte
	Version int
	ack     chan struct{}
}

// EventLoop is the single owner goroutine that serializes workspace mutations.
type EventLoop struct {
	ws     *Workspace
	events chan ChangeEvent
	done   chan struct{}
}

// NewEventLoop returns a loop bound to ws. Call Run in a goroutine, then Post.
func NewEventLoop(ws *Workspace) *EventLoop {
	return &EventLoop{
		ws:     ws,
		events: make(chan ChangeEvent, 64),
		done:   make(chan struct{}),
	}
}

// Post enqueues an event for the owner goroutine.
func (l *EventLoop) Post(ev ChangeEvent) { l.events <- ev }

// Close stops the owner goroutine after draining nothing further is posted.
func (l *EventLoop) Close() { close(l.done) }

// Run is the owner loop; run it in its own goroutine. It is the sole writer to
// the workspace, so mutations are serialized even with many event producers.
func (l *EventLoop) Run() {
	for {
		select {
		case <-l.done:
			return
		case ev := <-l.events:
			l.apply(ev)
			if ev.ack != nil {
				close(ev.ack)
			}
		}
	}
}

func (l *EventLoop) apply(ev ChangeEvent) {
	switch ev.Kind {
	case EventOpen:
		l.ws.Open(ev.Name, ev.Content, ev.Version)
	case EventChange:
		l.ws.Update(ev.Name, ev.Content, ev.Version)
	case EventClose:
		l.ws.Close(ev.Name)
	case EventCreate, EventModify:
		l.ws.SetOnDisk(ev.Name, ev.Content)
	case EventDelete:
		l.ws.Remove(ev.Name)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go build ./internal/core/... && go test ./internal/core/model/ -run 'TestEvent' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/model/event.go internal/core/model/event_test.go
go vet ./internal/core/model/
git add internal/core/model/event.go internal/core/model/event_test.go
git commit -m "feat(model): add single-owner event loop serializing workspace mutations"
```

### Task 6: Debounce coalescing

**Files:**
- Create: `internal/core/model/debounce.go`
- Test: `internal/core/model/debounce_test.go`

fsnotify (and rapid editor edits) can produce bursts of events for one file (e.g. a `git checkout` rewriting many files, or an editor saving with several write syscalls). A per-key debouncer coalesces a burst into a single delayed call. Keyed by document name so unrelated documents are not blocked by each other.

Design: `Debouncer` holds a window `time.Duration` and a per-key `*time.Timer`. `Trigger(key, fn)` (re)starts the key's timer; when it fires, `fn` runs once. A later `Trigger` for the same key before the window elapses resets the timer, coalescing the burst.

- [ ] **Step 1: Write the failing test**

```go
package model

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDebouncerCoalescesBurst(t *testing.T) {
	var calls int32
	d := NewDebouncer(30 * time.Millisecond)
	for i := 0; i < 5; i++ {
		d.Trigger("a.sysml", func() { atomic.AddInt32(&calls, 1) })
		time.Sleep(3 * time.Millisecond)
	}
	time.Sleep(80 * time.Millisecond)
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("calls = %d, want 1 (burst coalesced)", got)
	}
}

func TestDebouncerDistinctKeysIndependent(t *testing.T) {
	var a, b int32
	d := NewDebouncer(20 * time.Millisecond)
	d.Trigger("a.sysml", func() { atomic.AddInt32(&a, 1) })
	d.Trigger("b.sysml", func() { atomic.AddInt32(&b, 1) })
	time.Sleep(70 * time.Millisecond)
	if atomic.LoadInt32(&a) != 1 || atomic.LoadInt32(&b) != 1 {
		t.Fatalf("a=%d b=%d, want 1 and 1", a, b)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/model/ -run 'TestDebouncer' -v`
Expected: FAIL — `NewDebouncer` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
package model

import (
	"sync"
	"time"
)

// Debouncer coalesces bursts of triggers per key into a single delayed call.
type Debouncer struct {
	window time.Duration
	mu     sync.Mutex
	timers map[string]*time.Timer
}

// NewDebouncer returns a debouncer with the given coalescing window.
func NewDebouncer(window time.Duration) *Debouncer {
	return &Debouncer{window: window, timers: map[string]*time.Timer{}}
}

// Trigger (re)starts key's timer; fn runs once when the window elapses with no
// further trigger for that key. A trigger within the window resets the timer.
func (d *Debouncer) Trigger(key string, fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if t, ok := d.timers[key]; ok {
		t.Stop()
	}
	d.timers[key] = time.AfterFunc(d.window, func() {
		d.mu.Lock()
		delete(d.timers, key)
		d.mu.Unlock()
		fn()
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go build ./internal/core/... && go test ./internal/core/model/ -run 'TestDebouncer' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/model/debounce.go internal/core/model/debounce_test.go
go vet ./internal/core/model/
git add internal/core/model/debounce.go internal/core/model/debounce_test.go
git commit -m "feat(model): add per-key debouncer coalescing change bursts"
```

### Task 7: fsnotify watcher + open-buffer precedence

**Files:**
- Create: `internal/core/model/watcher.go`
- Test: `internal/core/model/watcher_test.go`

Per spec §9, external filesystem edits (create/modify/delete of `.sysml`/`.kerml` files) feed the *same* reindex pipeline as LSP buffer events. This task adds an fsnotify-backed `Watcher` that translates raw filesystem events into `ChangeEvent`s posted to an `EventLoop`, debounced per path, and honoring open-buffer precedence: while a document is open (LSP buffer authoritative), on-disk events for it are *dropped* at the watcher (the `Workspace.SetOnDisk` path also guards this, so this is a cheap early filter, not the sole guard).

`fsnotify` was added to `go.mod` via `go get github.com/fsnotify/fsnotify` (v1.10.1). The `Watcher` owns an `*fsnotify.Watcher`, a `*Debouncer`, and a predicate for open documents.

- [ ] **Step 1: Write the failing test**

```go
package model

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherPostsModifyEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.sysml")
	if err := os.WriteFile(path, []byte("package P { namespace N; }"), 0o644); err != nil {
		t.Fatal(err)
	}

	ws := NewWorkspace()
	loop := NewEventLoop(ws)
	go loop.Run()
	defer loop.Close()

	w, err := NewWatcher(loop, 10*time.Millisecond, func(string) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := w.Add(dir); err != nil {
		t.Fatal(err)
	}
	go w.Run()

	// Modify the file; watcher should debounce then post an EventModify.
	if err := os.WriteFile(path, []byte("package P { namespace M; }"), 0o644); err != nil {
		t.Fatal(err)
	}

	deadline := time.After(2 * time.Second)
	for {
		if syms := ws.Index().LookupQualified("P::M"); len(syms) == 1 {
			return // converged
		}
		select {
		case <-deadline:
			t.Fatalf("P::M not indexed after fs modify")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestWatcherIgnoresOpenDocuments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "b.sysml")
	if err := os.WriteFile(path, []byte("package P { namespace OnDisk; }"), 0o644); err != nil {
		t.Fatal(err)
	}

	ws := NewWorkspace()
	// Open with buffer content that differs from disk; buffer is authoritative.
	ws.Open("b.sysml", []byte("package P { namespace Buffered; }"), 1)

	loop := NewEventLoop(ws)
	go loop.Run()
	defer loop.Close()

	// isOpen reports the doc as open, so watcher drops its disk events.
	w, err := NewWatcher(loop, 10*time.Millisecond, func(name string) bool { return name == "b.sysml" })
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := w.Add(dir); err != nil {
		t.Fatal(err)
	}
	go w.Run()

	if err := os.WriteFile(path, []byte("package P { namespace Changed; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond) // allow any (wrongly) posted event to apply

	if syms := ws.Index().LookupQualified("P::Buffered"); len(syms) != 1 {
		t.Fatalf("P::Buffered = %d, want 1 (buffer must stay authoritative)", len(syms))
	}
	if syms := ws.Index().LookupQualified("P::Changed"); len(syms) != 0 {
		t.Fatalf("P::Changed = %d, want 0 (disk event must be ignored while open)", len(syms))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/model/ -run 'TestWatcher' -v`
Expected: FAIL — `NewWatcher` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
package model

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher translates filesystem events for .sysml/.kerml files into
// ChangeEvents posted to an EventLoop, debounced per path. Open documents
// (per the isOpen predicate) have their on-disk events dropped, so the LSP
// buffer stays authoritative.
type Watcher struct {
	loop    *EventLoop
	fsw     *fsnotify.Watcher
	deb     *Debouncer
	isOpen  func(name string) bool
	done    chan struct{}
}

// NewWatcher creates a Watcher posting to loop, coalescing bursts within
// window, and consulting isOpen(name) to skip open documents.
func NewWatcher(loop *EventLoop, window time.Duration, isOpen func(name string) bool) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if isOpen == nil {
		isOpen = func(string) bool { return false }
	}
	return &Watcher{
		loop:   loop,
		fsw:    fsw,
		deb:    NewDebouncer(window),
		isOpen: isOpen,
		done:   make(chan struct{}),
	}, nil
}

// Add registers a directory to watch.
func (w *Watcher) Add(dir string) error { return w.fsw.Add(dir) }

// Close stops watching and releases resources.
func (w *Watcher) Close() error {
	close(w.done)
	return w.fsw.Close()
}

// Run consumes filesystem events until Close. It is the sole caller of the
// debouncer, which in turn posts to the EventLoop (the sole workspace writer).
func (w *Watcher) Run() {
	for {
		select {
		case <-w.done:
			return
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.handle(ev)
		case _, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			// Errors are non-fatal for v1; drop them.
		}
	}
}

func (w *Watcher) handle(ev fsnotify.Event) {
	if !isModelSource(ev.Name) {
		return
	}
	name := filepath.Base(ev.Name)
	if w.isOpen(name) {
		return // buffer authoritative; ignore disk event
	}
	path := ev.Name
	switch {
	case ev.Op&fsnotify.Remove != 0 || ev.Op&fsnotify.Rename != 0:
		w.deb.Trigger(name, func() {
			w.loop.Post(ChangeEvent{Kind: EventDelete, Name: name})
		})
	case ev.Op&fsnotify.Create != 0:
		w.deb.Trigger(name, func() {
			content, err := os.ReadFile(path)
			if err != nil {
				return
			}
			w.loop.Post(ChangeEvent{Kind: EventCreate, Name: name, Content: content})
		})
	case ev.Op&fsnotify.Write != 0:
		w.deb.Trigger(name, func() {
			content, err := os.ReadFile(path)
			if err != nil {
				return
			}
			w.loop.Post(ChangeEvent{Kind: EventModify, Name: name, Content: content})
		})
	}
}

// isModelSource reports whether path is a SysML/KerML source file.
func isModelSource(path string) bool {
	return strings.HasSuffix(path, ".sysml") || strings.HasSuffix(path, ".kerml")
}
```

Note: `ChangeEvent.Name` is the file's base name (matching how documents are keyed in tests and by the LSP layer's document URIs later). The watcher posts through the `EventLoop`, so all mutations remain serialized through the single owner goroutine — the fsnotify goroutine never touches the `Workspace` directly.

- [ ] **Step 4: Run test to verify it passes**

Run: `go build ./internal/core/... && go test ./internal/core/model/ -run 'TestWatcher' -v`
Expected: PASS (both watcher tests).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/model/watcher.go internal/core/model/watcher_test.go
go vet ./internal/core/model/
git add internal/core/model/watcher.go internal/core/model/watcher_test.go go.mod go.sum
git commit -m "feat(model): add fsnotify watcher feeding reindex with open-buffer precedence"
```

### Task 8: Integration golden tests (scripted change sequences)

**Files:**
- Create: `internal/core/model/integration_test.go`

Per spec §9, all change sources feed one reindex path and the index + diagnostics must **converge** to reflect the latest content regardless of the change sequence. This task scripts a realistic sequence — open a buffer, edit it, "save" (buffer stays authoritative), then an external on-disk modify after close — and asserts the workspace state converges at each step. It exercises the incremental index (Task 1/3), open-buffer precedence (Task 3), and lazy diagnostics + conservative invalidation (Task 4) together as a whole.

- [ ] **Step 1: Write the failing test**

```go
package model

import "testing"

// hasSym reports whether the workspace index resolves fqn to exactly one symbol.
func hasSym(t *testing.T, ws *Workspace, fqn string) bool {
	t.Helper()
	return len(ws.Index().LookupQualified(fqn)) == 1
}

func TestWorkspaceConvergesAcrossChangeSequence(t *testing.T) {
	ws := NewWorkspace()

	// 1. Open a buffer.
	ws.Open("a.sysml", []byte("package P { namespace First; }"), 1)
	if !hasSym(t, ws, "P::First") {
		t.Fatal("after open: P::First missing")
	}
	if d := ws.Diagnostics("a.sysml"); len(d) != 0 {
		t.Fatalf("after open: %d diagnostics, want 0", len(d))
	}

	// 2. Edit the buffer (incremental reindex must drop the stale entry).
	ws.Update("a.sysml", []byte("package P { namespace Second; }"), 2)
	if hasSym(t, ws, "P::First") {
		t.Fatal("after edit: stale P::First still indexed")
	}
	if !hasSym(t, ws, "P::Second") {
		t.Fatal("after edit: P::Second missing")
	}

	// 3. External on-disk write while OPEN must NOT change the model (buffer wins).
	ws.SetOnDisk("a.sysml", []byte("package P { namespace Disk; }"))
	if hasSym(t, ws, "P::Disk") {
		t.Fatal("while open: on-disk content leaked into index")
	}
	if !hasSym(t, ws, "P::Second") {
		t.Fatal("while open: buffer content lost")
	}

	// 4. Close: on-disk content now takes effect.
	ws.Close("a.sysml")
	if hasSym(t, ws, "P::Second") {
		t.Fatal("after close: buffer content still indexed")
	}
	if !hasSym(t, ws, "P::Disk") {
		t.Fatal("after close: on-disk content not indexed")
	}
}

func TestWorkspaceCrossFileConverges(t *testing.T) {
	ws := NewWorkspace()
	// Two files; b imports a. Diagnostics must be clean once both present.
	ws.Open("a.sysml", []byte("package Lib { public namespace Widgets; }"), 1)
	ws.Open("b.sysml", []byte("package App { import Lib::*; alias W for Lib::Widgets; }"), 1)
	if d := ws.Diagnostics("b.sysml"); len(d) != 0 {
		t.Fatalf("cross-file clean: %d diagnostics, want 0: %v", len(d), d)
	}

	// Remove the provider; b's alias target must now be unresolved.
	ws.Remove("a.sysml")
	d := ws.Diagnostics("b.sysml")
	if len(d) == 0 {
		t.Fatal("after removing provider: expected unresolved diagnostic on b")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/model/ -run 'TestWorkspaceConverges|TestWorkspaceCrossFile' -v`
Expected: PASS immediately (these exercise already-implemented Tasks 1–4; this task adds coverage, no new production code). If any assertion fails, it reveals an incremental-reindex or precedence bug to fix in the relevant task before proceeding.

- [ ] **Step 3: (No new implementation)**

This is a pure integration test over Tasks 1–4. If Step 2 fails, fix the implicated task; otherwise proceed.

- [ ] **Step 4: Run full suite**

Run: `go build ./... && go test ./... -count=1 && go vet ./...`
Expected: all packages PASS, vet clean.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/model/integration_test.go
git add internal/core/model/integration_test.go
git commit -m "test(model): add scripted change-sequence and cross-file convergence tests"
```

## Self-Review

**Spec §9 coverage checklist:**

| §9 requirement | Task |
| --- | --- |
| Workspace owns doc set + global symbol index + diagnostic cache; single source of truth, one per session | Task 3 (`Workspace`), Task 4 (`diagCache`) |
| Document holds source text, current AST, per-doc scope, version | Task 2 (`Document`) |
| Two content sources: LSP open-buffer (authoritative when open) OR on-disk | Task 3 (`open`/`onDisk` maps, `SetOnDisk` skips reindex while open), Task 7 (watcher drops open-doc events) |
| Reindex pipeline is ONE path all change sources feed | Task 3 (`reindexLocked`), Task 5 (`EventLoop.apply` dispatches every source to it), Task 7 (fs events post to same loop) |
| (2) Debounce coalesce bursts | Task 6 (`Debouncer`), Task 7 (watcher uses it) |
| (3) Reparse → new AST + syntax diagnostics + fresh scope tree | Task 2 (`newDocument`), Task 3 (reindex rebuilds doc) |
| (4) Update index: swap doc local scope, recompute qualified-name entries (incremental) | Task 1 (`Index.RemoveDocument` + per-symbol `contributions`), Task 3 (`AddDocument` removes-then-adds) |
| (5) Invalidate memoized resolutions/diagnostics for reparsed + cross-doc dependents | Task 4 (`invalidateLocked` clears whole cache — see note) |
| (6) Lazy recompute on demand, not eager | Task 4 (`Diagnostics` computes on first request after change) |
| Concurrency: mutations serialized through single owner goroutine; reads take snapshot/read-lock | Task 5 (`EventLoop` sole writer via channel), Task 3 (`sync.RWMutex`) |
| Startup: discover files → eager-index names → stdlib lazy | Partially — discovery/eager-index is the caller's job (LSP Plan 6 walks the root and calls `SetOnDisk`); stdlib is Plan 5b |

**Placeholder scan:** none — every task has full code, exact commands, and commit messages.

**Type consistency:** `Document`, `Workspace`, `ChangeEvent`/`EventKind`, `Debouncer`, `Watcher` field/method names are consistent across tasks. `Workspace.reindexLocked`/`removeLocked`/`invalidateLocked` signatures stable from Task 3 (invalidateLocked body added in Task 4). `ChangeEvent.Name` = base file name everywhere (event loop, watcher, tests). Consumed APIs match delivered Plan 1–4 signatures (`parser.New(sf).ParseFile()` + `parser.Parser.Diagnostics`; `symbols.NewIndex`/`AddDocument`/`RemoveDocument`[Task 1]/`Build`/`LookupQualified`/`DocumentRoot`; `passes.Analyze` + `passes.Diagnostic`; `source.New`).

**Conservative-invalidation note (deliberate):** spec §9 step 5 calls for invalidating "reparsed documents + cross-document dependents (via resolution-layer dependency tracking)." v1 clears the **entire** diagnostic cache on any change. This is correct (never serves stale results — analysis recomputes lazily against the current index) but coarser than optimal. Fine-grained cross-document dependency tracking is an explicit **deferred optimization**, not a scope gap. The resolver's per-node memo is already discarded because `passes.Analyze` builds a fresh resolver each call (no accumulation — addresses the b77-M2 note).

**Scope note:** §10 (bundled stdlib + persistent cache) and §11 (project deps + manifest + fsnotify of `sysml.toml`) are Plans 5b/5c. `Watcher` here watches source files only; manifest-change handling and dependency re-resolution are out of scope. Startup file-discovery is left to the frontends (Plan 6/7) that own the workspace root.
