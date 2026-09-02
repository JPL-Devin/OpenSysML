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
    out = []
    for line in open(src):
        s = line.strip()
        if any((p + " ") in s for p in preds):
            if s.endswith('.') and out:
                prev = out[-1].rstrip()
                if prev.endswith(';'):
                    out[-1] = prev[:-1].rstrip() + ' .\n'
            continue
        out.append(line)
    open(dst, 'w').write(''.join(out))
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

## Recording

Shell-only; no GUI. No recording needed.

## Devin Secrets Needed

None.
