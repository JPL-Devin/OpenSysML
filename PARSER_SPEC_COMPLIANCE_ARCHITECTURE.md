# Architecture Changes for 100% SysML v2 Spec Compliance

**Current Status:** 83.2% coverage (79/95 stdlib files)  
**Goal:** 100% coverage (95/95 stdlib files)  
**Branch:** feat/parser-stdlib-coverage

## Executive Summary

Achieving 100% spec compliance requires **5 architectural changes** addressing the remaining 16 files. Analysis shows these are not simple feature additions but fundamental grammar extensions requiring AST/parser architecture changes.

**Estimated effort:** 3-5 days  
**Risk:** Medium (changes core parsing flow)  
**Benefit:** Full spec compliance, no stdlib workarounds needed

---

## Error Distribution Analysis

**Remaining errors by type:**
- 63: "expected a body member" (47% of errors)
- 47: "expected a namespace member" (35% of errors)
- 45: "expected '{' or ';' after declaration" (15% of errors)
- 27: "expected a name" (cascading errors)
- 14: "expected 'to' between connector ends"
- Others: <10 occurrences each

**Top 5 problematic files:**
1. Occurrences.kerml (focus of constraint statements)
2. StatePerformances.kerml (state body syntax)
3. TransitionPerformances.kerml (transition connector syntax)
4. Actions.sysml (accept/send action syntax)
5. Base.kerml (namespace-level constraints)

---

## Required Architecture Changes

### 1. Constraint Declaration Statements
**Impact:** ~14 errors (22% of "expected body member")  
**Files affected:** Occurrences.kerml, Base.kerml, Links.kerml

**Current issue:**
```sysml
feature timeEnclosedOccurrences {
    subset laterOccurrence.successors subsets earlierOccurrence.successors;
    disjoint earlierOccurrence.successors from laterOccurrence.predecessors;
}
```

These are **constraint declaration statements** (not assignments, not relationships). Spec defines them as standalone declarative statements establishing subsetting/disjointness constraints between features.

**Architecture change required:**

#### AST Extension (internal/core/ast/)
```go
// New node type in ast/namespace.go or ast/constraint.go
type ConstraintStatement struct {
    NodeBase
    Kind       ConstraintKind      // Subset, Disjoint, Equal, etc.
    Subject    *ast.QualifiedName  // Left side
    Targets    []*ast.QualifiedName // Right side(s)
}

type ConstraintKind int
const (
    ConstraintSubset ConstraintKind = iota
    ConstraintDisjoint
    ConstraintEqual
    // Future: equals, istype, hastype, etc.
)
```

#### Parser Changes (internal/core/parser/)
1. **parseBodyMember()** - Add statement detection:
   ```go
   if p.atKeyword("subset") || p.atKeyword("disjoint") {
       return p.parseConstraintStatement()
   }
   ```

2. **parseConstraintStatement()** - New function:
   ```go
   func (p *Parser) parseConstraintStatement() *ast.Membership {
       kind := determineConstraintKind() // subset/disjoint/etc.
       subject := p.parseQualifiedName()
       keyword := expectKeyword() // "subsets", "from", etc.
       targets := p.parseQualifiedNameList()
       
       stmt := &ast.ConstraintStatement{
           Kind: kind,
           Subject: subject,
           Targets: targets,
       }
       return wrapInMembership(stmt)
   }
   ```

3. **Namespace parsing** - Similar for namespace-level constraints

**Semantic interpretation:** These don't define new elements, they assert structural constraints. Resolver should collect and validate them during type checking phase.

**Alternative approach:** Parse as anonymous constraints with auto-generated names. More complex, less faithful to spec intent.

---

### 2. Connector "references" Keyword Pattern
**Impact:** ~11 errors (17% of "expected body member")  
**Files affected:** Occurrences.kerml

**Current issue:**
```sysml
connector all HappensWhile :> TemporallyCoincidentOccurrences {
    end theCauses [*] occurrence theCause :> causes
        references thisOccurrence
        to [1] longerOccurrence references thatOccurrence;
}
```

Pattern: `references X to [mult] Y references Z` - connector end with mid-clause `references` keyword before `to` separator.

**Interpretation:** `references thisOccurrence` declares what the `from` end references. Parser currently expects `to` immediately after end declaration.

**Architecture change required:**

#### Parser Changes (internal/core/parser/defusage.go)
```go
// In parseConnectorEnds() after parsing first end:
if p.at(lexer.Ident) && p.acceptKeyword("references") {
    // Parse reference target for first end
    refTarget := p.parseRelationshipTarget()
    // Store in first end's relationships
    ends[0].Relationships = append(ends[0].Relationships, &ast.Relationship{
        Kind: ast.RelReferences,
        Target: refTarget,
    })
}

// Then expect "to" keyword and second end
p.expectKeyword("to")
secondEnd := p.parseConnectorEnd()

// Check for "references" on second end too
if p.acceptKeyword("references") {
    refTarget := p.parseRelationshipTarget()
    secondEnd.Relationships = append(secondEnd.Relationships, &ast.Relationship{
        Kind: ast.RelReferences,
        Target: refTarget,
    })
}
```

