package ast

// Behavioral AST nodes for SysML v2 actions and state machines.
// These nodes implement the Node interface and populate Usage.Members
// for action and state usages (UsageAction, UsageState).

// InitialNode is the entry point for action execution.
type InitialNode struct {
	NodeBase
	Name string // optional identifier for edge referencing
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
	Expression Node            // inline expression (mutually exclusive with ActionRef)
}

// StateNode represents a state in a state machine (simple, composite, or orthogonal).
type StateNode struct {
	NodeBase
	Name      string
	IsInitial bool           // initial state marker
	IsFinal   bool           // final state marker
	Entry     []Node         // entry behaviors (action sequence)
	Do        []Node         // do activity (ongoing action)
	Exit      []Node         // exit behaviors (action sequence)
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
	PseudostateChoice PseudostateKind = iota // conditional branch
	PseudostateJunction                       // merge point
	PseudostateFork                           // parallel split
	PseudostateJoin                           // parallel sync
	PseudostateEntry                          // entry point (submachine)
	PseudostateExit                           // exit point (submachine)
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
	Guard  Node            // boolean guard expression
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

// TimeEvent fires after a specified duration.
type TimeEvent struct {
	NodeBase
	Duration Node // time expression (literal or variable)
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

// CallEvent fires when an operation is invoked.
type CallEvent struct {
	NodeBase
	Operation *QualifiedName // operation to invoke
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
// Syntax: subject <name> : <Type>;
type SubjectMember struct {
	NodeBase
	Name     string
	TypeRef  *QualifiedName // subject type
}

// AssumeMember represents an assumption in a requirement body.
// Syntax: assume <expression>;
type AssumeMember struct {
	NodeBase
	Expression Node // assumption condition
}

// RequireMember represents a requirement constraint.
// Syntax: require <expression>;
type RequireMember struct {
	NodeBase
	Expression Node // requirement condition
}

// ActorMember represents an actor declaration in a requirement/use case.
// Syntax: actor <name> : <Type>;
type ActorMember struct {
	NodeBase
	Name     string
	TypeRef  *QualifiedName // actor type
}
