# RDF Corpus Round-Trip Gate

## Overview

**Corpus:** every `.sysml` and `.kerml` under `examples/` — the committed models, the OMG training
corpus (`./scripts/download-training-examples.sh` → `examples/sysml-v2-training`) and the three
OMG pilot corpora (`./scripts/download-pilot-corpora.sh` → `examples/pilot-corpora/*`), all at the
pin in `scripts/pilot-pin.sh`.

| Root | Files |
|---|---|
| `committed` (everything under `examples/` outside the downloaded roots) | 32 |
| `sysml-v2-training` | 100 |
| `pilot-corpora/kerml-examples` | 58 |
| `pilot-corpora/sysml-examples` | 99 |
| `pilot-corpora/sysml-validation` | 56 |

**Gate:** `TestCorpusRoundTrip` in `internal/core/export/corpus_roundtrip_test.go` converts each file
notation → Turtle (hop 1) → notation → Turtle (hop 2) and records one verdict per file in
`internal/core/export/testdata/corpus_roundtrip_expected.txt`, so a file whose verdict moves in
either direction, or that appears or disappears, fails the test
**Regenerate:** `go test ./internal/core/export -run TestCorpusRoundTrip -update-corpus-roundtrip`
**Required in CI:** `OPENSYSML_REQUIRE_TRAINING_CORPUS=1` and `OPENSYSML_REQUIRE_PILOT_CORPORA=1`
in both `.circleci/config.yml` and `.github/workflows/pr.yml`, under which an absent or empty
downloaded root fails instead of skipping. Both configurations also run the gate on its own so
its summary line is legible in the log and a skip cannot pass.

## Why

The fixture round-trip tests in `internal/core/export/export_test.go` (`TestRoundTripIsLossless`,
`TestGoldenConversions` and the per-construct tests) assert byte-stability over a few dozen
authored models, and they pass. The example corpus is an order of magnitude larger and is not
clean under the mapping: some files are refused, some cannot be written back, and some come back
as a different graph. Those defects are documented one by one in
[rdf-mapping.md § Limitations](../reference/rdf-mapping.md#limitations), but nothing measured them
together, so a writer or encoder change could move a file from one verdict to another without
any test noticing. This gate is that measurement, pinned per file.

## Verdicts

Each file gets exactly one of:

| Verdict | Meaning |
|---|---|
| `stable` | Hop 2 is byte-identical to hop 1. |
| `whitespace-only` | The two Turtle documents differ as bytes, but their triple sets are equal once the whitespace inside every `sysx:sourceText` literal is collapsed to single spaces. This is the shape of the mapping's known instability: the writer re-indents a body, and the text the encoder records for it changes with the indentation. |
| `graph-diff` | The triple sets differ beyond `sysx:sourceText` whitespace: hop 2 gained, lost or changed a triple. |
| `unwritable` | Hop 1 succeeded but Turtle → notation was refused. |
| `unparseable` | The notation written back was refused on its way to Turtle again. |
| `refused:<class>` | Notation → Turtle was refused on the first hop. `<class>` is the kind of construct the refusal names (`feature-declaration`, `prefix-metadata`, `succession`, `operator-expr`, `duplicate-declaration`, …), derived from `export.UnsupportedError.What` with its location and identifiers removed so the class is the same wherever in the file the construct sits; `syntax` for source that does not parse; `error` for anything else. |

The comparison is over triple sets, not bytes, so the order in which triples are written is never
a difference, and only `sysx:sourceText` is normalised: whitespace anywhere else, a datatype, a
language tag or a missing triple is a `graph-diff`.

## Baseline

Recorded against the corpus above, reproduced byte-identically on a second run:

| Verdict | Files |
|---|---|
| `stable` | 184 |
| `whitespace-only` | 78 |
| `graph-diff` | 7 |
| `unwritable` | 2 |
| `unparseable` | 4 |
| `refused` | 70 |
| **total** | **345** |

So 275 of 345 files convert to Turtle, and of those 262 come back as the same graph (184 exactly,
78 up to `sysx:sourceText` whitespace). The refusals by class: 19 `feature-declaration`,
18 `prefix-metadata`, 9 `event-declaration`, 6 `operator-expr`,
4 `duplicate-declaration`, 3 each of `snapshot-declaration`, `invocation-expr` and
`assert-declaration`, 2 `feature-chain-expr`, and 1 each of `timeslice-declaration`,
`feature-reference` and `constructor-expr`.

## Policy

This is a **per-file ratchet**, like the [pilot corpora gate](pilot-corpora.md), not an assertion
that the corpus round trips. The mapping is experimental and the corpus is not clean under it, so
there is nothing to assert yet; the verdicts are pinned instead, and every movement has to be
adjudicated:

- A verdict that improves (`graph-diff` → `stable`, `refused:…` → `whitespace-only`) fails the
  gate too. Regenerate the expectation file in the PR that made the improvement and review the
  diff: it should move only the files the change was meant to move, in the direction it was meant
  to move them.
- A verdict that regresses fails the gate and must be fixed at its root, not re-baselined. The
  update flag rewrites the whole file, so a regression can only be recorded by a PR whose diff
  shows it; review the movement, not the summary count.
- The header records the file count of each root, so a root whose count differs from the header
  is a provisioning question — a stale or partial download — before it is a behaviour question.

## What the gate does not do

- **It does not say the recorded verdicts are right.** A `stable` file's graph may still be a
  poor mapping of the model, and a `refused` file's refusal may be a defect. Correctness is the
  fixture tests' job; this gate measures whether the second hop reproduces the first.
- **It does not exercise the structural predicates on their own.** Every node carries
  `sysx:sourceText`, and the decoder writes notation from it where it can, so a `stable` verdict
  here does not prove the graph could be written back without the text. That is what the
  `sysx:sourceText`-stripping tests in `export_test.go` are for; see
  `.agents/skills/testing-rdf-roundtrip/SKILL.md`.
- **It does not run when the downloaded corpora are absent**, except in CI, where the require
  variables make absence a failure. Locally the skip is announced with a `GATE NOT RUN` banner on
  stderr and the fetch command to run.
