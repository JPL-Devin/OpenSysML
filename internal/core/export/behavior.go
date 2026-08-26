package export

// The behavioral members an action or state body declares: control nodes,
// statements, loops, conditionals, states, regions and transitions. Each one is
// mapped to a metaclass and to the properties its notation is rebuilt from, so a
// model that states behavior converts rather than being refused. Guards and
// conditions are expression graphs, as every expression-valued position is.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// Metaclass names for the behavioral nodes the SysML metamodel has no
// counterpart for, typed in the OpenSysML namespace.
const (
	mInitialNode     = "InitialNode"
	mFinalNode       = "FinalNode"
	mActionExecution = "ActionExecutionNode"
	mIfBranch        = "IfBranch"
	mStateRegion     = "StateRegion"
	mPseudostate     = "Pseudostate"
	mDeferMember     = "DeferMember"
)

// The OMG metaclasses of the behavioral nodes that have one.
const (
	mFork       = "ForkNode"
	mJoin       = "JoinNode"
	mMerge      = "MergeNode"
	mDecision   = "DecisionNode"
	mPerform    = "PerformActionUsage"
	mAssignment = "AssignmentActionUsage"
	mSend       = "SendActionUsage"
	mTerminate  = "TerminateActionUsage"
	mWhileLoop  = "WhileLoopActionUsage"
	mForLoop    = "ForLoopActionUsage"
	mIfAction   = "IfActionUsage"
	mSubaction  = "StateSubactionMembership"
	mSuccession = "SuccessionAsUsage"
	mTransition = "TransitionUsage"
	mStateUsage = "StateUsage"
)

// Property names for the parts of a behavioral node that the SysML vocabulary
// has no predicate for; see docs/reference/rdf-mapping.md § Behavior.
const (
	xGuard            = "guard"
	xExpression       = "expression"
	xIsElse           = "isElse"
	xWhileCondition   = "whileCondition"
	xUntilCondition   = "untilCondition"
	xLoopVariable     = "loopVariable"
	xCollection       = "collection"
	xBranchKind       = "branchKind"
	xSubactionKind    = "subactionKind"
	xPseudostateKind  = "pseudostateKind"
	xTransitionSyntax = "transitionSyntax"
	xTrigger          = "trigger"
	xTriggerKeyword   = "triggerKeyword"
	xDeferredEvent    = "deferredEvent"
	xAssignOperator   = "assignmentOperator"
	xPayload          = "payload"
	xReceiver         = "receiver"
	xIsVia            = "isVia"
	xTarget           = "target"
)

