package view

import (
	"errors"
	"fmt"
)

// Form is a written form of a rendering: the human-readable text every kind has,
// and the machine-readable form of the kind — a Mermaid diagram for the
// graph-shaped kinds, a Markdown table for the tabular one.
type Form string

const (
	// FormText is the human-readable form, which the REPL prints.
	FormText Form = "text"
	// FormMermaid is a Mermaid diagram of a graph-shaped rendering.
	FormMermaid Form = "mermaid"
	// FormMarkdown is a Markdown table of a tabular rendering.
	FormMarkdown Form = "markdown"
)

// Forms are the forms a rendering can be asked for, in the order they are
// offered.
func Forms() []Form { return []Form{FormText, FormMermaid, FormMarkdown} }

// The widths the text form is written to.
const (
	// WidthUnbounded writes every column as wide as its widest cell, which is
	// what a form written to a file or a pipe rather than a terminal is.
	WidthUnbounded = 0
	// minColumnWidth is how narrow a column is made to fit a terminal before the
	// row is left to overflow it.
	minColumnWidth = 8
	// columnGap separates the columns of the text form.
	columnGap = 2
)

// MachineForm is the machine-readable form of renderings of this kind.
func (k Kind) MachineForm() Form {
	if k == KindTable {
		return FormMarkdown
	}
	return FormMermaid
}

// ErrWrongForm is a form asked for that renderings of the kind are not written
// in. WrongFormError wraps it.
var ErrWrongForm = errors.New("rendering is not written in that form")

// WrongFormError is a form asked for that does not fit the rendering's kind: a
// Mermaid diagram of a table, or a Markdown table of a diagram. It names the
// form the kind is written in instead, and never writes another form silently.
type WrongFormError struct {
	// Form is the form asked for, Kind the rendering's kind, and View the view
	// rendered, by qualified name.
	Form Form
	Kind Kind
	View string
}

func (e *WrongFormError) Error() string {
	return fmt.Sprintf("%s: %s %s rendering is not written as %s; ask for %s or %s",
		e.View, e.Kind.article(), e.Kind, e.Form, FormText, e.Kind.MachineForm())
}

func (e *WrongFormError) Unwrap() error { return ErrWrongForm }

// Write is the rendering in form, written to no particular width.
func (r *Rendering) Write(form Form) (string, error) {
	return r.WriteWidth(form, WidthUnbounded)
}

// WriteWidth is the rendering in form, the text form written to fit width. A
// form the kind is not written in is a *WrongFormError, and an unknown form
// names the ones there are.
func (r *Rendering) WriteWidth(form Form, width int) (string, error) {
	switch form {
	case FormText:
		return r.TextWidth(width), nil
	case FormMermaid, FormMarkdown:
		if form != r.Kind.MachineForm() {
			return "", &WrongFormError{Form: form, Kind: r.Kind, View: r.View}
		}
		if form == FormMarkdown {
			return r.Markdown(), nil
		}
		return r.Mermaid(), nil
	}
	return "", fmt.Errorf("unknown rendering form %q; the forms are %s, %s and %s", form, FormText, FormMermaid, FormMarkdown)
}
