# Reference

Looking one thing up — every flag, command, symbol and triple, and nothing about
why. The handbook that puts them in order is [the guide](../guide/).

- **[CLI](cli.md)** — every flag of `sysml`, the modes, and the exit status
- **[REPL commands](repl-commands.md)** — every `%` command and its arguments
- **[LSP extensions](lsp.md)** — the custom render requests `sysml-lsp` serves a diagram client
- **[Environment variables](environment.md)** — the bounds one run may spend, and paths
- **[Client libraries](clients.md)** — the five ways to reach the engine from a program, what each
  covers, and which to pick
- **[Go packages](api.md)** — `pkg/opensysml` and the packages behind it, type by type
- **[Python API](python-api.md)** — `opensysml`, its generated typed classes and latency
- **[Service transports](service-transports.md)** — what `sysml-grpc` serves on one port, which
  body encoding a client should choose, and the flags for CORS, TLS and health
- **[RDF mapping](rdf-mapping.md)** — which triples a model becomes, what is not mapped, and
  why the mapping is experimental
- **[OSLC Query text](oslc-query.md)** — element-identification query syntax and semantics
- **[Grammar](grammar/README.md)** — grammar production → parser implementation
