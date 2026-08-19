package main

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/chzyer/readline"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// runRender renders the view -render names of the model named on the command
// line, writing the artifact to -o or to stdout and every notice to stderr.
func runRender(files []string) error {
	form := view.Form(renderForm)
	if renderForm != "" && !slices.Contains(view.Forms(), form) {
		return fmt.Errorf("unknown rendering form %q; -render-form takes %s", renderForm, formList())
	}
	if len(files) == 0 {
		return errors.New("no model to render; name the file the view is declared in, as `sysml model.sysml -render MyView`")
	}
	if len(files) > 1 {
		return fmt.Errorf("-render renders a view of one model; unexpected extra argument %q", files[1])
	}

	sess := newSession()
	// Loaded onto stderr rather than through loadFiles: stdout carries the
	// artifact alone, so it can be piped into a tool that reads it.
	report, err := sess.LoadPathsReport(files)
	if err != nil {
		return err
	}
	writeLines(os.Stderr, report.Loaded)
	writeLines(os.Stderr, report.Found)
	writeLines(os.Stderr, report.Declared)
	if report.Errors {
		return fmt.Errorf("%s did not analyse cleanly; nothing was rendered", files[0])
	}

	rendering, err := sess.ViewRendering(renderView)
	if err != nil {
		return err
	}
	if form == "" {
		form = defaultRenderForm(rendering.Kind, outputPath, atStdoutTerminal())
	}
	artifact, err := rendering.WriteWidth(form, artifactWidth(outputPath, terminalWidth()))
	if err != nil {
		return err
	}
	reportRenderNotices(rendering)
	return writeArtifact(artifact, form)
}

// defaultRenderForm is the form -render writes where -render-form named none:
// the text form on a terminal, read by a person, and the machine-readable form
// of the kind rendered into a file or a pipe, read by a tool.
func defaultRenderForm(kind view.Kind, output string, terminal bool) view.Form {
	if output == "" && terminal {
		return view.FormText
	}
	return kind.MachineForm()
}

// artifactWidth is the width -render writes the text form to fit: view.WidthUnbounded
// into a file, so a saved artifact does not depend on the window it was written from.
func artifactWidth(output string, width int) int {
	if output != "" {
		return view.WidthUnbounded
	}
	return width
}

// terminalWidth is stdout's width, and view.WidthUnbounded where stdout is no
// terminal.
func terminalWidth() int {
	width, _, err := readline.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return view.WidthUnbounded
	}
	return width
}

// atStdoutTerminal reports whether the artifact is written to a terminal.
func atStdoutTerminal() bool { return readline.IsTerminal(int(os.Stdout.Fd())) }

// formList names the forms -render-form takes, as its help and errors spell them.
func formList() string {
	names := make([]string, 0, len(view.Forms()))
	for _, form := range view.Forms() {
		names = append(names, string(form))
	}
	return strings.Join(names, ", ")
}

// reportRenderNotices reports on stderr what the rendering says about itself: an
// empty artifact, and every element it could not represent.
func reportRenderNotices(rendering *view.Rendering) {
	if rendering.Empty() {
		fmt.Fprintf(os.Stderr, "note: %s renders empty\n", rendering.View)
	}
	for _, notice := range rendering.Notices {
		fmt.Fprintf(os.Stderr, "note: %s\n", notice)
	}
}

// writeArtifact writes the rendering to -o, or to stdout when no file was named.
func writeArtifact(artifact string, form view.Form) error {
	out := []byte(strings.TrimRight(artifact, "\n") + "\n")
	if outputPath == "" {
		_, err := os.Stdout.Write(out)
		return err
	}
	replaced, err := export.WriteFile(outputPath, out)
	if err != nil {
		return err
	}
	what := ""
	if replaced {
		what = ", replaced the existing file"
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%s, %d bytes%s)\n", outputPath, form, len(out), what)
	return nil
}
