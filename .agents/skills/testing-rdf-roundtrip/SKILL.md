---
name: testing-rdf-roundtrip
description: How to end-to-end test the `internal/core/export` RDF mapping (`sysml -convert ttl|sysml`) so the test is load-bearing — stripping `sysx:sourceText` to force the structural predicates to carry the round trip, proving `.ttl` idempotence, and the negative controls that distinguish a working mapping from a decorative one.
---

# Testing the SysML ↔ RDF Turtle round trip (`internal/core/export`)

The mapping is reachable from the CLI:

```bash
export PATH=/usr/local/go/bin:$PATH
go build -o /tmp/sysml ./cmd/sysml
/tmp/sysml model.sysml -convert ttl   -o hop1.ttl
/tmp/sysml hop1.ttl    -convert sysml -o back.sysml
/tmp/sysml back.sysml  -convert ttl   -o hop2.ttl
cmp hop1.ttl hop2.ttl && echo IDEMPOTENT
```

Every RDF run prints an "RDF conversion is experimental" note on **stderr**; that is not an error.

## The pitfall that makes a naive round-trip test vacuous

The encoder writes `sysx:sourceText "<the element's lines as written>"` (and `sysx:sourceTail`
for the lines closing a body) for every element, and the decoder writes that text back wherever it
still agrees with the graph — that is what makes comments survive the hop. But it also means a
plain `.sysml → .ttl → .sysml` round trip **passes even if a structural predicate is broken**, as
long as the broken predicate is missing from both graphs alike.

To make the test load-bearing, strip `sysx:sourceText` and `sysx:sourceTail` from the intermediate
`.ttl` first (the same thing the `withoutTriples` test helper in
`internal/core/export/export_test.go` does in-process) and only then convert back. Every literal is
written on one line, newlines escaped, so a small Python filter is enough — drop any line containing
the predicate, and when the dropped line ended the triple block with ` .`, turn the previous line's
trailing `;` into ` .`:

```python
def strip(src, preds, dst):
    out, lines, i = [], open(src).read().split('\n'), 0
    while i < len(lines):
        s = lines[i].strip()
        if any((p + " ") in s for p in preds):
            j = i
            if s.count('"""') % 2 == 1:          # multi-line """...""" literal
                j += 1
                while '"""' not in lines[j]:
                    j += 1
            if lines[j].rstrip().endswith('.') and out and out[-1].rstrip().endswith(';'):
                out[-1] = out[-1].rstrip()[:-1].rstrip() + ' .'
            i = j + 1
            continue
        out.append(lines[i]); i += 1
    open(dst, 'w').write('\n'.join(out))
```

Pass fully-prefixed predicate names —
`strip("hop1.ttl", ["sysx:sourceText", "sysx:sourceTail"], "stripped.ttl")` — since not every
load-bearing predicate lives in the `sysx:` namespace (the `sysml:isVariant` / `sysml:includes`
controls below use the `sysml:` prefix).

Pass criterion: the source-text-free back-conversion is the canonical notation of the model —
identical to the source-text-backed one up to trivia and keyword synonyms — and re-encoding it gives
a `.ttl` byte-identical to `hop1.ttl` once both are stripped of the source text the same way. The
source-text-backed conversion has its own criterion: it must be **byte-identical to the formatted
input** (`sysml -fmt` it first), which `TestSourceTextComesBackByteForByte` locks in-process.

For corpus files compare `hop1.ttl` and `hop2.ttl` as **triple sets**, not bytes (`pip install rdflib`,
parse both with `rdflib.Graph().parse(p, format='turtle')`, diff the sets), and report
`sysx:sourceText` differences separately from structural ones: the writer normalises whitespace and
drops optional keywords (`connector x from a to b` → `connector x a to b`), so sourceText will legally
differ on reformatted heads while every structural triple must still match.

### Heads that are *not* expected to survive without sourceText (as of this writing)

