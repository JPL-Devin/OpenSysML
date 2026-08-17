package repl

import (
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/runtime"
)

// satisfyText names a satisfaction assertion the way the prompt names every other
// element, quoting each inner name the notation quotes.
func satisfyText(a *runtime.SatisfyAssertion) string {
	if a == nil {
		return ""
	}
	var b strings.Builder
	if a.Negated {
		b.WriteString("not ")
	}
	b.WriteString("satisfy ")
	switch {
	case a.RequirementRef != "":
		b.WriteString(notationName(a.RequirementRef))
	case a.Symbol != nil && a.Symbol.Name != "":
		b.WriteString(notationName(a.Symbol.Name))
	default:
		b.WriteString("?")
	}
	if a.SubjectRef != "" {
		b.WriteString(" by ")
		b.WriteString(notationName(a.SubjectRef))
	}
	return b.String()
}
