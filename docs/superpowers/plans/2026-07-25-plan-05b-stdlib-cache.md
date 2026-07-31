# Plan 5b: Bundled Standard Library & Persistent Cache

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the spec §10 library-loading and persistent-cache machinery in a new `internal/core/libs` package: an `embed.FS`-bundled standard library (with `SYSML_LIBRARY_PATH` on-disk override), a lazy per-file loader that parses a library file and builds its symbol scope, and a persistent on-disk cache of a serializable index record keyed by content hash + format version.

**Architecture:** New package `internal/core/libs`. A `Source` abstraction yields library file bytes by logical path (default = embedded FS; overridden by `SYSML_LIBRARY_PATH`). A `Loader` lazily loads a library file → parses via `parser` → builds a scope via `symbols.Build` → registers into a caller-supplied `*symbols.Index`. A `Cache` persists a reduced, gob-encodable `IndexRecord` (qualified names + kinds + spans + specialization-edge placeholders — NOT the AST-backed `symbols.Symbol`) under `$XDG_CACHE_HOME/sysml-ls/libs/<hash>.idx`, keyed by content-hash + format-version; a hit deserializes and skips parsing, a miss/stale/version-bump reparses and rewrites. The same cache code path is reusable for git dependencies (Plan 5c).

**Tech Stack:** Go 1.25 stdlib only (embed, os, crypto/sha256, encoding/gob, path, path/filepath). Consumes existing `parser`, `symbols`, `source`, `ast`. No new external deps.

**Bundling decision (v1):** The current parser does not yet parse the SysML definition/usage taxonomy (`datatype`/`specializes`/etc.), so real OMG stdlib files produce mostly `ErrorNode`s. This plan therefore bundles a **small curated, parser-compatible** library payload (namespace-core only: `package`/`namespace`/`alias`/`import`) that indexes cleanly, and wires the **full** §10 machinery (embed, override, lazy load, persistent cache) so the real OMG stdlib drops in unchanged once the def/usage grammar lands. This mirrors the Option-A scoping used in Plans 3/4/5a.

---

## Scope

**In scope (spec §10):**

- `Source` abstraction: read a library file's bytes and list available library files by logical path. Default implementation reads from an `embed.FS` compiled into the binary. If `SYSML_LIBRARY_PATH` is set (non-empty), an on-disk directory source overrides the embedded one (dev/custom stdlib).
- A **curated, parser-compatible** bundled payload under `internal/core/libs/stdlib/` (namespace-core constructs only, indexes cleanly — see Bundling decision above).
- `IndexRecord`: a reduced, gob-encodable snapshot of a library file's indexed symbols (per-symbol qualified name, `SymbolKind`, and `source.Span`). This is the persisted form; it deliberately excludes AST-backed fields (`Decl`, `Scope`, `OwnerScope`) which are not gob-friendly. Specialization edges are persisted as a placeholder field (empty in v1) reserved for the future def/usage taxonomy.
- `Cache`: persists one `IndexRecord` per library file to `$XDG_CACHE_HOME/sysml-ls/libs/<key>.idx` (fallback `os.UserCacheDir()`), keyed by content SHA-256 hash + a `formatVersion` constant. Hit → deserialize (skip lexer/parser). Miss / stale / format-version bump → reparse, rebuild, serialize, rewrite.
- `Loader`: given a logical library file name and a target `*symbols.Index`, lazily loads the file (cache hit → apply `IndexRecord`; miss → `Source` bytes → `parser.ParseFile` → `symbols.Build` → register into the index, then write cache). Idempotent per file.

**Deferred (NOT this plan):**

- Full OMG stdlib content (blocked on def/usage grammar; bundle curated payload for v1).
- Wiring the loader into `model.Workspace` resolution order (workspace → deps → stdlib): the loader is delivered as a standalone, tested unit here; the resolve/model integration lands with Plan 5c (deps) or a small follow-up, since it also needs the dependency layer. This plan builds and unit-tests `libs` in isolation.
- Git dependency fetching + `sysml.toml`/`sysml.lock` (Plan 5c §11). The `Cache` code path is designed to be reused there but is not wired to git here.
- Specialization-edge content (placeholder only; needs def/usage taxonomy).

## File Structure

- Create: `internal/core/libs/source.go` — `Source` interface + `embedSource` (default, wraps `embed.FS`) + `dirSource` (on-disk override) + `DefaultSource()` selecting via `SYSML_LIBRARY_PATH`.
- Create: `internal/core/libs/stdlib/ScalarValues.kerml` — curated parser-compatible payload (embedded via `//go:embed`).
- Create: `internal/core/libs/record.go` — `IndexRecord` gob type + `recordFromIndex` (extract reduced records from a `*symbols.Index` for a doc) + `formatVersion` const.
- Create: `internal/core/libs/cache.go` — `Cache{dir string}` + `NewCache()` (resolves cache dir) + `keyFor(content []byte) string` (sha256 + formatVersion) + `Load(key) (*IndexRecord, bool)` + `Store(key, *IndexRecord) error`.
- Create: `internal/core/libs/loader.go` — `Loader{src Source; cache *Cache}` + `NewLoader(src, cache)` + `Load(name string, idx *symbols.Index) error` (cache-integrated lazy load).
- Create tests: `source_test.go`, `record_test.go`, `cache_test.go`, `loader_test.go`, `integration_test.go`.
- Create: `internal/core/libs/testdata/` fixtures for on-disk override + integration.

## Package Reference

Consumed (delivered, disk-verified) APIs:

- `parser.New(sf *source.SourceFile) *parser.Parser`; `(*Parser).ParseFile() *ast.RootNamespace`; `parser.Parser.Diagnostics []parser.Diagnostic{Span source.Span; Message string}`.
- `symbols.NewIndex() *Index`; `(*Index).AddDocument(name string, root *ast.RootNamespace)`; `(*Index).LookupQualified(fqn string) []*Symbol`; `(*Index).DocumentRoot(name string) *Scope`; `symbols.Build(root *ast.RootNamespace) *Scope`.
- `symbols.Symbol{Name string; Kind SymbolKind; Decl ast.Node; Visibility ast.Visibility; DeclSpan source.Span; Scope *Scope; OwnerScope *Scope}`. `SymbolKind` int enum {`SymbolUnknown`,`SymbolPackage`,`SymbolNamespace`,`SymbolAlias`,`SymbolDependency`,`SymbolComment`,`SymbolDocumentation`,`SymbolTextualRepresentation`} + `String()`.
- `source.New(name string, content []byte) *SourceFile`; `(*SourceFile).Bytes() []byte`; `source.Span{Offset,Len int}` + `End()`.

Reduced record note: `IndexRecord` persists `[]symRecord{FQN string; Kind symbols.SymbolKind; Span source.Span; Supers []string}` (Supers = specialization-edge placeholder, empty in v1). To enumerate a doc's indexed symbols for extraction, the record builder walks the doc's scope tree from `Index.DocumentRoot(name)` (same traversal `Index.indexScope` uses), NOT the private `fqn`/`contributions` maps. If `symbols` needs a small exported helper to enumerate `(fqn, *Symbol)` pairs for a document, add it in Task 3 (note the deviation).

`formatVersion` is a package const (start at `1`); bumping it invalidates all cached records. Cache key = `hex(sha256(content)) + "-v" + formatVersion`.

### Task 1: Source abstraction (embed.FS default + SYSML_LIBRARY_PATH override)

**Files:**
- Create: `internal/core/libs/source.go`
- Create: `internal/core/libs/stdlib/ScalarValues.kerml` (needed so `//go:embed` compiles)
- Test: `internal/core/libs/source_test.go`
- Test fixture: `internal/core/libs/testdata/override/Custom.kerml`

- [ ] **Step 1: Write the failing test** — `internal/core/libs/source_test.go`

```go
package libs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEmbedSourceListsAndReads(t *testing.T) {
	src := DefaultSource()
	names := src.List()
	found := false
	for _, n := range names {
		if n == "ScalarValues.kerml" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ScalarValues.kerml in embedded list, got %v", names)
	}
	data, err := src.Read("ScalarValues.kerml")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty embedded library content")
	}
}

func TestDirSourceOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Custom.kerml"), []byte("package Custom;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SYSML_LIBRARY_PATH", dir)
	src := DefaultSource()
	names := src.List()
	if len(names) != 1 || names[0] != "Custom.kerml" {
		t.Fatalf("expected [Custom.kerml] from override dir, got %v", names)
	}
	data, err := src.Read("Custom.kerml")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(data) != "package Custom;\n" {
		t.Fatalf("unexpected override content: %q", data)
	}
}

func TestReadUnknownFileErrors(t *testing.T) {
	src := DefaultSource()
	if _, err := src.Read("Nope.kerml"); err == nil {
		t.Fatal("expected error reading unknown library file")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/libs/ -run 'TestEmbedSource|TestDirSource|TestReadUnknown' -v`
