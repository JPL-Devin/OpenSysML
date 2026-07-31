# Plan 01: Source & Lexer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the foundational `source` and `lexer` packages that tokenize any SysML v2 / KerML file into a lazy, span-tracked token stream.

**Architecture:** Two packages in a single Go module. `internal/core/source` owns file content, byte-offset spans, and line/column mapping. `internal/core/lexer` is a hand-written, pull-based scanner that produces tokens on demand for a parser to drive. AST/parser are out of scope (Plan 02).

**Tech Stack:** Go 1.25, standard library only. Table-driven `go test`.

---

## File Structure

Created in this plan (module root = repo root `/home/han/IdeaProjects/Systems-Modeling`):

- `go.mod` — Go module declaration, module path `github.com/Open-MBEE/Systemica`, Go 1.25.
- `internal/core/source/source.go` — `SourceFile` (name + byte content), `Span` (byte Offset+Len), `Pos` (Line+Col, 1-based). Responsibility: own file bytes and span types.
- `internal/core/source/lineindex.go` — `LineIndex` built from content; maps byte offset → `Pos` (line/col) and back. Responsibility: offset↔line/col mapping only.
- `internal/core/source/source_test.go` — table-driven tests for spans + line index.
- `internal/core/lexer/token.go` — `Kind` (int enum) + `Token` struct (Kind, Span, plus lazily-derived text via SourceFile). Responsibility: token vocabulary.
- `internal/core/lexer/keywords.go` — `keywords map[string]Kind`, the union keyword set. Responsibility: keyword recognition table.
- `internal/core/lexer/lexer.go` — `Lexer` struct + `New(*source.SourceFile) *Lexer` + `Next() Token` pull API + all scan routines. Responsibility: the scanner.
- `internal/core/lexer/lexer_test.go` — table-driven input→token-sequence tests.
- `testdata/lex/*.sysml`, `testdata/lex/*.kerml` — small fixtures for integration tokenization test.

No files modified (greenfield). The spec at `docs/superpowers/specs/2026-07-25-sysml-v2-go-design.md` section 5 (Lexer) is the design authority.

## Grammar Reference

Authoritative lexical rules (from pilot `org.omg.kerml.expressions.xtext/.../KerMLExpressions.xtext`). Implement exactly these:

- **Hidden trivia** (skipped by parser, but lexer must track spans): `WS` = `(' '|'\t'|'\r'|'\n')+`; `ML_NOTE` = `//* ... */`; `SL_NOTE` = `// ...` to end of line. `//*` MUST be matched before `//`.
- **REGULAR_COMMENT** = `/* ... */` — NOT hidden (it is a real token; block comment used for `comment`/`doc` bodies).
- **ID** = `[A-Za-z_][A-Za-z_0-9]*` — ASCII ONLY. Non-ASCII names only via unrestricted form.
- **UNRESTRICTED_NAME** = single-quoted `'...'`. Escapes: `\b \t \n \f \r \" \' \\`. Body char = escape sequence OR any char except `\` and `'`.
- **STRING_VALUE** = double-quoted `"..."`, same escape set.
- **DECIMAL_VALUE** = `[0-9]+`.
- **EXP_VALUE** = `DECIMAL_VALUE ('e'|'E') ('+'|'-')? DECIMAL_VALUE`.
- **Real** = `DECIMAL_VALUE? '.' (DECIMAL_VALUE | EXP_VALUE)` OR `EXP_VALUE`.
- **Operators/punctuation** (each its own Kind): `? ?? | & == != === !== @ @@ < > <= >= .. + - * / % ** ^ ~ . # ( ) [ ] -> .? , :: $ = { } ;`
- **Keywords** — union of KerML.xtext + SysML.xtext lowercase literals (~200). Recognized by scanning an ID then looking up `keywords`. Contextual keywords (`end`, `variation`, `individual`, etc.) are STILL emitted as their keyword Kind here is acceptable ONLY if parser disambiguates; per spec the lexer emits them as identifiers — SEE Task 7 decision: lexer looks up keyword map and emits keyword Kind, EXCEPT the parser treats a documented contextual-keyword subset as usable identifiers. For Plan 01 the lexer emits a keyword Kind for every keyword-map hit; contextual handling is a parser concern (Plan 02).
- **Longest-match / ordering rules:** `//*` before `//`; `->` and `.?` and `::` and `**` and `==`/`===`/`!==`/`!=`/`<=`/`>=`/`??`/`@@` multi-char operators before their single-char prefixes; `.` before `..` resolved by longest match (`..` wins when two dots).

---

### Task 1: Module bootstrap

**Files:**
- Create: `go.mod`

- [ ] **Step 1: Initialize the module**

Run: `go mod init github.com/Open-MBEE/Systemica`
Expected: creates `go.mod` containing `module github.com/Open-MBEE/Systemica` and a `go 1.25` line.

- [ ] **Step 2: Verify toolchain builds**

Run: `go build ./...`
Expected: exits 0, no output (no packages yet).

- [ ] **Step 3: Commit**

```bash
git init
git add go.mod
git commit -m "chore: initialize go module"
```

Note: repo is not yet a git repo; `git init` here establishes it. If the user prefers no git, skip the commit steps throughout but keep all other steps.

### Task 2: Source package — SourceFile & spans

**Files:**
- Create: `internal/core/source/source.go`
- Test: `internal/core/source/source_test.go`

- [ ] **Step 1: Write the failing test**

```go
package source

import "testing"

func TestSpanText(t *testing.T) {
	sf := New("test.sysml", []byte("part def Engine;"))
	sp := Span{Offset: 9, Len: 6}
	if got := sf.Text(sp); got != "Engine" {
		t.Fatalf("Text(%v) = %q, want %q", sp, got, "Engine")
	}
	if sf.Name() != "test.sysml" {
		t.Fatalf("Name() = %q, want %q", sf.Name(), "test.sysml")
	}
	if sf.Len() != 16 {
		t.Fatalf("Len() = %d, want 16", sf.Len())
	}
}

func TestSpanEnd(t *testing.T) {
	sp := Span{Offset: 4, Len: 3}
	if sp.End() != 7 {
		t.Fatalf("End() = %d, want 7", sp.End())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/source/ -run TestSpan -v`
Expected: FAIL — `undefined: New`, `undefined: Span`.

- [ ] **Step 3: Write minimal implementation**

