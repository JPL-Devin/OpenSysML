package runtime

import (
	"errors"
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
	// nested queues deliveries to pins of the nodes under a node (`leg.inner.v`) ahead
	// of its next performance, which forwards them to its own nodes.
	nested map[ast.Node][]nestedDelivery
	// ended marks a performance that has completed, so a delivery to a node under it
	// waits for the next performance rather than reaching one that is over.
	ended bool
}

// nestedDelivery is a value bound for a pin of a node under another: path leads to the
// node from the one the delivery waits at.
type nestedDelivery struct {
	path  []ast.Node
	pin   string
	value Value
}

// boundEnd is a binding with an end at a performance's pin, and the performance of the
// node the binding was written at: perf's own for `p.v`, an enclosing one for `leg.inner.v`.
type boundEnd struct {
	lower.PinBinding
	at *actionFrame
}

// pinText renders the bound end as written: the node path and the pin.
func (b boundEnd) pinText() string {
	text := ActionNodeName(b.Node)
	for _, node := range b.Path {
		text += "." + ActionNodeName(node)
	}
	return text + "." + b.Pin
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
	if err := e.takeDeliveries(parent, node, perf); err != nil {
		return nil, err
	}
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
	perf.ended = true
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

// deliver stores a value at a pin of node, a node of flow in f, ahead of its next
// performance, which is how a flow or binding reaches a node not yet running. A path
// leads on to a node under it: the value goes into node's running performance, else
// waits for its next one to forward it.
func (e *ActionExecutor) deliver(f *actionFrame, flow *lower.ActionGraph, node ast.Node, path []ast.Node, pin string, value Value) error {
	if len(path) > 0 {
		if err := e.checkNestedDelivery(flow, node, path, pin, value); err != nil {
			return err
		}
		if sub, performed := f.subactions[node]; performed && !sub.ended {
			return e.deliver(sub, lower.NestedFlow(flow, node, path[0]), path[0], path[1:], pin, value)
		}
		if f.nested == nil {
			f.nested = make(map[ast.Node][]nestedDelivery)
		}
		f.nested[node] = append(f.nested[node], nestedDelivery{path: path, pin: pin, value: value})
		return nil
	}
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

// checkNestedDelivery checks that path leads from node through the flows under it to a
// node declaring pin, so a delivery waiting for a performance is known to have somewhere to go.
func (e *ActionExecutor) checkNestedDelivery(flow *lower.ActionGraph, node ast.Node, path []ast.Node, pin string, value Value) error {
	for _, next := range path {
		flow = lower.NestedFlow(flow, node, next)
		if flow == nil {
			return fmt.Errorf("%w: %s holds no nested action %s", ErrNodePin, nodeDescription(node), ActionNodeName(next))
		}
		node = next
	}
	pins, _, err := e.nodePins(flow, node)
	if err != nil {
		return err
	}
	if _, declared := pins[pin]; !declared {
		return fmt.Errorf("%w: %s declares no %s", ErrNodePin, nodeDescription(node), pin)
	}
	return e.ctx.checkNamedWrite(flow.Scopes[node], nodeDescription(node), pin, value)
}

// takeDeliveries moves the oldest delivery at each pin of node into perf, so that
// performances of one node begun in turn each start with their own inputs, and
// forwards what waits for the nodes under it.
func (e *ActionExecutor) takeDeliveries(f *actionFrame, node ast.Node, perf *actionFrame) error {
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
	nested := f.nested[node]
	delete(f.nested, node)
	for _, d := range nested {
		if err := e.deliver(perf, lower.NestedFlow(perf.flow, node, d.path[0]), d.path[0], d.path[1:], d.pin, d.value); err != nil {
			return err
		}
	}
	return nil
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
	local, holder, ok := enclosingHolder(perf, name)
	switch {
	case !ok:
		return false, nil
	case local != nil:
		local[name] = value
		return true, nil
	default:
		return true, e.setFrameFeature(holder, name, value)
	}
}

// enclosingHolder finds the innermost block-local or performance around perf that
// holds name: the block's locals, else the performance.
func enclosingHolder(perf *actionFrame, name string) (local map[string]Value, holder *actionFrame, ok bool) {
	for f := perf; f != nil; f = f.parent {
		for i := len(f.locals) - 1; i >= 0; i-- {
			if _, ok := f.locals[i][name]; ok {
				return f.locals[i], nil, true
			}
		}
		if f.parent != nil && f.parent.holds(name) {
			return nil, f.parent, true
		}
	}
	return nil, nil, false
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

// bindInputPins seeds the pins a performance reads from the bindings at them, where nothing
// delivered ahead of it holds a value; bindings must agree, and an unvalued undirected end waits.
func (e *ActionExecutor) bindInputPins(perf *actionFrame, activation int64) error {
	bound := make(map[string]boundEnd)
	for _, end := range e.bindingsAt(perf) {
		dir, err := e.boundPin(perf, end)
		if err != nil {
			return err
		}
		if dir == ast.DirOut {
			continue
		}
		earlier, alreadyBound := bound[end.Pin]
		if _, held := perf.data[end.Pin]; held && !alreadyBound {
			continue
		}
		value, err := e.bindingOtherValue(end, activation)
		if err != nil {
			if dir == ast.DirNone && e.unheldEnd(end, err) {
				continue
			}
			return err
		}
		if alreadyBound {
			if held := perf.data[end.Pin]; !valueEqual(held, value) {
				return &BindingConflictError{
					Target:     end.pinText(),
					Left:       bindingEndText(earlier.Other),
					Right:      bindingEndText(end.Other),
					LeftValue:  held,
					RightValue: value,
				}
			}
			continue
		}
		if err := e.setFrameFeature(perf, end.Pin, value); err != nil {
			return err
		}
		bound[end.Pin] = end
	}
	return nil
}

// bindOutputPins carries what a performance's output pins hold to the other ends of the
// bindings at them, and what an undirected pin holds where its other end differs from it.
func (e *ActionExecutor) bindOutputPins(perf *actionFrame) error {
	for _, end := range e.bindingsAt(perf) {
		dir, err := e.boundPin(perf, end)
		if err != nil {
			return err
		}
		value, ok := perf.data[end.Pin]
		switch dir {
		case ast.DirOut, ast.DirInOut:
			if !ok {
				return fmt.Errorf("%w: %s produced no value at %s to bind %s to",
					ErrBindingEnd, perf.describe(), end.Pin, bindingEndText(end.Other))
			}
		case ast.DirNone:
			if !ok {
				continue
			}
			if other, held := e.otherEndHeld(end); held && valueEqual(other, value) {
				continue
			}
		default:
			continue
		}
		if end.OtherNode != nil {
			if err := e.deliver(end.at.parent, end.at.flow, end.OtherNode, end.OtherPath, end.OtherPin, value); err != nil {
				return err
			}
			continue
		}
		name := simpleEndName(end.Other)
		if name == "" {
			return fmt.Errorf("%w: %s is bound to %s, which names no feature to hold its value",
				ErrBindingEnd, end.pinText(), bindingEndText(end.Other))
		}
		written, err := e.assignEnclosing(end.at, name, value)
		if err != nil {
			return err
		}
		if !written {
			return fmt.Errorf("%w: %s is bound to %s, which no enclosing action holds",
				ErrBindingEnd, end.pinText(), name)
		}
	}
	return nil
}

// bindingsAt returns the bindings with an end at perf's pins: those of the flow its node
// is a node of written at it (`p.v`), and those an enclosing flow wrote reaching down to
// it through the performances between (`leg.inner.v`).
func (e *ActionExecutor) bindingsAt(perf *actionFrame) []boundEnd {
	var at []boundEnd
	var path []ast.Node
	for anc := perf; anc.parent != nil; anc = anc.parent {
		for _, binding := range anc.flow.Bindings {
			if binding.Node == anc.node && slices.Equal(binding.Path, path) {
				at = append(at, boundEnd{PinBinding: binding, at: anc})
			}
		}
		path = append([]ast.Node{anc.node}, path...)
	}
	return at
}

// boundPin returns the direction of the pin a binding ends at, which must be a
// feature the performance holds.
func (e *ActionExecutor) boundPin(perf *actionFrame, end boundEnd) (ast.FeatureDirection, error) {
	dir, declared := perf.features[end.Pin]
	if !declared {
		return ast.DirNone, fmt.Errorf("%w: %s names no parameter or attribute of %s",
			ErrBindingEnd, end.pinText(), perf.describe())
	}
	return dir, nil
}

// otherPerformance returns the latest performance of the node the other end of a binding
// addresses, reached from the flow the binding was written in; performed is false where
// that node, or one on the way to it, has not run.
func (e *ActionExecutor) otherPerformance(end boundEnd) (other *actionFrame, performed bool) {
	other, performed = end.at.parent.subactions[end.OtherNode]
	for _, node := range end.OtherPath {
		if !performed {
			return nil, false
		}
		other, performed = other.subactions[node]
	}
	return other, performed
}

// bindingOtherValue reads the value the other end of a binding at a performance's pin
// holds: another node's pin, or an expression over the enclosing performances.
func (e *ActionExecutor) bindingOtherValue(end boundEnd, activation int64) (Value, error) {
	if end.OtherNode != nil {
		other, performed := e.otherPerformance(end)
		if !performed {
			return Value{}, fmt.Errorf("%w: %s is bound to %s, which is read before it runs",
				ErrNodeNotPerformed, end.pinText(), bindingEndText(end.Other))
		}
		value, err := other.pin(end.OtherPin)
		if err != nil {
			return Value{}, fmt.Errorf("%w: bound to %s", err, end.pinText())
		}
		return value, nil
	}
	ec := e.evalContextAround(end.at, end.Scope)
	ec.activation = activation
	value, err := ec.Eval(end.Other)
	if err != nil {
		return Value{}, fmt.Errorf("%w: %s is bound to %s: %v",
			ErrBindingEnd, end.pinText(), bindingEndText(end.Other), err)
	}
	return value, nil
}

// otherEndHeld reads what the other end of a binding holds now, without evaluating
// it: a performed node's pin, or an enclosing feature named outright.
func (e *ActionExecutor) otherEndHeld(end boundEnd) (Value, bool) {
	if end.OtherNode != nil {
		other, performed := e.otherPerformance(end)
		if !performed {
			return Value{}, false
		}
		value, held := other.data[end.OtherPin]
		return value, held
	}
	if name := simpleEndName(end.Other); name != "" {
		return e.evalContextAround(end.at, end.Scope).Lookup(name)
	}
	return Value{}, false
}

// unheldEnd reports whether err, from reading the other end of a binding, says that end
// holds no value yet: an unperformed node's pin or an unvalued enclosing feature.
func (e *ActionExecutor) unheldEnd(end boundEnd, err error) bool {
	var noValue *NoValueError
	if errors.Is(err, ErrNodeNotPerformed) || errors.As(err, &noValue) {
		return true
	}
	name := simpleEndName(end.Other)
	if end.OtherNode != nil || name == "" {
		return false
	}
	_, valued := e.evalContextAround(end.at, end.Scope).Lookup(name)
	_, _, holds := enclosingHolder(end.at, name)
	return !valued && holds
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
