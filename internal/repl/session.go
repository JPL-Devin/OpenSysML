// Package repl implements an interactive SysML v2 read-eval-print loop as a
// thin frontend over model.Workspace (spec §13).
package repl

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/model"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/runtime"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

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

	// Runtime execution context
	rtCtx     *runtime.Context
	idx       *symbols.Index               // index over the session document, shared by lookup and runtime
	instances map[string]*runtime.Instance // FQN -> instance for %instantiate tracking

	// Active executor sessions for debugging
	actionExec *actionSession
	stateExec  *stateSession

	verbosity Verbosity
}

// actionSession holds an active action executor debugging session.
type actionSession struct {
	name string
	// rootName is the top-level declaration the debugged action lives under.
	// Resubmitting that declaration rewrites the graph the executor is running,
	// which is what ends the session; an unrelated submission does not.
	rootName string
	symbol   *symbols.Symbol
	executor *runtime.ActionExecutor
}

// stateSession holds an active state machine executor debugging session.
type stateSession struct {
	name string
	// rootName is the top-level declaration the debugged state machine lives
	// under; see actionSession.rootName.
	rootName string
	symbol   *symbols.Symbol
	executor *runtime.StateExecutor
	// now is the debugger's clock. The executor's own clock only moves when an
	// event is processed, so successive %advance calls accumulate here.
	now float64
}

// NewSession returns a session over a fresh workspace.
func NewSession() *Session {
	return &Session{
		ws:        model.NewWorkspace(),
		instances: make(map[string]*runtime.Instance),
		verbosity: VerbosityNormal,
	}
}

// List returns a one-line summary per surviving snippet.
func (s *Session) List() []string {
	out := make([]string, 0, len(s.snippets))
	for _, sn := range s.snippets {
		out = append(out, sn.src)
	}
	return out
}

// accept parses src to compute its declared names, drops any earlier snippet
// whose names intersect, appends the new snippet, and returns the joined
// <repl> content plus the byte offset where src begins in it. That offset is
// what scopes a report to the submission just made. It does NOT touch the
// workspace (Task 4 does).
func (s *Session) accept(src string) (joined string, offset int) {
	root := parser.New(source.New(docName, []byte(src))).ParseFile()
	names := declaredNames(root)
	if len(names) > 0 {
		set := make(map[string]bool, len(names))
		for _, n := range names {
			set[n] = true
		}
		kept := s.snippets[:0]
		for _, sn := range s.snippets {
			if !intersects(sn.names, set) {
				kept = append(kept, sn)
			}
		}
		s.snippets = kept
	}
	s.snippets = append(s.snippets, snippet{src: src, names: names})
	joined = s.joined()
	return joined, len(joined) - len(src)
}

func (s *Session) joined() string {
	parts := make([]string, len(s.snippets))
	for i, sn := range s.snippets {
		parts[i] = sn.src
	}
	return strings.Join(parts, "\n")
}

func intersects(names []string, set map[string]bool) bool {
	for _, n := range names {
		if set[n] {
			return true
		}
	}
	return false
}

// Submit accumulates src into the <repl> document, reindexes and eagerly
// analyzes the whole buffer, and returns a Result. Submissions are always
// accumulated (even with parse errors) so diagnostics are reported against the
// live session context; a later redeclaration of the same name replaces the
// prior snippet (see accept).
func (s *Session) Submit(src string) Result {
	declared := declaredNames(parser.New(source.New(docName, []byte(src))).ParseFile())
	joined, offset := s.accept(src)
	s.version++
	s.ws.Open(docName, []byte(joined), s.version)
	// The document is a new AST and scope tree, so anything derived from the
	// previous one is stale — including instances, whose IDs restart with the
	// new runtime context.
	s.idx = nil
	s.rtCtx = nil
	s.instances = make(map[string]*runtime.Instance)
	notices := s.dropStaleDebugSessions(declared)
	diags := s.ws.Diagnostics(docName)
	var members []ast.Node
	if doc := s.ws.Document(docName); doc != nil && doc.AST != nil {
		members = doc.AST.Members
	}
	return Result{
		Members:     members,
		Declared:    declared,
		Diagnostics: diags,
		Source:      joined,
		Offset:      offset,
		Notices:     notices,
	}
}

// dropStaleDebugSessions ends the debugging sessions whose declaration this
// submission rewrote, and reports each one it ended. A session over an
// untouched declaration survives: it keeps running against the graph and
// runtime context it started with, so stepping through a behavior does not
// require avoiding the prompt.
func (s *Session) dropStaleDebugSessions(declared []string) []string {
	if len(declared) == 0 {
		return nil
	}
	redeclared := make(map[string]bool, len(declared))
	for _, n := range declared {
		redeclared[n] = true
	}
	var notices []string
	if s.actionExec != nil && redeclared[s.actionExec.rootName] {
		notices = append(notices, debugSessionEnded("action", s.actionExec.name, s.actionExec.rootName))
		s.actionExec = nil
	}
	if s.stateExec != nil && redeclared[s.stateExec.rootName] {
		notices = append(notices, debugSessionEnded("state", s.stateExec.name, s.stateExec.rootName))
		s.stateExec = nil
	}
	return notices
}

func debugSessionEnded(kind, name, rootName string) string {
	return fmt.Sprintf("note: %s debugging session for %q ended (%s was redeclared)", kind, name, rootName)
}

// rootNameOf returns the top-level declaration name a fully-qualified name
// lives under, which is the granularity at which a submission replaces
// declarations.
func rootNameOf(fqn, fallback string) string {
	if fqn == "" {
		fqn = fallback
	}
	if cut := strings.Index(fqn, "::"); cut >= 0 {
		return fqn[:cut]
	}
	return fqn
}

// Clear resets the session, dropping all accumulated declarations.
func (s *Session) Clear() {
	s.ws.Remove(docName)
	s.snippets = nil
	s.version = 0
	s.rtCtx = nil
	s.idx = nil
	s.instances = make(map[string]*runtime.Instance)
	s.actionExec = nil
	s.stateExec = nil
}

// getOrCreateRuntime lazily creates runtime context when first needed.
func (s *Session) getOrCreateRuntime() (*runtime.Context, error) {
	if s.rtCtx != nil {
		return s.rtCtx, nil
	}

	idx := s.symbolIndex()
	if idx == nil {
		return nil, fmt.Errorf("no document loaded")
	}

	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	s.rtCtx = runtime.NewContext(model, resolver, 100000)
	return s.rtCtx, nil
}

// symbolIndex lazily indexes the session document, returning nil when nothing
// is loaded. Name lookup and the runtime context share it.
func (s *Session) symbolIndex() *symbols.Index {
	if s.idx != nil {
		return s.idx
	}
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return nil
	}
	idx := symbols.NewIndex()
	idx.AddDocument(docName, doc.AST)
	s.idx = idx
	return idx
}
