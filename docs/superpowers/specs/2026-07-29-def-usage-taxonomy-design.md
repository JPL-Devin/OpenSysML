# Def/Usage Taxonomy — Design Spec

## 1. Overview & Goal

The parser today covers only the namespace-core layer (package/namespace/import/alias/comment/dependency + the full expression grammar). Any SysML definition or usage declaration — `part def Vehicle;`, `attribute mass : Real;` — currently falls through the member-dispatch switch and becomes an `ast.ErrorNode` ("expected a namespace member"). This blocks ingestion of real SysML v2 models and the real OMG standard library.

**Goal:** deliver a full vertical slice of the def/usage taxonomy — AST → parser → symbols → resolve → semantic validation → tests — for a small, representative kind set (`part` and `attribute`), including the shared specialization/typing relationship grammar, recursive nested bodies, common modifiers, multiplicity, and feature-value expressions. Proving the whole pipeline end-to-end for two kinds makes every remaining kind (item/port/connection/action/state/...) a mechanical repeat: add an enum value and a keyword, no new architecture.

This is the first cut of a multi-plan effort. It is intentionally narrow in *kind coverage* but *complete in depth* for the kinds it covers.

## 2. Scope

### 2.1 In Scope

- **Definition kinds:** `part def`, `attribute def`.
- **Usage kinds:** `part` usage, `attribute` usage.
- **Relationship grammar** (keyword **and** symbolic forms):
  - typing: `:` / `defined by`
  - specialization: `specializes` / `:>`
  - subsetting: `subsets` / `:>`
  - redefinition: `redefines` / `:>>`
  - references: `references` / `::>`
  - crosses: `crosses` / `=>`
  - multiple targets per clause: `specializes A, B`.
- **Modifiers:** `abstract`, `variation` (defs); on usages: `ref`, `in`/`out`/`inout` direction, `composite`/`portion`, `derived`, `ordered`, `nonunique`.
- **Multiplicity:** `[n]`, `[lo..hi]`, `[*]`, `[0..*]` — bounds reuse the existing expression AST; `*` → Infinity literal.
- **Feature values:** `= expr` on usages, using the existing expression parser; value expressions are resolved.
- **Recursive nested bodies:** a def/usage body `{ ... }` may contain nested defs and usages, each becoming a child-scope symbol resolvable by qualified name. Body members support optional visibility (`public`/`private`/`protected`), uniform with top-level members.
- **Name resolution:** all relationship targets, multiplicity bounds, and value expressions are resolved against the symbol index; unresolved targets emit the existing name-resolution diagnostic.
- **Kind-compatibility validation:** a new `LevelType` pass (the first inhabitant of the currently-empty type tier) validates that relationship targets are of compatible kinds.

### 2.2 Out of Scope (Deferred)

- All other definition/usage kinds (item, port, connection, interface, action, state, calc, constraint, requirement, concern, case, analysis, verification, view, viewpoint, enum, occurrence, ...). Follow-up plans.
- The KerML base layer as an explicit abstraction (classifier/feature/type). Modeled implicitly via the generic Definition/Usage nodes for now.
- Transitive/deep inheritance validation (e.g. that a `redefines` target is *actually* an inherited feature). This cut checks only that the target is a usage of a compatible kind.
- Multiplicity bound sanity (lower ≤ upper), abstract-usage well-formedness rules, variation/variant semantics.
- Populating inherited members into scopes (inheritance-aware lookup). Resolution is declaration-local + normal scope chain only.
- Auto-loading the standard library into the workspace (pre-existing gap, shared with the LSP).

### 2.3 Constraints Discovered

- **No AST visitor infrastructure.** Traversal is a manual type-switch duplicated in three places: `ast.Dump` (`dump.go:40`), `symbols.buildDecl` (`builder.go:47`), `resolve.resolveDecl` (`document.go:51`). Every new node type must be added to all three or it silently no-ops. The new `LevelType` pass adds a fourth traversal.
- **All def/usage keywords already lex.** `part`, `def`, `attribute`, `specializes`, `subsets`, `redefines`, `references`, `crosses`, `abstract`, `variation`, `ref`, `in`, `out`, `inout`, `composite`, `portion`, `derived`, `ordered`, `nonunique` are all in `lexer/keywords.go` and tokenize as `Kind==Keyword`. **No keyword-table changes needed.** They must be matched via `atKeyword`, not as identifiers.
- **Symbolic relationship operators are NOT single tokens today.** `:>`, `:>>`, `::>`, `=>` currently lex as separate tokens (`Colon`+`Gt`, etc.). The lexer must be extended with maximal-munch compound tokens.
- **The `LevelType` and `LevelConstraint` pass tiers exist but have zero passes.** Tier gating (`registry.go:28`) already skips higher tiers when a lower tier errored. The type pass slots into the ready machinery.
- **Existing fixtures assert current non-support.** `parser/recovery_test.go:10` and `parser/namespace_test.go:133` assert `part def X;` → `ErrorNode`. These MUST be updated to assert real `Definition` nodes — a required change, not a regression.
- **`libs/record.go:20`** has a `Supers []string` placeholder field, currently empty; this cut populates it from raw specialization target names.

