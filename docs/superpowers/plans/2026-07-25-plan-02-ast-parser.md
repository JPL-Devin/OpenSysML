# Plan 02: AST & Parser (Expressions + Namespace Core) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the AST node framework and a hand-written recursive-descent parser covering the full KerMLExpressions expression grammar plus the namespace-core declaration grammar (root namespace, packages/namespaces, imports, aliases, dependencies, comments/docs/reps), producing an immutable concrete syntax tree with error recovery.

**Architecture:** Two packages. `internal/core/ast` defines immutable CST node types — every node carries a `source.Span`, semantics live in later side tables (not here). `internal/core/parser` drives a hand-written recursive descent parser over the `lexer.Lexer` token stream: one method per grammar production, a Pratt/precedence-climbing sub-parser for the expression operator ladder, single-token lookahead with a small buffer, and panic-mode error recovery that always produces a tree (error nodes, never bail). Parsing is syntax-only; name resolution/typing are out of scope (Plans 3+).

**Tech Stack:** Go 1.25, standard library only. Consumes `internal/core/lexer` (Task-1-of-Plan-1 `Lexer`, `Token`, `Kind`) and `internal/core/source` (`Span`, `SourceFile`). Tests: table-driven unit tests + golden-file AST dumps (S-expression form) under `testdata/parse/`.

---

## Scope

**In scope (Plan 2):**
- AST framework: `Node` interface (`Span() source.Span`), base node embedding, trivia attachment (leading/trailing trivia tokens for doc-comment hover), an `ErrorNode` for unparseable spans, and an S-expression dumper for golden tests.
- AST node kinds for: root namespace, `Namespace`, `Package`, `LibraryPackage`, membership wrappers (`OwningMembership` with visibility), `Import` (membership/namespace/recursive), `Alias`, `Dependency`, `Comment`, `Documentation`, `TextualRepresentation`, `PrefixMetadataMember`/annotation reference, `QualifiedName`, `Identification`.
- AST node kinds for the **full** expression grammar: literals (bool/string/integer/real/infinity), null, feature-reference, metadata-access, invocation (positional + named args), constructor (`new`), body `{ ... }`, sequence, and `OperatorExpression` with an operator field covering every operator in the ladder, plus `FeatureChainExpression`, `IndexExpression`, `CollectExpression`, `SelectExpression`.
- Parser: recursive-descent driver, token buffer, diagnostic list, error nodes; parse methods for every in-scope production; Pratt expression sub-parser; postfix chain/index/invoke/collect/select; panic-mode recovery with sync tokens (`;`, `}`, top-level keywords) + missing-token insertion for `}`/`;`.
- Integration: parse real fixtures (the Plan-1 `testdata/lex/basic.*` files parse without error nodes) + golden AST dumps.

**Explicitly deferred to a later plan (Plan 2b / renumbered):**
- The SysML/KerML **definition & usage taxonomy**: `Type`, `Classifier`, `Class`, `Structure`, `Metaclass`, `DataType`, `Association`, `Behavior`, `Function`, `Predicate`, `Feature`, `Step`, `Connector`, `BindingConnector`, `Succession`, `Flow`, and all SysML `*Definition`/`*Usage` kinds (part/attribute/port/action/state/connection/constraint/requirement/…), specialization/redefinition/subsetting relationship declarations, multiplicity, feature-value/typing parts.
- Consequence: in Plan 2, a namespace/package body admits members that are namespaces, packages, imports, aliases, dependencies, comments/docs/reps, and `filter` expressions. Encountering a definition keyword (e.g. `part`, `type`, `feature`) that is out of scope produces a diagnostic + `ErrorNode` spanning that member (recovery), NOT a parse abort. This keeps Plan 2 a coherent, testable increment.
- Name resolution, typing, validation passes: Plans 3+.

## File Structure

- Create `internal/core/ast/node.go` — `Node` interface, `NodeBase` (embeds `source.Span`), `Trivia`, `ErrorNode`.
- Create `internal/core/ast/namespace.go` — namespace/package/membership/import/alias/dependency/comment/doc/rep node types + `QualifiedName`, `Identification`.
- Create `internal/core/ast/expr.go` — all expression node types + `OperatorKind` enum.
- Create `internal/core/ast/dump.go` — `Dump(Node) string` S-expression serializer for golden tests.
- Create `internal/core/parser/parser.go` — `Parser` struct, token buffer, `New`, `ParseFile`, lookahead/consume/expect helpers, diagnostic recording, sync/recovery.
- Create `internal/core/parser/namespace.go` — parse methods for namespace-core productions.
- Create `internal/core/parser/expr.go` — Pratt expression sub-parser + postfix + primary/base.
- Create `internal/core/parser/diagnostic.go` — `Diagnostic{Span, Message}` type (parser-local; unified with passes in Plan 4).
- Tests alongside each file (`*_test.go`) + `internal/core/parser/integration_test.go`.
- Fixtures: `testdata/parse/*.sysml` + `testdata/parse/*.golden`.

One responsibility per file; parser split by production family (driver / declarations / expressions) so each stays focused and reviewable.

## Grammar Reference

Authoritative source: pilot Xtext, verified against on-disk files.

**Expression precedence ladder** (from `org.omg.kerml.expressions.xtext/.../KerMLExpressions.xtext`, low→high binding). Each level is left-associative unless noted:
1. Conditional: `if C ? A else B` (ternary; `if` is `ConditionalOperator`).
2. Null-coalescing: `??`.
3. Implies: `implies`.
4. Or: `|` (operand) and `or` (reference).
5. Xor: `xor`.
6. And: `&` (operand) and `and` (reference).
7. Equality: `==` `!=` `===` `!==`.
8. Classification (postfix-operator): `hastype` `istype` `@` (test), `@@` (meta test), `as` (cast), `meta` (meta cast) — RHS is a type reference/result.
9. Relational: `<` `>` `<=` `>=`.
10. Range: `..` (single, optional).
11. Additive: `+` `-`.
12. Multiplicative: `*` `/` `%`.
13. Exponentiation: `**` `^` — **right-associative**.
14. Unary (prefix): `+` `-` `~` `not`.
15. Extent (prefix): `all`.
16. Primary: base expression followed by chains/postfix.

**Primary/postfix** (`PrimaryExpression`): base, then optional `.` feature-chain member, then a repeatable postfix group of one of: `#` `(` seq `)` (index), `[` seq `]` (operator-index), `->` type-member ( body | funcref | argList ) (invocation), `.` body (collect), `.?` body (select); each postfix may be followed by another `.` feature-chain.

**Base** (`BaseExpression`): `null` | `( )` | literal | feature-reference (`QualifiedName`) | metadata-access (`ref . metadata`) | invocation (`Type ( args )`) | constructor (`new Type ( args )`) | body (`{ ... }`) | `( SequenceExpression )`.

**Literals:** boolean `true`/`false`; string `STRING_VALUE`; integer `DECIMAL_VALUE`; real `RealValue`; infinity `*` (`LiteralInfinity`). Note: lexer already produces a single `Real` token for `RealValue`; `LiteralInfinity` is a bare `*` where an expression is expected.

**Argument lists:** `( )` empty, positional (`arg (, arg)*`), or named (`name = arg (, name = arg)*`).

**Sequence:** `expr` optionally followed by `,` (with or without a trailing sequence) — flattens comma-separated operands.

**Names** (`KerMLExpressions.xtext:536-550`): `Name = ID | UNRESTRICTED_NAME`; `GlobalQualification = '$' '::'`; `Qualification = (Name '::')+`; `QualifiedName = GlobalQualification? Qualification? Name`.

**Namespace core** (from `org.omg.kerml.xtext/.../KerML.xtext`, cross-checked with `SysML.xtext` where they differ; Plan 2 follows **KerML.xtext** shapes as the base engine, noting SysML deltas inline):
- `RootNamespace` = `NamespaceBodyElement*` (a bare sequence of members, no braces).
- `Identification` = `'<' shortName '>' name?` | `name`.
- `RelationshipBody` = `';'` | `'{' RelationshipOwnedElement* '}'` (KerML) / `'{' OwnedAnnotation* '}'` (SysML). Plan 2 uses `';' | '{' body '}'` where body is the annotation/member list.
- `NamespaceBodyElement` / `PackageBody` member = `NamespaceMember` | `AliasMember` | `Import` | (`PackageBody` also) `ElementFilterMember`.
- `MemberPrefix` = optional `VisibilityIndicator` (`public`|`private`|`protected`).
- `Namespace` = `PrefixMetadataMember* 'namespace' Identification? NamespaceBody`.
- `Package` = `PrefixMetadataMember* 'package' Identification? PackageBody`.
- `LibraryPackage` = `'standard'? 'library' PrefixMetadataMember* 'package' Identification? PackageBody`.
- `Import` = (`MembershipImport` | `NamespaceImport`) `RelationshipBody`; prefix = `visibility 'import' 'all'?`; membership import = `QualifiedName ('::' '**')?`; namespace import = `QualifiedName '::' '*' ('::' '**')?` or a filter package.
- `AliasMember` = `MemberPrefix 'alias' ('<' shortName '>')? name? 'for' QualifiedName RelationshipBody`.
- `Dependency` = `PrefixMetadataAnnotation* 'dependency' (Identification? 'from')? clientList 'to' supplierList RelationshipBody` where lists are comma-separated `QualifiedName`.
- `Comment` = `('comment' Identification? ('about' Annotation (',' Annotation)*)?)? ('locale' STRING)? REGULAR_COMMENT`.
- `Documentation` = `'doc' Identification? ('locale' STRING)? REGULAR_COMMENT`.
- `TextualRepresentation` = `('rep' Identification?)? 'language' STRING REGULAR_COMMENT`.
- `ElementFilterMember` = `MemberPrefix 'filter' OwnedExpression ';'`.
- `PrefixMetadataMember` = `'#' QualifiedName` (metadata typing; Plan 2 records the reference only).
- `VisibilityIndicator` enum: `public` | `private` | `protected`.

**Lexer facts carried from Plan 1:** `lexer.New(sf).Next()` yields `Token{Kind, Span, KeywordID}`; keywords all have `Kind==Keyword` with `KeywordID` = the literal (contextual keywords like `individual`/`variation` disambiguated HERE in the parser). `Token.IsTrivia()` true for `Whitespace`/`SLNote`/`MLNote`; `RegularComment` is NOT trivia (it is the body of comment/doc/rep). Operator/punct Kinds: `ColonColon Dot DotDot DotQuestion Arrow Hash LParen RParen LBracket RBracket LBrace RBrace Comma Semicolon Colon Dollar Eq EqEq NotEq EqEqEq NotEqEq Lt Gt Le Ge Plus Minus Star Slash Percent StarStar Caret Tilde Question QuestionQ Pipe Amp At AtAt`. Literal Kinds: `Decimal Real String`. Name Kinds: `Identifier UnrestrictedName`.

---

### Task 1: AST node framework (Node interface, spans, trivia)

**Files:**
- Create: `internal/core/ast/node.go`
- Test: `internal/core/ast/node_test.go`

- [ ] **Step 1: Write the failing test**

```go
package ast

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestNodeBaseSpan(t *testing.T) {
	n := &ErrorNode{NodeBase: NodeBase{NodeSpan: source.Span{Offset: 3, Len: 5}}}
	if got := n.Span(); got.Offset != 3 || got.Len != 5 {
		t.Fatalf("Span() = %+v, want {3 5}", got)
	}
}

func TestErrorNodeMessage(t *testing.T) {
	n := &ErrorNode{Message: "unexpected token"}
	if n.Message != "unexpected token" {
		t.Fatalf("Message = %q", n.Message)
	}
}

func TestTriviaAttachment(t *testing.T) {
	n := &ErrorNode{}
	n.SetLeadingTrivia([]Trivia{{Kind: TriviaComment, Span: source.Span{Offset: 0, Len: 4}}})
	if len(n.LeadingTrivia()) != 1 || n.LeadingTrivia()[0].Kind != TriviaComment {
		t.Fatalf("leading trivia not attached: %+v", n.LeadingTrivia())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/ast/ -run 'TestNodeBaseSpan|TestErrorNodeMessage|TestTriviaAttachment' -v`
Expected: FAIL — build error, `ErrorNode`/`NodeBase`/`Trivia` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
package ast

import "github.com/Open-MBEE/Systemica/internal/core/source"

// Node is the common interface for every AST/CST node. The tree is
// syntax-only and immutable after parsing; all semantic information
// (resolved references, types) lives in side tables keyed by node,
// added in later plans.
type Node interface {
	Span() source.Span
	LeadingTrivia() []Trivia
	TrailingTrivia() []Trivia
}

// TriviaKind classifies a trivia token attached to a node.
type TriviaKind int

const (
	TriviaWhitespace TriviaKind = iota
	TriviaComment               // REGULAR_COMMENT that is not a comment/doc/rep body
	TriviaLineNote              // SL_NOTE
	TriviaBlockNote             // ML_NOTE
)

// Trivia is a non-semantic token (whitespace/notes/free comments) attached
// to a node for features like doc-comment hover and future formatting.
type Trivia struct {
	Kind TriviaKind
	Span source.Span
}

// NodeBase is embedded by every concrete node to provide span + trivia
// storage and satisfy the Node interface's accessor methods.
type NodeBase struct {
	NodeSpan source.Span
	leading  []Trivia
	trailing []Trivia
}

func (b *NodeBase) Span() source.Span         { return b.NodeSpan }
func (b *NodeBase) LeadingTrivia() []Trivia   { return b.leading }
func (b *NodeBase) TrailingTrivia() []Trivia  { return b.trailing }
func (b *NodeBase) SetLeadingTrivia(t []Trivia)  { b.leading = t }
func (b *NodeBase) SetTrailingTrivia(t []Trivia) { b.trailing = t }

