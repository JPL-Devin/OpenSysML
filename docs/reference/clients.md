# Client libraries

Five ways to reach OpenSysML from a program: the Go API in your own process, and four clients of
the `sysml-grpc` service. This page is the map — which one to pick, what each covers, and what it
deliberately does not. Each client's own README is the detailed reference, linked per section.

| Surface | Reaches the engine by | Published | Full reference |
|---|---|---|---|
| **Go**, `pkg/opensysml` | in process; or Connect, to a service someone else runs | with the core (`v*` tags) | [pkg/opensysml/README.md](../../pkg/opensysml/README.md) |
| **Python**, `opensysml` | gRPC, to a private child service or one you name | PyPI, on `opensysml-v*` tags | [Python API](python-api.md) |
| **Node/TypeScript**, `@opensysml/client` | Connect, to a private child service, one you name, or one a browser page addresses | not yet | [clients/node/README.md](../../clients/node/README.md) |
| **Java**, `io.github.open-mbee:opensysml-client` | Connect, over the JDK's own HTTP client | not yet | [clients/java/README.md](../../clients/java/README.md) |
| **Rust**, `opensysml` | Connect, blocking, no async runtime | not yet | [rust/README.md](../../rust/README.md) |

What each protocol is and what the service serves on one port is
[service transports](service-transports.md); how a release of each is cut is
[releasing](../project/releasing.md).

## Which one

- **In a Go program: `pkg/opensysml`.** It links the parser, the semantic engine and the runtime
  directly, so there is no port, no child process and no serialization round trip. A Go program
  that starts a service to talk to itself has paid for a child whose only job is to run the code
  it already links.
- **In a notebook, or for the whole RPC surface: Python.** `opensysml` is the one client that
  covers every RPC the service has — verification, behaviour execution, RDF conversion, edits,
  `Query` — plus generated typed classes, Jupyter display hooks and DataFrame integration.
- **In a browser or a Node service: `@opensysml/client`.** No native addon, and the browser entry
  point needs only `fetch` against a service that allows the page's origin.
- **In a JVM host you do not own — an Eclipse-based tool, a Cameo plugin, a web application:
  Java.** Its transport is `java.net.http.HttpClient`, so no gRPC, Netty or `tcnative` reaches the
  host application.
- **In a Rust program: `opensysml`.** Blocking, with no asynchronous runtime in its default
  dependency tree, and safe to call from inside one.

## What the four newer surfaces cover

The Go, Node, Java and Rust clients are v1 surfaces with the same scope: connection lifecycle,
capability negotiation, parsing (a file or inline source), diagnostics, symbol lookup, expression
evaluation and instantiation. Deliberately **not** in v1, in all four rather than half-implemented
in some:

- the edit API (`ApplyEdits`) and generated model-ergonomics types;
- RDF conversion (`Convert`);
- verification (`VerifyConstraint`, `VerifyRequirement`, `VerifySatisfaction`) and `EvaluateCalc`;
- behaviour execution (`ExecuteAction`, `ExecuteState`);
- `Query` and OSLC query.

Those RPCs exist and are served. Reach them from the Python client, or from the generated stubs
each client ships beside its ergonomic layer, until a v2 wraps them. Each client's conformance
report names, per scenario, which of these a skip belongs to, so a shrinking surface cannot pass
quietly.

## Two lifecycle modes, and one guarantee

Every client that can start a service starts a **private child** of the calling process —
`sysml-grpc -port 0 -health-port 0 -report-address -exit-with-parent` — and reads the address the
kernel assigned from the child's first stdout line. No port is chosen, probed or retried, so two
processes starting at once cannot collide, and no service left listening by anyone else is ever
adopted. The child is shared within its scope, which shares its parse cache: per interpreter in
Python, per thread in Node, per classloader in Java (`isolatedService(true)` opts out), per
process in Rust. `pkg/opensysml` starts nothing, because in process there is nothing to start.

Reaching a service you did not start is always explicit — an address argument, or
`$OPENSYSML_SERVICE` — and closing such a connection disconnects and nothing else.

**No orphans, and the mechanism is not an exit hook.** Each client holds the write end of the
child's stdin pipe and never writes to it; the child exits at end of file. The kernel closes that
pipe when the holder dies however it dies, which survives what a shutdown hook does not:
`SIGKILL`, `Runtime.halt`, `process.abort()`, a crash during shutdown. Every client pins this
with a test that kills its own parent process and asserts the service is gone.

## Protobuf bodies, and JSON for debugging

Every client sends protobuf bodies by default and offers JSON for `curl`-shaped debugging. That
is a measurement, not a preference: a 468 KB response costs ~6.5 ms with a protobuf body against
~42 ms with JSON, in `protojson` and `json_format` CPU rather than bytes on the wire — see
[service transports](service-transports.md).

## Providing the service binary

No client downloads a binary except the Python one, which pins a SHA-256 per release asset and
verifies the release's sigstore-signed manifest before it does. The others resolve one that is
already there, and the resolution order is the same everywhere:
`$OPENSYSML_GRPC_BINARY` (`$OPENSYSML_BINARY` in Node) first, then `~/.opensysml/bin/sysml-grpc`
— where the Python client's verified download puts it — then `PATH`. The Node client adds its
per-platform npm package, whose tarball npm verifies, with no postinstall script and no download.
The Java client additionally verifies a digest the caller pins with `expectedBinarySha256`.

An unresolvable binary is an error naming every way to supply one, rather than a fetch.

## Every client runs the same conformance suite

The scenarios in [`conformance/`](https://github.com/Open-MBEE/OpenSysML/tree/main/conformance)
are the service contract, and each client runs them **through its own public API** rather than
through generated stubs — so "it speaks to `sysml-grpc`" is measured per language, on the same
scenarios, comparing the same answers:

```bash
make conformance             # the reference runner: gRPC, Connect, Connect-JSON
make conformance-pkg         # the public Go API, in process and remote
make conformance-rust
npm --prefix clients/node run conformance -- --allow-skips
```

The Java runner is launched from its own classpath rather than by a Maven goal — the exact two
commands are in [clients/java/README.md](../../clients/java/README.md#conformance).

Each runner writes the report shape `cmd/conformance` writes, and each is checked against
deliberate corruption — a mutated response must fail a scenario — so a runner that asserts
nothing cannot pass. Current per-client scenario counts are in each client's README; they move as
v1 gaps close, which is why they live beside the code rather than here.