## 3. AST Design

New file `internal/core/ast/defusage.go`. Modeled per the **generic Definition + Usage** approach (Option A): one `Definition` struct and one `Usage` struct, each discriminated by a `Kind` enum. This mirrors the KerML/SysML metamodel (which itself treats these as one Definition concept and one Usage concept parameterized by kind) and keeps all four traversal switches (dump, builder, resolver, type pass) tiny. Adding a future kind = a new enum value + a keyword, with no new switch cases.

### 3.1 New Enums

```go
type DefinitionKind int
const (
    DefPart DefinitionKind = iota
    DefAttribute
    // extensible: DefItem, DefPort, DefAction, ...
)

type UsageKind int
const (
    UsagePart UsageKind = iota
    UsageAttribute
)

type RelationshipKind int
const (
    RelTyping       RelationshipKind = iota // ':' / 'defined by'
    RelSpecializes                          // 'specializes' / ':>'
    RelSubsets                              // 'subsets' / ':>'
    RelRedefines                            // 'redefines' / ':>>'
    RelReferences                           // 'references' / '::>'
    RelCrosses                              // 'crosses' / '=>'
)

type FeatureDirection int
const (
    DirNone FeatureDirection = iota
    DirIn
    DirOut
    DirInOut
)
```

Each enum gets a `String()` method for dump/debug output.

### 3.2 Relationship & Multiplicity Nodes

```go
type Relationship struct {
    NodeBase
    Kind   RelationshipKind
    Target *QualifiedName
}

type Multiplicity struct {
    NodeBase
    Lower   Node // expression; nil if single-bound form '[n]'
    Upper   Node // expression; for '[n]' Upper holds n and IsRange=false
    IsRange bool // true for '[lo..hi]'
}
```

Note: `Relationship` carries exactly one `Target`. A clause with multiple targets (`specializes A, B`) produces multiple `Relationship` entries with the same `Kind`. `Multiplicity` bounds are expression `Node`s (reusing the existing expression AST); `*` becomes a `LiteralInfinity`.

### 3.3 Definition Node

```go
type Definition struct {
    NodeBase
    Prefixes      []*PrefixMetadata
    Kind          DefinitionKind
    IsAbstract    bool
    IsVariation   bool
    Ident         Identification
    Relationships []*Relationship // specializes/subsets/... at the declaration head
    Members       []Node          // nested defs/usages/namespace-members (recursive)
    HasBody       bool
}
```

### 3.4 Usage Node

```go
type Usage struct {
    NodeBase
    Prefixes      []*PrefixMetadata
    Kind          UsageKind
    IsAbstract    bool
    IsReference   bool             // 'ref'
    Direction     FeatureDirection // in/out/inout
    IsComposite   bool             // 'composite' / 'portion'
    IsDerived     bool
    IsOrdered     bool
    IsNonunique   bool
    Ident         Identification
    Relationships []*Relationship  // typing (: T), specializes, subsets, redefines, ...
    Multiplicity  *Multiplicity    // nil if absent
    Value         Node             // '= expr'; nil if absent
    Members       []Node           // nested usages/defs (recursive)
    HasBody       bool
}
```

Typing is folded into `Relationships` as a `RelTyping` entry, so all specialization/typing edges resolve through one uniform mechanism.

### 3.5 Dump Support

`ast/dump.go` (`dumpNode` switch at line 40) gains cases for `*Definition`, `*Usage`, `*Relationship`, `*Multiplicity` so golden tests render them deterministically. Dump output includes kind, ident, modifier flags, relationships (kind + target), multiplicity, value presence, and recursively dumped members.

## 4. Lexer & Parser Design

### 4.1 Lexer: Compound Operators

