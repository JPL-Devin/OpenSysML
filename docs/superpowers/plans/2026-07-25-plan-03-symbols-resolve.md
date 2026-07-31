# Symbols & Name Resolution Implementation Plan (Plan 3)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a per-document scope tree, a global qualified-name index, and a lazy name resolver over the currently-parsed grammar (packages/namespaces/members/imports/aliases + expressions), producing definition targets and unresolved/ambiguity diagnostics.

**Architecture:** Two new packages. `internal/core/symbols` walks an `ast.RootNamespace` to build an immutable per-doc scope tree (scopes hold members keyed by short+full name) plus a global map from fully-qualified name to declaration node(s). `internal/core/resolve` performs lazy lookup: qualified names walk from a root scope segment-by-segment; unqualified names search outward through enclosing scopes, then imports, then the global root. Results memoize in side tables keyed by the reference node. Inheritance/specialization is out of scope (deferred until the def/usage taxonomy is parsed).

**Tech Stack:** Go 1.25, standard library only. Consumes `internal/core/ast`, `internal/core/source`, `internal/core/parser`.

---

## Scope

**In scope (Plan 3):**

- **Scope tree.** For a parsed `*ast.RootNamespace`, build an immutable tree of `Scope` nodes mirroring namespace nesting: root namespace -> packages -> nested namespaces -> members. Each scope owns a table of locally-declared `Symbol`s keyed by both short name (`<x>`) and primary name.
- **Global qualified-name index.** A map from fully-qualified name (e.g. `A::B::C`) to the declaring `Symbol`(s). Built eagerly by walking the scope tree once. Multiple declarations of the same qualified name are retained (drives ambiguity diagnostics).
- **Qualified-name resolution.** `A::B::C` resolves by walking from the appropriate root scope segment-by-segment: resolve `A` as a member of the root, then `B` as a member of `A`'s scope, then `C` as a member of `B`'s scope. `$::A::B` (global qualification) always starts at the document root.
- **Unqualified resolution.** A bare name resolves by searching outward: the innermost enclosing scope first, then each ancestor scope, then names brought in by `import`s visible at that point, then the global root. First match wins.
- **Import resolution.** Membership import (`import A::B;`) brings the single named member into the importing scope. Namespace import (`import A::B::*;`) brings all members of `A::B` into scope. Recursive membership import (`import A::B::**;`) and recursive namespace import (`import A::*::**;`) bring members transitively through nested namespaces. `import all` widens visibility filtering (see below).
- **Alias resolution.** An `alias X for A::B;` declares name `X` in its enclosing scope that resolves to whatever `A::B` resolves to. Alias chains resolve transitively with a cycle guard.
- **Visibility filtering.** Members declared `private` are not visible to importers/qualified lookups from outside their owning namespace; `protected`/`public`/default handled per SysML rules (default = public for package members). `import all` re-exports otherwise-private members.
- **Diagnostics.** Unresolved reference -> error diagnostic at the reference span. Multiple equally-valid targets -> ambiguity diagnostic. Import of a nonexistent name -> diagnostic on the import.
- **Memoization.** Resolution results are memoized in side tables keyed by the reference AST node (`ast.Node` -> resolved `*Symbol` or unresolved marker). The side table is per-`Resolver`; a `Resolver` is constructed for a given index snapshot and discarded on reparse (invalidation = drop the resolver).

**Explicitly out of scope (deferred to later plans):**

- Inheritance-aware lookup and specialization/redefinition edges (`:>`, `:`, `subsets`, `redefines`). The def/usage taxonomy (`part`/`attribute`/`def`/`feature`/`connection`/etc.) is not yet parsed, so there are no supertypes to walk. Deferred until a grammar-extension plan lands.
- Cross-project dependency and bundled-stdlib indexing (Plan 5). Plan 3 indexes only the documents handed to it (workspace docs).
- Type checking, multiplicity, constraint validation (Plan 4+).
- Expression-internal name resolution beyond `FeatureReference`/`QualifiedName` leaf references reachable from the current grammar. Full expression semantics deferred.

## File Structure

New package `internal/core/symbols`:

- `symbol.go` — `SymbolKind` enum + `Symbol` type (name, kind, decl node, owning scope, visibility, span). One responsibility: the value object describing a declared name.
- `scope.go` — `Scope` type (parent pointer, owning node, member tables keyed by short/primary name, ordered child scopes) + local-lookup methods. One responsibility: the tree structure and local member access.
- `builder.go` — `Build(root *ast.RootNamespace) *Scope` walks the AST producing the scope tree. One responsibility: AST -> scope tree translation.
- `index.go` — `Index` type: global map from fully-qualified name to `[]*Symbol`, plus the per-doc root scope(s). `NewIndex()` / `Index.AddDocument(name, root)` / accessors. One responsibility: the global qualified-name lookup structure.
- `*_test.go` — unit tests per file.

New package `internal/core/resolve`:

- `diagnostic.go` — `Diagnostic{Span source.Span; Message string}` (mirrors parser.Diagnostic; kept local to avoid a parser import cycle) + severity if needed.
- `resolver.go` — `Resolver` type holding the `*symbols.Index`, the memo side tables, and accumulated `Diagnostics`. `New(idx *symbols.Index) *Resolver`. Entry points `ResolveQualified(scope *symbols.Scope, qn *ast.QualifiedName) (*symbols.Symbol, bool)` and `ResolveName(scope *symbols.Scope, name string, at ast.Node) (*symbols.Symbol, bool)`. One responsibility: orchestration + memo + diagnostics.
- `qualified.go` — qualified-name walk-from-root logic.
- `unqualified.go` — outward scope search logic.
- `imports.go` — import expansion (membership/namespace/recursive) contributing names to a scope's visible set.
- `*_test.go` — unit tests per file.

Test fixtures: `testdata/resolve/*.sysml` (+ golden expectation files where a golden approach is used).

Dependency direction (no cycles): `resolve` -> `symbols` -> `ast` -> `source`. `resolve` also imports `ast`/`source`. Neither new package imports `parser`; tests may import `parser` to produce ASTs.

## Grammar / AST Reference

The scope builder walks `*ast.RootNamespace` produced by `parser.New(sf).ParseFile()`. Relevant node shapes (package `internal/core/ast`, all embed `NodeBase` which provides `Span()`):

- `RootNamespace{ Members []Node }` — top level. `Members` entries are `*Membership`, `*Import`, `*Alias`, or `*ErrorNode`.
- `Membership{ Visibility Visibility; Member Node }` — wraps a declaration. `Member` is `*Package`, `*Namespace`, `*Dependency`, `*Comment`, `*Documentation`, `*TextualRepresentation`, or `*FilterMember`.
- `Package{ Prefixes []*PrefixMetadata; Ident Identification; IsLibrary, IsStandard bool; Members []Node; HasBody bool }` — introduces a named scope; `Members` same shape as `RootNamespace.Members`.
- `Namespace{ Prefixes []*PrefixMetadata; Ident Identification; Members []Node; HasBody bool }` — introduces a named scope.
- `Identification{ ShortName string; ShortNameSpan source.Span; Name string; NameSpan source.Span }` — a declaration's names. Either may be `""`. A `Symbol` registers under both non-empty names.
- `Import{ Visibility Visibility; IsAll bool; Kind ImportKind; Imported *QualifiedName; IsRecursive bool; Body []Node; HasBody bool }`. `Kind` is `ImportMembership` or `ImportNamespace`. Corrected wildcard semantics: `A::B::**` (no `::*`) => `ImportMembership`+`IsRecursive`; `A::B::*` => `ImportNamespace`; `A::*::**` => `ImportNamespace`+`IsRecursive`.
- `Alias{ Visibility Visibility; Ident Identification; For *QualifiedName; ... }` — `Ident` names the alias, `For` is the target.
- `QualifiedName{ Global bool; Parts []NameSegment }`; `NameSegment{ Text string; Span source.Span }`. `Global==true` means a leading `$::` (resolve from document root). `Parts[i].Text` keeps quotes for unrestricted (single-quoted) names — treat the raw `Text` as the segment key (do NOT strip quotes; declarations from `Identification.Name` also keep the same raw form as produced by the parser, so keys match).
- `Visibility` enum: `VisibilityDefault`, `VisibilityPublic`, `VisibilityPrivate`, `VisibilityProtected`. For package/namespace members, `VisibilityDefault` is treated as public.
- `Dependency`, `Comment`, `Documentation`, `TextualRepresentation`, `FilterMember` do not introduce reusable named scopes for resolution, but `Dependency`/`Comment`/`Documentation`/`TextualRepresentation` MAY carry an `Ident` — register a `Symbol` when `Ident` has a name so the name is discoverable, but they own no child scope (empty member set). `FilterMember` has no name.

Name-key convention (used everywhere): a declaration contributes up to two keys — `Ident.ShortName` and `Ident.Name` — each non-empty one mapping to the same `*Symbol`. A qualified-name segment or unqualified reference matches a `Symbol` if its raw `Text` equals either registered key.

Reference nodes to resolve in Plan 3: `*ast.QualifiedName` (in `Import.Imported`, `Alias.For`, `Dependency.Clients`/`Suppliers`, `PrefixMetadata.Type`, and inside expressions via `FeatureReference.Name` and `OperatorExpr.TypeRef`) and `*ast.FeatureReference`.

### Task 1: Symbol kinds and Symbol type

**Files:**
- Create: `internal/core/symbols/symbol.go`
- Test: `internal/core/symbols/symbol_test.go`

- [ ] **Step 1: Write the failing test**

