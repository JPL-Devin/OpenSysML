// Package lower provides AST → execution IR lowering.
// Converts declarative members (TransitionMember, EntryMember) to
// operational graphs (nodes + edges) that executors consume.
package lower

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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

	// Edges: source node → successions in declaration order.
	Edges map[ast.Node][]ActionEdge

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

	// StatementRuns marks the nodes of a block's own flow that stand for a run of
	// statements rather than for an action node (block_graph.go). Such a node is
	// keyed by the first statement of the run, whose name names no step.
	StatementRuns map[ast.Node]bool
}

// ActionEdge is one succession out of a node: the target it reaches, the guard
// it carries, and the declaration it was written as.
type ActionEdge struct {
	Target ast.Node
	Guard  ast.Node
	Decl   ast.Node
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
// Target is the name the send addressed, empty for a broadcast. IsVia records
// that the name is a port of the sender rather than a receiver, in which case it
// is the whole path the port was written as and the message goes to whatever the
// graph's Connections join that port to.
type Send struct {
	Message ast.Node
	Target  string
	// TargetPath records that Target is a feature chain (`a.b`) reaching through
	// the sender's features, rather than a name in a namespace (`R`, `P::R`).
	TargetPath bool
	IsVia      bool
	// Receiver is the name addressed by a routed send, empty when omitted.
	Receiver     string
	ReceiverPath bool
	Scope        *symbols.Scope // the scope the statement was declared in
}

func (Send) statement() {}

// Assign is a lowered assignment: `assign <Target> := <Value>`. Target is the
// feature written — the target's last segment — empty when the target names no
// feature.
type Assign struct {
	Target string
	// Chain is the chained target the assignment writes through (`s.reading`),
	// nil when the target was a plain name the body's host binds.
	Chain *AssignTarget
	Value ast.Node
	Node  ast.Node       // the statement itself, for diagnostics
	Scope *symbols.Scope // the scope the statement was declared in
}

func (Assign) statement() {}

// AssignTarget is a chained assignment target: `assign a.b.c := v` walks `b`
// from `a` and writes `c` on the object it reaches.
type AssignTarget struct {
	// Base is the expression the chain starts from, evaluated in the statement's
	// own scope.
	Base ast.Node
	// Steps are the features walked from Base to the object written, in order.
	Steps []string
	// Text is the target as written (`a.b.c`), for diagnostics.
	Text string
}

// assignTarget reports the chained target an assignment states, flattening a
// nested chain into one walk. It reports false for a plain or
// namespace-qualified name, neither of which reaches through an object.
func assignTarget(node ast.Node) (*AssignTarget, string, bool) {
	chain, ok := node.(*ast.FeatureChainExpr)
	if !ok {
		return nil, "", false
	}
	base, segments := flattenChain(chain)
	text := FeaturePath(node)
	if base == nil || len(segments) == 0 || text == "" {
		return nil, "", false
	}
	return &AssignTarget{
		Base:  base,
		Steps: segments[:len(segments)-1],
		Text:  text,
	}, segments[len(segments)-1], true
}

// flattenChain returns the node a feature chain starts from and every feature
// segment walked from it: `a.b.c` is one walk from `a`, not a walk through the
// value of `a.b`. It returns no segments when one of them names nothing.
func flattenChain(chain *ast.FeatureChainExpr) (ast.Node, []string) {
	var segments []string
	for {
		if chain.Member == nil || len(chain.Member.Parts) == 0 {
			return nil, nil
		}
		names := make([]string, 0, len(chain.Member.Parts))
		for _, part := range chain.Member.Parts {
			if part.Text == "" {
				return nil, nil
			}
			names = append(names, part.Text)
		}
		segments = append(names, segments...)
		inner, nested := chain.Operand.(*ast.FeatureChainExpr)
		if !nested {
			return chain.Operand, segments
		}
		chain = inner
	}
}

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
	// Graph is the block's own token flow, present where a member of the block is
	// an action node rather than a statement — a nested action declaration, a
	// `perform` — which only a flow of its own executes with the succession
	// semantics it has (block_graph.go). Statements is empty for such a block: the
	// statements are the bodies of the flow's nodes.
	Graph *ActionGraph
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
	// Scope is the scope the declaration was written in, in which its default
	// resolves; nil where the owner's own scope resolves it.
	Scope *symbols.Scope
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
	// Decl is the declaration the flow was written as, for a consumer that
	// reports where it comes from.
	Decl ast.Node
}

