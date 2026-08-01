# Parser Stdlib Coverage - Handoff Document

## Current Status

**Branch:** `feat/parser-stdlib-coverage`

**Coverage:** 76.8% (73/95 files clean)

**Progress:** +38.9 percentage points from 37.9% starting point

**Commits:** 36 commits (2c8bd10...1d659c6)

**Top Remaining Errors:**
- 194: expected '{' or ';' after declaration
- 155: expected a body member
- 102: expected a namespace member
- 26: expected a name
- 14: expected 'to' between connector ends
- 12: expected an expression

**Remaining Files:** 22/95 (mostly blocked by complex/unsupported patterns)


## Major Achievements

**Milestone:** 70% stdlib coverage achieved (Task 39, commit da947ce)

**Tasks Completed:** 41 tasks (Tasks 6-46)
- Task 6: 'end' feature modifier
- Task 7: 'subject'/'objective' usage keywords
- Task 8: Arrow invocation single-arg without parens
- Task 9: Connector end multiplicities
- Task 10: 'succession' connection keyword
- Task 11: Return parameter with initializer
- Task 12: Return statement usage kind + modifiers
- Task 13: Return modifiers after type
- Task 14: Body expression invocations
- Task 16: Typed body parameters
- Task 17: **'inv' keyword + parseConstraintBody infinite loop fix** (breakthrough)
- Task 18-20: Enum literal support (values, bare literals)
- Task 21: Keywords as identifiers
- Task 22-26: Parameter patterns, doc comments, implicit returns
- Task 27: Anonymous feature pattern + doc comment fix
- Task 28: Return parameter with multiplicity and value
- Task 29: Doc comments in body expressions
- Task 30: Requirement body improvements
- Task 31: Enum literal bodies and keyword names
- Task 32: 'step' usage keyword
- Task 33: 'expr' usage keyword (lambda parameters)
- Task 34: 'chain' feature modifier (partial)
- Task 36: Fix inv keyword regression
- Task 37: Anonymous feature modifiers + return param enhancements
- Task 38: Anonymous feature multiplicity support
- Task 39: Fix constraint usage parsing in calc body
- Task 40: **Relationship.Target feature chain support** (architectural change)
- Task 41: 'interaction' usage keyword
- Task 42: Fix anonymous connector identification
- Task 43: Comma-separated relationship targets in shorthand
- Task 44: Fix bool/predicate usage body parsing
- Task 45: Keywords as expression arguments
- Task 46: Body expression parameters with default values

**Key Architectural Fixes:**
1. **Relationship.Target feature chains** (Task 40): Changed from `*QualifiedName` to `Node` interface, created `parseRelationshipTarget()` helper to handle qualified names (`A::B::C`) and feature chains (`A.B.C`) without consuming body expressions
2. **parseConstraintBody infinite loop** (Task 17): Added offset-advancement safety check preventing infinite loop when expression parsing fails
3. **Enum literal pattern conflicts** (Tasks 31, 36, 39, 44): Multiple regressions fixed by excluding usage kind keywords from enum literal pattern matching
4. **Parser state corruption debugging** (Task 40): Discovered ParseExpression greediness issue - consumed function bodies as invocation arguments when relationship targets preceded body expressions


## Recent Work Summary (Tasks 27-46)

### Return Parameter Enhancements
- **Task 28:** Multiplicity with value: `return name [mult] = expr`
- **Task 37 Part B:** Multiplicity with body: `return ref result[0..*] { doc }`
- **Task 37 Part C:** Relationships support: `return verdict : Type :>> base;`
- **Task 30 Parts A-C:** Doc comments, subject with body, return with value+body, return in body member dispatch

### Anonymous Feature Patterns
- **Task 27:** Basic pattern with modifier: `private name : Type :>> rel { body }`
- **Task 37 Part A:** Modifier before name: `ref stateSpace: StateSpace;`
- **Task 38:** Multiplicity support: `ref explanation : Anything [0..1] { doc }`
- **Task 40:** Relationships with feature chains: `ref :>> x :> parent.field { body }`
  - Major architectural change: Relationship.Target from `*QualifiedName` to `Node`
  - Created parseRelationshipTarget() to handle both `::` and `.` separators

