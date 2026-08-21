# Testing the pilot execution referee

The execution referee compares model-level expression evaluation between the
pinned OMG SysML v2 pilot and OpenSysML. It does not compare actions, state
machines, token flow, traces, or exhibit/perform execution: those surfaces are
out of reach of the pinned pilot artifact.

## Provision

The pinned shaded jar and standard library must already be available from the
pilot validator setup. Build the headless expression evaluator with:

```sh
./scripts/download-pilot-evaluator.sh
```

The script checks the pinned artifact, Java 21+, the standard library, and both
expression-evaluation classes before writing `build/pilot-evaluator`.

## Run

Run the default fixtures with:

```sh
go run ./cmd/pilot-exec-diff
```

Use `-cases DIR` for another directory of `.cases` files. Each file lists
`model: <repo-relative-path>` lines followed by `id :: target :: expression`
lines. Reports are written to `build/pilot-exec-diff/pilot-exec-diff.txt` and
`.json`.

## Interpret

`agree` means the normalized results match. `disagree` means they do not.
`kind-only` records an exact numeric match whose kinds differ, while
`order-only` records a sequence with the same multiset in another order.
`pilot-unevaluated` means the pilot returned a non-Literal node, and
`pilot-error`, `ours-error`, and `both-error` record failures from either side.
`nondeterministic` takes precedence whenever either side differs from itself
between the two runs.

Reals are compared after rounding both sides to two decimal places, matching
OpenSysML's display precision. Pilot exponent-form rationals are parsed exactly
with `math/big` before integer-vs-real comparison.
