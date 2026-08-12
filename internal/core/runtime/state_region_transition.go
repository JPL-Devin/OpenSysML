package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/lower"
)

// transientPseudostate reports whether a pseudostate merely routes a transition
// onwards — choice, junction, entry point and exit point — as opposed to fork,
// join and history, which rewrite the whole active configuration.
func transientPseudostate(kind ast.PseudostateKind) bool {
	switch kind {
	case ast.PseudostateChoice, ast.PseudostateJunction, ast.PseudostateEntry, ast.PseudostateExit:
		return true
	}
	return false
}

// transitionTarget returns the state a transition ends at, following any chain of
// transient pseudostates on the way.
func (e *StateExecutor) transitionTarget(trans *lower.Transition) (*ast.StateNode, error) {
	switch target := trans.Target.(type) {
	case *ast.StateNode:
		return target, nil
	case *ast.PseudostateNode:
		return e.pseudostateTarget(target)
	default:
		return nil, fmt.Errorf("transition target must be a state or pseudostate, got %T", trans.Target)
	}
}

// pseudostateTarget follows the outgoing transitions of a transient pseudostate
// until a state is reached, so a chain such as exit point → junction → state ends
// at the state the compound transition actually enters.
//
// A choice's guards are evaluated when it is reached and a junction's when its
// incoming transition is evaluated; both are evaluated here, in declaration
// order, which makes the two indistinguishable for a guard over state data.
func (e *StateExecutor) pseudostateTarget(ps *ast.PseudostateNode) (*ast.StateNode, error) {
	visited := make(map[*ast.PseudostateNode]bool)
	for {
		if visited[ps] {
			return nil, fmt.Errorf("%s %s: outgoing transitions form a cycle between pseudostates", ps.Kind, ps.Name)
		}
		visited[ps] = true

		branch, err := e.pseudostateBranch(ps)
		if err != nil {
			return nil, err
		}
		switch target := branch.Target.(type) {
		case *ast.StateNode:
			return target, nil
		case *ast.PseudostateNode:
			if !transientPseudostate(target.Kind) {
				return nil, fmt.Errorf("%s %s: a transition into %s %s is not supported", ps.Kind, ps.Name, target.Kind, target.Name)
			}
			ps = target
		default:
			return nil, fmt.Errorf("%s %s: target must be a state or pseudostate, got %T", ps.Kind, ps.Name, branch.Target)
		}
	}
}

// pseudostateBranch returns the outgoing transition a pseudostate routes along:
// the first whose guard is satisfied, in declaration order, an unguarded one
// being UML's else branch.
func (e *StateExecutor) pseudostateBranch(ps *ast.PseudostateNode) (*lower.Transition, error) {
	outgoing := e.graph.Transitions[ps]
	if len(outgoing) == 0 {
		return nil, fmt.Errorf("%s %s has no outgoing transitions", ps.Kind, ps.Name)
	}
	for _, trans := range outgoing {
		pass, err := e.passesGuard(trans)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", ps.Kind, ps.Name, err)
		}
		if pass {
			return trans, nil
		}
	}
	return nil, fmt.Errorf("%s %s: no guard evaluated to true", ps.Kind, ps.Name)
}

// regionContains reports whether state is declared in region or nested below a
// state that is.
func (e *StateExecutor) regionContains(region *ast.StateRegion, state *ast.StateNode) bool {
	if region == nil || state == nil {
		return false
	}
	for _, ancestor := range e.getParentChain(state) {
		if e.graph.RegionOf[ancestor] == region {
			return true
		}
	}
	return false
}

// branchesTo names, for every orthogonal region on the path from `from` down to
// target, the deepest state of that path inside it. Entering `from` with those
// branches therefore ends at target instead of at the regions' initial states.
func (e *StateExecutor) branchesTo(from, target *ast.StateNode) map[*ast.StateRegion]*ast.StateNode {
	branches := make(map[*ast.StateRegion]*ast.StateNode)
	var region *ast.StateRegion
	for _, state := range e.descendantChain(from, target) {
		if declaring := e.graph.RegionOf[state]; declaring != nil {
			region = declaring
		}
		if region != nil {
			branches[region] = state
		}
	}
	return branches
}

// entryPlan returns the state to enter below lca in order to reach target,
// together with the region branches that get there. Entering stops at the
// outermost state on the path whose own orthogonal regions the path descends
// into, because entering that state enters the rest of the path through them.
func (e *StateExecutor) entryPlan(lca, target *ast.StateNode) (*ast.StateNode, map[*ast.StateRegion]*ast.StateNode) {
	chain := e.descendantChain(lca, target)
	for i := 0; i+1 < len(chain); i++ {
		region := e.graph.RegionOf[chain[i+1]]
		if region != nil && e.graph.RegionOwner[region] == chain[i] {
			return chain[i], e.branchesTo(chain[i], target)
		}
	}
	return target, nil
}

