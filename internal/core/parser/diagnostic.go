package parser

import "github.com/Open-MBEE/Systemica/internal/core/source"

// Diagnostic is a parser-emitted syntax error. It is unified with the
// pass/validation diagnostic model in Plan 4; kept local here to avoid a
// premature cross-package dependency.
type Diagnostic struct {
	Span    source.Span
	Message string
}
