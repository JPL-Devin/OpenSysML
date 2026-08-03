# Phase 3: Unified Member Parsing (Proper Implementation)

## Problem

Current parser has 20+ body-specific member parsers with keyword whitelists:
- `parseRequirementMember`: Dispatches on subject/assume/require/actor, **terminal error** otherwise
- `parseConstraintBody`: Whitelist checks for return/doc/assert/assume, fallback to expression
- `parseActionMember`: Dispatches on if/perform/send/accept, fallback to parseBodyMember
- `parseStateMember`: Dispatches on entry/do/exit/first/accept/state, fallback to parseBodyMember

**Root cause**: Each specialized body parser maintains its own keyword whitelist and error handling. Any valid-but-unanticipated construct causes terminal error.

## Design: parseBodyMemberUnified

### Strategy: Try-Parse with Backtracking

```go
func (p *Parser) parseBodyMemberUnified() ast.Node {
    start := p.peek().Span.Offset
    
    // Try general declaration/usage first (covers 90% of cases)
    if node := p.tryParseDeclaration(); node != nil {
        return node
    }
    
    // Context-specific specialized members (statements, expressions)
    // These are genuinely specialized syntax, not just keyword dispatch
    return p.parseSpecializedMember()
}
```

### tryParseDeclaration

Attempts to parse without consuming tokens on failure:

```go
func (p *Parser) tryParseDeclaration() ast.Node {
    checkpoint := p.checkpoint()
    
    node := p.parseDeclaration()
    
    // Check if parse succeeded (not ErrorNode, advanced past start)
    if _, isError := node.(*ast.ErrorNode); isError {
        p.restore(checkpoint)
        return nil
    }
    
    return node
}
```

### parseSpecializedMember

Context-specific fallback (no terminal errors):

```go
func (p *Parser) parseSpecializedMember() ast.Node {
    // Context provided by caller (requirement/constraint/action/state)
    // Return appropriate specialized node or generic expression
    
    // Example for constraint context:
    return p.parseConstraintExpression() // assert/assume or bare expression
    
    // Example for action context:
    return p.parseStatementOrExpression() // if/send/accept or expression
}
```

## Migration Plan

### Step 1: Add Checkpoint/Restore

Parser needs backtracking support:

```go
type parseCheckpoint struct {
    pos        int
    token      lexer.Token
    errorCount int
}

func (p *Parser) checkpoint() parseCheckpoint {
    return parseCheckpoint{
        pos:        p.pos,
        token:      p.current,
        errorCount: len(p.diagnostics),
    }
}

func (p *Parser) restore(cp parseCheckpoint) {
    p.pos = cp.pos
    p.current = cp.token
    p.diagnostics = p.diagnostics[:cp.errorCount]
}
```

### Step 2: Implement tryParseDeclaration

### Step 3: Convert One Body Parser

Start with parseRequirementMember (has terminal error):

**Before**:
```go
func (p *Parser) parseRequirementMember() ast.Node {
    if p.acceptKeyword("subject") {
        return p.parseSubjectMember(start)
    }
    // ... other keywords ...
    if p.atDefUsageStart() {
        return p.parseBodyMember()
    }
    p.error(..., "expected 'subject', 'assume', ...") // TERMINAL ERROR
    return &ast.ErrorNode{...}
}
```

**After**:
```go
func (p *Parser) parseRequirementMember() ast.Node {
    start := p.peek().Span.Offset
    
    // Try general declaration first
    if node := p.tryParseDeclaration(); node != nil {
        return node
    }
    
    // Requirement-specific expressions (no terminal error)
    return p.parseRequirementExpression() // handles subject/assume/require/actor
}
```

### Step 4: Remove Debug Cruft

Delete lines 1533-1535 in behavior.go (unused debug variable).

### Step 5: Verify

- Stdlib: 94/94 clean
- Golden ASTs: all pass
- Training examples: parse rate should improve
- Negative tests: still reject garbage

## Success Criteria

1. ✅ No terminal errors in any body parser (search for "expected.*keyword.*body")
2. ✅ parseBodyMemberUnified used by all *Body functions
3. ✅ Stdlib 94/94 clean maintained
4. ✅ Golden AST suite passes
5. ✅ Can parse legal constructs without keyword enumeration

## Files to Modify

- `internal/core/parser/parser.go`: Add checkpoint/restore
- `internal/core/parser/namespace.go`: Add tryParseDeclaration
- `internal/core/parser/behavior.go`: Convert all *Member functions
- `internal/core/parser/defusage.go`: Update parseBodyMember to be fallback-only

## Estimated Scope

- Checkpoint/restore: ~30 lines
- tryParseDeclaration: ~20 lines
- Requirement member conversion: ~30 lines (removal of whitelist)
- 5 other body parsers: ~150 lines total
- **Total**: ~230 lines changed, ~50 lines removed

## Risk

**Medium**. Backtracking parser changes can cause infinite loops if not careful. Mitigation:
- Add loop guards in all *Body functions (already exist at line 1465-1467 pattern)
- Checkpoint only covers single member parse, not full body
- Test incrementally: convert one body parser at a time
