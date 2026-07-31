Let me verify the current in-progress state compiles before documenting it.
Build and tests green. Here are the handoff notes.

# Handoff Notes — SysML v2 Go: Validation Depth C

**Date:** 2026-07-30
**Repo:** [/home/han/IdeaProjects/Systems-Modeling](cci:9://file:///home/han/IdeaProjects/Systems-Modeling:0:0-0:0)
**Branch state:** all code compiles; `go build ./...`, `go test ./...`, `go vet ./...` green. No `gofmt` findings in new files (pre-existing `internal/core/source/source_test.go` is unrelated).

## 1. Context — where this sits

Two design docs were just written and are the source of truth:

- **`@/home/han/IdeaProjects/Systems-Modeling/docs/superpowers/specs/2026-07-30-tier-c-behavioral-grammar-design.md`** — future work (Tier C behavioral grammar, phased C1–C4). *Not started.*
- **`@/home/han/IdeaProjects/Systems-Modeling/docs/superpowers/specs/2026-07-30-validation-depth-c-design.md`** — **the active track.** Read §3 (infrastructure), §4 (rule families), §6 (phasing V-C1…V-C5).

We are implementing **Validation Depth C** (roadmap finish-line item). V-C1/V-C2/V-C3 have **no** dependency on Tier C grammar; V-C4 is gated on it.

## 2. What is DONE

### V-C1 — semantic infrastructure (COMPLETE, fully tested)
New package **`@/home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/`**:

- **[model.go](cci:7://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/model.go:0:0-0:0)** — [Model](cci:2://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/model.go:19:0-24:1) over [*resolve.Resolver](cci:2://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/resolve/resolver.go:16:0-20:1). [NewModel(resolver)](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/model.go:26:0-35:1). Specialization graph: [DirectSupertypes](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/model.go:63:0-95:1), [AllSupertypes](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/model.go:97:0-123:1) (transitive, cycle-safe, memoized), [Conforms(a,b)](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/model.go:125:0-140:1), [HasSpecializationCycle](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/model.go:142:0-160:1) (detects self/2-node/3-node). Generalization edges = `specializes|subsets|redefines|typing`.
- **[eval.go](cci:7://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/eval.go:0:0-0:0)** — bounded model-level evaluator. `(m *Model) Eval(n) (Value, bool)`; `Value{Kind: ValInt|ValReal|ValBool|ValInfinity}`. Handles int/real/bool literals, `*`, arithmetic/relational/boolean/conditional operators. Returns `ok=false` for anything outside the subset (feature refs, div-by-zero, non-integer `**` exponent).
- **[multiplicity.go](cci:7://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/multiplicity.go:0:0-0:0)** — [MultiplicityOf(sym) (Range, bool)](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/multiplicity.go:62:0-73:1); `Range{Lower,Upper Bound}`, `Bound{Value,Infinite,Known}`; [Range.LowerLeUpper() (valid, ok bool)](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/multiplicity.go:75:0-91:1). **Gotcha discovered:** the parser stores a single-bound `[n]` in `ast.Multiplicity.Lower` (NOT `Upper`, despite the misleading AST comment at `@/home/han/IdeaProjects/Systems-Modeling/internal/core/ast/defusage.go:249`). Extraction accounts for this.
- **[members.go](cci:7://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/members.go:0:0-0:0)** — [MembersOf(sym)](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/members.go:4:0-45:1) (local + inherited with name masking, deterministic) and [LookupMember(sym, name)](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/members.go:47:0-68:1).
- Tests: [model_test.go](cci:7://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/model_test.go:0:0-0:0), [eval_test.go](cci:7://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/eval_test.go:0:0-0:0), [multiplicity_test.go](cci:7://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/multiplicity_test.go:0:0-0:0), [members_test.go](cci:7://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/members_test.go:0:0-0:0) — all passing. Test helpers [buildModel(t, src)](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/model_test.go:11:0-25:1) and [sym(t, scope, key)](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/model_test.go:27:0-35:1) live in [model_test.go](cci:7://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/model_test.go:0:0-0:0).

### V-C2 — started (wiring only)
- **`@/home/han/IdeaProjects/Systems-Modeling/internal/core/passes/pass.go`** — added [Context.Model()](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/passes/pass.go:70:0-78:1) (lazy [semantics.NewModel(c.Resolver())](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/model.go:26:0-35:1)). `LevelConstraint` already existed in the enum. No import cycle (`semantics` imports [resolve](cci:9://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/resolve:0:0-0:0)+[symbols](cci:9://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/symbols:0:0-0:0); [passes](cci:9://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/passes:0:0-0:0) imports `semantics`).

## 3. What is NOT done — pick up HERE

Immediate next steps (todo IDs 10, 11, 12), in order:

1. **Specialization-cycle check pass** (`passes/constraint_cycle.go` or similar):
   - New pass type at [Level() == LevelConstraint](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/passes/pass.go:39:1-39:18).
   - Walk the document root scope (`ctx.Index.DocumentRoot(name)`) recursively; for each def/usage **symbol**, call [ctx.Model().HasSpecializationCycle(sym)](cci:2://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/model.go:19:0-24:1); emit `Diagnostic{Severity: SeverityError, Span: sym.DeclSpan, Source: "constraint", Code: "specialization-cycle"}`.
   - **Dedupe symbols by pointer** when walking — a decl with short+primary names registers the same `*Symbol` under two keys.
   - Register it in `DefaultRegistry()` at `@/home/han/IdeaProjects/Systems-Modeling/internal/core/passes/analyze.go:11`.
2. **Multiplicity-range check** — for each usage, [MultiplicityOf](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/multiplicity.go:62:0-73:1) → [LowerLeUpper()](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/multiplicity.go:75:0-91:1); if `ok && !valid`, emit constraint diagnostic (code e.g. `multiplicity-range`). Mirror pilot `checkMultiplicityRange`.
3. **Subsetting multiplicity conformance** (pilot `checkSubsetting`) — compare subsetting vs subsetted [MultiplicityOf](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/multiplicity.go:62:0-73:1). This is more involved; see design §4.1.
4. **Tests** — add `passes/constraint_*_test.go`. Generalize the existing `typeDiags` helper (`@/home/han/IdeaProjects/Systems-Modeling/internal/core/passes/typecheck_test.go:13`) to filter `Source: "constraint"`. Add a [model/](cci:9://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/model:0:0-0:0) integration test proving a cyclic spec surfaces a diagnostic end-to-end via `Workspace.Diagnostics`.

## 4. Key conventions / patterns to follow

- **Pass registry:** `DefaultRegistry()` in [analyze.go](cci:7://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/passes/analyze.go:0:0-0:0); passes ordered by [Level()](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/passes/pass.go:39:1-39:18); a pass at level N is skipped if a lower level emitted an error (so cycle checks only run on name-res/type-clean docs).
- **Diagnostics from constraint passes:** always `Source: "constraint"`, stable `Code:` (design §5 suggests pilot-derived names like `INVALID_SPECIALIZATION_...`; the simpler kebab codes above are fine if consistent — pick one and stay consistent).
- **How TypeCheckPass walks** (`@/home/han/IdeaProjects/Systems-Modeling/internal/core/passes/typecheck.go:36`) is the template: it walks AST + [childScopeOf](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/passes/typecheck.go:304:0-311:1). For constraint checks, walking the **symbol tree** (scope → [LookupLocalAll](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/symbols/scope.go:57:0-60:1) → [sym.Scope](cci:2://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/symbols/scope.go:7:0-13:1)) is cleaner because you get `*Symbol` directly for the [Model](cci:2://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/model.go:19:0-24:1) calls.
- **Resolver caveat:** [Model](cci:2://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/semantics/model.go:19:0-24:1) calls [resolver.ResolveQualified](cci:1://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/resolve/resolver.go:30:0-43:1) on demand (memoized); no need to call `ResolveDocument` first inside passes (matches [TypeCheckPass](cci:2://file:///home/han/IdeaProjects/Systems-Modeling/internal/core/passes/typecheck.go:13:0-13:27)).
- Test style: table-driven, `parseOneMember`-style helpers; keep source snippets minimal; 1 rule per fixture for parity clarity.

## 5. Verification commands

```
go build ./...
go test ./...
go vet ./...
gofmt -l internal/core/semantics internal/core/passes
```

## 6. Progress tracker (todo list, current)

Done: V-C1 multiplicity, evaluator, inherited members; V-C2 Context.Model() wiring.
In progress / next: specialization-cycle pass → multiplicity-range + subsetting conformance → full build/test/vet + model integration test.

There is also a persistent memory (`SysML-Go: design docs + Validation Depth C kickoff`) summarizing this same state.
