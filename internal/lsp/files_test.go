package lsp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/Systemica/internal/core/model"
)

const (
	libSource  = "package Lib {\n    part def Widget;\n}\n"
	mainSource = "package Main {\n    import Lib::*;\n    part w : Widget;\n}\n"
)

// multiFileWorkspace writes lib.sysml and main.sysml into a temporary folder and
// returns a server initialized on it, with the folder scan already done.
func multiFileWorkspace(t *testing.T) (*Server, *fakeClient, string, string, string) {
	t.Helper()
	dir := t.TempDir()
	lib := filepath.Join(dir, "lib.sysml")
	main := filepath.Join(dir, "main.sysml")
	if err := os.WriteFile(lib, []byte(libSource), 0o600); err != nil {
		t.Fatalf("write lib: %v", err)
	}
	if err := os.WriteFile(main, []byte(mainSource), 0o600); err != nil {
		t.Fatalf("write main: %v", err)
	}

	s := NewServer(model.NewWorkspace())
	fc := &fakeClient{}
	s.client = fc
	ctx := context.Background()
	if _, err := s.Initialize(ctx, &protocol.InitializeParams{
		WorkspaceFolders: []protocol.WorkspaceFolder{{URI: string(uri.File(dir)), Name: "multi"}},
	}); err != nil {
		t.Fatalf("Initialize err = %v", err)
	}
	if err := s.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		t.Fatalf("Initialized err = %v", err)
	}
	return s, fc, dir, lib, main
}

// diagnosticsFor returns the messages of the last diagnostics published for name.
func diagnosticsFor(fc *fakeClient, name string) []string {
	var msgs []string
	for _, p := range fc.published {
		if p.URI != uri.File(name) {
			continue
		}
		msgs = msgs[:0]
		for _, d := range p.Diagnostics {
			msgs = append(msgs, d.Message)
		}
	}
	return msgs
}

func openFile(t *testing.T, s *Server, path, text string) {
	t.Helper()
	if err := s.DidOpen(context.Background(), &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        uri.File(path),
			LanguageID: "sysml",
			Version:    1,
			Text:       text,
		},
	}); err != nil {
		t.Fatalf("DidOpen err = %v", err)
	}
}

func TestFolderScanResolvesNamesFromUnopenedFiles(t *testing.T) {
	s, fc, _, lib, main := multiFileWorkspace(t)

	// Only main.sysml is opened; Lib::Widget lives in a file the editor never
	// showed, and must still resolve.
	openFile(t, s, main, mainSource)
	if msgs := diagnosticsFor(fc, main); len(msgs) != 0 {
		t.Fatalf("diagnostics for main = %v, want none", msgs)
	}
	if s.ws.Document(lib) == nil {
		t.Errorf("lib.sysml not indexed by the folder scan")
	}
	if s.ws.IsOpen(lib) {
		t.Errorf("lib.sysml reported open; the scan records on-disk content only")
	}
}

func TestFolderScanSkipsHiddenDirectories(t *testing.T) {
	s, _, dir, _, _ := multiFileWorkspace(t)

	hidden := filepath.Join(dir, ".git", "hidden.sysml")
	if err := os.MkdirAll(filepath.Dir(hidden), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(hidden, []byte("package Hidden;\n"), 0o600); err != nil {
		t.Fatalf("write hidden: %v", err)
	}
	s.loadFolder(dir)
	if s.ws.Document(hidden) != nil {
		t.Errorf("indexed %q, want hidden directories skipped", hidden)
	}
}

func TestWatchedFileCreateAndChangeReindex(t *testing.T) {
	s, fc, dir, _, main := multiFileWorkspace(t)
	ctx := context.Background()
	openFile(t, s, main, mainSource)

	// A second definition file appears outside the editor; main.sysml can use it
	// as soon as the client reports the creation.
	extra := filepath.Join(dir, "extra.sysml")
	if err := os.WriteFile(extra, []byte("package Extra {\n    part def Gizmo;\n}\n"), 0o600); err != nil {
		t.Fatalf("write extra: %v", err)
	}
	if err := s.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []*protocol.FileEvent{{URI: uri.File(extra), Type: protocol.FileChangeTypeCreated}},
	}); err != nil {
		t.Fatalf("DidChangeWatchedFiles err = %v", err)
	}
	if s.ws.Document(extra) == nil {
		t.Fatalf("extra.sysml not indexed after a create event")
	}

	usesGizmo := "package Main {\n    import Extra::*;\n    part g : Gizmo;\n}\n"
	if err := s.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri.File(main)},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{{Text: usesGizmo}},
	}); err != nil {
		t.Fatalf("DidChange err = %v", err)
	}
	if msgs := diagnosticsFor(fc, main); len(msgs) != 0 {
		t.Fatalf("diagnostics for main = %v, want none", msgs)
	}

	// Rewriting the file on disk renames what it declares; the open document's
	// diagnostics follow.
	if err := os.WriteFile(extra, []byte("package Extra {\n    part def Doohickey;\n}\n"), 0o600); err != nil {
		t.Fatalf("rewrite extra: %v", err)
	}
	if err := s.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []*protocol.FileEvent{{URI: uri.File(extra), Type: protocol.FileChangeTypeChanged}},
	}); err != nil {
		t.Fatalf("DidChangeWatchedFiles err = %v", err)
	}
	if msgs := diagnosticsFor(fc, main); len(msgs) == 0 {
		t.Fatalf("diagnostics for main = none, want an unresolved Gizmo")
	}
}

