# SysML v2 Training Examples - Parser Status

## Summary
- Total examples: 57 files
- Clean (no errors): 30 files (53%)
- Parser errors: 27 files (47%)

## Parser Feature Gaps

### 1. State Machine Syntax (8 files affected)
**Missing features:**
- State transitions: `first <state> then <state>;`
- Accept transitions: `accept <signal> then <state>;`
- State actions: `entry <action>`, `do action <action>`, `exit action <action>`

**Affected files:**
- 24. States/State Actions.sysml
- 24. States/State Decomposition-1.sysml
- 24. States/State Decomposition-2.sysml

**Implementation needed:**
- Extend `isStateKeyword()` to recognize `first`, `accept`, `transition`
- Add handlers in `parseStateMember()` for succession/transition syntax
- Parse `entry`/`do`/`exit` with action reference (not just block)

### 2. Requirement Body Syntax (19 files affected)
**Missing features:**
- `assume constraint { <body> }` - constraint body after assume
- `require constraint { <body> }` - constraint body after require
- Nested constraint definitions in requirement bodies

**Affected files:**
- 32. Requirements/*.sysml (4 files)
- 33. Analysis/*.sysml (3 files)  
- 31. Constraints/*.sysml (10 files)
- 30. Calculations/*.sysml (1 file)
- 29. Expressions/*.sysml (1 file)

**Implementation needed:**
- Modify `parseAssumeMember()` to check for `constraint` keyword
- Parse constraint body (with doc, nested expressions)
- Same for `parseRequireMember()`

### 3. Connection Syntax (1 file affected)
**Missing feature:**
- `connect [mult] <end> to [mult] <end>;` - multiplicity on ends

**Affected files:**
- 09. Connections/Connections Example.sysml

**Implementation needed:**
- Parse optional multiplicity before connector ends
- Update ConnectorEnd AST to store multiplicity

### 4. Flow Syntax (1 file affected)
**Missing feature:**
- `flow from <ref> to <ref>;` - parser expects different keyword

**Affected files:**
- 15. Actions/Action Decomposition.sysml

**Implementation needed:**
- Debug flow statement parsing (may be partially implemented)

### 5. Semantic Errors (6 files)
**Issue:**
- `item cannot be typed by partDef (kind mismatch)`
- `<symbol> participates in specialization cycle`
- `<symbol> redefines <symbol>, but <symbol> is not inherited member`

**Affected files:**
- 10. Ports/Port Example.sysml
- 10. Ports/Port Conjugation Example.sysml  
- 07. Parts/Parts Example-1.sysml
- 07. Parts/Parts Example-2.sysml

**Note:** These are semantic validation errors, not parser issues. May indicate model issues or overly strict checks.

## Working Examples (30 files)
All examples in these directories parse successfully:
- 01. Packages (3/3 files)
- 02. Part Definitions (1/1 file)  
- Plus many others

## Recommendations
1. **High priority:** State machine syntax (enables 3 files, common feature)
2. **High priority:** Requirement constraint bodies (enables 19 files)
3. **Medium priority:** Connection/flow syntax tweaks (enables 2 files)
4. **Low priority:** Investigate semantic errors (may be spec-compliant validation)

## Testing Command
```bash
# Count clean vs error files
for f in examples/sysml-v2-training/*/*.sysml; do
  ./sysml -l "$f" 2>&1 | grep -q "error:" || echo "CLEAN: $f"
done | wc -l
```
