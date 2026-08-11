package runtime

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lower"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

// stmtEnv is the environment an action node's body statements execute in: the
// token's data, plus one frame per body-local block currently entered.
//
// A loop body and an `if` branch body are namespaces of their own
// (symbols/builder.go), so a name such a block declares lives in that block's
// frame and is discarded when the block exits — it never reaches the token, and
// so never leaks into the enclosing behavior. An assignment to a name no entered
// block declares reaches the token's data, which is how a loop body updates an
// attribute of the action it runs in.
type stmtEnv struct {
	data   map[string]Value
	frames []map[string]Value
}

// enter pushes a frame for a block about to run and returns it.
func (env *stmtEnv) enter() map[string]Value {
	frame := make(map[string]Value)
	env.frames = append(env.frames, frame)
	return frame
}

// leave discards the frame the innermost entered block declares into.
func (env *stmtEnv) leave() {
	if len(env.frames) > 0 {
		env.frames = env.frames[:len(env.frames)-1]
	}
}

// declare binds a name the innermost entered block declares.
func (env *stmtEnv) declare(name string, value Value) {
	if depth := len(env.frames); depth > 0 {
		env.frames[depth-1][name] = value
		return
	}
	env.data[name] = value
}

// assign writes to the innermost entered block that declares name, and to the
// token's data when none does.
func (env *stmtEnv) assign(name string, value Value) {
	for i := len(env.frames) - 1; i >= 0; i-- {
		if _, ok := env.frames[i][name]; ok {
			env.frames[i][name] = value
			return
		}
	}
	env.data[name] = value
}

// evalIn returns an evaluation context reading the token's data and the frames
// of the blocks currently entered, innermost last so that a block-local name
// shadows an outer one of the same name.
func (e *ActionExecutor) evalIn(env *stmtEnv) *EvalContext {
	ec := NewEvalContext(e.ctx, nil)
	ec.Push(env.data)
	for _, frame := range env.frames {
		ec.Push(frame)
	}
	return ec
}

// execStatements runs lowered body statements in declaration order.
func (e *ActionExecutor) execStatements(node ast.Node, stmts []lower.Statement, env *stmtEnv) error {
	for _, stmt := range stmts {
		if err := e.execStatement(node, stmt, env); err != nil {
			return err
		}
	}
	return nil
}

// execStatement runs one lowered body statement.
func (e *ActionExecutor) execStatement(node ast.Node, stmt lower.Statement, env *stmtEnv) error {
	switch s := stmt.(type) {
	case lower.Send:
		msg, err := e.evalIn(env).buildMessage(e.action.Scope, s)
		if err != nil {
			return err
		}
		e.ctx.post(e.graph.Connections, msg, s)
		return nil
	case lower.Assign:
		if s.Target == "" {
			return fmt.Errorf("action node %s: unsupported assignment target", ActionNodeName(node))
		}
		value, err := e.evalIn(env).Eval(s.Value)
		if err != nil {
			return fmt.Errorf("eval assignment RHS: %w", err)
		}
		env.assign(s.Target, value)
		return nil
	case lower.Declare:
		value := Value{Kind: ValNull}
		if s.Value != nil {
			evaluated, err := e.evalIn(env).Eval(s.Value)
			if err != nil {
				return fmt.Errorf("eval declaration %s: %w", s.Name, err)
			}
			value = evaluated
		}
		env.declare(s.Name, value)
		return nil
	case lower.If:
		return e.execIf(node, s, env)
	case lower.Loop:
		return e.execLoop(node, s, env)
	case lower.Unsupported:
		return fmt.Errorf("action node %s: %s in a body is not executable", ActionNodeName(node), s.Description)
	default:
		return fmt.Errorf("action node %s: unsupported statement %T", ActionNodeName(node), stmt)
	}
}

// execIf runs the branch its condition selects, or nothing when the condition
// is false and the conditional declared no else branch.
func (e *ActionExecutor) execIf(node ast.Node, stmt lower.If, env *stmtEnv) error {
	// The condition is evaluated outside both branches, so neither branch's
	// declarations are visible to it.
	holds, err := e.evalCondition(node, env, stmt.Condition, "condition of 'if'")
	if err != nil {
		return err
	}
	if holds {
		return e.execBlock(node, stmt.Then, env)
	}
	if stmt.Else != nil {
		return e.execBlock(node, *stmt.Else, env)
	}
	return nil
}