```go
// Package source owns file content and byte-offset span types.
package source

// Span is a byte range within a SourceFile: [Offset, Offset+Len).
type Span struct {
	Offset int
	Len    int
}

// End returns the exclusive end offset.
func (s Span) End() int { return s.Offset + s.Len }

// Pos is a 1-based line/column location.
type Pos struct {
	Line int
	Col  int
}

// SourceFile owns the raw bytes of one source file.
type SourceFile struct {
	name    string
	content []byte
}

// New creates a SourceFile from a name and its raw bytes.
func New(name string, content []byte) *SourceFile {
	return &SourceFile{name: name, content: content}
}

// Name returns the file name.
func (sf *SourceFile) Name() string { return sf.name }

// Len returns the byte length of the content.
func (sf *SourceFile) Len() int { return len(sf.content) }

// Bytes returns the raw content (do not mutate).
func (sf *SourceFile) Bytes() []byte { return sf.content }

// Text returns the substring covered by the span.
func (sf *SourceFile) Text(sp Span) string {
	return string(sf.content[sp.Offset:sp.End()])
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/source/ -run TestSpan -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/source/source.go internal/core/source/source_test.go
git commit -m "feat(source): add SourceFile and Span types"
```

### Task 3: Source package — line index (line/col mapping)

**Files:**
- Create: `internal/core/source/lineindex.go`
- Test: `internal/core/source/source_test.go` (add cases)

- [ ] **Step 1: Write the failing test**

```go
func TestLineIndex(t *testing.T) {
	// bytes:  p a r t \n d e f  \n E
	// offset: 0 1 2 3 4  5 6 7  8  9
	sf := New("t.sysml", []byte("part\ndef\nE"))
	li := sf.Lines()
	cases := []struct {
		offset    int
		line, col int
	}{
		{0, 1, 1},  // 'p'
		{3, 1, 4},  // 't'
		{4, 1, 5},  // '\n' belongs to line 1
		{5, 2, 1},  // 'd'
		{9, 3, 1},  // 'E' (offset 8 is the second '\n', belongs to line 2)
	}
	for _, c := range cases {
		got := li.PosAt(c.offset)
		if got.Line != c.line || got.Col != c.col {
			t.Errorf("PosAt(%d) = %+v, want Line %d Col %d", c.offset, got, c.line, c.col)
		}
	}
}

func TestLineIndexOffsetAt(t *testing.T) {
	sf := New("t.sysml", []byte("part\ndef\nE"))
	li := sf.Lines()
	if got := li.OffsetAt(Pos{Line: 2, Col: 1}); got != 5 {
		t.Fatalf("OffsetAt(2,1) = %d, want 5", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/source/ -run TestLineIndex -v`
Expected: FAIL — `sf.Lines undefined`.

- [ ] **Step 3: Write minimal implementation**

```go
package source

// LineIndex maps byte offsets to 1-based line/column positions.
// lineStarts[i] is the byte offset where line (i+1) begins.
type LineIndex struct {
	content    []byte
	lineStarts []int
}

func newLineIndex(content []byte) *LineIndex {
	starts := []int{0}
	for i, b := range content {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &LineIndex{content: content, lineStarts: starts}
}

// PosAt returns the 1-based line/col for a byte offset.
// Col counts bytes from the line start + 1.
func (li *LineIndex) PosAt(offset int) Pos {
	// binary search for the greatest lineStart <= offset
	lo, hi := 0, len(li.lineStarts)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if li.lineStarts[mid] <= offset {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return Pos{Line: lo + 1, Col: offset - li.lineStarts[lo] + 1}
}

// OffsetAt returns the byte offset for a 1-based line/col.
func (li *LineIndex) OffsetAt(p Pos) int {
	if p.Line < 1 || p.Line > len(li.lineStarts) {
		return -1
	}
	return li.lineStarts[p.Line-1] + (p.Col - 1)
}
```

Add to `source.go`:

```go
// Lines returns a LineIndex for this file (recomputed each call; cache at call site if hot).
func (sf *SourceFile) Lines() *LineIndex {
	return newLineIndex(sf.content)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/source/ -run TestLineIndex -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/source/lineindex.go internal/core/source/source.go internal/core/source/source_test.go
git commit -m "feat(source): add LineIndex offset<->line/col mapping"
```

Note on Col semantics: Col is a byte column (1-based). LSP requires UTF-16 code-unit columns; that conversion is an LSP-layer concern (Plan 06), not the source package. Document this in a comment so the boundary is explicit.

### Task 4: Lexer — token kinds & Token type

**Files:**
- Create: `internal/core/lexer/token.go`
- Test: `internal/core/lexer/token_test.go`

- [ ] **Step 1: Write the failing test**

```go
package lexer

import "testing"

func TestKindString(t *testing.T) {
	cases := map[Kind]string{
		EOF:        "EOF",
		Identifier: "Identifier",
		Decimal:    "Decimal",
		ColonColon: "::",
		LBrace:     "{",
		Error:      "Error",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", k, got, want)
		}
	}
}

func TestTokenIsTrivia(t *testing.T) {
	if !(Token{Kind: Whitespace}).IsTrivia() {
		t.Error("Whitespace should be trivia")
	}
	if !(Token{Kind: SLNote}).IsTrivia() {
		t.Error("SLNote should be trivia")
	}
	if (Token{Kind: Identifier}).IsTrivia() {
		t.Error("Identifier should not be trivia")
	}
	// REGULAR_COMMENT is NOT hidden trivia.
	if (Token{Kind: RegularComment}).IsTrivia() {
		t.Error("RegularComment is not hidden trivia")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/lexer/ -run TestKind -v`
Expected: FAIL — undefined `Kind`, `EOF`, etc.

- [ ] **Step 3: Write minimal implementation**

