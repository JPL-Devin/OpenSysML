package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/export"
	"github.com/Open-MBEE/Systemica/internal/core/view"
)

// renderForms are the forms -render-form takes: the Mermaid diagram a tool reads,
// and the indented text a person reads.
const (
	formMermaid = "mermaid"
	formText    = "text"
)

// runRender renders the view -render names of the model named on the command
// line, writing the artifact to -o or to stdout and every notice to stderr.
func runRender(files []string) error {
	form := renderForm
	if form == "" {
		form = formMermaid
	}
	if form != formMermaid && form != formText {
		return fmt.Errorf("unknown rendering form %q; -render-form takes %s or %s", renderForm, formMermaid, formText)
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
	if report.Errors {
		return fmt.Errorf("%s did not analyse cleanly; nothing was rendered", files[0])
	}

	rendering, err := sess.ViewRendering(renderView)
	if err != nil {
		return err
	}
	artifact := rendering.Text()
	if form == formMermaid {
		artifact = rendering.Mermaid()
	}
	reportRenderNotices(rendering)
	return writeArtifact(artifact, form)
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
func writeArtifact(artifact, form string) error {
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
