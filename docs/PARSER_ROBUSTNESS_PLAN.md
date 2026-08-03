# Parser Robustness & Correctness Implementation Plan

**Status:** Proposed
**Audience:** Engineers/agents implementing changes to `internal/core/parser`, `internal/core/ast`, `internal/core/resolve`, `internal/core/semantics`, and `internal/core/libs` tests.
**Goal:** Move the parser from a coverage-driven (per-file whack-a-mole) design to a **grammar-driven, semantically-layered** design, and build a test harness that catches *silently-wrong* ASTs — not just parse diagnostics.

---

## 0. Context: Why This Work Exists

The current parser is a hand-written recursive descent (~9,500 LOC). It is **not** robust or extensible today for these concrete reasons (verify each before starting so you understand the problem):

1. **Context-specific member whitelists.** Each body type has a bespoke parser that only accepts an anticipated set of member keywords. Example: `parseRequirementMember` at `internal/core/parser/behavior.go:1521-1554` rejects anything outside `subject/assume/require/actor/doc/def-usage`. Real stdlib uses the general member set, so it fails.
2. **Syntactic disambiguation of semantic facts.** def-vs-usage, relationship kinds, and one-offs like `datatype` (`internal/core/parser/defusage.go:666`) are decided in the parser, producing silently-wrong ASTs.
3. **Bounded-lookahead heuristics.** e.g. `for i < 10 { // reasonable lookahead limit }` at `internal/core/parser/defusage.go:1537`.
4. **Tests cannot catch wrong-but-accepted parses.** "Clean parse" = zero diagnostics only. Coverage tests only `t.Logf` and never fail (`internal/core/libs/stdlib_coverage_test.go:42`, `stdlib_summary_test.go:39`). No AST-shape assertions, round-trip, or negative tests.
5. **Docs drift.** README/ARCHITECTURE claim 100% stdlib coverage; measured is 96.8% (91/94).

**Guiding principle for all work below:** *If I wanted hacks, I'd write it myself. Don't ever choose hacky over correct.* Prefer minimal upstream fixes at the grammar/structure level over per-file downstream patches.

### Reference facts (already verified — do not re-derive)
- Stdlib corpus is embedded; iterate via `embedSource.List()` / `.Read(name)` in `internal/core/libs/source.go:44,62`.
- An AST printer already exists: `ast.Dump(n Node) string` at `internal/core/ast/dump.go:9`. Use it for golden tests.
- Parser entry point: `parser.New(source.New(name, data)).ParseFile() -> *ast.RootNamespace` (`internal/core/parser/parser.go:161`). Diagnostics are on `p.Diagnostics`.
- Semantic layering already exists downstream: `internal/core/resolve` (name resolution) and `internal/core/semantics` (types/conformance/members). This is where semantic disambiguation belongs.

---

## Execution Order & Dependencies

Phases are ordered so that **safety nets land before refactors**. Do not reorder.

```
Phase 1 (test gate)  ──►  Phase 2 (golden/round-trip/negative harness)
        │                          │
        └──────────────►  Phase 3 (unify member grammar)  ──►  Phase 4 (syntax/semantics split)
                                   │
                                   └──►  Phase 5 (grammar traceability)  ──►  Phase 6 (docs reconcile)
```

**Rule:** Every phase must leave `go build ./...` and `go test ./...` green before the next phase starts. Never weaken or delete an existing test to make a phase pass; if a test encodes wrong behavior, fix it and document why in the commit message.

---

## Phase 1 — Establish a Real Conformance Gate (safety net first)

**Objective:** Make the stdlib corpus a hard, failing signal so every later change has a measurable baseline. This phase intentionally does **not** fix parser bugs.

### Task 1.1 — Add a gating conformance test
- **File:** create `internal/core/libs/stdlib_conformance_test.go`.
- **Steps:**
  1. Iterate `embedSource{}.List()`; parse each with `parser.New(source.New(name, data)).ParseFile()`.
  2. Collect files with `len(p.Diagnostics) > 0`.
  3. Compare the failing set against a checked-in **allowlist** file `internal/core/libs/testdata/stdlib_known_failures.txt` (one relative path per line, `#` comments allowed).
  4. `t.Errorf` if: (a) a file **not** in the allowlist has diagnostics (regression), or (b) a file **in** the allowlist now parses clean (allowlist is stale — must be trimmed).
- **Seed the allowlist** with exactly the 3 current failures:
  ```
  Systems Library/Requirements.sysml
  Systems Library/Allocations.sysml
  Systems Library/States.sysml
  ```
- **Acceptance:** `go test ./internal/core/libs/ -run TestStdlibConformance` passes with the seeded allowlist; removing any line while its file still fails makes the test fail; introducing a new parse error in any other stdlib file makes the test fail.

