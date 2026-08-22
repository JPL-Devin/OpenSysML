// Package conformance names how strictly a document is judged against the
// pinned SysML v2 and KerML grammars. It is a value threaded explicitly through
// the analysis pipeline — no global, no environment read — so every entry point
// states the question it is asking.
package conformance

import "fmt"

// Mode is the strictness a parse/analysis run is asked for.
type Mode int

const (
	// ModeDefault accepts the OpenSysML notation extensions, reporting each as a
	// warning. It is the zero value: a caller that says nothing gets it.
	ModeDefault Mode = iota
	// ModeStrict answers "is this conforming SysML v2?": notation no pinned
	// production admits is an error, so a file using it is rejected.
	ModeStrict
)

// String returns the mode's spelling, the same one every entry point accepts.
func (m Mode) String() string {
	switch m {
	case ModeStrict:
		return "strict"
	case ModeDefault:
		return "default"
	default:
		return fmt.Sprintf("Mode(%d)", int(m))
	}
}

// IsStrict reports whether extension notation is an error in this mode.
func (m Mode) IsStrict() bool { return m == ModeStrict }

// ModeOf maps a boolean surface (a CLI flag, an LSP setting, a request field)
// onto the mode it selects.
func ModeOf(strict bool) Mode {
	if strict {
		return ModeStrict
	}
	return ModeDefault
}

// ParseMode reads a mode by name, as the CLI and the REPL spell it. The empty
// string is the default mode, so an unset setting is not an error.
func ParseMode(s string) (Mode, error) {
	switch s {
	case "", "default":
		return ModeDefault, nil
	case "strict":
		return ModeStrict, nil
	default:
		return ModeDefault, fmt.Errorf("unknown conformance mode %q: want \"default\" or \"strict\"", s)
	}
}