```go
package symbols

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestSymbolKindString(t *testing.T) {
	if SymbolPackage.String() != "package" {
		t.Fatalf("SymbolPackage.String() = %q, want %q", SymbolPackage.String(), "package")
	}
	if SymbolNamespace.String() != "namespace" {
		t.Fatalf("SymbolNamespace.String() = %q, want %q", SymbolNamespace.String(), "namespace")
	}
	if SymbolAlias.String() != "alias" {
		t.Fatalf("SymbolAlias.String() = %q, want %q", SymbolAlias.String(), "alias")
	}
}

func TestSymbolFields(t *testing.T) {
	pkg := &ast.Package{}
	pkg.NodeSpan = source.Span{Offset: 3, Len: 7}
	s := &Symbol{
		Name:       "P",
		Kind:       SymbolPackage,
		Decl:       pkg,
		Visibility: ast.VisibilityPublic,
		DeclSpan:   source.Span{Offset: 3, Len: 7},
	}
	if s.Name != "P" || s.Kind != SymbolPackage {
		t.Fatalf("unexpected symbol fields: %+v", s)
	}
	if s.DeclSpan.Offset != 3 || s.DeclSpan.Len != 7 {
		t.Fatalf("unexpected DeclSpan: %+v", s.DeclSpan)
	}
	var _ ast.Node = s.Decl
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/symbols/ -run 'TestSymbolKindString|TestSymbolFields' -v`
Expected: FAIL — `undefined: SymbolPackage`, `undefined: Symbol`.

- [ ] **Step 3: Write minimal implementation**

```go
// Package symbols builds an immutable per-document scope tree and a global
// qualified-name index over a parsed ast.RootNamespace.
package symbols

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// SymbolKind classifies a declared name.
type SymbolKind int

const (
	SymbolUnknown SymbolKind = iota
	SymbolPackage
	SymbolNamespace
	SymbolAlias
	SymbolDependency
	SymbolComment
	SymbolDocumentation
	SymbolTextualRepresentation
)

var symbolKindNames = map[SymbolKind]string{
	SymbolUnknown:               "unknown",
	SymbolPackage:               "package",
	SymbolNamespace:             "namespace",
	SymbolAlias:                 "alias",
	SymbolDependency:            "dependency",
	SymbolComment:               "comment",
	SymbolDocumentation:         "documentation",
	SymbolTextualRepresentation: "textualRepresentation",
}

// String returns the display name of the kind.
func (k SymbolKind) String() string {
	if s, ok := symbolKindNames[k]; ok {
		return s
	}
	return "unknown"
}

// Symbol describes one declared name. The same declaration may be reachable
// through more than one Symbol only when it declares both a short and a
// primary name; in that case a single Symbol is registered under both keys.
type Symbol struct {
	Name       string         // the key this Symbol was primarily created for
	Kind       SymbolKind     // classification
	Decl       ast.Node       // the declaring AST node
	Visibility ast.Visibility // declared visibility
	DeclSpan   source.Span    // span of the declaration (for diagnostics)
	Scope      *Scope         // the child scope this declaration owns, or nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/symbols/ -run 'TestSymbolKindString|TestSymbolFields' -v`
Expected: PASS. (`Scope` is referenced as a field type; it is defined in Task 2. To keep this task compiling on its own, also add the minimal `type Scope struct{}` placeholder at the bottom of `symbol.go` — Task 2 will move/replace it into `scope.go`. If preferred, implement Task 1 and Task 2 in one commit; the plan keeps them separate for review granularity but they may be squashed.)

Note for the implementer: because `Symbol.Scope *Scope` references a type defined in Task 2, add this temporary declaration to the end of `symbol.go` so Task 1 compiles standalone:

```go
// Scope is defined in scope.go (Task 2). Temporary forward declaration removed there.
type Scope struct{}
```

When implementing Task 2, DELETE this placeholder from `symbol.go` and define the real `Scope` in `scope.go`.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/symbols/symbol.go internal/core/symbols/symbol_test.go
git add internal/core/symbols/symbol.go internal/core/symbols/symbol_test.go
git commit -m "feat(symbols): add SymbolKind and Symbol types"
```

### Task 2: Scope tree types

**Files:**
- Create: `internal/core/symbols/scope.go`
- Modify: `internal/core/symbols/symbol.go` (delete the temporary `type Scope struct{}` placeholder from Task 1)
- Test: `internal/core/symbols/scope_test.go`

- [ ] **Step 1: Write the failing test**

```go
package symbols

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

func TestScopeDefineAndLookupLocal(t *testing.T) {
	root := NewScope(nil, nil)
	sym := &Symbol{Name: "P", Kind: SymbolPackage}
	root.Define("P", sym)

	got, ok := root.LookupLocal("P")
	if !ok || got != sym {
		t.Fatalf("LookupLocal(P) = %v, %v; want the defined symbol", got, ok)
	}
	if _, ok := root.LookupLocal("Q"); ok {
		t.Fatalf("LookupLocal(Q) unexpectedly found a symbol")
	}
}

func TestScopeShortAndPrimaryName(t *testing.T) {
	root := NewScope(nil, nil)
	sym := &Symbol{Name: "Vehicle", Kind: SymbolPackage}
	root.Define("v", sym)       // short name
	root.Define("Vehicle", sym) // primary name

	for _, key := range []string{"v", "Vehicle"} {
		got, ok := root.LookupLocal(key)
		if !ok || got != sym {
			t.Fatalf("LookupLocal(%q) = %v, %v; want the symbol", key, got, ok)
		}
	}
}

func TestScopeParentAndChildren(t *testing.T) {
	root := NewScope(nil, nil)
	pkgNode := &ast.Package{}
	child := NewScope(root, pkgNode)
	root.AddChild(child)

	if child.Parent() != root {
		t.Fatalf("child.Parent() != root")
	}
	if len(root.Children()) != 1 || root.Children()[0] != child {
		t.Fatalf("root.Children() = %v; want [child]", root.Children())
	}
	if child.Node() != pkgNode {
		t.Fatalf("child.Node() != pkgNode")
	}
}

func TestScopeDefineDuplicateKeeps All(t *testing.T) {
	root := NewScope(nil, nil)
	a := &Symbol{Name: "X", Kind: SymbolPackage}
	b := &Symbol{Name: "X", Kind: SymbolNamespace}
	root.Define("X", a)
	root.Define("X", b)

	all := root.LookupLocalAll("X")
	if len(all) != 2 {
		t.Fatalf("LookupLocalAll(X) len = %d, want 2", len(all))
	}
	// LookupLocal returns the first-defined symbol.
	got, ok := root.LookupLocal("X")
	if !ok || got != a {
		t.Fatalf("LookupLocal(X) = %v; want first-defined a", got)
	}
}
```

(Note: rename the last test function to `TestScopeDefineDuplicateKeepsAll` — no space — when writing the file. The space above is a typo guard; use the valid identifier.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/symbols/ -run 'TestScope' -v`
Expected: FAIL — `undefined: NewScope`.

- [ ] **Step 3: Write minimal implementation**

First DELETE the temporary `type Scope struct{}` placeholder at the bottom of `symbol.go` (added in Task 1). Then create `scope.go`:

```go
package symbols

import "github.com/Open-MBEE/Systemica/internal/core/ast"

// Scope is a node in the immutable per-document scope tree. It owns the
// symbols declared directly within a namespace-like construct and links to
// its parent and child scopes.
type Scope struct {
	parent   *Scope
	node     ast.Node            // the owning declaration node (nil for the doc root)
	members  map[string][]*Symbol // name key -> symbols defined under that key (in definition order)
	children []*Scope
}

// NewScope creates an empty scope with the given parent and owning node.
func NewScope(parent *Scope, node ast.Node) *Scope {
	return &Scope{
		parent:  parent,
		node:    node,
		members: make(map[string][]*Symbol),
	}
}

// Parent returns the enclosing scope, or nil for the document root.
func (s *Scope) Parent() *Scope { return s.parent }

// Node returns the AST node that owns this scope, or nil for the document root.
func (s *Scope) Node() ast.Node { return s.node }

// Children returns the child scopes in definition order.
func (s *Scope) Children() []*Scope { return s.children }

// AddChild appends a child scope.
func (s *Scope) AddChild(c *Scope) { s.children = append(s.children, c) }

// Define registers sym under the given name key. Multiple symbols may share a
// key (duplicate declarations); all are retained in definition order.
func (s *Scope) Define(name string, sym *Symbol) {
	if name == "" {
		return
	}
	s.members[name] = append(s.members[name], sym)
}

// LookupLocal returns the first symbol defined under name in this scope only.
func (s *Scope) LookupLocal(name string) (*Symbol, bool) {
	syms := s.members[name]
	if len(syms) == 0 {
		return nil, false
	}
	return syms[0], true
}

// LookupLocalAll returns every symbol defined under name in this scope only.
func (s *Scope) LookupLocalAll(name string) []*Symbol {
	return s.members[name]
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/symbols/ -run 'TestScope|TestSymbol' -v`
Expected: PASS (all Task 1 + Task 2 tests).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/symbols/scope.go internal/core/symbols/scope_test.go internal/core/symbols/symbol.go
git add internal/core/symbols/scope.go internal/core/symbols/scope_test.go internal/core/symbols/symbol.go
git commit -m "feat(symbols): add Scope tree type with local member tables"
```

### Task 3: Scope builder over ast.RootNamespace

**Files:**
- Create: `internal/core/symbols/builder.go`
- Test: `internal/core/symbols/builder_test.go`

The builder walks an `*ast.RootNamespace` and produces the document's scope tree.
Each `*ast.Package` and `*ast.Namespace` creates a child scope AND a `*Symbol`
registered in its enclosing scope under both `Ident.ShortName` and `Ident.Name`
(each non-empty key maps to the SAME symbol). Leaf declarations
(`*ast.Dependency`, `*ast.Comment`, `*ast.Documentation`,
`*ast.TextualRepresentation`) register a symbol under their identification names
but create no child scope. `*ast.Import` and `*ast.Alias` are recorded as
symbols where they have names (aliases do) but their reference targets are
resolved in later tasks; `*ast.ErrorNode` and `*ast.FilterMember` are skipped
(filters hold expressions, not declarations). Members may appear directly in the
root list or wrapped in `*ast.Membership`; the builder unwraps membership to find
the declaration and carries the membership visibility onto the symbol.

- [ ] **Step 1: Write the failing test**

```go
package symbols

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func build(t *testing.T, src string) *Scope {
	t.Helper()
	sf := source.New("test", []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %v", p.Diagnostics)
	}
	return Build(root)
}