// encodeBehavior emits the triples of a behavioral node, reporting whether the
// node was one. head writes the properties every member carries.
func (e *encoder) encodeBehavior(node ast.Node, head func(rdf.Term), subject rdf.Term, fqn, owner string, index int) (bool, error) {
	switch n := node.(type) {
	case *ast.InitialNode:
		head(rdf.OpenSysMLTerm(mInitialNode))
		// `first x` names the node the body starts at, so the name is a reference
		// to a member rather than one this element declares.
		if n.Name != "" {
			e.graph.Add(subject, e.sysml(pSourceFeature), e.reference(owner, n.Name))
		}
		e.expression(subject, e.sysx(xGuard), xGuard, owner, n.Guard)
		if successor := qualifiedText(n.Successor); successor != "" {
			e.graph.Add(subject, e.sysml(pTargetFeature), e.reference(owner, successor))
		} else if n.Guard != nil {
			return true, &UnsupportedError{
				What: fmt.Sprintf("the guarded initial node at %s", e.where(n)),
				Note: "it names no successor, so the branch its guard states cannot be written back",
			}
		}
		return true, nil

	case *ast.FinalNode:
		head(rdf.OpenSysMLTerm(mFinalNode))
		e.writtenKeyword(subject, n, "done", "final")
		return true, nil

	case *ast.ForkNode:
		head(rdf.SysMLTerm(mFork))
		e.name(subject, n.Name)
		return true, nil

	case *ast.JoinNode:
		head(rdf.SysMLTerm(mJoin))
		e.name(subject, n.Name)
		return true, nil

	case *ast.MergeNode:
		head(rdf.SysMLTerm(mMerge))
		e.name(subject, n.Name)
		return true, nil

	case *ast.DecisionNode:
		head(rdf.SysMLTerm(mDecision))
		e.name(subject, n.Name)
		return true, nil

	case *ast.ActionExecutionNode:
		// `action [<name>] <ref>;` performs an action declared elsewhere;
		// `action <name> { <expr> }` performs the expression it states.
		head(rdf.OpenSysMLTerm(mActionExecution))
		e.name(subject, n.Name)
		switch {
		case n.Expression != nil:
			e.expression(subject, e.sysx(xExpression), xExpression, owner, n.Expression)
		case qualifiedText(n.ActionRef) != "":
			e.graph.Add(subject, e.sysml(relationshipProperty[ast.RelReferences]),
				e.reference(owner, qualifiedText(n.ActionRef)))
		default:
			return true, &UnsupportedError{
				What: fmt.Sprintf("the action node at %s", e.where(n)),
				Note: "it states neither an action to perform nor an expression to evaluate",
			}
		}
		return true, nil

	case *ast.PerformActionNode:
		head(rdf.SysMLTerm(mPerform))
		e.expression(subject, e.sysx(xExpression), xExpression, owner, n.ActionRef)
		return true, nil

	case *ast.AssignmentActionNode:
		head(rdf.SysMLTerm(mAssignment))
		e.writtenKeyword(subject, n, "", "assign")
		if operator := e.between(n.Target, n.Value); operator != "" && operator != ":=" {
			e.graph.Add(subject, e.sysx(xAssignOperator), rdf.String(operator))
		}
		e.expression(subject, e.sysx(xTarget), xTarget, owner, n.Target)
		e.expression(subject, e.sysml(pValue), pValue, owner, n.Value)
		return true, nil

	case *ast.SendStatement:
		head(rdf.SysMLTerm(mSend))
		e.expression(subject, e.sysx(xPayload), xPayload, owner, n.Message)
		e.expression(subject, e.sysx(xReceiver), xReceiver, owner, n.Target)
		if n.IsVia {
			e.graph.Add(subject, e.sysx(xIsVia), rdf.Bool(true))
		}
		return true, nil

	case *ast.TerminateStatement:
		head(rdf.SysMLTerm(mTerminate))
		e.expression(subject, e.sysx(xExpression), xExpression, owner, n.Target)
		return true, nil

	case *ast.SuccessionEdge:
		head(rdf.SysMLTerm(mSuccession))
		implied := impliedSource(n, n.Source)
		if implied {
			// `then b;` states no source end: the notation sequences from the
			// member written before it, which the form records.
			e.graph.Add(subject, e.sysx(xEndForm), rdf.String(formThen))
		}
		return true, e.edgeEnds(subject, n, owner,
			edgeEnd{name: n.Source, member: n.SourceMember, implied: implied},
			edgeEnd{name: n.Target, member: n.TargetMember})

	case *ast.ControlFlowEdge:
		// A guarded branch of a decision, or the `else` branch taken when no
		// guarded one is. Which keyword introduced it decides how it is written.
		head(rdf.SysMLTerm(mSuccession))
		e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(firstWord(e.text(n))))
		e.expression(subject, e.sysx(xGuard), xGuard, owner, n.Guard)
		if n.IsElse {
			e.graph.Add(subject, e.sysx(xIsElse), rdf.Bool(true))
		}
		return true, e.edgeEnds(subject, n, owner,
			edgeEnd{name: n.Source, member: n.SourceMember, implied: impliedSource(n, n.Source)},
			edgeEnd{name: n.Target, member: n.TargetMember})

	case *ast.WhileLoopActionNode:
		return true, e.encodeLoop(n, head, subject, fqn, owner)

	case *ast.IfActionNode:
		head(rdf.SysMLTerm(mIfAction))
		e.expression(subject, e.sysx(xCondition), xCondition, owner, n.Condition)
		branches := make([]ast.Node, 0, 2)
		for _, branch := range n.Branches() {
			branches = append(branches, branch)
		}
		return true, e.encode(branches, fqn, subject)

	case *ast.IfBranchNode:
		head(rdf.OpenSysMLTerm(mIfBranch))
		e.graph.Add(subject, e.sysx(xBranchKind), rdf.String(n.Kind.String()))
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(e.bracedBranch(n)))
		return true, e.encode(n.Body, fqn, subject)

	case *ast.StateNode:
		// A state of a state machine, including the `initial <name>;` and
		// `final <name>;` markers, whose keyword says which of the two it is.
		head(rdf.SysMLTerm(mStateUsage))
		e.name(subject, n.Name)
		switch {
		case n.IsInitial:
			e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String("initial"))
		case n.IsFinal:
			e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String("final"))
		}
		members := stateBody(n)
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(len(members) > 0))
		return true, e.encode(members, fqn, subject)

	case *ast.SubstateMember:
		head(rdf.SysMLTerm(mStateUsage))
		e.name(subject, n.Name)
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(false))
		return true, nil

	case *ast.StateRegion:
		head(rdf.OpenSysMLTerm(mStateRegion))
		e.name(subject, n.Name)
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(true))
		return true, e.encode(n.States, fqn, subject)

	case *ast.EntryMember:
		return true, e.encodeSubaction(n, n.Actions, "entry", head, subject, fqn)

	case *ast.DoMember:
		return true, e.encodeSubaction(n, n.Actions, "do", head, subject, fqn)

	case *ast.ExitMember:
		return true, e.encodeSubaction(n, n.Actions, "exit", head, subject, fqn)

	case *ast.DeferMember:
		head(rdf.OpenSysMLTerm(mDeferMember))
		for _, trigger := range n.Triggers {
			e.graph.Add(subject, e.sysx(xDeferredEvent), rdf.String(e.text(trigger)))
		}
		return true, nil

	case *ast.PseudostateNode:
		head(rdf.OpenSysMLTerm(mPseudostate))
		e.name(subject, n.Name)
		e.graph.Add(subject, e.sysx(xPseudostateKind), rdf.String(n.Kind.String()))
		e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(e.pseudostateKeyword(n)))
		return true, nil

	case *ast.TransitionMember:
		return true, e.encodeTransition(n, head, subject, fqn, owner)
	}
	return false, nil
}

