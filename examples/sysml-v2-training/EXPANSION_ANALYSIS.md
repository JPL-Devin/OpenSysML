# Training Examples Expansion Analysis

## Current Status
- **Files:** 34 → 100 (full SysML v2 training set)
- **Parser success:** 39/100 (39%)
- **Chapters:** 11/42 → 42/42

## Parse Success by Chapter

### ✅ Fully Passing (20 chapters, 32 files)
- 01. Packages (3/3)
- 02. Part Definitions (1/1)
- 03. Generalization (1/1)
- 04. Subsetting (1/1)
- 05. Redefinition (1/1)
- 06. Enumeration Definitions (2/2)
- 07. Parts (2/2)
- 08. Items (1/1)
- 09. Connections (1/1)
- 10. Ports (2/2)
- 12. Binding Connectors (2/2)
- 15. Actions (1/1)
- 22. Opaque Actions (1/1)
- 23. State Definitions (2/2)
- 24. States (3/3)
- 29. Expressions (3/4)
- 30. Calculations (2/3)
- 32. Requirements (3/4)
- 33. Analysis (1/3)
- 37. Dependencies (1/1)

### ⚠️ Partially Passing (7 chapters, 7/21 files)
- 14. Action Definitions (2/4)
- 27. Occurrences (1/7)
- 36. Variability (1/3)
- 41. Language Extension (1/3)

### ❌ Failing (15 chapters, 0/47 files)
- 11. Interfaces (0/2)
- 13. Flows (0/3)
- 16. Conditional Succession (0/2)
- 17. Control (0/5)
- 18. Action Performance (0/1)
- 19. Terminate Actions (0/2)
- 20. Assignment Actions (0/1)
- 21. Asynchronous Messaging (0/2)
- 25. Transitions (0/3)
- 26. State Exhibition (0/1)
- 28. Individuals (0/3)
- 31. Constraints (0/7)
- 34. Verification (0/2)
- 35. Use Cases (0/2)
- 38. Allocation (0/2)
- 39. Metadata (0/2)
- 40. Filtering (0/2)
- 42. Views (0/2)

## Top Parser Errors

| Error | Count | Category |
|-------|-------|----------|
| expected a body member | 142 | Body syntax gaps |
| expected a namespace member | 88 | Top-level syntax |
| expected action node or edge keyword | 87 | Action body syntax |
| expected '{' or ';' after declaration | 20 | Declaration syntax |
| expected a name | 18 | Identification |
| expected ';' after action execution node | 15 | Action statements |
| expected 'then' after signal type | 10 | Transitions |
| expected an expression | 8 | Expression context |
| unknown action keyword: out | 5 | Action parameters |
| unknown action keyword: item | 5 | Action parameters |
| unknown action keyword: accept | 5 | Action parameters |
| expected ';' after succession edge | 5 | Action flow |
| unknown action keyword: via | 4 | Action parameters |
| expected 'requirement' keyword after 'satisfy' | 4 | Satisfy syntax |
| unknown action keyword: to | 3 | Flow shorthand |

## Feature Gaps by Priority

### P0 - Critical Action Body Syntax (blocks 25+ files)
1. **Action parameters with direction**
   - `in item scene;` / `out item picture;`
   - `in ref item x;` / `out ref item y;`
   - `accept signal;` / `via connector;`
   - Syntax: `<direction> [ref] <kind> <name> [: <type>] [= <value>];`

2. **Flow shorthand**
   - `flow X to Y;` (no `from` keyword)
   - Current: only supports `flow from X to Y;`

3. **Action body succession edges**
   - More complex succession patterns with guards
   - Multi-line action graphs with merge/join nodes

### P1 - Interface & Flow Advanced (blocks 5 files)
4. **Interface inline connectors**
   - `interface : Type connect end1 ::> X to end2 ::> Y;`
   - Combines interface usage + connector definition + end redefines

5. **Flow multiline/of-typing**
   - `flow of Type\n  from X\n  to Y;` (newlines before `from`)
   - Current: expects `to` immediately after type

### P2 - Constraint Bodies (blocks 7 files)
6. **Constraint body patterns**
   - Complex assertion/assumption patterns
   - Mixed constraint members

### P3 - Advanced Features (blocks 15 files)
7. **Metadata/annotations**
8. **View definitions**
9. **Allocation connectors**
10. **Use case/verification case patterns**

## Implementation Strategy

### Phase 1: Action Bodies (Target: +25 files → 64%)
- Implement action parameter direction syntax
- Add flow shorthand
- Test chapters 13, 16-21

### Phase 2: Interface & Flow (Target: +5 files → 69%)
- Interface inline connectors
- Flow multiline
- Test chapters 11, 13

### Phase 3: Constraint Bodies (Target: +7 files → 76%)
- Complex constraint patterns
- Test chapter 31

### Phase 4: Advanced Features (Target: +15 files → 91%)
- Metadata, views, allocation, verification
- Test chapters 34-42

## Next Steps

1. Create feature/action-body-syntax branch
2. Implement P0 features
3. Verify progress with test suite
4. Iterate through phases

## Notes

- Semantic errors (unresolved references, type mismatches) excluded from analysis
- Focus on parser/syntax gaps only
- Some files may have multiple error categories
