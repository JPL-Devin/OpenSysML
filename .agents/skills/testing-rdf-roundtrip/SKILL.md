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

The encoder writes `sysx:sourceText "<the head as written>"` for most elements. With that triple
present the decoder can rebuild the notation from the text alone, so a plain
`.sysml → .ttl → .sysml` round trip **passes even if every structural predicate is broken**.

To make the test load-bearing, strip `sysx:sourceText` from the intermediate `.ttl` first (the same
thing the `withoutTriples` test helper in `internal/core/export/export_test.go` does in-process) and
only then convert back. A small Python filter is enough — drop any line containing
`sysx:sourceText `, keep dropping while the literal it opened with `"""` is still open (a multi-line
head, condition or `doc` body is stored as written, line breaks included), and when the dropped
triple ended the block with ` .`, turn the previous line's trailing `;` into ` .`:

```python
def strip(src, preds, dst):
    out, dropping = [], False
    for line in open(src):
        s = line.strip()
        if dropping or any((p + " ") in s for p in preds):
            if not dropping and s.count('"""') == 1:
                dropping = True          # the literal continues on later lines
            elif dropping and '"""' in s:
                dropping = False
            if not dropping and s.endswith('.') and out:
                prev = out[-1].rstrip()
                if prev.endswith(';'):
                    out[-1] = prev[:-1].rstrip() + ' .\n'
            continue
        out.append(line)
    open(dst, 'w').write(''.join(out))
```

Pass fully-prefixed predicate names — `strip("hop1.ttl", ["sysx:sourceText"], "stripped.ttl")` —
since not every load-bearing predicate lives in the `sysx:` namespace (the `sysml:isVariant` /
`sysml:includes` controls below use the `sysml:` prefix).

Pass criterion: the source-text-free back-conversion is **byte-identical** to the source-text-backed
one when the model was written in the writer's own spelling, and re-encoding it gives a `.ttl`
byte-identical to `hop1.ttl` once `sysx:sourceText` is stripped from both. A model laid out by hand
(a head broken across lines, say) comes back in the writer's spelling without its text — the layout
was the text — so compare the two `.ttl` files stripped, not the notation.

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
| `sourceText` only | identical notation comes back — the structural predicates carried it |
| `sourceText` + `endForm`/`endVerb`/`sourceMember`/`targetMember` | refusal: `cannot convert the element <urn:sysmlv2:element:…>: it has no sysx:endForm, and the ends it relates are written in the form the head states, so no valid declaration can be written for it` — **no file is written** |
| `sourceText` + `endVerb` only | the noun-keyword heads degrade visibly: `connection c connect left to right;` → `connection c left to right;`, `binding bb of a = b;` → `binding bb a = b;`, `succession s first p then q;` → `succession s p then q;` |

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
`corpus round trip: 345 files: 255 stable, 0 whitespace-only, 8 graph-diff, 2 unwritable,
3 unparseable, 77 refused` at the time of writing.

Verdicts, in the order the trip can end: `refused:<class>` (hop 1 refused; the class is the
construct kind from `UnsupportedError.What` without location or identifiers, e.g.
`refused:feature-declaration`, `refused:prefix-metadata`, `refused:succession`), `unwritable`
(Turtle → notation refused), `unparseable` (the written notation no longer converts), then a
triple-set comparison of hop 1 against hop 2: `stable` (byte-identical), `whitespace-only` (equal
once whitespace inside `sysx:sourceText` literals is collapsed — the writer re-laid a stored text
out, which it no longer does: the stored text is written verbatim while it still states the graph,
see `docs/reference/rdf-mapping.md` § Stored text is layout), `graph-diff` (anything else).

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
