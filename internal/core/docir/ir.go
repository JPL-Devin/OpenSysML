// Package docir evaluates compiled document plans into an immutable,
// backend-agnostic document tree with provenance on every node.
package docir

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/provenance"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// ContentKind classifies one evaluated content node.
type ContentKind string

const (
	ContentSection   ContentKind = "section"
	ContentParagraph ContentKind = "paragraph"
	ContentTable     ContentKind = "table"
	ContentList      ContentKind = "list"
	ContentDiagram   ContentKind = "diagram"
)

// ListStyle is the rendering style of an evaluated list.
type ListStyle string

const (
	ListBullet ListStyle = "bullet"
	ListNumber ListStyle = "number"
)

// TextRun is one piece of paragraph or list-item text with its provenance:
// static text carries its declaration, query-backed text its query value.
type TextRun struct {
	text   string
	origin provenance.Origin
}

// Text returns the run's text.
func (r TextRun) Text() string { return r.text }

// Origin returns the source declaration or query value behind the run.
func (r TextRun) Origin() provenance.Origin { return r.origin }

// ListItem is one evaluated list item, produced from one query row.
type ListItem struct {
	runs   []TextRun
	origin provenance.Origin
}

// Runs returns the item's text runs in column order.
func (i ListItem) Runs() []TextRun { return append([]TextRun(nil), i.runs...) }

// Origin returns the query row behind the item.
func (i ListItem) Origin() provenance.Origin { return i.origin }

// Content is one evaluated content node: a section, paragraph, table, list,
// or diagram.
type Content struct {
	kind        ContentKind
	name        string
	title       string
	caption     string
	style       ListStyle
	runs        []TextRun
	columns     []queryexec.Column
	rows        []queryexec.Row
	items       []ListItem
	rendering   *view.Rendering
	direction   view.Direction
	children    []Content
	query       string
	queryOrigin provenance.Origin
	origin      provenance.Origin
}

// Kind returns the classification of the node.
func (c Content) Kind() ContentKind { return c.kind }

// Name returns the declared name of the node, empty when anonymous.
func (c Content) Name() string { return c.name }

// Title returns the title of a section.
func (c Content) Title() string { return c.title }

// Caption returns the caption of a table or diagram.
func (c Content) Caption() string { return c.caption }

// Style returns the style of a list.
func (c Content) Style() ListStyle { return c.style }

// Runs returns the text runs of a paragraph in row and column order.
func (c Content) Runs() []TextRun { return append([]TextRun(nil), c.runs...) }

// Columns returns the ordered projected columns of a table, preserved even
// when the query returned no rows.
func (c Content) Columns() []queryexec.Column { return append([]queryexec.Column(nil), c.columns...) }

// Rows returns the ordered rows of a table with their typed cells.
func (c Content) Rows() []queryexec.Row { return append([]queryexec.Row(nil), c.rows...) }

// Items returns the ordered items of a list.
func (c Content) Items() []ListItem {
	out := make([]ListItem, len(c.items))
	for i, item := range c.items {
		out[i] = ListItem{runs: append([]TextRun(nil), item.runs...), origin: item.origin}
	}
	return out
}

// Rendering returns a copy of the resolved view content of a diagram, nil
// for every other kind.
func (c Content) Rendering() *view.Rendering { return c.rendering.Clone() }

// Direction returns the stated flow direction of a diagram, empty for the
// kind's default.
func (c Content) Direction() view.Direction { return c.direction }

// Children returns the nested content of a section in declaration order.
func (c Content) Children() []Content { return cloneContent(c.children) }

// Query returns the fully-qualified name of the query behind a query-backed
// node, empty for sections and static paragraphs.
func (c Content) Query() string { return c.query }

// QueryOrigin returns the declaration of the query behind a query-backed node.
func (c Content) QueryOrigin() provenance.Origin { return c.queryOrigin }

// Origin returns the source declaration behind the node.
func (c Content) Origin() provenance.Origin { return c.origin }

func cloneContent(content []Content) []Content {
	out := make([]Content, len(content))
	for i, child := range content {
		out[i] = Content{
			kind:        child.kind,
			name:        child.name,
			title:       child.title,
			caption:     child.caption,
			style:       child.style,
			runs:        append([]TextRun(nil), child.runs...),
			columns:     append([]queryexec.Column(nil), child.columns...),
			rows:        append([]queryexec.Row(nil), child.rows...),
			items:       child.Items(),
			rendering:   child.rendering.Clone(),
			direction:   child.direction,
			children:    cloneContent(child.children),
			query:       child.query,
			queryOrigin: child.queryOrigin,
			origin:      child.origin,
		}
	}
	return out
}

// Document is an immutable evaluated document.
type Document struct {
	name    string
	title   string
	content []Content
	origin  provenance.Origin
}

// Name returns the fully-qualified name of the document definition.
func (d *Document) Name() string { return d.name }

// Title returns the document title.
func (d *Document) Title() string { return d.title }

// Content returns the evaluated top-level content in declaration order.
func (d *Document) Content() []Content { return cloneContent(d.content) }

// Origin returns the source declaration behind the document.
func (d *Document) Origin() provenance.Origin { return d.origin }