// activeLeavesBelow returns the deepest active states inside state — one per
// active orthogonal region, recursively — which are the states whose outgoing
// transitions have to be scheduled after entering it.
func (e *StateExecutor) activeLeavesBelow(state *ast.StateNode) []*ast.StateNode {
	regions, composite := e.graph.CompositeStates[state]
	if !composite {
		return []*ast.StateNode{state}
	}
	leaves := make([]*ast.StateNode, 0, len(regions))
	for _, region := range regions {
		active, ok := e.activeConfig.regionStates[region]
		if !ok || active == state {
			continue
		}
		leaves = append(leaves, e.activeLeavesBelow(active)...)
	}
	return leaves
}

// scheduleFromEntered schedules the outgoing transitions of every state the move
// just made active below and including state.
func (e *StateExecutor) scheduleFromEntered(state *ast.StateNode) error {
	for _, leaf := range e.activeLeavesBelow(state) {
		if err := e.scheduleTransitionsForState(leaf); err != nil {
			return fmt.Errorf("schedule transitions: %w", err)
		}
	}
	return nil
}

// runEffect performs the actions of a transition's effect, in order.
func (e *StateExecutor) runEffect(trans *lower.Transition) error {
	for _, action := range trans.Effect {
		if err := e.executeAction(action, trans.BodyScope); err != nil {
			return fmt.Errorf("transition effect: %w", err)
		}
	}
	return nil
}

// fireTransitionInRegion fires a transition whose source is the active state of
// an orthogonal region. A target inside the same region moves only that region;
// a target outside it leaves the whole region set, which is what makes a
// transition through a choice, junction or entry/exit point reachable from
// inside a region.
func (e *StateExecutor) fireTransitionInRegion(region *ast.StateRegion, trans *lower.Transition) error {
	// Fork, join and history replace the entire active configuration rather than
	// move one region, so they are fired whole.
	if isSynchronizationTarget(trans.Target) {
		return e.fireTransition(trans)
	}

	pass, err := e.passesGuard(trans)
	if err != nil || !pass {
		return err
	}

	target, err := e.transitionTarget(trans)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("transition out of region %s has no target state", region.Name)
	}

	if e.regionContains(region, target) {
		return e.moveWithinRegion(region, trans, target)
	}
	return e.leaveRegion(region, trans, target)
}

// moveWithinRegion moves one orthogonal region from its active state to a target
// inside the same region, leaving its sibling regions untouched. The exit stops
// at the region boundary even when the least common ancestor lies above it, since
// the region is not left.
func (e *StateExecutor) moveWithinRegion(region *ast.StateRegion, trans *lower.Transition, target *ast.StateNode) error {
	source := e.activeConfig.regionStates[region]

	// The region is not left, so the move stops at the region's own boundary even
	// when the least common ancestor lies above it: the state owning the region
	// stays active and is not re-entered.
	lca := e.getLCA(source, target)
	if !e.regionContains(region, lca) {
		lca = e.graph.RegionOwner[region]
	}

	delete(e.activeConfig.regionStates, region)
	for current := source; current != nil && current != lca && e.regionContains(region, current); current = e.graph.ParentState[current] {
		if err := e.exitState(current); err != nil {
			return fmt.Errorf("exit state: %w", err)
		}
	}

	if err := e.runEffect(trans); err != nil {
		return err
	}

	enter, branches := e.entryPlan(lca, target)
	leaf := target
	if branch, ok := branches[region]; ok {
		leaf = branch
	}
	// The region's own entry is recorded before entering, so a state entered
	// inside it is not mistaken for the single active state of a simple machine.
	e.activeConfig.regionStates[region] = leaf
	for _, state := range e.descendantChain(lca, enter) {
		if err := e.enterStateInto(state, branches); err != nil {
			return fmt.Errorf("enter state: %w", err)
		}
	}

	if err := e.scheduleFromEntered(leaf); err != nil {
		return err
	}
	e.recordTransitionTrace(trans, source, target)
	return nil
}

// leaveRegion takes a transition whose target lies outside the orthogonal region
// its source is active in. UML's least common ancestor for such a transition is
// the region containing the composite state that owns the region set, so the
// whole set is left: every sibling region is exited — recording its
// configuration for history — before the target is entered.
func (e *StateExecutor) leaveRegion(region *ast.StateRegion, trans *lower.Transition, target *ast.StateNode) error {
	source := e.activeConfig.regionStates[region]
	owner := e.graph.RegionOwner[region]
	if owner == nil {
		return e.leaveTopRegions(trans, source, target)
	}

	// A target inside another region of the same composite state re-enters that
	// state: the regions the target does not name restart at their initial
	// states, since their previous configuration was left.
	if e.ownsRegionContaining(owner, target) {
		if err := e.exitState(owner); err != nil {
			return fmt.Errorf("exit state: %w", err)
		}
		if err := e.runEffect(trans); err != nil {
			return err
		}
		branches := e.branchesTo(owner, target)
		if err := e.enterStateInto(owner, branches); err != nil {
			return fmt.Errorf("enter state: %w", err)
		}
		if err := e.scheduleFromEntered(owner); err != nil {
			return err
		}
		e.recordTransitionTrace(trans, source, target)
		return nil
	}

	// The target is outside the composite state: exit it and its ancestors up to
	// the least common ancestor, then enter down to the target.
	lca := e.getLCA(owner, target)
	for current := owner; current != nil && current != lca; current = e.graph.ParentState[current] {
		if err := e.exitState(current); err != nil {
			return fmt.Errorf("exit state: %w", err)
		}
	}
	if err := e.runEffect(trans); err != nil {
		return err
	}
	return e.enterOutside(trans, source, lca, target)
}

