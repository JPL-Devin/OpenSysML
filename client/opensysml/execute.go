package opensysml

import (
	"context"
	"maps"
	"slices"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// ActionRun is what one action execution produced.
type ActionRun struct {
	// Outputs are the action's output parameters by name, empty for an action
	// that produces none.
	Outputs map[string]Value
	// Diagnostics the execution reported.
	Diagnostics []Diagnostic
}

// StateRun is what one state machine execution produced.
type StateRun struct {
	// Visited is the trace of states entered, in order.
	Visited []string
	// Context is the machine's context when execution stopped, by feature name.
	Context map[string]Value
	// Diagnostics the execution reported.
	Diagnostics []Diagnostic
}

func (c *client) ExecuteAction(
	ctx context.Context,
	model *Model,
	actionSymbolID string,
	inputs map[string]Value,
) (*ActionRun, error) {
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	req := &pb.ExecuteActionRequest{ModelHash: hash, ActionSymbolId: actionSymbolID}
	if len(inputs) > 0 {
		if err := c.requireComplexValues(ctx, slices.Collect(maps.Values(inputs))...); err != nil {
			return nil, err
		}
		req.Inputs = make(map[string]*pb.Value, len(inputs))
		for name, value := range inputs {
			sent, err := valueToProto(value)
			if err != nil {
				return nil, err
			}
			req.Inputs[name] = sent
		}
	}
	resp, err := c.caller.executeAction(ctx, req)
	if err != nil {
		return nil, err
	}
	diagnostics := diagnosticsFromProto(resp.Diagnostics)
	if resp.Error != "" {
		return nil, &FailureError{Op: "ExecuteAction", Message: resp.Error, Diagnostics: diagnostics}
	}
	return &ActionRun{Outputs: valuesFromProto(resp.Outputs), Diagnostics: diagnostics}, nil
}

func (c *client) ExecuteState(
	ctx context.Context,
	model *Model,
	stateMachineSymbolID string,
	events []string,
) (*StateRun, error) {
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	resp, err := c.caller.executeState(ctx, &pb.ExecuteStateRequest{
		ModelHash:            hash,
		StateMachineSymbolId: stateMachineSymbolID,
		Events:               append([]string(nil), events...),
	})
	if err != nil {
		return nil, err
	}
	diagnostics := diagnosticsFromProto(resp.Diagnostics)
	if resp.Error != "" {
		return nil, &FailureError{Op: "ExecuteState", Message: resp.Error, Diagnostics: diagnostics}
	}
	return &StateRun{
		Visited:     append([]string(nil), resp.StatesVisited...),
		Context:     valuesFromProto(resp.FinalContext),
		Diagnostics: diagnostics,
	}, nil
}