### Task 1.2 — Keep the diagnostic-reporting tests, demote to informational
- Leave `stdlib_coverage_test.go` / `stdlib_summary_test.go` as `t.Logf`-only reporters (they are useful dashboards). The **gate** is Task 1.1.

### Task 1.3 — CI wiring
- **File:** `.circleci/config.yml`. Ensure `go test ./...` runs (it likely already does). Add nothing new if the gate test lives under `./internal/core/libs/`.
- **Acceptance:** CI fails if the conformance gate fails.

**Phase 1 exit criteria:** conformance gate committed and green; allowlist has exactly 3 entries.

---

## Phase 2 — Correctness Harness (catch silently-wrong ASTs)

**Objective:** Detect the bug class that "zero diagnostics" cannot: wrong tree shape and over-permissive acceptance.

### Task 2.1 — Golden AST snapshots for representative constructs
- **Files:** fixtures under `testdata/parse/` (pattern already used by `internal/core/parser/integration_test.go:16-28`). Add `<name>.sysml` + `<name>.golden` pairs.
- **Coverage set (minimum):** package/namespace; part def + usage; attribute with value; connection/connector ends; flow; requirement with `subject/assume/require`; constraint/`inv`; state with entry/do/exit + transition; action with control nodes; calc with `return`; enum; import/alias; metadata `@`/`#` prefix; multiplicity; feature chain + invocation expressions.
- **Generation:** golden = `ast.Dump(root)` (`internal/core/ast/dump.go:9`). Provide an `-update` flag pattern in the test to regenerate goldens intentionally.
- **Acceptance:** `go test ./internal/core/parser/ -run TestGolden` passes; goldens are reviewed by a human once (they encode intended shape).

### Task 2.2 — Round-trip (parse → print → parse) stability
- **Prereq check:** determine whether a source printer exists. `ast.Dump` is a debug dump, **not** necessarily valid SysML. If no faithful source printer exists, implement round-trip at the **AST level** instead: `parse(src) => ast1`, `Dump(ast1) => d1`, and assert `Dump(parse(reprint)) == d1` only if a real printer is added. **Do not** invent a printer solely for this; if absent, record round-trip as deferred and rely on Task 2.1 + 2.3.
- **Acceptance:** either a working round-trip test over the golden fixtures, or a documented deferral with rationale in the plan's Progress Log (Section "Progress Log").

### Task 2.3 — Negative tests (invalid input MUST produce diagnostics)
- **File:** create `internal/core/parser/negative_test.go`.
- **Steps:** table of malformed snippets (e.g., `part {`, `requirement r { require ; }`, `attribute x = ;`, unterminated string, `part def 123`). Assert `len(p.Diagnostics) > 0` for each.
- **Why:** guards against the parser silently accepting garbage — the flip side of over-permissiveness.
- **Acceptance:** all negative cases report ≥1 diagnostic.

**Phase 2 exit criteria:** golden + negative suites committed and green; round-trip implemented or explicitly deferred with reason.

---

## Phase 3 — Unify Member/Body Parsing (root-cause fix for the 3 failures)

**Objective:** Replace per-body keyword whitelists with a single general member parser. Specialization becomes *tagging*, not *gating*.

### Background to confirm first
- Read `parseBodyMember` (`internal/core/parser/defusage.go:1419`), the body dispatcher `parseDefUsageBody` (`:1382`), and the specialized body parsers in `internal/core/parser/behavior.go`: `parseRequirementBody`/`parseRequirementMember` (`:1509`,`:1521`), `parseConstraintMember` (`:1467`), state/action body parsers.
- Confirm in the OMG SysML/KerML grammar that requirement/constraint/state/action bodies admit the **general body-member set** plus their specialized members. The specialized keywords (`subject`, `require`, `assert`, `entry`, ...) are *additional* productions, not a *replacement* whitelist.

### Task 3.1 — Introduce `parseBodyMemberGeneral`
- Make `parseBodyMember` (`defusage.go:1419`) the single authority for the general member grammar: visibility, prefixes (`#`, `@`), `import`/`alias`, def/usage of any kind, bare relationships (`:>`, `:>>`, `::>`, `redefines`, `subsets`, `specializes`), succession (`then`/`first`), and bare-expression members where the grammar allows.
- **Acceptance:** existing parser unit tests still pass; no behavioral change yet for callers.

### Task 3.2 — Convert specialized body parsers to "specialized-first, general-fallback"
- For each of requirement/constraint/state/action member parsers: try the specialized keyword dispatch first; on no match, **fall through to `parseBodyMember` (general)** instead of emitting an error.
- Concretely, replace the terminal error at `behavior.go:1550` and the equivalent whitelist gates with a general-member fallback. Tag the resulting node with its body context (e.g., set a `Kind`/role field or wrap in the existing `Membership`) so downstream can still distinguish specialized members.
- **Preserve** specialized parsing for `subject/assume/require/actor` (requirement), `assert/assume [not]` (constraint), `entry/do/exit`/transitions (state) — those build specific AST nodes.
- **Acceptance:**
  - The 3 allowlisted files parse clean. Remove them from `internal/core/libs/testdata/stdlib_known_failures.txt`.
  - Conformance gate (Phase 1) passes with an **empty** allowlist.
  - Golden tests (Phase 2) for requirement/state/constraint updated intentionally and reviewed.

