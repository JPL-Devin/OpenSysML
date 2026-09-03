- **A census of the pilot's named validation constraints.** `docs/project/validation-constraints.md`
  lists every `validate*` constraint the pinned pilot validators name (217, re-extracted from the
  pinned jar into `docs/project/validation-constraints-baseline.json`) with what it checks, where
  OpenSysML implements it, how our wording differs, its negative case, and an honest status — a
  constraint without a case or an identifiable pass is recorded as unknown rather than absent. Every
  faithful or approximate row is backed by a probe model `go test ./cmd/validation-census` runs.
  `go run ./cmd/validation-census -check` (in `make docs-counts` and CI) fails when the baseline no
  longer matches the jar, when the table and baseline disagree, or when a quoted figure is edited by
  hand.