// execBlock runs a body-local block in a frame of its own.
func (e *ActionExecutor) execBlock(node ast.Node, block lower.Block, env *stmtEnv) error {
	env.enter()
	defer env.leave()
	return e.execStatements(node, block.Statements, env)
}

// execLoop runs a loop to termination. Every iteration spends one step of the
// context's budget, so a loop whose condition never holds — or one written
// without a condition at all — ends the execution with ErrStepLimitExceeded
// instead of hanging the caller that drove it.
//
// The body's frame is entered once and cleared at the start of every iteration:
// each iteration re-runs the body's declarations, while a post-condition
// (`loop … until c`) still reads what the iteration it follows declared.
func (e *ActionExecutor) execLoop(node ast.Node, stmt lower.Loop, env *stmtEnv) error {
	if stmt.Kind == ast.LoopFor {
		return e.execForLoop(node, stmt, env)
	}

	frame := env.enter()
	defer env.leave()

	for {
		if err := e.ctx.incrementStep(); err != nil {
			return err
		}

		if stmt.Kind == ast.LoopWhile {
			holds, err := e.evalCondition(node, env, stmt.Condition, "condition of 'while'")
			if err != nil {
				return err
			}
			if !holds {
				return nil
			}
		}

		clear(frame)
		if err := e.execStatements(node, stmt.Body.Statements, env); err != nil {
			return err
		}

		if stmt.Kind == ast.LoopUntil && stmt.Condition != nil {
			holds, err := e.evalCondition(node, env, stmt.Condition, "condition of 'until'")
			if err != nil {
				return err
			}
			if holds {
				return nil
			}
		}
	}
}

// execForLoop runs the body once per element of the loop's collection, with the
// element bound to the loop's variable in the body's own frame.
func (e *ActionExecutor) execForLoop(node ast.Node, stmt lower.Loop, env *stmtEnv) error {
	if stmt.Variable == "" {
		return fmt.Errorf("action node %s: 'for' loop declares no iteration variable", ActionNodeName(node))
	}

	// The collection is evaluated once, before the loop is entered, so the
	// iteration is over the value the loop started with.
	value, err := e.evalIn(env).Eval(stmt.Collection)
	if err != nil {
		return fmt.Errorf("eval 'for' collection: %w", err)
	}
	elements, err := forElements(value)
	if err != nil {
		return fmt.Errorf("action node %s: %w", ActionNodeName(node), err)
	}

	frame := env.enter()
	defer env.leave()

	for _, element := range elements {
		if err := e.ctx.incrementStep(); err != nil {
			return err
		}
		clear(frame)
		frame[stmt.Variable] = element
		if err := e.execStatements(node, stmt.Body.Statements, env); err != nil {
			return err
		}
	}
	return nil
}

// forElements returns the elements a `for` loop iterates over, in the order it
// visits them. A set has no order of its own and its backing map does not
// iterate in a stable one, so its elements are visited in the order their
// canonical rendering sorts in — the same order a trace renders them in.
func forElements(value Value) ([]Value, error) {
	switch value.Kind {
	case ValSequence:
		if value.Sequence == nil {
			return nil, nil
		}
		return value.Sequence.Elements(), nil
	case ValSet:
		if value.Set == nil {
			return nil, nil
		}
		elements := value.Set.Elements()
		sort.Slice(elements, func(i, j int) bool {
			return FormatTraceValue(elements[i]) < FormatTraceValue(elements[j])
		})
		return elements, nil
	default:
		return nil, fmt.Errorf("'for' collection must be a sequence or a set, got %s", value.Kind)
	}
}

// evalCondition evaluates a loop or branch condition. A condition that is not
// Boolean is a type error the typecheck pass reports (passes/typecheck.go
// checkBehaviorMember); an execution that reaches one was never checked, so it
// is reported here rather than coerced.
func (e *ActionExecutor) evalCondition(node ast.Node, env *stmtEnv, expr ast.Node, what string) (bool, error) {
	if expr == nil {
		return false, fmt.Errorf("action node %s: %s is missing", ActionNodeName(node), what)
	}
	value, err := e.evalIn(env).Eval(expr)
	if err != nil {
		return false, fmt.Errorf("eval %s: %w", what, err)
	}
	if value.Kind != ValConst || value.Const.Kind != semantics.ValBool {
		return false, fmt.Errorf("action node %s: %s must evaluate to a Boolean, got %s",
			ActionNodeName(node), what, value.Kind)
	}
	return value.Const.Bool, nil
}
