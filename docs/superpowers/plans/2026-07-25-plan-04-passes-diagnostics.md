# Plan 04: Validation Passes & Unified Diagnostics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a pluggable validation-pass framework (`internal/core/passes`) with a rich unified `Diagnostic` model, an ordered pass registry with dependency-level prerequisite skipping, and v1 passes for syntax (parser error nodes) and name-resolution (unresolved/ambiguous references), plus a single `Analyze` entry point producing sorted unified diagnostics for a document.

**Architecture:** A new `internal/core/passes` package defines its own richer `Diagnostic{Severity, Span, Message, Code, Source}`, a `Pass` interface `Run(ctx, name, root) []Diagnostic` tagged with a `PassLevel`, and a `Registry` whose runner executes passes in level order and records which levels produced errors so higher-level passes can be skipped to avoid cascade noise. v1 ships two passes: `SyntaxPass` (adapts parse error-node diagnostics) and `NameResolutionPass` (adapts `resolve.Resolver.ResolveDocument` diagnostics). `Context` carries the `*symbols.Index` (and constructs the `*resolve.Resolver`). Dependency direction: `passes -> {resolve, symbols, ast, source}`; parser only in tests.

**Tech Stack:** Go 1.25 stdlib only; consumes internal/core/{ast, source, symbols, resolve}; tests additionally import internal/core/parser to build ASTs.

---

## Scope

**In scope (v1, spec §8 depth = syntax + name-resolution only):**

- Rich `Diagnostic` model: `Severity` (Error/Warning/Info/Hint), `Span`, `Message`, stable `Code` string, `Source` pass-ID string.
- `Pass` interface: `Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic`. Whole-document granularity for v1 (per-node lazy invocation is a Plan-6 LSP concern; the interface takes the whole root so the runner stays simple and both v1 passes are naturally whole-document).
- `PassLevel` enum (`LevelSyntax`, `LevelNameResolution`, `LevelType`, `LevelConstraint`) establishing dependency order. Only the first two are used in v1; the latter two exist so roadmap passes register without runner changes.
- `Context`: carries the `*symbols.Index` and lazily builds/holds a shared `*resolve.Resolver`, plus the document name. Passes read from it.
- Ordered `Registry` + runner: runs registered passes sorted by level; if any pass at a level emits an Error-severity diagnostic, passes at strictly higher levels are skipped (prerequisite failure → avoid cascade). Same-level passes never skip each other.
- `SyntaxPass` (LevelSyntax): adapts `parser`-produced error-node diagnostics into `passes.Diagnostic` (Severity Error, Code "syntax", Source "syntax"). Because `passes` must not import `parser`, syntax diagnostics are passed INTO the Context by the caller (who ran the parser), not produced by the pass re-parsing.
- `NameResolutionPass` (LevelNameResolution): runs `resolve.Resolver.ResolveDocument`, adapts its `resolve.Diagnostic`s into `passes.Diagnostic` (Severity Error, Code "unresolved"/"ambiguous" derived from message prefix, Source "name-resolution").
- `Analyze(name, root, parseDiags, idx) []Diagnostic`: builds a Context, runs the default registry (SyntaxPass + NameResolutionPass), returns diagnostics sorted by Span.Offset then Source.

**Deferred (roadmap, NOT this plan):** type/multiplicity/redefinition/specialization-cycle/full-constraint passes (LevelType/LevelConstraint reserved but unimplemented); per-document diagnostic caching + incremental recompute (Plan 5 workspace territory — Analyze is stateless here); per-node lazy pass invocation (Plan 6 LSP); quick-fix actions keyed off Code.

## File Structure

New package `internal/core/passes/`:

- `diagnostic.go` — `Severity` enum + `severityNames`/`String()`; `Diagnostic{Severity; Span source.Span; Message; Code; Source string}`.
- `pass.go` — `PassLevel` enum + `String()`; `Pass` interface; `Context` struct + `NewContext`.
- `registry.go` — `Registry` (ordered `[]Pass`); `NewRegistry`/`Register`; `Run(ctx, name, root) []Diagnostic` with level-ordered execution + prerequisite skipping.
- `syntax.go` — `SyntaxPass` struct implementing `Pass`; adapts `Context.ParseDiagnostics`.
- `nameres.go` — `NameResolutionPass` struct implementing `Pass`; runs resolver, adapts `resolve.Diagnostic`s.
- `analyze.go` — `Analyze(...)` convenience entry building Context + default registry + sorted result; `DefaultRegistry()`.
- `*_test.go` — one per source file + `integration_test.go`.
- `testdata/passes/{clean,errors}.sysml` + `.golden`.

