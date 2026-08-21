# Pilot Rejection Oracle

Every other oracle in this project is one-directional. The
[differential](pilot-differential.md) compares diagnostics over the OMG corpora — models written
to *demonstrate* the notation, so almost all of them are valid — and therefore measures notation
the reference accepts and we reject. Nothing in it tests the opposite direction: does OpenSysML
**reject** what the reference rejects? `cmd/pilot-reject` answers that with a hand-written
negative corpus, validated by both implementations. A case the pinned pilot rejects and we accept
is a **permissiveness gap** — the finding this oracle exists to surface.

The oracle is advisory: nothing in the build or test suite depends on its verdicts. Its verdicts
are externally refereed — the pilot's verdict on every case comes from actually running the
pinned validators, not from our reading of the grammar. Our adjudication of *why* each gap exists
(the "likely root cause" column) is self-assessed.

## Pinned reference

The same pin as the differential: OMG SysML v2 Pilot Implementation `2026-05`
(`jupyter-sysml-kernel 0.60.1`, see `scripts/pilot-pin.sh`). Two validators referee:

- `build/pilot-sysml-validator/validate-sysml-batch` for `.sysml` cases
  (`./scripts/download-pilot-sysml-validator.sh`)
- `build/pilot-kerml-validator/validate-kerml` for `.kerml` cases
  (`./scripts/download-pilot-kerml-validator.sh`)

`./scripts/download-pilot-reject-validators.sh` provisions both. Both load the pinned standard
library, so verdicts that require library-relative semantics (implicit specialization, implicit
typing) are refereed under the same conditions our workspace validates under.

## Corpus derivation

The corpus is committed under `cmd/pilot-reject/testdata/negative/`. Every file's first line is a
mandatory header — `// Invalid: <rule> (<citation>).` — naming the one rule the case violates and
where that rule comes from; the harness refuses a corpus file without it. Cases were derived
systematically from three sources, one subdirectory each:

1. **`grammar/` — grammar mutation** (20 cases). For productions our corpus exercises in the
   pinned Xtext grammars (`build/pilot-grammars/`, see the `testing-grammar-coverage` skill), the
   minimal violation: a required keyword removed (`g03` alias without `for`), a mandatory element
   omitted (`g04`, `g05`, `k01`, `k03`), a clause in a position the production forbids (`g06`
   multiplicity on a definition, `g07`/`g08` state members in a part def body), a token from a
   sibling production (`g15` a keyword as a name, `k02` a SysML keyword in KerML), and unterminated
   bodies and comments (`g01`, `g12`, `k05`).
2. **`extensions/` — the notation we invented** (7 cases). Every state-machine construct our
   `examples/` tree uses that no pinned production admits: `initial`, `choice`, `junction`,
   `history`, `region`, `defer`, and the `transition <src> to <tgt>` shorthand. The pinned grammar
   spells entry as `entry; then <state>`, concurrency as `state ... parallel`, and transitions as
   `first <src> then <tgt>`, and has no pseudostates or deferral at all. Adjudication: these are
   **intended OpenSysML extensions**, not accidents — each has dedicated parser tests
   (`internal/core/parser/state_notation_test.go`) and runtime support. But they are not yet
   documented as extensions nor gated behind anything a conformance-minded user can turn off;
   that gating is an open finding for a later wave, recorded in the gap table below.
3. **`xpect/` — the pilot's own negative expectations** (7 cases). The Xpect suites declare 513
   `errors` expectations ([pilot-xpect.md](pilot-xpect.md)); where a suite declares an error we do
   not report anywhere in the file, that is a candidate rejection gap. Each case here re-derives
   one such declared error as a standalone model, citing the KerML clause and the originating
   `.xt` suite. One caveat found while deriving: some Xpect negatives (e.g.
   `Feature_invalid_noType.kerml.xt`) only error in a library-less resource set — with the
   standard library loaded, `feature f;` gets an implicit type and is legal — so only
   library-independent expectations became cases.

What this corpus cannot see: it tests the invalid models we thought to write. It is a **sample of
the rejection surface, not a proof** — a clean bucket here does not mean OpenSysML rejects
everything the reference rejects, and no official conformance suite exists to make that claim
testable.

## Running it

```bash
./scripts/download-pilot-reject-validators.sh   # once; needs Java 17+ and Maven
go run ./cmd/pilot-reject
```

