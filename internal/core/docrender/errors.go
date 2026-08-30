package docrender

import "fmt"

// ErrorKind classifies a document-rendering failure.
type ErrorKind string

const (
	ErrorNilDocument    ErrorKind = "nil-document"
	ErrorUnknownContent ErrorKind = "unknown-content"
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
	default:
		return "document rendering failed"
	}
}
