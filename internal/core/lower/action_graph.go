// Package lower provides AST → execution IR lowering.
// Converts declarative members (TransitionMember, EntryMember) to
// operational graphs (nodes + edges) that executors consume.
package lower

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// ActionGraph is the execution IR for actions.
// Nodes represent control flow points, edges represent flow paths.
type ActionGraph struct {
	// Scope is the scope the action's body was declared in, in which every
	// expression written directly among its members resolves its names. A nested
	// node or a body-local block carries its own scope instead.
	Scope *symbols.Scope

	// Attributes are the attribute defaults the action declares, in order.
	Attributes []Attribute

	// Nodes in the graph (InitialNode, FinalNode, ExecutionNode, etc.)
	Nodes []ast.Node

	// Edges: source node → list of target nodes
	Edges map[ast.Node][]ast.Node

	// Guards: source → target → guard expression
	Guards map[ast.Node]map[ast.Node]ast.Node

	// DataFlows: source node → list of object flows
	DataFlows map[ast.Node][]ObjectFlow

	// Bodies: node → the statements that node executes, in declaration order
	Bodies map[ast.Node][]Statement

	// Accepts: node → the message that node waits for
	Accepts map[ast.Node]Accept

	// InitialNode (required)
	Initial ast.Node

	// FinalNodes (may be multiple)
	Finals []ast.Node

	// Connections are the connectors declared in the action body, which is how
	// a `send ... via <port>` finds the ports it reaches.
	Connections []Connection
}

// Statement is one lowered statement in an action node's body. Statements are
// kept in declaration order so the executor never walks the node's members
// again to find them.
type Statement interface {
	statement()
}

// Send is a lowered send statement. Message stays an expression because its
// value is only known at execution time.
//
// Target is the simple name the send addressed, empty for a broadcast. IsVia
// records that the name is a port of the sender rather than a receiver, in which
// case the message goes to whatever the graph's Connections join that port to.
type Send struct {
	Message ast.Node
	Target  string
	IsVia   bool
	Scope   *symbols.Scope // the scope the statement was declared in
}

func (Send) statement() {}

// Assign is a lowered assignment: `assign <Target> := <Value>`. Target is the
// simple name assigned to, empty when the target was not a plain name.
type Assign struct {
	Target string
	Value  ast.Node
	Node   ast.Node       // the statement itself, for diagnostics
	Scope  *symbols.Scope // the scope the statement was declared in
}

func (Assign) statement() {}

// Declare is a lowered declaration in a body-local block: `attribute i = 0;`
// written inside a loop or an `if` branch. The name it declares is a member of
// that block, so the executor binds it in the block's own frame and discards it
// when the block exits. Value is nil when the declaration carried none.
type Declare struct {
	Name  string
	Value ast.Node
	Node  ast.Node       // the declaration itself, for diagnostics
	Scope *symbols.Scope // the scope the declaration was written in
}

func (Declare) statement() {}

// DeclareUsage is a calc usage declared in a body-local block: `calc p : Pair {
// in k = h; }` written inside a loop or an `if` branch. It states no step of the
// computation; it declares the usage and marks where it becomes reachable, so
// the statements after it read its outputs from one evaluation of its body per
// execution of the block.
type DeclareUsage struct {
	Name  string
	Node  *ast.Usage     // the declaration itself, for diagnostics
	Scope *symbols.Scope // the scope the usage was declared in
}

func (DeclareUsage) statement() {}

// Block is a lowered body-local statement list: the body of a loop or of one
// branch of a conditional. It is a namespace of its own (symbols/builder.go), so
// the names its Declare statements introduce do not leak out of it.
type Block struct {
	Statements []Statement
	Node       ast.Node // the loop or branch the block belongs to
	// Scope is the block's own scope, which its declarations, and a loop's
	// condition, resolve in.
	Scope *symbols.Scope
}

// A block is a statement in its own right: the anonymous action usage a loop or
// branch body is written as (`loop action { … } until c;`) runs its statements
// in its own namespace.
func (Block) statement() {}

