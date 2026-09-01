# Performance census, September 2026

A measurement of where the toolchain stands after the past week of changes:
week-over-week benchmark movement, whole-binary scaling against the figures
recorded in `docs/internals/performance.md`, and a cross-implementation
comparison against the two independent SysML v2 validators the repository
already pins, prompted by a third-party claim of significantly faster runtime
on compliant SysML.

All figures were taken on one machine — `Intel Xeon Platinum 8559C`, 8 CPUs,
Go 1.25, Linux — the same processor family the figures in
`docs/internals/performance.md` were taken on, so the two sets compare
directly. Treat absolute numbers as ratios elsewhere.

## Week over week: `main` against `main` seven days earlier

`go test ./internal/repl -run '^$' -bench . -benchmem -count 6`, compared with
`benchstat`. The baseline is the merge at the top of `main` seven days before
the measurement (`01b2b86d`, 2026-08-25); the measured revision is `3e5bacb7`.
The benchmark's synthetic model changed between the two revisions — the
notation it used was removed from the grammar in the interval — so the two
sides load equivalent but not byte-identical models; the run benchmarks
(state machine, calc, instantiate) also execute under a runtime whose
completion semantics changed in the same interval.

What moved, with `benchstat` confidence (p ≤ 0.004 unless marked):

| figure | 2026-08-25 | 2026-09-01 | movement |
| ------ | ---------- | ---------- | -------- |
| load wall, 250/1000/4000 elements | 16.8 / 66.8 / 277 ms | 19.4 / 77.9 / 291 ms | +15% / +17% / +5% |
| load live heap, 250/1000/4000 | 2.49 / 9.77 / 39.0 MiB | 2.15 / 8.26 / 32.8 MiB | −14% / −16% / −16% |
| load allocations, 250/1000/4000 | 124k / 486k / 1.94M | 88k / 341k / 1.35M | −29% / −30% / −30% |
| empty-session load wall | 162 µs | 1024 µs | +532% |
| empty-session bytes allocated | 111 KiB | 695 KiB | +525% |
| state-machine start, 4000 elements | 158 µs | 221 µs | +40% |
| state-machine start bytes/allocs | 5.3 KiB / 81 | 9.0 KiB / 120 | +71% / +48% |
| diagnostics benchmarks | — | — | unchanged |

Loading holds noticeably less and allocates far fewer objects than a week ago,
at a modest wall-time cost at small sizes that fades by 4 000 elements. Two
movements deserve attention:

- **The empty-session floor grew six-fold.** A heap profile attributes the
  growth to `semantics.(*Model).collectAboutAnnotations`: building the
  `about`-metadata index walks every document in the index — the bundled
  standard library included — once per session, and the visited-symbol set for
  that walk is most of the new allocation. The cost is fixed per session
  (~0.9 ms, ~580 KiB of garbage; live heap is unchanged at 32 KiB), so a large
  model amortizes it, but a process serving many short sessions pays it each
  time. Restricting the walk to workspace documents unless the library itself
  declares `about` usages — a fact computable once when the library index is
  built — would restore the old floor.
- **Starting a state machine costs 40% more wall and 71% more memory** than a
  week ago at 4 000 elements. The interval removed the final-state marker in
  favor of completing on a transition to `done` and rewrote the benchmark's
  machine into standard notation, so the two sides execute different models;
  the movement is explained rather than adjudicated here, and the new figures
  are the baseline to watch.

## Whole-binary scaling

`sysml -validate` over generated compliant models in standard notation that
report nothing, three runs each. The documented figures are the table in
`docs/internals/performance.md`.

| elements | documented | measured, this census | peak RSS documented / measured |
| -------- | ---------- | --------------------- | ------------------------------ |
| 3 000    | 0.30 s     | 0.33 s                | 117 / ~120 MiB                 |
| 6 000    | 0.49 s     | 0.55–0.61 s           | 159 / ~180 MiB                 |
| 12 000   | 0.92 s     | 1.04 s                | 272 / ~285 MiB                 |

