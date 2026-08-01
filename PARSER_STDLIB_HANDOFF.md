# Parser Stdlib Coverage - Handoff Document

## Current Status

**Branch:** `feat/parser-stdlib-coverage`

**Coverage:** 62.1% (59/95 files clean)

**Progress:** +24.2 percentage points from 37.9% starting point

**Top Remaining Errors:**
- 407: expected '{' or ';' after declaration
- 262: expected a namespace member  
- 256: expected a body member
- 48: expected an expression

## Recent Work Completed (Tasks 22-26)

### Task 22: Parameter without type + 'default' keyword + implicit return
**Commit:** ee46f39
- Added `isPostModifierKeyword()` helper for ordered/nonunique detection
- `parseDefUsage` fallback: check name+multiplicity/modifiers pattern (`in seq[1..*] ordered;`)
- `parseUsage`: accept 'default' keyword for feature values (in addition to '=')
- `parseCalcBody`: detect implicit return expressions (heuristic: name+arrow pattern)
- Pattern: calc def with parameters + final expression without 'return' keyword
- **+3 files:** Interfaces.sysml, etc.

### Task 23: Doc in constraint bodies + inline specialization detection
**Commit:** f39eab3
- `parseConstraintBody`: check for 'doc' keyword, call `parseDocumentation`
- `parseBase`: only treat 'name {' as body expr invocation if '{' followed by 'in' keyword
- Fixes inline specialization pattern: 'TypeName { doc }' (not body expression)
- **+3 files:** Constraints.sysml, Time.sysml, etc.
- Expression errors: 70→48 (-22, 31% reduction)

### Task 24: Exclude 'default' keyword from identification
**Commit:** 50fcb24
- `parseIdentification`: check for 'default' keyword, skip name parsing if found
- **Bug fix:** Task 21 allowed all keywords as names, but 'default' has special syntax
- Pattern: `:>> target default (tuple) { body }` now parses correctly
- **+2 files:** ISQSpaceTime.sysml, etc.

### Task 25: 'if' conditional as implicit return expression  
**Commit:** d5b7ce2
- `atExprStart`: add 'if' keyword check (conditional expressions)
- `parseCalcBody`: use `atExprStart()` for implicit return detection
- Improved heuristic: `atExprStart + !isNameDecl` pattern
- Handles: `function { if cond ? then else else }` without 'return' keyword
- **+1 file:** CollectionFunctions.kerml
- Body member errors: 260→256 (-4)

### Task 26: 'satisfy' usage keyword (PARTIAL - incomplete)
**Commit:** c386b67
- Added `UsageSatisfy` AST enum and String() case
- Added 'satisfy' to `usageKindKeywords` map
- `parseUsage`: special handling for `satisfy requirement <ref> by <subj> { body }`
- `parseBodyMember`: exclude usage-only keywords from name-before-keyword pattern
- **Views.sysml:** 5→2 errors (progress but not complete)
- **INCOMPLETE:** 'require' usage inside satisfy body not yet supported

## Known Issues & Next Steps

### PRIORITY 1: Complete 'satisfy' support
**File:** `Systems Library/Views.sysml` (2 errors remaining)

**Problem:** Error at "require" keyword inside satisfy body (line 43):
```sysml
satisfy requirement viewpointConformance by that {
    require viewpointSatisfactions {  // ← ERROR: "expected a body member"
        doc /* ... */
    }
}
```

**Root cause:** "require" keyword not in `usageKindKeywords` map. It's only checked in `isRequirementMemberKeyword()` for requirement body parsing.

**Solution:**
1. Check if "require" needs new AST enum (UsageRequire?) or if it's a RequireMember node type
2. Add to `usageKindKeywords` if it's a usage kind, OR
3. Update `parseBodyMember` to handle requirement-specific keywords ("require", "assume", "subject", "actor")
4. May need special body parsing for satisfy usages (use `parseRequirementBody` instead of `parseDefUsageBody`)

**Files to check:**
- `internal/core/parser/behavior.go` - `parseRequirementBody()` and `isRequirementMemberKeyword()`
- `internal/core/parser/defusage.go` - `parseUsage()` UsageSatisfy case (line ~476)

### PRIORITY 2: Investigate declaration errors (407 occurrences)

**Sample failures:**
- `Kernel Libraries/Kernel Function Library/ControlFunctions.kerml` (79 diagnostics)
- `Kernel Libraries/Kernel Semantic Library/KerML.kerml` (136 diagnostics)

**Pattern:** "expected '{' or ';' after declaration" errors are widespread. Need systematic investigation:
1. Run detailed diagnostic on one file: `test_controlfunctions_detail_test.go`
2. Identify common patterns causing the error
3. Check if it's:
   - Missing keyword support
   - Incorrect relationship parsing
   - Body/semicolon terminator handling
   - Special syntax not yet supported

### PRIORITY 3: State machine keywords

**Sample failures:**
- `Systems Library/Actions.sysml` (26 diagnostics)
  - "expected 'to' after transition source"
  - "unknown state keyword: first"
  - "expected state keyword (entry/do/exit/state/transition)"

**Investigation needed:**
- State/transition parsing in `behavior.go`
- "first" keyword support
- Transition syntax variations

## Testing Infrastructure

### Stdlib Coverage Test
```bash
cd internal/core/libs
go test -run TestStdlib -v
```

Shows:
- Parse coverage percentage
- Top error patterns with counts
- Sample failures (first 5 files)
- Category breakdown (Domain/Kernel/Systems libraries)