// Loop is a lowered loop statement. Kind says when the condition is tested:
// before each iteration (`while`), after each iteration (`loop … until`), or
// not at all, iteration being driven by a collection (`for`).
//
// Condition and Collection stay expressions because their values are only known
// at execution time. The iteration count is bounded by the executor's step
// budget, so a loop that never terminates fails the run rather than hanging it.
type Loop struct {
	Kind      ast.LoopKind
	Condition ast.Node // nil for `for`, and for a `loop` written without `until`
	// Until is the condition a `while` loop's `until` clause tests after each
	// iteration (`while c { … } until d;`), nil when it carries none.
	Until      ast.Node
	Variable   string   // `for` only: the name each element is bound to
	Collection ast.Node // `for` only: the collection iterated over
	Body       Block
	Node       ast.Node // the loop itself, for diagnostics
	// Scope is the scope the loop was declared in, which its collection resolves
	// in; its condition resolves in Body.Scope, which the body declares into.
	Scope *symbols.Scope
}

func (Loop) statement() {}

// If is a lowered conditional. The condition is evaluated in the enclosing
// body, outside both branches. Else is nil when the conditional declared none.
type If struct {
	Condition ast.Node
	Then      Block
	Else      *Block
	Node      ast.Node       // the conditional itself, for diagnostics
	Scope     *symbols.Scope // the scope the conditional, and so its condition, was declared in
}

func (If) statement() {}

// Return is a lowered `return`: the value the enclosing behavior computes,
// possibly from inside a block. Value is nil when it named no expression.
type Return struct {
	Value ast.Node
	Node  ast.Node       // the return itself, for diagnostics
	Scope *symbols.Scope // the scope the returned expression was written in
}

func (Return) statement() {}

// EffectKind names a statement that acts on the world outside the body it
// stands in.
type EffectKind int

const (
	EffectPerform EffectKind = iota
	EffectAccept
	EffectTerminate
)

func (k EffectKind) String() string {
	switch k {
	case EffectPerform:
		return "perform"
	case EffectAccept:
		return "accept"
	case EffectTerminate:
		return "terminate"
	default:
		return "effect"
	}
}

// Effect is a statement acting on the world outside the body — perform, accept,
// terminate — lowered so a host rejecting it (a calculation) can say so.
type Effect struct {
	Kind  EffectKind
	Node  ast.Node
	Scope *symbols.Scope // the scope the statement was declared in
}

func (Effect) statement() {}

// Unsupported is a body member the lowering layer recognizes but cannot yet
// turn into an executable statement. It is lowered rather than dropped so that
// reaching it fails the execution with a diagnostic instead of silently
// producing a wrong answer. Description names the construct.
type Unsupported struct {
	Description string
	Node        ast.Node
	Scope       *symbols.Scope // the scope the member was declared in
}

func (Unsupported) statement() {}

// Accept is a lowered accept parameter: `action r accept msg : Warning;`.
// SignalType is the parameter's declared type, empty when it was declared
// without one, in which case the node accepts a message of any type.
//
// ViaPort is the port named by `accept msg : Warning via p`, empty when the
// accept named none. A port-routed message is only offered to an accept on the
// port it arrived at, so the two forms do not consume each other's messages.
//
// SubsetsEvent is the event feature the payload subsets (`accept :> shutDown`),
// empty when it subsets none. Such an accept waits for an occurrence of that
// one event rather than for any occurrence of a type.
//
// Trigger is the time or change event of `accept at t` / `accept after d` /
// `accept when c`, nil when the accept waits for a message instead.
type Accept struct {
	ParamName    string
	SignalType   string
	ViaPort      string
	SubsetsEvent string
	Trigger      ast.Node
}

// Attribute is a lowered attribute default written among a behavior's members
// (`attribute h : LengthValue = 500.0 [m];`), whose Value resolves in the
// graph's own scope.
type Attribute struct {
	Name  string
	Value ast.Node
	Node  ast.Node // the declaration itself, for diagnostics
}

// ObjectFlow represents a data flow edge between pins.
type ObjectFlow struct {
	// Name is the flow's own name, when it was declared with one
	// (`flow generateToAmplify from a.out to b.in;`), and "" for the anonymous
	// form and for a flow the notation writes as an edge.
	Name      string
	SourcePin string
	TargetPin string
	Target    ast.Node
}