// encodeLoop emits a loop of an action body. Which conditions it carries is what
// tells the three forms apart: a `while` states its condition before the body, a
// `loop` only in an `until` clause after it, and a `for` iterates a collection.
func (e *encoder) encodeLoop(n *ast.WhileLoopActionNode, head func(rdf.Term), subject rdf.Term, fqn, owner string) error {
	if n.Kind == ast.LoopFor {
		head(rdf.SysMLTerm(mForLoop))
		if n.Variable.Name == "" {
			return &UnsupportedError{
				What: fmt.Sprintf("the loop at %s", e.where(n)),
				Note: "its iteration variable has no name, so the variable the body binds cannot be written back",
			}
		}
		e.graph.Add(subject, e.sysx(xLoopVariable), rdf.String(n.Variable.Name))
		e.expression(subject, e.sysx(xCollection), xCollection, owner, n.Collection)
	} else {
		head(rdf.SysMLTerm(mWhileLoop))
		switch n.Kind {
		case ast.LoopWhile:
			e.expression(subject, e.sysx(xWhileCondition), xWhileCondition, owner, n.Condition)
			e.expression(subject, e.sysx(xUntilCondition), xUntilCondition, owner, n.Until)
		default:
			// A `loop` tests its condition after each iteration, which is what an
			// `until` clause states; without one it has no condition at all.
			e.expression(subject, e.sysx(xUntilCondition), xUntilCondition, owner, n.Condition)
		}
	}
	e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(e.bracedBody(n, n.Body)))
	return e.encode(n.Body, fqn, subject)
}

// encodeSubaction emits one of a state's entry/do/exit subactions. The kind is
// the membership's, so the same three notations — empty, a braced sequence, a
// single action — are told apart by the body rather than by the keyword.
func (e *encoder) encodeSubaction(n ast.Node, actions []ast.Node, kind string, head func(rdf.Term), subject rdf.Term, fqn string) error {
	head(rdf.SysMLTerm(mSubaction))
	e.graph.Add(subject, e.sysx(xSubactionKind), rdf.String(kind))
	// `entry do { … }` states the subaction's own keyword and `do` as well, with
	// or without a space or a comment between them and the body.
	if written := strings.Fields(withoutComments(e.text(n))); len(written) > 1 && bareWord(written[1]) == "do" && kind != "do" {
		e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(kind+" do"))
	}
	e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(e.bracedBody(n, actions)))
	return e.encode(actions, fqn, subject)
}

// encodeTransition emits a transition of a state machine: its ends as
// references, its trigger and guard as the text they were written as, and its
// effect as the actions it owns.
func (e *encoder) encodeTransition(n *ast.TransitionMember, head func(rdf.Term), subject rdf.Term, fqn, owner string) error {
	head(rdf.SysMLTerm(mTransition))
	e.name(subject, n.Name)
	e.graph.Add(subject, e.sysx(xTransitionSyntax), rdf.String(e.transitionSyntax(n)))
	if source := qualifiedText(n.Source); source != "" {
		e.graph.Add(subject, e.sysml(pSourceFeature), e.reference(owner, source))
	}
	target := qualifiedText(n.Target)
	if target == "" {
		return &UnsupportedError{
			What: fmt.Sprintf("the transition at %s", e.where(n)),
			Note: "it names no target state, so the edge it declares cannot be written back",
		}
	}
	e.graph.Add(subject, e.sysml(pTargetFeature), e.reference(owner, target))
	if n.Trigger != nil {
		e.graph.Add(subject, e.sysx(xTrigger), rdf.String(e.text(n.Trigger)))
		e.graph.Add(subject, e.sysx(xTriggerKeyword), rdf.String(e.introducer(n, n.Trigger)))
	}
	if n.Via != nil {
		e.graph.Add(subject, e.sysml(relationshipProperty[ast.RelVia]), e.reference(owner, qualifiedText(n.Via)))
	}
	e.expression(subject, e.sysx(xGuard), xGuard, owner, n.Guard)
	if len(n.Effect) > 0 {
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(e.bracedBody(n, n.Effect)))
	}
	return e.encode(n.Effect, fqn, subject)
}

// impliedSource reports whether the source name came from the member before the
// edge rather than the edge itself, which its span outside the edge's tells.
func impliedSource(edge ast.Node, source *ast.QualifiedName) bool {
	return source != nil && source.Span().Offset < edge.Span().Offset
}

// edgeEnd is one end of a succession: the member it names, or the member the
// notation reaches by position where it names none.
type edgeEnd struct {
	name   *ast.QualifiedName
	member ast.Node
	// implied marks an end the notation states no name for, whose name the
	// parser took from the member before the edge.
	implied bool
}

// edgeEnds writes the ends of a succession: a name as a feature reference, an
// unnamed end as the member it binds by position, anything else refused.
func (e *encoder) edgeEnds(subject rdf.Term, node ast.Node, owner string, src, tgt edgeEnd) error {
	ends := []struct {
		end      edgeEnd
		feature  string
		member   string
		sequence string
	}{
		{src, pSourceFeature, xSourceMember, "sequences from"},
		{tgt, pTargetFeature, xTargetMember, "sequences to"},
	}
	for _, end := range ends {
		name := qualifiedText(end.end.name)
		if name == "" {
			fqn, ok := e.fqn[end.end.member]
			if !ok {
				return &UnsupportedError{
					What: fmt.Sprintf("the succession at %s", e.where(node)),
					Note: fmt.Sprintf("it neither names nor reaches the member it %s, so the order it declares cannot be written back", end.sequence),
				}
			}
			e.graph.Add(subject, e.sysx(end.member), rdf.ElementIRI(fqn))
			continue
		}
		// The two-name form is read only as basic names, so an end needing quotes
		// is refused; an implied end is not written, so its spelling is free.
		if quotedName(name) && !end.end.implied {
			return &UnsupportedError{
				What: fmt.Sprintf("the succession at %s", e.where(node)),
				Note: fmt.Sprintf("it sequences %s, whose name is not a basic name, and the two-end form the graph is written back as reads only basic names", name),
			}
		}
		e.graph.Add(subject, e.sysml(end.feature), e.reference(owner, name))
	}
	return nil
}

