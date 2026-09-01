# The OpenSysML guide

The chapters are intended to be read in order on a first pass; each one assumes the material of
those preceding it.

1. [Install](01-install.md) — a release build, Homebrew, or from source
2. [Your first model](02-first-model.md) — declare, instantiate, evaluate
3. [From the command line](03-command-line.md) — checking a file, exit status, scripting
4. [The REPL](04-repl.md) — the session model and the meta-commands
5. [Expressions, calculations, constraints and requirements](05-checking.md) — what a model asserts
6. [Behavior: actions and state machines](06-behavior.md) — running and debugging behavior
7. [Saving, and converting to RDF](07-saving-and-rdf.md) — `%save`, `-convert`, round trips
8. [Editors](08-editors.md) — `sysml-lsp` and the VS Code extension
9. [From your own program](09-python.md) — the Go, Python, Node, Java and Rust clients
10. [Troubleshooting](10-troubleshooting.md) — diagnosing a run that stops short

Chapter 9 has a section per client — Go, Python, Node/TypeScript, Java and Rust — and Python's is the
longest because it is the oldest and most complete, not because it is the intended one.
[Client libraries](../reference/clients.md) describes which to select and what each covers.

For looking up a specific detail rather than reading through, the [reference](../reference/)
documents the CLI flags, the REPL commands, the environment variables, the service API, the client
libraries, each client's API and the RDF mapping.
