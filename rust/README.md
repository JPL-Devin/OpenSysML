# OpenSysML Rust client

`opensysml` is a blocking Rust client for the local `sysml-grpc` service. It
is not published to crates.io yet. The crate name still needs to be checked
for availability, and publishing is a maintainer decision.

## Installation

For now, use a path dependency while developing against a checkout:

```toml
[dependencies]
opensysml = { path = "../OpenSysML/rust/opensysml" }
```

The current Git dependency form is:

```toml
[dependencies]
opensysml = { git = "https://github.com/Open-MBEE/OpenSysML.git", branch = "main" }
```

The minimum supported Rust version is **Rust 1.83**.

## Why blocking

The client is blocking by default and has no asynchronous runtime anywhere in
its normal dependency tree. All 15 service RPCs are unary, and the usual
consumer talks to a local child that answers in milliseconds. Async buys the
average consumer little here, while putting a private `tokio::Runtime` in a
library taxes every consumer. That is why this client does not use `tonic`.

The runtime-free design also has no nested-runtime hazard: the library has no
`Runtime::new` that could panic inside an existing runtime. The
`blocking_calls_work_inside_a_runtime` test calls the client from
`Runtime::block_on` to pin this property. An async surface could be added
later behind a feature flag without changing the default.

## Transport

Protobuf request and response bodies are the default transport. JSON remains
available as the `curl` and debugging affordance. The measured comparison in
[`docs/internals/design/transport-evaluation.md`](../docs/internals/design/transport-evaluation.md)
was 6.5 ms for protobuf versus 42 ms for JSON on a 468 KB response.

## Service lifecycle

There are two connection modes.

* `Connection::private()` starts one child process per parent process with
  `-port 0 -health-port 0 -report-address -exit-with-parent`. The address is
  read from the child's first stdout line. The child is shared, so its parse
  cache is shared too.
* `Connection::external(host, port)` explicitly connects to an existing
  service. `Connection::connect()` also accepts `$OPENSYSML_SERVICE`. Closing
  an external connection does not stop that service.

`Drop` gives deterministic cleanup rather than relying on Python garbage
collection or JVM finalizers. `Drop` does not run for `std::process::exit`,
`abort`, or `SIGKILL`, so the stronger guarantee is the stdin pipe: the client
holds it, never writes to it, and the child observes EOF when the kernel closes
it as the process dies. The SIGKILL lifecycle test pins this orphan-cleanup
behavior.

## Binary provisioning

Version 1 does not download binaries. Resolution is:

1. `$OPENSYSML_GRPC_BINARY`
2. `~/.opensysml/bin/sysml-grpc`
3. `PATH`

If a downloader is added, it must match the Python client's per-release pinned
SHA-256 checks in [`python/opensysml/binary.py`](../python/opensysml/binary.py)
and its sigstore-verified manifest in
[`python/opensysml/signing.py`](../python/opensysml/signing.py). A client must
never fetch and execute a binary without verifying its integrity. Release
pinning belongs with that downloader; this v1 client does not pretend to pin a
child version.

## Capability negotiation

The service's advertised capability list is the negotiation surface, and the
client checks it before it calls rather than relying on the refusal: a request
that needs a capability the service does not have is refused with
`UNIMPLEMENTED` naming that capability, and checking first turns that into a
local error naming what to install instead of a transport round trip.
Capabilities that only describe how a response is populated omit the fields they
name rather than refusing the call. The client checks request-side requirements
for:

* `strict_conformance` when strict parsing is requested;
* `inline_language` for inline KerML content; and
* `evaluate_subject` when a subject symbol is supplied for evaluation.

Decoding a response is never gated on capabilities: if a service sends an
enum, unset value, or feature-value arm, the client understands that answer.
Consumers can inspect `Capabilities::has` or use `Capabilities::require` when
they need to gate their own use of `enum_values`, `unset_value`,
`feature_values`, or another advertised operation.

## Conformance runner

The workspace includes `opensysml-conformance`, which runs the language-neutral
scenarios through the typed client API. It uses the committed protobuf
descriptor to decode requests, calls only the public client surface, and reads
responses through domain `wire()` accessors before comparing normalized JSON.
The report has per-outcome totals for passed, failed, skipped, and errored
scenarios, including skipped scenarios.

Run it from the repository root:

```bash
make conformance-rust
```

Or run the binary directly:

```bash
cargo run --manifest-path rust/Cargo.toml -p opensysml-conformance -- \
  -binary bin/sysml-grpc \
  -scenarios conformance/scenarios \
  -fixtures conformance/fixtures \
  -report bin/conformance-report-rust.json
```

The runner accepts:

* `-binary PATH` to select the service binary;
* `-run SUBSTRING` to select scenario IDs;
* `-report FILE` or `-report -` for the JSON report;
* `-allow-skips` to allow capability-dependent skips;
* `-v` to print per-scenario timing.

The `-binary` default is `$OPENSYSML_GRPC_BINARY`, then `bin/sysml-grpc`
relative to the repository root. The two expected v1 boundary skips are
`v1 API does not cover <RPC>` and
`unrepresentable by the typed API: ParseFile with no source`. Other skips name
the missing capability and fail the run unless `-allow-skips` is supplied.

When a covered RPC answers successfully with a non-empty top-level `error`, the
typed API exposes `Error::Model(message)` and does not retain the rest of that
response. The runner therefore compares a partial reconstruction,
`{"error": message}`. This is intentionally fail-safe: an expectation that
names another response field alongside the top-level error fails rather than
passing. Widening this representation requires the client to carry the whole
response on an in-band error, which is outside the v1 boundary.

## v1 boundary

The current API deliberately does not include generated model-ergonomics types
beyond its existing domain objects, the edit API, RDF conversion, or
verification helpers. The conformance runner consequently skips RPCs that the
typed v1 API does not cover.

## Release procedure

Before a release, `cargo package -p opensysml` must succeed cleanly. `cargo
publish` is a maintainer action; CI never publishes this crate.
