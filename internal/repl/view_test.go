package repl

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/passes"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
)

const viewModel = `package Demo {
    part def Vehicle;
    part def Wheel;
    part v : Vehicle;
    view def Overview;
    view summary : Overview {
        expose Demo::Vehicle;
        expose Demo::v;
        view detail {
            expose Demo::Wheel;
        }
    }
    view empty : Overview;
}`

func viewSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	res := s.Submit(viewModel)
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("model did not load: %v", res.Diagnostics)
		}
	}
	return s
}

func TestViewListsWhatItExposes(t *testing.T) {
	s := viewSession(t)
	out, _, err := s.RunMeta("%view Demo::summary")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(out, "\n")
	for _, want := range []string{"view Demo::summary", "exposes", "Demo::Vehicle", "Demo::v"} {
		if !strings.Contains(text, want) {
			t.Errorf("%%view output is missing %q:\n%s", want, text)
		}
	}
}

func TestViewListsNestedViews(t *testing.T) {
	s := viewSession(t)
	out, _, err := s.RunMeta("%view Demo::summary")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(out, "\n")
	if !strings.Contains(text, "nested views") || !strings.Contains(text, "detail") {
		t.Errorf("%%view did not report the nested view:\n%s", text)
	}
	// The nested view's own exposed set is asked of it directly, and is not
	// folded into its parent's.
	if strings.Contains(text, "Demo::Wheel") {
		t.Errorf("a nested view's exposures leaked into its parent's:\n%s", text)
	}
	out, _, err = s.RunMeta("%view Demo::summary::detail")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "Demo::Wheel") {
		t.Errorf("the nested view did not report its own exposures: %v", out)
	}
}

func TestViewExposingNothingIsNoError(t *testing.T) {
	s := viewSession(t)
	out, _, err := s.RunMeta("%view Demo::empty")
	if err != nil {
		t.Fatalf("a view exposing nothing errored: %v", err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "exposes nothing") {
		t.Errorf("out = %v, want it to say the view exposes nothing", out)
	}
}

func TestViewOfANonViewIsTyped(t *testing.T) {
	s := viewSession(t)
	if _, err := s.View("Demo::Vehicle"); !errors.Is(err, semantics.ErrNotAView) {
		t.Errorf("err = %v, want semantics.ErrNotAView", err)
	}
	// At the prompt it is a line, as a mistyped constraint or instance name is.
	out, _, err := s.RunMeta("%view Demo::Vehicle")
	if err != nil {
		t.Fatalf("a non-view should not fail the command: %v", err)
	}
	if text := strings.Join(out, "\n"); !strings.HasPrefix(text, "error: ") {
		t.Errorf("out = %v, want an error line", out)
	}
}

func TestViewOfAnUnknownNameReports(t *testing.T) {
	s := viewSession(t)
	out, _, err := s.RunMeta("%view Demo::Nope")
	if err != nil {
		t.Fatalf("an unknown name should not fail the command: %v", err)
	}
	if text := strings.Join(out, "\n"); !strings.HasPrefix(text, "error: ") {
		t.Errorf("view of an unknown name did not report anything: %v", out)
	}
}

func TestViewWithoutANameShowsUsage(t *testing.T) {
	s := viewSession(t)
	out, _, err := s.RunMeta("%view")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || !strings.HasPrefix(out[0], "usage:") {
		t.Errorf("out = %v, want the usage line", out)
	}
}

func TestViewIsInHelpAndCompletion(t *testing.T) {
	if !strings.Contains(strings.Join(helpText(), "\n"), "%view") {
		t.Error("the view command is dispatched but not in help")
	}
	found := false
	for _, name := range metaCommands() {
		if name == "%view" {
			found = true
		}
	}
	if !found {
		t.Error("the view command is not in the command table")
	}
}

const conformanceModel = `package Demo {
    part def Vehicle { attribute mass : ScalarValues::Real = 1200.0; }
    part def Wheel;
    part vehicle : Vehicle;
    part wheel : Wheel;
    concern def MassBudget { subject s : Vehicle; require constraint { s.mass < 1000.0 } }
    concern def Modularity { subject s : Vehicle; require constraint { s.mass > 1.0 } }
    concern def Documented { subject s : Vehicle; }
    viewpoint def StructurePerspective {
        stakeholder engineer;
        frame concern budget : MassBudget;
        frame concern modularity : Modularity;
        frame concern documented : Documented;
    }
    viewpoint structure : StructurePerspective;
    view def StructureView { satisfy structure; }
    view report : StructureView {
        expose Demo::vehicle;
        expose Demo::wheel;
        frame concern modularity : Modularity;
        frame concern documented : Documented;
    }
}`

func conformanceSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	res := s.Submit(conformanceModel)
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("model did not load: %v", res.Diagnostics)
		}
	}
	return s
}