Add compound operator tokens for the symbolic relationship forms (`lexer/token.go` `Kind` enum + `lexer/lexer.go` scanner):

| Token | `Kind` | Meaning |
|-------|--------|---------|
| `:>`  | `ColonGt`      | specializes / subsets |
| `:>>` | `ColonGtGt`    | redefines |
| `::>` | `ColonColonGt` | references |
| `=>`  | `EqGt`         | crosses |

The scanner must maximal-munch: on `:` peek for `>` → `:>`, then peek again for `>` → `:>>`; on `::` peek for `>` → `::>`; on `=` peek for `>` → `=>`. Existing single tokens `Colon`, `Eq`, `ColonColon`, `Gt` remain for their other uses (typing colon, value `=`, qualified-name `::`, short-name `>`). Care: short-name close `>` must not be swallowed by `=>`/`:>` munching — munching only triggers from the *leading* `:`/`::`/`=`, never from a bare `>`.

**Multiplicity range token:** verify during implementation whether `..` lexes as one token or two `Dot`s; the parser's `parseMultiplicity` handles whichever the lexer produces. (Plan task will confirm against disk before coding.)

### 4.2 Declaration Dispatch & Detection

Extend `parser/namespace.go` `parseDeclaration` switch (line 142). Detection order inside the switch's handling of def/usage keywords:

1. Consume leading **feature modifiers** (`abstract`, `variation`, `ref`, `in`/`out`/`inout`, `composite`, `portion`, `derived`, `ordered`, `nonunique`) via `parseFeatureModifiers()`.
2. Read the **kind keyword** (`part` / `attribute`).
3. Lookahead (`peekN`) for keyword `def`:
   - `part def` / `attribute def` → `parseDefinition`.
   - `part` / `attribute` not followed by `def` → `parseUsage`.

Because modifiers precede the kind keyword, dispatch must recognize modifier keywords as *possible declaration starts* and peek past them to the kind keyword. `variation` is a def-only modifier; `ref`/direction/composite/derived/ordered/nonunique are usage-only — mismatches (e.g. `variation part x;` without `def`) produce a diagnostic but still parse into the best-fit node.

### 4.3 New Parse Functions

- `parseDefinition(start, prefixes, mods) *ast.Definition`
- `parseUsage(start, prefixes, mods) *ast.Usage`
- `parseFeatureModifiers() featureMods` — collects modifier flags (a small struct), consuming leading modifier keywords.
- `parseRelationships() []*ast.Relationship` — loops parsing `(specializes | :> | subsets | redefines | :>> | references | ::> | crosses | => | : | defined by) QualifiedName (, QualifiedName)*`, one `Relationship` per target. Note `:>` maps to `RelSpecializes` when written as `specializes` and `RelSubsets` when written as `subsets`; **as a symbol `:>` is ambiguous** — spec convention: `:>` on a definition = specializes, `:>` on a usage = subsets. The parser records the kind based on context (def vs usage) for the symbolic form, and directly from the keyword for the keyword form.
- `parseMultiplicity() *ast.Multiplicity` — `[ expr ]` or `[ expr .. expr ]`; `*` → `LiteralInfinity`.
- `parseDefinitionBody() ([]ast.Node, bool)` — analogous to `parseNamespaceBody` but its per-member dispatch also accepts nested defs/usages (see 4.5).
- Value: after multiplicity, `accept(Eq)` → `parseExpression()` (existing).

### 4.4 Declaration Grammar Order

Matches the spec `DefinitionDeclaration` / usage form:

```
[prefixes] [modifiers] kind [def] Identification [relationships] [multiplicity] [= value] (body | ';')
```

`Identification` = optional `<shortName>` then optional `name` (existing `parseIdentification`). Anonymous usages (no name) are legal.

### 4.5 Bodies & Nested Members

`parseDefinitionBody`: `;` → `(nil, false)`; else expect `{`, loop a **body-member dispatch** until `}`/EOF, expect `}`. The body-member dispatch:

1. Parse optional trivia + optional visibility (`parseVisibility`).
2. Attempt def/usage detection (4.2). If it matches → nested `Definition`/`Usage`.
3. Otherwise fall back to the existing namespace-member path (comments, docs, nested packages/namespaces, imports, aliases).
4. On no match → `errorNodeSkip`.

Nested members are stored directly in `.Members` (wrapped in `*ast.Membership` when they carry visibility, consistent with the existing top-level convention so `unwrapMember` in the symbol builder works unchanged).

### 4.6 Error Recovery

