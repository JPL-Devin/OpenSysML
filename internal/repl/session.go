// Package repl implements an interactive SysML v2 read-eval-print loop as a
// thin frontend over model.Workspace (spec §13).
package repl

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

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

// snippet is one accepted submission source, the top-level names it declares,
// and the file it was loaded from, so a finding about it can be reported where
// its reader would look for it. The key is that file itself, identifying it
// across the ways one path can be written.
type snippet struct {
	src   string
	names []string
	// origin is the file this snippet was read from, empty for a submission
	// typed at the prompt, and key identifies that file across the ways its path
	// can be written.
	origin string
	key    string
	// gen is the submission that appended this snippet, which is what scopes a
	// report to the files just loaded rather than the whole buffer.
	gen int
	// prefix is the bytes of earlier comment lines folded into src, which are
	// not part of what was submitted.
	prefix int
	// own locates, within src, the text a merge added to a namespace already in
	// the buffer. It is empty for a snippet that is wholly its submission's.
	own []source.Span
}

// Session accumulates submissions into a single implicit <repl> document.
type Session struct {
	// mu serializes the session's exported entry points, which a frontend may
	// call from more than one goroutine: readline answers Tab from its own input
	// goroutine while the loop is still evaluating the previous line.
	mu sync.Mutex

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
	// fqn is the debugged action's qualified name. Superseding it, or a namespace
	// it lives in, rewrites the graph being run and ends the session.
	fqn      string
	symbol   *symbols.Symbol
	executor *runtime.ActionExecutor
}

// fqnOf returns the debugged action's qualified name, or "" when no session runs.
func (a *actionSession) fqnOf() string {
	if a == nil {
		return ""
	}
	return a.fqn
}

// stateSession holds an active state machine executor debugging session.
type stateSession struct {
	name string
	// fqn is the debugged state machine's qualified name; see actionSession.fqn.
	fqn      string
	symbol   *symbols.Symbol
	executor *runtime.StateExecutor
	// now is the debugger's clock. The executor's own clock only moves when an
	// event is processed, so successive %advance calls accumulate here.
	now float64
}

// fqnOf returns the debugged state machine's qualified name, or "" when no
// session runs.
func (s *stateSession) fqnOf() string {
	if s == nil {
		return ""
	}
	return s.fqn
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
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.list()
}

func (s *Session) list() []string {
	out := make([]string, 0, len(s.snippets))
	for _, sn := range s.snippets {
		out = append(out, sn.src)
	}
	return out
}

// accept accepts src as a submission of its own, from origin when it came from a
// file.
func (s *Session) accept(origin, src string) {
	s.version++
	s.acceptFrom(origin, src)
}

// acceptFrom parses src to compute its declared names, drops any snippet from
// an earlier submission whose names intersect, and appends the new snippet under
// the current submission generation. It does NOT touch the workspace (Submit
// does).
//
// Only earlier submissions are replaced: redeclaring a name supersedes what was
// submitted before it, but two files of one load are both part of the model, so
// a name they share is a conflict for the analysis to report rather than a
// reason to drop one of them.
//
// A submission that declares names takes over the comment lines typed just
// before it. They document what follows, so folding them into the same snippet
// makes a later redeclaration replace the comments along with the declaration
// instead of leaving stale documentation above whatever is current.
//
// A loaded file supersedes only itself and what the prompt said about the same
// names, since several files of one model commonly open the same package.
func (s *Session) acceptFrom(origin, src string) (drops []dropReport) {
	root := parser.New(source.New(docName, []byte(src))).ParseFile()
	names := declaredNames(root)
	text := src
	var (
		comments  string
		mergedOwn []source.Span
	)
	if origin != "" {
		set := nameSet(names)
		key := fileKey(origin)
		top := topLevelMembers(root)
		kept := s.snippets[:0]
		for _, sn := range s.snippets {
			switch {
			case sn.key == key:
				// Re-reading the same file is a refresh the load reports
				// itself, so only what it rewrote is recorded.
				drops = append(drops, dropReport{gone: sn.names})
			case sn.origin == "" && intersects(sn.names, set):
				// The file supersedes what was typed about the same names,
				// which is a loss to report like any other.
				drops = append(drops, replacedReport(sn, set, top))
			default:
				kept = append(kept, sn)
			}
		}
		s.snippets = append(kept, snippet{src: src, names: names, origin: origin, key: key, gen: s.version})
		return drops
	}
	if len(names) > 0 {
		set := nameSet(names)
		comments = s.takeLeadingComments()
		// Re-typing a namespace adds to the one already in the buffer. The
		// merged text stands for both — including any other declaration of the
		// snippet it absorbed, so the names it replaces are its own, not just
		// the submitted ones — and is appended like any other submission so a
		// report still scopes to the tail of the buffer.
		if merged, added, drop, ok := s.mergeSubmission(src, root, comments); ok {
			text, comments, mergedOwn = merged, "", added
			names = declaredNames(parser.New(source.New(docName, []byte(merged))).ParseFile())
			drops = append(drops, drop)
		}
		top := topLevelMembers(root)
		kept := s.snippets[:0]
		for _, sn := range s.snippets {
			if sn.gen == s.version || !intersects(sn.names, set) {
				kept = append(kept, sn)
				continue
			}
			drops = append(drops, replacedReport(sn, set, top))
		}
		s.snippets = kept
	}
	// The prefix marks the comments folded in front of what was submitted, so
	// diagnostics keep the line numbers of the submission. A merge folded the
	// comments into the text it rewrote, where own marks what was added.
	s.snippets = append(s.snippets, snippet{
		src:    comments + text,
		names:  names,
		origin: origin,
		gen:    s.version,
		prefix: len(comments),
		own:    mergedOwn,
	})
	return drops
}

