# Tier C Behavioral Grammar — Design

**Date:** 2026-07-30
**Builds on:** Plan 8 (part/attribute def/usage taxonomy) and the Def/Usage Remaining Kinds cycle (`2026-07-29-def-usage-remaining-kinds-design.md`), which recognized the Tier C kinds (`action`, `state`, `calc`, `constraint`, `requirement`, `case`, `analysis`/`verification`/`use case`) at declaration level but parsed their bodies with the **generic** def/usage body and left `TODO(feature-behavioral)` for the kind-specific grammar. This design fills in that behavioral grammar.

## 1. Goal

Parse, resolve, and (structurally) validate the distinctive **body grammar** of the behavioral def/usage kinds: calc/constraint result & return, requirement subject/assume/require/frame/actor/stakeholder, case objective/subject, action control-flow nodes and successions, and state entry/do/exit subactions and transitions. Kind-checking of the new semantic edges (does an `assert` target a constraint?) is largely deferred to the Validation Depth C cycle; this cycle focuses on **grammar + AST + name resolution**.

## 2. Why phased

The behavioral grammar is ~939 lines of the pilot `SysML.xtext` (≈38% of the whole SysML grammar) and the densest part — action nodes alone are 11 productions with control flow, and transitions weave triggers/guards/effects/successions. It is too large and too internally-dependent for one clean cycle. We split into four phases in dependency order; each is independently shippable, test-complete, and leaves the tree green.