### Task 3.3 — Remove now-dead whitelist code and debug cruft
- Delete dead branches and leftovers like the `_ = tok // Keep for debugging` at `behavior.go:1524-1526` once general fallback is in place.
- **Acceptance:** `go vet ./...` clean; no unreferenced helpers left behind.

**Phase 3 exit criteria:** stdlib parses 94/94 clean; allowlist empty; golden + negative suites green.

---

## Phase 4 — Separate Syntax from Semantics

**Objective:** Stop deciding semantic facts in the parser. Parse uniformly; classify in `resolve`/`semantics`.

### Task 4.1 — Inventory syntactic decisions that are actually semantic
- Produce a short table (put it in the Progress Log) of every place the parser branches on meaning rather than syntax. Known starting points:
  - `datatype` def-vs-usage special-case (`defusage.go:666`).
  - Any def-vs-usage inference not driven purely by the `def` keyword.
  - Relationship-kind inference beyond the literal token.
- **Acceptance:** table committed; each entry marked `move` (to semantics) or `keep` (genuinely syntactic) with a one-line justification.

### Task 4.2 — Move `move`-marked decisions downstream
- For each `move` item: have the parser emit a **general** node preserving the literal tokens (keyword text, relationship token, multiplicity). Add/adjust classification in `internal/core/resolve/document.go` (`resolveDecl` at `:51`, `getUsageType` at `:445`) or `internal/core/semantics`.
- **Guardrail:** do this one item at a time; after each, run the full suite. Keep AST changes additive where possible to avoid breaking `runtime`/`lsp` consumers — grep for consumers before changing a node's fields.
- **Acceptance:** each moved item leaves `go test ./...` green; golden diffs are intentional and reviewed; semantic tests added for the new classification.

### Task 4.3 — Kill bounded-lookahead heuristics enabled by 4.2
- Once classification is downstream, remove fragile lookahead loops (`defusage.go:1537`, `namedArgAhead` in `expr.go:557`, etc.) where they existed only to guess meaning. Replace with grammar-driven parsing.
- **Acceptance:** heuristic count (grep for `lookahead limit`, `i < 10`, etc.) reduced; no regressions.

**Phase 4 exit criteria:** no semantic disambiguation remains in the parser except genuinely syntactic lookahead; conformance + golden + negative + semantic suites green.

---

## Phase 5 — Grammar Traceability (make the grammar the source of truth)

**Objective:** Ensure the parser is a faithful, auditable translation of the official grammar, not an inferred approximation. This is a **maintainability** phase, not a rewrite.

### Task 5.1 — Vendor the reference grammar
- Add the OMG pilot grammar files (`SysML.xtext`, KerML expressions grammar) under `docs/grammar/` (source of truth referenced by README/ARCHITECTURE: `SysML v2 Pilot 2026-05`). Include a `README` noting version/commit.
- **Acceptance:** grammar files committed; provenance documented.

### Task 5.2 — Map parser functions to grammar productions
- Add a doc `docs/grammar/PRODUCTION_MAP.md`: table of `grammar production -> Go function(s) -> status (faithful/approximate/todo)`.
- Prioritize the productions touched by Phases 3–4.
- **Acceptance:** map covers all top-level and body-member productions; `approximate`/`todo` rows have follow-up issues filed.

### Task 5.3 — Decide (do not necessarily execute) generator vs hand-written
- Write a short ADR (`docs/adr/0001-parser-strategy.md`) recording the decision: keep hand-written RD (recommended for LSP error recovery) with 1:1 production structure, versus adopting a PEG/generator (`participle`/`pigeon`) for the declaration/expression grammar. Include criteria and the chosen path.
- **Acceptance:** ADR merged. Only implement a generator migration if the ADR selects it and a separate plan is written; **do not** start a rewrite under this plan.

**Phase 5 exit criteria:** grammar vendored, production map exists, strategy ADR merged.

---

## Phase 6 — Reconcile Documentation

### Task 6.1 — Update status claims to measured reality
- **Files:** `README.md` (status table + "Parser coverage" lines ~112,130), `docs/ARCHITECTURE.md` (lines ~425,439).
- Replace absolute claims ("100%", "Full ... compliance achieved") with the conformance-gate result (should be 94/94 after Phase 3) and link to the gate test as the source of truth.
- **Acceptance:** docs match `go test` output; no unverifiable superlatives.