### Single File Detail Tests
Pattern: Create `test_<filename>_detail_test.go` with:
```go
func TestSingleFile_<Name>_Details(t *testing.T) {
    src := &embedSource{}
    data, err := src.Read("<path/to/file.sysml>")
    // ... parse and show all diagnostics with offsets
}
```

Useful for drilling into specific failures.

### Unit Tests for New Features
Always create parser unit tests in `internal/core/parser/`:
- `test_<feature>_test.go`
- Test both package-level and body-level parsing
- Test edge cases

## Key Parser Architecture Points

### parseDefUsage() - Entry point for definitions/usages
**Location:** `internal/core/parser/defusage.go:241`

Flow:
1. Parse feature modifiers (abstract, ref, in, out, etc.)
2. Check for special patterns (flow, usecase)
3. Check usage-only keywords (subject, objective, succession, inv, connector, **satisfy**)
4. Check definition keywords vs fallback to usage
5. Parse `all` modifier
6. Dispatch to `parseDefinition()` or `parseUsage()`

### parseUsage() - Parse usage declarations
**Location:** `internal/core/parser/defusage.go:462`

Special cases handled at start:
- **UsageSatisfy:** Custom syntax `satisfy requirement <ref> by <subj> { body }`
- Flow shorthand
- Succession (no declaration name)

Then:
- Relationship parsing (pre-identification)
- Identification parsing
- Post-identification relationships
- Multiplicity
- Post-multiplicity modifiers
- Value parsing (= or default)
- Tier B ends (connectors, flows)
- Body parsing (kind-specific dispatch)

### parseBodyMember() - Parse members inside def/usage bodies
**Location:** `internal/core/parser/defusage.go:638`

Checks in order:
1. Import/alias keywords
2. Enum literal pattern: `name = expr;` or `name;`
3. **Name-before-keyword pattern:** `<name> <keyword> { ... }`
   - Excludes usage-only keywords (subject, objective, succession, inv, connector, **satisfy**)
4. Generic declaration via `parseDeclaration()`

### parseCalcBody() - Parse calc/function bodies
**Location:** `internal/core/parser/behavior.go:11`

Handles:
- 'return' keyword → `parseResultMember()`
- Implicit return expressions (using `atExprStart()` heuristic)
- Generic body members (parameters, doc, etc.)

**Heuristic:** If `atExprStart()` but NOT name-declaration pattern, parse as implicit return expression.

## Useful Patterns & Helpers

### atExprStart() - Check if token can start expression
**Location:** `internal/core/parser/expr.go:200`

Currently checks: name, literals, `(`, `{`, keywords: null, true, false, new, **if**

### Keyword exclusion patterns
When adding new keywords that are usage/definition kinds:
1. Add to `usageKindKeywords` or `definitionKindKeywords` map
2. If usage-only (no def form), add to usage-only check in `parseDefUsage` (line ~270)
3. Exclude from name-before-keyword pattern in `parseBodyMember` if needed (line ~724)
4. Add special syntax handling in `parseUsage()` if non-standard (like satisfy, flow, succession)

### Anonymous vs Named declarations
- Anonymous: No identification parsed, relationships consume target names
- Named: Identification parsed between relationships
- Pattern `:>> target` - target is relationship reference, NOT declaration name
- Pattern `name :>> target` - name is declaration, target is relationship

## Common Pitfalls

1. **Keyword conflicts:** Keywords can be identifiers in some contexts (Task 21 fix), but NOT all (Task 24 'default' fix)

2. **Name-before-keyword ambiguity:** `<keyword1> <keyword2>` could be:
   - Declaration with keyword1 kind
   - Named declaration where keyword1 is name, keyword2 is kind
   - Requires careful exclusion logic

3. **Body parsing dispatch:** Different usage kinds have specialized body parsers:
   - Action → `parseActionBody`
   - Calc → `parseCalcBody`  
   - Constraint → `parseConstraintBody`
   - Requirement → `parseRequirementBody`
   - Default → `parseDefUsageBody`

4. **Relationship vs identification ordering:** Pre-identification relationships parse differently than post-identification. Symbolic operators (`:`, `:>`, `:>>`) are relationships. Keywords like "requirement" in satisfy are NOT automatically relationships.

5. **Testing both contexts:** Always test new features at:
   - Package/namespace level
   - Inside definition/usage bodies
   - Parsing logic can differ!

## Git Workflow

Current branch has 18 commits with good granularity. Continue pattern:
- One commit per feature/fix
- Descriptive commit messages with:
  - feat/fix prefix
  - Brief description
  - Coverage impact (+N files)
  - Error reduction if applicable

## Commands Reference

```bash
# Run stdlib coverage test
cd internal/core/libs && go test -run TestStdlib -v

# Run all parser tests
cd internal/core/parser && go test

# Run specific test
cd internal/core/parser && go test -run TestSatisfy -v

# Check git status
git status
git log --oneline -10

# Stage and commit
git add -A
git commit -m "feat(parser): <description>"
```

## Goal

**Target:** 70%+ stdlib coverage (67/95 files)

**Estimated:** ~8 more files needed, 15-20 issues to fix

**Strategy:** 
1. Fix high-impact issues (affecting multiple files)
2. Target files with few errors (2-5 diagnostics)
3. Investigate systematic patterns causing declaration errors

Good luck! The parser is in great shape - solid foundation to build on.
