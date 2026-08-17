package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lower"
)

// actionStmtHost runs an action node's body statements: a send posts through
// the action graph's connections, and an assignment to a name the body does not
// declare reaches the action's own features, which is how a loop body updates
// an attribute of the action it runs in.
type actionStmtHost struct {
	exec *ActionExecutor
	node ast.Node // the action node whose body is running, for diagnostics
	// engine runs the body, and holds the values a `perform` in it reads and
	// writes: the action's own, and those of every block entered around it.
	engine *stmtEngine
}

// executeBody runs the lowered statements of the given action node against the
// action's feature space.
func (e *ActionExecutor) executeBody(node ast.Node) error {
	host := &actionStmtHost{exec: e, node: node}
	engine := newStmtEngine(e.ctx, host, e.data)
	host.engine = engine
	// The body's activation ends with this execution of it, so a run stepping the
	// node many times does not hold what every execution computed.
	defer engine.finish()
	_, err := engine.run(e.graph.Bodies[node])
	return err
}

func (h *actionStmtHost) describe() string {
	return "action node " + ActionNodeName(h.node)
}

func (h *actionStmtHost) send(ec *EvalContext, s lower.Send) error {
	msg, err := ec.buildMessage(h.exec.action.Scope, s)
	if err != nil {
		return err
	}
	return h.exec.ctx.post(h.exec.graph.Connections, msg, s, h.exec.self)
}

// assignOuter writes a name the body's blocks do not declare to the action's
// features, which every one of its tokens shares.
func (h *actionStmtHost) assignOuter(env *stmtEnv, name string, value Value, _ lower.Assign) error {
	env.data[name] = value
	return nil
}

// acceptReturn rejects a `return`: an action node computes no result to return.
func (h *actionStmtHost) acceptReturn(Value, lower.Return) error {
	return fmt.Errorf("%w: %s", ErrReturnOutsideCalc, h.describe())
}

// effect performs the action a `perform` names, as a node of the flow the block
// holding it states (lower/block_graph.go); any other effect is reported.
func (h *actionStmtHost) effect(s lower.Effect) error {
	if s.Kind != lower.EffectPerform {
		return fmt.Errorf("%s: '%s' in a body is not executable", h.describe(), s.Kind)
	}
	inv, ok := performedInvocation(s.Node)
	if !ok {
		return fmt.Errorf("%s: 'perform' names no action to perform", h.describe())
	}
	// The performed action reads the values in scope where it is performed and its
	// outputs come back to them, so a perform in a loop body sees that iteration.
	env := h.engine.env
	outputs, err := invokeAction(h.exec.ctx, s.Scope, inv, env.values())
	if err != nil {
		return fmt.Errorf("%s: %w", h.describe(), err)
	}
	for name, value := range outputs {
		if !env.assign(name, value) {
			env.data[name] = value
		}
	}
	return nil
}

// declaredOutput reports no output features: an action node's parameters live
// in the action's feature space, which an assignment writes directly.
func (h *actionStmtHost) declaredOutput(string) bool {
	return false
}