**No AST changes** - `references` already exists as RelationshipKind, just need to parse mid-connector.

**Risk:** Low - localized to connector parsing

---

### 3. Redefines Statement (Body Member)
**Impact:** ~5 errors (8% of "expected body member")  
**Files affected:** FeatureReferencingPerformances.kerml, Occurrences.kerml

**Current issue:**
```sysml
feature result {
    redefines result redefines values;
}
```

Pattern: `redefines X redefines Y;` as standalone statement (not part of feature declaration).

**Interpretation:** Declarative statement asserting "this feature redefines both X and Y". Similar to constraint statements.

**Architecture change required:**

#### Option A: Parse as anonymous feature
```go
// Treat "redefines X..." as anonymous feature declaration
if p.atKeyword("redefines") && !parseContextExpectsRelationship() {
    return p.parseAnonymousFeatureWithRedefines()
}
```

**Implementation:** Create Usage node with no name, only relationships. This matches "feature without name" pattern already supported.

#### Option B: New statement type
Like constraint statements above, create `RedefinesStatement` node type. More explicit but adds AST complexity.

**Recommendation:** Option A (anonymous feature) - reuses existing machinery, more consistent with spec's "everything is a feature" philosophy.

**Risk:** Low - pattern similar to anonymous features already supported

---

### 4. State Body Mid-Keyword Patterns
**Impact:** ~6 errors ("expected state keyword")  
**Files affected:** StatePerformances.kerml, Actions.sysml

**Current issue:**
```sysml
behavior StateTransitionPerformance {
    first accept then [1] transitionLinkSource.exit;
}
```

Parser error: `unknown state keyword: first` in state body context.

**Interpretation:** `first X then Y` is **succession syntax** valid in state bodies (behavioral context), not state-specific keyword.

**Root cause:** State body parser (parseStateBody in behavior.go) has restricted keyword set. Doesn't recognize succession/flow syntax allowed in general behavioral bodies.

**Architecture change required:**

#### Parser Changes (internal/core/parser/behavior.go)
```go
// In parseStateBody():
func (p *Parser) parseStateBody() []ast.Node {
    for !p.at(lexer.RBrace) && !p.atEOF() {
        switch {
        case p.atKeyword("entry"), p.atKeyword("do"), p.atKeyword("exit"):
            // State-specific keywords
            
        case p.atKeyword("state"):
            // Nested state
            
        case p.atKeyword("transition"):
            // Transition definition
            
        case p.atKeyword("first"), p.atKeyword("succession"), p.atKeyword("flow"):
            // NEW: Allow succession/flow syntax in state bodies
            members = append(members, p.parseBodyMember())
            
        case p.atKeyword("bind"), p.atKeyword("binding"):
            // NEW: Allow binding in state bodies
            members = append(members, p.parseBodyMember())
            
        default:
            // Fallback to general body member parsing
            members = append(members, p.parseBodyMember())
        }
    }
}
```

**Alternative:** Make state bodies accept **any** body member, not just state-specific keywords. State semantics enforced during semantic analysis, not parsing.

**Risk:** Low - broadens accepted syntax, no breaking changes

---

### 5. Feature Chain Connectors & Complex Patterns
**Impact:** ~14 errors ("expected 'to' between connector ends")  
**Files affected:** TransitionPerformances.kerml, Flows.sysml, Ports.sysml

**Current issue:**
```sysml
connector all guardConstraint
    from [0..1] transitionLink to [1..*] trigger;

private connector
    from [1] transitionLink.transitionAction.accept
    to [1] accept.receiver = triggerTarget;
```

**Two patterns:**
1. **Connector shorthand declarations** - Named feature chain as connector end without explicit `end` keyword
2. **Connector end with assignment** - `to [1] X = expr` pattern

**Current behavior:** 
- Parser expects `connect X to Y` or `from X to Y` with simple identifiers
- Feature chains (`.` navigation) cause confusion
- Assignment after `to` not supported

**Architecture change required:**

#### Parser Changes (internal/core/parser/defusage.go)
```go
// In parseConnectorEnds():
func (p *Parser) parseConnectorEnds(u *ast.Usage, keyword string) {
    // ... existing code ...
    
    // Parse "from" end
    if p.acceptKeyword("from") {
        mult := p.parseMultiplicity()
        
        // NEW: Support feature chains as connector ends
        target := p.parseFeatureChainOrName()  // Was: parseIdentification()
        
        fromEnd := &ast.Usage{
            Kind: ast.UsageGeneric,
            Multiplicity: mult,
            // Store feature chain in relationships
        }
        
        // Convert feature chain to typing relationship
        if isFeatureChain(target) {
            fromEnd.Relationships = append(fromEnd.Relationships, &ast.Relationship{
                Kind: ast.RelTyping,
                Target: target,
            })
        }
    }
    
    p.expectKeyword("to")
    
    // Parse "to" end with optional assignment
    toMult := p.parseMultiplicity()
    toTarget := p.parseFeatureChainOrName()
    
    toEnd := &ast.Usage{ ... }
    
    // NEW: Support assignment in connector end
    if p.accept2(lexer.Eq) {
        toEnd.Value = p.ParseExpression()
    }
    
    // ... rest of connector creation ...
}
```

