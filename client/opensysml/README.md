# opensysml — the public Go API

`github.com/Open-MBEE/OpenSysML/client/opensysml` is the Go surface of OpenSysML:
parse SysML v2 models, look up symbols, evaluate expressions and instantiate
parts, from Go code, with the engine that is already linked into your binary.

```go
client, err := opensysml.New()
if err != nil { ... }
defer client.Close()

model, err := client.ParseFile(ctx, "vehicle.sysml")
mass, err := client.Evaluate(ctx, model, "mass", opensysml.WithSubject("Demo::sedan"))
inst, err := client.Instantiate(ctx, model, "Demo::Vehicle")
```

## Why in-process is the default

This is a Go repository: a Go program that imports this module already links
the parser, the semantic engine and the runtime. `New` calls them directly —
no port, no child process, no serialization round trip. It answers through the
same service implementation (`internal/grpc.Service`) the wire transports
serve, so the semantics are the service's semantics: the same content-addressed
parse cache and model hashes, the same capability list, the same in-band
failures, the same runtime budgets (read from the environment, as the service
reads them).

`Dial` is for the service you did not start: a shared long-lived `sysml-grpc`
someone else runs, addressed explicitly (`"host:50051"` or
`"https://sysml.example.com"`). It speaks the Connect protocol with protobuf
bodies by default — JSON (`WithJSONBody`) costs an order of magnitude in
encoding time on large responses and is a debugging affordance, per
`docs/internals/design/transport-evaluation.md`. This package never spawns a
service: a Go process that wants the engine in-process uses `New`, which is
strictly better than a private child — the child's entire job would be to serve
the code `New` already calls.

## Errors

A call fails in exactly one of two ways, and the difference is part of the API
because it is part of the wire contract:

| Failure | Type | Test with | Example |
| --- | --- | --- | --- |
| The call is refused | `*StatusError` | `errors.Is(err, opensysml.CodeNotFound)` | an unknown model hash, an unreadable path |
| The answer reports a failure | `*FailureError` | `errors.Is(err, opensysml.ErrFailure)` | an unparsable expression, an unknown symbol |

`StatusError.Code` is the canonical gRPC status code, whichever implementation
answered: in process it is the code the handler refused with; remotely it is
the Connect error code, which numbers identically. A transport failure that
never reached the service is `CodeUnavailable`. A panic in the engine does not
cross the boundary: it arrives as `CodeInternal`.

Syntax errors are neither: parsing broken source succeeds, and the errors are
`Model.Diagnostics` — the same shape the LSP and the wire report.

## Ownership

Everything a `Client` returns is a copy the caller owns. In process there is no
serialization boundary to force this, so it is a documented promise instead:
no returned value aliases engine state, and mutating a returned `Model`,
`Symbol`, `Value` or `Instance` changes nothing about the model, the cache or
later answers.

## Capabilities

Negotiate on names, not versions: `ServerInfo.Capabilities` lists what the
answering implementation supports, and `ServerInfo.Has` checks one. A request
that asks for an unavailable capability is refused with `CodeUnimplemented`;
capabilities that describe response population instead omit the fields they
name. Check the list first for an operation-specific error (the `Capability*`
constants name the known ones).

## Stability

The module is `v0`, so the Go compatibility promise does not yet formally bind
it (see `docs/project/releasing.md` for how releases are cut). Within `v0`,
this package's exported surface is what OpenSysML commits to keeping
compatible:

- **Stable**: `Client` and its seven operations, `New`, `Dial`, the option
  functions, the error model (`Code`, `StatusError`, `FailureError`,
  `ErrFailure`), and the data types they exchange. Changes will be additive.
- **Experimental**: nothing currently. A future addition that is not yet a
  commitment will say so in its Godoc.

The `Client` interface is sealed, so adding a method is not a breaking change.

Explicitly **out of v1**, deliberately rather than half-implemented: generated
model-ergonomics types, the edit API (`ApplyEdits`), RDF conversion
(`Convert`), verification helpers (`VerifyConstraint`, `VerifyRequirement`,
`VerifySatisfaction`), behavior execution (`ExecuteAction`, `ExecuteState`),
calc evaluation (`EvaluateCalc`) and queries (`Query`). For those, use the
generated stubs in `api/proto` / `api/proto/protoconnect` against a running
service.

None of the v1 operations shells out to an SMT solver, so an in-process caller
needs nothing installed. The verification RPCs are the ones that discover
`z3`/`cvc5` (or `$OPENSYSML_SMT`); when verification helpers arrive here, the
package will document that requirement and report a missing solver in the
verdict rather than failing the call.

## Conformance

The language-independent suite in `conformance/` runs through this package —
not through raw stubs — as the `pkg` (in-process) and `pkg-connect` (remote)
protocols of the reference runner:

```sh
make conformance-pkg
# or: go run ./cmd/conformance -protocols pkg,pkg-connect -allow-skips
```

Scenarios for RPCs outside the v1 surface are reported as skips, with the
reason named per scenario in the report; nothing covered is skipped.
