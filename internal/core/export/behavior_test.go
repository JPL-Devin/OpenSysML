package export_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/export"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

// behavioralModels are the models whose whole point is behavior: every action
// node, loop, conditional, state and transition the mapping covers.
var behavioralModels = []string{"action_nodes", "loops_conditionals", "state_machine"}

// The fidelity contract for behavior: a model that states behavior comes back
// from RDF as the same bytes it was written as, not merely as a graph that says
// something similar.
func TestBehavioralModelsComeBackByteIdentical(t *testing.T) {
	for _, name := range behavioralModels {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("testdata", "convert", name+".sysml")
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			turtle, err := export.Convert(path, src, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := export.Convert(name+".ttl", turtle, export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			if string(back) != string(src) {
				t.Errorf("the notation changed\n--- want ---\n%s\n--- got ---\n%s", src, back)
			}
		})
	}
}

// Each action node has a metaclass of its own, so the graph says which node it
// is rather than leaving the reader to parse the notation back out of a literal.
func TestActionNodeMetaclasses(t *testing.T) {
	for _, want := range []string{
		"sysx:InitialNode", "sysx:FinalNode",
		"sysml:ForkNode", "sysml:JoinNode", "sysml:MergeNode", "sysml:DecisionNode",
		"sysml:AssignmentActionUsage", "sysml:SendActionUsage",
		"sysml:TerminateActionUsage", "sysml:SuccessionAsUsage",
	} {
		turtle := toTurtle(t, filepath.Join("testdata", "convert", "action_nodes.sysml"))
		if !strings.Contains(turtle, want) {
			t.Errorf("the graph should type a node as %s:\n%s", want, turtle)
		}
	}
}

// The loop and conditional metaclasses, likewise: a `while`, a `loop`, a `for`
// and an `if` are told apart by their type and their conditions, not by text.
func TestLoopAndConditionalMetaclasses(t *testing.T) {
	turtle := toTurtle(t, filepath.Join("testdata", "convert", "loops_conditionals.sysml"))
	for _, want := range []string{
		"sysml:WhileLoopActionUsage", "sysml:ForLoopActionUsage", "sysml:IfActionUsage",
		"sysx:IfBranch", "sysx:whileCondition", "sysx:untilCondition",
		"sysx:loopVariable", "sysx:collection",
	} {
		if !strings.Contains(turtle, want) {
			t.Errorf("the graph should carry %s:\n%s", want, turtle)
		}
	}
}

// A state machine's structure is in the graph as its own elements: the states,
// their subactions, the regions that hold them and the transitions between.
func TestStateMachineMetaclasses(t *testing.T) {
	turtle := toTurtle(t, filepath.Join("testdata", "convert", "state_machine.sysml"))
	for _, want := range []string{
		"sysml:StateUsage", "sysml:StateSubactionMembership", "sysml:TransitionUsage",
		"sysx:Pseudostate", "sysx:StateRegion", "sysx:DeferMember",
		"sysx:subactionKind", "sysx:trigger", "sysx:guard",
		"sysml:sourceFeature", "sysml:targetFeature",
	} {
		if !strings.Contains(turtle, want) {
			t.Errorf("the graph should carry %s:\n%s", want, turtle)
		}
	}
}

// Each shape of each behavioral statement round trips on its own, so a failure
// names the notation that broke rather than a whole model.
func TestBehavioralStatementsRoundTrip(t *testing.T) {
	actions := map[string]string{
		"initial node":        "first start;",
		"guarded initial":     "first start if speed > 0 then brake;",
		"final node":          "done finish;",
		"fork":                "fork split;",
		"join":                "join sync;",
		"merge":               "merge gather;",
		"decision":            "decision pick;",
		"decide keyword":      "decide pick;",
		"perform":             "perform brake;",
		"action node":         "action Brake;",
		"assignment":          "assign log := speed;",
		"assigned expression": "assign log := speed + 1;",
		"send to":             "send Command() to antenna;",
		"send via":            "send Command() via antenna;",
		"accept":              "accept when speed > 0;",
		"action accept":       "action watch accept when speed > 0;",
		"terminate":           "terminate;",
		"terminate a node":    "terminate brake;",
		"succession":          "succession first brake then finish;",
		"edge succession":     "done finish;\n        then brake finish;",
		"while loop":          "while speed > 0 {\n            perform brake;\n        }",
		"loop":                "loop {\n            perform brake;\n        }",
		"loop until":          "loop {\n            perform brake;\n        } until speed > 0;",
		"for loop":            "for idx in 1..10 {\n            perform brake;\n        }",
		"if":                  "if speed > 0 {\n            perform brake;\n        }",
		"if else":             "if speed > 0 {\n            perform brake;\n        } else {\n            perform brake;\n        }",
		"else if":             "if speed > 0 {\n            perform brake;\n        } else if speed > 1 {\n            perform brake;\n        }",
	}
	for name, statement := range actions {
		t.Run(name, func(t *testing.T) {
			src := "package P {\n    attribute def Command;\n    port def Radio;\n    action def Brake;\n" +
				"    action def Drive {\n        in attribute speed;\n        attribute log;\n" +
				"        port antenna : Radio;\n        action brake : Brake;\n        " +
				statement + "\n    }\n}\n"
			checkRoundTrip(t, src)
		})
	}
}

