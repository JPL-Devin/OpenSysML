---
name: testing-pilot-execution-referee
description: How to verify the pilot execution referee (cmd/pilot-exec-diff + scripts/download-pilot-evaluator.sh) end to end on Linux — provisioning the headless pilot expression evaluator, reproducing the committed bucket counts, proving the determinism and disagree detectors are live, and the adversarial paths that actually distinguish working from broken.
---

# Testing the pilot execution referee

The execution referee compares model-level expression evaluation between the
pinned OMG SysML v2 pilot and OpenSysML. It does not compare actions, state
machines, token flow, traces, or exhibit/perform execution: those surfaces are
out of reach of the pinned pilot artifact.

## Provision

The pinned shaded jar and standard library must already be available from the
pilot validator setup (`build/pilot-validator/target/sysml-download/sysml/`,
provisioned by `./scripts/download-pilot-validator.sh` — slow, **check before
rebuilding**). Then:

```sh
./scripts/download-pilot-evaluator.sh
```

The script checks the pinned artifact, Java 21+, the standard library, and both
expression-evaluation classes before writing `build/pilot-evaluator`.

Provisioning notes worth knowing before you test it:

- There is **no already-built early exit**: every run recompiles and rewrites
  the launcher. Observed at `f3af23a2`: a second run reprints
  `Compiling ...` / `Built ... (pilot 2026-05, 0.60.1)`, exit 0, and both
  `classes/EvalSysML.class` and `eval-sysml` are **byte-identical**
  (same sha256) with only the mtime advancing. So assert idempotency by
  `sha256sum`, not by mtime or by an "already built" message.
- All fail-loud checks (`pilot_jar`, `library`, `source`, java/javac/jar, java
  version, the two required classes) run **before** `mkdir -p "$classes"`, so a
  failure leaves `build/pilot-evaluator` completely absent. Verify that by
  `mv build/pilot-evaluator /tmp/...` first, then `ls` after — a stale directory
  from a previous run makes the "wrote nothing" assertion vacuous.
- `PILOT_ARTIFACT_VERSION=wrong-version` → exit 1,
  `error: pilot shaded jar not found at .../jupyter-sysml-kernel-wrong-version-all.jar`.
  Moving the pinned jar aside gives the same error with the real version;
  moving `sysml.library` aside gives
  `error: pilot standard library not found at .../sysml.library`.
- **`PILOT_TAG` is not a real pin for this script.** `PILOT_TAG=bogus
  ./scripts/download-pilot-evaluator.sh` exits **0** and builds normally,
  printing `Built ... (pilot artifact 0.60.1)`. The artifact is located purely
  by `PILOT_ARTIFACT_VERSION`; `PILOT_TAG` is provisioned and checked by
  `download-pilot-validator.sh`, not used to locate it here.
- `grep -c jupyter-sysml-kernel-<version>-all.jar build/pilot-evaluator/eval-sysml`
  must be 1 and `__PILOT_ARTIFACT_VERSION__` must not survive in the launcher.
- `build/pilot-evaluator/eval-sysml --help` → exit 0 and
  `usage: eval-sysml --library DIR --cases FILE [--model FILE]...`.

## Run

```sh
go run ./cmd/pilot-exec-diff          # ~13 s wall with the default fixtures
```

Use `-cases DIR` for another directory of `.cases` files, `-out DIR`,
`-launcher PATH`, `-repo DIR`. Each `.cases` file lists `model: <repo-relative-path>`
lines followed by `id :: target :: expression` lines. Reports go to
`build/pilot-exec-diff/pilot-exec-diff.{txt,json}`.

Reference values at the current implementation (32 cases, both default
fixtures):
`agree 20 · kind-only 1 · order-only 0 · disagree 0 · pilot-unevaluated 2 ·
pilot-silent 2 · pilot-error 2 · ours-error 2 · both-error 3 ·
nondeterministic 0`.

## Checks that actually distinguish working from broken

- **Determinism.** Run twice into different `-out` dirs and diff the JSONs after
  stripping pilot UUIDs
  (`sed -E 's/\([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\)//g' | jq -S .`).
  The stripped diff must be empty; the **unstripped** diff must be non-empty and
  contain only `rawPilot` UUID lines (observed 100 diff lines) — that is what
  proves the stripping is doing real work rather than the comparison passing
  trivially.
- **`nondeterministic` is not vacuously 0.** Point `-launcher` at a throwaway
  `/tmp/flaky/eval-sysml` shell wrapper that calls the real launcher and, on its
  **second** invocation only (a counter file), `sed`s one result line to a
  different literal. The harness invokes the launcher twice per `.cases` file,
  so invocation 2 mutates the first file's second run. Observed: `dot-perform-out`
  moves to `nondeterministic`, `ours-error` drops 3 → 2, `nondeterministic: 1`.
