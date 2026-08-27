# OpenSysML documentation

## Using it

**[The guide](guide/)** — a handbook, in reading order: [install](guide/01-install.md),
[your first model](guide/02-first-model.md), [the command line](guide/03-command-line.md),
[the REPL](guide/04-repl.md), [checks](guide/05-checking.md), [behavior](guide/06-behavior.md),
[saving and RDF](guide/07-saving-and-rdf.md), [editors](guide/08-editors.md),
[Python](guide/09-python.md), [troubleshooting](guide/10-troubleshooting.md).

Runnable models are in [examples/](../examples/), with a catalog of what each one shows.

## Looking one thing up

- **[CLI](reference/cli.md)** — every flag of `sysml`, the modes, and the exit status
- **[REPL commands](reference/repl-commands.md)** — every `%` command and its arguments
- **[LSP extensions](reference/lsp.md)** — the custom render requests `sysml-lsp` serves a diagram client
- **[Environment variables](reference/environment.md)** — the bounds one run may spend, and paths
- **[Client libraries](reference/clients.md)** — the Go, Python, Node, Java and Rust surfaces, and which to pick
- **[Go packages](reference/api.md)** — `pkg/opensysml` and the packages behind it, type by type
- **[Python API](reference/python-api.md)** — `opensysml`, its generated typed classes and latency
- **[Service transports](reference/service-transports.md)** — what `sysml-grpc` serves on one port, and what an absent capability does
- **[RDF mapping](reference/rdf-mapping.md)** — which triples a model becomes, what is not mapped, and why the mapping is experimental
- **[Grammar](reference/grammar/README.md)** — grammar production → parser implementation

## How it works

- **[Architecture](internals/architecture.md)** — the pipeline, the tiers, the test contracts
- **[Testing](internals/testing.md)** — the test contracts each kind of change must satisfy
- **[Performance](internals/performance.md)** — profiling, and what a large model costs
- **[Design notes](internals/design/)** and **[plans](internals/notes/)** — for maintainers

## Where the project stands

- **[Spec compliance](project/spec-compliance.md)** — faithful, approximate, or not implemented
- **[Training examples](project/training-examples.md)** — the OMG corpus, 100/100 clean
- **[Pilot corpora gate](project/pilot-corpora.md)** — our diagnostics on the three pinned OMG pilot corpora, ratcheted in CI
- **[Pilot differential](project/pilot-differential.md)** — our diagnostics against the OMG pilot implementation
- **[Grammar coverage](project/grammar-coverage.md)** — which OMG grammar productions our inputs exercise
- **[Roadmap](project/roadmap.md)** — the known gaps, in the order they should be picked up
- **[Releasing](project/releasing.md)** — the pre-tag gate, tagging, artifacts, Homebrew
- **[macOS distribution](project/macos-distribution.md)** — Gatekeeper and the signing decision
- **[Demo](project/demo.md)** — a scripted walkthrough of the whole surface

Contributing, including where a new page belongs, is [CONTRIBUTING.md](../CONTRIBUTING.md).
