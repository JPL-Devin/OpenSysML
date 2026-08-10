package repl

import (
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/runtime"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

// loadFixture submits a testdata model into a fresh session.
func loadFixture(t *testing.T, path string) *Session {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := NewSession()
	if res := s.Submit(string(data)); len(res.Diagnostics) > 0 {
		t.Fatalf("fixture %s has diagnostics: %v", path, res.Diagnostics)
	}
	return s
}

// run executes one meta command and returns its output as a single string.
func run(t *testing.T, s *Session, line string) string {
	t.Helper()
	out, quit, err := s.RunMeta(line)
	if err != nil {
		t.Fatalf("%s: %v", line, err)
	}
	if quit {
		t.Fatalf("%s unexpectedly quit", line)
	}
	return strings.Join(out, "\n")
}

// wants asserts every fragment appears in got.
func wants(t *testing.T, got string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(got, fragment) {
			t.Errorf("expected %q in output:\n%s", fragment, got)
		}
	}
}

// rejects asserts no fragment appears in got.
func rejects(t *testing.T, got string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if strings.Contains(got, fragment) {
			t.Errorf("did not expect %q in output:\n%s", fragment, got)
		}
	}
}

// TestREPLPackagedModelWorkflow is the end-to-end smoke test for the advertised
// workflow on a packaged model: nothing in the fixture is declared at the top
// level, so every command has to resolve a package member.
func TestREPLPackagedModelWorkflow(t *testing.T) {
	script := []string{
		"%load testdata/vehicle_package.sysml",
		"%instantiate Vehicle",
		"%slots Vehicle",
		"%eval Demo::Vehicle::mass",
		"%calc add 2 3",
		"%constraint withinMassLimit",
		"%instances",
	}
	var out strings.Builder
	if err := Loop(&scriptReader{lines: script}, &out, NewSession()); err != nil {
		t.Fatalf("Loop error: %v", err)
	}
	got := out.String()

	wants(t, got,
		"✓ Created instance of Demo::Vehicle",
		"Instance: Demo::Vehicle",
		"mass = 1500.00",
		"engine = Instance(",
		"✓ Demo::Vehicle::mass",
		"= 1500.00",
		"✓ add(2, 3)",
		"= 5",
		"✓ Constraint withinMassLimit passed",
		"Instances:",
	)
	rejects(t, got, "not found", "error:")
}

func TestInstantiateFindsPackageMemberBySimpleAndQualifiedName(t *testing.T) {
	for _, name := range []string{"Vehicle", "Demo::Vehicle"} {
		t.Run(name, func(t *testing.T) {
			s := loadFixture(t, "testdata/vehicle_package.sysml")
			got := run(t, s, "%instantiate "+name)
			wants(t, got, "✓ Created instance of Demo::Vehicle")

			// Instances are keyed by the resolved name, so either spelling
			// reaches the instance the other created.
			wants(t, run(t, s, "%slots Vehicle"), "mass = 1500.00")
			wants(t, run(t, s, "%slots Demo::Vehicle"), "mass = 1500.00")
		})
	}
}

func TestInstantiateUnknownSymbol(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%instantiate Nope"), `error: symbol "Nope" not found`)
	wants(t, run(t, s, "%instantiate Demo::Nope"), `error: symbol "Demo::Nope" not found`)
}

// A simple name matching in two packages is reported with the candidates rather
// than resolved to whichever the scope walk reached first.
func TestAmbiguousSimpleNameReportsCandidates(t *testing.T) {
	s := NewSession()
	s.Submit(`package A { part def Widget { attribute size = 1.0; } }
package B { part def Widget { attribute size = 2.0; } }`)

	got := run(t, s, "%instantiate Widget")
	wants(t, got, `symbol "Widget" is ambiguous`, "A::Widget", "B::Widget")

	// The qualified name disambiguates.
	wants(t, run(t, s, "%instantiate B::Widget"), "✓ Created instance of B::Widget")
	wants(t, run(t, s, "%slots B::Widget"), "size = 2.00")
}