**Dependency direction:** `passes -> resolve -> symbols -> ast -> source`. `passes` also imports `symbols`, `ast`, `source` directly. `passes` MUST NOT import `parser` (syntax diagnostics enter via `Context`, produced by the caller who parsed). Test files import `parser` + `source` to build ASTs and parse diagnostics.

## Grammar / Package Reference

Delivered APIs this plan consumes (authoritative — verify on disk before use):

- `parser`: `parser.New(sf *source.SourceFile) *parser.Parser`; `(*Parser).ParseFile() *ast.RootNamespace`; field `Parser.Diagnostics []parser.Diagnostic` where `parser.Diagnostic{Span source.Span; Message string}`. (Used only in tests + by Analyze's caller.)
- `resolve`: `resolve.New(idx *symbols.Index) *resolve.Resolver`; `(*Resolver).ResolveDocument(name string, root *ast.RootNamespace)` accumulates onto exported field `Resolver.Diagnostics []resolve.Diagnostic` where `resolve.Diagnostic{Span source.Span; Message string}`. Messages: `"unresolved reference: <qn>"`, `"ambiguous reference: <qn> (<n> candidates)"`.
- `symbols`: `symbols.NewIndexFromDoc(name string, root *ast.RootNamespace) *symbols.Index`; `symbols.NewIndex()`; `(*Index).AddDocument(name, root)`.
- `source`: `source.Span{Offset, Len int}` + `End() int`; `source.New(name string, content []byte) *SourceFile`; `(*SourceFile).Text(Span) string`; `(*SourceFile).Lines().PosAt(offset int) source.Pos{Line, Col int}` (1-based).
- `ast`: `ast.RootNamespace{...; Members []ast.Node}`; `ast.Node` interface getters only `Span()/LeadingTrivia()/TrailingTrivia()`.

### Task 1: Diagnostic model (Severity + Diagnostic)

**Files:**
- Create: `internal/core/passes/diagnostic.go`
- Test: `internal/core/passes/diagnostic_test.go`

- [ ] **Step 1: Write the failing test**

```go
package passes

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestSeverityString(t *testing.T) {
	cases := map[Severity]string{
		SeverityError:   "error",
		SeverityWarning: "warning",
		SeverityInfo:    "info",
		SeverityHint:    "hint",
		Severity(999):   "unknown",
	}
	for sev, want := range cases {
		if got := sev.String(); got != want {
			t.Errorf("Severity(%d).String() = %q, want %q", int(sev), got, want)
		}
	}
}

func TestDiagnosticFields(t *testing.T) {
	d := Diagnostic{
		Severity: SeverityError,
		Span:     source.Span{Offset: 3, Len: 5},
		Message:  "boom",
		Code:     "unresolved",
		Source:   "name-resolution",
	}
	if d.Severity != SeverityError || d.Span.Offset != 3 || d.Span.Len != 5 {
		t.Fatalf("unexpected diag: %+v", d)
	}
	if d.Message != "boom" || d.Code != "unresolved" || d.Source != "name-resolution" {
		t.Fatalf("unexpected diag: %+v", d)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/passes/ -run 'TestSeverity|TestDiagnostic' -v`
Expected: FAIL — `undefined: Severity`, `undefined: Diagnostic`.

- [ ] **Step 3: Write minimal implementation**

```go
package passes

import "github.com/Open-MBEE/Systemica/internal/core/source"

// Severity classifies the impact of a Diagnostic.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInfo
	SeverityHint
)

var severityNames = map[Severity]string{
	SeverityError:   "error",
	SeverityWarning: "warning",
	SeverityInfo:    "info",
	SeverityHint:    "hint",
}

// String returns the lowercase name of the severity, or "unknown".
func (s Severity) String() string {
	if name, ok := severityNames[s]; ok {
		return name
	}
	return "unknown"
}

// Diagnostic is a single validation finding produced by a pass.
type Diagnostic struct {
	Severity Severity
	Span     source.Span
	Message  string
	Code     string // stable identifier for filtering / future quick-fixes
	Source   string // the pass ID that produced this diagnostic
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/passes/ -run 'TestSeverity|TestDiagnostic' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/passes/diagnostic.go internal/core/passes/diagnostic_test.go
go vet ./internal/core/passes/
git add internal/core/passes/diagnostic.go internal/core/passes/diagnostic_test.go
git commit -m "feat(passes): add Severity and Diagnostic model"
```

### Task 2: Pass interface, PassLevel, Context

**Files:**
- Create: `internal/core/passes/pass.go`
- Test: `internal/core/passes/pass_test.go`

- [ ] **Step 1: Write the failing test**

```go
package passes

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func TestPassLevelString(t *testing.T) {
	cases := map[PassLevel]string{
		LevelSyntax:         "syntax",
		LevelNameResolution: "name-resolution",
		LevelType:           "type",
		LevelConstraint:     "constraint",
		PassLevel(999):      "unknown",
	}
	for lvl, want := range cases {
		if got := lvl.String(); got != want {
			t.Errorf("PassLevel(%d).String() = %q, want %q", int(lvl), got, want)
		}
	}
}

func TestPassLevelOrdering(t *testing.T) {
	if !(LevelSyntax < LevelNameResolution && LevelNameResolution < LevelType && LevelType < LevelConstraint) {
		t.Fatal("pass levels must be strictly increasing in dependency order")
	}
}

func TestNewContext(t *testing.T) {
	root := &ast.RootNamespace{}
	idx := symbols.NewIndexFromDoc("d.sysml", root)
	ctx := NewContext("d.sysml", idx, []Diagnostic{{Source: "syntax"}})
	if ctx.Name != "d.sysml" {
		t.Fatalf("Name = %q", ctx.Name)
	}
	if ctx.Index != idx {
		t.Fatal("Index not stored")
	}
	if len(ctx.ParseDiagnostics) != 1 || ctx.ParseDiagnostics[0].Source != "syntax" {
		t.Fatalf("ParseDiagnostics = %+v", ctx.ParseDiagnostics)
	}
	if ctx.Resolver() == nil {
		t.Fatal("Resolver() returned nil")
	}
	if ctx.Resolver() != ctx.Resolver() {
		t.Fatal("Resolver() must return the same shared instance")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/passes/ -run 'TestPassLevel|TestNewContext' -v`
Expected: FAIL — `undefined: PassLevel`, `undefined: NewContext`.

- [ ] **Step 3: Write minimal implementation**

```go
package passes

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// PassLevel is the dependency tier of a validation pass. Passes at a higher
// level are skipped when a lower level produced an error, avoiding cascade
// noise (e.g. no type-check on an unresolved reference).
type PassLevel int

const (
	LevelSyntax PassLevel = iota
	LevelNameResolution
	LevelType
	LevelConstraint
)

var passLevelNames = map[PassLevel]string{
	LevelSyntax:         "syntax",
	LevelNameResolution: "name-resolution",
	LevelType:           "type",
	LevelConstraint:     "constraint",
}

// String returns the lowercase name of the level, or "unknown".
func (l PassLevel) String() string {
	if name, ok := passLevelNames[l]; ok {
		return name
	}
	return "unknown"
}

// Pass is a single validation rule. Level reports its dependency tier; Run
// executes it over a whole document and returns any diagnostics found.
type Pass interface {
	Level() PassLevel
	Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic
}

// Context carries shared state made available to every pass in a run: the
// document name, the global symbol index, a lazily-created shared resolver,
// and the syntax diagnostics produced by the caller's parse (passes must not
// import the parser, so these enter here).
type Context struct {
	Name             string
	Index            *symbols.Index
	ParseDiagnostics []Diagnostic

	resolver *resolve.Resolver
}

// NewContext builds a Context for a document.
func NewContext(name string, idx *symbols.Index, parseDiags []Diagnostic) *Context {
	return &Context{Name: name, Index: idx, ParseDiagnostics: parseDiags}
}

// Resolver returns the shared resolver for this context, creating it on first
// use so multiple passes reuse one memoized instance.
func (c *Context) Resolver() *resolve.Resolver {
	if c.resolver == nil {
		c.resolver = resolve.New(c.Index)
	}
	return c.resolver
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/passes/ -run 'TestPassLevel|TestNewContext' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/passes/pass.go internal/core/passes/pass_test.go
go vet ./internal/core/passes/
git add internal/core/passes/pass.go internal/core/passes/pass_test.go
git commit -m "feat(passes): add Pass interface, PassLevel, and Context"
```

### Task 3: Registry + runner with prerequisite skipping

**Files:**
- Create: `internal/core/passes/registry.go`
- Test: `internal/core/passes/registry_test.go`

The registry holds passes and runs them in ascending `PassLevel` order. If any pass at a level produces an `Error`-severity diagnostic, passes at *strictly higher* levels are skipped (prerequisite failed → avoid cascade noise). Passes at the same level never skip each other.

- [ ] **Step 1: Write the failing test**

```go
package passes

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

type stubPass struct {
	level PassLevel
	diags []Diagnostic
}

func (s stubPass) Level() PassLevel { return s.level }

func (s stubPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	return s.diags
}

func TestRegistryRunsPassesInLevelOrder(t *testing.T) {
	reg := NewRegistry()
	reg.Register(stubPass{level: LevelNameResolution, diags: []Diagnostic{{Source: "b"}}})
	reg.Register(stubPass{level: LevelSyntax, diags: []Diagnostic{{Source: "a"}}})
	got := reg.Run(NewContext("t", nil, nil), "t", nil)
	if len(got) != 2 {
		t.Fatalf("got %d diagnostics, want 2", len(got))
	}
	if got[0].Source != "a" || got[1].Source != "b" {
		t.Fatalf("passes ran out of level order: %q then %q", got[0].Source, got[1].Source)
	}
}

func TestRegistrySkipsHigherLevelAfterError(t *testing.T) {
	reg := NewRegistry()
	reg.Register(stubPass{level: LevelSyntax, diags: []Diagnostic{{Severity: SeverityError, Source: "syntax"}}})
	reg.Register(stubPass{level: LevelType, diags: []Diagnostic{{Source: "type"}}})
	got := reg.Run(NewContext("t", nil, nil), "t", nil)
	if len(got) != 1 {
		t.Fatalf("got %d diagnostics, want 1 (type pass must be skipped)", len(got))
	}
	if got[0].Source != "syntax" {
		t.Fatalf("got source %q, want syntax", got[0].Source)
	}
}

func TestRegistrySameLevelNeverSkips(t *testing.T) {
	reg := NewRegistry()
	reg.Register(stubPass{level: LevelNameResolution, diags: []Diagnostic{{Severity: SeverityError, Source: "a"}}})
	reg.Register(stubPass{level: LevelNameResolution, diags: []Diagnostic{{Source: "b"}}})
	got := reg.Run(NewContext("t", nil, nil), "t", nil)
	if len(got) != 2 {
		t.Fatalf("got %d diagnostics, want 2 (same-level passes never skip)", len(got))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/passes/ -run 'TestRegistry' -v`
Expected: FAIL — `undefined: NewRegistry`.

- [ ] **Step 3: Write minimal implementation**

```go
package passes

import (
	"sort"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

// Registry holds an ordered set of validation passes.
type Registry struct {
	passes []Pass
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a pass to the registry.
func (r *Registry) Register(p Pass) {
	r.passes = append(r.passes, p)
}

// Run executes all registered passes in ascending PassLevel order and returns
// their accumulated diagnostics. If a pass at some level emits an
// Error-severity diagnostic, passes at strictly higher levels are skipped to
// avoid cascade noise. Passes at the same level always run.
func (r *Registry) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	ordered := make([]Pass, len(r.passes))
	copy(ordered, r.passes)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Level() < ordered[j].Level()
	})

	var all []Diagnostic
	// failedBelow is the lowest level at which an Error occurred; passes at a
	// strictly higher level are skipped.
	failed := false
	var failedLevel PassLevel
	for _, p := range ordered {
		if failed && p.Level() > failedLevel {
			continue
		}
		diags := p.Run(ctx, name, root)
		all = append(all, diags...)
		if hasError(diags) {
			if !failed || p.Level() < failedLevel {
				failedLevel = p.Level()
			}
			failed = true
		}
	}
	return all
}

func hasError(diags []Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/passes/ -run 'TestRegistry' -v`
Expected: PASS (all three tests).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/passes/registry.go internal/core/passes/registry_test.go
go vet ./internal/core/passes/
git add internal/core/passes/registry.go internal/core/passes/registry_test.go
git commit -m "feat(passes): add ordered registry with prerequisite skipping"
```

### Task 4: SyntaxPass

**Files:**
- Create: `internal/core/passes/syntax.go`
- Test: `internal/core/passes/syntax_test.go`

The `passes` package must not import `parser`. Parser (syntax) diagnostics are produced by the caller (who ran the parser), adapted to `passes.Diagnostic`, and handed to the `Context` via `ParseDiagnostics`. `SyntaxPass` simply surfaces those pre-stamped diagnostics at `LevelSyntax` so the registry treats parse errors as a prerequisite that can gate higher-level passes.

- [ ] **Step 1: Write the failing test**

```go
package passes

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestSyntaxPassSurfacesParseDiagnostics(t *testing.T) {
	parseDiags := []Diagnostic{
		{Severity: SeverityError, Span: source.Span{Offset: 3, Len: 2}, Message: "expected '}'", Code: "syntax", Source: "syntax"},
	}
	ctx := NewContext("t", nil, parseDiags)
	p := SyntaxPass{}
	if p.Level() != LevelSyntax {
		t.Fatalf("Level() = %v, want LevelSyntax", p.Level())
	}
	got := p.Run(ctx, "t", nil)
	if len(got) != 1 || got[0].Message != "expected '}'" || got[0].Source != "syntax" {
		t.Fatalf("got %+v, want the single parse diagnostic", got)
	}
}

func TestSyntaxPassEmptyWhenNoParseDiagnostics(t *testing.T) {
	got := SyntaxPass{}.Run(NewContext("t", nil, nil), "t", nil)
	if len(got) != 0 {
		t.Fatalf("got %d diagnostics, want 0", len(got))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/passes/ -run 'TestSyntaxPass' -v`
Expected: FAIL — `undefined: SyntaxPass`.

- [ ] **Step 3: Write minimal implementation**

```go
package passes

import "github.com/Open-MBEE/Systemica/internal/core/ast"

// SyntaxPass surfaces the parser-produced diagnostics carried on the Context.
// The passes package does not import parser; the caller adapts parser
// diagnostics into passes.Diagnostic and stores them on Context.ParseDiagnostics.
type SyntaxPass struct{}

// Level reports that syntax is the lowest dependency level.
func (SyntaxPass) Level() PassLevel { return LevelSyntax }

// Run returns the parse diagnostics carried on the context.
func (SyntaxPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || len(ctx.ParseDiagnostics) == 0 {
		return nil
	}
	out := make([]Diagnostic, len(ctx.ParseDiagnostics))
	copy(out, ctx.ParseDiagnostics)
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/passes/ -run 'TestSyntaxPass' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/passes/syntax.go internal/core/passes/syntax_test.go
go vet ./internal/core/passes/
git add internal/core/passes/syntax.go internal/core/passes/syntax_test.go
git commit -m "feat(passes): add SyntaxPass surfacing parser diagnostics"
```

### Task 5: NameResolutionPass

**Files:**
- Create: `internal/core/passes/nameres.go`
- Test: `internal/core/passes/nameres_test.go`

`NameResolutionPass` runs the Plan-3 resolver over the document and adapts each `resolve.Diagnostic` into a `passes.Diagnostic`. The stable `Code` is derived from the resolver message prefix: `"ambiguous reference: ..."` → `"ambiguous"`, everything else → `"unresolved"`. `Source` is `"name-resolution"`, severity `Error`.

- [ ] **Step 1: Write the failing test**

```go
package passes

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func nameresCtx(t *testing.T, name, src string) (*Context, *ast.RootNamespace) {
	t.Helper()
	sf := source.New(name, []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc(name, root)
	return NewContext(name, idx, nil), root
}

func TestNameResolutionPassReportsUnresolved(t *testing.T) {
	ctx, root := nameresCtx(t, "a.sysml", "package P { alias A for P::Missing; }")
	got := NameResolutionPass{}.Run(ctx, "a.sysml", root)
	if len(got) == 0 {
		t.Fatalf("expected an unresolved diagnostic, got none")
	}
	d := got[0]
	if d.Source != "name-resolution" || d.Code != "unresolved" || d.Severity != SeverityError {
		t.Fatalf("got %+v, want source=name-resolution code=unresolved severity=error", d)
	}
}

func TestNameResolutionPassCleanWhenAllResolve(t *testing.T) {
	ctx, root := nameresCtx(t, "a.sysml", "package P { namespace N; alias A for P::N; }")
	got := NameResolutionPass{}.Run(ctx, "a.sysml", root)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

func TestNameResolutionPassLevel(t *testing.T) {
	if NameResolutionPass{}.Level() != LevelNameResolution {
		t.Fatalf("Level() = %v, want LevelNameResolution", NameResolutionPass{}.Level())
	}
}
```

Add the `ast` import to the test file (used by `nameresCtx` return type):

```go
	"github.com/Open-MBEE/Systemica/internal/core/ast"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/passes/ -run 'TestNameResolutionPass' -v`
Expected: FAIL — `undefined: NameResolutionPass`.

- [ ] **Step 3: Write minimal implementation**

```go
package passes

import (
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

// NameResolutionPass resolves every reference in a document via the Plan-3
// resolver and reports unresolved / ambiguous references.
type NameResolutionPass struct{}

// Level reports the name-resolution dependency level.
func (NameResolutionPass) Level() PassLevel { return LevelNameResolution }

// Run resolves the document and adapts resolver diagnostics.
func (NameResolutionPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	r := ctx.Resolver()
	r.ResolveDocument(name, root)
	rd := r.Diagnostics
	if len(rd) == 0 {
		return nil
	}
	out := make([]Diagnostic, 0, len(rd))
	for _, d := range rd {
		code := "unresolved"
		if strings.HasPrefix(d.Message, "ambiguous") {
			code = "ambiguous"
		}
		out = append(out, Diagnostic{
			Severity: SeverityError,
			Span:     d.Span,
			Message:  d.Message,
			Code:     code,
			Source:   "name-resolution",
		})
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/passes/ -run 'TestNameResolutionPass' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/passes/nameres.go internal/core/passes/nameres_test.go
go vet ./internal/core/passes/
git add internal/core/passes/nameres.go internal/core/passes/nameres_test.go
git commit -m "feat(passes): add NameResolutionPass adapting resolver diagnostics"
```

### Task 6: Analyze entry point (default registry over a document)

**Files:**
- Create: `internal/core/passes/analyze.go`
- Test: `internal/core/passes/analyze_test.go`

`DefaultRegistry()` registers `SyntaxPass` + `NameResolutionPass`. `Analyze` builds a `Context`, runs the default registry, and returns diagnostics sorted deterministically by `Span.Offset`, then `Source`, then `Message`.

- [ ] **Step 1: Write the failing test**

```go
package passes

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func analyzeInputs(t *testing.T, name, src string) (*ast.RootNamespace, []Diagnostic, *symbols.Index) {
	t.Helper()
	sf := source.New(name, []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	parseDiags := make([]Diagnostic, 0, len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		parseDiags = append(parseDiags, Diagnostic{
			Severity: SeverityError, Span: d.Span, Message: d.Message,
			Code: "syntax", Source: "syntax",
		})
	}
	idx := symbols.NewIndexFromDoc(name, root)
	return root, parseDiags, idx
}

func TestAnalyzeCleanDocument(t *testing.T) {
	root, pd, idx := analyzeInputs(t, "a.sysml", "package P { namespace N; alias A for P::N; }")
	got := Analyze("a.sysml", root, pd, idx)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

func TestAnalyzeReportsNameResolution(t *testing.T) {
	root, pd, idx := analyzeInputs(t, "a.sysml", "package P { alias A for P::Missing; }")
	got := Analyze("a.sysml", root, pd, idx)
	if len(got) != 1 || got[0].Source != "name-resolution" {
		t.Fatalf("got %+v, want one name-resolution diagnostic", got)
	}
}

func TestAnalyzeSortsByOffset(t *testing.T) {
	root, pd, idx := analyzeInputs(t, "a.sysml", "package P { alias A for P::X; alias B for P::Y; }")
	got := Analyze("a.sysml", root, pd, idx)
	if len(got) != 2 {
		t.Fatalf("got %d diagnostics, want 2", len(got))
	}
	if got[0].Span.Offset > got[1].Span.Offset {
		t.Fatalf("diagnostics not sorted by offset: %d then %d", got[0].Span.Offset, got[1].Span.Offset)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/passes/ -run 'TestAnalyze' -v`
Expected: FAIL — `undefined: Analyze`.

- [ ] **Step 3: Write minimal implementation**

```go
package passes

import (
	"sort"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// DefaultRegistry returns the v1 pass registry: syntax + name resolution.
func DefaultRegistry() *Registry {
	reg := NewRegistry()
	reg.Register(SyntaxPass{})
	reg.Register(NameResolutionPass{})
	return reg
}

// Analyze runs the default validation passes over a single parsed document and
// returns diagnostics sorted by span offset, then source, then message.
// parseDiags are the parser's diagnostics, already adapted to passes.Diagnostic
// by the caller.
func Analyze(name string, root *ast.RootNamespace, parseDiags []Diagnostic, idx *symbols.Index) []Diagnostic {
	ctx := NewContext(name, idx, parseDiags)
	diags := DefaultRegistry().Run(ctx, name, root)
	sort.SliceStable(diags, func(i, j int) bool {
		a, b := diags[i], diags[j]
		if a.Span.Offset != b.Span.Offset {
			return a.Span.Offset < b.Span.Offset
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return a.Message < b.Message
	})
	return diags
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/passes/ -run 'TestAnalyze' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/passes/analyze.go internal/core/passes/analyze_test.go
go vet ./internal/core/passes/
git add internal/core/passes/analyze.go internal/core/passes/analyze_test.go
git commit -m "feat(passes): add Analyze entry point and default registry"
```

### Task 7: Integration golden tests over fixtures

**Files:**
- Create: `internal/core/passes/integration_test.go`
- Create: `testdata/passes/clean.sysml`, `testdata/passes/clean.golden`
- Create: `testdata/passes/errors.sysml`, `testdata/passes/errors.golden`

End-to-end golden test: parse a fixture, adapt parser diagnostics, build the symbol index, run `Analyze`, serialize the sorted diagnostics as `line:col severity [source/code] message` lines, compare against the golden file (regenerate with `-update`).

- [ ] **Step 1: Write the failing test**

```go
package passes

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func runPassesGolden(t *testing.T, name string) {
	t.Helper()
	srcPath := filepath.Join("..", "..", "..", "testdata", "passes", name+".sysml")
	data, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	sf := source.New(name+".sysml", data)
	p := parser.New(sf)
	root := p.ParseFile()

	parseDiags := make([]Diagnostic, 0, len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		parseDiags = append(parseDiags, Diagnostic{
			Severity: SeverityError, Span: d.Span, Message: d.Message,
			Code: "syntax", Source: "syntax",
		})
	}
	idx := symbols.NewIndexFromDoc(name+".sysml", root)
	diags := Analyze(name+".sysml", root, parseDiags, idx)

	var b strings.Builder
	if len(diags) == 0 {
		b.WriteString("(no diagnostics)\n")
	} else {
		lines := sf.Lines()
		for _, d := range diags {
			pos := lines.PosAt(d.Span.Offset)
			fmt.Fprintf(&b, "%d:%d %s [%s/%s] %s\n", pos.Line, pos.Col, d.Severity, d.Source, d.Code, d.Message)
		}
	}
	got := b.String()

	goldenPath := filepath.Join("..", "..", "..", "testdata", "passes", name+".golden")
	if *updateGolden {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Fatalf("golden mismatch for %s:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestPassesGoldenClean(t *testing.T)  { runPassesGolden(t, "clean") }
func TestPassesGoldenErrors(t *testing.T) { runPassesGolden(t, "errors") }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/passes/ -run 'TestPassesGolden' -v`
Expected: FAIL — fixtures/goldens do not exist yet (`read fixture` error).

- [ ] **Step 3: Create fixtures**

`testdata/passes/clean.sysml` (all references resolve → no diagnostics):

```
package Lib {
	public namespace Widgets;
	namespace Gadgets;
}

package App {
	import Lib::*;
	alias W for Lib::Widgets;
	dependency from App to Lib;
}
```

`testdata/passes/errors.sysml` (unresolved references, no parse errors):

```
package P {
	alias Bad for P::Missing;
	filter Undefined;
	import Nowhere::*;
}
```

- [ ] **Step 4: Generate and inspect goldens, then verify**

Run: `go test ./internal/core/passes/ -run 'TestPassesGolden' -update`

Then inspect: `testdata/passes/clean.golden` MUST be exactly `(no diagnostics)`. If it is not, a reference that should resolve is failing — fix the cause, do not accept a dirty clean golden. `testdata/passes/errors.golden` MUST list the three unresolved references (`P::Missing`, `Undefined`, `Nowhere`) sorted by offset, each formatted `line:col error [name-resolution/unresolved] unresolved reference: <qn>`, with no spurious extras.

Run: `go test ./internal/core/passes/ -run 'TestPassesGolden' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
go build ./... && go test ./... -count=1
go vet ./...
git add internal/core/passes/integration_test.go testdata/passes/
git commit -m "test(passes): add golden integration tests for unified diagnostics"
```

## Self-Review

**Spec coverage (spec §8):**

| Spec §8 requirement | Task |
|---|---|
| Pluggable rule passes over resolved model | Task 2 (Pass interface), Task 3 (registry) |
| Pass interface `Run(ctx, node) []Diagnostic`, stateless, ctx gives resolver+index | Task 2 (whole-document Run; Context.Resolver()/Index) |
| Ordered registry tagged by dependency level | Task 2 (PassLevel), Task 3 (level-ordered Run) |
| Runner skips passes whose prerequisites failed | Task 3 (skip higher levels after Error) |
| Diagnostic model: severity, span, message, stable code, source pass ID | Task 1 (Diagnostic) |
| v1 syntax pass (parser error nodes → diagnostics) | Task 4 (SyntaxPass) |
| v1 name-resolution pass (unresolved/ambiguous/visibility) | Task 5 (NameResolutionPass) |
| Unified analysis entry producing sorted diagnostics | Task 6 (Analyze) |
| End-to-end validation | Task 7 (golden) |

**Deferred (roadmap, explicitly NOT this plan):** type/multiplicity/redefinition/specialization-cycle/full-constraint passes (drop in as registry entries at LevelType/LevelConstraint — infrastructure already present, exercised only by `TestRegistrySkipsHigherLevelAfterError`); per-document diagnostic caching + incremental recompute (Plan 5, Workspace); per-node lazy invocation (Plan 6, LSP); quick-fix actions keyed off `Code`. Lazy per-node invocation is spec §8 point 3 but is a consumer concern (LSP/REPL decide when to call `Analyze`); v1 provides whole-document `Analyze`.

**Placeholder scan:** No `TBD`/`TODO`/`<FILL>` remain in tasks; all code blocks complete.

**Type consistency:** `Diagnostic` fields (Severity/Span/Message/Code/Source) identical across all tasks. `Pass` interface (`Level() PassLevel`, `Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic`) matched by `SyntaxPass`, `NameResolutionPass`, and test `stubPass`. `Context` (`Name`, `Index *symbols.Index`, `ParseDiagnostics []Diagnostic`, `Resolver()`) consistent Task 2 → 4/5/6. `Analyze` signature `(name string, root *ast.RootNamespace, parseDiags []Diagnostic, idx *symbols.Index) []Diagnostic` used identically in Task 6 tests and Task 7 integration harness. Consumed Plan-1/2/3 APIs verified on disk: `parser.Parser.Diagnostics []parser.Diagnostic{Span,Message}`, `resolve.Resolver.Diagnostics []resolve.Diagnostic{Span,Message}` via `ResolveDocument(name, root)`, `symbols.NewIndexFromDoc(name, root)`, `source.SourceFile.Lines().PosAt(offset)`.

**Boundary note:** `passes` imports `{ast, source, symbols, resolve}` only — NOT `parser` (syntax diagnostics enter via `Context`). Tests import `parser` to build ASTs. No import cycle.

**Execution order:** Task 1 (Diagnostic) → 2 (Pass/Context) → 3 (Registry) → 4 (SyntaxPass) → 5 (NameResolutionPass) → 6 (Analyze) → 7 (golden). Tasks 3–6 depend on 1–2; Task 7 depends on all. Apply in order.
