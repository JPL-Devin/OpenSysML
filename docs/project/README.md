# Project

Where the project stands: what is implemented, what is known to be missing, and how
a release is cut.

The conformance records below are engineering records rather than user documentation. Where one
still uses a short internal label — a numbered development round, or an `F<n>` follow-up row — the
label is defined in the record that uses it and means nothing outside this repository; each record
says so at the top.

- **[Spec compliance](spec-compliance.md)** — faithful, approximate, or not implemented,
  rule by rule
- **[Training examples](training-examples.md)** — the OMG corpus, adjudicated per file
- **[Pilot corpora gate](pilot-corpora.md)** — our diagnostics on the three pinned OMG pilot
  corpora, ratcheted in CI
- **[Pilot differential](pilot-differential.md)** — our diagnostics against the OMG pilot
  implementation, advisory
- **[Pilot execution referee](pilot-execution-referee.md)** — how far the pinned pilot's
  execution surface reaches, and which behavior rows it can adjudicate
- **[Pilot Xpect expectations](pilot-xpect.md)** — the pilot's own inline `.xt` assertions as a
  declared-intent oracle, advisory
- **[Grammar coverage](grammar-coverage.md)** — which OMG grammar productions our inputs
  exercise, on input-presence evidence, advisory
- **[Validation adjudications](wave10-decisions.md)** — the three adjudications the validation work
  depends on, with the measurements behind them
- **[Parser findings](wave11b-parser-findings.md)** — the two parsing rows that were parser
  defects, and what each of the other 18 actually needs
- **[Metadata and evaluability](wave11d-metadata-evaluability.md)** — what the metadata
  annotation and model-level evaluability rules now do, and the two divergences left standing
- **[KerML validation and visibility decisions](wave11e-decisions.md)** — the KerML validation and
  visibility rows that stay open, and why
- **[Lossless library records](wave12c-lossless-library-records.md)** — the record format
  and version-compatibility surface roadmap L3 needs, the measurements that chose it, and the rows
  it leaves open
- **[Scope enumeration and visibility decisions](wave12e-decisions.md)** — the visible-name enumeration rule, the two
  resolver defects it closes, and the import-visibility divergence it keeps
- **[Roadmap](roadmap.md)** — the known gaps, in the order they should be picked up
- **[Releasing](releasing.md)** — the pre-tag gate, tagging, artifacts, Homebrew
- **[macOS distribution](macos-distribution.md)** — Gatekeeper and the signing decision
- **[Bugs in the OMG materials](omg-issues.md)** — defects in the vendored specification
  libraries
- **[Demo](demo.md)** — a scripted walkthrough of the whole surface
