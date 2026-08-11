// Package export saves a SysML v2 model to a file and converts between the
// two representations Systemica can write: SysML textual notation and RDF
// Turtle.
//
// # SysML output
//
// Saving a model that came from source writes that source, re-indented by
// internal/core/format. Printing the AST instead would drop comments, notes and
// anything the parser recorded as an ErrorNode, so a save has to keep the token
// stream (see the format package doc).
//
// # RDF output
//
// The graph uses the SysML vocabulary and element IRIs of the Flexo MMS SysML
// v2 service (https://www.omg.org/spec/SysML# and urn:sysmlv2:element:), so a
// converted model loads into that service's triplestore. Elements are addressed
// by qualified name (`urn:sysmlv2:element:Demo::Vehicle`), which makes the IRIs
// stable across conversions of the same model rather than newly generated each
// time.
//
// Each element carries its metaclass as rdf:type and its declaration as SysML
// metamodel properties: declaredName, declaredShortName, owningNamespace,
// visibility, direction, the feature flags, the typing and specialization
// clauses, multiplicity bounds and its value. Three properties the metamodel
// does not define live in a separate urn:systemica:sysml: namespace so a
// consumer can tell them from the standard vocabulary: memberIndex (declaration
// order, which the notation is sensitive to and RDF is not), hasBody, and
// sourceText.
//
// # Known limitations
//
// Expression-valued positions — feature values, multiplicity bounds, filter
// conditions, succession guards — are carried as their source text rather than
// as expression trees. They convert back exactly, and a consumer that only
// reads the model structure is unaffected, but SPARQL cannot see inside them.
//
// A declaration whose head binds ends rather than naming a single feature —
// connect, bind, flow, succession, transition, accept and satisfy — is carried
// as sourceText, with its structural properties alongside. Those heads convert
// back exactly, but a graph produced by another tool will not have the text, and
// converting such an element to notation then reports it as unsupported rather
// than guessing at the ends.
//
// Lexical comments do not survive a round trip through RDF. Saving straight to
// notation keeps them, because that path writes the source; converting to Turtle
// keeps only what the model declares, and `//` and `/* */` trivia is attached to
// no element. The comment and doc keywords are declarations rather than trivia,
// so those do convert both ways. A model whose comments matter should be saved
// to notation, which is why the two paths differ.
package export

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/format"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/rdf"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// Format is one of the representations a model can be read from or written to.
type Format int

const (
	// FormatSysML is SysML v2 / KerML textual notation.
	FormatSysML Format = iota
	// FormatTurtle is RDF in Turtle syntax.
	FormatTurtle
)

func (f Format) String() string {
	if f == FormatTurtle {
		return "ttl"
	}
	return "sysml"
}

// formatNames are the names accepted on the command line for each format.
var formatNames = map[string]Format{
	"sysml":  FormatSysML,
	"kerml":  FormatSysML,
	"text":   FormatSysML,
	"ttl":    FormatTurtle,
	"turtle": FormatTurtle,
	"rdf":    FormatTurtle,
}

// ParseFormat resolves a format name, as given to `-from`/`-to`.
func ParseFormat(name string) (Format, error) {
	if f, ok := formatNames[strings.ToLower(strings.TrimSpace(name))]; ok {
		return f, nil
	}
	return 0, fmt.Errorf("unknown format %q: expected sysml, kerml, ttl, turtle or rdf", name)
}

// FormatOfPath infers the format from a file extension, so that the common case
// needs no -from/-to.
func FormatOfPath(path string) (Format, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".sysml", ".kerml":
		return FormatSysML, nil
	case ".ttl", ".turtle":
		return FormatTurtle, nil
	case "":
		return 0, fmt.Errorf("cannot tell the format of %q: it has no extension, so pass -from/-to", path)
	default:
		return 0, fmt.Errorf("cannot tell the format of %q: expected .sysml, .kerml or .ttl, so pass -from/-to", path)
	}
}

// SyntaxError reports that the input could not be read as its format. It lists
// every syntax error rather than only the first, so one conversion attempt
// shows everything that needs fixing.
type SyntaxError struct {
	Name     string
	Messages []string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("%s: %d syntax error(s):\n  %s", e.Name, len(e.Messages), strings.Join(e.Messages, "\n  "))
}

// Convert reads data in the from format and writes it in the to format. name is
// used in diagnostics and needs no relation to a file on disk.
//
// Converting between two formats requires the input to be syntactically valid:
// a model with syntax errors is rejected, because the tree the parser recovers
// from broken input does not hold the declarations it could not read, and a
// graph built from it would be quietly missing them.
func Convert(name string, data []byte, from, to Format) ([]byte, error) {
	switch {
	case from == FormatSysML && to == FormatSysML:
		// A save of textual notation: keep every lexeme, fix the indentation.
		// The syntax is still checked, so no direction accepts input the parser
		// cannot read.
		if err := checkSyntax(name, data); err != nil {
			return nil, err
		}
		out, err := format.Source(name, data, format.DefaultOptions)
		if err != nil {
			return nil, err
		}
		return out, nil

	case from == FormatSysML && to == FormatTurtle:
		graph, err := SysMLToRDF(name, data)
		if err != nil {
			return nil, err
		}
		return rdf.WriteTurtle(graph), nil

	case from == FormatTurtle && to == FormatSysML:
		graph, err := rdf.ParseTurtle(data)
		if err != nil {
			return nil, &SyntaxError{Name: name, Messages: []string{err.Error()}}
		}
		return ToSysML(graph)

	default:
		// Turtle to Turtle: read and rewrite, which normalizes the document
		// and reports anything the reader cannot represent.
		graph, err := rdf.ParseTurtle(data)
		if err != nil {
			return nil, &SyntaxError{Name: name, Messages: []string{err.Error()}}
		}
		return rdf.WriteTurtle(graph), nil
	}
}

// SysMLToRDF parses SysML notation and converts it to a graph.
func SysMLToRDF(name string, data []byte) (*rdf.Graph, error) {
	file := source.New(name, data)
	p := parser.New(file)
	root := p.ParseFile()
	if err := syntaxError(name, file, p); err != nil {
		return nil, err
	}
	return ToRDF(file, root)
}

// checkSyntax reports the notation's syntax errors, if any.
func checkSyntax(name string, data []byte) error {
	file := source.New(name, data)
	p := parser.New(file)
	p.ParseFile()
	return syntaxError(name, file, p)
}

// syntaxError turns a parse's diagnostics into a SyntaxError, or nil when the
// input parsed clean.
func syntaxError(name string, file *source.SourceFile, p *parser.Parser) error {
	if len(p.Diagnostics) == 0 {
		return nil
	}
	lines := file.Lines()
	messages := make([]string, 0, len(p.Diagnostics))
	for _, diag := range p.Diagnostics {
		pos := lines.PosAt(diag.Span.Offset)
		messages = append(messages, fmt.Sprintf("%d:%d: %s", pos.Line, pos.Col, diag.Message))
	}
	return &SyntaxError{Name: name, Messages: messages}
}