// ToActionGraph converts an action AST (Usage or Definition) to an ActionGraph.
// scope is the scope the action's body was declared in — the scope the action
// itself owns — which every expression the graph carries is evaluated in.
// Returns error if graph is malformed (e.g., no initial node, dangling edges).
func ToActionGraph(actionDecl ast.Node, scope *symbols.Scope) (*ActionGraph, error) {
	graph := &ActionGraph{
		Scope:     scope,
		Nodes:     make([]ast.Node, 0),
		Edges:     make(map[ast.Node][]ast.Node),
		Guards:    make(map[ast.Node]map[ast.Node]ast.Node),
		DataFlows: make(map[ast.Node][]ObjectFlow),
		Bodies:    make(map[ast.Node][]Statement),
		Accepts:   make(map[ast.Node]Accept),
		Finals:    make([]ast.Node, 0),
	}

	// Extract members from Usage or Definition
	var members []ast.Node
	switch n := actionDecl.(type) {
	case *ast.Usage:
		members = n.Members
	case *ast.Definition:
		members = n.Members
	default:
		return nil, fmt.Errorf("action must be Usage or Definition, got %T", actionDecl)
	}

	// A succession can bind a member with no name of its own by position, which
	// is what puts an action node member written as a statement (`then send …;`,
	// `then loop action { … } until c;`) in the token flow.
	sequenced := sequencedMembers(members)

	// First pass: collect nodes
	for _, member := range members {
		actualMember := unwrapMembership(member)

		switch n := actualMember.(type) {
		case *ast.InitialNode:
			if graph.Initial != nil {
				return nil, fmt.Errorf("action has multiple initial nodes")
			}
			graph.Initial = n
			graph.Nodes = append(graph.Nodes, n)
		case *ast.FinalNode:
			graph.Finals = append(graph.Finals, n)
			graph.Nodes = append(graph.Nodes, n)
		case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode, *ast.ActionExecutionNode:
			graph.Nodes = append(graph.Nodes, n)
		case *ast.Usage:
			// Nested action usage (treat as execution node)
			if n.Kind == ast.UsageAction {
				graph.Nodes = append(graph.Nodes, n)
				lowerBody(graph, n, childScope(scope, n))
			}
		case *ast.WhileLoopActionNode, *ast.IfActionNode, *ast.AssignmentActionNode,
			*ast.SendStatement, *ast.TerminateStatement:
			if !sequenced[actualMember] {
				// A statement is executed as part of an action node's body; written
				// directly among the action's own members, with no succession
				// binding it, it has no position in the token flow.
				return nil, fmt.Errorf("%s written directly in an action body has no position in the token flow: declare it inside an action node", statementKeyword(n))
			}
			// An action node member of its own: a node whose body is the one
			// statement it was written as, run when a token reaches it.
			graph.Nodes = append(graph.Nodes, n)
			graph.Bodies[n] = []Statement{lowerStatement(n, scope)}
		}
	}

	graph.Connections = lowerConnections(members)
	graph.Attributes = lowerAttributes(members)

	// Note: Initial node is optional at graph construction time.
	// The executor's initialize() will validate and return the error if missing.

	// Second pass: build edges
	for _, member := range members {
		actualMember := unwrapMembership(member)

		switch n := actualMember.(type) {
		case *ast.InitialNode:
			// Handle implicit successor from `first X then Y` syntax
			if n.Successor != nil {
				targetNode := findNodeByName(graph.Nodes, n.Successor)
				if targetNode == nil {
					return nil, fmt.Errorf("initial node %s successor references undefined target %s", n.Name, edgeEndName(n.Successor))
				}
				graph.Edges[n] = append(graph.Edges[n], targetNode)
				if n.Guard != nil {
					if graph.Guards[n] == nil {
						graph.Guards[n] = make(map[ast.Node]ast.Node)
					}
					graph.Guards[n][targetNode] = n.Guard
				}
			}
		case *ast.SuccessionEdge:
			sourceNode := resolveEnd(graph.Nodes, n.Source, n.SourceMember)
			targetNode := resolveEnd(graph.Nodes, n.Target, n.TargetMember)

			if sourceNode == nil {
				return nil, fmt.Errorf("succession edge references undefined source node %s", edgeEnd(n.Source, n.SourceMember))
			}
			if targetNode == nil {
				return nil, fmt.Errorf("succession edge references undefined target node %s", edgeEnd(n.Target, n.TargetMember))
			}
			graph.Edges[sourceNode] = append(graph.Edges[sourceNode], targetNode)
		case *ast.ControlFlowEdge:
			sourceNode := resolveEnd(graph.Nodes, n.Source, n.SourceMember)
			targetNode := resolveEnd(graph.Nodes, n.Target, n.TargetMember)

			if sourceNode == nil {
				return nil, fmt.Errorf("control flow edge references undefined source %s", edgeEnd(n.Source, n.SourceMember))
			}
			if targetNode == nil {
				return nil, fmt.Errorf("control flow edge references undefined target %s", edgeEnd(n.Target, n.TargetMember))
			}
			graph.Edges[sourceNode] = append(graph.Edges[sourceNode], targetNode)

			// Store guard expression
			if n.Guard != nil {
				if graph.Guards[sourceNode] == nil {
					graph.Guards[sourceNode] = make(map[ast.Node]ast.Node)
				}
				graph.Guards[sourceNode][targetNode] = n.Guard
			}
		case *ast.ObjectFlowEdge:
			sourceNode, sourcePin := parsePinReference(graph.Nodes, n.Source)
			targetNode, targetPin := parsePinReference(graph.Nodes, n.Target)

			if sourceNode == nil {
				return nil, fmt.Errorf("object flow edge references undefined source %v", n.Source)
			}
			if targetNode == nil {
				return nil, fmt.Errorf("object flow edge references undefined target %v", n.Target)
			}

			graph.DataFlows[sourceNode] = append(graph.DataFlows[sourceNode], ObjectFlow{
				SourcePin: sourcePin,
				TargetPin: targetPin,
				Target:    targetNode,
			})
		case *ast.Usage:
			if n.Kind != ast.UsageFlow || n.FlowEnds == nil {
				continue
			}
			source, flow, err := lowerFlow(graph.Nodes, n)
			if err != nil {
				return nil, err
			}
			graph.DataFlows[source] = append(graph.DataFlows[source], flow)
		}
	}

	return graph, nil
}