// The members of a state body round trip in each of the shapes the notation
// allows: a subaction stated with braces, with a single action, or empty.
func TestStateMembersRoundTrip(t *testing.T) {
	members := map[string]string{
		"initial state":        "initial launch;",
		"final state":          "final stopped;",
		"substate":             "state waiting;",
		"empty entry":          "entry;",
		"entry action":         "entry perform Warm;",
		"entry braced":         "entry {\n            perform Warm;\n        }",
		"entry do braced":      "entry do {\n            perform Warm;\n        }",
		"anonymous action":     "entry action {\n            perform Warm;\n        }",
		"do action":            "do action running : Warm;",
		"exit action":          "exit perform Warm;",
		"defer":                "defer sig;",
		"defer several":        "defer sig, other;",
		"choice":               "choice pick;",
		"junction":             "junction meet;",
		"fork pseudostate":     "fork split;",
		"join pseudostate":     "join sync;",
		"entry point":          "entry point way_in;",
		"exit point":           "exit point way_out;",
		"shallow history":      "shallow history last;",
		"deep history":         "deep history deepest;",
		"history synonym":      "history last;",
		"nested state":         "state working {\n            entry perform Warm;\n        }",
		"nested transition":    "state working {\n            state a;\n            transition first a then a;\n        }",
		"nested substates":     "state working {\n            state first_gear;\n            state second_gear;\n        }",
		"nested regions":       "state working {\n            region left {\n                state stopped;\n            }\n            region right {\n                state moving;\n            }\n        }",
		"nested full body":     "state working {\n            entry perform Warm;\n            do action spin : Warm;\n            exit perform Warm;\n            defer sig;\n            state deeper;\n        }",
		"unordered subactions": "state working {\n            do action spin : Warm;\n            entry perform Warm;\n        }",
		"region":               "region primary {\n            state idle;\n        }",
		"transition first":     "transition first idle then idle;",
		"transition to":        "transition idle to idle;",
		"named transition":     "transition go first idle then idle;",
		"trigger":              "transition first idle accept sig : Signal then idle;",
		"guard":                "transition first idle if speed > 0 then idle;",
		"effect":               "transition first idle do action stop : Warm then idle;",
	}
	for name, member := range members {
		t.Run(name, func(t *testing.T) {
			src := "package P {\n    attribute def Signal;\n    action def Warm;\n" +
				"    state def Machine {\n        in attribute speed;\n        state idle;\n        " +
				member + "\n    }\n}\n"
			checkRoundTrip(t, src)
		})
	}
}

