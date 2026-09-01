# OpenSysML modelled in SysML v2

This is OpenSysML's own architecture written in the language OpenSysML implements: the
analysis pipeline as parts, ports and item flows, the tier ladder and the two execution
engines as state machines, the invariants of
[AGENTS.md §4](../../AGENTS.md) as requirements the tool itself checks, and views that
render the diagrams. Nothing here is a special case in the tool — it is analysed,
executed and rendered by the same `bin/sysml` any other model goes through.

Two things follow from that, and are the reason it lives in the repository rather than in
the documentation as prose. The architecture diagrams are *generated* from one model, so
they cannot drift apart from each other; and the invariants are *evaluated*, so a claim
that stops describing the implementation fails a test
([`../self_model_test.go`](../self_model_test.go)) instead of quietly reading as true in a
diagram.

The five files:

| File | What it holds |
| --- | --- |
| [pipeline.sysml](pipeline.sysml) | `OpenSysMLArtifacts` — what travels between stages (source text, tokens, tree, symbol index, side tables, diagnostics, IR graphs, traces, RDF, document trees), the ports and channels it travels over, and the layer metadata the filtered views select on. `OpenSysMLPipeline` — the thirteen stages from `internal/core/source` to `internal/core/solve`, each naming the Go package that implements it, the five validation passes, and `AnalysisPipeline`, which wires them together |
| [behavior.sysml](behavior.sysml) | one document analysed end to end (`AnalyzeDocument`, whose four decision nodes are the tier gates), the editor's edit-then-sweep path (`ServeEdit`), and four state machines: the validation tier ladder, the runtime's five tiers, Petri-net token flow with its deadlock and budget exits, and run-to-completion event dispatch with deferral |
| [surfaces.sysml](surfaces.sysml) | the four interfaces over one pipeline (REPL, LSP, gRPC service, CLI), the four generated clients and the VS Code extension, the document path from query to Markdown or PDF, the exporter, the six conformance oracles with their committed baselines, and `Toolchain`, which holds all of it |
| [quality.sysml](quality.sysml) | eight architecture invariants as `requirement def`s bound to the modelled parts, the test runs that verify them as `verification def`s, the contributor's use case, and the allocation of every logical unit onto its directory in the source tree |
| [views.sysml](views.sysml) | sixteen views — the pipeline and toolchain as interconnection diagrams, the stage and invariant tables, four action/state flows, the four architectural layers as filtered exposes, and an overview that frames a maintainer's concern |

## Analyse it

```bash
./bin/sysml examples/self-model/*.sysml -validate
```

```
✓ package OpenSysMLBehavior
✓ package OpenSysMLArtifacts
✓ package OpenSysMLPipeline
✓ package OpenSysMLInvariants
✓ package OpenSysMLGates
✓ package OpenSysMLCodebase
✓ package OpenSysMLSurfaces
✓ package OpenSysMLViews
✓ examples/self-model/behavior.sysml, examples/self-model/pipeline.sysml, examples/self-model/quality.sysml, examples/self-model/surfaces.sysml, examples/self-model/views.sysml: no errors
```

## Ask whether the invariants hold

