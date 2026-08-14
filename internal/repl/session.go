// Package repl implements an interactive SysML v2 read-eval-print loop as a
// thin frontend over model.Workspace (spec §13).
package repl

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
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
	rtCtx      *runtime.Context
	idx        *symbols.Index               // index over the session document, shared by lookup and runtime
	idxVersion int                          // document version idx holds, 0 when it holds none
	instances  map[string]*runtime.Instance // FQN -> instance for %instantiate tracking

	// Active executor sessions for debugging
	actionExec *actionSession
	stateExec  *stateSession

	// Why the debugging sessions and the instances are gone, when a submission
	// ended them. A command that finds nothing active reports this rather than
	// leaving the user to guess.
	endedAction   *endedSession
	endedState    *endedSession
	lostInstances int // how many instances the last submission that dropped any took
	lostAt        int // the submission that dropped them

	// trace records execution steps while tracing is on, nil otherwise.
	trace *runtime.TraceRecorder

	// budgets bounds every runtime context this session creates.
	budgets runtime.Budgets

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
		budgets:   runtime.DefaultBudgets(),
		verbosity: VerbosityNormal,
	}
}

// SetBudgets sets the bounds for runtime contexts created from here on. It
// errors on a non-positive bound, which no run could make progress under.
func (s *Session) SetBudgets(budgets runtime.Budgets) error {
	if err := budgets.Validate(); err != nil {
		return err
	}
	s.budgets = budgets
	// Dropping the context invalidates everything derived from it, instances
	// included: their IDs restart with the next context.
	s.rtCtx = nil
	s.instances = make(map[string]*runtime.Instance)
	return nil
}

// Budgets returns the bounds this session gives its runtime contexts.
func (s *Session) Budgets() runtime.Budgets {
	return s.budgets
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
//
// A submission that declares names takes over the comment lines typed just
// before it. They document what follows, so folding them into the same snippet
// makes a later redeclaration replace the comments along with the declaration
// instead of leaving stale documentation above whatever is current.
func (s *Session) accept(src string) (joined string, offset int, drops []dropReport) {
	root := parser.New(source.New(docName, []byte(src))).ParseFile()
	names := declaredNames(root)
	text := src
	var comments string
	if len(names) > 0 {
		set := make(map[string]bool, len(names))
		for _, n := range names {
			set[n] = true
		}
		// Re-typing a namespace adds to the one already in the buffer. The
		// merged text stands for both, and is appended like any other
		// submission so a report still scopes to the tail of the buffer. The
		// comments above the addition document its members, which the merged
		// text already carries, so they are left where they are.
		if merged, drop, ok := s.mergeSubmission(src, root); ok {
			text = merged
			drops = append(drops, drop)
		} else {
			comments = s.takeLeadingComments()
		}
		top := topLevelMembers(root)
		kept := s.snippets[:0]
		for _, sn := range s.snippets {
			if !intersects(sn.names, set) {
				kept = append(kept, sn)
				continue
			}
			drops = append(drops, replacedReport(sn, set, src, top))
		}
		s.snippets = kept
	}
	s.snippets = append(s.snippets, snippet{src: comments + text, names: names})
	joined = s.joined()
	// The offset marks what the user typed, not the comments folded in front of
	// it, so diagnostics keep the line numbers of the submission.
	return joined, len(joined) - len(text), drops
}

// takeLeadingComments removes the trailing run of comment-only snippets and
// returns their text, ready to prefix the declaration they document.
func (s *Session) takeLeadingComments() string {
	cut := len(s.snippets)
	for cut > 0 && isCommentOnly(s.snippets[cut-1].src) {
		cut--
	}
	if cut == len(s.snippets) {
		return ""
	}
	var b strings.Builder
	for _, sn := range s.snippets[cut:] {
		b.WriteString(sn.src)
		b.WriteString("\n")
	}
	s.snippets = s.snippets[:cut]
	return b.String()
}

// isCommentOnly reports whether a submission is nothing but comments, and so
// declares nothing of its own to be replaced by name.
func isCommentOnly(src string) bool {
	lx := lexer.New(source.New(docName, []byte(src)))
	comments := 0
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		switch tok.Kind {
		case lexer.Whitespace:
		case lexer.SLNote, lexer.MLNote, lexer.RegularComment:
			comments++
		default:
			return false
		}
	}
	return comments > 0
}

// sessionOrigin names the accumulated session buffer in diagnostics, which
// belongs to no file on disk.
const sessionOrigin = "<session>"

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
	joined, offset, drops := s.accept(src)
	s.version++
	s.ws.Open(docName, []byte(joined), s.version)
	// The document is a new AST and scope tree, so anything derived from the
	// previous one is stale — including instances, whose IDs restart with the
	// new runtime context. The index is re-used and brought up to date on the
	// next lookup instead, which is why it records the version it holds.
	s.rtCtx = nil
	notices := dropNotices(drops)
	if n := len(s.instances); n > 0 {
		s.instances = make(map[string]*runtime.Instance)
		s.lostInstances, s.lostAt = n, s.version
		notices = append(notices, instancesDroppedNotice(n))
	}
	notices = append(notices, s.dropStaleDebugSessions(declared)...)
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
		s.endedAction = &endedSession{
			kind:     "action",
			name:     s.actionExec.name,
			rootName: s.actionExec.rootName,
			version:  s.version,
		}
		s.actionExec = nil
	}
	if s.stateExec != nil && redeclared[s.stateExec.rootName] {
		notices = append(notices, debugSessionEnded("state", s.stateExec.name, s.stateExec.rootName))
		s.endedState = &endedSession{
			kind:     "state machine",
			name:     s.stateExec.name,
			rootName: s.stateExec.rootName,
			version:  s.version,
		}
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
	if s.idx != nil {
		// Drop the document, keep the library the index was built with.
		s.idx.RemoveDocument(docName)
		s.idxVersion = 0
	}
	s.instances = make(map[string]*runtime.Instance)
	s.actionExec = nil
	s.stateExec = nil
	s.endedAction = nil
	s.endedState = nil
	s.lostInstances, s.lostAt = 0, 0
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
	ctx := runtime.NewContext(model, resolver, s.budgets.MaxSteps)
	if err := ctx.SetBudgets(s.budgets); err != nil {
		return nil, err
	}
	// Give the runtime the buffer's text, so an error about a declaration reports
	// the line it was submitted on rather than a byte offset.
	if doc := s.ws.Document(docName); doc != nil {
		ctx.RegisterSource(source.New(docName, doc.Content))
	}
	s.rtCtx = ctx
	s.rtCtx.SetTrace(s.trace)
	return s.rtCtx, nil
}

// symbolIndex indexes the session document, returning nil when nothing is
// loaded. Name lookup and the runtime context share it, and it carries the
// standard library too, which the runtime resolves names against — the
// measurement unit of a quantity expression is one.
//
// One index serves the whole session: the library is loaded into it once, and
// re-indexing the document takes back the names the previous submission declared
// and the ones its wildcard imports surfaced, so a submission costs its own
// document rather than a reload of the library.
func (s *Session) symbolIndex() *symbols.Index {
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return nil
	}
	if s.idx == nil {
		s.idx = symbols.NewIndex()
		model.LoadStdlibInto(s.idx)
	} else if s.idxVersion == doc.Version {
		return s.idx
	}
	s.idx.AddDocument(docName, doc.AST)
	s.idx.ExpandWildcardImports()
	s.idxVersion = doc.Version
	return s.idx
}
