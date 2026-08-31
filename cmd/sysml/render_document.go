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
// document is staged in the directory under a fresh temporary name, existing
// destinations are set aside, and the staged set is renamed into place; any
// failure restores the directory's previous contents.
func commitDocumentSet(documents []repl.RenderedDocument) error {
	for _, document := range documents {
		path := filepath.Join(renderDocsDir, document.FileName)
		if info, err := os.Lstat(path); err == nil && info.IsDir() {
			return fmt.Errorf("cannot write %s: it is a directory", path)
		}
	}
	staged := make([]string, len(documents))
	backups := make([]string, len(documents))
	committed := make([]bool, len(documents))
	replaced := make([]bool, len(documents))
	rollback := func() {
		for i := range documents {
			path := filepath.Join(renderDocsDir, documents[i].FileName)
			if staged[i] != "" {
				_ = os.Remove(staged[i])
			}
			if committed[i] {
				_ = os.Remove(path)
			}
			if backups[i] != "" {
				_ = os.Rename(backups[i], path)
			}
		}
	}
	for i, document := range documents {
		name, err := stageDocument(document)
		if err != nil {
			rollback()
			return err
		}
		staged[i] = name
	}
	for i, document := range documents {
		path := filepath.Join(renderDocsDir, document.FileName)
		if _, err := os.Lstat(path); err != nil {
			continue
		}
		replaced[i] = true
		name, err := setAside(path)
		if err != nil {
			rollback()
			return fmt.Errorf("write %s: %w", path, err)
		}
		backups[i] = name
	}
	for i, document := range documents {
		path := filepath.Join(renderDocsDir, document.FileName)
		if err := os.Rename(staged[i], path); err != nil {
			rollback()
			return fmt.Errorf("write %s: %w", path, err)
		}
		staged[i] = ""
		committed[i] = true
	}
	for i, document := range documents {
		if backups[i] != "" {
			_ = os.Remove(backups[i])
		}
		path := filepath.Join(renderDocsDir, document.FileName)
		what := ""
		if replaced[i] {
			what = ", replaced the existing file"
		}
		out := len(strings.TrimRight(document.Markdown, "\n")) + 1
		fmt.Fprintf(os.Stderr, "wrote %s (%s, %d bytes%s)\n", path, view.FormMarkdown, out, what)
	}
	return nil
}

// stageDocument writes a document to a fresh temporary file in the rendering
// directory, so staging never touches a file the directory already holds.
func stageDocument(document repl.RenderedDocument) (string, error) {
	file, err := os.CreateTemp(renderDocsDir, ".sysml-staged-*")
	if err != nil {
		return "", fmt.Errorf("stage %s: %w", document.FileName, err)
	}
	name := file.Name()
	out := strings.TrimRight(document.Markdown, "\n") + "\n"
	if _, err := file.WriteString(out); err != nil {
		_ = file.Close()
		return name, fmt.Errorf("stage %s: %w", document.FileName, err)
	}
	if err := file.Close(); err != nil {
		return name, fmt.Errorf("stage %s: %w", document.FileName, err)
	}
	// #nosec G302 -- a rendered document is not a secret.
	if err := os.Chmod(name, 0o644); err != nil {
		return name, fmt.Errorf("stage %s: %w", document.FileName, err)
	}
	return name, nil
}

// setAside moves an existing destination to a fresh temporary name so a
// failed commit can restore it.
func setAside(path string) (string, error) {
	file, err := os.CreateTemp(renderDocsDir, ".sysml-replaced-*")
	if err != nil {
		return "", err
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	if err := os.Rename(path, name); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
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
