package lsp

import "go.lsp.dev/uri"

// uriToName converts an LSP document URI to the workspace document name (an OS
// filesystem path).
func uriToName(u uri.URI) string { return u.Filename() }

// nameToURI converts a workspace document name (an OS path) to an LSP URI.
func nameToURI(name string) uri.URI { return uri.File(name) }