**Risk:** Medium - changes connector parsing flow, but isolated to connector code

---

## Implementation Strategy

### Phase 1: Low-Risk Fixes (1-2 days)
1. **Connector "references" keyword** (Change #2)
   - Localized to parseConnectorEnds()
   - No AST changes
   - Test with Occurrences.kerml

2. **State body broadening** (Change #4)
   - Fallback to general body parsing
   - Test with StatePerformances.kerml

3. **Redefines statements** (Change #3)
   - Use anonymous feature pattern
   - Test with FeatureReferencingPerformances.kerml

### Phase 2: Medium-Risk Changes (1-2 days)
4. **Feature chain connectors** (Change #5)
   - Refactor parseConnectorEnds() to accept complex targets
   - Add assignment support
   - Test with TransitionPerformances.kerml, Flows.sysml

### Phase 3: High-Impact Change (1 day)
5. **Constraint statements** (Change #1)
   - New AST node type
   - Parser integration in body/namespace
   - Test with Occurrences.kerml, Base.kerml

### Phase 4: Integration & Verification (0.5 days)
- Run full stdlib coverage test
- Verify all 95 files parse cleanly
- Check for regressions in existing tests
- Update documentation

---

## Alternative: Relaxed Compliance

If 100% compliance too costly, consider **"pragmatic 85% compliance"**:
- Current 83.2% already covers all common SysML patterns
- Remaining 16 files use advanced/rare constructs
- Users can work around with equivalent syntax

**Pros:**
- No architecture changes needed
- Focus effort on tooling/LSP/runtime features
- Still production-ready

**Cons:**
- Not fully spec-compliant
- Stdlib requires workarounds/patches
- May hit edge cases in real models

---

## Recommendation

**Implement all 5 changes** for full spec compliance. Rationale:
1. Estimated 3-5 days total effort is manageable
2. Changes are well-scoped and low-risk individually
3. Stdlib is reference implementation - should parse cleanly
4. Avoids technical debt / future "why doesn't X work?" issues
5. Strong signal of project maturity and spec fidelity

**Phased approach** allows early wins and risk mitigation. Can pause after Phase 1/2 if issues arise.

---

## Testing Strategy

For each change:
1. **Unit tests** - Isolated pattern tests (e.g., test_constraint_statement_test.go)
2. **Integration tests** - Real stdlib file samples
3. **Coverage test** - Run TestStdlibParserCoverage after each phase
4. **Regression test** - Ensure existing demos/tests still pass

**Success criteria:**
- TestStdlibParserCoverage reports 95/95 files (100%)
- All existing tests pass
- No performance degradation (stdlib parse time <100ms)

---

## Open Questions

1. **Constraint statement semantics** - Should resolver validate constraint statements immediately or defer to validation pass?
2. **Feature chain connectors** - Should AST store feature chains as QualifiedName or new FeatureChain node type?
3. **State body strictness** - Allow all body members or maintain restricted keyword set with explicit allowlist?

---

## Appendix: Error Breakdown by File

**16 files remaining:**

| File | Errors | Primary Blocker |
|------|--------|-----------------|
| Occurrences.kerml | ~30 | Constraint statements, connector references |
| StatePerformances.kerml | ~22 | State body succession syntax |
| Actions.sysml | ~31 | State body patterns, bind statements |
| TransitionPerformances.kerml | ~7 | Feature chain connectors |
| Base.kerml | ~10 | Namespace constraint statements |
| FeatureReferencingPerformances.kerml | ~15 | Redefines statements |
| Flows.sysml | ~8 | Feature chain connectors, succession |
| Ports.sysml | ~3 | Feature chain connectors |
| SysML.sysml | ~6 | Metadata patterns |
| States.sysml | ~4 | Bind in state bodies |
| Views.sysml | ~1 | Require keyword |
| ShapeItems.sysml | ~3 | Complex constraints |
| TradeStudies.sysml | ~6 | Body expr with nested body |
| StateSpaceRepresentation.sysml | ~2 | Namespace patterns |
| Links.kerml | ~2 | Constraint statements |
| ControlFunctions.kerml | ~1 | Edge case |

**Total:** ~155 errors across 16 files

---

## Revision History

- 2026-08-01: Initial proposal based on 83.2% coverage analysis
- Branch: feat/parser-stdlib-coverage
- Author: OpenCode AI + han