// Declarations submitted after a lookup must be visible: the symbol index and
// runtime context are derived from a document that Submit replaces.
func TestLookupSeesDeclarationsAddedAfterFirstLookup(t *testing.T) {
	s := NewSession()
	s.Submit(`package Demo { part def Vehicle { attribute mass = 1500.0; } }`)
	wants(t, run(t, s, "%instantiate Demo::Vehicle"), "✓ Created instance of Demo::Vehicle")

	s.Submit(`package Demo { part def Trailer { attribute mass = 900.0; } }`)
	wants(t, run(t, s, "%instantiate Demo::Trailer"), "✓ Created instance of Demo::Trailer")
	wants(t, run(t, s, "%slots Trailer"), "mass = 900.00")
	wants(t, run(t, s, "%eval Demo::Trailer::mass"), "900.00")
}

// A submission discards the runtime context, which restarts instance IDs, so
// instances created before it must not survive into the new one.
func TestInstancesDoNotOutliveTheirRuntimeContext(t *testing.T) {
	s := NewSession()
	s.Submit(`package Demo { part def Vehicle { attribute mass = 1500.0; } }`)
	wants(t, run(t, s, "%instantiate Demo::Vehicle"), "ID: 1")
	wants(t, run(t, s, "%instances"), "Demo::Vehicle")

	s.Submit(`package Demo { part def Trailer { attribute mass = 900.0; } }`)
	wants(t, run(t, s, "%instances"), "(no instances created)")
	wants(t, run(t, s, "%instantiate Demo::Trailer"), "ID: 1")
	rejects(t, run(t, s, "%instances"), "Demo::Vehicle")
}

func TestSlotsWithoutInstance(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%slots Vehicle"), "no instance of", "%instantiate")
}

func TestInstancesEmptyAndPopulated(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%instances"), "(no instances created)")
	run(t, s, "%instantiate Engine")
	wants(t, run(t, s, "%instances"), "Demo::Engine")
}

func TestEvalLiteralFeatureAndCompound(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")

	// Literal: evaluated without any session context.
	wants(t, run(t, s, "%eval 6 * 7"), "= 42")
	// Feature reference by qualified name, and by simple name through the
	// scope-tree walk.
	wants(t, run(t, s, "%eval Demo::Engine::power"), "= 300.00")
	wants(t, run(t, s, "%eval power"), "= 300.00")

	// Compound expression over a top-level feature.
	flat := NewSession()
	flat.Submit("attribute mass = 3.0;")
	wants(t, run(t, flat, "%eval mass + 1.0"), "= 4.00")
}

func TestEvalErrors(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%eval missing"), `symbol "missing" not found`)
	// A part def is a symbol, but not one with a value.
	wants(t, run(t, s, "%eval Demo::Vehicle"), "has no value to evaluate")

	empty := NewSession()
	wants(t, run(t, empty, "%eval mass"), "no declarations loaded")
}

func TestCalcWithPositionalArgs(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%calc add 20 22"), "✓ add(20, 22)", "= 42")
	wants(t, run(t, s, "%calc Demo::add 1 2"), "= 3")
	wants(t, run(t, s, "%calc nosuch 1"), `symbol "nosuch" not found`)
}

func TestConstraintPassAndFail(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%constraint withinMassLimit"), "✓ Constraint withinMassLimit passed")
	wants(t, run(t, s, "%constraint Demo::overMassLimit"), "✗ Constraint Demo::overMassLimit failed")
	wants(t, run(t, s, "%constraint nosuch"), `symbol "nosuch" not found`)
}

func TestRequirement(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%requirement SafeMass"), "✓ Requirement SafeMass satisfied")
	wants(t, run(t, s, "%requirement Demo::SafeMass"), "satisfied")
	wants(t, run(t, s, "%requirement nosuch"), `symbol "nosuch" not found`)
}