- **Any end-binding head whose source spans lines.** `endForm` in `internal/core/export/end_forms.go`
  is only emitted when rebuilding the head reproduces the text *exactly*, so a line break inside
  `connect a\n to b;` or `flow x\n to y;` means no `sysx:endForm`, and the sourceText-free hop is
  refused with `it has no sysx:endForm, and the ends it relates are written in the form the head
  states`. Corpus files with wrapped heads (e.g. `sysml-validation/03-Function-based Behavior/3a-…`)
  therefore fail the stripped trip even when the mapping is otherwise correct — check the source text
  for a newline before treating the refusal as a regression.
- **Named satisfy heads** `satisfy requirement req1 : Req1 by system;` come back as
  `satisfy req1 : Req1 by system;` (the `requirement` keyword is dropped, and the parser then reads
  `req1` as the *satisfied* requirement instead of a new one — `unresolved reference: req1`). Minimal
  fixture: `part ctx { satisfy requirement req1 : Req1 by system; }`. Affects
  `sysml-examples/Requirements Examples/RequirementDerivationExample.sysml`.

## Negative controls that prove each predicate is load-bearing

Strip the structural predicates the same way and re-convert. The four notation predicates below are
defined in `internal/core/export/rdf_out.go` (`xEndForm`, `xEndVerb`, `xSourceMember`,
`xTargetMember`) and are *additional* to the older end triples `sysx:endIndex`, `sysx:endRole` and
`sysx:relatedFeature`, which carry the participants rather than the notation — stripping the
notation ones is what makes the participants insufficient. Confirm the vocabulary before trusting
this table, since it is versioned with the mapping:

```bash
grep -o "sysx:[a-zA-Z]*" hop1.ttl | sort | uniq -c | sort -rn
```

| Stripped | Observed behaviour (working mapping) |
|---|---|
| `sourceText` + `sourceTail` only | canonical notation comes back — the structural predicates carried it; only comments and synonyms differ from the input |
| `sourceText` + `sourceTail` + `endForm`/`endVerb`/`sourceMember`/`targetMember` | refusal: `cannot convert the element <urn:sysmlv2:element:…>: it has no sysx:endForm, and the ends it relates are written in the form the head states, so no valid declaration can be written for it` — **no file is written** |
| `sourceText` + `sourceTail` + `endVerb` only | the noun-keyword heads degrade visibly: `connection c connect left to right;` → `connection c left to right;`, `binding bb of a = b;` → `binding bb a = b;`, `succession s first p then q;` → `succession s p then q;` |

The `endVerb` degradation is the cheapest single proof that a predicate is not decorative. Note the
degraded output still validates clean, so judge it by the *text*, not by the exit code.

## A fixture that exercises the end-binding heads

One package covering all of `connect` / `bind` / `flow` / `succession` / `transition` / `accept` /
`satisfy` plus an unnamed positional `then`, and a second covering the noun-keyword heads:

```sysml
package RT {
  part def Car { part e { port p; } part w { port q; } connect e.p to w.q;
    attribute x : ScalarValues::Real; attribute y : ScalarValues::Real; bind x = y; }
  action def Drive { action a : Move; action b : Move; flow from a.g to b.f; succession a then b; }
  action def Seq { action s1; then s2; action s2; }              // unnamed positional `then`
  state def Lights { entry; then off; state off; state on; transition t first off then on; }
  requirement def R1; requirement r1 : R1; part def Sat { satisfy r1; }
}
package V {                                                       // exercises sysx:endVerb
  part def Car { port left; port right; attribute a; attribute b;
    connection c connect left to right; binding bb of a = b; }
  action def Flow2 { action p; action q; succession s first p then q; }
}
```

Gotchas met while building it: `satisfy R1;` where `R1` is a requirement **def** is rejected
(`satisfy target must be a requirement usage`) — satisfy a usage. Always `-validate` the fixture
before converting; a fixture that does not analyse cleanly makes every later result meaningless.

## Predicate coverage to check before believing "all four are exercised"