// origins locates every file of the current submission in the joined buffer, in
// buffer order, so a diagnostic can be reported against the file it came from.
func (s *Session) origins() []Origin {
	var out []Origin
	acc := 0
	for _, sn := range s.snippets {
		if sn.gen == s.version && sn.origin != "" {
			out = append(out, Origin{Name: sn.origin, Offset: acc + sn.prefix})
		}
		acc += len(sn.src) + 1 // the newline joined() writes between snippets
	}
	return out
}

// ownSpans locates, in the joined buffer, the text this submission added to a
// namespace a merge folded it into, so its report covers what was typed rather
// than the whole snippet the merge absorbed.
func (s *Session) ownSpans() []source.Span {
	var (
		out    []source.Span
		merged bool
		acc    int
	)
	for _, sn := range s.snippets {
		if sn.gen == s.version {
			if len(sn.own) == 0 {
				// Appended whole, so all of it past the folded comments is this
				// submission's.
				out = append(out, source.Span{Offset: acc + sn.prefix, Len: len(sn.src) - sn.prefix})
			} else {
				merged = true
			}
			for _, sp := range sn.own {
				out = append(out, source.Span{Offset: acc + sp.Offset, Len: sp.Len})
			}
		}
		acc += len(sn.src) + 1 // the newline joined() writes between snippets
	}
	if !merged {
		// Nothing was merged, so the tail-of-buffer rule already describes the
		// submission and needs no spans.
		return nil
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Offset < out[j].Offset })
	return out
}

// firstText returns the offset of the first span with text in it, which is where
// a submission's own text begins; the zero-length spans only mark a declaration
// a merge folded into.
func firstText(spans []source.Span) (int, bool) {
	for _, sp := range spans {
		if sp.Len > 0 {
			return sp.Offset, true
		}
	}
	return 0, false
}

// genOffset returns the byte offset in the joined buffer where the current
// submission begins: the start of its first surviving snippet, past any comment
// lines folded in front of it, so diagnostics keep the line numbers submitted.
func (s *Session) genOffset(joined string) int {
	acc := 0
	for _, sn := range s.snippets {
		if sn.gen == s.version {
			return acc + sn.prefix
		}
		acc += len(sn.src) + 1 // the newline joined() writes between snippets
	}
	return len(joined)
}

// takeLeadingComments removes the trailing run of comment-only snippets typed at
// the prompt and returns their text, ready to prefix the declaration they
// document. A comment-only file is a file in its own right, so it stays.
func (s *Session) takeLeadingComments() string {
	cut := len(s.snippets)
	for cut > 0 && s.snippets[cut-1].origin == "" && isCommentOnly(s.snippets[cut-1].src) {
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

// fileKey identifies the file a path is written for, so the same file loaded
// under another spelling supersedes itself instead of accumulating a copy.
func fileKey(path string) string {
	full := expandHome(path)
	if abs, err := filepath.Abs(full); err == nil {
		full = abs
	}
	if resolved, err := filepath.EvalSymlinks(full); err == nil {
		return resolved
	}
	return filepath.Clean(full)
}

func nameSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
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
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.submitAll([]string{src})
}

// SourceFile is one source of a submission together with the file it was read
// from, which is what diagnostics over a multi-file load are reported against.
type SourceFile struct {
	Name string
	Text string
}

// SubmitAll accumulates every src as one submission, from no file in particular.
func (s *Session) SubmitAll(srcs []string) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.submitAll(srcs)
}

