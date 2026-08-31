// Package docrender renders evaluated documents into backend-specific
// artifacts, consuming only the document IR — never plans, symbols, or ASTs.
package docrender

import (
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/docir"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// elementColumn heads the single column of a table whose query projected no
// properties, so its rows are the elements themselves.
const elementColumn = "element"

// Markdown renders an evaluated document as deterministic CommonMark: the
// title as a level-1 ATX heading, each section one level deeper (saturating
// at 6), paragraphs from space-joined text runs, GitHub-flavored pipe tables
// with projected column headers, bullet or numbered lists, and diagrams as
// fenced Mermaid blocks (table-kind views as pipe tables). Metacharacters
// in content are escaped so no value can corrupt the document structure.
func Markdown(document *docir.Document) (string, error) {
	if document == nil {
		return "", &Error{Kind: ErrorNilDocument}
	}
	var blocks []string
	blocks = append(blocks, heading(1, document.Title()))
	for _, node := range document.Content() {
		rendered, err := renderContent(node, 2)
		if err != nil {
			return "", err
		}
		blocks = append(blocks, rendered...)
	}
	return strings.Join(blocks, "\n\n") + "\n", nil
}

// renderContent renders one content node, with level the ATX heading level a
// section at this depth writes.
func renderContent(node docir.Content, level int) ([]string, error) {
	switch node.Kind() {
	case docir.ContentSection:
		blocks := []string{heading(level, node.Title())}
		for _, child := range node.Children() {
			rendered, err := renderContent(child, level+1)
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, rendered...)
		}
		return blocks, nil
	case docir.ContentParagraph:
		return []string{blockText(node.Runs())}, nil
	case docir.ContentTable:
		return renderTable(node), nil
	case docir.ContentList:
		return renderList(node), nil
	case docir.ContentDiagram:
		return renderDiagram(node)
	default:
		return nil, &Error{Kind: ErrorUnknownContent, Content: node.Name(), Actual: string(node.Kind())}
	}
}

// heading writes one ATX heading, saturating at level 6.
func heading(level int, title string) string {
	if level > 6 {
		level = 6
	}
	return strings.Repeat("#", level) + " " + inline(title)
}

// renderTable writes one pipe table, preceded by its caption in emphasis.
// A query without projected columns gets a single "element" column, and a
// table without rows still writes its header and delimiter.
func renderTable(node docir.Content) []string {
	var blocks []string
	if node.Caption() != "" {
		blocks = append(blocks, "*"+inline(node.Caption())+"*")
	}
	columns := node.Columns()
	names := make([]string, 0, len(columns))
	for _, column := range columns {
		names = append(names, column.Name())
	}
	if len(names) == 0 {
		names = []string{elementColumn}
	}
	var b strings.Builder
	writeTableRow(&b, names)
	b.WriteString("|" + strings.Repeat(" --- |", len(names)) + "\n")
	for _, row := range node.Rows() {
		writeTableRow(&b, tableCells(row, len(columns)))
	}
	return append(blocks, strings.TrimRight(b.String(), "\n"))
}

// renderDiagram writes one diagram, preceded by its caption in emphasis: a
// table-kind view as a pipe table, every other supported kind as a fenced
// Mermaid block drawn in the diagram's direction.
func renderDiagram(node docir.Content) ([]string, error) {
	return diagramBlocks(node.Name(), node.Caption(), node.Rendering(), node.Direction())
}

// diagramBlocks writes a caption in emphasis, then the rendering itself: a
// table-kind view as a pipe table, every other supported kind as Mermaid.
func diagramBlocks(name, caption string, rendering *view.Rendering, direction view.Direction) ([]string, error) {
	if rendering == nil {
		return nil, &Error{Kind: ErrorMissingRendering, Content: name}
	}
	var blocks []string
	if caption != "" {
		blocks = append(blocks, "*"+inline(caption)+"*")
	}
	if rendering.Kind == view.KindTable {
		return append(blocks, strings.TrimRight(rendering.MarkdownCells(tableCell), "\n")), nil
	}
	if !rendering.Kind.Supported() {
		return nil, &Error{Kind: ErrorUnrenderableDiagram, Content: name, Actual: string(rendering.Kind)}
	}
	mermaid := strings.TrimRight(rendering.MermaidDirected(direction), "\n")
	return append(blocks, "```mermaid\n"+mermaid+"\n```"), nil
}

