package main

import (
	"errors"
	"fmt"
	"os"

	"path/filepath"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
	"github.com/Open-MBEE/OpenSysML/internal/docpdf"
)

// runRenderDocument renders the document -render-document names of the model
// named on the command line, writing the artifact to -o or to stdout and
// every notice to stderr.
func runRenderDocument(files []string) error {
	form, err := documentForm()
	if err != nil {
		return err
	}
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
	if form == docFormPDF {
		pdf, err := docpdf.Render(markdown, pdfEngine, docpdf.Options{
			TitlePage:      pdfTitlePage,
			TOC:            pdfTOC,
			NumberSections: pdfNumbering,
		})
		if err != nil {
			return err
		}
		return writePDFArtifact(pdf)
	}
	return writeArtifact(markdown, view.FormMarkdown)
}

// runRenderDocuments renders every document definition of the model named on
// the command line as linked Markdown files in the directory -render-documents
// names, so cross-document references resolve on disk.
func runRenderDocuments(files []string) error {
	if len(files) == 0 {
		return errors.New("no model to render; name the file the documents are declared in, as `sysml model.sysml -render-documents rendered`")
	}
	if len(files) > 1 {
		return fmt.Errorf("-render-documents renders the documents of one model; unexpected extra argument %q", files[1])
	}
	sess, err := loadRenderingModel(files)
	if err != nil {
		return err
	}
	documents, err := sess.RenderDocumentSetMarkdown()
	if err != nil {
		return err
	}
	if len(documents) == 0 {
		return errors.New("the model declares no documents; nothing was rendered")
	}
	if err := os.MkdirAll(renderDocsDir, 0o750); err != nil {
		return fmt.Errorf("create rendering directory %s: %w", renderDocsDir, err)
	}
	for _, document := range documents {
		path := filepath.Join(renderDocsDir, document.FileName)
		if err := writeArtifactFile(path, document.Markdown, view.FormMarkdown); err != nil {
			return err
		}
	}
	return nil
}

// The forms -doc-form takes.
const (
	docFormMarkdown = "markdown"
	docFormPDF      = "pdf"
)

// documentForm resolves -doc-form and checks the flag combination: the PDF
// options apply to PDF output, and a PDF is written to a file, not stdout.
func documentForm() (string, error) {
	form := docForm
	if form == "" {
		form = docFormMarkdown
	}
	switch form {
	case docFormMarkdown:
		if pdfEngine != "" || pdfTitlePage || pdfTOC || pdfNumbering {
			return "", errors.New("-pdf-engine, -pdf-title-page, -pdf-toc and -pdf-number-sections shape PDF output; ask for it with -doc-form pdf")
		}
	case docFormPDF:
		if outputPath == "" {
			return "", errors.New("-doc-form pdf writes a binary artifact; name the file to write with -o")
		}
	default:
		return "", fmt.Errorf("unknown document form %q; -doc-form takes %s or %s", form, docFormMarkdown, docFormPDF)
	}
	return form, nil
}

// writePDFArtifact writes the PDF bytes to -o, byte-exact.
func writePDFArtifact(pdf []byte) error {
	replaced, err := export.WriteFile(outputPath, pdf)
	if err != nil {
		return err
	}
	what := ""
	if replaced {
		what = ", replaced the existing file"
	}
	fmt.Fprintf(os.Stderr, "wrote %s (pdf, %d bytes%s)\n", outputPath, len(pdf), what)
	return nil
}
