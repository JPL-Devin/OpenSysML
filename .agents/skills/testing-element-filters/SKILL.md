---
name: testing-element-filters
description: How to observe SysML element filters (`filter <expr>;`, `import P::*[@T]`, `expose X::**[@T]`) end to end through the sysml REPL/CLI — which surfaces actually apply a filter, how to build a library-backed model so the on-disk index cache is exercised, and the fixture shapes that avoid false negatives.
---

# Testing element filters through the CLI/REPL

## The only observable surface is diagnostics, not `%instantiate`

`internal/repl/lookup.go` resolves a qualified name with `idx.LookupQualified` and a simple name with
`resolve.New(idx).LookupName` **without** `SetModel`, so no filter is ever evaluated on that path:
`%instantiate Facade::hiddenPart` succeeds even for an element a filter rejects. Do not read that as
"filters do not work".

What *does* apply filters is document analysis (`passes/pass.go` calls `Resolver().SetModel`). So the
test surface is the **unresolved-reference diagnostics** the REPL prints when a document is loaded:

```
package Client { private import Facade::*; part b :> hiddenPart; }   // -> error: unresolved reference: hiddenPart
package Client { part c :> Facade::hiddenPart; }                     // -> the qualified route
```

Fixture shapes that matter (copy them; the alternatives produce misleading results):

- Reference an imported *usage* with subsetting `part x :> seatBelt;`. Typing it (`part x : seatBelt;`)
  fails for an unrelated reason (`type must be a definition, found partUsage`).