```go
// Package lexer is a hand-written pull-based scanner for SysML v2 / KerML.
package lexer

import "github.com/Open-MBEE/Systemica/internal/core/source"

// Kind enumerates token categories.
type Kind int

const (
	Invalid Kind = iota
	EOF

	// Literals & names
	Identifier      // ID: [A-Za-z_][A-Za-z_0-9]*
	UnrestrictedName // '...'
	String          // "..."
	Decimal         // [0-9]+
	Real            // 1.5, .5, 1e3, 1.5e-2

	// Trivia
	Whitespace     // WS (hidden)
	SLNote         // // ...   (hidden)
	MLNote         // //* */   (hidden)
	RegularComment // /* */    (NOT hidden)

	// Keyword sentinel — actual keyword kinds are >= firstKeyword.
	// (Assigned in keywords.go via a contiguous block.)

	// Punctuation / operators
	Question     // ?
	QuestionQ    // ??
	Pipe         // |
	Amp          // &
	EqEq         // ==
	NotEq        // !=
	EqEqEq       // ===
	NotEqEq      // !==
	At           // @
	AtAt         // @@
	Lt           // <
	Gt           // >
	Le           // <=
	Ge           // >=
	DotDot       // ..
	Plus         // +
	Minus        // -
	Star         // *
	Slash        // /
	Percent      // %
	StarStar     // **
	Caret        // ^
	Tilde        // ~
	Dot          // .
	Hash         // #
	LParen       // (
	RParen       // )
	LBracket     // [
	RBracket     // ]
	Arrow        // ->
	DotQuestion  // .?
	Comma        // ,
	ColonColon   // ::
	Dollar       // $
	Eq           // =
	LBrace       // {
	RBrace       // }
	Semicolon    // ;
	Colon        // :

	Keyword // generic keyword marker; specific keyword identity via Token.Text

	Error // illegal char / unterminated literal
)

var kindNames = map[Kind]string{
	Invalid: "Invalid", EOF: "EOF",
	Identifier: "Identifier", UnrestrictedName: "UnrestrictedName",
	String: "String", Decimal: "Decimal", Real: "Real",
	Whitespace: "Whitespace", SLNote: "SLNote", MLNote: "MLNote",
	RegularComment: "RegularComment",
	Question: "?", QuestionQ: "??", Pipe: "|", Amp: "&",
	EqEq: "==", NotEq: "!=", EqEqEq: "===", NotEqEq: "!==",
	At: "@", AtAt: "@@", Lt: "<", Gt: ">", Le: "<=", Ge: ">=",
	DotDot: "..", Plus: "+", Minus: "-", Star: "*", Slash: "/",
	Percent: "%", StarStar: "**", Caret: "^", Tilde: "~", Dot: ".",
	Hash: "#", LParen: "(", RParen: ")", LBracket: "[", RBracket: "]",
	Arrow: "->", DotQuestion: ".?", Comma: ",", ColonColon: "::",
	Dollar: "$", Eq: "=", LBrace: "{", RBrace: "}", Semicolon: ";", Colon: ":",
	Keyword: "Keyword", Error: "Error",
}

// String returns a human-readable name for the kind.
func (k Kind) String() string {
	if s, ok := kindNames[k]; ok {
		return s
	}
	return "Kind(" + itoa(k) + ")"
}

func itoa(k Kind) string {
	// tiny local itoa to avoid strconv import churn; fine for error paths
	if k == 0 {
		return "0"
	}
	neg := k < 0
	n := int(k)
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// Token is a single lexeme: a Kind plus its source span.
// KeywordID is set only when Kind == Keyword, giving the specific keyword text.
type Token struct {
	Kind      Kind
	Span      source.Span
	KeywordID string // populated for Kind==Keyword
}

// IsTrivia reports whether the token is hidden trivia (skipped by the parser).
// WS, SLNote, MLNote are hidden. RegularComment is NOT hidden.
func (t Token) IsTrivia() bool {
	switch t.Kind {
	case Whitespace, SLNote, MLNote:
		return true
	default:
		return false
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/lexer/ -run "TestKind|TestToken" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/lexer/token.go internal/core/lexer/token_test.go
git commit -m "feat(lexer): add Kind enum and Token type"
```

### Task 5: Lexer — scanner skeleton & pull API

**Files:**
- Create: `internal/core/lexer/lexer.go`
- Test: `internal/core/lexer/lexer_test.go`

- [ ] **Step 1: Write the failing test**

```go
package lexer

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// lex is a test helper: collects all tokens including trivia until EOF.
func lex(t *testing.T, input string) []Token {
	t.Helper()
	sf := source.New("t.sysml", []byte(input))
	lx := New(sf)
	var toks []Token
	for {
		tok := lx.Next()
		toks = append(toks, tok)
		if tok.Kind == EOF {
			return toks
		}
		if len(toks) > 10000 {
			t.Fatal("lexer did not terminate")
		}
	}
}

func TestEmptyInputYieldsEOF(t *testing.T) {
	toks := lex(t, "")
	if len(toks) != 1 || toks[0].Kind != EOF {
		t.Fatalf("empty input toks = %v, want single EOF", toks)
	}
	if toks[0].Span.Offset != 0 {
		t.Fatalf("EOF offset = %d, want 0", toks[0].Span.Offset)
	}
}

func TestEOFIsIdempotent(t *testing.T) {
	sf := source.New("t.sysml", []byte(""))
	lx := New(sf)
	_ = lx.Next()
	if k := lx.Next().Kind; k != EOF {
		t.Fatalf("second Next() after EOF = %v, want EOF", k)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/lexer/ -run "TestEmpty|TestEOF" -v`
Expected: FAIL — `undefined: New`.

- [ ] **Step 3: Write minimal implementation**

```go
package lexer

import "github.com/Open-MBEE/Systemica/internal/core/source"

// Lexer is a hand-written, pull-based scanner. Call Next() repeatedly.
type Lexer struct {
	sf    *source.SourceFile
	src   []byte
	pos   int // current byte offset
	atEOF bool
}

// New creates a Lexer over a source file.
func New(sf *source.SourceFile) *Lexer {
	return &Lexer{sf: sf, src: sf.Bytes()}
}

// Next returns the next token, including trivia tokens. At end of input it
// returns EOF repeatedly (idempotent).
func (lx *Lexer) Next() Token {
	if lx.pos >= len(lx.src) {
		lx.atEOF = true
		return Token{Kind: EOF, Span: source.Span{Offset: len(lx.src), Len: 0}}
	}
	start := lx.pos
	c := lx.src[lx.pos]
	_ = c
	// Dispatch is filled in subsequent tasks. For now, emit a single-byte
	// Error token so the loop always advances and terminates.
	lx.pos++
	return Token{Kind: Error, Span: source.Span{Offset: start, Len: 1}}
}

// peek returns the byte at pos+n without advancing, or 0 if out of range.
func (lx *Lexer) peek(n int) byte {
	i := lx.pos + n
	if i >= len(lx.src) {
		return 0
	}
	return lx.src[i]
}

// span builds a Span from a start offset to the current pos.
func (lx *Lexer) span(start int) source.Span {
	return source.Span{Offset: start, Len: lx.pos - start}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/lexer/ -run "TestEmpty|TestEOF" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/lexer/lexer.go internal/core/lexer/lexer_test.go
git commit -m "feat(lexer): add scanner skeleton and pull-based Next API"
```

### Task 6: Lexer — whitespace & trivia (WS, notes, comments)

This task replaces the placeholder dispatch in `Next()` with a real dispatch that first handles whitespace, notes (`//*`, `//`), and block comments (`/* */`). Ordering matters: `//*` must be tried before `//`.

**Files:**
- Modify: `internal/core/lexer/lexer.go`
- Test: `internal/core/lexer/lexer_test.go` (add cases)

- [ ] **Step 1: Write the failing test**

