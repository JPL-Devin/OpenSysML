# Def/Usage Remaining Kinds — Design

**Date:** 2026-07-29
**Builds on:** Plan 8 (part/attribute def/usage taxonomy). Extends the existing generic `Definition`/`Usage` node model to cover the remaining SysML v2 def/usage kinds.

## 1. Goal

Add the remaining def/usage kinds beyond part/attribute so real-world SysML models parse, resolve, and type-check. Cover all kinds at declaration level; model the distinctive connection/flow/port grammar at full depth; defer kind-specific behavioral grammar (Tier C) to a later cycle.

## 2. Scope

### Kind tiers

- **Tier A — pure keyword-swaps of the part/attribute pattern** (zero new grammar): `item`, `occurrence`, `individual`, `metadata`, `enum`(eration), `view`, `viewpoint`, `rendering`, `concern`.
- **Tier B — distinctive declaration grammar, FULL depth this cycle**: `connection`, `flow`, `port`, `interface`, `allocation`. Includes connector ends, flow ends + payload, and port/interface conjugation.
- **Tier C — nested behavioral bodies, RECOGNIZED with generic body this cycle**: `action`, `state`, `calc`(ulation), `constraint`, `requirement`, `case`, `analysis` case, `verification` case, `use case`. These parse with the generic declaration shape (ident + relationships + multiplicity + value + generic nested body). Their kind-specific behavioral grammar (action steps, state transitions, calc result, constraint expressions, requirement subject/assume/require) is **deferred** with explicit `TODO(feature-behavioral)` markers.

### In scope
- All remaining def-kinds and usage-kinds recognized and dispatched.
- Tier B usage-side: connector ends (`connect a to b` binary, `connect (a, b, c)` n-ary; allocation uses `allocate` with the same connector-part shape), flow ends (`from x to y` or shorthand `x to y`) with optional payload (`of P`), and conjugation (`~`) on port/interface.
- One `SymbolKind` per def-kind and per usage-kind.
- Resolution of the new reference targets (connector ends, flow ends, payload).
- Type-check kind-compatibility extended to all new kinds via the existing compat table.

### Deferred (out of scope this cycle)
- Tier C behavioral grammar (steps, transitions, subjects, calc results, constraint bodies) — TODO markers.
- Enumeration variant-membership semantics (parsed as generic body members).
- `succession flow`, `message`, binding/succession connectors (`bind`, `=`, `first`/`then`).
- Kind-checking of connector/flow end targets (ends resolved by name only).
- Prefix-metadata population on def/usage (that is Feature 2).

### Constraints discovered
- **Lexer needs no changes.** All kind keywords (`item`, `occurrence`, `connection`, `flow`, `port`, `action`, `state`, `calc`, `constraint`, `requirement`, `case`, `concern`, `view`, etc.) plus `connect`, `from`, `to`, `of`, `allocate` are already in `keywordList`; the `~` (`Tilde`) token already exists (`lexer/token.go:49`).
- **No AST visitor infra** — four manual type-switch traversals (`ast/dump.go` `dumpNode`, `symbols/builder.go` `buildDecl`, `resolve/document.go` `resolveDecl`, `passes/typecheck.go` `walk`). New Usage fields require field-handling in each, but **no new switch cases** (still a single `*ast.Usage` case).

## 3. AST (`internal/core/ast/defusage.go`)

**No new node types per kind.** Extend the existing generic `Definition`/`Usage`:

### Enum extensions
- `DefinitionKind` gains: Tier A `DefItem, DefOccurrence, DefIndividual, DefMetadata, DefEnumeration, DefView, DefViewpoint, DefRendering, DefConcern`; Tier B `DefConnection, DefFlow, DefPort, DefInterface, DefAllocation`; Tier C `DefAction, DefState, DefCalc, DefConstraint, DefRequirement, DefCase, DefAnalysisCase, DefVerificationCase, DefUseCase`. Each gets a `String()` case.
- `UsageKind` gains the parallel set (`UsageItem`, `UsageOccurrence`, … `UsageUseCase`), each with a `String()` case.

### New optional fields on `Usage` (nil/zero for kinds that don't use them)
- `ConnectorEnds []*QualifiedName` — connection / interface / allocation usage ends.
- `FlowEnds *FlowEnds` — flow usage ends.
- `IsConjugated bool` — `~` on port / interface.

### New type
```
type FlowEnds struct {
    NodeBase
    From    *QualifiedName
    To      *QualifiedName
    Payload *QualifiedName // optional; from the `of` clause
}
```
`FlowEnds` embeds `NodeBase` (spannable) but is only ever reached through the `*ast.Usage` traversal case.

**No new fields on `Definition`** — the def-side of every kind is the generic part-def shape.

## 4. Lexer + Parser

### Lexer
No changes.

