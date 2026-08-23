# Project

Where the project stands: what is implemented, what is known to be missing, and how
a release is cut.

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
- **[Wave 10 decisions](wave10-decisions.md)** — the three adjudications the wave-10 slices depend
  on, with the measurements behind them
- **[Wave 11B parser findings](wave11b-parser-findings.md)** — the two parsing rows that were parser
  defects, and what each of the other 18 actually needs
- **[Wave 11D — metadata and evaluability](wave11d-metadata-evaluability.md)** — what the metadata
  annotation and model-level evaluability rules now do, and the two divergences left standing
- **[Wave 11E decisions](wave11e-decisions.md)** — the KerML validation and visibility rows wave 11E
  leaves open, and why
- **[Wave 12E decisions](wave12e-decisions.md)** — the visible-name enumeration rule, the two
  resolver defects it closes, and the import-visibility divergence it keeps
- **[Roadmap](roadmap.md)** — the known gaps, in the order they should be picked up
- **[Releasing](releasing.md)** — the pre-tag gate, tagging, artifacts, Homebrew
- **[macOS distribution](macos-distribution.md)** — Gatekeeper and the signing decision
- **[Bugs in the OMG materials](omg-issues.md)** — defects in the vendored specification
  libraries
- **[Demo](demo.md)** — a scripted walkthrough of the whole surface