func TestBuildTopLevelPackage(t *testing.T) {
	root := build(t, "package P;")
	sym, ok := root.LookupLocal("P")
	if !ok {
		t.Fatalf("P not found in root scope")
	}
	if sym.Kind != SymbolPackage {
		t.Fatalf("P kind = %v, want package", sym.Kind)
	}
	if _, isPkg := sym.Decl.(*ast.Package); !isPkg {
		t.Fatalf("P Decl type = %T, want *ast.Package", sym.Decl)
	}
}

func TestBuildNestedMembers(t *testing.T) {
	root := build(t, "package Outer { package Inner; namespace N; }")
	outer, ok := root.LookupLocal("Outer")
	if !ok {
		t.Fatalf("Outer not found")
	}
	outerScope := outer.Scope
	if outerScope == nil {
		t.Fatalf("Outer has no child scope")
	}
	if _, ok := outerScope.LookupLocal("Inner"); !ok {
		t.Fatalf("Inner not found in Outer scope")
	}
	nsym, ok := outerScope.LookupLocal("N")
	if !ok || nsym.Kind != SymbolNamespace {
		t.Fatalf("N not found as namespace in Outer scope")
	}
}

func TestBuildShortAndPrimaryNameKeys(t *testing.T) {
	root := build(t, "package <p> Primary;")
	for _, key := range []string{"p", "Primary"} {
		sym, ok := root.LookupLocal(key)
		if !ok {
			t.Fatalf("key %q not found", key)
		}
		if sym.Kind != SymbolPackage {
			t.Fatalf("key %q kind = %v, want package", key, sym.Kind)
		}
	}
	// Both keys must map to the same symbol.
	a, _ := root.LookupLocal("p")
	b, _ := root.LookupLocal("Primary")
	if a != b {
		t.Fatalf("short and primary keys map to different symbols")
	}
}

func TestBuildVisibilityCarried(t *testing.T) {
	root := build(t, "private package Secret;")
	sym, ok := root.LookupLocal("Secret")
	if !ok {
		t.Fatalf("Secret not found")
	}
	if sym.Visibility != ast.VisibilityPrivate {
		t.Fatalf("Secret visibility = %v, want private", sym.Visibility)
	}
}

func TestBuildAliasSymbol(t *testing.T) {
	root := build(t, "package P; alias A for P;")
	sym, ok := root.LookupLocal("A")
	if !ok || sym.Kind != SymbolAlias {
		t.Fatalf("alias A not found as alias symbol")
	}
}

