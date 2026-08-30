package docir

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/provenance"
)

// ErrorKind classifies a document-evaluation failure.
type ErrorKind string

const (
	ErrorInvalidContext ErrorKind = "invalid-context"
	ErrorInvalidPlan    ErrorKind = "invalid-plan"
	ErrorQueryExecution ErrorKind = "query-execution"
)

// Error is a typed document-evaluation failure with its source location.
type Error struct {
	Kind     ErrorKind
	Document string
	Content  string
	Query    string
	Origin   provenance.Origin
	Err      error
}

func (e *Error) Error() string {
	switch e.Kind {
	case ErrorInvalidContext:
		return "document evaluation requires an index, resolver, and semantic model"
	case ErrorInvalidPlan:
		return "document evaluation requires a compiled document plan"
	case ErrorQueryExecution:
		return fmt.Sprintf("document %s content %s query %s: %v", e.Document, e.Content, e.Query, e.Err)
	default:
		return fmt.Sprintf("document evaluation failed for %s", e.Document)
	}
}

// Unwrap returns the underlying query-execution failure, if any.
func (e *Error) Unwrap() error { return e.Err }
