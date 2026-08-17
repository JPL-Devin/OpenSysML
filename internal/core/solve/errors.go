package solve

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// ErrNotTranslatable is returned when a condition uses a construct outside the
// translatable subset. A query that cannot encode one conjunct fails as a whole:
// a partial script would answer sat or unsat about conditions it does not hold.
var ErrNotTranslatable = errors.New("not translatable for solving")

// ErrNoConditions is returned for an element that states no condition to
// translate, as evaluating it reports the same.
var ErrNoConditions = errors.New("states no condition")

// NotTranslatableError says which construct refused, why, and where it was
// written. It unwraps to ErrNotTranslatable.
type NotTranslatableError struct {
	// Construct names the construct that refused, as the notation writes it.
	Construct string

	// Reason says why it is outside the subset.
	Reason string

	// Element is the element whose condition was being translated.
	Element string

	// Condition is the condition as written, empty when the refusal is about a
	// declaration rather than a condition.
	Condition string

	// File is the document the construct was written in, empty when unknown.
	File string

	// Span is where in File it was written.
	Span source.Span

	// Location renders File and Span as `file:line:col`, empty when unknown.
	Location string
}

// Error reports the refusal, naming the construct, the condition it appeared in,
// and where it was written.
func (e *NotTranslatableError) Error() string {
	msg := fmt.Sprintf("%s: %s %s", e.Element, e.Construct, ErrNotTranslatable)
	if e.Reason != "" {
		msg += ": " + e.Reason
	}
	if e.Condition != "" {
		msg += fmt.Sprintf(" (in %s)", e.Condition)
	}
	if e.Location != "" {
		msg += " at " + e.Location
	}
	return msg
}

// Unwrap returns ErrNotTranslatable, so a caller tests the kind of failure
// rather than its text.
func (e *NotTranslatableError) Unwrap() error { return ErrNotTranslatable }
