package repl

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
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
		if a.SubjectChain != nil {
			b.WriteString(chainNotation(a.SubjectRef))
		} else {
			b.WriteString(notationName(a.SubjectRef))
		}
	}
	return b.String()
}

// chainNotation spells a chained `by` operand as the notation writes it: each
// feature quoted on its own, joined by the dots as written.
func chainNotation(ref string) string {
	segments := strings.Split(ref, ".")
	for i, seg := range segments {
		segments[i] = notationName(seg)
	}
	return strings.Join(segments, ".")
}
