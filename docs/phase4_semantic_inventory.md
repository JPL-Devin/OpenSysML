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

**MOVE to semantics (1 decision):**
1. **datatype def-vs-usage inference** (defusage.go:654-664): Parser should create uniform node, semantics decides → **COMPLETED in Phase 4.2a**

**KEEP in parser (9 decisions):**
- **`:>` relationship kind** (defusage.go:2998-3003): Context-sensitive grammar rule, NOT semantic inference. Parser has syntactic context (Definition vs Usage AST node) to disambiguate correctly. Downstream code requires distinct RelSubsets/RelSpecializes kinds.
- Multi-word keyword patterns (include, succession, event, perform)
- Grammar-level exceptions (step "do" identifier)
- Lookahead disambiguation (named succession)
- Keyword-driven parser path selection (calc body)
- Pattern matching (end shortname)

**REVIEW (1 decision):**
- `perform X` shorthand - borderline case, keyword is syntactic but bare usage without `action` keyword might be semantic inference

## Revised Recommendation

Phase 4.2 complete with datatype fix. The `:>` operator interpretation is NOT a semantic decision - it's a context-sensitive grammar rule that belongs in parser. Parser correctly uses AST node type (Definition vs Usage) to disambiguate the overloaded `:>` syntax.

**No further moves needed for Phase 4.2**. Proceed to Phase 4.3 verification.
