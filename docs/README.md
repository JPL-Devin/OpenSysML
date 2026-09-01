# OpenSysML documentation

## Using OpenSysML

**[The guide](guide/)** — a handbook, presented in reading order: [install](guide/01-install.md),
[your first model](guide/02-first-model.md), [the command line](guide/03-command-line.md),
[the REPL](guide/04-repl.md), [checks](guide/05-checking.md), [behavior](guide/06-behavior.md),
[saving and RDF](guide/07-saving-and-rdf.md), [editors](guide/08-editors.md),
[from your own program](guide/09-python.md), [troubleshooting](guide/10-troubleshooting.md).

Runnable models are provided in [examples/](../examples/), together with a catalog describing what
each one demonstrates.

**[Document generation manual](manual/README.md)** — generating documents from models with native
document queries: [concepts](manual/introduction.md), [getting started](manual/getting-started.md),
a [query cookbook](manual/query-cookbook.md), [document authoring](manual/authoring.md),
[outputs](manual/outputs.md) (Markdown and PDF), [interfaces](manual/interfaces.md),
a [complete worked example](manual/worked-example.md) and
[limitations and troubleshooting](manual/troubleshooting.md).

## Reference

- **[CLI](reference/cli.md)** — every flag of `sysml`, the modes, and the exit status
- **[REPL commands](reference/repl-commands.md)** — every `%` command and its arguments
- **[LSP extensions](reference/lsp.md)** — the custom render requests `sysml-lsp` serves a diagram client
- **[Environment variables](reference/environment.md)** — the bounds a single run may consume, and paths
- **[Client libraries](reference/clients.md)** — the Go, Python, Node, Java and Rust surfaces, and how to choose between them
- **[Go packages](reference/api.md)** — `client/opensysml` and the packages behind it, type by type
- **[Python API](reference/python-api.md)** — `opensysml`, its generated typed classes and latency
- **[Service transports](reference/service-transports.md)** — what `sysml-grpc` serves on a single port, and the behavior of an absent capability
- **[RDF mapping](reference/rdf-mapping.md)** — the triples a model becomes, the constructs that are not mapped, and why the mapping is experimental
- **[Grammar](reference/grammar/README.md)** — grammar production → parser implementation

## Internals

- **[Architecture](internals/architecture.md)** — the pipeline, the tiers and the test contracts
- **[Testing](internals/testing.md)** — the test contracts each kind of change must satisfy
- **[Performance](internals/performance.md)** — profiling, and the cost of a large model
- **[Design notes](internals/design/)** and **[plans](internals/notes/)** — maintainer material

## Project status

- **[Spec compliance](project/spec-compliance.md)** — faithful, approximate or not implemented
- **[Training examples](project/training-examples.md)** — the OMG corpus, 100/100 clean
- **[Pilot corpora gate](project/pilot-corpora.md)** — OpenSysML diagnostics on the three pinned OMG pilot corpora, ratcheted in CI
- **[Pilot differential](project/pilot-differential.md)** — OpenSysML diagnostics compared against the OMG pilot implementation
- **[Grammar coverage](project/grammar-coverage.md)** — the OMG grammar productions the project's inputs exercise
- **[Roadmap](project/roadmap.md)** — the known gaps, in the order they should be addressed
- **[Releasing](project/releasing.md)** — the pre-tag gate, tagging, artifacts and Homebrew
- **[macOS distribution](project/macos-distribution.md)** — Gatekeeper and the signing decision
- **[Demo](project/demo.md)** — a scripted walkthrough of the full surface

Contribution guidance, including where a new page belongs, is in
[CONTRIBUTING.md](../CONTRIBUTING.md).