```go
// kinds extracts just the Kind sequence for compact assertions.
func kinds(toks []Token) []Kind {
	ks := make([]Kind, len(toks))
	for i, t := range toks {
		ks[i] = t.Kind
	}
	return ks
}

func eq(a, b []Kind) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestWhitespace(t *testing.T) {
	toks := lex(t, "   \t\n ")
	want := []Kind{Whitespace, EOF}
	if !eq(kinds(toks), want) {
		t.Fatalf("kinds = %v, want %v", kinds(toks), want)
	}
	if toks[0].Span.Len != 6 {
		t.Fatalf("ws span len = %d, want 6", toks[0].Span.Len)
	}
}

func TestSLNote(t *testing.T) {
	toks := lex(t, "// hello\n")
	if !eq(kinds(toks), []Kind{SLNote, EOF}) {
		t.Fatalf("kinds = %v", kinds(toks))
	}
}

func TestMLNoteBeatsSLNote(t *testing.T) {
	toks := lex(t, "//* note */")
	if !eq(kinds(toks), []Kind{MLNote, EOF}) {
		t.Fatalf("kinds = %v, want MLNote EOF", kinds(toks))
	}
}

func TestRegularComment(t *testing.T) {
	toks := lex(t, "/* c */")
	if !eq(kinds(toks), []Kind{RegularComment, EOF}) {
		t.Fatalf("kinds = %v, want RegularComment EOF", kinds(toks))
	}
}

func TestUnterminatedBlockComment(t *testing.T) {
	toks := lex(t, "/* open")
	// Unterminated block comment: consume to EOF, emit RegularComment, no error token needed for span coverage.
	if toks[0].Kind != RegularComment {
		t.Fatalf("first kind = %v, want RegularComment", toks[0].Kind)
	}
	if toks[0].Span.Len != 7 {
		t.Fatalf("span len = %d, want 7 (to EOF)", toks[0].Span.Len)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/lexer/ -run "TestWhitespace|TestSLNote|TestMLNote|TestRegular|TestUnterminatedBlock" -v`
Expected: FAIL — trivia currently lexed as Error tokens.

- [ ] **Step 3: Write minimal implementation**

Replace the body of `Next()` and add scan helpers:

```go
func (lx *Lexer) Next() Token {
	if lx.pos >= len(lx.src) {
		lx.atEOF = true
		return Token{Kind: EOF, Span: source.Span{Offset: len(lx.src), Len: 0}}
	}
	start := lx.pos
	c := lx.src[lx.pos]

	switch {
	case c == ' ' || c == '\t' || c == '\r' || c == '\n':
		return lx.scanWhitespace(start)
	case c == '/' && lx.peek(1) == '/' && lx.peek(2) == '*':
		return lx.scanMLNote(start) // //* ... */
	case c == '/' && lx.peek(1) == '/':
		return lx.scanSLNote(start) // // ...
	case c == '/' && lx.peek(1) == '*':
		return lx.scanBlockComment(start) // /* ... */
	}

	// Not trivia: emit a single-byte Error for now; later tasks add cases
	// BEFORE this fallthrough point.
	lx.pos++
	return Token{Kind: Error, Span: source.Span{Offset: start, Len: 1}}
}

func (lx *Lexer) scanWhitespace(start int) Token {
	for lx.pos < len(lx.src) {
		c := lx.src[lx.pos]
		if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
			break
		}
		lx.pos++
	}
	return Token{Kind: Whitespace, Span: lx.span(start)}
}

func (lx *Lexer) scanSLNote(start int) Token {
	lx.pos += 2 // consume "//"
	for lx.pos < len(lx.src) && lx.src[lx.pos] != '\n' && lx.src[lx.pos] != '\r' {
		lx.pos++
	}
	// include the line terminator (\r?\n) in the note span, per SL_NOTE rule
	if lx.pos < len(lx.src) && lx.src[lx.pos] == '\r' {
		lx.pos++
	}
	if lx.pos < len(lx.src) && lx.src[lx.pos] == '\n' {
		lx.pos++
	}
	return Token{Kind: SLNote, Span: lx.span(start)}
}

func (lx *Lexer) scanMLNote(start int) Token {
	lx.pos += 3 // consume "//*"
	lx.consumeUntilStarSlash()
	return Token{Kind: MLNote, Span: lx.span(start)}
}

func (lx *Lexer) scanBlockComment(start int) Token {
	lx.pos += 2 // consume "/*"
	lx.consumeUntilStarSlash()
	return Token{Kind: RegularComment, Span: lx.span(start)}
}

// consumeUntilStarSlash advances until it consumes a closing "*/", or to EOF
// if unterminated.
func (lx *Lexer) consumeUntilStarSlash() {
	for lx.pos < len(lx.src) {
		if lx.src[lx.pos] == '*' && lx.peek(1) == '/' {
			lx.pos += 2
			return
		}
		lx.pos++
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/lexer/ -run "TestWhitespace|TestSLNote|TestMLNote|TestRegular|TestUnterminatedBlock" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/lexer/lexer.go internal/core/lexer/lexer_test.go
git commit -m "feat(lexer): scan whitespace, notes, and block comments"
```

### Task 7: Lexer — identifiers & keyword map

Adds ID scanning and keyword recognition. After scanning an ID, look it up in the keyword map; on hit emit `Kind: Keyword` with `KeywordID` set, else `Identifier`.

**Decision (contextual keywords):** For Plan 01 the lexer emits `Keyword` for every keyword-map hit and stores the literal in `KeywordID`. The parser (Plan 02) decides when a contextual keyword (e.g. `end`, `variation`, `individual`) is usable as an identifier by inspecting `KeywordID`. This keeps the lexer dumb and fast per the spec.

**Files:**
- Create: `internal/core/lexer/keywords.go`
- Modify: `internal/core/lexer/lexer.go`
- Test: `internal/core/lexer/lexer_test.go` (add cases)

- [ ] **Step 1: Write the failing test**

```go
func TestIdentifier(t *testing.T) {
	toks := lex(t, "Engine _x9 abc")
	want := []Kind{Identifier, Whitespace, Identifier, Whitespace, Identifier, EOF}
	if !eq(kinds(toks), want) {
		t.Fatalf("kinds = %v, want %v", kinds(toks), want)
	}
}

func TestKeyword(t *testing.T) {
	toks := lex(t, "part def package")
	// all three are keywords
	for i, ki := range []int{0, 2, 4} {
		if toks[ki].Kind != Keyword {
			t.Fatalf("token %d kind = %v, want Keyword", i, toks[ki].Kind)
		}
	}
	if toks[0].KeywordID != "part" {
		t.Fatalf("KeywordID = %q, want part", toks[0].KeywordID)
	}
}

func TestKeywordPrefixIsIdentifier(t *testing.T) {
	// "partial" is not a keyword even though it starts with "part"
	toks := lex(t, "partial")
	if toks[0].Kind != Identifier {
		t.Fatalf("kind = %v, want Identifier", toks[0].Kind)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/lexer/ -run "TestIdentifier|TestKeyword" -v`
Expected: FAIL — identifiers currently produce Error tokens.

- [ ] **Step 3: Write minimal implementation**

Create `keywords.go` with the union keyword set:

