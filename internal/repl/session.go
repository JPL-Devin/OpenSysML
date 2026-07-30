// Package repl implements an interactive SysML v2 read-eval-print loop as a
// thin frontend over model.Workspace (spec §13).
package repl

import "github.com/Open-MBEE/Systemica/internal/core/model"

// docName is the in-memory workspace key for the accumulated REPL buffer.
const docName = "<repl>"

// snippet is one accepted submission source plus the top-level names it declares.
type snippet struct {
	src   string
	names []string
}

// Session accumulates submissions into a single implicit <repl> document.
type Session struct {
	ws       *model.Workspace
	snippets []snippet
	version  int
}

// NewSession returns a session over a fresh workspace.
func NewSession() *Session {
	return &Session{ws: model.NewWorkspace()}
}

// List returns a one-line summary per surviving snippet (placeholder in Task 1).
func (s *Session) List() []string {
	out := make([]string, 0, len(s.snippets))
	for _, sn := range s.snippets {
		out = append(out, sn.src)
	}
	return out
}
