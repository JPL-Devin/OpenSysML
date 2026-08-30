package main

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// runRenderDocument renders the document -render-document names of the model
// named on the command line, writing the Markdown to -o or to stdout and
// every notice to stderr.
func runRenderDocument(files []string) error {
	if len(files) == 0 {
		return errors.New("no model to render; name the file the document is declared in, as `sysml model.sysml -render-document MyReport`")
	}
	if len(files) > 1 {
		return fmt.Errorf("-render-document renders a document of one model; unexpected extra argument %q", files[1])
	}
	sess, err := loadRenderingModel(files)
	if err != nil {
		return err
	}
	markdown, err := sess.RenderDocumentMarkdown(renderDoc)
	if err != nil {
		return err
	}
	return writeArtifact(markdown, view.FormMarkdown)
}