### Body Expression Improvements
- **Task 14:** Invocation syntax: `forAll { in i; expr }`
- **Task 16:** Typed parameters: `in fn : SampledFunction`
- **Task 29:** Doc comments in body expressions
- **Task 46:** Parameters with default values: `in name = expr`
  - Added Value field to ast.BodyParam
  - Constraint body disambiguation: detect `in` keyword, switch to body expression parsing

### Keyword Support Additions
- **Task 32:** 'step' usage keyword
- **Task 33:** 'expr' usage keyword (lambda/closure parameters)
- **Task 34:** 'chain' feature modifier (partial - chaining syntax without `=` not supported)
- **Task 41:** 'interaction' usage keyword
- **Task 45:** Keywords as expression arguments: `excluding(do)`, `then(entry)`

### Parser Bug Fixes
- **Task 36:** Inv keyword regression from Task 31 enum literal pattern
- **Task 39:** Constraint keyword consumed as enum literal in calc bodies
- **Task 42:** Anonymous connector identification + connector end feature chains
- **Task 43:** Comma-separated relationship targets: `feature redefines A, B, C`
  - Added lookahead to distinguish simple names (shorthand) from qualified names
- **Task 44:** Bool/predicate keywords excluded from enum literal pattern
  - Added constraint-style body parsing for UsageBool and UsagePredicate


## Known Issues & Blockers

### BLOCKED Patterns (Cannot implement with current architecture)

These patterns require significant architectural changes or are fundamentally unsupported:

1. **Feature chaining syntax without assignment operator**
   - **File:** ControlFunctions.kerml (2 errors)
   - **Pattern:** `private feature chain chains source.target;` (line 18)
   - **Issue:** Feature chaining `chains source.target` has no `=` operator between name and expression
   - **Status:** Deferred in Task 34 as TOO COMPLEX

2. **Multi-word usage kind keywords with modifiers**
   - **File:** UseCases.sysml (2 errors)
   - **Pattern:** `ref use case done: UseCase :>> done`
   - **Issue:** Two-word keyword `use case` combined with modifier prefix `ref`
   - **Status:** Multi-word kind keywords not fully supported

3. **RequireMember body with generic members**
   - **File:** Views.sysml (4 errors)
   - **Pattern:** `require viewpointSatisfactions { ref :>> ... }` inside satisfy body
   - **Issue:** RequireMember body expects requirement-specific keywords (subject/assume/require/actor), but contains generic features
   - **Status:** Task 30 Part E attempted, reverted due to architectural limitation

4. **Connector end + member hybrid syntax**
   - **File:** Items.sysml (3 errors)
   - **Pattern:** `end touches [0..*] item touchedItem :>> ...` (line 142-143)
   - **Issue:** Connector end declaration combined with feature declaration in single statement
   - **Status:** Task 35 attempted, deferred as TOO COMPLEX

5. **Type cast expressions ("as" keyword)**
   - **File:** CauseAndEffect.sysml (8 errors)
   - **Pattern:** `ref :>> baseType = causes as SysML::Usage;`
   - **Issue:** Type cast syntax `expr as Type` not supported in parser
   - **Status:** Requires expression grammar extension

6. **Named relationships with multiple types**
   - **File:** Actions.sysml (37 errors)
   - **Pattern:** `ref sentMessage :>> sentTransfer: MessageTransfer, MessageAction { ... }`
   - **Issue:** `:>> relationshipTarget: Type1, Type2` combines relationship with multi-typing
   - **Status:** Complex syntax requiring investigation

### Next Implementation Opportunities

Files with simpler patterns that might be addressable:

1. **State machine keywords** (Actions.sysml partial)
   - Errors: "unknown state keyword: first", "expected state keyword"
   - Investigation needed: State/transition parsing in behavior.go

2. **Observation.kerml** (3 errors)
   - Pattern 1: Body expression with no `in` parameters (only result expression)
   - Pattern 2: Multiple comma-separated relationship targets (may be fixed by Task 43)

3. **Binding syntax** (ControlPerformances.kerml, Transfers.kerml)
   - Pattern: `binding [mult] name of [mult] target`
   - Appears in multiple files, might have high impact