```go
package lexer

// keywords is the union of KerML and SysML lowercase keyword literals.
// A scanned ID matching a key is emitted as Kind==Keyword with KeywordID set.
var keywords = map[string]struct{}{}

func init() {
	for _, kw := range keywordList {
		keywords[kw] = struct{}{}
	}
}

var keywordList = []string{
	// KerML + SysML union (deduplicated). Contextual keywords included;
	// parser disambiguates identifier usage.
	"about", "abstract", "accept", "action", "actor", "after", "alias", "all",
	"allocate", "allocation", "analysis", "and", "as", "assert", "assign",
	"assoc", "assume", "at", "attribute", "behavior", "bind", "binding", "bool",
	"by", "calc", "case", "chains", "class", "classifier", "comment", "composite",
	"concern", "conjugate", "conjugates", "conjugation", "connect", "connection",
	"connector", "const", "constant", "constraint", "crosses", "datatype",
	"decide", "def", "default", "defined", "dependency", "derived", "differences",
	"disjoining", "disjoint", "do", "doc", "else", "end", "entry", "enum", "event",
	"exhibit", "exit", "expose", "expr", "false", "feature", "featured",
	"featuring", "filter", "first", "flow", "for", "fork", "frame", "from",
	"function", "hastype", "if", "implies", "import", "in", "include",
	"individual", "inout", "interaction", "interface", "intersects", "inv",
	"inverse", "inverting", "istype", "item", "join", "language", "library",
	"locale", "loop", "member", "merge", "message", "meta", "metaclass",
	"metadata", "multiplicity", "namespace", "new", "nonunique", "not", "null",
	"objective", "occurrence", "of", "or", "ordered", "out", "package", "parallel",
	"part", "perform", "port", "portion", "predicate", "private", "protected",
	"public", "redefines", "redefinition", "ref", "references", "render",
	"rendering", "rep", "require", "requirement", "return", "satisfy", "send",
	"snapshot", "specialization", "specializes", "stakeholder", "standard",
	"state", "step", "struct", "subclassifier", "subject", "subset", "subsets",
	"subtype", "succession", "terminate", "then", "timeslice", "to", "transition",
	"true", "type", "typed", "typing", "unions", "until", "use", "var", "variant",
	"variation", "verification", "verify", "via", "view", "viewpoint", "when",
	"while", "xor",
}
```

Add ID scanning to `lexer.go`. Insert this case in the `Next()` switch BEFORE the Error fallthrough:

```go
	case isIdentStart(c):
		return lx.scanIdentOrKeyword(start)
```

And add:

```go
func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentCont(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

func (lx *Lexer) scanIdentOrKeyword(start int) Token {
	lx.pos++ // first char already known to be identStart
	for lx.pos < len(lx.src) && isIdentCont(lx.src[lx.pos]) {
		lx.pos++
	}
	sp := lx.span(start)
	text := string(lx.src[start:lx.pos])
	if _, ok := keywords[text]; ok {
		return Token{Kind: Keyword, Span: sp, KeywordID: text}
	}
	return Token{Kind: Identifier, Span: sp}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/lexer/ -run "TestIdentifier|TestKeyword" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/lexer/keywords.go internal/core/lexer/lexer.go internal/core/lexer/lexer_test.go
git commit -m "feat(lexer): scan identifiers and recognize keywords"
```

### Task 8: Lexer — unrestricted names (single-quoted)

`'...'` with escape set `\b \t \n \f \r \" \' \\`. Body char = escape OR any char except `\` and `'`. Unterminated → Error token spanning to EOL/EOF.

**Files:**
- Modify: `internal/core/lexer/lexer.go`
- Test: `internal/core/lexer/lexer_test.go` (add cases)

- [ ] **Step 1: Write the failing test**

```go
func TestUnrestrictedName(t *testing.T) {
	toks := lex(t, "'my name'")
	if !eq(kinds(toks), []Kind{UnrestrictedName, EOF}) {
		t.Fatalf("kinds = %v", kinds(toks))
	}
	if toks[0].Span.Len != 9 {
		t.Fatalf("span len = %d, want 9", toks[0].Span.Len)
	}
}

func TestUnrestrictedNameWithEscape(t *testing.T) {
	toks := lex(t, `'a\'b'`)
	if !eq(kinds(toks), []Kind{UnrestrictedName, EOF}) {
		t.Fatalf("kinds = %v, want UnrestrictedName EOF", kinds(toks))
	}
	if toks[0].Span.Len != 6 {
		t.Fatalf("span len = %d, want 6", toks[0].Span.Len)
	}
}

func TestUnterminatedUnrestrictedName(t *testing.T) {
	toks := lex(t, "'open\n")
	if toks[0].Kind != Error {
		t.Fatalf("kind = %v, want Error", toks[0].Kind)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/lexer/ -run "TestUnrestricted|TestUnterminatedUnrestricted" -v`
Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**

Insert case in `Next()` switch before the Error fallthrough:

```go
	case c == '\'':
		return lx.scanQuoted(start, '\'', UnrestrictedName)
```

Add the shared quoted-literal scanner (used by strings too, Task 10):

```go
// scanQuoted scans a quoted literal delimited by quote (' or "), honoring the
// backslash escape set. On success emits kind; on unterminated (newline/EOF
// before closing quote) emits Error covering what was consumed.
func (lx *Lexer) scanQuoted(start int, quote byte, kind Kind) Token {
	lx.pos++ // opening quote
	for lx.pos < len(lx.src) {
		c := lx.src[lx.pos]
		switch {
		case c == '\\':
			// escape: consume backslash + next char if present
			lx.pos++
			if lx.pos < len(lx.src) {
				lx.pos++
			}
		case c == quote:
			lx.pos++ // closing quote
			return Token{Kind: kind, Span: lx.span(start)}
		case c == '\n' || c == '\r':
			// unterminated on this line
			return Token{Kind: Error, Span: lx.span(start)}
		default:
			lx.pos++
		}
	}
	// reached EOF without closing quote
	return Token{Kind: Error, Span: lx.span(start)}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/lexer/ -run "TestUnrestricted|TestUnterminatedUnrestricted" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/lexer/lexer.go internal/core/lexer/lexer_test.go
git commit -m "feat(lexer): scan single-quoted unrestricted names"
```

### Task 9: Lexer — numbers (decimal, real, exponent)

Grammar: `DECIMAL_VALUE = [0-9]+`; `EXP_VALUE = DECIMAL (e|E)(+|-)? DECIMAL`; `Real = DECIMAL? '.' (DECIMAL | EXP) | EXP`. A bare `[0-9]+` with no `.`/exponent is `Decimal`; anything with `.` or exponent is `Real`.

Note the `.` ambiguity: `1..2` is `Decimal DotDot Decimal` (range), NOT a real. So when scanning after digits, only treat `.` as part of a real if the char after `.` is a digit OR (the `.` is followed by an exponent). `1.` alone: grammar requires digits after `.` unless exponent — treat `1.` as `Decimal` then `Dot` (parser handles). To keep the lexer simple and match grammar: a `.` continues a real ONLY if followed by a digit. Leading-dot reals (`.5`) are scanned when `Next()` sees `.` followed by a digit (handled here, taking priority over the `Dot` operator in Task 11).

