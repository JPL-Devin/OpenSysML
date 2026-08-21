---
name: testing-pilot-corpora-gate
description: How to verify the four OMG corpus gates (internal/core/model/corpus_gate_test.go + pilot_corpora_test.go + training_examples_test.go and their testdata expectations) end to end on Linux — running both gates, proving the baselines are reproducible/machine-independent, the adversarial mutations that must fail, absence handling per root, and exercising the shared pilot-pin.sh downloader.
---

# Testing the OMG corpus gates (training assertion + pilot-corpora ratchet)

Shell-only; no GUI or recording needed. One full `./internal/core/model` run is ~60s; the two
corpus tests alone are ~4s, so iterate with `-run` and only do the full run at the start/end.

Four pinned OMG model roots, **one mechanism, two policies** (`internal/core/model/corpus_gate_test.go`):

| root | dir | policy | expectation file |
|---|---|---|---|
| training | `examples/sysml-v2-training` | **assertion** — errors only, `.sysml` only, must be 100% clean, no per-file counts allowed | `testdata/training_examples_expected.txt` |
| sysml-examples | `examples/pilot-corpora/sysml-examples` | per-file **ratchet**, every severity | `testdata/pilot_corpora_expected.txt` |
| sysml-validation | `examples/pilot-corpora/sysml-validation` | ratchet | same |
| kerml-examples | `examples/pilot-corpora/kerml-examples` | ratchet | same |

## Provision + run

```bash
./scripts/download-training-examples.sh   # examples/sysml-v2-training  (untracked/gitignored)
./scripts/download-pilot-corpora.sh       # examples/pilot-corpora      (untracked/gitignored)
OPENSYSML_REQUIRE_TRAINING_CORPUS=1 OPENSYSML_REQUIRE_PILOT_CORPORA=1 \
  go test -count=1 -v ./internal/core/model
```

The corpora are gitignored, so **copy them aside first** (`cp -a examples/pilot-corpora
examples/sysml-v2-training /tmp/pristine/`) — every absence/mutation test moves or edits them and
`git checkout --` cannot restore them. `diff -r` against the pristine copy after each case.

Expect `--- PASS` for `TestTrainingExamplesSemanticErrors`, `TestPilotCorporaDiagnostics` and
`TestCorpusGatesCacheStateIndependent` (subtests `training` and `pilot-corpora` — the older
`TestTrainingExamplesCacheStateIndependent` / `TestPilotCorporaCacheStateIndependent` no longer
exist), plus `100/100 training files clean` and three `N/M pilot corpus files clean` lines
(observed 2026-08: sysml-examples 76/98, sysml-validation 52/56, kerml-examples 40/58 — these move
whenever diagnostics change, so read the current numbers from `docs/project/pilot-corpora.md`
rather than hard-coding them).

Note the useful verdict lines are `t.Logf`, so they only appear with `-v`, and the ratchet test
logs one `pilot_corpora_test.go:37:` line per reporting file — filter those out
(`grep -vE 'pilot_corpora_test\.go:37:'`) or the real assertion messages scroll off. Filtering test
output with `grep -E "a|b"` breaks when a pattern starts with `-`; use `grep -E -e "..." -e "..."`.

## Reproducibility of the baselines

```bash
go test -count=1 ./internal/core/model -run TestPilotCorporaDiagnostics    -update-pilot-corpora
go test -count=1 ./internal/core/model -run TestTrainingExamplesSemanticErrors -update-training
git diff --exit-code -- internal/core/model/testdata/
```

Regeneration must be byte-identical across runs, from a different cwd
(`cd /tmp && XDG_CACHE_HOME=$(mktemp -d) go test -C <repo> ...`) and with a fresh `XDG_CACHE_HOME` —
each test sets `XDG_CACHE_HOME` to a temp dir itself, which is what makes it machine-independent.
Also check no absolute paths leak:
`grep -nE '(^|[[:space:]])/(home|tmp|Users)' internal/core/model/testdata/*_expected.txt` must find nothing.

## Adversarial mutations that must each fail (restore with `git checkout --` after each)

Mutate `internal/core/model/testdata/pilot_corpora_expected.txt` and re-run
`OPENSYSML_REQUIRE_PILOT_CORPORA=1 go test -count=1 ./internal/core/model -run TestPilotCorporaDiagnostics`:

| mutation | expected message |
|---|---|
| count up | `<path>: 2 diagnostic(s), expected 3 (fewer than recorded); adjudicate ...` |
| count down | `... (more than recorded) ...` |
| delete an entry | `<path>: N new diagnostic(s), previously clean; adjudicate ...` |
| add an entry for a currently-clean file | `<path>: expected 1 diagnostic(s) but the file is now clean` |
| change a `# files: <root> <n>` total | `<root> holds 98 model file(s), expectations were recorded against 97` |
| tab → space on a data line | `pilot_corpora_expected.txt:<line>: want "<count>\t<path>", got ...` |
| non-numeric count | `pilot_corpora_expected.txt:<line>: bad count: strconv.Atoi ...` |
| drop `<n>` from a `# files:` line | `pilot_corpora_expected.txt:<line>: want "# files: <root> <n>", got ...` |
| stray extra `.sysml` copied into a root | the `# files:` total check (`holds 99 ... recorded against 98`) |

