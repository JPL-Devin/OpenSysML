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