`sysx:targetMember` may be **unreachable from notation**: it is only written when a succession's
*target* end has no name, and every candidate spelling (`then;`, `succession a then;`,
`state s1; then; state s2;`) is a parse error. It also appears in no committed golden `.ttl` and no
test fixture. Grep before claiming coverage:

```bash
grep -rn "targetMember" internal/core/export/testdata/ internal/core/export/*_test.go
```

If it is still absent, say so — the predicate is decoder-only and its encoder branch is untested.

## Flag / import / succession-end predicates (fixtures under `internal/core/export/testdata/convert/`)

Each of these degrades visibly (exit 0, judge by text) or is refused when stripped together with
`sysx:sourceText`; if the notation comes back unchanged the predicate has become decorative:

| Fixture | Strip | Working behaviour |
|---|---|---|
| `ref_subsets.sysml` | `sysml:isReference` | `ref x subsets y;` → `x subsets y;` |
| `composite_multiplicity.kerml` | `sysml:isComposite` | `composite frontWheel[2] redefines w` → `frontWheel[2] redefines w` |
| `end_prefix_metadata.sysml` | `sysml:isEnd` | `end #derive r1_1 : Req1_1;` → `#derive r1_1 : Req1_1;` |
| `nested_namespace_import.sysml` | `sysx:isNamespaceImport` | `import Pkg1::*;` → `import Pkg1;`, `Pkg1::*::**` → `Pkg1::**` |
| `nested_namespace_import.sysml` | `sysx:isRecursive` | `import Pkg1::**;` → `import Pkg1;` |
| `quoted_succession_ends.sysml` | `sysml:targetFeature` | refusal: `does not name both of the members it sequences` |
| `quoted_succession_ends.sysml` | `sysml:sourceFeature` | refusal: `it has no sysml:sourceFeature, and a succession written as `then` …` |

Quoted succession ends are IRIs (`sysml:targetFeature elmt:…__generate_20torque`), except `start`/`done`,
which stay string literals because they name no element.

## The `variant` / `include` prefixes (formerly normalized away)

Since the parser started recording them, `variant part a : A;` and `include U;` round-trip. The
carriers to strip for their negative controls are **`sysml:` predicates, not `sysx:` ones** (with
the helper above: `strip("hop1.ttl", ["sysml:isVariant"], ...)`):

- `variant` rides on `sysml:isVariant "true"` — stripping it silently degrades the head to
  `part a : A;` (exit 0, judge by text).
- `include U;` rides on `sysml:includes <target>` — stripping it degrades the member to a bare
  `;` in the owner's body (exit 0). Stripping `sysx:isKindImplicit` alone does *not* degrade it.

A minimal fixture that validates clean:

```sysml
package RT2 {
  part def A;
  variation part def VP { variant part a : A; }
  use case def UD;
  use case U : UD;
  part def Sys { include U; }
}
```

## Metadata annotations

Every `@M;`, `@M { … }`, `metadata m : M about a, b;` and `#M part def P;` is a
`sysml:MetadataUsage` with `sysml:type`, one `sysml:annotatedElement` per `about` target,
`sysx:hasBody`, `sysx:declaredKeyword` `"@"`/`"#"` (absent for the `metadata` keyword) and body
members as owned members ordered by `sysx:memberIndex`. Fixtures:
`internal/core/export/testdata/convert/metadata_bodies.sysml` and `metadata_prefixes.sysml`
(neither validates clean on its own — unqualified `Integer`/`Real` and a `variant` outside a
`variation` — so judge semantic equality by identical `-validate` diagnostics, or add
`private import ScalarValues::*;` to a copy). Hand-edit the stripped `.ttl` for the controls:

| Edit | Working mapping |
|---|---|
| drop one IRI from `sysml:annotatedElement a, b` | `about a, b` → `about a` (exit 0) |
| `sysx:declaredKeyword "#"` → `"@"` on a prefix | `#M part def P;` → `part def P { @M; }` |
| `"@"` → `"#"` on a bodiless member `@M;` | it moves into the owner's head: `#M part car : Car {` |
| swap two body members' `sysx:memberIndex` | the body lines swap order |
| give a `"#"` annotation an `about`, a body or a `declaredName` | refused: `cannot convert the prefix annotation <urn:sysmlv2:element:…>: a prefix names only its metadata definition; …` — no file written |

