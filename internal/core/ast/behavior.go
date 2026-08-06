package ast

// Behavioral AST nodes for SysML v2 actions and state machines.
// These nodes implement the Node interface and populate Usage.Members
// for action and state usages (UsageAction, UsageState).

// InitialNode is the entry point for action execution.
type InitialNode struct {
	NodeBase
	Name      string         // optional identifier for edge referencing
	Successor *QualifiedName // optional target for implicit succession (from `first X then Y` syntax)
	Guard     Node           // optional guard condition for succession
}

// FinalNode is the termination point for action execution.
type FinalNode struct {
	NodeBase
	Name string
}

// ForkNode splits execution into concurrent flows (1 incoming → N outgoing).
type ForkNode struct {
	NodeBase
	Name string
}

// JoinNode synchronizes concurrent flows (N incoming → 1 outgoing).
type JoinNode struct {
	NodeBase
	Name string
}

// MergeNode merges alternative flows (N incoming → 1 outgoing, first-wins).
type MergeNode struct {
	NodeBase
	Name string
}

// DecisionNode is a conditional branch point (1 incoming → N guarded outgoing).
type DecisionNode struct {
	NodeBase
	Name string
}

// ActionExecutionNode performs action work: invokes nested action or evaluates inline expression.
type ActionExecutionNode struct {
	NodeBase
	Name       string
	ActionRef  *QualifiedName // reference to nested action (mutually exclusive with Expression)
	Expression Node           // inline expression (mutually exclusive with ActionRef)
}

// AssignmentActionNode represents an assignment statement: assign target := value;
type AssignmentActionNode struct {
	NodeBase
	Target Node // feature reference or qualified name
	Value  Node // expression to assign
}

// PerformActionNode represents a perform statement: perform action;
type PerformActionNode struct {
	NodeBase
	ActionRef Node // qualified name or invocation expression
}

// WhileLoopActionNode represents a while loop: while condition { body }
type WhileLoopActionNode struct {
	NodeBase
	Condition Node   // boolean expression
	Body      []Node // statements in loop body
}

// IfBranchKind discriminates the two branches of an if action.
type IfBranchKind int

const (
	// IfBranchThen is the branch taken when the condition holds.
	IfBranchThen IfBranchKind = iota
	// IfBranchElse is the branch taken when it does not.
	IfBranchElse
)

// String renders the branch's introducing keyword.
func (k IfBranchKind) String() string {
	if k == IfBranchElse {
		return "else"
	}
	return "then"
}

// IfBranchNode is the body of one branch of an if action. It exists so that a
// branch is an element in its own right: the declarations a branch body makes
// are members of the branch, not of the enclosing behavior, and only a node can
// own a scope.
type IfBranchNode struct {
	NodeBase
	Kind IfBranchKind
	Body []Node // statements in the branch body
}

// IfActionNode represents conditional: if condition { thenBody } else { elseBody }
type IfActionNode struct {
	NodeBase
	Condition Node          // boolean expression
	Then      *IfBranchNode // branch taken when the condition holds
	Else      *IfBranchNode // branch taken otherwise (nil when there is no else)
}

// Branches returns the branches the conditional declares, in source order,
// skipping an absent else.
func (n *IfActionNode) Branches() []*IfBranchNode {
	var branches []*IfBranchNode
	if n.Then != nil {
		branches = append(branches, n.Then)
	}
	if n.Else != nil {
		branches = append(branches, n.Else)
	}
	return branches
}

// StateNode represents a state in a state machine (simple, composite, or orthogonal).
type StateNode struct {
	NodeBase
	Name      string
	IsInitial bool   // initial state marker
	IsFinal   bool   // final state marker
	Entry     []Node // entry behaviors (action sequence)
	Do        []Node // do activity (ongoing action)
	Exit      []Node // exit behaviors (action sequence)
	// Defer names the events the state defers while it is active: an event no
	// transition of the active configuration handles is retained instead of
	// dropped, and delivered again once no active state defers it.
	Defer     []Node
	Substates []Node         // nested states (hierarchical)
	Regions   []*StateRegion // orthogonal regions (parallel)
}

