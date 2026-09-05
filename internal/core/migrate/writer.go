package migrate

import "strings"

// writer accumulates indented lines of notation.
type writer struct {
	b      strings.Builder
	indent int
}

func (w *writer) line(s string) {
	if s == "" {
		w.b.WriteByte('\n')
		return
	}
	w.b.WriteString(strings.Repeat("    ", w.indent))
	w.b.WriteString(s)
	w.b.WriteByte('\n')
}

func (w *writer) lines(ls []string) {
	for _, l := range ls {
		w.line(l)
	}
}

// block writes header with a brace-delimited body, or as `header;` when the
// body writes nothing.
func (w *writer) block(header string, body func()) {
	saved := w.b.String()
	w.b.Reset()
	w.indent++
	body()
	inner := w.b.String()
	w.indent--
	w.b.Reset()
	w.b.WriteString(saved)
	if inner == "" {
		w.line(header + ";")
		return
	}
	w.line(header + " {")
	w.b.WriteString(inner)
	w.line("}")
}

func (w *writer) String() string { return w.b.String() }
