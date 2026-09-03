<!-- Title follows conventional commits: feat(parser): ... / fix(runtime): ... / docs: ... -->

## What and why

<!-- What changed, and the reason it is needed. Link the issue it closes, if any. -->

## Specification basis

<!-- For a behaviour change: the clause it implements (document, version, section), and whether it
     moves a row in docs/project/spec-compliance.md. Delete this section for a pure refactor,
     tooling or docs change. -->

## How it was verified

<!-- The tests or gates that now cover this, and anything you checked by hand. -->

## Checklist

- [ ] `make test` and `make lint` pass locally
- [ ] Tests added or updated for the change
- [ ] Documentation extended where it already covers the surface (see CONTRIBUTING.md)
- [ ] Changelog entry added as `changes/unreleased/<slug>.<section>.md`, not as an edit to `CHANGELOG.md`
- [ ] `make docs-counts` run if a compliance row changed; baselines regenerated if a gate count moved
- [ ] No internal work-item labels (waves, slices, `F4`, `K5`) in the body, docs, or changelog