### Parser (`internal/core/parser/defusage.go`)
1. Extend `definitionKindKeywords` and `usageKindKeywords` with every new kind → enum entry. Dispatch auto-enables (`atDefUsageStart` keys off these maps). Add the new keywords to `declStartKeywords` for error recovery.
2. **Two-word `use case`:** peek-ahead — `use` followed by `case` → `DefUseCase`/`UsageUseCase`; `use` alone is not a kind.
3. **Tier A + Tier C:** existing `parseDefinition`/`parseUsage` handle them unchanged once the maps know the kind. Tier C bodies parse via the generic `parseDefUsageBody` with a `TODO(feature-behavioral)` marker.
4. **Tier B end-parsing** (usage side, after relationships/multiplicity/value, before body):
   - **connection / interface / allocation usage:** on `connect` (or `allocate` for allocation) parse binary `end to end` or n-ary `( end , end , … )`; each end is a `parseQualifiedName` → `ConnectorEnds`.
   - **flow usage:** optional `of <payload>`, then optional `from <x> to <y>`, or shorthand `<x> to <y>` → `FlowEnds`.
   - **port / interface (either side):** leading `~` → `IsConjugated` (def side too if `~` precedes).
5. **Malformed-end recovery:** on a malformed connector/flow end (e.g. `connect a to` with a missing end), record a parse diagnostic, stop end-parsing, and keep the ends parsed so far. The declaration still becomes a valid `Usage` node (diagnostic + partial ends; not a whole-declaration `ErrorNode`).

## 5. Symbols + Type Check

### Symbols (`symbols/symbol.go`, `builder.go`)
- One `SymbolKind` per def-kind and per usage-kind (`SymbolItemDef`, `SymbolItemUsage`, `SymbolConnectionDef`, `SymbolConnectionUsage`, … through all tiers). Mirrors the `SymbolPartDef`/`SymbolAttributeDef`/`SymbolPartUsage`/`SymbolAttributeUsage` precedent.
- `definitionSymbolKind`/`usageSymbolKind` switches gain a case per new kind.
- `buildDecl` needs no new node cases — the connector/flow-end `*QualifiedName`s are references, not declarations, and define no symbols.

### Type check (`passes/typecheck.go`)
- `defSymbolKind`/`usageWantsDefKind` extend so each usage-kind maps to its matching def-kind (item usage typed by item def, connection usage by connection def, …).
- `compatMessage` rules are structurally unchanged: `specializes` requires the same def-kind; `subsets`/`redefines` require a usage; typing (`:`) requires the matching def-kind; cross-kind → error. Only the kind coverage grows.
- Connector/flow ends are resolved by name but **not** kind-checked this cycle (deferred with Tier C).

## 6. Resolution (`resolve/document.go`)
The single `*ast.Usage` case in `resolveDecl` gains field-handling: resolve each `ConnectorEnds` entry, and `FlowEnds.From`/`.To`/`.Payload`, via `ResolveQualified` (nil-guarded). Unresolved ends produce a name-resolution diagnostic through the existing machinery. No new switch case.

## 7. Testing
- **Parser:** per-tier dispatch (A kinds → correct kind; C kinds → correct kind + generic body); Tier B ends — `connect a to b`, `connect (a,b,c)`, flow `from x to y`, flow `of P from x to y`, shorthand `x to y`, `~` conjugation; malformed-end diagnostic + partial-ends.
- **AST dump:** golden dumps rendering `ConnectorEnds`, `FlowEnds` (From/To/Payload), `IsConjugated`.
- **Symbols:** each new kind registers its `SymbolKind`; ends define no symbols.
- **Resolve:** connector ends / flow ends / payload resolve to declared targets; unresolved end → diagnostic.
- **Typecheck:** same-kind specialize OK; cross-kind error; typing wants matching def-kind — representative sample across new kinds (not all).
- **Integration (`model/defusage_test.go`):** a document mixing several new kinds resolves clean through the workspace.

## 8. Change checklist
1. `ast/defusage.go` — extend `DefinitionKind`/`UsageKind` enums + `String()`; add `ConnectorEnds`/`FlowEnds`/`IsConjugated` to `Usage`; add `FlowEnds` type.
2. `ast/dump.go` — render new Usage fields in the `*Usage` case.
3. `parser/defusage.go` — extend keyword maps + `declStartKeywords`; two-word `use case`; Tier B end/payload/conjugation parsing with diagnostic+partial-ends recovery.
4. `symbols/symbol.go` — new `SymbolKind` consts + names.
5. `symbols/builder.go` — extend `definitionSymbolKind`/`usageSymbolKind`.
6. `resolve/document.go` — resolve connector/flow ends + payload in the `*ast.Usage` case.
7. `passes/typecheck.go` — extend `defSymbolKind`/`usageWantsDefKind` for the new kinds.
8. Tests across parser / ast / symbols / resolve / passes / model.