// quotedName reports whether any segment of a qualified name is written with
// quotes, the spelling nameText gives it back.
func quotedName(qualified string) bool {
	for _, segment := range strings.Split(qualified, "::") {
		segment = strings.TrimSpace(segment)
		if nameText(unquote(segment)) != segment {
			return true
		}
	}
	return false
}

func (e *encoder) name(subject rdf.Term, name string) {
	if name != "" {
		e.graph.Add(subject, e.sysml(pDeclaredName), rdf.String(name))
	}
}

// writtenKeyword records the keyword a node was written with when it is a
// synonym of the canonical one, so the notation comes back as the author spelled
// it. canonical may be empty, for a statement whose keyword is optional.
func (e *encoder) writtenKeyword(subject rdf.Term, node ast.Node, canonical string, synonyms ...string) {
	word := firstWord(e.text(node))
	if word == canonical {
		return
	}
	for _, synonym := range synonyms {
		if word == synonym {
			e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(word))
			return
		}
	}
}

// pseudostateKeyword gives the notation of a pseudostate's kind: an entry or
// exit point states `point` as well, and a shallow history may be written with
// `history` alone.
func (e *encoder) pseudostateKeyword(n *ast.PseudostateNode) string {
	switch n.Kind {
	case ast.PseudostateEntry, ast.PseudostateExit:
		return n.Kind.String() + " point"
	case ast.PseudostateShallowHistory:
		if firstWord(e.text(n)) == "history" {
			return "history"
		}
	}
	return n.Kind.String()
}

// transitionSyntax names the spelling a transition was written in: `first`
// marking the source, `to` naming the target after it, or the `accept` of a
// transition that states only its trigger.
func (e *encoder) transitionSyntax(n *ast.TransitionMember) string {
	if n.Source == nil {
		return "accept"
	}
	// `first` is a legal state name, so `transition first to second;` names one
	// rather than marking the source.
	fields := strings.Fields(e.text(n))
	for i := 1; i < len(fields) && i <= 2; i++ {
		if fields[i] == "first" && (i+1 >= len(fields) || fields[i+1] != "to") {
			return "first"
		}
	}
	return "to"
}

// bracedBody reports whether a body was written with braces rather than as the
// single action or body parameter the notation also allows.
func (e *encoder) bracedBody(node ast.Node, body []ast.Node) bool {
	if len(body) == 0 {
		return strings.Contains(e.text(node), "{")
	}
	first, _ := unwrapMember(body[0])
	if first == nil {
		return false
	}
	return strings.HasSuffix(e.before(node, first), "{")
}

// bracedBranch reports whether a branch of a conditional was written with
// braces rather than as the `else if` or action parameter the notation allows.
func (e *encoder) bracedBranch(n *ast.IfBranchNode) bool {
	text := strings.TrimSpace(strings.TrimPrefix(e.text(n), "else"))
	return strings.HasPrefix(text, "{")
}

// before returns the text between the start of node and the start of inner.
func (e *encoder) before(node, inner ast.Node) string {
	start, end := node.Span().Offset, inner.Span().Offset
	if end <= start {
		return ""
	}
	return strings.TrimSpace(e.file.Text(source.Span{Offset: start, Len: end - start}))
}

// between returns the text between two nodes, which is the operator or keyword
// written there.
func (e *encoder) between(from, to ast.Node) string {
	if from == nil || to == nil {
		return ""
	}
	start, end := from.Span().End(), to.Span().Offset
	if end <= start {
		return ""
	}
	return strings.TrimSpace(e.file.Text(source.Span{Offset: start, Len: end - start}))
}