Expected: FAIL — `undefined: DefaultSource` (package doesn't compile yet).

- [ ] **Step 3: Write minimal implementation** — `internal/core/libs/source.go`

```go
// Package libs loads SysML/KerML standard-library files (bundled via embed.FS
// or overridden on disk) and maintains a persistent cache of their indexed
// symbols. See spec section 10.
package libs

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed stdlib/*.kerml
var stdlibFS embed.FS

// Source yields standard-library file contents by logical file name (base
// name, e.g. "ScalarValues.kerml").
type Source interface {
	// List returns the logical names of available library files, sorted.
	List() []string
	// Read returns the bytes of the named library file, or an error.
	Read(name string) ([]byte, error)
}

// DefaultSource returns a dirSource rooted at SYSML_LIBRARY_PATH when that
// environment variable is set and non-empty, otherwise the embedded source.
func DefaultSource() Source {
	if dir := os.Getenv("SYSML_LIBRARY_PATH"); dir != "" {
		return &dirSource{dir: dir}
	}
	return &embedSource{}
}

type embedSource struct{}

func (s *embedSource) List() []string {
	entries, err := stdlibFS.ReadDir("stdlib")
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

func (s *embedSource) Read(name string) ([]byte, error) {
	return stdlibFS.ReadFile("stdlib/" + name)
}

type dirSource struct{ dir string }

func (s *dirSource) List() []string {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".kerml") || strings.HasSuffix(n, ".sysml") {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

func (s *dirSource) Read(name string) ([]byte, error) {
	if name != filepath.Base(name) {
		return nil, fmt.Errorf("libs: invalid library file name %q", name)
	}
	return os.ReadFile(filepath.Join(s.dir, name))
}
```

Also create the embedded payload placeholder so `//go:embed` compiles (real content in Task 2) — `internal/core/libs/stdlib/ScalarValues.kerml`:

```
package ScalarValues;
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go build ./internal/core/libs/ && go test ./internal/core/libs/ -run 'TestEmbedSource|TestDirSource|TestReadUnknown' -v`
Expected: PASS (all three).

- [ ] **Step 5: gofmt, vet, commit**

```bash
gofmt -w internal/core/libs/
go vet ./internal/core/libs/
git add internal/core/libs/source.go internal/core/libs/source_test.go internal/core/libs/stdlib/ScalarValues.kerml
git commit -m "feat(libs): add library Source abstraction with embed.FS default and SYSML_LIBRARY_PATH override"
```

### Task 2: Curated bundled library payload

**Files:**
- Modify: `internal/core/libs/stdlib/ScalarValues.kerml` (replace placeholder with curated namespace-core content)
- Test: `internal/core/libs/payload_test.go`

Rationale: the payload must parse with ZERO diagnostics and index cleanly under the current namespace-core parser. Use only `package`/`namespace`/`alias`/`import` (NO `datatype`/`specializes` — those error today). This is a stand-in shaped like the real `ScalarValues.kerml` (a `package` containing named members) so the real file drops in once def/usage parsing lands.

- [ ] **Step 1: Write the failing test** — `internal/core/libs/payload_test.go`

```go
package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func TestBundledPayloadParsesCleanly(t *testing.T) {
	src := &embedSource{}
	for _, name := range src.List() {
		data, err := src.Read(name)
		if err != nil {
			t.Fatalf("Read(%q): %v", name, err)
		}
		p := parser.New(source.New(name, data))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("bundled %q produced %d parse diagnostics, want 0: %v", name, len(p.Diagnostics), p.Diagnostics)
		}
		idx := symbols.NewIndex()
		idx.AddDocument(name, root)
		if len(idx.LookupQualified("ScalarValues")) == 0 && name == "ScalarValues.kerml" {
			t.Fatalf("expected ScalarValues package indexed from %q", name)
		}
	}
}

func TestBundledScalarValuesHasMembers(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("ScalarValues.kerml")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(source.New("ScalarValues.kerml", data))
	root := p.ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("ScalarValues.kerml", root)
	if len(idx.LookupQualified("ScalarValues::Boolean")) != 1 {
		t.Fatal("expected ScalarValues::Boolean to be indexed")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/libs/ -run 'TestBundledPayload|TestBundledScalarValues' -v`
Expected: FAIL (`TestBundledScalarValuesHasMembers` — placeholder has no members; `ScalarValues::Boolean` not indexed).

- [ ] **Step 3: Write minimal implementation** — replace `internal/core/libs/stdlib/ScalarValues.kerml` with:

```
standard library package ScalarValues {
	doc /* Curated placeholder for the SysML v2 ScalarValues library.
	     Uses namespace-core constructs only until the definition/usage
	     grammar (datatype/specializes) is implemented; the real OMG
	     library file drops in here unchanged at that point. */

	namespace Boolean;
	namespace String;
	namespace Integer;
	namespace Rational;
	namespace Real;
	namespace Natural;
	namespace Positive;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/libs/ -run 'TestBundledPayload|TestBundledScalarValues' -v`
Expected: PASS (0 parse diagnostics; `ScalarValues` and `ScalarValues::Boolean` indexed).

- [ ] **Step 5: gofmt, vet, commit**

```bash
gofmt -w internal/core/libs/
go vet ./internal/core/libs/
git add internal/core/libs/stdlib/ScalarValues.kerml internal/core/libs/payload_test.go
git commit -m "feat(libs): add curated parser-compatible ScalarValues stdlib payload"
```

### Task 3: IndexRecord serializable form + gob round-trip

**Files:**
- Create: `internal/core/symbols/members.go` (small exported enumeration helper — see note)
- Create: `internal/core/libs/record.go`
- Test: `internal/core/libs/record_test.go`

**Why a new symbols helper:** `record.go` must enumerate the symbols a document
contributed, but `Scope.members` is unexported and `Index` has no public
per-document symbol iterator. Rather than reach into private maps, add one
focused exported method on `Scope` that returns its own distinct symbols in
definition order. `record.go` then walks the scope tree from
`Index.DocumentRoot(name)` using the existing `Children()` accessor, building
qualified names as it descends. This keeps `libs` dependent only on the public
`symbols` API.

- [ ] **Step 1: Write the failing test**

Create `internal/core/libs/record_test.go`:

```go
package libs

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func indexOf(t *testing.T, name, src string) *symbols.Index {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndex()
	idx.AddDocument(name, root)
	return idx
}

func fqnSet(rec *IndexRecord) map[string]symbols.SymbolKind {
	m := map[string]symbols.SymbolKind{}
	for _, s := range rec.Symbols {
		m[s.FQN] = s.Kind
	}
	return m
}

func TestRecordFromIndexCollectsReducedSymbols(t *testing.T) {
	idx := indexOf(t, "ScalarValues.kerml",
		"standard library package ScalarValues { namespace Boolean; namespace Real; }")
	rec := recordFromIndex("ScalarValues.kerml", idx)
	if rec == nil {
		t.Fatal("recordFromIndex returned nil")
	}
	if rec.Name != "ScalarValues.kerml" {
		t.Fatalf("Name = %q", rec.Name)
	}
	got := fqnSet(rec)
	if got["ScalarValues"] != symbols.SymbolPackage {
		t.Errorf("ScalarValues kind = %v, want package", got["ScalarValues"])
	}
	if got["ScalarValues::Boolean"] != symbols.SymbolNamespace {
		t.Errorf("ScalarValues::Boolean kind = %v, want namespace", got["ScalarValues::Boolean"])
	}
	if _, ok := got["ScalarValues::Real"]; !ok {
		t.Errorf("ScalarValues::Real missing from record")
	}
}

func TestIndexRecordGobRoundTrip(t *testing.T) {
	idx := indexOf(t, "a.kerml", "package P { namespace N; }")
	rec := recordFromIndex("a.kerml", idx)

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(rec); err != nil {
		t.Fatalf("encode: %v", err)
	}
	var got IndexRecord
	if err := gob.NewDecoder(&buf).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != rec.Name || len(got.Symbols) != len(rec.Symbols) {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, rec)
	}
	for i := range rec.Symbols {
		if got.Symbols[i] != rec.Symbols[i] {
			t.Errorf("symbol[%d] = %+v, want %+v", i, got.Symbols[i], rec.Symbols[i])
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/libs/ -run 'TestRecordFromIndex|TestIndexRecordGobRoundTrip' -v`
Expected: FAIL — `IndexRecord`, `recordFromIndex` undefined.

- [ ] **Step 3: Add the symbols enumeration helper**

Create `internal/core/symbols/members.go`:

```go
package symbols

// Members returns the distinct symbols declared directly in this scope, in
// definition order. A symbol registered under both its short and primary name
// appears once. Callers must not mutate the returned slice's symbols.
func (s *Scope) Members() []*Symbol {
	seen := map[*Symbol]bool{}
	var out []*Symbol
	for _, name := range s.memberOrder {
		for _, sym := range s.members[name] {
			if seen[sym] {
				continue
			}
			seen[sym] = true
			out = append(out, sym)
		}
	}
	return out
}
```

This requires `Scope` to track insertion order. In `internal/core/symbols/scope.go`
add a `memberOrder []string` field to the `Scope` struct and append the key in
`Define` the first time it is seen:

```go
// in Scope struct:
//   memberOrder []string

func (s *Scope) Define(name string, sym *Symbol) {
	if name == "" {
		return
	}
	if _, ok := s.members[name]; !ok {
		s.memberOrder = append(s.memberOrder, name)
	}
	s.members[name] = append(s.members[name], sym)
}
```

(Preserve any existing `Define` behavior; only add the order tracking. If a
`memberOrder` is impractical, `Members()` may instead iterate the `members` map
and sort by `DeclSpan.Offset` for determinism — pick one and keep record output
stable.)

- [ ] **Step 4: Write the record implementation**

Create `internal/core/libs/record.go`:

```go
package libs

import (
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// formatVersion is the on-disk index record format version. Bump it whenever
// the persisted shape changes; a mismatch invalidates all cached records.
const formatVersion = 1

// symRecord is the reduced, gob-encodable projection of a symbols.Symbol.
// It deliberately excludes the AST-backed Decl and the Scope/OwnerScope
// pointers, persisting only the fields the resolver needs to answer
// qualified-name lookups.
type symRecord struct {
	FQN    string
	Kind   symbols.SymbolKind
	Span   source.Span
	Supers []string // specialization-edge placeholder; empty until def/usage grammar lands
}

// IndexRecord is the serializable snapshot of one library document's symbols.
type IndexRecord struct {
	Name    string
	Symbols []symRecord
}

// recordFromIndex extracts a reduced, serializable record of every symbol the
// named document contributed to idx. Returns nil if the document is unknown.
func recordFromIndex(name string, idx *symbols.Index) *IndexRecord {
	root := idx.DocumentRoot(name)
	if root == nil {
		return nil
	}
	rec := &IndexRecord{Name: name}
	collectScope(root, "", rec)
	return rec
}

// collectScope walks scope's members (and child scopes) appending reduced
// records. prefix is the fully-qualified name of scope's owner ("" at root).
func collectScope(scope *symbols.Scope, prefix string, rec *IndexRecord) {
	for _, sym := range scope.Members() {
		fqn := sym.Name
		if prefix != "" {
			fqn = prefix + "::" + sym.Name
		}
		rec.Symbols = append(rec.Symbols, symRecord{
			FQN:  fqn,
			Kind: sym.Kind,
			Span: sym.DeclSpan,
		})
		if sym.Scope != nil {
			collectScope(sym.Scope, fqn, rec)
		}
	}
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go build ./internal/core/... && go test ./internal/core/symbols/ ./internal/core/libs/ -run 'TestRecordFromIndex|TestIndexRecordGobRoundTrip|TestScope|TestIndex' -count=1 -v`
Expected: PASS (new record tests + all pre-existing symbols tests still green).

- [ ] **Step 6: gofmt, vet, commit**

```bash
gofmt -w internal/core/symbols/ internal/core/libs/
go vet ./internal/core/symbols/ ./internal/core/libs/
git add internal/core/symbols/members.go internal/core/symbols/scope.go internal/core/libs/record.go internal/core/libs/record_test.go
git commit -m "feat(libs): add serializable IndexRecord with gob round-trip"
```

### Task 4: Cache (content-hash + format-version keyed persistence)

**Files:**
- Create: `internal/core/libs/cache.go`
- Test: `internal/core/libs/cache_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/core/libs/cache_test.go`:

```go
package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func sampleRecord(name string) *IndexRecord {
	return &IndexRecord{
		Name: name,
		Symbols: []symRecord{
			{FQN: "P", Kind: symbols.SymbolPackage, Span: source.Span{Offset: 0, Len: 1}},
			{FQN: "P::N", Kind: symbols.SymbolNamespace, Span: source.Span{Offset: 2, Len: 3}},
		},
	}
}

func TestCacheStoreLoadRoundTrip(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	rec := sampleRecord("a.kerml")
	key := c.keyFor([]byte("content-a"))
	if err := c.Store(key, rec); err != nil {
		t.Fatalf("store: %v", err)
	}
	got, ok := c.Load(key)
	if !ok {
		t.Fatal("Load miss after Store")
	}
	if got.Name != rec.Name || len(got.Symbols) != len(rec.Symbols) || got.Symbols[1] != rec.Symbols[1] {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, rec)
	}
}

func TestCacheLoadUnknownKeyMisses(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	if _, ok := c.Load(c.keyFor([]byte("never-stored"))); ok {
		t.Fatal("Load returned hit for unknown key")
	}
}

func TestCacheKeyDependsOnContentAndVersion(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	k1 := c.keyFor([]byte("alpha"))
	k2 := c.keyFor([]byte("beta"))
	if k1 == k2 {
		t.Fatal("distinct content produced identical cache keys")
	}
	// A record stored under content "alpha" must not be found by content
	// "beta" (stale-content miss) — the core cache-key invariant.
	if err := c.Store(k1, sampleRecord("x")); err != nil {
		t.Fatalf("store: %v", err)
	}
	if _, ok := c.Load(k2); ok {
		t.Fatal("stale content produced a cache hit")
	}
}

func TestNewCacheCreatesDir(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	c, err := NewCache()
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	if c.dir == "" {
		t.Fatal("NewCache produced empty dir")
	}
	// Store/Load must work against the freshly created dir.
	key := c.keyFor([]byte("z"))
	if err := c.Store(key, sampleRecord("z")); err != nil {
		t.Fatalf("store into new cache dir: %v", err)
	}
	if _, ok := c.Load(key); !ok {
		t.Fatal("Load miss from freshly created cache dir")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/libs/ -run 'TestCache|TestNewCache' -v`
Expected: FAIL — `Cache`, `NewCache`, `keyFor`, `Load`, `Store` undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/core/libs/cache.go`:

```go
package libs

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
)

