# The OpenSysML guide

Read in order the first time; each chapter assumes the ones before it.

1. [Install](01-install.md) — a release build, Homebrew, or from source
2. [Your first model](02-first-model.md) — declare, instantiate, evaluate
3. [From the command line](03-command-line.md) — checking a file, exit status, scripting
4. [The REPL](04-repl.md) — the session model and the meta-commands
5. [Expressions, calculations, constraints and requirements](05-checking.md) — what a model asserts
6. [Behavior: actions and state machines](06-behavior.md) — running and debugging behavior
7. [Saving, and converting to RDF](07-saving-and-rdf.md) — `%save`, `-convert`, round trips
8. [Editors](08-editors.md) — `sysml-lsp` and the VS Code extension
9. [From Python](09-python.md) — the `opensysml` client over `sysml-grpc`
10. [Troubleshooting](10-troubleshooting.md) — when a run stops short

Python is the chapter here because it is the oldest and widest client, not the only one: Go,
Node/TypeScript, Java and Rust reach the same engine, and [client
libraries](../reference/clients.md) is the map of which to pick and what each covers.

Looking up one thing rather than reading? The [reference](../reference/) has the CLI flags, the
REPL commands, the environment variables, the service API, the client libraries, the Python API and
the RDF mapping.