// introducer returns the keyword written immediately before inner, which is
// what the clause inner belongs to was introduced with.
func (e *encoder) introducer(node, inner ast.Node) string {
	fields := strings.Fields(e.before(node, inner))
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

func firstWord(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	return bareWord(fields[0])
}

// bareWord cuts a word at the punctuation it can run into, which the notation
// allows without a space between them.
func bareWord(field string) string {
	if cut := strings.IndexAny(field, ";{"); cut >= 0 {
		return field[:cut]
	}
	return field
}

// bareAcceptNode reports whether an accept node was written without the `action`
// keyword, which the notation makes optional and the parser records regardless.
func bareAcceptNode(n *ast.Usage, text string) bool {
	if n.Kind != ast.UsageAction || n.Keyword != "action" {
		return false
	}
	for _, word := range strings.Fields(text) {
		switch word {
		case "action":
			return false
		case "accept":
			return true
		}
	}
	return false
}

// stateBody gathers the members of a state, which the AST holds in one bucket
// per kind, back into the order they were written in.
func stateBody(n *ast.StateNode) []ast.Node {
	members := make([]ast.Node, 0,
		len(n.Entry)+len(n.Do)+len(n.Exit)+len(n.Defer)+len(n.Substates)+len(n.Regions))
	for _, bucket := range [][]ast.Node{n.Entry, n.Do, n.Exit, n.Defer, n.Substates} {
		members = append(members, bucket...)
	}
	for _, region := range n.Regions {
		members = append(members, region)
	}
	sort.SliceStable(members, func(i, j int) bool {
		return members[i].Span().Offset < members[j].Span().Offset
	})
	return members
}

// behaviorNameAndMembers returns the name a behavioral node declares and the
// members it owns, for the nodes declaredNameAndMembers does not cover.
func behaviorNameAndMembers(node ast.Node) (string, []ast.Node, bool) {
	switch n := node.(type) {
	case *ast.InitialNode:
		// Its name references the starting member, so it declares none.
		return "", nil, true
	case *ast.FinalNode:
		return "", nil, true
	case *ast.ForkNode:
		return n.Name, nil, true
	case *ast.JoinNode:
		return n.Name, nil, true
	case *ast.MergeNode:
		return n.Name, nil, true
	case *ast.DecisionNode:
		return n.Name, nil, true
	case *ast.ActionExecutionNode:
		return n.Name, nil, true
	case *ast.WhileLoopActionNode:
		return "", n.Body, true
	case *ast.IfActionNode:
		branches := make([]ast.Node, 0, 2)
		for _, branch := range n.Branches() {
			branches = append(branches, branch)
		}
		return "", branches, true
	case *ast.IfBranchNode:
		return "", n.Body, true
	case *ast.StateNode:
		return n.Name, stateBody(n), true
	case *ast.SubstateMember:
		return n.Name, nil, true
	case *ast.StateRegion:
		return n.Name, n.States, true
	case *ast.PseudostateNode:
		return n.Name, nil, true
	case *ast.EntryMember:
		return "", n.Actions, true
	case *ast.DoMember:
		return "", n.Actions, true
	case *ast.ExitMember:
		return "", n.Actions, true
	case *ast.TransitionMember:
		return n.Name, n.Effect, true
	}
	return "", nil, false
}

// behaviorHead builds the declaration text of a behavioral element whose
// notation is a head and a terminator, reporting whether the metaclass was one.
func (d *decoder) behaviorHead(el *element) (string, bool, error) {
	switch el.metaclass {
	case mInitialNode:
		words := []string{"first"}
		start, err := d.referenceText(el, rdf.SysML+pSourceFeature)
		if err != nil {
			return "", true, err
		}
		if start != "" {
			words = append(words, start)
		}
		if guard, ok := d.stringOf(el, rdf.OpenSysML+xGuard); ok {
			words = append(words, "if", guard)
		}
		successor, err := d.referenceText(el, rdf.SysML+pTargetFeature)
		if err != nil {
			return "", true, err
		}
		if successor != "" {
			words = append(words, "then", successor)
		}
		return strings.Join(words, " "), true, nil

	case mFinalNode:
		words := []string{d.keywordOr(el, "done")}
		return strings.Join(words, " "), true, nil

	case mFork, mJoin, mMerge, mDecision:
		words := []string{controlNodeKeyword[el.metaclass]}
		return strings.Join(append(words, d.identWords(el)...), " "), true, nil

	case mActionExecution:
		words := []string{"action"}
		words = append(words, d.identWords(el)...)
		reference, err := d.referenceText(el, rdf.SysML+relationshipProperty[ast.RelReferences])
		if err != nil {
			return "", true, err
		}
		switch expression, ok := d.stringOf(el, rdf.OpenSysML+xExpression); {
		case ok:
			words = append(words, "{ "+expression+" }")
		case reference != "":
			words = append(words, reference)
		default:
			return "", true, d.missing(el, "sysx:"+xExpression, "an action node performs an action or evaluates an expression")
		}
		return strings.Join(words, " "), true, nil

	case mPerform:
		action, ok := d.stringOf(el, rdf.OpenSysML+xExpression)
		if !ok {
			return "", true, d.missing(el, "sysx:"+xExpression, "a perform statement names the action it performs")
		}
		return "perform " + action, true, nil

	case mAssignment:
		target, hasTarget := d.stringOf(el, rdf.OpenSysML+xTarget)
		value, hasValue := d.stringOf(el, rdf.SysML+pValue)
		if !hasTarget || !hasValue {
			return "", true, d.missing(el, "sysx:"+xTarget+" and sysml:"+pValue, "an assignment states what it assigns to what")
		}
		var words []string
		if keyword, ok := d.stringOf(el, rdf.OpenSysML+xDeclaredKeyword); ok {
			words = append(words, keyword)
		}
		operator := ":="
		if written, ok := d.stringOf(el, rdf.OpenSysML+xAssignOperator); ok {
			operator = written
		}
		words = append(words, target, operator, value)
		return strings.Join(words, " "), true, nil

	case mSend:
		payload, hasPayload := d.stringOf(el, rdf.OpenSysML+xPayload)
		receiver, hasReceiver := d.stringOf(el, rdf.OpenSysML+xReceiver)
		if !hasPayload || !hasReceiver {
			return "", true, d.missing(el, "sysx:"+xPayload+" and sysx:"+xReceiver, "a send states what it sends and where")
		}
		keyword := "to"
		if d.boolOf(el, rdf.OpenSysML+xIsVia) {
			keyword = "via"
		}
		return strings.Join([]string{"send", payload, keyword, receiver}, " "), true, nil

	case mTerminate:
		words := []string{"terminate"}
		if target, ok := d.stringOf(el, rdf.OpenSysML+xExpression); ok {
			words = append(words, target)
		}
		return strings.Join(words, " "), true, nil

	case mDeferMember:
		events := d.graph.Objects(rdf.IRI(el.iri), rdf.OpenSysML+xDeferredEvent)
		if len(events) == 0 {
			return "", true, d.missing(el, "sysx:"+xDeferredEvent, "a defer member names the events it defers")
		}
		names := make([]string, 0, len(events))
		for _, event := range events {
			names = append(names, event.Value)
		}
		return "defer " + strings.Join(names, ", "), true, nil

	case mPseudostate:
		kind, ok := d.stringOf(el, rdf.OpenSysML+xPseudostateKind)
		if !ok {
			return "", true, d.missing(el, "sysx:"+xPseudostateKind, "a pseudostate states which kind it is")
		}
		words := []string{d.keywordOr(el, kind)}
		return strings.Join(append(words, d.identWords(el)...), " "), true, nil

	case mStateRegion:
		words := []string{"region"}
		return strings.Join(append(words, d.identWords(el)...), " "), true, nil
	}
	return "", false, nil
}

// controlNodeKeyword gives the notation of each control node metaclass.
var controlNodeKeyword = map[string]string{
	mFork:     "fork",
	mJoin:     "join",
	mMerge:    "merge",
	mDecision: "decide",
}

// successionHead writes a succession back using standard end notation.
func (d *decoder) successionHead(el *element) (string, error) {
	source, err := d.referenceText(el, rdf.SysML+pSourceFeature)
	if err != nil {
		return "", err
	}
	target, err := d.referenceText(el, rdf.SysML+pTargetFeature)
	if err != nil {
		return "", err
	}
	guard, hasGuard := d.stringOf(el, rdf.OpenSysML+xGuard)
	if target == "" {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the succession <%s>", el.iri),
			Note: "it does not name both of the members it sequences, so the order it declares cannot be written back",
		}
	}
	_, positionalSource := d.endMember(el, xSourceMember)
	switch keyword := d.keywordOr(el, "then"); {
	case keyword == "else" || d.boolOf(el, rdf.OpenSysML+xIsElse):
		return "else " + target, nil
	case keyword == "if":
		if !hasGuard {
			return "", d.missing(el, "sysx:"+xGuard, "a branch written with `if` states the condition it is taken under")
		}
		return "if " + guard + " then " + target, nil
	}
	if form, _ := d.stringOf(el, rdf.OpenSysML+xEndForm); form == formThen || positionalSource {
		// The source end is the member written before, which this form leaves
		// unwritten.
		return "then " + target, nil
	}
	if source == "" {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the succession <%s>", el.iri),
			Note: "it does not name a source end and is not a positional `then` succession",
		}
	}
	words := []string{"succession", "first", source, "then", target}
	if hasGuard {
		words = []string{"succession", "first", source, "if", guard, "then", target}
	}
	return strings.Join(words, " "), nil
}

