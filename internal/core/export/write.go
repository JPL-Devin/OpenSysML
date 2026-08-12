package export

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// newFileMode is the permission a model that did not exist yet is created with:
// readable like any other document the user writes, since a model is not a
// secret. A file that does exist keeps the permissions it has — a save is an
// edit of the user's file, not a decision about who may read it.
const newFileMode = 0o644

// WriteFile writes a converted model to path, reporting whether it replaced a
// file that was already there.
//
// The write is atomic: the bytes go to a temporary file in the same directory,
// are flushed to disk, and are renamed over path, so an interrupted or failing
// write leaves the previous model intact rather than a truncated or empty one.
// A symlink is written through rather than replaced, and an existing file keeps
// its permissions. A parent directory that does not exist is named in the
// error, which a bare open(2) failure does not make clear.
func WriteFile(path string, data []byte) (replaced bool, err error) {
	// The temporary file has to be created beside the file that is actually
	// being replaced, both so the rename stays within one filesystem and so
	// writing to a symlink updates what it points at instead of unlinking it.
	target, err := resolveLink(path)
	if err != nil {
		return false, err
	}
	dir := filepath.Dir(target)
	info, err := os.Stat(dir)
	switch {
	case os.IsNotExist(err):
		return false, fmt.Errorf("cannot write %s: directory %s does not exist", path, dir)
	case err != nil:
		return false, err
	case !info.IsDir():
		return false, fmt.Errorf("cannot write %s: %s is not a directory", path, dir)
	}
	mode := os.FileMode(newFileMode)
	if info, err := os.Stat(target); err == nil {
		if info.IsDir() {
			return false, fmt.Errorf("cannot write %s: it is a directory", path)
		}
		replaced = true
		mode = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(target)+".*")
	if err != nil {
		// The temporary name is ours, not something the user asked for, so the
		// failure is reported against the path they did name.
		return false, fmt.Errorf("cannot write %s: %w", path, underlying(err))
	}
	name := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(name)
			err = fmt.Errorf("cannot write %s: %w", path, underlying(err))
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return replaced, err
	}
	// Without this the rename can reach the disk before the bytes do, which
	// after a crash leaves an empty file where the previous model was.
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return replaced, err
	}
	if err = tmp.Close(); err != nil {
		return replaced, err
	}
	// CreateTemp makes the file 0600, which is not what a saved document is.
	// #nosec G302 -- deliberate: a new model gets newFileMode, an existing one
	// keeps the permissions it already had.
	if err = os.Chmod(name, mode); err != nil {
		return replaced, err
	}
	if err = os.Rename(name, target); err != nil {
		return replaced, err
	}
	syncDir(dir)
	return replaced, nil
}

// syncDir flushes the directory entry the rename created. It is best-effort:
// the model is already written, so a filesystem that does not allow this is no
// reason to report the save as failed.
func syncDir(dir string) {
	// #nosec G304 -- dir is the directory of the path the caller asked to write.
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = f.Sync()
	_ = f.Close()
}

// resolveLink returns the file a save should actually write: path itself, or
// what it points at when it is a symlink, so that saving over a linked model
// updates the model rather than turning the link into a regular file. A
// dangling link resolves to nothing, so path is written as it stands.
func resolveLink(path string) (string, error) {
	info, err := os.Lstat(path)
	switch {
	case os.IsNotExist(err):
		return path, nil
	case err != nil:
		return "", err
	case info.Mode()&os.ModeSymlink == 0:
		return path, nil
	}
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		// A link into a directory that does not exist yet: report it against
		// the destination rather than the link.
		if resolved, rerr := os.Readlink(path); rerr == nil {
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(filepath.Dir(path), resolved)
			}
			return resolved, nil
		}
		return "", err
	}
	return target, nil
}

// underlying strips the wrapper *os.PathError adds, whose path is the temporary
// file the caller never named.
func underlying(err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}
	return err
}