func TestBuildErrorNodeSkipped(t *testing.T) {
	// Unknown declaration keyword yields an ErrorNode; builder must not panic
	// and must still register the good package.
	root := build(t, "package Good;")
	if _, ok := root.LookupLocal("Good"); !ok {
		t.Fatalf("Good not registered")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/symbols/ -run 'TestBuild' -v`
Expected: FAIL — `undefined: Build`.

- [ ] **Step 3: Write minimal implementation**

```go
package symbols

import "github.com/Open-MBEE/Systemica/internal/core/ast"

// Build constructs the immutable scope tree for a parsed document.
func Build(root *ast.RootNamespace) *Scope {
	rootScope := NewScope(nil, nil)
	if root == nil {
		return rootScope
	}
	buildMembers(rootScope, root.Members)
	return rootScope
}

// buildMembers processes a member list into the given scope.
func buildMembers(scope *Scope, members []ast.Node) {
	for _, m := range members {
		decl, vis := unwrapMember(m)
		if decl == nil {
			continue
		}
		buildDecl(scope, decl, vis)
	}
}

// unwrapMember returns the underlying declaration node and its visibility.
// Membership wrappers carry visibility; directly-listed Import/Alias nodes
// carry their own.
func unwrapMember(m ast.Node) (ast.Node, ast.Visibility) {
	switch v := m.(type) {
	case *ast.Membership:
		return v.Member, v.Visibility
	case *ast.Import:
		return v, v.Visibility
	case *ast.Alias:
		return v, v.Visibility
	default:
		return m, ast.VisibilityDefault
	}
}

// buildDecl registers a symbol (and child scope, where applicable) for a single
// declaration node.
func buildDecl(scope *Scope, decl ast.Node, vis ast.Visibility) {
	switch d := decl.(type) {
	case *ast.Package:
		child := NewScope(scope, d)
		sym := newSymbol(d.Ident, SymbolPackage, d, vis, child)
		defineIdent(scope, d.Ident, sym)
		scope.AddChild(child)
		buildMembers(child, d.Members)
	case *ast.Namespace:
		child := NewScope(scope, d)
		sym := newSymbol(d.Ident, SymbolNamespace, d, vis, child)
		defineIdent(scope, d.Ident, sym)
		scope.AddChild(child)
		buildMembers(child, d.Members)
	case *ast.Alias:
		sym := newSymbol(d.Ident, SymbolAlias, d, vis, nil)
		defineIdent(scope, d.Ident, sym)
	case *ast.Dependency:
		sym := newSymbol(d.Ident, SymbolDependency, d, vis, nil)
		defineIdent(scope, d.Ident, sym)
	case *ast.Comment:
		sym := newSymbol(d.Ident, SymbolComment, d, vis, nil)
		defineIdent(scope, d.Ident, sym)
	case *ast.Documentation:
		sym := newSymbol(d.Ident, SymbolDocumentation, d, vis, nil)
		defineIdent(scope, d.Ident, sym)
	case *ast.TextualRepresentation:
		sym := newSymbol(d.Ident, SymbolTextualRepresentation, d, vis, nil)
		defineIdent(scope, d.Ident, sym)
	case *ast.Import, *ast.FilterMember, *ast.ErrorNode:
		// Imports are processed during resolution; filters hold expressions;
		// error nodes have no declaration. Nothing to register here.
	}
}

// newSymbol builds a Symbol from an identification.
func newSymbol(id ast.Identification, kind SymbolKind, decl ast.Node, vis ast.Visibility, scope *Scope) *Symbol {
	name := id.Name
	if name == "" {
		name = id.ShortName
	}
	sym := &Symbol{
		Name:       name,
		Kind:       kind,
		Decl:       decl,
		Visibility: vis,
		DeclSpan:   decl.Span(),
		Scope:      scope,
	}
	return sym
}

// defineIdent registers sym under both the short and primary name keys.
func defineIdent(scope *Scope, id ast.Identification, sym *Symbol) {
	scope.Define(id.ShortName, sym)
	scope.Define(id.Name, sym)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/symbols/ -run 'TestBuild|TestScope|TestSymbol' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/symbols/builder.go internal/core/symbols/builder_test.go
git add internal/core/symbols/builder.go internal/core/symbols/builder_test.go
git commit -m "feat(symbols): build scope tree and symbols from ast.RootNamespace"
```


### Task 4: Global qualified-name index

**Files:**
- Create: `internal/core/symbols/index.go`
- Test: `internal/core/symbols/index_test.go`

The `Index` aggregates documents. `AddDocument(name, root)` builds the scope
tree and walks it, assembling each symbol's fully-qualified name (parent QN
segments joined by `::`) and recording it in a global map
`fqn -> []*Symbol`. Duplicate FQNs from different documents (or repeated
declarations) are all retained so the resolver can report ambiguity. The index
also keeps each document's root scope keyed by document name for per-document
qualified/unqualified resolution.

- [ ] **Step 1: Write the failing test**

```go
package symbols

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func addDoc(t *testing.T, idx *Index, name, src string) {
	t.Helper()
	sf := source.New(name, []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics for %s: %v", name, p.Diagnostics)
	}
	idx.AddDocument(name, root)
}

func TestIndexQualifiedLookup(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { package Q { namespace N; } }")

	syms := idx.LookupQualified("P::Q::N")
	if len(syms) != 1 {
		t.Fatalf("LookupQualified(P::Q::N) len = %d, want 1", len(syms))
	}
	if syms[0].Kind != SymbolNamespace {
		t.Fatalf("P::Q::N kind = %v, want namespace", syms[0].Kind)
	}
	if len(idx.LookupQualified("P::Missing")) != 0 {
		t.Fatalf("LookupQualified(P::Missing) should be empty")
	}
}

func TestIndexAmbiguousQualified(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P { namespace D; }")
	addDoc(t, idx, "b.sysml", "package P { namespace D; }")

	if got := len(idx.LookupQualified("P::D")); got != 2 {
		t.Fatalf("LookupQualified(P::D) len = %d, want 2 (ambiguous)", got)
	}
}

func TestIndexDocumentRoot(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package P;")
	rs := idx.DocumentRoot("a.sysml")
	if rs == nil {
		t.Fatalf("DocumentRoot(a.sysml) = nil")
	}
	if _, ok := rs.LookupLocal("P"); !ok {
		t.Fatalf("document root missing P")
	}
	if idx.DocumentRoot("missing.sysml") != nil {
		t.Fatalf("DocumentRoot(missing) should be nil")
	}
}

func TestIndexShortNameNotDuplicatedInFQN(t *testing.T) {
	// A package with both short and primary names registers one symbol; the
	// FQN uses the primary name. Both local keys still resolve via the scope.
	idx := NewIndex()
	addDoc(t, idx, "a.sysml", "package <p> Primary { namespace N; }")
	if len(idx.LookupQualified("Primary::N")) != 1 {
		t.Fatalf("Primary::N not indexed")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/symbols/ -run 'TestIndex' -v`
Expected: FAIL — `undefined: NewIndex`.

- [ ] **Step 3: Write minimal implementation**

```go
package symbols

import "github.com/Open-MBEE/Systemica/internal/core/ast"

// Index aggregates symbol information across all documents in a workspace.
// It owns each document's root scope and a global map from fully-qualified
// name to the symbol(s) declared under it.
type Index struct {
	docRoots map[string]*Scope     // document name -> root scope
	fqn      map[string][]*Symbol  // fully-qualified name -> symbols
}

// NewIndex creates an empty index.
func NewIndex() *Index {
	return &Index{
		docRoots: make(map[string]*Scope),
		fqn:      make(map[string][]*Symbol),
	}
}

// AddDocument builds the scope tree for root and records its symbols under
// their fully-qualified names. Re-adding the same document name replaces its
// previous root scope but leaves stale global entries; callers rebuild the
// whole index on reparse (per-doc incremental invalidation is Plan 5).
func (idx *Index) AddDocument(name string, root *ast.RootNamespace) {
	rs := Build(root)
	idx.docRoots[name] = rs
	idx.indexScope(rs, "")
}

// indexScope walks a scope, recording each distinct symbol under its FQN and
// recursing into child scopes. prefix is the FQN of the owning scope ("" at
// the document root).
func (idx *Index) indexScope(scope *Scope, prefix string) {
	seen := make(map[*Symbol]bool)
	for _, syms := range scope.members {
		for _, sym := range syms {
			if seen[sym] {
				continue // symbol registered under both short and primary key
			}
			seen[sym] = true
			fqn := joinFQN(prefix, sym.Name)
			idx.fqn[fqn] = append(idx.fqn[fqn], sym)
			if sym.Scope != nil {
				idx.indexScope(sym.Scope, fqn)
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/symbols/ -run 'TestIndex' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/symbols/index.go internal/core/symbols/index_test.go
git add internal/core/symbols/index.go internal/core/symbols/index_test.go
git commit -m "feat(symbols): add global qualified-name index"
```

### Task 5: Resolver skeleton + memo side tables

**Files:**
- Create: `internal/core/resolve/diagnostic.go`
- Create: `internal/core/resolve/resolver.go`
- Test: `internal/core/resolve/resolver_test.go`

The `resolve` package depends on `symbols`, `ast`, `source` (never `parser`).
A `Resolver` wraps a `*symbols.Index`, memoizes results in a side table keyed by
the reference AST node, and accumulates `Diagnostic`s. This task establishes the
skeleton with the public entry points returning "unresolved" so later tasks fill
in the lookup logic. The memo stores both success and failure so repeat lookups
are stable and cheap.

- [ ] **Step 1: Write the failing test**

```go
package resolve

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func TestResolverNewAndMemoEmpty(t *testing.T) {
	idx := symbols.NewIndex()
	r := New(idx)
	if r == nil {
		t.Fatalf("New returned nil")
	}
	if len(r.Diagnostics) != 0 {
		t.Fatalf("fresh resolver has diagnostics: %v", r.Diagnostics)
	}
}

func TestResolverMemoizes(t *testing.T) {
	idx := symbols.NewIndex()
	r := New(idx)
	qn := &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "X"}}}

	// First call records the result in the memo.
	_, ok1 := r.ResolveQualified(nil, qn)
	// Second call must return the memoized result without appending a
	// duplicate diagnostic.
	_, ok2 := r.ResolveQualified(nil, qn)

	if ok1 != ok2 {
		t.Fatalf("memoized result differs: %v vs %v", ok1, ok2)
	}
	if got := len(r.Diagnostics); got != 1 {
		t.Fatalf("diagnostics recorded %d times, want 1 (memoized)", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/resolve/ -run 'TestResolver' -v`
Expected: FAIL — `undefined: New`.

- [ ] **Step 3: Write minimal implementation**

`diagnostic.go`:

```go
package resolve

import "github.com/Open-MBEE/Systemica/internal/core/source"

// Diagnostic is a name-resolution problem tied to a source span.
type Diagnostic struct {
	Span    source.Span
	Message string
}
```

`resolver.go`:

```go
package resolve

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// resolution is a memoized lookup outcome.
type resolution struct {
	sym *symbols.Symbol
	ok  bool
}

// Resolver performs lazy name resolution over a symbol index, memoizing results
// keyed by the reference AST node and collecting diagnostics.
type Resolver struct {
	idx         *symbols.Index
	memo        map[ast.Node]resolution
	Diagnostics []Diagnostic
}

// New creates a resolver over the given index.
func New(idx *symbols.Index) *Resolver {
	return &Resolver{
		idx:  idx,
		memo: make(map[ast.Node]resolution),
	}
}

// ResolveQualified resolves a qualified-name reference against the given scope.
// scope may be nil to resolve purely from the global index / document root.
// Later tasks implement the walk; this skeleton reports unresolved.
func (r *Resolver) ResolveQualified(scope *symbols.Scope, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	if qn == nil {
		return nil, false
	}
	if res, done := r.memo[qn]; done {
		return res.sym, res.ok
	}
	res := r.doResolveQualified(scope, qn)
	r.memo[qn] = res
	return res.sym, res.ok
}

// doResolveQualified is the uncached qualified-name resolution. Task 6 replaces
// the body; the skeleton always fails with an unresolved diagnostic.
func (r *Resolver) doResolveQualified(scope *symbols.Scope, qn *ast.QualifiedName) resolution {
	r.Diagnostics = append(r.Diagnostics, Diagnostic{
		Span:    qn.Span(),
		Message: "unresolved reference: " + qnText(qn),
	})
	return resolution{nil, false}
}

// qnText renders a qualified name for diagnostics (segments joined by "::",
// "$::" prefix when global).
func qnText(qn *ast.QualifiedName) string {
	s := ""
	for i, part := range qn.Parts {
		if i > 0 {
			s += "::"
		}
		s += part.Text
	}
	if qn.Global {
		s = "$::" + s
	}
	return s
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/resolve/ -run 'TestResolver' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/resolve/diagnostic.go internal/core/resolve/resolver.go internal/core/resolve/resolver_test.go
git add internal/core/resolve/diagnostic.go internal/core/resolve/resolver.go internal/core/resolve/resolver_test.go
git commit -m "feat(resolve): add resolver skeleton with memoized side tables"
```

### Task 6: Qualified-name resolution (walk from root)

**Files:**
- Create: `internal/core/resolve/qualified.go`
- Modify: `internal/core/resolve/resolver.go` (replace `doResolveQualified` body)
- Test: `internal/core/resolve/qualified_test.go`

Qualified resolution resolves the FIRST segment (via the enclosing scope search
for a non-global name, or the document root for a `$::`-global name), then walks
each subsequent segment as a local member of the previously-resolved symbol's
child scope. A symbol with no child scope cannot have members, so a further
segment fails. Multiple candidates for a segment produce an ambiguity
diagnostic. To keep Task 6 self-contained, first-segment lookup uses the global
index's exact-FQN map when scope is nil, and the scope's local table (walking
outward to ancestors) when a scope is provided; richer unqualified search
(imports) comes in Task 7.

- [ ] **Step 1: Write the failing test**

```go
package resolve

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func indexOf(t *testing.T, docs map[string]string) *symbols.Index {
	t.Helper()
	idx := symbols.NewIndex()
	for name, src := range docs {
		sf := source.New(name, []byte(src))
		p := parser.New(sf)
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("parse diagnostics for %s: %v", name, p.Diagnostics)
		}
		idx.AddDocument(name, root)
	}
	return idx
}

func qn(global bool, parts ...string) *ast.QualifiedName {
	q := &ast.QualifiedName{Global: global}
	for _, p := range parts {
		q.Parts = append(q.Parts, ast.NameSegment{Text: p})
	}
	return q
}

func TestResolveQualifiedFromRoot(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { package Q { namespace N; } }",
	})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")

	sym, ok := r.ResolveQualified(root, qn(false, "P", "Q", "N"))
	if !ok {
		t.Fatalf("P::Q::N unresolved; diagnostics: %v", r.Diagnostics)
	}
	if sym.Kind != symbols.SymbolNamespace {
		t.Fatalf("P::Q::N kind = %v, want namespace", sym.Kind)
	}
}

func TestResolveQualifiedMissingSegment(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { package Q; }",
	})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")

	if _, ok := r.ResolveQualified(root, qn(false, "P", "Missing")); ok {
		t.Fatalf("P::Missing should be unresolved")
	}
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected an unresolved diagnostic")
	}
}

func TestResolveQualifiedGlobal(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { namespace N; }",
	})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")

	// $::P::N resolves from the document root regardless of scope.
	sym, ok := r.ResolveQualified(root, qn(true, "P", "N"))
	if !ok || sym.Kind != symbols.SymbolNamespace {
		t.Fatalf("$::P::N unresolved or wrong kind: %v ok=%v", sym, ok)
	}
}

func TestResolveQualifiedSegmentIntoLeaf(t *testing.T) {
	// A leaf symbol (no child scope) cannot own further segments.
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { comment C /* x */ }",
	})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")
	if _, ok := r.ResolveQualified(root, qn(false, "P", "C", "Deeper")); ok {
		t.Fatalf("P::C::Deeper should fail past the leaf comment")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/resolve/ -run 'TestResolveQualified' -v`
Expected: FAIL — resolver skeleton reports everything unresolved.

- [ ] **Step 3: Write minimal implementation**

Replace the body of `doResolveQualified` in `resolver.go` with a call into the
new walker:

```go
func (r *Resolver) doResolveQualified(scope *symbols.Scope, qn *ast.QualifiedName) resolution {
	return r.walkQualified(scope, qn)
}
```

Create `qualified.go`:

```go
package resolve

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// walkQualified resolves a qualified name segment-by-segment.
func (r *Resolver) walkQualified(scope *symbols.Scope, qn *ast.QualifiedName) resolution {
	if len(qn.Parts) == 0 {
		return resolution{nil, false}
	}

	// Resolve the first segment.
	first := qn.Parts[0].Text
	var cur *symbols.Symbol
	if qn.Global {
		cur = r.lookupInRoot(scope, first)
	} else {
		cur = r.lookupOutward(scope, first)
	}
	if cur == nil {
		r.unresolved(qn)
		return resolution{nil, false}
	}

	// Walk remaining segments as local members of the current symbol's scope.
	for _, seg := range qn.Parts[1:] {
		if cur.Scope == nil {
			r.unresolved(qn)
			return resolution{nil, false}
		}
		all := cur.Scope.LookupLocalAll(seg.Text)
		if len(all) == 0 {
			r.unresolved(qn)
			return resolution{nil, false}
		}
		if len(all) > 1 {
			r.ambiguous(qn, len(all))
			return resolution{nil, false}
		}
		cur = all[0]
	}
	return resolution{cur, true}
}

// lookupInRoot finds a name in the document root scope reachable from scope.
func (r *Resolver) lookupInRoot(scope *symbols.Scope, name string) *symbols.Symbol {
	root := rootOf(scope)
	if root == nil {
		return nil
	}
	sym, _ := root.LookupLocal(name)
	return sym
}

// lookupOutward searches scope and its ancestors for a locally-defined name.
// Import-aware search is added in Task 7.
func (r *Resolver) lookupOutward(scope *symbols.Scope, name string) *symbols.Symbol {
	for s := scope; s != nil; s = s.Parent() {
		if sym, ok := s.LookupLocal(name); ok {
			return sym
		}
	}
	return nil
}

// rootOf returns the topmost ancestor of scope (the document root), or nil.
func rootOf(scope *symbols.Scope) *symbols.Scope {
	if scope == nil {
		return nil
	}
	for scope.Parent() != nil {
		scope = scope.Parent()
	}
	return scope
}

// unresolved records an unresolved-reference diagnostic.
func (r *Resolver) unresolved(qn *ast.QualifiedName) {
	r.Diagnostics = append(r.Diagnostics, Diagnostic{
		Span:    qn.Span(),
		Message: "unresolved reference: " + qnText(qn),
	})
}

// ambiguous records an ambiguity diagnostic.
func (r *Resolver) ambiguous(qn *ast.QualifiedName, n int) {
	r.Diagnostics = append(r.Diagnostics, Diagnostic{
		Span:    qn.Span(),
		Message: "ambiguous reference: " + qnText(qn),
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/resolve/ -run 'TestResolve' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/resolve/qualified.go internal/core/resolve/resolver.go internal/core/resolve/qualified_test.go
git add internal/core/resolve/qualified.go internal/core/resolve/resolver.go internal/core/resolve/qualified_test.go
git commit -m "feat(resolve): resolve qualified names by walking from root"
```

### Task 7: Unqualified resolution (outward scope search)

**Files:**
- Create: `internal/core/resolve/unqualified.go`
- Modify: `internal/core/resolve/resolver.go`
- Test: `internal/core/resolve/unqualified_test.go`

Add the public `ResolveName` entry point for a bare (single-segment) reference.
Resolution searches outward: the starting scope, then each enclosing ancestor,
first local match wins. If no enclosing scope matches, fall back to the global
document root. Results memoize on the `at` reference node. Import-aware search
(members brought in by `import`) is layered on in Task 8; this task establishes
the plain outward walk.

- [ ] **Step 1: Write the failing test**

```go
package resolve

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

func TestResolveNameOutward(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { namespace Inner; namespace N { } }",
	})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")
	pScope := scopeOf(t, root, "P")
	nScope := scopeOf(t, pScope, "N")
	// From inside N, unqualified "Inner" resolves to P::Inner (ancestor scope).
	sym, ok := r.ResolveName(nScope, "Inner", &ast.FeatureReference{})
	if !ok {
		t.Fatalf("Inner unresolved from N; diagnostics=%v", r.Diagnostics)
	}
	if sym.Name != "Inner" {
		t.Fatalf("resolved name = %q, want Inner", sym.Name)
	}
}

func TestResolveNameUnresolved(t *testing.T) {
	idx := indexOf(t, map[string]string{"a.sysml": "package P { }"})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")
	pScope := scopeOf(t, root, "P")
	if _, ok := r.ResolveName(pScope, "Missing", &ast.FeatureReference{}); ok {
		t.Fatalf("Missing should be unresolved")
	}
	if len(r.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %d, want 1", len(r.Diagnostics))
	}
}

func TestResolveNameMemoizes(t *testing.T) {
	idx := indexOf(t, map[string]string{"a.sysml": "package P { }"})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")
	pScope := scopeOf(t, root, "P")
	at := &ast.FeatureReference{}
	r.ResolveName(pScope, "Missing", at)
	r.ResolveName(pScope, "Missing", at)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %d, want 1 (memoized)", len(r.Diagnostics))
	}
}
```

Add this helper to `unqualified_test.go` (used by later tasks too):

```go
func scopeOf(t *testing.T, parent *symbols.Scope, name string) *symbols.Scope {
	t.Helper()
	sym, ok := parent.LookupLocal(name)
	if !ok || sym.Scope == nil {
		t.Fatalf("child scope %q not found", name)
	}
	return sym.Scope
}
```

Add the import block to `unqualified_test.go`:

```go
import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/resolve/ -run 'TestResolveName' -v`
Expected: FAIL with "r.ResolveName undefined" (compile error).

- [ ] **Step 3: Write minimal implementation**

Add to `resolver.go` (public entry + memo):

```go
// ResolveName resolves a single-segment (unqualified) reference from the given
// scope. The at node keys the memo table.
func (r *Resolver) ResolveName(scope *symbols.Scope, name string, at ast.Node) (*symbols.Symbol, bool) {
	if at != nil {
		if res, done := r.memo[at]; done {
			return res.sym, res.ok
		}
	}
	res := r.walkUnqualified(scope, name)
	if at != nil {
		r.memo[at] = res
	}
	if !res.ok {
		r.Diagnostics = append(r.Diagnostics, Diagnostic{
			Span:    spanOf(at),
			Message: "unresolved reference: " + name,
		})
	}
	return res.sym, res.ok
}
```

Add to `resolver.go` a span helper (used by both entry points):

```go
func spanOf(n ast.Node) source.Span {
	if n == nil {
		return source.Span{}
	}
	return n.Span()
}
```

Create `internal/core/resolve/unqualified.go`:

```go
package resolve

import "github.com/Open-MBEE/Systemica/internal/core/symbols"

// walkUnqualified searches the scope and its ancestors for a local match,
// then falls back to the document root. Import-aware search is added in Task 8.
func (r *Resolver) walkUnqualified(scope *symbols.Scope, name string) resolution {
	for s := scope; s != nil; s = s.Parent() {
		if sym, ok := s.LookupLocal(name); ok {
			return resolution{sym: sym, ok: true}
		}
	}
	if root := rootOf(scope); root != nil {
		if sym, ok := root.LookupLocal(name); ok {
			return resolution{sym: sym, ok: true}
		}
	}
	return resolution{}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/resolve/ -run 'TestResolveName' -v`
Expected: PASS (TestResolveNameOutward, TestResolveNameUnresolved, TestResolveNameMemoizes).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/resolve/unqualified.go internal/core/resolve/resolver.go internal/core/resolve/unqualified_test.go
git add internal/core/resolve/unqualified.go internal/core/resolve/resolver.go internal/core/resolve/unqualified_test.go
git commit -m "feat(resolve): resolve unqualified names by outward scope search"
```

### Task 8: Import resolution (membership / namespace / recursive)

**Files:**
- Create: `internal/core/resolve/imports.go`
- Modify: `internal/core/resolve/unqualified.go`
- Test: `internal/core/resolve/imports_test.go`

Imports declared in a scope make names from another namespace visible to
unqualified lookups in that scope. Three forms (corrected KerML semantics,
verified against `org.omg.kerml.xtext/.../KerML.xtext:179-198`):

- **Membership import** `import A::B;` (`Kind==ImportMembership`, not recursive):
  brings the single name `B` (last segment of the target).
- **Recursive membership import** `import A::B::**;`
  (`Kind==ImportMembership`, `IsRecursive`): brings `B` and all names nested
  transitively under `B`.
- **Namespace import** `import A::B::*;` (`Kind==ImportNamespace`): brings all
  direct members of `B` (but not `B` itself).
- **Recursive namespace import** `import A::*::**;` or `import A::B::*::**;`
  (`Kind==ImportNamespace`, `IsRecursive`): brings all members of the target
  namespace transitively.

`walkUnqualified` gains a step: at each scope on the outward walk, after the
local table misses, check that scope's imports. Import targets resolve via
`ResolveQualified` against the target's own scope. A cycle guard prevents
infinite recursion when following recursive imports.

- [ ] **Step 1: Write the failing test**

```go
package resolve

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

func TestImportMembership(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package Lib { namespace Widget; }",
		"b.sysml": "package App { import Lib::Widget; }",
	})
	r := New(idx)
	appScope := scopeOf(t, idx.DocumentRoot("b.sysml"), "App")
	sym, ok := r.ResolveName(appScope, "Widget", &ast.FeatureReference{})
	if !ok {
		t.Fatalf("Widget unresolved via membership import; diags=%v", r.Diagnostics)
	}
	if sym.Name != "Widget" {
		t.Fatalf("resolved %q, want Widget", sym.Name)
	}
}

func TestImportNamespaceStar(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package Lib { namespace Widget; namespace Gadget; }",
		"b.sysml": "package App { import Lib::*; }",
	})
	r := New(idx)
	appScope := scopeOf(t, idx.DocumentRoot("b.sysml"), "App")
	if _, ok := r.ResolveName(appScope, "Widget", &ast.FeatureReference{}); !ok {
		t.Fatalf("Widget unresolved via namespace import; diags=%v", r.Diagnostics)
	}
	if _, ok := r.ResolveName(appScope, "Gadget", &ast.FeatureReference{}); !ok {
		t.Fatalf("Gadget unresolved via namespace import")
	}
}

func TestImportRecursiveMembership(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package Lib { namespace Outer { namespace Deep; } }",
		"b.sysml": "package App { import Lib::Outer::**; }",
	})
	r := New(idx)
	appScope := scopeOf(t, idx.DocumentRoot("b.sysml"), "App")
	if _, ok := r.ResolveName(appScope, "Outer", &ast.FeatureReference{}); !ok {
		t.Fatalf("Outer unresolved via recursive import; diags=%v", r.Diagnostics)
	}
	if _, ok := r.ResolveName(appScope, "Deep", &ast.FeatureReference{}); !ok {
		t.Fatalf("Deep unresolved via recursive import")
	}
}

func TestImportDoesNotLeakNonImported(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package Lib { namespace Widget; namespace Hidden; }",
		"b.sysml": "package App { import Lib::Widget; }",
	})
	r := New(idx)
	appScope := scopeOf(t, idx.DocumentRoot("b.sysml"), "App")
	if _, ok := r.ResolveName(appScope, "Hidden", &ast.FeatureReference{}); ok {
		t.Fatalf("Hidden should NOT be visible (only Widget imported)")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/resolve/ -run 'TestImport' -v`
Expected: FAIL — imports not yet consulted, so all names unresolved.

- [ ] **Step 3: Write minimal implementation**

Extend `walkUnqualified` in `unqualified.go` to consult imports at each scope,
and add the import-matching logic:

```go
package resolve

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// walkUnqualified searches the scope and its ancestors for a local match or an
// imported match, then falls back to the document root.
func (r *Resolver) walkUnqualified(scope *symbols.Scope, name string) resolution {
	for s := scope; s != nil; s = s.Parent() {
		if sym, ok := s.LookupLocal(name); ok {
			return resolution{sym: sym, ok: true}
		}
		if sym, ok := r.lookupImports(s, name); ok {
			return resolution{sym: sym, ok: true}
		}
	}
	if root := rootOf(scope); root != nil {
		if sym, ok := root.LookupLocal(name); ok {
			return resolution{sym: sym, ok: true}
		}
	}
	return resolution{}
}

// lookupImports checks every import declared directly in scope for a member
// matching name.
func (r *Resolver) lookupImports(scope *symbols.Scope, name string) (*symbols.Symbol, bool) {
	node := scope.Node()
	for _, imp := range importsOf(node) {
		if sym, ok := r.matchImport(scope, imp, name); ok {
			return sym, true
		}
	}
	return nil, false
}

// importsOf returns the *ast.Import declarations directly in a namespace-bearing node.
func importsOf(node ast.Node) []*ast.Import {
	var members []ast.Node
	switch n := node.(type) {
	case *ast.Package:
		members = n.Members
	case *ast.Namespace:
		members = n.Members
	case *ast.RootNamespace:
		members = n.Members
	default:
		return nil
	}
	var out []*ast.Import
	for _, m := range members {
		if imp, ok := m.(*ast.Import); ok {
			out = append(out, imp)
		}
	}
	return out
}

// matchImport tries to satisfy name through a single import declaration.
func (r *Resolver) matchImport(scope *symbols.Scope, imp *ast.Import, name string) (*symbols.Symbol, bool) {
	if imp.Imported == nil || len(imp.Imported.Parts) == 0 {
		return nil, false
	}
	target, ok := r.ResolveQualified(scope, imp.Imported)
	if !ok {
		return nil, false
	}
	if imp.Kind == ast.ImportMembership {
		// The imported member itself (last segment) is visible by its own name.
		if target.Name == name {
			return target, true
		}
		if imp.IsRecursive && target.Scope != nil {
			if sym, ok := lookupInSubtree(target.Scope, name, map[*symbols.Scope]bool{}); ok {
				return sym, true
			}
		}
		return nil, false
	}
	// Namespace import: members of the target's scope are visible.
	if target.Scope == nil {
		return nil, false
	}
	if sym, ok := target.Scope.LookupLocal(name); ok {
		return sym, true
	}
	if imp.IsRecursive {
		if sym, ok := lookupInSubtree(target.Scope, name, map[*symbols.Scope]bool{}); ok {
			return sym, true
		}
	}
	return nil, false
}

// lookupInSubtree searches a scope and all descendant scopes for name.
func lookupInSubtree(scope *symbols.Scope, name string, seen map[*symbols.Scope]bool) (*symbols.Symbol, bool) {
	if scope == nil || seen[scope] {
		return nil, false
	}
	seen[scope] = true
	if sym, ok := scope.LookupLocal(name); ok {
		return sym, true
	}
	for _, child := range scope.Children() {
		if sym, ok := lookupInSubtree(child, name, seen); ok {
			return sym, true
		}
	}
	return nil, false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/resolve/ -run 'TestImport|TestResolveName|TestResolveQualified' -v`
Expected: PASS (all import tests plus prior name/qualified tests still green).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/resolve/imports.go internal/core/resolve/unqualified.go internal/core/resolve/imports_test.go
git add internal/core/resolve/unqualified.go internal/core/resolve/imports_test.go
git commit -m "feat(resolve): resolve imported names (membership, namespace, recursive)"
```

(Note: the import logic lives in `unqualified.go` in this task; the empty
`imports.go` file is not created — the plan file list anticipated a split that
proved unnecessary. Commit only the two touched files.)

### Task 9: Alias resolution

**Files:**
- Create: `internal/core/resolve/alias.go`
- Test: `internal/core/resolve/alias_test.go`

An `*ast.Alias` (`alias V for Target;`) introduces the name `V` (and its short
name) bound to whatever `Target` resolves to. `ResolveName`/`ResolveQualified`
already find the alias *symbol* (the builder registers it under `Ident`). This
task adds `ResolveAliasTarget`, which follows an alias symbol to its ultimate
non-alias target, transitively, with a cycle guard.

- [ ] **Step 1: Write the failing test**

```go
package resolve

import "testing"

func TestAliasResolvesTarget(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { namespace Real; alias A for P::Real; }",
	})
	r := New(idx)
	pScope := scopeOf(t, idx.DocumentRoot("a.sysml"), "P")
	aSym, ok := pScope.LookupLocal("A")
	if !ok {
		t.Fatalf("alias A not found")
	}
	target, ok := r.ResolveAliasTarget(aSym)
	if !ok {
		t.Fatalf("alias A target unresolved; diags=%v", r.Diagnostics)
	}
	if target.Name != "Real" {
		t.Fatalf("alias target = %q, want Real", target.Name)
	}
}

func TestAliasTransitive(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { namespace Real; alias A for P::Real; alias B for P::A; }",
	})
	r := New(idx)
	pScope := scopeOf(t, idx.DocumentRoot("a.sysml"), "P")
	bSym, _ := pScope.LookupLocal("B")
	target, ok := r.ResolveAliasTarget(bSym)
	if !ok {
		t.Fatalf("transitive alias B unresolved; diags=%v", r.Diagnostics)
	}
	if target.Name != "Real" {
		t.Fatalf("transitive target = %q, want Real", target.Name)
	}
}