// positionalSuccessions resolves the successions of one body stating no source
// name: each sequences from the member before it, folded in as `then action b;`.
func (d *decoder) positionalSuccessions(children []*element) ([]*element, error) {
	kept := make([]*element, 0, len(children))
	// before is the member written that many places back, the end a succession
	// stating no source name of its own sequences from.
	before := func(back int) *element {
		if len(kept) < back {
			return nil
		}
		return kept[len(kept)-back]
	}
	for _, child := range children {
		implied := false
		if form, ok := d.stringOf(child, rdf.OpenSysML+xEndForm); ok {
			implied = form == formThen
		}
		target, positional := d.endMember(child, xTargetMember)
		if child.metaclass != mSuccession || (!implied && !positional) {
			kept = append(kept, child)
			continue
		}
		if !positional {
			target = d.namedEnd(child, pTargetFeature)
		}
		if target == nil || target != before(1) {
			// The target is a member elsewhere in the body, so the succession
			// is written where it stands, sequencing from the member before it.
			if positional {
				return nil, d.positionalError(child, "to", "before it")
			}
			if err := d.impliedSource(child, before(1)); err != nil {
				return nil, err
			}
			kept = append(kept, child)
			continue
		}
		// The target is the member written just before, which this form
		// introduces: `then` is written ahead of that member's declaration.
		if err := d.attachable(child, before(2)); err != nil {
			return nil, err
		}
		target.prefix = "then "
	}
	return kept, nil
}

// endMember returns the element an end of a succession reaches by position, and
// whether the graph states that end that way at all.
func (d *decoder) endMember(el *element, property string) (*element, bool) {
	term, ok := d.graph.Object(rdf.IRI(el.iri), rdf.OpenSysML+property)
	if !ok {
		return nil, false
	}
	return d.byIRI[term.Value], true
}

// namedEnd returns the element an end of a succession names, or nil for a name
// this body does not declare.
func (d *decoder) namedEnd(el *element, property string) *element {
	term, ok := d.graph.Object(rdf.IRI(el.iri), rdf.SysML+property)
	if !ok {
		return nil
	}
	return d.byIRI[term.Value]
}

