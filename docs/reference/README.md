# Reference

Lookup documentation: every flag, command, symbol and triple, without the
supporting rationale. [The guide](../guide/) presents the same material in
reading order.

- **[CLI](cli.md)** — every flag of `sysml`, the modes, and the exit status
- **[REPL commands](repl-commands.md)** — every `%` command and its arguments
- **[LSP extensions](lsp.md)** — the custom render requests `sysml-lsp` serves a diagram client
- **[Environment variables](environment.md)** — the bounds one run may spend, and paths
- **[Client libraries](clients.md)** — the five ways to reach the engine from a program, what each
  covers, and how to choose between them
- **[Go packages](api.md)** — `client/opensysml` and the packages behind it, type by type
- **[Python API](python-api.md)** — `opensysml`, its generated typed classes and latency
- **[Node API](node-api.md)** — `@opensysml/client`, its two entry points and its typed unions
- **[Java API](java-api.md)** — `opensysml-client`, its immutable records and its exceptions
- **[Rust API](rust-api.md)** — the `opensysml` crate, blocking, and its one error enum
- **[Service transports](service-transports.md)** — what `sysml-grpc` serves on one port, which
  body encoding a client should choose, and the flags for CORS, TLS and health
- **[RDF mapping](rdf-mapping.md)** — which triples a model becomes, what is not mapped, and
  why the mapping is experimental
- **[OSLC Query text](oslc-query.md)** — element-identification query syntax and semantics
- **[Grammar](grammar/README.md)** — grammar production → parser implementation