## Testing Infrastructure

### Stdlib Coverage Test
```bash
cd internal/core/libs
go test -run TestStdlibParserCoverage -v
```

Shows:
- Parse coverage percentage
- Top error patterns with counts
- Sample failures (first 5 files)
- Total diagnostics count

### Single File Detail Tests
Pattern: Create `test_<filename>_detail_test.go` with:
```go
func TestSingleFile_<Name>_Details(t *testing.T) {
    src := &embedSource{}
    data, err := src.Read("<path/to/file.sysml>")
    if err != nil {
        t.Fatal(err)
    }
    
    sf := source.New("<filename>", data)
    p := parser.New(sf)
    _ = p.ParseFile()
    
    if len(p.Diagnostics) > 0 {
        t.Logf("Parse diagnostics (%d):", len(p.Diagnostics))
        for i, d := range p.Diagnostics {
            if i >= 10 { break }
            text := sf.Text(d.Span)
            if len(text) > 50 {
                text = text[:50] + "..."
            }
            t.Logf("  [%d] offset %d: %s [near: %q]", i+1, d.Span.Offset, d.Message, text)
        }
    } else {
        t.Log("Parsed cleanly!")
    }
}
```

Useful for:
- Getting exact error offsets
- Seeing error context snippets
- Drilling into specific file failures

### Unit Tests for New Features
Always create parser unit tests in `internal/core/parser/`:
- `test_<feature>_test.go`
- Test both package-level and body-level parsing
- Test edge cases (anonymous vs named, with/without modifiers, etc.)
- Verify AST structure matches expectations

**Example test structure:**
```go
func TestFeaturePattern(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"basic", "feature x;", false},
        {"with type", "feature x : Type;", false},
        {"anonymous", "feature : Type;", false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ... parse and verify
        })
    }
}
```


## Key Parser Architecture Points

### parseDefUsage() - Entry point for definitions/usages
**Location:** `internal/core/parser/defusage.go:241`

Flow:
1. Parse feature modifiers (abstract, ref, in, out, readonly, derived, composite, end, etc.)
2. Check for special patterns (flow, usecase)
3. Check usage-only keywords (subject, objective, succession, inv, connector, satisfy, step, expr, interaction)
4. Check definition keywords vs fallback to usage
5. Parse `all` modifier (after keyword)
6. Parse `chain` modifier (after all, before secondary keyword)
7. Dispatch to `parseDefinition()` or `parseUsage()`

**Fallback logic:** If no keyword found:
- Check for modifiers + relationship pattern (anonymous feature)
- Check for name + multiplicity + modifiers (parameter pattern)
- Default to UsageAttribute

### parseUsage() - Parse usage declarations
**Location:** `internal/core/parser/defusage.go:462`

Special cases handled at start:
- **UsageSatisfy:** Custom syntax `satisfy requirement <ref> by <subj> { body }`
- **UsageConnector:** Skip identification if anonymous (`connector : Type from x to y`)
- **Flow shorthand:** Creates implicit flow ends
- **Succession:** No declaration name

Then standard flow:
- Typing relationship (if colon after keyword)
- Relationship shorthand detection (simple name vs qualified)
- Identification parsing
- Post-identification relationships
- Multiplicity
- Post-multiplicity modifiers
- Value parsing (`=` or `default` keyword)
- Tier B ends (connectors via parseConnectorFromTo, flows)
- Body parsing (kind-specific dispatch)

**Body dispatch:**
- UsageConstraint, UsageBool, UsagePredicate → constraint body or body expression (detect `in` keyword)
- UsageAction → parseActionBody
- UsageCalc, UsageFunction → parseCalcBody
- Default → parseDefUsageBody

### parseBodyMember() - Parse members inside def/usage bodies
**Location:** `internal/core/parser/defusage.go:638`

Checks in order:
1. Import/alias keywords
2. Result keyword (return) → dispatch to parseResultMember
3. Anonymous feature with modifier: `ref name : Type`
4. Enum literal pattern: `name = expr;`, `name;`, or `name { body }`
   - **Exclusion list:** subject, objective, succession, inv, connector, satisfy, step, expr, constraint, interaction, bool, assoc, struct, class, predicate