// sourceEnd returns the element a succession sequences from, by position or by
// the name it states, and whether it states either.
func (d *decoder) sourceEnd(el *element) (*element, bool) {
	if member, positional := d.endMember(el, xSourceMember); positional {
		return member, true
	}
	if _, ok := d.graph.Object(rdf.IRI(el.iri), rdf.SysML+pSourceFeature); !ok {
		return nil, false
	}
	return d.namedEnd(el, pSourceFeature), true
}

// impliedSource checks a `then <target>`, whose source end is the member before
// it: the graph has to agree, or the order it states would not be written back.
func (d *decoder) impliedSource(el, from *element) error {
	source, states := d.sourceEnd(el)
	if !states {
		return d.missing(el, "sysml:"+pSourceFeature, "a succession written as `then` sequences from the member before it")
	}
	if source != from {
		return d.positionalError(el, "from", "before it")
	}
	return nil
}

// attachable checks a succession folded into the member it introduces: a plain
// `then` only, sequencing from the member written before that one.
func (d *decoder) attachable(el, from *element) error {
	if keyword := d.keywordOr(el, "then"); keyword != "then" {
		return &UnsupportedError{
			What: fmt.Sprintf("the succession <%s>", el.iri),
			Note: fmt.Sprintf("it reaches a member that states no name, which only `then` is written beside, not `%s`", keyword),
		}
	}
	if _, hasGuard := d.stringOf(el, rdf.OpenSysML+xGuard); hasGuard {
		return &UnsupportedError{
			What: fmt.Sprintf("the succession <%s>", el.iri),
			Note: "it states a guard and reaches a member that states no name, a form that carries no guard",
		}
	}
	source, states := d.sourceEnd(el)
	if states && source != from {
		return d.positionalError(el, "from", "before the member it introduces")
	}
	return nil
}

// positionalError reports a succession whose ends the graph states in an order
// the notation it is written in cannot express.
func (d *decoder) positionalError(el *element, end, where string) error {
	return &UnsupportedError{
		What: fmt.Sprintf("the succession <%s>", el.iri),
		Note: fmt.Sprintf("it sequences %s the member written %s, which is another member of this body", end, where),
	}
}

// printBehavior writes the behavioral elements whose notation is not a head
// followed by a body: the loops and conditionals whose conditions are written
// around the body, a state's subactions, and a transition's clauses.
// indent is what the declaration is written after, including the `then` of a
// succession folded into it.
func (d *decoder) printBehavior(b *strings.Builder, el *element, indent string, depth int) (bool, error) {
	switch el.metaclass {
	case mWhileLoop, mForLoop:
		text, err := d.loopText(el, depth)
		if err != nil {
			return true, err
		}
		b.WriteString(indent + text + "\n")
		return true, nil

	case mIfAction:
		text, err := d.conditionalText(el, depth)
		if err != nil {
			return true, err
		}
		b.WriteString(indent + text + "\n")
		return true, nil

	case mSubaction:
		text, err := d.subactionText(el, depth)
		if err != nil {
			return true, err
		}
		b.WriteString(indent + text + "\n")
		return true, nil

	case mIfBranch:
		return true, &UnsupportedError{
			What: fmt.Sprintf("the branch <%s>", el.iri),
			Note: "a branch is written inside the conditional that owns it, and no conditional owns this one",
		}

	case mTransition:
		// A transition carrying its ends as references is the one a state body
		// declares; a transition usage whose head was kept verbatim never
		// reaches here, since print() writes its source text.
		if _, structural := d.graph.Object(rdf.IRI(el.iri), rdf.SysML+pTargetFeature); !structural {
			return false, nil
		}
		text, err := d.transitionText(el, depth)
		if err != nil {
			return true, err
		}
		b.WriteString(indent + text + ";\n")
		return true, nil
	}
	return false, nil
}

// loopText writes a loop and the conditions around its body.
func (d *decoder) loopText(el *element, depth int) (string, error) {
	var head string
	switch el.metaclass {
	case mForLoop:
		variable, hasVariable := d.stringOf(el, rdf.OpenSysML+xLoopVariable)
		collection, hasCollection := d.stringOf(el, rdf.OpenSysML+xCollection)
		if !hasVariable || !hasCollection {
			return "", d.missing(el, "sysx:"+xLoopVariable+" and sysx:"+xCollection, "a for loop binds a variable over a collection")
		}
		head = "for " + nameText(variable) + " in " + collection
	default:
		if condition, ok := d.stringOf(el, rdf.OpenSysML+xWhileCondition); ok {
			head = "while " + condition
		} else {
			head = "loop"
		}
	}
	body, err := d.bodyText(el, depth)
	if err != nil {
		return "", err
	}
	text := head + " " + body
	// An `until` clause states the condition tested after each iteration, and
	// terminates the loop; so does a body written without braces.
	if until, ok := d.stringOf(el, rdf.OpenSysML+xUntilCondition); ok {
		return text + " until " + until + ";", nil
	}
	if !d.boolOf(el, rdf.OpenSysML+xHasBody) && !strings.HasSuffix(text, ";") && el.metaclass != mForLoop {
		return text + ";", nil
	}
	return text, nil
}