func TestFormatValue(t *testing.T) {
	cases := []struct {
		name string
		val  runtime.Value
		want string
	}{
		{"int", runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 7}}, "7"},
		{"real", runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 1.5}}, "1.50"},
		{"bool", runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: true}}, "true"},
		{"infinity", runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInfinity}}, "∞"},
		{"null", runtime.Value{Kind: runtime.ValNull}, "null"},
		{"string", runtime.Value{Kind: runtime.ValString, Str: "hi"}, `"hi"`},
		{"instance", runtime.Value{Kind: runtime.ValInstance, Instance: 3}, "Instance(ID: 3)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatValue(tc.val); got != tc.want {
				t.Errorf("formatValue = %q, want %q", got, tc.want)
			}
		})
	}
}

// --- Action debugger ---

func TestActionDebuggerRunsToResult(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")

	wants(t, run(t, s, "%action tally"), `✓ Started action executor for "tally"`, "Tokens: 1")
	wants(t, run(t, s, "%tokens"), "Active tokens (1):", "Token 1 @ start")
	wants(t, run(t, s, "%step"), "✓ Step complete")
	wants(t, run(t, s, "%continue"), "✓ Action completed", "total = 5")
	wants(t, run(t, s, "%continue"), "already completed")
	wants(t, run(t, s, "%stop"), `✓ Stopped debugging session for "tally"`)
}

// %tokens names the node a token sits on, not its Go type.
func TestTokensShowNodeNames(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")
	got := run(t, s, "%tokens")
	wants(t, got, "Token 1 @ start")
	rejects(t, got, "*ast.")
}

func TestActionDebuggerRejectsNonAction(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%action Vehicle"), "is not an action")
	wants(t, run(t, s, "%action nosuch"), `symbol "nosuch" not found`)
}

// %break stops a run when a token reaches the node, and the run resumes from
// there with the tokens intact.
func TestBreakpointStopsAndResumes(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")

	wants(t, run(t, s, "%break accumulate"), `✓ Breakpoint set at node "accumulate"`)

	paused := run(t, s, "%continue")
	wants(t, paused, `⏸ Paused at breakpoint "accumulate"`, "Tokens: 1")
	rejects(t, paused, "Action completed")

	wants(t, run(t, s, "%tokens"), "Token 1 @ accumulate", "total = 0")
	wants(t, run(t, s, "%continue"), "✓ Action completed", "total = 5")
}

// The initial node already holds a token when the run starts, so a breakpoint
// on it must still stop before the first step.
func TestBreakpointOnInitialNodeStops(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")
	run(t, s, "%break start")

	paused := run(t, s, "%continue")
	wants(t, paused, `⏸ Paused at breakpoint "start"`, "Tokens: 1")
	rejects(t, paused, "Action completed")

	wants(t, run(t, s, "%tokens"), "Token 1 @ start")
	wants(t, run(t, s, "%continue"), "✓ Action completed", "total = 5")
}

func TestBreakpointRejectsUnknownNode(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")
	got := run(t, s, "%break nosuchnode")
	wants(t, got, `has no node named "nosuchnode"`, "nodes: ")
}

// --- State machine debugger ---

func TestStateDebuggerAdvancesByTime(t *testing.T) {
	s := loadFixture(t, "testdata/state_debug.sysml")

	wants(t, run(t, s, "%state Cycle"), `✓ Started state machine executor for "Cycle"`, "Current state: init")
	wants(t, run(t, s, "%events"), "Event queue: 1 events")
	wants(t, run(t, s, "%current"), "Current state: init", "Time: 0.00")

	// The completion transition out of `init` is due now; the transition out of
	// `waiting` is scheduled at 10, so a shorter advance stops before it.
	wants(t, run(t, s, "%advance 1"), "Advanced to 1.00 (1 event(s) processed)", "Current state: waiting", "Last event at: 0.00")

	// Durations accumulate: the second advance reaches 10 even though no event
	// moved the executor's own clock during the first.
	wants(t, run(t, s, "%advance 9"), "Advanced to 10.00 (1 event(s) processed)", "Current state: working")
	wants(t, run(t, s, "%current"), "Time: 10.00")

	wants(t, run(t, s, "%advance 5"), "Current state: done", "Last event at: 15.00", "State machine completed")
	wants(t, run(t, s, "%advance 5"), "No pending work - simulation time is now 20.00")
}