// %view reports the conformance of a view to the viewpoint it satisfies: a
// concern that holds, one that does not, one the view never framed, and where
// the satisfy came from.
func TestViewReportsViewpointConformance(t *testing.T) {
	out, _, err := conformanceSession(t).RunMeta("%view Demo::report")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(out, "\n")
	for _, want := range []string{
		"viewpoint conformance",
		"satisfy structure (from Demo::StructureView)",
		"concern budget: violated (framed by the viewpoint but not by the view)",
		"concern modularity: conforms",
		"concern documented: unevaluable",
		"states no condition to evaluate",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("%%view output is missing %q:\n%s", want, text)
		}
	}
	// Only the elements the concern's subject admits are checked.
	if strings.Contains(text, "Demo::wheel: ") {
		t.Errorf("a concern was checked against an element its subject does not admit:\n%s", text)
	}
}

// A concern whose condition is false of an exposed element names that element.
func TestViewReportsAViolatedConcernPerElement(t *testing.T) {
	s := conformanceSession(t)
	res := s.Submit(`package Budgeted {
    private import Demo::*;
    view budgeted : StructureView {
        expose Demo::vehicle;
        frame concern budget : MassBudget;
        frame concern modularity : Modularity;
        frame concern documented : Documented;
    }
}`)
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("the view did not load: %v", res.Diagnostics)
		}
	}
	out, err := s.View("Budgeted::budgeted")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(out, "\n")
	for _, want := range []string{"concern budget: violated", "Demo::vehicle: ", "s.mass < 1000.0"} {
		if !strings.Contains(text, want) {
			t.Errorf("%%view output is missing %q:\n%s", want, text)
		}
	}
}

// A view satisfying nothing reports no conformance section at all.
func TestViewWithoutASatisfyReportsNoConformance(t *testing.T) {
	out, _, err := viewSession(t).RunMeta("%view Demo::summary")
	if err != nil {
		t.Fatal(err)
	}
	if text := strings.Join(out, "\n"); strings.Contains(text, "viewpoint conformance") {
		t.Errorf("a view satisfying no viewpoint reported conformance:\n%s", text)
	}
}

// A satisfy whose target is no viewpoint is reported as such, not silently
// treated as conformance.
func TestViewReportsASatisfyThatIsNoViewpoint(t *testing.T) {
	s := NewSession()
	s.Submit(`package Bad {
    part def Vehicle;
    part vehicle : Vehicle;
    requirement spec { subject s : Vehicle; }
    view v { expose Bad::vehicle; satisfy spec; }
}`)
	out, err := s.View("Bad::v")
	if err != nil {
		t.Fatal(err)
	}
	if text := strings.Join(out, "\n"); !strings.Contains(text, "not a viewpoint") {
		t.Errorf("out = %v, want it to say the satisfy target is not a viewpoint", out)
	}
}

// The conformance section is deterministic: the same model reported twice reads
// the same.
func TestViewConformanceOutputIsDeterministic(t *testing.T) {
	s := conformanceSession(t)
	first, err := s.View("Demo::report")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.View("Demo::report")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(first, "\n") != strings.Join(second, "\n") {
		t.Errorf("%%view output differs between runs:\n%v\n%v", first, second)
	}
}