// conditionalText writes an if action: its condition, the branch taken when the
// condition holds, and the one taken when it does not.
func (d *decoder) conditionalText(el *element, depth int) (string, error) {
	condition, ok := d.stringOf(el, rdf.OpenSysML+xCondition)
	if !ok {
		return "", d.missing(el, "sysx:"+xCondition, "an if action states the condition it branches on")
	}
	var then, otherwise *element
	for _, child := range el.children {
		if child.metaclass != mIfBranch {
			return "", &UnsupportedError{
				What: fmt.Sprintf("the member <%s> of the if action <%s>", child.iri, el.iri),
				Note: "an if action owns its branches, and this member is not one",
			}
		}
		kind, _ := d.stringOf(child, rdf.OpenSysML+xBranchKind)
		if kind == ast.IfBranchElse.String() {
			otherwise = child
		} else {
			then = child
		}
	}
	if then == nil {
		return "", d.missing(el, "sysx:"+xBranchKind, "an if action states the branch taken when its condition holds")
	}
	thenText, err := d.bodyText(then, depth)
	if err != nil {
		return "", err
	}
	text := "if " + condition + " " + thenText
	if otherwise == nil {
		return text, nil
	}
	elseText, err := d.bodyText(otherwise, depth)
	if err != nil {
		return "", err
	}
	return text + " else " + elseText, nil
}

// subactionText writes one of a state's entry/do/exit subactions in the shape it
// was written: empty, a braced sequence, or a single action.
func (d *decoder) subactionText(el *element, depth int) (string, error) {
	kind, ok := d.stringOf(el, rdf.OpenSysML+xSubactionKind)
	if !ok {
		return "", d.missing(el, "sysx:"+xSubactionKind, "a state subaction states whether it runs on entry, throughout or on exit")
	}
	keyword := d.keywordOr(el, kind)
	braced := d.boolOf(el, rdf.OpenSysML+xHasBody)
	if len(el.children) == 0 && !braced {
		return keyword + ";", nil
	}
	body, err := d.bodyText(el, depth)
	if err != nil {
		return "", err
	}
	if braced {
		return keyword + " " + body, nil
	}
	// A performed action states the subaction's keyword itself
	// (`entry warmUp;`), so writing the keyword again would declare it twice.
	if len(el.children) == 1 {
		if written, ok := d.stringOf(el.children[0], rdf.OpenSysML+xDeclaredKeyword); ok && written == kind {
			return body, nil
		}
	}
	return keyword + " " + body, nil
}

// transitionText writes a transition of a state machine, in the spelling it was
// written in, with its trigger, guard and effect.
func (d *decoder) transitionText(el *element, depth int) (string, error) {
	source, err := d.referenceText(el, rdf.SysML+pSourceFeature)
	if err != nil {
		return "", err
	}
	target, err := d.referenceText(el, rdf.SysML+pTargetFeature)
	if err != nil {
		return "", err
	}
	syntax := "first"
	if written, ok := d.stringOf(el, rdf.OpenSysML+xTransitionSyntax); ok {
		syntax = written
	}
	var words []string
	switch syntax {
	case "accept":
		// The transition of a state body that states only its trigger.
	default:
		if source == "" {
			return "", d.missing(el, "sysml:"+pSourceFeature, "a transition written with `transition` names the state it leaves")
		}
		words = append(words, "transition")
		words = append(words, d.identWords(el)...)
		if syntax == "to" {
			words = append(words, source, "to", target)
		} else {
			words = append(words, "first", source)
		}
	}
	if trigger, ok := d.stringOf(el, rdf.OpenSysML+xTrigger); ok {
		keyword := "accept"
		if written, ok := d.stringOf(el, rdf.OpenSysML+xTriggerKeyword); ok {
			keyword = written
		}
		words = append(words, keyword, trigger)
		via, err := d.referenceText(el, rdf.SysML+relationshipProperty[ast.RelVia])
		if err != nil {
			return "", err
		}
		if via != "" {
			words = append(words, "via", via)
		}
	}
	if guard, ok := d.stringOf(el, rdf.OpenSysML+xGuard); ok {
		words = append(words, "if", guard)
	}
	if len(el.children) > 0 {
		effect, err := d.bodyText(el, depth)
		if err != nil {
			return "", err
		}
		words = append(words, "do", strings.TrimSuffix(effect, ";"))
	}
	if syntax != "to" {
		words = append(words, "then", target)
	}
	return strings.Join(words, " "), nil
}

// bodyText writes the members of an element: braced when the notation was, and
// as the members alone when it stated them without braces.
func (d *decoder) bodyText(el *element, depth int) (string, error) {
	indent := strings.Repeat("    ", depth)
	var members strings.Builder
	for _, child := range el.children {
		if err := d.print(&members, child, depth+1); err != nil {
			return "", err
		}
	}
	if d.boolOf(el, rdf.OpenSysML+xHasBody) {
		return "{\n" + members.String() + indent + "}", nil
	}
	return strings.TrimSpace(members.String()), nil
}

// referenceMemberKeyword reports whether a keyword introduces a member that
// names an existing feature rather than declaring one — `perform doIt;`,
// `exhibit modes;` and a state's `entry warmUp;` — so the reference is its
// notation.
func referenceMemberKeyword(keyword string) bool {
	switch keyword {
	case "perform", "exhibit", "entry", "do", "exit":
		return true
	}
	return false
}
