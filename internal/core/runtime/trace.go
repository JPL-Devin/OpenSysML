package runtime

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
)

// TraceRecorder captures deterministic execution traces for testing.
// Used by golden trace tests to detect ordering/scheduling regressions.
type TraceRecorder struct {
	entries []string
	enabled bool
}

// NewTraceRecorder creates a new trace recorder.
func NewTraceRecorder() *TraceRecorder {
	return &TraceRecorder{
		entries: make([]string, 0),
		enabled: true,
	}
}

// Enable enables trace recording.
func (tr *TraceRecorder) Enable() {
	tr.enabled = true
}

// Disable disables trace recording.
func (tr *TraceRecorder) Disable() {
	tr.enabled = false
}

// RecordActionStep records an action executor step with active tokens.
// Tokens are sorted by ID for deterministic output.
func (tr *TraceRecorder) RecordActionStep(step int, tokens []Token) {
	if !tr.enabled {
		return
	}
	
	if len(tokens) == 0 {
		tr.entries = append(tr.entries, fmt.Sprintf("step %d: no active tokens", step))
		return
	}
	
	// Sort tokens by ID for determinism
	sorted := make([]Token, len(tokens))
	copy(sorted, tokens)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})
	
	// Format: step N: token T1@node1, token T2@node2
	var parts []string
	for _, t := range sorted {
		nodeName := nodeIdentifier(t.Location)
		parts = append(parts, fmt.Sprintf("token %d@%s", t.ID, nodeName))
	}
	
	tr.entries = append(tr.entries, fmt.Sprintf("step %d: %s", step, strings.Join(parts, ", ")))
}

// RecordStateTransition records a state transition with event.
func (tr *TraceRecorder) RecordStateTransition(fromState, toState string, event string) {
	if !tr.enabled {
		return
	}
	
	if event == "" {
		tr.entries = append(tr.entries, fmt.Sprintf("transition: %s -> %s", fromState, toState))
	} else {
		tr.entries = append(tr.entries, fmt.Sprintf("transition: %s -> %s (event: %s)", fromState, toState, event))
	}
}

// RecordStateEntry records entering a state with optional entry action execution.
func (tr *TraceRecorder) RecordStateEntry(state string, hasEntryAction bool) {
	if !tr.enabled {
		return
	}
	
	if hasEntryAction {
		tr.entries = append(tr.entries, fmt.Sprintf("enter: %s (entry action)", state))
	} else {
		tr.entries = append(tr.entries, fmt.Sprintf("enter: %s", state))
	}
}

// RecordStateExit records exiting a state with optional exit action execution.
func (tr *TraceRecorder) RecordStateExit(state string, hasExitAction bool) {
	if !tr.enabled {
		return
	}
	
	if hasExitAction {
		tr.entries = append(tr.entries, fmt.Sprintf("exit: %s (exit action)", state))
	} else {
		tr.entries = append(tr.entries, fmt.Sprintf("exit: %s", state))
	}
}

// RecordEvent records an event being processed.
func (tr *TraceRecorder) RecordEvent(event string, time float64) {
	if !tr.enabled {
		return
	}
	
	tr.entries = append(tr.entries, fmt.Sprintf("event: %s (t=%.1f)", event, time))
}

// Entries returns all recorded trace entries.
func (tr *TraceRecorder) Entries() []string {
	return tr.entries
}

// String returns the trace as a single string (newline-separated entries).
func (tr *TraceRecorder) String() string {
	return strings.Join(tr.entries, "\n")
}

// Clear clears all recorded entries.
func (tr *TraceRecorder) Clear() {
	tr.entries = make([]string, 0)
}

// nodeIdentifier returns a stable identifier for an AST node.
// Prefers named nodes (Ident.Name), falls back to node type.
func nodeIdentifier(node ast.Node) string {
	if node == nil {
		return "nil"
	}
	
	switch n := node.(type) {
	case *ast.Usage:
		if n.Ident.Name != "" {
			return n.Ident.Name
		}
		return fmt.Sprintf("usage_%s", n.Kind)
	case *ast.Definition:
		if n.Ident.Name != "" {
			return n.Ident.Name
		}
		return fmt.Sprintf("def_%s", n.Kind)
	case *ast.StateNode:
		if n.Name != "" {
			return n.Name
		}
		return "state_anonymous"
	default:
		return fmt.Sprintf("%T", node)
	}
}