// ErrorNode represents a span of source the parser could not parse into a
// valid construct. The parser always produces a tree; unparseable regions
// become ErrorNodes so downstream tooling (LSP) still gets a partial tree.
type ErrorNode struct {
	NodeBase
	Message string
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/ast/ -run 'TestNodeBaseSpan|TestErrorNodeMessage|TestTriviaAttachment' -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/core/ast/node.go internal/core/ast/node_test.go
git commit -m "feat(ast): add Node interface, NodeBase, Trivia, and ErrorNode"
```

### Task 2: AST node kinds — namespace/membership/import structure

**Files:**
- Create: `internal/core/ast/namespace.go`
- Test: `internal/core/ast/namespace_test.go`

These are pure data types (no logic). Every node embeds `NodeBase`. Cross-references (to `[Element|QualifiedName]` in the grammar) are stored as unresolved `*QualifiedName` here; resolution is Plan 3.

- [ ] **Step 1: Write the failing test**

```go
package ast

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func TestQualifiedNameParts(t *testing.T) {
	qn := &QualifiedName{
		Global: false,
		Parts:  []NameSegment{{Text: "A"}, {Text: "B"}, {Text: "C"}},
	}
	if len(qn.Parts) != 3 || qn.Parts[2].Text != "C" {
		t.Fatalf("parts = %+v", qn.Parts)
	}
}

func TestPackageIsNamespaceMember(t *testing.T) {
	var _ Node = &Package{}
	var _ Node = &Namespace{}
	var _ Node = &Import{}
	var _ Node = &Alias{}
	var _ Node = &Dependency{}
	var _ Node = &Comment{}
	var _ Node = &Documentation{}
	var _ Node = &TextualRepresentation{}
	var _ Node = &RootNamespace{}
	var _ Node = &Membership{}
	p := &Package{Ident: Identification{Name: "P"}}
	if p.Ident.Name != "P" {
		t.Fatalf("name = %q", p.Ident.Name)
	}
}

func TestMembershipVisibility(t *testing.T) {
	m := &Membership{Visibility: VisibilityPrivate}
	if m.Visibility != VisibilityPrivate {
		t.Fatalf("vis = %v", m.Visibility)
	}
	_ = source.Span{}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/ast/ -run 'TestQualifiedNameParts|TestPackageIsNamespaceMember|TestMembershipVisibility' -v`
Expected: FAIL — build error, types undefined.

- [ ] **Step 3: Write minimal implementation**

```go
package ast

import "github.com/Open-MBEE/Systemica/internal/core/source"

// Visibility mirrors SysML VisibilityKind.
type Visibility int

const (
	VisibilityDefault Visibility = iota // no explicit indicator
	VisibilityPublic
	VisibilityPrivate
	VisibilityProtected
)

// NameSegment is one identifier in a qualified name, with its source span.
type NameSegment struct {
	Text string
	Span source.Span
}

// QualifiedName is an unresolved dotted/`::`-separated name reference.
// Global is true when the name began with `$::`.
type QualifiedName struct {
	NodeBase
	Global bool
	Parts  []NameSegment
}

// Identification captures `<shortName> name` or `name` on a declaration.
type Identification struct {
	ShortName     string
	ShortNameSpan source.Span
	Name          string
	NameSpan      source.Span
}

// Membership wraps a namespace member with a visibility prefix. Member is
// the owned element (a Package/Namespace/Dependency/Comment/... or ErrorNode).
type Membership struct {
	NodeBase
	Visibility Visibility
	Member     Node
}

// RootNamespace is the top of every parsed file: a flat list of members.
type RootNamespace struct {
	NodeBase
	Members []Node // *Membership | *Import | *Alias | *ErrorNode
}

// PrefixMetadata records a `# QualifiedName` metadata annotation reference.
type PrefixMetadata struct {
	NodeBase
	Type *QualifiedName
}

// Namespace is `namespace <id> { ... }`.
type Namespace struct {
	NodeBase
	Prefixes []*PrefixMetadata
	Ident    Identification
	Members  []Node
	HasBody  bool // false when body was `;`
}

// Package is `package <id> { ... }`. Library/Standard flags cover
// `library package` and `standard library package`.
type Package struct {
	NodeBase
	Prefixes   []*PrefixMetadata
	Ident      Identification
	IsLibrary  bool
	IsStandard bool
	Members    []Node
	HasBody    bool
}

// ImportKind distinguishes membership vs namespace imports.
type ImportKind int

const (
	ImportMembership ImportKind = iota // import A::B ;
	ImportNamespace                    // import A::B::* ;
)

// Import is `[visibility] import [all] QualifiedName[::*][::**] ;|{}`.
type Import struct {
	NodeBase
	Visibility  Visibility
	IsAll       bool
	Kind        ImportKind
	Imported    *QualifiedName
	IsRecursive bool // `::**`
	Body        []Node
	HasBody     bool
}

// Alias is `alias <shortName> name for QualifiedName ;|{}`.
type Alias struct {
	NodeBase
	Visibility Visibility
	Ident      Identification
	For        *QualifiedName
	Body       []Node
	HasBody    bool
}

// Dependency is `dependency [<id> from] clients to suppliers ;|{}`.
type Dependency struct {
	NodeBase
	Prefixes  []*PrefixMetadata
	Ident     Identification
	Clients   []*QualifiedName
	Suppliers []*QualifiedName
	Body      []Node
	HasBody   bool
}

// Comment is `[comment <id> [about refs]] [locale s] /* ... */`.
type Comment struct {
	NodeBase
	Ident    Identification
	About    []*QualifiedName
	Locale   string
	BodySpan source.Span // the REGULAR_COMMENT token span
}

// Documentation is `doc <id> [locale s] /* ... */`.
type Documentation struct {
	NodeBase
	Ident    Identification
	Locale   string
	BodySpan source.Span
}

// TextualRepresentation is `[rep <id>] language s /* ... */`.
type TextualRepresentation struct {
	NodeBase
	Ident    Identification
	Language string
	BodySpan source.Span
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/ast/ -run 'TestQualifiedNameParts|TestPackageIsNamespaceMember|TestMembershipVisibility' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/ast/namespace.go internal/core/ast/namespace_test.go
git commit -m "feat(ast): add namespace, package, import, alias, dependency, comment nodes"
```

### Task 3: AST node kinds — expressions

**Files:**
- Create: `internal/core/ast/expr.go`
- Test: `internal/core/ast/expr_test.go`

Pure data types for the whole expression grammar. `OperatorExpression` is the single node for every binary/unary/postfix operator, carrying an `OperatorKind` plus its operand list (2 for binary, 1 for unary, 3 for conditional). Chains/index/collect/select/invocation/constructor get their own node types because their shape differs.

- [ ] **Step 1: Write the failing test**

```go
package ast

import "testing"

func TestExprNodesImplementNode(t *testing.T) {
	var _ Node = &LiteralBool{}
	var _ Node = &LiteralString{}
	var _ Node = &LiteralInteger{}
	var _ Node = &LiteralReal{}
	var _ Node = &LiteralInfinity{}
	var _ Node = &NullExpr{}
	var _ Node = &FeatureReference{}
	var _ Node = &OperatorExpr{}
	var _ Node = &FeatureChainExpr{}
	var _ Node = &IndexExpr{}
	var _ Node = &InvocationExpr{}
	var _ Node = &CollectExpr{}
	var _ Node = &SelectExpr{}
	var _ Node = &ConstructorExpr{}
	var _ Node = &BodyExpr{}
	var _ Node = &SequenceExpr{}
	var _ Node = &MetadataAccessExpr{}
}

func TestOperatorKindString(t *testing.T) {
	if OpAdd.String() != "+" {
		t.Fatalf("OpAdd = %q", OpAdd.String())
	}
	if OpImplies.String() != "implies" {
		t.Fatalf("OpImplies = %q", OpImplies.String())
	}
	if OpConditional.String() != "if" {
		t.Fatalf("OpConditional = %q", OpConditional.String())
	}
}

func TestOperatorExprOperands(t *testing.T) {
	e := &OperatorExpr{Operator: OpAdd, Operands: []Node{&LiteralInteger{Value: "1"}, &LiteralInteger{Value: "2"}}}
	if len(e.Operands) != 2 || e.Operator != OpAdd {
		t.Fatalf("bad operator expr: %+v", e)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/ast/ -run 'TestExprNodesImplementNode|TestOperatorKindString|TestOperatorExprOperands' -v`
Expected: FAIL — build error, types undefined.

- [ ] **Step 3: Write minimal implementation**

```go
package ast

import "github.com/Open-MBEE/Systemica/internal/core/source"

// OperatorKind enumerates every operator in the KerMLExpressions ladder,
// used by OperatorExpr regardless of arity.
type OperatorKind int

const (
	OpInvalid OperatorKind = iota
	OpConditional            // if C ? A else B
	OpNullCoalesce           // ??
	OpImplies                // implies
	OpOr                     // |
	OpConditionalOr          // or
	OpXor                    // xor
	OpAnd                    // &
	OpConditionalAnd         // and
	OpEq                     // ==
	OpNeq                    // !=
	OpEqEqEq                 // ===
	OpNeqEqEq                // !==
	OpHasType                // hastype
	OpIsType                 // istype
	OpAt                     // @
	OpMetaAt                 // @@
	OpAs                     // as
	OpMeta                   // meta
	OpLt                     // <
	OpGt                     // >
	OpLe                     // <=
	OpGe                     // >=
	OpRange                  // ..
	OpAdd                    // +
	OpSub                    // -
	OpMul                    // *
	OpDiv                    // /
	OpMod                    // %
	OpPow                    // ** or ^
	OpNeg                    // unary -
	OpPos                    // unary +
	OpBitNot                 // unary ~
	OpNot                    // unary not
	OpAll                    // extent: all
	OpIndex                  // [ ... ]
)

var operatorNames = map[OperatorKind]string{
	OpConditional: "if", OpNullCoalesce: "??", OpImplies: "implies",
	OpOr: "|", OpConditionalOr: "or", OpXor: "xor", OpAnd: "&",
	OpConditionalAnd: "and", OpEq: "==", OpNeq: "!=", OpEqEqEq: "===",
	OpNeqEqEq: "!==", OpHasType: "hastype", OpIsType: "istype", OpAt: "@",
	OpMetaAt: "@@", OpAs: "as", OpMeta: "meta", OpLt: "<", OpGt: ">",
	OpLe: "<=", OpGe: ">=", OpRange: "..", OpAdd: "+", OpSub: "-",
	OpMul: "*", OpDiv: "/", OpMod: "%", OpPow: "**", OpNeg: "-",
	OpPos: "+", OpBitNot: "~", OpNot: "not", OpAll: "all", OpIndex: "[]",
}

func (k OperatorKind) String() string {
	if s, ok := operatorNames[k]; ok {
		return s
	}
	return "OperatorKind(?)"
}

// LiteralBool is `true`/`false`.
type LiteralBool struct {
	NodeBase
	Value bool
}

// LiteralString is a double-quoted string literal (raw token text, quotes included).
type LiteralString struct {
	NodeBase
	Value string
}

// LiteralInteger is a DECIMAL_VALUE literal (raw text).
type LiteralInteger struct {
	NodeBase
	Value string
}

// LiteralReal is a RealValue literal (raw text).
type LiteralReal struct {
	NodeBase
	Value string
}

// LiteralInfinity is `*` in an expression position.
type LiteralInfinity struct{ NodeBase }

// NullExpr is `null` or `( )`.
type NullExpr struct{ NodeBase }

// FeatureReference is a bare QualifiedName used as an expression.
type FeatureReference struct {
	NodeBase
	Name *QualifiedName
}

// OperatorExpr is any operator application. Operands has 3 elements for
// OpConditional (cond, then, else), 1 for unary/extent, 2 otherwise.
// For OpAt/OpMetaAt/OpAs/OpMeta the RHS type reference is stored in TypeRef.
type OperatorExpr struct {
	NodeBase
	Operator OperatorKind
	Operands []Node
	TypeRef  *QualifiedName // classification/cast RHS, else nil
}

// FeatureChainExpr is `operand . member` (feature chain access).
type FeatureChainExpr struct {
	NodeBase
	Operand Node
	Member  *QualifiedName
}

// IndexExpr is `operand # ( seq )`.
type IndexExpr struct {
	NodeBase
	Operand Node
	Index   Node
}

// InvocationExpr is `Type ( args )` or `operand -> Type ( args | body | funcref )`.
type InvocationExpr struct {
	NodeBase
	Operand   Node           // receiver for `->` form, else nil
	Type      *QualifiedName // instantiated type
	Args      []Node         // positional args (Argument) or ...
	NamedArgs []NamedArg     // named args, mutually exclusive with Args
}

// NamedArg is `name = value` in an argument list.
type NamedArg struct {
	Name  *QualifiedName
	Value Node
}

// CollectExpr is `operand . body`.
type CollectExpr struct {
	NodeBase
	Operand Node
	Body    Node
}

// SelectExpr is `operand .? body`.
type SelectExpr struct {
	NodeBase
	Operand Node
	Body    Node
}

// ConstructorExpr is `new Type ( args )`.
type ConstructorExpr struct {
	NodeBase
	Type *QualifiedName
	Args []Node
}

// BodyExpr is `{ (in param ;)* resultExpr }`.
type BodyExpr struct {
	NodeBase
	Params []BodyParam
	Result Node
}

// BodyParam is `in name` inside a body expression.
type BodyParam struct {
	Name string
	Span source.Span
}

// SequenceExpr is a comma-separated list of expressions (flattened).
type SequenceExpr struct {
	NodeBase
	Elements []Node
}

// MetadataAccessExpr is `ref . metadata`.
type MetadataAccessExpr struct {
	NodeBase
	Ref *QualifiedName
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/ast/ -run 'TestExprNodesImplementNode|TestOperatorKindString|TestOperatorExprOperands' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/ast/expr.go internal/core/ast/expr_test.go
git commit -m "feat(ast): add expression node types and OperatorKind enum"
```

### Task 4: AST S-expression dumper (golden-test support)

**Files:**
- Create: `internal/core/ast/dump.go`
- Test: `internal/core/ast/dump_test.go`

`Dump` renders a node tree as an indented S-expression. This is the backbone of golden-file parser tests: deterministic, diffable, span-annotated. Format: one node per line, 2-space indent per depth, `(NodeType attr=val ...)` with children on following indented lines.

- [ ] **Step 1: Write the failing test**

```go
package ast

import (
	"strings"
	"testing"
)

func TestDumpLiteral(t *testing.T) {
	got := Dump(&LiteralInteger{Value: "42"})
	want := `(LiteralInteger value="42")`
	if strings.TrimSpace(got) != want {
		t.Fatalf("Dump = %q, want %q", got, want)
	}
}

func TestDumpOperatorExprNested(t *testing.T) {
	e := &OperatorExpr{
		Operator: OpAdd,
		Operands: []Node{
			&LiteralInteger{Value: "1"},
			&OperatorExpr{Operator: OpMul, Operands: []Node{
				&LiteralInteger{Value: "2"},
				&LiteralInteger{Value: "3"},
			}},
		},
	}
	want := strings.Join([]string{
		`(OperatorExpr operator="+"`,
		`  (LiteralInteger value="1")`,
		`  (OperatorExpr operator="*"`,
		`    (LiteralInteger value="2")`,
		`    (LiteralInteger value="3")))`,
	}, "\n")
	if got := strings.TrimSpace(Dump(e)); got != want {
		t.Fatalf("Dump =\n%s\nwant\n%s", got, want)
	}
}

func TestDumpQualifiedName(t *testing.T) {
	qn := &QualifiedName{Parts: []NameSegment{{Text: "A"}, {Text: "B"}}}
	got := strings.TrimSpace(Dump(&FeatureReference{Name: qn}))
	want := `(FeatureReference name="A::B")`
	if got != want {
		t.Fatalf("Dump = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/ast/ -run 'TestDump' -v`
Expected: FAIL — `Dump` undefined.

- [ ] **Step 3: Write minimal implementation**

The dumper uses a small builder. It renders known node types explicitly; children are appended and the trailing `)` count equals the depth closed. To keep the closing-paren logic simple, use a recursive writer that returns lines, and close parens on the last child's final line.

```go
package ast

import (
	"fmt"
	"strings"
)

// Dump renders a node tree as an indented S-expression for golden tests.
func Dump(n Node) string {
	var b strings.Builder
	dumpNode(&b, n, 0)
	return b.String()
}

func indent(b *strings.Builder, depth int) {
	for i := 0; i < depth; i++ {
		b.WriteString("  ")
	}
}

// qnString renders a QualifiedName as `A::B::C` (with `$::` prefix if global).
func qnString(qn *QualifiedName) string {
	if qn == nil {
		return ""
	}
	parts := make([]string, len(qn.Parts))
	for i, p := range qn.Parts {
		parts[i] = p.Text
	}
	s := strings.Join(parts, "::")
	if qn.Global {
		return "$::" + s
	}
	return s
}

// dumpNode writes `(Type attrs children...)`. It writes the open line with
// header and any leaf attributes, then children each on their own line at
// depth+1, closing all open parens on the final line.
func dumpNode(b *strings.Builder, n Node, depth int) {
	indent(b, depth)
	switch v := n.(type) {
	case *LiteralInteger:
		fmt.Fprintf(b, `(LiteralInteger value=%q)`, v.Value)
	case *LiteralReal:
		fmt.Fprintf(b, `(LiteralReal value=%q)`, v.Value)
	case *LiteralString:
		fmt.Fprintf(b, `(LiteralString value=%q)`, v.Value)
	case *LiteralBool:
		fmt.Fprintf(b, `(LiteralBool value=%t)`, v.Value)
	case *LiteralInfinity:
		b.WriteString(`(LiteralInfinity)`)
	case *NullExpr:
		b.WriteString(`(NullExpr)`)
	case *FeatureReference:
		fmt.Fprintf(b, `(FeatureReference name=%q)`, qnString(v.Name))
	case *MetadataAccessExpr:
		fmt.Fprintf(b, `(MetadataAccessExpr ref=%q)`, qnString(v.Ref))
	case *OperatorExpr:
		fmt.Fprintf(b, `(OperatorExpr operator=%q`, v.Operator.String())
		writeChildren(b, depth, operandsWithTypeRef(v))
		return
	case *FeatureChainExpr:
		fmt.Fprintf(b, `(FeatureChainExpr member=%q`, qnString(v.Member))
		writeChildren(b, depth, []Node{v.Operand})
		return
	case *IndexExpr:
		b.WriteString(`(IndexExpr`)
		writeChildren(b, depth, []Node{v.Operand, v.Index})
		return
	case *CollectExpr:
		b.WriteString(`(CollectExpr`)
		writeChildren(b, depth, []Node{v.Operand, v.Body})
		return
	case *SelectExpr:
		b.WriteString(`(SelectExpr`)
		writeChildren(b, depth, []Node{v.Operand, v.Body})
		return
	case *InvocationExpr:
		fmt.Fprintf(b, `(InvocationExpr type=%q`, qnString(v.Type))
		writeChildren(b, depth, invocationChildren(v))
		return
	case *ConstructorExpr:
		fmt.Fprintf(b, `(ConstructorExpr type=%q`, qnString(v.Type))
		writeChildren(b, depth, v.Args)
		return
	case *SequenceExpr:
		b.WriteString(`(SequenceExpr`)
		writeChildren(b, depth, v.Elements)
		return
	case *BodyExpr:
		b.WriteString(`(BodyExpr`)
		var kids []Node
		if v.Result != nil {
			kids = append(kids, v.Result)
		}
		writeChildren(b, depth, kids)
		return
	case *ErrorNode:
		fmt.Fprintf(b, `(ErrorNode message=%q)`, v.Message)
	default:
		fmt.Fprintf(b, `(%T)`, n)
	}
}

// writeChildren appends children lines under a header that was written
// WITHOUT its closing paren; it closes with `)` after the last child (or
// immediately if there are none).
func writeChildren(b *strings.Builder, depth int, kids []Node) {
	if len(kids) == 0 {
		b.WriteString(")")
		return
	}
	for _, k := range kids {
		b.WriteString("\n")
		dumpNode(b, k, depth+1)
	}
	b.WriteString(")")
}

func operandsWithTypeRef(v *OperatorExpr) []Node {
	kids := append([]Node{}, v.Operands...)
	if v.TypeRef != nil {
		kids = append(kids, &FeatureReference{Name: v.TypeRef})
	}
	return kids
}

func invocationChildren(v *InvocationExpr) []Node {
	kids := []Node{}
	if v.Operand != nil {
		kids = append(kids, v.Operand)
	}
	kids = append(kids, v.Args...)
	return kids
}
```

Note: this produces `)))` style closings because each `writeChildren` closes exactly one paren on the same final line, matching the golden expectation in the test.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/ast/ -run 'TestDump' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/ast/dump.go internal/core/ast/dump_test.go
git commit -m "feat(ast): add S-expression Dump for golden parser tests"
```

### Task 5: Parser driver, token buffer, diagnostics, error nodes

**Files:**
- Create: `internal/core/parser/diagnostic.go`
- Create: `internal/core/parser/parser.go`
- Test: `internal/core/parser/parser_test.go`

The driver wraps `lexer.Lexer`, skipping trivia into a pending-trivia buffer while exposing a non-trivia token stream with single-token lookahead plus a small ring for multi-token peeks. It records diagnostics and never panics out of the caller's control — recovery (Task 15) is layered on these primitives.

- [ ] **Step 1: Write the failing test**

```go
package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/lexer"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func newParser(src string) *Parser {
	sf := source.New("test", []byte(src))
	return New(sf)
}

func TestPeekSkipsTrivia(t *testing.T) {
	p := newParser("  \n // note\n part")
	tok := p.peek()
	if tok.Kind != lexer.Keyword || tok.KeywordID != "part" {
		t.Fatalf("peek = %+v", tok)
	}
}

func TestAdvanceAdvances(t *testing.T) {
	p := newParser("a b")
	first := p.advance()
	second := p.peek()
	if first.Kind != lexer.Identifier {
		t.Fatalf("first = %+v", first)
	}
	if second.Kind != lexer.Identifier || p.src.Text(second.Span) != "b" {
		t.Fatalf("second = %+v", second)
	}
}

func TestPeekN(t *testing.T) {
	p := newParser("a :: b")
	if p.peek().Kind != lexer.Identifier {
		t.Fatal("peek0")
	}
	if p.peekN(1).Kind != lexer.ColonColon {
		t.Fatalf("peek1 = %+v", p.peekN(1))
	}
	if p.peekN(2).Kind != lexer.Identifier {
		t.Fatalf("peek2 = %+v", p.peekN(2))
	}
}

func TestExpectRecordsDiagnostic(t *testing.T) {
	p := newParser("a")
	p.advance() // consume 'a'
	tok, ok := p.expect(lexer.Semicolon, "expected ';'")
	if ok {
		t.Fatal("expected failure at EOF")
	}
	if len(p.Diagnostics) != 1 || p.Diagnostics[0].Message != "expected ';'" {
		t.Fatalf("diags = %+v", p.Diagnostics)
	}
	if tok.Kind != lexer.EOF {
		t.Fatalf("tok = %+v", tok)
	}
}

func TestAtEOF(t *testing.T) {
	p := newParser("")
	if !p.atEOF() {
		t.Fatal("empty should be EOF")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/parser/ -run 'TestPeek|TestAdvance|TestExpect|TestAtEOF' -v`
Expected: FAIL — build error, `Parser`/`New` undefined.

- [ ] **Step 3: Write minimal implementation**

`diagnostic.go`:

```go
package parser

import "github.com/Open-MBEE/Systemica/internal/core/source"

// Diagnostic is a parser-emitted syntax error. It is unified with the
// pass/validation diagnostic model in Plan 4; kept local here to avoid a
// premature cross-package dependency.
type Diagnostic struct {
	Span    source.Span
	Message string
}
```

`parser.go`:

```go
package parser

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// Parser is a hand-written recursive-descent parser over a lexer token
// stream. It buffers non-trivia tokens for lookahead and collects
// diagnostics; it always produces a tree (ErrorNodes for bad input).
type Parser struct {
	src   *source.SourceFile
	lx    *lexer.Lexer
	buf   []lexer.Token   // lookahead ring of non-trivia tokens
	triv  []ast.Trivia    // trivia pending attachment to the next node
	Diagnostics []Diagnostic
}

// New creates a Parser for the given source file.
func New(sf *source.SourceFile) *Parser {
	return &Parser{src: sf, lx: lexer.New(sf)}
}

// fill ensures buf has at least n+1 tokens (pulling from the lexer, skipping
// and recording trivia). The final EOF token is sticky (re-returned).
func (p *Parser) fill(n int) {
	for len(p.buf) <= n {
		tok := p.lx.Next()
		for tok.IsTrivia() || tok.Kind == lexer.RegularComment {
			p.triv = append(p.triv, triviaOf(tok))
			if tok.Kind == lexer.EOF {
				break
			}
			tok = p.lx.Next()
		}
		p.buf = append(p.buf, tok)
		if tok.Kind == lexer.EOF {
			// keep EOF sticky: stop growing further with real tokens
			return
		}
	}
}

func triviaOf(tok lexer.Token) ast.Trivia {
	var k ast.TriviaKind
	switch tok.Kind {
	case lexer.SLNote:
		k = ast.TriviaLineNote
	case lexer.MLNote:
		k = ast.TriviaBlockNote
	case lexer.RegularComment:
		k = ast.TriviaComment
	default:
		k = ast.TriviaWhitespace
	}
	return ast.Trivia{Kind: k, Span: tok.Span}
}

// peek returns the current non-trivia token without consuming it.
func (p *Parser) peek() lexer.Token { return p.peekN(0) }

// peekN returns the token n positions ahead (0 = current).
func (p *Parser) peekN(n int) lexer.Token {
	p.fill(n)
	if n >= len(p.buf) {
		return p.buf[len(p.buf)-1] // EOF (sticky)
	}
	return p.buf[n]
}

// advance consumes and returns the current token.
func (p *Parser) advance() lexer.Token {
	p.fill(0)
	tok := p.buf[0]
	if tok.Kind != lexer.EOF {
		p.buf = p.buf[1:]
	}
	return tok
}

// atEOF reports whether the current token is EOF.
func (p *Parser) atEOF() bool { return p.peek().Kind == lexer.EOF }

// at reports whether the current token has the given kind.
func (p *Parser) at(k lexer.Kind) bool { return p.peek().Kind == k }

// atKeyword reports whether the current token is the given keyword literal.
func (p *Parser) atKeyword(kw string) bool {
	t := p.peek()
	return t.Kind == lexer.Keyword && t.KeywordID == kw
}

// accept consumes the current token if it matches kind, reporting success.
func (p *Parser) accept(k lexer.Kind) (lexer.Token, bool) {
	if p.at(k) {
		return p.advance(), true
	}
	return p.peek(), false
}

// acceptKeyword consumes the current token if it is the given keyword.
func (p *Parser) acceptKeyword(kw string) bool {
	if p.atKeyword(kw) {
		p.advance()
		return true
	}
	return false
}

// expect consumes a token of the given kind or records a diagnostic at the
// current position and returns ok=false (without consuming).
func (p *Parser) expect(k lexer.Kind, msg string) (lexer.Token, bool) {
	if p.at(k) {
		return p.advance(), true
	}
	p.error(p.peek().Span, msg)
	return p.peek(), false
}

// error records a diagnostic.
func (p *Parser) error(sp source.Span, msg string) {
	p.Diagnostics = append(p.Diagnostics, Diagnostic{Span: sp, Message: msg})
}

// takeTrivia returns and clears the pending leading trivia.
func (p *Parser) takeTrivia() []ast.Trivia {
	t := p.triv
	p.triv = nil
	return t
}

// spanFrom builds a span from a start offset to the end of the previously
// consumed token region (current token's start).
func (p *Parser) spanFrom(start int) source.Span {
	end := p.peek().Span.Offset
	if end < start {
		end = start
	}
	return source.Span{Offset: start, Len: end - start}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/parser/ -run 'TestPeek|TestAdvance|TestExpect|TestAtEOF' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/parser/diagnostic.go internal/core/parser/parser.go internal/core/parser/parser_test.go
git commit -m "feat(parser): add recursive-descent driver, lookahead buffer, diagnostics"
```

### Task 6: Parse QualifiedName + Identification

**Files:**
- Create: `internal/core/parser/namespace.go`
- Test: `internal/core/parser/namespace_test.go`

`parseQualifiedName` reads `[$::] Name (:: Name)*` where `Name` is an `Identifier`, `UnrestrictedName`, or any keyword token used as a name (SysML allows keyword-like names as `UNRESTRICTED_NAME`; for Plan 2 we accept `Identifier`/`UnrestrictedName` only and treat a leading keyword as not-a-name). `parseIdentification` reads `<shortName> name?` or `name`. These are shared building blocks used by every declaration parser (Tasks 7-11). They live in `namespace.go` alongside the declaration parsers that follow.

- [ ] **Step 1: Write the failing test**

```go
package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

func TestParseQualifiedNameSimple(t *testing.T) {
	p := newParser("A::B::C")
	qn := p.parseQualifiedName()
	if qn == nil || len(qn.Parts) != 3 {
		t.Fatalf("qn = %+v", qn)
	}
	if qn.Parts[0].Text != "A" || qn.Parts[2].Text != "C" {
		t.Fatalf("parts = %+v", qn.Parts)
	}
	if qn.Global {
		t.Fatal("should not be global")
	}
	if len(p.Diagnostics) != 0 {
		t.Fatalf("diags = %+v", p.Diagnostics)
	}
}

func TestParseQualifiedNameGlobal(t *testing.T) {
	p := newParser("$::Root::X")
	qn := p.parseQualifiedName()
	if qn == nil || !qn.Global {
		t.Fatalf("qn = %+v", qn)
	}
	if len(qn.Parts) != 2 || qn.Parts[0].Text != "Root" {
		t.Fatalf("parts = %+v", qn.Parts)
	}
}

func TestParseQualifiedNameUnrestricted(t *testing.T) {
	p := newParser("'my name'::B")
	qn := p.parseQualifiedName()
	if qn == nil || len(qn.Parts) != 2 {
		t.Fatalf("qn = %+v", qn)
	}
	if qn.Parts[0].Text != "'my name'" {
		t.Fatalf("part0 = %q", qn.Parts[0].Text)
	}
}

func TestParseQualifiedNameNoName(t *testing.T) {
	p := newParser(";")
	qn := p.parseQualifiedName()
	if qn != nil {
		t.Fatalf("expected nil, got %+v", qn)
	}
	if len(p.Diagnostics) != 1 {
		t.Fatalf("diags = %+v", p.Diagnostics)
	}
}

func TestParseIdentificationShortAndName(t *testing.T) {
	p := newParser("<v1> Vehicle")
	id := p.parseIdentification()
	if id.ShortName != "v1" || id.Name != "Vehicle" {
		t.Fatalf("id = %+v", id)
	}
}

func TestParseIdentificationNameOnly(t *testing.T) {
	p := newParser("Vehicle")
	id := p.parseIdentification()
	if id.ShortName != "" || id.Name != "Vehicle" {
		t.Fatalf("id = %+v", id)
	}
}

func TestParseIdentificationEmpty(t *testing.T) {
	p := newParser("{")
	id := p.parseIdentification()
	if id.Name != "" || id.ShortName != "" {
		t.Fatalf("expected empty id, got %+v", id)
	}
}

var _ = ast.Node(nil)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/parser/ -run 'TestParseQualifiedName|TestParseIdentification' -v`
Expected: FAIL — build error, `parseQualifiedName`/`parseIdentification` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
package parser

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
)

// atName reports whether the current token can begin a name segment.
func (p *Parser) atName() bool {
	k := p.peek().Kind
	return k == lexer.Identifier || k == lexer.UnrestrictedName
}

// parseNameSegment consumes one name token and returns its segment.
func (p *Parser) parseNameSegment() (ast.NameSegment, bool) {
	if !p.atName() {
		return ast.NameSegment{}, false
	}
	tok := p.advance()
	return ast.NameSegment{Text: p.src.Text(tok.Span), Span: tok.Span}, true
}

// parseQualifiedName parses `[$::] Name (:: Name)*`. It returns nil and
// records a diagnostic if no name is present.
func (p *Parser) parseQualifiedName() *ast.QualifiedName {
	start := p.peek().Span.Offset
	trivia := p.takeTrivia()

	global := false
	if p.at(lexer.Dollar) && p.peekN(1).Kind == lexer.ColonColon {
		p.advance() // $
		p.advance() // ::
		global = true
	}

	seg, ok := p.parseNameSegment()
	if !ok {
		if global {
			// `$::` with no following name — still a (degenerate) global name.
			qn := &ast.QualifiedName{Global: true}
			qn.NodeSpan = p.spanFrom(start)
			qn.SetLeadingTrivia(trivia)
			return qn
		}
		p.error(p.peek().Span, "expected a name")
		return nil
	}

	parts := []ast.NameSegment{seg}
	for p.at(lexer.ColonColon) {
		// Do not consume `::` if it introduces `*`/`**` (namespace import wildcard).
		if nk := p.peekN(1).Kind; nk == lexer.Star || nk == lexer.StarStar {
			break
		}
		p.advance() // ::
		next, ok := p.parseNameSegment()
		if !ok {
			p.error(p.peek().Span, "expected a name after '::'")
			break
		}
		parts = append(parts, next)
	}

	qn := &ast.QualifiedName{Global: global, Parts: parts}
	qn.NodeSpan = p.spanFrom(start)
	qn.SetLeadingTrivia(trivia)
	return qn
}

// parseIdentification parses `<shortName> name?` or `name` or nothing.
// A missing identification yields a zero-value Identification (no diagnostic).
func (p *Parser) parseIdentification() ast.Identification {
	var id ast.Identification
	if p.at(lexer.Lt) {
		p.advance() // <
		if seg, ok := p.parseNameSegment(); ok {
			id.ShortName = seg.Text
			id.ShortNameSpan = seg.Span
		} else {
			p.error(p.peek().Span, "expected short name after '<'")
		}
		p.expect(lexer.Gt, "expected '>'")
	}
	if seg, ok := p.parseNameSegment(); ok {
		id.Name = seg.Text
		id.NameSpan = seg.Span
	}
	return id
}
```

Note: `parseNameSegment` strips no quotes — `UnrestrictedName` text keeps its surrounding single quotes (`'my name'`), matching the raw-token convention used by literals. Unquoting is a resolution/display concern (Plan 3+).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/parser/ -run 'TestParseQualifiedName|TestParseIdentification' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/parser/namespace.go internal/core/parser/namespace_test.go
git commit -m "feat(parser): parse qualified names and identifications"
```

### Task 7: Parse RootNamespace + namespace body loop + RelationshipBody

**Files:**
- Modify: `internal/core/parser/parser.go` (add `ParseFile` entry point)
- Modify: `internal/core/parser/namespace.go` (add member dispatch + body loop)
- Test: `internal/core/parser/namespace_test.go`

`ParseFile` is the public entry point: it parses a `RootNamespace` (a flat, brace-less list of members to EOF). `parseNamespaceBody` parses `{ member* }` and is shared by every declaration with a body. `parseMember` reads an optional visibility prefix and dispatches on the leading keyword to the right declaration parser. Unknown/out-of-scope leading keywords (e.g. `part`, `type`, `feature` — deferred to Plan 2b) produce an `ErrorNode` spanning to the next sync point (Task 15 refines the skip; here we skip to `;` or `}`).

- [ ] **Step 1: Write the failing test**

```go
package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

func TestParseFileEmpty(t *testing.T) {
	p := newParser("")
	root := p.ParseFile()
	if root == nil || len(root.Members) != 0 {
		t.Fatalf("root = %+v", root)
	}
	if len(p.Diagnostics) != 0 {
		t.Fatalf("diags = %+v", p.Diagnostics)
	}
}

func TestParseFileVisibilityPrefix(t *testing.T) {
	p := newParser("private package P;")
	root := p.ParseFile()
	if len(root.Members) != 1 {
		t.Fatalf("members = %+v", root.Members)
	}
	m, ok := root.Members[0].(*ast.Membership)
	if !ok {
		t.Fatalf("member type = %T", root.Members[0])
	}
	if m.Visibility != ast.VisibilityPrivate {
		t.Fatalf("vis = %v", m.Visibility)
	}
	if _, ok := m.Member.(*ast.Package); !ok {
		t.Fatalf("inner type = %T", m.Member)
	}
}

func TestParseFileUnknownKeywordErrorNode(t *testing.T) {
	p := newParser("part def Vehicle;")
	root := p.ParseFile()
	if len(root.Members) != 1 {
		t.Fatalf("members = %+v", root.Members)
	}
	if _, ok := root.Members[0].(*ast.ErrorNode); !ok {
		t.Fatalf("expected ErrorNode, got %T", root.Members[0])
	}
	if len(p.Diagnostics) == 0 {
		t.Fatal("expected a diagnostic")
	}
}

func TestParseNamespaceBodyMembers(t *testing.T) {
	p := newParser("package Outer { package Inner; }")
	root := p.ParseFile()
	m := root.Members[0].(*ast.Membership)
	outer := m.Member.(*ast.Package)
	if !outer.HasBody || len(outer.Members) != 1 {
		t.Fatalf("outer = %+v", outer)
	}
	inner := outer.Members[0].(*ast.Membership).Member.(*ast.Package)
	if inner.Ident.Name != "Inner" {
		t.Fatalf("inner = %+v", inner)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/parser/ -run 'TestParseFile|TestParseNamespaceBody' -v`
Expected: FAIL — `ParseFile` undefined.

- [ ] **Step 3: Write minimal implementation**

Add to `parser.go`:

```go
// ParseFile parses the whole source as a RootNamespace (brace-less member list).
func (p *Parser) ParseFile() *ast.RootNamespace {
	start := p.peek().Span.Offset
	root := &ast.RootNamespace{}
	for !p.atEOF() {
		before := len(p.buf)
		beforeOff := p.peek().Span.Offset
		m := p.parseMember()
		if m != nil {
			root.Members = append(root.Members, m)
		}
		// Guarantee progress: if nothing was consumed, skip a token.
		if len(p.buf) == before && p.peek().Span.Offset == beforeOff && !p.atEOF() {
			p.advance()
		}
	}
	root.NodeSpan = p.spanFrom(start)
	return root
}
```

Add to `namespace.go`:

```go
// parseVisibility reads an optional public/private/protected prefix.
func (p *Parser) parseVisibility() ast.Visibility {
	switch {
	case p.acceptKeyword("public"):
		return ast.VisibilityPublic
	case p.acceptKeyword("private"):
		return ast.VisibilityPrivate
	case p.acceptKeyword("protected"):
		return ast.VisibilityProtected
	default:
		return ast.VisibilityDefault
	}
}

// parseMember parses one namespace member: an optional visibility prefix
// followed by a declaration. Import/Alias carry their own visibility and are
// returned directly; other declarations are wrapped in a Membership.
func (p *Parser) parseMember() ast.Node {
	start := p.peek().Span.Offset
	trivia := p.takeTrivia()
	vis := p.parseVisibility()

	// Import and Alias hold visibility internally and are not wrapped.
	if p.atKeyword("import") {
		imp := p.parseImport(start, vis)
		imp.SetLeadingTrivia(trivia)
		return imp
	}
	if p.atKeyword("alias") {
		al := p.parseAlias(start, vis)
		al.SetLeadingTrivia(trivia)
		return al
	}

	inner := p.parseDeclaration(start)
	if inner == nil {
		// No declaration recognized. Emit an error node spanning the skip.
		en := p.errorNodeSkip(start, "expected a namespace member")
		en.SetLeadingTrivia(trivia)
		return en
	}
	m := &ast.Membership{Visibility: vis, Member: inner}
	m.NodeSpan = p.spanFrom(start)
	m.SetLeadingTrivia(trivia)
	return m
}

// parseDeclaration dispatches on the leading keyword to a declaration parser.
// Returns nil if the current token starts no known (in-scope) declaration.
func (p *Parser) parseDeclaration(start int) ast.Node {
	switch {
	case p.atKeyword("package"), p.atKeyword("library"), p.atKeyword("standard"):
		return p.parsePackage(start)
	case p.atKeyword("namespace"):
		return p.parseNamespace(start)
	case p.atKeyword("dependency"):
		return p.parseDependency(start)
	case p.atKeyword("comment"):
		return p.parseComment(start)
	case p.atKeyword("doc"):
		return p.parseDocumentation(start)
	case p.atKeyword("rep"), p.atKeyword("language"):
		return p.parseTextualRepresentation(start)
	case p.at(lexer.Hash):
		// A bare prefix-metadata member with no following declaration is rare;
		// `parsePackage`/`parseNamespace` consume leading `#` prefixes themselves.
		// Reaching here with `#` means prefixes then a non-declaration keyword.
		return nil
	default:
		return nil
	}
}

// parseNamespaceBody parses `{ member* }` or `;`. Returns (members, hasBody).
// The caller has already consumed the declaration head up to this point.
func (p *Parser) parseNamespaceBody() ([]ast.Node, bool) {
	if p.accept2(lexer.Semicolon) {
		return nil, false
	}
	if _, ok := p.expect(lexer.LBrace, "expected '{' or ';'"); !ok {
		return nil, false
	}
	var members []ast.Node
	for !p.atEOF() && !p.at(lexer.RBrace) {
		before := p.peek().Span.Offset
		m := p.parseMember()
		if m != nil {
			members = append(members, m)
		}
		if p.peek().Span.Offset == before && !p.at(lexer.RBrace) && !p.atEOF() {
			p.advance()
		}
	}
	p.expect(lexer.RBrace, "expected '}'")
	return members, true
}

// accept2 is accept that discards the token (convenience for punctuation).
func (p *Parser) accept2(k lexer.Kind) bool {
	_, ok := p.accept(k)
	return ok
}

// errorNodeSkip builds an ErrorNode and skips tokens to the next `;`/`}`/EOF.
func (p *Parser) errorNodeSkip(start int, msg string) *ast.ErrorNode {
	p.error(p.peek().Span, msg)
	for !p.atEOF() && !p.at(lexer.Semicolon) && !p.at(lexer.RBrace) {
		p.advance()
	}
	p.accept2(lexer.Semicolon) // consume the terminator if present
	en := &ast.ErrorNode{Message: msg}
	en.NodeSpan = p.spanFrom(start)
	return en
}
```

Note: `parsePackage`, `parseNamespace`, `parseDependency`, `parseComment`, `parseDocumentation`, `parseTextualRepresentation`, `parseImport`, `parseAlias` are implemented in Tasks 8-11. To keep this task compiling and green on its own, add temporary stubs that will be replaced (each returns an `ErrorNode` via `errorNodeSkip`) OR implement Task 8 immediately after in the same working session. **This plan implements Task 8 next**, so add these minimal stubs now to compile, then replace in Tasks 8-11:

```go
// Temporary stubs (replaced in Tasks 8-11).
func (p *Parser) parsePackage(start int) ast.Node { return p.errorNodeSkip(start, "package: not yet implemented") }
func (p *Parser) parseNamespace(start int) ast.Node { return p.errorNodeSkip(start, "namespace: not yet implemented") }
func (p *Parser) parseDependency(start int) ast.Node { return p.errorNodeSkip(start, "dependency: not yet implemented") }
func (p *Parser) parseComment(start int) ast.Node { return p.errorNodeSkip(start, "comment: not yet implemented") }
func (p *Parser) parseDocumentation(start int) ast.Node { return p.errorNodeSkip(start, "doc: not yet implemented") }
func (p *Parser) parseTextualRepresentation(start int) ast.Node { return p.errorNodeSkip(start, "rep: not yet implemented") }
func (p *Parser) parseImport(start int, vis ast.Visibility) *ast.Import {
	en := p.errorNodeSkip(start, "import: not yet implemented")
	imp := &ast.Import{Visibility: vis}
	imp.NodeSpan = en.NodeSpan
	return imp
}
func (p *Parser) parseAlias(start int, vis ast.Visibility) *ast.Alias {
	en := p.errorNodeSkip(start, "alias: not yet implemented")
	al := &ast.Alias{Visibility: vis}
	al.NodeSpan = en.NodeSpan
	return al
}
```

With the stubs, `TestParseFileVisibilityPrefix` and `TestParseNamespaceBodyMembers` would fail (they expect real `*ast.Package`). **Therefore only run the two tests that pass with stubs in this task**, and enable the full-behavior tests in Task 8. Adjust Step 2/4 test filter accordingly:

- Run in this task: `go test ./internal/core/parser/ -run 'TestParseFileEmpty|TestParseFileUnknownKeywordErrorNode' -v`
- The `TestParseFileVisibilityPrefix` / `TestParseNamespaceBodyMembers` tests are written now (RED) but go green in Task 8.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/parser/ -run 'TestParseFileEmpty|TestParseFileUnknownKeywordErrorNode' -v`
Expected: FAIL — `ParseFile` undefined.

- [ ] **Step 3: (implementation above)**

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/parser/ -run 'TestParseFileEmpty|TestParseFileUnknownKeywordErrorNode' -v`
Expected: PASS. (Full-behavior tests remain RED until Task 8 — that is expected and noted.)

- [ ] **Step 5: Commit**

```bash
git add internal/core/parser/parser.go internal/core/parser/namespace.go internal/core/parser/namespace_test.go
git commit -m "feat(parser): add ParseFile, member dispatch, namespace body loop with stubs"
```

### Task 8: Parse Package / Namespace declarations

**Files:**
- Modify: `internal/core/parser/namespace.go` (replace `parsePackage`/`parseNamespace` stubs, add prefix-metadata parsing)
- Test: `internal/core/parser/namespace_test.go`

Implements `parsePackage` (`[standard] [library] package <id> body`), `parseNamespace` (`namespace <id> body`), and `parsePrefixMetadata` (`# QualifiedName`, repeatable, consumed before the declaration keyword). This task makes the two tests deferred from Task 7 (`TestParseFileVisibilityPrefix`, `TestParseNamespaceBodyMembers`) pass.

- [ ] **Step 1: Write the failing test**

```go
package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

func TestParsePackageEmptyBody(t *testing.T) {
	p := newParser("package P { }")
	root := p.ParseFile()
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	if pkg.Ident.Name != "P" || !pkg.HasBody || len(pkg.Members) != 0 {
		t.Fatalf("pkg = %+v", pkg)
	}
}

func TestParsePackageSemicolon(t *testing.T) {
	p := newParser("package P;")
	root := p.ParseFile()
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	if pkg.HasBody {
		t.Fatalf("expected no body: %+v", pkg)
	}
}

func TestParseLibraryPackage(t *testing.T) {
	p := newParser("standard library package Base;")
	root := p.ParseFile()
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	if !pkg.IsLibrary || !pkg.IsStandard {
		t.Fatalf("flags = %+v", pkg)
	}
}

func TestParseNamespaceDecl(t *testing.T) {
	p := newParser("namespace N { }")
	root := p.ParseFile()
	ns := root.Members[0].(*ast.Membership).Member.(*ast.Namespace)
	if !ns.HasBody {
		t.Fatalf("ns = %+v", ns)
	}
}

func TestParsePrefixMetadata(t *testing.T) {
	p := newParser("#Meta package P;")
	root := p.ParseFile()
	pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
	if len(pkg.Prefixes) != 1 || pkg.Prefixes[0].Type == nil {
		t.Fatalf("prefixes = %+v", pkg.Prefixes)
	}
	if pkg.Prefixes[0].Type.Parts[0].Text != "Meta" {
		t.Fatalf("prefix type = %+v", pkg.Prefixes[0].Type)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/parser/ -run 'TestParsePackage|TestParseLibraryPackage|TestParseNamespaceDecl|TestParsePrefixMetadata' -v`
Expected: FAIL — stubs return `ErrorNode`, so type assertions to `*ast.Package`/`*ast.Namespace` panic/fail.

- [ ] **Step 3: Write minimal implementation**

Replace the `parsePackage` and `parseNamespace` stubs in `namespace.go` with:

```go
// parsePrefixMetadata parses zero or more `# QualifiedName` prefix annotations.
func (p *Parser) parsePrefixMetadata() []*ast.PrefixMetadata {
	var prefixes []*ast.PrefixMetadata
	for p.at(lexer.Hash) {
		start := p.peek().Span.Offset
		p.advance() // #
		qn := p.parseQualifiedName()
		pm := &ast.PrefixMetadata{Type: qn}
		pm.NodeSpan = p.spanFrom(start)
		prefixes = append(prefixes, pm)
	}
	return prefixes
}

// parsePackage parses `[standard] [library] package <id> body`.
// Prefix metadata may precede `package`; it is consumed here.
func (p *Parser) parsePackage(start int) ast.Node {
	prefixes := p.parsePrefixMetadata()
	isStandard := p.acceptKeyword("standard")
	isLibrary := p.acceptKeyword("library")
	if !p.acceptKeyword("package") {
		return p.errorNodeSkip(start, "expected 'package'")
	}
	id := p.parseIdentification()
	members, hasBody := p.parseNamespaceBody()
	pkg := &ast.Package{
		Prefixes:   prefixes,
		Ident:      id,
		IsLibrary:  isLibrary,
		IsStandard: isStandard,
		Members:    members,
		HasBody:    hasBody,
	}
	pkg.NodeSpan = p.spanFrom(start)
	return pkg
}

// parseNamespace parses `namespace <id> body`.
func (p *Parser) parseNamespace(start int) ast.Node {
	prefixes := p.parsePrefixMetadata()
	if !p.acceptKeyword("namespace") {
		return p.errorNodeSkip(start, "expected 'namespace'")
	}
	id := p.parseIdentification()
	members, hasBody := p.parseNamespaceBody()
	ns := &ast.Namespace{Prefixes: prefixes, Ident: id, Members: members, HasBody: hasBody}
	ns.NodeSpan = p.spanFrom(start)
	return ns
}
```

Also remove the now-obsolete `parsePackage`/`parseNamespace` stub lines (leave the other stubs for Tasks 9-11). Update `parseDeclaration`'s `case p.at(lexer.Hash)` note is unaffected — prefix metadata is consumed inside `parsePackage`/`parseNamespace`; but a `#`-led member must reach one of those parsers. Extend `parseDeclaration` so a leading `#` peeks past the prefix run to the real keyword:

```go
	case p.at(lexer.Hash):
		// Look past `# QualifiedName ...` prefixes for the declaration keyword.
		if p.leadingPrefixIsPackage() {
			return p.parsePackage(start)
		}
		if p.leadingPrefixIsNamespace() {
			return p.parseNamespace(start)
		}
		return nil
```

with helpers (a cheap bounded peek — prefixes are `# Name (:: Name)*`):

```go
// prefixLookahead returns the buffer index of the token following all
// leading `# QualifiedName` prefixes, without consuming anything.
func (p *Parser) prefixLookahead() int {
	i := 0
	for p.peekN(i).Kind == lexer.Hash {
		i++ // '#'
		// QualifiedName: Name (:: Name)*
		if k := p.peekN(i).Kind; k != lexer.Identifier && k != lexer.UnrestrictedName {
			return i
		}
		i++
		for p.peekN(i).Kind == lexer.ColonColon {
			i++
			if k := p.peekN(i).Kind; k != lexer.Identifier && k != lexer.UnrestrictedName {
				return i
			}
			i++
		}
	}
	return i
}

func (p *Parser) leadingPrefixIsPackage() bool {
	t := p.peekN(p.prefixLookahead())
	return t.Kind == lexer.Keyword && (t.KeywordID == "package" || t.KeywordID == "library" || t.KeywordID == "standard")
}

func (p *Parser) leadingPrefixIsNamespace() bool {
	t := p.peekN(p.prefixLookahead())
	return t.Kind == lexer.Keyword && t.KeywordID == "namespace"
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/parser/ -run 'TestParseFile|TestParsePackage|TestParseLibraryPackage|TestParseNamespace|TestParsePrefixMetadata' -v`
Expected: PASS (including the two tests deferred from Task 7).

- [ ] **Step 5: Commit**

```bash
git add internal/core/parser/namespace.go internal/core/parser/namespace_test.go
git commit -m "feat(parser): parse package, namespace, and prefix-metadata declarations"
```

### Task 9: Parse Import (membership + namespace + recursive **)

**Files:**
- Modify: `internal/core/parser/namespace.go` (replace `parseImport` stub)
- Test: `internal/core/parser/namespace_test.go`

`parseImport` (visibility already consumed by `parseMember`) parses `import [all] QualifiedName [::*] [::**] body`. Forms:
- `import A::B ;` → membership import
- `import A::B::* ;` → namespace import (all members)
- `import A::B::** ;` → namespace import, recursive
- `import A::* ;` and `import A::** ;` likewise
- `import all A::B::* ;` → `IsAll`

The wildcard tail is `::` `*` (namespace) optionally followed by `::` `**`, OR `::` `**` directly (recursive from that namespace). Recall `parseQualifiedName` deliberately stops before a `::` that precedes `*`/`**`, leaving the tail for us to inspect.

- [ ] **Step 1: Write the failing test**

```go
package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

func importOf(t *testing.T, src string) *ast.Import {
	t.Helper()
	p := newParser(src)
	root := p.ParseFile()
	if len(root.Members) != 1 {
		t.Fatalf("members = %+v (diags %+v)", root.Members, p.Diagnostics)
	}
	imp, ok := root.Members[0].(*ast.Import)
	if !ok {
		t.Fatalf("member type = %T", root.Members[0])
	}
	return imp
}

func TestParseMembershipImport(t *testing.T) {
	imp := importOf(t, "import A::B;")
	if imp.Kind != ast.ImportMembership || imp.IsRecursive || imp.IsAll {
		t.Fatalf("imp = %+v", imp)
	}
	if imp.Imported == nil || len(imp.Imported.Parts) != 2 {
		t.Fatalf("imported = %+v", imp.Imported)
	}
}

func TestParseNamespaceImportStar(t *testing.T) {
	imp := importOf(t, "import A::B::*;")
	if imp.Kind != ast.ImportNamespace || imp.IsRecursive {
		t.Fatalf("imp = %+v", imp)
	}
}

func TestParseRecursiveImport(t *testing.T) {
	imp := importOf(t, "import A::B::**;")
	if imp.Kind != ast.ImportNamespace || !imp.IsRecursive {
		t.Fatalf("imp = %+v", imp)
	}
}

func TestParseStarThenRecursiveImport(t *testing.T) {
	imp := importOf(t, "import A::*::**;")
	if imp.Kind != ast.ImportNamespace || !imp.IsRecursive {
		t.Fatalf("imp = %+v", imp)
	}
}

func TestParseImportAll(t *testing.T) {
	imp := importOf(t, "import all A::B::*;")
	if !imp.IsAll || imp.Kind != ast.ImportNamespace {
		t.Fatalf("imp = %+v", imp)
	}
}

func TestParseImportPublicPrefix(t *testing.T) {
	p := newParser("public import A::B;")
	root := p.ParseFile()
	imp := root.Members[0].(*ast.Import)
	if imp.Visibility != ast.VisibilityPublic {
		t.Fatalf("vis = %v", imp.Visibility)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/parser/ -run 'TestParse.*Import' -v`
Expected: FAIL — stub returns `ErrorNode`.

- [ ] **Step 3: Write minimal implementation**

Replace the `parseImport` stub in `namespace.go` with:

```go
// parseImport parses `import [all] QualifiedName [::*|::**] body`.
// Visibility has already been consumed by the caller.
func (p *Parser) parseImport(start int, vis ast.Visibility) *ast.Import {
	p.advance() // 'import' (guaranteed by caller)
	isAll := p.acceptKeyword("all")

	qn := p.parseQualifiedName()
	imp := &ast.Import{
		Visibility: vis,
		IsAll:      isAll,
		Kind:       ast.ImportMembership,
		Imported:   qn,
	}

	// Wildcard tail: `:: *` (namespace) then optional `:: **` (recursive),
	// or `:: **` directly.
	if p.at(lexer.ColonColon) {
		nk := p.peekN(1).Kind
		if nk == lexer.Star {
			p.advance() // ::
			p.advance() // *
			imp.Kind = ast.ImportNamespace
			if p.at(lexer.ColonColon) && p.peekN(1).Kind == lexer.StarStar {
				p.advance() // ::
				p.advance() // **
				imp.IsRecursive = true
			}
		} else if nk == lexer.StarStar {
			p.advance() // ::
			p.advance() // **
			imp.Kind = ast.ImportNamespace
			imp.IsRecursive = true
		}
	}

	imp.Body, imp.HasBody = p.parseNamespaceBody()
	imp.NodeSpan = p.spanFrom(start)
	return imp
}
```

Note on lexer tokens: `**` lexes as a single `StarStar` token (Plan 1), so `A::B::**` is `Identifier ColonColon Identifier ColonColon StarStar`. `A::*::**` is `Identifier ColonColon Star ColonColon StarStar`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/parser/ -run 'TestParse.*Import' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/parser/namespace.go internal/core/parser/namespace_test.go
git commit -m "feat(parser): parse membership, namespace, and recursive imports"
```

### Task 10: Parse Alias + Dependency

**Files:**
- Modify: `internal/core/parser/namespace.go` (replace `parseAlias`/`parseDependency` stubs)
- Test: `internal/core/parser/namespace_test.go`

`parseAlias` (visibility already consumed) parses `alias [<shortName>] [name] for QualifiedName body`. `parseDependency` parses `[# prefixes] dependency [<id> from] client(, client)* to supplier(, supplier)* body`.

- [ ] **Step 1: Write the failing test**

```go
package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

func TestParseAlias(t *testing.T) {
	p := newParser("alias V for Vehicles::Vehicle;")
	root := p.ParseFile()
	al, ok := root.Members[0].(*ast.Alias)
	if !ok {
		t.Fatalf("member = %T", root.Members[0])
	}
	if al.Ident.Name != "V" {
		t.Fatalf("name = %q", al.Ident.Name)
	}
	if al.For == nil || len(al.For.Parts) != 2 {
		t.Fatalf("for = %+v", al.For)
	}
}

func TestParseAliasShortName(t *testing.T) {
	p := newParser("alias <v> Veh for A::B;")
	root := p.ParseFile()
	al := root.Members[0].(*ast.Alias)
	if al.Ident.ShortName != "v" || al.Ident.Name != "Veh" {
		t.Fatalf("ident = %+v", al.Ident)
	}
}

func TestParseDependencySimple(t *testing.T) {
	p := newParser("dependency A to B;")
	root := p.ParseFile()
	dep := root.Members[0].(*ast.Membership).Member.(*ast.Dependency)
	if len(dep.Clients) != 1 || len(dep.Suppliers) != 1 {
		t.Fatalf("dep = %+v", dep)
	}
	if dep.Clients[0].Parts[0].Text != "A" || dep.Suppliers[0].Parts[0].Text != "B" {
		t.Fatalf("dep names = %+v", dep)
	}
}

func TestParseDependencyLists(t *testing.T) {
	p := newParser("dependency X, Y to P, Q, R;")
	root := p.ParseFile()
	dep := root.Members[0].(*ast.Membership).Member.(*ast.Dependency)
	if len(dep.Clients) != 2 || len(dep.Suppliers) != 3 {
		t.Fatalf("dep = %+v", dep)
	}
}

func TestParseDependencyNamed(t *testing.T) {
	p := newParser("dependency <d1> Dep from A to B;")
	root := p.ParseFile()
	dep := root.Members[0].(*ast.Membership).Member.(*ast.Dependency)
	if dep.Ident.ShortName != "d1" || dep.Ident.Name != "Dep" {
		t.Fatalf("ident = %+v", dep.Ident)
	}
	if len(dep.Clients) != 1 || dep.Clients[0].Parts[0].Text != "A" {
		t.Fatalf("clients = %+v", dep.Clients)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/parser/ -run 'TestParseAlias|TestParseDependency' -v`
Expected: FAIL — stubs return `ErrorNode`.

- [ ] **Step 3: Write minimal implementation**

Replace the `parseAlias` and `parseDependency` stubs with:

```go
// parseQualifiedNameList parses `QualifiedName (, QualifiedName)*`.
func (p *Parser) parseQualifiedNameList() []*ast.QualifiedName {
	var list []*ast.QualifiedName
	if qn := p.parseQualifiedName(); qn != nil {
		list = append(list, qn)
	}
	for p.at(lexer.Comma) {
		p.advance() // ,
		if qn := p.parseQualifiedName(); qn != nil {
			list = append(list, qn)
		}
	}
	return list
}

// parseAlias parses `alias [<shortName>] [name] for QualifiedName body`.
func (p *Parser) parseAlias(start int, vis ast.Visibility) *ast.Alias {
	p.advance() // 'alias'
	id := p.parseIdentification()
	al := &ast.Alias{Visibility: vis, Ident: id}
	if !p.acceptKeyword("for") {
		p.error(p.peek().Span, "expected 'for' in alias")
	} else {
		al.For = p.parseQualifiedName()
	}
	al.Body, al.HasBody = p.parseNamespaceBody()
	al.NodeSpan = p.spanFrom(start)
	return al
}

// parseDependency parses
// `[# prefixes] dependency [<id> from] clients to suppliers body`.
func (p *Parser) parseDependency(start int) ast.Node {
	prefixes := p.parsePrefixMetadata()
	if !p.acceptKeyword("dependency") {
		return p.errorNodeSkip(start, "expected 'dependency'")
	}
	dep := &ast.Dependency{Prefixes: prefixes}

	// Optional `<id> [name] from`. The `from` keyword disambiguates: an
	// identification is present only if a `from` follows it.
	if p.identificationThenFrom() {
		dep.Ident = p.parseIdentification()
		p.acceptKeyword("from") // guaranteed
	}

	dep.Clients = p.parseQualifiedNameList()
	if !p.acceptKeyword("to") {
		p.error(p.peek().Span, "expected 'to' in dependency")
	} else {
		dep.Suppliers = p.parseQualifiedNameList()
	}
	dep.Body, dep.HasBody = p.parseNamespaceBody()
	dep.NodeSpan = p.spanFrom(start)
	return dep
}

// identificationThenFrom reports whether the upcoming tokens form an
// identification (`<x> y` / `y`) immediately followed by `from`.
func (p *Parser) identificationThenFrom() bool {
	i := 0
	if p.peekN(i).Kind == lexer.Lt {
		i++ // <
		if k := p.peekN(i).Kind; k == lexer.Identifier || k == lexer.UnrestrictedName {
			i++
		}
		if p.peekN(i).Kind == lexer.Gt {
			i++
		}
	}
	if k := p.peekN(i).Kind; k == lexer.Identifier || k == lexer.UnrestrictedName {
		i++
	}
	t := p.peekN(i)
	return t.Kind == lexer.Keyword && t.KeywordID == "from"
}
```

Note: `parseDependency` handles the ambiguity that a dependency without `from` starts directly with the client list (`dependency A to B`). The bounded `identificationThenFrom` lookahead only treats leading names as an identification when a `from` follows.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/parser/ -run 'TestParseAlias|TestParseDependency' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/parser/namespace.go internal/core/parser/namespace_test.go
git commit -m "feat(parser): parse alias and dependency declarations"
```

### Task 11: Parse Comment / Documentation / TextualRepresentation

**Files:**
- Modify: `internal/core/parser/parser.go` (add `pendingComment` capture in `fill`)
- Modify: `internal/core/parser/namespace.go` (replace `parseComment`/`parseDocumentation`/`parseTextualRepresentation` stubs)
- Test: `internal/core/parser/namespace_test.go`

The `/* ... */` body of these declarations is a `RegularComment` token, which the driver (Task 5) records as *trivia*, not as a stream token. To let these parsers claim it, `fill` also stores the most recent `RegularComment` span in `p.pendingComment`; a `takePendingComment` helper consumes it. Because `fill` records the comment while looking ahead for the next real token, the parser must `peek()` (forcing the comment into the pending slot) before taking it.

Grammar:
- `Comment`: `[comment [<id>] [about ref (, ref)*]] [locale STRING] /* */`. The `comment` keyword and identification are optional; a bare `/* */` is a comment. For Plan 2 we only reach `parseComment` via the `comment` keyword (a floating `/* */` is trivia attached to the next node, which is correct).
- `Documentation`: `doc [<id>] [locale STRING] /* */`.
- `TextualRepresentation`: `[rep [<id>]] language STRING /* */`.

- [ ] **Step 1: Write the failing test**

```go
package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

func TestParseComment(t *testing.T) {
	p := newParser("comment C /* hello */")
	root := p.ParseFile()
	c := root.Members[0].(*ast.Membership).Member.(*ast.Comment)
	if c.Ident.Name != "C" {
		t.Fatalf("ident = %+v", c.Ident)
	}
	if p.src.Text(c.BodySpan) != "/* hello */" {
		t.Fatalf("body = %q", p.src.Text(c.BodySpan))
	}
}

func TestParseCommentAbout(t *testing.T) {
	p := newParser("comment about A, B /* x */")
	root := p.ParseFile()
	c := root.Members[0].(*ast.Membership).Member.(*ast.Comment)
	if len(c.About) != 2 {
		t.Fatalf("about = %+v", c.About)
	}
}

func TestParseDocumentation(t *testing.T) {
	p := newParser("doc D /* the docs */")
	root := p.ParseFile()
	d := root.Members[0].(*ast.Membership).Member.(*ast.Documentation)
	if d.Ident.Name != "D" || p.src.Text(d.BodySpan) != "/* the docs */" {
		t.Fatalf("doc = %+v body=%q", d, p.src.Text(d.BodySpan))
	}
}

func TestParseTextualRepresentation(t *testing.T) {
	p := newParser(`rep R language "html" /* <b>hi</b> */`)
	root := p.ParseFile()
	r := root.Members[0].(*ast.Membership).Member.(*ast.TextualRepresentation)
	if r.Ident.Name != "R" || r.Language != `"html"` {
		t.Fatalf("rep = %+v", r)
	}
	if p.src.Text(r.BodySpan) != "/* <b>hi</b> */" {
		t.Fatalf("body = %q", p.src.Text(r.BodySpan))
	}
}

func TestParseTextualRepresentationNoRep(t *testing.T) {
	p := newParser(`language "text" /* body */`)
	root := p.ParseFile()
	r := root.Members[0].(*ast.Membership).Member.(*ast.TextualRepresentation)
	if r.Language != `"text"` {
		t.Fatalf("lang = %q", r.Language)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/parser/ -run 'TestParseComment|TestParseDocumentation|TestParseTextualRepresentation' -v`
Expected: FAIL — stubs return `ErrorNode`.

- [ ] **Step 3: Write minimal implementation**

In `parser.go`, add a `pendingComment` field and populate it in `fill`. Change the `Parser` struct and `fill` loop:

```go
// add to Parser struct:
//   pendingComment source.Span // span of the most recent RegularComment trivia
//   hasPendingComment bool
```

Update the trivia loop inside `fill` to capture the comment span:

```go
		for tok.IsTrivia() || tok.Kind == lexer.RegularComment {
			p.triv = append(p.triv, triviaOf(tok))
			if tok.Kind == lexer.RegularComment {
				p.pendingComment = tok.Span
				p.hasPendingComment = true
			}
			if tok.Kind == lexer.EOF {
				break
			}
			tok = p.lx.Next()
		}
```

And add the take helper:

```go
// takePendingComment consumes the most recent RegularComment trivia span, if
// any. Callers peek() first to force a pending comment into the slot.
func (p *Parser) takePendingComment() (source.Span, bool) {
	if !p.hasPendingComment {
		return source.Span{}, false
	}
	sp := p.pendingComment
	p.hasPendingComment = false
	return sp, true
}
```

In `namespace.go`, replace the three stubs:

```go
// expectCommentBody peeks (forcing any trailing `/* */` into the pending slot),
// then consumes it. Records a diagnostic when absent.
func (p *Parser) expectCommentBody(start int) source.Span {
	p.peek() // force fill so a trailing RegularComment lands in pendingComment
	if sp, ok := p.takePendingComment(); ok {
		return sp
	}
	p.error(p.peek().Span, "expected a /* ... */ comment body")
	return p.spanFrom(start)
}

// parseComment parses `comment [<id>] [about refs] [locale s] /* */`.
func (p *Parser) parseComment(start int) ast.Node {
	p.advance() // 'comment'
	c := &ast.Comment{}
	// Optional identification, but not if the next token is `about`/`locale`.
	if p.atName() && !p.atKeyword("about") && !p.atKeyword("locale") {
		c.Ident = p.parseIdentification()
	}
	if p.acceptKeyword("about") {
		c.About = p.parseQualifiedNameList()
	}
	if p.acceptKeyword("locale") {
		if tok, ok := p.expect(lexer.String, "expected locale string"); ok {
			c.Locale = p.src.Text(tok.Span)
		}
	}
	c.BodySpan = p.expectCommentBody(start)
	c.NodeSpan = p.spanFrom(start)
	return c
}

// parseDocumentation parses `doc [<id>] [locale s] /* */`.
func (p *Parser) parseDocumentation(start int) ast.Node {
	p.advance() // 'doc'
	d := &ast.Documentation{}
	if p.atName() && !p.atKeyword("locale") {
		d.Ident = p.parseIdentification()
	}
	if p.acceptKeyword("locale") {
		if tok, ok := p.expect(lexer.String, "expected locale string"); ok {
			d.Locale = p.src.Text(tok.Span)
		}
	}
	d.BodySpan = p.expectCommentBody(start)
	d.NodeSpan = p.spanFrom(start)
	return d
}

// parseTextualRepresentation parses `[rep [<id>]] language STRING /* */`.
func (p *Parser) parseTextualRepresentation(start int) ast.Node {
	r := &ast.TextualRepresentation{}
	if p.acceptKeyword("rep") {
		if p.atName() && !p.atKeyword("language") {
			r.Ident = p.parseIdentification()
		}
	}
	if !p.acceptKeyword("language") {
		return p.errorNodeSkip(start, "expected 'language'")
	}
	if tok, ok := p.expect(lexer.String, "expected representation language string"); ok {
		r.Language = p.src.Text(tok.Span)
	}
	r.BodySpan = p.expectCommentBody(start)
	r.NodeSpan = p.spanFrom(start)
	return r
}
```

Also add `"github.com/Open-MBEE/Systemica/internal/core/source"` to `namespace.go` imports (needed for `source.Span` return type of `expectCommentBody`).

Note: capturing the comment as a *span* (not attaching it as leading trivia to a following node) is fine because these declarations own the `/* */`. A floating `/* */` with no `comment`/`doc`/`rep`/`language` head remains trivia on the next node, which is the correct Plan 2 behavior.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/parser/ -run 'TestParseComment|TestParseDocumentation|TestParseTextualRepresentation' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/parser/parser.go internal/core/parser/namespace.go internal/core/parser/namespace_test.go
git commit -m "feat(parser): parse comment, documentation, textual representation bodies"
```

### Task 12: Expression parser — primary/base (literals, refs, paren, null)

**Files:**
- Create: `internal/core/parser/expr.go`
- Test: `internal/core/parser/expr_test.go`

The expression sub-parser is a Pratt/precedence-climbing parser. This task builds the *base* level: literals, `null`, parenthesized/sequence expressions, feature references, body expressions `{ ... }`, and `new` constructors — the leaves the operator ladder (Task 13) and postfix chain (Task 14) build upon. `ParseExpression` is the public entry (used by tests now and by `filter` members / feature values later). In Task 12 it delegates straight to `parsePrimary`; Tasks 13-14 insert the ladder above it.

Base forms (from `KerMLExpressions.xtext`):
- literals: `true` `false`, `STRING_VALUE`, `DECIMAL_VALUE` (integer), `Real`, `*` (infinity, only in expression/base position)
- `null`
- `( )` empty → SequenceExpr with no elements; `( expr (, expr)* )` → single expr if one element else SequenceExpr
- `new QualifiedName ( args )` → ConstructorExpr
- `QualifiedName` → FeatureReference; a bare `Type(args)` with no receiver → InvocationExpr recognized here when `(` follows the name
- `{ (in param ;)* resultExpr }` → BodyExpr

- [ ] **Step 1: Write the failing test**

```go
package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

func exprOf(t *testing.T, src string) ast.Node {
	t.Helper()
	p := newParser(src)
	e := p.ParseExpression()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("diags for %q = %+v", src, p.Diagnostics)
	}
	return e
}

func TestParseLiteralInteger(t *testing.T) {
	e := exprOf(t, "42")
	lit, ok := e.(*ast.LiteralInteger)
	if !ok || lit.Value != "42" {
		t.Fatalf("e = %#v", e)
	}
}

func TestParseLiteralReal(t *testing.T) {
	if _, ok := exprOf(t, "3.14").(*ast.LiteralReal); !ok {
		t.Fatalf("not a real")
	}
}

func TestParseLiteralBool(t *testing.T) {
	if b := exprOf(t, "true").(*ast.LiteralBool); !b.Value {
		t.Fatalf("expected true")
	}
	if b := exprOf(t, "false").(*ast.LiteralBool); b.Value {
		t.Fatalf("expected false")
	}
}

func TestParseLiteralString(t *testing.T) {
	if s := exprOf(t, `"hi"`).(*ast.LiteralString); s.Value != `"hi"` {
		t.Fatalf("s = %q", s.Value)
	}
}

func TestParseNull(t *testing.T) {
	if _, ok := exprOf(t, "null").(*ast.NullExpr); !ok {
		t.Fatalf("not null")
	}
}

func TestParseFeatureReference(t *testing.T) {
	fr := exprOf(t, "A::B").(*ast.FeatureReference)
	if fr.Name == nil || len(fr.Name.Parts) != 2 {
		t.Fatalf("fr = %+v", fr)
	}
}

func TestParseParenSingle(t *testing.T) {
	// A single parenthesized expression collapses to that expression.
	if _, ok := exprOf(t, "(42)").(*ast.LiteralInteger); !ok {
		t.Fatalf("expected literal inside parens")
	}
}

func TestParseSequence(t *testing.T) {
	seq := exprOf(t, "(1, 2, 3)").(*ast.SequenceExpr)
	if len(seq.Elements) != 3 {
		t.Fatalf("seq = %+v", seq)
	}
}

func TestParseEmptySequence(t *testing.T) {
	seq := exprOf(t, "()").(*ast.SequenceExpr)
	if len(seq.Elements) != 0 {
		t.Fatalf("seq = %+v", seq)
	}
}

func TestParseConstructor(t *testing.T) {
	c := exprOf(t, "new Vehicle(1, 2)").(*ast.ConstructorExpr)
	if c.Type == nil || len(c.Args) != 2 {
		t.Fatalf("c = %+v", c)
	}
}

func TestParseBodyExpr(t *testing.T) {
	b := exprOf(t, "{ in x; x }").(*ast.BodyExpr)
	if len(b.Params) != 1 || b.Params[0].Name != "x" {
		t.Fatalf("params = %+v", b.Params)
	}
	if _, ok := b.Result.(*ast.FeatureReference); !ok {
		t.Fatalf("result = %#v", b.Result)
	}
}

func TestParseInfinity(t *testing.T) {
	if _, ok := exprOf(t, "*").(*ast.LiteralInfinity); !ok {
		t.Fatalf("expected infinity literal")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/parser/ -run 'TestParseLiteral|TestParseNull|TestParseFeatureReference|TestParseParen|TestParseSequence|TestParseEmptySequence|TestParseConstructor|TestParseBodyExpr|TestParseInfinity' -v`
Expected: FAIL — `ParseExpression` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
package parser

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
)

// ParseExpression is the public entry to the expression sub-parser.
// Task 12 delegates to parsePrimary; Tasks 13-14 layer the operator ladder
// and postfix chain above it.
func (p *Parser) ParseExpression() ast.Node {
	return p.parsePrimary()
}

// parsePrimary parses a base expression (Task 14 extends it with postfixes).
func (p *Parser) parsePrimary() ast.Node {
	return p.parseBase()
}

// parseBase parses a leaf/base expression.
func (p *Parser) parseBase() ast.Node {
	start := p.peek().Span.Offset
	trivia := p.takeTrivia()

	setBase := func(n ast.Node) ast.Node {
		if nb, ok := n.(interface{ SetLeadingTrivia([]ast.Trivia) }); ok {
			nb.SetLeadingTrivia(trivia)
		}
		return n
	}

	switch {
	case p.atKeyword("null"):
		p.advance()
		n := &ast.NullExpr{}
		n.NodeSpan = p.spanFrom(start)
		return setBase(n)

	case p.atKeyword("true"), p.atKeyword("false"):
		tok := p.advance()
		n := &ast.LiteralBool{Value: tok.KeywordID == "true"}
		n.NodeSpan = p.spanFrom(start)
		return setBase(n)

	case p.atKeyword("new"):
		return setBase(p.parseConstructor(start))

	case p.at(lexer.Decimal):
		tok := p.advance()
		n := &ast.LiteralInteger{Value: p.src.Text(tok.Span)}
		n.NodeSpan = p.spanFrom(start)
		return setBase(n)

	case p.at(lexer.Real):
		tok := p.advance()
		n := &ast.LiteralReal{Value: p.src.Text(tok.Span)}
		n.NodeSpan = p.spanFrom(start)
		return setBase(n)

	case p.at(lexer.String):
		tok := p.advance()
		n := &ast.LiteralString{Value: p.src.Text(tok.Span)}
		n.NodeSpan = p.spanFrom(start)
		return setBase(n)

	case p.at(lexer.Star):
		// Infinity literal in expression position.
		p.advance()
		n := &ast.LiteralInfinity{}
		n.NodeSpan = p.spanFrom(start)
		return setBase(n)

	case p.at(lexer.LParen):
		return setBase(p.parseParenOrSequence(start))

	case p.at(lexer.LBrace):
		return setBase(p.parseBodyExpr(start))

	case p.atName():
		qn := p.parseQualifiedName()
		// A bare `Type(args)` invocation with no receiver is recognized here.
		if p.at(lexer.LParen) {
			return setBase(p.parseInvocationTail(start, nil, qn))
		}
		fr := &ast.FeatureReference{Name: qn}
		fr.NodeSpan = p.spanFrom(start)
		return setBase(fr)

	default:
		p.error(p.peek().Span, "expected an expression")
		en := &ast.ErrorNode{Message: "expected an expression"}
		if !p.atEOF() && !p.at(lexer.RParen) && !p.at(lexer.RBrace) && !p.at(lexer.Semicolon) {
			p.advance() // ensure progress
		}
		en.NodeSpan = p.spanFrom(start)
		return setBase(en)
	}
}

// parseParenOrSequence parses `( )`, `( expr )`, or `( expr, expr, ... )`.
func (p *Parser) parseParenOrSequence(start int) ast.Node {
	p.advance() // (
	var elems []ast.Node
	if !p.at(lexer.RParen) {
		elems = append(elems, p.ParseExpression())
		for p.at(lexer.Comma) {
			p.advance() // ,
			elems = append(elems, p.ParseExpression())
		}
	}
	p.expect(lexer.RParen, "expected ')'")
	if len(elems) == 1 {
		return elems[0]
	}
	seq := &ast.SequenceExpr{Elements: elems}
	seq.NodeSpan = p.spanFrom(start)
	return seq
}

// parseConstructor parses `new QualifiedName ( args )`.
func (p *Parser) parseConstructor(start int) ast.Node {
	p.advance() // new
	qn := p.parseQualifiedName()
	c := &ast.ConstructorExpr{Type: qn}
	if p.at(lexer.LParen) {
		c.Args, _ = p.parseArgList()
	}
	c.NodeSpan = p.spanFrom(start)
	return c
}

// parseArgList parses `( )`, positional `( a, b )`, or named `( n=a, m=b )`.
// Returns positional args and named args (one slice empty).
func (p *Parser) parseArgList() ([]ast.Node, []ast.NamedArg) {
	p.expect(lexer.LParen, "expected '('")
	var pos []ast.Node
	var named []ast.NamedArg
	if p.at(lexer.RParen) {
		p.advance()
		return pos, named
	}
	// Named if the first token is a name immediately followed by '='.
	if p.namedArgAhead() {
		for {
			name := p.parseQualifiedName()
			p.expect(lexer.Eq, "expected '=' in named argument")
			val := p.ParseExpression()
			named = append(named, ast.NamedArg{Name: name, Value: val})
			if !p.at(lexer.Comma) {
				break
			}
			p.advance()
		}
	} else {
		for {
			pos = append(pos, p.ParseExpression())
			if !p.at(lexer.Comma) {
				break
			}
			p.advance()
		}
	}
	p.expect(lexer.RParen, "expected ')'")
	return pos, named
}

// namedArgAhead reports whether the arg list is `name = ...` (named form).
func (p *Parser) namedArgAhead() bool {
	if !p.atName() {
		return false
	}
	// Skip a qualified name, then check for '='.
	i := 1
	for p.peekN(i).Kind == lexer.ColonColon {
		i++
		if k := p.peekN(i).Kind; k != lexer.Identifier && k != lexer.UnrestrictedName {
			return false
		}
		i++
	}
	return p.peekN(i).Kind == lexer.Eq
}

// parseInvocationTail parses `( args )` after a receiver/type has been read.
func (p *Parser) parseInvocationTail(start int, recv ast.Node, typ *ast.QualifiedName) ast.Node {
	args, named := p.parseArgList()
	inv := &ast.InvocationExpr{Operand: recv, Type: typ, Args: args, NamedArgs: named}
	inv.NodeSpan = p.spanFrom(start)
	return inv
}

// parseBodyExpr parses `{ (in param ;)* resultExpr }`.
func (p *Parser) parseBodyExpr(start int) ast.Node {
	p.advance() // {
	b := &ast.BodyExpr{}
	for p.atKeyword("in") {
		p.advance() // in
		if seg, ok := p.parseNameSegment(); ok {
			b.Params = append(b.Params, ast.BodyParam{Name: seg.Text, Span: seg.Span})
		}
		p.expect(lexer.Semicolon, "expected ';' after body parameter")
	}
	if !p.at(lexer.RBrace) {
		b.Result = p.ParseExpression()
	}
	p.expect(lexer.RBrace, "expected '}'")
	b.NodeSpan = p.spanFrom(start)
	return b
}
```

Note: `parseInvocationTail` and `parseArgList` are reused in Task 14 for the `->` and `Type(...)` postfix forms. `namedArgAhead` bounded lookahead distinguishes named from positional argument lists.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/parser/ -run 'TestParseLiteral|TestParseNull|TestParseFeatureReference|TestParseParen|TestParseSequence|TestParseEmptySequence|TestParseConstructor|TestParseBodyExpr|TestParseInfinity' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/parser/expr.go internal/core/parser/expr_test.go
git commit -m "feat(parser): parse base/primary expressions (literals, refs, parens, body, constructor)"
```

### Task 13: Expression parser — Pratt operator ladder (binary/unary)

**Files:**
- Modify: `internal/core/parser/expr.go`
- Test: `internal/core/parser/expr_test.go`

Insert the full operator precedence ladder between `ParseExpression` and `parsePrimary`. Uses precedence-climbing: a table maps each binary operator token to a `(precedence, rightAssoc)` entry; unary/prefix operators and the ternary conditional and the classification operators (`hastype`/`istype`/`as`/`meta`/`@`/`@@` with a type RHS) are handled explicitly.

Precedence (low→high binding), matching `KerMLExpressions.xtext`:
1. `if C ? A else B` (conditional ternary)
2. `??` (null-coalesce)
3. `implies`
4. `|` / `or`
5. `xor`
6. `&` / `and`
7. `==` `!=` `===` `!==`
8. classification `hastype` `istype` `@` `@@` `as` `meta` (RHS is a type reference)
9. `<` `>` `<=` `>=`
10. `..` (range, single, non-assoc)
11. `+` `-`
12. `*` `/` `%`
13. `**` `^` (right-assoc)
14. unary prefix `+` `-` `~` `not`
15. extent prefix `all`
16. primary/postfix (Tasks 12, 14)

- [ ] **Step 1: Write the failing test**

```go
package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

func dumpExpr(t *testing.T, src string) string {
	t.Helper()
	p := newParser(src)
	e := p.ParseExpression()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("diags for %q = %+v", src, p.Diagnostics)
	}
	return strings.TrimSpace(ast.Dump(e))
}

func TestPrecedenceAddMul(t *testing.T) {
	got := dumpExpr(t, "1 + 2 * 3")
	want := strings.Join([]string{
		`(OperatorExpr operator="+"`,
		`  (LiteralInteger value="1")`,
		`  (OperatorExpr operator="*"`,
		`    (LiteralInteger value="2")`,
		`    (LiteralInteger value="3")))`,
	}, "\n")
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestLeftAssoc(t *testing.T) {
	got := dumpExpr(t, "1 - 2 - 3")
	want := strings.Join([]string{
		`(OperatorExpr operator="-"`,
		`  (OperatorExpr operator="-"`,
		`    (LiteralInteger value="1")`,
		`    (LiteralInteger value="2"))`,
		`  (LiteralInteger value="3"))`,
	}, "\n")
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestPowRightAssoc(t *testing.T) {
	got := dumpExpr(t, "2 ** 3 ** 4")
	want := strings.Join([]string{
		`(OperatorExpr operator="**"`,
		`  (LiteralInteger value="2")`,
		`  (OperatorExpr operator="**"`,
		`    (LiteralInteger value="3")`,
		`    (LiteralInteger value="4")))`,
	}, "\n")
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestUnaryNeg(t *testing.T) {
	e := exprOf(t, "-5").(*ast.OperatorExpr)
	if e.Operator != ast.OpNeg || len(e.Operands) != 1 {
		t.Fatalf("e = %+v", e)
	}
}

func TestNotOperator(t *testing.T) {
	e := exprOf(t, "not x").(*ast.OperatorExpr)
	if e.Operator != ast.OpNot {
		t.Fatalf("op = %v", e.Operator)
	}
}

func TestAllExtent(t *testing.T) {
	e := exprOf(t, "all X").(*ast.OperatorExpr)
	if e.Operator != ast.OpAll {
		t.Fatalf("op = %v", e.Operator)
	}
}

func TestConditional(t *testing.T) {
	e := exprOf(t, "if c ? a else b").(*ast.OperatorExpr)
	if e.Operator != ast.OpConditional || len(e.Operands) != 3 {
		t.Fatalf("e = %+v", e)
	}
}

func TestClassificationAs(t *testing.T) {
	e := exprOf(t, "x as Integer").(*ast.OperatorExpr)
	if e.Operator != ast.OpAs || e.TypeRef == nil {
		t.Fatalf("e = %+v", e)
	}
	if e.TypeRef.Parts[0].Text != "Integer" {
		t.Fatalf("typeref = %+v", e.TypeRef)
	}
}

func TestRange(t *testing.T) {
	e := exprOf(t, "1 .. 10").(*ast.OperatorExpr)
	if e.Operator != ast.OpRange || len(e.Operands) != 2 {
		t.Fatalf("e = %+v", e)
	}
}

func TestImplies(t *testing.T) {
	e := exprOf(t, "a implies b").(*ast.OperatorExpr)
	if e.Operator != ast.OpImplies {
		t.Fatalf("op = %v", e.Operator)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/parser/ -run 'TestPrecedence|TestLeftAssoc|TestPowRightAssoc|TestUnary|TestNotOperator|TestAllExtent|TestConditional|TestClassification|TestRange|TestImplies' -v`
Expected: FAIL — `ParseExpression` still delegates straight to `parsePrimary`, so `1 + 2 * 3` returns only `1`, leaving diagnostics/leftover — assertions fail.

- [ ] **Step 3: Write minimal implementation**

Replace `ParseExpression`'s body and add the ladder. Keep `parsePrimary` as-is (Task 14 extends it).

```go
// ParseExpression parses a full expression (conditional at the lowest level).
func (p *Parser) ParseExpression() ast.Node {
	return p.parseConditional()
}

// parseConditional parses `if cond ? then else else` or falls through.
func (p *Parser) parseConditional() ast.Node {
	if p.atKeyword("if") {
		start := p.peek().Span.Offset
		p.advance() // if
		cond := p.parseBinary(precNullCoalesce)
		p.expect(lexer.Question, "expected '?' in conditional")
		thn := p.parseBinary(precNullCoalesce)
		p.expect2Keyword("else")
		els := p.parseConditional()
		e := &ast.OperatorExpr{Operator: ast.OpConditional, Operands: []ast.Node{cond, thn, els}}
		e.NodeSpan = p.spanFrom(start)
		return e
	}
	return p.parseBinary(precNullCoalesce)
}

// Precedence levels (higher binds tighter). Conditional is handled separately.
const (
	precNullCoalesce = iota + 1 // ??
	precImplies                 // implies
	precOr                      // | or
	precXor                     // xor
	precAnd                     // & and
	precEquality                // == != === !==
	precClassify                // hastype istype @ @@ as meta
	precRelational              // < > <= >=
	precRange                   // ..
	precAdditive                // + -
	precMultiplicative          // * / %
	precExponent                // ** ^  (right-assoc)
	precUnary                   // prefix + - ~ not
	precExtent                  // all
)

type binOp struct {
	op         ast.OperatorKind
	prec       int
	rightAssoc bool
	classify   bool // RHS is a type reference, stored in TypeRef
}

// binaryOpFor returns the binary operator for the current token, if any.
func (p *Parser) binaryOpFor() (binOp, bool) {
	t := p.peek()
	switch t.Kind {
	case lexer.QuestionQ:
		return binOp{ast.OpNullCoalesce, precNullCoalesce, false, false}, true
	case lexer.Pipe:
		return binOp{ast.OpOr, precOr, false, false}, true
	case lexer.Amp:
		return binOp{ast.OpAnd, precAnd, false, false}, true
	case lexer.EqEq:
		return binOp{ast.OpEq, precEquality, false, false}, true
	case lexer.NotEq:
		return binOp{ast.OpNeq, precEquality, false, false}, true
	case lexer.EqEqEq:
		return binOp{ast.OpEqEqEq, precEquality, false, false}, true
	case lexer.NotEqEq:
		return binOp{ast.OpNeqEqEq, precEquality, false, false}, true
	case lexer.At:
		return binOp{ast.OpAt, precClassify, false, true}, true
	case lexer.AtAt:
		return binOp{ast.OpMetaAt, precClassify, false, true}, true
	case lexer.Lt:
		return binOp{ast.OpLt, precRelational, false, false}, true
	case lexer.Gt:
		return binOp{ast.OpGt, precRelational, false, false}, true
	case lexer.Le:
		return binOp{ast.OpLe, precRelational, false, false}, true
	case lexer.Ge:
		return binOp{ast.OpGe, precRelational, false, false}, true
	case lexer.DotDot:
		return binOp{ast.OpRange, precRange, false, false}, true
	case lexer.Plus:
		return binOp{ast.OpAdd, precAdditive, false, false}, true
	case lexer.Minus:
		return binOp{ast.OpSub, precAdditive, false, false}, true
	case lexer.Star:
		return binOp{ast.OpMul, precMultiplicative, false, false}, true
	case lexer.Slash:
		return binOp{ast.OpDiv, precMultiplicative, false, false}, true
	case lexer.Percent:
		return binOp{ast.OpMod, precMultiplicative, false, false}, true
	case lexer.StarStar:
		return binOp{ast.OpPow, precExponent, true, false}, true
	case lexer.Caret:
		return binOp{ast.OpPow, precExponent, true, false}, true
	case lexer.Keyword:
		switch t.KeywordID {
		case "implies":
			return binOp{ast.OpImplies, precImplies, false, false}, true
		case "or":
			return binOp{ast.OpConditionalOr, precOr, false, false}, true
		case "xor":
			return binOp{ast.OpXor, precXor, false, false}, true
		case "and":
			return binOp{ast.OpConditionalAnd, precAnd, false, false}, true
		case "hastype":
			return binOp{ast.OpHasType, precClassify, false, true}, true
		case "istype":
			return binOp{ast.OpIsType, precClassify, false, true}, true
		case "as":
			return binOp{ast.OpAs, precClassify, false, true}, true
		case "meta":
			return binOp{ast.OpMeta, precClassify, false, true}, true
		}
	}
	return binOp{}, false
}

// parseBinary parses a binary expression at or above the given precedence.
func (p *Parser) parseBinary(minPrec int) ast.Node {
	start := p.peek().Span.Offset
	left := p.parseUnary()
	for {
		bop, ok := p.binaryOpFor()
		if !ok || bop.prec < minPrec {
			break
		}
		p.advance() // operator
		e := &ast.OperatorExpr{Operator: bop.op}
		if bop.classify {
			e.Operands = []ast.Node{left}
			e.TypeRef = p.parseQualifiedName()
		} else {
			nextMin := bop.prec + 1
			if bop.rightAssoc {
				nextMin = bop.prec
			}
			right := p.parseBinary(nextMin)
			e.Operands = []ast.Node{left, right}
		}
		e.NodeSpan = p.spanFrom(start)
		left = e
	}
	return left
}

// parseUnary parses prefix operators and the `all` extent, then a primary.
func (p *Parser) parseUnary() ast.Node {
	start := p.peek().Span.Offset
	var op ast.OperatorKind
	switch {
	case p.at(lexer.Plus):
		op = ast.OpPos
	case p.at(lexer.Minus):
		op = ast.OpNeg
	case p.at(lexer.Tilde):
		op = ast.OpBitNot
	case p.atKeyword("not"):
		op = ast.OpNot
	case p.atKeyword("all"):
		p.advance()
		operand := p.parseUnary()
		e := &ast.OperatorExpr{Operator: ast.OpAll, Operands: []ast.Node{operand}}
		e.NodeSpan = p.spanFrom(start)
		return e
	default:
		return p.parsePrimary()
	}
	p.advance() // prefix operator
	operand := p.parseUnary()
	e := &ast.OperatorExpr{Operator: op, Operands: []ast.Node{operand}}
	e.NodeSpan = p.spanFrom(start)
	return e
}

// expect2Keyword records a diagnostic if the given keyword is not present,
// consuming it when it is.
func (p *Parser) expect2Keyword(kw string) bool {
	if p.acceptKeyword(kw) {
		return true
	}
	p.error(p.peek().Span, "expected '"+kw+"'")
	return false
}
```

Note: the ternary uses `?` (`lexer.Question`) as the separator between condition and then-branch, and the `else` keyword before the else-branch, matching `ConditionalExpression` in the pilot. Classification operators consume a `QualifiedName` as the RHS type and store it in `TypeRef` (dumped as a trailing `FeatureReference` child via `operandsWithTypeRef`).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/parser/ -run 'TestPrecedence|TestLeftAssoc|TestPowRightAssoc|TestUnary|TestNotOperator|TestAllExtent|TestConditional|TestClassification|TestRange|TestImplies|TestParseLiteral|TestParseFeatureReference' -v`
Expected: PASS (base-expression tests from Task 12 still pass).

- [ ] **Step 5: Commit**

```bash
git add internal/core/parser/expr.go internal/core/parser/expr_test.go
git commit -m "feat(parser): add Pratt operator ladder (binary, unary, conditional, classification)"
```

### Task 14: Expression parser — postfix (chain, index, invoke, collect, select)

**Files:**
- Modify: `internal/core/parser/expr.go`
- Test: `internal/core/parser/expr_test.go`

Extend `parsePrimary` so that after a base expression it consumes a chain of postfix operators (left-associative, tightest binding):
- `. member` → FeatureChainExpr (feature chain access)
- `# ( index )` → IndexExpr (sequence index; `#` then a parenthesized expression)
- `[ index ]` → IndexExpr (operator index / bracket)
- `-> Type ( args )` or `-> Type body` or `-> Type funcref` → InvocationExpr with receiver
- `. { body }` → CollectExpr
- `.? { body }` → SelectExpr

Each postfix may be followed by more postfixes. `parsePrimary` becomes: parse base, then loop applying postfixes until none match.

- [ ] **Step 1: Write the failing test**

```go
package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

func TestPostfixFeatureChain(t *testing.T) {
	e := exprOf(t, "a.b").(*ast.FeatureChainExpr)
	if e.Member == nil || e.Member.Parts[0].Text != "b" {
		t.Fatalf("member = %+v", e.Member)
	}
	if _, ok := e.Operand.(*ast.FeatureReference); !ok {
		t.Fatalf("operand = %#v", e.Operand)
	}
}

func TestPostfixChainDeep(t *testing.T) {
	// a.b.c  => (chain (chain a b) c)
	e := exprOf(t, "a.b.c").(*ast.FeatureChainExpr)
	if e.Member.Parts[0].Text != "c" {
		t.Fatalf("outer member = %+v", e.Member)
	}
	if _, ok := e.Operand.(*ast.FeatureChainExpr); !ok {
		t.Fatalf("inner = %#v", e.Operand)
	}
}

func TestPostfixIndexHash(t *testing.T) {
	e := exprOf(t, "a#(0)").(*ast.IndexExpr)
	if _, ok := e.Index.(*ast.LiteralInteger); !ok {
		t.Fatalf("index = %#v", e.Index)
	}
}

func TestPostfixIndexBracket(t *testing.T) {
	e := exprOf(t, "a[1]").(*ast.IndexExpr)
	if _, ok := e.Index.(*ast.LiteralInteger); !ok {
		t.Fatalf("index = %#v", e.Index)
	}
}

func TestPostfixInvocationArrow(t *testing.T) {
	e := exprOf(t, "coll->select(x)").(*ast.InvocationExpr)
	if e.Type == nil || e.Type.Parts[0].Text != "select" {
		t.Fatalf("type = %+v", e.Type)
	}
	if e.Operand == nil {
		t.Fatalf("expected receiver operand")
	}
	if len(e.Args) != 1 {
		t.Fatalf("args = %+v", e.Args)
	}
}

func TestPostfixCollect(t *testing.T) {
	e := exprOf(t, "a.{ x }").(*ast.CollectExpr)
	if _, ok := e.Body.(*ast.BodyExpr); !ok {
		t.Fatalf("body = %#v", e.Body)
	}
}

func TestPostfixSelect(t *testing.T) {
	e := exprOf(t, "a.?{ x }").(*ast.SelectExpr)
	if _, ok := e.Body.(*ast.BodyExpr); !ok {
		t.Fatalf("body = %#v", e.Body)
	}
}

func TestPostfixArrowThenChain(t *testing.T) {
	// coll->size().x  => chain( invocation(coll,size), x )
	e := exprOf(t, "coll->size().x").(*ast.FeatureChainExpr)
	if e.Member.Parts[0].Text != "x" {
		t.Fatalf("member = %+v", e.Member)
	}
	if _, ok := e.Operand.(*ast.InvocationExpr); !ok {
		t.Fatalf("operand = %#v", e.Operand)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/parser/ -run 'TestPostfix' -v`
Expected: FAIL — `parsePrimary` does not yet apply postfixes.

- [ ] **Step 3: Write minimal implementation**

Replace `parsePrimary`:

```go
// parsePrimary parses a base expression and then any chain of postfix
// operators (feature chain, index, invocation, collect, select).
func (p *Parser) parsePrimary() ast.Node {
	start := p.peek().Span.Offset
	expr := p.parseBase()
	return p.parsePostfixes(start, expr)
}

// parsePostfixes applies zero or more postfix operators to expr.
func (p *Parser) parsePostfixes(start int, expr ast.Node) ast.Node {
	for {
		switch {
		case p.at(lexer.Dot):
			// `.member` (chain) or `.{ body }` (collect).
			p.advance() // .
			if p.at(lexer.LBrace) {
				body := p.parseBodyExpr(p.peek().Span.Offset)
				c := &ast.CollectExpr{Operand: expr, Body: body}
				c.NodeSpan = p.spanFrom(start)
				expr = c
				continue
			}
			member := p.parseQualifiedName()
			fc := &ast.FeatureChainExpr{Operand: expr, Member: member}
			fc.NodeSpan = p.spanFrom(start)
			expr = fc

		case p.at(lexer.DotQuestion):
			// `.?{ body }` (select).
			p.advance() // .?
			body := p.parseBodyExpr(p.peek().Span.Offset)
			s := &ast.SelectExpr{Operand: expr, Body: body}
			s.NodeSpan = p.spanFrom(start)
			expr = s

		case p.at(lexer.Hash):
			// `#( index )` sequence index.
			p.advance() // #
			p.expect(lexer.LParen, "expected '(' after '#'")
			idx := p.ParseExpression()
			p.expect(lexer.RParen, "expected ')'")
			ix := &ast.IndexExpr{Operand: expr, Index: idx}
			ix.NodeSpan = p.spanFrom(start)
			expr = ix

		case p.at(lexer.LBracket):
			// `[ index ]` operator index.
			p.advance() // [
			idx := p.ParseExpression()
			p.expect(lexer.RBracket, "expected ']'")
			ix := &ast.IndexExpr{Operand: expr, Index: idx}
			ix.NodeSpan = p.spanFrom(start)
			expr = ix

		case p.at(lexer.Arrow):
			// `-> Type ( args )` invocation with receiver.
			p.advance() // ->
			typ := p.parseQualifiedName()
			inv := &ast.InvocationExpr{Operand: expr, Type: typ}
			if p.at(lexer.LParen) {
				inv.Args, inv.NamedArgs = p.parseArgList()
			} else if p.at(lexer.LBrace) {
				// Function reference given as a body: store as a single arg.
				inv.Args = []ast.Node{p.parseBodyExpr(p.peek().Span.Offset)}
			}
			inv.NodeSpan = p.spanFrom(start)
			expr = inv

		default:
			return expr
		}
	}
}
```

Note: `parseBase` no longer needs its inline `Type(args)` recognition to be the *only* invocation path, but it stays (a bare `Type(args)` with no preceding receiver). The `->` form always sets `Operand` (receiver); the bare form leaves `Operand` nil. `parsePostfixes` starts from the same `start` offset so nested spans cover the whole postfix chain.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/parser/ -run 'TestPostfix|TestParse|TestPrecedence' -v`
Expected: PASS (all prior expression tests still green).

- [ ] **Step 5: Commit**

```bash
git add internal/core/parser/expr.go internal/core/parser/expr_test.go
git commit -m "feat(parser): parse postfix expressions (chain, index, invocation, collect, select)"
```

### Task 15: Error recovery — panic-mode sync + missing-token insertion

**Files:**
- Modify: `internal/core/parser/namespace.go` (refine `errorNodeSkip` to sync on top-level keywords)
- Test: `internal/core/parser/recovery_test.go`

The parser must ALWAYS produce a tree and never loop forever. Tasks 7-14 already guarantee progress (each body loop advances a token when a member parser consumes nothing) and insert missing `}`/`;`/`)` via `expect` (which records a diagnostic but does not consume, letting the enclosing loop terminate on the real delimiter). This task hardens `errorNodeSkip` to also stop at *top-level declaration keywords* (so one bad member does not swallow the next good declaration) and adds explicit recovery tests.

Sync set for skipping a bad member: `;`, `}`, EOF, or a token that begins a new declaration (`package`, `namespace`, `library`, `standard`, `dependency`, `comment`, `doc`, `rep`, `language`, `alias`, `import`, `public`, `private`, `protected`, `#`).

- [ ] **Step 1: Write the failing test**

```go
package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

func TestRecoverBadMemberThenGood(t *testing.T) {
	// `part def X;` is out of scope → ErrorNode, but the following package
	// must still parse.
	p := newParser("part def X; package P;")
	root := p.ParseFile()
	if len(root.Members) != 2 {
		t.Fatalf("members = %d: %+v", len(root.Members), root.Members)
	}
	if _, ok := root.Members[0].(*ast.ErrorNode); !ok {
		t.Fatalf("member0 = %T", root.Members[0])
	}
	if _, ok := root.Members[1].(*ast.Membership); !ok {
		t.Fatalf("member1 = %T", root.Members[1])
	}
}

func TestRecoverBadMemberNoSemicolon(t *testing.T) {
	// No `;` after the bad member; recovery syncs on the `package` keyword.
	p := newParser("garble package P;")
	root := p.ParseFile()
	if len(root.Members) != 2 {
		t.Fatalf("members = %d: %+v", len(root.Members), root.Members)
	}
	if _, ok := root.Members[1].(*ast.Membership); !ok {
		t.Fatalf("member1 = %T", root.Members[1])
	}
}

func TestRecoverMissingCloseBrace(t *testing.T) {
	// Unterminated body still yields a package with a diagnostic.
	p := newParser("package P { package Q;")
	root := p.ParseFile()
	if len(root.Members) != 1 {
		t.Fatalf("members = %+v", root.Members)
	}
	if len(p.Diagnostics) == 0 {
		t.Fatal("expected a diagnostic for missing '}'")
	}
}

func TestRecoverAlwaysTerminates(t *testing.T) {
	// Pathological input must not hang and must produce a root.
	p := newParser("} ] ) :: :: ;;; @@@ package X;")
	root := p.ParseFile()
	if root == nil {
		t.Fatal("nil root")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/parser/ -run 'TestRecover' -v`
Expected: FAIL — `TestRecoverBadMemberNoSemicolon` fails because the current `errorNodeSkip` skips past `package` (it only stops at `;`/`}`), so the good package is swallowed. (`TestRecoverAlwaysTerminates` should already pass thanks to progress guards.)

- [ ] **Step 3: Write minimal implementation**

Replace `errorNodeSkip` in `namespace.go`:

```go
// declStartKeywords are keywords that begin a namespace member. Recovery
// stops before these so one bad member does not consume the next good one.
var declStartKeywords = map[string]bool{
	"package": true, "namespace": true, "library": true, "standard": true,
	"dependency": true, "comment": true, "doc": true, "rep": true,
	"language": true, "alias": true, "import": true,
	"public": true, "private": true, "protected": true,
}

// atMemberSync reports whether the current token is a recovery sync point:
// a member terminator/closer, EOF, a declaration-start keyword, or `#`.
func (p *Parser) atMemberSync() bool {
	if p.atEOF() || p.at(lexer.Semicolon) || p.at(lexer.RBrace) || p.at(lexer.Hash) {
		return true
	}
	t := p.peek()
	return t.Kind == lexer.Keyword && declStartKeywords[t.KeywordID]
}

// errorNodeSkip builds an ErrorNode and skips tokens to the next sync point.
// It consumes a trailing `;` (member terminator) but leaves `}` and
// declaration-start keywords for the enclosing loop.
func (p *Parser) errorNodeSkip(start int, msg string) *ast.ErrorNode {
	p.error(p.peek().Span, msg)
	// Always consume at least one token to guarantee progress unless we are
	// already at a hard closer/EOF.
	if !p.atEOF() && !p.at(lexer.Semicolon) && !p.at(lexer.RBrace) {
		p.advance()
	}
	for !p.atMemberSync() {
		p.advance()
	}
	p.accept2(lexer.Semicolon) // consume terminator if present
	en := &ast.ErrorNode{Message: msg}
	en.NodeSpan = p.spanFrom(start)
	return en
}
```

Note: the initial forced `advance()` ensures progress even when the bad token is itself a declaration-start keyword reached in an unexpected position; the subsequent loop then stops at the next sync point. `TestRecoverBadMemberNoSemicolon` now stops before `package`, so the good package parses.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/parser/ -run 'TestRecover' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/parser/namespace.go internal/core/parser/recovery_test.go
git commit -m "feat(parser): sync recovery on declaration keywords; harden error skipping"
```

### Task 16: Integration — parse fixtures + golden AST dumps

**Files:**
- Modify: `internal/core/ast/dump.go` (add namespace-node dump cases)
- Create: `internal/core/parser/integration_test.go`
- Create: `testdata/parse/namespaces.sysml`, `testdata/parse/namespaces.golden`
- Create: `testdata/parse/expressions.sysml`, `testdata/parse/expressions.golden`

Two things happen here. First, `Dump` gains cases for the namespace-level nodes (Package/Namespace/Import/Membership/RootNamespace/Alias/Dependency/Comment/Documentation/TextualRepresentation/PrefixMetadata/QualifiedName) so whole-file trees are diffable. Second, golden-file integration tests parse real fixtures and compare against a committed `.golden` dump, with a `-update` flag to regenerate.

- [ ] **Step 1: Write the failing test**

Create `internal/core/parser/integration_test.go`:

```go
package parser

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

var update = flag.Bool("update", false, "update golden files")

func runGolden(t *testing.T, name string) {
	t.Helper()
	srcPath := filepath.Join("..", "..", "..", "testdata", "parse", name+".sysml")
	goldenPath := filepath.Join("..", "..", "..", "testdata", "parse", name+".golden")

	data, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("read %s: %v", srcPath, err)
	}
	p := New(source.New(name+".sysml", data))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics parsing %s: %+v", name, p.Diagnostics)
	}
	got := strings.TrimSpace(ast.Dump(root)) + "\n"

	if *update {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update)", goldenPath, err)
	}
	if got != string(want) {
		t.Fatalf("golden mismatch for %s:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestGoldenNamespaces(t *testing.T)  { runGolden(t, "namespaces") }
func TestGoldenExpressions(t *testing.T) { runGolden(t, "expressions") }
```

Create `testdata/parse/namespaces.sysml`:

```
package P {
	public import A::B::*;
	private import C::D;
	import E::**;
	namespace N {
		alias X for P::Q;
	}
	dependency from P to Q;
	comment /* a note */
	doc /* the docs */
}
```

Create `testdata/parse/expressions.sysml` (a `filter` member takes an expression, exercising the expression parser inside a namespace body):

```
package E {
	filter 1 + 2 * 3;
	filter a.b.c;
	filter coll->select(x);
	filter if c ? a else b;
	filter x as Integer;
}
```

Note: `filter` members are parsed by `parseMember` via a `filter` case that reads `filter OwnedExpression ;`. If Task 7's dispatch does not yet include `filter`, add it now (see Step 3).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/parser/ -run 'TestGolden' -v`
Expected: FAIL — golden files do not exist yet and `Dump` emits `(%T)` for namespace nodes.

- [ ] **Step 3: Write minimal implementation**

First add the `filter` member. In `namespace.go`, add to `parseDeclaration`'s keyword switch a case for `filter` (before the `default`):

```go
	case p.atKeyword("filter"):
		return p.parseFilter(start)
```

And add:

```go
// parseFilter parses `filter OwnedExpression ;` (ElementFilterMember).
func (p *Parser) parseFilter(start int) ast.Node {
	p.advance() // filter
	expr := p.ParseExpression()
	p.expect(lexer.Semicolon, "expected ';' after filter expression")
	f := &ast.FilterMember{Condition: expr}
	f.NodeSpan = p.spanFrom(start)
	return f
}
```

Add the `FilterMember` node to `internal/core/ast/namespace.go`:

```go
// FilterMember is an `filter <expression> ;` element filter.
type FilterMember struct {
	NodeBase
	Condition Node
}
```

Now add namespace-node cases to `dumpNode` in `dump.go` (before the `default`):

```go
	case *RootNamespace:
		b.WriteString(`(RootNamespace`)
		writeChildren(b, depth, v.Members)
		return
	case *Membership:
		fmt.Fprintf(b, `(Membership visibility=%q`, visibilityString(v.Visibility))
		writeChildren(b, depth, []Node{v.Member})
		return
	case *Package:
		fmt.Fprintf(b, `(Package name=%q library=%t standard=%t`, identName(v.Ident), v.IsLibrary, v.IsStandard)
		writeChildren(b, depth, prefixesAnd(v.Prefixes, v.Members))
		return
	case *Namespace:
		fmt.Fprintf(b, `(Namespace name=%q`, identName(v.Ident))
		writeChildren(b, depth, prefixesAnd(v.Prefixes, v.Members))
		return
	case *Import:
		fmt.Fprintf(b, `(Import visibility=%q all=%t kind=%s recursive=%t imported=%q`,
			visibilityString(v.Visibility), v.IsAll, importKindString(v.Kind), v.IsRecursive, qnString(v.Imported))
		writeChildren(b, depth, v.Body)
		return
	case *Alias:
		fmt.Fprintf(b, `(Alias visibility=%q name=%q for=%q`,
			visibilityString(v.Visibility), identName(v.Ident), qnString(v.For))
		writeChildren(b, depth, v.Body)
		return
	case *Dependency:
		fmt.Fprintf(b, `(Dependency clients=%q suppliers=%q`, qnList(v.Clients), qnList(v.Suppliers))
		writeChildren(b, depth, prefixesAnd(v.Prefixes, v.Body))
		return
	case *Comment:
		fmt.Fprintf(b, `(Comment about=%q locale=%q)`, qnList(v.About), v.Locale)
	case *Documentation:
		fmt.Fprintf(b, `(Documentation locale=%q)`, v.Locale)
	case *TextualRepresentation:
		fmt.Fprintf(b, `(TextualRepresentation language=%q)`, v.Language)
	case *PrefixMetadata:
		fmt.Fprintf(b, `(PrefixMetadata type=%q)`, qnString(v.Type))
	case *FilterMember:
		b.WriteString(`(FilterMember`)
		writeChildren(b, depth, []Node{v.Condition})
		return
```

Add these helpers to `dump.go`:

```go
func visibilityString(v Visibility) string {
	switch v {
	case VisibilityPublic:
		return "public"
	case VisibilityPrivate:
		return "private"
	case VisibilityProtected:
		return "protected"
	default:
		return "default"
	}
}

func importKindString(k ImportKind) string {
	if k == ImportNamespace {
		return "namespace"
	}
	return "membership"
}

func identName(id Identification) string {
	if id.ShortName != "" && id.Name != "" {
		return "<" + id.ShortName + "> " + id.Name
	}
	if id.ShortName != "" {
		return "<" + id.ShortName + ">"
	}
	return id.Name
}

func qnList(qns []*QualifiedName) string {
	parts := make([]string, len(qns))
	for i, qn := range qns {
		parts[i] = qnString(qn)
	}
	return strings.Join(parts, ", ")
}

// prefixesAnd renders prefix-metadata nodes (as children) followed by members.
func prefixesAnd(prefixes []*PrefixMetadata, members []Node) []Node {
	kids := make([]Node, 0, len(prefixes)+len(members))
	for _, pm := range prefixes {
		kids = append(kids, pm)
	}
	kids = append(kids, members...)
	return kids
}
```

- [ ] **Step 4: Generate goldens, then verify**

Run: `go test ./internal/core/parser/ -run 'TestGolden' -update`
Then inspect the generated `testdata/parse/*.golden` files to confirm they are sensible (no `(%T)` lines, no ErrorNodes, structure matches the source). Then run without `-update`:

Run: `go test ./... && go vet ./...`
Expected: PASS, vet clean. Also confirm the Plan 1 lexer fixtures still tokenize (unchanged) and that `testdata/parse` goldens contain no `ErrorNode` or `(*ast.` default lines.

- [ ] **Step 5: Commit**

```bash
git add internal/core/ast/dump.go internal/core/ast/namespace.go internal/core/parser/namespace.go internal/core/parser/integration_test.go testdata/parse/
git commit -m "feat(ast,parser): dump namespace nodes; add golden parse integration tests"
```

---

## Self-Review

**Spec coverage (Plan 2 scope = AST framework + parser infra + full KerMLExpressions + namespace core):**

| Scope item | Task(s) |
|---|---|
| AST node framework (Node, NodeBase, spans, trivia, ErrorNode) | 1 |
| Namespace-core AST nodes (QualifiedName, Identification, Membership, RootNamespace, Package, Namespace, Import, Alias, Dependency, Comment, Documentation, TextualRepresentation, PrefixMetadata) | 2 |
| Expression AST nodes (literals, refs, OperatorExpr ladder, chain/index/invoke/collect/select, constructor, body, sequence, metadata-access) | 3 |
| S-expression dumper (golden support) | 4, 16 |
| Parser driver (token buffer, trivia, lookahead, diagnostics, expect/accept helpers) | 5 |
| QualifiedName + Identification parsing | 6 |
| ParseFile + member dispatch + namespace body loop + RelationshipBody | 7 |
| Package / Namespace / prefix-metadata declarations | 8 |
| Import (membership, namespace `::*`, recursive `::**`) | 9 |
| Alias + Dependency | 10 |
| Comment / Documentation / TextualRepresentation (RegularComment body via pending-comment slot) | 11 |
| Expression: base/primary (literals, refs, paren/sequence, body, constructor) | 12 |
| Expression: full operator precedence ladder (binary, unary, conditional, classification) | 13 |
| Expression: postfix chain (`.`, `#()`, `[]`, `->`, `.{}`, `.?{}`) | 14 |
| Error recovery (panic-mode sync on decl keywords, missing-token via `expect`, always-terminates) | 15 |
| filter member + golden integration fixtures | 16 |

Out-of-scope items (deferred to a later "definitions" plan, per the scoping decision) and correctly NOT implemented here: the SysML/KerML definition & usage taxonomy (Type/Classifier/Class/Structure/Metaclass/DataType/Association/Behavior/Function/Predicate/Feature/Step/Connector/BindingConnector/Succession/Flow, all `*Definition`/`*Usage` — part/attribute/port/action/state/connection/constraint/requirement), specialization/redefinition/subsetting, multiplicity, feature typing/values. Encountering such a keyword mid-body yields a diagnostic + `ErrorNode` spanning the member (Task 15 recovery), NOT an abort. Name resolution, typing, and validation are Plans 3+.

**Placeholder scan:** No `<FILL>`, `TBD`, or `TODO` remain in task bodies; every code step contains complete code, and every run step names an exact command with expected result.

**Type consistency across tasks:**
- `Parser` methods used by expression tasks (`peek`, `peekN`, `advance`, `at`, `atKeyword`, `accept`, `acceptKeyword`, `expect`, `error`, `spanFrom`, `takeTrivia`, `atName`, `parseQualifiedName`, `parseNameSegment`) are all defined in Tasks 5-6 with matching signatures.
- `parseArgList()` returns `([]ast.Node, []ast.NamedArg)` in Task 12 and is consumed with that shape in Task 14 (`inv.Args, inv.NamedArgs = p.parseArgList()`).
- `parseBodyExpr(start int)` signature is consistent between Task 12 (definition) and Task 14 (calls pass `p.peek().Span.Offset`).
- `errorNodeSkip(start int, msg string) *ast.ErrorNode` — Task 7 introduces it (as a stub-friendly helper) and Task 15 refines the body without changing the signature; callers unaffected.
- `parseInvocationTail`/`namedArgAhead` defined in Task 12; `parseInvocationTail` unused by Task 14 (which builds `InvocationExpr` inline for the `->` form and reuses `parseArgList`). To avoid a dead-code/vet issue, Task 12's `parseBase` uses `parseInvocationTail` for the bare `Type(args)` form, so it IS referenced — consistent.
- AST field names referenced by the dumper (Task 4/16) match the struct definitions (Tasks 2-3): `Package.IsLibrary/IsStandard/Prefixes/Members/Ident`, `Import.IsAll/Kind/IsRecursive/Imported/Body/Visibility`, `OperatorExpr.Operator/Operands/TypeRef`, etc.
- `FilterMember` (Task 16) is added to `ast/namespace.go` and dumped in `dump.go`; `parseFilter` (Task 16) references it. Consistent.

**Ordering note for the implementer:** Task 13 rewrites `ParseExpression` (from the Task 12 delegate) and Task 14 rewrites `parsePrimary`. Apply tasks in order; each task's tests include the prior tasks' run filters to catch regressions. Golden files (Task 16) are generated with `-update` then committed.
