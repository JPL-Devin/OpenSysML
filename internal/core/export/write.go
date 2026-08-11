package export

import (
	"fmt"
	"os"
	"path/filepath"
)

// fileMode is the permission a saved model is created with: readable like any
// other document the user writes, since a model is not a secret.
const fileMode = 0o644

// WriteFile writes a converted model to path, reporting whether it replaced a
// file that was already there.
//
// The write is atomic: the bytes go to a temporary file in the same directory
// and are renamed over path, so an interrupted or failing write leaves the
// previous model intact rather than a truncated one. A parent directory that
// does not exist is named in the error, which a bare open(2) failure does not
// make clear.
func WriteFile(path string, data []byte) (replaced bool, err error) {
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	switch {
	case os.IsNotExist(err):
		return false, fmt.Errorf("cannot write %s: directory %s does not exist", path, dir)
	case err != nil:
		return false, err
	case !info.IsDir():
		return false, fmt.Errorf("cannot write %s: %s is not a directory", path, dir)
	}
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return false, fmt.Errorf("cannot write %s: it is a directory", path)
		}
		replaced = true
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return replaced, err
	}
	name := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(name)
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return replaced, err
	}
	if err = tmp.Close(); err != nil {
		return replaced, err
	}
	// CreateTemp makes the file 0600; a saved model is an ordinary document.
	// #nosec G302 -- deliberate: see fileMode.
	if err = os.Chmod(name, fileMode); err != nil {
		return replaced, err
	}
	if err = os.Rename(name, path); err != nil {
		return replaced, err
	}
	return replaced, nil
}
