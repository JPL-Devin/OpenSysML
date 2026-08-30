// Package docplan compiles native SysML v2 document definitions into
// immutable document plans that reference compiled query programs.
package docplan

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/provenance"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ContentKind classifies one planned content node.
type ContentKind string

const (
	ContentSection   ContentKind = "section"
	ContentParagraph ContentKind = "paragraph"
	ContentTable     ContentKind = "table"
	ContentList      ContentKind = "list"
)

// ListStyle is the declared rendering style of a planned list.
type ListStyle string

const (
	ListBullet ListStyle = "bullet"
	ListNumber ListStyle = "number"
)

// BindingKind classifies one planned binding value.
type BindingKind string

const (
	BindingElement BindingKind = "element"
	BindingString  BindingKind = "string"
	BindingInteger BindingKind = "integer"
	BindingReal    BindingKind = "real"
	BindingBoolean BindingKind = "boolean"
)

// BindingValue is one statically planned value for a query parameter.
type BindingValue struct {
	kind    BindingKind
	element *symbols.Symbol
	text    string
	integer int64
	real    float64
	boolean bool
	origin  provenance.Origin
}

// Kind returns the classification of the value.
func (v BindingValue) Kind() BindingKind { return v.kind }

// Element returns the bound model element when the value is an element.
func (v BindingValue) Element() (*symbols.Symbol, bool) {
	return v.element, v.kind == BindingElement
}

// String returns the bound text when the value is a string.
func (v BindingValue) String() (string, bool) { return v.text, v.kind == BindingString }

// Integer returns the bound integer when the value is an integer.
func (v BindingValue) Integer() (int64, bool) { return v.integer, v.kind == BindingInteger }

// Real returns the bound real when the value is a real.
func (v BindingValue) Real() (float64, bool) { return v.real, v.kind == BindingReal }

// Boolean returns the bound boolean when the value is a boolean.
func (v BindingValue) Boolean() (bool, bool) { return v.boolean, v.kind == BindingBoolean }

// Origin returns the source declaration behind the value.
func (v BindingValue) Origin() provenance.Origin { return v.origin }

// Binding supplies planned values to one parameter of a referenced query.
type Binding struct {
	parameter string
	values    []BindingValue
	origin    provenance.Origin
}

// Parameter returns the bound parameter name.
func (b Binding) Parameter() string { return b.parameter }

// Values returns the planned values in declaration order.
func (b Binding) Values() []BindingValue { return append([]BindingValue(nil), b.values...) }

// Origin returns the source declaration behind the binding.
func (b Binding) Origin() provenance.Origin { return b.origin }

// QueryRef is a planned reference to a compiled query with its bindings.
type QueryRef struct {
	entry    string
	program  *queryplan.Program
	bindings []Binding
	origin   provenance.Origin
}

// Entry returns the fully-qualified name of the referenced query.
func (q *QueryRef) Entry() string { return q.entry }

// Program returns the compiled program of the referenced query.
func (q *QueryRef) Program() *queryplan.Program { return q.program }

// Bindings returns the planned bindings in declaration order.
func (q *QueryRef) Bindings() []Binding {
	out := make([]Binding, len(q.bindings))
	for i, binding := range q.bindings {
		out[i] = Binding{
			parameter: binding.parameter,
			values:    append([]BindingValue(nil), binding.values...),
			origin:    binding.origin,
		}
	}
	return out
}

// Origin returns the source declaration behind the reference.
func (q *QueryRef) Origin() provenance.Origin { return q.origin }

// Content is one planned content node: a section, paragraph, table, or list.
type Content struct {
	kind     ContentKind
	name     string
	title    string
	text     string
	caption  string
	style    ListStyle
	query    *QueryRef
	children []Content
	origin   provenance.Origin
}

// Kind returns the classification of the node.
func (c Content) Kind() ContentKind { return c.kind }

// Name returns the declared name of the node, empty when anonymous.
func (c Content) Name() string { return c.name }

// Title returns the declared title of a section.
func (c Content) Title() string { return c.title }

// Text returns the static text of a paragraph, empty when query-backed.
func (c Content) Text() string { return c.text }

// Caption returns the declared caption of a table.
func (c Content) Caption() string { return c.caption }

// Style returns the declared style of a list.
func (c Content) Style() ListStyle { return c.style }

// Query returns the referenced query of a query-backed node, or nil.
func (c Content) Query() *QueryRef { return c.query }

// Children returns the nested content of a section in declaration order.
func (c Content) Children() []Content { return cloneContent(c.children) }

// Origin returns the source declaration behind the node.
func (c Content) Origin() provenance.Origin { return c.origin }

func cloneContent(content []Content) []Content {
	out := make([]Content, len(content))
	for i, child := range content {
		out[i] = Content{
			kind:     child.kind,
			name:     child.name,
			title:    child.title,
			text:     child.text,
			caption:  child.caption,
			style:    child.style,
			query:    child.query,
			children: cloneContent(child.children),
			origin:   child.origin,
		}
	}
	return out
}

// Plan is an immutable compiled document definition.
type Plan struct {
	compiled bool
	name     string
	title    string
	content  []Content
	origin   provenance.Origin
}

// Compiled reports whether the plan was produced by Compile.
func (p *Plan) Compiled() bool { return p != nil && p.compiled }

// Name returns the fully-qualified name of the document definition.
func (p *Plan) Name() string { return p.name }

// Title returns the declared document title.
func (p *Plan) Title() string { return p.title }

// Content returns the planned top-level content in declaration order.
func (p *Plan) Content() []Content { return cloneContent(p.content) }

// Origin returns the source declaration behind the document.
func (p *Plan) Origin() provenance.Origin { return p.origin }