5. **Name-before-keyword pattern:** `<name> <keyword> { ... }`
   - Excludes usage-only keywords
   - Excludes direction keywords (in, out)
6. Generic declaration via `parseDeclaration()`

### parseCalcBody() - Parse calc/function bodies
**Location:** `internal/core/parser/behavior.go:11`

Handles:
- 'return' keyword → `parseResultMember()`
- Implicit return expressions (using `atExprStart()` heuristic)
- Generic body members (parameters, doc, etc.)

**Implicit return heuristic:** If `atExprStart()` but NOT a name-declaration pattern, parse as implicit return expression.

### parseRelationshipTarget() - Parse relationship targets (Task 40)
**Location:** `internal/core/parser/defusage.go:926`

Handles both qualified names and feature chains:
- Start with `parseQualifiedName()` (handles `A::B::C`)
- Extend with dot separator loop for feature chains (`A.B.C`)
- Returns QualifiedName or FeatureChainExpr wrapped in FeatureReference
- **Key:** Does NOT consume body expressions (avoids ParseExpression greediness)

Used by:
- `parseRelationships()` for relationship targets
- `parseConnectorEnd()` for connector end targets

### parseConstraintBody() - Parse constraint bodies
**Location:** `internal/core/parser/behavior.go:552`

**Safety check added (Task 17):** Prevents infinite loop when expression parsing fails:
```go
for !p.at(lexer.RBrace) && !p.atEOF() {
    before := p.peek().Span.Offset
    members = append(members, p.parseConstraintMember())
    if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
        p.advance() // force progress
    }
}
```


## Useful Patterns & Helpers

### atExprStart() - Check if token can start expression
**Location:** `internal/core/parser/expr.go:200`

Checks: name, literals (Decimal, Real, String, Star), `(`, `{`, keywords: null, true, false, new, if

Used for:
- Implicit return detection in parseCalcBody
- Body expression invocation detection

### atNameOrKeyword() - Accept names or keywords as identifiers
**Location:** `internal/core/parser/defusage.go` (helper)

Used for:
- Keyword-as-identifier support (Task 21)
- Enum literal names (can be keywords)
- **Note:** Some keywords excluded (usage-only, 'default')

### parseNameSegmentRelaxed() - Parse name allowing keywords
**Location:** `internal/core/parser/defusage.go`

Allows keywords as name segments in:
- Identification parsing
- Qualified name segments (after `::`)

### Keyword exclusion patterns

When adding new keywords that are usage/definition kinds:

1. **Add to keyword map:**
   - `usageKindKeywords` (if usage kind)
   - `definitionKindKeywords` (if definition kind)
   - Both (if both def and usage forms exist)

2. **Usage-only check (parseDefUsage line ~287):**
   ```go
   if kw == "subject" || kw == "objective" || kw == "succession" || 
      kw == "inv" || kw == "connector" || kw == "satisfy" || 
      kw == "step" || kw == "expr" || kw == "interaction" {
       // Parse as usage, no def form
   }
   ```

3. **Enum literal exclusion (parseBodyMember line ~857):**
   ```go
   isUsageOnlyKwForEnum := p.at(lexer.Keyword) && (
       p.peek().KeywordID == "subject" || ... || 
       p.peek().KeywordID == "constraint" || 
       p.peek().KeywordID == "interaction" || 
       p.peek().KeywordID == "bool" || 
       p.peek().KeywordID == "assoc" || 
       p.peek().KeywordID == "struct" || 
       p.peek().KeywordID == "class" || 
       p.peek().KeywordID == "predicate")
   ```

4. **Name-before-keyword exclusion (parseBodyMember line ~877):**
   ```go
   isUsageOnlyKw := p.at(lexer.Keyword) && (same list as above)
   isDirectionKw := kw == "in" || kw == "out"
   if !isUsageOnlyKw && !isDirectionKw && p.atNameOrKeyword() ...
   ```

5. **Special syntax handling:** If non-standard syntax (like satisfy, succession), add custom logic at start of parseUsage()

### Anonymous vs Named declarations

- **Anonymous:** No identification parsed, relationships consume target names
  - Pattern: `:>> target` - target is relationship reference, NOT declaration name
  - Example: `attribute :>> elements = (0, 0, 0);`

