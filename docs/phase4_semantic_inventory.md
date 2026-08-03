# Phase 4.1 - Inventory of Syntactic vs Semantic Decisions in Parser

**Legend:**
- **KEEP**: Genuinely syntactic decision (belongs in parser)
- **MOVE**: Semantic decision (should move to resolve/semantics)
- **REVIEW**: Unclear, needs discussion

## Inventory

| Location | Pattern | Current Behavior | Classification | Justification |
|----------|---------|------------------|----------------|---------------|
| defusage.go:654-664 | `datatype` without colon | Parser infers definition vs usage based on lookahead for `:` | **MOVE** | Def-vs-usage is semantic. Parser should create uniform AST node, let semantics classify |
| defusage.go:2998-3003 | `:>` operator meaning | Parser interprets as `subsets` (usage) vs `specializes` (def) based on isUsage flag | **MOVE** | Relationship kind is semantic. Parser knows `:>` syntax exists, semantics should determine subsets vs specializes from context |
| defusage.go:425-447 | `include use case X` | Parser creates use case usage with includes relationship | **KEEP** | Multi-word keyword `include` is syntactic signal. Creates correct AST structure |
| defusage.go:449-458 | `succession flow from X to Y` | Parser creates flow with succession semantics | **KEEP** | Multi-word keyword `succession` is syntactic. Flow + succession typing is valid AST |
| defusage.go:464-480 | `perform X` shorthand | Parser infers action usage from `perform` keyword alone | **REVIEW** | `perform` IS syntactic keyword, but bare `perform X` without `action` keyword requires semantic inference? |
| defusage.go:502-523 | `event X` shorthand | Parser infers event occurrence from `event` keyword | **KEEP** | `event` keyword is syntactic marker for occurrence usage |
| defusage.go:551-576 | `include X` shorthand | Parser creates use case with includes relationship | **KEEP** | `include` keyword is syntactic marker |
| defusage.go:825 | Step usage allows "do" as identifier | Parser bypasses keyword check for specific usage kind | **KEEP** | Grammar-level exception for step usages |
| defusage.go:909-937 | Named succession pattern | Parser detects `identifier "first"` pattern to distinguish named succession from anonymous | **KEEP** | Lookahead-driven disambiguation is syntactic |
| defusage.go:1314-1337 | Calc body detection | Parser checks for `in`/`return` keywords to decide calc vs action body parsing | **KEEP** | Keyword presence drives parser path selection (syntactic) |
| defusage.go:2042-2049 | End shortname pattern | Parser detects `end name [mult] feature` pattern | **KEEP** | Pattern matching is syntactic |

## Summary

**MOVE to semantics (2 decisions):**
1. **datatype def-vs-usage inference** (defusage.go:654-664): Parser should create uniform node, semantics decides
2. **`:>` relationship kind** (defusage.go:2998-3003): Parser should mark as `:>` relationship, semantics interprets as subsets vs specializes

**KEEP in parser (8 decisions):**
- Multi-word keyword patterns (include, succession, event, perform)
- Grammar-level exceptions (step "do" identifier)
- Lookahead disambiguation (named succession)
- Keyword-driven parser path selection (calc body)
- Pattern matching (end shortname)

**REVIEW (1 decision):**
- `perform X` shorthand - borderline case, keyword is syntactic but bare usage without `action` keyword might be semantic inference

## Recommendation

Focus Phase 4.2 on the 2 clear **MOVE** cases:
1. Datatype inference
2. `:>` operator meaning

These are unambiguous semantic decisions currently made in parser. Moving them downstream will make parser more uniform and semantics more explicit.
