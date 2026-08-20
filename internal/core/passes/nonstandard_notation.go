package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// CodeNonstandardNotation marks notation OpenSysML accepts that no production of
// the pinned SysML v2 grammars admits.
const CodeNonstandardNotation = "nonstandard-notation"

// CodeKerMLNotation marks KerML notation used in a SysML file, where the SysML
// grammar has no production for it.
const CodeKerMLNotation = "kerml-notation"

// NonstandardNotationPass reports the constructs OpenSysML accepts beyond the
// pinned grammars, classified in docs/reference/grammar/conformance-audit.md.
// They stay parsed — models already use them — so this is a warning at
// LevelSyntax, never an error, and never gates a higher tier.
type NonstandardNotationPass struct{}

// Level reports the syntax level: the written notation is all it reads.
func (NonstandardNotationPass) Level() PassLevel { return LevelSyntax }

// Run walks the document for extension notation and for KerML notation in a
// SysML file.
func (NonstandardNotationPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if root == nil {
		return nil
	}
	// A document of no known kind — the REPL and CLI buffer — reads as SysML,
	// the notation its prompt takes; the REPL drops the finding for a snippet it
	// loaded from a .kerml file.
	w := &notationWalker{sysml: source.KindOf(name) != source.KindKerML}
	w.walk(root.Members)
	return w.diags
}

// notationWalker accumulates the diagnostics of one document.
type notationWalker struct {
	sysml bool
	diags []Diagnostic
}

// walk reports the extension notation in a member list and descends into the
// bodies its members carry.
func (w *notationWalker) walk(members []ast.Node) {
	for _, member := range members {
		switch n := unwrapMembership(member).(type) {
		case *ast.Namespace:
			w.kermlNamespace(n)
			w.walk(n.Members)
		case *ast.Package:
			w.walk(n.Members)
		case *ast.Definition:
			w.walk(n.Members)
		case *ast.Usage:
			w.walk(n.Members)
		case *ast.Import:
			w.walk(n.Body)
		case *ast.StateNode:
			w.stateNode(n)
		case *ast.StateRegion:
			w.extension(keywordSpan(n, "region"), "`region <name> { … }`",
				"the standard orthogonality marker is `parallel` before a state body")
			w.walk(n.States)
		case *ast.PseudostateNode:
			w.pseudostate(n)
		case *ast.DeferMember:
			w.extension(keywordSpan(n, "defer"), "`defer <event>;`",
				"no notation states a deferred event")
		case *ast.InitialNode:
			if n.Keyword == "initial" {
				w.extension(keywordSpan(n, n.Keyword), "`initial` as an action node",
					"the standard spelling of the node is `first`")
			}
		case *ast.FinalNode:
			if n.Keyword == "final" {
				w.extension(keywordSpan(n, n.Keyword), "`final` as an action node",
					"the standard spelling of the node is `done`, the library feature")
			}
		case *ast.DecisionNode:
			if n.Keyword == "decision" {
				w.extension(keywordSpan(n, n.Keyword), "`decision` as an action node",
					"the standard spelling of the node is `decide`")
			}
		case *ast.TransitionMember:
			if n.ToSpan.Len > 0 {
				w.extension(n.ToSpan, "`transition <source> to <target>;`",
					"a transition states its ends with `first` and `then`")
			}
			w.walk(n.Effect)
		case *ast.EntryMember:
			w.walk(n.Actions)
		case *ast.DoMember:
			w.walk(n.Actions)
		case *ast.ExitMember:
			w.walk(n.Actions)
		case *ast.IfActionNode:
			for _, branch := range n.Branches() {
				w.walk(branch.Body)
			}
		case *ast.WhileLoopActionNode:
			w.walk(n.Body)
		}
	}
}

// stateNode reports the `initial`/`final` state markers, then descends.
func (w *notationWalker) stateNode(n *ast.StateNode) {
	switch {
	case n.IsInitial:
		w.extension(keywordSpan(n, "initial"), "`initial <state>;`",
			"the standard way to mark the state a machine starts in is `entry; then <state>;`")
	case n.IsFinal:
		w.extension(keywordSpan(n, "final"), "`final <state>;`",
			"a final state is reached by a transition, and is written `state <name>;`")
	}
	w.walk(n.Entry)
	w.walk(n.Do)
	w.walk(n.Exit)
	w.walk(n.Defer)
	w.walk(n.Substates)
	for _, region := range n.Regions {
		w.walk([]ast.Node{region})
	}
}

// pseudostate reports the pseudostates that are ours. `fork` and `join` are not
// among them: both are action node literals a state body admits.
func (w *notationWalker) pseudostate(n *ast.PseudostateNode) {
	switch n.Kind {
	case ast.PseudostateFork, ast.PseudostateJoin:
		return
	}
	w.extension(keywordSpan(n, n.Keyword), fmt.Sprintf("`%s <name>;`", n.Keyword),
		"the grammars define no pseudostate notation")
}

// kermlNamespace reports a `namespace` declaration in a SysML file, whose root
// production admits package members only.
func (w *notationWalker) kermlNamespace(n *ast.Namespace) {
	if !w.sysml {
		return
	}
	w.diags = append(w.diags, Diagnostic{
		Severity: SeverityWarning,
		Span:     keywordSpan(n, "namespace"),
		Message: "`namespace` is KerML notation: the SysML v2 grammar has no namespace declaration, " +
			"so write `package` here or move the declaration to a .kerml file",
		Code:   CodeKerMLNotation,
		Source: "syntax",
	})
}

// extension reports one construct as an OpenSysML extension.
func (w *notationWalker) extension(span source.Span, construct, standard string) {
	w.diags = append(w.diags, Diagnostic{
		Severity: SeverityWarning,
		Span:     span,
		Message: fmt.Sprintf("%s is an OpenSysML extension with no SysML v2 production: %s",
			construct, standard),
		Code:   CodeNonstandardNotation,
		Source: "syntax",
	})
}

// keywordSpan spans the notation that opens a node, so the diagnostic points at
// the word rather than the whole declaration.
func keywordSpan(n ast.Node, keyword string) source.Span {
	sp := n.Span()
	if sp.Len > len(keyword) {
		sp.Len = len(keyword)
	}
	return sp
}
