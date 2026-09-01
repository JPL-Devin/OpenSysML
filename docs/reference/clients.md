# Client libraries

OpenSysML can be reached from a program in five ways: the Go API, used in the calling process, and
four clients of the `sysml-grpc` service. This page describes how to choose between them, what
each covers and what each intentionally omits. Each client has an API reference of its own, and
[guide chapter 9](../guide/09-python.md) uses each one as a task.

| Surface | Reaches the engine by | Published | Full reference |
|---|---|---|---|
| **Go**, `client/opensysml` | in process; or Connect, to a service someone else runs | with the core (`v*` tags) | [Go packages](api.md) |
| **Python**, `opensysml` | gRPC, to a private child service or a named service | PyPI, on `opensysml-v*` tags | [Python API](python-api.md) |
| **Node/TypeScript**, `@opensysml/client` | Connect, to a private child service, a named service, or one a browser page addresses | not yet | [Node API](node-api.md) |
| **Java**, `org.openmbee:opensysml-client` | Connect, over the JDK's own HTTP client | not yet | [Java API](java-api.md) |
| **Rust**, `opensysml` | Connect, blocking, no async runtime | not yet | [Rust API](rust-api.md) |

The protocols and what the service serves on a single port are described in
[service transports](service-transports.md); the release process for each client is described in
[releasing](../project/releasing.md).

## Choosing a client

- **In a Go program: `client/opensysml`.** It links the parser, the semantic engine and the runtime
  directly, so there is no port, no child process and no serialization round trip. A Go program
  that starts a service to talk to itself has paid for a child whose only job is to run the code
  it already links.
- **In a notebook: Python.** `opensysml` adds generated typed classes, Jupyter display hooks and
  DataFrame integration to the full RPC surface.
- **In a browser or a Node service: `@opensysml/client`.** No native addon, and the browser entry
  point needs only `fetch` against a service that allows the page's origin.
- **In a JVM host application the caller does not control — an Eclipse-based tool, a Cameo plugin,
  a web application: Java.** Its transport is `java.net.http.HttpClient`, so no gRPC, Netty or
  `tcnative` reaches the host application.
- **In a Rust program: `opensysml`.** Blocking, with no asynchronous runtime in its default
  dependency tree, and safe to call from inside one.

The Go and Python clients each cover every RPC the service has; the Node, Java and Rust clients
cover the v1 subset described below.

## What the newer surfaces cover

The Node, Java and Rust clients are v1 surfaces with the same scope: connection lifecycle,
capability negotiation, parsing (a file or inline source), diagnostics, symbol lookup, expression
evaluation and instantiation. Deliberately **not** in v1, in all three rather than
half-implemented in some:

- the edit API (`ApplyEdits`) and generated model-ergonomics types;
- RDF conversion (`Convert`);
- verification (`VerifyConstraint`, `VerifyRequirement`, `VerifySatisfaction`) and `EvaluateCalc`;
- behaviour execution (`ExecuteAction`, `ExecuteState`);
- `Query` and OSLC query;
- native document queries and rendering (`RunDocumentQuery`, `RenderDocument`).

Those RPCs exist and are served. Only the Node client offers an escape hatch to them —
`connection.rpc` is the generated Connect client — so from Java or Rust, reach them from the Go or
Python client until a v2 wraps them: those two ship the protobuf messages but no public call that
sends one. Each client's conformance report names, per scenario, which of these a skip belongs to,
so a shrinking surface cannot pass quietly.

The Go API covers all of them but the generated model-ergonomics types: it reads models through
`Symbol`, `Instance` and `Value` instead.

## Two lifecycle modes, and one guarantee

Every client that can start a service starts a **private child** of the calling process —
`sysml-grpc -port 0 -health-port 0 -report-address -exit-with-parent` — and reads the address the
kernel assigned from the child's first stdout line. No port is chosen, probed or retried, so two
processes starting at once cannot collide, and no service left listening by anyone else is ever
adopted. The child is shared within its scope, which shares its parse cache: per interpreter in
Python, per thread in Node, per classloader in Java (`isolatedService(true)` opts out), per
process in Rust. `client/opensysml` starts nothing, because in process there is nothing to start.

Reaching a service the client did not start is always explicit, through an address argument or
`$OPENSYSML_SERVICE`, and closing such a connection disconnects and does nothing further.

**No orphans, and the mechanism is not an exit hook.** Each client holds the write end of the
child's stdin pipe and never writes to it; the child exits at end of file. The kernel closes that
pipe when the holder dies however it dies, which survives what a shutdown hook does not:
`SIGKILL`, `Runtime.halt`, `process.abort()`, a crash during shutdown. Every client pins this
with a test that kills its own parent process and asserts the service is gone.

## Protobuf bodies, and JSON for debugging

Every client sends protobuf bodies by default and offers JSON for `curl`-based debugging. This
reflects a measurement rather than a preference: a 468 KB response costs approximately 6.5 ms with
a protobuf body against approximately 42 ms with JSON, in `protojson` and `json_format` CPU time
rather than bytes on the wire. See [service transports](service-transports.md).

## Providing the service binary

Only the Python and Node clients download a binary, and both pin a SHA-256 per release asset and
verify the release's sigstore-signed manifest before they do. The others resolve one that is
already there, and the resolution order is the same everywhere:
`$OPENSYSML_GRPC_BINARY` (`$OPENSYSML_BINARY` in the Node and Python clients) first, then
`~/.opensysml/bin/sysml-grpc` — where a verified download puts it — then `PATH`. The Node client adds its
per-platform npm package, whose tarball npm verifies, with no postinstall script; it is preferred
over a download, which happens only when no package matches the platform.
The Java client additionally verifies a digest the caller pins with `expectedBinarySha256`.

An unresolvable binary is an error naming every way to supply one, rather than a fetch.

## Every client runs the same conformance suite

The scenarios in [`conformance/`](https://github.com/Open-MBEE/OpenSysML/tree/main/conformance)
are the service contract, and each client runs them **through its own public API** rather than
through generated stubs, so conformance with `sysml-grpc` is measured per language, over the same
scenarios and comparing the same results:

```bash
make conformance             # the reference runner: gRPC, Connect, Connect-JSON
make conformance-pkg         # the public Go API, in process and remote
make conformance-rust
npm --prefix clients/node run conformance -- --allow-skips
```

The Java runner is launched from its own classpath rather than by a Maven goal; the two exact
commands are given in [clients/java/README.md](../../clients/java/README.md#conformance).

The reference runner also takes `-junit <file>`, writing the same run as JUnit XML — one suite per
configuration and protocol, one case per scenario — which is what `make conformance` stores beside
the JSON report and what CI renders as its test report. The JSON report stays the source of truth.

Each runner writes the report format produced by `cmd/conformance`, and each is checked against
deliberate corruption — a mutated response must fail a scenario — so a runner that asserts nothing
cannot pass. Current per-client scenario counts are given in each client's README; they change as
v1 gaps close, which is why they are maintained beside the code rather than here.