// Cache persists reduced library IndexRecords to disk keyed by content hash and
// format version. A hit lets the loader skip lexing/parsing entirely. The same
// mechanism serves git dependencies (Plan 5c) since it keys purely on content.
type Cache struct {
	dir string
}

// NewCache resolves the cache directory ($XDG_CACHE_HOME or os.UserCacheDir),
// appends "sysml-ls/libs", creates it, and returns a ready Cache.
func NewCache() (*Cache, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		d, err := os.UserCacheDir()
		if err != nil {
			return nil, err
		}
		base = d
	}
	dir := filepath.Join(base, "sysml-ls", "libs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Cache{dir: dir}, nil
}

// keyFor derives a cache key from the file content and the current format
// version. Any content change or version bump yields a distinct key, so stale
// entries are simply never found (miss) rather than requiring explicit
// invalidation.
func (c *Cache) keyFor(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]) + "-v" + strconv.Itoa(formatVersion)
}

func (c *Cache) path(key string) string {
	return filepath.Join(c.dir, key+".idx")
}

// Load returns the cached record for key, or (nil, false) on any miss
// (absent file, read error, or decode error — all treated as a benign miss).
func (c *Cache) Load(key string) (*IndexRecord, bool) {
	data, err := os.ReadFile(c.path(key))
	if err != nil {
		return nil, false
	}
	var rec IndexRecord
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&rec); err != nil {
		return nil, false
	}
	return &rec, true
}

