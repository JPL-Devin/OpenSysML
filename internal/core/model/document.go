// Package model owns the workspace: the set of documents, the global symbol
// index, and the reindex pipeline that keeps them consistent.
package model

import (
	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Document is the parsed state of one source file.
type Document struct {
	Name             string
	Content          []byte
	Version          int
	AST              *ast.RootNamespace
	ParseDiagnostics []parser.Diagnostic
	Scope            *symbols.Scope
	sf               *source.SourceFile
}

// newDocument parses content and builds the document's local scope tree.
func newDocument(name string, content []byte, version int) *Document {
	sf := source.New(name, content)
	p := parser.New(sf)
	root := p.ParseFile()
	return &Document{
		Name:             name,
		Content:          content,
		Version:          version,
		AST:              root,
		ParseDiagnostics: p.Diagnostics,
		Scope:            symbols.Build(root),
		sf:               sf,
	}
}