- **Named:** Identification parsed between relationships
  - Pattern: `name :>> target` - name is declaration, target is relationship
  - Example: `attribute myElements :>> elements = (0, 0, 0);`

- **Anonymous with modifier:**
  - Pattern: `ref :>> target : Type { body }`
  - Example: `ref :>> outgoingTransfers :> parent.field;`

### Relationship shorthand vs normal

**Shorthand** (Task 4, Task 43):
- Pattern: `feature redefines x` = `feature x redefines x`
- Detection: relationship keyword + SIMPLE name (not qualified, not feature chain)
- Lookahead: peekN(2) must NOT be `::` or `.`
- Comma-separated: `feature redefines A, B, C`

**Normal:**
- Pattern: `feature x redefines A::B::C` or `feature x redefines parent.field`
- Calls parseRelationships() which handles qualified names and feature chains


## Common Pitfalls

### 1. Keyword conflicts
Keywords can be identifiers in some contexts (Task 21 fix), but NOT all:
- **Allowed:** Declaration names, qualified name segments, expression arguments
- **Not allowed:** 'default' keyword has special syntax meaning (Task 24)
- **Pattern:** Check context before allowing keyword-as-identifier

### 2. Name-before-keyword ambiguity
`<keyword1> <keyword2>` could be:
- Declaration with keyword1 as kind: `feature redefines ...`
- Named declaration where keyword1 is name: `assert constraint { ... }`
- Requires careful exclusion logic and context checks

### 3. Enum literal pattern conflicts (Multiple task regressions)
**Problem:** Pattern `keyword { body }` matches enum literal check before usage parsing

**Symptoms:**
- "expected a body member" at valid usage keyword
- Keyword consumed as enum literal name
- Usage never parsed correctly

**Solution:** Maintain exclusion list in `isUsageOnlyKwForEnum` check (parseBodyMember line ~857)

**Affected keywords:** ALL usage kind keywords must be excluded:
- Usage-only: subject, objective, succession, inv, connector, satisfy, step, expr, constraint, interaction
- Both def+usage: bool, assoc, struct, class, predicate

**Regression history:**
- Task 31: Added atNameOrKeyword() to enum literal check → broke inv keyword
- Task 36: Fixed by excluding inv
- Task 39: Broke again with constraint keyword → fixed
- Task 44: Broke with bool keyword → fixed by excluding bool/assoc/struct/class/predicate

### 4. ParseExpression greediness (Task 40 critical bug)
**Problem:** ParseExpression consumes too much, including body expressions

**Example:**
```sysml
function abs specializes RationalFunctions::abs { in x: Integer[1]; ... }
```

When parsing relationship target `RationalFunctions::abs` followed by body `{ in ...}`:
- ParseExpression sees pattern `abs { in ...}` 
- Matches body expression invocation syntax (expr.go:353 check)
- Consumes ENTIRE function body as invocation argument
- Leaves nothing for body parser → catastrophic failure

**Solution:** Use `parseRelationshipTarget()` instead of `ParseExpression()` for:
- Relationship targets in parseRelationships
- Connector end targets in parseConnectorEnd

`parseRelationshipTarget()` handles qualified names + feature chains WITHOUT consuming body expressions.

### 5. Body parsing dispatch
Different usage kinds have specialized body parsers:
- Action → `parseActionBody` (entry/do/exit/state/transition)
- Calc/Function → `parseCalcBody` (parameters, implicit return)
- Constraint → `parseConstraintBody` (expressions) OR body expression (if starts with `in`)
- Bool/Predicate → same as Constraint (Task 44)
- Requirement → `parseRequirementBody` (subject/assume/require/actor)
- Default → `parseDefUsageBody` (generic members)

**Critical:** Constraint body disambiguation (Task 46):
- If body starts with `in` keyword: parse as body expression (typed constraint)
- Else: parse as constraint body (pure expressions)

### 6. Relationship vs identification ordering
Pre-identification relationships parse differently than post-identification:
- **Pre-identification:** `:`, `:>`, `:>>` relationship operators, typing comes first
- **Post-identification:** Full relationship keyword parsing, name already consumed
- **Shorthand:** Special pattern where relationship keyword + name creates implicit relationship