// lowerBody records a nested action node's statements and the message it waits
// for, so the executor reads them from the graph rather than walking the node's
// members again.
func lowerBody(graph *ActionGraph, node *ast.Usage, scope *symbols.Scope) {
	for _, member := range node.Members {
		switch m := unwrapMembership(member).(type) {
		case *ast.SendStatement, *ast.AssignmentActionNode, *ast.WhileLoopActionNode, *ast.IfActionNode:
			graph.Bodies[node] = append(graph.Bodies[node], lowerStatement(m, scope))
		case *ast.Usage:
			if !m.IsAccept {
				continue
			}
			graph.Accepts[node] = Accept{
				ParamName:    m.Ident.Name,
				SignalType:   typingTarget(m),
				ViaPort:      acceptPort(node),
				SubsetsEvent: subsettingTarget(m),
				Trigger:      m.Value,
			}
		}
	}
}

// lowerStatement lowers one executable body statement, in the scope it was
// written in. Every form it recognizes is lowered losslessly; a form it does not
// becomes Unsupported, so the executor reports it rather than skipping it.
func lowerStatement(member ast.Node, scope *symbols.Scope) Statement {
	switch m := member.(type) {
	case *ast.SendStatement:
		return Send{
			Message: m.Message,
			Target:  ast.SimpleName(m.Target),
			IsVia:   m.IsVia,
			Scope:   scope,
		}
	case *ast.AssignmentActionNode:
		return Assign{
			Target: ast.SimpleName(m.Target),
			Value:  m.Value,
			Node:   m,
			Scope:  scope,
		}
	case *ast.WhileLoopActionNode:
		return Loop{
			Kind:       m.Kind,
			Condition:  m.Condition,
			Until:      m.Until,
			Variable:   m.Variable.Name,
			Collection: m.Collection,
			Body:       lowerBlock(m, m.Body, childScope(scope, m)),
			Node:       m,
			Scope:      scope,
		}
	case *ast.IfActionNode:
		lowered := If{Condition: m.Condition, Node: m, Scope: scope}
		if m.Then != nil {
			lowered.Then = lowerBlock(m.Then, m.Then.Body, childScope(scope, m.Then))
		}
		if m.Else != nil {
			block := lowerBlock(m.Else, m.Else.Body, childScope(scope, m.Else))
			lowered.Else = &block
		}
		return lowered
	case *ast.ResultMember:
		return Return{Value: m.Expression, Node: m, Scope: scope}
	case *ast.PerformActionNode:
		return Effect{Kind: EffectPerform, Node: m, Scope: scope}
	case *ast.TerminateStatement:
		return Effect{Kind: EffectTerminate, Node: m, Scope: scope}
	case *ast.Usage:
		if stmt, ok := usageStatement(m, scope); ok {
			return stmt
		}
		// An anonymous action usage owning a body is the ActionBodyParameter a loop
		// or branch body is written as (SysML.xtext ActionBodyParameter).
		if m.Kind == ast.UsageAction && m.Ident.Name == "" && len(m.Members) > 0 {
			return lowerBlock(m, m.Members, childScope(scope, m))
		}
		return Unsupported{Description: usageDescription(m), Node: m, Scope: scope}
	default:
		return Unsupported{Description: fmt.Sprintf("%T", member), Node: member, Scope: scope}
	}
}