// A do behavior is due now, so a small advance must run it even when the only
// queued event is far past the deadline.
func TestAdvanceRunsDoWorkWithFarFutureEvent(t *testing.T) {
	s := loadFixture(t, "testdata/state_do_far_event.sysml")
	run(t, s, "%state Slow")

	wants(t, run(t, s, "%advance 1"),
		"Current state: working",
		"Do behavior actions run: 2")
	wants(t, run(t, s, "%current"), "count = 2", "Time: 1.00")
}

func TestAdvanceRejectsBadDuration(t *testing.T) {
	s := loadFixture(t, "testdata/state_debug.sysml")
	run(t, s, "%state Cycle")
	wants(t, run(t, s, "%advance soon"), "invalid time")
	wants(t, run(t, s, "%advance -1"), "must not be negative")
	// A trailing unit is accepted.
	wants(t, run(t, s, "%advance 10s"), "Current state: working")
}

// %current reports the whole active configuration of a machine whose top level
// is orthogonal regions, rather than <unknown>.
func TestCurrentShowsOrthogonalRegions(t *testing.T) {
	s := loadFixture(t, "../core/runtime/testdata/conformance/state_orthogonal_regions.sysml")
	wants(t, run(t, s, "%state TrafficLight"), "✓ Started state machine executor")
	got := run(t, s, "%current")
	wants(t, got, "Current state: start | start")
	rejects(t, got, "<unknown>")
}

func TestStateDebuggerRejectsNonStateMachine(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%state Vehicle"), "is not a state machine")
	wants(t, run(t, s, "%state nosuch"), `symbol "nosuch" not found`)
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in      string
		want    float64
		wantErr bool
	}{
		{"30", 30, false},
		{"30s", 30, false},
		{"0.5", 0.5, false},
		{"", 0, true},
		{"later", 0, true},
		{"-1", 0, true},
		// A non-finite duration would poison the debugger clock for good.
		{"NaN", 0, true},
		{"inf", 0, true},
		{"-Inf", 0, true},
	}
	for _, tc := range cases {
		got, err := parseDuration(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseDuration(%q) = %v, want error", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("parseDuration(%q) = %v, %v; want %v", tc.in, got, err, tc.want)
		}
	}
}

func TestAnonymousNodeLabel(t *testing.T) {
	cases := []struct {
		node ast.Node
		want string
	}{
		{&ast.InitialNode{}, "<initial>"},
		{&ast.FinalNode{}, "<final>"},
		{&ast.ForkNode{}, "<fork>"},
		{&ast.JoinNode{}, "<join>"},
		{&ast.MergeNode{}, "<merge>"},
		{&ast.DecisionNode{}, "<decision>"},
		{&ast.ActionExecutionNode{}, "<action>"},
		{nil, "<none>"},
		{&ast.Usage{}, "<anonymous>"},
	}
	for _, tc := range cases {
		if got := anonymousNodeLabel(tc.node); got != tc.want {
			t.Errorf("anonymousNodeLabel(%T) = %q, want %q", tc.node, got, tc.want)
		}
	}
}

func TestIsSymbolReference(t *testing.T) {
	for _, expr := range []string{"mass", "Demo::Vehicle::mass"} {
		if !isSymbolReference(expr) {
			t.Errorf("%q should be a symbol reference", expr)
		}
	}
	for _, expr := range []string{"", "a + b", "f(x)", "a.b", "a:b"} {
		if isSymbolReference(expr) {
			t.Errorf("%q should not be a symbol reference", expr)
		}
	}
}