**Files:**
- Modify: `internal/core/lexer/lexer.go`
- Test: `internal/core/lexer/lexer_test.go` (add cases)

- [ ] **Step 1: Write the failing test**

```go
func TestDecimal(t *testing.T) {
	toks := lex(t, "42")
	if !eq(kinds(toks), []Kind{Decimal, EOF}) {
		t.Fatalf("kinds = %v", kinds(toks))
	}
}

func TestReal(t *testing.T) {
	for _, in := range []string{"1.5", ".5", "1e3", "1.5e-2", "2E+10", "1.0e5"} {
		toks := lex(t, in)
		if !eq(kinds(toks), []Kind{Real, EOF}) {
			t.Fatalf("input %q kinds = %v, want Real EOF", in, kinds(toks))
		}
	}
}

func TestRangeNotReal(t *testing.T) {
	// 1..2 must be Decimal DotDot Decimal, not a malformed real.
	toks := lex(t, "1..2")
	want := []Kind{Decimal, DotDot, Decimal, EOF}
	if !eq(kinds(toks), want) {
		t.Fatalf("kinds = %v, want %v", kinds(toks), want)
	}
}

func TestTrailingDot(t *testing.T) {
	// "1." → Decimal then Dot (no digit after dot)
	toks := lex(t, "1.")
	want := []Kind{Decimal, Dot, EOF}
	if !eq(kinds(toks), want) {
		t.Fatalf("kinds = %v, want %v", kinds(toks), want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/lexer/ -run "TestDecimal|TestReal|TestRange|TestTrailingDot" -v`
Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**

Insert cases in `Next()` switch before the Error fallthrough. The digit case handles decimals/reals; the leading-dot-real case must come before the `.`-operator handling added in Task 11 (so put this digit/dot number logic ahead of punctuation):

```go
	case c >= '0' && c <= '9':
		return lx.scanNumber(start)
	case c == '.' && lx.peek(1) >= '0' && lx.peek(1) <= '9':
		return lx.scanNumber(start) // leading-dot real: .5
```

Add:

```go
func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// scanNumber scans Decimal or Real starting at start.
func (lx *Lexer) scanNumber(start int) Token {
	isReal := false

	// leading-dot real (.5) — start is '.'
	if lx.src[lx.pos] == '.' {
		isReal = true
		lx.pos++ // '.'
		lx.consumeDigits()
		lx.consumeExponent(&isReal)
		return Token{Kind: Real, Span: lx.span(start)}
	}

	lx.consumeDigits() // integer part

	// fractional part: '.' only if followed by a digit (avoids 1..2 and 1.)
	if lx.pos < len(lx.src) && lx.src[lx.pos] == '.' &&
		lx.pos+1 < len(lx.src) && isDigit(lx.src[lx.pos+1]) {
		isReal = true
		lx.pos++ // '.'
		lx.consumeDigits()
	}

	lx.consumeExponent(&isReal)

	if isReal {
		return Token{Kind: Real, Span: lx.span(start)}
	}
	return Token{Kind: Decimal, Span: lx.span(start)}
}

func (lx *Lexer) consumeDigits() {
	for lx.pos < len(lx.src) && isDigit(lx.src[lx.pos]) {
		lx.pos++
	}
}

// consumeExponent consumes an (e|E)(+|-)?[0-9]+ suffix if present, setting real.
func (lx *Lexer) consumeExponent(isReal *bool) {
	if lx.pos >= len(lx.src) {
		return
	}
	c := lx.src[lx.pos]
	if c != 'e' && c != 'E' {
		return
	}
	// lookahead: need optional sign then at least one digit, else not an exponent
	i := lx.pos + 1
	if i < len(lx.src) && (lx.src[i] == '+' || lx.src[i] == '-') {
		i++
	}
	if i >= len(lx.src) || !isDigit(lx.src[i]) {
		return // not a valid exponent; leave 'e' for identifier/other handling
	}
	*isReal = true
	lx.pos = i
	lx.consumeDigits()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/lexer/ -run "TestDecimal|TestReal|TestRange|TestTrailingDot" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/lexer/lexer.go internal/core/lexer/lexer_test.go
git commit -m "feat(lexer): scan decimal, real, and exponent numbers"
```

### Task 10: Lexer — string literals

`"..."` double-quoted, same escape set as unrestricted names. Reuses `scanQuoted` from Task 8.

**Files:**
- Modify: `internal/core/lexer/lexer.go`
- Test: `internal/core/lexer/lexer_test.go` (add cases)

- [ ] **Step 1: Write the failing test**

