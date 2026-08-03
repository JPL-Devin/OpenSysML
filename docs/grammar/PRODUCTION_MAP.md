# Grammar Production Mapping

Maps SysML v2 Pilot Implementation grammar productions to Systemica Go parser functions.

**Status:**
- ✅ **Faithful**: Parser implements production accurately
- ⚠️ **Approximate**: Parser handles pattern but structure differs
- ❌ **TODO**: Not yet implemented
- 🔍 **Review**: Needs verification

## Top-Level Structure

| Grammar Production | Grammar File | Go Function | File | Status | Notes |
|-------------------|--------------|-------------|------|--------|-------|
| `RootNamespace` | KerML.xtext:37 | `ParseFile()` | parser.go:23 | ✅ Faithful | Entry point |
| `Namespace` | KerML.xtext:119 | `parseNamespace()` | namespace.go | ✅ Faithful | Package/namespace bodies |
| `Package` | KerML.xtext:279 | `parseNamespace()` | namespace.go | ✅ Faithful | Unified with namespace |
| `LibraryPackage` | KerML.xtext:284 | `parseNamespace()` | namespace.go | ✅ Faithful | Standard/library prefix |

## Declarations

### Definitions

| Grammar Production | Grammar File | Go Function | File | Status | Notes |
|-------------------|--------------|-------------|------|--------|-------|
| `Type` | KerML.xtext:319 | `parseDefinition()` | defusage.go:730 | ✅ Faithful | All def kinds |
| `Classifier` | KerML.xtext:329 | `parseDefinition()` | defusage.go:730 | ✅ Faithful | Classifiers |
| `Feature` | KerML.xtext:379 | `parseUsage()` | defusage.go:1192 | ⚠️ Approximate | Feature as usage/def |
| `DataType` | KerML.xtext:425 | `parseDefinition()` | defusage.go:730 | ✅ Faithful | Datatype defs |
| `Class` | KerML.xtext:459 | `parseDefinition()` | defusage.go:730 | ✅ Faithful | Class defs |
| `Structure` | KerML.xtext:474 | `parseDefinition()` | defusage.go:730 | ✅ Faithful | Structure defs |

### Usages

| Grammar Production | Grammar File | Go Function | File | Status | Notes |
|-------------------|--------------|-------------|------|--------|-------|
| `Usage` | KerML.xtext:535 | `parseUsage()` | defusage.go:1192 | ✅ Faithful | Generic usage |
| `ReferenceUsage` | KerML.xtext:593 | `parseUsage()` | defusage.go:1192 | ✅ Faithful | Ref prefix |
| `AttributeUsage` | KerML.xtext:632 | `parseUsage()` | defusage.go:1192 | ✅ Faithful | Attribute usage |
| `ItemUsage` | SysML.xtext:50 | `parseUsage()` | defusage.go:1192 | ✅ Faithful | Item usage |
| `PartUsage` | SysML.xtext:64 | `parseUsage()` | defusage.go:1192 | ✅ Faithful | Part usage |
| `PortUsage` | SysML.xtext:116 | `parseUsage()` | defusage.go:1192 | ✅ Faithful | Port usage |

## Relationships

| Grammar Production | Grammar File | Go Function | File | Status | Notes |
|-------------------|--------------|-------------|------|--------|-------|
| `Dependency` | KerML.xtext:67 | `parseDependency()` | namespace.go:346 | ✅ Faithful | Dependency relations |
| `Specialization` | KerML.xtext:687 | `parseRelationships()` | defusage.go:2470 | ✅ Faithful | :> specializes/subsets |
| `FeatureTyping` | KerML.xtext:726 | `parseRelationships()` | defusage.go:2470 | ✅ Faithful | : typed by |
| `Subsetting` | KerML.xtext:735 | `parseRelationships()` | defusage.go:2470 | ✅ Faithful | :> subsets |
| `Redefinition` | KerML.xtext:744 | `parseRelationships()` | defusage.go:2470 | ✅ Faithful | :>> redefines |
| `FeatureInverting` | KerML.xtext:753 | `parseRelationships()` | defusage.go:2470 | ✅ Faithful | ~> inverting |

## Members & Body Elements

