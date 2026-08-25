// Package model owns the workspace: the set of documents, the global symbol
// index, and the reindex pipeline that keeps them consistent.
package model

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Document is the parsed state of one source file.
type Document struct {
	Name             string
	Content          []byte
	Version          int
	AST              *ast.RootNamespace
	ParseDiagnostics []parser.Diagnostic
	ParseWarnings    []parser.Diagnostic
	Scope            *symbols.Scope
	sf               *source.SourceFile
}

// newDocument parses content and builds the document's local scope tree.
func newDocument(name string, content []byte, version int) *Document {
	sf := source.New(name, content)
	root, diagnostics, warnings := parser.ParseFile(sf)
	scope := symbols.Build(root)
	symbols.SetDocName(scope, name)
	return &Document{
		Name:             name,
		Content:          content,
		Version:          version,
		AST:              root,
		ParseDiagnostics: diagnostics,
		ParseWarnings:    warnings,
		Scope:            scope,
		sf:               sf,
	}
}
