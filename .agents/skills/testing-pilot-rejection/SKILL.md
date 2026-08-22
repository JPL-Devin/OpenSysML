---
name: testing-pilot-rejection
description: How to verify the advisory pilot rejection oracle (cmd/pilot-reject + its committed negative corpus) end to end on Linux — provisioning the two pinned reference validators, reproducing the committed baseline, proving determinism, extending the corpus, and inspecting permissiveness gaps.
---

# Testing the pilot rejection oracle (`cmd/pilot-reject`)

Sibling of `testing-pilot-differential` and `testing-pilot-xpect` (same pin
`scripts/pilot-pin.sh`, same committed-baseline shape), but pointed the other way: the
differential measures what the reference accepts and we reject; this oracle measures what the
reference **rejects and we accept** — permissiveness gaps. Its corpus is committed under
`cmd/pilot-reject/testdata/negative/` (34 hand-written invalid models, one violated rule + citation
in each file's mandatory `// Invalid: ...` first line), so no corpus download exists. Method and
findings: `docs/project/pilot-rejection.md`.

## Prerequisites

- `export PATH=/usr/local/go/bin:$PATH`, Java 17+, Maven, network to github.com and Maven Central.
  No secrets of any kind are required.
- `./scripts/download-pilot-reject-validators.sh` — provisions both pinned reference validators
  (`build/pilot-sysml-validator/validate-sysml-batch` and
  `build/pilot-kerml-validator/validate-kerml`) by delegating to their own download scripts.
  First run downloads the pinned pilot jar (~minutes); re-runs are no-ops without `--force`.

## The core check (~10 s per run once provisioned)

```bash
rm -rf build/pilot-reject && go run ./cmd/pilot-reject
cmp build/pilot-reject/pilot-reject.json docs/project/pilot-rejection-baseline.json   # must be silent
```

Observed on the wave-8E branch (after 8A, 8B, 8C and 8F landed):
`34 case(s): 31 both reject, 3 only the pilot rejects, 0 only we reject, 0 both accept`,
byte-identical to the committed baseline. Any `both accept` case is a bug in the corpus (the case
is not actually invalid under the loaded standard library) — fix the case, never ignore it.

## Conformance policy (`-conformance auto|default|strict`)

The baseline is the default `auto` policy: the `extensions/` cases are judged under strict
conformance (OpenSysML notation the reference rejects as a syntax error), everything else in the
default mode. The report names each case's mode and lists the five strict-only agreements
separately, so a strict agreement never reads as a default one.

```bash
go run ./cmd/pilot-reject -conformance default -out build/pilot-reject-default
# 34 cases: 26 agreements, 8 gaps — the numbers strict mode leaves alone
go run ./cmd/pilot-reject -conformance lenient   # must fail: unknown conformance policy
```

The `default` numbers are the ones the other waves' rules produce and must not move: strict mode is
opt in and changes no default verdict.

## Determinism

The reports carry no timestamps and no absolute paths; case order is the sorted corpus paths.
Prove it with two independent runs:

```bash
go run ./cmd/pilot-reject && go run ./cmd/pilot-reject -out build/pilot-reject-2
cmp build/pilot-reject/pilot-reject.json build/pilot-reject-2/pilot-reject.json   # must be silent
rm -rf build/pilot-reject-2
```

## Adversarial behavior worth testing

- **A corpus file without the mandatory header** fails the run with
  `first line must be `// Invalid: <rule> (<citation>).`` — the header is what keeps every case
  non-anecdotal.
- **Missing validators** fail fast with the provisioning command to run, before any validation.
- **Unattributable pilot output** (a diagnostic line for a path outside the corpus) is echoed to
  stderr, never silently dropped — a swallowed error would masquerade as an acceptance.
- **Warnings do not count as rejection** on either side; only error-severity diagnostics do.

## Extending the corpus

Add a file under the subdirectory naming its derivation (`grammar/`, `extensions/`, `xpect/`), give
it the mandatory first-line header, re-run, and inspect its bucket. Then recommit the baseline
(`cp build/pilot-reject/pilot-reject.json docs/project/pilot-rejection-baseline.json`) and update
the counts and gap table in `docs/project/pilot-rejection.md`, the README's rejection-oracle line,
and the headline above. `TestPilotRejectionDocumentCountsMatchBaseline` (CI-cheap, reads only
committed files) fails on any stale count, and on a gap table that does not enumerate exactly the
baseline's `pilot-only-rejects` cases:

```bash
go test -count=1 ./cmd/pilot-reject
```

## Inspecting a gap

Every `pilot-only-rejects` case keeps the pilot's error messages in the JSON as evidence:

```bash
jq '.cases[] | select(.bucket=="pilot-only-rejects") | {path, rule, pilot}' build/pilot-reject/pilot-reject.json
```

The corpus file itself is the minimal reproducer. `docs/project/pilot-rejection.md` maps each gap
to the package its root cause is likely in. Gaps are findings to report, not to fix from this
harness's PRs.

## Limitations

The corpus tests the invalid models someone thought to write — a sample of the rejection surface,
not a proof. Some pilot Xpect negatives only error in a library-less resource set (with the
standard library loaded, e.g. `feature f;` gets an implicit type and is legal), so they cannot be
cases here. The pilot's verdicts are externally refereed; the root-cause attributions are
self-assessed.
