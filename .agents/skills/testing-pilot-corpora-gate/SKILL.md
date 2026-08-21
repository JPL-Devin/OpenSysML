---
name: testing-pilot-corpora-gate
description: How to verify the ratcheted OMG pilot-corpora CI gate (internal/core/model/pilot_corpora_test.go + testdata/pilot_corpora_expected.txt) end to end on Linux — running the gate, proving the baseline is reproducible/machine-independent, and the adversarial mutations that must fail.
---

# Testing the pilot-corpora ratchet gate

Shell-only; no GUI or recording needed. Runs in ~7s, so repeated runs are cheap.

## Provision + run

```bash
./scripts/download-pilot-corpora.sh            # examples/pilot-corpora is untracked/gitignored
OPENSYSML_REQUIRE_PILOT_CORPORA=1 go test -count=1 -v ./internal/core/model -run TestPilotCorpora
```

Expect `--- PASS` for `TestPilotCorporaDiagnostics` and `TestPilotCorporaCacheStateIndependent`,
plus three `N/M pilot corpus files clean` lines (baseline at the time of writing:
sysml-examples 70/98, sysml-validation 50/56, kerml-examples 38/58 — these move whenever
diagnostics change, so read the current numbers from
`docs/project/pilot-corpora.md` rather than hard-coding them).

Note the useful `-v` verdict lines are `t.Logf`, so they only appear with `-v`. Filtering test
output with `grep -E "a|b"` breaks when a pattern starts with `-`; use `grep -E -e "..." -e "..."`.

## Reproducibility of the baseline

```bash
go test -count=1 ./internal/core/model -run TestPilotCorporaDiagnostics -update-pilot-corpora
git diff --exit-code -- internal/core/model/testdata/pilot_corpora_expected.txt
```

Regeneration must be byte-identical across runs, from a different cwd
(`cd /tmp && go test -C <repo> ...`) and with a fresh `XDG_CACHE_HOME` — the test sets
`XDG_CACHE_HOME` to a temp dir itself, which is what makes it machine-independent. Also check no
absolute paths leak: `grep -nE '(^|\s)/(home|tmp|Users)' <expected file>` must find nothing.

## Adversarial mutations that must each fail (restore with `git checkout --` after each)

Mutate `internal/core/model/testdata/pilot_corpora_expected.txt`:
count up, count down, delete an entry, add an entry for a currently-clean file, change a
`# files: <root> <n>` total, replace the tab separator with a space, use a non-numeric count, drop
the `<n>` from a `# files:` line. Each must fail with a message naming the file/path (or
`<file>:<line>` for format errors). Also add a stray `.sysml` into a corpus root: the
`# files:` total check must catch it (a partial cache restore looks like this).

## Absence handling

Move a root aside, and also try an empty-but-present root and a root with only non-model files:
- with `OPENSYSML_REQUIRE_PILOT_CORPORA=1` → FAIL ("... is missing" / "holds no model files")
- without it → `--- SKIP` plus a `!!! GATE NOT RUN` stderr banner, which CI detects with
  `grep -qE '^\s*--- SKIP'` on the tee'd log.

## Tooling on this box

`actionlint`, `python3 -c "import yaml"`, `gofmt`, `go vet`, `make lint` (staticcheck+gosec, ~2min,
downloads tools via `go run`) all work. There is **no** `yamllint` and **no** `circleci` CLI, so
`.circleci/config.yml` can only be parsed as YAML, not schema-validated — say so in reports rather
than implying it was validated.

## Devin Secrets Needed

None.
