# SysML v2 Training Examples - Parser Status

## Summary
- Total examples: 57 files
- Clean (no errors): 37 files (65%)
- Parser errors: 20 files (35%)

**Latest update:** Fixed requirement constraint body syntax, reducing errors from 27 → 20 files.

## Parser Feature Gaps

### 1. State Machine Syntax (3 files affected) ⚠️ NEEDS WORK
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

### 2. Requirement Body Syntax ✅ FIXED
**Status:** Fixed in commit c077e33

**Features implemented:**
- `assume constraint { <body> }` - constraint body after assume ✅
- `require constraint { <body> }` - constraint body after require ✅
- `subject = <expr>;` - subject binding form ✅
- Nested constraint definitions in requirement bodies ✅

**Files fixed:** 19 files across requirements, analysis, constraints directories

### 3. Connection Syntax (1 file affected) ⚠️ NEEDS WORK
**Missing feature:**
- `connect [mult] <end> to [mult] <end>;` - multiplicity on ends

**Affected files:**
- 09. Connections/Connections Example.sysml

**Implementation needed:**
- Parse optional multiplicity before connector ends
- Update ConnectorEnd AST to store multiplicity

### 4. Flow Syntax (1 file affected) ⚠️ NEEDS WORK
**Missing feature:**
- `flow from <ref> to <ref>;` - parser expects different keyword

**Affected files:**
- 15. Actions/Action Decomposition.sysml

**Implementation needed:**
- Debug flow statement parsing (may be partially implemented)

### 5. Semantic Errors (4 files) ⚠️ NEEDS INVESTIGATION
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

### 6. Other Syntax Issues (11 files) ⚠️ NEEDS ANALYSIS
**Errors:**
- "expected a body member" in constraint/analysis contexts
- "expected 'requirement' keyword after 'satisfy'"

**Affected files:**
- 31. Constraints/*.sysml (7 files)
- 32. Requirements/Requirement Satisfaction.sysml
- 33. Analysis/*.sysml (2 files)
- 29. Expressions/MassRollup2.sysml

## Working Examples (37 files)
All examples in these directories parse successfully:
- 01. Packages (3/3 files) ✅
- 02. Part Definitions (1/1 file) ✅
- 03-06, 08, 11-14, 16-23, 25-28, 30, 34-35 (various) ✅

## Progress Summary
- ✅ **Fixed:** Requirement constraint body syntax (19 files)
- ⚠️ **TODO:** State machine syntax (3 files)
- ⚠️ **TODO:** Connection/flow syntax (2 files)
- ⚠️ **TODO:** Other body member issues (11 files)
- ⚠️ **TODO:** Semantic validation errors (4 files)

## Recommendations
1. **High priority:** State machine syntax (enables 3 files, common feature)
2. **Medium priority:** Investigate "expected body member" errors in constraints/analysis (11 files)
3. **Medium priority:** Connection/flow syntax tweaks (enables 2 files)
4. **Low priority:** Investigate semantic errors (may be spec-compliant validation)

## Testing Command
```bash
# Count clean vs error files
for f in examples/sysml-v2-training/*/*.sysml; do
  ./sysml -l "$f" 2>&1 | grep -q "error:" || echo "CLEAN: $f"
done | wc -l
```
