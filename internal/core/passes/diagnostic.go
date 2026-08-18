package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/quickfix"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// Severity classifies the impact of a Diagnostic.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInfo
	SeverityHint
)

var severityNames = map[Severity]string{
	SeverityError:   "error",
	SeverityWarning: "warning",
	SeverityInfo:    "info",
	SeverityHint:    "hint",
}

// String returns the lowercase name of the severity, or "unknown".
func (s Severity) String() string {
	if name, ok := severityNames[s]; ok {
		return name
	}
	return "unknown"
}

// Diagnostic is a single validation finding produced by a pass.
type Diagnostic struct {
	Severity Severity
	Span     source.Span
	Message  string
	Code     string // stable identifier for filtering / quick-fix selection
	Source   string // the pass ID that produced this diagnostic
	// Fixes are the unambiguous edits resolving the diagnostic, offered by an
	// editor as quick fixes.
	Fixes []quickfix.Fix
}