// Store gob-encodes rec and writes it to <dir>/<key>.idx.
func (c *Cache) Store(key string, rec *IndexRecord) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(rec); err != nil {
		return err
	}
	return os.WriteFile(c.path(key), buf.Bytes(), 0o644)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go build ./internal/core/... && go test ./internal/core/libs/ -run 'TestCache|TestNewCache' -count=1 -v`
Expected: PASS (round-trip, unknown-key miss, distinct/stale keys, new-dir).

- [ ] **Step 5: gofmt, vet, commit**

```bash
gofmt -w internal/core/libs/
go vet ./internal/core/libs/
git add internal/core/libs/cache.go internal/core/libs/cache_test.go
git commit -m "feat(libs): add content-hash + format-version keyed persistent cache"
```

### Task 5: Loader (lazy parse → build scope → register into index)

**Files:**
- Create: `internal/core/libs/loader.go`
- Test: `internal/core/libs/loader_test.go`

This task delivers the parse-and-register path with NO cache yet (Task 6 adds
the cache short-circuit). `Load` reads the named library file from the Source,
parses it, and registers its scope into the caller-supplied `*symbols.Index`
via the existing `AddDocument`.

- [ ] **Step 1: Write the failing test**

Create `internal/core/libs/loader_test.go`:

```go
package libs

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func TestLoaderLoadsBundledLibraryIntoIndex(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	ld := NewLoader(DefaultSource(), c)
	idx := symbols.NewIndex()

	if err := ld.Load("ScalarValues.kerml", idx); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := idx.LookupQualified("ScalarValues"); len(got) != 1 {
		t.Fatalf("ScalarValues lookup = %d, want 1", len(got))
	}
	if got := idx.LookupQualified("ScalarValues::Boolean"); len(got) != 1 {
		t.Fatalf("ScalarValues::Boolean lookup = %d, want 1", len(got))
	}
}