- **Phase C1 — Calc & Constraint bodies** (smallest; establishes the "body-with-result-expression" pattern; reuses the expression parser).
- **Phase C2 — Requirements & Cases** (subject/constraint/objective members; depends on C1's constraint members).
- **Phase C3 — Actions** (control-flow + primitive nodes, successions; the large one).
- **Phase C4 — States & Transitions** (entry/do/exit, transitions; depends on C3's action nodes and successions).

## 3. Cross-cutting design decisions

### 3.1 AST strategy — body-member nodes, not new Definition/Usage subtypes
The `Definition`/`Usage` nodes already carry `Kind` and a generic `Members []Node`. Behavioral grammar is expressed as **new member-node types** that appear inside those `Members` slices, plus a small number of new fields on `Usage` for the connector-like behavioral forms (transitions, successions). No new `Definition`/`Usage` Go types. This mirrors how Tier B added `ConnectorEnds`/`FlowEnds` to `Usage` rather than new node types.

New AST node types are introduced per phase (§ per-phase sections). Each embeds `NodeBase` (spannable) and is reached only through the enclosing def/usage traversal — so the four manual type-switch traversals (`ast/dump.go`, `symbols/builder.go`, `resolve/document.go`, `passes/typecheck.go`) each gain cases only for the members that declare names or hold references.

### 3.2 Body dispatch — per-kind body parser
Today `parseUsage`/`parseDefinition` call the single `parseDefUsageBody`. We introduce a **body-kind dispatch**: after the declaration head, choose the body parser by kind:

- generic body (Tiers A/B, part/attribute) → existing `parseDefUsageBody` (unchanged).
- calc/constraint/case/analysis/verification/use-case → `parseResultBody` (C1/C2): generic members + optional trailing result expression + `return` members.
- requirement/concern → `parseRequirementBody` (C2): generic members + subject/constraint/frame/actor/stakeholder.
- action → `parseActionBody` (C3): generic members + action nodes + successions.
- state → `parseStateBody` (C4): generic members + entry/do/exit + transitions, with optional `parallel` prefix.

The dispatch is a single `switch kind` in `parseUsage`/`parseDefinition` selecting the body function; each body function still accepts the same generic members via a shared `parseBodyMember` helper so nested part/attribute/etc. keep working everywhere.

### 3.3 Keywords
All required keywords already exist in `lexer/keywordList` (verified: `accept send assign if else while loop for terminate merge decide fork join parallel entry do exit transition first then via when at after until return subject assume require frame stakeholder actor objective verify include perform exhibit`). The `:=` assignment token and `.` feature-chain already lex. **No lexer changes in any phase.**

### 3.4 Error recovery
Behavioral member parsers follow the established pattern: on a malformed member, emit a diagnostic, sync to the next `;`/member-start/`}`, and keep the enclosing declaration a valid node. New member-start keywords are added to `declStartKeywords`/the body sync set per phase.

### 3.5 Deferred every phase
- Semantic kind-checking of behavioral edges (assert→constraint, satisfy→requirement, exhibit→state, perform→action, subject typing) → **Validation Depth C**.
- Model-level expression evaluation (guard truthiness, multiplicity bounds) → **Validation Depth C**.
- `succession flow`, `message`, binding connectors (`bind`) — tracked separately from Tier C.

---

## 4. Phase C1 — Calc & Constraint bodies

### Grammar covered
`CalculationBody` / `CalculationBodyPart` / `ReturnParameterMember` / `ResultExpressionMember` (xtext 1947–1969); `ConstraintDefinition`/`ConstraintUsage` (which reuse `CalculationBody`) and `AssertConstraintUsage` (1993–2013).

### AST
- `Usage` gains `ResultExpr Node` — the optional trailing owned expression of a calc/constraint/case body (the value the body computes). Nil when absent.
- New member node `ReturnParameter { NodeBase; Usage *Usage }` — a `return` member wrapping a nested usage; appears in `Members`.
- `Usage` gains `IsNegated bool` (for `assert not`), and a new `UsageAssertConstraint` kind (and `UsageSatisfy` deferred to C2). `assert` is a usage prefix producing a constraint usage; represent as `Kind = UsageConstraint` with `IsAsserted bool` + `IsNegated bool` on `Usage`, plus optional reference-subsetting target.

### Parser
- Add `parseResultBody`: loop of generic body members and `return` members; a trailing bare `OwnedExpression` (no leading keyword) becomes `ResultExpr`.
- `assert` handled in `parseDefUsage` dispatch as a usage prefix: consume `assert`, optional `not`, then either a reference-subsetting target or `constraint`-usage declaration; set `IsAsserted`/`IsNegated`.

### Symbols / Resolve / Typecheck / Dump
- `return` parameter usages register their name (if any) in the enclosing scope.
- `ResultExpr` and assert targets resolve via `resolveExpr`/`ResolveQualified`.
- Dump renders `result=` child and `asserted=`/`negated=` attributes.

### Tests
Calc def with `return x; x + 1` result; constraint def with boolean body expression; `assert c;`, `assert not c;`; result-expression resolution; malformed return recovery.

---

## 5. Phase C2 — Requirements & Cases

### Grammar covered
`RequirementBody`/`RequirementBodyItem` and members: `SubjectMember`, `RequirementConstraintMember` (`assume`/`require`), `FramedConcernMember` (`frame`), `ActorMember`, `StakeholderMember` (2035–2105); `RequirementUsage`, `SatisfyRequirementUsage` (2113–2141); `CaseBody`/`CaseBodyItem`, `ObjectiveMember` (2179–2204); analysis/verification/use-case bodies including `RequirementVerificationMember` (`verify`) and `IncludeUseCaseUsage` (`include`) (2218–2306).

### AST
- New member node `BehavioralMember { NodeBase; Role BehavioralRole; Usage *Usage; Expr Node }` — a uniform wrapper for the keyword-prefixed members. `BehavioralRole` enum: `RoleSubject, RoleAssume, RoleRequire, RoleFrame, RoleActor, RoleStakeholder, RoleObjective, RoleVerify`. This one node + enum covers all requirement/case member kinds, keeping the traversal switch small.
- `Usage` gains `SatisfyTarget *QualifiedName` and `SatisfyBy *QualifiedName` for `satisfy … by …`; new usage kinds `UsageSatisfy`, and reuse `UsageUseCase` with `IsIncluded bool` for `include`.

### Parser
- `parseRequirementBody`/`parseCaseBody`: generic members plus the role-prefixed members (`subject`, `assume`, `require`, `frame`, `actor`, `stakeholder`, `objective`, `verify`), each parsed into a `BehavioralMember`.
- `satisfy` and `include` handled as usage prefixes in dispatch (like `assert`).
- Case bodies also allow the C1 `return`/result-expression tail (CaseBody has `ResultExpressionMember`).

### Symbols / Resolve / Typecheck / Dump
- Role members that own a usage (subject, actor, stakeholder, objective, framed concern) register that usage's name and own a child scope; assume/require/verify own constraint/requirement usages resolved by reference.
- Dump renders `(BehavioralMember role="subject" …)`.

### Tests
Requirement def with subject + require constraint + actor; `satisfy R by s;`; case def with objective + subject; use-case `include`; verification `verify`; resolution of subject/constraint targets.

---

## 6. Phase C3 — Actions

### Grammar covered
`ActionBody`/`ActionBodyItem`, `InitialNodeMember` (`first`), action nodes `AcceptNode`/`SendNode`/`AssignmentNode`/`IfNode`/`WhileLoopNode`/`ForLoopNode`/`TerminateNode`, control nodes `merge`/`decide`/`join`/`fork`, and action successions `TargetSuccession`/`GuardedSuccession`/`DefaultTargetSuccession` (`then`/`else`) (xtext 1357–1726); `PerformActionUsage` (`perform`).

### AST
- New node `ActionNode { NodeBase; NodeKind ActionNodeKind; Ident Identification; Params []Node; Body []Node; ... }` where `ActionNodeKind` ∈ `{Accept, Send, Assign, If, While, For, Terminate, Merge, Decide, Join, Fork}`. Node-specific parts are held in typed sub-fields:
  - Accept: `Payload Node`, `Via *QualifiedName`, `Trigger *TriggerExpr`.
  - Send: `Payload Node`, `Via *QualifiedName`, `To *QualifiedName`.
  - Assign: `Target Node` (feature chain), `Value Node`.
  - If: `Cond Node`, `Then []Node`, `Else []Node` (Else may nest an If).
  - While/For: `Cond Node`/`Var Usage`+`In Node`, `Body []Node`, optional `Until Node`.
- New node `TriggerExpr { NodeBase; Kind TriggerKind /*at|after|when*/; Arg Node }`.
- New node `Succession { NodeBase; Source *QualifiedName; Target *QualifiedName; Guard Node; IsDefault bool }` — covers `first x`, `x then y`, guarded (`if g then y`) and default (`else y`) successions. `first` initial node → `Succession` with only `Source`/`Target` set as appropriate.
- `Usage` gains `IsPerformed bool` and a perform reference-subset target for `perform`.

### Parser
- `parseActionBody`: dispatch each item — `first` initial member, an action node (keyword-led), a control node, a nested generic usage, or a succession (`then`/`else` after a node, or a standalone `succession … first … then …`).
- One `parseActionNode` with a sub-switch per keyword; `if/while/for` recurse into nested action bodies (`{ … }`).
- Successions parse the `then`/`else` tails attached to the preceding node member.

### Symbols / Resolve / Typecheck / Dump
- Named action nodes and `for` variables register names + child scopes.
- Accept/send/assign payloads/targets/values and succession source/target/guard resolve via existing expression/qualified-name resolution.
- Dump renders `(ActionNode kind="if" …)`, `(Succession source= target= …)`, `(TriggerExpr kind="after" …)`.

### Tests
Action def with sequence of `action a; then b;`; `if` / `else if` / `while` / `for … in …`; `accept … via …`, `send … via … to …`, `assign x := y`; control nodes; `first start then a`; guarded/default successions; `perform`; malformed-node recovery.

---

## 7. Phase C4 — States & Transitions

### Grammar covered
`StateDefBody`/`StateBodyItem` with `parallel`, `EntryActionMember`/`DoActionMember`/`ExitActionMember`, `EntryTransitionMember`, `StateActionUsage` (2740–2825 region); `StateUsage`, `ExhibitStateUsage` (`exhibit`); `TransitionUsage`/`TargetTransitionUsage` with trigger (`accept`), guard (`if`), effect (`do`), and `then` successions; `TransitionSuccession` (1854–1928).

### AST
- `Usage` gains `IsParallel bool` (state body prefix).
- New member node `StateSubaction { NodeBase; Kind SubactionKind /*entry|do|exit*/; Action *ActionNode | *Usage; TransitionMembers []Node }`.
- New node `Transition { NodeBase; Ident Identification; Source *QualifiedName; Trigger *ActionNode /*accept*/; Guard Node; Effect *ActionNode /*do*/; Target *QualifiedName }` — reuses C3's `ActionNode` for trigger/effect and represents the `then` target via `Target`.
- `Usage` gains `IsExhibited bool` + exhibit reference-subset target for `exhibit`.

### Parser
- `parseStateBody`: optional `parallel`, then items — entry/do/exit subactions (each a `StateActionUsage`, possibly `perform`/`accept`/`send`/`assign` from C3), transitions (`transition …`/target transitions), or generic members.
- `parseTransition`: `[transition [name] first] source [accept …] [if guard] [do effect] then target`.
- `exhibit` handled as a usage prefix in dispatch.

### Symbols / Resolve / Typecheck / Dump
- Named states/transitions register; transition source/target, guard, trigger payload, effect resolve via existing machinery.
- Dump renders `(Transition source= target= …)`, `(StateSubaction kind="entry" …)`, `parallel=` attribute.

### Tests
State def with entry/do/exit; `parallel` state; `transition first s1 accept ev if g do act then s2`; target transitions; `exhibit`; nested state machine; malformed transition recovery.

---

## 8. Reuse & dependencies between phases
- C1 introduces `ResultExpr`/`return`; **C2 case bodies reuse** them.
- C2 introduces the constraint-member pattern reused by requirement `assume`/`require` and case `objective`.
- C3 introduces `ActionNode`, `TriggerExpr`, `Succession`; **C4 reuses** all three for state subactions and transitions.
- Every phase reuses the existing Pratt expression parser (`ParseExpression`) for guards, values, payloads, result expressions.

## 9. Testing strategy (all phases)
Follow the existing conventions: `parser/*_test.go` using `parseOneMember`; golden `ast/dump` tests; `symbols` registration tests; `resolve` clean + unresolved tests; `model` integration test mixing the new bodies. Each phase leaves `go build ./...`, `go test ./...`, and `go vet ./...` green and adds no `gofmt` findings.

## 10. Per-phase change checklist (repeated shape)
1. `ast/defusage.go` (+ new files as needed) — new member/node types, enums, `String()`.
2. `ast/dump.go` — render new nodes/fields.
3. `parser/defusage.go` (+ `actions.go`/`states.go`) — body dispatch + member parsers + recovery sync additions.
4. `symbols/builder.go` — register names for body members that declare them.
5. `resolve/document.go` — resolve references held by new nodes.
6. `passes/typecheck.go` — structural only this cycle; semantic edge-checks deferred to Depth C.
7. Tests across parser / ast / symbols / resolve / model.