Both time and memory remain linear in the model. The measured points sit
5–12% above the documented ones and the seven-day-old binary measures the
same within noise, so the drift against the documented table predates this
week and is within machine-to-machine variation.

## Cross-implementation comparison

The claim under test was that an independent implementation validates
compliant SysML significantly faster. The two independent validators the
repository already pins were measured on the same machine, over the same
files:

- **SysIDE** (Sensmetry `sysml-2ls` 0.9.1, TypeScript/Node 22), via
  `scripts/download-syside.sh` and its batch driver. A static checker only,
  bundling the 2024-12 standard library. Loading that library costs it a
  fixed ~11 s and ~1.2 GiB per invocation, measured on a one-line file;
  the figures below include it, since a user validating a file pays it.
- **The OMG pilot implementation** (jupyter kernel 0.60.1, JVM), via the
  batch oracle `scripts/download-pilot-sysml-validator.sh` provisions. Its
  fixed per-invocation cost — JVM start-up plus its own library load,
  ~2 s on a one-line file — is included and is negligible at these sizes.
  This implementation's fixed cost on the same one-line file is 0.11 s.

Generated compliant single-file models, wall clock and peak resident size:

| elements | OpenSysML | SysIDE 0.9.1 | pilot 0.60.1 |
| -------- | --------- | ------------ | ------------ |
| 3 000    | 0.33 s, 120 MiB | 21.9 s, 2.6 GiB | 75 s, 1.3 GiB |
| 6 000    | 0.57 s, 180 MiB | 35.3 s, 3.0 GiB | — |
| 12 000   | 1.05 s, 285 MiB | 72.7 s, 4.1 GiB | 2 305 s, 4.8 GiB |

On large compliant models this implementation is **~65× faster than SysIDE
and ~2 200× faster than the pilot**, in 11–22× less memory, and is the only
one of the three whose wall time is linear in the model at these sizes (the
pilot's grows ~31× for a 4× model).

The OMG training corpus — 100 compliant files, 572 KiB, validated as one
batch — is the many-small-files shape rather than the one-large-model shape:

| tool | wall | peak RSS |
| ---- | ---- | -------- |
| OpenSysML | 6.7 s | 168 MiB |
| pilot 0.60.1 | 6.6 s | 895 MiB |
| SysIDE 0.9.1 | 13.0 s | 1.35 GiB, exits reporting findings of its own |

SysIDE's 13.0 s is mostly its fixed ~11 s library load, so its marginal cost
on the corpus is small; and here our advantage over the pilot disappears
entirely: it matches us. The census's one
substantive finding about our own runtime explains why.

## Batch validation of many files is quadratic

Validating the training corpus's 100 files together costs 6.7 s, but its two
halves cost 0.6 s and 3.4 s separately — the whole costs far more than its
parts. Per-file, every file is fast alone (the slowest is 0.33 s). A CPU
profile of the batch shows where the time goes: the CLI submits files to the
session one at a time, and every submission reindexes the workspace
(`model.(*Workspace).setOpenBuffer` → `reindexLocked`, 52% of the profile),
re-running `symbols.(*Index).ExpandWildcardImports` over every document
loaded so far — so a batch of *n* files expands wildcard imports *n* times
over a growing buffer, and the total is quadratic in the file count.

This is not a regression — the seven-day-old binary measures the same — but
it is the one workload shape where an independent implementation can
plausibly claim comparable or better runtime today, and the cost is in the
submission loop rather than in analysis itself. Submitting a `-validate`
batch as one indexing unit, or expanding wildcard imports incrementally for
documents the new submission cannot affect, would make the corpus cost what
its parts cost.

## Verdict on the claim

For what "runtime for compliant SysML" ordinarily means — the cost of
parsing, resolving and validating a compliant model — the claim has no
substance against this implementation: both independent validators are
between one and three orders of magnitude slower on the same compliant
models, in multiples of the memory, and neither scales linearly where this
implementation does. The defensible kernel of such a claim is the many-file
batch shape above, where our quadratic submission loop gives away most of
the analysis engine's advantage; that is ours to fix, and cheap to.
