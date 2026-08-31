# opensysml — the public Go API

`github.com/Open-MBEE/OpenSysML/client/opensysml` is the Go surface of OpenSysML:
parse SysML v2 models, look up symbols, evaluate expressions and instantiate
parts, from Go code, using the engine already linked into the calling binary.

```sh
go get github.com/Open-MBEE/OpenSysML@latest
```

Nothing else is installed: the SysML standard library is embedded in the module,
and the v1 operations never shell out.

```go
client, err := opensysml.New()
if err != nil { ... }
defer client.Close()

model, err := client.ParseFile(ctx, "vehicle.sysml")
mass, err := client.Evaluate(ctx, model, "mass", opensysml.WithSubject("Demo::sedan"))
inst, err := client.Instantiate(ctx, model, "Demo::Vehicle")
```

## What a model is here

`ParseFile` reads the one path it is given and `ParseSource` the one string, and
neither follows an import into a sibling file: a name declared in another file is
an unresolved reference, reported as a diagnostic on the model rather than as a
failed call.

A model that lives in several files is parsed as one model by `ParseFiles`, or by
`ParseDocuments` for files and in-memory sources together:

```go
model, err := client.ParseFiles(ctx, paths)

model, err := client.ParseDocuments(ctx, []opensysml.Document{
	opensysml.File("lib.sysml"),
	opensysml.Source("top.sysml", generated),
})
```

Each document is parsed on its own and all of them are indexed together, so an
import between them resolves and every symbol of the set is one lookup,
evaluation or instantiation away. Nothing is concatenated: a document keeps its
own name, so a diagnostic locates itself in the file it came from, and
`Model.Roots` holds each document's root namespace in the order given —
`Model.Root` is the first, as it is for a one-document model. A set is cached by
what is in it, so parsing the same documents again answers the same model hash.

Two operations write one document's own notation back out, and they are refused
with `CodeFailedPrecondition` for a model of several rather than applied to one
of them: conversion from a model handle, and editing. Both are outside the v1
surface anyway (see below).

## Concurrency, contexts and lifetime

A `Client` is safe for concurrent use: any number of goroutines may call it, and
each answer belongs to its caller. One `Client` per process is the intended
shape — `New` builds and prewarms a standard-library index, which is what makes
it worth keeping.

Contexts are honoured as the wire honours them. A call whose context is already
done is refused with `CodeCanceled` or `CodeDeadlineExceeded`; a context that
ends while a call is running withholds the answer the same way, because the
engine — like a service answering a caller who stopped listening — still runs
the call it started. A deadline therefore bounds how long a caller waits, not
how long the engine works.

After `Close`, every call is refused with `CodeUnavailable`, and closing twice is
not an error, so a deferred `Close` beside an explicit one is safe.

`ServerInfo.Version` is the version of OpenSysML linked into the binary — the
module version an importing program resolved, `dev` when it was built from a
checkout. It is informational: negotiate on capabilities.

## Why in-process is the default

This is a Go repository: a Go program that imports this module already links
the parser, the semantic engine and the runtime. `New` calls them directly —
no port, no child process, no serialization round trip. It answers through the
same service implementation (`internal/grpc.Service`) the wire transports
serve, so the semantics are the service's semantics: the same content-addressed
parse cache and model hashes, the same capability list, the same in-band
failures, the same runtime budgets (read from the environment, as the service
reads them).

`Dial` addresses a service the caller did not start: a shared, long-lived
`sysml-grpc` run elsewhere, addressed explicitly (`"host:50051"` or
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

A `StatusError` renders as `opensysml: NOT_FOUND: model not found: …`, naming
the code canonically. `StatusError.Code` is the canonical gRPC status code,
whichever implementation answered: in process it is the code the handler refused
with; remotely it is the Connect error code, which numbers identically. A transport failure that
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