// tableCells renders one row's cells, padded or truncated to the column count.
// A row of a table without projected columns is its element alone.
func tableCells(row queryexec.Row, columns int) []string {
	if columns == 0 {
		return []string{valueText(row.Element())}
	}
	cells := row.Cells()
	out := make([]string, columns)
	for i := 0; i < columns; i++ {
		if i < len(cells) {
			out[i] = cellText(cells[i])
		}
	}
	return out
}

// cellText renders one projected cell: its values joined by ", ".
func cellText(cell queryexec.Cell) string {
	values := cell.Values()
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = valueText(value)
	}
	return strings.Join(parts, ", ")
}

func writeTableRow(b *strings.Builder, cells []string) {
	escaped := make([]string, len(cells))
	for i, cell := range cells {
		escaped[i] = tableCell(cell)
	}
	b.WriteString("| " + strings.Join(escaped, " | ") + " |\n")
}

// renderList writes one bullet or numbered list, one item per query row. An
// empty list renders as nothing, which is the valid Markdown for no items.
func renderList(node docir.Content) []string {
	items := node.Items()
	if len(items) == 0 {
		return nil
	}
	lines := make([]string, 0, len(items))
	for i, item := range items {
		marker := "-"
		if node.Style() == docir.ListNumber {
			marker = strconv.Itoa(i+1) + "."
		}
		lines = append(lines, marker+" "+itemText(item.Runs()))
	}
	return []string{strings.Join(lines, "\n")}
}

// blockText renders a paragraph's runs joined by single spaces, escaped so the
// first character cannot open a heading, list, or quote.
func blockText(runs []docir.TextRun) string {
	return blockStart(itemText(runs))
}

// itemText joins text runs by single spaces with inline escaping.
func itemText(runs []docir.TextRun) string {
	parts := make([]string, len(runs))
	for i, run := range runs {
		parts[i] = inline(run.Text())
	}
	return strings.Join(parts, " ")
}

// valueText renders one typed value as plain, unescaped text: elements by
// qualified name (falling back to declared name), strings as their text,
// integers in base 10, reals in shortest 'g' form, booleans, and infinity as "*".
func valueText(value queryexec.Value) string {
	if element, ok := value.Element(); ok {
		if fqn := symbols.FQNOf(element); fqn != "" {
			return fqn
		}
		return element.Name
	}
	if text, ok := value.String(); ok {
		return text
	}
	if integer, ok := value.Integer(); ok {
		return strconv.FormatInt(integer, 10)
	}
	if real, ok := value.Real(); ok {
		return strconv.FormatFloat(real, 'g', -1, 64)
	}
	if boolean, ok := value.Boolean(); ok {
		return strconv.FormatBool(boolean)
	}
	if value.Kind() == queryexec.ValueInfinity {
		return "*"
	}
	return ""
}

// inlineEscaper backslash-escapes the characters that open Markdown or HTML
// structure anywhere in a line.
var inlineEscaper = strings.NewReplacer(
	`\`, `\\`,
	"`", "\\`",
	"*", `\*`,
	"_", `\_`,
	"[", `\[`,
	"]", `\]`,
	"<", `\<`,
	"&", `\&`,
	"|", `\|`,
	"#", `\#`,
)

// newlineNormalizer folds CRLF and lone CR to LF, so a carriage return cannot
// end a Markdown line either.
var newlineNormalizer = strings.NewReplacer("\r\n", "\n", "\r", "\n")

// inline escapes prose for any position in a line, folding newlines to spaces
// since paragraph structure comes from the document, not from run content.
func inline(text string) string {
	return inlineEscaper.Replace(strings.ReplaceAll(newlineNormalizer.Replace(text), "\n", " "))
}

// tableCell escapes one table cell, folding newlines to <br> so they cannot
// end the row.
func tableCell(text string) string {
	return strings.ReplaceAll(inlineEscaper.Replace(newlineNormalizer.Replace(text)), "\n", "<br>")
}

// blockStart escapes a leading quote, bullet, or ordered-list marker that
// would open block structure; inline escaping already covered "#". Leading
// spaces and tabs are dropped first: they would open an indented code block
// or shelter a marker, and CommonMark collapses them in a paragraph anyway.
func blockStart(text string) string {
	text = strings.TrimLeft(text, " \t")
	if text == "" {
		return text
	}
	switch text[0] {
	case '>', '-', '+':
		return `\` + text
	}
	digits := 0
	for digits < len(text) && text[digits] >= '0' && text[digits] <= '9' {
		digits++
	}
	if digits > 0 && digits < len(text) && (text[digits] == '.' || text[digits] == ')') {
		return text[:digits] + `\` + text[digits:]
	}
	return text
}
