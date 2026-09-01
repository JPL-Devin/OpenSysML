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

The seven files:

| File | What it holds |
| --- | --- |
| [pipeline.sysml](pipeline.sysml) | `OpenSysMLArtifacts` — what travels between stages (source text, tokens, tree, symbol index, side tables, diagnostics, IR graphs, traces, RDF, document trees), the ports and channels it travels over, and the layer metadata the filtered views select on. `OpenSysMLPipeline` — the thirteen stages from `internal/core/source` to `internal/core/solve`, each naming the Go package that implements it, the five validation passes, and `AnalysisPipeline`, which wires them together |
| [behavior.sysml](behavior.sysml) | one document analysed end to end (`AnalyzeDocument`, whose four decision nodes are the tier gates), the editor's edit-then-sweep path (`ServeEdit`), and four state machines: the validation tier ladder, the runtime's five tiers, Petri-net token flow with its deadlock and budget exits, and run-to-completion event dispatch with deferral |
| [surfaces.sysml](surfaces.sysml) | the four interfaces over one pipeline (REPL, LSP, gRPC service, CLI), the four generated clients and the VS Code extension, the document path from a query in the model through the plan, the backend-agnostic tree and the two backends to Markdown or PDF (`RenderDocument` branches on the form, and on whether the PDF converters are installed), the exporter, the six conformance oracles with their committed baselines, and `Toolchain`, which holds all of it |
| [identity.sysml](identity.sysml) | the element-identity path: the `IdentityMetadata` library the ids are carried by, the encoder that derives an id from a qualified name, the side table that computes each element's effective id, the constraint-tier pass that checks the generated id space, the RDF writer and reader that carry identity through a graph, the Flexo harness that measures a live round trip, and the sync diff that is designed but not built ([design record](../../docs/project/element-identity-annotations.md)) |
| [quality.sysml](quality.sysml) | eight architecture invariants as `requirement def`s bound to the modelled parts, the test runs that verify them as `verification def`s, the contributor's use case, and the allocation of every logical unit onto its directory in the source tree |
| [document.sysml](document.sysml) | the architecture document itself, written in the notation: the queries it runs over the model, the sections and prose it is made of, the diagrams it embeds from [views.sysml](views.sysml), and the tables it generates from the model — so the document is a model element rather than a file someone maintains alongside one |
| [views.sysml](views.sysml) | twenty views — the pipeline, toolchain and identity path as interconnection diagrams, the stage and invariant tables, the action and state flows including the document and identity round trips, the architectural layers as filtered exposes, and an overview that frames a maintainer's concern |

## Analyse it

```bash
./bin/sysml examples/self-model/*.sysml -validate
```

```
✓ package OpenSysMLBehavior
✓ package OpenSysMLDocument
✓ package OpenSysMLIdentity
✓ package OpenSysMLArtifacts
✓ package OpenSysMLPipeline
✓ package OpenSysMLInvariants
✓ package OpenSysMLGates
✓ package OpenSysMLCodebase
✓ package OpenSysMLSurfaces
✓ package OpenSysMLViews
✓ examples/self-model/behavior.sysml, examples/self-model/document.sysml, examples/self-model/identity.sysml, examples/self-model/pipeline.sysml, examples/self-model/quality.sysml, examples/self-model/surfaces.sysml, examples/self-model/views.sysml: no errors
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

The solver timing is whatever your machine reports. The eight in `OpenSysMLInvariants` are
`treeIsImmutable`, `parserRecovers`, `resolutionIsLazy`, `tiersAreGated`,
`loweringIsLossless`, `executionIsBounded`, `libraryIsClean` and `exportRoundTrips`; three
more in `OpenSysMLIdentity` state what the identity design turns on —
`identityRoundTrips`, `idsDoNotCollide` and `identityIsBesideTheTree`; and `documentsAreTraceable`
in `OpenSysMLSurfaces` states that every rendered node can be traced back to the element it came
from. [`../self_model_test.go`](../self_model_test.go) evaluates all twelve, so an invariant the
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
(`-render` takes a single file, and this model is seven):

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

## Render the architecture document

[document.sysml](document.sysml) declares `OpenSysMLDocument::ArchitectureDocument`, an
architecture document written in the notation: its prose is authored, its diagrams are the
views above, and its tables are queries evaluated over the model. `make self-model` renders
it beside the views, or render it alone with:

```bash
./bin/sysml examples/self-model/*.sysml -render-documents build/self-model
```

```
wrote build/self-model/OpenSysMLDocument-ArchitectureDocument.md (markdown, …)
```

The stage table in it is written nowhere; it is what the query returned:

| name | goPackage |
| --- | --- |
| sources | internal/core/source |
| lexer | internal/core/lexer |
| parser | internal/core/parser |
| … | |

So moving a stage to another package rewrites that table on the next render, and a stage
added to the model appears in it without anyone editing the document. `-render-document`
takes a single file, so a model in several files renders through `-render-documents`; that
single-file form also takes `-doc-form pdf`, which needs the converters installed.

## Keeping it honest

The model describes this implementation, so it goes stale the way documentation does. Three
things push back, all in [`../self_model_test.go`](../self_model_test.go): the model must
analyse clean, its invariant requirements must evaluate true, and the figures and package
names it declares are compared against the implementation — the keyword count against
`lexer.Keywords()`, the bundled library count against `libs.DefaultSource()`, the tier count
against `passes.PassLevel`, and every `goPackage` against the directory it names. The
identity model is held to the same standard: the metadata definitions it names are compared
against `identity.ElementIdFQN` and `identity.ProjectRefFQN`, the library file it points at
must exist, and the tier it models the identity pass at must be the tier
`passes.IdentityMetadataPass` declares. So is the document path: the converters it lists are
compared against `docpdf.Engines()` and the library it names must exist, and the architecture
document must render — a query that stops binding, or an embedded view that is renamed, fails
the test rather than silently dropping a section.

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