| Grammar Production | Grammar File | Go Function | File | Status | Notes |
|-------------------|--------------|-------------|------|--------|-------|
| `NamespaceMember` | KerML.xtext:150 | `parseMember()` | namespace.go:261 | ✅ Faithful | Namespace members |
| `FeatureMember` | KerML.xtext:376 | `parseBodyMember()` | defusage.go:1438 | ✅ Faithful | Feature members |
| `Import` | KerML.xtext:174 | `parseImport()` | namespace.go:389 | ✅ Faithful | Import statements |
| `AliasMember` | KerML.xtext:162 | `parseAlias()` | namespace.go:464 | ✅ Faithful | Alias declarations |

## Behavioral Elements

| Grammar Production | Grammar File | Go Function | File | Status | Notes |
|-------------------|--------------|-------------|------|--------|-------|
| `ActionUsage` | SysML.xtext:204 | `parseUsage()` | defusage.go:1192 | ✅ Faithful | Action usage |
| `ActionDefinition` | SysML.xtext:246 | `parseDefinition()` | defusage.go:730 | ✅ Faithful | Action def |
| `StateUsage` | SysML.xtext:294 | `parseUsage()` | defusage.go:1192 | ✅ Faithful | State usage |
| `StateDefinition` | SysML.xtext:318 | `parseDefinition()` | defusage.go:730 | ✅ Faithful | State def |
| `TransitionUsage` | SysML.xtext:367 | `parseSuccessionStatement()` | behavior.go:1901 | ⚠️ Approximate | first/then syntax |

## Expressions

| Grammar Production | Grammar File | Go Function | File | Status | Notes |
|-------------------|--------------|-------------|------|--------|-------|
| `Expression` | KerML.xtext:814 | `parseExpression()` | expr.go:15 | ✅ Faithful | Base expressions |
| `ConditionalExpression` | expressions.xtext | `parseConditional()` | expr.go | ✅ Faithful | if ? then : else |
| `NullCoalescingExpression` | expressions.xtext | `parseNullCoalescing()` | expr.go | ✅ Faithful | ?? operator |
| `ImpliesExpression` | expressions.xtext | `parseBinary()` | expr.go | ✅ Faithful | implies operator |

## Comments

| Grammar Production | Grammar File | Go Function | File | Status | Notes |
|-------------------|--------------|-------------|------|--------|-------|
| `Comment` | KerML.xtext:94 | `parseComment()` | namespace.go:509 | ✅ Faithful | Comment annotations |
| `Documentation` | KerML.xtext:103 | `parseDocumentation()` | namespace.go:529 | ✅ Faithful | Doc annotations |
| `TextualRepresentation` | KerML.xtext:111 | `parseTextualRepresentation()` | namespace.go:549 | ✅ Faithful | Rep annotations |

## Key Findings

### Grammar Coverage
- **Core structure**: ✅ Faithful implementation (namespaces, packages, imports)
- **Definitions/Usages**: ✅ Unified parsing via keyword dispatch
- **Relationships**: ✅ All relationship kinds supported
- **Expressions**: ✅ Full expression grammar
- **Behavioral**: ⚠️ State transitions approximate (first/then vs AST succession)

### Parser Strengths
1. **Keyword-driven dispatch**: Flexible, handles all def/usage kinds uniformly
2. **Relationship parsing**: Single unified function handles all relationship operators
3. **Member parsing**: Body/namespace members share infrastructure

### Areas for Review
1. **State transitions**: `first X then Y` syntax maps to Connector/ConnectorEnd AST, not dedicated Transition node
2. **Feature chains**: Handled as FeatureChainExpr, verify alignment with grammar's chain production
3. **Multiplicities**: Parser accepts multiplicity in multiple positions (before/after relationships) - grammar may be more restrictive

## Phase 3 Fixes Mapped
- **Anonymous subjects**: Grammar allows `subject: Type;` (verified)
- **Constraint returns**: Grammar allows return members in constraint bodies (verified)
- **Parameter multiplicity**: Grammar allows `in name[mult]: Type` and `in :> target[mult]` (both supported)
- **Allocation bodies**: Grammar distinguishes connector form vs general allocation (fixed)

## Next Steps
1. 🔍 Deep-dive verification of behavioral constructs (actions, states, transitions)
2. 🔍 Expression precedence vs grammar precedence table
3. 🔍 Multiplicity placement rules (grammar spec vs parser flexibility)
4. ✅ Document that `:>` disambiguation (specializes vs subsets) is context-sensitive grammar, not semantic
