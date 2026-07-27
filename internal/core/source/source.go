// Package source owns file content and byte-offset span types.
package source

// Span is a byte range within a SourceFile: [Offset, Offset+Len).
type Span struct {
	Offset int
	Len    int
}

// End returns the exclusive end offset.
func (s Span) End() int { return s.Offset + s.Len }

// Pos is a 1-based line/column location.
type Pos struct {
	Line int
	Col  int
}

// SourceFile owns the raw bytes of one source file.
type SourceFile struct {
	name    string
	content []byte
}

// New creates a SourceFile from a name and its raw bytes.
func New(name string, content []byte) *SourceFile {
	return &SourceFile{name: name, content: content}
}

// Name returns the file name.
func (sf *SourceFile) Name() string { return sf.name }

// Len returns the byte length of the content.
func (sf *SourceFile) Len() int { return len(sf.content) }

// Bytes returns the raw content (do not mutate).
func (sf *SourceFile) Bytes() []byte { return sf.content }

// Text returns the substring covered by the span.
func (sf *SourceFile) Text(sp Span) string {
	return string(sf.content[sp.Offset:sp.End()])
}
