package runtime

import (
	"fmt"
	"iter"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// bodyRun is the work of one token's step run as a coroutine, so that a breakpoint
// met inside it — at a node of a block's flow, or a step of a flow a body runs —
// pauses the token there and a later step resumes the work where it stopped.
type bodyRun struct {
	// next resumes the work, yielding the breakpoint it paused at next, or false
	// once the work has ended with err.
	next  func() (string, bool)
	err   error
	after func(tokenIdx int) error
}

// runPausable runs work for the token at tokenIdx, then after with the token's
// index by then. With a breakpoint set, work runs as a coroutine a breakpoint met
// inside pauses: the token stays where it is, its performance half done, until it
// is stepped again. Without one, or inside a run already pausable, it runs in place.
func (e *ActionExecutor) runPausable(tokenIdx int, work func() error, after func(tokenIdx int) error) error {
	if len(e.breakpoints) == 0 || e.pause != nil {
		if err := work(); err != nil {
			return err
		}
		return after(tokenIdx)
	}
	run := &bodyRun{after: after}
	run.next, _ = iter.Pull(func(yield func(string) bool) {
		e.pause = yield
		defer func() { e.pause = nil }()
		run.err = work()
	})
	e.tokens[tokenIdx].body = run
	return e.resumeBody(tokenIdx)
}

// resumeBody lets the paused work of the token at tokenIdx go on to its next
// pause, or to its end, when what follows the work runs.
func (e *ActionExecutor) resumeBody(tokenIdx int) error {
	run := e.tokens[tokenIdx].body
	if name, paused := run.next(); paused {
		e.pausedAt = name
		e.state = StateSuspended
		return nil
	}
	e.tokens[tokenIdx].body = nil
	if run.err != nil {
		return run.err
	}
	return run.after(tokenIdx)
}

// pauseAt pauses the run before a node a breakpoint is set on performs, as the run
// pauses before a token steps such a node; a run not made pausable goes on.
func (e *ActionExecutor) pauseAt(node ast.Node) error {
	if name := e.breakpointNameOf(node); name != "" {
		return e.pauseRun(name)
	}
	return nil
}

// pauseRun pauses a pausable run at the named breakpoint until it is resumed.
func (e *ActionExecutor) pauseRun(name string) error {
	if e.pause == nil {
		return nil
	}
	if !e.pause(name) {
		return fmt.Errorf("%w: the run paused at breakpoint %q was abandoned", ErrActionDeadlock, name)
	}
	return nil
}

// drivenByBody reports whether the token runs in a flow a body statement runs
// (runSubflow), whose steps that statement takes rather than Step.
func (t Token) drivenByBody() bool {
	for f := t.frame; f != nil; f = f.parent {
		if f.inBody {
			return true
		}
	}
	return false
}