Add to `declStartKeywords` (`namespace.go:204`) so a malformed member resyncs to the next declaration: `part`, `attribute`, `def`, `abstract`, `variation`, `ref`, `in`, `out`, `inout`, `composite`, `portion`, `derived`, `ordered`, `nonunique`. `errorNodeSkip`/`atMemberSync` machinery is otherwise unchanged and still guarantees forward progress.

## 5. Symbols Design

### 5.1 New SymbolKinds

Add to `symbols/symbol.go` (`SymbolKind` enum, line 13) + `symbolKindNames` map:

```
SymbolPartDef
SymbolAttributeDef
SymbolPartUsage
SymbolAttributeUsage
```

Naming leaves room for future `SymbolItemDef`, `SymbolPortUsage`, etc.

### 5.2 buildDecl Cases & Child Scopes

Add cases to `symbols/builder.go` `buildDecl` switch (line 47):

- `*ast.Definition` → map `Kind` (`DefPart`→`SymbolPartDef`, `DefAttribute`→`SymbolAttributeDef`); `newSymbol` from `Ident`; if `HasBody`, create a child `Scope`, `defineIdent`, `AddChild`, recurse `buildMembers` into `.Members`.
- `*ast.Usage` → map `Kind` (`UsagePart`→`SymbolPartUsage`, `UsageAttribute`→`SymbolAttributeUsage`); same child-scope + recursion for nested members.

Definition/Usage are namespace-member-producing, so the existing `unwrapMember` (which strips `*ast.Membership`) delivers them directly. **Anonymous usages** (empty name): still create a child scope for nested members, but skip `defineIdent` (matching existing empty-name handling); they remain reachable structurally but not by name.

`Relationship`/`Multiplicity`/`Value` are **not** symbols — they are resolved in the resolve pass, never defined in scopes.

### 5.3 Supers Population

`RecordEntry` (`libs/record.go`) projection: populate the existing `Supers []string` field from the **raw (unresolved) target text** of each `Relationship` whose `Kind` is `RelSpecializes`, `RelSubsets`, or `RelRedefines` (typing, references, crosses excluded — those are not generalization edges). The target text is the `QualifiedName` rendered as `A::B::C`. This gives downstream tooling a cheap supertype-name list without requiring resolution; resolved/validated inheritance is deferred.

## 6. Resolve & Semantic Passes

### 6.1 resolveDecl Cases

Add cases to `resolve/document.go` `resolveDecl` switch (line 51):

- `*ast.Definition` → for each `Relationship`, `ResolveQualified(rel.Target, scope)`; recurse into the child scope for `.Members`.
- `*ast.Usage` → resolve each `Relationship.Target`; resolve `Multiplicity` bounds and `Value` via the existing `resolveExpr`; recurse `.Members`.

Unresolved targets emit the existing name-resolution diagnostic at `LevelNameResolution`. Lookup reuses `walkQualified`/`lookupOutward` unchanged, so qualified refs like `Car::engine` resolve through the normal scope chain.

### 6.2 Kind-Compatibility Pass (LevelType)

New file `passes/typecheck.go`, `Level() == LevelType`. Registered in `passes/analyze.go:11` `DefaultRegistry`. Runs only when the `LevelNameResolution` tier produced no errors (existing tier gating in `registry.go:28`). The pass walks the resolved AST (a fourth manual traversal), and for each relationship whose target **resolved**, looks up the target's `SymbolKind` (via the index / resolver memo) and checks it against the source node's kind using a small compatibility table. Emits `SeverityError`, `Code:"type"`, `Source:"type"`. Targets that did **not** resolve are skipped (already flagged by the name-resolution tier; the pass nil-guards defensively).

### 6.3 Compatibility Rules

| Relationship | Source | Required target kind |
|--------------|--------|----------------------|
| `RelTyping` (`: T` on usage) | part usage | a definition typing it compatibly (part usage → part def; attribute usage → attribute def or value type) |
| `RelSpecializes` | definition | a **definition of the same kind** (part def specializes part def; attribute def specializes attribute def) |
| `RelSubsets` | usage | a **usage** of the same feature-kind family |
| `RelRedefines` | usage | a **usage** of a compatible kind (deep "is-inherited" check deferred) |
| `RelReferences` | usage | a usage (light check) |
| `RelCrosses` | usage | a usage (light check) |

Cross-kind violations (e.g. a part def `specializes` an attribute def) → error.

### 6.4 Deferred Semantic Checks

