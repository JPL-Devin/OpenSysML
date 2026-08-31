package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
	"github.com/Open-MBEE/OpenSysML/internal/docpdf"
	"github.com/Open-MBEE/OpenSysML/internal/repl"
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
		return errors.New("no model to render; name the files the documents are declared in, as `sysml model.sysml -render-documents rendered`")
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
	return commitDocumentSet(documents)
}

// commitDocumentSet writes a rendered document set all-or-nothing: every
// document is staged beside its destination first, and the set is committed
// by rename only after all staging succeeded, so a failure leaves whatever
// was in the directory untouched.
func commitDocumentSet(documents []repl.RenderedDocument) (err error) {
	for _, document := range documents {
		path := filepath.Join(renderDocsDir, document.FileName)
		if info, statErr := os.Lstat(path); statErr == nil && info.IsDir() {
			return fmt.Errorf("cannot write %s: it is a directory", path)
		}
	}
	staged := make([]string, 0, len(documents))
	defer func() {
		if err != nil {
			for _, path := range staged {
				if path != "" {
					_ = os.Remove(path)
				}
			}
		}
	}()
	for _, document := range documents {
		path := filepath.Join(renderDocsDir, document.FileName)
		out := []byte(strings.TrimRight(document.Markdown, "\n") + "\n")
		// #nosec G306 -- a rendered document is not a secret.
		if err = os.WriteFile(path+stagingSuffix, out, 0o644); err != nil {
			return fmt.Errorf("stage %s: %w", path, err)
		}
		staged = append(staged, path+stagingSuffix)
	}
	for i, document := range documents {
		path := filepath.Join(renderDocsDir, document.FileName)
		_, statErr := os.Lstat(path)
		if err = os.Rename(staged[i], path); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		staged[i] = ""
		what := ""
		if statErr == nil {
			what = ", replaced the existing file"
		}
		out := len(strings.TrimRight(document.Markdown, "\n")) + 1
		fmt.Fprintf(os.Stderr, "wrote %s (%s, %d bytes%s)\n", path, view.FormMarkdown, out, what)
	}
	return nil
}

// stagingSuffix marks a document staged beside its destination before the
// set commits.
const stagingSuffix = ".staged"

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
