package parser

import (
	"github.com/Open-MBEE/Systemica/internal/core/quickfix"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// Diagnostic is a parser-emitted syntax error. It is unified with the
// pass/validation diagnostic model in Plan 4; kept local here to avoid a
// premature cross-package dependency.
type Diagnostic struct {
	Span    source.Span
	Message string
	// Code classifies a warning for the consumers that report it; errors carry
	// the "syntax" code their reporters give them.
	Code string
	// Fixes are the unambiguous edits resolving the diagnostic, offered by an
	// editor as quick fixes.
	Fixes []quickfix.Fix
}

// Warning codes.
const (
	codeReservedKeywordName = "reserved-keyword-name"
)