- **`disagree` is reachable.** A *stable* wrapper (mutates every invocation,
  e.g. `LiteralInteger 7 ` → `LiteralInteger 8 `) puts case `int` in `disagree`
  with `pilot={"kind":"int","value":"8"} ours={"kind":"int","value":"7"}` and
  keeps `nondeterministic: 0`.
- **Artifact-absent path.** `-launcher /nonexistent/eval-sysml` exits **0**,
  prints `pilot execution artifact is absent at <path>; run
  ./scripts/download-pilot-evaluator.sh to provision it`, and creates **no**
  output directory (the check precedes any `MkdirAll`). Same with the real
  launcher moved aside. Delete/`-out` to a fresh path so an existing
  `build/pilot-exec-diff` cannot fake the assertion.
- **Malformed cases.** A case line with the wrong number of ` :: ` fields →
  exit 1, `pilot-exec-diff: <file>:<line>: expected id :: target :: expression`,
  no output dir. An unknown `model:` path → exit 1 and no output dir, with the
  source line included:
  `pilot-exec-diff: <file>:<line>: model no/such/model.sysml: stat <abs>: no
  such file or directory`.
- **Additivity.** `go run ./cmd/pilot-diff` must still print
  `349 file(s), 283 fully agreeing; 20 agreed diagnostic(s), 232 only ours, 139
  only the pilot's` and `jq -S` diff clean against
  `docs/project/pilot-differential-baseline.json`; `git status --porcelain`
  empty at the end.

## Interpret — and the empty-output trap

`agree` means the normalized results match. `disagree` means they do not.
`kind-only` records an exact numeric match whose kinds differ, while
`order-only` records a sequence with the same multiset in another order.
`pilot-unevaluated` means the pilot returned a non-Literal node,
`pilot-silent` means it emitted no output, and `pilot-error` means it emitted
`ERROR:` or `EXCEPTION:`. Together these are the four pilot-side states:
value, error, unevaluated, and silence. `ours-error` and `both-error` record
failures from either side.
`nondeterministic` takes precedence whenever either side differs from itself
between the two runs.

Both runtimes render values as sequences; single-element sequences are unwrapped
on both sides, so a scalar-versus-singleton distinction is unobservable here.
Reals are compared after rounding both sides to two decimal places, matching
OpenSysML's display precision. Pilot exponent-form rationals are parsed exactly
with `math/big` before integer-vs-real comparison.

**Known limitation to re-check on any change to `normalize.go`:** the pilot
emits *literally nothing* between its `== case`/`== end` markers both when an
expression legitimately evaluates to an empty sequence (`Probe::Empty()`) and
when it silently gives up (`1 / 0`). The harness cannot distinguish those
pilot behaviors, so both are reported as `pilot-silent` rather than as a
successful empty value. The report says explicitly that the pilot cannot
distinguish "no value" from "declined to evaluate" when it emits no output.
Reproduce by hand with
`build/pilot-evaluator/eval-sysml --cases <tsv> --model <model>`; the pilot
does report real failures as `ERROR:` or `EXCEPTION:` lines (for example,
`Probe::Nope` uses `ERROR:`).

## Reproducing a reported case by hand

The report's `rawPilot`/`rawOurs` are byte-reproducible (modulo pilot UUIDs):

```sh
printf 'big\t\tProbe::Big()\n' > /tmp/c.tsv          # id <TAB> target <TAB> expression
build/pilot-evaluator/eval-sysml --cases /tmp/c.tsv \
  --model cmd/pilot-exec-diff/testdata/models/expr_values.sysml
# → LiteralRational 1.099511627776E12 (<uuid>)   [~150 library "Reading ..." lines first]
make build-sysml
./bin/sysml -e 'Probe::Big()' cmd/pilot-exec-diff/testdata/models/expr_values.sysml
# → ✓ package Probe / ✓ Probe::Big() /   = 1099511627776
```

`bin/sysml` prints an extra `✓ package Probe` load line that the harness's
`repl.Session` path does not; ignore it when byte-comparing against `rawOurs`.
For a case with a non-empty target, our side goes through
`%eval in <target> : <expr>`, not `-e`.

## Devin Secrets Needed

None. Network access is required only for re-provisioning the pilot validator.

## Recording

CLI-only work; per the testing-mode guidance this needs no screen recording —
collect verbatim stdout/stderr and exit codes (captured without a pipe) instead.
