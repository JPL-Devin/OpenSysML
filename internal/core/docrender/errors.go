package docrender

import "fmt"

// ErrorKind classifies a document-rendering failure.
type ErrorKind string

const (
	ErrorNilDocument         ErrorKind = "nil-document"
	ErrorUnknownContent      ErrorKind = "unknown-content"
	ErrorMissingRendering    ErrorKind = "missing-rendering"
	ErrorUnrenderableDiagram ErrorKind = "unrenderable-diagram"
)

// Error is a typed document-rendering failure.
type Error struct {
	Kind    ErrorKind
	Content string
	Actual  string
}

func (e *Error) Error() string {
	switch e.Kind {
	case ErrorNilDocument:
		return "rendering requires an evaluated document"
	case ErrorUnknownContent:
		return fmt.Sprintf("content %s has unknown kind %q", e.Content, e.Actual)
	case ErrorMissingRendering:
		return fmt.Sprintf("diagram %s carries no view rendering", e.Content)
	case ErrorUnrenderableDiagram:
		return fmt.Sprintf("diagram %s has kind %q, which Markdown cannot draw", e.Content, e.Actual)
	default:
		return "document rendering failed"
	}
}
