- **Changelog entries are written as fragments under `changes/unreleased/`.** Every pull request
  used to append to the `## Unreleased` section of `CHANGELOG.md`, so any two open branches
  conflicted there. A change now adds one file, `changes/unreleased/<slug>.<section>.md`, and
  `python3 scripts/changelog.py release X.Y.Z` folds the fragments into a dated entry when a
  release is cut. `make docs-check` and CI validate the fragments.