// A graph the notation has no words for is reported, naming the element — a
// notation that quietly dropped a graph's steps would be worse than a refusal.
func TestUnsupportedBehavioralShapesAreReported(t *testing.T) {
	prologue := "@prefix elmt: <urn:sysmlv2:element:> .\n" +
		"@prefix sysml: <https://www.omg.org/spec/SysML#> .\n" +
		"@prefix sysx: <urn:systemica:sysml:> .\n" +
		"@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .\n\n" +
		"elmt:P a sysml:Package ; sysml:qualifiedName \"P\" ; sysml:declaredName \"P\" ;\n" +
		"    sysx:memberIndex \"0\"^^xsd:integer ; sysx:hasBody \"true\"^^xsd:boolean .\n\n" +
		"elmt:P::A a sysml:ActionDefinition ; sysml:qualifiedName \"P::A\" ;\n" +
		"    sysml:owningNamespace elmt:P ; sysml:declaredName \"A\" ;\n" +
		"    sysx:memberIndex \"0\"^^xsd:integer ; sysx:hasBody \"true\"^^xsd:boolean .\n\n"
	member := func(name, metaclass, extra string) string {
		return "elmt:P::A::" + name + " a " + metaclass + " ;\n" +
			"    sysml:qualifiedName \"P::A::" + name + "\" ; sysml:owningNamespace elmt:P::A ;\n" +
			"    sysx:memberIndex \"0\"^^xsd:integer ;\n" + extra +
			"    sysx:hasBody \"false\"^^xsd:boolean .\n\n"
	}
	models := map[string]struct{ src, names string }{
		"a succession that names no end": {
			src:   prologue + member("edge", "sysml:SuccessionAsUsage", ""),
			names: "does not name both of the members it sequences",
		},
		"a branch written with `if` that states no guard": {
			src: prologue + member("edge", "sysml:SuccessionAsUsage",
				"    sysml:targetFeature \"other\" ; sysx:declaredKeyword \"if\" ;\n"),
			names: "sysx:guard",
		},
		"a state subaction of no stated kind": {
			src:   prologue + member("sub", "sysml:StateSubactionMembership", ""),
			names: "sysx:subactionKind",
		},
		"a pseudostate of no stated kind": {
			src:   prologue + member("pick", "sysx:Pseudostate", ""),
			names: "sysx:pseudostateKind",
		},
	}
	for name, model := range models {
		t.Run(name, func(t *testing.T) {
			_, err := export.Convert("m.ttl", []byte(model.src), export.FormatTurtle, export.FormatSysML)
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("want an UnsupportedError, got %v", err)
			}
			if !strings.Contains(err.Error(), model.names) {
				t.Errorf("the refusal should name the node: %v", err)
			}
		})
	}
}

// The `else` branch is marked with a term the SysML metamodel does not define,
// so it belongs to the Systemica namespace and is read from there on its own —
// without the branch's keyword having to say `else` as well.
func TestElseBranchIsMarkedUnderTheExtensionNamespace(t *testing.T) {
	src := "package P {\n\taction def A {\n\t\tdecision d;\n\t\tif x then a;\n\t\telse b;\n\t\taction a;\n\t\taction b;\n\t\tattribute x : Boolean;\n\t}\n}"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	if !strings.Contains(string(turtle), "sysx:isElse") {
		t.Errorf("the `else` marker should be a sysx: term:\n%s", turtle)
	}
	if strings.Contains(string(turtle), "sysml:isElse") {
		t.Errorf("the SysML vocabulary defines no isElse:\n%s", turtle)
	}

	// The keyword the branch was written with is dropped, so `else` can only
	// come back from the marker itself.
	graph := strings.ReplaceAll(string(turtle), "sysx:declaredKeyword \"else\" ;", "")
	back, err := export.Convert("m.ttl", []byte(graph), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v\n%s", err, graph)
	}
	if !strings.Contains(string(back), "else b;") {
		t.Errorf("the marker alone did not write the branch back:\n%s", back)
	}
}

// The `do` of a combined subaction is what it was written with, whether or not a
// space separates it from the body it introduces.
func TestSubactionDoSurvivesWithoutASpace(t *testing.T) {
	src := "package P {\n    action def Warm;\n    state def Machine {\n        state s {\n" +
		"            entry do{ perform Warm; }\n        }\n    }\n}\n"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	if !strings.Contains(string(turtle), "sysx:declaredKeyword \"entry do\"") {
		t.Fatalf("the `do` of the subaction was not recorded:\n%s", turtle)
	}
	back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "entry do {") {
		t.Fatalf("the subaction lost its `do`:\n%s", back)
	}
	// The notation it comes back as is what the printer writes, so converting that
	// again must change nothing at all.
	checkRoundTrip(t, string(back))
}

// checkRoundTrip converts a model to RDF and back, requiring the same notation
// and, converted again, the same graph.
func checkRoundTrip(t *testing.T, src string) {
	t.Helper()
	if p := parser.New(source.New("m.sysml", []byte(src))); len(p.ParseFile().Members) == 0 || len(p.Diagnostics) > 0 {
		t.Fatalf("the model itself does not parse: %v\n%s", p.Diagnostics, src)
	}
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v\n%s", err, turtle)
	}
	if string(back) != src {
		t.Fatalf("the notation changed\n--- want ---\n%s\n--- got ---\n%s", src, back)
	}
	again, err := export.Convert("m.sysml", back, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	if string(again) != string(turtle) {
		t.Errorf("round trip changed the graph\n--- first ---\n%s\n--- second ---\n%s", turtle, again)
	}
}

func toTurtle(t *testing.T, path string) string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	turtle, err := export.Convert(path, src, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	return string(turtle)
}
