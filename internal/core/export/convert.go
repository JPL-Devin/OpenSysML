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
	"errors"
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

// UnknownFormatError reports that a path does not say which format to write.
// The remedy differs by surface — the command line has -from/-to, the REPL only
// has the file name — so the caller supplies it with Advise.
type UnknownFormatError struct {
	Path string
	// NoExtension distinguishes a path with no extension from one whose
	// extension names a format we do not write.
	NoExtension bool
	// Advice is the surface's remedy, appended as "…, so <advice>".
	Advice string
}

func (e *UnknownFormatError) Error() string {
	reason := "expected .sysml, .kerml or .ttl"
	if e.NoExtension {
		reason = "it has no extension"
	}
	msg := fmt.Sprintf("cannot tell the format of %q: %s", e.Path, reason)
	if e.Advice != "" {
		msg += ", so " + e.Advice
	}
	return msg
}

// Advise returns err with the surface's remedy attached when it is an
// *UnknownFormatError, and unchanged otherwise.
func Advise(err error, advice string) error {
	var unknown *UnknownFormatError
	if errors.As(err, &unknown) {
		return &UnknownFormatError{Path: unknown.Path, NoExtension: unknown.NoExtension, Advice: advice}
	}
	return err
}

// FormatOfPath infers the format from a file extension, so that the common case
// needs no -from/-to. A path that names no format yields an
// *UnknownFormatError carrying no advice; pass it through Advise to add the
// remedy the calling surface offers.
func FormatOfPath(path string) (Format, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".sysml", ".kerml":
		return FormatSysML, nil
	case ".ttl", ".turtle":
		return FormatTurtle, nil
	case "":
		return 0, &UnknownFormatError{Path: path, NoExtension: true}
	default:
		return 0, &UnknownFormatError{Path: path}
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
	out, _, err := convert(name, data, from, to, false)
	return out, err
}

// ConvertTolerant is Convert with one difference: notation converted back to
// notation is written even when the parser could not read all of it, and its
// syntax errors are returned as a warning instead of an error. That direction
// re-indents the input rather than building anything from the parse tree, so the
// output is exactly as valid as the input and refusing it would only strand
// work that exists nowhere else — which is why the REPL's `%save` uses it for a
// session buffer. Every other direction builds a graph from the tree, where
// declarations the parser could not read would be silently missing, so a broken
// model is still rejected.
func ConvertTolerant(name string, data []byte, from, to Format) ([]byte, *SyntaxError, error) {
	return convert(name, data, from, to, true)
}

func convert(name string, data []byte, from, to Format, tolerateSyntaxErrors bool) ([]byte, *SyntaxError, error) {
	switch {
	case from == FormatSysML && to == FormatSysML:
		// A save of textual notation: keep every lexeme, fix the indentation.
		syntax := checkSyntax(name, data)
		if syntax != nil && !tolerateSyntaxErrors {
			return nil, nil, syntax
		}
		out, err := format.Source(name, data, format.DefaultOptions)
		if err != nil {
			return nil, nil, err
		}
		return out, syntax, nil

	case from == FormatSysML && to == FormatTurtle:
		graph, err := SysMLToRDF(name, data)
		if err != nil {
			return nil, nil, err
		}
		return rdf.WriteTurtle(graph), nil, nil

	case from == FormatTurtle && to == FormatSysML:
		graph, err := rdf.ParseTurtle(data)
		if err != nil {
			return nil, nil, &SyntaxError{Name: name, Messages: []string{err.Error()}}
		}
		out, err := ToSysML(graph)
		return out, nil, err

	default:
		// Turtle to Turtle: read and rewrite, which normalizes the document
		// and reports anything the reader cannot represent.
		graph, err := rdf.ParseTurtle(data)
		if err != nil {
			return nil, nil, &SyntaxError{Name: name, Messages: []string{err.Error()}}
		}
		return rdf.WriteTurtle(graph), nil, nil
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
func checkSyntax(name string, data []byte) *SyntaxError {
	file := source.New(name, data)
	p := parser.New(file)
	p.ParseFile()
	return syntaxError(name, file, p)
}

// syntaxError turns a parse's diagnostics into a SyntaxError, or nil when the
// input parsed clean.
func syntaxError(name string, file *source.SourceFile, p *parser.Parser) *SyntaxError {
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
