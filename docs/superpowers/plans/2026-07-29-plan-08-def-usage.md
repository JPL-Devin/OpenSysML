# Def/Usage Taxonomy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a full vertical slice of SysML v2 definition/usage taxonomy — `part`/`attribute` `def` and usage declarations with typing, specialization, subsetting, redefinition, references, crosses relationships, feature modifiers, multiplicity, feature values, and nested bodies — end-to-end from lexer through a new type-compatibility pass.

**Architecture:** Follows the existing hand-written recursive-descent pipeline. Two new discriminated AST nodes (`Definition{Kind}`, `Usage{Kind}`) plus `Relationship` and `Multiplicity` (spec Option A — mirrors the KerML metamodel, minimizes switch churn across the four manual traversals). The lexer gains four maximal-munch compound operator tokens (`:>`, `:>>`, `::>`, `=>`). The parser dispatches def/usage from `parseDeclaration`. Symbols/resolve gain cases in their existing `buildDecl`/`resolveDecl` switches. A new `TypeCheckPass` inhabits the previously-empty `LevelType` tier and validates relationship-target kind compatibility.

**Tech Stack:** Go 1.25; module `github.com/Open-MBEE/Systemica`. No new external dependencies. Reference grammar: OMG Pilot `SysML.xtext` / `KerML.xtext` (gitignored clone, read-only).

---

## Scope

**In scope (vertical slice):**
- Definition kinds: `part def`, `attribute def`. Usage kinds: `part`, `attribute`.
- Relationships (keyword + symbolic): typing (`:` / `defined by`), specializes (`:>` / `specializes`), subsets (`:>` / `subsets`), redefines (`:>>` / `redefines`), references (`::>` / `references`), crosses (`=>` / `crosses`). Multiple comma-separated targets per clause.
- Def modifiers: `abstract`, `variation`. Usage modifiers: `ref`, `in`/`out`/`inout`, `composite`/`portion`, `derived`, `ordered`, `nonunique`.
- Multiplicity `[n]` / `[lo..hi]` / `[*]` on usages (reuses expression parser for bounds).
- Feature value `= expr` on usages (reuses expression parser, resolved).
- Full recursive nested bodies; body members carry optional visibility (uniform with top level).
- Symbols for the four kinds; name resolution of all relationship targets, multiplicity bounds, and values; `libs` `Supers` populated from specialization-edge raw text.
- New `LevelType` type-compatibility pass (first pass in that tier).

**Deferred (out of scope):**
- The other ~13 def/usage kinds (`item`, `port`, `connection`, `action`, `state`, `constraint`, `requirement`, etc.) — mechanical repeats of this slice, added later.
- Transitive specialization checks, multiplicity-bound value validation, abstract-instantiation checks (`LevelConstraint` tier stays empty).
- Auto-loading the stdlib into the workspace (pre-existing gap, shared with LSP; unchanged here).

**Constraints discovered:**
- No AST visitor/Walk infrastructure. Traversal is a manual type-switch duplicated in FOUR places after this plan: `ast.Dump` (`dump.go:40`), `symbols.buildDecl` (`builder.go:47`), `resolve.resolveDecl` (`document.go:51`), and the new `passes/typecheck.go`. New nodes MUST be added to each or they silently no-op.
- All def/usage keywords already lex as `Kind==Keyword` with `KeywordID` set (no keyword-table change needed) — match via `atKeyword`.
- Symbolic `:> :>> ::> =>` are NOT single tokens today; the lexer must maximal-munch them from leading `:`/`::`/`=`. `..` already lexes as one `DotDot` token.
- `LevelType`/`LevelConstraint` tiers exist with zero passes; `registry.go` already gates higher tiers when a lower tier errored, so an unresolved target auto-skips the type pass — the type pass also nil-guards unresolved targets defensively.

## File Structure

**Create:**
- `internal/core/ast/defusage.go` — `Definition`, `Usage`, `Relationship`, `Multiplicity` node structs + the `DefinitionKind`/`UsageKind`/`RelationshipKind`/`FeatureDirection` enums.
- `internal/core/passes/typecheck.go` — `TypeCheckPass` at `LevelType`; relationship-target kind-compatibility validation.

**Modify:**
- `internal/core/ast/dump.go` — add `*Definition`/`*Usage`/`*Relationship`/`*Multiplicity` cases to `dumpNode`.
- `internal/core/lexer/token.go` — add `ColonGt`/`ColonGtGt`/`ColonColonGt`/`EqGt` `Kind` consts + `kindNames` entries.
- `internal/core/lexer/lexer.go` — maximal-munch the four compound operators in the `:` and `=` cases of `Next`.
- `internal/core/parser/namespace.go` — def/usage dispatch in `parseDeclaration`; `parseDefinition`/`parseUsage`/`parseFeatureModifiers`/`parseRelationships`/`parseMultiplicity`/`parseDefinitionBody`; extend `declStartKeywords`.
- `internal/core/symbols/symbol.go` — add `SymbolPartDef`/`SymbolAttributeDef`/`SymbolPartUsage`/`SymbolAttributeUsage` + names.
- `internal/core/symbols/builder.go` — `buildDecl` cases for `*ast.Definition`/`*ast.Usage`.
- `internal/core/resolve/document.go` — `resolveDecl` cases for `*ast.Definition`/`*ast.Usage`.
- `internal/core/libs/record.go` — populate `Supers` from specialization-edge raw text.
- `internal/core/passes/analyze.go` — register `TypeCheckPass` in `DefaultRegistry`.

**Tests:**
- `internal/core/lexer/lexer_test.go` — compound-operator maximal-munch cases.
- `internal/core/parser/defusage_test.go` — golden AST for def/usage declarations (new file).
- `internal/core/parser/recovery_test.go` + `namespace_test.go` — FLIP the two `part def`→ErrorNode fixtures to assert real `Definition`; add a separate malformed fixture that still proves recovery.
- `internal/core/symbols/*_test.go` — symbol kinds + nested scopes for def/usage.
- `internal/core/resolve/*_test.go` — relationship-target / value resolution.
- `internal/core/passes/typecheck_test.go` — kind-compat pass (new file).
- `internal/repl` or existing integration test — end-to-end `part def` smoke through the workspace.

---

## Task 1 — AST nodes + dump

**Files:**
- Create: `internal/core/ast/defusage.go`
- Modify: `internal/core/ast/dump.go:143` (add cases before `default`)
- Test: `internal/core/ast/defusage_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/core/ast/defusage_test.go`:

```go
package ast

import "testing"

func TestDumpDefinition(t *testing.T) {
	def := &Definition{
		Kind:       DefPart,
		IsAbstract: true,
		Ident:      Identification{Name: "Vehicle"},
		Relationships: []*Relationship{
			{Kind: RelSpecializes, Target: &QualifiedName{Parts: []NameSegment{{Text: "Base"}}}},
		},
		Members: []Node{
			&Usage{Kind: UsagePart, Ident: Identification{Name: "engine"}},
		},
		HasBody: true,
	}
	got := Dump(def)
	want := "(Definition kind=\"part\" abstract=true variation=false name=\"Vehicle\"\n" +
		"  (Relationship kind=\"specializes\" target=\"Base\")\n" +
		"  (Usage kind=\"part\" name=\"engine\" ref=false direction=\"none\" composite=false derived=false ordered=false nonunique=false))"
	if got != want {
		t.Fatalf("Dump mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestDumpUsageWithMultiplicityAndValue(t *testing.T) {
	u := &Usage{
		Kind:         UsageAttribute,
		Ident:        Identification{Name: "mass"},
		Relationships: []*Relationship{
			{Kind: RelTyping, Target: &QualifiedName{Parts: []NameSegment{{Text: "Real"}}}},
		},
		Multiplicity: &Multiplicity{Upper: &LiteralInteger{Value: "4"}},
		Value:        &LiteralInteger{Value: "42"},
	}
	got := Dump(u)
	want := "(Usage kind=\"attribute\" name=\"mass\" ref=false direction=\"none\" composite=false derived=false ordered=false nonunique=false\n" +
		"  (Relationship kind=\"typing\" target=\"Real\")\n" +
		"  (Multiplicity range=false\n" +
		"    (LiteralInteger value=\"4\"))\n" +
		"  (LiteralInteger value=\"42\"))"
	if got != want {
		t.Fatalf("Dump mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/ast/ -run TestDump -v`
Expected: FAIL — compile errors (`undefined: Definition`, `Usage`, `Relationship`, `Multiplicity`, `DefPart`, etc.).

- [ ] **Step 3: Create the AST node file**

Create `internal/core/ast/defusage.go`:

```go
package ast

// DefinitionKind discriminates the concrete definition taxonomy element.
type DefinitionKind int

const (
	DefPart DefinitionKind = iota
	DefAttribute
	// extensible: DefItem, DefPort, DefAction, ...
)

func (k DefinitionKind) String() string {
	switch k {
	case DefPart:
		return "part"
	case DefAttribute:
		return "attribute"
	default:
		return "unknown"
	}
}

// UsageKind discriminates the concrete usage taxonomy element.
type UsageKind int

const (
	UsagePart UsageKind = iota
	UsageAttribute
)

func (k UsageKind) String() string {
	switch k {
	case UsagePart:
		return "part"
	case UsageAttribute:
		return "attribute"
	default:
		return "unknown"
	}
}

// RelationshipKind discriminates a specialization/typing edge at a
// definition or usage declaration head.
type RelationshipKind int

const (
	RelTyping      RelationshipKind = iota // ':' / 'defined by'
	RelSpecializes                         // 'specializes' / ':>'
	RelSubsets                             // 'subsets' / ':>'
	RelRedefines                           // 'redefines' / ':>>'
	RelReferences                          // 'references' / '::>'
	RelCrosses                             // 'crosses' / '=>'
)

func (k RelationshipKind) String() string {
	switch k {
	case RelTyping:
		return "typing"
	case RelSpecializes:
		return "specializes"
	case RelSubsets:
		return "subsets"
	case RelRedefines:
		return "redefines"
	case RelReferences:
		return "references"
	case RelCrosses:
		return "crosses"
	default:
		return "unknown"
	}
}

// FeatureDirection is the in/out/inout direction modifier on a usage.
type FeatureDirection int

const (
	DirNone FeatureDirection = iota
	DirIn
	DirOut
	DirInOut
)

func (d FeatureDirection) String() string {
	switch d {
	case DirIn:
		return "in"
	case DirOut:
		return "out"
	case DirInOut:
		return "inout"
	default:
		return "none"
	}
}

// Relationship is one specialization/typing edge: exactly one Target. A
// clause with multiple targets (`specializes A, B`) produces multiple
// Relationship entries sharing the same Kind.
type Relationship struct {
	NodeBase
	Kind   RelationshipKind
	Target *QualifiedName
}

// Multiplicity is a `[n]` / `[lo..hi]` / `[*]` bound on a usage. Bounds are
// expression Nodes (reusing the expression AST); `*` becomes LiteralInfinity.
// For the single-bound form `[n]`, Upper holds n and IsRange is false.
type Multiplicity struct {
	NodeBase
	Lower   Node
	Upper   Node
	IsRange bool
}

// Definition is a `part def` / `attribute def` (and future kinds) node.
type Definition struct {
	NodeBase
	Prefixes      []*PrefixMetadata
	Kind          DefinitionKind
	IsAbstract    bool
	IsVariation   bool
	Ident         Identification
	Relationships []*Relationship
	Members       []Node
	HasBody       bool
}

// Usage is a `part` / `attribute` usage node.
type Usage struct {
	NodeBase
	Prefixes      []*PrefixMetadata
	Kind          UsageKind
	IsAbstract    bool
	IsReference   bool
	Direction     FeatureDirection
	IsComposite   bool
	IsDerived     bool
	IsOrdered     bool
	IsNonunique   bool
	Ident         Identification
	Relationships []*Relationship
	Multiplicity  *Multiplicity
	Value         Node
	Members       []Node
	HasBody       bool
}
```

- [ ] **Step 4: Add dump cases**

In `internal/core/ast/dump.go`, insert these cases immediately before the `default:` case at line 143:

```go
	case *Definition:
		fmt.Fprintf(b, `(Definition kind=%q abstract=%t variation=%t name=%q`,
			v.Kind.String(), v.IsAbstract, v.IsVariation, identName(v.Ident))
		writeChildren(b, depth, defusageChildren(v.Prefixes, v.Relationships, nil, nil, v.Members))
		return
	case *Usage:
		fmt.Fprintf(b, `(Usage kind=%q name=%q ref=%t direction=%q composite=%t derived=%t ordered=%t nonunique=%t`,
			v.Kind.String(), identName(v.Ident), v.IsReference, v.Direction.String(),
			v.IsComposite, v.IsDerived, v.IsOrdered, v.IsNonunique)
		writeChildren(b, depth, defusageChildren(v.Prefixes, v.Relationships, v.Multiplicity, v.Value, v.Members))
		return
	case *Relationship:
		fmt.Fprintf(b, `(Relationship kind=%q target=%q)`, v.Kind.String(), qnString(v.Target))
	case *Multiplicity:
		fmt.Fprintf(b, `(Multiplicity range=%t`, v.IsRange)
		var kids []Node
		if v.Lower != nil {
			kids = append(kids, v.Lower)
		}
		if v.Upper != nil {
			kids = append(kids, v.Upper)
		}
		writeChildren(b, depth, kids)
		return
```

Then add this helper at the end of `dump.go` (after `prefixesAnd`):

```go
// defusageChildren flattens the ordered child set for a Definition/Usage
// dump: prefixes, relationships, optional multiplicity, optional value,
// then members. nil multiplicity/value are omitted.
func defusageChildren(prefixes []*PrefixMetadata, rels []*Relationship, mult *Multiplicity, value Node, members []Node) []Node {
	kids := make([]Node, 0, len(prefixes)+len(rels)+2+len(members))
	for _, pm := range prefixes {
		kids = append(kids, pm)
	}
	for _, r := range rels {
		kids = append(kids, r)
	}
	if mult != nil {
		kids = append(kids, mult)
	}
	if value != nil {
		kids = append(kids, value)
	}
	kids = append(kids, members...)
	return kids
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/core/ast/ -run TestDump -v`
Expected: PASS (both `TestDumpDefinition` and `TestDumpUsageWithMultiplicityAndValue`).

- [ ] **Step 6: gofmt, vet, commit**

```bash
gofmt -w internal/core/ast/defusage.go internal/core/ast/dump.go internal/core/ast/defusage_test.go
go vet ./internal/core/ast/
git add internal/core/ast/defusage.go internal/core/ast/dump.go internal/core/ast/defusage_test.go
git commit -m "feat(ast): add Definition/Usage/Relationship/Multiplicity nodes + dump"
```

## Task 2 — Lexer compound relationship operators

**Files:**
- Modify: `internal/core/lexer/token.go` (Kind enum ~line 65, `kindNames` ~line 85)
- Modify: `internal/core/lexer/lexer.go` (`:` case lines 49-55, `=` case lines 81-91)
- Test: `internal/core/lexer/lexer_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/core/lexer/lexer_test.go`:

```go
func TestCompoundRelationshipOperators(t *testing.T) {
	cases := []struct {
		src  string
		want []Kind
	}{
		{":>", []Kind{ColonGt, EOF}},
		{":>>", []Kind{ColonGtGt, EOF}},
		{"::>", []Kind{ColonColonGt, EOF}},
		{"=>", []Kind{EqGt, EOF}},
		// disambiguation: bare forms still work
		{":", []Kind{Colon, EOF}},
		{"::", []Kind{ColonColon, EOF}},
		{"=", []Kind{Eq, EOF}},
		{">", []Kind{Gt, EOF}},
		// short-name close '>' after a name is not swallowed
		{"<x>", []Kind{Lt, Identifier, Gt, EOF}},
		// '::' then '>' with no compound: '::>' IS the compound, but ':> >' stays split
		{":> >", []Kind{ColonGt, Gt, EOF}},
		// '=' value vs '=>' crosses
		{"= 1", []Kind{Eq, Decimal, EOF}},
	}
	for _, tc := range cases {
		lx := New(source.New("<t>", []byte(tc.src)))
		var got []Kind
		for {
			tok := lx.Next()
			if tok.IsTrivia() {
				continue
			}
			got = append(got, tok.Kind)
			if tok.Kind == EOF {
				break
			}
		}
		if len(got) != len(tc.want) {
			t.Fatalf("%q: got %v want %v", tc.src, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("%q: token %d got %v want %v", tc.src, i, got[i], tc.want[i])
			}
		}
	}
}
```

Note: this test uses `New`, `source.New`, `Token.IsTrivia`, and `Kind` — all already available in the package/test file. If `source` is not already imported in `lexer_test.go`, add `"github.com/Open-MBEE/Systemica/internal/core/source"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/lexer/ -run TestCompoundRelationshipOperators -v`
Expected: FAIL — compile errors (`undefined: ColonGt`, `ColonGtGt`, `ColonColonGt`, `EqGt`).

- [ ] **Step 3: Add the Kind constants**

In `internal/core/lexer/token.go`, add four constants to the punctuation/operators block (immediately before `Colon` at line 65 is fine; placement does not matter functionally):

```go
	Colon       // :
	ColonGt     // :>
	ColonGtGt   // :>>
	ColonColonGt // ::>
	EqGt        // =>
```

And add their names to the `kindNames` map (after the `Colon: ":"` entry on line 85):

```go
	Colon: ":", ColonGt: ":>", ColonGtGt: ":>>", ColonColonGt: "::>", EqGt: "=>",
```

- [ ] **Step 4: Maximal-munch in the scanner**

In `internal/core/lexer/lexer.go`, replace the `:` case (lines 49-55) with:

```go
	case ':':
		if lx.peek(1) == ':' {
			if lx.peek(2) == '>' {
				lx.pos += 3
				return Token{Kind: ColonColonGt, Span: lx.span(start)} // ::>
			}
			lx.pos += 2
			return Token{Kind: ColonColon, Span: lx.span(start)} // ::
		}
		if lx.peek(1) == '>' {
			if lx.peek(2) == '>' {
				lx.pos += 3
				return Token{Kind: ColonGtGt, Span: lx.span(start)} // :>>
			}
			lx.pos += 2
			return Token{Kind: ColonGt, Span: lx.span(start)} // :>
		}
		lx.pos++
		return Token{Kind: Colon, Span: lx.span(start)}
```

And replace the `=` case (lines 81-91) with (add the `=>` branch AFTER the `===`/`==` branches so those keep priority):

```go
	case '=':
		if lx.peek(1) == '=' && lx.peek(2) == '=' {
			lx.pos += 3
			return Token{Kind: EqEqEq, Span: lx.span(start)}
		}
		if lx.peek(1) == '=' {
			lx.pos += 2
			return Token{Kind: EqEq, Span: lx.span(start)}
		}
		if lx.peek(1) == '>' {
			lx.pos += 2
			return Token{Kind: EqGt, Span: lx.span(start)} // =>
		}
		lx.pos++
		return Token{Kind: Eq, Span: lx.span(start)}
```

The bare `>` case (lines 111-117) is left UNCHANGED — munching only starts from a leading `:`/`=`, never from a bare `>`, so a short-name close `>` is never swallowed.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/core/lexer/ -run TestCompoundRelationshipOperators -v`
Expected: PASS. Then run the whole lexer suite to confirm no regression: `go test ./internal/core/lexer/`
Expected: PASS.

- [ ] **Step 6: gofmt, vet, commit**

```bash
gofmt -w internal/core/lexer/token.go internal/core/lexer/lexer.go internal/core/lexer/lexer_test.go
go vet ./internal/core/lexer/
git add internal/core/lexer/token.go internal/core/lexer/lexer.go internal/core/lexer/lexer_test.go
git commit -m "feat(lexer): maximal-munch :> :>> ::> => relationship operators"
```

## Task 3 — Parser: modifiers + def/usage dispatch

This task adds the dispatch skeleton: recognizing a def/usage declaration, consuming feature modifiers, distinguishing `def` from usage, and producing bare `Definition`/`Usage` nodes with just prefixes/modifiers/kind/ident and a `;`-or-body tail. Relationships, multiplicity, and value come in Task 4; nested bodies in Task 5. Until then bodies are parsed with the existing `parseNamespaceBody` (namespace members only) as a placeholder, replaced in Task 5.

**Files:**
- Create: `internal/core/parser/defusage.go`
- Modify: `internal/core/parser/namespace.go:142` (dispatch), `:204` (`declStartKeywords`)
- Test: `internal/core/parser/defusage_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/core/parser/defusage_test.go`:

```go
package parser

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// parseOneMember parses src and returns the single unwrapped top-level member.
func parseOneMember(t *testing.T, src string) ast.Node {
	t.Helper()
	p := New(source.New("<t>", []byte(src)))
	root := p.ParseFile()
	if len(root.Members) != 1 {
		t.Fatalf("%q: expected 1 member, got %d", src, len(root.Members))
	}
	m := root.Members[0]
	if mem, ok := m.(*ast.Membership); ok {
		return mem.Member
	}
	return m
}

func TestParseDefinitionDispatch(t *testing.T) {
	def, ok := parseOneMember(t, "part def Vehicle;").(*ast.Definition)
	if !ok {
		t.Fatalf("expected *ast.Definition")
	}
	if def.Kind != ast.DefPart || def.Ident.Name != "Vehicle" {
		t.Fatalf("got kind=%v name=%q", def.Kind, def.Ident.Name)
	}
	if def.HasBody {
		t.Fatalf("expected no body")
	}
}

func TestParseAttributeDefAndModifiers(t *testing.T) {
	def := parseOneMember(t, "abstract variation attribute def Mass;").(*ast.Definition)
	if def.Kind != ast.DefAttribute || !def.IsAbstract || !def.IsVariation {
		t.Fatalf("got kind=%v abstract=%v variation=%v", def.Kind, def.IsAbstract, def.IsVariation)
	}
}

func TestParseUsageDispatch(t *testing.T) {
	u, ok := parseOneMember(t, "part engine;").(*ast.Usage)
	if !ok {
		t.Fatalf("expected *ast.Usage")
	}
	if u.Kind != ast.UsagePart || u.Ident.Name != "engine" {
		t.Fatalf("got kind=%v name=%q", u.Kind, u.Ident.Name)
	}
}

func TestParseUsageModifiers(t *testing.T) {
	u := parseOneMember(t, "ref in composite derived ordered nonunique part p;").(*ast.Usage)
	if !u.IsReference || u.Direction != ast.DirIn || !u.IsComposite || !u.IsDerived || !u.IsOrdered || !u.IsNonunique {
		t.Fatalf("modifier flags wrong: %+v", u)
	}
	if u.Kind != ast.UsagePart {
		t.Fatalf("got kind=%v", u.Kind)
	}
}

func TestParseAnonymousUsage(t *testing.T) {
	u := parseOneMember(t, "attribute;").(*ast.Usage)
	if u.Kind != ast.UsageAttribute || u.Ident.Name != "" {
		t.Fatalf("got kind=%v name=%q", u.Kind, u.Ident.Name)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/parser/ -run 'TestParse(Definition|AttributeDef|Usage|Anonymous)' -v`
Expected: FAIL — top-level `part def Vehicle;` currently produces an `*ast.ErrorNode` (dispatch falls through), so the type assertion to `*ast.Definition` fails.

- [ ] **Step 3: Create the def/usage parser file (dispatch + modifiers)**

Create `internal/core/parser/defusage.go`:

```go
package parser

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
)

// definitionKindKeywords maps a kind keyword to its DefinitionKind.
var definitionKindKeywords = map[string]ast.DefinitionKind{
	"part":      ast.DefPart,
	"attribute": ast.DefAttribute,
}

// usageKindKeywords maps a kind keyword to its UsageKind.
var usageKindKeywords = map[string]ast.UsageKind{
	"part":      ast.UsagePart,
	"attribute": ast.UsageAttribute,
}

// featureModifierKeywords are modifier keywords that may precede a def/usage
// kind keyword.
var featureModifierKeywords = map[string]bool{
	"abstract":  true,
	"variation": true,
	"ref":       true,
	"in":        true,
	"out":       true,
	"inout":     true,
	"composite": true,
	"portion":   true,
	"derived":   true,
	"ordered":   true,
	"nonunique": true,
}

// featureMods collects the modifier flags gathered before the kind keyword.
type featureMods struct {
	isAbstract  bool
	isVariation bool
	isReference bool
	direction   ast.FeatureDirection
	isComposite bool
	isDerived   bool
	isOrdered   bool
	isNonunique bool
}

// atDefUsageStart reports whether the current token begins a def/usage
// declaration: a feature-modifier keyword or a kind keyword.
func (p *Parser) atDefUsageStart() bool {
	t := p.peek()
	if t.Kind != lexer.Keyword {
		return false
	}
	if featureModifierKeywords[t.KeywordID] {
		return true
	}
	_, isDef := definitionKindKeywords[t.KeywordID]
	return isDef
}

// parseFeatureModifiers consumes leading modifier keywords into a featureMods.
func (p *Parser) parseFeatureModifiers() featureMods {
	var m featureMods
	for {
		t := p.peek()
		if t.Kind != lexer.Keyword {
			return m
		}
		switch t.KeywordID {
		case "abstract":
			m.isAbstract = true
		case "variation":
			m.isVariation = true
		case "ref":
			m.isReference = true
		case "in":
			m.direction = ast.DirIn
		case "out":
			m.direction = ast.DirOut
		case "inout":
			m.direction = ast.DirInOut
		case "composite", "portion":
			m.isComposite = true
		case "derived":
			m.isDerived = true
		case "ordered":
			m.isOrdered = true
		case "nonunique":
			m.isNonunique = true
		default:
			return m
		}
		p.advance()
	}
}

// parseDefUsage parses a definition or usage declaration. The caller has
// already established (via atDefUsageStart) that a def/usage begins here.
func (p *Parser) parseDefUsage(start int) ast.Node {
	mods := p.parseFeatureModifiers()

	// After modifiers, the current token must be a kind keyword.
	t := p.peek()
	kw := ""
	if t.Kind == lexer.Keyword {
		kw = t.KeywordID
	}
	if _, ok := definitionKindKeywords[kw]; !ok {
		// Modifiers not followed by a kind keyword: not a valid def/usage.
		return nil
	}
	p.advance() // consume the kind keyword

	// `def` after the kind keyword ⇒ definition; otherwise usage.
	if p.atKeyword("def") {
		p.advance() // consume 'def'
		return p.parseDefinition(start, definitionKindKeywords[kw], mods)
	}
	return p.parseUsage(start, usageKindKeywords[kw], mods)
}

// parseDefinition parses the tail of a definition declaration (after the kind
// keyword and 'def' are consumed). Relationships/bodies land in later tasks.
func (p *Parser) parseDefinition(start int, kind ast.DefinitionKind, mods featureMods) *ast.Definition {
	def := &ast.Definition{
		Kind:        kind,
		IsAbstract:  mods.isAbstract,
		IsVariation: mods.isVariation,
		Ident:       p.parseIdentification(),
	}
	members, hasBody := p.parseNamespaceBody() // placeholder; Task 5 replaces
	def.Members = members
	def.HasBody = hasBody
	def.NodeSpan = p.spanFrom(start)
	return def
}

// parseUsage parses the tail of a usage declaration (after the kind keyword).
// Relationships/multiplicity/value/bodies land in later tasks.
func (p *Parser) parseUsage(start int, kind ast.UsageKind, mods featureMods) *ast.Usage {
	u := &ast.Usage{
		Kind:        kind,
		IsAbstract:  mods.isAbstract,
		IsReference: mods.isReference,
		Direction:   mods.direction,
		IsComposite: mods.isComposite,
		IsDerived:   mods.isDerived,
		IsOrdered:   mods.isOrdered,
		IsNonunique: mods.isNonunique,
		Ident:       p.parseIdentification(),
	}
	members, hasBody := p.parseNamespaceBody() // placeholder; Task 5 replaces
	u.Members = members
	u.HasBody = hasBody
	u.NodeSpan = p.spanFrom(start)
	return u
}
```

- [ ] **Step 4: Wire dispatch into parseDeclaration**

In `internal/core/parser/namespace.go`, add a case to the `parseDeclaration` switch (line 142) BEFORE the `default:` case. Place it after the `filter` case (line 156) and before the `Hash` case:

```go
	case p.atDefUsageStart():
		return p.parseDefUsage(start)
```

- [ ] **Step 5: Extend declStartKeywords**

In `internal/core/parser/namespace.go`, add these entries to the `declStartKeywords` map (line 204) so malformed members resync past def/usage starts:

```go
	"part":      true,
	"attribute": true,
	"def":       true,
	"abstract":  true,
	"variation": true,
	"ref":       true,
	"in":        true,
	"out":       true,
	"inout":     true,
	"composite": true,
	"portion":   true,
	"derived":   true,
	"ordered":   true,
	"nonunique": true,
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/core/parser/ -run 'TestParse(Definition|AttributeDef|Usage|Anonymous)' -v`
Expected: PASS (all five). Note: the two existing `part def`→ErrorNode fixtures (`recovery_test.go`, `namespace_test.go`) will now FAIL — that is expected and they are flipped in Task 5. Do NOT run the full parser suite green-gate until Task 5.

- [ ] **Step 7: gofmt, vet, commit**

```bash
gofmt -w internal/core/parser/defusage.go internal/core/parser/namespace.go internal/core/parser/defusage_test.go
go vet ./internal/core/parser/
git add internal/core/parser/defusage.go internal/core/parser/namespace.go internal/core/parser/defusage_test.go
git commit -m "feat(parser): dispatch part/attribute def/usage with feature modifiers"
```

## Task 4 — Parser: relationships + multiplicity + value

This task fills in the declaration tail: relationship clauses (`specializes`/`:>`, `subsets`/`:>`, `redefines`/`:>>`, `references`/`::>`, `crosses`/`=>`, typing `:`/`defined by`), a multiplicity `[ ... ]` on usages, and a `= expr` feature value on usages. Each clause allows a comma-separated target list, producing one `Relationship` per target that shares the clause's kind. The symbolic `:>` means `RelSpecializes` in a definition context but `RelSubsets` in a usage context, so the parser receives an `isUsage` flag.

**Files:**
- Modify: `internal/core/parser/defusage.go` (add `parseRelationships`, `parseMultiplicity`, wire into `parseDefinition`/`parseUsage`)
- Test: `internal/core/parser/defusage_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/core/parser/defusage_test.go`:

```go
func relTargets(rels []*ast.Relationship) []string {
	out := make([]string, len(rels))
	for i, r := range rels {
		var parts string
		for j, seg := range r.Target.Parts {
			if j > 0 {
				parts += "::"
			}
			parts += seg.Text
		}
		out[i] = parts
	}
	return out
}

func TestParseDefinitionSpecializes(t *testing.T) {
	def := parseOneMember(t, "part def Car specializes Vehicle, Machine;").(*ast.Definition)
	if len(def.Relationships) != 2 {
		t.Fatalf("expected 2 relationships, got %d", len(def.Relationships))
	}
	for _, r := range def.Relationships {
		if r.Kind != ast.RelSpecializes {
			t.Fatalf("expected RelSpecializes, got %v", r.Kind)
		}
	}
	if got := relTargets(def.Relationships); got[0] != "Vehicle" || got[1] != "Machine" {
		t.Fatalf("targets=%v", got)
	}
}

func TestParseDefinitionSpecializesSymbol(t *testing.T) {
	def := parseOneMember(t, "part def Car :> Vehicle;").(*ast.Definition)
	if len(def.Relationships) != 1 || def.Relationships[0].Kind != ast.RelSpecializes {
		t.Fatalf("rels=%+v", def.Relationships)
	}
}

func TestParseUsageTypingAndSubsets(t *testing.T) {
	u := parseOneMember(t, "part engine : Engine subsets vehicle::parts;").(*ast.Usage)
	if len(u.Relationships) != 2 {
		t.Fatalf("expected 2 relationships, got %d", len(u.Relationships))
	}
	if u.Relationships[0].Kind != ast.RelTyping {
		t.Fatalf("first should be RelTyping, got %v", u.Relationships[0].Kind)
	}
	if u.Relationships[1].Kind != ast.RelSubsets {
		t.Fatalf("second should be RelSubsets, got %v", u.Relationships[1].Kind)
	}
}

func TestParseUsageSubsetsSymbol(t *testing.T) {
	// `:>` in a usage context is subsets, not specializes.
	u := parseOneMember(t, "part p :> q;").(*ast.Usage)
	if len(u.Relationships) != 1 || u.Relationships[0].Kind != ast.RelSubsets {
		t.Fatalf("rels=%+v", u.Relationships)
	}
}

func TestParseUsageRedefinesReferencesCrosses(t *testing.T) {
	u := parseOneMember(t, "part p :>> a ::> b => c;").(*ast.Usage)
	if len(u.Relationships) != 3 {
		t.Fatalf("expected 3 relationships, got %d", len(u.Relationships))
	}
	want := []ast.RelationshipKind{ast.RelRedefines, ast.RelReferences, ast.RelCrosses}
	for i, r := range u.Relationships {
		if r.Kind != want[i] {
			t.Fatalf("rel[%d]=%v want %v", i, r.Kind, want[i])
		}
	}
}

func TestParseUsageMultiplicityRange(t *testing.T) {
	u := parseOneMember(t, "part wheels [4];").(*ast.Usage)
	if u.Multiplicity == nil || u.Multiplicity.IsRange {
		t.Fatalf("expected single-bound multiplicity, got %+v", u.Multiplicity)
	}
	if _, ok := u.Multiplicity.Lower.(*ast.LiteralInteger); !ok {
		t.Fatalf("lower should be LiteralInteger, got %T", u.Multiplicity.Lower)
	}
}

func TestParseUsageMultiplicityStarRange(t *testing.T) {
	u := parseOneMember(t, "part parts [0..*];").(*ast.Usage)
	if u.Multiplicity == nil || !u.Multiplicity.IsRange {
		t.Fatalf("expected range multiplicity, got %+v", u.Multiplicity)
	}
	if _, ok := u.Multiplicity.Upper.(*ast.LiteralInfinity); !ok {
		t.Fatalf("upper should be LiteralInfinity, got %T", u.Multiplicity.Upper)
	}
}

func TestParseUsageValue(t *testing.T) {
	u := parseOneMember(t, "attribute mass = 1500;").(*ast.Usage)
	if u.Value == nil {
		t.Fatalf("expected value expression")
	}
	if _, ok := u.Value.(*ast.LiteralInteger); !ok {
		t.Fatalf("value should be LiteralInteger, got %T", u.Value)
	}
}

func TestParseUsageMultiplicityThenValue(t *testing.T) {
	u := parseOneMember(t, "attribute xs [3] = 7;").(*ast.Usage)
	if u.Multiplicity == nil || u.Value == nil {
		t.Fatalf("expected both multiplicity and value, got mult=%v value=%v", u.Multiplicity, u.Value)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/core/parser/ -run 'TestParse(Definition(Specializes|SpecializesSymbol)|Usage(Typing|Subsets|Redefines|Multiplicity|Value))' -v`
Expected: FAIL — `parseDefinition`/`parseUsage` do not yet populate `Relationships`, `Multiplicity`, or `Value` (they immediately call `parseNamespaceBody`, which errors on `specializes`/`:>`/`[`/`=`).

- [ ] **Step 3: Add relationship-clause keyword map**

At the top of `internal/core/parser/defusage.go` (below the existing maps), add:

```go
// relationshipKeywords maps a spelled-out relationship keyword to its kind.
// `:>`/`subsets` vs `specializes` disambiguation by context is handled in
// parseRelationships (symbolic ColonGt), so this map covers word forms only.
var relationshipKeywords = map[string]ast.RelationshipKind{
	"specializes": ast.RelSpecializes,
	"subsets":     ast.RelSubsets,
	"redefines":   ast.RelRedefines,
	"references":  ast.RelReferences,
	"crosses":     ast.RelCrosses,
}
```

- [ ] **Step 4: Implement parseRelationships**

Add to `internal/core/parser/defusage.go`:

```go
// parseRelationships parses zero or more relationship clauses. isUsage selects
// the meaning of the symbolic `:>` operator (subsets on a usage, specializes on
// a definition). Each clause may carry a comma-separated target list; every
// target becomes its own Relationship sharing the clause kind.
func (p *Parser) parseRelationships(isUsage bool) []*ast.Relationship {
	var rels []*ast.Relationship
	for {
		kind, ok := p.relationshipClauseKind(isUsage)
		if !ok {
			return rels
		}
		// One or more comma-separated qualified-name targets.
		for {
			start := p.peek().Span.Offset
			qn := p.parseQualifiedName()
			r := &ast.Relationship{Kind: kind, Target: qn}
			r.NodeSpan = p.spanFrom(start)
			rels = append(rels, r)
			if !p.accept2(lexer.Comma) {
				break
			}
		}
	}
}

// relationshipClauseKind consumes the operator/keyword that begins a
// relationship clause and returns its kind. It reports ok=false (consuming
// nothing) when the current token does not begin a relationship clause.
func (p *Parser) relationshipClauseKind(isUsage bool) (ast.RelationshipKind, bool) {
	// Word-form keywords.
	if t := p.peek(); t.Kind == lexer.Keyword {
		if k, ok := relationshipKeywords[t.KeywordID]; ok {
			p.advance()
			return k, true
		}
		if t.KeywordID == "defined" {
			// `defined by` typing form.
			p.advance()
			p.expect2Keyword("by")
			return ast.RelTyping, true
		}
	}
	// Symbolic operator forms.
	switch p.peek().Kind {
	case lexer.Colon:
		p.advance()
		return ast.RelTyping, true
	case lexer.ColonGt:
		p.advance()
		if isUsage {
			return ast.RelSubsets, true
		}
		return ast.RelSpecializes, true
	case lexer.ColonGtGt:
		p.advance()
		return ast.RelRedefines, true
	case lexer.ColonColonGt:
		p.advance()
		return ast.RelReferences, true
	case lexer.EqGt:
		p.advance()
		return ast.RelCrosses, true
	}
	return 0, false
}
```

Note: `expect2Keyword` is the existing harness helper (used in `expr.go:21`) that consumes an expected keyword and records a diagnostic on mismatch.

- [ ] **Step 5: Implement parseMultiplicity**

Add to `internal/core/parser/defusage.go`:

```go
// parseMultiplicity parses `[ lower ( .. upper )? ]` when a `[` is present.
// A bare `*` bound becomes an ast.LiteralInfinity. Returns nil when there is
// no multiplicity.
func (p *Parser) parseMultiplicity() *ast.Multiplicity {
	if p.peek().Kind != lexer.LBracket {
		return nil
	}
	start := p.peek().Span.Offset
	p.advance() // '['
	m := &ast.Multiplicity{}
	m.Lower = p.parseMultiplicityBound()
	if p.accept2(lexer.DotDot) {
		m.IsRange = true
		m.Upper = p.parseMultiplicityBound()
	}
	p.expect(lexer.RBracket, "expected ']' to close multiplicity")
	m.NodeSpan = p.spanFrom(start)
	return m
}

// parseMultiplicityBound parses a single multiplicity bound: `*` (infinity) or
// an expression.
func (p *Parser) parseMultiplicityBound() ast.Node {
	if p.peek().Kind == lexer.Star {
		star := p.peek()
		p.advance()
		inf := &ast.LiteralInfinity{}
		inf.NodeSpan = star.Span
		return inf
	}
	return p.ParseExpression()
}
```

- [ ] **Step 6: Wire relationships into parseDefinition**

Replace the body of `parseDefinition` in `internal/core/parser/defusage.go` (from Task 3) so it parses relationships before the body:

```go
func (p *Parser) parseDefinition(start int, kind ast.DefinitionKind, mods featureMods) *ast.Definition {
	def := &ast.Definition{
		Kind:        kind,
		IsAbstract:  mods.isAbstract,
		IsVariation: mods.isVariation,
		Ident:       p.parseIdentification(),
	}
	def.Relationships = p.parseRelationships(false)
	members, hasBody := p.parseNamespaceBody() // placeholder; Task 5 replaces
	def.Members = members
	def.HasBody = hasBody
	def.NodeSpan = p.spanFrom(start)
	return def
}
```

- [ ] **Step 7: Wire relationships/multiplicity/value into parseUsage**

Replace the body of `parseUsage` in `internal/core/parser/defusage.go`:

```go
func (p *Parser) parseUsage(start int, kind ast.UsageKind, mods featureMods) *ast.Usage {
	u := &ast.Usage{
		Kind:        kind,
		IsAbstract:  mods.isAbstract,
		IsReference: mods.isReference,
		Direction:   mods.direction,
		IsComposite: mods.isComposite,
		IsDerived:   mods.isDerived,
		IsOrdered:   mods.isOrdered,
		IsNonunique: mods.isNonunique,
		Ident:       p.parseIdentification(),
	}
	u.Relationships = p.parseRelationships(true)
	u.Multiplicity = p.parseMultiplicity()
	if p.accept2(lexer.Eq) {
		u.Value = p.ParseExpression()
	}
	members, hasBody := p.parseNamespaceBody() // placeholder; Task 5 replaces
	u.Members = members
	u.HasBody = hasBody
	u.NodeSpan = p.spanFrom(start)
	return u
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/core/parser/ -run 'TestParse(Definition(Specializes|SpecializesSymbol)|Usage(Typing|Subsets|Redefines|Multiplicity|Value))' -v`
Expected: PASS (all new cases). The Task 3 dispatch tests must still pass. The two `part def`→ErrorNode fixtures still fail — flipped in Task 5.

- [ ] **Step 9: gofmt, vet, commit**

```bash
gofmt -w internal/core/parser/defusage.go internal/core/parser/defusage_test.go
go vet ./internal/core/parser/
git add internal/core/parser/defusage.go internal/core/parser/defusage_test.go
git commit -m "feat(parser): parse def/usage relationships, multiplicity, and value"
```

## Task 5 — Parser: nested bodies + flip recovery fixtures

This task replaces the placeholder `parseNamespaceBody` calls in `parseDefinition`/`parseUsage` with a body parser that accepts BOTH nested def/usage members and ordinary namespace members. Body members carry optional visibility, wrapped in a `*ast.Membership` exactly like `parseMember`. This task also flips the two fixtures that assert `part def`→`ErrorNode` (now valid) and adds a genuinely-unknown-keyword recovery fixture in their place.

**Files:**
- Modify: `internal/core/parser/defusage.go` (add `parseDefUsageBody`, `parseBodyMember`; use them in `parseDefinition`/`parseUsage`)
- Modify: `internal/core/parser/recovery_test.go:10`, `internal/core/parser/namespace_test.go:132`
- Test: `internal/core/parser/defusage_test.go`

- [ ] **Step 1: Write the failing nested-body test**

Append to `internal/core/parser/defusage_test.go`:

```go
func TestParseDefinitionNestedBody(t *testing.T) {
	def := parseOneMember(t, "part def Car { part engine; attribute mass; }").(*ast.Definition)
	if !def.HasBody {
		t.Fatalf("expected body")
	}
	if len(def.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(def.Members))
	}
	// Body members are wrapped in Membership; unwrap and check kinds.
	m0 := def.Members[0].(*ast.Membership).Member.(*ast.Usage)
	if m0.Kind != ast.UsagePart {
		t.Fatalf("member[0] kind=%v", m0.Kind)
	}
	m1 := def.Members[1].(*ast.Membership).Member.(*ast.Usage)
	if m1.Kind != ast.UsageAttribute {
		t.Fatalf("member[1] kind=%v", m1.Kind)
	}
}

func TestParseDefinitionBodyMixedMembers(t *testing.T) {
	// Body accepts ordinary namespace members (e.g. comment) too.
	def := parseOneMember(t, "part def Car { part wheel; comment /* c */ }").(*ast.Definition)
	if len(def.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(def.Members))
	}
}

func TestParseUsageNestedBody(t *testing.T) {
	u := parseOneMember(t, "part car { part engine; }").(*ast.Usage)
	if !u.HasBody || len(u.Members) != 1 {
		t.Fatalf("expected 1 body member, got hasBody=%v members=%d", u.HasBody, len(u.Members))
	}
}

func TestParseDefinitionBodyVisibility(t *testing.T) {
	def := parseOneMember(t, "part def Car { private part secret; }").(*ast.Definition)
	m := def.Members[0].(*ast.Membership)
	if m.Visibility != ast.VisibilityPrivate {
		t.Fatalf("expected private, got %v", m.Visibility)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/core/parser/ -run 'TestParse(DefinitionNestedBody|DefinitionBodyMixedMembers|UsageNestedBody|DefinitionBodyVisibility)' -v`
Expected: FAIL — `parseNamespaceBody` only recognizes namespace members, so `part engine;` inside a body errors, producing `ErrorNode` members instead of nested `*ast.Usage`.

- [ ] **Step 3: Implement the def/usage body parser**

Add to `internal/core/parser/defusage.go`:

```go
// parseDefUsageBody parses a definition/usage body: `;` (no body) or
// `{ member* }`. Body members may be nested def/usage declarations or ordinary
// namespace members, each carrying optional visibility.
func (p *Parser) parseDefUsageBody() (members []ast.Node, hasBody bool) {
	if p.accept2(lexer.Semicolon) {
		return nil, false
	}
	if _, ok := p.expect(lexer.LBrace, "expected '{' or ';' after declaration"); !ok {
		return nil, false
	}
	for !p.at(lexer.RBrace) && !p.atEOF() {
		before := p.peek().Span.Offset
		m := p.parseBodyMember()
		if m != nil {
			members = append(members, m)
		}
		// Progress guard: if nothing was consumed, skip a token.
		if p.peek().Span.Offset == before {
			p.advance()
		}
	}
	p.expect(lexer.RBrace, "expected '}' to close body")
	return members, true
}

// parseBodyMember parses one body member. It handles leading visibility, tries
// def/usage first, then falls back to the ordinary namespace-member forms
// (import/alias unwrapped, everything else via parseDeclaration). Non-import/
// alias members are wrapped in *ast.Membership, mirroring parseMember.
func (p *Parser) parseBodyMember() ast.Node {
	start := p.peek().Span.Offset
	trivia := p.takeTrivia()
	vis := p.parseVisibility()

	// def/usage takes priority over namespace-member dispatch.
	if p.atDefUsageStart() {
		member := p.parseDefUsage(start)
		if member == nil {
			en := p.errorNodeSkip(start, "expected a body member")
			en.SetLeadingTrivia(trivia)
			return en
		}
		mem := &ast.Membership{Visibility: vis, Member: member}
		mem.NodeSpan = p.spanFrom(start)
		mem.SetLeadingTrivia(trivia)
		return mem
	}

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
		en := p.errorNodeSkip(start, "expected a body member")
		en.SetLeadingTrivia(trivia)
		return en
	}
	mem := &ast.Membership{Visibility: vis, Member: inner}
	mem.NodeSpan = p.spanFrom(start)
	mem.SetLeadingTrivia(trivia)
	return mem
}
```

Note: this mirrors `parseMember` (namespace.go:109) exactly — same `start`/`takeTrivia`/`parseVisibility` order, same explicit `import`/`alias` handling via `parseImport`/`parseAlias` — but tries `parseDefUsage` first. `parseImport`/`parseAlias` are the existing harness methods called by `parseMember`.

- [ ] **Step 4: Use the new body parser in parseDefinition/parseUsage**

In `internal/core/parser/defusage.go`, replace the two placeholder `parseNamespaceBody()` calls with `parseDefUsageBody()`:

In `parseDefinition`:
```go
	members, hasBody := p.parseDefUsageBody()
```

In `parseUsage`:
```go
	members, hasBody := p.parseDefUsageBody()
```

- [ ] **Step 5: Run nested-body tests to verify they pass**

Run: `go test ./internal/core/parser/ -run 'TestParse(DefinitionNestedBody|DefinitionBodyMixedMembers|UsageNestedBody|DefinitionBodyVisibility)' -v`
Expected: PASS.

- [ ] **Step 6: Flip the two stale ErrorNode fixtures**

In `internal/core/parser/recovery_test.go:9-21`, replace `TestRecoverBadMemberThenGood` — `part def X;` is now a valid definition, so use a genuinely unknown keyword to exercise recovery:

```go
func TestRecoverBadMemberThenGood(t *testing.T) {
	p := newParser("@@@ package P;")
	root := p.ParseFile()
	if len(root.Members) != 2 {
		t.Fatalf("got %d members, want 2", len(root.Members))
	}
	if _, ok := root.Members[0].(*ast.ErrorNode); !ok {
		t.Errorf("member[0] = %T, want *ast.ErrorNode", root.Members[0])
	}
	if _, ok := root.Members[1].(*ast.Membership); !ok {
		t.Errorf("member[1] = %T, want *ast.Membership", root.Members[1])
	}
}
```

In `internal/core/parser/namespace_test.go:132-144`, replace `TestParseFileUnknownKeywordErrorNode` so it both proves `part def` now parses AND that an unknown keyword still errors:

```go
func TestParseFileDefinitionNowParses(t *testing.T) {
	p := newParser("part def Vehicle;")
	root := p.ParseFile()
	if len(root.Members) != 1 {
		t.Fatalf("members = %+v", root.Members)
	}
	def, ok := root.Members[0].(*ast.Membership).Member.(*ast.Definition)
	if !ok {
		t.Fatalf("expected *ast.Definition, got %T", root.Members[0])
	}
	if def.Kind != ast.DefPart || def.Ident.Name != "Vehicle" {
		t.Fatalf("kind=%v name=%q", def.Kind, def.Ident.Name)
	}
}

func TestParseFileUnknownKeywordErrorNode(t *testing.T) {
	p := newParser("@@@ Vehicle;")
	root := p.ParseFile()
	if len(root.Members) == 0 {
		t.Fatalf("members = %+v", root.Members)
	}
	if _, ok := root.Members[0].(*ast.ErrorNode); !ok {
		t.Fatalf("expected ErrorNode, got %T", root.Members[0])
	}
	if len(p.Diagnostics) == 0 {
		t.Fatal("expected a diagnostic")
	}
}
```

Note: `@@@` lexes as `@@` + `@` (metadata-access operators), neither of which is a declaration keyword or `import`/`alias`, so `parseDeclaration` returns nil and `errorNodeSkip` recovery fires — verified against the current lexer. (`+ package P;` is an equally valid alternative if needed.)

- [ ] **Step 7: Run the full parser suite green**

Run: `go test ./internal/core/parser/`
Expected: PASS (all fixtures, including the flipped ones). This is the first task where the full parser package must be green.

- [ ] **Step 8: gofmt, vet, commit**

```bash
gofmt -w internal/core/parser/defusage.go internal/core/parser/defusage_test.go internal/core/parser/recovery_test.go internal/core/parser/namespace_test.go
go vet ./internal/core/parser/
git add internal/core/parser/defusage.go internal/core/parser/defusage_test.go internal/core/parser/recovery_test.go internal/core/parser/namespace_test.go
git commit -m "feat(parser): nested def/usage bodies; flip stale recovery fixtures"
```

## Task 6 — Symbols: buildDecl cases + kinds

This task teaches the scope builder about `Definition`/`Usage` nodes: four new `SymbolKind`s, and `buildDecl` cases that register a symbol, create a child scope when the declaration has a body, and recurse into nested members. Anonymous usages (empty name) still get a child scope for their members but are not registered under a name.

**Files:**
- Modify: `internal/core/symbols/symbol.go:13` (kinds), `:24` (names map)
- Modify: `internal/core/symbols/builder.go:47` (`buildDecl` switch), `:104` (`defineIdent` anon guard)
- Test: `internal/core/symbols/builder_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/core/symbols/builder_test.go` (create the file with this `package symbols` header if it does not exist):

```go
func TestBuildDefinitionAndNestedUsages(t *testing.T) {
	src := "part def Car { part engine; attribute mass; }"
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	scope := Build(root)

	syms := scope.LookupLocalAll("Car")
	if len(syms) != 1 {
		t.Fatalf("expected 1 Car symbol, got %d", len(syms))
	}
	car := syms[0]
	if car.Kind != SymbolPartDef {
		t.Fatalf("Car kind = %v, want SymbolPartDef", car.Kind)
	}
	if car.Scope == nil {
		t.Fatalf("Car should own a child scope")
	}
	if len(car.Scope.LookupLocalAll("engine")) != 1 {
		t.Fatalf("engine not registered in Car scope")
	}
	eng := car.Scope.LookupLocalAll("engine")[0]
	if eng.Kind != SymbolPartUsage {
		t.Fatalf("engine kind = %v, want SymbolPartUsage", eng.Kind)
	}
	if len(car.Scope.LookupLocalAll("mass")) != 1 {
		t.Fatalf("mass not registered in Car scope")
	}
	if car.Scope.LookupLocalAll("mass")[0].Kind != SymbolAttributeUsage {
		t.Fatalf("mass kind wrong")
	}
}

func TestBuildAttributeDefKind(t *testing.T) {
	root := parser.New(source.New("<t>", []byte("attribute def Mass;"))).ParseFile()
	scope := Build(root)
	syms := scope.LookupLocalAll("Mass")
	if len(syms) != 1 || syms[0].Kind != SymbolAttributeDef {
		t.Fatalf("Mass symbol wrong: %+v", syms)
	}
}

func TestBuildAnonymousUsageNotNamed(t *testing.T) {
	// An anonymous usage inside a body must not register a "" name key,
	// but its members (if any) still live in a child scope reachable via
	// the owning definition's scope traversal.
	root := parser.New(source.New("<t>", []byte("part def Car { part; }"))).ParseFile()
	scope := Build(root)
	car := scope.LookupLocalAll("Car")[0]
	if len(car.Scope.LookupLocalAll("")) != 0 {
		t.Fatalf("anonymous usage should not be registered under empty name")
	}
	// The anonymous usage still produced a child scope of Car's scope.
	if len(car.Scope.Children()) != 1 {
		t.Fatalf("expected 1 child scope for the anonymous usage, got %d", len(car.Scope.Children()))
	}
}
```

Ensure the test file imports are present at the top of `builder_test.go`:

```go
import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/symbols/ -run 'TestBuild(DefinitionAndNestedUsages|AttributeDefKind|AnonymousUsageNotNamed)' -v`
Expected: FAIL — `SymbolPartDef` etc. are undefined, and `buildDecl` has no `*ast.Definition`/`*ast.Usage` cases so `Car`/`engine` symbols are never registered.

- [ ] **Step 3: Add the new SymbolKinds**

In `internal/core/symbols/symbol.go`, extend the const block (line 13) — append after `SymbolTextualRepresentation`:

```go
	SymbolPartDef
	SymbolAttributeDef
	SymbolPartUsage
	SymbolAttributeUsage
```

And extend the `symbolKindNames` map (line 24):

```go
	SymbolPartDef:        "partDef",
	SymbolAttributeDef:   "attributeDef",
	SymbolPartUsage:      "partUsage",
	SymbolAttributeUsage: "attributeUsage",
```

- [ ] **Step 4: Add buildDecl cases**

In `internal/core/symbols/builder.go`, add two cases to the `buildDecl` switch (before the `*ast.Import, *ast.FilterMember, *ast.ErrorNode` case at line 76):

```go
	case *ast.Definition:
		child := NewScope(scope, d)
		sym := newSymbol(d.Ident, definitionSymbolKind(d.Kind), d, vis, child, scope, trivia)
		defineIdent(scope, d.Ident, sym)
		scope.AddChild(child)
		buildMembers(child, d.Members)
	case *ast.Usage:
		child := NewScope(scope, d)
		sym := newSymbol(d.Ident, usageSymbolKind(d.Kind), d, vis, child, scope, trivia)
		defineIdent(scope, d.Ident, sym)
		scope.AddChild(child)
		buildMembers(child, d.Members)
```

Add the kind-mapping helpers at the end of `builder.go`:

```go
// definitionSymbolKind maps an ast.DefinitionKind to its SymbolKind.
func definitionSymbolKind(k ast.DefinitionKind) SymbolKind {
	switch k {
	case ast.DefPart:
		return SymbolPartDef
	case ast.DefAttribute:
		return SymbolAttributeDef
	default:
		return SymbolUnknown
	}
}

// usageSymbolKind maps an ast.UsageKind to its SymbolKind.
func usageSymbolKind(k ast.UsageKind) SymbolKind {
	switch k {
	case ast.UsagePart:
		return SymbolPartUsage
	case ast.UsageAttribute:
		return SymbolAttributeUsage
	default:
		return SymbolUnknown
	}
}
```

- [ ] **Step 5: Guard defineIdent against anonymous declarations**

`defineIdent` (line 104) currently registers under both `ShortName` and `Name`. For an anonymous usage both are empty, which would register a spurious `""` key. Change it to skip empty keys:

```go
// defineIdent registers sym under its short and primary name keys, skipping
// any that are empty (e.g. anonymous usages).
func defineIdent(scope *Scope, id ast.Identification, sym *Symbol) {
	if id.ShortName != "" {
		scope.Define(id.ShortName, sym)
	}
	if id.Name != "" {
		scope.Define(id.Name, sym)
	}
}
```

Note: this is safe for existing callers — package/namespace/alias/etc. all have at least one non-empty name in practice, and skipping a genuinely-empty key only removes a bogus `""` entry.

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/core/symbols/ -v`
Expected: PASS (new tests plus all existing symbols tests — the `defineIdent` guard must not break any).

- [ ] **Step 7: gofmt, vet, commit**

```bash
gofmt -w internal/core/symbols/symbol.go internal/core/symbols/builder.go internal/core/symbols/builder_test.go
go vet ./internal/core/symbols/
git add internal/core/symbols/symbol.go internal/core/symbols/builder.go internal/core/symbols/builder_test.go
git commit -m "feat(symbols): register part/attribute def/usage symbols and scopes"
```

## Task 7 — Libs: populate Supers from specialization edges

The `symRecord.Supers` field (`libs/record.go:20`) has been an empty placeholder. Now that `Definition`/`Usage` carry `Relationships`, populate `Supers` from the raw target text of the *specialization* edges only — `RelSpecializes`, `RelSubsets`, `RelRedefines`. Typing (`RelTyping`), `RelReferences`, and `RelCrosses` are NOT specialization edges and are excluded (per spec §5.3).

**Files:**
- Modify: `internal/core/libs/record.go:49` (`collectScope` — set `Supers`)
- Test: `internal/core/libs/record_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/core/libs/record_test.go` (create with `package libs` header + imports if absent):

```go
func TestRecordSupersFromSpecializationEdges(t *testing.T) {
	src := "part def Car specializes Vehicle, Machine; part def Vehicle; part def Machine;"
	root := parser.New(source.New("lib", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("lib", root)

	rec := recordFromIndex("lib", idx)
	if rec == nil {
		t.Fatalf("expected a record")
	}
	var car *symRecord
	for i := range rec.Symbols {
		if rec.Symbols[i].FQN == "Car" {
			car = &rec.Symbols[i]
		}
	}
	if car == nil {
		t.Fatalf("Car record not found")
	}
	if len(car.Supers) != 2 || car.Supers[0] != "Vehicle" || car.Supers[1] != "Machine" {
		t.Fatalf("Supers = %v, want [Vehicle Machine]", car.Supers)
	}
}

func TestRecordSupersExcludesTypingAndReferences(t *testing.T) {
	// Typing (`:`), references (`::>`), crosses (`=>`) are not specialization
	// edges and must not appear in Supers. Only subsets/redefines/specializes do.
	src := "part def Engine; part e : Engine subsets Engine;"
	root := parser.New(source.New("lib", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("lib", root)

	rec := recordFromIndex("lib", idx)
	var e *symRecord
	for i := range rec.Symbols {
		if rec.Symbols[i].FQN == "e" {
			e = &rec.Symbols[i]
		}
	}
	if e == nil {
		t.Fatalf("e record not found")
	}
	// The typing `: Engine` is excluded; only `subsets Engine` counts.
	if len(e.Supers) != 1 || e.Supers[0] != "Engine" {
		t.Fatalf("Supers = %v, want [Engine] (subsets only)", e.Supers)
	}
}
```

Ensure the imports at the top of `record_test.go` include:

```go
import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/libs/ -run 'TestRecordSupers' -v`
Expected: FAIL — `collectScope` never sets `Supers`, so `car.Supers` is empty (want `[Vehicle Machine]`).

- [ ] **Step 3: Populate Supers in collectScope**

In `internal/core/libs/record.go`, change the `symRecord` construction in `collectScope` (line 49) to compute `Supers` from the symbol's declaration:

```go
		rec.Symbols = append(rec.Symbols, symRecord{
			FQN:    fqn,
			Kind:   sym.Kind,
			Span:   sym.DeclSpan,
			Supers: supersOf(sym.Decl),
		})
```

Add the `supersOf` helper (and the `ast` import) at the end of `record.go`:

```go
// supersOf extracts the raw qualified-name text of the specialization edges
// (specializes/subsets/redefines) declared by a Definition or Usage. Typing,
// references, and crosses edges are not specializations and are excluded.
// Returns nil for any other node kind.
func supersOf(decl ast.Node) []string {
	var rels []*ast.Relationship
	switch d := decl.(type) {
	case *ast.Definition:
		rels = d.Relationships
	case *ast.Usage:
		rels = d.Relationships
	default:
		return nil
	}
	var out []string
	for _, r := range rels {
		switch r.Kind {
		case ast.RelSpecializes, ast.RelSubsets, ast.RelRedefines:
			out = append(out, qualifiedNameText(r.Target))
		}
	}
	return out
}

// qualifiedNameText renders a QualifiedName as "A::B::C" (no leading $:: marker;
// specialization targets are relative names). Returns "" for a nil name.
func qualifiedNameText(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	var b strings.Builder
	for i, seg := range qn.Parts {
		if i > 0 {
			b.WriteString("::")
		}
		b.WriteString(seg.Text)
	}
	return b.String()
}
```

Add `"strings"` and the ast import to the `record.go` import block:

```go
import (
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/core/libs/ -v`
Expected: PASS (new tests plus all existing libs tests).

- [ ] **Step 5: gofmt, vet, commit**

```bash
gofmt -w internal/core/libs/record.go internal/core/libs/record_test.go
go vet ./internal/core/libs/
git add internal/core/libs/record.go internal/core/libs/record_test.go
git commit -m "feat(libs): populate symRecord.Supers from specialization edges"
```

## Task 8 — Resolve: resolveDecl cases

Teach the resolver to (a) resolve each relationship target, (b) resolve expressions inside a usage's multiplicity bounds and feature value, and (c) recurse into nested-member child scopes. This is the name-resolution tier; the kind-compatibility check comes in Task 9 and relies on the memoized resolutions this task produces.

**Files:**
- Modify: `internal/core/resolve/document.go:81` (`resolveDecl` switch — add two cases)
- Test: `internal/core/resolve/defusage_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/core/resolve/defusage_test.go`:

```go
package resolve

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// resolveDoc parses src, indexes it, resolves it, and returns the resolver's
// diagnostics.
func resolveDoc(t *testing.T, src string) []Diagnostic {
	t.Helper()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("<t>", root)
	r := New(idx)
	r.ResolveDocument("<t>", root)
	return r.Diagnostics
}

func TestResolveDefinitionSpecializesResolves(t *testing.T) {
	// Vehicle is declared, so `specializes Vehicle` resolves with no diagnostics.
	diags := resolveDoc(t, "part def Vehicle; part def Car specializes Vehicle;")
	if len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
}

func TestResolveDefinitionSpecializesUnresolved(t *testing.T) {
	// Missing is not declared, so the specialization target is unresolved.
	diags := resolveDoc(t, "part def Car specializes Missing;")
	if len(diags) == 0 {
		t.Fatalf("expected an unresolved-target diagnostic")
	}
}

func TestResolveUsageValueReference(t *testing.T) {
	// The feature value references `base`, which is declared as a sibling usage.
	diags := resolveDoc(t, "part def Car { attribute base; attribute mass = base; }")
	if len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
}

func TestResolveUsageValueUnresolved(t *testing.T) {
	diags := resolveDoc(t, "part def Car { attribute mass = undefinedRef; }")
	if len(diags) == 0 {
		t.Fatalf("expected an unresolved value-reference diagnostic")
	}
}

func TestResolveNestedUsageTyping(t *testing.T) {
	// `part engine : Engine` inside Car resolves the typing target Engine.
	diags := resolveDoc(t, "part def Engine; part def Car { part engine : Engine; }")
	if len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
	_ = ast.RelTyping // keep ast imported for symmetry with other resolve tests
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/resolve/ -run 'TestResolve(Definition|Usage|Nested)' -v`
Expected: FAIL — `resolveDecl` has no `*ast.Definition`/`*ast.Usage` cases, so specialization targets, values, and nested members are never resolved; the "unresolved" tests find zero diagnostics.

- [ ] **Step 3: Add resolveDecl cases**

In `internal/core/resolve/document.go`, add two cases to the `resolveDecl` switch (before the closing `}` at line 81, after the `*ast.FilterMember` case):

```go
	case *ast.Definition:
		r.resolvePrefixes(scope, d.Prefixes)
		r.resolveRelationships(scope, d.Relationships)
		if child := r.childScope(scope, d); child != nil {
			r.walkMembers(child, d.Members)
		}
	case *ast.Usage:
		r.resolvePrefixes(scope, d.Prefixes)
		r.resolveRelationships(scope, d.Relationships)
		if d.Multiplicity != nil {
			r.resolveExpr(scope, d.Multiplicity.Lower)
			r.resolveExpr(scope, d.Multiplicity.Upper)
		}
		r.resolveExpr(scope, d.Value)
		if child := r.childScope(scope, d); child != nil {
			r.walkMembers(child, d.Members)
		}
```

Add the `resolveRelationships` helper after `resolvePrefixes` (line 100):

```go
// resolveRelationships resolves each relationship target as a qualified name.
func (r *Resolver) resolveRelationships(scope *symbols.Scope, rels []*ast.Relationship) {
	for _, rel := range rels {
		if rel != nil && rel.Target != nil {
			r.ResolveQualified(scope, rel.Target)
		}
	}
}
```

Note: `resolveExpr` already no-ops on `nil` (line 106) and on `LiteralInfinity` (falls through the switch), so passing an absent `Value` or a `[*]` bound is safe.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/core/resolve/ -v`
Expected: PASS (new tests plus all existing resolve tests).

- [ ] **Step 5: gofmt, vet, commit**

```bash
gofmt -w internal/core/resolve/document.go internal/core/resolve/defusage_test.go
go vet ./internal/core/resolve/
git add internal/core/resolve/document.go internal/core/resolve/defusage_test.go
git commit -m "feat(resolve): resolve def/usage relationships, values, and nested members"
```

## Task 9 — Type pass: LevelType kind-compat

Add the first `LevelType` pass. It walks the AST scope-tree, and for every relationship whose target resolves, checks that the target's symbol kind is compatible with the source node and relationship kind. Unresolved targets are skipped (the name-resolution tier already flagged them, and tier gating usually prevents this pass from running at all when name-res errored). Compatibility rules come from spec §6.3.

The pass reuses the context's shared, already-memoized resolver (the name-resolution pass ran first at a lower level and populated the memo). It descends scopes by matching each `Definition`/`Usage` node to the child `*symbols.Scope` whose `Node()` is that node.

**Files:**
- Create: `internal/core/passes/typecheck.go`
- Modify: `internal/core/passes/analyze.go:14` (register the pass)
- Test: `internal/core/passes/typecheck_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/core/passes/typecheck_test.go`:

```go
package passes

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// typeDiags parses src, indexes it, and runs the full default registry,
// returning only the diagnostics whose Source is "type".
func typeDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("<t>", root)
	all := Analyze("<t>", root, nil, idx)
	var out []Diagnostic
	for _, d := range all {
		if d.Source == "type" {
			out = append(out, d)
		}
	}
	return out
}

func TestTypeCheckSpecializesSameKindOK(t *testing.T) {
	// part def specializes part def — compatible, no type diagnostic.
	diags := typeDiags(t, "part def Vehicle; part def Car specializes Vehicle;")
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

func TestTypeCheckSpecializesCrossKindError(t *testing.T) {
	// part def specializes attribute def — incompatible kinds.
	diags := typeDiags(t, "attribute def Mass; part def Car specializes Mass;")
	if len(diags) != 1 {
		t.Fatalf("expected exactly one type diagnostic, got %v", diags)
	}
	if diags[0].Code != "type" {
		t.Fatalf("expected code %q, got %q", "type", diags[0].Code)
	}
}

func TestTypeCheckTypingWantsMatchingDef(t *testing.T) {
	// part usage typed by an attribute def — incompatible.
	diags := typeDiags(t, "attribute def Mass; part def Car { part p : Mass; }")
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
}

func TestTypeCheckTypingMatchingDefOK(t *testing.T) {
	// part usage typed by a part def — compatible.
	diags := typeDiags(t, "part def Engine; part def Car { part e : Engine; }")
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

func TestTypeCheckUnresolvedTargetSkipped(t *testing.T) {
	// Unresolved target: name-resolution errors, and the type pass is gated
	// out — so there are no "type" diagnostics (only a name-resolution one).
	diags := typeDiags(t, "part def Car specializes Missing;")
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics (gated), got %v", diags)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/passes/ -run TestTypeCheck -v`
Expected: FAIL — `TypeCheckPass` does not exist yet, so it is not registered and the cross-kind/typing tests find zero `"type"` diagnostics.

- [ ] **Step 3: Create the type-check pass**

Create `internal/core/passes/typecheck.go`:

```go
package passes

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// TypeCheckPass validates that each def/usage relationship target has a symbol
// kind compatible with the source node and relationship kind (spec §6.3).
// It runs at LevelType, after name resolution; unresolved targets are skipped.
type TypeCheckPass struct{}

// Level reports the type dependency level.
func (TypeCheckPass) Level() PassLevel { return LevelType }

// Run walks the scope tree checking relationship kind-compatibility.
func (TypeCheckPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	tc := &typeChecker{resolver: ctx.Resolver()}
	tc.walk(rootScope, root.Members)
	return tc.diags
}

// typeChecker accumulates diagnostics while walking the scope tree.
type typeChecker struct {
	resolver *resolve.Resolver
	diags    []Diagnostic
}

// walk visits each member in scope, checking def/usage relationships and
// recursing into child scopes.
func (tc *typeChecker) walk(scope *symbols.Scope, members []ast.Node) {
	for _, m := range members {
		switch d := unwrapType(m).(type) {
		case *ast.Definition:
			tc.checkRelationships(scope, d.Relationships, true, d.Kind, 0)
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		case *ast.Usage:
			tc.checkRelationships(scope, d.Relationships, false, 0, d.Kind)
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		case *ast.Package:
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		case *ast.Namespace:
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		}
	}
}

// checkRelationships validates each relationship of a def (isDef true, defKind
// set) or usage (isDef false, useKind set).
func (tc *typeChecker) checkRelationships(scope *symbols.Scope, rels []*ast.Relationship, isDef bool, defKind ast.DefinitionKind, useKind ast.UsageKind) {
	for _, rel := range rels {
		if rel == nil || rel.Target == nil {
			continue
		}
		sym, ok := tc.resolver.ResolveQualified(scope, rel.Target)
		if !ok || sym == nil {
			continue // unresolved: name-resolution tier owns this
		}
		if msg := compatMessage(isDef, defKind, useKind, rel.Kind, sym.Kind); msg != "" {
			tc.diags = append(tc.diags, Diagnostic{
				Severity: SeverityError,
				Span:     rel.Target.Span(),
				Message:  msg,
				Code:     "type",
				Source:   "type",
			})
		}
	}
}

// compatMessage returns "" if the relationship is kind-compatible, or an error
// message describing the mismatch (spec §6.3).
func compatMessage(isDef bool, defKind ast.DefinitionKind, useKind ast.UsageKind, rel ast.RelationshipKind, target symbols.SymbolKind) string {
	switch rel {
	case ast.RelSpecializes:
		// Source must be a definition; target must be a definition of the same kind.
		want := defSymbolKind(defKind)
		if !isDef {
			return fmt.Sprintf("only a definition may specialize; found a usage")
		}
		if !isDefKind(target) {
			return fmt.Sprintf("%s cannot specialize %s (target is not a definition)", defKind, target)
		}
		if target != want {
			return fmt.Sprintf("%s cannot specialize %s (kind mismatch)", defKind, target)
		}
	case ast.RelSubsets, ast.RelRedefines:
		// Source must be a usage; target must be a usage.
		if isDef {
			return fmt.Sprintf("a definition may not %s a feature", rel)
		}
		if !isUsageKind(target) {
			return fmt.Sprintf("%s target must be a usage, found %s", rel, target)
		}
	case ast.RelTyping:
		// Typing applies to a usage; the target definition kind must match.
		if isDef {
			return "" // typing on a definition is not produced by the parser; ignore
		}
		if !isDefKind(target) {
			return fmt.Sprintf("type must be a definition, found %s", target)
		}
		if target != usageWantsDefKind(useKind) {
			return fmt.Sprintf("%s cannot be typed by %s (kind mismatch)", useKind, target)
		}
	case ast.RelReferences, ast.RelCrosses:
		// Light check: reference/crosses relate usages to usages.
		if !isDef && !isUsageKind(target) {
			return fmt.Sprintf("%s target must be a usage, found %s", rel, target)
		}
	}
	return ""
}

// defSymbolKind maps a DefinitionKind to its SymbolKind.
func defSymbolKind(k ast.DefinitionKind) symbols.SymbolKind {
	switch k {
	case ast.DefPart:
		return symbols.SymbolPartDef
	case ast.DefAttribute:
		return symbols.SymbolAttributeDef
	}
	return symbols.SymbolUnknown
}

// usageWantsDefKind returns the definition kind a usage of the given kind
// expects when typed (part usage wants part def, etc.).
func usageWantsDefKind(k ast.UsageKind) symbols.SymbolKind {
	switch k {
	case ast.UsagePart:
		return symbols.SymbolPartDef
	case ast.UsageAttribute:
		return symbols.SymbolAttributeDef
	}
	return symbols.SymbolUnknown
}

// isDefKind reports whether k is a definition symbol kind.
func isDefKind(k symbols.SymbolKind) bool {
	return k == symbols.SymbolPartDef || k == symbols.SymbolAttributeDef
}

// isUsageKind reports whether k is a usage symbol kind.
func isUsageKind(k symbols.SymbolKind) bool {
	return k == symbols.SymbolPartUsage || k == symbols.SymbolAttributeUsage
}

// unwrapType strips a Membership wrapper to reach the underlying member node.
func unwrapType(n ast.Node) ast.Node {
	if m, ok := n.(*ast.Membership); ok {
		return m.Member
	}
	return n
}

// childScopeOf returns the child scope whose owning node is decl, or nil.
func childScopeOf(scope *symbols.Scope, decl ast.Node) *symbols.Scope {
	for _, c := range scope.Children() {
		if c.Node() == decl {
			return c
		}
	}
	return nil
}
```

Note: `defKind`/`useKind` are rendered in messages via their `String()` methods (defined in Task 1), and `symbols.SymbolKind` renders via its `String()` (Task 6).

- [ ] **Step 4: Register the pass**

In `internal/core/passes/analyze.go`, add the pass to `DefaultRegistry` after `NameResolutionPass{}` (line 14):

```go
	reg.Register(NameResolutionPass{})
	reg.Register(TypeCheckPass{})
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/core/passes/ -run TestTypeCheck -v`
Expected: PASS (all five). The unresolved-target test passes because the name-resolution error at `LevelNameResolution` gates out the `LevelType` pass (registry.go:41).

- [ ] **Step 6: Full passes suite green gate**

Run: `go test ./internal/core/passes/`
Expected: PASS (existing pass tests plus the new ones).

- [ ] **Step 7: gofmt, vet, commit**

```bash
gofmt -w internal/core/passes/typecheck.go internal/core/passes/analyze.go internal/core/passes/typecheck_test.go
go vet ./internal/core/passes/
git add internal/core/passes/typecheck.go internal/core/passes/analyze.go internal/core/passes/typecheck_test.go
git commit -m "feat(passes): add LevelType kind-compatibility check for def/usage"
```

## Task 10 — Integration test + review + handoff

Prove the whole vertical slice works end-to-end through the `model.Workspace` façade (the same entry point the LSP and REPL use), run the full suite green, then get a review and update the handoff.

**Files:**
- Test: `internal/core/model/defusage_test.go`

- [ ] **Step 1: Write the end-to-end test**

Create `internal/core/model/defusage_test.go`:

```go
package model

import "testing"

// hasTypeDiag reports whether any diagnostic came from the type-check pass.
func hasTypeDiag(diags []diagWithSource) bool {
	for _, d := range diags {
		if d.source == "type" {
			return true
		}
	}
	return false
}

type diagWithSource struct{ source string }

func TestWorkspaceDefUsageResolvesClean(t *testing.T) {
	ws := NewWorkspace()
	src := "part def Engine; part def Car specializes Engine { part e : Engine; }"
	ws.Open("m.sysml", []byte(src), 1)
	diags := ws.Diagnostics("m.sysml")
	if len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
}

func TestWorkspaceDefUsageCrossKindTypeError(t *testing.T) {
	ws := NewWorkspace()
	src := "attribute def Mass; part def Car specializes Mass;"
	ws.Open("m.sysml", []byte(src), 1)
	diags := ws.Diagnostics("m.sysml")
	found := false
	for _, d := range diags {
		if d.Source == "type" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a type diagnostic, got %v", diags)
	}
}
```

Note: `ws.Diagnostics` returns `[]passes.Diagnostic`, whose `Source` field is exported. The helper stubs above are unused scaffolding — delete `hasTypeDiag`/`diagWithSource` and write the second test using `d.Source` directly (as shown in `TestWorkspaceDefUsageCrossKindTypeError`). Keep only the two `TestWorkspace…` functions.

- [ ] **Step 2: Run the integration test**

Run: `go test ./internal/core/model/ -run TestWorkspaceDefUsage -v`
Expected: PASS — the clean model resolves with zero diagnostics; the cross-kind model surfaces a `Source=="type"` diagnostic.

- [ ] **Step 3: Full-suite green gate**

Run: `go test ./...`
Expected: all packages PASS (including the flipped parser fixtures from Task 5).

- [ ] **Step 4: Vet and race**

Run: `go vet ./... && go test -race ./internal/core/...`
Expected: vet clean, no data races.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/core/model/defusage_test.go
git add internal/core/model/defusage_test.go
git commit -m "test: def/usage end-to-end through workspace"
```

- [ ] **Step 6: Final review (REQUIRED)**

Dispatch a reviewer subagent (superpowers:requesting-code-review) over the whole vertical slice against the spec `docs/superpowers/specs/2026-07-29-def-usage-taxonomy-design.md`. Files to review: `internal/core/ast/defusage.go` + `ast/dump.go`; `internal/core/lexer/token.go` + `lexer.go`; `internal/core/parser/defusage.go` + `namespace.go`; `internal/core/symbols/symbol.go` + `builder.go`; `internal/core/resolve/document.go`; `internal/core/libs/record.go`; `internal/core/passes/typecheck.go` + `analyze.go`. The reviewer must verify (against disk):
- Spec §3–§6 coverage: all four relationship forms (symbolic + keyword), modifiers, multiplicity, value, nested bodies, Supers population, and the LevelType kind-compat pass.
- All FOUR manual traversals handle the new nodes: `ast/dump.go`, `symbols/builder.go`, `resolve/document.go`, `passes/typecheck.go`. A missing case is a silent no-op bug.
- No dead code / placeholders / TODO in non-test files; error handling (nil guards on Target/Multiplicity/Value/child scopes); parser error-recovery still terminates (progress guards).
- Anonymous usages do not register a spurious `""` symbol.

Record the verdict. Fix any BLOCKING issues (with tests) before declaring complete.

- [ ] **Step 7: Finish**

- `finishing-a-development-branch`: no-op (all commits on master, no PR).
- Update `handoff2.md`: mark Plan 8 COMPLETE, list the delivered def/usage API (AST nodes, lexer ops, parser entry points, new SymbolKinds, TypeCheckPass), record the commit chain tail, and note the deferrals (other ~13 def/usage kinds; transitive/multiplicity/abstract checks; stdlib auto-load) for the next agent.
