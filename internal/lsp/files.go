package lsp

import (
	"context"
	"errors"
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
	s.refreshOpenDiagnostics(ctx, "")
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
// unless an open buffer is authoritative. A file that is gone is forgotten; any
// other read failure leaves what was indexed in place, since it may be transient.
func (s *Server) loadFromDisk(path string) {
	// #nosec G304 -- path comes from the folder the client asked to serve.
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			s.ws.DeleteOnDisk(path)
		}
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
			s.publishDiagnostics(ctx, name)
			continue
		}
		s.loadFromDisk(name)
	}
	if changed {
		s.refreshOpenDiagnostics(ctx, "")
	}
	return nil
}

// DidChangeWorkspaceFolders indexes the model sources a folder added to the
// session holds, and drops those a removed folder contributed. The file watcher
// reports changes only, so a newly added folder has to be walked here.
func (s *Server) DidChangeWorkspaceFolders(ctx context.Context, params *protocol.DidChangeWorkspaceFoldersParams) error {
	for _, folder := range params.Event.Removed {
		s.dropFolder(uriToName(protocol.DocumentURI(folder.URI)))
	}
	for _, folder := range params.Event.Added {
		s.addFolder(uriToName(protocol.DocumentURI(folder.URI)))
	}
	s.refreshOpenDiagnostics(ctx, "")
	return nil
}

// addFolder records a folder and indexes the model sources under it.
func (s *Server) addFolder(folder string) {
	if folder == "" {
		return
	}
	s.mu.Lock()
	for _, known := range s.folders {
		if known == folder {
			s.mu.Unlock()
			return
		}
	}
	s.folders = append(s.folders, folder)
	s.mu.Unlock()
	s.loadFolder(folder)
}

// dropFolder forgets a folder and the documents it contributed; an open buffer
// stays, since the editor still shows it.
func (s *Server) dropFolder(folder string) {
	if folder == "" {
		return
	}
	s.mu.Lock()
	kept := s.folders[:0]
	for _, known := range s.folders {
		if known != folder {
			kept = append(kept, known)
		}
	}
	s.folders = kept
	s.mu.Unlock()

	for _, name := range s.ws.DocumentNames() {
		if underFolder(name, folder) {
			s.ws.DeleteOnDisk(name)
		}
	}
}

// underFolder reports whether name lies inside folder.
func underFolder(name, folder string) bool {
	rel, err := filepath.Rel(folder, name)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// refreshOpenDiagnostics republishes diagnostics for the open documents, whose
// analysis depends on the rest of the workspace, skipping the name a caller has
// just published on its own.
func (s *Server) refreshOpenDiagnostics(ctx context.Context, except string) {
	for _, name := range s.ws.OpenNames() {
		if name != except {
			s.publishDiagnostics(ctx, name)
		}
	}
}

// queueOpenDiagnostics is refreshOpenDiagnostics once an editor burst settles;
// a refresh re-analyzes every open document, too much to pay per keystroke.
func (s *Server) queueOpenDiagnostics(ctx context.Context, except string) {
	if s.crossDoc == nil {
		s.refreshOpenDiagnostics(ctx, except)
		return
	}
	// The sweep outlives the notification, whose context is cancelled on return.
	ctx = context.WithoutCancel(ctx)
	s.crossDoc.Trigger(except, func() { s.refreshOpenDiagnostics(ctx, except) })
}