func TestAliasCycleGuard(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": "package P { alias A for P::B; alias B for P::A; }",
	})
	r := New(idx)
	pScope := scopeOf(t, idx.DocumentRoot("a.sysml"), "P")
	aSym, _ := pScope.LookupLocal("A")
	if _, ok := r.ResolveAliasTarget(aSym); ok {
		t.Fatalf("cyclic alias should not resolve")
	}
}

func TestResolveAliasTargetNonAlias(t *testing.T) {
	idx := indexOf(t, map[string]string{"a.sysml": "package P { namespace Real; }"})
	r := New(idx)
	pScope := scopeOf(t, idx.DocumentRoot("a.sysml"), "P")
	realSym, _ := pScope.LookupLocal("Real")
	target, ok := r.ResolveAliasTarget(realSym)
	if !ok || target != realSym {
		t.Fatalf("non-alias symbol should resolve to itself")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/resolve/ -run 'TestAlias|TestResolveAliasTarget' -v`
Expected: FAIL with "r.ResolveAliasTarget undefined".

- [ ] **Step 3: Write minimal implementation**

Create `internal/core/resolve/alias.go`:

```go
package resolve

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// ResolveAliasTarget follows an alias symbol to its ultimate non-alias target.
// Non-alias symbols resolve to themselves. Cycles yield (nil, false).
func (r *Resolver) ResolveAliasTarget(sym *symbols.Symbol) (*symbols.Symbol, bool) {
	seen := map[*symbols.Symbol]bool{}
	cur := sym
	for cur != nil {
		if cur.Kind != symbols.SymbolAlias {
			return cur, true
		}
		if seen[cur] {
			return nil, false
		}
		seen[cur] = true
		al, ok := cur.Decl.(*ast.Alias)
		if !ok || al.For == nil {
			return nil, false
		}
		// Resolve the alias target qualified name from the alias's own scope.
		next, ok := r.ResolveQualified(aliasScope(cur), al.For)
		if !ok {
			return nil, false
		}
		cur = next
	}
	return nil, false
}

// aliasScope returns the scope in which an alias's target should be resolved:
// the alias symbol's enclosing scope (where it was declared).
func aliasScope(sym *symbols.Symbol) *symbols.Scope {
	return sym.Scope
}
```

Note: `Symbol.Scope` for a leaf declaration like an alias is the enclosing scope
it was defined in (the builder sets `Scope` to the enclosing scope for leaf
symbols; child-scope-bearing symbols store their own child scope). Confirm the
builder sets alias `Scope` to the enclosing scope — if it stores `nil`, adjust
`aliasScope` to walk from the document root instead. (Per Task 3, leaf symbols
are created with `newSymbol(..., scope)` where `scope` is the enclosing scope,
so `sym.Scope` is correct here.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/resolve/ -run 'TestAlias|TestResolveAliasTarget' -v`
Expected: PASS (all four alias tests).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/resolve/alias.go internal/core/resolve/alias_test.go
git add internal/core/resolve/alias.go internal/core/resolve/alias_test.go
git commit -m "feat(resolve): follow alias targets transitively with cycle guard"
```

### Task 10: Visibility filtering

**Files:**
- Create: `internal/core/resolve/visibility.go`
- Modify: `internal/core/resolve/unqualified.go` (filter imported members)
- Test: `internal/core/resolve/visibility_test.go`

Visibility rule for Plan 3 (namespace-core only): a member declared
`private` is visible only within its own declaring scope subtree — it must not
be reachable through a **namespace import** (`::*` / `::**`) from another scope,
unless that import is `import all` (which re-exports private members). Members
with `VisibilityDefault`, `public`, or `protected` are treated as visible for
import purposes in Plan 3 (protected specialization semantics are deferred with
inheritance). Qualified resolution (`A::B::C` walk-from-root) is NOT
visibility-filtered in Plan 3: an explicit qualified path may name a private
member (matching the pilot's lenient qualified access); filtering applies only
to the implicit member enumeration performed by namespace imports.

- [ ] **Step 1: Write the failing test**

```go
package resolve

import "testing"

func TestNamespaceImportSkipsPrivate(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": "package Lib { public part def Pub; private part def Sec; }",
		"app.sysml": "package App { import Lib::*; }",
	})
	// Wait: current grammar has no 'part def'; use namespace/package members.
	_ = idx
}
```

Because the current grammar (Plan 2 scope) has no `part def`, use
`package`/`namespace` members whose visibility is set by the `public`/`private`
prefix. Replace the test above with the real fixtures:

```go
package resolve

