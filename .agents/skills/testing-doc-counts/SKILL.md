---
name: testing-doc-counts
description: How to end-to-end test the generated documentation figures (cmd/doc-counts + internal/doccounts + `make docs-counts`) on Linux — proving `-check` is a real gate, that the block consumers cannot drift, that marker mutations fail loudly, and that no measured number moved.
---

# Testing the generated documentation figures (`cmd/doc-counts`)

`cmd/doc-counts` regenerates four kinds of derived documentation from committed sources:

1. the census header line in `docs/project/spec-compliance.md` (from that file's own status markers);
2. single-copy baseline lines in `README.md` (`**Reference differential:**`, `**Rejection oracle:**`);
3. the HTML-comment-delimited named block `<!-- doc-counts:begin refereed-figures -->` …
   `<!-- doc-counts:end refereed-figures -->`, rendered from **one** template in
   `internal/doccounts/doccounts.go` into **two** consumers (`README.md` and
   `docs/internals/architecture.md`), differing only by `Block.LinkPrefix`
   (`docs/project/` vs `../project/`);
4. the `landing-figures` block in `overrides/home.html` — the same census as markup, for the
   documentation site's landing band. Its links are emitted as MkDocs `|url` filters, so
   `scripts/mkdocs_landing.py` validates them and `make docs PYTHON=~/pv/bin/python` fails
   `--strict` when a conformance record is renamed.

Inputs are `docs/project/spec-compliance.md` and the three committed baselines
`docs/project/pilot-{differential,xpect,rejection}-baseline.json` (`doccounts.ReadRefereedCounts`).

`make docs-counts` = generate → `go run ./cmd/doc-counts -check` → `go test -count=1 ./cmd/pilot-diff
./cmd/pilot-reject ./cmd/doc-counts`.

## Never test in a checkout someone else is using

Use `git worktree add /home/ubuntu/wt-<name> <sha>`. Put it **under `/home/ubuntu`, not `/tmp`**
(tmpfs is a different device, so the `cp -al` hardlink provisioning below fails there), then
hardlink-copy the gitignored provisioning in from a provisioned checkout — this takes seconds and
costs no disk:

```bash
mkdir -p /home/ubuntu/wt-x/build
for d in pilot-validator pilot-sysml-validator pilot-kerml-validator pilot-xpect-corpus pilot-evaluator; do
  cp -al /home/ubuntu/repos/OpenSysML/build/$d /home/ubuntu/wt-x/build/$d; done
cp -al /home/ubuntu/repos/OpenSysML/examples/sysml-v2-training /home/ubuntu/wt-x/examples/
# examples/pilot-corpora already exists in a worktree: stage the hardlink copy aside and swap it in
```
Copy **all** `build/pilot-*` dirs together: the validator launchers resolve the shaded jar through
`$SCRIPT_DIR/../pilot-validator/...`, so copying one alone gives the silent-zero
`jar not found` run that still exits 0 (see `testing-pilot-differential`).

## The checks that actually distinguish working from broken

- **Idempotence:** `make docs-counts` twice; both must print `doc-counts: already current` for the
  generate *and* the `-check` step, and `git status --short` must stay empty.
- **`-check` is a gate, not decoration:** perturb one number *inside* the block in one consumer.
  `go run ./cmd/doc-counts -check` must exit **1**, print `doc-counts: <that path> is stale` plus a
  `--- <path> (current) / +++ <path> (generated)` diff with `@@ line N @@` hunks, name **only** that
  file, and leave the file's `sha256sum` unchanged. Then the plain generator must restore it
  byte-identically to the committed hash.
- **Read-only `-check`:** `chmod 444` the stale file — `-check` must still exit 1 with the same
  stale message (it must not attempt a write). The plain generator on the same read-only file must
  exit 1 with `open <path>: permission denied` and write nothing (writability of *all* pending files
  is checked before any is written).
- **Baseline propagation:** mutate one figure in each baseline JSON in turn; `-check` must name
  **every** consumer stating it — both Markdown pages, and `overrides/home.html` too when the
  figure is one of the four the landing band states — the guard tests must fail while the tree is stale
  (`TestCheckCommittedTreeIsCurrent`, `TestPilotDifferentialDocumentCountsMatchBaseline`,
  `TestW6FXpectDocumentCountsMatchBaseline`, `TestPilotRejectionDocumentCountsMatchBaseline`), and
  regeneration must write the new number into every one of them. Restore with `git checkout -- .`.
  **Pick a figure that is not cross-constrained.** Mutating the xpect `errors` kind's `rows`
  (510→509) makes `Silent = rows - agree - sameLocation - sameLine - severityDiffers -
  elsewhereInFile` negative, so the tool correctly *errors*
  (`errors agreements and tolerances exceed 509 rows`) instead of propagating. To exercise
  propagation use the `scope` kind's `agree`, the differential's `totals.filesFullyAgreeing`, or
  the rejection baseline's `totals.bothReject`.
- **Marker mutations must fail loudly:** delete the end marker, duplicate the begin marker, delete
  the whole block. Each must make *both* the generator and `-check` exit 1 with
  `named block "refereed-figures" is missing or unterminated` or
  `duplicate "<!-- doc-counts:begin refereed-figures -->" marker`, and `wc -c` on the file must be
  unchanged (no truncation, no `already current`).
- **The landing band states the same figures:** its four numbers must be the differential's
  `filesAgreeing`/`files`, the Xpect silence and scope pairs, and the rejection corpus's
  by-default gap — the same values the prose block states. Build the site and grep
  `site/index.html` for them, since a Jinja mistake renders as literal `{{ … }}`.
- **The two Markdown consumers cannot drift:** extract each block with
  `sed -n '/doc-counts:begin refereed-figures/,/doc-counts:end refereed-figures/p'`, normalise
  `(../project/` → `(docs/project/` in the architecture copy, and `diff` — must be empty. Then
  `test -f` all four link targets from each consumer's own directory.
- **No measured number moved:** compare digit *sequences*, not the files:
  `git show main:README.md | grep -o '[0-9][0-9]*'` vs the same on HEAD, `diff` must be empty (same
  for `docs/internals/architecture.md`), and `git diff main -- 'docs/project/pilot-*-baseline.json'`
  must be empty. This is the cheapest proof a "generate it instead of hand-maintaining it" refactor
  restated exactly what was there.
- **Live oracle reproduction is a separate claim** from doc↔baseline consistency: the guards read
  only committed JSON. Run all three under a fresh cache
  (`XDG_CACHE_HOME=$(mktemp -d) go run ./cmd/pilot-{xpect,reject,diff} -out /tmp/oN`) and `cmp`
  each against its committed baseline.

## Gotchas

- `sed -i 's/…230 of 230…/…229 of 230…/'` silently matches nothing if you guessed the number: always
  `grep -c` the mutated text and confirm it is 1 before believing an `already current` result.
- Both errors and the stale-file diff go to different streams: the diff report is on **stdout**,
  hard errors on **stderr**. Capture both and the exit code without a pipe.
- Find a python with mkdocs before building: run `python3 -m mkdocs --version` first. `python3` on
  PATH may be another repo's venv (sometimes one that *does* have mkdocs), and the blueprint's
  `~/pv` venv may lack mkdocs even though the maintenance step installs `docs-requirements.txt`
  there — fall back to `pip install -r docs-requirements.txt` into whichever interpreter you use.
  The build prints a red mkdocs-2.0 deprecation banner and still exits 0. `rm -rf site` afterwards.

## Recording

Shell-only; no GUI, so no recording is needed.

## Devin Secrets Needed

None. Network to github.com / Maven Central is needed only to re-provision the pilot validators.