- Prefer the metadata type **qualified** in the condition (`filter @Meta::Safety;`) when you want a
  known-good control. An unqualified `@Safety` reachable only through `public import Meta::*` used to
  be reported `filter-not-evaluable: the metadata type Safety does not resolve` (filter then **not
  applied**, so nothing is hidden and the test silently passes for the wrong reason). That path is
  expected to work now (a namespace's own filter must not gate lookups inside its own body), so
  assert both spellings and treat a `does not resolve` warning as a regression.
- A view's `expose` only surfaces names **inside the view body**; `import SomeView::*` from outside
  surfaces nothing at all, filtered or not. Put the `expose` and the `part a :> x;` references in the
  same `view { }`.
- A recursive `import X::**` does not surface grandchildren (members of a nested package/part) even
  unfiltered, so a "filtered recursive import hides X" assertion needs an unfiltered control run or
  it proves nothing.
- Any earlier-level error in the document (e.g. one `unresolved reference:`) suppresses the
  `filter-not-boolean` / `filter-not-evaluable` diagnostics (`ElementFilterPass` runs at
  `LevelType`). Keep the bad-filter fixture otherwise clean, or the diagnostics never appear.
- Corollary: a filter whose condition names an **unresolvable** metadata type (`filter @NoSuchType;`)
  never produces `filter-not-evaluable` — it produces a name-resolution error
  (`unresolved reference: NoSuchType`), which then suppresses the filter pass. The filter is simply
  not applied. To assert the `filter-not-evaluable` warning, use a condition that resolves but is
  outside the supported subset (`filter @Meta::Safety + 1;`).
- Filters at a document's **root** (no enclosing package) are a separate code path: both
  `import Lib::*[@Lib::Safety];` and a bare `filter @Lib::Safety;` written at file top level gate the
  resolver's top-level fallback (`resolve/qualified.go lookupGlobalTop`). Root filters are **per
  document**, so proving that needs a *second* document — and `%load` does not give you one (it just
  submits text into the single REPL session buffer). Supply the second document as a library file
  under `SYSML_LIBRARY_PATH` (e.g. `$LIB/Roots/other.sysml` containing its own root
  `import Lib::*;`), then assert both directions: the other document's unfiltered root import must
  not defeat the session document's root filter, and the other document's root filter must not
  restrict the session document.
- A typo inside an import's filter clause (`import Lib::*[@NoSuchMeta]`) is reported as
  `unresolved reference: NoSuchMeta`; check for exactly one such diagnostic (no duplicate) and that a
  valid clause contributes none of its own.
- Strongest regression technique: build the **previous commit** into a second binary
  (`git worktree add /tmp/wtN <sha> && go build -o /tmp/sysml-<sha> ./cmd/sysml`), run the whole
  fixture sweep through both binaries and `diff` the outputs. An empty diff proves no diagnostic was
  gained or lost, and the old binary doubles as before/after evidence for the fix under test.
- A `filter` member declared in a **definition or usage body** (`view v { expose X::*; filter @T; ... }`,
  and the same in a `part def` / `part usage` / `package` body) restricts what the imports declared
  beside it bring into that body. Test all of `expose X::*`, `expose X::**` and plain
  `private import X::*`, each with an unfiltered twin as the control, and reference one annotated and
  one unannotated name in each body.

Expected diagnostic text:

```
filter 3;                 -> error:   a filter condition must be boolean-valued, but this one yields an integer (KerML 8.2.4)
filter @Meta::Safety + 1; -> warning: this filter condition cannot be evaluated, so it selects nothing and is not applied: the `+` operator is not supported in a filter condition
```

## Exercising the on-disk index cache (cache-hit vs cache-miss equivalence)

A REPL session document is never cached; only *library* files are. Make the model a library:

```bash
cp -r internal/core/libs/stdlib/* /tmp/flib/          # SYSML_LIBRARY_PATH REPLACES the stdlib, so copy it
mkdir -p /tmp/flib/Filters && cp model.sysml /tmp/flib/Filters/
export SYSML_LIBRARY_PATH=/tmp/flib XDG_CACHE_HOME=/tmp/fc
rm -rf /tmp/fc                                        # run 1 = cache miss (parsed), run 2+ = cache hit (restored)
printf '%%load /tmp/client.sysml\n%%quit\n' | timeout 180 ./bin/sysml
```

- **Pitfall:** `VAR=x printf ... | ./bin/sysml` sets the variable for `printf` only. `export` it, or
  the run silently uses the embedded stdlib and your library is absent (`symbol "Lib::X" not found`).
- Cache files land in `$XDG_CACHE_HOME/sysml-ls/libs/*.idx` (~96 files for the stdlib); the key
  includes the format version, so a bump makes old entries unreachable rather than stale.
- Compare runs by grepping just the verdicts, which keeps a diff readable:
  `... | grep -o "unresolved reference: [A-Za-z:]*" | sort | uniq -c`
- Two things are known to differ between a parsed and a restored library and are worth asserting
  every time: whether a **qualified** name into a filtered namespace resolves, and whether a
  *nested* facade (`Outer::Facade::x`, i.e. a 3+-segment qualified name) resolves at all. A restored
  index has populated scopes and a parsed one may not, and only one of the two routes applies the
  filter gate. Run at least three runs (miss, hit, hit) and A/B against a `main` binary built in a
  worktree: a nested-qualified difference between run 1 and run 2 also reproduces on `main`.
- **Also diff the *whole* diagnostic list, not just the filter verdicts.** A cache-miss run can emit
  an extra diagnostic whose span belongs to a library file but is printed against the user's document
  (e.g. `18:22937: error: unresolved reference: Feature` on a 17-line file). Sanity check: any
  reported line/column beyond `wc -l` of the loaded file is a misattributed library span. That leak
  was fixed (`resolve.Resolver.aside`: a lookup made for a semantic query does not report) and is
  locked by `model/element_filter_test.go` `TestFilterEvaluationReportsOnlyThisDocument`, but the
  whole-list diff is still the check that catches its return.

## Driving a filter being added and removed without an LSP client

The REPL re-analyses the **whole buffer** on every submission and a submission redeclaring an
existing name replaces the earlier one, so filter invalidation is testable at the prompt:

```
./bin/sysml -debug
package Facade { public import V::vehicle::*; }                        # no filter
package C1 { private import Facade::*; part x :> keylessEntry; }       # resolves
package Facade { public import V::vehicle::*; filter @Meta::Safety; }  # add filter
                                     # -> C1 (earlier line) now errors: routes re-derived
package Facade { public import V::vehicle::*; }                        # remove filter
                                     # -> 0 diagnostics over the whole buffer again
```

`-debug` prints `submission at buffer line N; K diagnostic(s) over the whole buffer`, which is the
clearest evidence that the retroactive re-derivation happened.