import "testing"

func TestNamespaceImportSkipsPrivate(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": "package Lib { public namespace Pub; private namespace Sec; }",
		"app.sysml": "package App { import Lib::*; }",
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")
	r := New(idx)

	if _, ok := r.ResolveName(app, "Pub", ident("Pub")); !ok {
		t.Fatalf("expected public member Pub to be importable via Lib::*")
	}
	r2 := New(idx)
	if _, ok := r2.ResolveName(app, "Sec", ident("Sec")); ok {
		t.Fatalf("expected private member Sec to be hidden through namespace import")
	}
}

func TestImportAllReExportsPrivate(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": "package Lib { private namespace Sec; }",
		"app.sysml": "package App { import all Lib::*; }",
	})
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")
	r := New(idx)
	if _, ok := r.ResolveName(app, "Sec", ident("Sec")); !ok {
		t.Fatalf("expected 'import all' to re-export private member Sec")
	}
}

func TestQualifiedAccessIgnoresPrivate(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": "package Lib { private namespace Sec; }",
	})
	root := idx.DocumentRoot("lib.sysml")
	r := New(idx)
	if _, ok := r.ResolveQualified(root, qn(false, "Lib", "Sec")); !ok {
		t.Fatalf("expected qualified path Lib::Sec to resolve even though Sec is private")
	}
}
```

Add a tiny `ident` test helper (a throwaway AST node used only as a memo key)
in `visibility_test.go`:

```go
func ident(name string) *ast.QualifiedName {
	return qn(false, name)
}
```

(`ast` must be imported in `visibility_test.go`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/resolve/ -run 'TestNamespaceImportSkipsPrivate|TestImportAllReExportsPrivate|TestQualifiedAccessIgnoresPrivate' -v`
Expected: FAIL — `TestNamespaceImportSkipsPrivate` fails because namespace
imports currently expose private members (no filtering yet).