**Example flow:**
```sysml
feature : Type :>> base redefines x { ... }
         ↑      ↑         ↑
       typing  redef   additional
       (pre)   (pre)    (post-ident)
```

### 7. Testing both contexts
Always test new features at:
- **Package/namespace level** (top-level declarations)
- **Inside definition/usage bodies** (nested declarations)
- Parsing logic differs! parseBodyMember has different checks than parseDeclaration

**Example:**
- `feature x;` at package level → parseDeclaration
- `feature x;` inside struct body → parseBodyMember → may hit enum literal check first

### 8. Binary search limitations
When debugging file failures:
- **Cannot** chop file at arbitrary lines (leaves unclosed braces)
- **Cannot** rely on partial file parsing for bisection
- **Must** use incremental addition approach or isolated pattern extraction
- Cascading errors common (earlier failure corrupts parser state)

### 9. FeatureReference unwrapping
When using `ParseExpression()` or `parseRelationshipTarget()`:
- Names wrapped in FeatureReference nodes
- Code expecting `*QualifiedName` must unwrap:
  ```go
  if fr, ok := target.(*ast.FeatureReference); ok {
      target = fr.Name
  }
  ```
- Affects: relationship resolution, type checking, any AST traversal expecting QualifiedName

**Files requiring unwrapping (Task 40):**
- libs/record.go
- parser/defusage_test.go
- resolve/document.go
- passes/shape.go
- runtime/constraint.go
- semantics/typecheck.go
- semantics/model.go


## Git Workflow

Current branch has 36 commits with good granularity. Continue pattern:

**Commit style:**
- One commit per feature/fix
- Descriptive messages with:
  - `feat(parser):` or `fix(parser):` prefix
  - Brief description of what changed
  - Pattern/file it fixes if applicable

**Example commit messages:**
```
feat(parser): support feature chain relationship targets (Task 40)
fix(parser): exclude bool/assoc/struct/class/predicate from enum literal pattern (Task 44)
feat(parser): support body expression parameters with default values (Task 46)
```

**Commit range:** 2c8bd10 (Task 6: end modifier) → 1d659c6 (Task 46: body expr params)

**Branch status:**
- Clean history, all commits build successfully
- No merge conflicts
- Ready for PR or continued work

## Commands Reference

```bash
# Run stdlib coverage test
cd internal/core/libs && go test -run TestStdlibParserCoverage -v

# Run all parser tests  
cd internal/core/parser && go test

# Run specific test
cd internal/core/parser && go test -run TestAnonymousFeature -v

# Check git status
git status
git log --oneline -20

# Stage and commit
git add -A
git commit -m "feat(parser): <description>"

# View diff before commit
git diff
git diff --staged

# View specific commit
git show <commit-hash>
git show 1d659c6
```

## Goal

**Original Target:** 70%+ stdlib coverage (67/95 files)  
**Status:** ✅ **ACHIEVED** at Task 39 (commit da947ce)

**Current Coverage:** 76.8% (73/95 files clean)

**Remaining:** 22 files (23.2%)

**Analysis of remaining files:**
- ~6 files: BLOCKED by architectural limitations (feature chaining, multi-word keywords, RequireMember body, connector end hybrid, type cast, named multi-type relationships)
- ~16 files: Complex patterns requiring investigation (state machine keywords, binding syntax, etc.)

**Realistic next target:** 80% coverage (~76 files clean, +3 files)

**Estimated effort:**
- Low-hanging fruit mostly picked
- Remaining patterns increasingly complex
- Each additional file requires deeper investigation
- Diminishing returns on time investment

**Recommendation:**
- Current 76.8% coverage represents excellent parser maturity
- Most common stdlib patterns fully supported
- Remaining 23% are edge cases and architectural blockers
- Prioritize actual use cases over 100% stdlib coverage
- Document known limitations for users

**Session achievements:**
- +38.9 percentage points coverage increase
- 41 tasks completed
- 36 commits
- 4 major architectural fixes
- Solid foundation for production use

---

*Document last updated: 2026-08-01*  
*Coverage as of commit: 1d659c6*  
*Branch: feat/parser-stdlib-coverage*

