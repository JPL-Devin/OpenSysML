// Package opensysml is the public Go API for OpenSysML: parse SysML v2
// models, look up symbols, evaluate expressions and instantiate parts.
//
// # Two implementations, one interface
//
// New answers in process: the engine is already linked into a Go binary that
// imports this module, so the default implementation calls it directly — no
// port, no child process, no serialization. Dial answers through a sysml-grpc
// service someone else runs, named by an explicit address; this package never
// starts one. Both implement Client identically, and the conformance suite in
// conformance/ runs through both to hold them to it.
//
//	client, err := opensysml.New()
//	// or: client, err := opensysml.Dial("localhost:50051")
//
// # Errors
//
// A call fails in one of two documented ways, and the difference is part of
// the API because it is part of the wire contract:
//
//   - A refused call is a *StatusError carrying the canonical gRPC status
//     code, whichever implementation answered. errors.Is(err, CodeNotFound)
//     tests the code: an unknown model hash, for example, is CodeNotFound.
//   - A failure the service reports inside a successful answer — an
//     unparsable expression, an unknown symbol, a failed instantiation — is a
//     *FailureError, matched by errors.Is(err, ErrFailure), carrying the
//     service's message and any diagnostics.
//
// A panic in the engine does not cross this boundary: the in-process
// implementation reports it as a *StatusError with CodeInternal, the status
// the wire would answer for a crashed handler.
//
// # Ownership
//
// Everything a Client returns is a copy the caller owns. The in-process
// implementation has no serialization boundary, so this is a promise, not a
// consequence: no returned value aliases engine state, and mutating one
// changes nothing about the model, the cache or later answers.
//
// # Capabilities
//
// Negotiate on ServerInfo.Capabilities, not on versions. The service does not
// refuse a call whose capability it lacks — it answers without the behaviour —
// so require the capability name (ServerInfo.Has) before relying on it.
//
// # Stability
//
// The module is v0, so the Go compatibility promise does not yet bind it.
// Within v0, the surface of this package is what OpenSysML commits to keeping
// compatible: see the package README for the statement and the explicit list
// of what v1 leaves out (editing, conversion, verification, behavior
// execution, generated model types).
package opensysml