- Transitive/inheritance validation (redefines target actually inherited; specialization cycles).
- Multiplicity bound sanity (lower ≤ upper).
- Abstract-usage well-formedness, variation/variant rules.
- Inherited-member lookup (resolving a member defined only on a supertype).

## 7. Testing Strategy

### 7.1 Lexer

`lexer/lexer_test.go`: `:>`, `:>>`, `::>`, `=>` tokenize to the correct compound kinds; maximal-munch disambiguation (`:` vs `:>` vs `:>>`; `::` vs `::>`; `=` vs `=>`; bare `>` for short-name close is not swallowed).

### 7.2 Parser

New `parser/defusage_test.go`, table-driven with golden `ast.Dump`:

- Definitions: `part def Vehicle;`, `attribute def Mass;` → `Definition` with correct `Kind`.
- Usages: `part engine : Engine;`, `attribute mass : Real = 42;`.
- Modifiers: `abstract part def X;`, `variation part def V;`, `ref part p;`, `in`/`out`/`inout`, `composite part`, `derived`, `ordered`, `nonunique`.
- Multiplicity: `part wheels[4];`, `[0..*]`, `[*]`.
- All relationship forms, keyword + symbolic: `specializes`/`:>`, `subsets`/`:>`, `redefines`/`:>>`, `references`/`::>`, `crosses`/`=>`, `:`/`defined by`; multiple targets `specializes A, B`.
- Nested recursive bodies: `part def Car { part engine : Engine; attribute mass : Real; }`.
- Body-member visibility: `part def Car { private part secret : X; }`.
- Value expressions with references resolved later.
- **Flip existing fixtures:** `parser/recovery_test.go:10` and `parser/namespace_test.go:133` currently assert `part def X;` → `ErrorNode`; update both to assert a real `Definition` node. Keep a separate genuinely-malformed fixture to prove error recovery still resyncs.

### 7.3 Symbols

`symbols/*_test.go`: Definition/Usage produce correct `SymbolKind`; nested usages become child-scope members resolvable by qualified name; anonymous usage creates a scope but no name binding; `Supers` populated from specializes/subsets/redefines targets only.

### 7.4 Resolve

`resolve/*_test.go`: typing/specialization targets resolve; unresolved target → name-resolution diagnostic; nested qualified refs (`Car::engine`) resolve; value-expression name references resolve.

### 7.5 Type Pass

New `passes/typecheck_test.go`: kind-compat happy paths (part usage `: PartDef` OK; part def `specializes` part def OK); violations (part def `specializes` attribute def → `type` error; usage typed by wrong-kind def → error); skip-when-unresolved (no `type` error stacked on an `unresolved` error); tier gating (type diagnostics absent when name-resolution already errored).

### 7.6 Integration

Workspace/REPL smoke: submit `part def Car { part engine : Engine; }` → resolves clean; submit a kind violation → a `type`-source diagnostic rendered with a caret span. Full suite `go test ./...` + `go vet ./...` green; `go test -race` clean.

## 8. Change Checklist (Files Touched)

1. **`ast/defusage.go`** (new) — enums, `Relationship`, `Multiplicity`, `Definition`, `Usage`, `String()` methods.
2. **`ast/dump.go`** — dump cases for the four new node types.
3. **`lexer/token.go`** — `ColonGt`, `ColonGtGt`, `ColonColonGt`, `EqGt` kinds.
4. **`lexer/lexer.go`** — maximal-munch scanning of the compound operators.
5. **`parser/namespace.go`** — `parseDeclaration` dispatch cases; `parseDefinition`, `parseUsage`, `parseFeatureModifiers`, `parseRelationships`, `parseMultiplicity`, `parseDefinitionBody`, body-member dispatch; `declStartKeywords` additions.
6. **`symbols/symbol.go`** — new `SymbolKind`s + names.
7. **`symbols/builder.go`** — `buildDecl` cases + child scopes + recursion.
8. **`libs/record.go`** — populate `Supers` from specialization targets.
9. **`resolve/document.go`** — `resolveDecl` cases (resolve targets, bounds, value; recurse members).
10. **`passes/typecheck.go`** (new) + **`passes/analyze.go`** — kind-compat `LevelType` pass + registration.
11. **Tests** — `lexer_test.go`, `parser/defusage_test.go` (+ flip `recovery_test.go`, `namespace_test.go`), `symbols/*_test.go`, `resolve/*_test.go`, `passes/typecheck_test.go`, integration smoke.