```go
func TestString(t *testing.T) {
	toks := lex(t, `"hello world"`)
	if !eq(kinds(toks), []Kind{String, EOF}) {
		t.Fatalf("kinds = %v", kinds(toks))
	}
}

func TestStringWithEscape(t *testing.T) {
	toks := lex(t, `"a\"b\n"`)
	if !eq(kinds(toks), []Kind{String, EOF}) {
		t.Fatalf("kinds = %v, want String EOF", kinds(toks))
	}
}

func TestUnterminatedString(t *testing.T) {
	toks := lex(t, `"open`)
	if toks[0].Kind != Error {
		t.Fatalf("kind = %v, want Error", toks[0].Kind)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/lexer/ -run "TestString|TestUnterminatedString" -v`
Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**

Insert case in `Next()` switch before the Error fallthrough:

```go
	case c == '"':
		return lx.scanQuoted(start, '"', String)
```

No new helper needed — `scanQuoted` already handles the double-quote via its `quote` parameter.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/lexer/ -run "TestString|TestUnterminatedString" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/lexer/lexer.go internal/core/lexer/lexer_test.go
git commit -m "feat(lexer): scan double-quoted string literals"
```

### Task 11: Lexer — operators & punctuation

All remaining single/multi-char operators, using longest-match. Multi-char tokens must be tried before their single-char prefixes. Note `.` here is only reached when NOT a leading-dot real (Task 9 case precedes it), and `//`-prefixed forms are already consumed as notes (Task 6). `/` reaching here is the division `Slash`.

**Files:**
- Modify: `internal/core/lexer/lexer.go`
- Test: `internal/core/lexer/lexer_test.go` (add cases)

- [ ] **Step 1: Write the failing test**

```go
func TestOperators(t *testing.T) {
	cases := []struct {
		in   string
		want []Kind
	}{
		{"::", []Kind{ColonColon, EOF}},
		{":", []Kind{Colon, EOF}},
		{"->", []Kind{Arrow, EOF}},
		{".?", []Kind{DotQuestion, EOF}},
		{"..", []Kind{DotDot, EOF}},
		{".", []Kind{Dot, EOF}},
		{"**", []Kind{StarStar, EOF}},
		{"*", []Kind{Star, EOF}},
		{"==", []Kind{EqEq, EOF}},
		{"===", []Kind{EqEqEq, EOF}},
		{"!=", []Kind{NotEq, EOF}},
		{"!==", []Kind{NotEqEq, EOF}},
		{"<=", []Kind{Le, EOF}},
		{">=", []Kind{Ge, EOF}},
		{"??", []Kind{QuestionQ, EOF}},
		{"?", []Kind{Question, EOF}},
		{"@@", []Kind{AtAt, EOF}},
		{"@", []Kind{At, EOF}},
		{"|&+-%^~#()[]{},$=;<>", []Kind{
			Pipe, Amp, Plus, Minus, Percent, Caret, Tilde, Hash,
			LParen, RParen, LBracket, RBracket, LBrace, RBrace, Comma,
			Dollar, Eq, Semicolon, Lt, Gt, EOF,
		}},
	}
	for _, c := range cases {
		toks := lex(t, c.in)
		if !eq(kinds(toks), c.want) {
			t.Errorf("input %q kinds = %v, want %v", c.in, kinds(toks), c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/lexer/ -run TestOperators -v`
Expected: FAIL — operators currently produce Error tokens.

- [ ] **Step 3: Write minimal implementation**

Replace the Error fallthrough at the end of `Next()` with the operator dispatch, then a final Error for anything unrecognized:

```go
	// Operators & punctuation (longest match first).
	switch c {
	case ':':
		if lx.peek(1) == ':' {
			lx.pos += 2
			return Token{Kind: ColonColon, Span: lx.span(start)}
		}
		lx.pos++
		return Token{Kind: Colon, Span: lx.span(start)}
	case '-':
		if lx.peek(1) == '>' {
			lx.pos += 2
			return Token{Kind: Arrow, Span: lx.span(start)}
		}
		lx.pos++
		return Token{Kind: Minus, Span: lx.span(start)}
	case '.':
		if lx.peek(1) == '?' {
			lx.pos += 2
			return Token{Kind: DotQuestion, Span: lx.span(start)}
		}
		if lx.peek(1) == '.' {
			lx.pos += 2
			return Token{Kind: DotDot, Span: lx.span(start)}
		}
		lx.pos++
		return Token{Kind: Dot, Span: lx.span(start)}
	case '*':
		if lx.peek(1) == '*' {
			lx.pos += 2
			return Token{Kind: StarStar, Span: lx.span(start)}
		}
		lx.pos++
		return Token{Kind: Star, Span: lx.span(start)}
	case '=':
		if lx.peek(1) == '=' && lx.peek(2) == '=' {
			lx.pos += 3
			return Token{Kind: EqEqEq, Span: lx.span(start)}
		}
		if lx.peek(1) == '=' {
			lx.pos += 2
			return Token{Kind: EqEq, Span: lx.span(start)}
		}
		lx.pos++
		return Token{Kind: Eq, Span: lx.span(start)}
	case '!':
		if lx.peek(1) == '=' && lx.peek(2) == '=' {
			lx.pos += 3
			return Token{Kind: NotEqEq, Span: lx.span(start)}
		}
		if lx.peek(1) == '=' {
			lx.pos += 2
			return Token{Kind: NotEq, Span: lx.span(start)}
		}
		// bare '!' is not a defined token
		lx.pos++
		return Token{Kind: Error, Span: lx.span(start)}
	case '<':
		if lx.peek(1) == '=' {
			lx.pos += 2
			return Token{Kind: Le, Span: lx.span(start)}
		}
		lx.pos++
		return Token{Kind: Lt, Span: lx.span(start)}
	case '>':
		if lx.peek(1) == '=' {
			lx.pos += 2
			return Token{Kind: Ge, Span: lx.span(start)}
		}
		lx.pos++
		return Token{Kind: Gt, Span: lx.span(start)}
	case '?':
		if lx.peek(1) == '?' {
			lx.pos += 2
			return Token{Kind: QuestionQ, Span: lx.span(start)}
		}
		lx.pos++
		return Token{Kind: Question, Span: lx.span(start)}
	case '@':
		if lx.peek(1) == '@' {
			lx.pos += 2
			return Token{Kind: AtAt, Span: lx.span(start)}
		}
		lx.pos++
		return Token{Kind: At, Span: lx.span(start)}
	case '/':
		// // and /* already handled as trivia in Task 6; this is division
		lx.pos++
		return Token{Kind: Slash, Span: lx.span(start)}
	}

	// Single-char punctuation with a fixed 1:1 mapping.
	if k, ok := singleCharKind[c]; ok {
		lx.pos++
		return Token{Kind: k, Span: lx.span(start)}
	}

	// Unrecognized byte → Error, advance one byte to guarantee progress.
	lx.pos++
	return Token{Kind: Error, Span: lx.span(start)}
```

Add the single-char table:

```go
var singleCharKind = map[byte]Kind{
	'|': Pipe, '&': Amp, '+': Plus, '%': Percent, '^': Caret, '~': Tilde,
	'#': Hash, '(': LParen, ')': RParen, '[': LBracket, ']': RBracket,
	'{': LBrace, '}': RBrace, ',': Comma, '$': Dollar, ';': Semicolon,
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/lexer/ -run TestOperators -v`
Expected: PASS. Then run the whole package: `go test ./internal/core/lexer/ -v` — all prior tests still pass.

- [ ] **Step 5: Commit**

```bash
git add internal/core/lexer/lexer.go internal/core/lexer/lexer_test.go
git commit -m "feat(lexer): scan operators and punctuation with longest-match"
```

### Task 12: Lexer — error tokens & recovery

The Error token path already exists (unrecognized byte, unterminated quote). This task hardens the guarantees the parser depends on: (1) the lexer ALWAYS makes progress (never returns a zero-width non-EOF token), and (2) an unrecognized run is coalesced into a single Error token rather than one Error per byte, to reduce diagnostic noise.

**Files:**
- Modify: `internal/core/lexer/lexer.go`
- Test: `internal/core/lexer/lexer_test.go` (add cases)

- [ ] **Step 1: Write the failing test**

```go
func TestErrorCoalescing(t *testing.T) {
	// A run of unrecognized bytes becomes ONE Error token.
	toks := lex(t, "\x00\x01\x02")
	if !eq(kinds(toks), []Kind{Error, EOF}) {
		t.Fatalf("kinds = %v, want Error EOF", kinds(toks))
	}
	if toks[0].Span.Len != 3 {
		t.Fatalf("error span len = %d, want 3", toks[0].Span.Len)
	}
}

func TestErrorThenValid(t *testing.T) {
	toks := lex(t, "\x00part")
	want := []Kind{Error, Keyword, EOF}
	if !eq(kinds(toks), want) {
		t.Fatalf("kinds = %v, want %v", kinds(toks), want)
	}
}

func TestAlwaysMakesProgress(t *testing.T) {
	// Every non-EOF token must have Len >= 1 so parsers can't loop forever.
	toks := lex(t, "part\x00'unterminated")
	for _, tk := range toks {
		if tk.Kind != EOF && tk.Span.Len < 1 {
			t.Fatalf("token %v has zero width", tk)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/lexer/ -run "TestErrorCoalescing|TestErrorThenValid|TestAlwaysMakesProgress" -v`
Expected: FAIL on `TestErrorCoalescing` (currently one Error per byte).

- [ ] **Step 3: Write minimal implementation**

Replace the final unrecognized-byte fallthrough (the last two lines of `Next()`) with a coalescing scan:

```go
	// Unrecognized byte(s): coalesce a maximal run of bytes that cannot start
	// any known token into a single Error token. Guarantees progress.
	return lx.scanError(start)
```

Add:

```go
// scanError consumes a maximal run of bytes that cannot begin a valid token,
// emitting a single Error token. Always advances at least one byte.
func (lx *Lexer) scanError(start int) Token {
	lx.pos++ // consume the offending byte (guarantees progress)
	for lx.pos < len(lx.src) && !canStartToken(lx.src[lx.pos]) {
		lx.pos++
	}
	return Token{Kind: Error, Span: lx.span(start)}
}

// canStartToken reports whether c can begin some valid token. Used only to
// bound Error coalescing; conservative (false positives just end the run early).
func canStartToken(c byte) bool {
	switch {
	case c == ' ' || c == '\t' || c == '\r' || c == '\n':
		return true
	case isIdentStart(c):
		return true
	case isDigit(c):
		return true
	case c == '\'' || c == '"':
		return true
	}
	switch c {
	case ':', '-', '.', '*', '=', '!', '<', '>', '?', '@', '/',
		'|', '&', '+', '%', '^', '~', '#', '(', ')', '[', ']',
		'{', '}', ',', '$', ';':
		return true
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/lexer/ -run "TestErrorCoalescing|TestErrorThenValid|TestAlwaysMakesProgress" -v`
Expected: PASS. Full package: `go test ./internal/core/lexer/ -v`.

- [ ] **Step 5: Commit**

```bash
git add internal/core/lexer/lexer.go internal/core/lexer/lexer_test.go
git commit -m "feat(lexer): coalesce unrecognized bytes into single error tokens"
```

### Task 13: Integration — tokenize sample fixtures

End-to-end: tokenize real SysML/KerML snippets, assert no unexpected Error tokens and that a round-trip (concatenating every token's span text, including trivia) reproduces the exact input.

**Files:**
- Create: `testdata/lex/basic.sysml`
- Create: `testdata/lex/basic.kerml`
- Test: `internal/core/lexer/integration_test.go`

- [ ] **Step 1: Create fixtures**

`testdata/lex/basic.sysml`:

```sysml
package Vehicles {
    // a comment
    part def Vehicle {
        attribute mass : Real;
        part engine : Engine[1];
    }
    part def Engine;
}
```

`testdata/lex/basic.kerml`:

```kerml
package P {
    class A;
    feature x : A;
    /* block comment */
    feature 'quoted name' : A;
}
```

- [ ] **Step 2: Write the failing test**

```go
package lexer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func tokenizeFile(t *testing.T, path string) (*source.SourceFile, []Token) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sf := source.New(filepath.Base(path), data)
	lx := New(sf)
	var toks []Token
	for {
		tk := lx.Next()
		toks = append(toks, tk)
		if tk.Kind == EOF {
			break
		}
	}
	return sf, toks
}

func TestFixturesNoErrors(t *testing.T) {
	for _, f := range []string{"../../../testdata/lex/basic.sysml", "../../../testdata/lex/basic.kerml"} {
		_, toks := tokenizeFile(t, f)
		for _, tk := range toks {
			if tk.Kind == Error {
				t.Errorf("%s: unexpected Error token at offset %d", f, tk.Span.Offset)
			}
		}
	}
}

func TestFixturesRoundTrip(t *testing.T) {
	for _, f := range []string{"../../../testdata/lex/basic.sysml", "../../../testdata/lex/basic.kerml"} {
		sf, toks := tokenizeFile(t, f)
		var rebuilt []byte
		for _, tk := range toks {
			if tk.Kind == EOF {
				continue
			}
			rebuilt = append(rebuilt, sf.Bytes()[tk.Span.Offset:tk.Span.End()]...)
		}
		if string(rebuilt) != string(sf.Bytes()) {
			t.Errorf("%s: round-trip mismatch", f)
		}
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/core/lexer/ -run TestFixtures -v`
Expected: FAIL initially only if fixture paths wrong; otherwise these validate the full lexer. (If it passes immediately, that's acceptable — it's an integration guard, not TDD-of-new-code.)

- [ ] **Step 4: Run full suite + vet**

Run: `go test ./... && go vet ./...`
Expected: all PASS, vet clean.

- [ ] **Step 5: Commit**

```bash
git add testdata/lex internal/core/lexer/integration_test.go
git commit -m "test(lexer): add fixture tokenization and round-trip tests"
```

---

## Self-Review

Run after all tasks complete. This is a checklist, not a subagent dispatch.

**1. Spec coverage (section 5 Lexer + relevant section 14 Testing):**
- Hidden trivia WS/ML_NOTE/SL_NOTE tracked with spans — Task 6. ✓
- REGULAR_COMMENT not hidden — Task 4 `IsTrivia`, Task 6 scan. ✓
- ASCII-only ID + keyword map — Task 7. ✓
- Single-quoted unrestricted names w/ escapes — Task 8. ✓
- Double-quoted strings w/ escapes — Task 10. ✓
- Decimal/real/exponent numbers, `..` range disambiguation — Task 9. ✓
- Full operator/punctuation set, longest-match ordering — Task 11. ✓
- Illegal char → Error, lexer continues, always progresses — Task 12. ✓
- Byte offset+len spans, line/col via line index — Tasks 2,3. ✓
- Pull-based lazy `Next()` API — Task 5. ✓
- Table-driven tests + fixture integration + round-trip — Tasks throughout + 13. ✓

**2. Placeholder scan:** every code step contains complete code; no TBD/TODO. Confirm no `<!-- FILL -->` remains in the document.

**3. Type consistency:** `Kind`, `Token{Kind,Span,KeywordID}`, `source.Span{Offset,Len}`, `source.Pos{Line,Col}`, `Lexer.Next()`, helpers `peek/span/consumeDigits/consumeExponent/scanQuoted/scanError/canStartToken/isIdentStart/isIdentCont/isDigit` — names used identically across Tasks 2–13. `scanQuoted(start, quote, kind)` signature consistent between Tasks 8 and 10. ✓

**Known deferrals (correct for Plan 01, handled later):**
- Contextual-keyword-as-identifier disambiguation → parser, Plan 02.
- UTF-16 column conversion for LSP → Plan 06.
- `LineIndex` recomputed per `Lines()` call; caching deferred until a hot path needs it (Workspace, Plan 05).