func TestWatchedFileDeleteUnindexesClosedDocument(t *testing.T) {
	s, fc, _, lib, main := multiFileWorkspace(t)
	ctx := context.Background()
	openFile(t, s, main, mainSource)

	if err := os.Remove(lib); err != nil {
		t.Fatalf("remove lib: %v", err)
	}
	if err := s.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []*protocol.FileEvent{{URI: uri.File(lib), Type: protocol.FileChangeTypeDeleted}},
	}); err != nil {
		t.Fatalf("DidChangeWatchedFiles err = %v", err)
	}
	if s.ws.Document(lib) != nil {
		t.Errorf("lib.sysml still indexed after deletion")
	}
	if msgs := diagnosticsFor(fc, main); len(msgs) == 0 {
		t.Fatalf("diagnostics for main = none, want unresolved references")
	}
}

func TestWatchedFileDeleteKeepsOpenBuffer(t *testing.T) {
	s, fc, _, lib, main := multiFileWorkspace(t)
	ctx := context.Background()
	openFile(t, s, main, mainSource)
	openFile(t, s, lib, libSource)

	// The file is gone from disk, but its editor buffer is still authoritative.
	if err := os.Remove(lib); err != nil {
		t.Fatalf("remove lib: %v", err)
	}
	if err := s.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []*protocol.FileEvent{{URI: uri.File(lib), Type: protocol.FileChangeTypeDeleted}},
	}); err != nil {
		t.Fatalf("DidChangeWatchedFiles err = %v", err)
	}
	if s.ws.Document(lib) == nil {
		t.Fatalf("open buffer for lib.sysml dropped on disk deletion")
	}
	if msgs := diagnosticsFor(fc, main); len(msgs) != 0 {
		t.Fatalf("diagnostics for main = %v, want none", msgs)
	}
}

func TestDidCloseKeepsDocumentIndexedFromDisk(t *testing.T) {
	s, fc, _, lib, main := multiFileWorkspace(t)
	ctx := context.Background()
	openFile(t, s, main, mainSource)
	openFile(t, s, lib, libSource)

	if err := s.DidClose(ctx, &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(lib)},
	}); err != nil {
		t.Fatalf("DidClose err = %v", err)
	}
	if s.ws.Document(lib) == nil {
		t.Fatalf("lib.sysml unindexed by closing its tab")
	}
	if msgs := diagnosticsFor(fc, main); len(msgs) != 0 {
		t.Fatalf("diagnostics for main = %v, want none", msgs)
	}
}

func TestDidCloseDropsDocumentWithNoFileOnDisk(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	s.client = &fakeClient{}
	name := uri.File(filepath.Join(t.TempDir(), "untitled.sysml")).Filename()
	openFile(t, s, name, "package P;\n")

	if err := s.DidClose(context.Background(), &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
	}); err != nil {
		t.Fatalf("DidClose err = %v", err)
	}
	if s.ws.Document(name) != nil {
		t.Errorf("document with no file on disk kept after close")
	}
}

func TestInitializeFoldersFallsBackToRoot(t *testing.T) {
	dir := t.TempDir()
	got := initializeFolders(&protocol.InitializeParams{RootURI: uri.File(dir)})
	if len(got) != 1 || got[0] != uri.File(dir).Filename() {
		t.Errorf("folders = %v, want [%s]", got, uri.File(dir).Filename())
	}
	got = initializeFolders(&protocol.InitializeParams{RootPath: dir})
	if len(got) != 1 || got[0] != dir {
		t.Errorf("folders = %v, want [%s]", got, dir)
	}
}