// lowerBlock lowers the body of a loop or of one branch of a conditional. owner
// is the node the block belongs to, which is the element that owns the block's
// body-local namespace, and scope is the namespace it owns.
func lowerBlock(owner ast.Node, members []ast.Node, scope *symbols.Scope) Block {
	block := Block{Node: owner, Scope: scope}
	for _, member := range members {
		actual := unwrapMembership(member)
		if actual == nil {
			continue
		}
		block.Statements = append(block.Statements, lowerStatement(actual, scope))
	}
	return block
}

// lowerAttributes returns the attribute defaults declared among a behavior's
// members, in order. A redefinition names the attribute it overrides
// (`attribute :>> x = 5;`), so the effective name is the one bound.
func lowerAttributes(members []ast.Node) []Attribute {
	var attrs []Attribute
	for _, member := range members {
		usage, ok := unwrapMembership(member).(*ast.Usage)
		if !ok || usage.Kind != ast.UsageAttribute || usage.Value == nil {
			continue
		}
		name, _ := ast.EffectiveName(usage)
		if name == "" {
			continue
		}
		attrs = append(attrs, Attribute{Name: name, Value: usage.Value, Node: usage})
	}
	return attrs
}

// usageDescription names a usage declared where a statement was expected, for
// the error the executor reports when it reaches it.
func usageDescription(u *ast.Usage) string {
	kind := u.Kind.String()
	if name := getNodeName(u); name != "" {
		return fmt.Sprintf("%s usage %q", kind, name)
	}
	return fmt.Sprintf("anonymous %s usage", kind)
}

// acceptPort returns the port an accept action routes through
// (`action r accept msg : T via p`), which the parser records as a reference
// relationship on the accept action, or "" when it named none.
func acceptPort(node *ast.Usage) string {
	for _, rel := range node.Relationships {
		if rel == nil || rel.Kind != ast.RelVia {
			continue
		}
		if name := ast.SimpleName(rel.Target); name != "" {
			return name
		}
	}
	return ""
}

// subsettingTarget returns the name a usage subsets (`:> e`, `:>> e`), or ""
// when it subsets nothing. For an accept payload that name is the event it
// waits for.
func subsettingTarget(usage *ast.Usage) string {
	for _, rel := range usage.Relationships {
		if rel == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelSubsets, ast.RelRedefines, ast.RelSpecializes, ast.RelReferences:
		default:
			continue
		}
		if name := ast.SimpleName(rel.Target); name != "" {
			return name
		}
	}
	return ""
}

// typingTarget returns the name a usage was typed with (`: T`), or "" when it
// was declared without a type.
func typingTarget(usage *ast.Usage) string {
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Kind != ast.RelTyping {
			continue
		}
		if name := ast.SimpleName(rel.Target); name != "" {
			return name
		}
	}
	return ""
}