- [ ] **Step 3: Write minimal implementation**

Create `internal/core/resolve/visibility.go`:

```go
package resolve

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// importAllowsPrivate reports whether an import re-exports private members.
// Only `import all` widens visibility to include private members.
func importAllowsPrivate(imp *ast.Import) bool {
	return imp.IsAll
}

// visibleThroughImport reports whether sym may be surfaced by imp when
// enumerating a namespace's members. Private members are hidden unless the
// import is `import all`.
func visibleThroughImport(imp *ast.Import, sym *symbols.Symbol) bool {
	if sym.Visibility == ast.VisibilityPrivate {
		return importAllowsPrivate(imp)
	}
	return true
}
```

Modify `internal/core/resolve/unqualified.go` `matchImport` so that, in the
namespace-import branch, candidate members are filtered by
`visibleThroughImport`. Where the namespace branch currently does:

```go
	// namespace import: bring members of the target scope
	if target.Scope == nil {
		return nil, false
	}
	if sym, ok := target.Scope.LookupLocal(name); ok {
		return sym, true
	}
	if imp.IsRecursive {
		return r.lookupInSubtree(target.Scope, name, map[*symbols.Scope]bool{})
	}
	return nil, false
```

replace with a visibility-filtered form:

```go
	// namespace import: bring visible members of the target scope
	if target.Scope == nil {
		return nil, false
	}
	if sym, ok := target.Scope.LookupLocal(name); ok && visibleThroughImport(imp, sym) {
		return sym, true
	}
	if imp.IsRecursive {
		if sym, ok := r.lookupInSubtree(target.Scope, name, map[*symbols.Scope]bool{}); ok && visibleThroughImport(imp, sym) {
			return sym, true
		}
	}
	return nil, false
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/resolve/ -run 'TestNamespaceImportSkipsPrivate|TestImportAllReExportsPrivate|TestQualifiedAccessIgnoresPrivate' -v`
Expected: PASS (all three). Then run the full resolve suite to confirm no
regression:

Run: `go test ./internal/core/resolve/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/resolve/visibility.go internal/core/resolve/unqualified.go internal/core/resolve/visibility_test.go
git add internal/core/resolve/visibility.go internal/core/resolve/unqualified.go internal/core/resolve/visibility_test.go
git commit -m "feat(resolve): hide private members from namespace imports unless import all"
```

### Task 11: Resolution diagnostics (unresolved / ambiguous)

**Files:**
- Create: `internal/core/resolve/document.go`
- Test: `internal/core/resolve/document_test.go`

Add a document-level driver that walks a parsed `*ast.RootNamespace`, finds
every reference node, resolves each against the scope in which it appears, and
accumulates diagnostics on the `Resolver`. This is the entry point Plan 4
(diagnostics passes) and Plan 6 (LSP) will call. Reference nodes in the current
grammar: `Import.Imported`, `Alias.For`, `Dependency.Clients`/`Suppliers`,
`PrefixMetadata.Type`, and inside expressions (`FilterMember.Condition`)
`FeatureReference.Name` and `OperatorExpr.TypeRef`.

Each reference is resolved qualified (walk-from-root / outward for a
single-segment name) via `ResolveQualified` against the enclosing scope; the
existing `unresolved`/`ambiguous` diagnostics fire during that call, so the
walker only needs to drive resolution over the right (node, scope) pairs. The
walker maps each AST scope-bearing node to its `*symbols.Scope` using the
index's document root.

- [ ] **Step 1: Write the failing test**

```go
package resolve

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func resolveDoc(t *testing.T, name, src string) *Resolver {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := NewIndexFromDoc(name, root)
	r := New(idx)
	r.ResolveDocument(name, root)
	return r
}

func TestResolveDocumentReportsUnresolved(t *testing.T) {
	r := resolveDoc(t, "d.sysml",
		"package P { alias A for P::Missing; }")
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected unresolved diagnostic for P::Missing")
	}
}

func TestResolveDocumentCleanWhenAllResolve(t *testing.T) {
	r := resolveDoc(t, "d.sysml",
		"package P { namespace N; alias A for P::N; }")
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}
}

func TestResolveDocumentResolvesExpressionRefs(t *testing.T) {
	// FilterMember condition referencing an undefined name -> diagnostic.
	r := resolveDoc(t, "d.sysml",
		"package P { filter Undefined; }")
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected unresolved diagnostic for expression ref Undefined")
	}
}
```

The test uses two helpers that must exist: `NewIndexFromDoc(name, root)` and
`Resolver.ResolveDocument(name, root)`. Add `NewIndexFromDoc` as a convenience
constructor in `internal/core/symbols/index.go`:

```go
// NewIndexFromDoc builds an Index containing a single document.
func NewIndexFromDoc(name string, root *ast.RootNamespace) *Index {
	idx := NewIndex()
	idx.AddDocument(name, root)
	return idx
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/resolve/ -run 'TestResolveDocument' -v`
Expected: FAIL — `ResolveDocument` and `NewIndexFromDoc` undefined.

- [ ] **Step 3: Write minimal implementation**

Add `NewIndexFromDoc` to `internal/core/symbols/index.go` (above), then create
`internal/core/resolve/document.go`:

```go
package resolve

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// ResolveDocument walks the document's references and resolves each, recording
// diagnostics on the Resolver. name identifies the document in the index.
func (r *Resolver) ResolveDocument(name string, root *ast.RootNamespace) {
	rootScope := r.idx.DocumentRoot(name)
	if rootScope == nil {
		return
	}
	r.walkMembers(rootScope, membersOf(root))
}

// membersOf returns the top-level members of a RootNamespace.
func membersOf(root *ast.RootNamespace) []ast.Node {
	if root == nil {
		return nil
	}
	return root.Members
}

// walkMembers resolves references in each member, descending into child scopes.
func (r *Resolver) walkMembers(scope *symbols.Scope, members []ast.Node) {
	for _, m := range members {
		decl, _ := unwrapForResolve(m)
		r.resolveDecl(scope, decl)
	}
}

// unwrapForResolve mirrors the builder's unwrapMember: it strips *ast.Membership
// wrappers so we resolve against the inner declaration.
func unwrapForResolve(m ast.Node) (ast.Node, ast.Visibility) {
	switch v := m.(type) {
	case *ast.Membership:
		return v.Member, v.Visibility
	case *ast.Import:
		return v, v.Visibility
	case *ast.Alias:
		return v, v.Visibility
	default:
		return m, ast.VisibilityDefault
	}
}

// resolveDecl resolves references contributed by a single declaration and
// recurses into declarations that own a child scope.
func (r *Resolver) resolveDecl(scope *symbols.Scope, decl ast.Node) {
	switch d := decl.(type) {
	case *ast.Package:
		r.resolvePrefixes(scope, d.Prefixes)
		if child := r.childScope(scope, d); child != nil {
			r.walkMembers(child, d.Members)
		}
	case *ast.Namespace:
		r.resolvePrefixes(scope, d.Prefixes)
		if child := r.childScope(scope, d); child != nil {
			r.walkMembers(child, d.Members)
		}
	case *ast.Import:
		r.ResolveQualified(scope, d.Imported)
	case *ast.Alias:
		r.ResolveQualified(scope, d.For)
	case *ast.Dependency:
		r.resolvePrefixes(scope, d.Prefixes)
		for _, c := range d.Clients {
			r.ResolveQualified(scope, c)
		}
		for _, s := range d.Suppliers {
			r.ResolveQualified(scope, s)
		}
	case *ast.FilterMember:
		r.resolveExpr(scope, d.Condition)
	}
}

// childScope finds the child scope whose node is decl.
func (r *Resolver) childScope(scope *symbols.Scope, decl ast.Node) *symbols.Scope {
	for _, c := range scope.Children() {
		if c.Node() == decl {
			return c
		}
	}
	return nil
}

func (r *Resolver) resolvePrefixes(scope *symbols.Scope, prefixes []*ast.PrefixMetadata) {
	for _, p := range prefixes {
		if p != nil {
			r.ResolveQualified(scope, p.Type)
		}
	}
}

// resolveExpr walks an expression subtree resolving feature references and
// classification type references.
func (r *Resolver) resolveExpr(scope *symbols.Scope, e ast.Node) {
	switch v := e.(type) {
	case nil:
		return
	case *ast.FeatureReference:
		r.ResolveQualified(scope, v.Name)
	case *ast.OperatorExpr:
		for _, op := range v.Operands {
			r.resolveExpr(scope, op)
		}
		if v.TypeRef != nil {
			r.ResolveQualified(scope, v.TypeRef)
		}
	case *ast.FeatureChainExpr:
		r.resolveExpr(scope, v.Operand)
	case *ast.IndexExpr:
		r.resolveExpr(scope, v.Operand)
		r.resolveExpr(scope, v.Index)
	case *ast.InvocationExpr:
		r.resolveExpr(scope, v.Operand)
		if v.Type != nil {
			r.ResolveQualified(scope, v.Type)
		}
		for _, a := range v.Args {
			r.resolveExpr(scope, a)
		}
		for _, na := range v.NamedArgs {
			r.resolveExpr(scope, na.Value)
		}
	case *ast.CollectExpr:
		r.resolveExpr(scope, v.Operand)
		r.resolveExpr(scope, v.Body)
	case *ast.SelectExpr:
		r.resolveExpr(scope, v.Operand)
		r.resolveExpr(scope, v.Body)
	case *ast.ConstructorExpr:
		if v.Type != nil {
			r.ResolveQualified(scope, v.Type)
		}
		for _, a := range v.Args {
			r.resolveExpr(scope, a)
		}
	case *ast.BodyExpr:
		r.resolveExpr(scope, v.Result)
	case *ast.SequenceExpr:
		for _, el := range v.Elements {
			r.resolveExpr(scope, el)
		}
	case *ast.MetadataAccessExpr:
		r.ResolveQualified(scope, v.Ref)
	}
	// Literals (LiteralBool/String/Integer/Real/Infinity, NullExpr) have no refs.
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/resolve/ -run 'TestResolveDocument' -v`
Expected: PASS (all three). Then confirm the whole module:

Run: `go test ./... && go vet ./...`
Expected: PASS, vet clean.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/resolve/document.go internal/core/resolve/document_test.go internal/core/symbols/index.go
git add internal/core/resolve/document.go internal/core/resolve/document_test.go internal/core/symbols/index.go
git commit -m "feat(resolve): add document reference walker producing resolution diagnostics"
```

### Task 12: Integration golden tests over fixtures

**Files:**
- Create: `internal/core/resolve/integration_test.go`
- Create: `testdata/resolve/basic.sysml`
- Create: `testdata/resolve/basic.golden`
- Create: `testdata/resolve/errors.sysml`
- Create: `testdata/resolve/errors.golden`

Golden test: parse each fixture, build the index, run `ResolveDocument`, and
serialize the accumulated diagnostics (line:col + message) to a `.golden` file.
Use a `-update` flag like the parser integration tests (Plan 2). A clean fixture
produces an empty (or `"(no diagnostics)"`) golden; an error fixture lists each
diagnostic deterministically (sorted by span offset).

- [ ] **Step 1: Write the failing test**

Create `internal/core/resolve/integration_test.go`:

```go
package resolve

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

var update = flag.Bool("update", false, "update resolve golden files")

func runResolveGolden(t *testing.T, name string) {
	t.Helper()
	base := filepath.Join("..", "..", "..", "testdata", "resolve", name)
	srcBytes, err := os.ReadFile(base + ".sysml")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	sf := source.New(name+".sysml", srcBytes)
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := NewIndexFromDoc(name+".sysml", root)
	r := New(idx)
	r.ResolveDocument(name+".sysml", root)

	diags := append([]Diagnostic(nil), r.Diagnostics...)
	sort.Slice(diags, func(i, j int) bool {
		return diags[i].Span.Offset < diags[j].Span.Offset
	})
	var b strings.Builder
	if len(diags) == 0 {
		b.WriteString("(no diagnostics)\n")
	}
	for _, d := range diags {
		pos := sf.Lines().PosAt(d.Span.Offset)
		fmt.Fprintf(&b, "%d:%d %s\n", pos.Line, pos.Col, d.Message)
	}
	got := b.String()

	goldenPath := base + ".golden"
	if *update {
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
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestResolveGoldenBasic(t *testing.T)  { runResolveGolden(t, "basic") }
func TestResolveGoldenErrors(t *testing.T) { runResolveGolden(t, "errors") }
```

Create `testdata/resolve/basic.sysml` (all references resolve):

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

Create `testdata/resolve/errors.sysml` (some references fail):

```
package P {
	alias Bad for P::Missing;
	filter Undefined;
	import Nowhere::*;
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/resolve/ -run 'TestResolveGolden' -v`
Expected: FAIL — golden files do not exist yet.

- [ ] **Step 3: Generate the golden files**

Run: `go test ./internal/core/resolve/ -run 'TestResolveGolden' -update`

Then INSPECT both golden files:
- `testdata/resolve/basic.golden` MUST be exactly `(no diagnostics)` (one line).
  If it lists any diagnostic, a reference that should resolve is failing — fix
  the resolver/builder before proceeding, do NOT accept a dirty basic golden.
- `testdata/resolve/errors.golden` MUST list unresolved diagnostics for
  `P::Missing`, `Undefined`, and `Nowhere` (three lines, sorted by offset),
  each `line:col unresolved reference: <name>`. Verify the names and that there
  are no spurious extra diagnostics.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... && go vet ./...`
Expected: PASS (golden comparison green), vet clean.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/resolve/integration_test.go
git add internal/core/resolve/integration_test.go testdata/resolve/basic.sysml testdata/resolve/basic.golden testdata/resolve/errors.sysml testdata/resolve/errors.golden
git commit -m "test(resolve): add golden resolution diagnostics integration tests"
```

## Self-Review

**Spec coverage (Plan 3 scope, Option A) vs tasks:**

| Plan-3 scope item | Task(s) |
|---|---|
| Symbol type + kinds | Task 1 |
| Per-doc scope tree, local members keyed short+full name | Tasks 2, 3 |
| Scope builder over `ast.RootNamespace` | Task 3 |
| Global qualified-name index (workspace) | Task 4 |
| Lazy resolver + memoized side tables | Task 5 |
| Qualified resolution (walk-from-root, `$::` global) | Task 6 |
| Unqualified resolution (outward scope search) | Task 7 |
| Import resolution (membership / namespace `::*` / recursive `::**`) | Task 8 |
| Alias resolution (transitive + cycle guard) | Task 9 |
| Visibility filtering (private hidden from imports; `import all` re-exports) | Task 10 |
| Unresolved / ambiguity diagnostics + document reference walker | Tasks 6, 7, 11 |
| Integration golden tests | Task 12 |

Deferred (explicitly out of Plan 3, per Option A): inheritance-aware lookup +
specialization edges (`:>` / `:` supertypes) — needs def/usage taxonomy not yet
parsed; cross-project dependencies + bundled stdlib + persistent cache (Plan 5);
type / multiplicity / constraint validation (Plan 4+). Per-document incremental
index invalidation is deferred (Task 4 note): callers rebuild the whole index on
reparse for now.

**Placeholder scan:** No `TBD`/`TODO`/`<FILL>` remain in task bodies; every code
step contains complete Go. (This Self-Review is the last section.)

**Type consistency across tasks:**
- `symbols.Symbol{Name, Kind, Decl, Visibility, DeclSpan, Scope}` — defined Task 1,
  consumed unchanged in Tasks 3–10.
- `symbols.Scope` methods `Parent/Node/Children/AddChild/Define/LookupLocal/
  LookupLocalAll` — defined Task 2, used consistently thereafter.
- `symbols.Index` API `NewIndex/AddDocument/LookupQualified/DocumentRoot` (Task 4)
  plus `NewIndexFromDoc` (added Task 11) — all referenced consistently.
- `resolve.Resolver{idx, memo, Diagnostics}` with `New(idx)` (Task 5);
  `ResolveQualified(scope, *ast.QualifiedName)` (Tasks 5/6), `ResolveName(scope,
  name, at)` (Task 7), `ResolveAliasTarget(sym)` (Task 9), `ResolveDocument(name,
  root)` (Task 11) — signatures stable.
- `resolve.Diagnostic{Span source.Span; Message string}` — Task 5, used in Tasks
  7, 11, 12.
- Import-classification semantics match the corrected Plan-2 AST (verified vs
  KerML.xtext:179-198): `A::B::**` = `ImportMembership`+`IsRecursive`; `A::B::*` =
  `ImportNamespace`; `A::*::**` = `ImportNamespace`+`IsRecursive`. Task 8's
  `matchImport` branches on `imp.Kind` accordingly.
- `aliasScope(sym) = sym.Scope`: Task 3 builder sets leaf-symbol `Scope` to the
  enclosing scope via `newSymbol(..., scope)`, so Task 9 resolves alias targets
  in the correct scope. (Confirm during Task 3/9 execution.)

**Ambiguity check:** Visibility filtering (Task 10) deliberately applies ONLY to
namespace-import member enumeration, NOT to explicit qualified paths — this is
stated in Task 10 and pinned by `TestQualifiedAccessIgnoresPrivate`.

**Execution order note:** Tasks are strictly incremental; Task 6 replaces the
Task 5 `doResolveQualified` skeleton body, Task 8 extends the Task 7
`walkUnqualified`/`matchImport`, and Task 10 filters the Task 8 namespace branch.
Apply in order. Golden files (Task 12) are generated via `-update` then inspected
before commit.