func (s *Session) submitAll(srcs []string) Result {
	files := make([]SourceFile, 0, len(srcs))
	for _, src := range srcs {
		files = append(files, SourceFile{Text: src})
	}
	return s.submitFiles(files)
}

// submit accumulates src as Submit does, recording the file it came from when it
// came from one.
func (s *Session) submit(origin, src string) Result {
	return s.submitFiles([]SourceFile{{Name: origin, Text: src}})
}

// SubmitFiles accumulates every file as one submission: all of them are accepted
// before the buffer is reindexed and analyzed, so a declaration in one resolves
// against the others no matter which order they arrive in. This is what makes
// loading a multi-file project order-independent.
func (s *Session) SubmitFiles(files []SourceFile) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.submitFiles(files)
}

func (s *Session) submitFiles(files []SourceFile) Result {
	var (
		declared []string
		drops    []dropReport
	)
	seen := map[string]bool{}
	s.version++
	for _, f := range files {
		for _, name := range declaredNames(parser.New(source.New(docName, []byte(f.Text))).ParseFile()) {
			if !seen[name] {
				seen[name] = true
				declared = append(declared, name)
			}
		}
		drops = append(drops, s.acceptFrom(f.Name, f.Text)...)
	}
	joined := s.joined()
	offset := s.genOffset(joined)
	// A merge rewrote a snippet that was already accepted, so only the text the
	// merge added belongs to this submission; anything else is the buffer.
	own := s.ownSpans()
	if at, ok := firstText(own); ok {
		offset = at
	}
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
	notices = append(notices, s.dropStaleDebugSessions(drops)...)
	// The diagnostics already carry their own "did you mean" hints.
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
		Origins:     s.origins(),
		own:         own,
		Notices:     notices,
	}
}

// dropStaleDebugSessions ends the debugging sessions whose declaration this
// submission superseded, and reports each one it ended. A session over a
// declaration the submission left alone survives — including one merged into a
// namespace that only gained an unrelated member: it keeps running against the
// graph and runtime context it started with, so stepping through a behavior does
// not require avoiding the prompt.
func (s *Session) dropStaleDebugSessions(drops []dropReport) []string {
	var gone []string
	for _, d := range drops {
		gone = append(gone, d.gone...)
	}
	if len(gone) == 0 {
		return nil
	}
	var notices []string
	if by, ok := supersededBy(gone, s.actionExec.fqnOf()); ok {
		notices = append(notices, debugSessionEnded("action", s.actionExec.name, by))
		s.endedAction = &endedSession{
			kind:     "action",
			name:     s.actionExec.name,
			rootName: by,
			version:  s.version,
		}
		s.actionExec = nil
	}
	if by, ok := supersededBy(gone, s.stateExec.fqnOf()); ok {
		notices = append(notices, debugSessionEnded("state", s.stateExec.name, by))
		s.endedState = &endedSession{
			kind:     "state machine",
			name:     s.stateExec.name,
			rootName: by,
			version:  s.version,
		}
		s.stateExec = nil
	}
	return notices
}

func debugSessionEnded(kind, name, superseded string) string {
	return fmt.Sprintf("note: %s debugging session for %q ended (%s was redeclared)", kind, name, superseded)
}

// supersededBy reports which superseded name rewrote the declaration fqn names,
// counting the namespaces it lives in: replacing a package replaces its members.
func supersededBy(gone []string, fqn string) (string, bool) {
	if fqn == "" {
		return "", false
	}
	for _, g := range gone {
		if fqn == g || strings.HasPrefix(fqn, g+"::") {
			return g, true
		}
	}
	return "", false
}

// Clear resets the session, dropping all accumulated declarations.
func (s *Session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clear()
}

func (s *Session) clear() {
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

	// Falls back to the library index so a library symbol can be evaluated or
	// instantiated before the session declares anything.
	idx := s.browseIndex()
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

// qualifiedOr returns the looked-up qualified name, falling back to the name as
// typed when the lookup reported none.
func qualifiedOr(fqn, typed string) string {
	if fqn != "" {
		return fqn
	}
	return typed
}
