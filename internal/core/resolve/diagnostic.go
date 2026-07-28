package resolve

import "github.com/Open-MBEE/Systemica/internal/core/source"

// Diagnostic is a name-resolution problem tied to a source span.
type Diagnostic struct {
	Span    source.Span
	Message string
}
