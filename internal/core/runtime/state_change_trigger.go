package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lower"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

// changeWait is one change condition the active configuration is waiting on: the
// state watching it, the trigger as written, and why it did not fire.
type changeWait struct {
	state   string
	trigger string
	reason  string
}

// String renders one wait for a report of a machine that cannot progress.
func (w changeWait) String() string {
	return fmt.Sprintf("%s: %s (%s)", w.state, w.trigger, w.reason)
}

// changePoll is one poll of the change conditions the active configuration
// watches: a transition's condition and guard are evaluated once per poll,
// however many active leaves the state watching it encloses.
type changePoll struct {
	condition map[*lower.Transition]bool
	guard     map[*lower.Transition]bool
	waits     []changeWait
	waited    map[*lower.Transition]bool
}

// pollChangeEvents re-tests the ChangeEvent conditions the active configuration
// watches and takes the transitions they enable, reporting whether any fired. It
// walks outward from each active leaf, and selects and resolves conflicts exactly
// as an event dispatch does.
//
// A trigger fires on the condition rising: one that stays true does not take the
// same edge again, and only a condition observed false re-arms it.
func (e *StateExecutor) pollChangeEvents() (bool, error) {
	poll := &changePoll{
		condition: make(map[*lower.Transition]bool),
		guard:     make(map[*lower.Transition]bool),
		waited:    make(map[*lower.Transition]bool),
	}
	if err := e.observeChangeConditions(poll); err != nil {
		return false, err
	}
	if len(poll.condition) == 0 {
		e.changeWaits = nil
		return false, nil
	}

	candidates, err := e.selectCandidates(func(state *ast.StateNode) (*lower.Transition, error) {
		return e.risenChangeTransition(state, poll)
	})
	if err != nil {
		e.changeWaits = poll.waits
		return false, err
	}

	fired := false
	for _, candidate := range candidates {
		if e.losesToNestedTransition(candidates, candidate) || !e.isActive(candidate.leaf) {
			continue
		}
		// The edge is latched before it is taken: an effect that leaves the
		// condition true must not enable the same edge again.
		e.changeFired[candidate.trans] = true
		fired = true
		if err := e.fireFrom(candidate.source, candidate.trans); err != nil {
			return fired, fmt.Errorf("fire transition out of %s: %w", candidate.source.Name, err)
		}
		if e.state == StateCompleted {
			break
		}
	}
	e.changeWaits = poll.waits
	return fired, nil
}

// observeChangeConditions evaluates, once each, the change conditions of the
// transitions out of the active configuration. A false one re-arms its
// transition and is recorded as a wait.
func (e *StateExecutor) observeChangeConditions(poll *changePoll) error {
	for _, leaf := range e.activeLeaves() {
		for _, source := range e.getParentChain(leaf) {
			for _, trans := range e.graph.Transitions[source] {
				changeEvent, ok := trans.Trigger.(*ast.ChangeEvent)
				if !ok {
					continue
				}
				if _, seen := poll.condition[trans]; seen {
					continue
				}
				holds, err := e.changeConditionHolds(changeEvent, trans)
				if err != nil {
					return fmt.Errorf("state %s: %w", source.Name, err)
				}
				poll.condition[trans] = holds
				if !holds {
					delete(e.changeFired, trans)
					poll.wait(trans, source.Name, "condition is false")
					continue
				}
				if e.changeFired[trans] {
					poll.wait(trans, source.Name, "condition has not changed since it fired")
				}
			}
		}
	}
	return nil
}

// changeConditionHolds evaluates one change condition in the scope the
// transition was written in, the machine's data shadowing it.
func (e *StateExecutor) changeConditionHolds(changeEvent *ast.ChangeEvent, trans *lower.Transition) (bool, error) {
	condVal, err := e.evalStep(changeEvent.Condition, trans.Scope)
	if err != nil {
		return false, fmt.Errorf("eval change condition: %w", err)
	}
	if condVal.Kind != ValConst || condVal.Const.Kind != semantics.ValBool {
		return false, fmt.Errorf("change condition must be boolean, got %v", condVal.Kind)
	}
	return condVal.Const.Bool, nil
}

// risenChangeTransition returns the state's first change-triggered transition
// whose condition has risen and whose guard does not block it. A blocked one
// consumes nothing and stays armed, so its guard is re-tested next poll.
func (e *StateExecutor) risenChangeTransition(state *ast.StateNode, poll *changePoll) (*lower.Transition, error) {
	for _, trans := range e.graph.Transitions[state] {
		if _, ok := trans.Trigger.(*ast.ChangeEvent); !ok {
			continue
		}
		if !poll.condition[trans] || e.changeFired[trans] {
			continue
		}
		satisfied, ok := poll.guard[trans]
		if !ok {
			var err error
			satisfied, err = e.passesGuard(trans)
			if err != nil {
				return nil, fmt.Errorf("eval change guard: %w", err)
			}
			poll.guard[trans] = satisfied
		}
		if satisfied {
			return trans, nil
		}
		poll.wait(trans, state.Name, "guard is false")
	}
	return nil, nil
}

// wait records, once per transition, a change condition the configuration is
// waiting on.
func (p *changePoll) wait(trans *lower.Transition, state, reason string) {
	if p.waited[trans] {
		return
	}
	p.waited[trans] = true
	p.waits = append(p.waits, changeWait{state: state, trigger: triggerDescription(trans.Trigger), reason: reason})
}

// PollChangeEvents re-tests the change conditions the active configuration
// watches and takes the transitions they enable, reporting whether any fired:
// the step RunToCompletion takes, for a driver that steps the machine itself.
func (e *StateExecutor) PollChangeEvents() (bool, error) {
	defer e.ctx.beginExecutorRun(&e.runStarted)()

	return e.pollChangeEvents()
}

// ChangeWaits describes the change conditions the active configuration was
// waiting on as of the last poll, and nothing when it watches none.
func (e *StateExecutor) ChangeWaits() []string {
	waits := make([]string, 0, len(e.changeWaits))
	for _, wait := range e.changeWaits {
		waits = append(waits, wait.String())
	}
	return waits
}

// SuspendReason says why a machine that cannot progress cannot: the change
// conditions it waits on, or that nothing is left that could fire.
func (e *StateExecutor) SuspendReason() string {
	if e.state == StateCompleted || (e.state != StateSuspended && e.HasPendingWork()) {
		return ""
	}
	if waits := e.ChangeWaits(); len(waits) > 0 {
		return "waiting on change condition: " + strings.Join(waits, "; ")
	}
	return "quiesced: nothing left can fire (no queued event, signal in flight, running do behavior or watched change condition)"
}