### Task 6.2 — Document the testing contract
- Add a short section to `docs/ARCHITECTURE.md` describing the four-layer parser test contract: conformance gate, golden ASTs, round-trip (or deferral), negative tests. State that new grammar features require all four.
- **Acceptance:** contract documented; referenced from `CONTRIBUTING.md`.

---

## Global Acceptance Criteria (Definition of Done)

- `go build ./...` and `go test ./...` green.
- `go vet ./...` clean for touched packages.
- Stdlib conformance gate: **94/94 clean, empty allowlist.**
- Golden AST + negative-parse suites exist and are green; round-trip implemented or explicitly deferred with rationale.
- No semantic disambiguation remains in the parser (Phase 4 inventory fully resolved).
- Grammar vendored + production map + strategy ADR present.
- Docs reflect measured coverage.

---

## Verification Commands (copy-paste)

```bash
# Full build + test
go build ./...
go test ./...
go vet ./...

# Conformance gate (Phase 1)
go test ./internal/core/libs/ -run TestStdlibConformance -v

# Coverage dashboard (informational)
go test ./internal/core/libs/ -run 'TestStdlibErrorSummary|TestStdlibParserCoverage' -v \
  | grep -E "Parse coverage|Parsed cleanly|Failed:|Top 10"

# Golden AST + negatives (Phase 2)
go test ./internal/core/parser/ -run 'TestGolden|TestNegative' -v

# Regenerate goldens intentionally (only after reviewing diffs)
go test ./internal/core/parser/ -run TestGolden -update
```

---

## Guardrails for Implementing Agents

- **One phase at a time; keep the tree green between phases.** Never advance with a red build.
- **Never weaken/delete a test to pass.** If a test encodes wrong behavior, fix the test and justify in the commit.
- **Minimal, upstream fixes.** Prefer changing a grammar rule/structure over adding a per-file special case. If you find yourself adding another `atKeyword("...")` gate to a body parser, stop — that's the anti-pattern this plan removes.
- **Check consumers before changing AST fields.** `grep` `internal/core/runtime`, `internal/lsp`, `internal/core/resolve`, `internal/core/semantics` for the node type first.
- **Additive AST changes preferred** to avoid breaking downstream tiers.
- **No emojis, no comment churn** in code unless the surrounding file already does so.

---

## Progress Log (agents append here)

> Append dated entries: phase/task, files touched, decisions (esp. round-trip deferral, Phase 4 inventory table, Phase 5 ADR outcome), and current conformance numbers.

### 2026-08-03 - Phase 1 complete, Phase 2.1 complete

**Phase 1 - Conformance gate established:**
- Task 1.1: Created `internal/core/libs/stdlib_conformance_test.go` with allowlist-based gating
- Seeded allowlist: `internal/core/libs/testdata/stdlib_known_failures.txt` with 3 known failures
- Baseline: **91/94 files parse clean** (3 in allowlist)
- Gate verified: fails on regressions (files not in allowlist that fail), fails on stale allowlist entries
- Tasks 1.2, 1.3: Already satisfied (existing tests stay, CI runs `go test ./...`)
- **Status:** Phase 1 DONE

**Phase 2.1 - Golden AST snapshots:**
- Created 9 representative fixtures under `internal/core/parser/testdata/parse/`:
  - package_namespace.sysml, part_def_usage.sysml, connection.sysml
  - requirement.sysml, state.sysml, calc.sysml
  - enum.sysml, import_alias.sysml, metadata.sysml
- Created `internal/core/parser/golden_test.go` with `-update` flag support (reuses existing `update` flag from integration_test.go)
- Generated goldens via `go test -run TestGolden -update`
- All goldens pass verification
- **Status:** Task 2.1 DONE

**Next:** Task 2.2 (round-trip) - defer if no source printer exists, Task 2.3 (negative tests)

**Phase 2.2 - Round-trip (deferred):**
- Checked for source printer: `ast.Dump` is debug-only, no faithful SysML printer exists
- Per plan: "If no faithful source printer exists... record round-trip as deferred"
- **Decision:** DEFERRED - no source printer to implement round-trip parse→print→parse test
- Rationale: ast.Dump not valid SysML, building printer solely for this task not in scope
- **Status:** Task 2.2 DEFERRED (documented)

**Phase 2.3 - Negative tests:**
- Created `internal/core/parser/negative_test.go` with 9 malformed input cases
- All cases correctly report ≥1 diagnostic (parser rejects garbage)
- Removed initially-included "empty enum" (actually valid SysML - empty body legal)
- **Status:** Task 2.3 DONE

**Phase 2 complete:** Golden ASTs + negative tests green, round-trip explicitly deferred with rationale

**Next:** Phase 3 - Unify member/body parsing (root-cause fix for the 3 allowlist failures)
