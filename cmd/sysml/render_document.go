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
// document is staged beside its destination under a fresh temporary name,
// existing destinations are preserved through hard-linked backups, and the
// staged set is renamed over the destinations; any failure restores the
// directory's previous contents, and an interruption leaves every
// destination in place, old or new, never missing. A symlink is written
// through to its target, keeping the link. A pipe or device is written as it
// stands, last, after the regular set commits; bytes a pipe or device already
// received cannot be recalled, so the all-or-nothing guarantee covers the
// regular files of the set only.
func commitDocumentSet(documents []repl.RenderedDocument) error {
	targets := make([]string, len(documents))
	direct := make([]bool, len(documents))
	replaced := make([]bool, len(documents))
	perms := make([]os.FileMode, len(documents))
	// Two destinations may be symlinks resolving to one file; such a set
	// would silently keep only the later document.
	resolved := make(map[string]string, len(documents))
	infos := make([]os.FileInfo, len(documents))
	for i, document := range documents {
		path := filepath.Join(renderDocsDir, document.FileName)
		target, info, err := export.Destination(path)
		if err != nil {
			return fmt.Errorf("cannot write %s: %w", path, err)
		}
		if info != nil && info.IsDir() {
			return fmt.Errorf("cannot write %s: it is a directory", path)
		}
		if other, ok := resolved[target]; ok {
			return fmt.Errorf("cannot write %s: %s and %s both resolve to %s; repoint one of the links so each document has its own file", path, other, document.FileName, target)
		}
		resolved[target] = document.FileName
		for j := range i {
			if info != nil && infos[j] != nil && os.SameFile(info, infos[j]) {
				return fmt.Errorf("cannot write %s: %s and %s both resolve to one file; repoint one of the links so each document has its own file", path, documents[j].FileName, document.FileName)
			}
		}
		infos[i] = info
		targets[i] = target
		replaced[i] = info != nil
		perms[i] = 0o644
		if info != nil {
			perms[i] = info.Mode().Perm()
			direct[i] = !info.Mode().IsRegular()
		}
	}
	staged := make([]string, len(documents))
	backups := make([]string, len(documents))
	moved := make([]bool, len(documents))
	committed := make([]bool, len(documents))
	rollback := func() {
		for i := range documents {
			if staged[i] != "" {
				_ = os.Remove(staged[i])
			}
			switch {
			case committed[i] && backups[i] != "":
				// Windows refuses a rename over an existing file.
				_ = os.Remove(targets[i])
				_ = os.Rename(backups[i], targets[i])
			case committed[i]:
				_ = os.Remove(targets[i])
			case moved[i]:
				_ = os.Rename(backups[i], targets[i])
			case backups[i] != "":
				_ = os.Remove(backups[i])
			}
		}
	}
	for i, document := range documents {
		if direct[i] {
			continue
		}
		name, err := stageDocument(document, perms[i], filepath.Dir(targets[i]))
		staged[i] = name
		if err != nil {
			rollback()
			return err
		}
	}
	for i := range documents {
		if direct[i] || !replaced[i] {
			continue
		}
		name, movedAside, err := backUp(targets[i])
		if err != nil {
			rollback()
			return fmt.Errorf("write %s: %w", targets[i], err)
		}
		backups[i] = name
		moved[i] = movedAside
	}
	for i := range documents {
		if direct[i] {
			continue
		}
		if err := os.Rename(staged[i], targets[i]); err != nil {
			rollback()
			return fmt.Errorf("write %s: %w", targets[i], err)
		}
		staged[i] = ""
		committed[i] = true
	}
	// Flush the renames before the backups go, so a crash keeps one of the
	// two sets whole.
	dirs := make(map[string]bool)
	for i := range documents {
		if committed[i] {
			dirs[filepath.Dir(targets[i])] = true
		}
	}
	for dir := range dirs {
		syncDir(dir)
	}
	for i, document := range documents {
		if !direct[i] {
			continue
		}
		path := filepath.Join(renderDocsDir, document.FileName)
		if _, err := export.WriteFile(path, documentBytes(document)); err != nil {
			rollback()
			return err
		}
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
		fmt.Fprintf(os.Stderr, "wrote %s (%s, %d bytes%s)\n", path, view.FormMarkdown, len(documentBytes(document)), what)
	}
	return nil
}

// documentBytes is what a rendered document writes: the Markdown with one
// trailing newline.
func documentBytes(document repl.RenderedDocument) []byte {
	return []byte(strings.TrimRight(document.Markdown, "\n") + "\n")
}

// stageDocument writes a document to a fresh temporary file beside its
// destination, so staging never touches an existing file and the commit
// rename never crosses a filesystem.
func stageDocument(document repl.RenderedDocument, perm os.FileMode, dir string) (string, error) {
	file, err := os.CreateTemp(dir, ".sysml-staged-*")
	if err != nil {
		return "", fmt.Errorf("stage %s: %w", document.FileName, err)
	}
	name := file.Name()
	if _, err := file.Write(documentBytes(document)); err != nil {
		_ = file.Close()
		return name, fmt.Errorf("stage %s: %w", document.FileName, err)
	}
	// Without this a crash after the commit renames can leave an empty file
	// where a document was.
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return name, fmt.Errorf("stage %s: %w", document.FileName, err)
	}
	if err := file.Close(); err != nil {
		return name, fmt.Errorf("stage %s: %w", document.FileName, err)
	}
	// #nosec G302 -- a rendered document is not a secret, and an existing
	// destination keeps its permissions.
	if err := os.Chmod(name, perm); err != nil {
		return name, fmt.Errorf("stage %s: %w", document.FileName, err)
	}
	return name, nil
}

// syncDir flushes a directory's entries after renames. Best-effort: the set
// is already written, so a filesystem that refuses is not a failed render.
func syncDir(dir string) {
	// #nosec G304 -- dir holds a destination the caller asked to write.
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = f.Sync()
	_ = f.Close()
}

// backUp preserves an existing destination so a failed commit can restore
// it: a hard link keeps the destination itself in place through the commit,
// falling back to a set-aside rename where the filesystem has no hard links.
func backUp(target string) (name string, movedAside bool, err error) {
	file, err := os.CreateTemp(filepath.Dir(target), ".sysml-replaced-*")
	if err != nil {
		return "", false, err
	}
	name = file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(name)
		return "", false, err
	}
	if err := os.Remove(name); err != nil {
		return "", false, err
	}
	if os.Link(target, name) == nil {
		return name, false, nil
	}
	if err := os.Rename(target, name); err != nil {
		return "", false, err
	}
	return name, true, nil
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
