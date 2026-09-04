package libs

import (
	"fmt"
	"sync"
)

// snapshotSource serves a Source's files as they were first read through it,
// so an index built from it and the text served for that index are one set of
// bytes however the underlying files change afterwards. Once sealed it serves
// only the files read so far: the set the index was built from.
type snapshotSource struct {
	inner  Source
	mu     sync.Mutex
	names  []string
	files  map[string]snapshotFile
	sealed bool
}

type snapshotFile struct {
	content []byte
	err     error
}

func newSnapshotSource(inner Source) *snapshotSource {
	return &snapshotSource{inner: inner, files: map[string]snapshotFile{}}
}

func (s *snapshotSource) List() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.names == nil {
		s.names = append([]string{}, s.inner.List()...)
	}
	return append([]string(nil), s.names...)
}

func (s *snapshotSource) Read(name string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.files[name]
	if !ok {
		if s.sealed {
			return nil, fmt.Errorf("libs: %q is not a file of the loaded library", name)
		}
		f.content, f.err = s.inner.Read(name)
		s.files[name] = f
	}
	return f.content, f.err
}

// seal ends reading through to the underlying source.
func (s *snapshotSource) seal() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sealed = true
}
