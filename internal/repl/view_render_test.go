package repl

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/passes"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/view"
)

// A view stating no rendering renders as a containment tree, of what it exposes.
func TestRenderDefaultsToATree(t *testing.T) {
	out, _, err := viewSession(t).RunMeta("%render Demo::summary")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(out, "\n")
	for _, want := range []string{
		"Demo::summary — tree rendering",
		"the view states no rendering",
		"part def Demo::Vehicle",
		"part Demo::v (Vehicle)",
		"view Demo::summary::detail",
		"part def Demo::Wheel",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("%%render output is missing %q:\n%s", want, text)
		}
	}
}

// The Mermaid form is asked for by name, and is the same rendering.
func TestRenderWritesMermaidWhenAskedFor(t *testing.T) {
	out, _, err := viewSession(t).RunMeta("%render Demo::summary mermaid")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(out, "\n")
	if !strings.HasPrefix(text, "%% Demo::summary") || !strings.Contains(text, "flowchart TD") {
		t.Errorf("%%render mermaid = %q, want a Mermaid flowchart", text)
	}
}

// A view exposing nothing renders an empty artifact and says so.
func TestRenderOfAViewExposingNothingSaysSo(t *testing.T) {
	out, _, err := viewSession(t).RunMeta("%render Demo::empty")
	if err != nil {
		t.Fatalf("a view exposing nothing failed the command: %v", err)
	}
	if text := strings.Join(out, "\n"); !strings.Contains(text, "the rendering is empty") {
		t.Errorf("out = %v, want it to say the rendering is empty", out)
	}
}

// A name that is no view is the typed error %view returns, and a line at the
// prompt rather than a failed command.
func TestRenderOfANonViewIsTyped(t *testing.T) {
	s := viewSession(t)
	if _, err := s.ViewRendering("Demo::Vehicle"); !errors.Is(err, semantics.ErrNotAView) {
		t.Errorf("err = %v, want semantics.ErrNotAView", err)
	}
	out, _, err := s.RunMeta("%render Demo::Vehicle")
	if err != nil {
		t.Fatalf("a non-view should not fail the command: %v", err)
	}
	if text := strings.Join(out, "\n"); !strings.HasPrefix(text, "error: ") {
		t.Errorf("out = %v, want an error line", out)
	}
}

// A rendering kind this build does not produce names the kind and the view,
// rather than rendering something else.
func TestRenderOfAnUnsupportedKindNamesIt(t *testing.T) {
	s := viewSession(t)
	res := s.Submit(`package Tabular {
    private import Views::*;
    view table {
        expose Demo::Vehicle;
        render asElementTable;
    }
}`)
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("the view did not load: %v", res.Diagnostics)
		}
	}
	_, err := s.ViewRendering("Tabular::table")
	if !errors.Is(err, view.ErrUnsupportedKind) {
		t.Fatalf("err = %v, want view.ErrUnsupportedKind", err)
	}
	for _, want := range []string{"Tabular::table", "table"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to name %q", err, want)
		}
	}
}

func TestRenderOfAnUnknownNameReports(t *testing.T) {
	out, _, err := viewSession(t).RunMeta("%render Demo::Nope")
	if err != nil {
		t.Fatalf("an unknown name should not fail the command: %v", err)
	}
	if text := strings.Join(out, "\n"); !strings.HasPrefix(text, "error: ") {
		t.Errorf("out = %v, want an error line", out)
	}
}

func TestRenderMisuseShowsUsage(t *testing.T) {
	s := viewSession(t)
	for _, line := range []string{"%render", "%render Demo::summary dot"} {
		out, _, err := s.RunMeta(line)
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 1 || !strings.Contains(out[0], "%render <name>") {
			t.Errorf("%q = %v, want guidance naming the usage", line, out)
		}
	}
}

func TestRenderIsInHelpAndCompletion(t *testing.T) {
	if !strings.Contains(strings.Join(helpText(), "\n"), "%render") {
		t.Error("the render command is dispatched but not in help")
	}
	if !slices.Contains(metaCommands(), "%render") {
		t.Error("the render command is not in the command table")
	}
	s := viewSession(t)
	if got := s.Complete("%ren", len("%ren")); !slices.Contains(got.Candidates, "%render") {
		t.Errorf("completing %%ren offered %v, want %%render", got.Candidates)
	}
	// The view name completes as any name does, and the form after it does not.
	if got := s.Complete("%render Demo::sum", len("%render Demo::sum")); !slices.Contains(got.Candidates, "Demo::summary") {
		t.Errorf("completing a view name offered %v", got.Candidates)
	}
	if got := s.Complete("%render Demo::summary ", len("%render Demo::summary ")); !slices.Contains(got.Candidates, "mermaid") {
		t.Errorf("completing the form offered %v, want mermaid", got.Candidates)
	}
}

// %render is read-only with respect to the session: rendering between two steps
// of an action debugging session creates no object, ends no session, and leaves
// the object identities and the submission buffer as they were.
func TestRenderBetweenStepsDisturbsNothing(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	res := s.Submit(`package DebugViews {
    private import Views::*;
    view tallyView : StandardViewDefinitions::ActionFlowView {
        expose Debug::tally;
    }
}`)
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("the view did not load: %v", res.Diagnostics)
		}
	}
	run(t, s, "%instantiate Debug::tally")
	if started := run(t, s, "%action Debug::tally"); !strings.Contains(started, "Started action executor") {
		t.Fatalf("%%action failed: %s", started)
	}
	wants(t, run(t, s, "%step"), "✓ Step complete")

	before := s.actionExec
	beforeExecutor := before.executor
	beforeRuntime := s.rtCtx
	beforeIdentities := map[string]int64{}
	for name, obj := range s.instances {
		beforeIdentities[name] = obj.ID
	}
	beforeSnippets, beforeVersion := len(s.snippets), s.version

	rendered := run(t, s, "%render DebugViews::tallyView")
	if !strings.Contains(rendered, "action rendering") {
		t.Fatalf("%%render did not render the action: %s", rendered)
	}

	if s.actionExec != before || s.actionExec.executor != beforeExecutor {
		t.Error("%render replaced the action debugging session")
	}
	if s.rtCtx != beforeRuntime {
		t.Error("%render rebuilt the runtime context")
	}
	if len(s.instances) != len(beforeIdentities) {
		t.Errorf("objects after %%render = %v, want the %d there were", s.instances, len(beforeIdentities))
	}
	for name, id := range beforeIdentities {
		if got, ok := s.instances[name]; !ok || got.ID != id {
			t.Errorf("the object %s is now %v, want identity %d", name, got, id)
		}
	}
	if len(s.snippets) != beforeSnippets || s.version != beforeVersion {
		t.Errorf("submissions after %%render = %d at version %d, want %d at %d",
			len(s.snippets), s.version, beforeSnippets, beforeVersion)
	}
	// The session it was rendered in the middle of still runs to completion.
	wants(t, run(t, s, "%continue"), "✓ Action completed", "total = 5")
}
