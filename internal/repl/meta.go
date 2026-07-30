package repl

import (
	"fmt"
	"os"
	"strings"
)

// isMeta reports whether a trimmed input line is a meta command.
func isMeta(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "%")
}

var helpText = []string{
	"%help          show this help",
	"%list          list current session declarations",
	"%clear         reset the session",
	"%load <file>   read a file and submit its contents",
}

// runMeta executes a meta command line. Returns lines to print, whether to quit,
// and an error only for unrecoverable I/O (unknown commands print guidance).
func (s *Session) runMeta(line string) (out []string, quit bool, err error) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 {
		return nil, false, nil
	}
	switch fields[0] {
	case "%help":
		return helpText, false, nil
	case "%list":
		decls := s.List()
		if len(decls) == 0 {
			return []string{"(empty session)"}, false, nil
		}
		return decls, false, nil
	case "%clear":
		s.Clear()
		return []string{"session cleared"}, false, nil
	case "%load":
		if len(fields) < 2 {
			return []string{"usage: %load <file>"}, false, nil
		}
		data, rerr := os.ReadFile(fields[1])
		if rerr != nil {
			return nil, false, fmt.Errorf("load %s: %w", fields[1], rerr)
		}
		r := s.Submit(string(data))
		return renderResult(r), false, nil
	default:
		return []string{fmt.Sprintf("unknown command %q (try %%help)", fields[0])}, false, nil
	}
}
