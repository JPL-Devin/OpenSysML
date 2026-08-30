package docplan

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/provenance"
)

// ErrorKind classifies a document-planning failure.
type ErrorKind string

const (
	ErrorInvalidContext        ErrorKind = "invalid-context"
	ErrorLibraryUnavailable    ErrorKind = "library-unavailable"
	ErrorNotDocumentDefinition ErrorKind = "not-document-definition"
	ErrorMissingTitle          ErrorKind = "missing-title"
	ErrorInvalidAttribute      ErrorKind = "invalid-attribute"
	ErrorInvalidContent        ErrorKind = "invalid-content"
	ErrorNestedDocument        ErrorKind = "nested-document"
	ErrorMissingText           ErrorKind = "missing-text"
	ErrorConflictingText       ErrorKind = "conflicting-text"
	ErrorMissingQuery          ErrorKind = "missing-query"
	ErrorConflictingQuery      ErrorKind = "conflicting-query"
	ErrorUnknownQuery          ErrorKind = "unknown-query"
	ErrorQueryPlanning         ErrorKind = "query-planning"
	ErrorInvalidStyle          ErrorKind = "invalid-style"
	ErrorUnknownParameter      ErrorKind = "unknown-parameter"
	ErrorDuplicateBinding      ErrorKind = "duplicate-binding"
	ErrorMissingBinding        ErrorKind = "missing-binding"
	ErrorDefaultUnavailable    ErrorKind = "default-unavailable"
	ErrorUnsupportedBinding    ErrorKind = "unsupported-binding"
	ErrorBindingType           ErrorKind = "binding-type"
	ErrorBindingMultiplicity   ErrorKind = "binding-multiplicity"
)

// Error is a typed document-planning failure with its source location.
type Error struct {
	Kind      ErrorKind
	Document  string
	Content   string
	Query     string
	Parameter string
	Expected  string
	Actual    string
	Origin    provenance.Origin
	Err       error
}

func (e *Error) Error() string {
	switch e.Kind {
	case ErrorInvalidContext:
		return "document planning requires an index, resolver, and semantic model"
	case ErrorLibraryUnavailable:
		return "document query library is unavailable"
	case ErrorNotDocumentDefinition:
		return fmt.Sprintf("%s is not a document definition", e.Document)
	case ErrorMissingTitle:
		return fmt.Sprintf("document %s content %s has no title", e.Document, e.Content)
	case ErrorInvalidAttribute:
		return fmt.Sprintf(
			"document %s content %s attribute %s must be one string literal",
			e.Document,
			e.Content,
			e.Parameter,
		)
	case ErrorInvalidContent:
		return fmt.Sprintf("document %s contains unsupported content %s", e.Document, e.Content)
	case ErrorNestedDocument:
		return fmt.Sprintf("document %s nests document %s", e.Document, e.Content)
	case ErrorMissingText:
		return fmt.Sprintf("document %s paragraph %s has neither text nor a query", e.Document, e.Content)
	case ErrorConflictingText:
		return fmt.Sprintf("document %s paragraph %s has both text and a query", e.Document, e.Content)
	case ErrorMissingQuery:
		return fmt.Sprintf("document %s content %s references no query", e.Document, e.Content)
	case ErrorConflictingQuery:
		return fmt.Sprintf("document %s content %s references more than one query", e.Document, e.Content)
	case ErrorUnknownQuery:
		return fmt.Sprintf("document %s content %s references unknown query %s", e.Document, e.Content, e.Query)
	case ErrorQueryPlanning:
		return fmt.Sprintf("document %s content %s query %s: %v", e.Document, e.Content, e.Query, e.Err)
	case ErrorInvalidStyle:
		return fmt.Sprintf(
			"document %s list %s style must be %q or %q, got %q",
			e.Document,
			e.Content,
			ListBullet,
			ListNumber,
			e.Actual,
		)
	case ErrorUnknownParameter:
		return fmt.Sprintf("document %s content %s binds unknown parameter %s of %s", e.Document, e.Content, e.Parameter, e.Query)
	case ErrorDuplicateBinding:
		return fmt.Sprintf("document %s content %s binds parameter %s of %s more than once", e.Document, e.Content, e.Parameter, e.Query)
	case ErrorMissingBinding:
		return fmt.Sprintf("document %s content %s does not bind required parameter %s of %s", e.Document, e.Content, e.Parameter, e.Query)
	case ErrorDefaultUnavailable:
		return fmt.Sprintf(
			"document %s content %s must bind parameter %s of %s: default expressions are not evaluated",
			e.Document,
			e.Content,
			e.Parameter,
			e.Query,
		)
	case ErrorUnsupportedBinding:
		return fmt.Sprintf("document %s content %s binds parameter %s of %s with an unsupported expression", e.Document, e.Content, e.Parameter, e.Query)
	case ErrorBindingType:
		return fmt.Sprintf(
			"document %s content %s binds parameter %s of %s with type %s, expected %s",
			e.Document,
			e.Content,
			e.Parameter,
			e.Query,
			e.Actual,
			e.Expected,
		)
	case ErrorBindingMultiplicity:
		return fmt.Sprintf(
			"document %s content %s binds parameter %s of %s with multiplicity %s, expected %s",
			e.Document,
			e.Content,
			e.Parameter,
			e.Query,
			e.Actual,
			e.Expected,
		)
	default:
		return fmt.Sprintf("document planning failed for %s", e.Document)
	}
}

// Unwrap returns the underlying query-planning failure, if any.
func (e *Error) Unwrap() error { return e.Err }
