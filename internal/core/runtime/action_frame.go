package runtime

import (
	"fmt"
	"slices"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A nested action usage is a subperformance (`Actions::Action::subactions :> subperformances`):
// each token performs it anew in a frame of its own, with the enclosing performances in lexical reach.

// actionFrame is one performance: the action's own (node nil) or a nested node's.
type actionFrame struct {
	node  ast.Node
	graph *lower.ActionGraph // the flow this performance runs, nil for a leaf node
	// flow is the graph node is a node of: parent's own, or the flow of a block
	// in a body parent runs (lower/block_graph.go). nil for the action's own.
	flow   *lower.ActionGraph
	scope  *symbols.Scope // the namespace the performance's features resolve in
	parent *actionFrame
	// locals are the block-local bindings entered around node in parent's body,
	// outermost first: a loop variable the node's declarations read.
	locals []map[string]Value
	// live counts the tokens still running in this performance's flow, which a
	// fork inside it raises and a join or a retiring token lowers.
	live int
	// inBody marks a flow a body statement runs to completion (runSubflow) rather
	// than a token of the enclosing flow, so its last token retires instead of leaving.
	inBody bool
	// connections are the connectors a send in this flow may route through:
	// those of every flow around it, this one's own included.
	connections []lower.Connection
	// data holds the values the performance's own features hold.
	data map[string]Value
	// features are the parameters and attributes the performance holds, by name.
	features map[string]ast.FeatureDirection
	// result names the parameter a value read of the performance stands for,
	// "" when the action it performs states no result parameter.
	result string
	// began is the activation the performance began in, which orders performances.
	began int64
	// performs is the flow of the action a typed or invoked node performed, whose
	// subactions the node's performance adopted as its own. nil otherwise.
	performs *lower.ActionGraph
	// outputs names the output parameters of that action, which return to the
	// same-named enclosing features once the performance ends.
	outputs []string
	// subactions holds the latest performance of each node of graph, which is
	// what a read of the node's pins by name sees.
	subactions map[ast.Node]*actionFrame
	// pending queues what flows and bindings delivered to a node's pins ahead of
	// its performances, each of which takes the oldest delivery at each pin.
	pending map[ast.Node]map[string][]Value
}

// newRootFrame is the performance of the action itself.
func (e *ActionExecutor) newRootFrame() *actionFrame {
	root := &actionFrame{
		graph:       e.graph,
		scope:       e.graph.Scope,
		connections: e.graph.Connections,
		data:        make(map[string]Value),
		features:    make(map[string]ast.FeatureDirection),
		subactions:  make(map[ast.Node]*actionFrame),
	}
	for _, attr := range e.graph.Attributes {
		root.features[attr.Name] = ast.DirNone
	}
	e.addFeatureDirections(root.features, e.action)
	return root
}

// addFeatureDirections adds the parameters and attributes an action holds, the
// inherited ones included, to features by name.
func (e *ActionExecutor) addFeatureDirections(features map[string]ast.FeatureDirection, action *symbols.Symbol) {
	if action == nil {
		return
	}
	for _, member := range e.ctx.model.MembersOf(action) {
		usage, ok := member.Decl.(*ast.Usage)
		if !ok || !lower.DeclaresNodeFeature(usage) {
			continue
		}
		name, _ := ast.EffectiveName(usage)
		if name == "" {
			name = member.Name
		}
		if name == "" {
			continue
		}
		if _, held := features[name]; !held {
			features[name] = usage.Direction
		}
	}
}

// beginPerformance starts a performance of node (of parent's flow, or a block's with locals bound),
// seeding its pins from deliveries, then the arguments it passes its callee, then input bindings,
// then its own declared defaults.
func (e *ActionExecutor) beginPerformance(
	parent *actionFrame, flow *lower.ActionGraph, node ast.Node, locals []map[string]Value,
) (*actionFrame, error) {
	perf := &actionFrame{
		node:        node,
		flow:        flow,
		scope:       flow.Scopes[node],
		parent:      parent,
		locals:      locals,
		connections: parent.connections,
		data:        make(map[string]Value),
		features:    make(map[string]ast.FeatureDirection),
	}
	if perf.scope == nil {
		perf.scope = parent.scope
	}
	if sub, owns := e.subflowOf(flow, node); owns {
		perf.graph = sub.Graph
		perf.live = 1
		perf.connections = joinConnections(parent.connections, sub.Graph.Connections)
		perf.subactions = make(map[ast.Node]*actionFrame)
	}
	pins, result, err := e.nodePins(flow, node)
	if err != nil {
		return nil, err
	}
	perf.features = pins
	perf.result = result
	parent.takeDeliveries(node, perf)
	if parent.subactions == nil {
		parent.subactions = make(map[ast.Node]*actionFrame)
	}
	parent.subactions[node] = perf

	// The arguments, bindings and defaults are one evaluation: a calc usage two of them
	// read answers once, and another performance evaluates it anew.
	activation, endStep := e.ctx.beginStep()
	defer endStep()
	perf.began = activation
	if err := e.bindArguments(perf, activation); err != nil {
		return nil, err
	}
	if err := e.bindInputPins(perf, activation); err != nil {
		return nil, err
	}
	if err := e.seedDeclaredValues(perf, flow.Features[node], activation); err != nil {
		return nil, err
	}
	return perf, nil
}

// bindArguments writes the arguments a node passes its callee (`F(a = 3)`) to its pins,
// evaluated in the caller's context: they stand over deliveries, bindings and defaults.
func (e *ActionExecutor) bindArguments(perf *actionFrame, activation int64) error {
	usage, ok := perf.node.(*ast.Usage)
	if !ok {
		return nil
	}
	inv, performs := nestedInvocation(usage)
	if !performs || !inv.invoked {
		return nil
	}
	scope := nodeScope(perf.flow, perf.node)
	ec := e.evalContextAround(perf, scope)
	ec.activation = activation
	arguments, err := invocationArguments(e.ctx, scope, inv, ec)
	if err != nil {
		return err
	}
	for name, value := range arguments {
		if err := e.setFrameFeature(perf, name, value); err != nil {
			return err
		}
	}
	return nil
}

// seedDeclaredValues evaluates the values a node's own declarations give its
// features (`in a = 3;`), where no delivery, argument or binding at the pin holds one yet.
func (e *ActionExecutor) seedDeclaredValues(perf *actionFrame, features []lower.Feature, activation int64) error {
	for _, feature := range features {
		if feature.Value == nil {
			continue
		}
		if _, held := perf.data[feature.Name]; held {
			continue
		}
		ec := e.evalContextFor(perf, feature.Scope)
		ec.activation = activation
		value, err := ec.Eval(feature.Value)
		if err != nil {
			return fmt.Errorf("eval %s of %s: %w", feature.Name, nodeDescription(perf.node), err)
		}
		if err := e.ctx.checkNamedWrite(feature.Scope, perf.describe(), feature.Name, value); err != nil {
			return err
		}
		perf.data[feature.Name] = value
	}
	return nil
}

// endPerformance completes a performance: the outputs of the action it performed
// return to same-named enclosing features, and the bindings at its output pins
// carry what it produced to their other ends.
func (e *ActionExecutor) endPerformance(perf *actionFrame) error {
	for _, name := range perf.outputs {
		value, ok := perf.data[name]
		if !ok {
			continue
		}
		if _, err := e.assignEnclosing(perf, name, value); err != nil {
			return err
		}
	}
	return e.bindOutputPins(perf)
}

// nodePins returns the pins a performance of node holds (its own and its action's),
// and the pin read when the node is read as a value: `return`, else `out result`.
func (e *ActionExecutor) nodePins(graph *lower.ActionGraph, node ast.Node) (map[string]ast.FeatureDirection, string, error) {
	pins := make(map[string]ast.FeatureDirection)
	usage, ok := node.(*ast.Usage)
	if !ok {
		return pins, "", nil
	}
	result := ""
	for _, feature := range graph.Features[node] {
		pins[feature.Name] = feature.Direction
		if feature.IsResult {
			result = feature.Name
		}
	}
	if inv, performs := nestedInvocation(usage); performs {
		sym, err := resolveActionSymbol(e.ctx, nodeScope(graph, node), inv)
		if err != nil {
			return nil, "", err
		}
		e.addFeatureDirections(pins, e.ctx.actionBodySymbol(sym))
		for _, param := range e.ctx.actionParametersOf(sym) {
			if param.IsResult && result == "" {
				result = param.Name
			}
		}
	}
	if result == "" {
		if dir, ok := pins["result"]; ok && (dir == ast.DirOut || dir == ast.DirInOut) {
			result = "result"
		}
	}
	return pins, result, nil
}

// nodeScope is the namespace a node's declaration resolves in: its own where the
// flow retains it (an inherited node keeps its declaring action's), else the flow's.
func nodeScope(graph *lower.ActionGraph, node ast.Node) *symbols.Scope {
	if scope := graph.Scopes[node]; scope != nil {
		return scope
	}
	return graph.Scope
}

// describe names the performance for a diagnostic.
func (f *actionFrame) describe() string {
	if f.node == nil {
		return "action"
	}
	return "action node " + ActionNodeName(f.node)
}

// path is the dotted path naming the performance from the action's own, "" for it.
func (f *actionFrame) path() string {
	if f.node == nil || f.parent == nil {
		return ""
	}
	name := ActionNodeName(f.node)
	if prefix := f.parent.path(); prefix != "" {
		return prefix + "." + name
	}
	return name
}

// declares reports whether the performance holds a feature of this name.
func (f *actionFrame) declares(name string) bool {
	_, ok := f.features[name]
	return ok
}

// holds reports whether the performance holds name: a feature it declares, or a
// value its body stored.
func (f *actionFrame) holds(name string) bool {
	if f.declares(name) {
		return true
	}
	_, ok := f.data[name]
	return ok
}

// lexicalFrames returns the frames a body of f reads, outermost first: each enclosing
// performance followed by the block-locals around its node, and last f's own.
func (f *actionFrame) lexicalFrames() []frame {
	var frames []frame
	if f.parent != nil {
		frames = f.parent.lexicalFrames()
	}
	for _, local := range f.locals {
		frames = append(frames, mapFrame(local))
	}
	return append(frames, performanceFrame(f))
}

// nodesNamed returns the nodes named name among the performance's subactions: its
// flow's nodes, those its bodies' blocks declare, and those of the action it performed.
func (f *actionFrame) nodesNamed(name string) []ast.Node {
	var named []ast.Node
	add := func(node ast.Node) {
		if _, isUsage := node.(*ast.Usage); isUsage && slices.Contains(ActionNodeNames(node), name) {
			named = append(named, node)
		}
	}
	inFlow := func(graph *lower.ActionGraph) {
		for _, node := range graph.Nodes {
			add(node)
		}
		for _, node := range graph.Nodes {
			if _, isUsage := node.(*ast.Usage); isUsage {
				continue
			}
			for _, declared := range graph.BlockNodes[node] {
				add(declared)
			}
		}
	}
	if f.graph != nil {
		inFlow(f.graph)
	} else {
		for _, node := range f.flow.BlockNodes[f.node] {
			add(node)
		}
	}
	if f.performs != nil {
		inFlow(f.performs)
	}
	return named
}

// subaction returns the latest performance of f's node named name; declared reports
// whether f has such a node, one not yet performed is ErrNodeNotPerformed. Among
// same-named nodes (one per branch of a conditional), decl names the reader's own,
// else the one performed latest answers.
func (f *actionFrame) subaction(name string, decl ast.Node) (perf *actionFrame, declared bool, err error) {
	named := f.nodesNamed(name)
	if len(named) == 0 {
		return nil, false, nil
	}
	node := named[0]
	if slices.Contains(named, decl) {
		node = decl
	} else {
		for _, candidate := range named {
			performed, ok := f.subactions[candidate]
			if ok && (f.subactions[node] == nil || f.subactions[node].began < performed.began) {
				node = candidate
			}
		}
	}
	perf, performed := f.subactions[node]
	if !performed {
		return nil, true, fmt.Errorf("%w: action node %s has not been performed yet",
			ErrNodeNotPerformed, ActionNodeName(node))
	}
	return perf, true, nil
}

// pin reads the value the performance's pin holds.
func (f *actionFrame) pin(name string) (Value, error) {
	value, ok := f.data[name]
	if ok {
		return value, nil
	}
	if f.declares(name) {
		return Value{}, &NoValueError{Feature: f.path() + "." + name}
	}
	return Value{}, fmt.Errorf("%w: %s declares no %s", ErrNodePin, f.describe(), name)
}

// resultValue is what the performance stands for when read as a value: the
// result parameter of the action it performs.
func (f *actionFrame) resultValue() (Value, error) {
	if f.result == "" {
		return Value{}, fmt.Errorf("%w: %s declares no result to read it as a value by",
			ErrNodePin, f.describe())
	}
	if value, ok := f.data[f.result]; ok {
		return value, nil
	}
	return Value{}, &NoValueError{Feature: f.path() + "." + f.result}
}

// deliver stores a value at a pin of node ahead of its next performance, which is
// how a flow or binding reaches a node not yet running.
func (e *ActionExecutor) deliver(f *actionFrame, flow *lower.ActionGraph, node ast.Node, pin string, value Value) error {
	pins, _, err := e.nodePins(flow, node)
	if err != nil {
		return err
	}
	if _, declared := pins[pin]; !declared {
		return fmt.Errorf("%w: %s declares no %s", ErrNodePin, nodeDescription(node), pin)
	}
	if err := e.ctx.checkNamedWrite(flow.Scopes[node], nodeDescription(node), pin, value); err != nil {
		return err
	}
	if f.pending == nil {
		f.pending = make(map[ast.Node]map[string][]Value)
	}
	if f.pending[node] == nil {
		f.pending[node] = make(map[string][]Value)
	}
	f.pending[node][pin] = append(f.pending[node][pin], value)
	return nil
}

// takeDeliveries moves the oldest delivery at each pin of node into perf, so
// that performances of one node begun in turn each start with their own inputs.
func (f *actionFrame) takeDeliveries(node ast.Node, perf *actionFrame) {
	queues := f.pending[node]
	for pin, values := range queues {
		perf.data[pin] = values[0]
		if len(values) == 1 {
			delete(queues, pin)
		} else {
			queues[pin] = values[1:]
		}
	}
	if len(queues) == 0 {
		delete(f.pending, node)
	}
}

// setFrameFeature writes a feature the performance holds, through the action's
// performance occurrence for the action's own features.
func (e *ActionExecutor) setFrameFeature(f *actionFrame, name string, value Value) error {
	if f == e.root {
		return e.setFeature(name, value)
	}
	if err := e.ctx.checkNamedWrite(f.scope, f.describe(), name, value); err != nil {
		return err
	}
	f.data[name] = value
	return nil
}

// assignEnclosing writes name to the innermost block-local or performance feature
// around perf that holds it, reporting whether one did.
func (e *ActionExecutor) assignEnclosing(perf *actionFrame, name string, value Value) (bool, error) {
	for f := perf; f != nil; f = f.parent {
		for i := len(f.locals) - 1; i >= 0; i-- {
			if _, ok := f.locals[i][name]; ok {
				f.locals[i][name] = value
				return true, nil
			}
		}
		if f.parent != nil && f.parent.holds(name) {
			return true, e.setFrameFeature(f.parent, name, value)
		}
	}
	return false, nil
}

// lookupEnclosing reads name from the innermost binding around perf that holds
// a value for it.
func lookupEnclosing(perf *actionFrame, name string) (Value, bool) {
	for f := perf; f != nil; f = f.parent {
		for i := len(f.locals) - 1; i >= 0; i-- {
			if value, ok := f.locals[i][name]; ok {
				return value, true
			}
		}
		if f.parent != nil {
			if value, ok := f.parent.data[name]; ok {
				return value, true
			}
		}
	}
	return Value{}, false
}

// evalContextFor returns a context evaluating in scope with the performance and
// every frame around it in reach, innermost last.
func (e *ActionExecutor) evalContextFor(perf *actionFrame, scope *symbols.Scope) *EvalContext {
	ec := NewEvalContextIn(e.ctx, scope, e.self)
	for _, f := range perf.lexicalFrames() {
		ec.pushFrame(f)
	}
	return ec
}

// evalContextAround returns a context evaluating in scope what is written at perf's
// node: the enclosing performances and the block-locals around the node, not perf's own.
func (e *ActionExecutor) evalContextAround(perf *actionFrame, scope *symbols.Scope) *EvalContext {
	ec := NewEvalContextIn(e.ctx, scope, e.self)
	if perf.parent != nil {
		for _, f := range perf.parent.lexicalFrames() {
			ec.pushFrame(f)
		}
	}
	for _, local := range perf.locals {
		ec.pushFrame(mapFrame(local))
	}
	return ec
}

// lexicalValues merges the values a performance and the frames around it hold,
// the innermost winning, for a caller reading them as one map.
func lexicalValues(perf *actionFrame) map[string]Value {
	merged := make(map[string]Value)
	for _, f := range perf.lexicalFrames() {
		for name, value := range f.vars {
			merged[name] = value
		}
	}
	return merged
}

// collect reports the values the performance and its subactions hold, a node's
// under its path (`p.v`), the latest performance of each name standing for it.
func (f *actionFrame) collect(prefix string, into map[string]Value) {
	for name, value := range f.data {
		into[prefix+name] = value
	}
	latest := make(map[string]*actionFrame)
	for node, sub := range f.subactions {
		name := ActionNodeName(node)
		if name == "" {
			continue
		}
		if earlier, named := latest[name]; !named || earlier.began < sub.began {
			latest[name] = sub
		}
	}
	for name, sub := range latest {
		sub.collect(prefix+name+".", into)
	}
}

// bindInputPins seeds the pins a performance reads from the bindings at them, where
// nothing delivered ahead of it holds a value; bindings at one pin must agree.
func (e *ActionExecutor) bindInputPins(perf *actionFrame, activation int64) error {
	bound := make(map[string]lower.PinBinding)
	for _, binding := range e.bindingsAt(perf) {
		dir, err := e.boundPin(perf, binding)
		if err != nil {
			return err
		}
		if dir == ast.DirOut {
			continue
		}
		earlier, alreadyBound := bound[binding.Pin]
		if _, held := perf.data[binding.Pin]; held && !alreadyBound {
			continue
		}
		value, err := e.bindingOtherValue(perf, binding, activation)
		if err != nil {
			return err
		}
		if alreadyBound {
			if held := perf.data[binding.Pin]; !valueEqual(held, value) {
				return &BindingConflictError{
					Target:     ActionNodeName(perf.node) + "." + binding.Pin,
					Left:       bindingEndText(earlier.Other),
					Right:      bindingEndText(binding.Other),
					LeftValue:  held,
					RightValue: value,
				}
			}
			continue
		}
		if err := e.setFrameFeature(perf, binding.Pin, value); err != nil {
			return err
		}
		bound[binding.Pin] = binding
	}
	return nil
}

// bindOutputPins carries what a performance's output pins hold to the other
// ends of the bindings at them.
func (e *ActionExecutor) bindOutputPins(perf *actionFrame) error {
	for _, binding := range e.bindingsAt(perf) {
		dir, err := e.boundPin(perf, binding)
		if err != nil {
			return err
		}
		if dir != ast.DirOut && dir != ast.DirInOut {
			continue
		}
		value, ok := perf.data[binding.Pin]
		if !ok {
			return fmt.Errorf("%w: %s produced no value at %s to bind %s to",
				ErrBindingEnd, perf.describe(), binding.Pin, bindingEndText(binding.Other))
		}
		if binding.OtherNode != nil {
			if err := e.deliver(perf.parent, perf.flow, binding.OtherNode, binding.OtherPin, value); err != nil {
				return err
			}
			continue
		}
		name := simpleEndName(binding.Other)
		if name == "" {
			return fmt.Errorf("%w: %s.%s is bound to %s, which names no feature to hold its value",
				ErrBindingEnd, ActionNodeName(perf.node), binding.Pin, bindingEndText(binding.Other))
		}
		written, err := e.assignEnclosing(perf, name, value)
		if err != nil {
			return err
		}
		if !written {
			return fmt.Errorf("%w: %s.%s is bound to %s, which no enclosing action holds",
				ErrBindingEnd, ActionNodeName(perf.node), binding.Pin, name)
		}
	}
	return nil
}

// bindingsAt returns the bindings of the flow perf's node is a node of with an
// end at it.
func (e *ActionExecutor) bindingsAt(perf *actionFrame) []lower.PinBinding {
	var at []lower.PinBinding
	for _, binding := range perf.flow.Bindings {
		if binding.Node == perf.node {
			at = append(at, binding)
		}
	}
	return at
}

// boundPin returns the direction of the pin a binding ends at, which must be a
// feature the performance holds.
func (e *ActionExecutor) boundPin(perf *actionFrame, binding lower.PinBinding) (ast.FeatureDirection, error) {
	dir, declared := perf.features[binding.Pin]
	if !declared {
		return ast.DirNone, fmt.Errorf("%w: %s.%s names no parameter or attribute of %s",
			ErrBindingEnd, ActionNodeName(perf.node), binding.Pin, perf.describe())
	}
	return dir, nil
}

// bindingOtherValue reads the value the other end of a binding at perf's pin
// holds: another node's pin, or an expression over the enclosing performances.
func (e *ActionExecutor) bindingOtherValue(perf *actionFrame, binding lower.PinBinding, activation int64) (Value, error) {
	if binding.OtherNode != nil {
		other, performed := perf.parent.subactions[binding.OtherNode]
		if !performed {
			return Value{}, fmt.Errorf("%w: %s.%s is bound to %s, which is read before it runs",
				ErrNodeNotPerformed, ActionNodeName(perf.node), binding.Pin, bindingEndText(binding.Other))
		}
		value, err := other.pin(binding.OtherPin)
		if err != nil {
			return Value{}, fmt.Errorf("%w: bound to %s.%s", err, ActionNodeName(perf.node), binding.Pin)
		}
		return value, nil
	}
	ec := e.evalContextAround(perf, binding.Scope)
	ec.activation = activation
	value, err := ec.Eval(binding.Other)
	if err != nil {
		return Value{}, fmt.Errorf("%w: %s.%s is bound to %s: %v",
			ErrBindingEnd, ActionNodeName(perf.node), binding.Pin, bindingEndText(binding.Other), err)
	}
	return value, nil
}

// simpleEndName returns the name a binding end written as one name states, ""
// for any other expression.
func simpleEndName(end ast.Node) string {
	switch n := end.(type) {
	case *ast.FeatureReference:
		return simpleEndName(n.Name)
	case *ast.QualifiedName:
		if len(n.Parts) == 1 {
			return n.Parts[0].Text
		}
	}
	return ""
}

// bindingEndText renders a binding end for a diagnostic.
func bindingEndText(end ast.Node) string {
	switch n := end.(type) {
	case *ast.FeatureReference:
		return bindingEndText(n.Name)
	case *ast.QualifiedName:
		return qualifiedNameText(n)
	case *ast.FeatureChainExpr:
		return bindingEndText(n.Operand) + "." + ast.SimpleName(n.Member)
	}
	return fmt.Sprintf("%T", end)
}

// performInvocation performs the action a node names as a subperformance of perf: the node's
// pins (its arguments among them) bind the callee's inputs, its final values become the node's,
// and its outputs return to enclosing features when the node's own performance ends.
func (e *ActionExecutor) performInvocation(perf *actionFrame, inv actionInvocation) error {
	scope := nodeScope(perf.flow, perf.node)
	sym, err := resolveActionSymbol(e.ctx, scope, inv)
	if err != nil {
		return err
	}
	if e.ctx.actionDepth >= maxActionNestingDepth {
		return fmt.Errorf(
			"action invocation nested more than %d deep at %s (recursive action?)",
			maxActionNestingDepth, qualifiedNameText(inv.target),
		)
	}
	e.ctx.actionDepth++
	defer func() { e.ctx.actionDepth-- }()

	params := e.ctx.actionParametersOf(sym)
	in, out := parameterNames(params)
	inputs := make(map[string]Value, len(in))
	for _, name := range in {
		if value, ok := perf.data[name]; ok {
			inputs[name] = value
		}
	}
	if !inv.invoked {
		for _, name := range in {
			if _, bound := inputs[name]; bound {
				continue
			}
			if value, ok := lookupEnclosing(perf, name); ok {
				inputs[name] = value
			}
		}
	}
	if err := checkInputsBound(inv, params, inputs); err != nil {
		return err
	}

	callee, err := e.ctx.performAction(sym, e.self, inputs)
	if err != nil {
		return fmt.Errorf("invoke action %s: %w", qualifiedNameText(inv.target), err)
	}
	perf.adopt(callee)
	sort.Strings(out)
	perf.outputs = out
	return nil
}

// adopt makes the completed performance of the action a node performed the node's
// own: its features' values, and its subactions, read as `call.inner.v`.
func (f *actionFrame) adopt(callee *ActionExecutor) {
	for name, value := range callee.root.data {
		f.data[name] = value
	}
	f.performs = callee.graph
	if len(callee.root.subactions) > 0 && f.subactions == nil {
		f.subactions = make(map[ast.Node]*actionFrame, len(callee.root.subactions))
	}
	for node, sub := range callee.root.subactions {
		sub.parent = f
		f.subactions[node] = sub
	}
}

// checkInputsBound reports an input parameter that no argument, pin value or
// default binds, before the callee runs rather than when its body reads it.
func checkInputsBound(inv actionInvocation, params []actionParameter, inputs map[string]Value) error {
	for _, param := range params {
		if param.Direction != ast.DirIn && param.Direction != ast.DirInOut {
			continue
		}
		if _, bound := inputs[param.Name]; bound || param.HasDefault {
			continue
		}
		return fmt.Errorf("%w: action %s: input parameter %s is bound by no argument",
			ErrUnboundParameter, qualifiedNameText(inv.target), param.Name)
	}
	return nil
}

// performanceFrame is the frame an evaluation reads a performance's values
// through, which also answers for the nodes of its flow.
func performanceFrame(f *actionFrame) frame {
	return frame{vars: f.data, perf: f}
}
