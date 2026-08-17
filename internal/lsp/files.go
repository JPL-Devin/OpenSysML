package lsp

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/Systemica/internal/core/model"
)

//lint:file-ignore SA1019 rootUri/rootPath are deprecated, but see legacyRoot.

// maxScannedFileSize caps what the folder scan reads.
const maxScannedFileSize = 8 << 20

// setFolders records the folders the session was initialized with.
func (s *Server) setFolders(folders []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.folders = folders
}

// initializeFolders returns a session's folders, preferring workspaceFolders and
// falling back to the deprecated rootUri/rootPath older clients send instead.
func initializeFolders(params *protocol.InitializeParams) []string {
	var out []string
	seen := map[string]bool{}
	add := func(path string) {
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		out = append(out, path)
	}
	for _, folder := range params.WorkspaceFolders {
		add(uriToName(protocol.DocumentURI(folder.URI)))
	}
	if len(out) == 0 {
		add(legacyRoot(params))
	}
	return out
}

// legacyRoot returns the root a client that predates workspaceFolders sends;
// these deprecated fields are the only ones such a client populates.
func legacyRoot(params *protocol.InitializeParams) string {
	if params.RootURI != "" {
		return uriToName(params.RootURI)
	}
	return params.RootPath
}

// loadFolders indexes every model source under the session's folders, so a name
// declared in a file the editor never opened still resolves.
func (s *Server) loadFolders(ctx context.Context) {
	s.mu.Lock()
	folders := s.folders
	s.mu.Unlock()

	for _, folder := range folders {
		s.loadFolder(folder)
	}
	s.refreshOpenDiagnostics(ctx)
}

// loadFolder reads the model sources one folder holds, skipping hidden and
// vendored directories and passing over entries it cannot read.
func (s *Server) loadFolder(folder string) {
	if folder == "" {
		return
	}
	_ = filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != folder && skipDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !model.IsModelSource(path) {
			return nil
		}
		if info, statErr := d.Info(); statErr != nil || info.Size() > maxScannedFileSize {
			return nil
		}
		s.loadFromDisk(path)
		return nil
	})
}

// skipDir reports whether a directory holds no model sources worth indexing.
func skipDir(name string) bool {
	return strings.HasPrefix(name, ".") || name == "node_modules"
}

// loadFromDisk records a file's bytes as its on-disk content, reindexing it
// unless an open buffer is authoritative. An unreadable file is treated as gone.
func (s *Server) loadFromDisk(path string) {
	// #nosec G304 -- path comes from the folder the client asked to serve.
	content, err := os.ReadFile(path)
	if err != nil {
		s.ws.DeleteOnDisk(path)
		return
	}
	s.ws.SetOnDisk(path, content)
}

// DidChangeWatchedFiles reindexes model files changed outside the editor.
// A deletion leaves an open buffer alone; it is still authoritative.
func (s *Server) DidChangeWatchedFiles(ctx context.Context, params *protocol.DidChangeWatchedFilesParams) error {
	changed := false
	for _, event := range params.Changes {
		name := uriToName(event.URI)
		if !model.IsModelSource(name) {
			continue
		}
		changed = true
		if event.Type == protocol.FileChangeTypeDeleted {
			s.ws.DeleteOnDisk(name)
			continue
		}
		s.loadFromDisk(name)
	}
	if changed {
		s.refreshOpenDiagnostics(ctx)
	}
	return nil
}

// refreshOpenDiagnostics republishes diagnostics for the open documents, whose
// analysis depends on the rest of the workspace.
func (s *Server) refreshOpenDiagnostics(ctx context.Context) {
	for _, name := range s.ws.OpenNames() {
		s.publishDiagnostics(ctx, name)
	}
}