func TestLoaderReadErrorPropagates(t *testing.T) {
	c := &Cache{dir: t.TempDir()}
	ld := NewLoader(DefaultSource(), c)
	idx := symbols.NewIndex()
	if err := ld.Load("NoSuchLibrary.kerml", idx); err == nil {
		t.Fatal("Load of missing library returned nil error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/libs/ -run 'TestLoader' -v`
Expected: FAIL — `NewLoader`, `Load` undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/core/libs/loader.go`:

```go
package libs

import (
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Loader lazily loads library files from a Source and registers their symbols
// into a target index, using a Cache to skip parsing on repeat loads (Task 6).
type Loader struct {
	src   Source
	cache *Cache
}

// NewLoader returns a Loader over src, using cache for persistence.
func NewLoader(src Source, cache *Cache) *Loader {
	return &Loader{src: src, cache: cache}
}

// Load reads the named library file, parses it, and registers the resulting
// scope into idx. Cache integration is added in Task 6.
func (l *Loader) Load(name string, idx *symbols.Index) error {
	content, err := l.src.Read(name)
	if err != nil {
		return err
	}
	p := parser.New(source.New(name, content))
	root := p.ParseFile()
	idx.AddDocument(name, root)
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go build ./internal/core/... && go test ./internal/core/libs/ -run 'TestLoader' -count=1 -v`
Expected: PASS (bundled ScalarValues registered; missing file errors).

- [ ] **Step 5: gofmt, vet, commit**

```bash
gofmt -w internal/core/libs/
go vet ./internal/core/libs/
git add internal/core/libs/loader.go internal/core/libs/loader_test.go
git commit -m "feat(libs): add lazy library loader parsing and registering into index"
```

### Task 6: Loader cache integration (hit skips parse, miss writes)

**Files:**
- Create: `internal/core/symbols/records.go` (apply reduced records without AST — see note)
- Modify: `internal/core/libs/loader.go`
- Test: `internal/core/libs/loader_cache_test.go`

**Why a new symbols API:** on a cache hit the loader has an `IndexRecord` (FQN +
kind + span, no AST). It must make those symbols answerable via
`Index.LookupQualified` without re-parsing. Add a focused exported method
`Index.AddRecords(name string, entries []RecordEntry)` that registers synthetic,
AST-less symbols keyed by their full FQN into the same per-document
`contributions` bookkeeping that `RemoveDocument` uses — so a cached document is
removable/replaceable exactly like a parsed one. `RecordEntry` lives in
`symbols` so `libs` maps its private `symRecord` onto it.

- [ ] **Step 1: Write the failing test**

Create `internal/core/libs/loader_cache_test.go`:

```go
package libs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// countingSource wraps a Source and counts Read calls so we can prove a cache
// hit skips the (Read+parse) path on the second Load.
type countingSource struct {
	inner Source
	reads int
}

func (c *countingSource) List() []string { return c.inner.List() }
func (c *countingSource) Read(name string) ([]byte, error) {
	c.reads++
	return c.inner.Read(name)
}

func TestLoaderCacheMissThenHit(t *testing.T) {
	cacheDir := t.TempDir()
	cache := &Cache{dir: cacheDir}
	cs := &countingSource{inner: DefaultSource()}
	ld := NewLoader(cs, cache)

	// First load: miss -> reads source, parses, writes .idx.
	idx1 := symbols.NewIndex()
	if err := ld.Load("ScalarValues.kerml", idx1); err != nil {
		t.Fatalf("first Load: %v", err)
	}
	if cs.reads != 1 {
		t.Fatalf("reads after first load = %d, want 1", cs.reads)
	}
	if len(idx1.LookupQualified("ScalarValues::Boolean")) != 1 {
		t.Fatal("first load did not index ScalarValues::Boolean")
	}
	// A .idx file must now exist in the cache dir.
	entries, _ := os.ReadDir(cacheDir)
	found := false
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".idx" {
			found = true
		}
	}
	if !found {
		t.Fatal("no .idx file written after cache miss")
	}

	// Second load into a fresh index with a fresh loader sharing the cache:
	// hit -> still Reads content (to compute the key) but must NOT re-parse.
	// We assert the symbols are populated purely from the cached record.
	idx2 := symbols.NewIndex()
	if err := ld.Load("ScalarValues.kerml", idx2); err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if len(idx2.LookupQualified("ScalarValues")) != 1 ||
		len(idx2.LookupQualified("ScalarValues::Boolean")) != 1 {
		t.Fatal("cached load did not repopulate index")
	}
}

func TestIndexAddRecordsRemovable(t *testing.T) {
	idx := symbols.NewIndex()
	idx.AddRecords("lib.kerml", []symbols.RecordEntry{
		{FQN: "P", Kind: symbols.SymbolPackage},
		{FQN: "P::N", Kind: symbols.SymbolNamespace},
	})
	if len(idx.LookupQualified("P::N")) != 1 {
		t.Fatal("AddRecords did not register P::N")
	}
	idx.RemoveDocument("lib.kerml")
	if len(idx.LookupQualified("P::N")) != 0 {
		t.Fatal("RemoveDocument did not drop record-added symbols")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/libs/ -run 'TestLoaderCacheMissThenHit|TestIndexAddRecords' -v`
Expected: FAIL — `Index.AddRecords`, `symbols.RecordEntry` undefined; loader does not consult cache.

- [ ] **Step 3: Add the symbols record-apply API**

Create `internal/core/symbols/records.go`:

```go
package symbols

import "github.com/Open-MBEE/Systemica/internal/core/source"

// RecordEntry is a minimal, AST-less description of a symbol, used to populate
// the index from a persisted cache record instead of a parsed document.
type RecordEntry struct {
	FQN  string
	Kind SymbolKind
	Span source.Span
}

// AddRecords registers synthetic, AST-less symbols for a document directly by
// their fully-qualified names. It first removes any prior contributions for
// name (idempotent re-add), mirroring AddDocument. Symbols added this way carry
// no Decl/Scope and are keyed only by FQN, which is sufficient for
// qualified-name resolution against library content restored from cache.
func (idx *Index) AddRecords(name string, entries []RecordEntry) {
	idx.RemoveDocument(name)
	for _, e := range entries {
		sym := &Symbol{Name: e.FQN, Kind: e.Kind, DeclSpan: e.Span}
		idx.fqn[e.FQN] = append(idx.fqn[e.FQN], sym)
		idx.contributions[name] = append(idx.contributions[name], fqnEntry{fqn: e.FQN, sym: sym})
	}
}
```

(Field/type names — `idx.fqn`, `idx.contributions`, `fqnEntry{fqn, sym}` — must
match the current `index.go` definitions exactly; verify before writing. This
method lives in package `symbols` so it can touch those private fields.)

- [ ] **Step 4: Wire the cache into the loader**

Modify `internal/core/libs/loader.go` `Load`:

```go
func (l *Loader) Load(name string, idx *symbols.Index) error {
	content, err := l.src.Read(name)
	if err != nil {
		return err
	}
	key := l.cache.keyFor(content)

	// Cache hit: restore reduced records, skip lexing/parsing entirely.
	if rec, ok := l.cache.Load(key); ok {
		idx.AddRecords(name, recordEntries(rec))
		return nil
	}

	// Miss: parse, register, extract a reduced record, persist it.
	p := parser.New(source.New(name, content))
	root := p.ParseFile()
	idx.AddDocument(name, root)
	if rec := recordFromIndex(name, idx); rec != nil {
		_ = l.cache.Store(key, rec) // cache write failure is non-fatal
	}
	return nil
}

// recordEntries projects a persisted IndexRecord onto symbols.RecordEntry.
func recordEntries(rec *IndexRecord) []symbols.RecordEntry {
	out := make([]symbols.RecordEntry, len(rec.Symbols))
	for i, s := range rec.Symbols {
		out[i] = symbols.RecordEntry{FQN: s.FQN, Kind: s.Kind, Span: s.Span}
	}
	return out
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go build ./internal/core/... && go test ./internal/core/symbols/ ./internal/core/libs/ -count=1 -v`
Expected: PASS (cache miss writes .idx, hit repopulates without re-parse; AddRecords symbols are removable; all pre-existing symbols/libs tests green).

- [ ] **Step 6: gofmt, vet, commit**

```bash
gofmt -w internal/core/symbols/ internal/core/libs/
go vet ./internal/core/symbols/ ./internal/core/libs/
git add internal/core/symbols/records.go internal/core/libs/loader.go internal/core/libs/loader_cache_test.go
git commit -m "feat(libs): integrate cache into loader (hit skips parse, miss writes)"
```

### Task 7: Integration tests (embed load, override, cache hit/miss/stale)

**Files:**
- Create: `internal/core/libs/integration_test.go`
- Create: `internal/core/libs/testdata/customlib/Custom.kerml`

- [ ] **Step 1: Write the tests**

Create `internal/core/libs/testdata/customlib/Custom.kerml`:

```
package Custom { namespace Widget; }
```

Create `internal/core/libs/integration_test.go`:

```go
package libs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// TestEmbedLoadEndToEnd: default embedded source + real cache dir under a temp
// XDG_CACHE_HOME loads the bundled library and answers qualified lookups.
func TestEmbedLoadEndToEnd(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	cache, err := NewCache()
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	ld := NewLoader(DefaultSource(), cache)
	idx := symbols.NewIndex()
	if err := ld.Load("ScalarValues.kerml", idx); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(idx.LookupQualified("ScalarValues::Real")) != 1 {
		t.Fatal("ScalarValues::Real not indexed via embedded end-to-end load")
	}
}

// TestSysmlLibraryPathOverride: SYSML_LIBRARY_PATH points DefaultSource at an
// on-disk directory, and the loader indexes that custom library instead.
func TestSysmlLibraryPathOverride(t *testing.T) {
	libDir, err := filepath.Abs("testdata/customlib")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SYSML_LIBRARY_PATH", libDir)
	cache := &Cache{dir: t.TempDir()}
	ld := NewLoader(DefaultSource(), cache)
	idx := symbols.NewIndex()
	if err := ld.Load("Custom.kerml", idx); err != nil {
		t.Fatalf("Load custom: %v", err)
	}
	if len(idx.LookupQualified("Custom::Widget")) != 1 {
		t.Fatal("SYSML_LIBRARY_PATH override did not load Custom::Widget")
	}
	// The bundled library must NOT be visible under the override source.
	if _, err := DefaultSource().Read("ScalarValues.kerml"); err == nil {
		t.Fatal("override source unexpectedly served bundled ScalarValues.kerml")
	}
}

// TestCacheStaleContentReparsedNotServed: a cache entry stored under stale
// content must not be served when the source content differs; the loader
// reparses and produces the correct current symbols.
func TestCacheStaleContentReparsedNotServed(t *testing.T) {
	libDir := t.TempDir()
	libFile := filepath.Join(libDir, "Evolving.kerml")
	if err := os.WriteFile(libFile, []byte("package Evolving { namespace First; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SYSML_LIBRARY_PATH", libDir)
	cache := &Cache{dir: t.TempDir()}
	ld := NewLoader(DefaultSource(), cache)

	idx1 := symbols.NewIndex()
	if err := ld.Load("Evolving.kerml", idx1); err != nil {
		t.Fatalf("first Load: %v", err)
	}
	if len(idx1.LookupQualified("Evolving::First")) != 1 {
		t.Fatal("first load missing Evolving::First")
	}

	// Change the file content: the old cache key no longer matches, so the
	// loader must reparse and index the NEW member, not serve the stale one.
	if err := os.WriteFile(libFile, []byte("package Evolving { namespace Second; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx2 := symbols.NewIndex()
	if err := ld.Load("Evolving.kerml", idx2); err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if len(idx2.LookupQualified("Evolving::Second")) != 1 {
		t.Fatal("stale cache served: Evolving::Second not indexed after content change")
	}
	if len(idx2.LookupQualified("Evolving::First")) != 0 {
		t.Fatal("stale cache served: Evolving::First should be gone after content change")
	}
}
```

- [ ] **Step 2: Run tests**

Run: `go build ./internal/core/... && go test ./internal/core/libs/ -run 'TestEmbedLoadEndToEnd|TestSysmlLibraryPathOverride|TestCacheStaleContentReparsedNotServed' -count=1 -v`
Expected: PASS. (If the override test fails because `dirSource.List`/`Read`
filter or path handling differs, reconcile with the Task 1 implementation.)

- [ ] **Step 3: Full suite + gofmt + vet + commit**

```bash
go build ./... && go test ./... -count=1
gofmt -w internal/core/libs/
go vet ./...
git add internal/core/libs/integration_test.go internal/core/libs/testdata/customlib/Custom.kerml
git commit -m "test(libs): add stdlib load, override, and cache integration tests"
```
Expected: all packages green.

## Self-Review

**Spec §10 coverage (a–e):**

| §10 clause | Task(s) | How satisfied |
|---|---|---|
| (a) bundling via `embed.FS`; `SYSML_LIBRARY_PATH` override | 1, 2 | `//go:embed stdlib/*.kerml` + `embedSource`; `dirSource` selected by `DefaultSource()` when env set; curated payload parses cleanly |
| (b) lazy load — first reference parses that file, not all-upfront | 5, 6 | `Loader.Load(name, idx)` loads a single named file on demand |
| (c) cache dir `$XDG_CACHE_HOME/sysml-ls/libs/<hash>.idx`; key = content hash + format-version | 4 | `NewCache` resolves dir (fallback `os.UserCacheDir`); `keyFor` = `sha256(content)+"-v"+formatVersion` |
| (d) compact binary (gob) of index side tables; persist qualified names, kinds, spans, specialization edges | 3, 4 | `IndexRecord`/`symRecord` gob-encoded; FQN + Kind + Span persisted; `Supers` placeholder reserved for specialization edges |
| (e) same cache mechanism serves git deps | 4 | `Cache` keys purely on content bytes; Plan 5c reuses it unchanged (wiring deferred) |

**Placeholder scan:** no `<FILL>`/`TBD`/`TODO` remain; every task has full code, exact `go test -run` filters, and commit messages.

**Type consistency across tasks:** `Source` interface (`List`/`Read`) — Tasks 1, 5, 6, 7; `IndexRecord{Name, Symbols []symRecord}` + `symRecord{FQN, Kind, Span, Supers}` — Tasks 3, 4, 6; `Cache{dir}` + `NewCache`/`keyFor`/`Load`/`Store` — Tasks 4, 5, 6, 7; `Loader{src, cache}` + `NewLoader`/`Load` — Tasks 5, 6, 7; `symbols.RecordEntry{FQN, Kind, Span}` + `Index.AddRecords` — Task 6, consumed by loader `recordEntries`; `Scope.Members()` + `memberOrder` — Task 3, consumed by `collectScope`. All signatures stable.

**New exported symbols-package API introduced (deviations, deliberate):**
- `Scope.Members() []*Symbol` + `Scope.memberOrder` field (Task 3) — needed to enumerate a document's symbols over the public API instead of private maps.
- `symbols.RecordEntry` + `Index.AddRecords(name, []RecordEntry)` (Task 6) — needed to repopulate the index from a cache hit without an AST.
Both are minimal, documented, and keep `libs` dependent only on public `symbols`.

**Deferred (explicit, NOT scope gaps):** real OMG stdlib content (blocked on the definition/usage grammar — datatype/specializes produce ErrorNodes today; curated namespace-core payload ships as v1 and the real files drop in unchanged later); wiring `Loader` into `model.Workspace` resolution order (workspace → deps → stdlib) — deferred to Plan 5c/a follow-up since it interlocks with the dependency layer; git-dependency fetching and `sysml.toml`/`sysml.lock` (Plan 5c §11) — the `Cache` path is designed reusable but not wired to git here; specialization-edge content (`symRecord.Supers` is a reserved empty placeholder).

**Ambiguity/risk note:** cached symbols added via `AddRecords` are AST-less (no `Decl`/`Scope`), so they answer `LookupQualified` (qualified-name resolution) but cannot yet support features needing the declaration node (hover doc text, go-to-def into library source). For v1 stdlib that is acceptable — library go-to-def/hover can force a parse (miss path) when the AST is actually required; note this for Plan 6 LSP. Also: `AddRecords` registers each symbol only under its full FQN (not short names), matching how library qualified references resolve; if unqualified library-name resolution is later needed, extend the record to carry short names too.
