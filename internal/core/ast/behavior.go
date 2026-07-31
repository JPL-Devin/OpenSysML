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
