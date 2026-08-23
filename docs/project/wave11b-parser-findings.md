# Wave 11B — the parsing rows, and the ones that are not parser rows

Wave 11B owns the 20 Xpect disagreements in the pilot's parsing suites that were attributed to the
lexer and the parser, plus one row 11A escalated as parser-owned (`StateUsage_invalid.sysml.xt`:83).
Three of them were parser defects and two of those close; the rest are not parser defects, and this
page records what each one actually needs, so that a later reader does not look for the fix in
`internal/core/parser`.

Every number here is a fresh run of the three oracles against the pinned `PILOT_TAG=2026-05`
corpora, on `main` and on the branch, on one machine:

| oracle | `main` | branch |
| --- | --- | --- |
| xpect | 1197 agree (239 wording-only), 129 disagree | **1199 agree** (239 wording-only), **127 disagree** |
| differential | 312 fully agreeing; 25 agreed, 139 only ours, 73 only the pilot's | identical |
| rejection | 115 both reject, 5 only the pilot, 0 only we | identical |

`main` here is post-wave-11A `main` (the branch is merged with it), so these are not the wave-10
figures the task paste quotes. **Two rows close** — `ParsingTests_MultiplicityDeclaration.kerml.xt`
and `StateUsage_invalid.sysml.xt`:83 — and nothing else in any oracle moves. The differential's
`main` figures are also not the 311/142 the wave-10 report quotes; 25/139/73 is what consecutive runs
on `main` give, so that difference is in the committed baseline's provenance, not in this branch.

---

## Fixed — the plain-name `exhibit` is a reference, not a declaration

`StateUsage_invalid.sysml.xt`:83 declares `Must reference a state.` for `exhibit s;` where `s` is a
part usage. 11A's rule keys on an `ast.RelReferences` target and could not see this row, because we
parsed the plain-name form as a state usage *named* `s`:

```
ExhibitStateUsage : ExhibitStateUsageDeclaration ... // SysML.xtext
ExhibitStateUsageDeclaration : 'exhibit' ('state' StateUsageDeclaration | OwnedReferenceSubsetting ...)
```

Only `state` introduces a declaration; every other `exhibit` carries an `OwnedReferenceSubsetting`,
i.e. it names an existing state. The parser routed just the feature-chain form (`exhibit s.sa;`)
through `parseReferenceMemberUsage`; it now routes every non-`state` form there, so `exhibit s;` is
an unnamed state usage with a `references` relationship, and `exhibit state s { … }` still declares.
`perform`'s plain-name form was already correct (`parsePerformedActionReference`) and is unchanged;
`behavior_exhibit_reference.sysml` locks all four shapes side by side.

RDF conversion follows the representation: `referenceMemberKeyword` now lists `exhibit`, so the
keyword survives a round trip instead of being refused as unrebuildable, and `StateUsage references`
joins the known-violation list as the structural predicate carrying the named state.

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
the exclusive alternatives of the production. The row closes (`noErrors`). The RDF mapping carries
the new field too (`sysml:subsets`), since a form that now parses and did not before would otherwise
convert to Turtle as a bare `multiplicity m;`.

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

### The six `..` rows — the message names the defect, the rows stay open

`ParsingTests_BadScopeWithOnlyTwoSingleDot.kerml.xt` and `…AtTheEnd` write `test..A::a` and
`test::A..a`. A qualified name separates its segments with `::` and a feature chain with `.`
(`KerML.xtext` `QualifiedName`, `OwnedFeatureChaining`), so `..` chains over a segment that is not
written — there is no derivation, and the pilot emits three ANTLR failures per file
(`no viable alternative at input '..'`, one per token around the defect).

Before, we reported two diagnostics of our recovery's own expectations — `expected '{' or ';' after
declaration` **and** `expected a body member` — where the defect is one. `parseRelationshipTarget`
now reports `expected a name after '.'` once and reads the rest of the chain.

**What did not change, measured rather than assumed: the offset.** Our old diagnostics already sat on
the `..` (offset 642 in `…TwoSingleDot`, 647 in `…AtTheEnd`, unchanged after), the tolerance classes
are the same rows they were, and both binaries keep the declarations after the malformed one. So this
is a message-and-count improvement, not a locality one, and it moves no number.

**Known limitation.** Only the two-dot shape recovers. `:> a...b` and `:> ..a` still cascade — a
following declaration is dropped and a body member is hoisted to the enclosing scope — exactly as
before this change; measured on both binaries.

**The rows remain disagreements and cannot be closed here.** The declared text is a parser-internal
ANTLR message; the harness admits a wording difference only for the unresolved-reference rule
(`cmd/pilot-xpect/wording.go`), on rule and element identity, which a parse failure has neither of.
Emitting three diagnostics where the defect is one, or adopting the pilot's phrasing, would move the
number without changing what we detect. What is real is locked by
`internal/core/parser/chain_double_dot_test.go`: one diagnostic, at the dots' own offset, with the
following declaration still in the tree.

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
