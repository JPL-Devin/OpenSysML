# Service transports and encodings

What `sysml-grpc` serves on its port, which body encoding a client should choose, and the
flags that decide it. Written for someone about to write a client. The measurements and the
reasoning behind the choice are
[the transport evaluation](../internals/design/transport-evaluation.md).

## One port, four ways in

`sysml-grpc` serves, by default, all of these on `-port` (50051):

| Protocol | Content type | Who speaks it |
|----------|--------------|---------------|
| gRPC | `application/grpc` | `grpc-go`, `grpc-java`, `tonic`, `grpcio` (the Python client), `grpcurl` |
| gRPC-Web | `application/grpc-web`, `application/grpc-web+json` | browser clients using `fetch` |
| Connect protocol | `application/proto`, `application/json` | `connect-go`, `@connectrpc/connect-web`, and any HTTP client at all |
| gRPC server reflection | — | `grpcurl`, `grpcui` |

Plus `GET /health`, described below. A Connect unary call is an ordinary POST to
`/sysml.SysMLService/<Method>` whose whole body is the request message, so `curl` is a
first-class client and no generated code is needed to reach the service.

The service is one implementation behind all of them: the same fifteen RPCs of
[`api/proto/sysml.proto`](https://github.com/Open-MBEE/OpenSysML/blob/main/api/proto/sysml.proto),
the same semantics, the same status codes. An existing gRPC client — including a generated
`grpc-go` stub, `grpcurl` and the `opensysml` Python client — reaches the default server
unchanged.

`-transport grpc` serves the `grpc-go` server alone, as releases before this one did: no
Connect, no gRPC-Web, no `curl`, health on `-health-port` only. It is an escape hatch, not the
recommended path. `-transport stdio` is an evaluation prototype and no client should be built
on it; see the evaluation's recommendation.

## Choose protobuf, not JSON

**Protobuf is the body encoding for every client we ship or document. JSON is the debugging
affordance.** This is not a style preference — it is the one strong measurement in the
evaluation:

| 468 KB `Query` response | p50 | p95 | p99 |
|---|---|---|---|
| protobuf body | 6.34 ms | 9.18 ms | 9.88 ms |
| JSON body | 37.88 ms | 41.82 ms | 44.99 ms |

Six times slower, reproducible across runs. The cause is **not** payload size: the same
answer is 467,971 bytes as protobuf and 513,339 as JSON, 9.7% more. It is `protojson` encode
plus `json_format` decode CPU, so a faster link does not help and the cost falls on both
ends. Small answers show no measurable difference between the encodings, which is why the
warning is about large ones.

The server says so at runtime rather than only here: a JSON-encoded response over 256 KiB
logs a warning naming the procedure and the size. There is no cheaper JSON path to switch to
— `connect-go` marshals such a response once, not twice, so there is no double marshal to
remove, and `protojson` has no streaming encoder to substitute. The mitigation available is
the choice of encoding, and it belongs to the client.

A browser client over Connect-JSON has no protobuf option for its *body* if it is written
against `application/json` by hand — but `@connectrpc/connect-web` speaks
`application/proto` in the browser and should be generated rather than hand-written for
exactly this reason. `Query` over the whole model is the call where this decides whether a UI
feels responsive.

## Two things a hand-written JSON client must know

```console
$ curl -X POST http://localhost:50051/sysml.SysMLService/ParseFile \
    -H "Content-Type: application/json" \
    -d '{"content":"package Demo { part def Rover { attribute mass = 12.5; } }"}'
{"modelHash":"6245ef48…e78d","root":{"kind":"RootNamespace","childIds":["Demo"]}}

$ curl -X POST http://localhost:50051/sysml.SysMLService/Evaluate \
    -H "Content-Type: application/json" \
    -d '{"expression":"1 + 2 * 3","modelHash":"6245ef48…e78d"}'
{"result":{"intValue":"7"}}
```

1. **An `int64` is a JSON string.** `"7"`, not `7` — the proto3 JSON mapping, not a quirk of
   this service. `intValue`, `modelHash` lengths, step counts and every other 64-bit field
   read this way.
2. **Errors are an HTTP status plus a body**, not gRPC trailers:
   `{"code":"not_found","message":"…"}`. The code names are the Connect protocol's spelling of
   the same gRPC codes a gRPC client sees, so `not_found` here and `NOT_FOUND` there are one
   status. Every client in every protocol must map them identically; the conformance suite
   asserts that it does.

## A browser client

Three prerequisites, in the order they bite:

**CORS.** `-cors-allowed-origins` takes a comma-separated list of exact origins
(`https://studio.example.org,http://localhost:5173`) and is off when empty. A `*` entry is
**refused at startup**: a service that answers every origin is not a default worth having.
The allowed set drives the preflight response, and the response exposes gRPC-Web's trailer
headers (`Grpc-Status`, `Grpc-Message`, `Grpc-Status-Details-Bin`) — a gRPC-Web client whose
trailers are not exposed fails in a way that looks like a server bug. CORS is a browser-side
control and not authentication: a non-browser client is unaffected by the list, and this
service still has no authentication of any kind.

**TLS.** `-tls-cert` and `-tls-key` (both or neither) serve everything above over HTTPS on the
same port, negotiating `h2` and `http/1.1`, minimum TLS 1.2. A browser on an `https://` page
cannot post to `http://`, so this is a prerequisite rather than a hardening step. Without the
flags the port is cleartext with `h2c`, which is what a gRPC client needs against a port that
offers no TLS, and which is appropriate only inside a trusted network or behind a proxy that
terminates TLS. Bidirectional streaming, if it is ever added, would additionally require
HTTP/2 end to end — in a browser that means TLS.

**The `grpc-web-text` gap, stated rather than papered over.** `connect-go` v1.20 implements
`application/grpc-web` and `application/grpc-web+json` but not the base64 `grpc-web-text`
variant; posting that content type answers `415`, and a test pins that. `grpc-web-text` exists
for clients that cannot read a binary response body — the old `XMLHttpRequest` paths. Any
client using `fetch`, which is what `@connectrpc/connect-web` and `grpc-web`'s fetch transport
do, never asks for it. **This is acceptable for a `fetch`-based browser client and only for
one**: a client that needs `grpc-web-text` needs a proxy in front of this service.

## Health

`GET /health` answers on the main port and reports the build:

```console
$ curl -s http://localhost:50051/health
{"service":"sysml-grpc","status":"ok","version":"0.2.1"}
```

A separate HTTP health port existed because the gRPC-only server could not serve a plain GET.
It no longer has to, so `-health-port` is **deprecated**:

| | Behavior |
|---|---|
| Today, default (`-health-port 8081`) | `/health` answers on both the main port and 8081; the second listener logs a deprecation warning |
| Today, `-health-port 0` | no second listener; `/health` answers on the main port |
| A future release | the default becomes `0`; the flag stays accepted for a release after that |
| `-transport grpc` | unchanged — 8081 is the only health surface, and no warning is logged |

Poll the main port. Nothing in this repository polls 8081: the Python client's readiness probe
is a `GetDiagnostics` call over gRPC, not an HTTP GET, so it is unaffected by every row of that
table.

## Every protocol is tested, not just served

A second protocol surface that no test drives rots. The conformance suite
([`conformance/`](https://github.com/Open-MBEE/OpenSysML/tree/main/conformance)) runs its whole
scenario list once per protocol — gRPC, Connect with a protobuf body, Connect with a JSON body
— against one service, asserting identical results *and* identical status codes:

```console
$ make conformance                                    # all three protocols
$ go run ./cmd/conformance -protocols connect-json    # one of them
$ go run ./cmd/conformance -transport grpc -protocols grpc
```

The JSON-specific edges above — `int64` as a string, error shape — are exactly what that
parameterization covers, in the encoding a browser client will use.