In notation, `@M part def Q;` is a parse error (`expected ';' or '{' after a metadata usage`),
an annotation whose type does not resolve is still converted with `sysml:type "M"` as a literal,
and `@IdentityMetadata::ElementId { id = "…"; }` must **not** appear as a `sysml:MetadataUsage`
(it becomes the element's IRI/`sysml:elementId`). Unnamed `metadata M about x;` and
`metadata $::P::M about x;` come back spelled as written, typing bare.

The pilot validation files live in numbered subdirectories
(`examples/pilot-corpora/sysml-validation/13-Model Containment/13b-*.sysml`); a glob on the parent
directory alone matches nothing and `sysml` then complains about a missing extension.

## The corpus round-trip ratchet (run it before and after any writer/encoder change)

`TestCorpusRoundTrip` (`internal/core/export/corpus_roundtrip_test.go`) runs the three-hop trip
over **every** `.sysml`/`.kerml` under `examples/` — the 32 committed models, the 100-file
training corpus and the three pilot corpora (213 files) — and pins one verdict per file in
`internal/core/export/testdata/corpus_roundtrip_expected.txt`. It runs in about two seconds.
Record: `docs/project/rdf-corpus-roundtrip.md`.

```bash
./scripts/download-training-examples.sh && ./scripts/download-pilot-corpora.sh   # once
OPENSYSML_REQUIRE_TRAINING_CORPUS=1 OPENSYSML_REQUIRE_PILOT_CORPORA=1 \
  go test -count=1 -v ./internal/core/export -run TestCorpusRoundTrip
```

Without the require variables an absent corpus **skips** the gate with a `GATE NOT RUN` banner on
stderr; CI sets them so absence fails. The `-v` log holds the summary line CI greps for:
`corpus round trip: 345 files: 166 stable, 71 whitespace-only, 14 graph-diff, 15 unwritable,
2 unparseable, 77 refused` at the time of writing.

Verdicts, in the order the trip can end: `refused:<class>` (hop 1 refused; the class is the
construct kind from `UnsupportedError.What` without location or identifiers, e.g.
`refused:feature-declaration`, `refused:prefix-metadata`, `refused:succession`), `unwritable`
(Turtle → notation refused), `unparseable` (the written notation no longer converts), then a
triple-set comparison of hop 1 against hop 2: `stable` (byte-identical), `whitespace-only` (equal
once whitespace inside `sysx:sourceText` literals is collapsed — the writer re-indented a body),
`graph-diff` (anything else).

It is a **per-file ratchet**, like the pilot corpora gate: an improvement fails it just as a
regression does, so every movement is adjudicated. When your change moves files:

1. Run the gate; read the per-file lines it prints (`<path>: <got>, expected <want>`).
2. Confirm each movement is one your change should cause, in the direction it should go. A file
   that regresses (`stable` → `graph-diff`, anything → `unwritable`) is a defect to fix, not to
   record.
3. Regenerate and commit the expectation file in the same PR:
   ```bash
   go test ./internal/core/export -run TestCorpusRoundTrip -update-corpus-roundtrip
   git diff --stat internal/core/export/testdata/corpus_roundtrip_expected.txt
   ```
   Run the update twice and confirm the second run leaves the file unchanged; the run is
   deterministic (a worker pool, results indexed by sorted path) and a diff between two runs is a
   nondeterminism bug in the mapping, not something to record.
4. State the before/after counts by verdict in the PR description.

A `stable` verdict here is **not** proof that the structural predicates carry the round trip:
every node still has `sysx:sourceText` and the decoder writes from it where it can. Use the
stripping controls above for that. A `# files: <root> <n>` header that disagrees with the
checkout is a stale or partial download (re-run the fetch scripts), not a mapping change.

## Recording

Shell-only; no GUI. No recording needed.

## Devin Secrets Needed

None.
