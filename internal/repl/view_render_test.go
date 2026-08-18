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
	res := s.Submit(`package Sequenced {
    private import StandardViewDefinitions::*;
    view messages : SequenceView {
        expose Demo::Vehicle;
    }
}`)
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("the view did not load: %v", res.Diagnostics)
		}
	}
	_, err := s.ViewRendering("Sequenced::messages")
	if !errors.Is(err, view.ErrUnsupportedKind) {
		t.Fatalf("err = %v, want view.ErrUnsupportedKind", err)
	}
	for _, want := range []string{"Sequenced::messages", "sequence"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to name %q", err, want)
		}
	}
}

// A view stating the tabular rendering renders rows: aligned columns as text, a
// Markdown table as the machine-readable form, and a typed error for Mermaid.
func TestRenderOfATabularView(t *testing.T) {
	s := viewSession(t)
	res := s.Submit(`package Tabular {
    private import Views::*;
    view parts {
        expose Demo::Vehicle;
        render asElementTable;
    }
}`)
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("the view did not load: %v", res.Diagnostics)
		}
	}
	text := run(t, s, "%render Tabular::parts")
	for _, want := range []string{"Tabular::parts", "table rendering", "Element", "Declared in", "Demo::Vehicle"} {
		if !strings.Contains(text, want) {
			t.Errorf("the table is missing %q:\n%s", want, text)
		}
	}
	if markdown := run(t, s, "%render Tabular::parts markdown"); !strings.Contains(markdown, "| Element | Kind | Type | Declared in |") {
		t.Errorf("the Markdown form is no table:\n%s", markdown)
	}
	if out := run(t, s, "%render Tabular::parts mermaid"); !strings.Contains(out, "error: ") {
		t.Errorf("Mermaid of a table = %v, want an error line naming the form", out)
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
	for _, form := range []string{"text", "mermaid", "markdown"} {
		if got := s.Complete("%render Demo::summary ", len("%render Demo::summary ")); !slices.Contains(got.Candidates, form) {
			t.Errorf("completing the form offered %v, want %s", got.Candidates, form)
		}
	}
	// A form has been typed already, and a third argument is no command, so no
	// further form is offered.
	for _, head := range []string{"%render Demo::summary text ", "%render Demo::summary text mer"} {
		for _, form := range renderForms() {
			if got := s.Complete(head, len(head)); slices.Contains(got.Candidates, form) {
				t.Errorf("completing past the form offered %s: %v", form, got.Candidates)
			}
		}
	}
}

// A view whose name is unrestricted is rendered, named and completed with its
// quotes: the name holding a space stays one argument, and the form is offered
// only once that name is closed.
func TestRenderOfAViewWithAnUnrestrictedName(t *testing.T) {
	s := viewSession(t)
	res := s.Submit(`package Quoted {
    view 'My Summary' {
        expose Demo::Vehicle;
    }
    view 'frame' {
        expose Demo::Wheel;
    }
}`)
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("the views did not load: %v", res.Diagnostics)
		}
	}
	if text := run(t, s, "%render Quoted::'My Summary'"); !strings.Contains(text, "Quoted::'My Summary' — tree rendering") {
		t.Errorf("a quoted view name is not rendered under its written name:\n%s", text)
	}
	// A name spelling a keyword is written with its quotes, so it can be typed back.
	if text := run(t, s, "%render Quoted::'frame'"); !strings.Contains(text, "Quoted::'frame' — tree rendering") {
		t.Errorf("a keyword view name is not written with its quotes:\n%s", text)
	}
	// A quoted name still being typed is not yet the form position, so the
	// space inside it offers no form.
	head := "%render Quoted::'My Sum"
	for _, form := range renderForms() {
		if got := s.Complete(head, len(head)); slices.Contains(got.Candidates, form) {
			t.Errorf("completing inside an unfinished quoted name offered the form %s: %v", form, got.Candidates)
		}
	}
	// A closed quoted name holding a space is one argument, so the form follows it.
	head = "%render Quoted::'My Summary' "
	if got := s.Complete(head, len(head)); !slices.Contains(got.Candidates, "mermaid") {
		t.Errorf("completing the form after a quoted name offered %v", got.Candidates)
	}
	// The notation's own escape closes no name, so a name holding a quote is
	// finished and the form follows it too.
	head = `%render Quoted::'it\'s' `
	if got := s.Complete(head, len(head)); !slices.Contains(got.Candidates, "mermaid") {
		t.Errorf("completing the form after an escaped quote offered %v", got.Candidates)
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
