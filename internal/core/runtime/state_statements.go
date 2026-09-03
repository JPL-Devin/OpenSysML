package runtime

import (
	"fmt"
	"maps"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// stateStmtHost runs the statements of a state machine behavior written as an
// inline action body: an assignment to a name the body does not declare reaches
// the machine's state data, and a send posts through the machine's connections.
type stateStmtHost struct {
	exec     *StateExecutor
	behavior lower.StateBehavior
}

// executeBehavior runs one behavior of a state or transition: the statements it
// was lowered into, whichever form it was written in.
func (e *StateExecutor) executeBehavior(behavior lower.StateBehavior) error {
	if len(behavior.Body) == 0 {
		return nil
	}
	host := &stateStmtHost{exec: e, behavior: behavior}
	engine := newStmtEngineOver(e.ctx, host, e.stateData, e.attrFramesFor(behavior.Owner))
	// The activation ends with this execution of the body, so a behavior run
	// again does not read what an earlier execution computed.
	defer engine.finish()
	_, err := engine.run(behavior.Body)
	return err
}

func (h *stateStmtHost) describe() string {
	if h.behavior.Name != "" {
		return "state behavior " + h.behavior.Name
	}
	if usage, ok := h.behavior.Node.(*ast.Usage); ok {
		return "state behavior " + stateActionName(usage)
	}
	return "anonymous state behavior"
}

func (h *stateStmtHost) send(ec *EvalContext, s lower.Send) error {
	msg, err := ec.buildMessage(h.exec.stateMachine.Scope, s)
	if err != nil {
		return err
	}
	return h.exec.ctx.post(h.exec.graph.Connections, msg, s, h.exec.self)
}

// assignOuter writes a name the machine does not declare to the object
// exhibiting it, falling back to executor-local data.
func (h *stateStmtHost) assignOuter(env *stmtEnv, name string, value Value, s lower.Assign) error {
	if written, err := h.assignStateAttribute(name, value); written || err != nil {
		return err
	}
	if h.exec.declaresAttribute(name) {
		return h.exec.assignAttribute(name, value)
	}
	if written, err := assignPerformerFeature(h.exec.ctx, h.exec.self, s.Scope, name, value); written || err != nil {
		return err
	}
	return storeBodyValue(h.exec.ctx, h, env, name, value, s)
}

func (h *stateStmtHost) assignData(env *stmtEnv, name string, value Value, s lower.Assign) error {
	if written, err := h.assignStateAttribute(name, value); written || err != nil {
		return err
	}
	if h.exec.declaresAttribute(name) {
		return h.exec.assignAttribute(name, value)
	}
	return storeBodyValue(h.exec.ctx, h, env, name, value, s)
}

// assignChain writes the feature a chained target names on the object its chain
// reaches, which is the machine's state data for no chained target.
func (h *stateStmtHost) assignChain(ec *EvalContext, s lower.Assign, value Value) error {
	return assignThroughChain(ec, h.describe(), s, value)
}

// assignStateAttribute writes an attribute owned by the state running this
// behavior, or by one enclosing it, and reports whether it did. The value
// answers to the attribute's declaration as every other write does.
func (h *stateStmtHost) assignStateAttribute(name string, value Value) (bool, error) {
	data, scope, ok := h.exec.stateAttributeValues(h.behavior.Owner, name)
	if !ok {
		return false, nil
	}
	if err := h.exec.ctx.checkNamedWrite(scope, h.describe(), name, value); err != nil {
		return true, err
	}
	data[name] = value
	return true, nil
}

// performer is the object exhibiting the machine this behavior belongs to.
func (h *stateStmtHost) performer() *Instance {
	return h.exec.self
}

// acceptReturn rejects a `return`: a state behavior computes no result.
func (h *stateStmtHost) acceptReturn(Value, lower.Return) error {
	return fmt.Errorf("%w: %s", ErrReturnOutsideCalc, h.describe())
}

// effect performs the action a `perform` names; every other effect a body may
// state has no execution in a state behavior.
func (h *stateStmtHost) effect(s lower.Effect) error {
	if s.Kind == lower.EffectPerform {
		inv, ok := performedInvocation(s.Node)
		if !ok {
			return fmt.Errorf("%s performs no action", h.describe())
		}
		return h.exec.invokeNested(inv)
	}
	return fmt.Errorf("%s: '%s' in a body is not executable", h.describe(), s.Kind)
}

// performNode runs a nested action of a block in a frame of the body; a typed one
// performs its callee, whose final features become the node's pins and whose
// outputs return outward once the node's own body has run.
func (h *stateStmtHost) performNode(engine *stmtEngine, graph *lower.ActionGraph, node *ast.Usage) (stmtFlow, error) {
	inv, performs := nestedInvocation(node)
	if !performs {
		_, flow, err := engine.nodeInBlock(graph, node, nil)
		return flow, err
	}
	caller := engine.env.values()
	var outputs []string
	frame, flow, err := engine.nodeInBlock(graph, node, func(pins map[string]Value) (map[string]Value, error) {
		features, out, err := invokeAction(h.exec.ctx, graph.Scope, inv, pins, caller, h.exec.self)
		if err != nil {
			return nil, err
		}
		outputs = slices.Sorted(maps.Keys(out))
		return features, nil
	})
	if err != nil {
		return flow, err
	}
	for _, name := range outputs {
		if value, ok := frame[name]; ok {
			if err := h.returnOutput(engine.env, name, value); err != nil {
				return flow, err
			}
		}
	}
	return flow, nil
}

// returnOutput writes an output a node's performance produced to the innermost
// binding around the node that holds its name, if one does.
func (h *stateStmtHost) returnOutput(env *stmtEnv, name string, value Value) error {
	if env.assignLocal(name, value) {
		return nil
	}
	if written, err := h.assignStateAttribute(name, value); written || err != nil {
		return err
	}
	if h.exec.declaresAttribute(name) {
		return h.exec.assignAttribute(name, value)
	}
	if env.data.has(name) {
		env.data.set(name, value)
	}
	return nil
}

// performedInvocation reports the action a `perform` statement names, in either
// form the parser produces for one.
func performedInvocation(node ast.Node) (actionInvocation, bool) {
	switch n := node.(type) {
	case *ast.PerformActionNode:
		switch ref := n.ActionRef.(type) {
		case *ast.QualifiedName:
			return actionInvocation{target: ref}, true
		case *ast.InvocationExpr:
			if ref.Type != nil {
				return actionInvocation{target: ref.Type, invoked: true, args: ref.Args, named: ref.NamedArgs}, true
			}
		}
	case *ast.Usage:
		return nestedInvocation(n)
	case *ast.ActionExecutionNode:
		if n.ActionRef != nil {
			return actionInvocation{target: n.ActionRef}, true
		}
	}
	return actionInvocation{}, false
}

// declaredOutput reports no output features: a state behavior computes none.
func (h *stateStmtHost) declaredOutput(string) bool {
	return false
}