The harness validates every corpus file with our workspace and with the pinned validator for its
language, counts error-severity diagnostics on each side (warnings do not count as rejection), and
buckets every case:

- **both-reject** — agreement; the case is settled.
- **pilot-only-rejects** — a permissiveness gap; the report keeps the pilot's messages as evidence.
- **ours-only-rejects** — already the differential's business; counted and moved past.
- **both-accept** — the case itself is wrong and must be fixed; a corpus revision, not a finding.

It writes `build/pilot-reject/pilot-reject.txt` and `build/pilot-reject/pilot-reject.json`. The
JSON is committed as [pilot-rejection-baseline.json](pilot-rejection-baseline.json); the reports
carry no timestamps or absolute paths, so repeated runs are byte-identical
(`cmp build/pilot-reject/pilot-reject.json docs/project/pilot-rejection-baseline.json`).

## Totals

```
34 case(s): 24 both reject, 10 only the pilot rejects, 0 only we reject, 0 both accept
```

| Source | Cases | Both reject | Pilot only | Ours only | Both accept |
| --- | --- | --- | --- | --- | --- |
| extensions | 7 | 2 | 5 | 0 | 0 |
| grammar | 20 | 17 | 3 | 0 | 0 |
| xpect | 7 | 5 | 2 | 0 | 0 |

The two `extensions/` cases in *both-reject* (`x02` choice, `x03` junction) are rejected by us for
a different reason than by the pilot: our own state-connectivity validation flags a pseudostate
with no outgoing transition, while the pilot rejects the notation itself. The bucket records
rejection, not agreement on the rule.

## Permissiveness gaps

All 10 gaps, each with its reproducer (the corpus file is the minimal reproducer), both verdicts,
and the package the root cause is likely in. **None are fixed here** — this oracle measures;
fixing is later work.

| Reproducer (`cmd/pilot-reject/testdata/negative/`) | Ours | Pilot | Likely root cause |
| --- | --- | --- | --- |
| `extensions/x01-initial-state-marker.sysml` | accepts | syntax error at `initial` | `internal/core/parser` — intended extension, ungated |
| `extensions/x04-orthogonal-region.sysml` | accepts | syntax error at `region` | `internal/core/parser` — intended extension, ungated |
| `extensions/x05-defer-member.sysml` | accepts | syntax error at `defer` | `internal/core/parser` — intended extension, ungated |
| `extensions/x06-history-member.sysml` | accepts | syntax error at `history` | `internal/core/parser` — intended extension, ungated |
| `extensions/x07-transition-to.sysml` | accepts | syntax error at `to` | `internal/core/parser` — intended extension, ungated |
| `grammar/g02-import-without-visibility.sysml` | accepts | `mismatched input 'import'` | `internal/core/parser` — treats the import visibility keyword as optional; the pinned `ImportPrefix` requires it |
| `grammar/g15-keyword-as-name.sysml` | accepts | `no viable alternative at input 'part'` | `internal/core/parser` — allows a reserved keyword as a declared name |
| `grammar/k02-sysml-keyword-in-kerml.kerml` | accepts | `no viable alternative at input 'def'` | `internal/core/parser` — `.kerml` files are parsed with the full SysML grammar; no per-language restriction |
| `xpect/p04-nonunique-subsets-unique.kerml` | accepts | `Subsetting/redefining feature cannot be nonunique...` | `internal/core/passes` — uniqueness conformance (KerML 8.3.3.3) not implemented |
| `xpect/p06-private-member-reference.kerml` | accepts | `Couldn't resolve reference to Classifier 'A::X'` | `internal/core/resolve` — qualified-name resolution does not enforce `private` visibility |

Each pilot message above is the first error the validator reports for the case; the full lists are
in the baseline JSON's `pilot` arrays.

## Guard

`TestPilotRejectionDocumentCountsMatchBaseline` (in `cmd/pilot-reject`) re-derives every count in
this document, the README's rejection-oracle line, and the skill's headline from
[pilot-rejection-baseline.json](pilot-rejection-baseline.json), and checks the gap table above
enumerates exactly the baseline's `pilot-only-rejects` cases. It reads only committed files — no
validators, no downloads — so it runs in CI and fails the moment this prose goes stale.