// unwrapMembership extracts the actual member from a Membership wrapper.
func unwrapMembership(node ast.Node) ast.Node {
	if membership, ok := node.(*ast.Membership); ok {
		return membership.Member
	}
	return node
}

// findNodeByName looks up a node by its qualified name.
// edgeEndName renders the name an edge end names, for a message about a node
// the body does not declare.
func edgeEndName(qname *ast.QualifiedName) string {
	if qname == nil || len(qname.Parts) == 0 {
		return "an unnamed node"
	}
	var parts []string
	for _, part := range qname.Parts {
		parts = append(parts, part.Text)
	}
	return strconv.Quote(strings.Join(parts, "::"))
}

// edgeEnd names one end of an edge for a message about a node the body does not
// declare: the name the end references, or the kind of the member the notation
// bound to it by position, which has no name to report.
func edgeEnd(qname *ast.QualifiedName, member ast.Node) string {
	if member != nil {
		return "written as " + statementKeyword(member)
	}
	return edgeEndName(qname)
}

// resolveEnd resolves one end of an edge: the node the end names, or the member
// the notation bound to that end by position (SuccessionEdge.SourceMember), which
// is how an action node member with no name of its own is sequenced.
func resolveEnd(nodes []ast.Node, qname *ast.QualifiedName, member ast.Node) ast.Node {
	if member != nil {
		for _, node := range nodes {
			if node == member {
				return node
			}
		}
		return nil
	}
	return findNodeByName(nodes, qname)
}

// sequencedMembers collects the members of a body that an edge binds to one of
// its ends by position, the members a `then` sequences without a name.
func sequencedMembers(members []ast.Node) map[ast.Node]bool {
	sequenced := make(map[ast.Node]bool)
	mark := func(ends ...ast.Node) {
		for _, end := range ends {
			if end != nil {
				sequenced[end] = true
			}
		}
	}
	for _, member := range members {
		switch n := unwrapMembership(member).(type) {
		case *ast.SuccessionEdge:
			mark(n.SourceMember, n.TargetMember)
		case *ast.ControlFlowEdge:
			mark(n.SourceMember, n.TargetMember)
		}
	}
	return sequenced
}

func findNodeByName(nodes []ast.Node, qname *ast.QualifiedName) ast.Node {
	if qname == nil || len(qname.Parts) == 0 {
		return nil
	}

	targetName := qname.Parts[len(qname.Parts)-1].Text
	for _, node := range nodes {
		if nodeAnswersTo(node, targetName) {
			return node
		}
	}
	return nil
}

// nodeAnswersTo reports whether name is one of the keys a node is declared
// under: its effective name or, for a usage, its declared short name. A short
// name is a name of its own, so `action <s> :>> takePhoto;` is reachable as
// both `s` and `takePhoto`.
func nodeAnswersTo(node ast.Node, name string) bool {
	if name == "" {
		return false
	}
	if getNodeName(node) == name {
		return true
	}
	u, ok := node.(*ast.Usage)
	return ok && u.Ident.ShortName == name
}

// getNodeName extracts the name from a node.
func getNodeName(node ast.Node) string {
	switch n := node.(type) {
	case *ast.InitialNode:
		return n.Name
	case *ast.FinalNode:
		return n.Name
	case *ast.ForkNode:
		return n.Name
	case *ast.JoinNode:
		return n.Name
	case *ast.MergeNode:
		return n.Name
	case *ast.DecisionNode:
		return n.Name
	case *ast.ActionExecutionNode:
		return n.Name
	case *ast.Usage:
		// An unnamed usage is named after the feature it references or redefines
		// (`perform increment;` is a node named increment).
		if name, _ := ast.EffectiveName(n); name != "" {
			return name
		}
		return n.Ident.ShortName
	}
	return ""
}

