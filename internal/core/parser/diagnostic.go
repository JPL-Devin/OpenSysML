package parser

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/quickfix"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
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

// Recurring diagnostic messages, spelled once so every site that reports the
// same syntax error words it the same way.
const (
	msgExpectedActionBrace  = "expected '}' after action expression"
	msgExpectedReturnSemi   = "expected ';' after return parameter"
	msgExpectedBodyClose    = "expected '}' to close body"
	msgExpectedBraceOrSemi  = "expected '{' or ';'"
	msgExpectedBodyMember   = "expected a body member"
	msgExpectedCloseParen   = "expected ')'"
	msgExpectedShortName    = "expected short name after '<'"
	msgExpectedCloseAngle   = "expected '>'"
	msgExpectedLocaleString = "expected locale string"
)

// Warning codes.
const (
	codeReservedKeywordName = "reserved-keyword-name"
	// codeAmbiguousModifierKind marks `<modifier> <kind>` with no name after it.
	codeAmbiguousModifierKind = "ambiguous-modifier-kind"
)
