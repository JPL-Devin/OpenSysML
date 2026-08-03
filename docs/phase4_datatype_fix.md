# Phase 4.2a - Datatype Def-vs-Usage Fix Analysis

## Current Behavior (BROKEN)

Test results show parser's datatype inference is **backwards**:

| Input | Current Result | Expected Result |
|-------|---------------|-----------------|
| `datatype MyType specializes BaseType;` | Usage (attribute) | Definition (attributeDef) |
| `datatype MyType;` | Usage (attribute) | Definition (attributeDef) |
| `datatype myValue : MyType;` | Definition (attributeDef) | Usage (attribute) |
| `datatype MyType { ... }` | Usage (attribute) | Definition (attributeDef) |

## Root Cause

Parser logic at defusage.go:657-664 checks for colon lookahead:
- **Without colon** → calls `parseDefinition()` (correct intent)
- **With colon** → calls `parseUsage()` (correct intent)

BUT: This special case is placed AFTER the dual-keyword path (lines 634-651) which already consumed tokens and decided. The datatype check at line 657 executes TOO LATE - parser already committed to usage path.

## Why Semantics Should Decide

1. **Syntactic ambiguity**: `datatype X` alone doesn't syntactically distinguish def from usage
2. **Context-dependent**: Presence of `:` or `specializes` or body are semantic clues, not syntax
3. **Stdlib uses both**: Need semantic analysis of full context (relationships, body) to classify correctly

## Fix Strategy

**Remove parser's broken inference entirely**. Parser should:
1. Parse `datatype` keyword uniformly 
2. Create consistent AST structure (Usage with Kind=UsageAttribute)
3. Let semantics/symbols/builder classify based on full context:
   - Has specializes/subsets → likely definition
   - Has typing (`:`) → likely usage
   - Has structured body with features → likely definition  
   - Standalone with no relationships → ambiguous, use heuristics

This aligns with Phase 4 objective: "Parse uniformly; classify in resolve/semantics"

## Implementation Plan

1. Remove lines 654-664 special case from defusage.go
2. Let datatype always parse as usage (uniform path)
3. Move classification to symbols/builder.go where we have full AST context
4. Update test expectations to match uniform parsing
5. Verify stdlib still works (94/94 clean)