// lowerFlow lowers a flow declared among an action's members
// (`flow generateToAmplify from generateTorque.engineTorque to
// amplifyTorque.engineTorque;`) to the data flow the executor applies when a
// token leaves the source node: the value at the source's pin becomes the value
// at the target's pin. The flow's own name is carried so a diagnostic and the
// REPL can name it.
//
// Both ends must name action nodes of this graph, since a data flow moves a
// value from one node's output to another's input; an end naming anything else
// is reported rather than dropped.
func lowerFlow(nodes []ast.Node, flow *ast.Usage) (ast.Node, ObjectFlow, error) {
	name, _ := ast.EffectiveName(flow)
	sourceNode, sourcePin := flowEnd(nodes, flow.FlowEnds.From)
	targetNode, targetPin := flowEnd(nodes, flow.FlowEnds.To)

	if sourceNode == nil {
		return nil, ObjectFlow{}, fmt.Errorf(
			"flow %s: source %s does not name an action node of this action",
			orAnonymous(name), flowEndText(flow.FlowEnds.From),
		)
	}
	if targetNode == nil {
		return nil, ObjectFlow{}, fmt.Errorf(
			"flow %s: target %s does not name an action node of this action",
			orAnonymous(name), flowEndText(flow.FlowEnds.To),
		)
	}

	// `flow of engineTorque from a to b` names the feature that flows rather
	// than a pin of each end, so it names the pin at both ends.
	if payload := ast.SimpleName(flow.FlowEnds.Payload); payload != "" {
		if sourcePin == "" {
			sourcePin = payload
		}
		if targetPin == "" {
			targetPin = payload
		}
	}

	return sourceNode, ObjectFlow{
		Name:      name,
		SourcePin: sourcePin,
		TargetPin: targetPin,
		Target:    targetNode,
	}, nil
}

// flowEnd resolves one end of a flow to the node it belongs to and the pin it
// names. A chain (`generateTorque.engineTorque`) names a node and its pin; a
// bare name (`generateTorque`) names the node alone, and the pin is whatever
// the flow's payload names.
func flowEnd(nodes []ast.Node, end ast.Node) (ast.Node, string) {
	switch e := end.(type) {
	case *ast.QualifiedName:
		return parsePinReference(nodes, e)
	case *ast.FeatureChainExpr:
		node, _ := flowEnd(nodes, e.Operand)
		return node, ast.SimpleName(e.Member)
	case *ast.FeatureReference:
		return flowEnd(nodes, e.Name)
	}
	return nil, ""
}

// orAnonymous names a declaration that may have been written without a name.
func orAnonymous(name string) string {
	if name == "" {
		return "(anonymous)"
	}
	return name
}

// flowEndText renders one end of a flow for a diagnostic: the chain `a.out` for
// a feature chain, the qualified name for a reference, and a placeholder for an
// end the notation left out, so the message never reads as an empty position.
func flowEndText(end ast.Node) string {
	switch e := end.(type) {
	case *ast.QualifiedName:
		return edgeEndName(e)
	case *ast.FeatureChainExpr:
		base := strings.Trim(flowEndText(e.Operand), `"`)
		return strconv.Quote(base + "." + ast.SimpleName(e.Member))
	case *ast.FeatureReference:
		return edgeEndName(e.Name)
	}
	return "(nothing)"
}

// parsePinReference extracts node and pin name from a qualified reference.
// Format: "nodeName.pinName" or just "nodeName" (pin = "")
func parsePinReference(nodes []ast.Node, qname *ast.QualifiedName) (ast.Node, string) {
	if qname == nil || len(qname.Parts) == 0 {
		return nil, ""
	}

	if len(qname.Parts) == 1 {
		// Just node name
		node := findNodeByName(nodes, qname)
		return node, ""
	}

	// Node.pin format
	nodeName := qname.Parts[0].Text
	pinName := qname.Parts[1].Text

	nodeQname := &ast.QualifiedName{Parts: []ast.NameSegment{{Text: nodeName}}}
	node := findNodeByName(nodes, nodeQname)
	return node, pinName
}

// statementKeyword names a body statement for a diagnostic.
func statementKeyword(node ast.Node) string {
	switch n := node.(type) {
	case *ast.WhileLoopActionNode:
		return "a '" + n.Kind.String() + "' loop"
	case *ast.IfActionNode:
		return "an 'if' conditional"
	case *ast.AssignmentActionNode:
		return "an assignment"
	case *ast.SendStatement:
		return "a 'send'"
	case *ast.TerminateStatement:
		return "a 'terminate'"
	default:
		return fmt.Sprintf("a %T statement", n)
	}
}
