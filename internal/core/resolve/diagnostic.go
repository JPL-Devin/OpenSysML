package resolve

import "github.com/Open-MBEE/Systemica/internal/core/source"

// Diagnostic is a name-resolution problem tied to a source span.
type Diagnostic struct {
	Span    source.Span
	Message string
	// Code names the kind of problem; empty leaves it to the message.
	Code string
}

// CodeNameConflict marks a name declared twice in one namespace, counting the
// members it inherits (SysML 7.6.1, KerML 7.3.2.1).
const CodeNameConflict = "name-conflict"