// ToActionGraph converts an action AST (Usage or Definition) to an ActionGraph.
// scope is the scope the action's body was declared in — the scope the action
// itself owns — which every expression the graph carries is evaluated in.
// Returns error if graph is malformed (e.g., no initial node, dangling edges).
func ToActionGraph(actionDecl ast.Node, scope *symbols.Scope) (*ActionGraph, error) {
	graph, members, firstNode, err := collectActionNodes(actionDecl, scope)
	if err != nil {
		return nil, err
	}

	// Note: Initial node is optional at graph construction time.
	// The executor's initialize() will validate and return the error if missing.

	// Second pass: build edges
	for _, member := range members {
		actualMember := unwrapMembership(member)

		switch n := actualMember.(type) {
		case *ast.InitialNode:
			// Handle implicit successor from `first X then Y` syntax
			if n.Successor != nil {
				sourceNode := ast.Node(n)
				if named, ok := firstNode[n]; ok {
					sourceNode = named
				}
				targetNode := resolveActionEndpoint(graph, n.Successor, false)
				if targetNode == nil {
					return nil, fmt.Errorf("initial node %s successor references undefined target %s", n.Name, edgeEndName(n.Successor))
				}
				graph.Edges[sourceNode] = append(graph.Edges[sourceNode], ActionEdge{
					Target: targetNode,
					Guard:  n.Guard,
					Decl:   n,
				})
			}
		case *ast.SuccessionEdge:
			sourceNode := resolveActionEndpointForEdge(graph, n.Source, n.SourceMember, true)
			targetNode := resolveActionEndpointForEdge(graph, n.Target, n.TargetMember, false)

			if sourceNode == nil {
				return nil, fmt.Errorf("succession edge references undefined source node %s", edgeEnd(n.Source, n.SourceMember))
			}
			if targetNode == nil {
				return nil, fmt.Errorf("succession edge references undefined target node %s", edgeEnd(n.Target, n.TargetMember))
			}
			graph.Edges[sourceNode] = append(graph.Edges[sourceNode], ActionEdge{
				Target: targetNode,
				Decl:   n,
			})
		case *ast.ControlFlowEdge:
			sourceNode := resolveActionEndpointForEdge(graph, n.Source, n.SourceMember, true)
			targetNode := resolveActionEndpointForEdge(graph, n.Target, n.TargetMember, false)

			if sourceNode == nil {
				return nil, fmt.Errorf("control flow edge references undefined source %s", edgeEnd(n.Source, n.SourceMember))
			}
			if targetNode == nil {
				return nil, fmt.Errorf("control flow edge references undefined target %s", edgeEnd(n.Target, n.TargetMember))
			}
			graph.Edges[sourceNode] = append(graph.Edges[sourceNode], ActionEdge{
				Target: targetNode,
				Guard:  n.Guard,
				Decl:   n,
			})
		case *ast.TransitionMember:
			sourceNode := resolveActionEndpoint(graph, n.Source, true)
			targetNode := resolveActionEndpoint(graph, n.Target, false)
			if sourceNode == nil {
				return nil, fmt.Errorf("succession references undefined source node %s", edgeEndName(n.Source))
			}
			if targetNode == nil {
				return nil, fmt.Errorf("succession references undefined target node %s", edgeEndName(n.Target))
			}
			graph.Edges[sourceNode] = append(graph.Edges[sourceNode], ActionEdge{
				Target: targetNode,
				Guard:  n.Guard,
				Decl:   n,
			})
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
				Decl:      n,
			})
		case *ast.Usage:
			if n.Kind == ast.UsageSuccession {
				if len(n.ConnectorEnds) != 2 {
					return nil, fmt.Errorf("action succession must have exactly two connector ends, got %d", len(n.ConnectorEnds))
				}
				if n.Multiplicity != nil {
					return nil, fmt.Errorf("action succession has unsupported multiplicity")
				}
				if n.HasBody || len(n.Members) != 0 {
					return nil, fmt.Errorf("action succession has unsupported body")
				}
				for i, end := range n.ConnectorEnds {
					if end.Multiplicity != nil {
						return nil, fmt.Errorf("action succession end %d has unsupported multiplicity", i+1)
					}
				}
				sourceRef := connectorEndReference(n.ConnectorEnds[0])
				targetRef := connectorEndReference(n.ConnectorEnds[1])
				sourceNode := resolveActionEndpoint(graph, sourceRef, true)
				targetNode := resolveActionEndpoint(graph, targetRef, false)
				if sourceNode == nil {
					return nil, fmt.Errorf("action succession references undefined source node %s", successionEndText(sourceRef))
				}
				if targetNode == nil {
					return nil, fmt.Errorf("action succession references undefined target node %s", successionEndText(targetRef))
				}
				graph.Edges[sourceNode] = append(graph.Edges[sourceNode], ActionEdge{
					Target: targetNode,
					Decl:   n,
				})
				continue
			}
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

// resolveFirstNode reinterprets a `first a …;` whose name is a node the body
// declares: a is the flow's first node, so the initial node it parsed as is not
// a node of the graph. Returns the named node each such initial node stands for.
func resolveFirstNode(graph *ActionGraph) (map[*ast.InitialNode]ast.Node, error) {
	initial, ok := graph.Initial.(*ast.InitialNode)
	if !ok || initial.Name == "" {
		return nil, nil
	}
	var named ast.Node
	for _, node := range graph.Nodes {
		if node != ast.Node(initial) && nodeAnswersTo(node, initial.Name) {
			named = node
			break
		}
	}
	if named == nil {
		return nil, nil
	}
	if _, isFinal := named.(*ast.FinalNode); isFinal {
		// A flow cannot start where it ends: naming a final node would retire the
		// token before any succession out of it is taken.
		return nil, fmt.Errorf("first names the final node %s, so the action would end before it started", initial.Name)
	}
	graph.Initial = named
	graph.Nodes = slices.DeleteFunc(graph.Nodes, func(node ast.Node) bool {
		return node == ast.Node(initial)
	})
	return map[*ast.InitialNode]ast.Node{initial: named}, nil
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

// lowerNodeBody records the statements the body of an action node declares, so
// a body the notation admits on a control node or a succession executes when a
// token reaches it rather than being dropped.
func lowerNodeBody(graph *ActionGraph, node ast.Node, members []ast.Node, scope *symbols.Scope) {
	body := childScope(scope, node)
	for _, member := range BodyStatementMembers(members) {
		graph.Bodies[node] = append(graph.Bodies[node], lowerStatement(unwrapMembership(member), body))
	}
}

// BodyStatementMembers returns the members of a node body that state work to
// perform, in declaration order: what a body declares (a parameter, a doc
// comment) is a feature of the node, not a step of the flow through it.
func BodyStatementMembers(members []ast.Node) []ast.Node {
	var stmts []ast.Node
	for _, member := range members {
		switch m := unwrapMembership(member).(type) {
		case *ast.SendStatement, *ast.AssignmentActionNode, *ast.WhileLoopActionNode,
			*ast.IfActionNode, *ast.TerminateStatement:
			stmts = append(stmts, member)
		case *ast.Usage:
			// A declared action is a feature of the node; only one naming the action
			// it performs is a step (`perform a;`).
			if m.Kind == ast.UsageAction && !m.IsBodyParameter && performsAction(m) {
				stmts = append(stmts, member)
			}
		}
	}
	return stmts
}

// lowerStatement lowers one executable body statement, in the scope it was
// written in. Every form it recognizes is lowered losslessly; a form it does not
// becomes Unsupported, so the executor reports it rather than skipping it.
func lowerStatement(member ast.Node, scope *symbols.Scope) Statement {
	switch m := member.(type) {
	case *ast.SendStatement:
		// A target is either a chain through features (`alpha.inPort`) or a name in
		// a namespace (`P::Driver`), which resolve differently. A `via` target names
		// a port of the sender, rendered as connector ends are so the two match.
		target, isPath := SendTarget(m.Target)
		if m.IsVia {
			target, isPath = FeaturePath(m.Target), true
		}
		message := m.Message
		if message == nil {
			message = sendPayload(m)
		}
		if message == nil {
			return Unsupported{
				Description: "a send declaring no message",
				Node:        m,
				Scope:       scope,
			}
		}
		receiver, receiverPath := SendTarget(m.Receiver)
		return Send{
			Message:      message,
			Target:       target,
			TargetPath:   isPath,
			IsVia:        m.IsVia,
			Receiver:     receiver,
			ReceiverPath: receiverPath,
			Scope:        scope,
		}
	case *ast.AssignmentActionNode:
		// A chained target writes a feature of the object its chain reaches, so the
		// whole walk is carried rather than truncated to the last segment.
		if chain, feature, ok := assignTarget(m.Target); ok {
			return Assign{Target: feature, Chain: chain, Value: m.Value, Node: m, Scope: scope}
		}
		// A namespace-qualified target names no object to write on: an assignment
		// writes a feature of its target occurrence (Actions::AssignmentAction).
		if qname := ast.AsQualifiedName(m.Target); qname != nil && len(qname.Parts) > 1 {
			return Unsupported{
				Description: "assignment to a qualified target",
				Node:        m,
				Scope:       scope,
			}
		}
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
	case *ast.PerformActionNode:
		return Effect{Kind: EffectPerform, Node: m, Scope: scope}
	case *ast.TerminateStatement:
		return Effect{Kind: EffectTerminate, Node: m, Scope: scope}
	case *ast.Usage:
		if stmt, ok := usageStatement(m, scope); ok {
			return stmt
		}
		// The ActionBodyParameter a loop or branch body is written as is the block
		// itself, so its members are the statements: a name it declares only scopes
		// them (`loop action charging { … } until charging.done`).
		if m.Kind == ast.UsageAction && m.IsBodyParameter {
			return lowerBlock(m, m.Members, childScope(scope, m))
		}
		// An action usage naming the action it performs is a performed action, which
		// the host executes or rejects as its own purity demands.
		if m.Kind == ast.UsageAction && performsAction(m) {
			return Effect{Kind: EffectPerform, Node: m, Scope: scope}
		}
		return Unsupported{Description: usageDescription(m), Node: m, Scope: scope}
	default:
		return Unsupported{Description: fmt.Sprintf("%T", member), Node: member, Scope: scope}
	}
}

// sendPayload returns the message a send with no argument carries: the value
// its body binds the payload parameter to (`send { in :>> payload = s; }`).
func sendPayload(m *ast.SendStatement) ast.Node {
	for _, member := range m.Members {
		if u, ok := unwrapMembership(member).(*ast.Usage); ok && u.Direction == ast.DirIn && u.Value != nil {
			return u.Value
		}
	}
	return nil
}

// lowerBlock lowers the body of a loop or of one branch of a conditional. owner
// is the node the block belongs to, which is the element that owns the block's
// body-local namespace, and scope is the namespace it owns.
func lowerBlock(owner ast.Node, members []ast.Node, scope *symbols.Scope) Block {
	if blockNeedsFlow(members) {
		return Block{Node: owner, Scope: scope, Graph: lowerBlockFlow(members, scope, false)}
	}
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

// lowerAttributes returns every attribute declared among a behavior's members,
// in order. An unvalued attribute is still owned by the behavior even though it
// supplies no initial value. A redefinition names the attribute it overrides
// (`attribute :>> x = 5;`), so the effective name is the one bound.
func lowerAttributes(members []ast.Node) []Attribute {
	var attrs []Attribute
	for _, member := range members {
		usage, ok := unwrapMembership(member).(*ast.Usage)
		if !ok || usage.Kind != ast.UsageAttribute {
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
// relationship on the accept action, or "" when it named none. The port is the
// whole path it was written as, so a nested one is the port it names.
func acceptPort(node *ast.Usage) string {
	for _, rel := range node.Relationships {
		if rel == nil || rel.Kind != ast.RelVia {
			continue
		}
		if name := FeaturePath(rel.Target); name != "" {
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

// resolveActionEndpointForEdge resolves an edge end, which the notation may have
// bound to a member by position rather than named.
func resolveActionEndpointForEdge(graph *ActionGraph, ref ast.Node, member ast.Node, source bool) ast.Node {
	if member != nil {
		return resolveEnd(graph.Nodes, nil, member)
	}
	return resolveActionEndpoint(graph, ref, source)
}

// resolveActionEndpoint resolves an action edge end, or the implied start/done
// node it names when the body declares no such node.
func resolveActionEndpoint(graph *ActionGraph, ref ast.Node, source bool) ast.Node {
	node := findNodeByReference(graph.Nodes, ref)
	if node != nil {
		return node
	}
	if node = ensureInheritedActionNode(graph, ref); node != nil {
		return node
	}

	name := ast.SimpleName(ref)
	if !impliedMarker(name, source, graph.Initial == nil) {
		return nil
	}
	if source {
		initial := &ast.InitialNode{NodeBase: ast.NodeBase{NodeSpan: ref.Span()}, Name: "start"}
		graph.Initial = initial
		graph.Nodes = append(graph.Nodes, initial)
		return initial
	}
	for _, final := range graph.Finals {
		if getNodeName(final) == "done" {
			return final
		}
	}
	final := &ast.FinalNode{NodeBase: ast.NodeBase{NodeSpan: ref.Span()}}
	graph.Finals = append(graph.Finals, final)
	graph.Nodes = append(graph.Nodes, final)
	return final
}

// findNodeByReference resolves a plain name or a chain to its graph-node root.
// A chain attaches to the node whose body contains that feature.
func findNodeByReference(nodes []ast.Node, ref ast.Node) ast.Node {
	if qname := ast.AsQualifiedName(ref); qname != nil {
		return findNodeByName(nodes, qname)
	}
	chain, ok := ref.(*ast.FeatureChainExpr)
	if !ok {
		return nil
	}
	for {
		operand := chain.Operand
		if nested, ok := operand.(*ast.FeatureChainExpr); ok {
			chain = nested
			continue
		}
		return findNodeByName(nodes, ast.AsQualifiedName(operand))
	}
}

// successionEndText formats an endpoint for an action-succession diagnostic.
func successionEndText(ref ast.Node) string {
	if text := FeaturePath(ref); text != "" {
		return strconv.Quote(text)
	}
	return "an unnamed node"
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
		// The node declares no name of its own: a succession reaches it by the
		// name of the library feature it is, `done`.
		return "done"
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

	// A flow carries the value of a feature, so an end naming a node alone with
	// no payload to name its pin identifies nothing to move.
	if sourcePin == "" || targetPin == "" {
		return nil, ObjectFlow{}, fmt.Errorf(
			"flow %s: names no feature to carry; write `flow of <payload> from %s to %s` or name a pin at each end",
			orAnonymous(name), flowEndText(flow.FlowEnds.From), flowEndText(flow.FlowEnds.To),
		)
	}

	return sourceNode, ObjectFlow{
		Name:      name,
		SourcePin: sourcePin,
		TargetPin: targetPin,
		Target:    targetNode,
		Decl:      flow,
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
	case *ast.PerformActionNode:
		return "a 'perform'"
	case *ast.Usage:
		// A member bound to an edge by position may be a declaration rather than a
		// statement (`then part { … }`), named by its kind.
		return usageDescription(n)
	default:
		return "a member the body declares"
	}
}
