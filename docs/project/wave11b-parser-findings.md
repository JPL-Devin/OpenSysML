# Wave 11B — the parsing rows, and the four that are not parser rows

Wave 11B owns the 20 Xpect disagreements in the pilot's parsing suites that were attributed to the
lexer and the parser. Two of them were parser defects and are fixed; the rest are not parser
defects, and this page records what each one actually needs, so that a later reader does not look
for the fix in `internal/core/parser`.

Measured on a fresh run of `go run ./cmd/pilot-xpect` before and after the change, with the pinned
`PILOT_TAG=2026-05` corpus.

---

## Fixed — two grammar gaps in the parser

### `ParsingTests_MultiplicityDeclaration.kerml.xt` — the `MultiplicitySubset` form

`Multiplicity` has two alternatives, and we admitted only one (`KerML.xtext:750-760`):

```
Multiplicity returns SysML::Multiplicity :
	MultiplicitySubset | MultiplicityRange
;
MultiplicitySubset returns SysML::Multiplicity :
	'multiplicity' Identification? Subsets TypeBody
;
MultiplicityRange returns SysML::MultiplicityRange :
	'multiplicity' Identification? MultiplicityBounds TypeBody
;
```

`Subsets` is `(':>' | 'subsets') OwnedSubsetting`, so a named multiplicity may state its bounds by
subsetting another multiplicity instead of writing them: `multiplicity m subsets zeroOrMore;`. We
parsed the bounds form only and rejected the subsetting form with `expected '{' or ';'`.
`ast.MultiplicityDecl` now carries the subsetted name alongside the optional range, the two being
the exclusive alternatives of the production. The row closes (`noErrors`).

### `ParsingTests_Indexing.kerml.xt` — an index is a `SequenceExpression`

Both index forms take a *sequence*, not a single expression
(`KerMLExpressions.xtext`, `PrimaryExpression`):

```
'#' '(' operand += SequenceExpression ')'
'[' operand += SequenceExpression ']'
```

We parsed the operand with `ParseExpression`, which stops at the comma, so every
multi-dimensional index (`arr#(1,3)`) produced `expected ')'`. Six of the fixture's seven errors
were this defect and are gone. **The row does not close**, for the reason in the next section.

---

## Not parser rows — what the remaining 18 need

### `ParsingTests_Indexing.kerml.xt`:12 — a feature typed only by its value

```kerml
feature a : A[*];
feature b = a#(1).b;
feature c = b.c;      // ours: unresolved member: c
```

`b` declares no type; its type is the result type of its value expression. Resolving `b.c` therefore
requires the value expression's type, which `internal/core/resolve` does not derive — the same
diagnostic appears without any indexing at all:

```kerml
feature p = a.b;
feature q = p.c;      // ours: unresolved member: c
```

This is a resolution gap (feature typing from `FeatureValue`), not a grammar gap. No parser change
can close the row.

### `ParsingTests_Metadata.kerml.xt`:74 — a conditional `baseType`

```kerml
metaclass M :> Metaobjects::SemanticMetadata {
  :>> annotatedElement : KerML::Class;
  :>> baseType = if annotatedElement istype KerML::Structure ?
                     SS meta KerML::Type else CC meta KerML::Class;
}
#M struct T { feature :>> cc; }
```

The parse is faithful (the round-trip through `-convert kerml` reproduces the conditional). The
unresolved `cc` is the implicit specialization the semantic metadata implies: `internal/core/semantics`
reads `baseType` only when its value names a type directly, so a conditional value yields no base
type and `T` inherits nothing from `SS`. Written without the conditional, the same fixture is clean:

```kerml
metaclass M2 :> Metaobjects::SemanticMetadata {
  :>> annotatedElement : KerML::Class;
  :>> baseType = SS meta KerML::Type;
}
#M2 struct T2 { feature :>> cc; }   // clean
```

So the row needs `baseTypeOf` in `internal/core/semantics/metadata.go` to fold a conditional in the
context of the annotated element — semantics, not parser.

### The four `non` rows and the two `ScopeWithFourDotAndDot` rows — reference metaclass is not checked

```kerml
classifier non{}
feature aa subsets non;              // pilot: Couldn't resolve reference to Feature 'non'.
feature c : OuterPackage::B.b;       // pilot: … to Feature 'OuterPackage::B'. and … to Feature 'b'.
```

`Subsetting::subsettedFeature` and `Feature::chainingFeature` are typed `Feature`, so a `Classifier`
in either position is a linking failure for the pilot: it looks for a `Feature` of that name and
finds none. We resolve a name in these positions without regard to the metaclass the position
admits, so `non` resolves to the classifier and `OuterPackage::B.b` resolves through it, and we
report nothing. Closing these six rows means resolving references against the metaclass the
reference position requires (`internal/core/resolve`), which is a cross-cutting change to name
resolution and is not owned by the parser.

Two of the four `non` rows (in the two `..` fixtures) additionally sit behind a syntax error in the
same file, so the name-resolution tier does not run there at all; they cannot be closed before the
metaclass check exists either way.

### The six `..` rows — recovery improved, the rows stay open

`ParsingTests_BadScopeWithOnlyTwoSingleDot.kerml.xt` and `…AtTheEnd` write `test..A::a` and
`test::A..a`. A qualified name separates its segments with `::` and a feature chain with `.`
(`KerML.xtext` `QualifiedName`, `OwnedFeatureChaining`), so `..` chains over a segment that is not
written — there is no derivation, and the pilot emits three ANTLR failures per file
(`no viable alternative at input '..'`, one per token around the defect).

Before, our recovery abandoned the enclosing body and reported `expected '{' or ';' after
declaration` **and** `expected a body member` at the end of the file — nine lines away from the
defect in one fixture, and the members after it were dropped. Now `parseRelationshipTarget` reports
`expected a name after '.'` on the `..` itself and reads the rest of the chain, so the declaration
still ends at its `;` and the following members parse. `…TwoSingleDotAtTheEnd`:26 is
`disagree(same-location)` as a result, having been `disagree(same-line)` before.

**These rows remain disagreements and cannot be closed here.** The declared text is a parser-internal
ANTLR message; the harness admits a wording difference only for the unresolved-reference rule
(`cmd/pilot-xpect/wording.go`), and it is admitted on rule and element identity, which a parse
failure has neither of. Emitting three diagnostics where the defect is one, or adopting the pilot's
phrasing, would move the number without changing what we detect. The improvement that is real is
locality and recovery, which is what the fixture-level test
(`internal/core/parser/chain_double_dot_test.go`) locks.

### `ParsingTests_BadScopeWithOnlyTwoDot.kerml.xt`:26 — unchanged, and already documented

`feature b: test:A::a;` — the pilot cannot resolve `test`; we resolve it and reject the kind (`type
must be a type, found package`). This row is already recorded in
[pilot-xpect.md](pilot-xpect.md) as the `same-location` row where our answer is the more precise one.

---

## One row this wave unmasks

`Type_Multiplicity_invalid.kerml.xt`:20 declares `Only one multiplicity is allowed` for

```kerml
classifier C {
	multiplicity subsets Base::zeroOrOne;
	multiplicity subsets Base::zeroToMany;
}
```

Before the `MultiplicitySubset` fix we produced `expected '{' or ';'` on that line, which the harness
scored as `disagree(same-line)`. We now parse the fixture, so the row is a plain `disagree`: the
declared rule — a type owns at most one multiplicity — is a validation rule we do not implement. The
verdict and the totals are unchanged; only the tolerance class is. It belongs to whoever owns the
validation suites, not to the parser.