// leaveTopRegions leaves the machine's own orthogonal regions, which no state
// owns: every region is exited in declaration order and the target is then
// entered, either as one region's state — the others restarting at their initial
// states — or as the machine's single active state.
func (e *StateExecutor) leaveTopRegions(trans *lower.Transition, source, target *ast.StateNode) error {
	for _, region := range e.graph.TopRegions {
		active, ok := e.activeConfig.regionStates[region]
		if !ok {
			continue
		}
		delete(e.activeConfig.regionStates, region)
		for current := active; current != nil; current = e.graph.ParentState[current] {
			if err := e.exitState(current); err != nil {
				return fmt.Errorf("exit state: %w", err)
			}
		}
	}
	e.activeConfig.regionStates = make(map[*ast.StateRegion]*ast.StateNode)
	e.activeConfig.simpleState = nil

	if err := e.runEffect(trans); err != nil {
		return err
	}

	if e.topRegionContaining(target) != nil {
		branches := e.branchesTo(nil, target)
		if err := e.enterRegionsInto(nil, e.graph.TopRegions, branches); err != nil {
			return err
		}
		for _, region := range e.graph.TopRegions {
			if active, ok := e.activeConfig.regionStates[region]; ok {
				if err := e.scheduleFromEntered(active); err != nil {
					return err
				}
			}
		}
		e.recordTransitionTrace(trans, source, target)
		return nil
	}

	return e.enterOutside(trans, source, nil, target)
}

// enterOutside enters target below lca after the region set the transition left
// has been torn down, and finishes the move.
func (e *StateExecutor) enterOutside(trans *lower.Transition, source, lca, target *ast.StateNode) error {
	enter, branches := e.entryPlan(lca, target)
	for _, state := range e.descendantChain(lca, enter) {
		if err := e.enterStateInto(state, branches); err != nil {
			return fmt.Errorf("enter state: %w", err)
		}
	}

	// Record the entered path: the deepest entered state of every orthogonal
	// region on it becomes that region's active state, and a target inside none
	// of them becomes the machine's single active state.
	onPath := e.branchesTo(nil, target)
	for region, leaf := range onPath {
		e.activeConfig.regionStates[region] = leaf
	}
	if len(onPath) == 0 && len(e.activeConfig.regionStates) == 0 {
		e.activeConfig.simpleState = target
	}

	e.stateStack = e.rootToLeaf(target)
	if err := e.scheduleFromEntered(enter); err != nil {
		return err
	}
	if target.IsFinal {
		e.state = StateCompleted
	}
	e.recordTransitionTrace(trans, source, target)
	return nil
}

// ownsRegionContaining reports whether one of owner's own orthogonal regions
// contains state.
func (e *StateExecutor) ownsRegionContaining(owner, state *ast.StateNode) bool {
	for _, region := range e.graph.CompositeStates[owner] {
		if e.regionContains(region, state) {
			return true
		}
	}
	return false
}

// topRegionContaining returns the machine's own region containing state, nil
// when state is outside all of them.
func (e *StateExecutor) topRegionContaining(state *ast.StateNode) *ast.StateRegion {
	for _, region := range e.graph.TopRegions {
		if e.regionContains(region, state) {
			return region
		}
	}
	return nil
}

// rootToLeaf returns state's ancestors and state itself, outermost first.
func (e *StateExecutor) rootToLeaf(state *ast.StateNode) []*ast.StateNode {
	chain := e.getParentChain(state)
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

// recordTransitionTrace records one transition in the trace, naming the state
// left and the state entered.
func (e *StateExecutor) recordTransitionTrace(trans *lower.Transition, source, target *ast.StateNode) {
	if e.trace() == nil {
		return
	}
	from := ""
	if source != nil {
		from = source.Name
	}
	e.trace().RecordStateTransition(from, target.Name, triggerName(trans.Trigger))
}

// orderedActiveRegions returns the active orthogonal regions in declaration
// order. The order regions react to a broadcast event in is observable, so it
// must not depend on map iteration order.
func (e *StateExecutor) orderedActiveRegions() []*ast.StateRegion {
	regions := make([]*ast.StateRegion, 0, len(e.activeConfig.regionStates))
	for _, region := range e.graph.TopRegions {
		if _, active := e.activeConfig.regionStates[region]; active {
			regions = append(regions, region)
		}
	}
	for _, state := range e.graph.States {
		for _, region := range e.graph.CompositeStates[state] {
			if _, active := e.activeConfig.regionStates[region]; active {
				regions = append(regions, region)
			}
		}
	}
	return regions
}