Each invariant is a requirement whose subject is a part of the modelled toolchain, so its
condition is evaluated against that part rather than left abstract. `%requirement`
evaluates one; `%check` asks the solver whether it *can* hold and reports the assignment
that satisfies it (needs `z3` or `cvc5` — see
[installing a solver](../../docs/guide/01-install.md#installing-a-solver-optional)).

```bash
./bin/sysml examples/self-model/*.sysml
```

```
> %requirement OpenSysMLInvariants::treeIsImmutable
✓ Requirement OpenSysMLInvariants::treeIsImmutable satisfied

> %requirement OpenSysMLInvariants::tiersAreGated
✓ Requirement OpenSysMLInvariants::tiersAreGated satisfied

> %check OpenSysMLInvariants::executionIsBounded
✓ Requirement executionIsBounded is satisfiable (z3, 7ms)
  OpenSysMLInvariants::executionIsBounded::'runtime.stepBudgeted' = true
```

The solver timing is whatever your machine reports. The eight are `treeIsImmutable`,
`parserRecovers`, `resolutionIsLazy`, `tiersAreGated`, `loweringIsLossless`,
`executionIsBounded`, `libraryIsClean` and `exportRoundTrips`.
[`../self_model_test.go`](../self_model_test.go) evaluates all eight, so an invariant the
implementation stops satisfying — the standard library growing past its clean file count,
say — fails `go test ./examples/`.

## Read a view

```
> %view OpenSysMLViews::overview
view OpenSysMLViews::overview
  exposes
    OpenSysMLSurfaces::opensysml (part)
    OpenSysMLSurfaces::Toolchain::lsp (part)
  nested views
    OpenSysMLViews::overview::pipelineSubview (view)
  viewpoint conformance
    satisfy maintainerPerspective: conforms
      concern latency: conforms
```

The concern the viewpoint frames is keystroke latency, and it conforms because the exposed
language server declares incremental synchronisation. Expose a server that does not, and
the overview stops conforming.

## Render the diagrams

```bash
make self-model
```

That writes every view into `build/self-model/` — Mermaid for the structure, action and
state views, Markdown for the tables. Override the destination with
`make self-model SELF_MODEL_OUT=/tmp/views`, or render one view at a time in the REPL
(`-render` takes a single file, and this model is five):

```
> %render OpenSysMLViews::tierStates mermaid
```

```
%% OpenSysMLViews::tierStates — state rendering (view def StateTransitionView)
stateDiagram-v2
  state "state def OpenSysMLBehavior::TierProgression" as n0 {
    state "state syntaxTier (initial)" as n1
    [*] --> n1
    ...
  }
  n1 --> n2 : [failures == 0]
  n1 --> n8 : [failures #gt; 0]
```

The stage table is the model's answer to "which package implements this stage"; like the
Mermaid above, its first line is a comment naming the view, elided here:

```
> %render OpenSysMLViews::stageTable markdown
```

| Element | Kind | Type | Declared in |
| --- | --- | --- | --- |
| OpenSysMLPipeline::AnalysisPipeline | part def |  |  |
| sources | part | SourceStore | OpenSysMLPipeline::AnalysisPipeline |
| lexer | part | Lexer | OpenSysMLPipeline::AnalysisPipeline |
| parser | part | Parser | OpenSysMLPipeline::AnalysisPipeline |
| … | | | |

To turn the Mermaid into images, pipe it through the Mermaid CLI:

```bash
npx -y @mermaid-js/mermaid-cli -i build/self-model/OpenSysMLViews.pipelineStructure.mmd \
  -o pipeline.svg
```

## Keeping it honest

The model describes this implementation, so it goes stale the way documentation does. Three
things push back, all in [`../self_model_test.go`](../self_model_test.go): the model must
analyse clean, its invariant requirements must evaluate true, and the figures and package
names it declares are compared against the implementation — the keyword count against
`lexer.Keywords()`, the bundled library count against `libs.DefaultSource()`, the tier count
against `passes.PassLevel`, and every `goPackage` against the directory it names.

What that cannot do is re-verify behaviour: the invariants are conditions over the model's
own attributes, so they catch a claim edited out of agreement with itself or with those
figures, not a regression inside the parser. The `verification def`s in
[quality.sysml](quality.sysml) name the gates that do that — `TestGolden`/`TestNegative`,
`TestStdlibConformance`, `TestExecutionConformance`/`TestRobustness` and the export tests.

When a stage moves, a pass is added or a client lands, the model is the place the change is
recorded once and every diagram picks it up.

The authoritative prose account of the same architecture is
[docs/internals/architecture.md](../../docs/internals/architecture.md); this model is the
structured view of it, not a replacement.