Pick a stable victim entry with a literal `sed` match (e.g.
`2\tkerml-examples/Simple Tests/Associations.kerml`); to find a *clean* file for the "added entry"
case, `comm -23` the root's `find`-listing against the paths already in the expectation file.

## The training gate is an assertion, not a ratchet — prove it

Two distinct properties, both worth testing:

1. Adding any `<count>\tpath` line to `training_examples_expected.txt` (which is otherwise
   comment-only plus `# files: 100`) must fail with
   `... records 1 per-file count(s) ([...]); the training corpus is asserted clean, not ratcheted, ...`
2. With a real error injected into a training file (append
   `package BogusForTest { part x : NoSuchTypeAnywhere; }` to an existing `.sysml`), the plain gate
   fails with `<path>: 1 semantic error(s); the training corpus must stay clean`, **and**
   `-update-training` must also fail with
   `1 file(s) report errors ([...]); the training gate asserts a clean corpus, so -update-training cannot record them`
   leaving the expectation file byte-identical.

Pitfall: `printf ... >> "examples/sysml-v2-training/<guessed name>.sysml"` silently *creates* a new
file if the name is wrong, which trips the `# files: 100` total check instead of the error check and
leaves an untracked corpus file behind. `ls` the directory and confirm the file exists first
(e.g. `01. Packages/Package Example.sysml` is real; `01. Packages/Packages.sysml` is not).

## Absence handling — test all four roots separately

For each of the four roots, in three shapes (moved aside / empty-but-present / only a `README.txt`):

- with the root's require-env (`OPENSYSML_REQUIRE_TRAINING_CORPUS` or
  `OPENSYSML_REQUIRE_PILOT_CORPORA`) → FAIL, `<env>=1 but <dir> is missing: ...` or
  `... holds no model files: ...`
- without it → exit 0 with `--- SKIP` plus the `!!! GATE NOT RUN` stderr banner naming the right
  fetch script for that gate.
- CI catch: `go test -count=1 -v ./internal/core/model -run 'TestTrainingExamples|TestCorpusGates' | tee corpus-gate.log`
  then `grep -qE '^\s*--- SKIP' corpus-gate.log`. Note an absent **pilot** root surfaces in that CI
  command only as `--- SKIP: TestCorpusGatesCacheStateIndependent/pilot-corpora` (the ratchet test
  itself is not in the `-run` pattern) — the grep still catches it, and that is worth verifying.

## Shared downloader (`scripts/pilot-pin.sh`)

`pilot_fetch_subtrees "<path in pilot repo>:<dest>" ...` is the single fetch behind
`download-training-examples.sh` (1 entry) and `download-pilot-corpora.sh` (3 entries).

- Already-present destinations print `Already present at <dest>` and are skipped; if *all* are
  present no clone happens at all.
- Re-fetch a moved-aside root and `diff -r` it against the pristine copy — must be identical
  (58 kerml-examples, 98 sysml-examples, 56 sysml-validation, 100 training model files).
- To prove pinning/sparseness without touching the script, put a logging `git` wrapper first on
  `PATH` (`echo "$*" >> log; /usr/bin/git "$@"`, and after `sparse-checkout` also dump
  `git -C <dir> sparse-checkout list`, `git -C <dir> log -1 --decorate` and `ls <dir>`). Expect
  `clone --quiet --filter=blob:none --sparse --depth 1 --branch 2026-05 ...`,
  `sparse-checkout set <only the requested paths>`, HEAD decorated `tag: 2026-05`, and no
  unrequested subtree on disk.
- Error paths: a bogus source path exits 1 with
  `error: <path> is missing from <repo> at <tag>`; `PILOT_TAG=9999-99` exits 128 with
  `fatal: Remote branch 9999-99 not found in upstream origin` (the `set -e` then produces a
  follow-on `cannot change to '<tmp>/pilot'` line — noisy but non-zero).

## Tooling on this box

`actionlint`, `shellcheck`, `python3 scripts/check-doc-links.py`, `gofmt`, `go vet`,
`go run ./cmd/pilot-diff` (validators pre-downloaded; ~4min, prints e.g.
the headline the committed baseline holds — `349 file(s), 291 fully agreeing; 20 agreed
diagnostic(s), 167 only ours, 139 only the pilot's` after the wave-5 rebaseline, so read it from
`docs/project/pilot-differential-baseline.json` rather than from this line)
and `make lint` (staticcheck+gosec, ~2min) all work. There is **no** `yamllint` and **no**
`circleci` CLI, so `.circleci/config.yml` can only be parsed as YAML, not schema-validated — say so
in reports rather than implying it was validated.

## Devin Secrets Needed

None. Network access to github.com is required for the downloader tests.
