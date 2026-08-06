package runtime

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

// traceIndent is one nesting level of evaluation depth in a recorded trace.
const traceIndent = "  "

// TraceRecorder captures deterministic execution traces for testing.
// Used by golden trace tests to detect ordering/scheduling regressions.
//
// Evaluation entries are recorded in post-order: the sub-expressions of an
// expression appear before it, indented one level deeper, so sibling evaluation
// order and nesting are both readable off the trace. A constant sub-expression
// is answered by the semantic constant folder without evaluating its operands,
// so it appears with no children.
type TraceRecorder struct {
	entries []string
	enabled bool
	depth   int
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

// RecordCalcEnter records entering a calc invocation and opens a nesting level.
func (tr *TraceRecorder) RecordCalcEnter(name string) {
	tr.record(fmt.Sprintf("enter calc %s", name))
	tr.depth++
}

// RecordCalcBind records binding one calc input parameter. source names where
// the value came from ("argument" or "default").
func (tr *TraceRecorder) RecordCalcBind(param string, value Value, source string) {
	tr.record(fmt.Sprintf("bind %s = %s [%s]", param, FormatTraceValue(value), source))
}

// RecordCalcExit closes a calc invocation's nesting level and records its result.
func (tr *TraceRecorder) RecordCalcExit(name string, result Value) {
	tr.closeCalc()
	tr.record(fmt.Sprintf("exit calc %s -> %s", name, FormatTraceValue(result)))
}

// RecordCalcExitError closes a calc invocation that failed, recording why.
// The failure is part of the ordering contract: it says how far binding and
// evaluation got before the calc gave up.
func (tr *TraceRecorder) RecordCalcExitError(name string, err error) {
	tr.closeCalc()
	tr.record(fmt.Sprintf("exit calc %s -> error: %v", name, err))
}

// BeginEval opens a nesting level for one expression's sub-expressions.
func (tr *TraceRecorder) BeginEval() {
	tr.depth++
}

// EndEval closes the level BeginEval opened and records the expression's
// outcome, so the entry appears after the sub-expressions it consumed.
func (tr *TraceRecorder) EndEval(label string, value Value, err error) {
	if tr.depth > 0 {
		tr.depth--
	}
	if err != nil {
		tr.record(fmt.Sprintf("eval %s -> error: %v", label, err))
		return
	}
	tr.record(fmt.Sprintf("eval %s -> %s", label, FormatTraceValue(value)))
}

// closeCalc closes the nesting level a calc's enter entry opened.
func (tr *TraceRecorder) closeCalc() {
	if tr.depth > 0 {
		tr.depth--
	}
}

// record appends one entry at the current nesting depth. Depth is tracked
// whether or not recording is enabled, so nesting stays consistent across a
// recorder that is disabled and re-enabled mid-evaluation.
func (tr *TraceRecorder) record(entry string) {
	if !tr.enabled {
		return
	}
	tr.entries = append(tr.entries, strings.Repeat(traceIndent, tr.depth)+entry)
}

// RecordDoStep records one action of a state's do behavior, which is how the
// interleaving of concurrently active states' do behaviors becomes visible.
func (tr *TraceRecorder) RecordDoStep(state string) {
	if !tr.enabled {
		return
	}

	tr.entries = append(tr.entries, fmt.Sprintf("do: %s", state))
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

// Clear clears all recorded entries and resets nesting depth.
func (tr *TraceRecorder) Clear() {
	tr.entries = make([]string, 0)
	tr.depth = 0
}

// FormatTraceValue renders a runtime value canonically for a trace. Set
// elements are sorted by their rendering, since a set has no order of its own
// and its backing map does not iterate in a stable one.
func FormatTraceValue(v Value) string {
	switch v.Kind {
	case ValConst:
		return formatConst(v.Const)
	case ValNull:
		return "null"
	case ValString:
		return strconv.Quote(v.Str)
	case ValInstance:
		return fmt.Sprintf("instance#%d", v.Instance)
	case ValSequence:
		if v.Sequence == nil {
			return "()"
		}
		parts := make([]string, 0, v.Sequence.Size())
		for _, elem := range v.Sequence.Elements() {
			parts = append(parts, FormatTraceValue(elem))
		}
		return "(" + strings.Join(parts, ", ") + ")"
	case ValSet:
		if v.Set == nil {
			return "{}"
		}
		parts := make([]string, 0, v.Set.Size())
		for _, elem := range v.Set.Elements() {
			parts = append(parts, FormatTraceValue(elem))
		}
		sort.Strings(parts)
		return "{" + strings.Join(parts, ", ") + "}"
	case ValExpr:
		return fmt.Sprintf("expr(%s)", TraceLabel(v.Expr))
	default:
		return v.Kind.String()
	}
}

// formatConst renders a folded constant. Reals print with the shortest form
// that round-trips, so the same value always renders the same way.
func formatConst(c semantics.Value) string {
	switch c.Kind {
	case semantics.ValInt:
		return strconv.FormatInt(c.Int, 10)
	case semantics.ValReal:
		// A whole real keeps a ".0" so it is not mistaken for an integer.
		text := strconv.FormatFloat(c.Real, 'g', -1, 64)
		if !strings.ContainsAny(text, ".eEnN") {
			text += ".0"
		}
		return text
	case semantics.ValBool:
		return strconv.FormatBool(c.Bool)
	case semantics.ValInfinity:
		return "*"
	default:
		return "invalid"
	}
}

// TraceLabel names an expression node for a trace: its kind plus the token that
// identifies it, which is stable across reformatting of the source.
func TraceLabel(node ast.Node) string {
	switch n := node.(type) {
	case nil:
		return "nil"
	case *ast.LiteralInteger:
		return "literal " + n.Value
	case *ast.LiteralReal:
		return "literal " + n.Value
	case *ast.LiteralBool:
		return "literal " + strconv.FormatBool(n.Value)
	case *ast.LiteralString:
		return "literal " + n.Value
	case *ast.LiteralInfinity:
		return "literal *"
	case *ast.NullExpr:
		return "null"
	case *ast.FeatureReference:
		return "feature " + qualifiedNameToString(n.Name)
	case *ast.FeatureChainExpr:
		return "chain " + qualifiedNameToString(n.Member)
	case *ast.OperatorExpr:
		return "operator " + n.Operator.String()
	case *ast.InvocationExpr:
		return "invoke " + qualifiedNameToString(n.Type)
	case *ast.SequenceExpr:
		return fmt.Sprintf("sequence of %d", len(n.Elements))
	case *ast.CollectExpr:
		return "collect"
	case *ast.SelectExpr:
		return "select"
	case *ast.IndexExpr:
		return "index"
	case *ast.BodyExpr:
		return "body"
	case *ast.ConstructorExpr:
		return "construct " + qualifiedNameToString(n.Type)
	case *ast.MetadataAccessExpr:
		return "metadata"
	default:
		return fmt.Sprintf("%T", node)
	}
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
