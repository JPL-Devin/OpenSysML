# Pilot Corpora Gate

## Overview

**Corpora:** the three pinned OMG pilot corpora, fetched by `./scripts/download-pilot-corpora.sh`

| Root | Directory | Files |
|---|---|---|
| `sysml-examples` | `examples/pilot-corpora/sysml-examples` | 98 `.sysml` |
| `sysml-validation` | `examples/pilot-corpora/sysml-validation` | 56 `.sysml` |
| `kerml-examples` | `examples/pilot-corpora/kerml-examples` | 58 `.kerml` |

**Gate:** `TestPilotCorporaDiagnostics` in `internal/core/model/pilot_corpora_test.go` records
every file's diagnostic count in `internal/core/model/testdata/pilot_corpora_expected.txt`, so a
count going up, a count going down, a file that becomes clean and a file that starts reporting all
fail the test
**Regenerate:** `go test ./internal/core/model -run TestPilotCorporaDiagnostics -update-pilot-corpora`
**Required in CI:** `OPENSYSML_REQUIRE_PILOT_CORPORA=1` in both `.circleci/config.yml` and
`.github/workflows/pr.yml`, under which an absent or empty corpus fails instead of skipping

## What the gate does not do

- **It is not a conformance claim.** The counts are *our* verdicts on those 212 files, recorded as
  measured. Many of them are diagnostics the reference implementation does not report; the starting
  baseline is where the implementation actually is, not where it should be.
- **It is not a comparison against the reference implementation.** That is
  [pilot-differential.md](pilot-differential.md) (`go run ./cmd/pilot-diff`), which needs the pinned
  Java validators, is advisory, and is deliberately not wired into CI. This gate needs no validator:
  it is pure Go and runs in seconds, which is why it can gate every PR.
- **It does not adjudicate.** Like the [training-examples](training-examples.md) gate, the
  expectation file is a snapshot, so `-update-pilot-corpora` re-baselines a regression as quietly as
  it records a fix. Every count that moves must be judged against the OMG model that produced it
  before the new number is committed; a file going clean is only an improvement if the references it
  used to report now resolve, since a file also goes clean when a construct stops being checked.

## How the counts are measured

- Every file of a root is opened into one workspace **before** any diagnostic is read, because the
  corpora import across files: diagnosing a file while later ones are unopened would measure the
  alphabetical order of the corpus rather than the implementation. This is what the training gate
  and `cmd/pilot-diff` both do.
- Each root is loaded as one batch per language, mirroring `cmd/pilot-diff`, where a KerML file and
  a SysML file do not share a resource set.
- Diagnostics of **every** severity are counted, not errors alone, so a warning that appears or
  disappears is a movement the gate reports. Only the count is recorded, so a diagnostic that merely
  changes severity leaves the count untouched and passes. Paths are recorded relative to their root,
  so the file is machine-independent.
- The run sets `XDG_CACHE_HOME` to a temporary directory, so it measures the implementation on an
  empty semantic cache — what a fresh checkout and CI do — rather than the developer's machine.
- `TestPilotCorporaCacheStateIndependent` pins that a run restored from a populated library cache
  agrees with a cold one. `TestTrainingExamplesCacheStateIndependent` already asserts that property
  for SysML; this is the KerML half, which the training corpus contains no files for.

## Starting baseline

Generated from the code at the time the gate was added, and reproduced byte-identically across
repeated runs:

| Root | Files reporting diagnostics | Diagnostics |
|---|---|---|
| `sysml-examples` | 28 / 98 | 138 |
| `sysml-validation` | 6 / 56 | 22 |
| `kerml-examples` | 20 / 58 | 80 |

## Local runs

The corpora are not vendored, so the gate skips while they are absent — and announces the skip on
stderr with a `GATE NOT RUN` banner, because `go test` hides skip reasons without `-v` and a gate
that never ran must not look like a gate that passed. Fetch them once:

```bash
./scripts/download-pilot-corpora.sh
go test -count=1 ./internal/core/model -run TestPilotCorpora
```