// StateRegion is an orthogonal region within a composite state.
type StateRegion struct {
	NodeBase
	Name   string
	States []Node // states in this region
}

// PseudostateKind discriminates pseudostate types.
type PseudostateKind int

const (
	PseudostateChoice   PseudostateKind = iota // conditional branch
	PseudostateJunction                        // merge point
	PseudostateFork                            // parallel split
	PseudostateJoin                            // parallel sync
	PseudostateEntry                           // entry point (submachine)
	PseudostateExit                            // exit point (submachine)
	// PseudostateShallowHistory re-enters the substate of its composite state
	// that was active when that state was last exited.
	PseudostateShallowHistory
	// PseudostateDeepHistory re-enters the innermost substate that was active
	// when its composite state was last exited.
	PseudostateDeepHistory
)

func (k PseudostateKind) String() string {
	switch k {
	case PseudostateChoice:
		return "choice"
	case PseudostateJunction:
		return "junction"
	case PseudostateFork:
		return "fork"
	case PseudostateJoin:
		return "join"
	case PseudostateEntry:
		return "entry"
	case PseudostateExit:
		return "exit"
	case PseudostateShallowHistory:
		return "shallow history"
	case PseudostateDeepHistory:
		return "deep history"
	default:
		return "unknown"
	}
}

// PseudostateNode is a transient state for control flow.
type PseudostateNode struct {
	NodeBase
	Kind PseudostateKind
	Name string
}

// SuccessionEdge is sequential control flow in actions (source then target).
type SuccessionEdge struct {
	NodeBase
	Source *QualifiedName // source action node
	Target *QualifiedName // target action node
}

// ControlFlowEdge is guarded control flow from decision nodes.
type ControlFlowEdge struct {
	NodeBase
	Source *QualifiedName // source node (typically DecisionNode)
	Target *QualifiedName // target node
	Guard  Node           // boolean guard expression
}

// ObjectFlowEdge is data flow between action parameters/pins (Tier 5).
type ObjectFlowEdge struct {
	NodeBase
	Source *QualifiedName // source pin/parameter
	Target *QualifiedName // target pin/parameter
}

// TransitionEdge is a state machine transition.
type TransitionEdge struct {
	NodeBase
	Source  *QualifiedName // source state
	Target  *QualifiedName // target state
	Trigger TriggerEvent   // event that fires transition (interface, see below)
	Guard   Node           // optional guard expression
	Effect  []Node         // optional effect actions
}

// TriggerEvent is the interface for state transition triggers.
type TriggerEvent interface {
	Node
	triggerEvent() // unexported marker method (closed set)
}

// TimeEvent fires at a point in time. Absolute distinguishes `accept at <time>`,
// which fires at the given instant, from `accept after <duration>`, which fires
// that far after the source state is entered.
type TimeEvent struct {
	NodeBase
	Duration Node // time expression (literal or variable)
	Absolute bool // true for `at`, false for `after`
}

func (*TimeEvent) triggerEvent() {}

// ChangeEvent fires when a condition becomes true.
type ChangeEvent struct {
	NodeBase
	Condition Node // boolean expression
}

func (*ChangeEvent) triggerEvent() {}

// AcceptEvent fires when a signal is received.
type AcceptEvent struct {
	NodeBase
	SignalType *QualifiedName // signal type to accept
}

func (*AcceptEvent) triggerEvent() {}

// CallEvent fires when an operation is invoked. Parameters are the argument
// names the trigger declares (`accept op(speed)`); an empty list matches a call
// to that operation whatever arguments it carries.
type CallEvent struct {
	NodeBase
	Operation  *QualifiedName // operation to invoke
	Parameters []NameSegment  // declared argument names, in written order
}

func (*CallEvent) triggerEvent() {}

// Phase C1: Calculation and Constraint Body Members

