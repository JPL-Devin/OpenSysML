package export_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// behavioralModels are the models whose whole point is behavior: every action
// node, loop, conditional, state and transition the mapping covers.
var behavioralModels = []string{"action_nodes", "loops_conditionals", "state_machine", "then_after_members"}

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
		"sysx:Pseudostate", "sysx:DeferMember",
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
		"final node":          "done;",
		"fork":                "fork split;",
		"join":                "join sync;",
		"merge":               "merge gather;",
		"decision":            "decide pick;",
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
		"edge succession":     "done;\n        succession first brake then finish;",
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
		"entry succession":     "entry;\n        then launch;",
		"start state":          "state launch;",
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
		"nested regions":       "state working parallel {\n            state left {\n                state stopped;\n            }\n            state right {\n                state moving;\n            }\n        }",
		"nested full body":     "state working {\n            entry perform Warm;\n            do action spin : Warm;\n            exit perform Warm;\n            defer sig;\n            state deeper;\n        }",
		"unordered subactions": "state working {\n            do action spin : Warm;\n            entry perform Warm;\n        }",
		"transition first":     "transition first idle then idle;",
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
		"@prefix sysx: <urn:opensysml:sysml:> .\n" +
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

// A `then` sequences from the member before it that is not an edge — past a
// flow, a bind, a succession, and past the members that declare no action at
// all — the way the parser reads it. The graph alone, without the text each
// member was written as, has to bring every such `then` back where it stood.
func TestThenComesBackPastTheMembersTheParserSkips(t *testing.T) {
	path := filepath.Join("testdata", "convert", "then_after_members.sysml")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	turtle := toTurtle(t, path)
	back, err := export.Convert("m.ttl", withoutTriples(t, []byte(turtle), "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the mapping alone: %v", err)
	}
	if string(back) != string(src) {
		t.Fatalf("the notation changed\n--- want ---\n%s\n--- got ---\n%s", src, back)
	}
	again, err := export.Convert("m.sysml", back, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	if string(again) != turtle {
		t.Errorf("the second hop changed the graph\n--- first ---\n%s\n--- second ---\n%s", turtle, again)
	}
}

// A usage that declares no name answers to the feature that names it, so a
// `then` after `perform walk;` or `action redefines walk;` sequences from walk.
func TestThenSequencesFromTheNameAnUnnamedUsageAnswersTo(t *testing.T) {
	for name, body := range map[string]string{
		"perform":    "    action walk : Step;\n    action def A {\n        perform walk;\n        then action b : Step;\n    }\n",
		"redefines":  "    action def Base {\n        action walk : Step;\n    }\n    action def A specializes Base {\n        action redefines walk;\n        then action b : Step;\n    }\n",
		"named":      "    action def A {\n        action walk : Step;\n        then action b : Step;\n    }\n",
		"introduced": "    action walk : Step;\n    action run : Step;\n    action def A {\n        perform walk;\n        then perform run;\n        then action b : Step;\n    }\n",
	} {
		t.Run(name, func(t *testing.T) {
			src := "package P {\n    action def Step;\n" + body + "}\n"
			turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := export.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation from the mapping alone: %v\n%s", err, turtle)
			}
			if string(back) != src {
				t.Fatalf("the notation changed\n--- want ---\n%s\n--- got ---\n%s", src, back)
			}
		})
	}
}

// A `then` folded into the member it introduces sequences from one member only,
// so a graph whose source end is any other member — an earlier action, or the
// flow the `then` is read past — cannot be written in that form and is refused.
func TestThenIsRefusedWhenTheGraphSequencesFromAnotherMember(t *testing.T) {
	src := "package P {\n    item def Signal;\n    action def Step {\n        in item x : Signal;\n        out item y : Signal;\n    }\n" +
		"    action def A {\n        action x : Step;\n        action a : Step;\n        flow from a.y to b.x;\n        then action b : Step;\n    }\n}\n"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	const stated = "sysml:sourceFeature elmt:P__A__a"
	if strings.Count(string(turtle), stated) != 1 {
		t.Fatalf("the succession should state its source once as %s:\n%s", stated, turtle)
	}
	for name, source := range map[string]string{
		"an earlier action":       "elmt:P__A__x",
		"the flow written before": "elmt:P__A___402",
	} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(string(turtle), "\n"+source+"\n") {
				t.Fatalf("%s is not an element of the graph:\n%s", source, turtle)
			}
			moved := strings.Replace(string(turtle), stated, "sysml:sourceFeature "+source, 1)
			_, err := export.Convert("m.ttl", []byte(moved), export.FormatTurtle, export.FormatSysML)
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("want an UnsupportedError, got %v", err)
			}
			if !strings.Contains(err.Error(), "sequences from the member written before the member it introduces") {
				t.Errorf("the refusal should say which order the graph states: %v", err)
			}
		})
	}
}

// The `else` branch is marked with a term the SysML metamodel does not define,
// so it belongs to the OpenSysML namespace and is read from there on its own —
// without the branch's keyword having to say `else` as well.
func TestElseBranchIsMarkedUnderTheExtensionNamespace(t *testing.T) {
	src := "package P {\n\taction def A {\n\t\tdecide d;\n\t\tif x then a;\n\t\telse b;\n\t\taction a;\n\t\taction b;\n\t\tattribute x : Boolean;\n\t}\n}"
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
	for _, subaction := range []string{
		"entry do { perform Warm; }",
		"entry do{ perform Warm; }",
		"entry do{perform Warm;}",
		"entry /* a note */ do { perform Warm; }",
		"entry do/* a note */{ perform Warm; }",
	} {
		src := "package P {\n    action def Warm;\n    state def Machine {\n        state s {\n" +
			"            " + subaction + "\n        }\n    }\n}\n"
		turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
		if err != nil {
			t.Fatalf("to turtle: %v", err)
		}
		if !strings.Contains(string(turtle), "sysx:declaredKeyword \"entry do\"") {
			t.Fatalf("the `do` of %q was not recorded:\n%s", subaction, turtle)
		}
		// With its source text the graph comes back as written; without, as the
		// printer writes the keyword the graph recorded.
		if back, want := toNotation(t, turtle), src; back != want {
			t.Fatalf("%q did not come back as written:\n--- want ---\n%s--- got ---\n%s", subaction, want, back)
		}
		back := toNotation(t, withoutTriples(t, turtle, "sysx:sourceText"))
		if !strings.Contains(back, "entry do {") {
			t.Fatalf("%q lost its `do`:\n%s", subaction, back)
		}
		// The notation it comes back as is what the printer writes, so converting
		// that again must change nothing at all.
		checkRoundTrip(t, back)
	}
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