// ResultMember represents a return expression in a calculation body.
// Syntax: return <expression>;
type ResultMember struct {
	NodeBase
	Expression Node // the value expression
}

// ConstraintMember represents an assertion/assumption in a constraint body.
// Syntax: assert <expression>; or assume <expression>;
type ConstraintMember struct {
	NodeBase
	IsAssert   bool // true for 'assert', false for 'assume'
	IsNegated  bool // true if 'not' keyword present (assert not expr)
	Expression Node // the constraint expression
}

// Phase C2: Requirement Body Members

// SubjectMember represents the subject declaration in a requirement body.
// Syntax: subject <name> : <Type>; OR subject = <expr>;
type SubjectMember struct {
	NodeBase
	Name         string
	TypeRef      *QualifiedName // subject type (for declaration form)
	Multiplicity *Multiplicity  // optional multiplicity
	Body         []Node         // optional nested members
	BindingExpr  Node           // binding expression (for binding form: subject = <expr>;)
}

// AssumeMember represents an assumption in a requirement body.
// Syntax: assume <expression>;
type AssumeMember struct {
	NodeBase
	Expression Node // assumption condition
}

// RequireMember represents a requirement constraint.
// Syntax: require <expression>; OR require <name> { body }
type RequireMember struct {
	NodeBase
	Expression Node   // requirement condition (for expression form)
	Name       string // optional name (for body form)
	Body       []Node // optional nested members (for body form)
}

// ActorMember represents an actor declaration in a requirement/use case.
// Syntax: actor <name> : <Type>; OR actor <name> = <expr>;
type ActorMember struct {
	NodeBase
	Name        string
	TypeRef     *QualifiedName // actor type (for declaration form)
	BindingExpr Node           // binding expression (for binding form: actor = <expr>;)
}

// Phase C4: State Body Members

// EntryMember represents entry behavior in a state body.
// Syntax: entry { <actions> }
type EntryMember struct {
	NodeBase
	Actions []Node // action sequence
}

// DoMember represents ongoing activity in a state body.
// Syntax: do { <actions> }
type DoMember struct {
	NodeBase
	Actions []Node // action sequence
}

// ExitMember represents exit behavior in a state body.
// Syntax: exit { <actions> }
type ExitMember struct {
	NodeBase
	Actions []Node // action sequence
}

// DeferMember represents the events a state defers while it is active.
// Syntax: defer <event> [, <event>]* ;
type DeferMember struct {
	NodeBase
	Triggers []Node // deferred triggers, in declaration order
}

// SubstateMember represents a nested state declaration.
// Syntax: state <name>;
type SubstateMember struct {
	NodeBase
	Name string // substate name
}

// TransitionMember represents a state transition in textual form.
// Syntax: transition <source> to <target> [when <trigger>] [if <guard>] [do { <effect> }];
type TransitionMember struct {
	NodeBase
	Source  *QualifiedName // source state
	Target  *QualifiedName // target state
	Trigger Node           // optional trigger event (TimeEvent/ChangeEvent/etc)
	Guard   Node           // optional guard expression
	Effect  []Node         // optional effect actions
}

// SendStatement sends a message to a target.
// Syntax: send <message> to <target>; | send <message> via <port>;
//
// The two forms address different things and are not interchangeable: `to`
// names the receiver directly, while `via` names a port of the sender, and the
// message reaches whatever that port is connected to. IsVia records which form
// was written so routing can tell them apart.
type SendStatement struct {
	NodeBase
	Message Node // message expression (often NewExpression)
	Target  Node // target expression: the receiver (`to`) or the sending port (`via`)
	IsVia   bool // the target is a port to route through, not a receiver
}

// TerminateStatement terminates an action or lifecycle.
// Syntax: terminate <target>;
type TerminateStatement struct {
	NodeBase
	Target Node // target to terminate (action/part reference)
}

// AcceptActionUsage represents an action that waits for a signal.
// Syntax: action <name> accept <param> : Type;
type AcceptActionUsage struct {
	NodeBase
	Name      string
	ParamName string
	ParamType *QualifiedName
}
